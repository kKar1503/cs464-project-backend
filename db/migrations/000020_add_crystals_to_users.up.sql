-- Add crystals currency to users for the card leveling system.
ALTER TABLE users
    ADD COLUMN crystals INT NOT NULL DEFAULT 0;

-- Re-add max_level to cards (dropped in 000019 when leveling was removed).
-- Default cap is 5 levels per card.
ALTER TABLE cards
    ADD COLUMN max_level INT NOT NULL DEFAULT 5;
