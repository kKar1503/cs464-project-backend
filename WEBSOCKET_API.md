# WebSocket API Reference

## Connection

```
GET /ws?session_id=<session_id>&token=<auth_token>
```

Upgrades to WebSocket. The `token` is the auth bearer token from login. The `session_id` is obtained from matchmaking after both players accept.

---

## Client → Server Messages

All client messages follow this format:

```json
{
  "action": "ACTION_NAME",
  "params": { ... },
  "state_hash_after": 0,
  "sequence_number": <last_tick_number_received>
}
```

| Field | Type | Description |
|---|---|---|
| `action` | string | The action to perform |
| `params` | object | Action-specific parameters |
| `state_hash_after` | number | Currently unused (send `0`) |
| `sequence_number` | number | The last `tick_number` received from the server. Must be within 8 ticks of the server's current tick or the action is rejected. |

### Actions

#### `JOIN_GAME`

Sent automatically when connecting. Initializes the player in the session. When both players have joined, the game transitions to `PRE_TURN` phase.

**Params:** `{}` (empty)

**When:** On connection (sent automatically by server)

---

#### `SELECT_CARD`

Move a single card from the draw pile into your hand. Can be called multiple times during pre-turn (up to hand max of 4).

**Params:**
```json
{
  "card_id": 3
}
```

| Field | Type | Description |
|---|---|---|
| `card_id` | int | Card ID to move from draw pile to hand |

**When:** Only during `PRE_TURN` phase (10 seconds).

**Errors:**
- `"can only select cards during PRE_TURN phase"` — wrong phase
- `"hand is full (4/4)"` — hand already has 4 cards
- `"card N not in draw pile"` — card ID not found in your draw pile

---

#### `DESELECT_CARD`

Move a single card from your hand back to the draw pile. Allows changing your mind during the pre-turn phase.

**Params:**
```json
{
  "card_id": 3
}
```

| Field | Type | Description |
|---|---|---|
| `card_id` | int | Card ID to move from hand back to draw pile |

**When:** Only during `PRE_TURN` phase (10 seconds).

**Errors:**
- `"can only deselect cards during PRE_TURN phase"` — wrong phase
- `"card N not in hand"` — card ID not found in your hand

---

#### `CARD_PLACED`

Place a card from your hand onto the board. Costs elixir equal to the card's mana cost. The card is removed from your hand, placed on the board with a 10-second charge timer, and immediately returned to the back of your deck.

**Params:**
```json
{
  "card_id": 3,
  "row": 0,
  "col": 1
}
```

| Field | Type | Description |
|---|---|---|
| `card_id` | int | The card ID to play (must be in your hand) |
| `row` | int | Board row: `0` = front row, `1` = back row |
| `col` | int | Board column: `0`, `1`, or `2` |

**When:** During `ACTIVE` phase.

**Errors:**
- `"card N not in hand"` — card not found in your hand
- `"not enough elixir: have X, need Y"` — insufficient elixir
- `"board position [R][C] is occupied"` — slot already has a card
- `"invalid board position: row R, col C"` — out of bounds

---

#### `SURRENDER`

Forfeit the game. The opponent is declared the winner and the game ends immediately.

**Params:** `{}` (empty)

**When:** Any time during the game.

---

#### `RECONNECT`

Rejoin a game after disconnecting. Behaves the same as `JOIN_GAME` — sends the current game state.

**Params:** `{}` (empty)

---

## Server → Client Messages

All server messages follow this format:

```json
{
  "message_type": "...",
  "action": "...",
  "params": { ... },
  "result": "success" | "failure",
  "error_message": "...",
  "state_view": { ... },
  "sequence_number": 0,
  "tick_number": 0,
  "timestamp": "2026-04-06T..."
}
```

| Field | Type | Description |
|---|---|---|
| `message_type` | string | `"tick_update"`, `"action_result"`, `"state_update"`, or `"error"` |
| `action` | string | The action that triggered this message |
| `params` | object | Action-specific data (see below) |
| `result` | string | `"success"` or `"failure"` |
| `error_message` | string | Error description (only when `result = "failure"`) |
| `state_view` | object | Player-specific view of the game state |
| `tick_number` | number | Server tick counter (use as `sequence_number` in your next action) |
| `timestamp` | string | ISO 8601 timestamp |

### Message Types

#### `tick_update`

Sent every 250ms (4 times per second) when the game state changes. This is the primary way the client receives game state. Contains the full game state from the player's perspective.

**Params:**

