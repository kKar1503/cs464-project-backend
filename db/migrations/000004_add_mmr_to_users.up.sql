-- Add MMR (Matchmaking Rating) to users table
ALTER TABLE users
ADD COLUMN mmr INT NOT NULL DEFAULT 1000,
ADD INDEX idx_mmr (mmr);
