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
	PackID          int64          `json:"pack_id"`
	PackType        string         `json:"pack_type"`
	Cards           []PackCardItem `json:"cards"`
	CrystalsAwarded int32          `json:"crystals_awarded"`
	CrystalsTotal   int32          `json:"crystals_total"`
}

// crystalsPerPack maps pack type → crystals awarded on open.
var crystalsPerPack = map[string]int32{
	"common":    100,
	"rare":      500,
	"epic":      1000,
	"legendary": 5000,
}

// PackCard is the raw card entry from generatePackCards (may contain repeats).
type PackCard struct {
	CardID   int64  `json:"card_id"`
	CardName string `json:"card_name"`
	Rarity   string `json:"rarity"`
}

// PackCardItem is the client-facing version returned in the open-pack
// response. Duplicates are collapsed into a single entry with a quantity.
type PackCardItem struct {
	CardID   int64  `json:"card_id"`
	CardName string `json:"card_name"`
	Rarity   string `json:"rarity"`
	Quantity int    `json:"quantity"`
}

// collapsePackCards groups duplicate PackCard entries by card_id, preserving
// order of first appearance.
func collapsePackCards(cards []PackCard) []PackCardItem {
	type entry struct {
		idx  int
		item PackCardItem
	}
	seen := make(map[int64]*entry, len(cards))
	var ordered []*entry
	for _, c := range cards {
		if e, ok := seen[c.CardID]; ok {
			e.item.Quantity++
		} else {
			e = &entry{idx: len(ordered), item: PackCardItem{
				CardID:   c.CardID,
				CardName: c.CardName,
				Rarity:   c.Rarity,
				Quantity: 1,
			}}
			seen[c.CardID] = e
			ordered = append(ordered, e)
		}
	}
	result := make([]PackCardItem, len(ordered))
	for _, e := range ordered {
		result[e.idx] = e.item
	}
	return result
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

	crystals := crystalsPerPack[pack.PackType]
	if crystals > 0 {
		if err := queries.AddPlayerCrystals(ctx, db.AddPlayerCrystalsParams{
			Crystals: crystals,
			ID:       userID,
		}); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to award crystals"})
			return
		}
	}

	total, err := queries.GetPlayerCrystals(ctx, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load crystals"})
		return
	}

	respondJSON(w, http.StatusOK, OpenPackResponse{
		PackID:          packID,
		PackType:        pack.PackType,
		Cards:           collapsePackCards(cards),
		CrystalsAwarded: crystals,
		CrystalsTotal:   total,
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

const maxUnopenedPacks = 4

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

	count, err := queries.CountUnopenedPacks(r.Context(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if count >= maxUnopenedPacks {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("cannot have more than %d unopened packs", maxUnopenedPacks),
		})
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
