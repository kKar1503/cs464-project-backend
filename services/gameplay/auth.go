package main

import (
	"encoding/json"
	"fmt"
	"io"
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

// extractToken extracts token from Authorization header or query parameter
func extractToken(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Fallback to query parameter (needed for WebSocket connections)
	return r.URL.Query().Get("token")
}
