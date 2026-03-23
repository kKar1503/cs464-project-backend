-- Add player readiness columns to game_sessions
ALTER TABLE game_sessions
ADD COLUMN player1_ready BOOLEAN DEFAULT FALSE AFTER status,
ADD COLUMN player2_ready BOOLEAN DEFAULT FALSE AFTER player1_ready;
