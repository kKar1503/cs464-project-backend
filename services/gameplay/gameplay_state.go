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

const (
	MaxDrawPile   = 8 // max cards in the draw pile
	DrawPileTopUp = 5 // cards added from deck to draw pile each pre-turn
	MaxHand       = 4 // max cards in hand
)

// PlayerHand tracks the three card zones for a single player.
// Flow: Deck → DrawPile → Hand → Board → back to Deck
type PlayerHand struct {
	Deck     []HandCard `json:"deck"`      // the queue — cards cycle back here after being played
	DrawPile []HandCard `json:"draw_pile"` // cards offered each pre-turn (max 8)
	Hand     []HandCard `json:"hand"`      // cards selected from draw pile, ready to play (max 4)
}

const (
	MilliElixirPerElixir = 1000
	ElixirChargeSeconds  = 5
	MilliElixirPerTick   = MilliElixirPerElixir / (TickRate * ElixirChargeSeconds) // 50
	MaxElixir            = 8
	MaxMilliElixir       = MaxElixir * MilliElixirPerElixir
	StartingElixir       = 3  // elixir amount at round 1
	StartingElixirCap    = 5  // elixir cap at round 1
	LeaderAttack = 10 // leader counterattack damage
)

// CombatEventType represents the kind of combat event.
type CombatEventType string

const (
	CombatEventAttack        CombatEventType = "attack"
	CombatEventCounterAttack CombatEventType = "counter_attack"
	CombatEventSummonEffect  CombatEventType = "summon_effect"
	CombatEventOnAttack      CombatEventType = "on_attack"
	CombatEventOnDamaged     CombatEventType = "on_damaged"
	CombatEventOnDeath       CombatEventType = "on_death"
	CombatEventBuff          CombatEventType = "buff"
	CombatEventDebuff        CombatEventType = "debuff"
	CombatEventHeal          CombatEventType = "heal"
	CombatEventCardDeath     CombatEventType = "card_death"
	CombatEventTransform     CombatEventType = "transform"
	CombatEventBounce        CombatEventType = "bounce"
	CombatEventSummon        CombatEventType = "summon"
)

// CombatEvent records a single combat-related event for broadcast to clients.
type CombatEvent struct {
	Type CombatEventType `json:"type"`

	// Who triggered this event
	SourcePlayerID int64 `json:"source_player_id,omitempty"` // user ID of the player who owns the source
	SourceCardID   int   `json:"source_card_id,omitempty"`
	SourceRow      int   `json:"source_row,omitempty"`
	SourceCol      int   `json:"source_col,omitempty"`

	// Who is affected
	TargetCardID   int  `json:"target_card_id,omitempty"` // 0 if target is leader
	TargetRow      int  `json:"target_row,omitempty"`
	TargetCol      int  `json:"target_col,omitempty"`
	TargetIsLeader bool `json:"target_is_leader,omitempty"`

	// Values
	Value    int    `json:"value,omitempty"`    // damage dealt, HP healed, buff amount, etc.
	ValueHP  int    `json:"value_hp,omitempty"` // for buffs that modify both atk and hp
	CardName string `json:"card_name,omitempty"` // for context (e.g. "Pig transformed into Technoblade")
	Message  string `json:"message,omitempty"`   // human-readable description
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
	Player1Hand *PlayerHand
	Player2Hand *PlayerHand

	CombatLog []CombatEvent // attacks resolved in the most recent tick
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
			ElixirCap:          StartingElixirCap,
			Player1Hand:        &PlayerHand{},
			Player2Hand:        &PlayerHand{},
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
	g.CombatLog = nil

	// Tick charge timers and resolve attacks instantly when charged
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if g.BoardPlayer1[r][c].CardID != 0 && g.BoardPlayer1[r][c].IsCharging {
				g.BoardPlayer1[r][c].ChargeTicksRemaining--
				changed = true
				if g.BoardPlayer1[r][c].ChargeTicksRemaining <= 0 {
					if event := gh.resolveAttack(true, r, c); event != nil {
						g.CombatLog = append(g.CombatLog, event...)
					}
				}
			}
			if g.BoardPlayer2[r][c].CardID != 0 && g.BoardPlayer2[r][c].IsCharging {
				g.BoardPlayer2[r][c].ChargeTicksRemaining--
				changed = true
				if g.BoardPlayer2[r][c].ChargeTicksRemaining <= 0 {
					if event := gh.resolveAttack(false, r, c); event != nil {
						g.CombatLog = append(g.CombatLog, event...)
					}
				}
			}
		}
	}

	// Clean up dead cards (cards already returned to deck when played)
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
func (gh *GameplayManager) resolveAttack(isPlayer1 bool, row, col int) []CombatEvent {
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

	var sourcePlayerID int64
	if isPlayer1 {
		sourcePlayerID = gh.player1ID
	} else {
		sourcePlayerID = gh.player2ID
	}

	var events []CombatEvent

	// Find target: front row first, then back row, then leader
	if defenderBoard[0][col].CardID != 0 {
		target := &defenderBoard[0][col]
		target.CurrentHealth -= damage
		events = append(events, CombatEvent{
			Type:           CombatEventAttack,
			SourcePlayerID: sourcePlayerID,
			SourceCardID:   attacker.CardID,
			SourceRow:      row,
			SourceCol:      col,
			TargetCardID:   target.CardID,
			TargetRow:      0,
			TargetCol:      col,
			Value:          damage,
		})
	} else if defenderBoard[1][col].CardID != 0 {
		target := &defenderBoard[1][col]
		target.CurrentHealth -= damage
		events = append(events, CombatEvent{
			Type:           CombatEventAttack,
			SourcePlayerID: sourcePlayerID,
			SourceCardID:   attacker.CardID,
			SourceRow:      row,
			SourceCol:      col,
			TargetCardID:   target.CardID,
			TargetRow:      1,
			TargetCol:      col,
			Value:          damage,
		})
	} else {
		*defenderHealth -= damage
		events = append(events, CombatEvent{
			Type:           CombatEventAttack,
			SourcePlayerID: sourcePlayerID,
			SourceCardID:   attacker.CardID,
			SourceRow:      row,
			SourceCol:      col,
			TargetIsLeader: true,
			Value:          damage,
		})
		// Leader counterattack
		attacker.CurrentHealth -= defenderLeaderAtk
		events = append(events, CombatEvent{
			Type:           CombatEventCounterAttack,
			TargetCardID:   attacker.CardID,
			TargetRow:      row,
			TargetCol:      col,
			TargetIsLeader: false,
			Value:          defenderLeaderAtk,
			Message:        "Leader counterattack",
		})
	}

	// Reset charge timer immediately — next attack cycle starts now
	attacker.ChargeTicksRemaining = handlers.ChargeTicksTotal
	attacker.IsCharging = true

	return events
}

