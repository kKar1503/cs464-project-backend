package effects

import (
	"encoding/json"
	"fmt"
)

type DoubleAttackSpeedEffect struct {
	trigger string
	Chance  int  `json:"chance,omitempty"` // 1/Chance probability, 0 = always
	Invert  bool `json:"invert,omitempty"` // true = set speed to 0 (Lazy Chick)
}

func NewDoubleAttackSpeed(trigger string, params json.RawMessage) (Ability, error) {
	e := &DoubleAttackSpeedEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *DoubleAttackSpeedEffect) TriggerType() string { return e.trigger }

func (e *DoubleAttackSpeedEffect) Execute(ctx *EffectContext) []EffectEvent {
	// Check chance
	if e.Chance > 0 {
		if ctx.RNG.Intn(e.Chance) != 0 {
			return nil
		}
	}

	if e.Invert {
		// Lazy Chick: set attack speed to 0 (never attacks)
		ctx.Source.ChargeTicksTotal = 0
		ctx.Source.ChargeTicksRemaining = 0
		ctx.Source.IsCharging = false
		return []EffectEvent{
			makeEvent("summon_effect", ctx, ctx.Source, 0,
				fmt.Sprintf("%s's attack speed is set to 0!", ctx.Source.Definition.Name)),
		}
	}

	// Double attack speed = halve the charge time
	ctx.Source.ChargeTicksTotal /= 2
	if ctx.Source.ChargeTicksTotal < 1 {
		ctx.Source.ChargeTicksTotal = 1
	}
	ctx.Source.ChargeTicksRemaining = ctx.Source.ChargeTicksTotal

	return []EffectEvent{
		makeEvent("summon_effect", ctx, ctx.Source, 0,
			fmt.Sprintf("%s doubles attack speed!", ctx.Source.Definition.Name)),
	}
}
