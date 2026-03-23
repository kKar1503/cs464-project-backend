# Click Counter Demo - Gameplay State Engine Usage Guide

This is a simple demonstration game that shows how to interact with the gameplay state engine. It implements a turn-based click counter where players take turns clicking to increment their counter.

## Game Rules

1. Two players join a game session
2. Players take turns (Player 1 starts first)
3. During your turn, you can click as many times as you want
4. Each click increments your click counter
5. When you're done clicking, you end your turn
6. The other player can then take their turn
7. Each turn has a 90-second timeout

## WebSocket Protocol

### Connection
```
ws://localhost:8083/ws?session_id=<session_id>&user_id=<user_id>&username=<username>&player_id=<1|2>
```

### Client Actions

All client messages follow this format:
```json
{
    "action": "ACTION_NAME",
    "params": {},
    "state_hash_after": 12345678901234,
    "sequence_number": 42
}
```

## Available Actions

### 1. JOIN_GAME
Join the game session and initialize your state.

**Request:**
```json
{
    "action": "JOIN_GAME",
    "params": {},
    "state_hash_after": 0,
    "sequence_number": 0
}
```

**Response (ACK):**
```json
{
    "message_type": "action_result",
    "action": "JOIN_GAME",
    "result": "success",
    "state_view": {
        "session_id": "abc123",
        "phase": "PLAYER1_TURN",
        "turn_number": 1,
        "current_player": 1,
        "sequence_number": 1,
        "your_user_id": 123,
        "your_username": "Alice",
        "your_game_data": {
            "click_count": 0
        },
        "opponent_user_id": 456,
        "opponent_username": "Bob",
        "opponent_connected": true,
        "opponent_game_data": {
            "click_count": 0
        },
        "state_hash": 9876543210
    },
    "sequence_number": 1,
    "timestamp": "2026-03-23T..."
}
```

### 2. CLICK
Increment your click counter (only during your turn).

**Request:**
```json
{
    "action": "CLICK",
    "params": {},
    "state_hash_after": 1234567890,
    "sequence_number": 1
}
```

**Response (ACK):**
```json
{
    "message_type": "action_result",
    "action": "CLICK",
    "result": "success",
    "state_view": {
        "your_game_data": {
            "click_count": 5
        },
        "opponent_game_data": {
            "click_count": 3
        },
        ...
    },
    "sequence_number": 2
}
```

**Opponent receives (state_update):**
```json
{
    "message_type": "state_update",
    "action": "CLICK",
    "result": "success",
    "state_view": {
        "your_game_data": {
            "click_count": 3
        },
        "opponent_game_data": {
            "click_count": 5
        },
        ...
    }
}
```

### 3. END_TURN
End your turn and let the opponent play.

**Request:**
```json
{
    "action": "END_TURN",
    "params": {},
    "state_hash_after": 1234567890,
    "sequence_number": 5
}
```

**Response (ACK):**
```json
{
    "message_type": "action_result",
    "action": "END_TURN",
    "result": "success",
    "state_view": {
        "phase": "PLAYER2_TURN",
        "current_player": 2,
        ...
    },
    "sequence_number": 6
}
```

**Both players receive (TURN_END broadcast):**
```json
{
    "message_type": "state_update",
    "action": "TURN_END",
    "params": {
        "reason": "player_initiated",
        "player_id": 1
    },
    "state_view": { ... }
}
```

### 4. SURRENDER
End the game and declare opponent as winner.

**Request:**
```json
{
    "action": "SURRENDER",
    "params": {},
    "state_hash_after": 1234567890,
    "sequence_number": 10
}
```

## Server-Initiated Actions

These are sent by the server without client request:

### TURN_END
Sent when a turn ends (either by player action or timeout).

```json
{
    "message_type": "state_update",
    "action": "TURN_END",
    "params": {
        "reason": "player_initiated",  // or "timed_out"
        "player_id": 1
    },
    "state_view": { ... }
}
```

### OPPONENT_DISCONNECT / OPPONENT_RECONNECT
Notifies when opponent connection status changes.

```json
{
    "message_type": "state_update",
    "action": "OPPONENT_DISCONNECT",
    "state_view": {
        "opponent_connected": false,
        ...
    }
}
```

## Error Handling

If an action fails, you receive an error message:

```json
{
    "message_type": "error",
    "action": "CLICK",
    "result": "failure",
    "error_message": "not your turn",
    "timestamp": "2026-03-23T..."
}
```

Common errors:
- `"not your turn"` - Tried to perform action outside your turn
- `"Sequence mismatch: expected 5, got 3"` - Client out of sync
- `"State hash mismatch after action"` - Client state diverged from server

