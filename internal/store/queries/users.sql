-- name: GetUser :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ?;

-- name: ListUsers :many
SELECT * FROM users ORDER BY email;

-- name: UpsertUser :exec
INSERT INTO users (id, email, name, password_hash, role, disabled) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    email = excluded.email,
    name = excluded.name,
    password_hash = excluded.password_hash,
    role = excluded.role,
    disabled = excluded.disabled;

-- name: ListUserCompetitions :many
SELECT competition_id FROM user_competitions WHERE user_id = ? ORDER BY competition_id;

-- name: AddUserCompetition :exec
INSERT OR IGNORE INTO user_competitions (user_id, competition_id) VALUES (?, ?);

-- name: DeleteUserCompetitions :exec
DELETE FROM user_competitions WHERE user_id = ?;

-- name: GetSession :one
SELECT * FROM sessions WHERE token_hash = ? AND expires_at > ?;

-- name: InsertSession :exec
INSERT INTO sessions (token_hash, user_id, csrf_token, expires_at, created_at) VALUES (?, ?, ?, ?, ?);

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= ?;
