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
SELECT card_id, card_name, affiliation, rarity, mana_cost, max_level, description, icon_url
FROM cards ORDER BY card_id ASC;

-- name: GetAllCardsByRarity :many
SELECT card_id, card_name, affiliation, rarity, mana_cost, max_level, description, icon_url
FROM cards WHERE rarity = ? ORDER BY card_id ASC;

-- name: GetAllCardsByAffiliation :many
SELECT card_id, card_name, affiliation, rarity, mana_cost, max_level, description, icon_url
FROM cards WHERE affiliation = ? ORDER BY card_id ASC;

-- name: GetAllCardsByRarityAndAffiliation :many
SELECT card_id, card_name, affiliation, rarity, mana_cost, max_level, description, icon_url
FROM cards WHERE rarity = ? AND affiliation = ? ORDER BY card_id ASC;

-- name: GetPlayerCards :many
SELECT c.card_id, c.card_name, c.affiliation, c.rarity, c.mana_cost, c.max_level,
       c.description, c.icon_url, pc.level, pc.quantity
FROM player_cards pc
JOIN cards c ON pc.card_id = c.card_id
WHERE pc.player_id = ?
ORDER BY c.rarity DESC, c.card_id ASC;

-- name: GetPlayerDeckList :many
SELECT deck_id, name, created_at FROM decks WHERE player_id = ? ORDER BY deck_id ASC;

-- name: GetPlayerCardsNotInDeck :many
SELECT c.card_id, c.card_name, c.affiliation, c.rarity, c.mana_cost, c.max_level,
       c.description, c.icon_url, pc.level,
       pc.quantity - COALESCE(in_deck.cnt, 0) AS quantity
FROM player_cards pc
JOIN cards c ON pc.card_id = c.card_id
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
SELECT dc.position, c.card_id, c.card_name, c.affiliation, c.rarity, c.mana_cost,
       COALESCE(cs.power, 0) AS attack, COALESCE(cs.hp, 0) AS hp
FROM deck_cards dc
JOIN cards c ON dc.card_id = c.card_id
LEFT JOIN card_stats cs ON cs.card_id = c.card_id AND cs.level = 1
WHERE dc.deck_id = ?
ORDER BY dc.position;

-- name: SetActiveDeck :exec
UPDATE users SET active_deck_id = ? WHERE id = ?;

-- name: GetActiveDeck :one
SELECT active_deck_id FROM users WHERE id = ?;
