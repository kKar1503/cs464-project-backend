package effects

import (
	"encoding/json"
	"fmt"
)

type TransformEffect struct {
	trigger    string
	Target     string `json:"target,omitempty"`     // "enemy_in_front" for Magic Swordman, empty for Pig (self)
	IntoCardID int    `json:"into_card_id"`
	Chance     int    `json:"chance,omitempty"`      // 1/Chance probability, 0 = always
}

func NewTransform(trigger string, params json.RawMessage) (Ability, error) {
	e := &TransformEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *TransformEffect) TriggerType() string { return e.trigger }

func (e *TransformEffect) Execute(ctx *EffectContext) []EffectEvent {
	return e.executeWithDepth(ctx, 0)
}

func (e *TransformEffect) executeWithDepth(ctx *EffectContext, depth int) []EffectEvent {
	if depth >= 5 {
		return nil // prevent infinite transform chains
	}

	// Check chance
	if e.Chance > 0 {
		if ctx.RNG.Intn(e.Chance) != 0 {
			return nil
		}
	}

	newDef := ctx.CardStore.Get(e.IntoCardID)
	if newDef == nil {
		return nil
	}

	var targetCard *CardInstance
	var targetBoard *[2][3]*CardInstance
	var targetRow, targetCol int

	if e.Target == "" {
		// Self-transform (Pig → Technoblade)
		targetCard = ctx.Source
		targetBoard, targetRow, targetCol, _ = FindCardOnAnyBoard(ctx, ctx.Source)
	} else {
		// Target transform (Magic Swordman → enemy in front becomes Pig)
		targets := resolveTargets(e.Target, ctx)
		if len(targets) == 0 {
			return nil
		}
		targetCard = targets[0]
		targetBoard, targetRow, targetCol, _ = FindCardOnAnyBoard(ctx, targetCard)
	}

	if targetBoard == nil {
		return nil
	}

	// Create new card instance
	newInstance, err := NewCardInstance(newDef, targetCard.InstanceID)
	if err != nil {
		return nil
	}

	// Replace on board
	targetBoard[targetRow][targetCol] = newInstance

	events := []EffectEvent{
		makeEvent("transform", ctx, targetCard, e.IntoCardID,
			fmt.Sprintf("%s transforms %s into %s", ctx.Source.Definition.Name, targetCard.Definition.Name, newDef.Name)),
	}

	// Fire summon trigger on the new card (e.g., Pig's summon trigger for Technoblade chain)
	summonCtx := &EffectContext{
		Source:                  newInstance,
		SourcePos:               BoardPosition{targetRow, targetCol},
		SourcePlayerID:          ctx.SourcePlayerID,
		IsPlayer1:               ctx.IsPlayer1,
		Board1:                  ctx.Board1,
		Board2:                  ctx.Board2,
		Player1HP:               ctx.Player1HP,
		Player2HP:               ctx.Player2HP,
		Player1LeaderAtk:        ctx.Player1LeaderAtk,
		Player2LeaderAtk:        ctx.Player2LeaderAtk,
		Player1ElixirCap:        ctx.Player1ElixirCap,
		Player2ElixirCap:        ctx.Player2ElixirCap,
		ReturnToHand:            ctx.ReturnToHand,
		CardStore:               ctx.CardStore,
		SourcePlayerDeckColours: ctx.SourcePlayerDeckColours,
		RNG:                     ctx.RNG,
	}

	// If the transformed card is on the opponent's board, adjust context
	if (ctx.IsPlayer1 && targetBoard == ctx.Board2) || (!ctx.IsPlayer1 && targetBoard == ctx.Board1) {
		summonCtx.IsPlayer1 = !ctx.IsPlayer1
		// Note: SourcePlayerID for the new card should be the board owner, not the caster
	}

	summonEvents := FireTrigger("summon", newInstance, summonCtx)
	events = append(events, summonEvents...)

	return events
}
