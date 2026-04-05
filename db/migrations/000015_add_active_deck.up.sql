ALTER TABLE users ADD COLUMN active_deck_id INT NULL;
ALTER TABLE users ADD FOREIGN KEY fk_active_deck (active_deck_id) REFERENCES decks(deck_id) ON DELETE SET NULL;
