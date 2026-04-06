package handlers

import (
	"encoding/json"
	"fmt"
)

type SelectCardsRequest struct {
	CardIDs []int `json:"card_ids"`
}

// HandleSelectCards handles moving cards from draw pile to hand during pre-turn.
func HandleSelectCards(ctx HandlerContext, msg *ClientMessage) error {
	var req SelectCardsRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return fmt.Errorf("invalid select cards request")
	}

	gm := ctx.GetGameplayManager()
	playerID := ctx.GetUserID()

	if err := gm.SelectFromDrawPile(playerID, req.CardIDs); err != nil {
		return err
	}

	return nil
}
