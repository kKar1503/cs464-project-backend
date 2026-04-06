package main

import (
	"fmt"
	"hash/fnv"
	"math/rand"

	"github.com/kKar1503/cs464-backend/services/gameplay/effects"
)

// HandCard represents a card in a player's deck/hand with ability info.
type HandCard struct {
	CardID    int                       `json:"card_id"`
	CardName  string                    `json:"card_name"`
	Colour    string                    `json:"colour"`
	Rarity    string                    `json:"rarity"`
	ManaCost  int                       `json:"mana_cost"`
	Attack    int                       `json:"attack"`
	HP        int                       `json:"hp"`
	Abilities []effects.AbilityDefinition `json:"abilities,omitempty"`
}

const (
	MaxDrawPile   = 8 // max cards in the draw pile
	DrawPileTopUp = 5 // cards added from deck to draw pile each pre-turn
	MaxHand       = 4 // max cards in hand
)

// PlayerHand tracks the three card zones for a single player.
type PlayerHand struct {
	Deck             []HandCard  `json:"deck"`
	DrawPile         []HandCard  `json:"draw_pile"`
	Hand             []HandCard  `json:"hand"`
	SelectedThisTurn map[int]int `json:"-"`
}

const (
	MilliElixirPerElixir = 1000
	ElixirChargeSeconds  = 5
	MilliElixirPerTick   = MilliElixirPerElixir / (TickRate * ElixirChargeSeconds) // 50
	MaxElixir            = 8
	MaxMilliElixir       = MaxElixir * MilliElixirPerElixir
	StartingElixir       = 3
	StartingElixirCap    = 5
	LeaderAttack         = 10
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

	SourcePlayerID int64 `json:"source_player_id,omitempty"`
	SourceCardID   int   `json:"source_card_id,omitempty"`
	SourceRow      int   `json:"source_row,omitempty"`
	SourceCol      int   `json:"source_col,omitempty"`

	TargetCardID   int  `json:"target_card_id,omitempty"`
	TargetRow      int  `json:"target_row,omitempty"`
	TargetCol      int  `json:"target_col,omitempty"`
	TargetIsLeader bool `json:"target_is_leader,omitempty"`

	Value    int    `json:"value,omitempty"`
	ValueHP  int    `json:"value_hp,omitempty"`
	CardName string `json:"card_name,omitempty"`
	Message  string `json:"message,omitempty"`
}

type GameplayState struct {
	SessionID          string
	Player1Health      int
	Player2Health      int
	Player1LeaderAtk   int
	Player2LeaderAtk   int
	MilliElixirPlayer1 int
	MilliElixirPlayer2 int
	BoardPlayer1       [2][3]*effects.CardInstance
	BoardPlayer2       [2][3]*effects.CardInstance
	RoundNumber        int
	ElixirCap          int
	Player1Hand        *PlayerHand
	Player2Hand        *PlayerHand
	CombatLog          []CombatEvent
}

// GameplayManager manages gameplay state.
type GameplayManager struct {
	game      *GameplayState
	player1ID int64
	player2ID int64
	CardStore *effects.CardDefinitionStore
	RNG       *rand.Rand
}

func NewGameplayManager(sessionID string, player1ID int64, player2ID int64) *GameplayManager {
	// Seed RNG deterministically from session ID
	h := fnv.New64a()
	h.Write([]byte(sessionID))
	seed := int64(h.Sum64())

	return &GameplayManager{
		game: &GameplayState{
			SessionID:          sessionID,
			Player1Health:      250,
			Player2Health:      250,
			Player1LeaderAtk:   LeaderAttack,
			Player2LeaderAtk:   LeaderAttack,
			MilliElixirPlayer1: StartingElixir * MilliElixirPerElixir,
			MilliElixirPlayer2: StartingElixir * MilliElixirPerElixir,
			RoundNumber:        1,
			ElixirCap:          StartingElixirCap,
			Player1Hand:        &PlayerHand{SelectedThisTurn: make(map[int]int)},
			Player2Hand:        &PlayerHand{SelectedThisTurn: make(map[int]int)},
		},
		player1ID: player1ID,
		player2ID: player2ID,
		RNG:       rand.New(rand.NewSource(seed)),
	}
}

