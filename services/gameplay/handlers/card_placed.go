package handlers

import (
	"encoding/json"
	"fmt"
	"time"
)

type CardPlaced struct {
	PlayerID int64 `json:"player_id"`
	CardID   int   `json:"card_id"`
	XPos     int   `json:"x_pos"`
	YPos     int   `json:"y_pos"`
}

func HandleCardPlaced(ctx HandlerContext, msg *ClientMessage) error {
	var cardPlaced CardPlaced
	var err = json.Unmarshal(msg.Params, &cardPlaced)
	if err != nil {
		return fmt.Errorf("Unable to deserialise json file")
	}

	if cardPlaced.XPos < 0 || cardPlaced.YPos < 0 || cardPlaced.XPos > 2 || cardPlaced.YPos > 3 {
		return fmt.Errorf("Invalid Card Placement")
	}

	// Somehow check if there is valid elixir for
	var GameplayManager = ctx.GetGameplayManager()
	// var isPlayer1 = GameplayManager.GetPlayer1ID() == cardPlaced.PlayerID;
	
	// TODO: Get the proper card request from the other service
	card := &Card{
		CardID:        cardPlaced.CardID,
		ElixerCost:    0,
		CurrentHealth: 0,
		TimeToAttack:  0,
	}

	// For current mock -> true == too little elixer, false is enough
	if GameplayManager.GetElixer(cardPlaced.PlayerID) == 0 { // TODO: Add the condition to get the card information
		return fmt.Errorf("Not enough elixer")
	}

	card.LastMessage = time.Now()

	err = GameplayManager.PlaceCard(cardPlaced.PlayerID, card, cardPlaced.XPos, cardPlaced.YPos)
	if err != nil {
		return err
	}
	
	return nil
}
