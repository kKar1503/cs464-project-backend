package main

import (
	"testing"
	"time"
)

// TestCalculateMMRRange tests the MMR range calculation
func TestCalculateMMRRange(t *testing.T) {
	tests := []struct {
		name            string
		mmr             int
		waitTimeSeconds int
		expectedMin     int
		expectedMax     int
	}{
		{
			name:            "Initial range - no wait time",
			mmr:             1000,
			waitTimeSeconds: 0,
			expectedMin:     900,
			expectedMax:     1100,
		},
		{
			name:            "After 10 seconds - first expansion",
			mmr:             1000,
			waitTimeSeconds: 10,
			expectedMin:     850,
			expectedMax:     1150,
		},
		{
			name:            "After 20 seconds - second expansion",
			mmr:             1000,
			waitTimeSeconds: 20,
			expectedMin:     800,
			expectedMax:     1200,
		},
		{
			name:            "After 80 seconds - max range",
			mmr:             1000,
			waitTimeSeconds: 80,
			expectedMin:     500,
			expectedMax:     1500,
		},
		{
			name:            "After 100 seconds - capped at max",
			mmr:             1000,
			waitTimeSeconds: 100,
			expectedMin:     500,
			expectedMax:     1500,
		},
		{
			name:            "High MMR player",
			mmr:             2500,
			waitTimeSeconds: 0,
			expectedMin:     2400,
			expectedMax:     2600,
		},
		{
			name:            "Low MMR player with wait time",
			mmr:             500,
			waitTimeSeconds: 30,
			expectedMin:     250, // 500 - (100 + 3*50) = 500 - 250 = 250
			expectedMax:     750, // 500 + (100 + 3*50) = 500 + 250 = 750
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMMRRange(tt.mmr, tt.waitTimeSeconds)
			if result.min != tt.expectedMin {
				t.Errorf("Expected min %d, got %d", tt.expectedMin, result.min)
			}
			if result.max != tt.expectedMax {
				t.Errorf("Expected max %d, got %d", tt.expectedMax, result.max)
			}
		})
	}
}

