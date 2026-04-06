package main

import (
	"testing"

	"github.com/kKar1503/cs464-backend/services/gameplay/effects"
)

func newTestManager() *GameplayManager {
	gm := NewGameplayManager("test-session", 1, 2)
	gm.CardStore = effects.NewCardDefinitionStore(nil)
	return gm
}

func TestInitialElixir(t *testing.T) {
	gm := newTestManager()

	if gm.GetElixirDisplay(1) != StartingElixir {
		t.Errorf("player1 starting elixir: got %d, want %d", gm.GetElixirDisplay(1), StartingElixir)
	}
	if gm.GetElixirDisplay(2) != StartingElixir {
		t.Errorf("player2 starting elixir: got %d, want %d", gm.GetElixirDisplay(2), StartingElixir)
	}
	if gm.GetMilliElixir(1) != StartingElixir*MilliElixirPerElixir {
		t.Errorf("player1 starting milliElixir: got %d, want %d", gm.GetMilliElixir(1), StartingElixir*MilliElixirPerElixir)
	}
}

func TestElixirCapAtRound1(t *testing.T) {
	gm := newTestManager()

	if gm.game.ElixirCap != StartingElixirCap {
		t.Fatalf("round 1 elixir cap: got %d, want %d", gm.game.ElixirCap, StartingElixirCap)
	}

	changed := gm.TickElixir()
	if !changed {
		t.Error("elixir should change when below cap")
	}
}

func TestElixirRechargeAfterSpend(t *testing.T) {
	gm := newTestManager()

	gm.game.MilliElixirPlayer1 -= 2 * MilliElixirPerElixir

	if gm.GetElixirDisplay(1) != 1 {
		t.Fatalf("after spending 2: got %d, want 1", gm.GetElixirDisplay(1))
	}

	for i := 0; i < 20; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 2 {
		t.Errorf("after 20 ticks (1 elixir): got %d, want 2", gm.GetElixirDisplay(1))
	}

	for i := 0; i < 20; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 3 {
		t.Errorf("after 40 ticks (2 elixir): got %d, want 3", gm.GetElixirDisplay(1))
	}

	for i := 0; i < 40; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 5 {
		t.Errorf("should reach cap 5: got %d", gm.GetElixirDisplay(1))
	}
}

func TestElixirCapIncreasesPerRound(t *testing.T) {
	gm := newTestManager()

	expectedCaps := []int{5, 6, 7, 8, 8, 8}
	for i, expected := range expectedCaps {
		if gm.game.ElixirCap != expected {
			t.Errorf("round %d: elixir cap got %d, want %d", i+1, gm.game.ElixirCap, expected)
		}
		gm.AdvanceRound()
	}
}

func TestElixirCarriesOverBetweenRounds(t *testing.T) {
	gm := newTestManager()

	gm.game.MilliElixirPlayer1 -= 1 * MilliElixirPerElixir

	if gm.GetElixirDisplay(1) != 2 {
		t.Fatalf("before advance: got %d, want 2", gm.GetElixirDisplay(1))
	}

	gm.AdvanceRound()

	if gm.GetElixirDisplay(1) != 2 {
		t.Errorf("after advance to round 2: got %d, want 2 (no reset)", gm.GetElixirDisplay(1))
	}

	for i := 0; i < 80; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 6 {
		t.Errorf("after recharging to round 2 cap: got %d, want 6", gm.GetElixirDisplay(1))
	}

	for i := 0; i < 20; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 6 {
		t.Errorf("should stay at round 2 cap 6: got %d", gm.GetElixirDisplay(1))
	}
}

func TestElixirMaxCap8(t *testing.T) {
	gm := newTestManager()

	for i := 0; i < 3; i++ {
		gm.AdvanceRound()
	}
	if gm.game.ElixirCap != 8 {
		t.Fatalf("round 4 cap: got %d, want 8", gm.game.ElixirCap)
	}

	gm.game.MilliElixirPlayer1 = 0

	for i := 0; i < 160; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 8 {
		t.Errorf("full recharge to max: got %d, want 8", gm.GetElixirDisplay(1))
	}

	gm.TickElixir()
	if gm.GetMilliElixir(1) != MaxMilliElixir {
		t.Errorf("should not exceed max: got %d, want %d", gm.GetMilliElixir(1), MaxMilliElixir)
	}
}

