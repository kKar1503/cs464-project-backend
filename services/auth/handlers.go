package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	db "github.com/kKar1503/cs464-backend/db/sqlc"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionTTL    = 24 * time.Hour
	tokenLength   = 32
	bcryptCost    = 12
	sessionKeyFmt = "session:%s"
)

// Request/Response structures
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ValidateResponse struct {
	Valid    bool   `json:"valid"`
	UserID   int64  `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
}

type BanRequest struct {
	UserID int64  `json:"user_id"`
	Reason string `json:"reason"`
}

type UnbanRequest struct {
	UserID int64 `json:"user_id"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type SessionData struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	ExpiresAt int64  `json:"expires_at"`
}

// handleRegister creates a new user account
func handleRegister(w http.ResponseWriter, r *http.Request) {


	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		respondError(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 {
		respondError(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Insert user into database
	result, err := queries.CreateUserAuth(r.Context(), db.CreateUserAuthParams{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
	})
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			respondError(w, "Username already exists", http.StatusConflict)
			return
		}
		log.Printf("Failed to create user: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userID, _ := result.LastInsertId()

	respondJSON(w, map[string]any{
		"user_id":  userID,
		"username": req.Username,
		"message":  "User registered successfully",
	}, http.StatusCreated)
}

// handleLogin authenticates a user and creates a session
func handleLogin(w http.ResponseWriter, r *http.Request) {


	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		respondError(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Fetch user from database
	user, err := queries.GetUserByUsernameAuth(r.Context(), req.Username)
	if err == sql.ErrNoRows {
		respondError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		log.Printf("Database error: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if user is banned
	if user.IsBanned {
		reason := "No reason provided"
		if user.BanReason != nil {
			reason = *user.BanReason
		}
		respondError(w, fmt.Sprintf("Account is banned: %s", reason), http.StatusForbidden)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		respondError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Invalidate previous sessions (single session per user)
	if err := invalidateUserSessions(user.ID); err != nil {
		log.Printf("Failed to invalidate previous sessions: %v", err)
	}

	// Generate session token
	token, err := generateToken()
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(sessionTTL)

	// Store session in MySQL
	ip := getClientIP(r)
	ua := r.UserAgent()
	err = queries.CreateSession(r.Context(), db.CreateSessionParams{
		UserID:    user.ID,
		Token:     token,
		IpAddress: &ip,
		UserAgent: &ua,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		log.Printf("Failed to create session in DB: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store session in Redis for fast validation
	ctx := context.Background()
	sessionData := SessionData{
		UserID:    user.ID,
		Username:  user.Username,
		ExpiresAt: expiresAt.Unix(),
	}
	sessionJSON, _ := json.Marshal(sessionData)
	redisKey := fmt.Sprintf(sessionKeyFmt, token)
	if err := redisClient.Set(ctx, redisKey, sessionJSON, sessionTTL).Err(); err != nil {
		log.Printf("Failed to cache session in Redis: %v", err)
	}

	respondJSON(w, AuthResponse{
		Token:     token,
		UserID:    user.ID,
		Username:  user.Username,
		ExpiresAt: expiresAt,
	}, http.StatusOK)
}

// handleLogout revokes the current session
func handleLogout(w http.ResponseWriter, r *http.Request) {


	token := extractToken(r)
	if token == "" {
		respondError(w, "Missing authorization token", http.StatusUnauthorized)
		return
	}

	// Delete from Redis
	ctx := context.Background()
	redisKey := fmt.Sprintf(sessionKeyFmt, token)
	redisClient.Del(ctx, redisKey)

	// Revoke in MySQL
	err := queries.RevokeSessionByToken(r.Context(), token)
	if err != nil {
		log.Printf("Failed to revoke session: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"message": "Logged out successfully"}, http.StatusOK)
}

// handleValidate validates a session token (used by other services)
func handleValidate(w http.ResponseWriter, r *http.Request) {


	token := extractToken(r)
	if token == "" {
		respondJSON(w, ValidateResponse{Valid: false}, http.StatusOK)
		return
	}

	ctx := context.Background()
	redisKey := fmt.Sprintf(sessionKeyFmt, token)

	// Try Redis first (fast path)
	sessionJSON, err := redisClient.Get(ctx, redisKey).Result()
	if err == nil {
		var sessionData SessionData
		if err := json.Unmarshal([]byte(sessionJSON), &sessionData); err == nil {
			if time.Now().Unix() < sessionData.ExpiresAt {
				respondJSON(w, ValidateResponse{
					Valid:    true,
					UserID:   sessionData.UserID,
					Username: sessionData.Username,
				}, http.StatusOK)
				return
			}
		}
	}

	// Fallback to MySQL
	session, err := queries.ValidateSession(r.Context(), token)
	if err == sql.ErrNoRows {
		respondJSON(w, ValidateResponse{Valid: false}, http.StatusOK)
		return
	}
	if err != nil {
		log.Printf("Database error: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if user is banned
	if session.IsBanned {
		respondJSON(w, ValidateResponse{Valid: false}, http.StatusOK)
		return
	}

	// Repopulate Redis cache
	sessionData := SessionData{
		UserID:    session.UserID,
		Username:  session.Username,
		ExpiresAt: session.ExpiresAt.Unix(),
	}
	sessionJSONBytes, _ := json.Marshal(sessionData)
	ttl := time.Until(session.ExpiresAt)
	if ttl > 0 {
		redisClient.Set(ctx, redisKey, string(sessionJSONBytes), ttl)
	}

	respondJSON(w, ValidateResponse{
		Valid:    true,
		UserID:   session.UserID,
		Username: session.Username,
	}, http.StatusOK)
}

// handleMe returns current user info
func handleMe(w http.ResponseWriter, r *http.Request) {


	token := extractToken(r)
	if token == "" {
		respondError(w, "Missing authorization token", http.StatusUnauthorized)
		return
	}

	// Reuse validation logic
	ctx := context.Background()
	redisKey := fmt.Sprintf(sessionKeyFmt, token)

	sessionJSON, err := redisClient.Get(ctx, redisKey).Result()
	if err == nil {
		var sessionData SessionData
		if err := json.Unmarshal([]byte(sessionJSON), &sessionData); err == nil {
			if time.Now().Unix() < sessionData.ExpiresAt {
				respondJSON(w, map[string]any{
					"user_id":  sessionData.UserID,
					"username": sessionData.Username,
				}, http.StatusOK)
				return
			}
		}
	}

	respondError(w, "Unauthorized", http.StatusUnauthorized)
}

// Helper functions

func invalidateUserSessions(userID int64) error {
	ctx := context.Background()

	// Get all active tokens for this user
	tokens, err := queries.GetActiveTokensByUser(ctx, userID)
	if err != nil {
		return err
	}

	// Delete from Redis
	for _, token := range tokens {
		redisKey := fmt.Sprintf(sessionKeyFmt, token)
		redisClient.Del(ctx, redisKey)
	}

	// Revoke in MySQL
	return queries.RevokeAllUserSessions(ctx, userID)
}

func generateToken() (string, error) {
	bytes := make([]byte, tokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func extractToken(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	// Fallback to query parameter
	return r.URL.Query().Get("token")
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}

func respondJSON(w http.ResponseWriter, data any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, message string, statusCode int) {
	respondJSON(w, ErrorResponse{Error: message}, statusCode)
}

// handleBanUser bans a user and revokes all their sessions
func handleBanUser(w http.ResponseWriter, r *http.Request) {


	var req BanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		respondError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Ban the user in database
	err := queries.BanUser(r.Context(), db.BanUserParams{
		BanReason: &req.Reason,
		ID:        req.UserID,
	})
	if err != nil {
		log.Printf("Failed to ban user: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Revoke all active sessions for this user
	if err := invalidateUserSessions(req.UserID); err != nil {
		log.Printf("Failed to invalidate sessions: %v", err)
	}

	respondJSON(w, map[string]any{
		"message": "User banned successfully",
		"user_id": req.UserID,
		"reason":  req.Reason,
	}, http.StatusOK)
}

// handleUnbanUser unbans a user
func handleUnbanUser(w http.ResponseWriter, r *http.Request) {


	var req UnbanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		respondError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Unban the user
	err := queries.UnbanUser(r.Context(), req.UserID)
	if err != nil {
		log.Printf("Failed to unban user: %v", err)
		respondError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]any{
		"message": "User unbanned successfully",
		"user_id": req.UserID,
	}, http.StatusOK)
}
