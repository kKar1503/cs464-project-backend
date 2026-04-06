package effects

import (
	"encoding/json"
	"fmt"
)

type DealDamageEffect struct {
	trigger string
	Target  string `json:"target"`
	Damage  int    `json:"damage"`
}

func NewDealDamage(trigger string, params json.RawMessage) (Ability, error) {
	e := &DealDamageEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *DealDamageEffect) TriggerType() string { return e.trigger }

func (e *DealDamageEffect) Execute(ctx *EffectContext) []EffectEvent {
	var events []EffectEvent

	switch e.Target {
	case "enemy_leader":
		hp := ctx.GetOpponentHP()
		*hp -= e.Damage
		events = append(events, makeLeaderEvent("summon_effect", ctx, false, e.Damage,
			fmt.Sprintf("%s deals %d damage to enemy leader", ctx.Source.Definition.Name, e.Damage)))

	case "all_units":
		targets := resolveTargets("all_units", ctx)
		for _, t := range targets {
			t.CurrentHP -= e.Damage
			events = append(events, makeEvent("summon_effect", ctx, t, e.Damage,
				fmt.Sprintf("%s deals %d damage to %s", ctx.Source.Definition.Name, e.Damage, t.Definition.Name)))
		}

	default:
		targets := resolveTargets(e.Target, ctx)
		for _, t := range targets {
			t.CurrentHP -= e.Damage
			events = append(events, makeEvent("summon_effect", ctx, t, e.Damage,
				fmt.Sprintf("%s deals %d damage to %s", ctx.Source.Definition.Name, e.Damage, t.Definition.Name)))
		}
	}

	return events
}
