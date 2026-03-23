package handlers

import (
	"encoding/json"
)

// PlayCardParams represents parameters for playing a card
type PlayCardParams struct {
	CardID int  `json:"card_id"`
	Target *int `json:"target,omitempty"` // Target card ID or nil for no target
}

// AttackParams represents parameters for an attack action
type AttackParams struct {
	AttackerID int `json:"attacker_id"` // Card ID of attacker
	TargetID   int `json:"target_id"`   // Card ID of target (or 0 for face/player)
}

// ClientMessage represents a message from the client
// Params is json.RawMessage - unparsed JSON that handler will decode based on action type
type ClientMessage struct {
	Action         string          `json:"action"`
	Params         json.RawMessage `json:"params"`            // Raw JSON params (handler parses based on action)
	StateHashAfter uint64          `json:"state_hash_after"`  // Hash of client's state AFTER applying action locally
	SequenceNumber int64           `json:"sequence_number"`
}
