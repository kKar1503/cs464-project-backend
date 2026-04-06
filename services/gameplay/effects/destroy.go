package effects

import (
	"encoding/json"
	"fmt"
)

type destroySelfBuff struct {
	Attack int `json:"attack"`
	HP     int `json:"hp"`
}

type DestroyEffect struct {
	trigger        string
	Target         string           `json:"target"`
	SelfBuffPerKill *destroySelfBuff `json:"self_buff_per_kill,omitempty"`
}

func NewDestroy(trigger string, params json.RawMessage) (Ability, error) {
	e := &DestroyEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *DestroyEffect) TriggerType() string { return e.trigger }

func (e *DestroyEffect) Execute(ctx *EffectContext) []EffectEvent {
	var events []EffectEvent
	targets := resolveTargets(e.Target, ctx)

	if len(targets) == 0 {
		return nil
	}

	killCount := 0
	for _, t := range targets {
		t.CurrentHP = 0
		killCount++
		events = append(events, makeEvent("summon_effect", ctx, t, 0,
			fmt.Sprintf("%s destroys %s", ctx.Source.Definition.Name, t.Definition.Name)))
	}

	// Town Hero: self buff per kill
	if e.SelfBuffPerKill != nil && killCount > 0 {
		atkBuff := e.SelfBuffPerKill.Attack * killCount
		hpBuff := e.SelfBuffPerKill.HP * killCount
		ctx.Source.CurrentAtk += atkBuff
		ctx.Source.CurrentHP += hpBuff
		ctx.Source.MaxHP += hpBuff
		events = append(events, makeEvent("buff", ctx, ctx.Source, atkBuff,
			fmt.Sprintf("%s gains +%d/+%d from %d kills", ctx.Source.Definition.Name, atkBuff, hpBuff, killCount)))
	}

	return events
}
