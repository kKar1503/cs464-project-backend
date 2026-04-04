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
	"github.com/gorilla/websocket"
	db "github.com/kKar1503/cs464-backend/db/sqlc"
	"github.com/redis/go-redis/v9"
)

var (
	sqlDB           *sql.DB
	queries         *db.Queries
	redisClient     *redis.Client
	stateManager    *GameStateManager
	upgrader        = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// TODO: Add proper origin checking for production
			return true
		},
	}
)

func main() {
	log.Println("Gameplay service starting...")

	// Initialize database connection
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	sqlDB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer sqlDB.Close()

	// Test database connection
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	queries = db.New(sqlDB)
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

	// Initialize game state manager
	stateManager = NewGameStateManager()
	log.Println("Game state manager initialized")

	// Start cleanup goroutine for inactive sessions
	go sessionCleanupLoop()

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/ws", handleWebSocket)
	mux.HandleFunc("/game/stats", handleGameStats)

	// Get port from environment
	port := os.Getenv("SERVICE_PORT")
	if port == "" {
		port = "8002"
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
		log.Printf("Gameplay service listening on port %s", port)
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

// handleHealth returns service health status
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"gameplay"}`))
}

// handleWebSocket handles WebSocket connection requests
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	// Authenticate via token
	token := extractToken(r)
	if token == "" {
		http.Error(w, "Missing authorization token", http.StatusUnauthorized)
		return
	}

	authUser, err := validateToken(token)
	if err != nil {
		log.Printf("Auth validation failed: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := authUser.UserID
	username := authUser.Username

	// Fetch session info from database
	sessionInfo, err := queries.GetGameSessionPlayers(r.Context(), sessionID)
	if err != nil {
		log.Printf("Failed to fetch session %s: %v", sessionID, err)
		http.Error(w, "Session not found or not active", http.StatusNotFound)
		return
	}
	player1ID := sessionInfo.Player1ID
	player2ID := sessionInfo.Player2ID
	player1Name := sessionInfo.Username
	player2Name := sessionInfo.Username_2

	// Determine which player this is
	var playerID PlayerID
	if userID == player1ID {
		playerID = Player1
	} else if userID == player2ID {
		playerID = Player2
	} else {
		http.Error(w, "You are not a player in this session", http.StatusForbidden)
		return
	}

	// Get or create game session
	session, created, err := stateManager.GetOrCreateSession(sessionID, player1ID, player2ID, player1Name, player2Name)
	if err != nil {
		log.Printf("Failed to get/create session %s: %v", sessionID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if created {
		log.Printf("Created new game session %s for players %d and %d", sessionID, player1ID, player2ID)
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	// Create player connection
	playerConn := NewPlayerConnection(sessionID, playerID, userID, username, conn, session, stateManager)
	session.SetPlayerConnection(playerID, playerConn)

	log.Printf("Player %d (%s) connected to session %s via WebSocket", playerID, username, sessionID)

	// Start connection handlers
	go playerConn.WritePump()
	go playerConn.ReadPump()

	// Send initial JOIN_GAME action to initialize state
	playerConn.ProcessAction(&ClientMessage{
		Action:         ActionJoinGame,
		Params:         json.RawMessage("{}"), // Empty params for JOIN_GAME
		StateHashAfter: 0, // First connection, hash will be sent in response
		SequenceNumber: session.State.GetPlayerSequence(playerID),
	})
}

// handleGameStats returns statistics about active games
func handleGameStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	stats := map[string]interface{}{
		"active_sessions": stateManager.SessionCount(),
		"session_ids":     stateManager.GetAllSessionIDs(),
	}

	json.NewEncoder(w).Encode(stats)
}

// sessionCleanupLoop periodically cleans up inactive sessions
func sessionCleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		removed := stateManager.CleanupInactiveSessions(30 * time.Minute)
		if removed > 0 {
			log.Printf("Cleaned up %d inactive sessions", removed)
		}
	}
}
