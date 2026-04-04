-- name: GetUserForMatchmaking :one
SELECT username, mmr, is_banned FROM users WHERE id = ?;

-- name: GetOngoingGameSession :one
SELECT session_id FROM game_sessions
WHERE (player1_id = ? OR player2_id = ?) AND status IN ('waiting', 'in_progress');

-- name: GetQueueEntryByUserID :one
SELECT id FROM matchmaking_queue WHERE user_id = ?;

-- name: InsertIntoQueue :exec
INSERT INTO matchmaking_queue (user_id, mmr) VALUES (?, ?);

-- name: DeleteFromQueue :execresult
DELETE FROM matchmaking_queue WHERE user_id = ?;

-- name: GetQueueStatus :one
SELECT mmr, joined_at FROM matchmaking_queue WHERE user_id = ?;

-- name: GetActiveQueue :many
SELECT q.user_id, u.username, q.mmr, q.joined_at
FROM matchmaking_queue q
JOIN users u ON q.user_id = u.id
WHERE u.is_banned = FALSE
ORDER BY q.joined_at ASC;

-- name: DeleteFromQueueByUsers :exec
DELETE FROM matchmaking_queue WHERE user_id IN (?, ?);

-- name: RequeuePlayer :exec
INSERT INTO matchmaking_queue (user_id, mmr, joined_at) VALUES (?, ?, NOW())
ON DUPLICATE KEY UPDATE joined_at = NOW();
