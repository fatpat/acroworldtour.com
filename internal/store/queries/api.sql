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

-- name: ApiCountPublishedCompetitionsOfSeason :one
SELECT COUNT(*) FROM competitions c
JOIN competition_seasons cs ON cs.competition_id = c.id
WHERE cs.season_code = ? AND c.published = 1 AND c.deleted_at IS NULL;

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

-- name: ApiListCompetitionSeasonCodes :many
SELECT season_code FROM competition_seasons WHERE competition_id = ? ORDER BY position;

-- name: ApiCountFlightsOfRun :one
SELECT COUNT(*) FROM flights WHERE competition_id = ? AND run_idx = ? AND published = 1;

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

-- name: ApiCompetitionCounts :one
SELECT
    (SELECT COUNT(*) FROM competition_pilots x WHERE x.competition_id = ?1) AS pilots,
    (SELECT COUNT(*) FROM competition_teams x WHERE x.competition_id = ?1) AS teams,
    (SELECT COUNT(*) FROM competition_judges x WHERE x.competition_id = ?1) AS judges,
    (SELECT COUNT(*) FROM runs x WHERE x.competition_id = ?1) AS runs;
