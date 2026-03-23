package main

import (
	"log"
	"sync"
	"time"
)

const (
	// TurnTimeout is how long a player has to take their turn
	TurnTimeout = 90 * time.Second
)

// TurnTimer manages turn timeouts for a game session
type TurnTimer struct {
	session       *GameSession
	timer         *time.Timer
	currentPlayer PlayerID
	mu            sync.Mutex
	stopped       bool
}

// NewTurnTimer creates a new turn timer
func NewTurnTimer(session *GameSession) *TurnTimer {
	return &TurnTimer{
		session: session,
		stopped: true,
	}
}

// StartTurn starts the timer for a player's turn
func (tt *TurnTimer) StartTurn(playerID PlayerID) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	// Stop existing timer
	if tt.timer != nil {
		tt.timer.Stop()
	}

	tt.currentPlayer = playerID
	tt.stopped = false

	// Start new timer
	tt.timer = time.AfterFunc(TurnTimeout, func() {
		tt.onTimeout()
	})

	log.Printf("Turn timer started for player %d in session %s (%v timeout)",
		playerID, tt.session.State.SessionID, TurnTimeout)
}

// StopTurn stops the current turn timer
func (tt *TurnTimer) StopTurn() {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	if tt.timer != nil {
		tt.timer.Stop()
		tt.timer = nil
	}
	tt.stopped = true

	log.Printf("Turn timer stopped for session %s", tt.session.State.SessionID)
}

// ResetTurn resets the timer for the current player
func (tt *TurnTimer) ResetTurn() {
	tt.mu.Lock()
	currentPlayer := tt.currentPlayer
	tt.mu.Unlock()

	if !tt.stopped {
		tt.StartTurn(currentPlayer)
	}
}

// onTimeout is called when the timer expires
func (tt *TurnTimer) onTimeout() {
	tt.mu.Lock()
	if tt.stopped {
		tt.mu.Unlock()
		return
	}
	playerID := tt.currentPlayer
	tt.stopped = true
	tt.mu.Unlock()

	log.Printf("Player %d timed out in session %s", playerID, tt.session.State.SessionID)

	// Check if it's still this player's turn
	currentPhase := tt.session.State.Phase
	isStillPlayerTurn := (playerID == Player1 && currentPhase == PhasePlayer1Turn) ||
		(playerID == Player2 && currentPhase == PhasePlayer2Turn)

	if !isStillPlayerTurn {
		log.Printf("Player %d already ended turn before timeout, ignoring", playerID)
		return
	}

	// Execute server-initiated end turn with timeout reason
	ctx := &ServerActionContext{
		Session:  tt.session,
		PlayerID: playerID,
	}

	turnEndParams := TurnEndParams{
		Reason:   TurnEndReasonTimedOut,
		PlayerID: playerID,
	}

	if err := ctx.ExecuteServerAction(ServerActionTurnEnd, turnEndParams); err != nil {
		log.Printf("Failed to execute timeout end turn for player %d: %v", playerID, err)
		return
	}

	// Start timer for next player
	opponentID := Player1
	if playerID == Player1 {
		opponentID = Player2
	}
	tt.StartTurn(opponentID)
}

// Shutdown stops the timer completely
func (tt *TurnTimer) Shutdown() {
	tt.StopTurn()
}
