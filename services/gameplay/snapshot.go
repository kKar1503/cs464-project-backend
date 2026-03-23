package main

import (
	"encoding/json"
	"sync"
	"time"
)

// StateSnapshot represents a point-in-time snapshot of the game state
type StateSnapshot struct {
	Player1SequenceNumber int64      `json:"player1_sequence_number"`
	Player2SequenceNumber int64      `json:"player2_sequence_number"`
	Timestamp             time.Time  `json:"timestamp"`
	Action                GameAction `json:"action"`
	ActorPlayerID         PlayerID   `json:"actor_player_id"`
	StateData             []byte     `json:"state_data"` // JSON serialized GameState
	Player1Hash           uint64     `json:"player1_hash"`
	Player2Hash           uint64     `json:"player2_hash"`
}

// CircularBuffer implements a fixed-size circular buffer for state snapshots
type CircularBuffer struct {
	buffer   []*StateSnapshot
	size     int
	head     int // Index where next write will occur
	count    int // Number of items in buffer
	mu       sync.RWMutex
}

// NewCircularBuffer creates a new circular buffer with the specified size
func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		buffer: make([]*StateSnapshot, size),
		size:   size,
		head:   0,
		count:  0,
	}
}

// Push adds a new snapshot to the buffer (overwrites oldest if full)
func (cb *CircularBuffer) Push(snapshot *StateSnapshot) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.buffer[cb.head] = snapshot
	cb.head = (cb.head + 1) % cb.size

	if cb.count < cb.size {
		cb.count++
	}
}

// GetAll returns all snapshots in chronological order (oldest to newest)
func (cb *CircularBuffer) GetAll() []*StateSnapshot {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.count == 0 {
		return nil
	}

	result := make([]*StateSnapshot, cb.count)

	// Calculate starting position (oldest entry)
	start := cb.head - cb.count
	if start < 0 {
		start += cb.size
	}

	for i := 0; i < cb.count; i++ {
		idx := (start + i) % cb.size
		result[i] = cb.buffer[idx]
	}

	return result
}

// GetLatest returns the most recent N snapshots
func (cb *CircularBuffer) GetLatest(n int) []*StateSnapshot {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.count == 0 {
		return nil
	}

	if n > cb.count {
		n = cb.count
	}

	result := make([]*StateSnapshot, n)

	// Start from most recent and go backwards
	for i := 0; i < n; i++ {
		idx := cb.head - 1 - i
		if idx < 0 {
			idx += cb.size
		}
		result[n-1-i] = cb.buffer[idx]
	}

	return result
}

// GetBySequence retrieves a snapshot by player sequence number
func (cb *CircularBuffer) GetBySequence(playerID PlayerID, seqNum int64) *StateSnapshot {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	for i := 0; i < cb.count; i++ {
		idx := (cb.head - cb.count + i + cb.size) % cb.size
		if playerID == Player1 && cb.buffer[idx].Player1SequenceNumber == seqNum {
			return cb.buffer[idx]
		}
		if playerID == Player2 && cb.buffer[idx].Player2SequenceNumber == seqNum {
			return cb.buffer[idx]
		}
	}
	return nil
}

// Count returns the number of snapshots currently in the buffer
func (cb *CircularBuffer) Count() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.count
}

// Clear removes all snapshots from the buffer
func (cb *CircularBuffer) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.head = 0
	cb.count = 0
	cb.buffer = make([]*StateSnapshot, cb.size)
}

// SnapshotManager manages snapshots for a game session
type SnapshotManager struct {
	sessionID      string
	circularBuffer *CircularBuffer
	mu             sync.RWMutex
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(sessionID string, bufferSize int) *SnapshotManager {
	return &SnapshotManager{
		sessionID:      sessionID,
		circularBuffer: NewCircularBuffer(bufferSize),
	}
}

// TakeSnapshot creates and stores a snapshot of the current game state
func (sm *SnapshotManager) TakeSnapshot(gs *GameState, action GameAction, actorPlayerID PlayerID) error {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	// Serialize the full game state
	stateData, err := json.Marshal(gs)
	if err != nil {
		return err
	}

	// Generate player views and hashes
	player1View := gs.GetPlayerView(Player1)
	player2View := gs.GetPlayerView(Player2)

	snapshot := &StateSnapshot{
		Player1SequenceNumber: gs.Player1SequenceNumber,
		Player2SequenceNumber: gs.Player2SequenceNumber,
		Timestamp:             time.Now(),
		Action:                action,
		ActorPlayerID:         actorPlayerID,
		StateData:             stateData,
		Player1Hash:           player1View.StateHash,
		Player2Hash:           player2View.StateHash,
	}

	sm.circularBuffer.Push(snapshot)
	return nil
}

// GetAllSnapshots returns all snapshots in chronological order
func (sm *SnapshotManager) GetAllSnapshots() []*StateSnapshot {
	return sm.circularBuffer.GetAll()
}

// GetRecentSnapshots returns the N most recent snapshots
func (sm *SnapshotManager) GetRecentSnapshots(n int) []*StateSnapshot {
	return sm.circularBuffer.GetLatest(n)
}

// GetSnapshotBySequence retrieves a specific snapshot by player sequence number
func (sm *SnapshotManager) GetSnapshotBySequence(playerID PlayerID, seqNum int64) *StateSnapshot {
	return sm.circularBuffer.GetBySequence(playerID, seqNum)
}

// SnapshotCount returns the number of snapshots stored
func (sm *SnapshotManager) SnapshotCount() int {
	return sm.circularBuffer.Count()
}
