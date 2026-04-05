package handlers

import (
	"encoding/json"
	"fmt"
)

type DrawCardsRequest struct {
	SelectedCardIDs []int `json:"selected_card_ids"`
}

// HandleDrawCards handles the DRAW_CARDS action during the pre-turn phase.
// The client sends the card IDs they want to pick from the offered set.
func HandleDrawCards(ctx HandlerContext, msg *ClientMessage) error {
	var req DrawCardsRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return fmt.Errorf("invalid draw cards request")
	}

	gm := ctx.GetGameplayManager()
	playerID := ctx.GetUserID()

	if err := gm.SelectCards(playerID, req.SelectedCardIDs); err != nil {
		return err
	}

	return nil
}
