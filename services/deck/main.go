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
)

var db *sql.DB

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
    db, err = sql.Open("mysql", dsn)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer db.Close()

    if err := db.Ping(); err != nil {
        log.Fatalf("Failed to ping database: %v", err)
    }
    log.Println("Connected to MySQL database")

    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"healthy","service":"deck"}`))
    })

    // set up http routes for card apis
    mux.HandleFunc("/cards", handleGetAllCards)

    // set up http routes for deck apis
    mux.HandleFunc("/deck/create", handleCreateDeck)
    mux.HandleFunc("/deck/get", handleGetDeck)
    mux.HandleFunc("/deck/update", handleUpdateDeck)
    mux.HandleFunc("/deck/delete", handleDeleteDeck)

	// set up http routes for pack apis
	mux.HandleFunc("/pack/get", handleGetPacks)
    mux.HandleFunc("/pack/open", handleOpenPack)

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