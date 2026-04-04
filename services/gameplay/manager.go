package main

import (
	"fmt"
	"sync"
	"time"
)

const (
	// SnapshotBufferSize defines how many snapshots to keep in memory before persisting
	SnapshotBufferSize = 100

	// SnapshotPersistThreshold defines when to trigger persistence (e.g., every 20 snapshots)
	SnapshotPersistThreshold = 20
)

// GameSession represents an active game session with its state and connections
type GameSession struct {
	State           *GameState
	SnapshotManager *SnapshotManager
	Player1Conn     *PlayerConnection
	Player2Conn     *PlayerConnection
	TurnTimer       *TurnTimer
	CreatedAt       time.Time
	LastActivityAt  time.Time
	GameplayManager *GameplayManager
	mu              sync.RWMutex
}

// NewGameSession creates a new game session
func NewGameSession(sessionID string, player1ID, player2ID int64, player1Name, player2Name string) *GameSession {
	now := time.Now()
	session := &GameSession{
		State:           NewGameState(sessionID, player1ID, player2ID, player1Name, player2Name),
		SnapshotManager: NewSnapshotManager(sessionID, SnapshotBufferSize),
		CreatedAt:       now,
		GameplayManager: NewGameplayManager(sessionID, player1ID, player2ID),
		LastActivityAt:  now,
	}
	session.TurnTimer = NewTurnTimer(session)
	return session
}

// UpdateActivity updates the last activity timestamp
func (gs *GameSession) UpdateActivity() {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.LastActivityAt = time.Now()
}

// GetPlayerConnection returns the connection for a specific player
func (gs *GameSession) GetPlayerConnection(playerID PlayerID) *PlayerConnection {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	if playerID == Player1 {
		return gs.Player1Conn
	}
	return gs.Player2Conn
}

// SetPlayerConnection sets the connection for a specific player
func (gs *GameSession) SetPlayerConnection(playerID PlayerID, conn *PlayerConnection) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if playerID == Player1 {
		gs.Player1Conn = conn
	} else {
		gs.Player2Conn = conn
	}
}

// BroadcastToOpponent sends a message to the opponent
func (gs *GameSession) BroadcastToOpponent(playerID PlayerID, message *ServerMessage) error {
	var opponentConn *PlayerConnection
	if playerID == Player1 {
		opponentConn = gs.GetPlayerConnection(Player2)
	} else {
		opponentConn = gs.GetPlayerConnection(Player1)
	}

	if opponentConn == nil {
		return fmt.Errorf("opponent not connected")
	}

	return opponentConn.SendMessage(message)
}

// BroadcastToAll sends a message to both players
func (gs *GameSession) BroadcastToAll(message *ServerMessage) {
	p1Conn := gs.GetPlayerConnection(Player1)
	p2Conn := gs.GetPlayerConnection(Player2)

	if p1Conn != nil {
		p1Conn.SendMessage(message)
	}

	if p2Conn != nil {
		p2Conn.SendMessage(message)
	}
}

// GameStateManager manages all active game sessions
type GameStateManager struct {
	sessions map[string]*GameSession // sessionID -> GameSession
	mu       sync.RWMutex
}

// NewGameStateManager creates a new game state manager
func NewGameStateManager() *GameStateManager {
	return &GameStateManager{
		sessions: make(map[string]*GameSession),
	}
}

// CreateSession creates a new game session
func (gsm *GameStateManager) CreateSession(sessionID string, player1ID, player2ID int64, player1Name, player2Name string) (*GameSession, error) {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	// Check if session already exists
	if _, exists := gsm.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	session := NewGameSession(sessionID, player1ID, player2ID, player1Name, player2Name)
	gsm.sessions[sessionID] = session

	return session, nil
}

// GetSession retrieves a game session by ID
func (gsm *GameStateManager) GetSession(sessionID string) (*GameSession, error) {
	gsm.mu.RLock()
	defer gsm.mu.RUnlock()

	session, exists := gsm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

// GetOrCreateSession retrieves an existing session or creates a new one
func (gsm *GameStateManager) GetOrCreateSession(sessionID string, player1ID, player2ID int64, player1Name, player2Name string) (*GameSession, bool, error) {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	session, exists := gsm.sessions[sessionID]
	if exists {
		return session, false, nil
	}

	session = NewGameSession(sessionID, player1ID, player2ID, player1Name, player2Name)
	gsm.sessions[sessionID] = session

	return session, true, nil
}

// RemoveSession removes a game session
func (gsm *GameStateManager) RemoveSession(sessionID string) error {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	if _, exists := gsm.sessions[sessionID]; !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	delete(gsm.sessions, sessionID)
	return nil
}

// GetAllSessionIDs returns all active session IDs
func (gsm *GameStateManager) GetAllSessionIDs() []string {
	gsm.mu.RLock()
	defer gsm.mu.RUnlock()

	ids := make([]string, 0, len(gsm.sessions))
	for id := range gsm.sessions {
		ids = append(ids, id)
	}
	return ids
}

// SessionCount returns the number of active sessions
func (gsm *GameStateManager) SessionCount() int {
	gsm.mu.RLock()
	defer gsm.mu.RUnlock()
	return len(gsm.sessions)
}

// CleanupInactiveSessions removes sessions that have been inactive for too long
func (gsm *GameStateManager) CleanupInactiveSessions(inactiveThreshold time.Duration) int {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	now := time.Now()
	removed := 0

	for sessionID, session := range gsm.sessions {
		session.mu.RLock()
		lastActivity := session.LastActivityAt
		phase := session.State.Phase
		session.mu.RUnlock()

		// Only cleanup sessions that are finished or have been inactive
		if phase == PhaseGameOver || now.Sub(lastActivity) > inactiveThreshold {
			delete(gsm.sessions, sessionID)
			removed++
		}
	}

	return removed
}
