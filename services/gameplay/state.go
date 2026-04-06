package main

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

// GameAction represents possible actions a player can take
type GameAction string

const (
	ActionJoinGame   GameAction = "JOIN_GAME"
	ActionSurrender  GameAction = "SURRENDER"
	ActionDisconnect GameAction = "DISCONNECT"
	ActionReconnect  GameAction = "RECONNECT"
	ActionCardPlaced    GameAction = "CARD_PLACED"
	ActionDebugPlaceUnit GameAction = "DEBUG_PLACE_UNIT"
)

// GamePhase represents the current phase of the game
type GamePhase string

const (
	PhaseWaitingForPlayers GamePhase = "WAITING_FOR_PLAYERS"
	PhaseInitializing      GamePhase = "INITIALIZING"
	PhasePreTurn  GamePhase = "PRE_TURN" // draw phase — both players pick cards
	PhaseActive   GamePhase = "ACTIVE"   // both players act simultaneously
	PhaseGameOver GamePhase = "GAME_OVER"
)

// PlayerID represents which player (1 or 2)
type PlayerID int

const (
	Player1 PlayerID = 1
	Player2 PlayerID = 2
)

// PlayerState represents the state for a single player
type PlayerState struct {
	UserID         int64     `json:"user_id"`
	Username       string    `json:"username"`
	IsConnected    bool      `json:"is_connected"`
	LastActionTime time.Time `json:"last_action_time"`
}

// GameState represents the complete game state
type GameState struct {
	SessionID string    `json:"session_id"`
	Phase     GamePhase `json:"phase"`
	Player1   *PlayerState `json:"player1"`
	Player2   *PlayerState `json:"player2"`
	StartedAt    time.Time `json:"started_at"`
	LastUpdateAt time.Time `json:"last_update_at"`
	WinnerID     PlayerID  `json:"winner_id,omitempty"`

	// Per-player sequence numbers
	Player1SequenceNumber int64 `json:"player1_sequence_number"`
	Player2SequenceNumber int64 `json:"player2_sequence_number"`

	// Turn number tracks the round (used by game loop)
	TurnNumber int `json:"-"`

	// Tick-based sequencing (server-authoritative)
	TickNumber uint64 `json:"tick_number"`

	mu sync.RWMutex
}

// PlayerView represents a partial view of the game state for a specific player
type PlayerView struct {
	SessionID      string    `json:"session_id"`
	Phase          GamePhase `json:"phase"`
	SequenceNumber int64     `json:"sequence_number"`

	// Your player info
	YourUserID   int64  `json:"your_user_id"`
	YourUsername string `json:"your_username"`

	// Opponent's info
	OpponentUserID    int64  `json:"opponent_user_id"`
	OpponentUsername  string `json:"opponent_username"`
	OpponentConnected bool   `json:"opponent_connected"`

	// Server tick number
	TickNumber uint64 `json:"tick_number"`

	// Computed hash of this view
	StateHash uint64 `json:"state_hash"`
}

// NewGameState creates a new game state for a session
func NewGameState(sessionID string, player1ID, player2ID int64, player1Name, player2Name string) *GameState {
	now := time.Now()

	return &GameState{
		SessionID: sessionID,
		Phase:     PhaseWaitingForPlayers,
		Player1: &PlayerState{
			UserID:         player1ID,
			Username:       player1Name,
			IsConnected:    false,
			LastActionTime: now,
		},
		Player2: &PlayerState{
			UserID:         player2ID,
			Username:       player2Name,
			IsConnected:    false,
			LastActionTime: now,
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

	var playerSeqNum int64
	if playerID == Player1 {
		playerSeqNum = gs.Player1SequenceNumber
	} else {
		playerSeqNum = gs.Player2SequenceNumber
	}

	view := &PlayerView{
		SessionID:      gs.SessionID,
		Phase:          gs.Phase,
		SequenceNumber: playerSeqNum,

		YourUserID:   yourState.UserID,
		YourUsername: yourState.Username,

		OpponentUserID:    opponentState.UserID,
		OpponentUsername:  opponentState.Username,
		OpponentConnected: opponentState.IsConnected,

		TickNumber: gs.TickNumber,
	}

	view.StateHash = view.ComputeHash()
	return view
}

// ComputeHash computes the xxhash64 of this player view
func (pv *PlayerView) ComputeHash() uint64 {
	h := xxhash.New()

	h.Write([]byte(pv.SessionID))
	h.Write([]byte(pv.Phase))
	binary.Write(h, binary.LittleEndian, pv.SequenceNumber)
	binary.Write(h, binary.LittleEndian, pv.TickNumber)

	binary.Write(h, binary.LittleEndian, pv.YourUserID)
	h.Write([]byte(pv.YourUsername))

	binary.Write(h, binary.LittleEndian, pv.OpponentUserID)
	h.Write([]byte(pv.OpponentUsername))
	if pv.OpponentConnected {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
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

// IncrementSequence increments the sequence number for a specific player
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
