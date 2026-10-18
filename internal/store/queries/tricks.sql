-- name: GetTrick :one
SELECT * FROM tricks WHERE id = ?;

-- name: ListTricksByYear :many
SELECT * FROM tricks WHERE year = ? AND deleted_at IS NULL ORDER BY name;

-- name: UpsertTrick :exec
INSERT INTO tricks (id, year, name, acronym, solo, solo_awt, synchro, directions, technical_coefficient, bonuses, bonus_constraints,
    first_maneuver, no_first_maneuver, last_maneuver, no_last_maneuver, repeatable, types, sample_video, deleted_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    year = excluded.year,
    name = excluded.name,
    acronym = excluded.acronym,
    solo = excluded.solo,
    solo_awt = excluded.solo_awt,
    synchro = excluded.synchro,
    directions = excluded.directions,
    technical_coefficient = excluded.technical_coefficient,
    bonuses = excluded.bonuses,
    bonus_constraints = excluded.bonus_constraints,
    first_maneuver = excluded.first_maneuver,
    no_first_maneuver = excluded.no_first_maneuver,
    last_maneuver = excluded.last_maneuver,
    no_last_maneuver = excluded.no_last_maneuver,
    repeatable = excluded.repeatable,
    types = excluded.types,
    sample_video = excluded.sample_video,
    deleted_at = excluded.deleted_at;

-- name: ListUniqueTricksByYear :many
SELECT * FROM unique_tricks WHERE year = ? ORDER BY acronym;

-- name: GetUniqueTrick :one
SELECT * FROM unique_tricks WHERE year = ? AND acronym = ?;

-- name: DeleteUniqueTricksOfTrick :exec
DELETE FROM unique_tricks WHERE trick_id = ?;

-- name: InsertUniqueTrick :exec
INSERT INTO unique_tricks (trick_id, year, acronym, name, base_trick, technical_coefficient, bonus, bonus_types, uniqueness, bonuses, solo, solo_awt, synchro, types)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
