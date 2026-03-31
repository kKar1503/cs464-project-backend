package main

import (
	"encoding/json"

	"github.com/kKar1503/cs464-backend/services/gameplay/handlers"
)

// PlayerConnectionContext adapts PlayerConnection to implement handlers.HandlerContext
type PlayerConnectionContext struct {
	conn *PlayerConnection
}

// NewHandlerContext creates a new handler context from a player connection
func NewHandlerContext(conn *PlayerConnection) *PlayerConnectionContext {
	return &PlayerConnectionContext{conn: conn}
}

// Player information
func (ctx *PlayerConnectionContext) GetPlayerID() int {
	return int(ctx.conn.PlayerID)
}

func (ctx *PlayerConnectionContext) GetUserID() int64 {
	return ctx.conn.UserID
}

func (ctx *PlayerConnectionContext) GetUsername() string {
	return ctx.conn.Username
}

func (ctx *PlayerConnectionContext) GetSessionID() string {
	return ctx.conn.SessionID
}

// State access
func (ctx *PlayerConnectionContext) GetGameState() handlers.GameState {
	return &GameStateAdapter{state: ctx.conn.Session.State}
}

func (ctx *PlayerConnectionContext) GetGameplayManager() handlers.GameplayManager {
	return &GameplayAdapter{gameplay: ctx.conn.Session.GameplayManager}
}

func (ctx *PlayerConnectionContext) GetPlayerState(playerID int) handlers.PlayerState {
	ps := ctx.conn.Session.State.GetPlayerState(PlayerID(playerID))
	return &PlayerStateAdapter{state: ps}
}

func (ctx *PlayerConnectionContext) GetOpponentID() int {
	if ctx.conn.PlayerID == Player1 {
		return int(Player2)
	}
	return int(Player1)
}

// State verification
func (ctx *PlayerConnectionContext) GetCurrentSequence() int64 {
	return ctx.conn.Session.State.GetPlayerSequence(ctx.conn.PlayerID)
}

func (ctx *PlayerConnectionContext) GetPlayerView(playerID int) handlers.PlayerView {
	view := ctx.conn.Session.State.GetPlayerView(PlayerID(playerID))

	// Parse game data for the view
	var yourGameData interface{}
	var opponentGameData interface{}

	if len(view.YourGameData) > 0 {
		json.Unmarshal(view.YourGameData, &yourGameData)
	}
	if len(view.OpponentGameData) > 0 {
		json.Unmarshal(view.OpponentGameData, &opponentGameData)
	}

	return handlers.PlayerView{
		SessionID:         view.SessionID,
		Phase:             string(view.Phase),
		TurnNumber:        view.TurnNumber,
		CurrentPlayer:     int(view.CurrentPlayer),
		SequenceNumber:    view.SequenceNumber,
		YourUserID:        view.YourUserID,
		YourUsername:      view.YourUsername,
		YourGameData:      yourGameData,
		OpponentUserID:    view.OpponentUserID,
		OpponentUsername:  view.OpponentUsername,
		OpponentConnected: view.OpponentConnected,
		OpponentGameData:  opponentGameData,
		StateHash:         view.StateHash,
	}
}

func (ctx *PlayerConnectionContext) IsPlayerTurn() bool {
	currentPhase := ctx.conn.Session.State.Phase
	if ctx.conn.PlayerID == Player1 && currentPhase == PhasePlayer1Turn {
		return true
	}
	if ctx.conn.PlayerID == Player2 && currentPhase == PhasePlayer2Turn {
		return true
	}
	return false
}

// State modification
func (ctx *PlayerConnectionContext) LockState() {
	ctx.conn.Session.State.mu.Lock()
}

func (ctx *PlayerConnectionContext) UnlockState() {
	ctx.conn.Session.State.mu.Unlock()
}

func (ctx *PlayerConnectionContext) IncrementSequence() {
	ctx.conn.Session.State.IncrementSequence(ctx.conn.PlayerID)
}

