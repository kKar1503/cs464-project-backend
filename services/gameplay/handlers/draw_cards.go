package handlers

import (
	"encoding/json"
	"fmt"
)

type DrawCardsRequest struct {
	SelectedCardIDs []int `json:"selected_card_ids"`
}

// HandleDrawCards handles the DRAW_CARDS action during the pre-turn phase.
// The pre-turn phase lasts a fixed 10 seconds regardless of when players draw.
func HandleDrawCards(ctx HandlerContext, msg *ClientMessage) error {
	var req DrawCardsRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return fmt.Errorf("invalid draw cards request")
	}

	gm := ctx.GetGameplayManager()
	playerID := ctx.GetUserID()

	// Auto-offer cards if none offered yet
	gm.OfferCards(playerID)

	if err := gm.SelectCards(playerID, req.SelectedCardIDs); err != nil {
		return err
	}

	// Mark this player as having completed their draw
	gm.MarkPlayerDrew(playerID)

	return nil
}
