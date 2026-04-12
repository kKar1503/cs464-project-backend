-- name: CreateDeck :execresult
INSERT INTO decks (player_id, name) VALUES (?, ?);

-- name: InsertDeckCard :exec
INSERT INTO deck_cards (deck_id, card_id, position) VALUES (?, ?, ?);

-- name: GetDeckByIDAndPlayer :one
SELECT deck_id, name, created_at FROM decks WHERE deck_id = ? AND player_id = ?;

-- name: GetDeckCards :many
SELECT card_id, position FROM deck_cards WHERE deck_id = ? ORDER BY position;

-- name: UpdateDeck :execresult
UPDATE decks SET name = ? WHERE deck_id = ? AND player_id = ?;

-- name: DeleteDeckCards :exec
DELETE FROM deck_cards WHERE deck_id = ?;

-- name: DeleteDeck :execresult
DELETE FROM decks WHERE deck_id = ? AND player_id = ?;

-- name: GetAllCards :many
SELECT card_id, card_name, affiliation, rarity, mana_cost, description, icon_url
FROM cards ORDER BY card_id ASC;

-- name: GetAllCardsByRarity :many
SELECT card_id, card_name, affiliation, rarity, mana_cost, description, icon_url
FROM cards WHERE rarity = ? ORDER BY card_id ASC;

-- name: GetAllCardsByAffiliation :many
SELECT card_id, card_name, affiliation, rarity, mana_cost, description, icon_url
FROM cards WHERE affiliation = ? ORDER BY card_id ASC;

-- name: GetAllCardsByRarityAndAffiliation :many
SELECT card_id, card_name, affiliation, rarity, mana_cost, description, icon_url
FROM cards WHERE rarity = ? AND affiliation = ? ORDER BY card_id ASC;

-- name: GetPlayerCards :many
-- Returns the player's collection with per-card level and max_level. Base
-- stats come from card_stats at level=1; the gameplay service re-scales by
-- level when cards are actually placed, so exposing level here is enough.
SELECT c.card_id, c.card_name, c.affiliation, c.rarity, c.mana_cost,
       c.description, c.icon_url, c.max_level, pc.level, pc.quantity,
       COALESCE(cs.power, 0) AS base_attack,
       COALESCE(cs.hp, 0) AS base_hp
FROM player_cards pc
JOIN cards c ON pc.card_id = c.card_id
LEFT JOIN card_stats cs ON cs.card_id = c.card_id AND cs.level = 1
WHERE pc.player_id = ?
ORDER BY c.rarity DESC, c.card_id ASC;

-- name: GetPlayerDeckList :many
SELECT deck_id, name, created_at FROM decks WHERE player_id = ? ORDER BY deck_id ASC;

-- name: GetPlayerCardsNotInDeck :many
SELECT c.card_id, c.card_name, c.affiliation, c.rarity, c.mana_cost,
       c.description, c.icon_url, c.max_level, pc.level,
       pc.quantity - COALESCE(in_deck.cnt, 0) AS quantity,
       COALESCE(cs.power, 0) AS base_attack,
       COALESCE(cs.hp, 0) AS base_hp
FROM player_cards pc
JOIN cards c ON pc.card_id = c.card_id
LEFT JOIN card_stats cs ON cs.card_id = c.card_id AND cs.level = 1
LEFT JOIN (
    SELECT card_id, COUNT(*) AS cnt
    FROM deck_cards
    WHERE deck_id = ?
    GROUP BY card_id
) in_deck ON in_deck.card_id = pc.card_id
WHERE pc.player_id = ?
  AND pc.quantity > COALESCE(in_deck.cnt, 0)
ORDER BY c.rarity DESC, c.card_id ASC;

-- name: GetPlayerPacks :many
SELECT pack_id, pack_type, is_opened, created_at FROM card_packs
WHERE player_id = ? ORDER BY created_at DESC;

-- name: GetPackByIDAndPlayer :one
SELECT pack_type, is_opened FROM card_packs WHERE pack_id = ? AND player_id = ?;

-- name: OpenPack :exec
UPDATE card_packs SET is_opened = TRUE, opened_at = NOW() WHERE pack_id = ?;

-- name: UpsertPlayerCard :exec
INSERT INTO player_cards (player_id, card_id, level, quantity) VALUES (?, ?, 1, 1)
ON DUPLICATE KEY UPDATE quantity = quantity + 1;

-- name: SetPlayerCardQuantity :exec
INSERT INTO player_cards (player_id, card_id, level, quantity) VALUES (?, ?, 1, ?)
ON DUPLICATE KEY UPDATE quantity = VALUES(quantity);

-- name: GetRandomCardsByRarity :many
SELECT card_id, card_name, rarity FROM cards WHERE rarity = ? ORDER BY RAND() LIMIT ?;

-- name: CreatePack :execresult
INSERT INTO card_packs (player_id, pack_type) VALUES (?, ?);

