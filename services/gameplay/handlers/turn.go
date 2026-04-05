package handlers

import (
	"log"
)

// HandleSurrender handles a player surrendering
func HandleSurrender(ctx HandlerContext, msg *ClientMessage) error {
	state := ctx.GetGameState()

	state.SetPhase("GAME_OVER")
	state.SetWinnerID(ctx.GetOpponentID())

	// Increment sequence
	ctx.IncrementSequence()

	// Send updated state to both players
	myView := ctx.GetPlayerView(ctx.GetPlayerID())
	ctx.SendStateUpdate("SURRENDER", myView)

	opponentView := ctx.GetPlayerView(ctx.GetOpponentID())
	ctx.BroadcastToOpponent("SURRENDER", opponentView)

	log.Printf("Player %d surrendered in session %s - Player %d wins",
		ctx.GetPlayerID(), ctx.GetSessionID(), ctx.GetOpponentID())
	return nil
}
