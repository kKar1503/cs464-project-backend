package main

import (
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
	Deck         []HandCard   `json:"deck"`
	Remaining    []HandCard   `json:"remaining"`
	Offered      []HandCard   `json:"offered"`
	Hand         []HandCard   `json:"hand"`
	DrawnCardIDs map[int]bool `json:"-"`
}

const (
	MilliElixirPerElixir = 1000
	ElixirChargeSeconds  = 5
	MilliElixirPerTick   = MilliElixirPerElixir / (TickRate * ElixirChargeSeconds) // 50
	MaxElixir            = 8
	MaxMilliElixir       = MaxElixir * MilliElixirPerElixir
	StartingElixir       = 3
	LeaderAttack = 10 // leader counterattack damage
)

// AttackEvent records a single attack resolution for broadcast to clients.
type AttackEvent struct {
	AttackerCardID int  `json:"attacker_card_id"`
	AttackerRow    int  `json:"attacker_row"`
	AttackerCol    int  `json:"attacker_col"`
	TargetCardID   int  `json:"target_card_id"`
	TargetRow      int  `json:"target_row"`
	TargetCol      int  `json:"target_col"`
	Damage         int  `json:"damage"`
	CounterDamage  int  `json:"counter_damage"`
	TargetIsLeader bool `json:"target_is_leader"`
}

type GameplayState struct {
	SessionID          string
	Player1Health      int
	Player2Health      int
	Player1LeaderAtk   int
	Player2LeaderAtk   int
	MilliElixirPlayer1 int
	MilliElixirPlayer2 int
	BoardPlayer1       *[2][3]handlers.Card
	BoardPlayer2       *[2][3]handlers.Card
	RoundNumber        int
	ElixirCap          int
	Player1Hand        *PlayerHand
	Player2Hand        *PlayerHand
	Player1DrewThisRound bool
	Player2DrewThisRound bool

	LastAttackLog []AttackEvent // attacks resolved in the most recent tick
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
			Player1LeaderAtk:   LeaderAttack,
			Player2LeaderAtk:   LeaderAttack,
			MilliElixirPlayer1: StartingElixir * MilliElixirPerElixir,
			MilliElixirPlayer2: StartingElixir * MilliElixirPerElixir,
			BoardPlayer1:       &boardPlayer1,
			BoardPlayer2:       &boardPlayer2,
			RoundNumber:        1,
			ElixirCap:          StartingElixir,
			Player1Hand:        &PlayerHand{DrawnCardIDs: make(map[int]bool)},
			Player2Hand:        &PlayerHand{DrawnCardIDs: make(map[int]bool)},
		},
		player1ID: player1ID,
		player2ID: player2ID,
	}
}

// ──────────────────────────────────────────────
// Board / Combat
// ──────────────────────────────────────────────

// TickBoard processes one tick of board state: charge cards, resolve attacks instantly when ready.
// Returns true if any state changed.
func (gh *GameplayManager) TickBoard() bool {
	changed := false
	g := gh.game
	g.LastAttackLog = nil

	// Tick charge timers and resolve attacks instantly when charged
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if g.BoardPlayer1[r][c].CardID != 0 && g.BoardPlayer1[r][c].IsCharging {
				g.BoardPlayer1[r][c].ChargeTicksRemaining--
				changed = true
				if g.BoardPlayer1[r][c].ChargeTicksRemaining <= 0 {
					if event := gh.resolveAttack(true, r, c); event != nil {
						g.LastAttackLog = append(g.LastAttackLog, *event)
					}
				}
			}
			if g.BoardPlayer2[r][c].CardID != 0 && g.BoardPlayer2[r][c].IsCharging {
				g.BoardPlayer2[r][c].ChargeTicksRemaining--
				changed = true
				if g.BoardPlayer2[r][c].ChargeTicksRemaining <= 0 {
					if event := gh.resolveAttack(false, r, c); event != nil {
						g.LastAttackLog = append(g.LastAttackLog, *event)
					}
				}
			}
		}
	}

	// Clean up dead cards
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if g.BoardPlayer1[r][c].CardID != 0 && g.BoardPlayer1[r][c].CurrentHealth <= 0 {
				g.BoardPlayer1[r][c] = handlers.Card{}
				changed = true
			}
			if g.BoardPlayer2[r][c].CardID != 0 && g.BoardPlayer2[r][c].CurrentHealth <= 0 {
				g.BoardPlayer2[r][c] = handlers.Card{}
				changed = true
			}
		}
	}

	return changed
}

