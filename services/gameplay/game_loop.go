package main

import (
	"log"
	"time"

	"github.com/kKar1503/cs464-backend/services/gameplay/effects"
	"github.com/kKar1503/cs464-backend/services/gameplay/handlers"
)

const (
	TickRate     = 4                                    // ticks per second
	TickInterval = time.Second / time.Duration(TickRate) // 250ms
)

// EventType represents the type of game event
type EventType int

const (
	EventClientAction    EventType = iota
	EventPreTurnEnd                // pre-turn timer expired → start active phase
	EventRoundEnd                  // round timer expired → next pre-turn
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

	phase := gl.session.State.Phase
	if phase == PhaseActive {
		// Tick elixir (only during active phase)
		if gl.session.GameplayManager.TickElixir() {
			gl.dirty = true
		}
		// Tick card charges and resolve attacks
		if gl.session.GameplayManager.TickBoard() {
			gl.dirty = true
		}
	}

	// Check win conditions (always)
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
	case EventPreTurnEnd:
		gl.handlePreTurnEnd()
	case EventRoundEnd:
		gl.handleRoundEnd()
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

	// If JOIN_GAME just set PRE_TURN, top up draw piles and start the pre-turn timer
	if string(msg.Action) == "JOIN_GAME" && gl.session.State.Phase == PhasePreTurn {
		gm := gl.session.GameplayManager
		gm.TopUpDrawPile(gm.player1ID)
		gm.TopUpDrawPile(gm.player2ID)
		if gl.session.RoundTimer != nil {
			gl.session.RoundTimer.StartPreTurn()
		}
	}

	// Take snapshot
	gl.session.SnapshotManager.TakeSnapshot(gl.session.State, msg.Action, event.PlayerID)

	// Send ACK to the initiating player
	updatedView := gl.session.State.GetPlayerView(event.PlayerID)
	conn.SendActionAck(msg.Action, updatedView)

	gl.dirty = true

	log.Printf("Action %s completed for player %d (tick: %d)", msg.Action, event.PlayerID, gl.tickCount)
}

// enterPreTurn sets up the pre-turn phase: tops up draw piles, starts 10s timer.
func (gl *GameLoop) enterPreTurn() {
	gm := gl.session.GameplayManager
	gl.session.State.Phase = PhasePreTurn
	gl.session.State.TurnNumber = gm.game.RoundNumber
	gl.session.State.LastUpdateAt = time.Now()

	// Top up both players' draw piles from their decks
	gm.TopUpDrawPile(gm.player1ID)
	gm.TopUpDrawPile(gm.player2ID)

	if gl.session.RoundTimer != nil {
		gl.session.RoundTimer.StartPreTurn()
	}
}

// handlePreTurnEnd fires when the 10s pre-turn timer expires — transition to active phase
func (gl *GameLoop) handlePreTurnEnd() {
	if gl.session.State.Phase == PhaseGameOver {
		return
	}
	log.Printf("Pre-turn ended, starting active phase in session %s", gl.session.State.SessionID)
	gl.StartRound()
}

// handleRoundEnd processes a round ending — advances to next round's pre-turn phase
func (gl *GameLoop) handleRoundEnd() {
	if gl.session.State.Phase == PhaseGameOver {
		return
	}
	log.Printf("Round %d ended in session %s", gl.session.State.TurnNumber, gl.session.State.SessionID)

	// Advance round in gameplay manager (increases elixir cap)
	gl.session.GameplayManager.AdvanceRound()

	// Move to pre-turn phase — offer cards and start 10s timer
	gl.enterPreTurn()

	gl.dirty = true
}

// StartRound transitions from pre-turn to active and starts the round timer.
// Called after both players have completed their draw phase.
func (gl *GameLoop) StartRound() {
	gm := gl.session.GameplayManager

	// Clear per-turn selection tracking — cards selected this pre-turn are now locked in hand
	gm.ClearSelectedThisTurn(gm.player1ID)
	gm.ClearSelectedThisTurn(gm.player2ID)

	gl.session.State.Phase = PhaseActive
	gl.session.State.LastUpdateAt = time.Now()

	if gl.session.RoundTimer != nil {
		gl.session.RoundTimer.StartRound(gm.game.RoundNumber)
	}

	gl.dirty = true
	log.Printf("Round %d active in session %s", gl.session.GameplayManager.game.RoundNumber, gl.session.State.SessionID)
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

	// Stop the round timer
	if gl.session.RoundTimer != nil {
		gl.session.RoundTimer.Stop()
	}

	log.Printf("Game over in session %s, winner: Player %d", gl.session.State.SessionID, winner)
}

// BoardCardView is the client-facing representation of a card on the board.
type BoardCardView struct {
	CardID               int    `json:"card_id"`
	CardName             string `json:"card_name"`
	Colour               string `json:"colour"`
	CurrentHealth        int    `json:"current_health"`
	MaxHealth            int    `json:"max_health"`
	CardAttack           int    `json:"card_attack"`
	ChargeTicksRemaining int    `json:"charge_ticks_remaining"`
	ChargeTicksTotal     int    `json:"charge_ticks_total"`
	Row                  int    `json:"row"`
	Col                  int    `json:"col"`
}

