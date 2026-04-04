package handlers

import (
	"encoding/json"
	"fmt"
)

type AttackRequest struct {
	PlayerID int64 `json:"player_id"`
	XPos     int   `json:"x_pos"`
	YPos     int   `json:"y_pos"`
}

func HandleAttackRequest(ctx HandlerContext, msg *ClientMessage) error {
	var attReq AttackRequest
	var err = json.Unmarshal(msg.Params, &attReq)
	if err != nil {
		return fmt.Errorf("Unable to deserialise json file")
	}

	if attReq.XPos < 0 || attReq.YPos < 0 || attReq.XPos > 2 || attReq.YPos > 3 {
		return fmt.Errorf("Invalid Card Placement")
	}

	var GameplayManager = ctx.GetGameplayManager()
	
	err = GameplayManager.AttackCard(attReq.PlayerID, attReq.XPos, attReq.YPos)
	
	if err != nil {
		return fmt.Errorf("Card attempted attack too early")
	}

	return nil
}
