package effects

import (
	"encoding/json"
	"fmt"
)

type ResetAttackEffect struct {
	trigger string
	Target  string `json:"target"`
}

func NewResetAttack(trigger string, params json.RawMessage) (Ability, error) {
	e := &ResetAttackEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *ResetAttackEffect) TriggerType() string { return e.trigger }

func (e *ResetAttackEffect) Execute(ctx *EffectContext) []EffectEvent {
	targets := resolveTargets(e.Target, ctx)
	var events []EffectEvent
	for _, t := range targets {
		t.ChargeTicksRemaining = t.ChargeTicksTotal
		events = append(events, makeEvent("summon_effect", ctx, t, 0,
			fmt.Sprintf("%s resets %s's attack gauge", ctx.Source.Definition.Name, t.Definition.Name)))
	}
	return events
}
