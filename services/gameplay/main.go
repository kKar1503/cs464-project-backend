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
	"github.com/kKar1503/cs464-backend/services/gameplay/effects"
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

	// Load card definitions from deck service (for transform/summon effects)
	loadCardDefinitions()

	// Start cleanup goroutine for inactive sessions
	go sessionCleanupLoop()

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /ws", handleWebSocket)
	mux.HandleFunc("GET /game/stats", handleGameStats)

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
		// Load both players' active decks from deck service
		loadPlayerDeck(session, player1ID)
		loadPlayerDeck(session, player2ID)
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

// cardDefinitionStore holds all card definitions, loaded at startup.
var cardDefinitionStore *effects.CardDefinitionStore

type abilityJSON struct {
	TriggerType string          `json:"trigger_type"`
	EffectType  string          `json:"effect_type"`
	Params      json.RawMessage `json:"params"`
}

// activeDeckResponse is the JSON response from the deck service's internal endpoint.
type activeDeckResponse struct {
	DeckID int `json:"deck_id"`
	Cards  []struct {
		CardID    int           `json:"card_id"`
		CardName  string        `json:"card_name"`
		Colour    string        `json:"colour"`
		Rarity    string        `json:"rarity"`
		ManaCost  int           `json:"mana_cost"`
		Attack    int           `json:"attack"`
		HP        int           `json:"hp"`
		Abilities []abilityJSON `json:"abilities"`
	} `json:"cards"`
}

// loadCardDefinitions fetches all card definitions from the deck service at startup.
func loadCardDefinitions() {
	deckServiceURL := os.Getenv("DECK_SERVICE_URL")
	if deckServiceURL == "" {
		deckServiceURL = "http://deck-service:8005"
	}

	resp, err := http.Get(fmt.Sprintf("%s/internal/card-definitions", deckServiceURL))
	if err != nil {
		log.Printf("WARNING: Failed to load card definitions from deck service: %v", err)
		cardDefinitionStore = effects.NewCardDefinitionStore(nil)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("WARNING: Deck service returned %d for card definitions", resp.StatusCode)
		cardDefinitionStore = effects.NewCardDefinitionStore(nil)
		return
	}

	var result struct {
		Cards []struct {
			CardID    int           `json:"card_id"`
			CardName  string        `json:"card_name"`
			Colour    string        `json:"colour"`
			Rarity    string        `json:"rarity"`
			ManaCost  int           `json:"mana_cost"`
			Attack    int           `json:"attack"`
			HP        int           `json:"hp"`
			Abilities []abilityJSON `json:"abilities"`
		} `json:"cards"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("WARNING: Failed to decode card definitions: %v", err)
		cardDefinitionStore = effects.NewCardDefinitionStore(nil)
		return
	}

	var defs []*effects.CardDefinition
	for _, c := range result.Cards {
		var abilities []effects.AbilityDefinition
		for _, a := range c.Abilities {
			abilities = append(abilities, effects.AbilityDefinition{
				TriggerType: a.TriggerType,
				EffectType:  a.EffectType,
				Params:      a.Params,
			})
		}
		defs = append(defs, &effects.CardDefinition{
			CardID:    c.CardID,
			Name:      c.CardName,
			Colour:    c.Colour,
			Rarity:    c.Rarity,
			Cost:      c.ManaCost,
			BaseAtk:   c.Attack,
			BaseHP:    c.HP,
			Abilities: abilities,
		})
	}

	cardDefinitionStore = effects.NewCardDefinitionStore(defs)
	log.Printf("Loaded %d card definitions from deck service", len(defs))
}

// loadPlayerDeck calls the deck service to get a player's active deck and loads it into the session.
func loadPlayerDeck(session *GameSession, userID int64) {
	deckServiceURL := os.Getenv("DECK_SERVICE_URL")
	if deckServiceURL == "" {
		deckServiceURL = "http://deck-service:8005"
	}

	resp, err := http.Get(fmt.Sprintf("%s/internal/active-deck?user_id=%d", deckServiceURL, userID))
	if err != nil {
		log.Printf("Failed to call deck service for player %d: %v", userID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Deck service returned %d for player %d active deck", resp.StatusCode, userID)
		return
	}

	var deckResp activeDeckResponse
	if err := json.NewDecoder(resp.Body).Decode(&deckResp); err != nil {
		log.Printf("Failed to decode deck response for player %d: %v", userID, err)
		return
	}

	var deck []HandCard
	for _, c := range deckResp.Cards {
		var abilities []effects.AbilityDefinition
		for _, a := range c.Abilities {
			abilities = append(abilities, effects.AbilityDefinition{
				TriggerType: a.TriggerType,
				EffectType:  a.EffectType,
				Params:      a.Params,
			})
		}
		deck = append(deck, HandCard{
			CardID:    c.CardID,
			CardName:  c.CardName,
			Colour:    c.Colour,
			Rarity:    c.Rarity,
			ManaCost:  c.ManaCost,
			Attack:    c.Attack,
			HP:        c.HP,
			Abilities: abilities,
		})
	}

	session.GameplayManager.SetPlayerDeck(userID, deck)
	log.Printf("Loaded deck (id=%d, %d cards) for player %d", deckResp.DeckID, len(deck), userID)
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
