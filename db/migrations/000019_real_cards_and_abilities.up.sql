-- =============================================================================
-- Migration 000019: Replace placeholder cards with real card data + abilities
-- =============================================================================

-- Step 1: Clean existing data (order matters due to FK constraints)
DELETE FROM deck_cards;
DELETE FROM card_abilities;
DELETE FROM card_stats;
DELETE FROM player_cards;
DELETE FROM decks;
DELETE FROM cards;
DELETE FROM affiliations;

-- Step 2: Drop max_level column from cards table (leveling removed)
ALTER TABLE cards DROP COLUMN max_level;

-- Step 3: Reset auto-increment
ALTER TABLE cards AUTO_INCREMENT = 1;

-- Step 4: Seed affiliations as colours
INSERT INTO affiliations (affiliation_id, name, description) VALUES
(1, 'Grey', 'Neutral cards'),
(2, 'Red', 'Aggressive cards'),
(3, 'Blue', 'Tricky cards'),
(4, 'Green', 'Growth cards'),
(5, 'Purple', 'Risk/reward cards'),
(6, 'Yellow', 'Defensive cards'),
(7, 'Colourless', 'Token cards');

-- Step 5: Insert all 38 real cards
-- Grey cards (affiliation=1)
INSERT INTO cards (card_id, card_name, affiliation, rarity, mana_cost, description, icon_url) VALUES
(1,  'Pig',                  1, 'common',    1, 'summon: 1/2048 chance to transform into 999/5 technoblade', ''),
(2,  'Farmer',               1, 'common',    2, 'No effect', ''),
(3,  'Mercenary',            1, 'rare',      3, 'No effect', ''),
(4,  'Town Guard',           1, 'rare',      4, 'No effect', ''),
(5,  'Town Hero',            1, 'epic',      5, 'summon: destroy all with same colour and get +5/+5 for each card destroyed', ''),
(6,  'Travelling Merchant',  1, 'legendary', 6, 'summon: +5/+5 for each colour in your deck', '');

-- Red cards (affiliation=2)
INSERT INTO cards (card_id, card_name, affiliation, rarity, mana_cost, description, icon_url) VALUES
(7,  'Barbarian',            2, 'common',    3, 'summon: 1/2048 chance to double attack speed', ''),
(8,  'Dwarf',                2, 'common',    3, 'summon: give vertically adjacent unit +10/0', ''),
(9,  'Bombadier',            2, 'rare',      4, 'summon: deal 15 to enemy leader', ''),
(10, 'Ninja',                2, 'rare',      3, 'summon: doubles its own attack speed', ''),
(11, 'Apache',               2, 'epic',      5, 'on damaged: gain +10/0', ''),
(12, 'Dinosaur',             2, 'legendary', 8, 'summon: deal 15 to all units on battlefield', '');

-- Blue cards (affiliation=3)
INSERT INTO cards (card_id, card_name, affiliation, rarity, mana_cost, description, icon_url) VALUES
(13, 'Penguin',              3, 'common',    3, 'summon: give enemy units in the same column -10/0', ''),
(14, 'Apprentice Magician',  3, 'common',    3, 'Attacks a random enemy unit or leader', ''),
(15, 'Krazy Kraken',         3, 'rare',      4, 'Attacks a random unit or leader', ''),
(16, 'Pufferfish',           3, 'rare',      4, 'summon: return an enemy unit in front to opponent''s hand', ''),
(17, 'Magic Swordman',       3, 'epic',      5, 'summon: Transform an enemy unit in front into a pig', ''),
(18, 'Big Whale',            3, 'legendary', 7, 'Whenever a unit attacks this unit, deal damage equal to its attack', '');

-- Green cards (affiliation=4)
INSERT INTO cards (card_id, card_name, affiliation, rarity, mana_cost, description, icon_url) VALUES
(19, 'Swamp Ogre',           4, 'common',    4, 'summon: 1/2048 chance to bounce all enemy units in the front row', ''),
(20, 'Nymph',                4, 'common',    3, 'summon: give adjacent units +10/+10', ''),
(21, 'Living Tree',          4, 'rare',      3, 'summon: increase your maximum elixir by 1', ''),
(22, 'Dryad',                4, 'rare',      4, 'summon: if there is an enemy unit in front, gain +10/+10', ''),
(23, 'Alpha Wolf',           4, 'epic',      5, 'summon: summon 2 15/15 wolves on your board', ''),
(24, 'Quetzalcoatl',         4, 'legendary', 7, 'summon: give all friendly units +10/+10', '');

