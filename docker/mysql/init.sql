-- Initial database setup for game backend
-- This script runs automatically when MySQL container is first created

-- Create database if not exists (already created by env vars, but good practice)
CREATE DATABASE IF NOT EXISTS gamedb CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE gamedb;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    mmr INT NOT NULL DEFAULT 1000,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_username (username),
    INDEX idx_mmr (mmr)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Game sessions table
CREATE TABLE IF NOT EXISTS game_sessions (
    id VARCHAR(36) PRIMARY KEY,
    player1_id VARCHAR(36) NOT NULL,
    player2_id VARCHAR(36) NOT NULL,
    status ENUM('pending', 'in_progress', 'completed', 'cancelled') NOT NULL DEFAULT 'pending',
    winner_id VARCHAR(36),
    started_at TIMESTAMP NULL,
    ended_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (player1_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (player2_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (winner_id) REFERENCES users(id) ON DELETE SET NULL,
    INDEX idx_status (status),
    INDEX idx_player1 (player1_id),
    INDEX idx_player2 (player2_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Friendships table
CREATE TABLE IF NOT EXISTS friendships (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    friend_id VARCHAR(36) NOT NULL,
    status ENUM('pending', 'accepted', 'blocked') NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (friend_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE KEY unique_friendship (user_id, friend_id),
    INDEX idx_user_id (user_id),
    INDEX idx_friend_id (friend_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Game replays table
CREATE TABLE IF NOT EXISTS game_replays (
    id VARCHAR(36) PRIMARY KEY,
    game_session_id VARCHAR(36) NOT NULL,
    replay_data LONGTEXT NOT NULL,
    duration_seconds INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (game_session_id) REFERENCES game_sessions(id) ON DELETE CASCADE,
    INDEX idx_game_session (game_session_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Matchmaking queue table (for tracking players waiting for matches)
CREATE TABLE IF NOT EXISTS matchmaking_queue (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    mmr INT NOT NULL,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_mmr (mmr),
    INDEX idx_joined_at (joined_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Insert some test data (optional, can be removed for production)
INSERT INTO users (id, username, email, password_hash, mmr) VALUES
    ('test-user-1', 'player1', 'player1@example.com', '$2a$10$test', 1200),
    ('test-user-2', 'player2', 'player2@example.com', '$2a$10$test', 1150)
ON DUPLICATE KEY UPDATE id=id;
