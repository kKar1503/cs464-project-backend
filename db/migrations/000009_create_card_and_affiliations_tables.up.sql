-- card table 
CREATE TABLE IF NOT EXISTS cards (
    card_id INT AUTO_INCREMENT PRIMARY KEY,
    card_name VARCHAR(255) NOT NULL, 
    affiliation INT NOT NULL, 
    rarity VARCHAR(255) NOT NULL, 
    mana_cost INT NOT NULL, 
    max_level INT NOT NULL, 
    description VARCHAR(500) NOT NULL, 
    icon_url VARCHAR(255) NOT NULL,
    INDEX idx_affiliation (affiliation),
    INDEX idx_rarity (rarity)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- affiliations table
CREATE TABLE IF NOT EXISTS affiliations (
    affiliation_id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(500) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;