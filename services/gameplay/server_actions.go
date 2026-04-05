package main

import (
	"encoding/json"
	"log"

	"github.com/kKar1503/cs464-backend/services/gameplay/handlers"
)

// Server-initiated actions that don't come from client
const (
	ServerActionRoundEnd           GameAction = "ROUND_END"
	ServerActionOpponentDisconnect GameAction = "OPPONENT_DISCONNECT"
	ServerActionOpponentReconnect  GameAction = "OPPONENT_RECONNECT"
	ServerActionGameStart          GameAction = "GAME_START"
	ServerActionGameEnd            GameAction = "GAME_END"
	ServerActionRoundStart         GameAction = "ROUND_START"
)

// ServerActionContext wraps a game session for server-initiated actions
type ServerActionContext struct {
	Session  *GameSession
	PlayerID PlayerID
}

// ExecuteServerAction executes a server-initiated action
// Unlike client actions, these don't validate sequence/hash beforehand
func (ctx *ServerActionContext) ExecuteServerAction(action GameAction, params interface{}) error {
	log.Printf("Server executing action %s for player %d in session %s", action, ctx.PlayerID, ctx.Session.State.SessionID)

	// Convert params to json.RawMessage
	var paramsJSON json.RawMessage
	if params != nil {
		paramsBytes, err := json.Marshal(params)
		if err != nil {
			return err
		}
		paramsJSON = json.RawMessage(paramsBytes)
	} else {
		paramsJSON = json.RawMessage("{}")
	}

	// Create handler context
	conn := ctx.Session.GetPlayerConnection(ctx.PlayerID)
	if conn == nil {
		// Player not connected, update state anyway
		return ctx.executeActionWithoutConnection(action, paramsJSON)
	}

	handlerCtx := NewHandlerContext(conn)

	// Create message for handler
	handlerMsg := &handlers.ClientMessage{
		Action:         string(action),
		Params:         paramsJSON,
		StateHashAfter: 0, // Server actions don't validate hash
		SequenceNumber: ctx.Session.State.GetPlayerSequence(ctx.PlayerID),
	}

	// Route to handler
	handler := handlers.GetActionHandler(string(action))
	if handler == nil {
		return ctx.executeDefaultServerAction(action, paramsJSON)
	}

	// Execute handler
	if err := handler(handlerCtx, handlerMsg); err != nil {
		log.Printf("Server action %s failed for player %d in session %s: %v", action, ctx.PlayerID, ctx.Session.State.SessionID, err)
		return err
	}

	// Take snapshot
	ctx.Session.SnapshotManager.TakeSnapshot(ctx.Session.State, action, ctx.PlayerID)

	// Broadcast to both players with params
	ctx.broadcastServerAction(action, params)

	log.Printf("Server action %s completed for player %d in session %s", action, ctx.PlayerID, ctx.Session.State.SessionID)
	return nil
}

// executeActionWithoutConnection handles actions when player is disconnected
func (ctx *ServerActionContext) executeActionWithoutConnection(action GameAction, params json.RawMessage) error {
	// For now, just execute the default behavior
	return ctx.executeDefaultServerAction(action, params)
}

// executeDefaultServerAction handles server actions that don't have custom handlers
func (ctx *ServerActionContext) executeDefaultServerAction(action GameAction, params json.RawMessage) error {
	switch action {
	case ServerActionOpponentDisconnect:
		return ctx.handleOpponentDisconnect()
	case ServerActionOpponentReconnect:
		return ctx.handleOpponentReconnect()
	case ServerActionGameStart:
		return ctx.handleGameStart()
	case ServerActionGameEnd:
		return ctx.handleGameEnd(params)
	default:
		log.Printf("Unknown server action: %s", action)
		return nil
	}
}

// handleOpponentDisconnect notifies player that opponent disconnected
func (ctx *ServerActionContext) handleOpponentDisconnect() error {
	// Just broadcast state update
	return nil
}

// handleOpponentReconnect notifies player that opponent reconnected
func (ctx *ServerActionContext) handleOpponentReconnect() error {
	// Just broadcast state update
	return nil
}

// handleGameStart initializes the game (decks, starting hands, etc.)
func (ctx *ServerActionContext) handleGameStart() error {
	state := ctx.Session.State

	state.Phase = PhaseActive
	state.TurnNumber = 1

	// TODO: Initialize decks, shuffle, deal starting hands

	log.Printf("Game started in session %s", ctx.Session.State.SessionID)
	return nil
}

// GameEndParams contains information about game end
type GameEndParams struct {
	Reason   string   `json:"reason"`   // "surrender", "victory", "timeout", "disconnect"
	WinnerID PlayerID `json:"winner_id"`
}

// handleGameEnd ends the game
func (ctx *ServerActionContext) handleGameEnd(params json.RawMessage) error {
	var endParams GameEndParams
	if err := json.Unmarshal(params, &endParams); err != nil {
		return err
	}

	state := ctx.Session.State

	state.Phase = PhaseGameOver
	state.WinnerID = endParams.WinnerID

	log.Printf("Game ended in session %s, winner: Player %d, reason: %s",
		ctx.Session.State.SessionID, endParams.WinnerID, endParams.Reason)
	return nil
}

// broadcastServerAction broadcasts the action result to all connected players
func (ctx *ServerActionContext) broadcastServerAction(action GameAction, params interface{}) {
	// Send to Player 1
	p1Conn := ctx.Session.GetPlayerConnection(Player1)
	if p1Conn != nil {
		p1View := ctx.Session.State.GetPlayerView(Player1)
		p1Conn.SendStateUpdateWithParams(action, p1View, params)
	}

	// Send to Player 2
	p2Conn := ctx.Session.GetPlayerConnection(Player2)
	if p2Conn != nil {
		p2View := ctx.Session.State.GetPlayerView(Player2)
		p2Conn.SendStateUpdateWithParams(action, p2View, params)
	}
}
