package effects

import "math/rand"

// resolveTargets returns all CardInstances matching the target mode.
// The returned slice contains pointers to the actual board cards so effects can mutate them.
func resolveTargets(targetMode string, ctx *EffectContext) []*CardInstance {
	switch targetMode {
	case "self":
		return []*CardInstance{ctx.Source}

	case "adjacent":
		return getAdjacent(ctx.GetOwnBoard(), ctx.SourcePos)

	case "vertical":
		return getVertical(ctx.GetOwnBoard(), ctx.SourcePos)

	case "all_allies":
		return getAllCards(ctx.GetOwnBoard())

	case "all_enemies":
		return getAllCards(ctx.GetOpponentBoard())

	case "all_units":
		result := getAllCards(ctx.Board1)
		result = append(result, getAllCards(ctx.Board2)...)
		return result

	case "opponent_column":
		return getColumn(ctx.GetOpponentBoard(), ctx.SourcePos.Col)

	case "opponent_back_row":
		return getRow(ctx.GetOpponentBoard(), 1) // row 1 = back

	case "enemy_in_front":
		return getEnemyInFront(ctx.GetOpponentBoard(), ctx.SourcePos)

	case "enemy_front_row":
		return getRow(ctx.GetOpponentBoard(), 0) // row 0 = front

	case "same_colour":
		return getSameColour(ctx, ctx.Source.Definition.Colour)

	case "random_enemy":
		cards := getAllCards(ctx.GetOpponentBoard())
		if len(cards) == 0 {
			return nil
		}
		return []*CardInstance{cards[ctx.RNG.Intn(len(cards))]}

	case "random_all":
		cards := getAllCards(ctx.Board1)
		cards = append(cards, getAllCards(ctx.Board2)...)
		if len(cards) == 0 {
			return nil
		}
		return []*CardInstance{cards[ctx.RNG.Intn(len(cards))]}

	default:
		return nil
	}
}

// getAdjacent returns cards immediately left and right in the same row.
func getAdjacent(board *[2][3]*CardInstance, pos BoardPosition) []*CardInstance {
	var result []*CardInstance
	row := pos.Row
	if pos.Col-1 >= 0 && board[row][pos.Col-1] != nil {
		result = append(result, board[row][pos.Col-1])
	}
	if pos.Col+1 < 3 && board[row][pos.Col+1] != nil {
		result = append(result, board[row][pos.Col+1])
	}
	return result
}

// getVertical returns cards in the same column but different row.
func getVertical(board *[2][3]*CardInstance, pos BoardPosition) []*CardInstance {
	var result []*CardInstance
	otherRow := 1 - pos.Row
	if board[otherRow][pos.Col] != nil {
		result = append(result, board[otherRow][pos.Col])
	}
	return result
}

// getAllCards returns all non-nil cards on a board.
func getAllCards(board *[2][3]*CardInstance) []*CardInstance {
	var result []*CardInstance
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] != nil {
				result = append(result, board[r][c])
			}
		}
	}
	return result
}

// getColumn returns all non-nil cards in a specific column on a board.
func getColumn(board *[2][3]*CardInstance, col int) []*CardInstance {
	var result []*CardInstance
	for r := 0; r < 2; r++ {
		if board[r][col] != nil {
			result = append(result, board[r][col])
		}
	}
	return result
}

// getRow returns all non-nil cards in a specific row on a board.
func getRow(board *[2][3]*CardInstance, row int) []*CardInstance {
	var result []*CardInstance
	for c := 0; c < 3; c++ {
		if board[row][c] != nil {
			result = append(result, board[row][c])
		}
	}
	return result
}

// getEnemyInFront returns the enemy card directly in front (same column, front row first, then back row).
func getEnemyInFront(opponentBoard *[2][3]*CardInstance, sourcePos BoardPosition) []*CardInstance {
	col := sourcePos.Col
	// Front row first
	if opponentBoard[0][col] != nil {
		return []*CardInstance{opponentBoard[0][col]}
	}
	// Then back row
	if opponentBoard[1][col] != nil {
		return []*CardInstance{opponentBoard[1][col]}
	}
	return nil
}

// getSameColour returns all units on both boards matching the given colour, excluding the source.
func getSameColour(ctx *EffectContext, colour string) []*CardInstance {
	var result []*CardInstance
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if ctx.Board1[r][c] != nil && ctx.Board1[r][c] != ctx.Source && ctx.Board1[r][c].Definition.Colour == colour {
				result = append(result, ctx.Board1[r][c])
			}
			if ctx.Board2[r][c] != nil && ctx.Board2[r][c] != ctx.Source && ctx.Board2[r][c].Definition.Colour == colour {
				result = append(result, ctx.Board2[r][c])
			}
		}
	}
	return result
}

// findCardPosition finds the row and col of a card on a specific board.
func findCardPosition(board *[2][3]*CardInstance, card *CardInstance) (int, int, bool) {
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] == card {
				return r, c, true
			}
		}
	}
	return 0, 0, false
}

// FindCardOnAnyBoard finds a card on either board and returns its position and which board.
func FindCardOnAnyBoard(ctx *EffectContext, card *CardInstance) (board *[2][3]*CardInstance, row, col int, found bool) {
	r, c, ok := findCardPosition(ctx.Board1, card)
	if ok {
		return ctx.Board1, r, c, true
	}
	r, c, ok = findCardPosition(ctx.Board2, card)
	if ok {
		return ctx.Board2, r, c, true
	}
	return nil, 0, 0, false
}

// getRandomOpenSlot finds a random empty slot on a board.
func getRandomOpenSlot(board *[2][3]*CardInstance, rng *rand.Rand) (int, int, bool) {
	var slots []BoardPosition
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] == nil {
				slots = append(slots, BoardPosition{r, c})
			}
		}
	}
	if len(slots) == 0 {
		return 0, 0, false
	}
	slot := slots[rng.Intn(len(slots))]
	return slot.Row, slot.Col, true
}

// hasEnemyInFront checks if there's an enemy unit in front of the source card.
func hasEnemyInFront(ctx *EffectContext) bool {
	targets := getEnemyInFront(ctx.GetOpponentBoard(), ctx.SourcePos)
	return len(targets) > 0
}

// makeEvent is a helper to create an EffectEvent.
func makeEvent(eventType string, ctx *EffectContext, target *CardInstance, value int, msg string) EffectEvent {
	ev := EffectEvent{
		Type:           eventType,
		SourcePlayerID: ctx.SourcePlayerID,
		SourceCardID:   ctx.Source.Definition.CardID,
		SourceRow:      ctx.SourcePos.Row,
		SourceCol:      ctx.SourcePos.Col,
		Value:          value,
		CardName:       ctx.Source.Definition.Name,
		Message:        msg,
	}
	if target != nil {
		ev.TargetCardID = target.Definition.CardID
		// Try to find target position
		_, r, c, found := FindCardOnAnyBoard(ctx, target)
		if found {
			ev.TargetRow = r
			ev.TargetCol = c
		}
	}
	return ev
}

// makeLeaderEvent creates an event targeting a leader.
func makeLeaderEvent(eventType string, ctx *EffectContext, isOwnLeader bool, value int, msg string) EffectEvent {
	return EffectEvent{
		Type:           eventType,
		SourcePlayerID: ctx.SourcePlayerID,
		SourceCardID:   ctx.Source.Definition.CardID,
		SourceRow:      ctx.SourcePos.Row,
		SourceCol:      ctx.SourcePos.Col,
		TargetIsLeader: true,
		Value:          value,
		CardName:       ctx.Source.Definition.Name,
		Message:        msg,
	}
}

