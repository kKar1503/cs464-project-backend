package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

// GameAction represents possible actions a player can take
type GameAction string

const (
	ActionJoinGame   GameAction = "JOIN_GAME"
	ActionClick      GameAction = "CLICK"
	ActionEndTurn    GameAction = "END_TURN"
	ActionSurrender  GameAction = "SURRENDER"
	ActionDisconnect GameAction = "DISCONNECT"
	ActionReconnect  GameAction = "RECONNECT"
)

// GamePhase represents the current phase of the game
type GamePhase string

const (
	PhaseWaitingForPlayers GamePhase = "WAITING_FOR_PLAYERS"
	PhaseInitializing      GamePhase = "INITIALIZING"
	PhasePlayer1Turn       GamePhase = "PLAYER1_TURN"
	PhasePlayer2Turn       GamePhase = "PLAYER2_TURN"
	PhaseGameOver          GamePhase = "GAME_OVER"
)

// PlayerID represents which player (1 or 2)
type PlayerID int

const (
	Player1 PlayerID = 1
	Player2 PlayerID = 2
)

// PlayerState represents the state for a single player
// Game-specific data should be stored separately by your game logic
type PlayerState struct {
	UserID         int64     `json:"user_id"`
	Username       string    `json:"username"`
	IsConnected    bool      `json:"is_connected"`
	LastActionTime time.Time `json:"last_action_time"`

	// GameData is where you'll store your custom game state as JSON
	// This allows you to define your own game structure
	GameData       json.RawMessage `json:"game_data,omitempty"`
}

// GameState represents the complete game state
type GameState struct {
	SessionID      string             `json:"session_id"`
	Phase          GamePhase          `json:"phase"`
	TurnNumber     int                `json:"turn_number"`
	CurrentPlayer  PlayerID           `json:"current_player"`
	Player1        *PlayerState       `json:"player1"`
	Player2        *PlayerState       `json:"player2"`
	StartedAt      time.Time          `json:"started_at"`
	LastUpdateAt   time.Time          `json:"last_update_at"`
	WinnerID       PlayerID           `json:"winner_id,omitempty"`

	// Metadata - per-player sequence numbers
	Player1SequenceNumber int64 `json:"player1_sequence_number"` // Increments with each Player1 action
	Player2SequenceNumber int64 `json:"player2_sequence_number"` // Increments with each Player2 action

	mu sync.RWMutex // Protects concurrent access
}

// PlayerView represents a partial view of the game state for a specific player
// Your custom game data should go in YourGameData/OpponentGameData
type PlayerView struct {
	SessionID      string    `json:"session_id"`
	Phase          GamePhase `json:"phase"`
	TurnNumber     int       `json:"turn_number"`
	CurrentPlayer  PlayerID  `json:"current_player"`
	SequenceNumber int64     `json:"sequence_number"`

	// Your player info
	YourUserID     int64  `json:"your_user_id"`
	YourUsername   string `json:"your_username"`
	YourGameData   json.RawMessage `json:"your_game_data,omitempty"`  // Your custom game state

	// Opponent's info
	OpponentUserID    int64  `json:"opponent_user_id"`
	OpponentUsername  string `json:"opponent_username"`
	OpponentConnected bool   `json:"opponent_connected"`
	OpponentGameData  json.RawMessage `json:"opponent_game_data,omitempty"`  // Opponent's custom game state (partial)

	// Computed hash of this view
	StateHash uint64 `json:"state_hash"`
}

// NewGameState creates a new game state for a session
func NewGameState(sessionID string, player1ID, player2ID int64, player1Name, player2Name string) *GameState {
	now := time.Now()

	return &GameState{
		SessionID:      sessionID,
		Phase:          PhaseWaitingForPlayers,
		TurnNumber:     0,
		CurrentPlayer:  Player1,
		Player1: &PlayerState{
			UserID:         player1ID,
			Username:       player1Name,
			IsConnected:    false,
			LastActionTime: now,
			GameData:       nil, // You'll initialize this with your game logic
		},
		Player2: &PlayerState{
			UserID:         player2ID,
			Username:       player2Name,
			IsConnected:    false,
			LastActionTime: now,
			GameData:       nil, // You'll initialize this with your game logic
		},
		StartedAt:             now,
		LastUpdateAt:          now,
		Player1SequenceNumber: 0,
		Player2SequenceNumber: 0,
	}
}