// ──────────────────────────────────────────────
// Board / Combat
// ──────────────────────────────────────────────

// TickBoard processes one tick of board state.
func (gh *GameplayManager) TickBoard() bool {
	changed := false
	g := gh.game
	g.CombatLog = nil

	// Tick charge timers and resolve attacks
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			if card := g.BoardPlayer1[r][c]; card != nil && card.IsCharging && card.ChargeTicksTotal > 0 {
				card.ChargeTicksRemaining--
				changed = true
				if card.ChargeTicksRemaining <= 0 {
					if events := gh.resolveAttack(true, r, c); events != nil {
						g.CombatLog = append(g.CombatLog, events...)
					}
				}
			}
			if card := g.BoardPlayer2[r][c]; card != nil && card.IsCharging && card.ChargeTicksTotal > 0 {
				card.ChargeTicksRemaining--
				changed = true
				if card.ChargeTicksRemaining <= 0 {
					if events := gh.resolveAttack(false, r, c); events != nil {
						g.CombatLog = append(g.CombatLog, events...)
					}
				}
			}
		}
	}

	// Process deaths in waves (handles on_death cascades)
	if deathEvents := gh.processDeaths(); len(deathEvents) > 0 {
		g.CombatLog = append(g.CombatLog, deathEvents...)
		changed = true
	}

	return changed
}

// processDeaths handles death cascades: collect dead cards, fire on_death, remove, repeat.
func (gh *GameplayManager) processDeaths() []CombatEvent {
	var allEvents []CombatEvent
	g := gh.game

	for wave := 0; wave < 10; wave++ { // max 10 waves to prevent infinite loops
		var deadCards []struct {
			card      *effects.CardInstance
			board     *[2][3]*effects.CardInstance
			row, col  int
			isPlayer1 bool
		}

		// Collect dead cards from both boards
		for r := 0; r < 2; r++ {
			for c := 0; c < 3; c++ {
				if g.BoardPlayer1[r][c] != nil && g.BoardPlayer1[r][c].CurrentHP <= 0 {
					deadCards = append(deadCards, struct {
						card      *effects.CardInstance
						board     *[2][3]*effects.CardInstance
						row, col  int
						isPlayer1 bool
					}{g.BoardPlayer1[r][c], &g.BoardPlayer1, r, c, true})
				}
				if g.BoardPlayer2[r][c] != nil && g.BoardPlayer2[r][c].CurrentHP <= 0 {
					deadCards = append(deadCards, struct {
						card      *effects.CardInstance
						board     *[2][3]*effects.CardInstance
						row, col  int
						isPlayer1 bool
					}{g.BoardPlayer2[r][c], &g.BoardPlayer2, r, c, false})
				}
			}
		}

		if len(deadCards) == 0 {
			break
		}

		for _, dc := range deadCards {
			// Fire on_death trigger before removing
			ctx := gh.makeEffectContext(dc.isPlayer1, dc.row, dc.col, dc.card)
			deathEvents := effects.FireTrigger("on_death", dc.card, ctx)
			allEvents = append(allEvents, gh.convertEffectEvents(deathEvents)...)

			// Emit card_death event
			var sourcePlayerID int64
			if dc.isPlayer1 {
				sourcePlayerID = gh.player1ID
			} else {
				sourcePlayerID = gh.player2ID
			}
			allEvents = append(allEvents, CombatEvent{
				Type:         CombatEventCardDeath,
				SourcePlayerID: sourcePlayerID,
				SourceCardID: dc.card.Definition.CardID,
				SourceRow:    dc.row,
				SourceCol:    dc.col,
				CardName:     dc.card.Definition.Name,
			})

			// Remove from board
			dc.board[dc.row][dc.col] = nil
		}
	}

	return allEvents
}

