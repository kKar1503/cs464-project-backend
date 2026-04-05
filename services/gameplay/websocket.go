package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// WebSocket configuration
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second
	PingPeriod     = (PongWait * 9) / 10
	MaxMessageSize = 512 * 1024 // 512 KB
)

// ClientMessage represents a message from the client
type ClientMessage struct {
	Action         GameAction      `json:"action"`
	Params         json.RawMessage `json:"params"`           // Raw JSON params (unparsed until handler)
	StateHashAfter uint64          `json:"state_hash_after"` // Hash of client's state AFTER applying action
	SequenceNumber int64           `json:"sequence_number"`
}

// ServerMessage represents a message to the client
type ServerMessage struct {
	MessageType    string      `json:"message_type"` // "state_update", "action_result", "tick_update", "error"
	Action         GameAction  `json:"action,omitempty"`
	Params         interface{} `json:"params,omitempty"`
	Result         string      `json:"result,omitempty"` // "success", "failure"
	ErrorMessage   string      `json:"error_message,omitempty"`
	StateView      *PlayerView `json:"state_view,omitempty"`
	SequenceNumber int64       `json:"sequence_number"`
	TickNumber     uint64      `json:"tick_number"`
	Timestamp      time.Time   `json:"timestamp"`
}

// PlayerConnection represents a WebSocket connection for a player
type PlayerConnection struct {
	SessionID       string
	PlayerID        PlayerID
	UserID          int64
	Username        string
	Conn            *websocket.Conn
	Send            chan *ServerMessage
	Session         *GameSession
	Manager         *GameStateManager
	mu              sync.Mutex
	closed          bool
}

// NewPlayerConnection creates a new player connection
func NewPlayerConnection(sessionID string, playerID PlayerID, userID int64, username string, conn *websocket.Conn, session *GameSession, manager *GameStateManager) *PlayerConnection {
	return &PlayerConnection{
		SessionID: sessionID,
		PlayerID:  playerID,
		UserID:    userID,
		Username:  username,
		Conn:      conn,
		Send:      make(chan *ServerMessage, 256),
		Session:   session,
		Manager:   manager,
		closed:    false,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (pc *PlayerConnection) ReadPump() {
	defer func() {
		pc.Close()
	}()

	pc.Conn.SetReadDeadline(time.Now().Add(PongWait))
	pc.Conn.SetPongHandler(func(string) error {
		pc.Conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})
	pc.Conn.SetReadLimit(MaxMessageSize)

	for {
		_, message, err := pc.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for player %d in session %s: %v", pc.PlayerID, pc.SessionID, err)
			}
			break
		}

		// Parse client message
		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			log.Printf("Failed to parse message from player %d in session %s: %v", pc.PlayerID, pc.SessionID, err)
			pc.SendError("Invalid message format", clientMsg.Action)
			continue
		}

		// Queue the action for the game loop to process
		pc.Session.GameLoop.QueueEvent(GameEvent{
			Type:      EventClientAction,
			PlayerID:  pc.PlayerID,
			Message:   &clientMsg,
			Conn:      pc,
			Timestamp: time.Now(),
		})
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (pc *PlayerConnection) WritePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		pc.Close()
	}()

	for {
		select {
		case message, ok := <-pc.Send:
			pc.Conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				// Channel closed
				pc.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send JSON message
			if err := pc.Conn.WriteJSON(message); err != nil {
				log.Printf("Failed to write message to player %d in session %s: %v", pc.PlayerID, pc.SessionID, err)
				return
			}

		case <-ticker.C:
			pc.Conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := pc.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SendMessage sends a message to the client
func (pc *PlayerConnection) SendMessage(msg *ServerMessage) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return fmt.Errorf("connection closed")
	}

	select {
	case pc.Send <- msg:
		return nil
	default:
		log.Printf("Send buffer full for player %d in session %s", pc.PlayerID, pc.SessionID)
		return fmt.Errorf("send buffer full")
	}
}

// SendError sends an error message to the client
func (pc *PlayerConnection) SendError(errorMsg string, action GameAction) {
	msg := &ServerMessage{
		MessageType:  "error",
		Action:       action,
		Result:       "failure",
		ErrorMessage: errorMsg,
		Timestamp:    time.Now(),
	}
	pc.SendMessage(msg)
}

// SendStateUpdate sends a state update to the client
func (pc *PlayerConnection) SendStateUpdate(action GameAction, stateView *PlayerView) {
	pc.SendStateUpdateWithParams(action, stateView, nil)
}

// SendStateUpdateWithParams sends a state update with action parameters
func (pc *PlayerConnection) SendStateUpdateWithParams(action GameAction, stateView *PlayerView, params interface{}) {
	msg := &ServerMessage{
		MessageType:    "state_update",
		Action:         action,
		Params:         params,
		Result:         "success",
		StateView:      stateView,
		SequenceNumber: stateView.SequenceNumber,
		Timestamp:      time.Now(),
	}
	pc.SendMessage(msg)
}

// SendActionAck sends an acknowledgment for a client-initiated action
func (pc *PlayerConnection) SendActionAck(action GameAction, stateView *PlayerView) {
	msg := &ServerMessage{
		MessageType:    "action_result",
		Action:         action,
		Result:         "success",
		StateView:      stateView,
		SequenceNumber: stateView.SequenceNumber,
		Timestamp:      time.Now(),
	}
	pc.SendMessage(msg)
}

// Close closes the connection
func (pc *PlayerConnection) Close() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return
	}

	pc.closed = true
	close(pc.Send)
	pc.Conn.Close()

	// Queue disconnect event for the game loop to handle
	if pc.Session != nil && pc.Session.GameLoop != nil {
		pc.Session.GameLoop.QueueEvent(GameEvent{
			Type:      EventPlayerDisconnect,
			PlayerID:  pc.PlayerID,
			Timestamp: time.Now(),
		})
		log.Printf("Player %d disconnected from session %s", pc.PlayerID, pc.SessionID)
	}
}

// ProcessAction is called from main.go for the initial JOIN_GAME action
// before the game loop is fully active. For all other actions, events are
// queued to the game loop via ReadPump.
func (pc *PlayerConnection) ProcessAction(msg *ClientMessage) {
	// Queue to game loop
	pc.Session.GameLoop.QueueEvent(GameEvent{
		Type:      EventClientAction,
		PlayerID:  pc.PlayerID,
		Message:   msg,
		Conn:      pc,
		Timestamp: time.Now(),
	})
}
