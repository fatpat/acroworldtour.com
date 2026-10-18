-- name: CountPilots :one
SELECT count(*) FROM pilots;

-- name: CountCompetitions :one
SELECT count(*) FROM competitions;

-- Delete queries below clear every table the importer fills, children first.

-- name: DeleteAllRankingEntries :exec
DELETE FROM ranking_entries;

-- name: DeleteAllCompetitions :exec
DELETE FROM competitions;

-- name: DeleteAllTeams :exec
DELETE FROM teams;

-- name: DeleteAllJudges :exec
DELETE FROM judges;

-- name: DeleteAllPilots :exec
DELETE FROM pilots;

-- name: DeleteAllSeasons :exec
DELETE FROM seasons;

-- name: DeleteAllTricks :exec
DELETE FROM tricks;

-- name: DeleteAllFiles :exec
DELETE FROM files;
