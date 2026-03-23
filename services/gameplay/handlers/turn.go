package handlers

import (
	"fmt"
	"log"
)

// TurnEndParams contains information about turn end
type TurnEndParams struct {
	Reason   string `json:"reason"`    // "player_initiated" or "timed_out"
	PlayerID int    `json:"player_id"` // Which player's turn ended
}

// HandleEndTurn handles a player ending their turn
func HandleEndTurn(ctx HandlerContext, msg *ClientMessage) error {
	// Verify it's the player's turn
	if !ctx.IsPlayerTurn() {
		return fmt.Errorf("not your turn")
	}

	playerID := ctx.GetPlayerID()

	// Stop the current timer
	ctx.StopTurnTimer()

	// Execute server action TURN_END with player_initiated reason
	turnEndParams := TurnEndParams{
		Reason:   "player_initiated",
		PlayerID: playerID,
	}

	if err := ctx.ExecuteServerAction("TURN_END", turnEndParams); err != nil {
		log.Printf("Failed to execute TURN_END for player %d: %v", playerID, err)
		return err
	}

	// Start timer for next player
	opponentID := ctx.GetOpponentID()
	ctx.StartTurnTimer(opponentID)

	log.Printf("Player %d ended turn in session %s", playerID, ctx.GetSessionID())
	return nil
}

// HandleSurrender handles a player surrendering
func HandleSurrender(ctx HandlerContext, msg *ClientMessage) error {
	state := ctx.GetGameState()

	ctx.LockState()
	state.SetPhase("GAME_OVER")
	state.SetWinnerID(ctx.GetOpponentID())
	ctx.UnlockState()

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
