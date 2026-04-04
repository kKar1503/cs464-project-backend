package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const (
	authServiceURL        = "http://localhost:8000"
	matchmakingServiceURL = "http://localhost:8001"
)

// IntegrationTest tests the full matchmaking flow with real services
func TestMatchmakingIntegration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Step 1: Register two users via auth service
	user1 := registerUser(t, "testplayer1", "password123")
	user2 := registerUser(t, "testplayer2", "password123")

	t.Logf("Created users: %d and %d", user1.UserID, user2.UserID)

	// Give a small delay for services to sync
	time.Sleep(100 * time.Millisecond)

	// Step 2: Both users join the matchmaking queue
	joinQueue(t, user1.UserID)
	joinQueue(t, user2.UserID)

	t.Logf("Both users joined the queue")

	// Step 3: Wait for matchmaker loop to run (runs every 2 seconds)
	// Add extra buffer time to ensure at least one tick happens
	time.Sleep(3 * time.Second)

	// Step 4: Check if both users have been matched
	match1 := checkMatch(t, user1.UserID)
	match2 := checkMatch(t, user2.UserID)

	// Step 5: Verify they were matched together
	if !match1.Matched {
		t.Fatalf("User 1 was not matched")
	}
	if !match2.Matched {
		t.Fatalf("User 2 was not matched")
	}

	if match1.SessionID != match2.SessionID {
		t.Fatalf("Users were matched to different sessions: %s vs %s", match1.SessionID, match2.SessionID)
	}

	if match1.Opponent != "testplayer2" {
		t.Errorf("User 1's opponent should be testplayer2, got %s", match1.Opponent)
	}
	if match2.Opponent != "testplayer1" {
		t.Errorf("User 2's opponent should be testplayer1, got %s", match2.Opponent)
	}

	t.Logf("✅ Successfully matched users into session: %s", match1.SessionID)
	t.Logf("   User 1 (%s) MMR: %d", "testplayer1", match1.YourMMR)
	t.Logf("   User 2 (%s) MMR: %d", "testplayer2", match2.YourMMR)
}

// registerUser registers a new user and returns their user ID
func registerUser(t *testing.T, username, password string) RegisterResponse {
	reqBody := map[string]string{
		"username": username,
		"password": password,
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(
		authServiceURL+"/auth/register",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		t.Fatalf("Failed to register user %s: %v", username, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to register user %s: status %d, body: %s", username, resp.StatusCode, string(body))
	}

	var result RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode register response: %v", err)
	}

	return result
}

// joinQueue adds a user to the matchmaking queue
func joinQueue(t *testing.T, userID int64) {
	reqBody := map[string]int64{
		"user_id": userID,
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(
		matchmakingServiceURL+"/matchmaking/join",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		t.Fatalf("Failed to join queue for user %d: %v", userID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to join queue for user %d: status %d, body: %s", userID, resp.StatusCode, string(body))
	}

	t.Logf("User %d joined matchmaking queue", userID)
}

// checkMatch checks if a user has been matched
func checkMatch(t *testing.T, userID int64) MatchCheckResponse {
	resp, err := http.Get(
		fmt.Sprintf("%s/matchmaking/match?user_id=%d", matchmakingServiceURL, userID),
	)
	if err != nil {
		t.Fatalf("Failed to check match for user %d: %v", userID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to check match for user %d: status %d, body: %s", userID, resp.StatusCode, string(body))
	}

	var result MatchCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode match response: %v", err)
	}

	return result
}

// Response types
type RegisterResponse struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type MatchCheckResponse struct {
	Matched   bool   `json:"matched"`
	SessionID string `json:"session_id"`
	Opponent  string `json:"opponent"`
	YourMMR   int    `json:"your_mmr"`
	TheirMMR  int    `json:"their_mmr"`
}
