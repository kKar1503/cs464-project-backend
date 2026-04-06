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
	YourUserID   int64  `json:"your_user_id"`
	YourUsername string `json:"your_username"`

	YourGameData interface{} `json:"your_game_data,omitempty"` // Parsed game data

	// Opponent info
	OpponentUserID    int64       `json:"opponent_user_id"`
	OpponentUsername  string      `json:"opponent_username"`
	OpponentConnected bool        `json:"opponent_connected"`
	OpponentGameData  interface{} `json:"opponent_game_data,omitempty"` // Parsed opponent game data

	StateHash uint64 `json:"state_hash"`
}

type OnAttack func(playerId int64, gameplayManager GameplayManager, attackX int, attackY int, randX int, randY int)
type OnSpawn func(summonParam SummonParam)
type OnDefend func(playerId int64, gameplayManager GameplayManager, attackX int, attackY int)
type OnDeath func(playerId int64, gameplayManager GameplayManager, attackX int, attackY int)

// Attack, summon, defence, and death registries are stubs for future card effect implementation.
// Attack resolution is handled by the game loop's TickBoard(), not these registries.

var attackRegistry = map[string]OnAttack{}


type SummonParam struct {
	playerID        int64
	gameplayManager GameplayManager
	attackX         int
	attackY         int
	card            *Card
	validPercentage bool
}

var summonRegistry = map[string]OnSpawn{
	"basic": func(summonParam SummonParam) {
		var yours, _ = summonParam.gameplayManager.GetBoard(summonParam.playerID)
		yours[summonParam.attackX][summonParam.attackY] = (*summonParam.card)
	},
	"technoblade": func(summonParam SummonParam) {

	},
	"destroy_same_colour": func(summonParam SummonParam) {

	},
	"double_attack_speed": func(summonParam SummonParam) {

	},
	"self_stats_change": func(summonParam SummonParam) {

	},
	"buff_all_allies": func(summonParam SummonParam) {

	},
	"vertical_stats_change": func(summonParam SummonParam) {

	},
	"adjacent_stats_change": func(summonParam SummonParam) {

	},
	"opponent_stats_change": func(summonParam SummonParam) {

	},
	"into_pig":func(summonParam SummonParam){

	},
	"deal_damage": func(summonParam SummonParam) {

	},
	"bounce":func(summonParam SummonParam) {

	},
	"elixer_overflow": func(summonParam SummonParam) {

	},
	"destroy_enemy_infront": func(summonParam SummonParam) {

	},
	"set_all_hp_1": func(summonParam SummonParam) {

	},
	"reset_attack": func(summonParam SummonParam) {

	},
	"summon_wolves": func(summonParam SummonParam) {

	},
}

var defenceRegistry = map[string]OnDefend{
	"basic": func(playerId int64, gameplayManager GameplayManager, attackX int, attackY int) {

	},
	"buff_attack": func(playerId int64, gameplayManager GameplayManager, attackX int, attackY int) {

	},
	"reflect": func(playerId int64, gameplayManager GameplayManager, attackX int, attackY int) {

	},
	"shield": func(playerId int64, gameplayManager GameplayManager, attackX int, attackY int) {

	},
}

var deathRegistry = map[string]OnDeath{
	"basic": func(playerId int64, gameplayManager GameplayManager, attackX int, attackY int) {

	},
}

const (
	ChargeTicksTotal = 40 // 10 seconds at 4 ticks/sec
)

type Card struct {
	CardID        int
	CardName      string
	ElixerCost    int
	CurrentHealth int
	MaxHealth     int
	CardAttack    int
	Colour        string

	// Charge-based auto-attack
	ChargeTicksRemaining int  // starts at ChargeTicksTotal, counts down each tick
	IsCharging           bool // true once placed on board

	// Effect callbacks (for future use)
	OnAttack OnAttack
	OnSpawn  OnSpawn
	OnDefend OnDefend
}

// HandCardInfo represents a card in a player's hand/deck for the draw phase.
type HandCardInfo struct {
	CardID   int    `json:"card_id"`
	CardName string `json:"card_name"`
	Colour   string `json:"colour"`
	Rarity   string `json:"rarity"`
	ManaCost int    `json:"mana_cost"`
	Attack   int    `json:"attack"`
	HP       int    `json:"hp"`
}

type GameplayManager interface {
	GetElixer(playerID int64) int
	RemoveElixer(playerID int64, amount int)
	GetPlayer1ID() int64
	GetBoard(playerID int64) (yours *[2][3]Card, opponents *[2][3]Card)
	GetPlayerHealth(playerID int64) (you *int, opponent *int)
	PlaceCard(playerID int64, card *Card, xPos int, yPos int) error

	// Hand drawing
	OfferCards(playerID int64) []HandCardInfo
	SelectCards(playerID int64, selectedIDs []int) error
	GetHand(playerID int64) []HandCardInfo
	RemoveFromHand(playerID int64, cardID int) error
	MarkPlayerDrew(playerID int64) bool
}
