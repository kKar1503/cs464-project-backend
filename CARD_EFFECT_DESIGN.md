# Card Effect System Design

## Overview

This document describes how to model the card effect system using **data-driven design** — store
card ability definitions in the DB, and use Go **interfaces + a factory registry** to resolve them
into polymorphic runtime behaviors. No giant switch statements, no deep inheritance trees.

---

## 1. Effect Taxonomy (from card_data)

Every card ability falls into one of four **triggers** and one of several **effect types**:

### Triggers

| Trigger       | When it fires                          |
|---------------|----------------------------------------|
| `summon`      | Card enters the battlefield            |
| `on_attack`   | Card performs an attack                 |
| `on_damaged`  | Card receives damage                   |
| `on_death`    | Card is destroyed                      |

### Effect Types

| Effect Type           | Description                                      | Cards                                          |
|-----------------------|--------------------------------------------------|-------------------------------------------------|
| `stats_change`        | Modify attack/HP of target(s)                    | Dwarf, Nymph, Angel, Penguin, Plague Doctor, Travelling Merchant, Dryad, Quetzalcoatl, Apache, Archangel |
| `deal_damage`         | Deal flat damage to unit(s) or leader            | Bombadier, Dinosaur                            |
| `destroy`             | Destroy a unit (conditional)                     | Town Hero, Dullahan                            |
| `bounce`              | Return a unit to opponent's hand                 | Pufferfish, Swamp Ogre                         |
| `transform`           | Change a unit into a different card               | Magic Swordman, Pig                            |
| `summon_units`        | Spawn token units on the board                   | Alpha Wolf                                     |
| `set_hp`              | Set all units' HP to a fixed value               | Shikigami                                      |
| `elixir_overflow`     | Increase max elixir/mana                         | Living Tree                                    |
| `reset_attack`        | Reset an enemy's attack gauge                    | Cat Sith                                       |
| `double_attack_speed` | Double a unit's attack speed                     | Barbarian, Ninja                               |
| `random_target`       | Override attack targeting to random enemy/all     | Apprentice Magician, Krazy Kraken              |
| `skip_front_row`      | Bypass the front row when attacking              | Holy Spear Knight                              |
| `reflect`             | Deal attacker's damage back to them              | Big Whale                                      |
| `shield`              | Flat damage reduction on incoming damage         | Paladin                                        |
| `self_damage`         | Deal damage to self on attack                    | Glass Bones                                    |
| `damage_leader`       | Deal damage to own leader                        | Traitor                                        |
| `destroy_random`      | Destroy a random enemy unit                      | Witch                                          |

### Targeting Modes (orthogonal to effect type)

Effects that modify stats or deal damage need a **target selector**. This is a separate concept
stored in the `params` JSON:

| Target            | Meaning                                      |
|-------------------|----------------------------------------------|
| `self`            | The card itself                              |
| `adjacent`        | Cards immediately left/right on the board    |
| `vertical`        | Cards in the same column (above/below)       |
| `all_allies`      | All friendly units                           |
| `all_enemies`     | All enemy units                              |
| `opponent_column`  | Enemy units in the same column              |
| `opponent_back_row`| Enemy units in the back row                 |
| `enemy_in_front`  | The enemy unit directly in front             |
| `random_enemy`    | One random enemy unit or leader              |
| `random_all`      | One random unit or leader (either side)      |
| `same_colour`     | All units sharing this card's colour         |

---

## 2. DB Schema

### What stays the same

The existing `cards` table is fine for the static card template data (name, colour, rarity, cost,
icon). The `card_stats` table is fine for per-level base attack/HP.

### Revised `card_abilities` table

The current `card_abilities` table has the right idea but needs clearer column semantics:

```sql
ALTER TABLE card_abilities
    ADD COLUMN effect_type VARCHAR(50) NOT NULL AFTER trigger_type,
    CHANGE COLUMN abilties params JSON NOT NULL,
    DROP COLUMN effect;
```

Or as a fresh migration:

