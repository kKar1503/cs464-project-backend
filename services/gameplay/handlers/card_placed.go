package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/kKar1503/cs464-backend/services/gameplay/effects"
)

func HandleCardPlaced(ctx HandlerContext, msg *ClientMessage) error {
	if ctx.GetGameState().GetPhase() != "ACTIVE" {
		return fmt.Errorf("can only place cards during ACTIVE phase")
	}

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

	// Build CardDefinition from hand card data
	def := &effects.CardDefinition{
		CardID:    handCard.CardID,
		Name:      handCard.CardName,
		Colour:    handCard.Colour,
		Rarity:    handCard.Rarity,
		Cost:      handCard.ManaCost,
		BaseAtk:   handCard.Attack,
		BaseHP:    handCard.HP,
		Abilities: handCard.Abilities,
	}

	// Create a CardInstance with resolved abilities
	instance, err := effects.NewCardInstance(def, handCard.CardID)
	if err != nil {
		return fmt.Errorf("failed to create card instance: %w", err)
	}

	if err := gm.PlaceCard(playerID, instance, req.Row, req.Col); err != nil {
		return err
	}

	// Fire summon effects
	gm.FireSummonEffects(playerID, instance, req.Row, req.Col)

	return nil
}