// Communication
func (ctx *PlayerConnectionContext) SendStateUpdate(action string, view handlers.PlayerView) {
	mainView := &PlayerView{
		SessionID:         view.SessionID,
		Phase:             GamePhase(view.Phase),
		TurnNumber:        view.TurnNumber,
		CurrentPlayer:     PlayerID(view.CurrentPlayer),
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
		TurnNumber:        view.TurnNumber,
		CurrentPlayer:     PlayerID(view.CurrentPlayer),
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

// Session management
func (ctx *PlayerConnectionContext) UpdateActivity() {
	ctx.conn.Session.UpdateActivity()
}

func (ctx *PlayerConnectionContext) StartTurnTimer(playerID int) {
	if ctx.conn.Session.TurnTimer != nil {
		ctx.conn.Session.TurnTimer.StartTurn(PlayerID(playerID))
	}
}

func (ctx *PlayerConnectionContext) StopTurnTimer() {
	if ctx.conn.Session.TurnTimer != nil {
		ctx.conn.Session.TurnTimer.StopTurn()
	}
}

func (ctx *PlayerConnectionContext) ExecuteServerAction(action string, params interface{}) error {
	serverCtx := &ServerActionContext{
		Session:  ctx.conn.Session,
		PlayerID: ctx.conn.PlayerID,
	}
	return serverCtx.ExecuteServerAction(GameAction(action), params)
}

// I do not know the point to this just follow like blind sheep
type GameplayAdapter struct {
	gameplay *GameplayManager
}

func (gpa *GameplayAdapter) GetElixer(playerID int64) int {
	if playerID == gpa.gameplay.player1ID {
		return gpa.gameplay.game.ElixerPlayer1
	} else {
		return gpa.gameplay.game.ElixerPlayer2
	}
}

func (gpa *GameplayAdapter) RemoveElixer(playerID int64, elixerToRemove int) {
	if playerID == gpa.gameplay.player1ID {
		gpa.gameplay.elixerMutex1.Lock()
		defer gpa.gameplay.elixerMutex1.Unlock()
		gpa.gameplay.game.ElixerPlayer1 -= elixerToRemove
	} else {
		gpa.gameplay.elixerMutex2.Lock()
		defer gpa.gameplay.elixerMutex2.Unlock()
		gpa.gameplay.game.ElixerPlayer2 -= elixerToRemove
	}
}

func (gpa *GameplayAdapter) AttackCard(playerID int64, xPos int, yPos int) error {
	return gpa.gameplay.AttackCard(playerID, xPos, yPos)
}

func (gpa *GameplayAdapter) PlaceCard(playerID int64, card *handlers.Card, xPos int, yPos int) error {
	return gpa.gameplay.PlayCard(playerID, card, xPos, yPos)
}

func (gpa *GameplayAdapter) GetPlayer1ID() int64 {
	return gpa.gameplay.player1ID
}

// GameStateAdapter adapts GameState to handlers.GameState interface
type GameStateAdapter struct {
	state *GameState
}

func (gsa *GameStateAdapter) GetPhase() string {
	return string(gsa.state.Phase)
}

func (gsa *GameStateAdapter) SetPhase(phase string) {
	gsa.state.Phase = GamePhase(phase)
}

func (gsa *GameStateAdapter) GetTurnNumber() int {
	return gsa.state.TurnNumber
}

func (gsa *GameStateAdapter) SetTurnNumber(turn int) {
	gsa.state.TurnNumber = turn
}

func (gsa *GameStateAdapter) GetCurrentPlayer() int {
	return int(gsa.state.CurrentPlayer)
}

func (gsa *GameStateAdapter) SetCurrentPlayer(playerID int) {
	gsa.state.CurrentPlayer = PlayerID(playerID)
}

func (gsa *GameStateAdapter) GetWinnerID() int {
	return int(gsa.state.WinnerID)
}

func (gsa *GameStateAdapter) SetWinnerID(playerID int) {
	gsa.state.WinnerID = PlayerID(playerID)
}

// PlayerStateAdapter adapts PlayerState to handlers.PlayerState interface
type PlayerStateAdapter struct {
	state *PlayerState
}

func (psa *PlayerStateAdapter) GetUserID() int64 {
	return psa.state.UserID
}

func (psa *PlayerStateAdapter) GetUsername() string {
	return psa.state.Username
}

func (psa *PlayerStateAdapter) GetGameData() []byte {
	return psa.state.GameData
}

func (psa *PlayerStateAdapter) SetGameData(data []byte) {
	psa.state.GameData = data
}
