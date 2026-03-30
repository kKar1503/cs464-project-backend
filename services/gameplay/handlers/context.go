package handlers

// HandlerContext provides the interface for handlers to interact with game state
// This allows handlers to be decoupled from the main package implementation
type HandlerContext interface {
	// Player information
	GetPlayerID() int
	GetUserID() int64
	GetUsername() string
	GetSessionID() string

	// State access
	GetGameState() GameState
	GetPlayerState(playerID int) PlayerState
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
	GetTurnNumber() int
	SetTurnNumber(turn int)
	GetCurrentPlayer() int
	SetCurrentPlayer(playerID int)
	GetWinnerID() int
	SetWinnerID(playerID int)
}

// PlayerState represents player state interface
type PlayerState interface {
	GetUserID() int64
	GetUsername() string
	GetGameData() []byte
	SetGameData(data []byte)
}

// PlayerView represents a player's view of the game state
type PlayerView struct {
	SessionID      string `json:"session_id"`
	Phase          string `json:"phase"`
	TurnNumber     int    `json:"turn_number"`
	CurrentPlayer  int    `json:"current_player"`
	SequenceNumber int64  `json:"sequence_number"`

	// Your info
	YourUserID   int64       `json:"your_user_id"`
	YourUsername string      `json:"your_username"`
	
	YourGameData interface{} `json:"your_game_data,omitempty"` // Parsed game data
	
	// Opponent info
	OpponentUserID    int64       `json:"opponent_user_id"`
	OpponentUsername  string      `json:"opponent_username"`
	OpponentConnected bool        `json:"opponent_connected"`
	OpponentGameData  interface{} `json:"opponent_game_data,omitempty"` // Parsed opponent game data

	StateHash uint64 `json:"state_hash"`
}

type Card struct {
	
} 

type GameplayManager interface {
	GetElixer(playerID int64) int 
	RemoveElixer(playerID int64, amount int)
	GetPlayer1ID() int64
	// First board is the player's, the second is the opponent's 
	GetBoard(playerID int64) (*[2][3]Card, *[2][3]Card)
}