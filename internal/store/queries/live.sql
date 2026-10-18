-- name: LiveLatestFlight :one
-- The most recently saved published flight of a run (broadcast overlay).
SELECT id FROM flights
WHERE competition_id = ? AND run_idx = ? AND published = 1
ORDER BY save_seq DESC, id DESC
LIMIT 1;
