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
	initialMMRRange   = 100 // ±100 MMR initially
	rangeExpansionRate = 50  // +50 MMR per 10 seconds
	expansionInterval  = 10  // Expand every 10 seconds
	maxMMRRange       = 500 // Maximum ±500 MMR
	mmrWinGain        = 15  // MMR gained on win
	mmrLossDeduction  = 10  // MMR lost on loss
	matchAcceptTimeout = 30 // Seconds to accept/reject match
)

// Request/Response structures
// JoinQueueRequest - no fields needed, uses auth context
type JoinQueueRequest struct {
}

// LeaveQueueRequest - no fields needed, uses auth context
type LeaveQueueRequest struct {
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

	// Get authenticated user from context
	authUser, err := getAuthenticatedUser(r)
	if err != nil {
		respondError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := authUser.UserID

	// Get user's MMR from database
	var username string
	var mmr int
	var isBanned bool
	err = db.QueryRow(
		"SELECT username, mmr, is_banned FROM users WHERE id = ?",
		userID,
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

	// Check if user has an ongoing game session
	var ongoingSessionID string
	err = db.QueryRow(
		"SELECT session_id FROM game_sessions WHERE (player1_id = ? OR player2_id = ?) AND status IN ('waiting', 'in_progress')",
		userID, userID,
	).Scan(&ongoingSessionID)
	if err == nil {
		respondError(w, fmt.Sprintf("Cannot join queue: already in game session %s", ongoingSessionID), http.StatusConflict)
		return
	}

	// Check if already in queue
	var existingID int64
	err = db.QueryRow("SELECT id FROM matchmaking_queue WHERE user_id = ?", userID).Scan(&existingID)
	if err == nil {
		respondError(w, "Already in queue", http.StatusConflict)
		return
	}

	// Add to matchmaking queue
	_, err = db.Exec(
		"INSERT INTO matchmaking_queue (user_id, mmr) VALUES (?, ?)",
		userID, mmr,
	)
	if err != nil {
		log.Printf("Failed to add to queue: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Also add to Redis for faster lookups
	ctx := context.Background()
	queueKey := fmt.Sprintf("queue:%d", userID)
	queueData := map[string]interface{}{
		"user_id":   userID,
		"username":  username,
		"mmr":       mmr,
		"joined_at": time.Now().Unix(),
	}
	queueJSON, _ := json.Marshal(queueData)
	redisClient.Set(ctx, queueKey, queueJSON, 10*time.Minute)

	respondJSON(w, map[string]interface{}{
		"message":  "Added to matchmaking queue",
		"user_id":  userID,
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

	// Get authenticated user from context
	authUser, err := getAuthenticatedUser(r)
	if err != nil {
		respondError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := authUser.UserID

	// Remove from database queue
	result, err := db.Exec("DELETE FROM matchmaking_queue WHERE user_id = ?", userID)
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
	queueKey := fmt.Sprintf("queue:%d", userID)
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

	// Get authenticated user from context
	authUser, err := getAuthenticatedUser(r)
	if err != nil {
		respondError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := authUser.UserID

	var mmr int
	var joinedAt time.Time
	err = db.QueryRow(
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

// GameSessionUpdateRequest represents a request to update game session state
type GameSessionUpdateRequest struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"` // "in_progress", "completed", "cancelled"
	WinnerID  *int64 `json:"winner_id,omitempty"`
}

// MatchResponseRequest represents a request to accept or reject a match
type MatchResponseRequest struct {
	SessionID string `json:"session_id"`
}

// createGameSession creates a new game session for matched players
func createGameSession(player1, player2 QueueEntry) (string, error) {
	sessionID := uuid.New().String()

	// Set expiry time for match acceptance
	expiresAt := time.Now().Add(matchAcceptTimeout * time.Second)

	_, err := db.Exec(
		"INSERT INTO game_sessions (session_id, player1_id, player2_id, status, created_at, match_expires_at) VALUES (?, ?, ?, 'waiting', NOW(), ?)",
		sessionID, player1.UserID, player2.UserID, expiresAt,
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

// handleAcceptMatch marks a player as accepting the match and starts the game if both players accept
func handleAcceptMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user from context
	authUser, err := getAuthenticatedUser(r)
	if err != nil {
		respondError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := authUser.UserID

	var req MatchResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		respondError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Get session details to determine which player is ready
	var player1ID, player2ID int64
	var player1Ready, player2Ready bool
	var status string
	err = db.QueryRow(
		"SELECT player1_id, player2_id, player1_ready, player2_ready, status FROM game_sessions WHERE session_id = ?",
		req.SessionID,
	).Scan(&player1ID, &player2ID, &player1Ready, &player2Ready, &status)

	if err == sql.ErrNoRows {
		respondError(w, "Session not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Failed to fetch session: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if session is in waiting state
	if status != "waiting" {
		respondError(w, "Session is not in waiting state", http.StatusBadRequest)
		return
	}

	// Check if user is part of this session
	if userID != player1ID && userID != player2ID {
		respondError(w, "User is not part of this session", http.StatusForbidden)
		return
	}

	// Determine which player is ready
	isPlayer1 := userID == player1ID
	if isPlayer1 {
		player1Ready = true
	} else {
		player2Ready = true
	}

	// Check if both players are now ready
	bothReady := player1Ready && player2Ready

	// Update the session
	var result sql.Result
	if bothReady {
		// Both players ready - start the game and clear expiry
		result, err = db.Exec(
			"UPDATE game_sessions SET player1_ready = ?, player2_ready = ?, status = 'in_progress', started_at = NOW(), match_expires_at = NULL WHERE session_id = ? AND status = 'waiting'",
			player1Ready, player2Ready, req.SessionID,
		)
	} else {
		// Mark this player as ready
		if isPlayer1 {
			result, err = db.Exec(
				"UPDATE game_sessions SET player1_ready = TRUE WHERE session_id = ? AND status = 'waiting'",
				req.SessionID,
			)
		} else {
			result, err = db.Exec(
				"UPDATE game_sessions SET player2_ready = TRUE WHERE session_id = ? AND status = 'waiting'",
				req.SessionID,
			)
		}
	}

	if err != nil {
		log.Printf("Failed to update session: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondError(w, "Failed to update session", http.StatusInternalServerError)
		return
	}

	if bothReady {
		respondJSON(w, map[string]interface{}{
			"message":       "Both players ready - game started",
			"session_id":    req.SessionID,
			"status":        "in_progress",
			"player1_ready": true,
			"player2_ready": true,
		}, http.StatusOK)
	} else {
		respondJSON(w, map[string]interface{}{
			"message":       "Waiting for other player",
			"session_id":    req.SessionID,
			"status":        "waiting",
			"player1_ready": player1Ready,
			"player2_ready": player2Ready,
		}, http.StatusOK)
	}
}

// handleRejectMatch handles when a player rejects the match, puts both players back in queue
func handleRejectMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user from context
	authUser, err := getAuthenticatedUser(r)
	if err != nil {
		respondError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := authUser.UserID

	var req MatchResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		respondError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Get session details
	var player1ID, player2ID int64
	var player1Username, player2Username string
	var player1MMR, player2MMR int
	var status string
	err = db.QueryRow(
		"SELECT gs.player1_id, gs.player2_id, u1.username, u2.username, u1.mmr, u2.mmr, gs.status "+
			"FROM game_sessions gs "+
			"JOIN users u1 ON gs.player1_id = u1.id "+
			"JOIN users u2 ON gs.player2_id = u2.id "+
			"WHERE gs.session_id = ?",
		req.SessionID,
	).Scan(&player1ID, &player2ID, &player1Username, &player2Username, &player1MMR, &player2MMR, &status)

	if err == sql.ErrNoRows {
		respondError(w, "Session not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Failed to fetch session: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if session is in waiting state
	if status != "waiting" {
		respondError(w, "Session is not in waiting state", http.StatusBadRequest)
		return
	}

	// Check if user is part of this session
	if userID != player1ID && userID != player2ID {
		respondError(w, "User is not part of this session", http.StatusForbidden)
		return
	}

	// Cancel the session
	_, err = db.Exec(
		"UPDATE game_sessions SET status = 'cancelled' WHERE session_id = ?",
		req.SessionID,
	)
	if err != nil {
		log.Printf("Failed to cancel session: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Only requeue the OTHER player (not the one who rejected)
	ctx := context.Background()

	// Determine which player to requeue (the one who didn't reject)
	var requeueUserID int64
	var requeueUsername string
	var requeueMMR int

	if userID == player1ID {
		// Player 1 rejected, requeue player 2
		requeueUserID = player2ID
		requeueUsername = player2Username
		requeueMMR = player2MMR
	} else {
		// Player 2 rejected, requeue player 1
		requeueUserID = player1ID
		requeueUsername = player1Username
		requeueMMR = player1MMR
	}

	// Requeue the non-rejecting player
	_, err = db.Exec(
		"INSERT INTO matchmaking_queue (user_id, mmr, joined_at) VALUES (?, ?, NOW()) ON DUPLICATE KEY UPDATE joined_at = NOW()",
		requeueUserID, requeueMMR,
	)
	if err != nil {
		log.Printf("Failed to requeue user %d: %v", requeueUserID, err)
	} else {
		// Add to Redis
		queueKey := fmt.Sprintf("queue:%d", requeueUserID)
		queueData := map[string]interface{}{
			"user_id":  requeueUserID,
			"username": requeueUsername,
			"mmr":      requeueMMR,
		}
		queueJSON, _ := json.Marshal(queueData)
		redisClient.Set(ctx, queueKey, queueJSON, 0)
	}

	// Clear match notifications from Redis for both players
	redisClient.Del(ctx, fmt.Sprintf("match:%d", player1ID))
	redisClient.Del(ctx, fmt.Sprintf("match:%d", player2ID))

	log.Printf("Match rejected by user %d, session %s cancelled, user %d requeued", userID, req.SessionID, requeueUserID)

	respondJSON(w, map[string]interface{}{
		"message":    "Match rejected, other player returned to queue",
		"session_id": req.SessionID,
	}, http.StatusOK)
}

// handleEndGame transitions a game session to 'completed' and updates MMR
func handleEndGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GameSessionUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		respondError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Fetch session details
	var player1ID, player2ID int64
	var player1MMR, player2MMR int
	err := db.QueryRow(
		"SELECT gs.player1_id, gs.player2_id, u1.mmr, u2.mmr FROM game_sessions gs "+
			"JOIN users u1 ON gs.player1_id = u1.id "+
			"JOIN users u2 ON gs.player2_id = u2.id "+
			"WHERE gs.session_id = ? AND gs.status = 'in_progress'",
		req.SessionID,
	).Scan(&player1ID, &player2ID, &player1MMR, &player2MMR)

	if err == sql.ErrNoRows {
		respondError(w, "Session not found or not in progress", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Failed to fetch session: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Calculate MMR changes
	var player1MMRChange, player2MMRChange int
	if req.WinnerID != nil {
		if *req.WinnerID == player1ID {
			player1MMRChange = mmrWinGain
			player2MMRChange = -mmrLossDeduction
		} else if *req.WinnerID == player2ID {
			player1MMRChange = -mmrLossDeduction
			player2MMRChange = mmrWinGain
		} else {
			respondError(w, "Winner must be one of the players", http.StatusBadRequest)
			return
		}

		// Update players' MMR in database
		db.Exec("UPDATE users SET mmr = mmr + ? WHERE id = ?", player1MMRChange, player1ID)
		db.Exec("UPDATE users SET mmr = mmr + ? WHERE id = ?", player2MMRChange, player2ID)
	}

	// Update session status to completed
	_, err = db.Exec(
		"UPDATE game_sessions SET status = 'completed', ended_at = NOW(), winner_id = ?, player1_mmr_change = ?, player2_mmr_change = ? WHERE session_id = ?",
		req.WinnerID, player1MMRChange, player2MMRChange, req.SessionID,
	)
	if err != nil {
		log.Printf("Failed to end game: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"message":            "Game completed",
		"session_id":         req.SessionID,
		"status":             "completed",
		"winner_id":          req.WinnerID,
		"player1_mmr_change": player1MMRChange,
		"player2_mmr_change": player2MMRChange,
		"player1_new_mmr":    player1MMR + player1MMRChange,
		"player2_new_mmr":    player2MMR + player2MMRChange,
	}, http.StatusOK)
}

// handleGetSessionStatus returns the current status of a game session
func handleGetSessionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		respondError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	var player1ID, player2ID int64
	var status string
	var winnerID sql.NullInt64
	var startedAt, endedAt sql.NullTime
	var player1MMRChange, player2MMRChange int

	err := db.QueryRow(
		"SELECT player1_id, player2_id, status, winner_id, started_at, ended_at, player1_mmr_change, player2_mmr_change FROM game_sessions WHERE session_id = ?",
		sessionID,
	).Scan(&player1ID, &player2ID, &status, &winnerID, &startedAt, &endedAt, &player1MMRChange, &player2MMRChange)

	if err == sql.ErrNoRows {
		respondError(w, "Session not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Failed to fetch session status: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"session_id":         sessionID,
		"player1_id":         player1ID,
		"player2_id":         player2ID,
		"status":             status,
		"player1_mmr_change": player1MMRChange,
		"player2_mmr_change": player2MMRChange,
	}

	if winnerID.Valid {
		response["winner_id"] = winnerID.Int64
	}
	if startedAt.Valid {
		response["started_at"] = startedAt.Time
	}
	if endedAt.Valid {
		response["ended_at"] = endedAt.Time
	}

	respondJSON(w, response, http.StatusOK)
}
