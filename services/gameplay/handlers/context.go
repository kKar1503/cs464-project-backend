package handlers

import (
	"time"
)

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

func attackBoard(attackX int, attackY int, attackingPlayer *[2][3]Card, defendingPlayer *[2][3]Card, playerHealth *int) int {
	y_coord := -1
	if (*defendingPlayer)[0][attackX].CardID == 0 && (*defendingPlayer)[1][attackX].CardID == 0 {
		*playerHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		(*attackingPlayer)[attackY][attackX].CurrentHealth -= 5 // To be replaced by the actual attack health
	} else if (*defendingPlayer)[0][attackX].CardID == 0 {
		(*defendingPlayer)[1][attackX].CurrentHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		if (*defendingPlayer)[1][attackX].CurrentHealth == 0 {
			(*defendingPlayer)[1][attackX].CardID = 0
		}
		y_coord = 1
	} else {
		(*defendingPlayer)[0][attackX].CurrentHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		if (*defendingPlayer)[0][attackX].CurrentHealth == 0 {
			(*defendingPlayer)[0][attackX].CardID = 0
		}
		y_coord = 0
	}

	attackingPlayer[attackY][attackX].LastMessage = time.Now()
	return y_coord
}

var attackRegistry = map[string]OnAttack{
	"basic": func(playerID int64, gameplayManager GameplayManager, attackX int, attackY int, randX int, randY int) {
		var self, opponent = gameplayManager.GetBoard(playerID)
		var myHealth, _ = gameplayManager.GetPlayerHealth(playerID)
		attackBoard(attackX, attackY, self, opponent, myHealth)

	},
	// TODO: To be implemented on a later date cause I still havent figured out how I want to do this
	// NOTE: For the time being, I am setting -1, -1 as attack self player, -2, -2 as attacking opponent player
	"random_all": func(playerID int64, gameplayManager GameplayManager, attackX int, attackY int, randX int, randY int) {
		var self, opponent = gameplayManager.GetBoard(playerID)
		var myHealth, opponentHealth = gameplayManager.GetPlayerHealth(playerID)
		if randX == -1 && randY == -1 {
			*myHealth -= (*self)[attackY][attackY].CardAttack
		} else if randX == -2 && randY == -2 {
			*opponentHealth -= (*self)[attackY][attackY].CardAttack
			(*self)[attackY][attackY].CurrentHealth -= 5
		} else if randX/2 >= 2 {
			(*opponent)[randX-2][randY].CurrentHealth -= (*self)[attackY][attackY].CardAttack
		} else {
			(*self)[randX][randY].CurrentHealth -= (*self)[attackY][attackY].CardAttack
		}
	},
	// TODO: To be implemented on a later date cause I still havent figured out how I want to do this
	"random_enemy": func(playerID int64, gameplayManager GameplayManager, attackX int, attackY int, randX int, randY int) {
		var self, opponent = gameplayManager.GetBoard(playerID)
		var _, opponentHealth = gameplayManager.GetPlayerHealth(playerID)

		if randX < 0 && randY < 0 {
			*opponentHealth -= (*self)[attackY][attackY].CardAttack
			(*self)[attackY][attackY].CurrentHealth -= 5
		} else {
			(*opponent)[randX-2][randY].CurrentHealth -= (*self)[attackY][attackY].CardAttack
		}
	},
	"suicide": func(playerID int64, gameplayManager GameplayManager, attackX int, attackY int, randX int, randY int) {
		var self, opponent = gameplayManager.GetBoard(playerID)
		var myHealth, _ = gameplayManager.GetPlayerHealth(playerID)
		attackBoard(attackX, attackY, self, opponent, myHealth)
		(*self)[attackY][attackX].CurrentHealth -= 10
	},
	"back_row": func(playerID int64, gameplayManager GameplayManager, attackX int, attackY int, randX int, randY int) {
		var self, opponent = gameplayManager.GetBoard(playerID)
		var myHealth, _ = gameplayManager.GetPlayerHealth(playerID)

		if (*opponent)[1][attackX].CardID == 0 {
			*myHealth -= (*self)[attackX][attackY].CardAttack
			(*self)[attackY][attackX].CurrentHealth -= 5 // To be replaced by the actual attack health
		} else {
			(*opponent)[1][attackX].CurrentHealth -= (*self)[attackX][attackY].CardAttack
			if (*opponent)[1][attackX].CurrentHealth == 0 {
				(*opponent)[1][attackX].CardID = 0
			}
		}
	},
	"reset_attack": func(playerID int64, gameplayManager GameplayManager, attackX int, attackY int, randX int, randY int) {
		var self, opponent = gameplayManager.GetBoard(playerID)
		var myHealth, _ = gameplayManager.GetPlayerHealth(playerID)
		y_coord := attackBoard(attackX, attackY, self, opponent, myHealth)
		// Eh theoretically should reset timer
		if y_coord != -1 {
			opponent[y_coord][attackX].LastMessage = time.Now()
		}
	},
	"heal_adj": func(playerID int64, gameplayManager GameplayManager, attackX int, attackY int, randX int, randY int) {
		var self, opponent = gameplayManager.GetBoard(playerID)
		var myHealth, _ = gameplayManager.GetPlayerHealth(playerID)
		attackBoard(attackX, attackY, self, opponent, myHealth)
	},
}

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
		yours[summonParam.attackY][summonParam.attackX] = (*summonParam.card)
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

type Card struct {
	CardID        int
	ElixerCost    int
	CurrentHealth int
	CardAttack    int
	TimeToAttack  int
	LastMessage   time.Time

	OnAttack OnAttack
	OnSpawn  OnSpawn
	OnDefend OnDefend
}

type GameplayManager interface {
	GetElixer(playerID int64) int
	RemoveElixer(playerID int64, amount int)
	GetPlayer1ID() int64
	GetBoard(playerID int64) (yours *[2][3]Card, opponents *[2][3]Card)
	GetPlayerHealth(playerID int64) (you *int, opponent *int)
	// First board is the player's, the second is the opponent's
	PlaceCard(playerID int64, card *Card, xPos int, yPos int) error
	AttackCard(playerID int64, xPos int, yPos int) error
}
