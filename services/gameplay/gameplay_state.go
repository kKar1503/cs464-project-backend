package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/kKar1503/cs464-backend/services/gameplay/handlers"
)

// HandCard represents a card in a player's deck/hand with colour info for draw validation.
type HandCard struct {
	CardID   int    `json:"card_id"`
	CardName string `json:"card_name"`
	Colour   string `json:"colour"`
	Rarity   string `json:"rarity"`
	ManaCost int    `json:"mana_cost"`
	Attack   int    `json:"attack"`
	HP       int    `json:"hp"`
}

// PlayerHand tracks the draw state for a single player.
type PlayerHand struct {
	Deck          []HandCard `json:"deck"`           // full deck (12 cards, set at game start)
	Remaining     []HandCard `json:"remaining"`      // cards not yet drawn
	Offered       []HandCard `json:"offered"`         // 5 cards offered this pre-turn (subset of remaining)
	Hand          []HandCard `json:"hand"`            // cards the player has picked (persists across turns)
	DrawnCardIDs  map[int]bool `json:"-"`             // card IDs already drawn in previous turns
}

const (
	MilliElixirPerElixir = 1000                              // 1 elixir = 1000 milliElixir
	ElixirChargeSeconds  = 5                                 // 1 elixir charges every 5 seconds
	MilliElixirPerTick   = MilliElixirPerElixir / (TickRate * ElixirChargeSeconds) // 50 per tick at 4 ticks/sec
	MaxElixir            = 8                                 // max elixir a player can hold
	MaxMilliElixir       = MaxElixir * MilliElixirPerElixir  // 8000
	StartingElixir       = 3                                 // elixir at round 1
)

type GameplayState struct {
	SessionID          string
	Player1Health      int
	Player2Health      int
	MilliElixirPlayer1 int // stored as milliElixir (1000 = 1 elixir)
	MilliElixirPlayer2 int
	BoardPlayer1       *[2][3]handlers.Card
	BoardPlayer2       *[2][3]handlers.Card
	RoundNumber        int
	ElixirCap          int // current max elixir for this round (3 at round 1, +1 per round, max 8)
	Player1Hand        *PlayerHand
	Player2Hand        *PlayerHand
}

// GameplayManager manages gameplay state. All methods are called from the
// game loop goroutine only — no mutexes needed.
type GameplayManager struct {
	game      *GameplayState
	player1ID int64
	player2ID int64
}

func NewGameplayManager(sessionID string, player1ID int64, player2ID int64) *GameplayManager {
	var boardPlayer1 [2][3]handlers.Card
	var boardPlayer2 [2][3]handlers.Card
	return &GameplayManager{
		game: &GameplayState{
			SessionID:          sessionID,
			Player1Health:      250,
			Player2Health:      250,
			MilliElixirPlayer1: StartingElixir * MilliElixirPerElixir,
			MilliElixirPlayer2: StartingElixir * MilliElixirPerElixir,
			BoardPlayer1:       &boardPlayer1,
			BoardPlayer2:       &boardPlayer2,
			RoundNumber:        1,
			ElixirCap:          StartingElixir, // round 1 cap = 3
			Player1Hand:        &PlayerHand{DrawnCardIDs: make(map[int]bool)},
			Player2Hand:        &PlayerHand{DrawnCardIDs: make(map[int]bool)},
		},
		player1ID: player1ID,
		player2ID: player2ID,
	}
}

// SetPlayerDeck sets the deck for a player (called at game start after loading from DB).
func (gh *GameplayManager) SetPlayerDeck(playerID int64, deck []HandCard) {
	hand := gh.getPlayerHand(playerID)
	hand.Deck = deck
	hand.Remaining = make([]HandCard, len(deck))
	copy(hand.Remaining, deck)
}