// resolveAttack resolves a single attack instantly when a card's charge completes.
func (gh *GameplayManager) resolveAttack(isPlayer1 bool, row, col int) *AttackEvent {
	g := gh.game

	var attackerBoard, defenderBoard *[2][3]handlers.Card
	var defenderHealth *int
	var defenderLeaderAtk int

	if isPlayer1 {
		attackerBoard = g.BoardPlayer1
		defenderBoard = g.BoardPlayer2
		defenderHealth = &g.Player2Health
		defenderLeaderAtk = g.Player2LeaderAtk
	} else {
		attackerBoard = g.BoardPlayer2
		defenderBoard = g.BoardPlayer1
		defenderHealth = &g.Player1Health
		defenderLeaderAtk = g.Player1LeaderAtk
	}

	attacker := &attackerBoard[row][col]
	if attacker.CardID == 0 {
		return nil
	}

	damage := attacker.CardAttack

	event := &AttackEvent{
		AttackerCardID: attacker.CardID,
		AttackerRow:    row,
		AttackerCol:    col,
		Damage:         damage,
	}

	// Find target: front row first, then back row, then leader
	if defenderBoard[0][col].CardID != 0 {
		target := &defenderBoard[0][col]
		target.CurrentHealth -= damage
		event.TargetCardID = target.CardID
		event.TargetRow = 0
		event.TargetCol = col
	} else if defenderBoard[1][col].CardID != 0 {
		target := &defenderBoard[1][col]
		target.CurrentHealth -= damage
		event.TargetCardID = target.CardID
		event.TargetRow = 1
		event.TargetCol = col
	} else {
		*defenderHealth -= damage
		event.TargetIsLeader = true
		attacker.CurrentHealth -= defenderLeaderAtk
		event.CounterDamage = defenderLeaderAtk
	}

	// Reset charge timer immediately — next attack cycle starts now
	attacker.ChargeTicksRemaining = handlers.ChargeTicksTotal
	attacker.IsCharging = true

	return event
}

// ──────────────────────────────────────────────
// Card placement
// ──────────────────────────────────────────────

func (gh *GameplayManager) PlayCard(playerID int64, card *handlers.Card, row int, col int) error {
	isPlayer1 := playerID == gh.player1ID
	costInMilli := card.ElixerCost * MilliElixirPerElixir

	if isPlayer1 {
		if gh.game.MilliElixirPlayer1 < costInMilli {
			return fmt.Errorf("not enough elixir: have %d, need %d", gh.game.MilliElixirPlayer1/MilliElixirPerElixir, card.ElixerCost)
		}
		if gh.game.BoardPlayer1[row][col].CardID != 0 {
			return fmt.Errorf("board position [%d][%d] is occupied", row, col)
		}
		gh.game.BoardPlayer1[row][col] = *card
		gh.game.MilliElixirPlayer1 -= costInMilli
	} else {
		if gh.game.MilliElixirPlayer2 < costInMilli {
			return fmt.Errorf("not enough elixir: have %d, need %d", gh.game.MilliElixirPlayer2/MilliElixirPerElixir, card.ElixerCost)
		}
		if gh.game.BoardPlayer2[row][col].CardID != 0 {
			return fmt.Errorf("board position [%d][%d] is occupied", row, col)
		}
		gh.game.BoardPlayer2[row][col] = *card
		gh.game.MilliElixirPlayer2 -= costInMilli
	}

	return nil
}

// ──────────────────────────────────────────────
// Hand / Draw
// ──────────────────────────────────────────────

func (gh *GameplayManager) SetPlayerDeck(playerID int64, deck []HandCard) {
	hand := gh.getPlayerHand(playerID)
	hand.Deck = deck
	hand.Remaining = make([]HandCard, len(deck))
	copy(hand.Remaining, deck)
}