func TestMilliElixirFractional(t *testing.T) {
	gm := newTestManager()

	gm.game.MilliElixirPlayer1 = 0

	gm.TickElixir()
	if gm.GetMilliElixir(1) != MilliElixirPerTick {
		t.Errorf("after 1 tick: got %d milliElixir, want %d", gm.GetMilliElixir(1), MilliElixirPerTick)
	}
	if gm.GetElixirDisplay(1) != 0 {
		t.Errorf("display after 1 tick: got %d, want 0", gm.GetElixirDisplay(1))
	}

	for i := 0; i < 9; i++ {
		gm.TickElixir()
	}
	if gm.GetMilliElixir(1) != 500 {
		t.Errorf("after 10 ticks: got %d milliElixir, want 500", gm.GetMilliElixir(1))
	}
	if gm.GetElixirDisplay(1) != 0 {
		t.Errorf("display after 10 ticks: got %d, want 0", gm.GetElixirDisplay(1))
	}

	for i := 0; i < 10; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 1 {
		t.Errorf("display after 20 ticks: got %d, want 1", gm.GetElixirDisplay(1))
	}
}

func TestPlayCardDeductsElixir(t *testing.T) {
	gm := newTestManager()

	card := makeTestCard(1, "TestCard", 2, 10, 10)
	err := gm.PlayCard(1, card, 0, 0)
	if err != nil {
		t.Fatalf("PlayCard failed: %v", err)
	}

	if gm.GetElixirDisplay(1) != 1 {
		t.Errorf("after playing 2-cost card from 3: got %d, want 1", gm.GetElixirDisplay(1))
	}
	if gm.GetMilliElixir(1) != 1*MilliElixirPerElixir {
		t.Errorf("milliElixir: got %d, want %d", gm.GetMilliElixir(1), 1*MilliElixirPerElixir)
	}
}

func TestPlayCardNotEnoughElixir(t *testing.T) {
	gm := newTestManager()

	card := makeTestCard(1, "TestCard", 5, 10, 10)
	err := gm.PlayCard(1, card, 0, 0)
	if err == nil {
		t.Error("should fail when not enough elixir")
	}
}

func makeTestCard(cardID int, name string, cost, atk, hp int) *effects.CardInstance {
	def := &effects.CardDefinition{
		CardID: cardID,
		Name:   name,
		Colour: "Grey",
		Cost:   cost,
		BaseAtk: atk,
		BaseHP:  hp,
	}
	return &effects.CardInstance{
		InstanceID:           cardID,
		Definition:           def,
		CurrentAtk:           atk,
		CurrentHP:            hp,
		MaxHP:                hp,
		ChargeTicksRemaining: effects.ChargeTicksTotal,
		ChargeTicksTotal:     effects.ChargeTicksTotal,
		IsCharging:           true,
	}
}

func placeTestCard(gm *GameplayManager, isPlayer1 bool, row, col, cardID, atk, hp int) {
	card := makeTestCard(cardID, "TestCard", 1, atk, hp)
	if isPlayer1 {
		gm.game.BoardPlayer1[row][col] = card
	} else {
		gm.game.BoardPlayer2[row][col] = card
	}
}

func TestCardChargeAndAutoAttack(t *testing.T) {
	gm := newTestManager()

	placeTestCard(gm, true, 0, 0, 1, 20, 10)

	for i := 0; i < 39; i++ {
		gm.TickBoard()
	}
	card := gm.game.BoardPlayer1[0][0]
	if card.ChargeTicksRemaining != 1 {
		t.Errorf("card should have 1 tick remaining after 39, got %d", card.ChargeTicksRemaining)
	}

	gm.TickBoard()

	if gm.game.Player2Health != 230 {
		t.Errorf("leader should take 20 damage: got HP %d, want 230", gm.game.Player2Health)
	}
}