-- Purple cards (affiliation=5)
INSERT INTO cards (card_id, card_name, affiliation, rarity, mana_cost, description, icon_url) VALUES
(25, 'Glass Bones',          5, 'common',    2, 'on attack: deal 10 damage to itself', ''),
(26, 'Traitor',              5, 'common',    2, 'on death: deal 20 damage to your leader', ''),
(27, 'Plague Doctor',        5, 'rare',      5, 'summon: give enemy units in the back row -15/-10', ''),
(28, 'Witch',                5, 'rare',      3, 'on death: destroy a random enemy unit', ''),
(29, 'Dullahan',             5, 'epic',      6, 'summon: destroy an enemy in front of it', ''),
(30, 'Shikigami',            5, 'legendary', 3, 'summon: set all units hp to 1', '');

-- Yellow cards (affiliation=6)
INSERT INTO cards (card_id, card_name, affiliation, rarity, mana_cost, description, icon_url) VALUES
(31, 'Lazy Chick',           6, 'common',    2, 'summon: 1/2048 chance to set its attack speed to 0', ''),
(32, 'Angel',                6, 'common',    3, 'summon: give vertically adjacent unit 0/+10', ''),
(33, 'Holy Spear Knight',    6, 'rare',      4, 'Ignore the front row of enemy units when attacking', ''),
(34, 'Cat Sith',             6, 'rare',      3, 'summon & on attack: reset the attack gauge for the enemy in front', ''),
(35, 'Paladin',              6, 'epic',      5, 'whenever this unit takes damage, reduce it by 10', ''),
(36, 'Archangel',            6, 'legendary', 6, 'on attack: give adjacent units +0/+10', '');

-- Token cards (affiliation=7, rarity='token')
INSERT INTO cards (card_id, card_name, affiliation, rarity, mana_cost, description, icon_url) VALUES
(37, 'Technoblade',          7, 'token',     1, 'No effect', ''),
(38, 'Wolf',                 4, 'token',     3, 'No effect', '');

-- Step 6: Seed card stats (level 1 only)
INSERT INTO card_stats (card_id, level, power, hp) VALUES
(1,  1, 5,   5),     -- Pig
(2,  1, 10,  10),    -- Farmer
(3,  1, 15,  15),    -- Mercenary
(4,  1, 20,  20),    -- Town Guard
(5,  1, 25,  25),    -- Town Hero
(6,  1, 10,  10),    -- Travelling Merchant
(7,  1, 20,  10),    -- Barbarian
(8,  1, 5,   5),     -- Dwarf
(9,  1, 25,  15),    -- Bombadier
(10, 1, 20,  10),    -- Ninja
(11, 1, 25,  25),    -- Apache
(12, 1, 50,  30),    -- Dinosaur
(13, 1, 5,   5),     -- Penguin
(14, 1, 20,  15),    -- Apprentice Magician
(15, 1, 30,  20),    -- Krazy Kraken
(16, 1, 15,  15),    -- Pufferfish
(17, 1, 15,  15),    -- Magic Swordman
(18, 1, 30,  40),    -- Big Whale
(19, 1, 20,  20),    -- Swamp Ogre
(20, 1, 5,   5),     -- Nymph
(21, 1, 10,  10),    -- Living Tree
(22, 1, 15,  15),    -- Dryad
(23, 1, 15,  15),    -- Alpha Wolf
(24, 1, 15,  15),    -- Quetzalcoatl
(25, 1, 10,  20),    -- Glass Bones
(26, 1, 20,  20),    -- Traitor
(27, 1, 5,   10),    -- Plague Doctor
(28, 1, 15,  15),    -- Witch
(29, 1, 20,  20),    -- Dullahan
(30, 1, 15,  100),   -- Shikigami
(31, 1, 0,   30),    -- Lazy Chick
(32, 1, 5,   5),     -- Angel
(33, 1, 15,  15),    -- Holy Spear Knight
(34, 1, 5,   15),    -- Cat Sith
(35, 1, 15,  25),    -- Paladin
(36, 1, 15,  25),    -- Archangel
(37, 1, 999, 5),     -- Technoblade
(38, 1, 15,  15);    -- Wolf