```json
{
  "milli_elixir": 3500,
  "elixir": 3,
  "elixir_cap": 5,
  "your_board": [ ... ],
  "enemy_board": [ ... ],
  "your_hp": 250,
  "enemy_hp": 240,
  "leader_atk": 10,
  "draw_pile": [ ... ],
  "hand": [ ... ],
  "deck_size": 7,
  "phase": "ACTIVE",
  "round_number": 1,
  "winner_id": 0,
  "attack_log": [ ... ]
}
```

| Field | Type | Description |
|---|---|---|
| `milli_elixir` | int | Raw elixir value (1000 = 1 elixir). Use for smooth bar animation. |
| `elixir` | int | Whole elixir available for spending |
| `elixir_cap` | int | Current round's max elixir (starts at 5, +1 per round, max 8) |
| `your_board` | BoardCard[] | Your cards on the board |
| `enemy_board` | BoardCard[] | Opponent's cards on the board |
| `your_hp` | int | Your leader's HP (starts at 250) |
| `enemy_hp` | int | Opponent's leader's HP |
| `leader_atk` | int | Your leader's counterattack damage (10) |
| `draw_pile` | HandCard[] | Cards in your draw pile (visible, selectable during pre-turn) |
| `hand` | HandCard[] | Cards in your hand (playable during active phase) |
| `deck_size` | int | Number of cards remaining in your deck (not visible) |
| `phase` | string | Current game phase (see Phases below) |
| `round_number` | int | Current round number |
| `winner_id` | int | Player ID of the winner (`1` or `2`). `0` if game is not over. |
| `combat_log` | CombatEvent[] | Combat events that occurred this tick (attacks, counters, effects, deaths). Empty on most ticks. |

---

#### `action_result`

Sent in response to a client action (`CARD_PLACED`, `SELECT_CARDS`, etc.) to acknowledge it was processed.

Contains `state_view` with the updated game state after the action.

---

#### `state_update`

Sent when a server-initiated event occurs (opponent disconnect/reconnect, game start, round transitions).

Contains `state_view` with the updated game state.

---

#### `error`

Sent when a client action fails validation.

```json
{
  "message_type": "error",
  "action": "CARD_PLACED",
  "result": "failure",
  "error_message": "not enough elixir: have 2, need 3"
}
```

---

## Data Types

### BoardCard