```sql
DROP TABLE IF EXISTS card_abilities;

CREATE TABLE card_abilities (
    ability_id    INT AUTO_INCREMENT PRIMARY KEY,
    card_id       INT NOT NULL,
    trigger_type  VARCHAR(20) NOT NULL,   -- 'summon', 'on_attack', 'on_damaged', 'on_death'
    effect_type   VARCHAR(50) NOT NULL,   -- 'stats_change', 'deal_damage', etc.
    params        JSON NOT NULL,          -- effect-specific configuration
    FOREIGN KEY (card_id) REFERENCES cards(card_id) ON DELETE CASCADE,
    INDEX idx_card_id (card_id),
    INDEX idx_trigger_type (trigger_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### Example rows

```sql
-- Dwarf (id=8): summon → give vertically adjacent units +10/0
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(8, 'summon', 'stats_change', '{"target":"vertical","attack":10,"hp":0}');

-- Big Whale (id=18): on_damaged → reflect damage
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(18, 'on_damaged', 'reflect', '{}');

-- Pig (id=1): summon → 1/2048 chance to transform into Technoblade
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(1, 'summon', 'transform', '{"chance":2048,"into_card_id":37}');

-- Cat Sith (id=34): TWO abilities — summon + on_attack
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(34, 'summon', 'reset_attack', '{"target":"enemy_in_front"}'),
(34, 'on_attack', 'reset_attack', '{"target":"enemy_in_front"}');

-- Town Hero (id=5): summon → destroy all same colour, +5/+5 per destroyed
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(5, 'summon', 'destroy', '{"target":"same_colour","self_buff_per_kill":{"attack":5,"hp":5}}');

-- Paladin (id=35): on_damaged → reduce incoming damage by 10
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(35, 'on_damaged', 'shield', '{"reduction":10}');

-- Alpha Wolf (id=23): summon → spawn 2 wolves (card_id=38)
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(23, 'summon', 'summon_units', '{"card_id":38,"count":2}');

-- Travelling Merchant (id=6): summon → +5/+5 per unique colour in your deck
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(6, 'summon', 'stats_change', '{"target":"self","per_colour_in_deck":{"attack":5,"hp":5}}');

-- Bombadier (id=9): summon → deal 15 to enemy leader
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(9, 'summon', 'deal_damage', '{"target":"enemy_leader","damage":15}');

-- Dinosaur (id=12): summon → deal 15 to ALL units on the battlefield
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(12, 'summon', 'deal_damage', '{"target":"all_units","damage":15}');
```

### All card ability inserts

```sql
-- Cards with no effect (just "basic" summon): Farmer(2), Mercenary(3), Town Guard(4) — no rows needed

-- Grey
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(1,  'summon',     'transform',           '{"chance":2048,"into_card_id":37}'),
(5,  'summon',     'destroy',             '{"target":"same_colour","self_buff_per_kill":{"attack":5,"hp":5}}'),
(6,  'summon',     'stats_change',        '{"target":"self","per_colour_in_deck":{"attack":5,"hp":5}}');

-- Red
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(7,  'summon',     'double_attack_speed', '{"chance":2048}'),
(8,  'summon',     'stats_change',        '{"target":"vertical","attack":10,"hp":0}'),
(9,  'summon',     'deal_damage',         '{"target":"enemy_leader","damage":15}'),
(10, 'summon',     'double_attack_speed', '{}'),
(11, 'on_damaged', 'stats_change',        '{"target":"self","attack":10,"hp":0}'),
(12, 'summon',     'deal_damage',         '{"target":"all_units","damage":15}');

-- Blue
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(13, 'summon',     'stats_change',        '{"target":"opponent_column","attack":-10,"hp":0}'),
(14, 'on_attack',  'random_target',       '{"pool":"enemy"}'),
(15, 'on_attack',  'random_target',       '{"pool":"all"}'),
(16, 'summon',     'bounce',              '{"target":"enemy_in_front"}'),
(17, 'summon',     'transform',           '{"target":"enemy_in_front","into_card_id":1}'),
(18, 'on_damaged', 'reflect',             '{}');

-- Green
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(19, 'summon',     'bounce',              '{"target":"enemy_front_row","chance":2048}'),
(20, 'summon',     'stats_change',        '{"target":"adjacent","attack":10,"hp":10}'),
(21, 'summon',     'elixir_overflow',     '{"amount":1}'),
(22, 'summon',     'stats_change',        '{"target":"self","attack":10,"hp":10,"condition":"enemy_in_front"}'),
(23, 'summon',     'summon_units',        '{"card_id":38,"count":2}'),
(24, 'summon',     'stats_change',        '{"target":"all_allies","attack":10,"hp":10}');

