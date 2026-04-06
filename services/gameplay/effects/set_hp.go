package effects

import (
	"encoding/json"
	"fmt"
)

type SetHPEffect struct {
	trigger string
	Target  string `json:"target"`
	HP      int    `json:"hp"`
}

func NewSetHP(trigger string, params json.RawMessage) (Ability, error) {
	e := &SetHPEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *SetHPEffect) TriggerType() string { return e.trigger }

func (e *SetHPEffect) Execute(ctx *EffectContext) []EffectEvent {
	targets := resolveTargets(e.Target, ctx)
	var events []EffectEvent
	for _, t := range targets {
		t.CurrentHP = e.HP
		t.MaxHP = e.HP
		events = append(events, makeEvent("summon_effect", ctx, t, e.HP,
			fmt.Sprintf("%s sets %s HP to %d", ctx.Source.Definition.Name, t.Definition.Name, e.HP)))
	}
	return events
}
