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