## State Hash Computation

The `state_hash_after` field is critical for integrity checking. It must be computed using xxHash64 on the player view after applying the action locally.

**Important:** Compute the hash on YOUR player view (not the full game state). The view includes:
- Session metadata (session_id, phase, turn_number, current_player, sequence_number)
- Your data (user_id, username, game_data)
- Opponent data (user_id, username, connected status, game_data)

The hash ensures your local state matches the server's authoritative state.

## Example Game Flow

### Player 1 Perspective:
```
1. Connect to WebSocket
2. Send JOIN_GAME
   → Receive ACK (your_game_data: {click_count: 0})
   → Receive state_update from server
3. Send CLICK
   → Receive ACK (your_game_data: {click_count: 1})
4. Send CLICK
   → Receive ACK (your_game_data: {click_count: 2})
5. Send END_TURN
   → Receive ACK
   → Both players receive TURN_END (reason: "player_initiated", player_id: 1)
6. Wait for Player 2's turn
7. Receive state_update when Player 2 clicks (opponent_game_data updates)
```

### Player 2 Perspective:
```
1. Connect to WebSocket
2. Send JOIN_GAME
   → Receive ACK (your_game_data: {click_count: 0})
3. Wait (not your turn)
4. Receive state_update when Player 1 clicks (opponent_game_data: {click_count: 1})
5. Receive state_update when Player 1 clicks again (opponent_game_data: {click_count: 2})
6. Receive TURN_END (reason: "player_initiated", player_id: 1)
   → Now it's your turn!
7. Send CLICK
   → Receive ACK (your_game_data: {click_count: 1})
```

## How to Extend This for Your Game

### 1. Define Your Game Data Structure

```go
// In handlers/your_game.go
type YourGameData struct {
    // Your custom fields
    Health     int       `json:"health"`
    Cards      []Card    `json:"cards"`
    Resources  int       `json:"resources"`
}
```

### 2. Create Action Handlers

```go
// In handlers/your_action.go
func HandleYourAction(ctx HandlerContext, msg *ClientMessage) error {
    // Verify turn if needed
    if !ctx.IsPlayerTurn() {
        return fmt.Errorf("not your turn")
    }

    // Parse params from msg.Params
    var params YourActionParams
    if err := json.Unmarshal(msg.Params, &params); err != nil {
        return err
    }

    // Get and update player state
    playerState := ctx.GetPlayerState(ctx.GetPlayerID())

    ctx.LockState()
    defer ctx.UnlockState()

    var gameData YourGameData
    json.Unmarshal(playerState.GetGameData(), &gameData)

    // Apply your game logic
    gameData.Health -= params.Damage

    // Save updated state
    updatedData, _ := json.Marshal(gameData)
    playerState.SetGameData(updatedData)

    // Increment sequence
    ctx.IncrementSequence()

    // Send updates to both players
    myView := ctx.GetPlayerView(ctx.GetPlayerID())
    ctx.SendStateUpdate("YOUR_ACTION", myView)

    opponentView := ctx.GetPlayerView(ctx.GetOpponentID())
    ctx.BroadcastToOpponent("YOUR_ACTION", opponentView)

    return nil
}
```

### 3. Register Your Handler

```go
// In handlers/registry.go
func init() {
    RegisterActionHandler("YOUR_ACTION", HandleYourAction)
}
```

### 4. Initialize Game Data

```go
// In handlers/join.go
initialData := YourGameData{
    Health: 100,
    Cards: []Card{},
    Resources: 10,
}
dataBytes, _ := json.Marshal(initialData)
playerState.SetGameData(dataBytes)
```

## Testing Tips

1. **Use wscat for manual testing:**
   ```bash
   npm install -g wscat
   wscat -c "ws://localhost:8083/ws?session_id=test123&user_id=1&username=Alice&player_id=1"
   ```

2. **Sequence numbers start at 0** and increment with each action from that player

3. **Each player has their own sequence** - Player 1's sequence is independent from Player 2's

4. **State hash must match** - If hashes don't match, server sends error and resyncs client

5. **Turn timer is 90 seconds** - If a player doesn't end their turn, server auto-ends with TURN_END (reason: "timed_out")

## Architecture Benefits

- **Server-authoritative**: Server maintains the truth, clients verify
- **Integrity checking**: Hash validation detects tampering
- **Replay support**: All actions are snapshotted for replay
- **Generic game data**: Easy to adapt for any game logic
- **Turn management**: Built-in turn timer and phase tracking
- **Connection resilience**: Handles disconnect/reconnect gracefully
