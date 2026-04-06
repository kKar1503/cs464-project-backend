package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	db "github.com/kKar1503/cs464-backend/db/sqlc"
)

// cardRarityCache caches card_id -> rarity mappings in memory.
// Invalidate by calling cardRarityCache.Invalidate() when the cards table changes.
var cardRarityCache = &rarityCache{}

type rarityCache struct {
	mu       sync.RWMutex
	rarities map[int32]string
}

func (rc *rarityCache) Get(ctx context.Context) (map[int32]string, error) {
	rc.mu.RLock()
	if rc.rarities != nil {
		defer rc.mu.RUnlock()
		return rc.rarities, nil
	}
	rc.mu.RUnlock()

	rc.mu.Lock()
	defer rc.mu.Unlock()
	// Double-check after acquiring write lock
	if rc.rarities != nil {
		return rc.rarities, nil
	}

	allCards, err := queries.GetAllCards(ctx)
	if err != nil {
		return nil, err
	}
	rc.rarities = make(map[int32]string, len(allCards))
	for _, c := range allCards {
		rc.rarities[c.CardID] = c.Rarity
	}
	log.Printf("Card rarity cache loaded: %d cards", len(rc.rarities))
	return rc.rarities, nil
}

func (rc *rarityCache) Invalidate() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.rarities = nil
	log.Println("Card rarity cache invalidated")
}

// req/res types for deck apis
type CreateDeckRequest struct {
    Name    string  `json:"name"`
    CardIDs []int64 `json:"card_ids"`
}
type UpdateDeckRequest struct {
    DeckID   int64   `json:"deck_id"`
    Name     string  `json:"name"`
    CardIDs  []int64 `json:"card_ids"`
}
type DeckCard struct {
    CardID   int64 `json:"card_id"`
    Position int   `json:"position"`
}
type DeckResponse struct {
    DeckID    int64      `json:"deck_id"`
    Name      string     `json:"name"`
    CardIDs   []int64    `json:"card_ids"`
    Cards     []DeckCard `json:"cards"`
    CreatedAt string     `json:"created_at"`
}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

const (
	maxDeckSize        = 12
	maxCopiesPerCard   = 2
	maxLegendaryCards  = 1
)

// validateDeckCards checks deck composition rules:
// - at most 12 cards
// - at most 2 copies of each card
// - at most 1 legendary card
// - player must own each card with sufficient quantity
func validateDeckCards(ctx context.Context, userID int64, cardIDs []int64) error {
	if len(cardIDs) > maxDeckSize {
		return fmt.Errorf("deck cannot have more than %d cards", maxDeckSize)
	}
	counts := make(map[int64]int)
	for _, id := range cardIDs {
		counts[id]++
		if counts[id] > maxCopiesPerCard {
			return fmt.Errorf("cannot have more than %d copies of card %d", maxCopiesPerCard, id)
		}
	}

	// Check legendary limit using cached card rarities
	rarityMap, err := cardRarityCache.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to validate card rarities")
	}

	legendaryCount := 0
	for _, id := range cardIDs {
		if rarityMap[int32(id)] == "legendary" {
			legendaryCount++
			if legendaryCount > maxLegendaryCards {
				return fmt.Errorf("deck cannot have more than %d legendary card", maxLegendaryCards)
			}
		}
	}

	// Check player owns each card with sufficient quantity
	owned, err := queries.GetPlayerCardOwnership(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check card ownership")
	}
	ownershipMap := make(map[int32]int32, len(owned))
	for _, o := range owned {
		ownershipMap[o.CardID] = o.Quantity
	}
	for cardID, needed := range counts {
		available, ok := ownershipMap[int32(cardID)]
		if !ok {
			return fmt.Errorf("you do not own card %d", cardID)
		}
		if int32(needed) > available {
			return fmt.Errorf("you only own %d copies of card %d but need %d", available, cardID, needed)
		}
	}

	return nil
}


func handleCreateDeck(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var req CreateDeckRequest
	//validate req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if len(req.CardIDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one card is required"})
		return
	}
	if err := validateDeckCards(r.Context(), userID, req.CardIDs); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	ctx := r.Context()

	result, err := queries.CreateDeck(ctx, db.CreateDeckParams{
		PlayerID: userID,
		Name:     req.Name,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create deck"})
		return
	}

	deckID, err := result.LastInsertId()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get deck id"})
		return
	}

	for i, cardID := range req.CardIDs {
		err := queries.InsertDeckCard(ctx, db.InsertDeckCardParams{
			DeckID:   int32(deckID),
			CardID:   int32(cardID),
			Position: int32(i),
		})
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
			return
		}
	}

	respondJSON(w, http.StatusCreated, DeckResponse{
		DeckID:    deckID,
		Name:      req.Name,
		CardIDs:   req.CardIDs,
		Cards:     []DeckCard{},
		CreatedAt: time.Now().Format(time.RFC3339),
	})
}

