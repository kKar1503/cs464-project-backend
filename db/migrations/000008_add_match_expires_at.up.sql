-- Add match expiry timestamp to game_sessions
ALTER TABLE game_sessions
ADD COLUMN match_expires_at TIMESTAMP NULL AFTER created_at;
