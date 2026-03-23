package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ValidateResponse from auth service
type ValidateResponse struct {
	Valid    bool   `json:"valid"`
	UserID   int64  `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
}

// AuthenticatedUser contains validated user info
type AuthenticatedUser struct {
	UserID   int64
	Username string
}

// contextKey for storing user in request context
type contextKey string

const userContextKey contextKey = "user"

// validateToken calls the auth service to validate a token
func validateToken(token string) (*AuthenticatedUser, error) {
	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://auth-service:8000"
	}

	url := fmt.Sprintf("%s/auth/validate", authServiceURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call auth service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var validateResp ValidateResponse
	if err := json.Unmarshal(body, &validateResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !validateResp.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return &AuthenticatedUser{
		UserID:   validateResp.UserID,
		Username: validateResp.Username,
	}, nil
}

// extractToken extracts token from Authorization header
func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}

	return ""
}

// requireAuth is a middleware that validates authentication
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			respondError(w, "Missing authorization token", http.StatusUnauthorized)
			return
		}

		user, err := validateToken(token)
		if err != nil {
			log.Printf("Auth validation failed: %v", err)
			respondError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Store user in request context
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// getAuthenticatedUser retrieves user from request context
func getAuthenticatedUser(r *http.Request) (*AuthenticatedUser, error) {
	user, ok := r.Context().Value(userContextKey).(*AuthenticatedUser)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}
	return user, nil
}

// requireInternalAuth is a middleware that validates internal service authentication
func requireInternalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for internal service secret in header
		secret := r.Header.Get("X-Internal-Secret")
		expectedSecret := os.Getenv("INTERNAL_SERVICE_SECRET")

		if expectedSecret == "" {
			log.Printf("WARNING: INTERNAL_SERVICE_SECRET not set")
			respondError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if secret == "" {
			respondError(w, "Missing internal service authentication", http.StatusUnauthorized)
			return
		}

		if secret != expectedSecret {
			log.Printf("Invalid internal service secret attempted")
			respondError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Internal auth successful, proceed
		next.ServeHTTP(w, r)
	}
}