// OfferCards picks 5 random cards from the player's remaining pool for the pre-turn phase.
func (gh *GameplayManager) OfferCards(playerID int64) []HandCard {
	hand := gh.getPlayerHand(playerID)

	// Filter out already-drawn cards from remaining
	var available []HandCard
	for _, c := range hand.Remaining {
		if !hand.DrawnCardIDs[c.CardID] {
			available = append(available, c)
		}
	}

	// Shuffle and pick up to 5
	shuffled := make([]HandCard, len(available))
	copy(shuffled, available)
	for i := len(shuffled) - 1; i > 0; i-- {
		j := int(time.Now().UnixNano()) % (i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	offerCount := 5
	if len(shuffled) < offerCount {
		offerCount = len(shuffled)
	}
	hand.Offered = shuffled[:offerCount]
	return hand.Offered
}

// SelectCards validates and adds selected cards to the player's hand.
// Rules:
//   - up to 4 cards from the offered set
//   - all selected cards must be the same colour (colourless doesn't count as a colour)
//   - cannot re-take cards already in hand from previous turns
func (gh *GameplayManager) SelectCards(playerID int64, selectedIDs []int) error {
	hand := gh.getPlayerHand(playerID)

	if len(selectedIDs) > 4 {
		return fmt.Errorf("can only select up to 4 cards")
	}
	if len(selectedIDs) == 0 {
		return nil // selecting nothing is valid
	}

	// Build a lookup of offered cards
	offeredMap := make(map[int]HandCard)
	for _, c := range hand.Offered {
		offeredMap[c.CardID] = c
	}

	// Validate all selected cards are in the offered set and not already drawn
	var selected []HandCard
	for _, id := range selectedIDs {
		card, ok := offeredMap[id]
		if !ok {
			return fmt.Errorf("card %d was not offered", id)
		}
		if hand.DrawnCardIDs[id] {
			return fmt.Errorf("card %d was already drawn in a previous turn", id)
		}
		selected = append(selected, card)
	}

	// Validate colour constraint: all non-colourless cards must be the same colour
	var requiredColour string
	for _, c := range selected {
		if c.Colour == "Grey" || c.Colour == "Colourless" {
			continue // colourless doesn't count
		}
		if requiredColour == "" {
			requiredColour = c.Colour
		} else if c.Colour != requiredColour {
			return fmt.Errorf("can only select 1 colour type per turn (got %s and %s)", requiredColour, c.Colour)
		}
	}

	// Add to hand and mark as drawn
	for _, c := range selected {
		hand.Hand = append(hand.Hand, c)
		hand.DrawnCardIDs[c.CardID] = true
	}

	// Clear offered
	hand.Offered = nil
	return nil
}

// GetPlayerHand returns the hand state for a player.
func (gh *GameplayManager) GetPlayerHandState(playerID int64) *PlayerHand {
	return gh.getPlayerHand(playerID)
}

func (gh *GameplayManager) getPlayerHand(playerID int64) *PlayerHand {
	if playerID == gh.player1ID {
		return gh.game.Player1Hand
	}
	return gh.game.Player2Hand
}

// TickElixir increments milliElixir for both players each tick.
// Caps at the current round's elixir cap (not the absolute max).
// Returns true if either player's elixir changed (for dirty flag).
func (gh *GameplayManager) TickElixir() bool {
	capMilli := gh.game.ElixirCap * MilliElixirPerElixir

	p1Before := gh.game.MilliElixirPlayer1
	p2Before := gh.game.MilliElixirPlayer2

	gh.game.MilliElixirPlayer1 = min(capMilli, gh.game.MilliElixirPlayer1+MilliElixirPerTick)
	gh.game.MilliElixirPlayer2 = min(capMilli, gh.game.MilliElixirPlayer2+MilliElixirPerTick)

	return gh.game.MilliElixirPlayer1 != p1Before || gh.game.MilliElixirPlayer2 != p2Before
}

// AdvanceRound increments the round number and raises the elixir cap by 1 (max 8).
func (gh *GameplayManager) AdvanceRound() {
	gh.game.RoundNumber++
	if gh.game.ElixirCap < MaxElixir {
		gh.game.ElixirCap++
	}
}

// GetElixirDisplay returns the whole elixir value for a player (for display/spending).
func (gh *GameplayManager) GetElixirDisplay(playerID int64) int {
	if playerID == gh.player1ID {
		return gh.game.MilliElixirPlayer1 / MilliElixirPerElixir
	}
	return gh.game.MilliElixirPlayer2 / MilliElixirPerElixir
}

// GetMilliElixir returns the raw milliElixir value for a player (for client-side smooth display).
func (gh *GameplayManager) GetMilliElixir(playerID int64) int {
	if playerID == gh.player1ID {
		return gh.game.MilliElixirPlayer1
	}
	return gh.game.MilliElixirPlayer2
}

// CheckWinCondition returns whether the game is over and which player won (1 or 2).
func (gh *GameplayManager) CheckWinCondition() (gameOver bool, winnerID int) {
	if gh.game.Player1Health <= 0 {
		return true, 2
	}
	if gh.game.Player2Health <= 0 {
		return true, 1
	}
	return false, 0
}

func (gh *GameplayManager) PlayCard(playerID int64, card *handlers.Card, xPos int, yPos int) error {
	isPlayer1 := playerID == gh.player1ID
	costInMilli := card.ElixerCost * MilliElixirPerElixir

	if isPlayer1 {
		if gh.game.MilliElixirPlayer1 < costInMilli {
			return fmt.Errorf("not enough elixir: have %d, need %d", gh.game.MilliElixirPlayer1/MilliElixirPerElixir, card.ElixerCost)
		}
		if err := placeCard(xPos, yPos, gh.game.BoardPlayer1, card); err != nil {
			return err
		}
		gh.game.MilliElixirPlayer1 -= costInMilli
	} else {
		if gh.game.MilliElixirPlayer2 < costInMilli {
			return fmt.Errorf("not enough elixir: have %d, need %d", gh.game.MilliElixirPlayer2/MilliElixirPerElixir, card.ElixerCost)
		}
		if err := placeCard(xPos, yPos, gh.game.BoardPlayer2, card); err != nil {
			return err
		}
		gh.game.MilliElixirPlayer2 -= costInMilli
	}

	return nil
}

func (gh *GameplayManager) AttackCard(playerID int64, attackX int, attackY int) error {
	isPlayer1 := playerID == gh.player1ID
	if isPlayer1 {
		return attackBoard(attackX, attackY, gh.game.BoardPlayer1, gh.game.BoardPlayer2, &gh.game.Player2Health)
	}
	return attackBoard(attackX, attackY, gh.game.BoardPlayer2, gh.game.BoardPlayer1, &gh.game.Player1Health)
}

// attackBoard resolves an attack from one board position against the opposing board.
func attackBoard(attackX int, attackY int, attackingPlayer *[2][3]handlers.Card, defendingPlayer *[2][3]handlers.Card, playerHealth *int) error {
	if attackingPlayer[attackX][attackY].LastMessage.Sub(time.Now()) < time.Duration(attackingPlayer[attackX][attackY].TimeToAttack)*time.Second {
		return errors.New("Attack Message sent too early")
	}

	if (*defendingPlayer)[0][attackY].CardID == 0 && (*defendingPlayer)[1][attackY].CardID == 0 {
		*playerHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
	} else if (*defendingPlayer)[0][attackY].CardID == 0 {
		(*defendingPlayer)[1][attackY].CurrentHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		if (*defendingPlayer)[1][attackY].CurrentHealth <= 0 {
			(*defendingPlayer)[1][attackY].CardID = 0
		}
	} else {
		(*defendingPlayer)[0][attackY].CurrentHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		if (*defendingPlayer)[0][attackY].CurrentHealth <= 0 {
			(*defendingPlayer)[0][attackY].CardID = 0
		}
	}

	attackingPlayer[attackX][attackY].LastMessage = time.Now()
	return nil
}

func placeCard(xPos int, yPos int, board *[2][3]handlers.Card, card *handlers.Card) error {
	if board[xPos][yPos].CardID != 0 {
		return fmt.Errorf("Card already exists")
	}
	board[xPos][yPos] = *card
	return nil
}
