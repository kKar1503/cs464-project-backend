# Card Leveling System API

All endpoints require authentication via bearer token unless marked as **internal**.

---

## New Endpoints

### GET /players/me/crystals

Returns the player's current crystal balance.

**Response 200**

```json
{
  "crystals": 1500
}
```

---

### POST /players/me/cards/{cardId}/level-up

Level up a card by consuming copies and crystals.

**Cost formula:**
- Cards: `4 ^ current_level` (4, 16, 64, 256, ...)
- Crystals: `100 * 10 ^ (current_level - 1)` (100, 1000, 10000, 100000, ...)

**Stat scaling formula:**
- `ceil((1 + 0.2 * (level - 1)) * base_stat)`

**Request:** No body required. Card ID is in the URL path.

**Response 200**

```json
{
  "card_id": 1,
  "new_level": 2,
  "cards_consumed": 4,
  "crystals_spent": 100,
  "crystals_left": 1400,
  "quantity_left": 1,
  "pruned_decks": [
    {
      "deck_id": 7,
      "deck_name": "Starter Deck 1",
      "removed": 1
    }
  ]
}
```

`pruned_decks` lists any decks that had excess copies of this card removed because the player no longer owns enough. Empty array if no decks were affected.

**Error responses:**

| Status | Condition |
|--------|-----------|
| 400 | Card already at max level |
| 400 | Not enough copies (e.g. `"need 4 copies, have 2"`) |
| 400 | Not enough crystals (e.g. `"need 100 crystals, have 50"`) |
| 404 | Player does not own this card |
| 409 | Crystal balance or card state changed concurrently (retry) |

---

### POST /players/me/cards/{cardId}/disenchant

Convert extra copies of a max-level card into crystals. At least 1 copy must remain.

**Crystals per copy by rarity:**

| Rarity | Crystals |
|--------|----------|
| common | 10 |
| rare | 50 |
| epic | 100 |
| legendary | 500 |

**Request body**

```json
{
  "quantity": 5
}
```

**Response 200**

```json
{
  "card_id": 1,
  "disenchanted": 5,
  "crystals_awarded": 50,
  "crystals_total": 1550,
  "quantity_left": 3,
  "pruned_decks": []
}
```

**Error responses:**

| Status | Condition |
|--------|-----------|
| 400 | Card is not at max level |
| 400 | Would leave 0 copies (e.g. `"can disenchant at most 7 copies (need to keep 1)"`) |
| 400 | Quantity must be positive |
| 404 | Player does not own this card |
| 409 | Card state changed concurrently (retry) |

---

### GET /internal/validate-active-deck?user_id={id}

**Internal endpoint** called by the matchmaking service before allowing a player to join the queue.

**Response 200 (always)**

Valid deck:
```json
{
  "valid": true
}
```

Invalid deck:
```json
{
  "valid": false,
  "reason": "active deck must have exactly 12 cards, currently has 10"
}
```

Possible reasons:
- `"no active deck set"`
- `"active deck must have exactly 12 cards, currently has N"`
- `"cannot have more than 2 copies of card X"`
- `"deck cannot have more than 1 legendary card"`
- `"you do not own card X"`
- `"you only own N copies of card X but need M"`

---

## Modified Endpoints

### POST /packs/open?pack_id={id}

Now additionally awards crystals and returns the amount.

**Crystals per pack type:**

| Pack Type | Crystals |
|-----------|----------|
| common | 100 |
| rare | 500 |
| epic | 1000 |
| legendary | 5000 |

**Response 200** (new fields: `crystals_awarded`, `crystals_total`)

```json
{
  "pack_id": 42,
  "pack_type": "rare",
  "cards": [
    { "card_id": 3, "card_name": "Farmer", "rarity": "common" },
    { "card_id": 7, "card_name": "Barbarian", "rarity": "rare" }
  ],
  "crystals_awarded": 500,
  "crystals_total": 2000
}
```

---

### GET /players/me/cards

Player card collection response now includes leveling and stat information.

**Response 200** (new fields per card: `max_level`, `attack`, `hp`, `base_attack`, `base_hp`, `next_level_cards_cost`, `next_level_crystals_cost`, `disenchant_crystals_per_copy`)

```json
{
  "cards": [
    {
      "card_id": 1,
      "card_name": "Pig",
      "affiliation": 1,
      "rarity": "common",
      "mana_cost": 3,
      "description": "A humble pig.",
      "icon_url": "/cards/pig.png",
      "level": 2,
      "max_level": 5,
      "quantity": 8,
      "attack": 12,
      "hp": 12,
      "base_attack": 10,
      "base_hp": 10,
      "next_level_cards_cost": 16,
      "next_level_crystals_cost": 1000,
      "disenchant_crystals_per_copy": 10
    }
  ],
  "count": 1
}
```

When `level == max_level`, `next_level_cards_cost` and `next_level_crystals_cost` are both `0`.

---

### GET /players/me/cards/available

Same enriched `PlayerCardResponse` shape as `/players/me/cards` above. The `quantity` field reflects copies not already placed in a specific deck.

---

### POST /queue/join (matchmaking service)

Now validates the player's active deck before allowing queue entry. Returns `400` with a descriptive reason if the deck is invalid or missing.

**New error response (400)**

```json
{
  "error": "Cannot queue: active deck must have exactly 12 cards, currently has 10"
}
```

---

## Reference Tables

### Stat Scaling by Level

| Level | Multiplier | Example (base 10) |
|-------|------------|-------------------|
| 1 | 1.0x | 10 |
| 2 | 1.2x | 12 |
| 3 | 1.4x | 14 |
| 4 | 1.6x | 16 |
| 5 | 1.8x | 18 |

### Level-Up Costs

| From Level | Cards Required | Crystals Required |
|------------|---------------|-------------------|
| 1 -> 2 | 4 | 100 |
| 2 -> 3 | 16 | 1,000 |
| 3 -> 4 | 64 | 10,000 |
| 4 -> 5 | 256 | 100,000 |
