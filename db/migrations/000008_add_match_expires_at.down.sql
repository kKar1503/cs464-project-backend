-- Remove match expiry timestamp from game_sessions
ALTER TABLE game_sessions
DROP COLUMN match_expires_at;
