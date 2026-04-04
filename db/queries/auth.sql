-- name: CreateUserAuth :execresult
INSERT INTO users (username, password_hash) VALUES (?, ?);

-- name: GetUserByUsernameAuth :one
SELECT id, username, password_hash, is_banned, ban_reason FROM users WHERE username = ?;

-- name: CreateSession :exec
INSERT INTO user_sessions (user_id, token, ip_address, user_agent, expires_at) VALUES (?, ?, ?, ?, ?);

-- name: RevokeSessionByToken :exec
UPDATE user_sessions SET revoked_at = NOW() WHERE token = ? AND revoked_at IS NULL;

-- name: ValidateSession :one
SELECT s.user_id, u.username, s.expires_at, u.is_banned
FROM user_sessions s
JOIN users u ON s.user_id = u.id
WHERE s.token = ? AND s.revoked_at IS NULL AND s.expires_at > NOW();

-- name: GetActiveTokensByUser :many
SELECT token FROM user_sessions WHERE user_id = ? AND revoked_at IS NULL AND expires_at > NOW();

-- name: RevokeAllUserSessions :exec
UPDATE user_sessions SET revoked_at = NOW() WHERE user_id = ? AND revoked_at IS NULL;

-- name: BanUser :exec
UPDATE users SET is_banned = TRUE, banned_at = NOW(), ban_reason = ? WHERE id = ?;

-- name: UnbanUser :exec
UPDATE users SET is_banned = FALSE, banned_at = NULL, ban_reason = NULL WHERE id = ?;
