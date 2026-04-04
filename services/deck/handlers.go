package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	db "github.com/kKar1503/cs464-backend/db/sqlc"
)

// req/res types for deck apis
type CreateDeckRequest struct {
    Name    string  `json:"name"`
    CardIDs []int64 `json:"card_ids"`
}
type UpdateDeckRequest struct {
    DeckID  int64   `json:"deck_id"`
    Name    string  `json:"name"`
    CardIDs []int64 `json:"card_ids"`
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


func handleCreateDeck(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()

	// decks table requires 1 card_id
	firstCardID := req.CardIDs[0]
	result, err := queries.CreateDeck(ctx, db.CreateDeckParams{
		PlayerID: userID,
		CardID:   int32(firstCardID),
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
		DeckID: deckID,
		Name: req.Name,
		CardIDs: req.CardIDs,
		Cards: []DeckCard{},
		CreatedAt: time.Now().Format(time.RFC3339),
	})
}

func handleGetDeck(w http.ResponseWriter, r *http.Request) {
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

	deckIDStr := r.URL.Query().Get("deck_id")
	if deckIDStr == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "deck_id is required"})
		return
	}

	var deckID int64

	if _, err := fmt.Sscanf(deckIDStr, "%d", &deckID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid deck_id"})
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

	respondJSON(w, http.StatusOK, DeckResponse{
		DeckID:    deckID,
		Name:      deck.Name,
		CardIDs:   cardIDs,
		Cards:     cards,
		CreatedAt: deck.CreatedAt.Format(time.RFC3339),
	})
}

func handleUpdateDeck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
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

	var req UpdateDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.DeckID == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "deck_id is required"})
		return
	}
	if len(req.CardIDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one card is required"})
		return
	}

	ctx := r.Context()

	result, err := queries.UpdateDeck(ctx, db.UpdateDeckParams{
		Name:     req.Name,
		CardID:   int32(req.CardIDs[0]),
		DeckID:   int32(req.DeckID),
		PlayerID: userID,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update deck"})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "deck not found"})
		return
	}

	queries.DeleteDeckCards(ctx, int32(req.DeckID))
	for i, cardID := range req.CardIDs {
		queries.InsertDeckCard(ctx, db.InsertDeckCardParams{
			DeckID:   int32(req.DeckID),
			CardID:   int32(cardID),
			Position: int32(i),
		})
	}

	respondJSON(w, http.StatusOK, DeckResponse{
		DeckID: req.DeckID,
		Name: req.Name,
		CardIDs: req.CardIDs,
		Cards: []DeckCard{},
		CreatedAt: time.Now().Format(time.RFC3339),
	})
}

func handleDeleteDeck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
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

	deckIDStr := r.URL.Query().Get("deck_id")
	if deckIDStr == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "deck_id is required"})
		return
	}

	var deckID int64
	if _, err := fmt.Sscanf(deckIDStr, "%d", &deckID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid deck_id"})
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

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Write([]byte(`{"error":"method not allowed"}`))
}

type CardResponse struct {
	CardID      int    `json:"card_id"`
	CardName    string `json:"card_name"`
	Affiliation int    `json:"affiliation"`
	Rarity      string `json:"rarity"`
	ManaCost    int    `json:"mana_cost"`
	MaxLevel    int    `json:"max_level"`
	Description string `json:"description"`
	IconURL     string `json:"icon_url"`
}

func handleGetAllCards(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

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
			MaxLevel:    int(c.MaxLevel),
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
	MaxLevel    int    `json:"max_level"`
	Description string `json:"description"`
	IconURL     string `json:"icon_url"`
	Level       int    `json:"level"`
	Quantity    int    `json:"quantity"`
	IsInDeck    bool   `json:"is_in_deck"`
}

func handleGetPlayerCards(w http.ResponseWriter, r *http.Request) {
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
			MaxLevel:    int(c.MaxLevel),
			Description: c.Description,
			IconURL:     c.IconUrl,
			Level:       int(c.Level),
			Quantity:    int(c.Quantity),
			IsInDeck:    c.IsInDeck,
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
				MaxLevel:    int(c.MaxLevel),
				Description: c.Description,
				IconURL:     c.IconUrl,
				Level:       int(c.Level),
				Quantity:    int(c.Quantity),
				IsInDeck:    c.IsInDeck,
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