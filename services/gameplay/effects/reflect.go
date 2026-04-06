package effects

import (
	"encoding/json"
	"fmt"
)

type ReflectEffect struct {
	trigger string
}

func NewReflect(trigger string, params json.RawMessage) (Ability, error) {
	return &ReflectEffect{trigger: trigger}, nil
}

func (e *ReflectEffect) TriggerType() string { return e.trigger }

// Execute reflects damage back to the attacker.
// ctx.Target should be set to the attacker when this is called in on_damaged context.
func (e *ReflectEffect) Execute(ctx *EffectContext) []EffectEvent {
	if ctx.Target == nil {
		return nil
	}

	// The "Source" here is the damaged card (Big Whale), and ctx.Target is the attacker.
	// Reflect the attacker's damage back.
	damage := ctx.Target.CurrentAtk
	ctx.Target.CurrentHP -= damage

	return []EffectEvent{
		makeEvent("on_damaged", ctx, ctx.Target, damage,
			fmt.Sprintf("%s reflects %d damage back to %s", ctx.Source.Definition.Name, damage, ctx.Target.Definition.Name)),
	}
}