A card currently on the board (yours or the enemy's).

```json
{
  "card_id": 3,
  "card_name": "Earth Golem",
  "colour": "Grey",
  "current_health": 7,
  "max_health": 10,
  "card_attack": 10,
  "charge_ticks_remaining": 25,
  "charge_ticks_total": 40,
  "row": 0,
  "col": 1
}
```

| Field | Type | Description |
|---|---|---|
| `card_id` | int | Unique card identifier |
| `card_name` | string | Display name |
| `colour` | string | Card colour (Grey, Red, Blue, Green, Purple, Yellow) |
| `current_health` | int | Current HP (decreases when attacked) |
| `max_health` | int | Maximum HP (original value when placed) |
| `card_attack` | int | Damage dealt when this card attacks |
| `charge_ticks_remaining` | int | Ticks until next attack. At 0, the card attacks. |
| `charge_ticks_total` | int | Total ticks for a full charge (always 40 = 10 seconds) |
| `row` | int | Board row: `0` = front, `1` = back |
| `col` | int | Board column: `0`, `1`, `2` |

**Charge progress:** `1 - (charge_ticks_remaining / charge_ticks_total)` gives 0.0→1.0 progress.

---

### HandCard

A card in the draw pile or hand.

```json
{
  "card_id": 3,
  "card_name": "Earth Golem",
  "colour": "Grey",
  "mana_cost": 3,
  "attack": 10,
  "hp": 10
}
```

| Field | Type | Description |
|---|---|---|
| `card_id` | int | Unique card identifier |
| `card_name` | string | Display name |
| `colour` | string | Card colour |
| `mana_cost` | int | Elixir cost to play this card |
| `attack` | int | Attack power when placed on board |
| `hp` | int | Health points when placed on board |

---

### CombatEvent

A combat-related event that occurred this tick. Multiple events can occur per tick. Only present in ticks where combat happened.

**Example — attack hitting leader with counterattack:**
```json
[
  {
    "type": "attack",
    "source_player_id": 12345,
    "source_card_id": 3,
    "source_row": 0,
    "source_col": 1,
    "target_is_leader": true,
    "value": 10
  },
  {
    "type": "counter_attack",
    "target_card_id": 3,
    "target_row": 0,
    "target_col": 1,
    "value": 10,
    "message": "Leader counterattack"
  }
]
```

**Example — attack hitting an enemy card:**
```json
[
  {
    "type": "attack",
    "source_player_id": 12345,
    "source_card_id": 3,
    "source_row": 0,
    "source_col": 1,
    "target_card_id": 7,
    "target_row": 0,
    "target_col": 1,
    "value": 10
  }
]
```

| Field | Type | Description |
|---|---|---|
| `type` | string | Event type (see table below) |
| `source_player_id` | int | User ID of the player who triggered the event. Compare with your user ID to determine friend/foe. |
| `source_card_id` | int | Card ID that caused the event |
| `source_row` | int | Row of the source card |
| `source_col` | int | Column of the source card |
| `target_card_id` | int | Card ID of the target. `0` if target is leader. |
| `target_row` | int | Row of the target card |
| `target_col` | int | Column of the target card |
| `target_is_leader` | bool | `true` if the target is the leader |
| `value` | int | Primary value (damage, heal amount, buff amount, etc.) |
| `value_hp` | int | Secondary value for effects that modify both ATK and HP |
| `card_name` | string | Card name for context (e.g. transformations) |
| `message` | string | Human-readable description |

**Event Types:**

| Type | Description |
|---|---|
| `attack` | A card auto-attacked a target (card or leader) |
| `counter_attack` | Leader dealt damage back to an attacking card |
| `summon_effect` | An effect triggered when a card was placed on the board |
| `on_attack` | An effect triggered when a card attacked |
| `on_damaged` | An effect triggered when a card took damage |
| `on_death` | An effect triggered when a card died |
| `buff` | A positive stat modification (ATK/HP increase) |
| `debuff` | A negative stat modification (ATK/HP decrease) |
| `heal` | HP restored to a card |
| `card_death` | A card was removed from the board (HP ≤ 0) |
| `transform` | A card was transformed into another card |
| `bounce` | A card was returned to the opponent's hand |
| `summon` | A new card was summoned onto the board by an effect |

Note: Currently only `attack` and `counter_attack` are implemented. Other types are defined for future card effect implementation.

---

### StateView

Included in `action_result` and `state_update` messages. Player-specific view of the game.

```json
{
  "session_id": "abc-123",
  "phase": "ACTIVE",
  "turn_number": 1,
  "current_player": 1,
  "sequence_number": 5,
  "your_user_id": 12345,
  "your_username": "player1",
  "opponent_user_id": 67890,
  "opponent_username": "player2",
  "opponent_connected": true,
  "tick_number": 42,
  "state_hash": 1234567890
}
```

---

## Game Phases

| Phase | Description | Duration |
|---|---|---|
| `WAITING_FOR_PLAYERS` | Waiting for both players to connect via WebSocket | — |
| `INITIALIZING` | Brief transition while game state is set up | Instant |
| `PRE_TURN` | Draw pile is topped up. Players select cards from draw pile to hand. | 10 seconds |
| `ACTIVE` | Both players act simultaneously — place cards, elixir charges, cards auto-attack. | 30 seconds |
| `GAME_OVER` | A leader's HP reached 0. `winner_id` indicates the winner. | Final |

**Round cycle:** `PRE_TURN` (10s) → `ACTIVE` (30s) → `PRE_TURN` (10s) → `ACTIVE` (30s) → ...

---

## Card Lifecycle

```
Deck (shuffled queue)
  ↓ Pre-turn: up to 5 cards from front of deck (draw pile max 8)
Draw Pile (visible to player)
  ↓ SELECT_CARDS: player picks up to 4 cards (hand max 4)
Hand (playable cards)
  ↓ CARD_PLACED: card goes to board, costs elixir
Board (card charges for 10s, then auto-attacks)
  ↓ Immediately when played (not when card dies)
Back of Deck → repeats
```

---

## Combat

- Cards auto-attack every 10 seconds (40 ticks) after being placed
- Attacks target the first enemy card in the same column: front row → back row → leader
- Only the leader counterattacks (deals `leader_atk` damage back to the attacker)
- Cards with HP ≤ 0 are removed from the board
- When a leader's HP ≤ 0, the game ends

---

## Elixir

- Charges at 1 elixir per 5 seconds (50 milliElixir per tick)
- Starting amount: 3 elixir
- Cap per round: starts at 5, increases by 1 each round, max 8
- Carries over between rounds (no reset)
- Only charges during `ACTIVE` phase
