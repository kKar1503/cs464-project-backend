-- card stats table
CREATE TABLE IF NOT EXISTS card_stats (
    card_stats_id INT AUTO_INCREMENT PRIMARY KEY,
    card_id INT NOT NULL, 
    level INT NOT NULL, 
    power INT NOT NULL, 
    hp INT NOT NULL, 
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (card_id) REFERENCES cards(card_id) ON DELETE CASCADE,
    INDEX idx_card_id (card_id),
    UNIQUE KEY unique_card_level (card_id, level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- card abilites table
CREATE TABLE IF NOT EXISTS card_abilities (
    abilities_id INT AUTO_INCREMENT PRIMARY KEY,
    card_id INT NOT NULL, 
    trigger_type VARCHAR(255) NOT NULL, 
    effect VARCHAR(500) NOT NULL,
    abilties JSON NULL, 
    FOREIGN KEY (card_id) REFERENCES cards(card_id) ON DELETE CASCADE,
    INDEX idx_card_id (card_id), 
    INDEX idx_trigger_type (trigger_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;