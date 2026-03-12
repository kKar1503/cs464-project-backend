CREATE TABLE IF NOT EXISTS deck_cards (
    deck_card_id INT AUTO_INCREMENT PRIMARY KEY,
    deck_id INT NOT NULL,
    card_id INT NOT NULL,
    position INT NOT NULL,
    FOREIGN KEY (deck_id) REFERENCES decks(deck_id) ON DELETE CASCADE,
    FOREIGN KEY (card_id) REFERENCES cards(card_id) ON DELETE CASCADE,
    INDEX idx_deck_id (deck_id),
    INDEX idx_card_id (card_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;