package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"

	db "github.com/kKar1503/cs464-backend/db/sqlc"
)

// LevelUpCost returns the cards and crystals required to advance a card from
// `currentLevel` to `currentLevel + 1`.
//   cards    = 4 ^ currentLevel   (4, 16, 64, 256, ...)
//   crystals = 100 * 10^(level-1) (100, 1000, 10000, 100000, ...)
// The crystals curve matches the worked examples in the design doc
// (level 1 → 100, level 2 → 1000, level 3 → 10000). Copies consumed do not
// include the card itself — `quantity` represents spare duplicates only.
func LevelUpCost(currentLevel int) (cards int32, crystals int32) {
	if currentLevel < 1 {
		currentLevel = 1
	}
	return int32(math.Pow(4, float64(currentLevel))), int32(100 * math.Pow(10, float64(currentLevel-1)))
}

// ScaleStat applies the per-level multiplier to a base stat.
// Formula: ceil((1 + 0.2 * (level - 1)) * base)
func ScaleStat(base int, level int) int {
	if level < 1 {
		level = 1
	}
	return int(math.Ceil((1 + 0.2*float64(level-1)) * float64(base)))
}

// disenchantValuePerRarity is the crystals refunded for disenchanting one
// extra copy of a maxed card, keyed by card rarity. Calibrated to roughly
// 10% of the matching pack's crystal payout so that farming packs for
// dupes is still worthwhile but not the optimal crystal source.
var disenchantValuePerRarity = map[string]int32{
	"common":    10,
	"rare":      50,
	"epic":      100,
	"legendary": 500,
}

// DisenchantValue returns the crystals earned per copy for a given rarity.
// Unknown rarities return 0 (caller should treat as invalid).
func DisenchantValue(rarity string) int32 {
	return disenchantValuePerRarity[rarity]
}

// PrunedDeck describes a single deck that had copies removed as a side effect
// of level-up or disenchant. Returned to the client so the UI can surface it.
type PrunedDeck struct {
	DeckID   int32  `json:"deck_id"`
	DeckName string `json:"deck_name"`
	Removed  int32  `json:"removed"`
}

// pruneDecksForPlayerCard trims deck_cards rows so that no deck references
// more copies of `cardID` than the player's remaining `newQuantity`. Only
// excess slots are removed — decks at or below the cap are left untouched.
//
// Returns the list of affected decks. Errors from individual deletes are
// logged to stderr and the walk continues so the primary level-up /
// disenchant operation isn't rolled back over a best-effort cleanup.
func pruneDecksForPlayerCard(
	ctx context.Context,
	playerID int64,
	cardID int32,
	newQuantity int32,
) ([]PrunedDeck, error) {
	usage, err := queries.GetDeckUsageForPlayerCard(ctx, db.GetDeckUsageForPlayerCardParams{
		CardID:   cardID,
		PlayerID: playerID,
	})
	if err != nil {
		return nil, err
	}

	var pruned []PrunedDeck
	for _, u := range usage {
		excess := int32(u.CopiesInDeck) - newQuantity
		if excess <= 0 {
			continue
		}
		res, err := queries.PruneDeckCardsForCard(ctx, db.PruneDeckCardsForCardParams{
			DeckID: u.DeckID,
			CardID: cardID,
			Limit:  excess,
		})
		if err != nil {
			log.Printf("prune deck %d card %d: %v", u.DeckID, cardID, err)
			continue
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			continue
		}
		pruned = append(pruned, PrunedDeck{
			DeckID:   u.DeckID,
			DeckName: u.Name,
			Removed:  int32(n),
		})
	}
	return pruned, nil
}

type CrystalsResponse struct {
	Crystals int32 `json:"crystals"`
}

