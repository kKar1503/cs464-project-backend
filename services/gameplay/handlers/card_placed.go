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

	if req.Row < 0 || req.Row > 1 || req.Col < 0 || req.Col > 2 {
		return fmt.Errorf("invalid board position: row %d, col %d", req.Row, req.Col)
	}

	gm := ctx.GetGameplayManager()
	playerID := ctx.GetUserID()

	// Take the card from the hand (also returns it to back of deck)
	handCard, err := gm.PlayFromHand(playerID, req.CardID)
	if err != nil {
		return err
	}

	if gm.GetElixir(playerID) < handCard.ManaCost {
		return fmt.Errorf("not enough elixir: have %d, need %d", gm.GetElixir(playerID), handCard.ManaCost)
	}

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

	if err := gm.PlaceCard(playerID, card, req.Row, req.Col); err != nil {
		return err
	}

	return nil
}
