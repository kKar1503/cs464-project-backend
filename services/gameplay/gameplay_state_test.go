package main

import (
	"testing"

	"github.com/kKar1503/cs464-backend/services/gameplay/handlers"
)

func newTestManager() *GameplayManager {
	return NewGameplayManager("test-session", 1, 2)
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

	if gm.game.ElixirCap != 3 {
		t.Fatalf("round 1 elixir cap: got %d, want 3", gm.game.ElixirCap)
	}

	// Already at cap (3), ticking should not increase
	changed := gm.TickElixir()
	if changed {
		t.Error("elixir should not change when already at cap")
	}
	if gm.GetElixirDisplay(1) != 3 {
		t.Errorf("player1 elixir after tick at cap: got %d, want 3", gm.GetElixirDisplay(1))
	}
}

func TestElixirRechargeAfterSpend(t *testing.T) {
	gm := newTestManager()

	// Spend 2 elixir (2000 milliElixir) from player 1
	gm.game.MilliElixirPlayer1 -= 2 * MilliElixirPerElixir

	if gm.GetElixirDisplay(1) != 1 {
		t.Fatalf("after spending 2: got %d, want 1", gm.GetElixirDisplay(1))
	}

	// Tick 20 times = 1 elixir recharged (20 ticks * 50 milli = 1000)
	for i := 0; i < 20; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 2 {
		t.Errorf("after 20 ticks (1 elixir): got %d, want 2", gm.GetElixirDisplay(1))
	}

	// Tick 20 more = back to cap of 3
	for i := 0; i < 20; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 3 {
		t.Errorf("after 40 ticks (2 elixir): got %d, want 3", gm.GetElixirDisplay(1))
	}

	// Tick more — should stay at 3 (round 1 cap)
	for i := 0; i < 20; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 3 {
		t.Errorf("should stay at cap 3: got %d", gm.GetElixirDisplay(1))
	}
}

func TestElixirCapIncreasesPerRound(t *testing.T) {
	gm := newTestManager()

	expectedCaps := []int{3, 4, 5, 6, 7, 8, 8, 8}
	for i, expected := range expectedCaps {
		if gm.game.ElixirCap != expected {
			t.Errorf("round %d: elixir cap got %d, want %d", i+1, gm.game.ElixirCap, expected)
		}
		gm.AdvanceRound()
	}
}

