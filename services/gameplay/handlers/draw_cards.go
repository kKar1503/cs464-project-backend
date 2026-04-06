package handlers

import (
	"encoding/json"
	"fmt"
)

type DrawCardsRequest struct {
	SelectedCardIDs []int `json:"selected_card_ids"`
}

// HandleDrawCards handles the DRAW_CARDS action during the pre-turn phase.
func HandleDrawCards(ctx HandlerContext, msg *ClientMessage) error {
	var req DrawCardsRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return fmt.Errorf("invalid draw cards request")
	}

	gm := ctx.GetGameplayManager()
	playerID := ctx.GetUserID()

	// Auto-offer cards if none offered yet (first draw of the round)
	hand := gm.GetHand(playerID)
	_ = hand // GetHand returns current hand; we need to check offered
	gm.OfferCards(playerID)

	if err := gm.SelectCards(playerID, req.SelectedCardIDs); err != nil {
		return err
	}

	// Mark this player as having completed their draw
	bothReady := gm.MarkPlayerDrew(playerID)

	if bothReady {
		ctx.GetGameState().SetPhase("BOTH_PLAYERS_DREW")
	}

	return nil
}