// resolveAttack resolves a single attack when a card's charge completes.
func (gh *GameplayManager) resolveAttack(isPlayer1 bool, row, col int) []CombatEvent {
	g := gh.game

	var attackerBoard, defenderBoard *[2][3]*effects.CardInstance
	var defenderHealth *int
	var defenderLeaderAtk int
	var sourcePlayerID int64

	if isPlayer1 {
		attackerBoard = &g.BoardPlayer1
		defenderBoard = &g.BoardPlayer2
		defenderHealth = &g.Player2Health
		defenderLeaderAtk = g.Player2LeaderAtk
		sourcePlayerID = gh.player1ID
	} else {
		attackerBoard = &g.BoardPlayer2
		defenderBoard = &g.BoardPlayer1
		defenderHealth = &g.Player1Health
		defenderLeaderAtk = g.Player1LeaderAtk
		sourcePlayerID = gh.player2ID
	}

	attacker := attackerBoard[row][col]
	if attacker == nil {
		return nil
	}

	var events []CombatEvent

	// Check for TargetModifier abilities (random_target, skip_front_row)
	ctx := gh.makeEffectContext(isPlayer1, row, col, attacker)
	tm := effects.GetTargetModifier(attacker)

	// Fire on_attack trigger (Glass Bones self-damage, Cat Sith reset, Archangel heal adj)
	onAttackEvents := effects.FireTrigger("on_attack", attacker, ctx)
	events = append(events, gh.convertEffectEvents(onAttackEvents)...)

	// Check if attacker died from self-damage (Glass Bones)
	if attacker.CurrentHP <= 0 {
		attacker.ChargeTicksRemaining = attacker.ChargeTicksTotal
		return events
	}

	damage := attacker.CurrentAtk

	var targetCard *effects.CardInstance
	var targetRow, targetCol int
	targetIsLeader := false

	if tm != nil {
		// Use modified targeting
		override := tm.ModifyTarget(ctx)
		if override != nil {
			targetCard = override.TargetCard
			targetRow = override.TargetRow
			targetCol = override.TargetCol
			targetIsLeader = override.TargetIsLeader
		}
	}

	if tm == nil {
		// Default targeting: front row → back row → leader (same column)
		if defenderBoard[0][col] != nil {
			targetCard = defenderBoard[0][col]
			targetRow = 0
			targetCol = col
		} else if defenderBoard[1][col] != nil {
			targetCard = defenderBoard[1][col]
			targetRow = 1
			targetCol = col
		} else {
			targetIsLeader = true
		}
	}

	if targetIsLeader {
		*defenderHealth -= damage
		events = append(events, CombatEvent{
			Type:           CombatEventAttack,
			SourcePlayerID: sourcePlayerID,
			SourceCardID:   attacker.Definition.CardID,
			SourceRow:      row,
			SourceCol:      col,
			TargetIsLeader: true,
			Value:          damage,
		})
		// Leader counterattack
		attacker.CurrentHP -= defenderLeaderAtk
		events = append(events, CombatEvent{
			Type:         CombatEventCounterAttack,
			TargetCardID: attacker.Definition.CardID,
			TargetRow:    row,
			TargetCol:    col,
			Value:        defenderLeaderAtk,
			Message:      "Leader counterattack",
		})
	} else if targetCard != nil {
		// Apply shield (DamageReduction)
		actualDamage := damage
		if targetCard.DamageReduction > 0 {
			actualDamage -= targetCard.DamageReduction
			if actualDamage < 0 {
				actualDamage = 0
			}
		}
		targetCard.CurrentHP -= actualDamage

		events = append(events, CombatEvent{
			Type:           CombatEventAttack,
			SourcePlayerID: sourcePlayerID,
			SourceCardID:   attacker.Definition.CardID,
			SourceRow:      row,
			SourceCol:      col,
			TargetCardID:   targetCard.Definition.CardID,
			TargetRow:      targetRow,
			TargetCol:      targetCol,
			Value:          actualDamage,
		})

		// Fire on_damaged trigger on defender (reflect, shield, Apache buff)
		if targetCard.CurrentHP > 0 {
			defenderIsPlayer1 := !isPlayer1
			defenderCtx := gh.makeEffectContext(defenderIsPlayer1, targetRow, targetCol, targetCard)
			defenderCtx.Target = attacker // for reflect
			onDamagedEvents := effects.FireTrigger("on_damaged", targetCard, defenderCtx)
			events = append(events, gh.convertEffectEvents(onDamagedEvents)...)
		}
	}

	// Reset charge timer
	attacker.ChargeTicksRemaining = attacker.ChargeTicksTotal
	attacker.IsCharging = true

	return events
}

