-- name: GetPilotFlight :one
SELECT * FROM flights WHERE competition_id = ? AND run_idx = ? AND civlid = ?;

-- name: GetTeamFlight :one
SELECT * FROM flights WHERE competition_id = ? AND run_idx = ? AND team_id = ?;

-- name: MaxFlightSaveSeq :one
SELECT CAST(COALESCE(MAX(save_seq), 0) AS INTEGER) AS max_seq FROM flights WHERE competition_id = ? AND run_idx = ?;

-- name: GetUniqueTrickByName :one
SELECT * FROM unique_tricks WHERE year = ? AND name = ? ORDER BY acronym LIMIT 1;

-- name: ListSeasonCompetitionIDs :many
-- Active competitions of a season, by name (legacy season order).
SELECT c.id FROM competitions c
JOIN competition_seasons cs ON cs.competition_id = c.id
WHERE cs.season_code = ? AND c.deleted_at IS NULL
ORDER BY c.name, c.id;
