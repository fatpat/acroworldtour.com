-- name: GetFlight :one
SELECT * FROM flights WHERE id = ?;

-- name: ListFlightsOfRun :many
SELECT * FROM flights WHERE competition_id = ? AND run_idx = ? ORDER BY id;

-- name: InsertFlight :one
INSERT INTO flights (competition_id, run_idx, civlid, team_id, did_not_start, tip_touches, published, mark_type, save_seq, final_mark, score, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateFlight :exec
UPDATE flights SET did_not_start = ?, tip_touches = ?, published = ?, mark_type = ?, save_seq = ?, final_mark = ?, score = ?, updated_at = ?
WHERE id = ?;

-- name: ListFlightTricks :many
SELECT * FROM flight_tricks WHERE flight_id = ? ORDER BY position;

-- name: DeleteFlightTricks :exec
DELETE FROM flight_tricks WHERE flight_id = ?;

-- name: InsertFlightTrick :exec
INSERT INTO flight_tricks (flight_id, position, name, acronym, base_trick, technical_coefficient, bonus, bonus_types, uniqueness, bonuses,
    solo, solo_awt, synchro, types, technical_mark, technical_marks)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListFlightMarks :many
SELECT * FROM flight_marks WHERE flight_id = ? ORDER BY judge_id;

-- name: DeleteFlightMarks :exec
DELETE FROM flight_marks WHERE flight_id = ?;

-- name: InsertFlightMark :exec
INSERT INTO flight_marks (flight_id, judge_id, technical, technical_per_trick, choreography, landing, synchro)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListFlightWarnings :many
SELECT * FROM flight_warnings WHERE flight_id = ? ORDER BY category, position;

-- name: DeleteFlightWarnings :exec
DELETE FROM flight_warnings WHERE flight_id = ?;

-- name: InsertFlightWarning :exec
INSERT INTO flight_warnings (flight_id, category, position, text) VALUES (?, ?, ?, ?);

-- name: ListRankingEntries :many
SELECT * FROM ranking_entries WHERE scope = ? AND scope_key = ? AND list_key = ? ORDER BY position;

-- name: DeleteRankingEntries :exec
DELETE FROM ranking_entries WHERE scope = ? AND scope_key = ? AND list_key = ?;

-- name: InsertRankingEntry :exec
INSERT INTO ranking_entries (scope, scope_key, list_key, position, rank, civlid, team_id, score, details)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteRankingScope :exec
DELETE FROM ranking_entries WHERE scope = ? AND scope_key = ?;

-- name: ListRankingListKeys :many
SELECT DISTINCT list_key FROM ranking_entries WHERE scope = ? AND scope_key = ? ORDER BY list_key;
