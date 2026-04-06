package handlers

import (
	"encoding/json"
	"math/rand"

	"github.com/kKar1503/cs464-backend/services/gameplay/effects"
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

type CardPlaced struct {
	CardID int `json:"card_id"`
	Row    int `json:"row"`
	Col    int `json:"col"`
}

// HandCardInfo represents a card in a player's hand/deck for the draw phase.
type HandCardInfo struct {
	CardID    int                       `json:"card_id"`
	CardName  string                    `json:"card_name"`
	Colour    string                    `json:"colour"`
	Rarity    string                    `json:"rarity"`
	ManaCost  int                       `json:"mana_cost"`
	Attack    int                       `json:"attack"`
	HP        int                       `json:"hp"`
	Abilities []effects.AbilityDefinition `json:"abilities,omitempty"`
}

type GameplayManager interface {
	GetElixir(playerID int64) int
	RemoveElixir(playerID int64, amount int)
	GetPlayer1ID() int64
	GetBoard(playerID int64) (yours *[2][3]*effects.CardInstance, opponents *[2][3]*effects.CardInstance)
	GetPlayerHealth(playerID int64) (you *int, opponent *int)
	PlaceCard(playerID int64, card *effects.CardInstance, row int, col int) error

	// Draw pile & hand
	GetDrawPile(playerID int64) []HandCardInfo
	GetHandCards(playerID int64) []HandCardInfo
	SelectCard(playerID int64, cardID int) error
	DeselectCard(playerID int64, cardID int) error
	PlayFromHand(playerID int64, cardID int) (*HandCardInfo, error)

	// Effects support
	GetCardStore() *effects.CardDefinitionStore
	GetRNG() *rand.Rand
	GetElixirCap(playerID int64) *int
	ReturnToHand(playerID int64, def *effects.CardDefinition)
	IsPlayer1(playerID int64) bool
	FireSummonEffects(playerID int64, card *effects.CardInstance, row, col int)
}
