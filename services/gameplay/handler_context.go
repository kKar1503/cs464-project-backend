package main

import (
	"math/rand"

	"github.com/kKar1503/cs464-backend/services/gameplay/effects"
	"github.com/kKar1503/cs464-backend/services/gameplay/handlers"
)

// PlayerConnectionContext adapts PlayerConnection to implement handlers.HandlerContext
type PlayerConnectionContext struct {
	conn *PlayerConnection
}

func NewHandlerContext(conn *PlayerConnection) *PlayerConnectionContext {
	return &PlayerConnectionContext{conn: conn}
}

func (ctx *PlayerConnectionContext) GetPlayerID() int    { return int(ctx.conn.PlayerID) }
func (ctx *PlayerConnectionContext) GetUserID() int64    { return ctx.conn.UserID }
func (ctx *PlayerConnectionContext) GetUsername() string  { return ctx.conn.Username }
func (ctx *PlayerConnectionContext) GetSessionID() string { return ctx.conn.SessionID }

func (ctx *PlayerConnectionContext) GetGameState() handlers.GameState {
	return &GameStateAdapter{state: ctx.conn.Session.State}
}

func (ctx *PlayerConnectionContext) GetGameplayManager() handlers.GameplayManager {
	return &GameplayAdapter{gameplay: ctx.conn.Session.GameplayManager}
}

func (ctx *PlayerConnectionContext) GetOpponentID() int {
	if ctx.conn.PlayerID == Player1 {
		return int(Player2)
	}
	return int(Player1)
}

func (ctx *PlayerConnectionContext) GetCurrentSequence() int64 {
	return ctx.conn.Session.State.GetPlayerSequence(ctx.conn.PlayerID)
}

func (ctx *PlayerConnectionContext) GetPlayerView(playerID int) handlers.PlayerView {
	view := ctx.conn.Session.State.GetPlayerView(PlayerID(playerID))

	return handlers.PlayerView{
		SessionID:         view.SessionID,
		Phase:             string(view.Phase),
		SequenceNumber:    view.SequenceNumber,
		YourUserID:        view.YourUserID,
		YourUsername:      view.YourUsername,
		OpponentUserID:    view.OpponentUserID,
		OpponentUsername:  view.OpponentUsername,
		OpponentConnected: view.OpponentConnected,
		StateHash:         view.StateHash,
	}
}

func (ctx *PlayerConnectionContext) IsPlayerTurn() bool {
	return ctx.conn.Session.State.Phase == PhaseActive
}

func (ctx *PlayerConnectionContext) LockState()   {}
func (ctx *PlayerConnectionContext) UnlockState() {}

func (ctx *PlayerConnectionContext) IncrementSequence() {
	ctx.conn.Session.State.IncrementSequence(ctx.conn.PlayerID)
}

func (ctx *PlayerConnectionContext) SendStateUpdate(action string, view handlers.PlayerView) {
	mainView := &PlayerView{
		SessionID:         view.SessionID,
		Phase:             GamePhase(view.Phase),
		SequenceNumber:    view.SequenceNumber,
		YourUserID:        view.YourUserID,
		YourUsername:      view.YourUsername,
		OpponentUserID:    view.OpponentUserID,
		OpponentUsername:  view.OpponentUsername,
		OpponentConnected: view.OpponentConnected,
		StateHash:         view.StateHash,
	}
	ctx.conn.SendStateUpdate(GameAction(action), mainView)
}

func (ctx *PlayerConnectionContext) BroadcastToOpponent(action string, view handlers.PlayerView) {
	mainView := &PlayerView{
		SessionID:         view.SessionID,
		Phase:             GamePhase(view.Phase),
		SequenceNumber:    view.SequenceNumber,
		YourUserID:        view.YourUserID,
		YourUsername:      view.YourUsername,
		OpponentUserID:    view.OpponentUserID,
		OpponentUsername:  view.OpponentUsername,
		OpponentConnected: view.OpponentConnected,
		StateHash:         view.StateHash,
	}

	opponentID := Player1
	if ctx.conn.PlayerID == Player1 {
		opponentID = Player2
	}

	ctx.conn.Session.BroadcastToOpponent(ctx.conn.PlayerID, &ServerMessage{
		MessageType:    "state_update",
		Action:         GameAction(action),
		StateView:      mainView,
		SequenceNumber: ctx.conn.Session.State.GetPlayerSequence(opponentID),
	})
}

func (ctx *PlayerConnectionContext) SendError(errorMsg string, action string) {
	ctx.conn.SendError(errorMsg, GameAction(action))
}

func (ctx *PlayerConnectionContext) UpdateActivity() {
	ctx.conn.Session.UpdateActivity()
}

func (ctx *PlayerConnectionContext) StartTurnTimer(playerID int) {}
func (ctx *PlayerConnectionContext) StopTurnTimer()              {}

func (ctx *PlayerConnectionContext) ExecuteServerAction(action string, params interface{}) error {
	serverCtx := &ServerActionContext{
		Session:  ctx.conn.Session,
		PlayerID: ctx.conn.PlayerID,
	}
	return serverCtx.ExecuteServerAction(GameAction(action), params)
}

