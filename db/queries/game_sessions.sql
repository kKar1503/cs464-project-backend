-- name: CreateGameSession :exec
INSERT INTO game_sessions (session_id, player1_id, player2_id, status, created_at, match_expires_at)
VALUES (?, ?, ?, 'waiting', NOW(), ?);

-- name: GetSessionForAccept :one
SELECT player1_id, player2_id, player1_ready, player2_ready, status
FROM game_sessions WHERE session_id = ?;

-- name: StartGameSession :execresult
UPDATE game_sessions
SET player1_ready = ?, player2_ready = ?, status = 'in_progress', started_at = NOW(), match_expires_at = NULL
WHERE session_id = ? AND status = 'waiting';

-- name: SetPlayer1Ready :execresult
UPDATE game_sessions SET player1_ready = TRUE
WHERE session_id = ? AND status = 'waiting';

-- name: SetPlayer2Ready :execresult
UPDATE game_sessions SET player2_ready = TRUE
WHERE session_id = ? AND status = 'waiting';

-- name: GetSessionWithPlayers :one
SELECT gs.player1_id, gs.player2_id, u1.username, u2.username, u1.mmr, u2.mmr, gs.status
FROM game_sessions gs
JOIN users u1 ON gs.player1_id = u1.id
JOIN users u2 ON gs.player2_id = u2.id
WHERE gs.session_id = ?;

-- name: CancelGameSession :exec
UPDATE game_sessions SET status = 'cancelled' WHERE session_id = ?;

-- name: GetSessionForEndGame :one
SELECT gs.player1_id, gs.player2_id, u1.mmr, u2.mmr
FROM game_sessions gs
JOIN users u1 ON gs.player1_id = u1.id
JOIN users u2 ON gs.player2_id = u2.id
WHERE gs.session_id = ? AND gs.status = 'in_progress';

-- name: UpdateUserMMR :exec
UPDATE users SET mmr = mmr + ? WHERE id = ?;

-- name: CompleteGameSession :exec
UPDATE game_sessions
SET status = 'completed', ended_at = NOW(), winner_id = ?, player1_mmr_change = ?, player2_mmr_change = ?
WHERE session_id = ?;

-- name: GetSessionStatus :one
SELECT player1_id, player2_id, status, winner_id, started_at, ended_at, player1_mmr_change, player2_mmr_change
FROM game_sessions WHERE session_id = ?;

-- name: GetExpiredWaitingSessions :many
SELECT gs.session_id, gs.player1_id, gs.player2_id,
       u1.username, u2.username, u1.mmr, u2.mmr
FROM game_sessions gs
JOIN users u1 ON gs.player1_id = u1.id
JOIN users u2 ON gs.player2_id = u2.id
WHERE gs.status = 'waiting'
  AND gs.match_expires_at IS NOT NULL
  AND gs.match_expires_at < NOW();
