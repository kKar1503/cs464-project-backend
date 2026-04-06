package effects

import (
	"encoding/json"
	"fmt"
)

type ShieldEffect struct {
	trigger   string
	Reduction int `json:"reduction"`
}

func NewShield(trigger string, params json.RawMessage) (Ability, error) {
	e := &ShieldEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *ShieldEffect) TriggerType() string { return e.trigger }

// Execute sets the DamageReduction field on the source card.
// The game loop should check DamageReduction before applying damage,
// then reset it. This is called during the on_damaged trigger.
func (e *ShieldEffect) Execute(ctx *EffectContext) []EffectEvent {
	ctx.Source.DamageReduction = e.Reduction
	return []EffectEvent{
		makeEvent("on_damaged", ctx, ctx.Source, e.Reduction,
			fmt.Sprintf("%s's shield absorbs %d damage", ctx.Source.Definition.Name, e.Reduction)),
	}
}
