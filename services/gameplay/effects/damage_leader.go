package effects

import (
	"encoding/json"
	"fmt"
)

type DamageLeaderEffect struct {
	trigger string
	Target  string `json:"target"` // "own_leader"
	Damage  int    `json:"damage"`
}

func NewDamageLeader(trigger string, params json.RawMessage) (Ability, error) {
	e := &DamageLeaderEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *DamageLeaderEffect) TriggerType() string { return e.trigger }

func (e *DamageLeaderEffect) Execute(ctx *EffectContext) []EffectEvent {
	hp := ctx.GetOwnHP()
	*hp -= e.Damage
	return []EffectEvent{
		makeLeaderEvent("on_death", ctx, true, e.Damage,
			fmt.Sprintf("%s deals %d damage to own leader on death", ctx.Source.Definition.Name, e.Damage)),
	}
}
