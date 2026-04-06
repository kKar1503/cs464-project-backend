package effects

import (
	"encoding/json"
)

type SkipFrontRowEffect struct {
	trigger string
}

func NewSkipFrontRow(trigger string, params json.RawMessage) (Ability, error) {
	return &SkipFrontRowEffect{trigger: trigger}, nil
}

func (e *SkipFrontRowEffect) TriggerType() string { return e.trigger }

// Execute is a no-op — targeting is handled by ModifyTarget.
func (e *SkipFrontRowEffect) Execute(ctx *EffectContext) []EffectEvent {
	return nil
}

// ModifyTarget implements TargetModifier to skip the front row.
func (e *SkipFrontRowEffect) ModifyTarget(ctx *EffectContext) *TargetOverride {
	oppBoard := ctx.GetOpponentBoard()
	col := ctx.SourcePos.Col

	// Check back row first (skip front row)
	if oppBoard[1][col] != nil {
		return &TargetOverride{
			TargetCard:     oppBoard[1][col],
			TargetRow:      1,
			TargetCol:      col,
			TargetIsLeader: false,
		}
	}

	// If back row empty, check front row anyway
	if oppBoard[0][col] != nil {
		return &TargetOverride{
			TargetCard:     oppBoard[0][col],
			TargetRow:      0,
			TargetCol:      col,
			TargetIsLeader: false,
		}
	}

	// No cards, target leader
	return &TargetOverride{
		TargetCard:     nil,
		TargetIsLeader: true,
	}
}