func handleGetDeckByID(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var deckID int64
	if _, err := fmt.Sscanf(r.PathValue("id"), "%d", &deckID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid deck id"})
		return
	}

	ctx := r.Context()

	deck, err := queries.GetDeckByIDAndPlayer(ctx, db.GetDeckByIDAndPlayerParams{
		DeckID:   int32(deckID),
		PlayerID: userID,
	})
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "deck not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	deckCards, err := queries.GetDeckCards(ctx, int32(deckID))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var cards []DeckCard
	var cardIDs []int64
	for _, c := range deckCards {
		cards = append(cards, DeckCard{CardID: int64(c.CardID), Position: int(c.Position)})
		cardIDs = append(cardIDs, int64(c.CardID))
	}

	if cards == nil {
		cards = []DeckCard{}
		cardIDs = []int64{}
	}

	respondJSON(w, http.StatusOK, DeckResponse{
		DeckID:    int64(deck.DeckID),
		Name:      deck.Name,
		CardIDs:   cardIDs,
		Cards:     cards,
		CreatedAt: deck.CreatedAt.Format(time.RFC3339),
	})
}

func handleUpdateDeck(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var deckID int64
	if _, err := fmt.Sscanf(r.PathValue("id"), "%d", &deckID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid deck id"})
		return
	}

	var req UpdateDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(req.CardIDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one card is required"})
		return
	}
	if err := validateDeckCards(r.Context(), userID, req.CardIDs); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	ctx := r.Context()

	// Verify deck ownership first — avoids false "not found" from RowsAffected
	// when name/card_id are unchanged but deck_cards still need updating.
	existing, err := queries.GetDeckByIDAndPlayer(ctx, db.GetDeckByIDAndPlayerParams{
		DeckID:   int32(req.DeckID),
		PlayerID: userID,
	})
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "deck not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	if _, err := queries.UpdateDeck(ctx, db.UpdateDeckParams{
		Name:     req.Name,
		DeckID:   int32(deckID),
		PlayerID: userID,
	}); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update deck"})
		return
	}

	queries.DeleteDeckCards(ctx, int32(deckID))
	for i, cardID := range req.CardIDs {
		queries.InsertDeckCard(ctx, db.InsertDeckCardParams{
			DeckID:   int32(deckID),
			CardID:   int32(cardID),
			Position: int32(i),
		})
	}

	respondJSON(w, http.StatusOK, DeckResponse{
		DeckID:    deckID,
		Name:      req.Name,
		CardIDs:   req.CardIDs,
		Cards:     []DeckCard{},
		CreatedAt: existing.CreatedAt.Format(time.RFC3339),
	})
}

func handleDeleteDeck(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var deckID int64
	if _, err := fmt.Sscanf(r.PathValue("id"), "%d", &deckID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid deck id"})
		return
	}

	result, err := queries.DeleteDeck(r.Context(), db.DeleteDeckParams{
		DeckID:   int32(deckID),
		PlayerID: userID,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete deck"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "deck not found"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "deck deleted"})
}


type UpdateAllDecksRequest struct {
	Decks []UpdateDeckRequest `json:"decks"`
}

func handleUpdateAllDecks(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var req UpdateAllDecksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(req.Decks) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one deck is required"})
		return
	}

	ctx := r.Context()

	updated := make([]DeckResponse, 0, len(req.Decks))
	for _, d := range req.Decks {
		if d.DeckID == 0 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "deck_id is required for each deck"})
			return
		}
		if err := validateDeckCards(ctx, userID, d.CardIDs); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		// Verify ownership before updating to avoid false "not found" from RowsAffected.
		existing, err := queries.GetDeckByIDAndPlayer(ctx, db.GetDeckByIDAndPlayerParams{
			DeckID:   int32(d.DeckID),
			PlayerID: userID,
		})
		if err == sql.ErrNoRows {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("deck %d not found", d.DeckID)})
			return
		}
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
			return
		}

		if _, err := queries.UpdateDeck(ctx, db.UpdateDeckParams{
			Name:     d.Name,
			DeckID:   int32(d.DeckID),
			PlayerID: userID,
		}); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update deck"})
			return
		}

		queries.DeleteDeckCards(ctx, int32(d.DeckID))
		for i, cardID := range d.CardIDs {
			queries.InsertDeckCard(ctx, db.InsertDeckCardParams{
				DeckID:   int32(d.DeckID),
				CardID:   int32(cardID),
				Position: int32(i),
			})
		}

		cards := make([]DeckCard, 0, len(d.CardIDs))
		cardIDs := make([]int64, 0, len(d.CardIDs))
		for i, id := range d.CardIDs {
			cards = append(cards, DeckCard{CardID: id, Position: i})
			cardIDs = append(cardIDs, id)
		}

		updated = append(updated, DeckResponse{
			DeckID:    d.DeckID,
			Name:      d.Name,
			CardIDs:   cardIDs,
			Cards:     cards,
			CreatedAt: existing.CreatedAt.Format(time.RFC3339),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"decks": updated, "count": len(updated)})
}

