-- name: GetSeasonByCode :one
SELECT * FROM seasons WHERE code = ?;

-- name: ListSeasons :many
SELECT * FROM seasons WHERE deleted_at IS NULL ORDER BY year DESC, sort_index, code;

-- name: UpsertSeason :exec
INSERT INTO seasons (id, code, name, year, country, image_file, sort_index, deleted_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    code = excluded.code,
    name = excluded.name,
    year = excluded.year,
    country = excluded.country,
    image_file = excluded.image_file,
    sort_index = excluded.sort_index,
    deleted_at = excluded.deleted_at;

-- name: GetCompetition :one
SELECT * FROM competitions WHERE id = ?;

-- name: GetCompetitionByCode :one
SELECT * FROM competitions WHERE code = ? AND deleted_at IS NULL;

-- name: ListCompetitions :many
SELECT * FROM competitions WHERE deleted_at IS NULL ORDER BY start_date DESC, id;

-- name: UpsertCompetition :exec
INSERT INTO competitions (id, code, name, type, state, start_date, end_date, location, published, website, image_file, logo_file,
    config, frozen, results_final, updated_at, deleted_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    code = excluded.code,
    name = excluded.name,
    type = excluded.type,
    state = excluded.state,
    start_date = excluded.start_date,
    end_date = excluded.end_date,
    location = excluded.location,
    published = excluded.published,
    website = excluded.website,
    image_file = excluded.image_file,
    logo_file = excluded.logo_file,
    config = excluded.config,
    frozen = excluded.frozen,
    results_final = excluded.results_final,
    updated_at = excluded.updated_at,
    deleted_at = excluded.deleted_at;

-- name: ListCompetitionSeasons :many
SELECT season_code FROM competition_seasons WHERE competition_id = ? ORDER BY position;

-- name: DeleteCompetitionSeasons :exec
DELETE FROM competition_seasons WHERE competition_id = ?;

-- name: AddCompetitionSeason :exec
INSERT INTO competition_seasons (competition_id, position, season_code) VALUES (?, ?, ?);

-- name: ListCompetitionPilots :many
SELECT civlid FROM competition_pilots WHERE competition_id = ? ORDER BY position;

-- name: DeleteCompetitionPilots :exec
DELETE FROM competition_pilots WHERE competition_id = ?;

-- name: AddCompetitionPilot :exec
INSERT INTO competition_pilots (competition_id, position, civlid) VALUES (?, ?, ?);

-- name: ListCompetitionTeams :many
SELECT team_id FROM competition_teams WHERE competition_id = ? ORDER BY position;

-- name: DeleteCompetitionTeams :exec
DELETE FROM competition_teams WHERE competition_id = ?;

-- name: AddCompetitionTeam :exec
INSERT INTO competition_teams (competition_id, position, team_id) VALUES (?, ?, ?);

-- name: ListCompetitionJudges :many
SELECT judge_id FROM competition_judges WHERE competition_id = ? ORDER BY position;

-- name: DeleteCompetitionJudges :exec
DELETE FROM competition_judges WHERE competition_id = ?;

-- name: AddCompetitionJudge :exec
INSERT INTO competition_judges (competition_id, position, judge_id) VALUES (?, ?, ?);

-- name: ListCompetitionRepeatableTricks :many
SELECT trick_name FROM competition_repeatable_tricks WHERE competition_id = ? ORDER BY position;

-- name: DeleteCompetitionRepeatableTricks :exec
DELETE FROM competition_repeatable_tricks WHERE competition_id = ?;

-- name: AddCompetitionRepeatableTrick :exec
INSERT INTO competition_repeatable_tricks (competition_id, position, trick_name) VALUES (?, ?, ?);

-- name: GetRun :one
SELECT * FROM runs WHERE competition_id = ? AND idx = ?;

-- name: ListRuns :many
SELECT * FROM runs WHERE competition_id = ? ORDER BY idx;

-- name: UpsertRun :exec
INSERT INTO runs (competition_id, idx, state, config, reset_policy, results_final, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (competition_id, idx) DO UPDATE SET
    state = excluded.state,
    config = excluded.config,
    reset_policy = excluded.reset_policy,
    results_final = excluded.results_final,
    updated_at = excluded.updated_at;

-- name: ListRunParticipants :many
SELECT * FROM run_participants WHERE competition_id = ? AND run_idx = ? ORDER BY position;

-- name: DeleteRunParticipants :exec
DELETE FROM run_participants WHERE competition_id = ? AND run_idx = ?;

-- name: AddRunParticipant :exec
INSERT INTO run_participants (competition_id, run_idx, position, civlid, team_id) VALUES (?, ?, ?, ?, ?);

-- name: ListRunJudges :many
SELECT judge_id FROM run_judges WHERE competition_id = ? AND run_idx = ? ORDER BY position;

-- name: DeleteRunJudges :exec
DELETE FROM run_judges WHERE competition_id = ? AND run_idx = ?;

-- name: AddRunJudge :exec
INSERT INTO run_judges (competition_id, run_idx, position, judge_id) VALUES (?, ?, ?, ?);
