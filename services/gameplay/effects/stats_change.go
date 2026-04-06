package effects

import (
	"encoding/json"
	"fmt"
)

type statsChangeBuff struct {
	Attack int `json:"attack"`
	HP     int `json:"hp"`
}

type StatsChangeEffect struct {
	trigger        string
	Target         string          `json:"target"`
	AtkDelta       int             `json:"attack"`
	HPDelta        int             `json:"hp"`
	Condition      string          `json:"condition,omitempty"`
	PerColourBuff  *statsChangeBuff `json:"per_colour_in_deck,omitempty"`
}

func NewStatsChange(trigger string, params json.RawMessage) (Ability, error) {
	e := &StatsChangeEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *StatsChangeEffect) TriggerType() string { return e.trigger }

func (e *StatsChangeEffect) Execute(ctx *EffectContext) []EffectEvent {
	// Check optional condition
	if e.Condition == "enemy_in_front" && !hasEnemyInFront(ctx) {
		return nil
	}

	atk, hp := e.AtkDelta, e.HPDelta

	// Travelling Merchant: +5/+5 per unique colour in deck
	if e.PerColourBuff != nil {
		colours := countUniqueColours(ctx.SourcePlayerDeckColours)
		atk = e.PerColourBuff.Attack * colours
		hp = e.PerColourBuff.HP * colours
	}

	targets := resolveTargets(e.Target, ctx)
	var events []EffectEvent
	for _, t := range targets {
		t.CurrentAtk += atk
		t.CurrentHP += hp
		if hp > 0 {
			t.MaxHP += hp
		}
		eventType := "buff"
		if atk < 0 || hp < 0 {
			eventType = "debuff"
		}
		events = append(events, makeEvent(eventType, ctx, t, atk,
			fmt.Sprintf("%s gives %s %+d/%+d", ctx.Source.Definition.Name, t.Definition.Name, atk, hp)))
	}
	return events
}

func countUniqueColours(colours []string) int {
	seen := make(map[string]bool)
	for _, c := range colours {
		if c != "" && c != "Colourless" {
			seen[c] = true
		}
	}
	return len(seen)
}
