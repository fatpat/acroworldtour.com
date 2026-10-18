-- name: LiveLatestFlight :one
-- The most recently saved published flight of a run (broadcast overlay).
SELECT id FROM flights
WHERE competition_id = ? AND run_idx = ? AND published = 1
ORDER BY save_seq DESC, id DESC
LIMIT 1;

-- name: PurgeCompetitionPilots :many
-- Pilots whose profile and API results show a competition: its solo
-- participants, its teams' members and anyone with a flight in it.
SELECT p.civlid, p.name FROM pilots p
WHERE p.civlid IN (
    SELECT cp.civlid FROM competition_pilots cp WHERE cp.competition_id = sqlc.arg(competition_id)
    UNION SELECT tp.civlid FROM competition_teams ct JOIN team_pilots tp ON tp.team_id = ct.team_id
        WHERE ct.competition_id = sqlc.arg(competition_id)
    UNION SELECT f.civlid FROM flights f WHERE f.competition_id = sqlc.arg(competition_id) AND f.civlid IS NOT NULL)
ORDER BY p.civlid;
