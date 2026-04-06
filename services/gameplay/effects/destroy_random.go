package effects

import (
	"encoding/json"
	"fmt"
)

type DestroyRandomEffect struct {
	trigger string
	Target  string `json:"target"` // "random_enemy"
}

func NewDestroyRandom(trigger string, params json.RawMessage) (Ability, error) {
	e := &DestroyRandomEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *DestroyRandomEffect) TriggerType() string { return e.trigger }

func (e *DestroyRandomEffect) Execute(ctx *EffectContext) []EffectEvent {
	enemies := getAllCards(ctx.GetOpponentBoard())
	if len(enemies) == 0 {
		return nil
	}

	target := enemies[ctx.RNG.Intn(len(enemies))]
	target.CurrentHP = 0

	return []EffectEvent{
		makeEvent("on_death", ctx, target, 0,
			fmt.Sprintf("%s destroys %s on death", ctx.Source.Definition.Name, target.Definition.Name)),
	}
}