-- Purple
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(25, 'on_attack',  'self_damage',         '{"damage":10}'),
(26, 'on_death',   'damage_leader',       '{"target":"own_leader","damage":20}'),
(27, 'summon',     'stats_change',        '{"target":"opponent_back_row","attack":-15,"hp":-10}'),
(28, 'on_death',   'destroy_random',      '{"target":"random_enemy"}'),
(29, 'summon',     'destroy',             '{"target":"enemy_in_front"}'),
(30, 'summon',     'set_hp',              '{"target":"all_units","hp":1}');

-- Yellow
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(31, 'summon',     'double_attack_speed', '{"chance":2048,"invert":true}'),
(32, 'summon',     'stats_change',        '{"target":"vertical","attack":0,"hp":10}'),
(33, 'on_attack',  'skip_front_row',      '{}'),
(34, 'summon',     'reset_attack',        '{"target":"enemy_in_front"}'),
(34, 'on_attack',  'reset_attack',        '{"target":"enemy_in_front"}'),
(35, 'on_damaged', 'shield',              '{"reduction":10}'),
(36, 'on_attack',  'stats_change',        '{"target":"adjacent","attack":0,"hp":10}');

-- Tokens: Technoblade(37) and Wolf(38) have no abilities — no rows needed
```

---

## 3. Go Domain Model

### Template types (loaded once from DB, shared/immutable)

```go
// shared/card.go

// CardDefinition is the immutable template loaded from the DB at startup.
type CardDefinition struct {
    CardID    int
    Name      string
    Colour    string
    Rarity    string
    Cost      int
    BaseAtk   int
    BaseHP    int
    Abilities []AbilityDefinition
}

// AbilityDefinition maps 1:1 to a card_abilities row.
type AbilityDefinition struct {
    TriggerType string          // "summon", "on_attack", "on_damaged", "on_death"
    EffectType  string          // "stats_change", "deal_damage", etc.
    Params      json.RawMessage // effect-specific config
}
```

### Runtime types (per-game, mutable)

```go
// gameplay/card_instance.go

// CardInstance is a live card on the board with mutable state.
type CardInstance struct {
    InstanceID  int              // unique within a game session
    Definition  *CardDefinition  // pointer back to shared template
    CurrentAtk  int
    CurrentHP   int
    Position    BoardPosition
    Abilities   []Ability        // resolved from AbilityDefinition via factory
}
```

---

## 4. The Ability Interface (polymorphism)

```go
// effects/ability.go

// Ability is the core polymorphic interface.
// Each effect_type in the DB becomes a struct that implements this.
type Ability interface {
    TriggerType() string
    Execute(ctx *EffectContext) error
}

// EffectContext provides everything an ability needs to read/mutate game state.
type EffectContext struct {
    Source       *CardInstance
    Target       *CardInstance    // may be nil for AoE / self effects
    Board        *Board
    ActivePlayer *PlayerState
    Opponent     *PlayerState
    RNG          *rand.Rand       // deterministic seed for replays
}
```

---

## 5. Factory Registry (effect_type → struct)

```go
// effects/registry.go

type EffectFactory func(trigger string, params json.RawMessage) (Ability, error)

var registry = map[string]EffectFactory{
    "stats_change":        NewStatsChange,
    "deal_damage":         NewDealDamage,
    "destroy":             NewDestroy,
    "bounce":              NewBounce,
    "transform":           NewTransform,
    "summon_units":        NewSummonUnits,
    "set_hp":              NewSetHP,
    "elixir_overflow":     NewElixirOverflow,
    "reset_attack":        NewResetAttack,
    "double_attack_speed": NewDoubleAttackSpeed,
    "random_target":       NewRandomTarget,
    "skip_front_row":      NewSkipFrontRow,
    "reflect":             NewReflect,
    "shield":              NewShield,
    "self_damage":         NewSelfDamage,
    "damage_leader":       NewDamageLeader,
    "destroy_random":      NewDestroyRandom,
}

