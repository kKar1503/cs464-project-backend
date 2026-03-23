-- Remove player readiness columns from game_sessions
ALTER TABLE game_sessions
DROP COLUMN player1_ready,
DROP COLUMN player2_ready;