-- name: GetPlayerCardOwnership :many
SELECT card_id, quantity FROM player_cards WHERE player_id = ?;

-- name: GetDeckCardsWithDetails :many
-- Returns deck cards with stats scaled by the owning player's card level.
-- Formula: ceil((1 + 0.2 * (level - 1)) * base_stat). base_stat comes from
-- card_stats at level=1 (seeded row). Player-card level lives in player_cards.
SELECT dc.position, c.card_id, c.card_name, c.affiliation, c.rarity, c.mana_cost,
       COALESCE(pc.level, 1) AS card_level,
       CAST(CEIL((1 + 0.2 * (COALESCE(pc.level, 1) - 1)) * COALESCE(cs.power, 0)) AS SIGNED) AS attack,
       CAST(CEIL((1 + 0.2 * (COALESCE(pc.level, 1) - 1)) * COALESCE(cs.hp, 0)) AS SIGNED) AS hp
FROM deck_cards dc
JOIN decks d ON d.deck_id = dc.deck_id
JOIN cards c ON dc.card_id = c.card_id
LEFT JOIN card_stats cs ON cs.card_id = c.card_id AND cs.level = 1
LEFT JOIN player_cards pc ON pc.player_id = d.player_id AND pc.card_id = c.card_id
WHERE dc.deck_id = ?
ORDER BY dc.position;

-- name: SetActiveDeck :exec
UPDATE users SET active_deck_id = ? WHERE id = ?;

-- name: GetActiveDeck :one
SELECT active_deck_id FROM users WHERE id = ?;

-- name: GetAbilitiesForDeck :many
SELECT ca.card_id, ca.trigger_type, ca.effect_type, ca.params
FROM card_abilities ca
WHERE ca.card_id IN (
    SELECT dc.card_id FROM deck_cards dc WHERE dc.deck_id = ?
)
ORDER BY ca.card_id, ca.ability_id;

-- name: GetAllCardDefinitions :many
SELECT c.card_id, c.card_name, c.affiliation, c.rarity, c.mana_cost,
       COALESCE(cs.power, 0) AS attack, COALESCE(cs.hp, 0) AS hp
FROM cards c
LEFT JOIN card_stats cs ON cs.card_id = c.card_id AND cs.level = 1
ORDER BY c.card_id;

-- name: GetAllCardAbilities :many
SELECT card_id, trigger_type, effect_type, params
FROM card_abilities ORDER BY card_id, ability_id;

-- name: GetPlayerCrystals :one
SELECT crystals FROM users WHERE id = ?;

-- name: AddPlayerCrystals :exec
UPDATE users SET crystals = crystals + ? WHERE id = ?;

-- name: DeductPlayerCrystals :execresult
UPDATE users SET crystals = crystals - ? WHERE id = ? AND crystals >= ?;

-- name: GetPlayerCard :one
SELECT pc.player_id, pc.card_id, pc.level, pc.quantity, c.max_level, c.rarity
FROM player_cards pc
JOIN cards c ON c.card_id = pc.card_id
WHERE pc.player_id = ? AND pc.card_id = ?;

-- name: LevelUpPlayerCard :execresult
-- Increments player_card level and consumes `cards_cost` quantity.
-- Uses LEAST to avoid going below zero — caller must pre-check ownership.
UPDATE player_cards
SET level = level + 1,
    quantity = quantity - ?
WHERE player_id = ? AND card_id = ? AND level = ? AND quantity >= ?;

-- name: GetDeckUsageForPlayerCard :many
-- For each deck the player owns, how many copies of `card_id` it holds.
-- Used after level-up / disenchant to prune decks whose usage now exceeds
-- the player's remaining quantity.
SELECT d.deck_id, d.name, COUNT(dc.deck_card_id) AS copies_in_deck
FROM decks d
LEFT JOIN deck_cards dc ON dc.deck_id = d.deck_id AND dc.card_id = ?
WHERE d.player_id = ?
GROUP BY d.deck_id, d.name
HAVING copies_in_deck > 0;

-- name: PruneDeckCardsForCard :execresult
-- Removes the `excess` highest-positioned copies of `card_id` from a single
-- deck. Called when the player's remaining quantity can no longer support
-- the deck's current usage. Highest positions go first so the deck's lead
-- slot (position 0) is preserved where possible.
DELETE FROM deck_cards
WHERE deck_id = ? AND card_id = ?
ORDER BY position DESC
LIMIT ?;

-- name: DisenchantPlayerCard :execresult
-- Removes `quantity_to_remove` copies from a maxed-out player card.
-- Caller must pass `min_quantity = quantity_to_remove + 1` so the player
-- retains at least one copy. Gated on level = max_level (atomic).
UPDATE player_cards pc
JOIN cards c ON c.card_id = pc.card_id
SET pc.quantity = pc.quantity - ?
WHERE pc.player_id = ? AND pc.card_id = ?
  AND pc.level = c.max_level
  AND pc.quantity >= ?;
