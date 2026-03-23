package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	initialMMRRange   = 100  // ±100 MMR initially
	rangeExpansionRate = 50   // +50 MMR per 10 seconds
	expansionInterval  = 10   // Expand every 10 seconds
	maxMMRRange       = 500  // Maximum ±500 MMR
	mmrWinGain        = 15   // MMR gained on win
	mmrLossDeduction  = 10   // MMR lost on loss
)

// Request/Response structures
type JoinQueueRequest struct {
	UserID int64 `json:"user_id"`
}

type LeaveQueueRequest struct {
	UserID int64 `json:"user_id"`
}

type QueueStatus struct {
	InQueue    bool      `json:"in_queue"`
	QueuedAt   time.Time `json:"queued_at,omitempty"`
	MMR        int       `json:"mmr,omitempty"`
	WaitTime   int       `json:"wait_time_seconds,omitempty"`
	MMRRange   string    `json:"mmr_range,omitempty"`
}

type MatchFoundResponse struct {
	SessionID string `json:"session_id"`
	Opponent  string `json:"opponent"`
	YourMMR   int    `json:"your_mmr"`
	TheirMMR  int    `json:"their_mmr"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// QueueEntry represents a player in the matchmaking queue
type QueueEntry struct {
	UserID   int64
	Username string
	MMR      int
	JoinedAt time.Time
}

// handleJoinQueue adds a player to the matchmaking queue
func handleJoinQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JoinQueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		respondError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get user's MMR from database
	var username string
	var mmr int
	var isBanned bool
	err := db.QueryRow(
		"SELECT username, mmr, is_banned FROM users WHERE id = ?",
		req.UserID,
	).Scan(&username, &mmr, &isBanned)

	if err == sql.ErrNoRows {
		respondError(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Database error: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if isBanned {
		respondError(w, "Banned users cannot join matchmaking", http.StatusForbidden)
		return
	}

	// Check if already in queue
	var existingID int64
	err = db.QueryRow("SELECT id FROM matchmaking_queue WHERE user_id = ?", req.UserID).Scan(&existingID)
	if err == nil {
		respondError(w, "Already in queue", http.StatusConflict)
		return
	}

	// Add to matchmaking queue
	_, err = db.Exec(
		"INSERT INTO matchmaking_queue (user_id, mmr) VALUES (?, ?)",
		req.UserID, mmr,
	)
	if err != nil {
		log.Printf("Failed to add to queue: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Also add to Redis for faster lookups
	ctx := context.Background()
	queueKey := fmt.Sprintf("queue:%d", req.UserID)
	queueData := map[string]interface{}{
		"user_id":   req.UserID,
		"username":  username,
		"mmr":       mmr,
		"joined_at": time.Now().Unix(),
	}
	queueJSON, _ := json.Marshal(queueData)
	redisClient.Set(ctx, queueKey, queueJSON, 10*time.Minute)

	respondJSON(w, map[string]interface{}{
		"message":  "Added to matchmaking queue",
		"user_id":  req.UserID,
		"mmr":      mmr,
		"position": "searching",
	}, http.StatusOK)
}

// handleLeaveQueue removes a player from the matchmaking queue
func handleLeaveQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LeaveQueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		respondError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Remove from database queue
	result, err := db.Exec("DELETE FROM matchmaking_queue WHERE user_id = ?", req.UserID)
	if err != nil {
		log.Printf("Failed to remove from queue: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondError(w, "Not in queue", http.StatusNotFound)
		return
	}

	// Remove from Redis
	ctx := context.Background()
	queueKey := fmt.Sprintf("queue:%d", req.UserID)
	redisClient.Del(ctx, queueKey)

	respondJSON(w, map[string]string{
		"message": "Removed from matchmaking queue",
	}, http.StatusOK)
}

// handleQueueStatus checks if a player is in queue and their status
func handleQueueStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		respondError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	var mmr int
	var joinedAt time.Time
	err := db.QueryRow(
		"SELECT mmr, joined_at FROM matchmaking_queue WHERE user_id = ?",
		userID,
	).Scan(&mmr, &joinedAt)

	if err == sql.ErrNoRows {
		respondJSON(w, QueueStatus{InQueue: false}, http.StatusOK)
		return
	}
	if err != nil {
		log.Printf("Database error: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	waitTime := int(time.Since(joinedAt).Seconds())
	mmrRange := calculateMMRRange(mmr, waitTime)

	respondJSON(w, QueueStatus{
		InQueue:  true,
		QueuedAt: joinedAt,
		MMR:      mmr,
		WaitTime: waitTime,
		MMRRange: fmt.Sprintf("%d - %d", mmrRange.min, mmrRange.max),
	}, http.StatusOK)
}

// MMR range calculation
type mmrRange struct {
	min int
	max int
}

func calculateMMRRange(mmr int, waitTimeSeconds int) mmrRange {
	// Initial range: ±100
	// Expand by 50 every 10 seconds
	// Max range: ±500

	expansions := waitTimeSeconds / expansionInterval
	rangeSize := initialMMRRange + (expansions * rangeExpansionRate)
	if rangeSize > maxMMRRange {
		rangeSize = maxMMRRange
	}

	return mmrRange{
		min: mmr - rangeSize,
		max: mmr + rangeSize,
	}
}

// Check if two MMR ranges overlap
func rangesOverlap(r1, r2 mmrRange) bool {
	return r1.max >= r2.min && r2.max >= r1.min
}

// Helper functions
func respondJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, message string, statusCode int) {
	respondJSON(w, ErrorResponse{Error: message}, statusCode)
}

// createGameSession creates a new game session for matched players
func createGameSession(player1, player2 QueueEntry) (string, error) {
	sessionID := uuid.New().String()

	_, err := db.Exec(
		"INSERT INTO game_sessions (session_id, player1_id, player2_id, status, started_at) VALUES (?, ?, ?, 'waiting', NOW())",
		sessionID, player1.UserID, player2.UserID,
	)
	if err != nil {
		return "", err
	}

	// Remove both players from queue
	db.Exec("DELETE FROM matchmaking_queue WHERE user_id IN (?, ?)", player1.UserID, player2.UserID)

	// Remove from Redis
	ctx := context.Background()
	redisClient.Del(ctx, fmt.Sprintf("queue:%d", player1.UserID))
	redisClient.Del(ctx, fmt.Sprintf("queue:%d", player2.UserID))

	// Store match info in Redis for players to retrieve
	matchKey1 := fmt.Sprintf("match:%d", player1.UserID)
	matchKey2 := fmt.Sprintf("match:%d", player2.UserID)

	match1 := MatchFoundResponse{
		SessionID: sessionID,
		Opponent:  player2.Username,
		YourMMR:   player1.MMR,
		TheirMMR:  player2.MMR,
	}
	match2 := MatchFoundResponse{
		SessionID: sessionID,
		Opponent:  player1.Username,
		YourMMR:   player2.MMR,
		TheirMMR:  player1.MMR,
	}

	match1JSON, _ := json.Marshal(match1)
	match2JSON, _ := json.Marshal(match2)

	redisClient.Set(ctx, matchKey1, match1JSON, 5*time.Minute)
	redisClient.Set(ctx, matchKey2, match2JSON, 5*time.Minute)

	log.Printf("Match created: %s vs %s (MMR: %d vs %d, Session: %s)",
		player1.Username, player2.Username, player1.MMR, player2.MMR, sessionID)

	return sessionID, nil
}
