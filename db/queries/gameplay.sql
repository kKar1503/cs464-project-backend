-- name: GetGameSessionPlayers :one
SELECT gs.player1_id, gs.player2_id, u1.username, u2.username
FROM game_sessions gs
JOIN users u1 ON gs.player1_id = u1.id
JOIN users u2 ON gs.player2_id = u2.id
WHERE gs.session_id = ? AND gs.status = 'in_progress';
