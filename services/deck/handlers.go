package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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

	// decks table requires 1 card_id
	firstCardID := req.CardIDs[0]
	result, err := db.Exec(
		"INSERT INTO decks (player_id, card_id, name) VALUES (?, ?, ?)", 
		userID, firstCardID, req.Name,
	)
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
		_, err := db.Exec(
			"INSERT INTO deck_cards (deck_id, card_id, position) VALUES (?, ?, ?)", 
			deckID, cardID, i,
		)
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

	var name string 
	var createdAt []uint8
	err = db.QueryRow(
		"SELECT name, created_at FROM decks WHERE deck_id = ? AND player_id = ?", 
		deckID, userID,
	).Scan(&name, &createdAt)

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "deck not found"})
		return
	}

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	rows, err := db.Query(
		"SELECT card_id, position FROM deck_cards WHERE deck_id = ? ORDER BY position",
		deckID,
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	defer rows.Close()

	var cards []DeckCard
	var cardIDs []int64
	for rows.Next() {
		var c DeckCard
		if err := rows.Scan(&c.CardID, &c.Position); err != nil {
			continue
		}
		cards = append(cards, c)
		cardIDs = append(cardIDs, c.CardID)
	}

	respondJSON(w, http.StatusOK, DeckResponse{
		DeckID:    deckID,
		Name:      name,
		CardIDs:   cardIDs,
		Cards:     cards,
		CreatedAt: string(createdAt),
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

	result, err := db.Exec(
		"UPDATE decks SET name = ?, card_id = ? WHERE deck_id = ? AND player_id = ?",
		req.Name, req.CardIDs[0], req.DeckID, userID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update deck"})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "deck not found"})
		return
	}

	if len(req.CardIDs) > 0 {
		_, _ = db.Exec("DELETE FROM deck_cards WHERE deck_id = ?", req.DeckID)
		for i, cardID := range req.CardIDs {
			_, _ = db.Exec(
				"INSERT INTO deck_cards (deck_id, card_id, position) VALUES (?, ?, ?)",
				req.DeckID, cardID, i,
			)
		}
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

	result, err := db.Exec(
		"DELETE FROM decks WHERE deck_id = ? AND player_id = ?", 
		deckID, userID,
	)
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

	query := "SELECT card_id, card_name, affiliation, rarity, mana_cost, max_level, description, icon_url FROM cards WHERE 1=1"
	args := []interface{}{}

	if rarity := r.URL.Query().Get("rarity"); rarity != "" {
		query += " AND rarity = ?"
		args = append(args, rarity)
	}
	if affiliation := r.URL.Query().Get("affiliation"); affiliation != "" {
		query += " AND affiliation = ?"
		args = append(args, affiliation)
	}

	query += " ORDER BY card_id ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get cards"})
		return
	}
	defer rows.Close()

	var cards []CardResponse
	for rows.Next() {
		var c CardResponse
		if err := rows.Scan(&c.CardID, &c.CardName, &c.Affiliation, &c.Rarity, &c.ManaCost, &c.MaxLevel, &c.Description, &c.IconURL); err != nil {
			continue
		}
		cards = append(cards, c)
	}

	if cards == nil {
		cards = []CardResponse{}
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

	rows, err := db.Query(`
		SELECT c.card_id, c.card_name, c.affiliation, c.rarity, c.mana_cost, c.max_level,
		       c.description, c.icon_url, pc.level, pc.quantity, pc.is_in_deck
		FROM player_cards pc
		JOIN cards c ON pc.card_id = c.card_id
		WHERE pc.player_id = ?
		ORDER BY c.rarity DESC, c.card_id ASC
	`, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get player cards"})
		return
	}
	defer rows.Close()

	var cards []PlayerCardResponse
	for rows.Next() {
		var c PlayerCardResponse
		if err := rows.Scan(
			&c.CardID, &c.CardName, &c.Affiliation, &c.Rarity, &c.ManaCost, &c.MaxLevel,
			&c.Description, &c.IconURL, &c.Level, &c.Quantity, &c.IsInDeck,
		); err != nil {
			continue
		}
		cards = append(cards, c)
	}

	if cards == nil {
		cards = []PlayerCardResponse{}
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

	// Fetch all decks belonging to the user
	deckRows, err := db.Query(
		"SELECT deck_id, name FROM decks WHERE player_id = ? ORDER BY deck_id ASC",
		userID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get decks"})
		return
	}
	defer deckRows.Close()

	type deckInfo struct {
		id   int64
		name string
	}
	var decks []deckInfo
	for deckRows.Next() {
		var d deckInfo
		if err := deckRows.Scan(&d.id, &d.name); err != nil {
			continue
		}
		decks = append(decks, d)
	}

	result := make([]DeckCardsNotInDeck, 0, len(decks))
	for _, d := range decks {
		cardRows, err := db.Query(`
			SELECT c.card_id, c.card_name, c.affiliation, c.rarity, c.mana_cost, c.max_level,
			       c.description, c.icon_url, pc.level, pc.quantity, pc.is_in_deck
			FROM player_cards pc
			JOIN cards c ON pc.card_id = c.card_id
			WHERE pc.player_id = ?
			  AND pc.card_id NOT IN (
			      SELECT dc.card_id FROM deck_cards dc WHERE dc.deck_id = ?
			  )
			ORDER BY c.rarity DESC, c.card_id ASC
		`, userID, d.id)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get cards for deck"})
			return
		}

		var cards []PlayerCardResponse
		for cardRows.Next() {
			var c PlayerCardResponse
			if err := cardRows.Scan(
				&c.CardID, &c.CardName, &c.Affiliation, &c.Rarity, &c.ManaCost, &c.MaxLevel,
				&c.Description, &c.IconURL, &c.Level, &c.Quantity, &c.IsInDeck,
			); err != nil {
				continue
			}
			cards = append(cards, c)
		}
		cardRows.Close()

		if cards == nil {
			cards = []PlayerCardResponse{}
		}

		result = append(result, DeckCardsNotInDeck{
			DeckID:   d.id,
			DeckName: d.name,
			Cards:    cards,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"decks": result})
}