// HandCardView is a card in the draw pile or hand.
type HandCardView struct {
	CardID   int    `json:"card_id"`
	CardName string `json:"card_name"`
	Colour   string `json:"colour"`
	ManaCost int    `json:"mana_cost"`
	Attack   int    `json:"attack"`
	HP       int    `json:"hp"`
}

// TickUpdateParams is sent with each tick update.
type TickUpdateParams struct {
	MilliElixir int              `json:"milli_elixir"`
	Elixir      int              `json:"elixir"`
	ElixirCap   int              `json:"elixir_cap"`
	YourBoard   []BoardCardView  `json:"your_board"`
	EnemyBoard  []BoardCardView  `json:"enemy_board"`
	YourHP      int              `json:"your_hp"`
	EnemyHP     int              `json:"enemy_hp"`
	LeaderAtk   int              `json:"leader_atk"`
	DrawPile    []HandCardView   `json:"draw_pile"`
	Hand        []HandCardView   `json:"hand"`
	DeckSize    int              `json:"deck_size"`
	Phase       string           `json:"phase"`
	RoundNumber int              `json:"round_number"`
	WinnerID    int              `json:"winner_id,omitempty"`
	CombatLog   []CombatEvent    `json:"combat_log,omitempty"`
}

func handCardsToView(cards []HandCard) []HandCardView {
	result := make([]HandCardView, len(cards))
	for i, c := range cards {
		result[i] = HandCardView{
			CardID: c.CardID, CardName: c.CardName, Colour: c.Colour,
			ManaCost: c.ManaCost, Attack: c.Attack, HP: c.HP,
		}
	}
	return result
}

func boardToView(board *[2][3]*effects.CardInstance) []BoardCardView {
	var cards []BoardCardView
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] != nil {
				card := board[r][c]
				cards = append(cards, BoardCardView{
					CardID:               card.Definition.CardID,
					CardName:             card.Definition.Name,
					Colour:               card.Definition.Colour,
					CurrentHealth:        card.CurrentHP,
					MaxHealth:            card.MaxHP,
					CardAttack:           card.CurrentAtk,
					ChargeTicksRemaining: card.ChargeTicksRemaining,
					ChargeTicksTotal:     card.ChargeTicksTotal,
					Row:                  r,
					Col:                  c,
				})
			}
		}
	}
	if cards == nil {
		cards = []BoardCardView{}
	}
	return cards
}

// broadcastTickUpdate sends the current state to all connected players
func (gl *GameLoop) broadcastTickUpdate() {
	gm := gl.session.GameplayManager
	g := gm.game

	p1Conn := gl.session.GetPlayerConnection(Player1)
	if p1Conn != nil {
		p1View := gl.session.State.GetPlayerView(Player1)
		p1Conn.SendStateUpdateWithParams("TICK_UPDATE", p1View, TickUpdateParams{
			MilliElixir: gm.GetMilliElixir(gm.player1ID),
			Elixir:      gm.GetElixirDisplay(gm.player1ID),
			ElixirCap:   g.ElixirCap,
			YourBoard:   boardToView(&g.BoardPlayer1),
			EnemyBoard:  boardToView(&g.BoardPlayer2),
			YourHP:      g.Player1Health,
			EnemyHP:     g.Player2Health,
			LeaderAtk:   g.Player1LeaderAtk,
			DrawPile:    handCardsToView(g.Player1Hand.DrawPile),
			Hand:        handCardsToView(g.Player1Hand.Hand),
			DeckSize:    len(g.Player1Hand.Deck),
			Phase:       string(gl.session.State.Phase),
			RoundNumber: g.RoundNumber,
			WinnerID:    int(gl.session.State.WinnerID),
			CombatLog:   g.CombatLog,
		})
	}

	p2Conn := gl.session.GetPlayerConnection(Player2)
	if p2Conn != nil {
		p2View := gl.session.State.GetPlayerView(Player2)
		p2Conn.SendStateUpdateWithParams("TICK_UPDATE", p2View, TickUpdateParams{
			MilliElixir: gm.GetMilliElixir(gm.player2ID),
			Elixir:      gm.GetElixirDisplay(gm.player2ID),
			ElixirCap:   g.ElixirCap,
			YourBoard:   boardToView(&g.BoardPlayer2),
			EnemyBoard:  boardToView(&g.BoardPlayer1),
			YourHP:      g.Player2Health,
			EnemyHP:     g.Player1Health,
			LeaderAtk:   g.Player2LeaderAtk,
			DrawPile:    handCardsToView(g.Player2Hand.DrawPile),
			Hand:        handCardsToView(g.Player2Hand.Hand),
			DeckSize:    len(g.Player2Hand.Deck),
			Phase:       string(gl.session.State.Phase),
			RoundNumber: g.RoundNumber,
			WinnerID:    int(gl.session.State.WinnerID),
			CombatLog:   g.CombatLog,
		})
	}
}
