package main

import (
	"errors"
	"fmt"
	"sync"
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

type GameplayManager struct {
	game      *GameplayState
	player1ID int64
	player2ID int64

	// Management of the struct
	ticker           *time.Ticker
	done             chan bool
	elixerMutex1     sync.RWMutex
	elixerMutex2     sync.RWMutex
	boardMutexAttack sync.Mutex
}

func NewGameplayManager(sessionID string, player1ID int64, player2ID int64) *GameplayManager {
	var BoardPlayer1 [2][3]handlers.Card
	var BoardPlayer2 [2][3]handlers.Card
	var ticker = time.NewTicker(5 * time.Second)
	var done = make(chan bool)
	var gh = &GameplayManager{
		game: &GameplayState{
			sessionID,
			100,
			100,
			0,
			&BoardPlayer1,
			0,
			&BoardPlayer2,
			1,
		},
		player1ID: player1ID,
		player2ID: player2ID,
		ticker:    ticker,
		done:      done,
	}

	go gh.generateElixer(ticker, done)
	return gh
}

func (gh *GameplayManager) generateElixer(ticker *time.Ticker, done <-chan bool) {
	for {
		select {
		case <-done:
			return
		case _ = <-ticker.C:
			gh.elixerMutex1.Lock()
			gh.game.ElixerPlayer1 += max(gh.game.RoundNumber+5, gh.game.ElixerPlayer1+1)
			gh.elixerMutex1.Unlock()

			gh.elixerMutex2.Lock()
			gh.game.ElixerPlayer2 += max(gh.game.RoundNumber+5, gh.game.ElixerPlayer2+1)
			gh.elixerMutex2.Unlock()
		}
	}
}

func (gh *GameplayManager) DrawRound() {
	gh.ticker.Stop()
	// TODO: Make sure this is the correct timing
	time.Sleep(10 * time.Second)
	gh.ticker.Reset(5 * time.Second)
}

func (gh *GameplayManager) PlayCard(playerID int64, card *handlers.Card, xPos int, yPos int) error {
	var isPlayer1 bool = playerID == gh.player1ID

	if isPlayer1 {
		err := placeCard(xPos, yPos, gh.game.BoardPlayer1, card)
		if err != nil {
			return err
		}
		gh.elixerMutex1.Lock()
		defer gh.elixerMutex1.Unlock()
		gh.game.ElixerPlayer1 -= card.ElixerCost
	} else {
		err := placeCard(xPos, yPos, gh.game.BoardPlayer2, card)
		if err != nil {
			return err
		}
		gh.elixerMutex2.Lock()
		defer gh.elixerMutex2.Unlock()
		gh.game.ElixerPlayer2 -= card.ElixerCost
	}

	return nil
}

func (gh *GameplayManager) AttackCard(playerID int64, attackX int, attackY int) error {
	var isPlayer1 bool = playerID == gh.player1ID // Replace this with a valid check later

	gh.boardMutexAttack.Lock()
	defer gh.boardMutexAttack.Unlock()
	if isPlayer1 {
		return attackBoard(attackX, attackY, gh.game.BoardPlayer1, gh.game.BoardPlayer2, &gh.game.Player2Health)
	}
	return attackBoard(attackX, attackY, gh.game.BoardPlayer2, gh.game.BoardPlayer1, &gh.game.Player1Health)

}

func (gh *GameplayManager) EndGameplay() {
	gh.done <- true
	gh.ticker.Stop()
}

// Helper functions
// TODO: Add verification to check whether the card is allowed to attack
// TODO: Replace this with a for loop (you wont)
func attackBoard(attackX int, attackY int, attackingPlayer *[2][3]handlers.Card, defendingPlayer *[2][3]handlers.Card, playerHealth *int) error {
	if attackingPlayer[attackY][attackX].LastMessage.Sub(time.Now()) < time.Duration(attackingPlayer[attackY][attackX].TimeToAttack)*time.Second {
		return errors.New("Attack Message sent too early")
	}

	if (*defendingPlayer)[0][attackX].CardID == 0 && (*defendingPlayer)[0][attackX].CardID == 0 {
		*playerHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		(*defendingPlayer)[attackY][attackX].CurrentHealth -= 5 // To be replaced by the actual attack health
	} else if (*defendingPlayer)[0][attackX].CardID == 0 {
		(*defendingPlayer)[1][attackX].CurrentHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		if (*defendingPlayer)[1][attackX].CurrentHealth == 0 {
			(*defendingPlayer)[1][attackX].CardID = 0
		}
	} else {
		(*defendingPlayer)[0][attackX].CurrentHealth -= (*attackingPlayer)[attackX][attackY].CardAttack
		if (*defendingPlayer)[0][attackX].CurrentHealth == 0 {
			(*defendingPlayer)[0][attackX].CardID = 0
		}
	}

	attackingPlayer[attackY][attackX].LastMessage = time.Now()
	return nil
}

func placeCard(xPos int, yPos int, board *[2][3]handlers.Card, card *handlers.Card) error {
	if board[xPos][yPos].CardID != 0 {
		// cause some sort of error
		return fmt.Errorf("Card already exists")
	}
	// This looks wrong
	board[xPos][yPos] = *card
	return nil
}
