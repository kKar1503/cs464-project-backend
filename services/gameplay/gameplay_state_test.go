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
