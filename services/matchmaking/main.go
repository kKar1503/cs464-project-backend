package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

var (
	db          *sql.DB
	redisClient *redis.Client
)

func main() {
	log.Println("Matchmaking service starting...")

	// Initialize database connection
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to MySQL database")

	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	// Start background matchmaker
	go matchmakerLoop()

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/matchmaking/join", handleJoinQueue)
	mux.HandleFunc("/matchmaking/leave", handleLeaveQueue)
	mux.HandleFunc("/matchmaking/status", handleQueueStatus)
	mux.HandleFunc("/matchmaking/match", handleCheckMatch)
	mux.HandleFunc("/health", handleHealth)

	// Get port from environment
	port := os.Getenv("SERVICE_PORT")
	if port == "" {
		port = "8001"
	}

	// Create server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Matchmaking service listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// matchmakerLoop runs continuously to find matches
func matchmakerLoop() {
	ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds
	defer ticker.Stop()

	log.Println("Matchmaker loop started")

	for range ticker.C {
		if err := findMatches(); err != nil {
			log.Printf("Error in matchmaker: %v", err)
		}
	}
}

// findMatches attempts to match players in the queue
func findMatches() error {
	// Get all players in queue ordered by join time (FIFO fairness)
	rows, err := db.Query(`
		SELECT q.user_id, u.username, q.mmr, q.joined_at
		FROM matchmaking_queue q
		JOIN users u ON q.user_id = u.id
		WHERE u.is_banned = FALSE
		ORDER BY q.joined_at ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var queue []QueueEntry
	for rows.Next() {
		var entry QueueEntry
		if err := rows.Scan(&entry.UserID, &entry.Username, &entry.MMR, &entry.JoinedAt); err != nil {
			log.Printf("Error scanning queue entry: %v", err)
			continue
		}
		queue = append(queue, entry)
	}

	if len(queue) < 2 {
		return nil // Not enough players
	}

	ranges := make([]mmrRange, len(queue))
	for i, player := range queue {
		waitTime := int(time.Since(player.JoinedAt).Seconds())
		ranges[i] = calculateMMRRange(player.MMR, waitTime)
	}

	// Try to match players
	matched := make(map[int64]bool)

	for i := range queue {
		if matched[queue[i].UserID] {
			continue
		}

		player1 := queue[i]
		range1 := ranges[i]

		// Find best match for player1
		for j := i + 1; j < len(queue); j++ {
			if matched[queue[j].UserID] {
				continue
			}

			player2 := queue[j]
			range2 := ranges[j]

			// Check if ranges overlap
			if rangesOverlap(range1, range2) {
				// Match found!
				if _, err := createGameSession(player1, player2); err != nil {
					log.Printf("Failed to create game session: %v", err)
					continue
				}

				matched[player1.UserID] = true
				matched[player2.UserID] = true
				break
			}
		}
	}

	return nil
}

// handleCheckMatch allows a player to check if they've been matched
func handleCheckMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		respondError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Check Redis for match
	ctx := context.Background()
	matchKey := fmt.Sprintf("match:%s", userID)
	matchJSON, err := redisClient.Get(ctx, matchKey).Result()
	if err != nil {
		// No match found yet
		respondJSON(w, map[string]interface{}{
			"matched": false,
		}, http.StatusOK)
		return
	}

	// Match found! Return the match details
	var match MatchFoundResponse
	if err := json.Unmarshal([]byte(matchJSON), &match); err != nil {
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Delete the match notification (one-time retrieval)
	redisClient.Del(ctx, matchKey)

	respondJSON(w, map[string]interface{}{
		"matched":    true,
		"session_id": match.SessionID,
		"opponent":   match.Opponent,
		"your_mmr":   match.YourMMR,
		"their_mmr":  match.TheirMMR,
	}, http.StatusOK)
}

// Health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"matchmaking"}`))
}
