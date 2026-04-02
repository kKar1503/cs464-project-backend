package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
)

type PackResponse struct {
	PackID    int64  `json:"pack_id"`
	PackType  string `json:"pack_type"`
	IsOpened  bool   `json:"is_opened"`
	CreatedAt string `json:"created_at"`
}

type OpenPackResponse struct {
	PackID   int64      `json:"pack_id"`
	PackType string     `json:"pack_type"`
	Cards    []PackCard `json:"cards"`
}

type PackCard struct {
	CardID   int64  `json:"card_id"`
	CardName string `json:"card_name"`
	Rarity   string `json:"rarity"`
}

func handleGetPacks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	rows, err := db.Query(
		"SELECT pack_id, pack_type, is_opened, created_at FROM card_packs WHERE player_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get packs"})
		return
	}
	defer rows.Close()

	var packs []PackResponse
	for rows.Next() {
		var p PackResponse
		var createdAt []uint8
		if err := rows.Scan(&p.PackID, &p.PackType, &p.IsOpened, &createdAt); err != nil {
			continue
		}
		p.CreatedAt = string(createdAt)
		packs = append(packs, p)
	}

	if packs == nil {
		packs = []PackResponse{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"packs": packs})
}

func handleOpenPack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	packIDStr := r.URL.Query().Get("pack_id")
	if packIDStr == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "pack_id is required"})
		return
	}
	var packID int64
	if _, err := fmt.Sscanf(packIDStr, "%d", &packID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid pack_id"})
		return
	}

	var packType string
	var isOpened bool
	err = db.QueryRow(
		"SELECT pack_type, is_opened FROM card_packs WHERE pack_id = ? AND player_id = ?",
		packID, userID,
	).Scan(&packType, &isOpened)

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "pack not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if isOpened {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "pack already opened"})
		return
	}

	cards, err := generatePackCards(packType)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate cards"})
		return
	}

	_, err = db.Exec(
		"UPDATE card_packs SET is_opened = TRUE, opened_at = NOW() WHERE pack_id = ?",
		packID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to open pack"})
		return
	}

	for _, c := range cards {
		_, err = db.Exec(
			"INSERT INTO player_cards (player_id, card_id, level, quantity) VALUES (?, ?, 1, 1) "+
				"ON DUPLICATE KEY UPDATE quantity = quantity + 1",
			userID, c.CardID,
		)
		if err != nil {
			continue
		}
	}

	respondJSON(w, http.StatusOK, OpenPackResponse{
		PackID:   packID,
		PackType: packType,
		Cards:    cards,
	})
}

// Common 79%, Rare 15%, Epic 5%, Legendary 1%
func rollPackType() string {
	roll := rand.Intn(100)
	switch {
	case roll < 79:
		return "common"
	case roll < 94:
		return "rare"
	case roll < 99:
		return "epic"
	default:
		return "legendary"
	}
}

func generatePackCards(packType string) ([]PackCard, error) {
	type rarityRange struct {
		rarity string
		min    int
		max    int
	}

	var totalCards int
	var distribution []rarityRange

	switch packType {
	case "common":
		totalCards = 10
		distribution = []rarityRange{
			{"common", 8, 9},
			{"rare", 1, 2},
		}
	case "rare":
		totalCards = 20
		distribution = []rarityRange{
			{"common", 15, 17},
			{"rare", 2, 4},
			{"epic", 0, 1},
		}
	case "epic":
		totalCards = 40
		distribution = []rarityRange{
			{"common", 20, 25},
			{"rare", 5, 10},
			{"epic", 1, 2},
			{"legendary", 0, 1},
		}
	case "legendary":
		totalCards = 50
		distribution = []rarityRange{
			{"common", 25, 30},
			{"rare", 14, 20},
			{"epic", 3, 6},
			{"legendary", 1, 1},
		}
	default:
		return nil, fmt.Errorf("unknown pack type: %s", packType)
	}

	var result []PackCard

	for _, d := range distribution {
		count := d.min
		if d.max > d.min {
			count = d.min + rand.Intn(d.max-d.min+1)
		}
		if count == 0 {
			continue
		}

		rows, err := db.Query(
			"SELECT card_id, card_name, rarity FROM cards WHERE rarity = ? ORDER BY RAND() LIMIT ?",
			d.rarity, count,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var c PackCard
			if err := rows.Scan(&c.CardID, &c.CardName, &c.Rarity); err != nil {
				continue
			}
			result = append(result, c)
		}
		rows.Close()
	}

	if len(result) < totalCards {
		remaining := totalCards - len(result)
		rows, err := db.Query(
			"SELECT card_id, card_name, rarity FROM cards WHERE rarity = 'common' ORDER BY RAND() LIMIT ?",
			remaining,
		)
		if err == nil {
			for rows.Next() {
				var c PackCard
				if err := rows.Scan(&c.CardID, &c.CardName, &c.Rarity); err != nil {
					continue
				}
				result = append(result, c)
			}
			rows.Close()
		}
	}

	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})

	return result, nil
}

// GivePackToPlayer creates a new pack for a player (call after a battle ends).
func GivePackToPlayer(playerID int64) (int64, string, error) {
	packType := rollPackType()
	result, err := db.Exec(
		"INSERT INTO card_packs (player_id, pack_type) VALUES (?, ?)",
		playerID, packType,
	)
	if err != nil {
		return 0, "", err
	}
	packID, _ := result.LastInsertId()
	return packID, packType, nil
}
