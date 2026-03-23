-- Add ban status columns to users table
ALTER TABLE users
ADD COLUMN is_banned BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN banned_at TIMESTAMP NULL,
ADD COLUMN ban_reason TEXT NULL,
ADD INDEX idx_is_banned (is_banned);
