package main

import (
	"log"
	"sync"
	"time"
)

const (
	// PreTurnDuration is how long the pre-turn draw phase lasts
	PreTurnDuration = 10 * time.Second
	// RoundDuration is how long each active round lasts (both players act simultaneously)
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

// StartPreTurn starts the 10s pre-turn timer. When it expires, fires EventPreTurnEnd.
func (rt *RoundTimer) StartPreTurn() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.timer != nil {
		rt.timer.Stop()
	}

	rt.stopped = false
	rt.timer = time.AfterFunc(PreTurnDuration, func() {
		rt.mu.Lock()
		if rt.stopped {
			rt.mu.Unlock()
			return
		}
		rt.stopped = true
		rt.mu.Unlock()

		log.Printf("Pre-turn ended in session %s", rt.session.State.SessionID)
		if rt.session.GameLoop != nil {
			rt.session.GameLoop.QueueEvent(GameEvent{
				Type:      EventPreTurnEnd,
				Timestamp: time.Now(),
			})
		}
	})

	log.Printf("Pre-turn started in session %s (%v duration)", rt.session.State.SessionID, PreTurnDuration)
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
