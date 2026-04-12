package main

import "testing"

func TestScaleStat(t *testing.T) {
	cases := []struct {
		base  int
		level int
		want  int
	}{
		// Spec examples from the leveling design doc.
		{10, 1, 10},
		{10, 2, 12},
		{10, 3, 14},
		{10, 4, 16},
		{10, 5, 18},
		// Ceiling behavior: 0.2 increments of odd bases round up.
		{7, 2, 9},   // 1.2 * 7 = 8.4 → 9
		{7, 3, 10},  // 1.4 * 7 = 9.8 → 10
		{1, 2, 2},   // 1.2 * 1 = 1.2 → 2
		// Invalid/defensive inputs
		{10, 0, 10},
		{10, -1, 10},
	}
	for _, c := range cases {
		got := ScaleStat(c.base, c.level)
		if got != c.want {
			t.Errorf("ScaleStat(base=%d, level=%d) = %d, want %d", c.base, c.level, got, c.want)
		}
	}
}

func TestDisenchantValue(t *testing.T) {
	cases := []struct {
		rarity string
		want   int32
	}{
		{"common", 10},
		{"rare", 50},
		{"epic", 100},
		{"legendary", 500},
		{"unknown", 0},
		{"", 0},
	}
	for _, c := range cases {
		if got := DisenchantValue(c.rarity); got != c.want {
			t.Errorf("DisenchantValue(%q) = %d, want %d", c.rarity, got, c.want)
		}
	}
}

func TestLevelUpCost(t *testing.T) {
	cases := []struct {
		level        int
		wantCards    int32
		wantCrystals int32
	}{
		{1, 4, 100},
		{2, 16, 1000},
		{3, 64, 10000},
		{4, 256, 100000},
		// Defensive: level below 1 is treated as 1.
		{0, 4, 100},
	}
	for _, c := range cases {
		cards, crystals := LevelUpCost(c.level)
		if cards != c.wantCards || crystals != c.wantCrystals {
			t.Errorf("LevelUpCost(%d) = (%d, %d), want (%d, %d)",
				c.level, cards, crystals, c.wantCards, c.wantCrystals)
		}
	}
}