// ResolveAbility turns a DB row into a live Ability.
func ResolveAbility(def AbilityDefinition) (Ability, error) {
    factory, ok := registry[def.EffectType]
    if !ok {
        return nil, fmt.Errorf("unknown effect type: %s", def.EffectType)
    }
    return factory(def.TriggerType, def.Params)
}
```

---

## 6. Example Concrete Effect

```go
// effects/stats_change.go

type StatsChangeEffect struct {
    trigger        string
    Target         string `json:"target"`         // "self", "adjacent", "vertical", ...
    AtkDelta       int    `json:"attack"`
    HPDelta        int    `json:"hp"`
    Condition      string `json:"condition"`       // optional: "enemy_in_front"
    PerColourBuff  *Buff  `json:"per_colour_in_deck"` // optional: Travelling Merchant
}

type Buff struct {
    Attack int `json:"attack"`
    HP     int `json:"hp"`
}

func NewStatsChange(trigger string, params json.RawMessage) (Ability, error) {
    e := &StatsChangeEffect{trigger: trigger}
    if err := json.Unmarshal(params, e); err != nil {
        return nil, err
    }
    return e, nil
}

func (e *StatsChangeEffect) TriggerType() string { return e.trigger }

func (e *StatsChangeEffect) Execute(ctx *EffectContext) error {
    // Check optional condition
    if e.Condition == "enemy_in_front" && !hasEnemyInFront(ctx) {
        return nil
    }

    // Calculate deltas (may be dynamic for Travelling Merchant)
    atk, hp := e.AtkDelta, e.HPDelta
    if e.PerColourBuff != nil {
        colours := countColoursInDeck(ctx.ActivePlayer)
        atk = e.PerColourBuff.Attack * colours
        hp = e.PerColourBuff.HP * colours
    }

    // Resolve targets and apply
    for _, t := range resolveTargets(e.Target, ctx) {
        t.CurrentAtk += atk
        t.CurrentHP += hp
    }
    return nil
}
```

---

## 7. Trigger Dispatch (in gameplay loop)

```go
// gameplay/triggers.go

func (g *Game) fireTrigger(trigger string, source *CardInstance) {
    for _, ability := range source.Abilities {
        if ability.TriggerType() == trigger {
            ctx := g.makeEffectContext(source)
            ability.Execute(ctx)
        }
    }
}

// Usage:
func (g *Game) OnCardPlayed(card *CardInstance)              { g.fireTrigger("summon", card) }
func (g *Game) OnAttack(attacker *CardInstance)              { g.fireTrigger("on_attack", attacker) }
func (g *Game) OnDamaged(card *CardInstance)                 { g.fireTrigger("on_damaged", card) }
func (g *Game) OnDeath(card *CardInstance)                   { g.fireTrigger("on_death", card) }
```

---

## 8. Adding a New Card

### If its abilities use existing effect types → DB only

Just insert into `cards`, `card_stats`, and `card_abilities`. Zero Go changes.

### If it needs a new effect type

1. Add a new struct implementing `Ability` in `effects/`
2. Register it in the factory registry
3. Insert the DB rows with the new `effect_type` string

---

## 9. What goes where

| Stored in DB                              | Implemented in Go code                          |
|-------------------------------------------|-------------------------------------------------|
| Card name, stats, cost, rarity, icon      | `Ability` interface                             |
| `trigger_type` (when it fires)            | Concrete effect structs (one per `effect_type`) |
| `effect_type` (what it does)              | Factory registry (`effect_type` → struct)       |
| `params` JSON (targeting, amounts, odds)  | Target resolver (`resolveTargets`)              |
|                                           | Trigger dispatch (`fireTrigger`)                |

---

## 10. Design Principles

- **Data-driven**: New cards with existing effects = DB inserts only
- **Composition over inheritance**: Cards have a *list* of abilities, not a type hierarchy. Cat Sith naturally has two abilities
- **Interface polymorphism**: `Ability` interface + factory gives runtime dispatch without `switch` hell
- **Template vs Instance**: `CardDefinition` (immutable, shared) vs `CardInstance` (mutable, per-game) — clean separation for serialization and replays
- **Orthogonal targeting**: Target selectors ("adjacent", "vertical", "all_allies") are reusable across multiple effect types
- **Deterministic RNG**: Seeded `rand.Rand` in `EffectContext` for reproducible replays of chance effects (Pig, Barbarian, Swamp Ogre)
