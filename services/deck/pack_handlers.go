package main

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"

	db "github.com/kKar1503/cs464-backend/db/sqlc"
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
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	dbPacks, err := queries.GetPlayerPacks(r.Context(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get packs"})
		return
	}

	packs := make([]PackResponse, 0, len(dbPacks))
	for _, p := range dbPacks {
		packs = append(packs, PackResponse{
			PackID:    int64(p.PackID),
			PackType:  p.PackType,
			IsOpened:  p.IsOpened,
			CreatedAt: p.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"packs": packs})
}

func handleOpenPack(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()

	pack, err := queries.GetPackByIDAndPlayer(ctx, db.GetPackByIDAndPlayerParams{
		PackID:   int32(packID),
		PlayerID: userID,
	})
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "pack not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if pack.IsOpened {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "pack already opened"})
		return
	}

	cards, err := generatePackCards(ctx, pack.PackType)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate cards"})
		return
	}

	if err := queries.OpenPack(ctx, int32(packID)); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to open pack"})
		return
	}

	for _, c := range cards {
		queries.UpsertPlayerCard(ctx, db.UpsertPlayerCardParams{
			PlayerID: userID,
			CardID:   int32(c.CardID),
		})
	}

	respondJSON(w, http.StatusOK, OpenPackResponse{
		PackID:   packID,
		PackType: pack.PackType,
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

func generatePackCards(ctx context.Context, packType string) ([]PackCard, error) {
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

		dbCards, err := queries.GetRandomCardsByRarity(ctx, db.GetRandomCardsByRarityParams{
			Rarity: d.rarity,
			Limit:  int32(count),
		})
		if err != nil {
			return nil, err
		}
		for _, c := range dbCards {
			result = append(result, PackCard{CardID: int64(c.CardID), CardName: c.CardName, Rarity: c.Rarity})
		}
	}

	if len(result) < totalCards {
		remaining := totalCards - len(result)
		dbCards, err := queries.GetRandomCardsByRarity(ctx, db.GetRandomCardsByRarityParams{
			Rarity: "common",
			Limit:  int32(remaining),
		})
		if err == nil {
			for _, c := range dbCards {
				result = append(result, PackCard{CardID: int64(c.CardID), CardName: c.CardName, Rarity: c.Rarity})
			}
		}
	}

	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})

	return result, nil
}

// GivePackToPlayer creates a new pack for a player (call after a battle ends).
func GivePackToPlayer(ctx context.Context, playerID int64) (int64, string, error) {
	packType := rollPackType()
	result, err := queries.CreatePack(ctx, db.CreatePackParams{
		PlayerID: playerID,
		PackType: packType,
	})
	if err != nil {
		return 0, "", err
	}
	packID, _ := result.LastInsertId()
	return packID, packType, nil
}

func handleBuyPack(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	packID, packType, err := GivePackToPlayer(r.Context(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create pack"})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"pack_id":   packID,
		"pack_type": packType,
		"message":   "Pack acquired successfully",
	})
}
