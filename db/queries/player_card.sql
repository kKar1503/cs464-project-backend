-- name: ListPlayerCards :many
SELECT
    pc.card_id,
    c.card_name,
    c.rarity,
    c.mana_cost,
    c.icon_url,
    pc.level,
    pc.quantity,
    pc.is_in_deck
FROM player_cards pc
JOIN cards c ON c.card_id = pc.card_id
WHERE pc.player_id = ?
ORDER BY pc.card_id;
