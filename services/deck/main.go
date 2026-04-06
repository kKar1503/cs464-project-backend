package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	db "github.com/kKar1503/cs464-backend/db/sqlc"
)

var (
	sqlDB   *sql.DB
	queries *db.Queries
)

func main() {
	log.Println("Deck service starting...")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	var err error
	sqlDB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	queries = db.New(sqlDB)
	log.Println("Connected to MySQL database")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"deck"}`))
	})

	// Card routes
	mux.HandleFunc("GET /cards", handleGetAllCards)
	mux.HandleFunc("GET /players/me/cards", handleGetPlayerCards)
	mux.HandleFunc("GET /players/me/cards/available", handleGetCardsNotInDecks)

	// Deck routes
	mux.HandleFunc("PUT /players/me/decks", handleUpdateAllDecks)
	mux.HandleFunc("POST /decks", handleCreateDeck)
	mux.HandleFunc("GET /decks", handleGetAllDecks)
	mux.HandleFunc("GET /decks/{id}", handleGetDeckByID)
	mux.HandleFunc("PUT /decks/{id}", handleUpdateDeck)
	mux.HandleFunc("DELETE /decks/{id}", handleDeleteDeck)
	mux.HandleFunc("PUT /decks/active", handleSetActiveDeck)
	mux.HandleFunc("GET /decks/active", handleGetActiveDeck)

	// Internal routes (called by other services)
	mux.HandleFunc("POST /internal/starter-content", handleInitStarterContent)
	mux.HandleFunc("GET /internal/active-deck", handleGetActiveDeckForGame)

	// Pack routes
	mux.HandleFunc("POST /packs", handleBuyPack)
	mux.HandleFunc("GET /packs", handleGetPacks)
	mux.HandleFunc("POST /packs/open", handleOpenPack)

	port := os.Getenv("SERVICE_PORT")
	if port == "" {
		port = "8005"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Deck service listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}