func handleGetCrystals(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	crystals, err := queries.GetPlayerCrystals(r.Context(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get crystals"})
		return
	}

	respondJSON(w, http.StatusOK, CrystalsResponse{Crystals: crystals})
}

type LevelUpCardResponse struct {
	CardID        int32        `json:"card_id"`
	NewLevel      int32        `json:"new_level"`
	CardsConsumed int32        `json:"cards_consumed"`
	CrystalsSpent int32        `json:"crystals_spent"`
	CrystalsLeft  int32        `json:"crystals_left"`
	QuantityLeft  int32        `json:"quantity_left"`
	PrunedDecks   []PrunedDeck `json:"pruned_decks"`
}

func handleLevelUpCard(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var cardID int64
	if _, err := fmt.Sscanf(r.PathValue("cardId"), "%d", &cardID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid card id"})
		return
	}

	ctx := r.Context()

	pc, err := queries.GetPlayerCard(ctx, db.GetPlayerCardParams{
		PlayerID: userID,
		CardID:   int32(cardID),
	})
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "you do not own this card"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	if pc.Level >= pc.MaxLevel {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "card is already at max level"})
		return
	}

	cardsCost, crystalsCost := LevelUpCost(int(pc.Level))

	if pc.Quantity < cardsCost {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("need %d copies, have %d", cardsCost, pc.Quantity),
		})
		return
	}

	crystals, err := queries.GetPlayerCrystals(ctx, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read crystals"})
		return
	}
	if crystals < crystalsCost {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("need %d crystals, have %d", crystalsCost, crystals),
		})
		return
	}

	// Deduct crystals first. The WHERE clause makes this a safe atomic check:
	// if the row wasn't updated, the user's balance changed between the read
	// above and here (concurrent pack open / level up). Bail out.
	res, err := queries.DeductPlayerCrystals(ctx, db.DeductPlayerCrystalsParams{
		Crystals:   crystalsCost,
		ID:         userID,
		Crystals_2: crystalsCost,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to deduct crystals"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "crystal balance changed, please retry"})
		return
	}

	// Consume cards and advance level in a single atomic UPDATE gated on the
	// current level (optimistic concurrency — if a concurrent request already
	// leveled the card, this one fails and we refund.)
	res, err = queries.LevelUpPlayerCard(ctx, db.LevelUpPlayerCardParams{
		Quantity:   cardsCost,
		PlayerID:   userID,
		CardID:     int32(cardID),
		Level:      pc.Level,
		Quantity_2: cardsCost,
	})
	if err != nil {
		// Refund crystals on failure.
		queries.AddPlayerCrystals(ctx, db.AddPlayerCrystalsParams{Crystals: crystalsCost, ID: userID})
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to level up card"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		queries.AddPlayerCrystals(ctx, db.AddPlayerCrystalsParams{Crystals: crystalsCost, ID: userID})
		respondJSON(w, http.StatusConflict, map[string]string{"error": "card state changed, please retry"})
		return
	}

	// Level-up consumed copies from the collection. If any of the player's
	// decks referenced more copies than they now own, quietly remove the
	// excess so the deck doesn't become unsavable. The affected decks are
	// returned so the client can show a toast.
	newQuantity := pc.Quantity - cardsCost
	pruned, err := pruneDecksForPlayerCard(r.Context(), userID, int32(cardID), newQuantity)
	if err != nil {
		log.Printf("level-up prune failed for player %d card %d: %v", userID, cardID, err)
	}

	respondJSON(w, http.StatusOK, LevelUpCardResponse{
		CardID:        int32(cardID),
		NewLevel:      pc.Level + 1,
		CardsConsumed: cardsCost,
		CrystalsSpent: crystalsCost,
		CrystalsLeft:  crystals - crystalsCost,
		QuantityLeft:  newQuantity,
		PrunedDecks:   pruned,
	})
}

type DisenchantRequest struct {
	Quantity int32 `json:"quantity"`
}

type DisenchantResponse struct {
	CardID          int32        `json:"card_id"`
	Disenchanted    int32        `json:"disenchanted"`
	CrystalsAwarded int32        `json:"crystals_awarded"`
	CrystalsTotal   int32        `json:"crystals_total"`
	QuantityLeft    int32        `json:"quantity_left"`
	PrunedDecks     []PrunedDeck `json:"pruned_decks"`
}

// handleDisenchantCard converts extra copies of a maxed card into crystals.
// The card must be at its max level, and at least one copy must remain after
// the operation so the player keeps ownership.
func handleDisenchantCard(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var cardID int64
	if _, err := fmt.Sscanf(r.PathValue("cardId"), "%d", &cardID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid card id"})
		return
	}

	var req DisenchantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Quantity <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "quantity must be positive"})
		return
	}

	ctx := r.Context()

	pc, err := queries.GetPlayerCard(ctx, db.GetPlayerCardParams{
		PlayerID: userID,
		CardID:   int32(cardID),
	})
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "you do not own this card"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	if pc.Level < pc.MaxLevel {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "card must be max level before disenchanting"})
		return
	}
	// Keep at least one copy so the player still owns the card.
	if pc.Quantity-req.Quantity < 1 {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("can disenchant at most %d copies (need to keep 1)", pc.Quantity-1),
		})
		return
	}

	perCopy := DisenchantValue(pc.Rarity)
	if perCopy == 0 {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "unknown card rarity"})
		return
	}
	crystalsAwarded := perCopy * req.Quantity

	// Atomic decrement gated on max_level and minimum remaining quantity
	// (min_quantity = quantity_to_remove + 1, keeping one copy behind).
	res, err := queries.DisenchantPlayerCard(ctx, db.DisenchantPlayerCardParams{
		Quantity:   req.Quantity,
		PlayerID:   userID,
		CardID:     int32(cardID),
		Quantity_2: req.Quantity + 1,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to disenchant"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "card state changed, please retry"})
		return
	}

	if err := queries.AddPlayerCrystals(ctx, db.AddPlayerCrystalsParams{
		Crystals: crystalsAwarded,
		ID:       userID,
	}); err != nil {
		// Best-effort rollback of the copies.
		_, _ = queries.DisenchantPlayerCard(ctx, db.DisenchantPlayerCardParams{
			Quantity:   -req.Quantity,
			PlayerID:   userID,
			CardID:     int32(cardID),
			Quantity_2: 0,
		})
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to award crystals"})
		return
	}

	total, err := queries.GetPlayerCrystals(ctx, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read crystals"})
		return
	}

	newQuantity := pc.Quantity - req.Quantity
	pruned, err := pruneDecksForPlayerCard(ctx, userID, int32(cardID), newQuantity)
	if err != nil {
		log.Printf("disenchant prune failed for player %d card %d: %v", userID, cardID, err)
	}

	respondJSON(w, http.StatusOK, DisenchantResponse{
		CardID:          int32(cardID),
		Disenchanted:    req.Quantity,
		CrystalsAwarded: crystalsAwarded,
		CrystalsTotal:   total,
		QuantityLeft:    newQuantity,
		PrunedDecks:     pruned,
	})
}
