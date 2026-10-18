-- Read queries of the public results pages (internal/web/public). Only
-- published, non-deleted competitions and non-deleted seasons are visible.

-- name: PublicListCompetitions :many
SELECT c.id, c.code, c.name, c.type, c.state, c.start_date, c.end_date, c.location, c.image_file, c.results_final,
    (SELECT COUNT(*) FROM competition_pilots x WHERE x.competition_id = c.id) AS pilots,
    (SELECT COUNT(*) FROM competition_teams x WHERE x.competition_id = c.id) AS teams
FROM competitions c
WHERE c.published = 1 AND c.deleted_at IS NULL
ORDER BY c.start_date DESC, c.id;

-- name: PublicListCompetitionSeasonLinks :many
SELECT cs.competition_id, cs.season_code
FROM competition_seasons cs
JOIN competitions c ON c.id = cs.competition_id
WHERE c.published = 1 AND c.deleted_at IS NULL
ORDER BY cs.competition_id, cs.position;

-- name: PublicGetCompetition :one
SELECT * FROM competitions WHERE code = ? AND published = 1 AND deleted_at IS NULL;

-- name: PublicCompetitionSeasons :many
SELECT cs.season_code, COALESCE(s.name, '') AS season_name
FROM competition_seasons cs
LEFT JOIN seasons s ON s.code = cs.season_code AND s.deleted_at IS NULL
WHERE cs.competition_id = ?
ORDER BY cs.position;

-- name: PublicRankingEntries :many
-- A ranking scope with the participants' display data.
SELECT re.list_key, re.position, re.rank, re.civlid, re.team_id, re.score, re.details,
    COALESCE(p.name, '') AS pilot_name, COALESCE(p.country, '') AS pilot_country,
    COALESCE(t.name, '') AS team_name
FROM ranking_entries re
LEFT JOIN pilots p ON p.civlid = re.civlid
LEFT JOIN teams t ON t.id = re.team_id
WHERE re.scope = ? AND re.scope_key = ?
ORDER BY re.list_key, re.position;

-- name: PublicRunEntries :many
-- The run rankings of a competition, restricted to published flights.
SELECT f.id AS flight_id, f.run_idx, f.did_not_start, f.final_mark, f.score AS flight_score,
    re.list_key, re.position, re.civlid, re.team_id, re.score,
    COALESCE(p.name, '') AS pilot_name, COALESCE(p.country, '') AS pilot_country,
    COALESCE(t.name, '') AS team_name
FROM flights f
JOIN ranking_entries re ON re.scope = 'run'
    AND re.scope_key = f.competition_id || '/' || f.run_idx
    AND (re.civlid = f.civlid OR re.team_id = f.team_id)
LEFT JOIN pilots p ON p.civlid = re.civlid
LEFT JOIN teams t ON t.id = re.team_id
WHERE f.competition_id = ? AND f.run_idx = ? AND f.published = 1
ORDER BY re.list_key, re.position;

-- name: PublicCompetitionRunPlaces :many
-- Every run's ranking in one list, restricted to published flights: the
-- overall tab's per-run cells re-rank these like the run tabs do.
SELECT f.run_idx, re.position, re.civlid, re.team_id, re.score
FROM flights f
JOIN ranking_entries re ON re.scope = 'run'
    AND re.scope_key = f.competition_id || '/' || f.run_idx
    AND (re.civlid = f.civlid OR re.team_id = f.team_id)
WHERE f.competition_id = ? AND re.list_key = ? AND f.published = 1
ORDER BY f.run_idx, re.position;

-- name: PublicRunFlightTricks :many
SELECT ft.flight_id, ft.position, ft.name, ft.acronym
FROM flight_tricks ft
JOIN flights f ON f.id = ft.flight_id
WHERE f.competition_id = ? AND f.run_idx = ? AND f.published = 1
ORDER BY ft.flight_id, ft.position;

-- name: PublicRunFlightMarks :many
SELECT fm.flight_id, fm.judge_id, fm.technical, fm.choreography, fm.landing, fm.synchro,
    COALESCE(j.name, fm.judge_id) AS judge_name
FROM flight_marks fm
JOIN flights f ON f.id = fm.flight_id
LEFT JOIN judges j ON j.id = fm.judge_id
WHERE f.competition_id = ? AND f.run_idx = ? AND f.published = 1
ORDER BY fm.flight_id, judge_name;

-- name: PublicCompetitionPilots :many
SELECT p.* FROM competition_pilots cp JOIN pilots p ON p.civlid = cp.civlid
WHERE cp.competition_id = ?
ORDER BY p.rank, p.name;

-- name: PublicCompetitionTeams :many
SELECT t.id, t.name FROM competition_teams ct JOIN teams t ON t.id = ct.team_id
WHERE ct.competition_id = ?
ORDER BY t.name;

-- name: PublicCompetitionJudges :many
SELECT j.id, j.name, j.country, j.level, j.civlid,
    COALESCE(p.name, '') AS pilot_name, COALESCE(p.photo_url, '') AS photo_url
FROM competition_judges cj
JOIN judges j ON j.id = cj.judge_id
LEFT JOIN pilots p ON p.civlid = j.civlid
WHERE cj.competition_id = ?
ORDER BY cj.position;

