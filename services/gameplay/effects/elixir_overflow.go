package effects

import (
	"encoding/json"
	"fmt"
)

type ElixirOverflowEffect struct {
	trigger string
	Amount  int `json:"amount"`
}

func NewElixirOverflow(trigger string, params json.RawMessage) (Ability, error) {
	e := &ElixirOverflowEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *ElixirOverflowEffect) TriggerType() string { return e.trigger }

func (e *ElixirOverflowEffect) Execute(ctx *EffectContext) []EffectEvent {
	cap := ctx.GetOwnElixirCap()
	*cap += e.Amount
	return []EffectEvent{
		makeEvent("summon_effect", ctx, ctx.Source, e.Amount,
			fmt.Sprintf("%s increases max elixir by %d", ctx.Source.Definition.Name, e.Amount)),
	}
}
