CREATE TABLE IF NOT EXISTS card_packs (
    pack_id INT AUTO_INCREMENT PRIMARY KEY,
    player_id BIGINT NOT NULL,
    pack_type VARCHAR(20) NOT NULL,
    is_opened BOOLEAN NOT NULL DEFAULT FALSE,
    opened_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (player_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_player_id (player_id),
    INDEX idx_is_opened (is_opened)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;