-- Step 7: Drop and recreate card_abilities with cleaner schema
DROP TABLE IF EXISTS card_abilities;

CREATE TABLE card_abilities (
    ability_id    INT AUTO_INCREMENT PRIMARY KEY,
    card_id       INT NOT NULL,
    trigger_type  VARCHAR(20) NOT NULL,
    effect_type   VARCHAR(50) NOT NULL,
    params        JSON NOT NULL,
    FOREIGN KEY (card_id) REFERENCES cards(card_id) ON DELETE CASCADE,
    INDEX idx_card_id (card_id),
    INDEX idx_trigger_type (trigger_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Step 8: Seed all card abilities

-- Grey cards
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(1,  'summon',     'transform',           '{"chance":2048,"into_card_id":37}'),
(5,  'summon',     'destroy',             '{"target":"same_colour","self_buff_per_kill":{"attack":5,"hp":5}}'),
(6,  'summon',     'stats_change',        '{"target":"self","per_colour_in_deck":{"attack":5,"hp":5}}');

-- Red cards
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(7,  'summon',     'double_attack_speed', '{"chance":2048}'),
(8,  'summon',     'stats_change',        '{"target":"vertical","attack":10,"hp":0}'),
(9,  'summon',     'deal_damage',         '{"target":"enemy_leader","damage":15}'),
(10, 'summon',     'double_attack_speed', '{}'),
(11, 'on_damaged', 'stats_change',        '{"target":"self","attack":10,"hp":0}'),
(12, 'summon',     'deal_damage',         '{"target":"all_units","damage":15}');

-- Blue cards
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(13, 'summon',     'stats_change',        '{"target":"opponent_column","attack":-10,"hp":0}'),
(14, 'on_attack',  'random_target',       '{"pool":"enemy"}'),
(15, 'on_attack',  'random_target',       '{"pool":"all"}'),
(16, 'summon',     'bounce',              '{"target":"enemy_in_front"}'),
(17, 'summon',     'transform',           '{"target":"enemy_in_front","into_card_id":1}'),
(18, 'on_damaged', 'reflect',             '{}');

-- Green cards
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(19, 'summon',     'bounce',              '{"target":"enemy_front_row","chance":2048}'),
(20, 'summon',     'stats_change',        '{"target":"adjacent","attack":10,"hp":10}'),
(21, 'summon',     'elixir_overflow',     '{"amount":1}'),
(22, 'summon',     'stats_change',        '{"target":"self","attack":10,"hp":10,"condition":"enemy_in_front"}'),
(23, 'summon',     'summon_units',        '{"card_id":38,"count":2}'),
(24, 'summon',     'stats_change',        '{"target":"all_allies","attack":10,"hp":10}');

-- Purple cards
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(25, 'on_attack',  'self_damage',         '{"damage":10}'),
(26, 'on_death',   'damage_leader',       '{"target":"own_leader","damage":20}'),
(27, 'summon',     'stats_change',        '{"target":"opponent_back_row","attack":-15,"hp":-10}'),
(28, 'on_death',   'destroy_random',      '{"target":"random_enemy"}'),
(29, 'summon',     'destroy',             '{"target":"enemy_in_front"}'),
(30, 'summon',     'set_hp',              '{"target":"all_units","hp":1}');

-- Yellow cards
INSERT INTO card_abilities (card_id, trigger_type, effect_type, params) VALUES
(31, 'summon',     'double_attack_speed', '{"chance":2048,"invert":true}'),
(32, 'summon',     'stats_change',        '{"target":"vertical","attack":0,"hp":10}'),
(33, 'on_attack',  'skip_front_row',      '{}'),
(34, 'summon',     'reset_attack',        '{"target":"enemy_in_front"}'),
(34, 'on_attack',  'reset_attack',        '{"target":"enemy_in_front"}'),
(35, 'on_damaged', 'shield',              '{"reduction":10}'),
(36, 'on_attack',  'stats_change',        '{"target":"adjacent","attack":0,"hp":10}');
