package main

import (
	"log"
	"time"

	"github.com/kKar1503/cs464-backend/services/gameplay/handlers"
)

const (
	TickRate     = 4                                    // ticks per second
	TickInterval = time.Second / time.Duration(TickRate) // 250ms
	ElixirEvery  = 20                                    // tick elixir every 20 ticks (5 seconds)
)

// EventType represents the type of game event
type EventType int

const (
	EventClientAction    EventType = iota
	EventTurnTimeout
	EventPlayerDisconnect
	EventShutdown
)

// GameEvent represents an event to be processed by the game loop
type GameEvent struct {
	Type      EventType
	PlayerID  PlayerID
	Message   *ClientMessage // non-nil for EventClientAction
	Conn      *PlayerConnection // the connection that sent this event
	Timestamp time.Time
}

// GameLoop runs the server-authoritative game loop at a fixed tick rate
type GameLoop struct {
	session   *GameSession
	events    chan GameEvent
	done      chan struct{}
	ticker    *time.Ticker
	tickCount uint64
	elixirAcc int
	dirty     bool
}

// NewGameLoop creates a new game loop for a session
func NewGameLoop(session *GameSession) *GameLoop {
	return &GameLoop{
		session: session,
		events:  make(chan GameEvent, 64),
		done:    make(chan struct{}),
	}
}

// Run starts the game loop. Should be called as a goroutine.
func (gl *GameLoop) Run() {
	gl.ticker = time.NewTicker(TickInterval)
	defer gl.ticker.Stop()

	log.Printf("Game loop started for session %s at %d ticks/sec", gl.session.State.SessionID, TickRate)

	for {
		select {
		case <-gl.done:
			log.Printf("Game loop stopped for session %s", gl.session.State.SessionID)
			return
		case <-gl.ticker.C:
			gl.processTick()
		}
	}
}

// QueueEvent adds an event to the game loop's event queue (thread-safe)
func (gl *GameLoop) QueueEvent(event GameEvent) {
	select {
	case gl.events <- event:
	default:
		log.Printf("Game loop event queue full for session %s, dropping event type %d", gl.session.State.SessionID, event.Type)
	}
}

// Stop signals the game loop to shut down
func (gl *GameLoop) Stop() {
	close(gl.done)
}

// processTick handles one game tick
func (gl *GameLoop) processTick() {
	// Drain all queued events
	gl.drainEvents()

	// Tick elixir
	gl.elixirAcc++
	if gl.elixirAcc >= ElixirEvery {
		gl.elixirAcc = 0
		gl.session.GameplayManager.TickElixir()
		gl.dirty = true
	}

	// Check win conditions
	if gameOver, winnerID := gl.session.GameplayManager.CheckWinCondition(); gameOver {
		gl.handleGameOver(winnerID)
	}

	// Broadcast if state changed
	if gl.dirty {
		gl.tickCount++
		gl.session.State.TickNumber = gl.tickCount
		gl.broadcastTickUpdate()
		gl.dirty = false
	}
}

// drainEvents processes all queued events
func (gl *GameLoop) drainEvents() {
	for {
		select {
		case event := <-gl.events:
			gl.handleEvent(event)
		default:
			return
		}
	}
}

// handleEvent processes a single game event
func (gl *GameLoop) handleEvent(event GameEvent) {
	switch event.Type {
	case EventClientAction:
		gl.handleClientAction(event)
	case EventTurnTimeout:
		gl.handleTurnTimeout(event)
	case EventPlayerDisconnect:
		gl.handlePlayerDisconnect(event)
	case EventShutdown:
		gl.Stop()
	}
}