func TestElixirCarriesOverBetweenRounds(t *testing.T) {
	gm := newTestManager()

	// Spend 1 elixir, leaving 2 at round 1
	gm.game.MilliElixirPlayer1 -= 1 * MilliElixirPerElixir

	if gm.GetElixirDisplay(1) != 2 {
		t.Fatalf("before advance: got %d, want 2", gm.GetElixirDisplay(1))
	}

	// Advance to round 2 (cap becomes 4)
	gm.AdvanceRound()

	// Elixir should still be 2 — no reset
	if gm.GetElixirDisplay(1) != 2 {
		t.Errorf("after advance to round 2: got %d, want 2 (no reset)", gm.GetElixirDisplay(1))
	}

	// Now it can recharge up to 4
	for i := 0; i < 40; i++ { // 40 ticks = 2 elixir
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 4 {
		t.Errorf("after recharging to round 2 cap: got %d, want 4", gm.GetElixirDisplay(1))
	}

	// Should not exceed 4 in round 2
	for i := 0; i < 20; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 4 {
		t.Errorf("should stay at round 2 cap 4: got %d", gm.GetElixirDisplay(1))
	}
}

func TestElixirMaxCap8(t *testing.T) {
	gm := newTestManager()

	// Advance to round 6+ where cap is 8
	for i := 0; i < 6; i++ {
		gm.AdvanceRound()
	}
	if gm.game.ElixirCap != 8 {
		t.Fatalf("round 7 cap: got %d, want 8", gm.game.ElixirCap)
	}

	// Spend all elixir
	gm.game.MilliElixirPlayer1 = 0

	// Recharge to 8 (160 ticks = 8 elixir)
	for i := 0; i < 160; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 8 {
		t.Errorf("full recharge to max: got %d, want 8", gm.GetElixirDisplay(1))
	}

	// Should not exceed 8
	gm.TickElixir()
	if gm.GetMilliElixir(1) != MaxMilliElixir {
		t.Errorf("should not exceed max: got %d, want %d", gm.GetMilliElixir(1), MaxMilliElixir)
	}
}

func TestMilliElixirFractional(t *testing.T) {
	gm := newTestManager()

	// Spend all elixir
	gm.game.MilliElixirPlayer1 = 0

	// 1 tick = 50 milliElixir
	gm.TickElixir()
	if gm.GetMilliElixir(1) != MilliElixirPerTick {
		t.Errorf("after 1 tick: got %d milliElixir, want %d", gm.GetMilliElixir(1), MilliElixirPerTick)
	}
	// Display should still be 0 (not yet 1 full elixir)
	if gm.GetElixirDisplay(1) != 0 {
		t.Errorf("display after 1 tick: got %d, want 0", gm.GetElixirDisplay(1))
	}

	// 10 ticks = 500 milliElixir = 0 display
	for i := 0; i < 9; i++ {
		gm.TickElixir()
	}
	if gm.GetMilliElixir(1) != 500 {
		t.Errorf("after 10 ticks: got %d milliElixir, want 500", gm.GetMilliElixir(1))
	}
	if gm.GetElixirDisplay(1) != 0 {
		t.Errorf("display after 10 ticks: got %d, want 0", gm.GetElixirDisplay(1))
	}

	// 20 ticks total = 1000 milliElixir = 1 display
	for i := 0; i < 10; i++ {
		gm.TickElixir()
	}
	if gm.GetElixirDisplay(1) != 1 {
		t.Errorf("display after 20 ticks: got %d, want 1", gm.GetElixirDisplay(1))
	}
}

func TestPlayCardDeductsElixir(t *testing.T) {
	gm := newTestManager()

	card := &handlers.Card{
		CardID:     1,
		ElixerCost: 2,
	}

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

	card := &handlers.Card{
		CardID:     1,
		ElixerCost: 5, // costs more than round 1 cap of 3
	}

	err := gm.PlayCard(1, card, 0, 0)
	if err == nil {
		t.Error("should fail when not enough elixir")
	}
}

func placeTestCard(gm *GameplayManager, isPlayer1 bool, row, col, cardID, atk, hp int) {
	board := gm.game.BoardPlayer1
	if !isPlayer1 {
		board = gm.game.BoardPlayer2
	}
	board[row][col] = handlers.Card{
		CardID:               cardID,
		CardName:             "TestCard",
		CardAttack:           atk,
		CurrentHealth:        hp,
		MaxHealth:            hp,
		ChargeTicksRemaining: handlers.ChargeTicksTotal,
		IsCharging:           true,
	}
}

func TestCardChargeAndAutoAttack(t *testing.T) {
	gm := newTestManager()

	// Place a card for player 1 in row 0, col 0
	placeTestCard(gm, true, 0, 0, 1, 20, 10)

	// Tick 39 times — card should still be charging
	for i := 0; i < 39; i++ {
		gm.TickBoard()
	}
	card := gm.game.BoardPlayer1[0][0]
	if card.ChargeTicksRemaining != 1 {
		t.Errorf("card should have 1 tick remaining after 39, got %d", card.ChargeTicksRemaining)
	}

	// Tick once more — attack resolves instantly
	gm.TickBoard()

	// Should have hit leader (no enemies in column 0)
	if gm.game.Player2Health != 230 { // 250 - 20
		t.Errorf("leader should take 20 damage: got HP %d, want 230", gm.game.Player2Health)
	}
}

func TestAutoAttackHitsLeader(t *testing.T) {
	gm := newTestManager()

	// Place a card for player 1 — no enemy cards in column 0
	placeTestCard(gm, true, 0, 0, 1, 20, 50)

	// Charge fully — attack resolves on tick 40 (cooldown starts at 0)
	for i := 0; i < 40; i++ {
		gm.TickBoard()
	}

	// Player 2 leader should take 20 damage (250 - 20 = 230)
	if gm.game.Player2Health != 230 {
		t.Errorf("leader HP after attack: got %d, want 230", gm.game.Player2Health)
	}

	// Attacker should take counter damage (LeaderAttack = 10)
	attackerHP := gm.game.BoardPlayer1[0][0].CurrentHealth
	if attackerHP != 40 { // 50 - 10 = 40
		t.Errorf("attacker HP after leader counter: got %d, want 40", attackerHP)
	}

	// Attack log should have the event
	if len(gm.game.LastAttackLog) != 1 {
		t.Fatalf("expected 1 attack event, got %d", len(gm.game.LastAttackLog))
	}
	event := gm.game.LastAttackLog[0]
	if !event.TargetIsLeader {
		t.Error("attack should target leader")
	}
	if event.CounterDamage != LeaderAttack {
		t.Errorf("counter damage: got %d, want %d", event.CounterDamage, LeaderAttack)
	}
}

func TestAutoAttackHitsEnemyCard(t *testing.T) {
	gm := newTestManager()

	// Player 1 card in row 0, col 1
	placeTestCard(gm, true, 0, 1, 1, 15, 50)
	// Player 2 card in front row, col 1 (blocks the attack)
	placeTestCard(gm, false, 0, 1, 2, 10, 20)

	// Charge and resolve player 1's attack
	for i := 0; i < 40; i++ {
		gm.TickBoard()
	}


	// Enemy card should take 15 damage (20 - 15 = 5)
	enemyHP := gm.game.BoardPlayer2[0][1].CurrentHealth
	if enemyHP != 5 {
		t.Errorf("enemy card HP: got %d, want 5", enemyHP)
	}

	// Leader should not be damaged
	if gm.game.Player2Health != 250 {
		t.Errorf("leader should not be hit: got %d, want 250", gm.game.Player2Health)
	}
}

func TestCardDeathRemoval(t *testing.T) {
	gm := newTestManager()

	// Player 1 card with 30 attack
	placeTestCard(gm, true, 0, 0, 1, 30, 50)
	// Player 2 card with only 10 HP in same column
	placeTestCard(gm, false, 0, 0, 2, 5, 10)

	// Charge and resolve
	for i := 0; i < 40; i++ {
		gm.TickBoard()
	}


	// Enemy card should be dead and removed
	if gm.game.BoardPlayer2[0][0].CardID != 0 {
		t.Errorf("dead card should be removed, got CardID %d", gm.game.BoardPlayer2[0][0].CardID)
	}
}

func TestMultipleCardsAttackSameTick(t *testing.T) {
	gm := newTestManager()

	// Place 2 cards that will charge at the same time
	placeTestCard(gm, true, 0, 0, 1, 10, 50)
	placeTestCard(gm, true, 0, 1, 2, 20, 50)

	// Charge both fully — both resolve instantly on tick 40
	for i := 0; i < 40; i++ {
		gm.TickBoard()
	}

	// Both attacks hit leader (no enemies), total damage = 10 + 20 = 30
	if gm.game.Player2Health != 220 { // 250 - 30
		t.Errorf("leader HP after both attacks: got %d, want 220", gm.game.Player2Health)
	}

	// Both should have attack events logged
	if len(gm.game.LastAttackLog) != 2 {
		t.Errorf("expected 2 attack events, got %d", len(gm.game.LastAttackLog))
	}

	// Both cards should have restarted charging
	if !gm.game.BoardPlayer1[0][0].IsCharging {
		t.Error("card at [0][0] should be charging again")
	}
	if !gm.game.BoardPlayer1[0][1].IsCharging {
		t.Error("card at [0][1] should be charging again")
	}
}

func TestWinConditionOnLeaderDeath(t *testing.T) {
	gm := newTestManager()

	// Set player 2 HP very low
	gm.game.Player2Health = 5

	// Place a strong card
	placeTestCard(gm, true, 0, 0, 1, 10, 50)

	// Charge and resolve — should kill leader
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

func TestBothPlayersIndependent(t *testing.T) {
	gm := newTestManager()

	// Spend all of player 1's elixir
	gm.game.MilliElixirPlayer1 = 0

	// Player 2 should still be at 3
	if gm.GetElixirDisplay(2) != 3 {
		t.Errorf("player2 should be unaffected: got %d, want 3", gm.GetElixirDisplay(2))
	}

	// Tick — player 1 recharges, player 2 stays at cap
	gm.TickElixir()
	if gm.GetMilliElixir(1) != MilliElixirPerTick {
		t.Errorf("player1 after tick: got %d, want %d", gm.GetMilliElixir(1), MilliElixirPerTick)
	}
	if gm.GetMilliElixir(2) != 3*MilliElixirPerElixir {
		t.Errorf("player2 should stay at cap: got %d, want %d", gm.GetMilliElixir(2), 3*MilliElixirPerElixir)
	}
}