// makeEffectContext creates an EffectContext for a card.
func (gh *GameplayManager) makeEffectContext(isPlayer1 bool, row, col int, card *effects.CardInstance) *effects.EffectContext {
	var sourcePlayerID int64
	if isPlayer1 {
		sourcePlayerID = gh.player1ID
	} else {
		sourcePlayerID = gh.player2ID
	}

	// Collect deck colours for Travelling Merchant
	hand := gh.getPlayerHand(sourcePlayerID)
	var deckColours []string
	for _, c := range hand.Deck {
		deckColours = append(deckColours, c.Colour)
	}
	for _, c := range hand.DrawPile {
		deckColours = append(deckColours, c.Colour)
	}
	for _, c := range hand.Hand {
		deckColours = append(deckColours, c.Colour)
	}

	return &effects.EffectContext{
		Source:                  card,
		SourcePos:               effects.BoardPosition{Row: row, Col: col},
		SourcePlayerID:          sourcePlayerID,
		IsPlayer1:               isPlayer1,
		Board1:                  &gh.game.BoardPlayer1,
		Board2:                  &gh.game.BoardPlayer2,
		Player1HP:               &gh.game.Player1Health,
		Player2HP:               &gh.game.Player2Health,
		Player1LeaderAtk:        gh.game.Player1LeaderAtk,
		Player2LeaderAtk:        gh.game.Player2LeaderAtk,
		Player1ElixirCap:        &gh.game.ElixirCap,
		Player2ElixirCap:        &gh.game.ElixirCap,
		ReturnToHand:            gh.returnToHandCallback(),
		CardStore:               gh.CardStore,
		SourcePlayerDeckColours: deckColours,
		RNG:                     gh.RNG,
	}
}

func (gh *GameplayManager) returnToHandCallback() func(playerID int64, def *effects.CardDefinition) {
	return func(playerID int64, def *effects.CardDefinition) {
		gh.ReturnCardToHand(playerID, def)
	}
}

// ReturnCardToHand adds a card definition back to a player's hand (used by bounce).
func (gh *GameplayManager) ReturnCardToHand(playerID int64, def *effects.CardDefinition) {
	hand := gh.getPlayerHand(playerID)
	card := HandCard{
		CardID:   def.CardID,
		CardName: def.Name,
		Colour:   def.Colour,
		Rarity:   def.Rarity,
		ManaCost: def.Cost,
		Attack:   def.BaseAtk,
		HP:       def.BaseHP,
		Abilities: def.Abilities,
	}
	hand.Deck = append(hand.Deck, card)
}

// FireSummonEffects fires summon triggers for a newly placed card.
func (gh *GameplayManager) FireSummonEffects(playerID int64, card *effects.CardInstance, row, col int) {
	isPlayer1 := playerID == gh.player1ID
	ctx := gh.makeEffectContext(isPlayer1, row, col, card)
	events := effects.FireTrigger("summon", card, ctx)
	gh.game.CombatLog = append(gh.game.CombatLog, gh.convertEffectEvents(events)...)
}

// convertEffectEvents converts effects.EffectEvent to CombatEvent.
func (gh *GameplayManager) convertEffectEvents(events []effects.EffectEvent) []CombatEvent {
	result := make([]CombatEvent, len(events))
	for i, e := range events {
		result[i] = CombatEvent{
			Type:           CombatEventType(e.Type),
			SourcePlayerID: e.SourcePlayerID,
			SourceCardID:   e.SourceCardID,
			SourceRow:      e.SourceRow,
			SourceCol:      e.SourceCol,
			TargetCardID:   e.TargetCardID,
			TargetRow:      e.TargetRow,
			TargetCol:      e.TargetCol,
			TargetIsLeader: e.TargetIsLeader,
			Value:          e.Value,
			ValueHP:        e.ValueHP,
			CardName:       e.CardName,
			Message:        e.Message,
		}
	}
	return result
}

// ──────────────────────────────────────────────
// Card placement
// ──────────────────────────────────────────────

