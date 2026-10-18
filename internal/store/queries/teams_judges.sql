-- name: GetTeam :one
SELECT * FROM teams WHERE id = ?;

-- name: ListTeams :many
SELECT * FROM teams WHERE deleted_at IS NULL ORDER BY name;

-- name: UpsertTeam :exec
INSERT INTO teams (id, name, deleted_at) VALUES (?, ?, ?)
ON CONFLICT (id) DO UPDATE SET name = excluded.name, deleted_at = excluded.deleted_at;

-- name: ListTeamPilots :many
SELECT * FROM team_pilots WHERE team_id = ? ORDER BY position;

-- name: SetTeamPilot :exec
INSERT INTO team_pilots (team_id, position, civlid) VALUES (?, ?, ?)
ON CONFLICT (team_id, position) DO UPDATE SET civlid = excluded.civlid;

-- name: GetJudge :one
SELECT * FROM judges WHERE id = ?;

-- name: ListJudges :many
SELECT * FROM judges WHERE deleted_at IS NULL ORDER BY name;

-- name: UpsertJudge :exec
INSERT INTO judges (id, name, country, level, civlid, deleted_at) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    country = excluded.country,
    level = excluded.level,
    civlid = excluded.civlid,
    deleted_at = excluded.deleted_at;
