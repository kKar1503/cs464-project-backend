-- Remove ban status columns from users table
ALTER TABLE users 
DROP COLUMN ban_reason,
DROP COLUMN banned_at,
DROP COLUMN is_banned;
