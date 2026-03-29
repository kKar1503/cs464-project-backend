package handlers

import (
	"encoding/json"
	"fmt"
)

type CardPlaced struct {
	PlayerID int  `json:"player_id"`
	CardID   int  `json:"card_id"`
	XPos     uint `json:"x_pos"`
	YPos     uint `json:"y_pos"`
}

func HandleCardPlaced(ctx HandlerContext, msg *ClientMessage) error {
	
	var cardPlaced CardPlaced; 
	var err = json.Unmarshal(msg.Params, &cardPlaced)
	if err != nil {
		return fmt.Errorf("Unable to deserialise json file")
	}

	if cardPlaced.XPos > 2 || cardPlaced.YPos > 3 {
		return fmt.Errorf("Invalid Card Placement")
	}
	
	
	// Somehow check if there is valid elixir for 
	
	
	
	return nil
}