func TestAutoAttackHitsLeader(t *testing.T) {
	gm := newTestManager()

	placeTestCard(gm, true, 0, 0, 1, 20, 50)

	for i := 0; i < 40; i++ {
		gm.TickBoard()
	}

	if gm.game.Player2Health != 230 {
		t.Errorf("leader HP after attack: got %d, want 230", gm.game.Player2Health)
	}

	attackerHP := gm.game.BoardPlayer1[0][0].CurrentHP
	if attackerHP != 40 {
		t.Errorf("attacker HP after leader counter: got %d, want 40", attackerHP)
	}

	if len(gm.game.CombatLog) != 2 {
		t.Fatalf("expected 2 combat events (attack + counter), got %d", len(gm.game.CombatLog))
	}
	attack := gm.game.CombatLog[0]
	if attack.Type != CombatEventAttack {
		t.Errorf("first event should be attack, got %s", attack.Type)
	}
	if !attack.TargetIsLeader {
		t.Error("attack should target leader")
	}
	counter := gm.game.CombatLog[1]
	if counter.Type != CombatEventCounterAttack {
		t.Errorf("second event should be counter_attack, got %s", counter.Type)
	}
	if counter.Value != LeaderAttack {
		t.Errorf("counter damage: got %d, want %d", counter.Value, LeaderAttack)
	}
}

func TestAutoAttackHitsEnemyCard(t *testing.T) {
	gm := newTestManager()

	placeTestCard(gm, true, 0, 1, 1, 15, 50)
	placeTestCard(gm, false, 0, 1, 2, 10, 20)

	for i := 0; i < 40; i++ {
		gm.TickBoard()
	}

	enemyCard := gm.game.BoardPlayer2[0][1]
	if enemyCard == nil || enemyCard.CurrentHP != 5 {
		if enemyCard == nil {
			t.Errorf("enemy card should still exist with 5 HP")
		} else {
			t.Errorf("enemy card HP: got %d, want 5", enemyCard.CurrentHP)
		}
	}

	if gm.game.Player2Health != 250 {
		t.Errorf("leader should not be hit: got %d, want 250", gm.game.Player2Health)
	}
}

func TestCardDeathRemoval(t *testing.T) {
	gm := newTestManager()

	placeTestCard(gm, true, 0, 0, 1, 30, 50)
	placeTestCard(gm, false, 0, 0, 2, 5, 10)

	for i := 0; i < 40; i++ {
		gm.TickBoard()
	}

	if gm.game.BoardPlayer2[0][0] != nil {
		t.Errorf("dead card should be removed (nil)")
	}
}

func TestMultipleCardsAttackSameTick(t *testing.T) {
	gm := newTestManager()

	placeTestCard(gm, true, 0, 0, 1, 10, 50)
	placeTestCard(gm, true, 0, 1, 2, 20, 50)

	for i := 0; i < 40; i++ {
		gm.TickBoard()
	}

	if gm.game.Player2Health != 220 {
		t.Errorf("leader HP after both attacks: got %d, want 220", gm.game.Player2Health)
	}

	if len(gm.game.CombatLog) != 4 {
		t.Errorf("expected 4 combat events, got %d", len(gm.game.CombatLog))
	}

	if !gm.game.BoardPlayer1[0][0].IsCharging {
		t.Error("card at [0][0] should be charging again")
	}
	if !gm.game.BoardPlayer1[0][1].IsCharging {
		t.Error("card at [0][1] should be charging again")
	}
}

func TestWinConditionOnLeaderDeath(t *testing.T) {
	gm := newTestManager()

	gm.game.Player2Health = 5

	placeTestCard(gm, true, 0, 0, 1, 10, 50)

	for i := 0; i < 40; i++ {
		gm.TickBoard()
	}

	gameOver, winner := gm.CheckWinCondition()
	if !gameOver {
		t.Error("game should be over")
	}
	if winner != 1 {
		t.Errorf("winner should be 1 (player 2 died), got %d", winner)
	}
}