// GetPlayerView generates a partial view for a specific player
func (gs *GameState) GetPlayerView(playerID PlayerID) *PlayerView {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	var yourState, opponentState *PlayerState
	if playerID == Player1 {
		yourState = gs.Player1
		opponentState = gs.Player2
	} else {
		yourState = gs.Player2
		opponentState = gs.Player1
	}

	// Get the sequence number for this specific player
	var playerSeqNum int64
	if playerID == Player1 {
		playerSeqNum = gs.Player1SequenceNumber
	} else {
		playerSeqNum = gs.Player2SequenceNumber
	}

	view := &PlayerView{
		SessionID:      gs.SessionID,
		Phase:          gs.Phase,
		TurnNumber:     gs.TurnNumber,
		CurrentPlayer:  gs.CurrentPlayer,
		SequenceNumber: playerSeqNum, // Per-player sequence

		YourUserID:     yourState.UserID,
		YourUsername:   yourState.Username,
		YourGameData:   yourState.GameData, // Your custom game state

		OpponentUserID:    opponentState.UserID,
		OpponentUsername:  opponentState.Username,
		OpponentConnected: opponentState.IsConnected,
		OpponentGameData:  opponentState.GameData, // Opponent's custom game state (you control what's visible)
	}

	// Compute hash of this view
	view.StateHash = view.ComputeHash()

	return view
}

// ComputeHash computes the xxHash64 of this player view
func (pv *PlayerView) ComputeHash() uint64 {
	h := xxhash.New()

	// Hash all fields in deterministic order
	// Session and game metadata
	h.Write([]byte(pv.SessionID))
	h.Write([]byte(pv.Phase))
	binary.Write(h, binary.LittleEndian, int32(pv.TurnNumber))
	binary.Write(h, binary.LittleEndian, int32(pv.CurrentPlayer))
	binary.Write(h, binary.LittleEndian, pv.SequenceNumber)

	// Your state
	binary.Write(h, binary.LittleEndian, pv.YourUserID)
	h.Write([]byte(pv.YourUsername))
	if pv.YourGameData != nil {
		h.Write(pv.YourGameData)
	}

	// Opponent state
	binary.Write(h, binary.LittleEndian, pv.OpponentUserID)
	h.Write([]byte(pv.OpponentUsername))
	if pv.OpponentConnected {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	if pv.OpponentGameData != nil {
		h.Write(pv.OpponentGameData)
	}

	return h.Sum64()
}

// VerifyHash checks if the provided hash matches the expected hash for this view
func (pv *PlayerView) VerifyHash(providedHash uint64) error {
	expectedHash := pv.ComputeHash()
	if providedHash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %d, got %d", expectedHash, providedHash)
	}
	return nil
}

// IncrementSequence increments the sequence number for a specific player (call after each action)
func (gs *GameState) IncrementSequence(playerID PlayerID) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if playerID == Player1 {
		gs.Player1SequenceNumber++
	} else {
		gs.Player2SequenceNumber++
	}
	gs.LastUpdateAt = time.Now()
}

// GetPlayerSequence returns the sequence number for a specific player
func (gs *GameState) GetPlayerSequence(playerID PlayerID) int64 {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	if playerID == Player1 {
		return gs.Player1SequenceNumber
	}
	return gs.Player2SequenceNumber
}

// GetPlayerState returns the state for a specific player
func (gs *GameState) GetPlayerState(playerID PlayerID) *PlayerState {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	if playerID == Player1 {
		return gs.Player1
	}
	return gs.Player2
}

// SetPlayerConnected updates a player's connection status
func (gs *GameState) SetPlayerConnected(playerID PlayerID, connected bool) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if playerID == Player1 {
		gs.Player1.IsConnected = connected
	} else {
		gs.Player2.IsConnected = connected
	}
	gs.LastUpdateAt = time.Now()
}
