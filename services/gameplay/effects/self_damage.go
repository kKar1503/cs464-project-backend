package effects

import (
	"encoding/json"
	"fmt"
)

type SelfDamageEffect struct {
	trigger string
	Damage  int `json:"damage"`
}

func NewSelfDamage(trigger string, params json.RawMessage) (Ability, error) {
	e := &SelfDamageEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *SelfDamageEffect) TriggerType() string { return e.trigger }

func (e *SelfDamageEffect) Execute(ctx *EffectContext) []EffectEvent {
	ctx.Source.CurrentHP -= e.Damage
	return []EffectEvent{
		makeEvent("on_attack", ctx, ctx.Source, e.Damage,
			fmt.Sprintf("%s deals %d damage to itself", ctx.Source.Definition.Name, e.Damage)),
	}
}
