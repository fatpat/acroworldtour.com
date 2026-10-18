-- Read queries of the public JSON API (internal/web/api). Only published,
-- non-deleted competitions are visible.

-- name: ApiListPublishedCompetitions :many
SELECT * FROM competitions WHERE published = 1 AND deleted_at IS NULL ORDER BY start_date DESC, id;

-- name: ApiGetPublishedCompetitionByCode :one
SELECT * FROM competitions WHERE code = ? AND published = 1 AND deleted_at IS NULL;

-- name: ApiListPublishedCompetitionsOfSeason :many
SELECT c.* FROM competitions c
JOIN competition_seasons cs ON cs.competition_id = c.id
WHERE cs.season_code = ? AND c.published = 1 AND c.deleted_at IS NULL
ORDER BY c.start_date, c.id;

-- name: ApiListPublishedFlightsOfRun :many
SELECT * FROM flights WHERE competition_id = ? AND run_idx = ? AND published = 1 ORDER BY id;

-- name: ApiListRankingEntriesOfScope :many
SELECT * FROM ranking_entries WHERE scope = ? AND scope_key = ? ORDER BY list_key, position;

-- name: ApiListCompetitionPilotRows :many
SELECT p.* FROM competition_pilots cp JOIN pilots p ON p.civlid = cp.civlid
WHERE cp.competition_id = ? ORDER BY cp.position;

-- name: ApiListCompetitionTeamRows :many
SELECT t.* FROM competition_teams ct JOIN teams t ON t.id = ct.team_id
WHERE ct.competition_id = ? ORDER BY ct.position;

-- name: ApiListCompetitionJudgeRows :many
SELECT j.* FROM competition_judges cj JOIN judges j ON j.id = cj.judge_id
WHERE cj.competition_id = ? ORDER BY cj.position;

-- name: ApiListPilotCompetitionRanks :many
SELECT c.code AS competition_code, re.rank AS final_rank, re.score AS score
FROM ranking_entries re
JOIN competitions c ON c.id = re.scope_key
WHERE re.scope = 'competition' AND re.list_key = 'overall' AND re.civlid = ?
  AND c.published = 1 AND c.deleted_at IS NULL
ORDER BY c.start_date DESC, c.id;

-- name: ApiListPilotSeasonRanks :many
SELECT s.code AS season_code, re.rank AS final_rank, re.score AS score
FROM ranking_entries re
JOIN seasons s ON s.code = re.scope_key
WHERE re.scope = 'season' AND re.list_key = 'overall' AND re.civlid = ?
  AND s.deleted_at IS NULL
ORDER BY s.year DESC, s.sort_index, s.code;

-- name: ApiListTeamPilots :many
SELECT tp.team_id, p.* FROM team_pilots tp JOIN pilots p ON p.civlid = tp.civlid
ORDER BY tp.team_id, tp.position;

-- name: ApiListPublishedCompetitionCounts :many
SELECT c.id AS competition_id,
    (SELECT COUNT(*) FROM competition_pilots x WHERE x.competition_id = c.id) AS pilots,
    (SELECT COUNT(*) FROM competition_teams x WHERE x.competition_id = c.id) AS teams,
    (SELECT COUNT(*) FROM competition_judges x WHERE x.competition_id = c.id) AS judges,
    (SELECT COUNT(*) FROM runs x WHERE x.competition_id = c.id) AS runs
FROM competitions c WHERE c.published = 1 AND c.deleted_at IS NULL;

-- name: ApiListPublishedCompetitionSeasonCodes :many
SELECT cs.competition_id, cs.season_code FROM competition_seasons cs
JOIN competitions c ON c.id = cs.competition_id
WHERE c.published = 1 AND c.deleted_at IS NULL
ORDER BY cs.competition_id, cs.position;

-- name: ApiCountPublishedFlightsPerRun :many
SELECT run_idx, COUNT(*) AS flights FROM flights WHERE competition_id = ? AND published = 1 GROUP BY run_idx;

-- name: ApiListTricksOfRun :many
SELECT ft.* FROM flight_tricks ft JOIN flights f ON f.id = ft.flight_id
WHERE f.competition_id = ? AND f.run_idx = ? AND f.published = 1 ORDER BY ft.flight_id, ft.position;

-- name: ApiListMarksOfRun :many
SELECT fm.* FROM flight_marks fm JOIN flights f ON f.id = fm.flight_id
WHERE f.competition_id = ? AND f.run_idx = ? AND f.published = 1 ORDER BY fm.flight_id, fm.judge_id;

-- name: ApiListWarningsOfRun :many
SELECT fw.* FROM flight_warnings fw JOIN flights f ON f.id = fw.flight_id
WHERE f.competition_id = ? AND f.run_idx = ? AND f.published = 1 ORDER BY fw.flight_id, fw.category, fw.position;
