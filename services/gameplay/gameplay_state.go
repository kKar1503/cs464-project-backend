package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/kKar1503/cs464-backend/services/gameplay/handlers"
)

type GameplayState struct {
	SessionID     string
	Player1Health int
	Player2Health int
	ElixerPlayer1 int
	BoardPlayer1  *[2][3]handlers.Card
	ElixerPlayer2 int
	BoardPlayer2  *[2][3]handlers.Card
	RoundNumber   int
}

// GameplayManager manages gameplay state. All methods are called from the
// game loop goroutine only — no mutexes needed.
type GameplayManager struct {
	game      *GameplayState
	player1ID int64
	player2ID int64
}

func NewGameplayManager(sessionID string, player1ID int64, player2ID int64) *GameplayManager {
	var boardPlayer1 [2][3]handlers.Card
	var boardPlayer2 [2][3]handlers.Card
	return &GameplayManager{
		game: &GameplayState{
			SessionID:     sessionID,
			Player1Health: 100,
			Player2Health: 100,
			ElixerPlayer1: 0,
			BoardPlayer1:  &boardPlayer1,
			ElixerPlayer2: 0,
			BoardPlayer2:  &boardPlayer2,
			RoundNumber:   1,
		},
		player1ID: player1ID,
		player2ID: player2ID,
	}
}

// TickElixir increments elixir for both players. Called by the game loop every ElixirEvery ticks.
func (gh *GameplayManager) TickElixir() {
	maxElixir := gh.game.RoundNumber + 5
	gh.game.ElixerPlayer1 = min(maxElixir, gh.game.ElixerPlayer1+1)
	gh.game.ElixerPlayer2 = min(maxElixir, gh.game.ElixerPlayer2+1)
}

// CheckWinCondition returns whether the game is over and which player won (1 or 2).
func (gh *GameplayManager) CheckWinCondition() (gameOver bool, winnerID int) {
	if gh.game.Player1Health <= 0 {
		return true, 2
	}
	if gh.game.Player2Health <= 0 {
		return true, 1
	}
	return false, 0
}

func (gh *GameplayManager) PlayCard(playerID int64, card *handlers.Card, xPos int, yPos int) error {
	isPlayer1 := playerID == gh.player1ID

	if isPlayer1 {
		if err := placeCard(xPos, yPos, gh.game.BoardPlayer1, card); err != nil {
			return err
		}
		gh.game.ElixerPlayer1 -= card.ElixerCost
	} else {
		if err := placeCard(xPos, yPos, gh.game.BoardPlayer2, card); err != nil {
			return err
		}
		gh.game.ElixerPlayer2 -= card.ElixerCost
	}

	return nil
}

func (gh *GameplayManager) AttackCard(playerID int64, attackX int, attackY int) error {
	isPlayer1 := playerID == gh.player1ID
	if isPlayer1 {
		return attackBoard(attackX, attackY, gh.game.BoardPlayer1, gh.game.BoardPlayer2, &gh.game.Player2Health)
	}
	return attackBoard(attackX, attackY, gh.game.BoardPlayer2, gh.game.BoardPlayer1, &gh.game.Player1Health)
}

// attackBoard resolves an attack from one board position against the opposing board.
func attackBoard(attackX int, attackY int, attackingPlayer *[2][3]handlers.Card, defendingPlayer *[2][3]handlers.Card, playerHealth *int) error {
	if attackingPlayer[attackX][attackY].LastMessage.Sub(time.Now()) < time.Duration(attackingPlayer[attackX][attackY].TimeToAttack)*time.Second {
		return errors.New("Attack Message sent too early")
	}

	if (*defendingPlayer)[0][attackY].CardID == 0 && (*defendingPlayer)[1][attackY].CardID == 0 {
		*playerHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
	} else if (*defendingPlayer)[0][attackY].CardID == 0 {
		(*defendingPlayer)[1][attackY].CurrentHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		if (*defendingPlayer)[1][attackY].CurrentHealth <= 0 {
			(*defendingPlayer)[1][attackY].CardID = 0
		}
	} else {
		(*defendingPlayer)[0][attackY].CurrentHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		if (*defendingPlayer)[0][attackY].CurrentHealth <= 0 {
			(*defendingPlayer)[0][attackY].CardID = 0
		}
	}

	attackingPlayer[attackX][attackY].LastMessage = time.Now()
	return nil
}

func placeCard(xPos int, yPos int, board *[2][3]handlers.Card, card *handlers.Card) error {
	if board[xPos][yPos].CardID != 0 {
		return fmt.Errorf("Card already exists")
	}
	board[xPos][yPos] = *card
	return nil
}
