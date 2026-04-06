package handlers

import (
	"encoding/json"
	"fmt"
)

type SelectCardRequest struct {
	CardID int `json:"card_id"`
}

type DeselectCardRequest struct {
	CardID int `json:"card_id"`
}

// HandleSelectCard moves a single card from draw pile to hand. Only allowed during PRE_TURN.
func HandleSelectCard(ctx HandlerContext, msg *ClientMessage) error {
	if ctx.GetGameState().GetPhase() != "PRE_TURN" {
		return fmt.Errorf("can only select cards during PRE_TURN phase")
	}

	var req SelectCardRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return fmt.Errorf("invalid select card request")
	}

	return ctx.GetGameplayManager().SelectCard(ctx.GetUserID(), req.CardID)
}

// HandleDeselectCard moves a single card from hand back to draw pile. Only allowed during PRE_TURN.
func HandleDeselectCard(ctx HandlerContext, msg *ClientMessage) error {
	if ctx.GetGameState().GetPhase() != "PRE_TURN" {
		return fmt.Errorf("can only deselect cards during PRE_TURN phase")
	}

	var req DeselectCardRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return fmt.Errorf("invalid deselect card request")
	}

	return ctx.GetGameplayManager().DeselectCard(ctx.GetUserID(), req.CardID)
}