func handleGetAllDecks(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	ctx := r.Context()

	deckList, err := queries.GetPlayerDeckList(ctx, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get decks"})
		return
	}

	decks := make([]DeckResponse, 0, len(deckList))
	for _, d := range deckList {
		deckCards, err := queries.GetDeckCards(ctx, d.DeckID)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get deck cards"})
			return
		}

		cards := make([]DeckCard, 0, len(deckCards))
		cardIDs := make([]int64, 0, len(deckCards))
		for _, c := range deckCards {
			cards = append(cards, DeckCard{CardID: int64(c.CardID), Position: int(c.Position)})
			cardIDs = append(cardIDs, int64(c.CardID))
		}

		decks = append(decks, DeckResponse{
			DeckID:    int64(d.DeckID),
			Name:      d.Name,
			CardIDs:   cardIDs,
			Cards:     cards,
			CreatedAt: d.CreatedAt.Format(time.RFC3339),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"decks": decks, "count": len(decks)})
}

type CardResponse struct {
	CardID      int    `json:"card_id"`
	CardName    string `json:"card_name"`
	Affiliation int    `json:"affiliation"`
	Rarity      string `json:"rarity"`
	ManaCost    int    `json:"mana_cost"`
	Description string `json:"description"`
	IconURL     string `json:"icon_url"`
}

func handleGetAllCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rarity := r.URL.Query().Get("rarity")
	affiliation := r.URL.Query().Get("affiliation")

	var dbCards []db.Card
	var err error

	switch {
	case rarity != "" && affiliation != "":
		var aff int64
		fmt.Sscanf(affiliation, "%d", &aff)
		dbCards, err = queries.GetAllCardsByRarityAndAffiliation(ctx, db.GetAllCardsByRarityAndAffiliationParams{
			Rarity:      rarity,
			Affiliation: int32(aff),
		})
	case rarity != "":
		dbCards, err = queries.GetAllCardsByRarity(ctx, rarity)
	case affiliation != "":
		var aff int64
		fmt.Sscanf(affiliation, "%d", &aff)
		dbCards, err = queries.GetAllCardsByAffiliation(ctx, int32(aff))
	default:
		dbCards, err = queries.GetAllCards(ctx)
	}

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get cards"})
		return
	}

	cards := make([]CardResponse, 0, len(dbCards))
	for _, c := range dbCards {
		cards = append(cards, CardResponse{
			CardID:      int(c.CardID),
			CardName:    c.CardName,
			Affiliation: int(c.Affiliation),
			Rarity:      c.Rarity,
			ManaCost:    int(c.ManaCost),
			Description: c.Description,
			IconURL:     c.IconUrl,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"cards": cards, "count": len(cards)})
}

type PlayerCardResponse struct {
	CardID      int    `json:"card_id"`
	CardName    string `json:"card_name"`
	Affiliation int    `json:"affiliation"`
	Rarity      string `json:"rarity"`
	ManaCost    int    `json:"mana_cost"`
	Description string `json:"description"`
	IconURL     string `json:"icon_url"`
	Level       int    `json:"level"`
	Quantity    int    `json:"quantity"`
}

