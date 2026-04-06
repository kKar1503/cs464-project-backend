package effects

import (
	"encoding/json"
	"fmt"
)

var nextInstanceID = 1000 // global counter for generated instance IDs

type SummonUnitsEffect struct {
	trigger string
	CardID  int `json:"card_id"`
	Count   int `json:"count"`
}

func NewSummonUnits(trigger string, params json.RawMessage) (Ability, error) {
	e := &SummonUnitsEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *SummonUnitsEffect) TriggerType() string { return e.trigger }

func (e *SummonUnitsEffect) Execute(ctx *EffectContext) []EffectEvent {
	def := ctx.CardStore.Get(e.CardID)
	if def == nil {
		return nil
	}

	ownBoard := ctx.GetOwnBoard()
	var events []EffectEvent

	for i := 0; i < e.Count; i++ {
		row, col, found := getRandomOpenSlot(ownBoard, ctx.RNG)
		if !found {
			break // board full
		}

		nextInstanceID++
		instance, err := NewCardInstance(def, nextInstanceID)
		if err != nil {
			continue
		}

		ownBoard[row][col] = instance
		events = append(events, EffectEvent{
			Type:           "summon",
			SourcePlayerID: ctx.SourcePlayerID,
			SourceCardID:   ctx.Source.Definition.CardID,
			SourceRow:      ctx.SourcePos.Row,
			SourceCol:      ctx.SourcePos.Col,
			TargetCardID:   def.CardID,
			TargetRow:      row,
			TargetCol:      col,
			CardName:       def.Name,
			Message:        fmt.Sprintf("%s summons %s at [%d,%d]", ctx.Source.Definition.Name, def.Name, row, col),
		})
	}

	return events
}
