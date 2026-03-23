package handlers

import (
	"encoding/json"
	"log"
)

// HandleJoinGame handles a player joining the game
func HandleJoinGame(ctx HandlerContext, msg *ClientMessage) error {
	state := ctx.GetGameState()
	playerID := ctx.GetPlayerID()

	// Initialize player's game data if empty
	playerState := ctx.GetPlayerState(playerID)
	if len(playerState.GetGameData()) == 0 {
		ctx.LockState()
		initialData := ClickGameData{ClickCount: 0}
		dataBytes, _ := json.Marshal(initialData)
		playerState.SetGameData(dataBytes)
		ctx.UnlockState()
		log.Printf("Initialized game data for player %d in session %s", playerID, ctx.GetSessionID())
	}

	// Check if both players are connected and initialize game if needed
	if state.GetPhase() == "WAITING_FOR_PLAYERS" {
		// TODO: Check both players connected status
		ctx.LockState()
		state.SetPhase("INITIALIZING")

		// Initialize both players' game data
		p1State := ctx.GetPlayerState(1)
		p2State := ctx.GetPlayerState(2)

		if len(p1State.GetGameData()) == 0 {
			initialData := ClickGameData{ClickCount: 0}
			dataBytes, _ := json.Marshal(initialData)
			p1State.SetGameData(dataBytes)
		}
		if len(p2State.GetGameData()) == 0 {
			initialData := ClickGameData{ClickCount: 0}
			dataBytes, _ := json.Marshal(initialData)
			p2State.SetGameData(dataBytes)
		}

		// For now, just transition to Player 1's turn
		state.SetPhase("PLAYER1_TURN")
		state.SetTurnNumber(1)
		ctx.UnlockState()

		// Start turn timer for Player 1
		ctx.StartTurnTimer(1)

		log.Printf("Game initialized for session %s, turn timer started for Player 1", ctx.GetSessionID())
	}

	// Increment sequence and send state to both players
	ctx.IncrementSequence()

	// Send to player who just joined
	myView := ctx.GetPlayerView(ctx.GetPlayerID())
	ctx.SendStateUpdate("JOIN_GAME", myView)

	// Send to opponent if connected
	opponentView := ctx.GetPlayerView(ctx.GetOpponentID())
	ctx.BroadcastToOpponent("JOIN_GAME", opponentView)

	log.Printf("Player %d joined session %s", ctx.GetPlayerID(), ctx.GetSessionID())
	return nil
}

// HandleReconnect handles a player reconnecting
func HandleReconnect(ctx HandlerContext, msg *ClientMessage) error {
	// Same as join game - send current state
	return HandleJoinGame(ctx, msg)
}