func handleGetPlayerCards(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	dbCards, err := queries.GetPlayerCards(r.Context(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get player cards"})
		return
	}

	cards := make([]PlayerCardResponse, 0, len(dbCards))
	for _, c := range dbCards {
		cards = append(cards, PlayerCardResponse{
			CardID:      int(c.CardID),
			CardName:    c.CardName,
			Affiliation: int(c.Affiliation),
			Rarity:      c.Rarity,
			ManaCost:    int(c.ManaCost),
			Description: c.Description,
			IconURL:     c.IconUrl,
			Level:       int(c.Level),
			Quantity:    int(c.Quantity),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"cards": cards, "count": len(cards)})
}

type DeckCardsNotInDeck struct {
	DeckID   int64                `json:"deck_id"`
	DeckName string               `json:"deck_name"`
	Cards    []PlayerCardResponse `json:"cards_not_in_deck"`
}

func handleGetCardsNotInDecks(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	ctx := r.Context()

	// Fetch all decks belonging to the user
	decks, err := queries.GetPlayerDeckList(ctx, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get decks"})
		return
	}

	result := make([]DeckCardsNotInDeck, 0, len(decks))
	for _, d := range decks {
		dbCards, err := queries.GetPlayerCardsNotInDeck(ctx, db.GetPlayerCardsNotInDeckParams{
			PlayerID: userID,
			DeckID:   d.DeckID,
		})
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get cards for deck"})
			return
		}

		cards := make([]PlayerCardResponse, 0, len(dbCards))
		for _, c := range dbCards {
			cards = append(cards, PlayerCardResponse{
				CardID:      int(c.CardID),
				CardName:    c.CardName,
				Affiliation: int(c.Affiliation),
				Rarity:      c.Rarity,
				ManaCost:    int(c.ManaCost),
				Description: c.Description,
				IconURL:     c.IconUrl,
				Level:       int(c.Level),
				Quantity:    int(c.Quantity),
			})
		}

		result = append(result, DeckCardsNotInDeck{
			DeckID:   int64(d.DeckID),
			DeckName: d.Name,
			Cards:    cards,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"decks": result})
}

type SetActiveDeckRequest struct {
	DeckID int64 `json:"deck_id"`
}

func handleSetActiveDeck(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var req SetActiveDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	ctx := r.Context()

	// Verify the deck belongs to this player
	_, err = queries.GetDeckByIDAndPlayer(ctx, db.GetDeckByIDAndPlayerParams{
		DeckID:   int32(req.DeckID),
		PlayerID: userID,
	})
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "deck not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	activeDeckID := int32(req.DeckID)
	if err := queries.SetActiveDeck(ctx, db.SetActiveDeckParams{
		ActiveDeckID: &activeDeckID,
		ID:           userID,
	}); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to set active deck"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "active deck set"})
}

func handleGetActiveDeck(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserFromToken(r)
	if err != nil {
		if err == errUnauthorized {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	activeDeckID, err := queries.GetActiveDeck(r.Context(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	if activeDeckID == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"active_deck_id": nil})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"active_deck_id": *activeDeckID})
}

// Internal endpoint — called by auth service after user registration
type InitStarterContentRequest struct {
	UserID int64 `json:"user_id"`
}

// Starter cards: Pig, Farmer, Barbarian, Dwarf, Penguin, Apprentice Magician
var starterCardIDs = []int32{1, 2, 7, 8, 13, 14}

func handleInitStarterContent(w http.ResponseWriter, r *http.Request) {
	var req InitStarterContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.UserID == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}

	ctx := r.Context()

	// Give 2 copies of each starter card
	for _, cardID := range starterCardIDs {
		queries.UpsertPlayerCard(ctx, db.UpsertPlayerCardParams{
			PlayerID: req.UserID,
			CardID:   cardID,
		})
		// Second copy
		queries.UpsertPlayerCard(ctx, db.UpsertPlayerCardParams{
			PlayerID: req.UserID,
			CardID:   cardID,
		})
	}

	// Create 3 starter decks
	deckCards := []int32{1, 1, 2, 2, 7, 7, 8, 8, 13, 13, 14, 14}
	var firstDeckID int32
	for i := 1; i <= 3; i++ {
		result, err := queries.CreateDeck(ctx, db.CreateDeckParams{
			PlayerID: req.UserID,
			Name:     fmt.Sprintf("Starter Deck %d", i),
		})
		if err != nil {
			continue
		}
		deckID, _ := result.LastInsertId()
		if i == 1 {
			firstDeckID = int32(deckID)
		}
		for pos, cardID := range deckCards {
			queries.InsertDeckCard(ctx, db.InsertDeckCardParams{
				DeckID:   int32(deckID),
				CardID:   cardID,
				Position: int32(pos),
			})
		}
	}

	// Set first deck as active
	if firstDeckID > 0 {
		queries.SetActiveDeck(ctx, db.SetActiveDeckParams{
			ActiveDeckID: &firstDeckID,
			ID:           req.UserID,
		})
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message":        "starter content created",
		"active_deck_id": firstDeckID,
	})
}