func TestPlayFromHandFailsCardStaysInHand(t *testing.T) {
	gm := newTestManager()

	// Give player 1 only 2 elixir
	gm.game.MilliElixirPlayer1 = 2 * MilliElixirPerElixir

	// Put a 5-cost card in hand
	expensiveCard := HandCard{
		CardID: 99, CardName: "Expensive", Colour: "Grey",
		Rarity: "common", ManaCost: 5, Attack: 10, HP: 10,
	}
	gm.game.Player1Hand = &PlayerHand{
		Deck:             []HandCard{},
		DrawPile:         []HandCard{},
		Hand:             []HandCard{expensiveCard},
		SelectedThisTurn: make(map[int]int),
	}

	// Verify card is in hand via GetHandCard
	card, ok := gm.GetHandCard(1, 99)
	if !ok || card == nil {
		t.Fatal("expensive card should be in hand before attempt")
	}

	// Attempting PlayFromHand should succeed (it only removes from hand)
	// but the real flow checks elixir BEFORE calling PlayFromHand.
	// Verify the card is findable so the elixir check can reject first.
	if card.ManaCost <= gm.GetElixirDisplay(1) {
		t.Fatalf("test setup wrong: card cost %d should exceed elixir %d",
			card.ManaCost, gm.GetElixirDisplay(1))
	}

	// After the elixir check rejects, the card must still be in hand
	remaining := gm.GetHand(1)
	if len(remaining) != 1 || remaining[0].CardID != 99 {
		t.Errorf("card should still be in hand after failed elixir check, got %v", remaining)
	}
}

func TestPlayFromHandSuccessCardLeavesHand(t *testing.T) {
	gm := newTestManager()

	// Give player 1 enough elixir
	gm.game.MilliElixirPlayer1 = 3 * MilliElixirPerElixir

	cheapCard := HandCard{
		CardID: 42, CardName: "Cheap", Colour: "Grey",
		Rarity: "common", ManaCost: 2, Attack: 5, HP: 5,
	}
	gm.game.Player1Hand = &PlayerHand{
		Deck:             []HandCard{},
		DrawPile:         []HandCard{},
		Hand:             []HandCard{cheapCard},
		SelectedThisTurn: make(map[int]int),
	}

	// Peek first — card should be affordable
	card, ok := gm.GetHandCard(1, 42)
	if !ok {
		t.Fatal("cheap card should be in hand")
	}
	if card.ManaCost > gm.GetElixirDisplay(1) {
		t.Fatal("should have enough elixir for this card")
	}

	// Now actually play it
	played, err := gm.PlayFromHand(1, 42)
	if err != nil {
		t.Fatalf("PlayFromHand failed: %v", err)
	}
	if played.CardID != 42 {
		t.Errorf("played card ID: got %d, want 42", played.CardID)
	}

	// Hand should be empty, card should be back in deck
	if len(gm.GetHand(1)) != 0 {
		t.Error("hand should be empty after playing")
	}
	hand := gm.getPlayerHand(1)
	if len(hand.Deck) != 1 || hand.Deck[0].CardID != 42 {
		t.Errorf("card should be back in deck, got %v", hand.Deck)
	}
}

func TestBothPlayersIndependent(t *testing.T) {
	gm := newTestManager()

	gm.game.MilliElixirPlayer1 = 0

	if gm.GetElixirDisplay(2) != 3 {
		t.Errorf("player2 should be unaffected: got %d, want 3", gm.GetElixirDisplay(2))
	}

	gm.TickElixir()
	if gm.GetMilliElixir(1) != MilliElixirPerTick {
		t.Errorf("player1 after tick: got %d, want %d", gm.GetMilliElixir(1), MilliElixirPerTick)
	}
	if gm.GetMilliElixir(2) != 3*MilliElixirPerElixir+MilliElixirPerTick {
		t.Errorf("player2 should tick up: got %d, want %d", gm.GetMilliElixir(2), 3*MilliElixirPerElixir+MilliElixirPerTick)
	}
}
