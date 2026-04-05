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
	db "github.com/kKar1503/cs464-backend/db/sqlc"
)

const (
	initialMMRRange    = 100 // ±100 MMR initially
	rangeExpansionRate = 50  // +50 MMR per 10 seconds
	expansionInterval  = 10  // Expand every 10 seconds
	maxMMRRange        = 500 // Maximum ±500 MMR
	mmrWinGain         = 15  // MMR gained on win
	mmrLossDeduction   = 10  // MMR lost on loss
	matchAcceptTimeout = 30  // Seconds to accept/reject match
)

// Request/Response structures
// JoinQueueRequest - no fields needed, uses auth context
type JoinQueueRequest struct {
}

// LeaveQueueRequest - no fields needed, uses auth context
type LeaveQueueRequest struct {
}

type QueueStatus struct {
	InQueue  bool      `json:"in_queue"`
	QueuedAt time.Time `json:"queued_at,omitempty"`
	MMR      int       `json:"mmr,omitempty"`
	WaitTime int       `json:"wait_time_seconds,omitempty"`
	MMRRange string    `json:"mmr_range,omitempty"`
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


	// Get authenticated user from context
	authUser, err := getAuthenticatedUser(r)
	if err != nil {
		respondError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := authUser.UserID
	ctx := r.Context()

	// Get user's MMR from database
	user, err := queries.GetUserForMatchmaking(ctx, userID)
	if err == sql.ErrNoRows {
		respondError(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Database error: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if user.IsBanned {
		respondError(w, "Banned users cannot join matchmaking", http.StatusForbidden)
		return
	}

	// Check if user has an ongoing game session
	ongoingSessionID, err := queries.GetOngoingGameSession(ctx, db.GetOngoingGameSessionParams{
		Player1ID: userID,
		Player2ID: userID,
	})
	if err == nil {
		respondError(w, fmt.Sprintf("Cannot join queue: already in game session %s", ongoingSessionID), http.StatusConflict)
		return
	}

	// Check if already in queue
	_, err = queries.GetQueueEntryByUserID(ctx, userID)
	if err == nil {
		respondError(w, "Already in queue", http.StatusConflict)
		return
	}

	// Add to matchmaking queue
	err = queries.InsertIntoQueue(ctx, db.InsertIntoQueueParams{
		UserID: userID,
		Mmr:    user.Mmr,
	})
	if err != nil {
		log.Printf("Failed to add to queue: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Also add to Redis for faster lookups
	queueKey := fmt.Sprintf("queue:%d", userID)
	queueData := map[string]interface{}{
		"user_id":   userID,
		"username":  user.Username,
		"mmr":       user.Mmr,
		"joined_at": time.Now().Unix(),
	}
	queueJSON, _ := json.Marshal(queueData)
	redisClient.Set(ctx, queueKey, queueJSON, 10*time.Minute)

	respondJSON(w, map[string]interface{}{
		"message":  "Added to matchmaking queue",
		"user_id":  userID,
		"mmr":      user.Mmr,
		"position": "searching",
	}, http.StatusOK)
}

// handleLeaveQueue removes a player from the matchmaking queue
func handleLeaveQueue(w http.ResponseWriter, r *http.Request) {


	// Get authenticated user from context
	authUser, err := getAuthenticatedUser(r)
	if err != nil {
		respondError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := authUser.UserID
	ctx := r.Context()

	// Remove from database queue
	result, err := queries.DeleteFromQueue(ctx, userID)
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
	queueKey := fmt.Sprintf("queue:%d", userID)
	redisClient.Del(ctx, queueKey)

	respondJSON(w, map[string]string{
		"message": "Removed from matchmaking queue",
	}, http.StatusOK)
}

// handleQueueStatus checks if a player is in queue and their status
func handleQueueStatus(w http.ResponseWriter, r *http.Request) {


	// Get authenticated user from context
	authUser, err := getAuthenticatedUser(r)
	if err != nil {
		respondError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := authUser.UserID
	ctx := r.Context()

	status, err := queries.GetQueueStatus(ctx, userID)
	if err == sql.ErrNoRows {
		respondJSON(w, QueueStatus{InQueue: false}, http.StatusOK)
		return
	}
	if err != nil {
		log.Printf("Database error: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	waitTime := int(time.Since(status.JoinedAt).Seconds())
	mmrRange := calculateMMRRange(int(status.Mmr), waitTime)

	respondJSON(w, QueueStatus{
		InQueue:  true,
		QueuedAt: status.JoinedAt,
		MMR:      int(status.Mmr),
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
	ctx := context.Background()

	// Set expiry time for match acceptance
	expiresAt := time.Now().Add(matchAcceptTimeout * time.Second)

	err := queries.CreateGameSession(ctx, db.CreateGameSessionParams{
		SessionID:      sessionID,
		Player1ID:      player1.UserID,
		Player2ID:      player2.UserID,
		MatchExpiresAt: &expiresAt,
	})
	if err != nil {
		return "", err
	}

	// Remove both players from queue
	queries.DeleteFromQueueByUsers(ctx, db.DeleteFromQueueByUsersParams{
		UserID:   player1.UserID,
		UserID_2: player2.UserID,
	})

	// Remove from Redis
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

	ctx := r.Context()

	// Get session details to determine which player is ready
	session, err := queries.GetSessionForAccept(ctx, req.SessionID)
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
	if session.Status != db.GameSessionsStatusWaiting {
		respondError(w, "Session is not in waiting state", http.StatusBadRequest)
		return
	}

	// Check if user is part of this session
	if userID != session.Player1ID && userID != session.Player2ID {
		respondError(w, "User is not part of this session", http.StatusForbidden)
		return
	}

	// Determine which player is ready
	isPlayer1 := userID == session.Player1ID
	player1Ready := session.Player1Ready != nil && *session.Player1Ready
	player2Ready := session.Player2Ready != nil && *session.Player2Ready
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
		t := true
		result, err = queries.StartGameSession(ctx, db.StartGameSessionParams{
			Player1Ready: &t,
			Player2Ready: &t,
			SessionID:    req.SessionID,
		})
	} else if isPlayer1 {
		result, err = queries.SetPlayer1Ready(ctx, req.SessionID)
	} else {
		result, err = queries.SetPlayer2Ready(ctx, req.SessionID)
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

	ctx := r.Context()

	// Get session details
	session, err := queries.GetSessionWithPlayers(ctx, req.SessionID)
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
	if session.Status != db.GameSessionsStatusWaiting {
		respondError(w, "Session is not in waiting state", http.StatusBadRequest)
		return
	}

	// Check if user is part of this session
	if userID != session.Player1ID && userID != session.Player2ID {
		respondError(w, "User is not part of this session", http.StatusForbidden)
		return
	}

	// Cancel the session
	if err := queries.CancelGameSession(ctx, req.SessionID); err != nil {
		log.Printf("Failed to cancel session: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Determine which player to requeue (the one who didn't reject)
	var requeueUserID int64
	var requeueUsername string
	var requeueMMR int32

	if userID == session.Player1ID {
		requeueUserID = session.Player2ID
		requeueUsername = session.Username_2
		requeueMMR = session.Mmr_2
	} else {
		requeueUserID = session.Player1ID
		requeueUsername = session.Username
		requeueMMR = session.Mmr
	}

	// Requeue the non-rejecting player
	if err := queries.RequeuePlayer(ctx, db.RequeuePlayerParams{UserID: requeueUserID, Mmr: requeueMMR}); err != nil {
		log.Printf("Failed to requeue user %d: %v", requeueUserID, err)
	} else {
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
	redisClient.Del(ctx, fmt.Sprintf("match:%d", session.Player1ID))
	redisClient.Del(ctx, fmt.Sprintf("match:%d", session.Player2ID))

	log.Printf("Match rejected by user %d, session %s cancelled, user %d requeued", userID, req.SessionID, requeueUserID)

	respondJSON(w, map[string]interface{}{
		"message":    "Match rejected, other player returned to queue",
		"session_id": req.SessionID,
	}, http.StatusOK)
}

// handleEndGame transitions a game session to 'completed' and updates MMR
func handleEndGame(w http.ResponseWriter, r *http.Request) {


	var req GameSessionUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		respondError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Fetch session details
	session, err := queries.GetSessionForEndGame(ctx, req.SessionID)
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
	var player1MMRChange, player2MMRChange int32
	if req.WinnerID != nil {
		if *req.WinnerID == session.Player1ID {
			player1MMRChange = mmrWinGain
			player2MMRChange = -mmrLossDeduction
		} else if *req.WinnerID == session.Player2ID {
			player1MMRChange = -mmrLossDeduction
			player2MMRChange = mmrWinGain
		} else {
			respondError(w, "Winner must be one of the players", http.StatusBadRequest)
			return
		}

		// Update players' MMR in database
		queries.UpdateUserMMR(ctx, db.UpdateUserMMRParams{Mmr: player1MMRChange, ID: session.Player1ID})
		queries.UpdateUserMMR(ctx, db.UpdateUserMMRParams{Mmr: player2MMRChange, ID: session.Player2ID})
	}

	// Update session status to completed
	err = queries.CompleteGameSession(ctx, db.CompleteGameSessionParams{
		WinnerID:         req.WinnerID,
		Player1MmrChange: &player1MMRChange,
		Player2MmrChange: &player2MMRChange,
		SessionID:        req.SessionID,
	})
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
		"player1_new_mmr":    session.Mmr + player1MMRChange,
		"player2_new_mmr":    session.Mmr_2 + player2MMRChange,
	}, http.StatusOK)
}

// handleGetSessionStatus returns the current status of a game session
func handleGetSessionStatus(w http.ResponseWriter, r *http.Request) {


	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		respondError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	session, err := queries.GetSessionStatus(ctx, sessionID)
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
		"player1_id":         session.Player1ID,
		"player2_id":         session.Player2ID,
		"status":             session.Status,
		"player1_mmr_change": session.Player1MmrChange,
		"player2_mmr_change": session.Player2MmrChange,
	}

	if session.WinnerID != nil {
		response["winner_id"] = *session.WinnerID
	}
	if session.StartedAt != nil {
		response["started_at"] = *session.StartedAt
	}
	if session.EndedAt != nil {
		response["ended_at"] = *session.EndedAt
	}

	respondJSON(w, response, http.StatusOK)
}