func (gh *GameplayManager) PlayCard(playerID int64, card *effects.CardInstance, row int, col int) error {
	isPlayer1 := playerID == gh.player1ID
	costInMilli := card.Definition.Cost * MilliElixirPerElixir

	if isPlayer1 {
		if gh.game.MilliElixirPlayer1 < costInMilli {
			return fmt.Errorf("not enough elixir: have %d, need %d", gh.game.MilliElixirPlayer1/MilliElixirPerElixir, card.Definition.Cost)
		}
		if gh.game.BoardPlayer1[row][col] != nil {
			return fmt.Errorf("board position [%d][%d] is occupied", row, col)
		}
		gh.game.BoardPlayer1[row][col] = card
		gh.game.MilliElixirPlayer1 -= costInMilli
	} else {
		if gh.game.MilliElixirPlayer2 < costInMilli {
			return fmt.Errorf("not enough elixir: have %d, need %d", gh.game.MilliElixirPlayer2/MilliElixirPerElixir, card.Definition.Cost)
		}
		if gh.game.BoardPlayer2[row][col] != nil {
			return fmt.Errorf("board position [%d][%d] is occupied", row, col)
		}
		gh.game.BoardPlayer2[row][col] = card
		gh.game.MilliElixirPlayer2 -= costInMilli
	}

	return nil
}

// ──────────────────────────────────────────────
// Hand / Draw
// ──────────────────────────────────────────────

func (gh *GameplayManager) SetPlayerDeck(playerID int64, deck []HandCard) {
	hand := gh.getPlayerHand(playerID)

	shuffled := make([]HandCard, len(deck))
	copy(shuffled, deck)
	for i := len(shuffled) - 1; i > 0; i-- {
		j := gh.RNG.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	hand.Deck = shuffled
}

func (gh *GameplayManager) TopUpDrawPile(playerID int64) {
	hand := gh.getPlayerHand(playerID)

	space := MaxDrawPile - len(hand.DrawPile)
	if space <= 0 {
		return
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

func (gh *GameplayManager) SelectCard(playerID int64, cardID int) error {
	hand := gh.getPlayerHand(playerID)

	if len(hand.Hand) >= MaxHand {
		return fmt.Errorf("hand is full (%d/%d)", len(hand.Hand), MaxHand)
	}

	for i, c := range hand.DrawPile {
		if c.CardID == cardID {
			hand.Hand = append(hand.Hand, c)
			hand.DrawPile = append(hand.DrawPile[:i], hand.DrawPile[i+1:]...)
			hand.SelectedThisTurn[cardID]++
			return nil
		}
	}
	return fmt.Errorf("card %d not in draw pile", cardID)
}

func (gh *GameplayManager) DeselectCard(playerID int64, cardID int) error {
	hand := gh.getPlayerHand(playerID)

	if hand.SelectedThisTurn[cardID] <= 0 {
		return fmt.Errorf("card %d cannot be deselected (not selected this turn)", cardID)
	}

	for i, c := range hand.Hand {
		if c.CardID == cardID {
			hand.DrawPile = append(hand.DrawPile, c)
			hand.Hand = append(hand.Hand[:i], hand.Hand[i+1:]...)
			hand.SelectedThisTurn[cardID]--
			if hand.SelectedThisTurn[cardID] <= 0 {
				delete(hand.SelectedThisTurn, cardID)
			}
			return nil
		}
	}
	return fmt.Errorf("card %d not in hand", cardID)
}

func (gh *GameplayManager) ClearSelectedThisTurn(playerID int64) {
	hand := gh.getPlayerHand(playerID)
	hand.SelectedThisTurn = make(map[int]int)
}

func (gh *GameplayManager) PlayFromHand(playerID int64, cardID int) (*HandCard, error) {
	hand := gh.getPlayerHand(playerID)
	for i, c := range hand.Hand {
		if c.CardID == cardID {
			card := hand.Hand[i]
			hand.Hand = append(hand.Hand[:i], hand.Hand[i+1:]...)
			hand.Deck = append(hand.Deck, card)
			return &card, nil
		}
	}
	return nil, fmt.Errorf("card %d not in hand", cardID)
}

func (gh *GameplayManager) GetDrawPile(playerID int64) []HandCard {
	return gh.getPlayerHand(playerID).DrawPile
}

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