// TestRangesOverlap tests the range overlap detection
func TestRangesOverlap(t *testing.T) {
	tests := []struct {
		name     string
		range1   mmrRange
		range2   mmrRange
		expected bool
	}{
		{
			name:     "Complete overlap",
			range1:   mmrRange{min: 900, max: 1100},
			range2:   mmrRange{min: 950, max: 1050},
			expected: true,
		},
		{
			name:     "Partial overlap",
			range1:   mmrRange{min: 900, max: 1100},
			range2:   mmrRange{min: 1050, max: 1150},
			expected: true,
		},
		{
			name:     "No overlap - range2 higher",
			range1:   mmrRange{min: 900, max: 1100},
			range2:   mmrRange{min: 1200, max: 1300},
			expected: false,
		},
		{
			name:     "No overlap - range2 lower",
			range1:   mmrRange{min: 1200, max: 1300},
			range2:   mmrRange{min: 900, max: 1100},
			expected: false,
		},
		{
			name:     "Edge touching - lower bound",
			range1:   mmrRange{min: 900, max: 1100},
			range2:   mmrRange{min: 1100, max: 1200},
			expected: true,
		},
		{
			name:     "Edge touching - upper bound",
			range1:   mmrRange{min: 1100, max: 1200},
			range2:   mmrRange{min: 900, max: 1100},
			expected: true,
		},
		{
			name:     "Identical ranges",
			range1:   mmrRange{min: 1000, max: 1200},
			range2:   mmrRange{min: 1000, max: 1200},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rangesOverlap(tt.range1, tt.range2)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestComputeMatchesTwoPlayersIdenticalMMR tests instant match with same MMR
func TestComputeMatchesTwoPlayersIdenticalMMR(t *testing.T) {
	now := time.Now()

	// Two players with identical MMR should match instantly
	queue := []QueueEntry{
		{
			UserID:   1,
			Username: "player1",
			MMR:      1000,
			JoinedAt: now,
		},
		{
			UserID:   2,
			Username: "player2",
			MMR:      1000,
			JoinedAt: now,
		},
	}

	matches := computeMatches(queue)

	// Should produce exactly 1 match
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	// Verify the match
	match := matches[0]
	if match.Player1.UserID != 1 || match.Player2.UserID != 2 {
		t.Errorf("Expected players 1 and 2 to match, got %d and %d",
			match.Player1.UserID, match.Player2.UserID)
	}
}

// TestComputeMatchesFIFO tests that FIFO fairness is preserved
func TestComputeMatchesFIFO(t *testing.T) {
	now := time.Now()

	// Create test queue with overlapping MMR ranges
	// Player 1 can match with both Player 2 and Player 3
	// Should match with Player 2 (joined earlier)
	queue := []QueueEntry{
		{
			UserID:   1,
			Username: "player1",
			MMR:      1000,
			JoinedAt: now.Add(-60 * time.Second), // Joined first
		},
		{
			UserID:   2,
			Username: "player2",
			MMR:      1020,
			JoinedAt: now.Add(-50 * time.Second), // Joined second
		},
		{
			UserID:   3,
			Username: "player3",
			MMR:      1010,
			JoinedAt: now.Add(-40 * time.Second), // Joined third (closer MMR but joined later)
		},
	}

	matches := computeMatches(queue)

	// Should produce exactly 1 match
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	// Player 1 should match with Player 2 (earliest joined among overlaps)
	match := matches[0]
	if match.Player1.UserID != 1 {
		t.Errorf("Expected Player1 to be user 1, got %d", match.Player1.UserID)
	}
	if match.Player2.UserID != 2 {
		t.Errorf("Expected Player2 to be user 2 (FIFO), got %d", match.Player2.UserID)
	}
}

// TestMatchingWithDifferentWaitTimes tests range expansion over time
func TestMatchingWithDifferentWaitTimes(t *testing.T) {
	tests := []struct {
		name        string
		player1MMR  int
		player1Wait int
		player2MMR  int
		player2Wait int
		shouldMatch bool
	}{
		{
			name:        "Similar MMR, no wait - should match",
			player1MMR:  1000,
			player1Wait: 0,
			player2MMR:  1050,
			player2Wait: 0,
			shouldMatch: true,
		},
		{
			name:        "Different MMR, no wait - should match at edge",
			player1MMR:  1000,
			player1Wait: 0,
			player2MMR:  1200,
			player2Wait: 0,
			shouldMatch: true, // ranges touch at 1100
		},
		{
			name:        "Different MMR, with wait - should match after expansion",
			player1MMR:  1000,
			player1Wait: 20,
			player2MMR:  1200,
			player2Wait: 20,
			shouldMatch: true,
		},
		{
			name:        "Very different MMR, max wait - should match at max range",
			player1MMR:  1000,
			player1Wait: 100,
			player2MMR:  1400,
			player2Wait: 100,
			shouldMatch: true,
		},
		{
			name:        "Too different even at max range - should match at edge",
			player1MMR:  1000,
			player1Wait: 100,
			player2MMR:  2000,
			player2Wait: 100,
			shouldMatch: true, // ranges touch at 1500: [500-1500] vs [1500-2500]
		},
		{
			name:        "One player waited long, other just joined",
			player1MMR:  1000,
			player1Wait: 60,
			player2MMR:  1350,
			player2Wait: 0,
			shouldMatch: true, // player1's range expanded to include 1350
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			range1 := calculateMMRRange(tt.player1MMR, tt.player1Wait)
			range2 := calculateMMRRange(tt.player2MMR, tt.player2Wait)
			result := rangesOverlap(range1, range2)

			if result != tt.shouldMatch {
				t.Errorf("Expected match=%v, got match=%v (range1: %d-%d, range2: %d-%d)",
					tt.shouldMatch, result, range1.min, range1.max, range2.min, range2.max)
			}
		})
	}
}

// TestComputeMatchesMultiplePlayers tests complex matching scenarios
func TestComputeMatchesMultiplePlayers(t *testing.T) {
	now := time.Now()

	// Create a queue with multiple potential matches
	queue := []QueueEntry{
		{UserID: 1, Username: "p1", MMR: 1000, JoinedAt: now.Add(-90 * time.Second)},
		{UserID: 2, Username: "p2", MMR: 1050, JoinedAt: now.Add(-80 * time.Second)},
		{UserID: 3, Username: "p3", MMR: 1500, JoinedAt: now.Add(-70 * time.Second)},
		{UserID: 4, Username: "p4", MMR: 1520, JoinedAt: now.Add(-60 * time.Second)},
		{UserID: 5, Username: "p5", MMR: 2000, JoinedAt: now.Add(-50 * time.Second)},
		{UserID: 6, Username: "p6", MMR: 2020, JoinedAt: now.Add(-40 * time.Second)},
	}

	matches := computeMatches(queue)

	// Expected: 3 matches total
	// p1 (1000) matches p2 (1050) - close MMR
	// p3 (1500) matches p4 (1520) - close MMR
	// p5 (2000) matches p6 (2020) - close MMR

	if len(matches) != 3 {
		t.Fatalf("Expected 3 matches, got %d", len(matches))
	}

	// Verify specific matches
	expectedPairs := [][2]int64{
		{1, 2}, // p1 matches p2
		{3, 4}, // p3 matches p4
		{5, 6}, // p5 matches p6
	}

	for i, match := range matches {
		expected := expectedPairs[i]
		if match.Player1.UserID != expected[0] {
			t.Errorf("Match %d: Expected Player1 to be %d, got %d", i, expected[0], match.Player1.UserID)
		}
		if match.Player2.UserID != expected[1] {
			t.Errorf("Match %d: Expected Player2 to be %d, got %d", i, expected[1], match.Player2.UserID)
		}
	}
}

// BenchmarkComputeMatches benchmarks the matching algorithm
func BenchmarkComputeMatches(b *testing.B) {
	now := time.Now()

	// Create a large queue
	queue := make([]QueueEntry, 1000)
	for i := 0; i < 1000; i++ {
		queue[i] = QueueEntry{
			UserID:   int64(i + 1),
			Username: "player",
			MMR:      1000 + (i * 10), // Spread MMR from 1000-11000
			JoinedAt: now.Add(-time.Duration(i) * time.Second),
		}
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = computeMatches(queue)
	}
}

// TestComputeMatchesEmptyQueue tests edge case with no players
func TestComputeMatchesEmptyQueue(t *testing.T) {
	matches := computeMatches([]QueueEntry{})
	if matches != nil {
		t.Errorf("Expected nil for empty queue, got %d matches", len(matches))
	}
}

// TestComputeMatchesSinglePlayer tests edge case with only one player
func TestComputeMatchesSinglePlayer(t *testing.T) {
	now := time.Now()
	queue := []QueueEntry{
		{UserID: 1, Username: "lonely", MMR: 1000, JoinedAt: now},
	}

	matches := computeMatches(queue)
	if matches != nil {
		t.Errorf("Expected nil for single player, got %d matches", len(matches))
	}
}

// TestComputeMatchesNoOverlap tests when no players can match
func TestComputeMatchesNoOverlap(t *testing.T) {
	now := time.Now()
	queue := []QueueEntry{
		{UserID: 1, Username: "p1", MMR: 1000, JoinedAt: now},
		{UserID: 2, Username: "p2", MMR: 2000, JoinedAt: now},
		{UserID: 3, Username: "p3", MMR: 3000, JoinedAt: now},
	}

	matches := computeMatches(queue)
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches for non-overlapping MMRs, got %d", len(matches))
	}
}
