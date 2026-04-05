# Spec Compliance Check

Comparison of `game_logic.pdf` spec against current backend implementation.

## Pre-game Phase (Deck Building)

| Spec Requirement | Status | Implementation Notes |
|---|---|---|
| Choose up to 12 cards in deck | **Done** | `validateDeckCards` enforces max 12 in both create and update |
| Max 2 copies of each card | **Done** | `validateDeckCards` counts per-card and rejects >2 |
| Max 1 legendary card per deck | **Done** | `validateDeckCards` queries card rarities and rejects >1 legendary |

**Files:** `services/deck/handlers.go` (`handleCreateDeck`, `handleUpdateDeck`)

## Pre-turn Phase (Hand Drawing)

| Spec Requirement | Status | Implementation Notes |
|---|---|---|
| Shown 5 out of 12 cards from deck | **Done** | `OfferCards()` picks 5 random from remaining pool |
| Pick up to 4 cards to add to hand | **Done** | `SelectCards()` validates max 4 selection |
| Only 1 colour type per hand selection | **Done** | `SelectCards()` enforces single colour constraint |
| Can't re-take cards already in hand from previous turns | **Done** | `DrawnCardIDs` map tracks previously drawn cards |
| Colourless doesn't count as a colour type | **Done** | Grey/Colourless skipped in colour validation |

**Files:** `services/gameplay/gameplay_state.go` (`HandCard`, `PlayerHand`, `OfferCards`, `SelectCards`), `services/gameplay/handlers/draw_cards.go` (`DRAW_CARDS` action)

## Turn Phase (Combat)

| Spec Requirement | Status | Implementation Notes |
|---|---|---|
| Turn duration: 30 seconds | **Done** | `TurnTimeout = 30 * time.Second` in `turn_timer.go` |
| Energy takes 5 seconds to charge | **Correct** | `ElixirEvery = 20` ticks x 250ms = 5s in `game_loop.go` |
| Start with 3 energy on round 1 | **Done** | `ElixerPlayer1/2` starts at 3 in `NewGameplayManager` |
| Card takes 10 seconds to charge before attacking | **Partial** | `TimeToAttack` field exists on `Card` struct but is set to 0 in `HandleCardPlaced` |
| Auto-attack: card attacks first card/leader in column after charging | **Missing** | No auto-attack system — attacks require manual `CARD_ATTACK` from client |
| Only the leader counterattacks | **Missing** | No counterattack logic, no leader entity |
| Attack queue: FIFO when multiple cards charged simultaneously | **Missing** | No attack queue system |
| Each card animation takes 3 seconds to resolve | **Missing** | No animation timing on server side |
| Board layout: 2 rows x 3 columns | **Correct** | `[2][3]handlers.Card` in `gameplay_state.go` |
| Leader HP: 250 | **Done** | `Player1Health/Player2Health = 250` in `NewGameplayManager` |
| Leader has attack power (10 per screenshot) | **Missing** | No leader attack stat |

**Files:** `services/gameplay/gameplay_state.go`, `services/gameplay/game_loop.go`, `services/gameplay/turn_timer.go`, `services/gameplay/handlers/card_placed.go`, `services/gameplay/handlers/card_attack.go`

## Pack System

| Spec Requirement | Status | Implementation Notes |
|---|---|---|
| Pack type probability: Common 79%, Rare 15%, Epic 5%, Legendary 1% | **Correct** | `rollPackType()` in `pack_handlers.go` matches exactly |
| Common pack: 10 cards (8-9 C, 1-2 R) | **Correct** | Matches |
| Rare pack: 20 cards (15-17 C, 2-4 R, 0-1 E) | **Correct** | Matches |
| Epic pack: 40 cards (20-25 C, 5-10 R, 1-2 E, 0-1 L) | **Correct** | Matches |
| Legendary pack: 50 cards (25-30 C, 14-20 R, 3-6 E, 1 L) | **Correct** | Matches |
| Win rewards: currency + pack | **Deferred** | `GivePackToPlayer` exists, packs are free for now — no currency system yet (intentional) |

**Files:** `services/deck/pack_handlers.go`

## Priority Breakdown

### Critical (gameplay won't work without these)

1. **Auto-attack system** — spec says cards attack automatically after 10s charge. Current impl requires manual `CARD_ATTACK` messages from client. This is the biggest architectural gap. The game loop should iterate over all placed cards each tick, decrement charge timers, and trigger attacks when ready.

2. ~~**Hand/draw system** — the entire pre-turn phase is missing.~~ **Done** — `OfferCards`, `SelectCards`, `DRAW_CARDS` handler, `PlayerHand` state, colour validation

3. ~~**Turn timeout: 30s not 90s**~~ **Done**

4. ~~**Starting energy: 3 not 0**~~ **Done**

5. ~~**Leader HP: 250 not 100**~~ **Done**

6. **Leader entity** — needs attack power (10), counterattack mechanic (leader deals damage back when a card attacks it directly).

### Important (needed for correct gameplay)

7. ~~**Deck validation** — enforce max 12 cards, max 2 copies per card, max 1 legendary in `handleCreateDeck`/`handleUpdateDeck`.~~ **Done**

8. **Card charge timer** — `TimeToAttack` should default to 10 seconds (40 ticks). Card placed on board starts at 0, increments each tick, attacks at 40.

9. **Attack queue** — when multiple cards finish charging on the same tick, queue them FIFO with 3s (12 tick) spacing for animation resolution.

10. ~~**Colour-based hand selection constraint** — pre-turn draw step should only allow 1 colour per selection.~~ **Done**

### Nice to have

11. ~~**Win reward currency** — no currency system yet, only packs.~~ **Deferred** — packs are free for now

12. **Animation timing** — 3 second resolve per card attack is primarily a client concern, but server needs to space out attack resolution accordingly.
