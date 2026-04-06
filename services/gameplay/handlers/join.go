package handlers

import (
	"log"
)

// HandleJoinGame handles a player joining the game
func HandleJoinGame(ctx HandlerContext, msg *ClientMessage) error {
	state := ctx.GetGameState()

	// Check if both players are connected and initialize game if needed
	if state.GetPhase() == "WAITING_FOR_PLAYERS" {
		ctx.LockState()
		state.SetPhase("INITIALIZING")

		// Start in pre-turn phase for card drawing
		state.SetPhase("PRE_TURN")
		ctx.UnlockState()

		log.Printf("Game initialized for session %s, entering PRE_TURN", ctx.GetSessionID())
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
	return HandleJoinGame(ctx, msg)
}