// affiliationToColour maps affiliation IDs to colour names.
var affiliationToColour = map[int32]string{
	1: "Grey",
	2: "Red",
	3: "Blue",
	4: "Green",
	5: "Purple",
	6: "Yellow",
	7: "Colourless",
}

type AbilityForGame struct {
	TriggerType string          `json:"trigger_type"`
	EffectType  string          `json:"effect_type"`
	Params      json.RawMessage `json:"params"`
}

type DeckCardForGame struct {
	CardID    int              `json:"card_id"`
	CardName  string           `json:"card_name"`
	Colour    string           `json:"colour"`
	Rarity    string           `json:"rarity"`
	ManaCost  int              `json:"mana_cost"`
	Attack    int              `json:"attack"`
	HP        int              `json:"hp"`
	Position  int              `json:"position"`
	Abilities []AbilityForGame `json:"abilities"`
}

// Internal endpoint — returns the full active deck for a player (used by gameplay service)
func handleGetActiveDeckForGame(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}

	var userID int64
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	ctx := r.Context()

	activeDeckID, err := queries.GetActiveDeck(ctx, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get active deck"})
		return
	}
	if activeDeckID == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "no active deck set"})
		return
	}

	cards, err := queries.GetDeckCardsWithDetails(ctx, *activeDeckID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load deck cards"})
		return
	}

	// Fetch abilities for all cards in this deck
	abilities, err := queries.GetAbilitiesForDeck(ctx, *activeDeckID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load card abilities"})
		return
	}

	// Group abilities by card_id
	abilityMap := make(map[int32][]AbilityForGame)
	for _, a := range abilities {
		abilityMap[a.CardID] = append(abilityMap[a.CardID], AbilityForGame{
			TriggerType: a.TriggerType,
			EffectType:  a.EffectType,
			Params:      a.Params,
		})
	}

	result := make([]DeckCardForGame, 0, len(cards))
	for _, c := range cards {
		colour := affiliationToColour[c.Affiliation]
		if colour == "" {
			colour = "Grey"
		}
		cardAbilities := abilityMap[c.CardID]
		if cardAbilities == nil {
			cardAbilities = []AbilityForGame{}
		}
		result = append(result, DeckCardForGame{
			CardID:    int(c.CardID),
			CardName:  c.CardName,
			Colour:    colour,
			Rarity:    c.Rarity,
			ManaCost:  int(c.ManaCost),
			Attack:    int(c.Attack),
			HP:        int(c.Hp),
			Position:  int(c.Position),
			Abilities: cardAbilities,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deck_id": *activeDeckID,
		"cards":   result,
	})
}

// Internal endpoint — returns ALL card definitions with stats and abilities (used by gameplay service at startup)
func handleGetAllCardDefinitions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cards, err := queries.GetAllCardDefinitions(ctx)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load card definitions"})
		return
	}

	abilities, err := queries.GetAllCardAbilities(ctx)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load card abilities"})
		return
	}

	// Group abilities by card_id
	abilityMap := make(map[int32][]AbilityForGame)
	for _, a := range abilities {
		abilityMap[a.CardID] = append(abilityMap[a.CardID], AbilityForGame{
			TriggerType: a.TriggerType,
			EffectType:  a.EffectType,
			Params:      a.Params,
		})
	}

	type CardDefinitionResponse struct {
		CardID    int              `json:"card_id"`
		CardName  string           `json:"card_name"`
		Colour    string           `json:"colour"`
		Rarity    string           `json:"rarity"`
		ManaCost  int              `json:"mana_cost"`
		Attack    int              `json:"attack"`
		HP        int              `json:"hp"`
		Abilities []AbilityForGame `json:"abilities"`
	}

	result := make([]CardDefinitionResponse, 0, len(cards))
	for _, c := range cards {
		colour := affiliationToColour[c.Affiliation]
		if colour == "" {
			colour = "Grey"
		}
		cardAbilities := abilityMap[c.CardID]
		if cardAbilities == nil {
			cardAbilities = []AbilityForGame{}
		}
		result = append(result, CardDefinitionResponse{
			CardID:    int(c.CardID),
			CardName:  c.CardName,
			Colour:    colour,
			Rarity:    c.Rarity,
			ManaCost:  int(c.ManaCost),
			Attack:    int(c.Attack),
			HP:        int(c.Hp),
			Abilities: cardAbilities,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"cards": result,
		"count": len(result),
	})
}