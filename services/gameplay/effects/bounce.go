package effects

import (
	"encoding/json"
	"fmt"
)

type BounceEffect struct {
	trigger string
	Target  string `json:"target"`
	Chance  int    `json:"chance,omitempty"` // 1/Chance probability, 0 = always
}

func NewBounce(trigger string, params json.RawMessage) (Ability, error) {
	e := &BounceEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *BounceEffect) TriggerType() string { return e.trigger }

func (e *BounceEffect) Execute(ctx *EffectContext) []EffectEvent {
	// Check chance
	if e.Chance > 0 {
		if ctx.RNG.Intn(e.Chance) != 0 {
			return nil
		}
	}

	targets := resolveTargets(e.Target, ctx)
	if len(targets) == 0 {
		return nil
	}

	var events []EffectEvent
	for _, t := range targets {
		// Remove from board
		board, r, c, found := FindCardOnAnyBoard(ctx, t)
		if !found {
			continue
		}
		board[r][c] = nil

		// Return to owner's hand
		if ctx.ReturnToHand != nil {
			// Determine which player owns this card
			var ownerID int64
			if board == ctx.Board1 {
				// This card was on Board1 — it belongs to player1
				// We need the player1 ID; use SourcePlayerID logic
				if ctx.IsPlayer1 {
					// Source is player1, opponent board is board2, so board1 card belongs to source
					ownerID = ctx.SourcePlayerID
				} else {
					// Source is player2, so board1 card belongs to opponent
					ownerID = 0 // sentinel — caller must handle
				}
			} else {
				if ctx.IsPlayer1 {
					ownerID = 0 // opponent
				} else {
					ownerID = ctx.SourcePlayerID
				}
			}
			ctx.ReturnToHand(ownerID, t.Definition)
		}

		events = append(events, makeEvent("bounce", ctx, t, 0,
			fmt.Sprintf("%s bounces %s back to hand", ctx.Source.Definition.Name, t.Definition.Name)))
	}

	return events
}
