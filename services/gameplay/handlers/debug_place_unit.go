package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/kKar1503/cs464-backend/services/gameplay/effects"
)

type DebugPlaceUnit struct {
	Row int `json:"row"`
	Col int `json:"col"`
	Atk int `json:"atk"`
	HP  int `json:"hp"`
}

// HandleDebugPlaceUnit places a vanilla unit (no abilities) at the given
// board position. It bypasses hand/deck/elixir checks entirely.
// The underlying CardDefinition uses CardID -1 so it won't collide with
// real cards. If the unit dies it is simply removed (no deck interaction).
func HandleDebugPlaceUnit(ctx HandlerContext, msg *ClientMessage) error {
	if ctx.GetGameState().GetPhase() != "ACTIVE" {
		return fmt.Errorf("can only place cards during ACTIVE phase")
	}

	var req DebugPlaceUnit
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return fmt.Errorf("invalid DEBUG_PLACE_UNIT params: %w", err)
	}

	if req.Row < 0 || req.Row > 1 || req.Col < 0 || req.Col > 2 {
		return fmt.Errorf("invalid board position: row %d, col %d", req.Row, req.Col)
	}

	if req.Atk <= 0 {
		req.Atk = 5
	}
	if req.HP <= 0 {
		req.HP = 5
	}

	def := &effects.CardDefinition{
		CardID:    -1,
		Name:      "Debug Unit",
		Colour:    "neutral",
		Rarity:    "common",
		Cost:      0,
		BaseAtk:   req.Atk,
		BaseHP:    req.HP,
		Abilities: nil, // vanilla — no effects
	}

	instance, err := effects.NewCardInstance(def, -1)
	if err != nil {
		return fmt.Errorf("failed to create debug card instance: %w", err)
	}

	gm := ctx.GetGameplayManager()
	if err := gm.PlaceCard(ctx.GetUserID(), instance, req.Row, req.Col); err != nil {
		return err
	}

	return nil
}
