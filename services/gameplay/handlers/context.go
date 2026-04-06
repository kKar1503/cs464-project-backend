package handlers

// HandlerContext provides the interface for handlers to interact with game state
type HandlerContext interface {
	// Player information
	GetPlayerID() int
	GetUserID() int64
	GetUsername() string
	GetSessionID() string

	// State access
	GetGameState() GameState
	GetOpponentID() int
	GetGameplayManager() GameplayManager

	// State verification
	GetCurrentSequence() int64
	GetPlayerView(playerID int) PlayerView
	IsPlayerTurn() bool

	// State modification
	LockState()
	UnlockState()
	IncrementSequence()

	// Communication
	SendStateUpdate(action string, view PlayerView)
	BroadcastToOpponent(action string, view PlayerView)
	SendError(errorMsg string, action string)

	// Session management
	UpdateActivity()
	StartTurnTimer(playerID int)
	StopTurnTimer()
	ExecuteServerAction(action string, params interface{}) error
}

// GameState represents the game state interface
type GameState interface {
	GetPhase() string
	SetPhase(phase string)
	GetWinnerID() int
	SetWinnerID(playerID int)
}

// PlayerView represents a player's view of the game state
type PlayerView struct {
	SessionID      string `json:"session_id"`
	Phase          string `json:"phase"`
	SequenceNumber int64  `json:"sequence_number"`

	// Your info
	YourUserID   int64  `json:"your_user_id"`
	YourUsername string `json:"your_username"`

	// Opponent info
	OpponentUserID    int64  `json:"opponent_user_id"`
	OpponentUsername  string `json:"opponent_username"`
	OpponentConnected bool   `json:"opponent_connected"`

	StateHash uint64 `json:"state_hash"`
}
