package handlers

import (
	"encoding/json"
	"fmt"
	"log"
)

// ClickGameData represents the game-specific data for the click counter demo
type ClickGameData struct {
	ClickCount int `json:"click_count"`
}

// HandleClick handles a player clicking (incrementing their counter)
func HandleClick(ctx HandlerContext, msg *ClientMessage) error {
	// Verify it's the player's turn
	if !ctx.IsPlayerTurn() {
		return fmt.Errorf("not your turn")
	}

	playerID := ctx.GetPlayerID()
	playerState := ctx.GetPlayerState(playerID)

	ctx.LockState()
	defer ctx.UnlockState()

	// Get current game data (or initialize if nil)
	var gameData ClickGameData
	if len(playerState.GetGameData()) > 0 {
		if err := json.Unmarshal(playerState.GetGameData(), &gameData); err != nil {
			return fmt.Errorf("failed to parse game data: %v", err)
		}
	}

	// Increment click count
	gameData.ClickCount++

	// Save updated game data
	updatedData, err := json.Marshal(gameData)
	if err != nil {
		return fmt.Errorf("failed to marshal game data: %v", err)
	}
	playerState.SetGameData(updatedData)

	// Increment sequence
	ctx.IncrementSequence()

	// Send updated state to both players
	myView := ctx.GetPlayerView(playerID)
	ctx.SendStateUpdate("CLICK", myView)

	opponentView := ctx.GetPlayerView(ctx.GetOpponentID())
	ctx.BroadcastToOpponent("CLICK", opponentView)

	log.Printf("Player %d clicked in session %s (count: %d)",
		playerID, ctx.GetSessionID(), gameData.ClickCount)

	return nil
}