-- name: PublicTeamMembers :many
-- Every team's pilots (a small table), so pages resolve team members in one query.
SELECT tp.team_id, tp.position, p.civlid, p.name, p.country, p.rank, p.photo_url
FROM team_pilots tp JOIN pilots p ON p.civlid = tp.civlid
ORDER BY tp.team_id, tp.position;

-- name: PublicListSeasons :many
SELECT s.id, s.code, s.name, s.year, s.country, s.image_file, s.sort_index,
    (SELECT COUNT(*) FROM competition_seasons cs JOIN competitions c ON c.id = cs.competition_id
        WHERE cs.season_code = s.code AND c.published = 1 AND c.deleted_at IS NULL) AS competitions,
    (SELECT COUNT(DISTINCT cp.civlid) FROM competition_pilots cp
        JOIN competition_seasons cs ON cs.competition_id = cp.competition_id
        JOIN competitions c ON c.id = cp.competition_id
        WHERE cs.season_code = s.code AND c.published = 1 AND c.deleted_at IS NULL) AS pilots,
    (SELECT COUNT(DISTINCT ct.team_id) FROM competition_teams ct
        JOIN competition_seasons cs ON cs.competition_id = ct.competition_id
        JOIN competitions c ON c.id = ct.competition_id
        WHERE cs.season_code = s.code AND c.published = 1 AND c.deleted_at IS NULL) AS teams
FROM seasons s
WHERE s.deleted_at IS NULL
ORDER BY s.year DESC, s.sort_index, s.name;

-- name: PublicSeasonCompetitions :many
SELECT c.id, c.code, c.name, c.type, c.state, c.start_date, c.end_date, c.location, c.image_file, c.results_final,
    (SELECT COUNT(*) FROM competition_pilots x WHERE x.competition_id = c.id) AS pilots,
    (SELECT COUNT(*) FROM competition_teams x WHERE x.competition_id = c.id) AS teams
FROM competitions c
JOIN competition_seasons cs ON cs.competition_id = c.id
WHERE cs.season_code = ? AND c.published = 1 AND c.deleted_at IS NULL
ORDER BY c.start_date, c.id;

-- name: PublicSeasonCompetitionEntries :many
-- Each participant's place in the season's list of every closed, final
-- competition of the season (the competitions the season ranking counts).
SELECT re.scope_key AS competition_id, re.rank, re.score, re.civlid, re.team_id
FROM ranking_entries re
JOIN competitions c ON c.id = re.scope_key
WHERE re.scope = 'competition' AND re.list_key = ?1
    AND c.published = 1 AND c.deleted_at IS NULL AND c.state = 'closed' AND c.results_final = 1
    AND EXISTS (SELECT 1 FROM competition_seasons cs WHERE cs.competition_id = c.id AND cs.season_code = ?1)
ORDER BY c.start_date, re.position;

-- name: PublicSeasonPilots :many
SELECT DISTINCT p.civlid, p.name, p.country, p.rank, p.photo_url
FROM pilots p
JOIN competition_pilots cp ON cp.civlid = p.civlid
JOIN competition_seasons cs ON cs.competition_id = cp.competition_id
JOIN competitions c ON c.id = cp.competition_id
WHERE cs.season_code = ? AND c.published = 1 AND c.deleted_at IS NULL
ORDER BY p.rank, p.name;

-- name: PublicAllAwtYears :many
SELECT civlid, year FROM pilot_awt_years ORDER BY civlid, year;

-- name: PublicPilotCompetitionResults :many
-- A pilot's (or team's) places in the overall list and in the lists of the
-- competition's own seasons.
SELECT c.code, c.name, c.start_date, c.results_final, re.list_key, re.rank, re.score,
    COALESCE(s.name, '') AS season_name
FROM ranking_entries re
JOIN competitions c ON c.id = re.scope_key
LEFT JOIN seasons s ON s.code = re.list_key AND s.deleted_at IS NULL
WHERE re.scope = 'competition' AND (re.civlid = sqlc.narg('civlid') OR re.team_id = sqlc.narg('team_id'))
    AND c.published = 1 AND c.deleted_at IS NULL
    AND (re.list_key = 'overall' OR EXISTS (
        SELECT 1 FROM competition_seasons cs WHERE cs.competition_id = c.id AND cs.season_code = re.list_key))
ORDER BY c.start_date DESC, c.id, re.list_key;

-- name: PublicPilotSeasonResults :many
SELECT s.code, s.name, s.year, re.list_key, re.rank, re.score
FROM ranking_entries re
JOIN seasons s ON s.code = re.scope_key AND s.deleted_at IS NULL
WHERE re.scope = 'season' AND (re.civlid = sqlc.narg('civlid') OR re.team_id = sqlc.narg('team_id'))
ORDER BY s.year DESC, s.sort_index, s.code, re.list_key;

-- name: PublicListJudges :many
SELECT j.id, j.name, j.country, j.level, j.civlid,
    COALESCE(p.name, '') AS pilot_name, COALESCE(p.photo_url, '') AS photo_url
FROM judges j
LEFT JOIN pilots p ON p.civlid = j.civlid
WHERE j.deleted_at IS NULL
ORDER BY j.name;

-- name: PublicTrickYears :many
SELECT DISTINCT year FROM tricks WHERE deleted_at IS NULL ORDER BY year DESC;

-- name: PublicGetActiveFile :one
SELECT * FROM files WHERE id = ? AND deleted_at IS NULL;
