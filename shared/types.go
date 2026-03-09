package shared

// Common types and utilities shared across services

// User represents a game user
type User struct {
	ID       string
	Username string
	MMR      int
}

// GameSession represents an active game session
type GameSession struct {
	ID      string
	Player1 *User
	Player2 *User
	Status  string
}
