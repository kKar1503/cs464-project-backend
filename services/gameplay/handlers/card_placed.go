package handlers

import (
	"encoding/json"
	"fmt"
)

type CardPlaced struct {
	CardID int `json:"card_id"`
	Row    int `json:"row"`
	Col    int `json:"col"`
}

func HandleCardPlaced(ctx HandlerContext, msg *ClientMessage) error {
	var req CardPlaced
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return fmt.Errorf("invalid card placement request")
	}

	// Validate board bounds (2 rows, 3 cols)
	if req.Row < 0 || req.Row > 1 || req.Col < 0 || req.Col > 2 {
		return fmt.Errorf("invalid board position: row %d, col %d", req.Row, req.Col)
	}

	gm := ctx.GetGameplayManager()
	playerID := ctx.GetUserID()

	// Find the card in the player's hand
	hand := gm.GetHand(playerID)
	var handCard *HandCardInfo
	for i := range hand {
		if hand[i].CardID == req.CardID {
			handCard = &hand[i]
			break
		}
	}
	if handCard == nil {
		return fmt.Errorf("card %d is not in your hand", req.CardID)
	}

	// Check elixir
	if gm.GetElixir(playerID) < handCard.ManaCost {
		return fmt.Errorf("not enough elixir: have %d, need %d", gm.GetElixir(playerID), handCard.ManaCost)
	}

	// Build the board card with charge timer
	card := &Card{
		CardID:               handCard.CardID,
		CardName:             handCard.CardName,
		ElixirCost:           handCard.ManaCost,
		CurrentHealth:        handCard.HP,
		MaxHealth:            handCard.HP,
		CardAttack:           handCard.Attack,
		Colour:               handCard.Colour,
		ChargeTicksRemaining: ChargeTicksTotal,
		IsCharging:           true,
	}

	// Place on board (deducts elixir)
	if err := gm.PlaceCard(playerID, card, req.Row, req.Col); err != nil {
		return err
	}

	// Remove from hand
	if err := gm.RemoveFromHand(playerID, req.CardID); err != nil {
		return err
	}

	return nil
}