// handleClientAction processes a client action within the game loop
func (gl *GameLoop) handleClientAction(event GameEvent) {
	msg := event.Message
	conn := event.Conn
	if msg == nil || conn == nil {
		return
	}

	log.Printf("Processing action %s from player %d in session %s (tick: %d)",
		msg.Action, event.PlayerID, gl.session.State.SessionID, gl.tickCount)

	// Update session activity
	gl.session.UpdateActivity()

	// Validate tick number — client must be within 8 ticks (2 seconds)
	if gl.tickCount > 8 && uint64(msg.SequenceNumber) < gl.tickCount-8 {
		conn.SendError("Client too far behind server state", msg.Action)
		return
	}

	// Create handler context
	ctx := NewHandlerContext(conn)

	// Build handler message
	handlerMsg := &handlers.ClientMessage{
		Action:         string(msg.Action),
		Params:         msg.Params,
		StateHashAfter: msg.StateHashAfter,
		SequenceNumber: msg.SequenceNumber,
	}

	// Route to handler
	handler := handlers.GetActionHandler(string(msg.Action))
	if handler == nil {
		conn.SendError("Unknown action: "+string(msg.Action), msg.Action)
		return
	}

	// Execute handler (single-threaded, no locks needed)
	if err := handler(ctx, handlerMsg); err != nil {
		conn.SendError(err.Error(), msg.Action)
		log.Printf("Action %s failed for player %d: %v", msg.Action, event.PlayerID, err)
		return
	}

	// Take snapshot
	gl.session.SnapshotManager.TakeSnapshot(gl.session.State, msg.Action, event.PlayerID)

	// Send ACK to the initiating player
	updatedView := gl.session.State.GetPlayerView(event.PlayerID)
	conn.SendActionAck(msg.Action, updatedView)

	gl.dirty = true

	log.Printf("Action %s completed for player %d (tick: %d)", msg.Action, event.PlayerID, gl.tickCount)
}

// handleTurnTimeout processes a turn timeout event
func (gl *GameLoop) handleTurnTimeout(event GameEvent) {
	playerID := event.PlayerID

	log.Printf("Processing turn timeout for player %d in session %s", playerID, gl.session.State.SessionID)

	// Verify it's still this player's turn
	currentPhase := gl.session.State.Phase
	isStillPlayerTurn := (playerID == Player1 && currentPhase == PhasePlayer1Turn) ||
		(playerID == Player2 && currentPhase == PhasePlayer2Turn)

	if !isStillPlayerTurn {
		log.Printf("Player %d already ended turn before timeout, ignoring", playerID)
		return
	}

	// Switch turns
	if playerID == Player1 {
		gl.session.State.Phase = PhasePlayer2Turn
	} else {
		gl.session.State.Phase = PhasePlayer1Turn
		gl.session.State.TurnNumber++
	}
	gl.session.State.LastUpdateAt = time.Now()

	// Start timer for next player
	opponentID := Player1
	if playerID == Player1 {
		opponentID = Player2
	}
	if gl.session.TurnTimer != nil {
		gl.session.TurnTimer.StartTurn(opponentID)
	}

	gl.dirty = true
}

// handlePlayerDisconnect processes a player disconnect event
func (gl *GameLoop) handlePlayerDisconnect(event GameEvent) {
	playerID := event.PlayerID
	log.Printf("Processing disconnect for player %d in session %s", playerID, gl.session.State.SessionID)

	gl.session.State.SetPlayerConnected(playerID, false)
	gl.dirty = true
}

// handleGameOver handles the game ending
func (gl *GameLoop) handleGameOver(winnerID int) {
	if gl.session.State.Phase == PhaseGameOver {
		return // already handled
	}

	winner := Player1
	if winnerID == 2 {
		winner = Player2
	}

	gl.session.State.Phase = PhaseGameOver
	gl.session.State.WinnerID = winner
	gl.session.State.LastUpdateAt = time.Now()
	gl.dirty = true

	// Stop the turn timer
	if gl.session.TurnTimer != nil {
		gl.session.TurnTimer.StopTurn()
	}

	log.Printf("Game over in session %s, winner: Player %d", gl.session.State.SessionID, winner)
}

// broadcastTickUpdate sends the current state to all connected players
func (gl *GameLoop) broadcastTickUpdate() {
	p1Conn := gl.session.GetPlayerConnection(Player1)
	if p1Conn != nil {
		p1View := gl.session.State.GetPlayerView(Player1)
		p1Conn.SendStateUpdateWithParams("TICK_UPDATE", p1View, nil)
	}

	p2Conn := gl.session.GetPlayerConnection(Player2)
	if p2Conn != nil {
		p2View := gl.session.State.GetPlayerView(Player2)
		p2Conn.SendStateUpdateWithParams("TICK_UPDATE", p2View, nil)
	}
}