// GameplayAdapter adapts main.GameplayManager to handlers.GameplayManager interface
type GameplayAdapter struct {
	gameplay *GameplayManager
}

func (gpa *GameplayAdapter) GetElixir(playerID int64) int {
	return gpa.gameplay.GetElixirDisplay(playerID)
}

func (gpa *GameplayAdapter) RemoveElixir(playerID int64, elixirToRemove int) {
	milliToRemove := elixirToRemove * MilliElixirPerElixir
	if playerID == gpa.gameplay.player1ID {
		gpa.gameplay.game.MilliElixirPlayer1 -= milliToRemove
	} else {
		gpa.gameplay.game.MilliElixirPlayer2 -= milliToRemove
	}
}

func (gpa *GameplayAdapter) PlaceCard(playerID int64, card *effects.CardInstance, row int, col int) error {
	return gpa.gameplay.PlayCard(playerID, card, row, col)
}

func (gpa *GameplayAdapter) GetPlayer1ID() int64 {
	return gpa.gameplay.player1ID
}

func (gpa *GameplayAdapter) GetDrawPile(playerID int64) []handlers.HandCardInfo {
	return handCardsToInfo(gpa.gameplay.GetDrawPile(playerID))
}

func (gpa *GameplayAdapter) GetHandCards(playerID int64) []handlers.HandCardInfo {
	return handCardsToInfo(gpa.gameplay.GetHand(playerID))
}

func (gpa *GameplayAdapter) SelectCard(playerID int64, cardID int) error {
	return gpa.gameplay.SelectCard(playerID, cardID)
}

func (gpa *GameplayAdapter) DeselectCard(playerID int64, cardID int) error {
	return gpa.gameplay.DeselectCard(playerID, cardID)
}

func (gpa *GameplayAdapter) PlayFromHand(playerID int64, cardID int) (*handlers.HandCardInfo, error) {
	card, err := gpa.gameplay.PlayFromHand(playerID, cardID)
	if err != nil {
		return nil, err
	}
	return &handlers.HandCardInfo{
		CardID: card.CardID, CardName: card.CardName, Colour: card.Colour,
		Rarity: card.Rarity, ManaCost: card.ManaCost, Attack: card.Attack, HP: card.HP,
		Abilities: card.Abilities,
	}, nil
}

func (gpa *GameplayAdapter) GetBoard(playerID int64) (yours *[2][3]*effects.CardInstance, opponents *[2][3]*effects.CardInstance) {
	if playerID == gpa.gameplay.player1ID {
		return &gpa.gameplay.game.BoardPlayer1, &gpa.gameplay.game.BoardPlayer2
	}
	return &gpa.gameplay.game.BoardPlayer2, &gpa.gameplay.game.BoardPlayer1
}

func (gpa *GameplayAdapter) GetPlayerHealth(playerID int64) (you *int, opponent *int) {
	if playerID == gpa.gameplay.player1ID {
		return &gpa.gameplay.game.Player1Health, &gpa.gameplay.game.Player2Health
	}
	return &gpa.gameplay.game.Player2Health, &gpa.gameplay.game.Player1Health
}

func (gpa *GameplayAdapter) GetCardStore() *effects.CardDefinitionStore {
	return gpa.gameplay.CardStore
}

func (gpa *GameplayAdapter) GetRNG() *rand.Rand {
	return gpa.gameplay.RNG
}

func (gpa *GameplayAdapter) GetElixirCap(playerID int64) *int {
	return &gpa.gameplay.game.ElixirCap
}

func (gpa *GameplayAdapter) ReturnToHand(playerID int64, def *effects.CardDefinition) {
	gpa.gameplay.ReturnCardToHand(playerID, def)
}

func (gpa *GameplayAdapter) IsPlayer1(playerID int64) bool {
	return playerID == gpa.gameplay.player1ID
}

func (gpa *GameplayAdapter) FireSummonEffects(playerID int64, card *effects.CardInstance, row, col int) {
	gpa.gameplay.FireSummonEffects(playerID, card, row, col)
}

func handCardsToInfo(cards []HandCard) []handlers.HandCardInfo {
	result := make([]handlers.HandCardInfo, len(cards))
	for i, c := range cards {
		result[i] = handlers.HandCardInfo{
			CardID: c.CardID, CardName: c.CardName, Colour: c.Colour,
			Rarity: c.Rarity, ManaCost: c.ManaCost, Attack: c.Attack, HP: c.HP,
			Abilities: c.Abilities,
		}
	}
	return result
}

// GameStateAdapter adapts GameState to handlers.GameState interface
type GameStateAdapter struct {
	state *GameState
}

func (gsa *GameStateAdapter) GetPhase() string       { return string(gsa.state.Phase) }
func (gsa *GameStateAdapter) SetPhase(phase string)   { gsa.state.Phase = GamePhase(phase) }
func (gsa *GameStateAdapter) GetWinnerID() int         { return int(gsa.state.WinnerID) }
func (gsa *GameStateAdapter) SetWinnerID(playerID int) { gsa.state.WinnerID = PlayerID(playerID) }
