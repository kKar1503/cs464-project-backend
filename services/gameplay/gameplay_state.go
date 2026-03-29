package main

import (
	"sync"
	"time"
)

type CardInformation struct {
	cardID int
}

type GameplayState struct {
	SessionID     string
	ElixerPlayer1 int
	BoardPlayer1  *[2][3]int
	ElixerPlayer2 int
	BoardPlayer2  *[2][3]int
	RoundNumber   int
}

type GameplayHandler struct {
	game *GameplayState

	// Management of the struct
	ticker *time.Ticker
	done   chan bool
	mu     sync.RWMutex
}

func (gh *GameplayHandler) generateElixer(ticker *time.Ticker, done <-chan bool) {
	for {
		select {
		case <-done:
			return
		case _ = <-ticker.C:
			gh.mu.Lock()
			defer gh.mu.Unlock()
			gh.game.ElixerPlayer1 += max(gh.game.RoundNumber+5, gh.game.ElixerPlayer1+1)
		}
	}
}

func NewGamplayHandler(sessionID string) *GameplayHandler {
	var BoardPlayer1 [2][3]int
	var BoardPlayer2 [2][3]int
	var ticker = time.NewTicker(1 * time.Second)
	var done = make(chan bool)
	var gh = &GameplayHandler{
		game: &GameplayState{
			sessionID,
			0,
			&BoardPlayer1,
			0,
			&BoardPlayer2,
			1,
		},
		ticker: ticker,
		done:   done,
	}

	go gh.generateElixer(ticker, done)

	return gh

}

func (gh *GameplayHandler) PlayCardElixir(cardId int) {
	
}

func (gh *GameplayHandler) EndGameplay() {
	gh.done <- true
	gh.ticker.Stop()
}