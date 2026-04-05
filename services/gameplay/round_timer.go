package main

import (
	"log"
	"sync"
	"time"
)

const (
	// RoundDuration is how long each round lasts (both players act simultaneously)
	RoundDuration = 30 * time.Second
)

// RoundTimer manages round timeouts for a game session.
// When the timer expires, it queues a round-end event to the game loop.
type RoundTimer struct {
	session      *GameSession
	timer        *time.Timer
	currentRound int
	mu           sync.Mutex
	stopped      bool
}

// NewRoundTimer creates a new round timer
func NewRoundTimer(session *GameSession) *RoundTimer {
	return &RoundTimer{
		session: session,
		stopped: true,
	}
}

// StartRound starts the timer for a new round
func (rt *RoundTimer) StartRound(roundNumber int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.timer != nil {
		rt.timer.Stop()
	}

	rt.currentRound = roundNumber
	rt.stopped = false

	rt.timer = time.AfterFunc(RoundDuration, func() {
		rt.onTimeout()
	})

	log.Printf("Round %d started in session %s (%v duration)", roundNumber, rt.session.State.SessionID, RoundDuration)
}

// Stop stops the current round timer
func (rt *RoundTimer) Stop() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.timer != nil {
		rt.timer.Stop()
		rt.timer = nil
	}
	rt.stopped = true
}

// onTimeout is called when the round timer expires
func (rt *RoundTimer) onTimeout() {
	rt.mu.Lock()
	if rt.stopped {
		rt.mu.Unlock()
		return
	}
	round := rt.currentRound
	rt.stopped = true
	rt.mu.Unlock()

	log.Printf("Round %d ended in session %s", round, rt.session.State.SessionID)

	if rt.session.GameLoop != nil {
		rt.session.GameLoop.QueueEvent(GameEvent{
			Type:      EventRoundEnd,
			Timestamp: time.Now(),
		})
	}
}

// Shutdown stops the timer completely
func (rt *RoundTimer) Shutdown() {
	rt.Stop()
}
