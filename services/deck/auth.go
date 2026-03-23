package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type validateResponse struct {
	Valid bool `json:"valid"`
	UserID int64 `json:"user_id"`
	Username string `json:"username"`
	Email string `json:"email"`
}

func getUserFromToken(r *http.Request) (userID int64, err error) {
	authHeader := r.Header.Get("Authorization")
	var token string

	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			token = parts[1]
		}
	}

	if token == "" {
		return 0, errUnauthorized
	}

	baseURL := os.Getenv("AUTH_SERVICE_URL")

	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}

	url := strings.TrimSuffix(baseURL, "/") + "/auth/validate"
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()
	var v validateResponse

	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return 0, err
	}

	if !v.Valid {
		return 0, errUnauthorized
	}
	return v.UserID, nil
}

var errUnauthorized = &authError{msg: "unauthorized"}
type authError struct { msg string }

func (e *authError) Error() string { return e.msg }