// ──────────────────────────────────────────────
// Card placement
// ──────────────────────────────────────────────

func (gh *GameplayManager) PlayCard(playerID int64, card *handlers.Card, row int, col int) error {
	isPlayer1 := playerID == gh.player1ID
	costInMilli := card.ElixirCost * MilliElixirPerElixir

	if isPlayer1 {
		if gh.game.MilliElixirPlayer1 < costInMilli {
			return fmt.Errorf("not enough elixir: have %d, need %d", gh.game.MilliElixirPlayer1/MilliElixirPerElixir, card.ElixirCost)
		}
		if gh.game.BoardPlayer1[row][col].CardID != 0 {
			return fmt.Errorf("board position [%d][%d] is occupied", row, col)
		}
		gh.game.BoardPlayer1[row][col] = *card
		gh.game.MilliElixirPlayer1 -= costInMilli
	} else {
		if gh.game.MilliElixirPlayer2 < costInMilli {
			return fmt.Errorf("not enough elixir: have %d, need %d", gh.game.MilliElixirPlayer2/MilliElixirPerElixir, card.ElixirCost)
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

// SetPlayerDeck sets and shuffles the deck for a player (called at game start).
func (gh *GameplayManager) SetPlayerDeck(playerID int64, deck []HandCard) {
	hand := gh.getPlayerHand(playerID)

	// Shuffle the deck
	shuffled := make([]HandCard, len(deck))
	copy(shuffled, deck)
	for i := len(shuffled) - 1; i > 0; i-- {
		j := int(time.Now().UnixNano()) % (i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	hand.Deck = shuffled
}

// TopUpDrawPile moves up to DrawPileTopUp (5) cards from the front of the deck
// to the draw pile, as long as the draw pile hasn't reached MaxDrawPile (8).
// Called at the start of each pre-turn phase.
func (gh *GameplayManager) TopUpDrawPile(playerID int64) {
	hand := gh.getPlayerHand(playerID)

	space := MaxDrawPile - len(hand.DrawPile)
	if space <= 0 {
		return // draw pile is full
	}

	toAdd := DrawPileTopUp
	if toAdd > space {
		toAdd = space
	}
	if toAdd > len(hand.Deck) {
		toAdd = len(hand.Deck)
	}

	hand.DrawPile = append(hand.DrawPile, hand.Deck[:toAdd]...)
	hand.Deck = hand.Deck[toAdd:]
}

// SelectCard moves a single card from draw pile to hand.
func (gh *GameplayManager) SelectCard(playerID int64, cardID int) error {
	hand := gh.getPlayerHand(playerID)

	if len(hand.Hand) >= MaxHand {
		return fmt.Errorf("hand is full (%d/%d)", len(hand.Hand), MaxHand)
	}

	for i, c := range hand.DrawPile {
		if c.CardID == cardID {
			hand.Hand = append(hand.Hand, c)
			hand.DrawPile = append(hand.DrawPile[:i], hand.DrawPile[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("card %d not in draw pile", cardID)
}

// DeselectCard moves a single card from hand back to draw pile.
func (gh *GameplayManager) DeselectCard(playerID int64, cardID int) error {
	hand := gh.getPlayerHand(playerID)

	for i, c := range hand.Hand {
		if c.CardID == cardID {
			hand.DrawPile = append(hand.DrawPile, c)
			hand.Hand = append(hand.Hand[:i], hand.Hand[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("card %d not in hand", cardID)
}

// PlayFromHand removes a card from the hand (when played onto the board).
// The card goes to the board AND immediately back to the deck.
func (gh *GameplayManager) PlayFromHand(playerID int64, cardID int) (*HandCard, error) {
	hand := gh.getPlayerHand(playerID)
	for i, c := range hand.Hand {
		if c.CardID == cardID {
			card := hand.Hand[i]
			hand.Hand = append(hand.Hand[:i], hand.Hand[i+1:]...)
			// Immediately return to back of deck
			hand.Deck = append(hand.Deck, card)
			return &card, nil
		}
	}
	return nil, fmt.Errorf("card %d not in hand", cardID)
}

// GetDrawPile returns the current draw pile for a player.
func (gh *GameplayManager) GetDrawPile(playerID int64) []HandCard {
	return gh.getPlayerHand(playerID).DrawPile
}

// GetHand returns the current hand for a player.
func (gh *GameplayManager) GetHand(playerID int64) []HandCard {
	return gh.getPlayerHand(playerID).Hand
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
