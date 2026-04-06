package effects

import (
	"encoding/json"
)

type RandomTargetEffect struct {
	trigger string
	Pool    string `json:"pool"` // "enemy" or "all"
}

func NewRandomTarget(trigger string, params json.RawMessage) (Ability, error) {
	e := &RandomTargetEffect{trigger: trigger}
	if err := json.Unmarshal(params, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *RandomTargetEffect) TriggerType() string { return e.trigger }

// Execute is a no-op — targeting is handled by ModifyTarget.
func (e *RandomTargetEffect) Execute(ctx *EffectContext) []EffectEvent {
	return nil
}

// ModifyTarget implements TargetModifier to override attack targeting.
func (e *RandomTargetEffect) ModifyTarget(ctx *EffectContext) *TargetOverride {
	var candidates []*CardInstance
	var candidateBoards []*[2][3]*CardInstance

	ownBoard := ctx.GetOwnBoard()
	oppBoard := ctx.GetOpponentBoard()

	// Collect enemy units
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if oppBoard[r][c] != nil {
				candidates = append(candidates, oppBoard[r][c])
				candidateBoards = append(candidateBoards, oppBoard)
			}
		}
	}

	// For "all" pool, also include friendly units (excluding self) and both leaders
	if e.Pool == "all" {
		for r := 0; r < 2; r++ {
			for c := 0; c < 3; c++ {
				if ownBoard[r][c] != nil && ownBoard[r][c] != ctx.Source {
					candidates = append(candidates, ownBoard[r][c])
					candidateBoards = append(candidateBoards, ownBoard)
				}
			}
		}
	}

	// Add leaders as options (represented as nil cards)
	// We use a special convention: leader targets have TargetIsLeader=true
	leaderCount := 0
	if e.Pool == "enemy" {
		leaderCount = 1 // enemy leader only
	} else {
		leaderCount = 2 // both leaders
	}

	totalCandidates := len(candidates) + leaderCount
	if totalCandidates == 0 {
		return nil
	}

	pick := ctx.RNG.Intn(totalCandidates)

	if pick < len(candidates) {
		target := candidates[pick]
		board := candidateBoards[pick]
		r, c, _ := findCardPosition(board, target)

		// Determine if target is an opponent
		isOpponent := (board == oppBoard)
		var targetPlayerID int64
		if isOpponent {
			targetPlayerID = 0 // opponent sentinel
		} else {
			targetPlayerID = ctx.SourcePlayerID
		}

		return &TargetOverride{
			TargetCard:     target,
			TargetRow:      r,
			TargetCol:      c,
			TargetIsLeader: false,
			TargetPlayerID: targetPlayerID,
		}
	}

	// Picked a leader
	leaderIndex := pick - len(candidates)
	if leaderIndex == 0 {
		// Enemy leader
		return &TargetOverride{
			TargetCard:     nil,
			TargetIsLeader: true,
			TargetPlayerID: 0, // opponent
		}
	}
	// Own leader (only for "all" pool)
	return &TargetOverride{
		TargetCard:     nil,
		TargetIsLeader: true,
		TargetPlayerID: ctx.SourcePlayerID,
	}
}