func (gh *GameplayManager) OfferCards(playerID int64) []HandCard {
	hand := gh.getPlayerHand(playerID)

	var available []HandCard
	for _, c := range hand.Remaining {
		if !hand.DrawnCardIDs[c.CardID] {
			available = append(available, c)
		}
	}

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

func (gh *GameplayManager) SelectCards(playerID int64, selectedIDs []int) error {
	hand := gh.getPlayerHand(playerID)

	if len(selectedIDs) > 4 {
		return fmt.Errorf("can only select up to 4 cards")
	}
	if len(selectedIDs) == 0 {
		return nil
	}

	offeredMap := make(map[int]HandCard)
	for _, c := range hand.Offered {
		offeredMap[c.CardID] = c
	}

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

	var requiredColour string
	for _, c := range selected {
		if c.Colour == "Grey" || c.Colour == "Colourless" {
			continue
		}
		if requiredColour == "" {
			requiredColour = c.Colour
		} else if c.Colour != requiredColour {
			return fmt.Errorf("can only select 1 colour type per turn (got %s and %s)", requiredColour, c.Colour)
		}
	}

	for _, c := range selected {
		hand.Hand = append(hand.Hand, c)
		hand.DrawnCardIDs[c.CardID] = true
	}

	hand.Offered = nil
	return nil
}

// MarkPlayerDrew marks a player as having completed their draw for this round.
// Returns true if both players have now drawn (ready to start active phase).
func (gh *GameplayManager) MarkPlayerDrew(playerID int64) bool {
	if playerID == gh.player1ID {
		gh.game.Player1DrewThisRound = true
	} else {
		gh.game.Player2DrewThisRound = true
	}
	return gh.game.Player1DrewThisRound && gh.game.Player2DrewThisRound
}

// ResetDrawState resets draw flags for a new round.
func (gh *GameplayManager) ResetDrawState() {
	gh.game.Player1DrewThisRound = false
	gh.game.Player2DrewThisRound = false
}

// RemoveFromHand removes the first instance of a card from the player's hand.
func (gh *GameplayManager) RemoveFromHand(playerID int64, cardID int) error {
	hand := gh.getPlayerHand(playerID)
	for i, c := range hand.Hand {
		if c.CardID == cardID {
			hand.Hand = append(hand.Hand[:i], hand.Hand[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("card %d not found in hand", cardID)
}

func (gh *GameplayManager) GetPlayerHandState(playerID int64) *PlayerHand {
	return gh.getPlayerHand(playerID)
}

func (gh *GameplayManager) getPlayerHand(playerID int64) *PlayerHand {
	if playerID == gh.player1ID {
		return gh.game.Player1Hand
	}
	return gh.game.Player2Hand
}

// ──────────────────────────────────────────────
// Elixir
// ──────────────────────────────────────────────

func (gh *GameplayManager) TickElixir() bool {
	capMilli := gh.game.ElixirCap * MilliElixirPerElixir
	p1Before := gh.game.MilliElixirPlayer1
	p2Before := gh.game.MilliElixirPlayer2
	gh.game.MilliElixirPlayer1 = min(capMilli, gh.game.MilliElixirPlayer1+MilliElixirPerTick)
	gh.game.MilliElixirPlayer2 = min(capMilli, gh.game.MilliElixirPlayer2+MilliElixirPerTick)
	return gh.game.MilliElixirPlayer1 != p1Before || gh.game.MilliElixirPlayer2 != p2Before
}

func (gh *GameplayManager) AdvanceRound() {
	gh.game.RoundNumber++
	if gh.game.ElixirCap < MaxElixir {
		gh.game.ElixirCap++
	}
}

func (gh *GameplayManager) GetElixirDisplay(playerID int64) int {
	if playerID == gh.player1ID {
		return gh.game.MilliElixirPlayer1 / MilliElixirPerElixir
	}
	return gh.game.MilliElixirPlayer2 / MilliElixirPerElixir
}

func (gh *GameplayManager) GetMilliElixir(playerID int64) int {
	if playerID == gh.player1ID {
		return gh.game.MilliElixirPlayer1
	}
	return gh.game.MilliElixirPlayer2
}

func (gh *GameplayManager) CheckWinCondition() (gameOver bool, winnerID int) {
	if gh.game.Player1Health <= 0 {
		return true, 2
	}
	if gh.game.Player2Health <= 0 {
		return true, 1
	}
	return false, 0
}
