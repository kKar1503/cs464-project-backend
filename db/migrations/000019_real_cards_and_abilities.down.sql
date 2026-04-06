-- =============================================================================
-- Migration 000019 DOWN: Revert to placeholder cards
-- =============================================================================

-- Step 1: Clean new data
DELETE FROM deck_cards;
DELETE FROM player_cards;
DELETE FROM decks;

-- Step 2: Drop new card_abilities and recreate old schema
DROP TABLE IF EXISTS card_abilities;
CREATE TABLE card_abilities (
    abilities_id INT AUTO_INCREMENT PRIMARY KEY,
    card_id INT NOT NULL,
    trigger_type VARCHAR(255) NOT NULL,
    effect VARCHAR(500) NOT NULL,
    abilties JSON NULL,
    FOREIGN KEY (card_id) REFERENCES cards(card_id) ON DELETE CASCADE,
    INDEX idx_card_id (card_id),
    INDEX idx_trigger_type (trigger_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Step 3: Clean card data
DELETE FROM card_stats;
DELETE FROM cards;

-- Step 4: Re-add max_level column
ALTER TABLE cards ADD COLUMN max_level INT NOT NULL DEFAULT 5 AFTER mana_cost;

-- Step 5: Reset auto-increment
ALTER TABLE cards AUTO_INCREMENT = 1;

-- Step 6: Restore old affiliations
DELETE FROM affiliations;

-- Step 7: Re-seed old 22 placeholder cards
INSERT INTO cards (card_name, affiliation, rarity, mana_cost, max_level, description, icon_url) VALUES
('Fire Imp', 1, 'common', 2, 5, 'A small fire creature', 'https://example.com/fire_imp.png'),
('Water Sprite', 1, 'common', 2, 5, 'A playful water elemental', 'https://example.com/water_sprite.png'),
('Earth Golem', 1, 'common', 3, 5, 'A sturdy earth golem', 'https://example.com/earth_golem.png'),
('Wind Walker', 1, 'common', 2, 5, 'A swift wind unit', 'https://example.com/wind_walker.png'),
('Shadow Cat', 1, 'common', 1, 5, 'A sneaky shadow cat', 'https://example.com/shadow_cat.png'),
('Iron Shield', 1, 'common', 3, 5, 'A defensive shield bearer', 'https://example.com/iron_shield.png'),
('Frost Archer', 1, 'common', 2, 5, 'A cold precision shooter', 'https://example.com/frost_archer.png'),
('Stone Sentry', 1, 'common', 3, 5, 'A vigilant stone guardian', 'https://example.com/stone_sentry.png'),
('Vine Creeper', 1, 'common', 1, 5, 'A tangling vine creature', 'https://example.com/vine_creeper.png'),
('Spark Wisp', 1, 'common', 1, 5, 'A tiny spark of lightning', 'https://example.com/spark_wisp.png'),
('Sand Beetle', 1, 'common', 2, 5, 'A resilient desert insect', 'https://example.com/sand_beetle.png'),
('Cloud Puff', 1, 'common', 1, 5, 'A fluffy cloud creature', 'https://example.com/cloud_puff.png'),
('Flame Knight', 1, 'rare', 4, 8, 'A powerful fire knight', 'https://example.com/flame_knight.png'),
('Ice Mage', 1, 'rare', 4, 8, 'A frost-wielding mage', 'https://example.com/ice_mage.png'),
('Storm Rider', 1, 'rare', 5, 8, 'A lightning cavalry unit', 'https://example.com/storm_rider.png'),
('Dark Rogue', 1, 'rare', 3, 8, 'A shadow assassin', 'https://example.com/dark_rogue.png'),
('Holy Cleric', 1, 'rare', 4, 8, 'A healer of the light', 'https://example.com/holy_cleric.png'),
('Thunder Drake', 1, 'epic', 6, 10, 'A lightning-breathing dragon', 'https://example.com/thunder_drake.png'),
('Crystal Golem', 1, 'epic', 7, 10, 'A massive crystal construct', 'https://example.com/crystal_golem.png'),
('Void Walker', 1, 'epic', 6, 10, 'A being from the void', 'https://example.com/void_walker.png'),
('Phoenix Lord', 1, 'legendary', 8, 10, 'A legendary phoenix of rebirth', 'https://example.com/phoenix_lord.png'),
('Dragon King', 1, 'legendary', 9, 10, 'The king of all dragons', 'https://example.com/dragon_king.png');

-- Step 8: Re-seed old card stats
INSERT INTO card_stats (card_id, level, power, hp) VALUES
(1, 1, 5, 5),       -- Fire Imp
(2, 1, 5, 5),       -- Water Sprite
(3, 1, 10, 10),     -- Earth Golem
(4, 1, 5, 5),       -- Wind Walker
(5, 1, 3, 3),       -- Shadow Cat
(6, 1, 8, 15),      -- Iron Shield
(7, 1, 7, 5),       -- Frost Archer
(8, 1, 8, 12),      -- Stone Sentry
(9, 1, 3, 3),       -- Vine Creeper
(10, 1, 3, 3),      -- Spark Wisp
(11, 1, 5, 5),      -- Sand Beetle
(12, 1, 3, 3),      -- Cloud Puff
(13, 1, 12, 10),    -- Flame Knight
(14, 1, 10, 10),    -- Ice Mage
(15, 1, 15, 12),    -- Storm Rider
(16, 1, 8, 8),      -- Dark Rogue
(17, 1, 10, 10),    -- Holy Cleric
(18, 1, 20, 15),    -- Thunder Drake
(19, 1, 18, 20),    -- Crystal Golem
(20, 1, 15, 12),    -- Void Walker
(21, 1, 25, 20),    -- Phoenix Lord
(22, 1, 30, 25);    -- Dragon King
