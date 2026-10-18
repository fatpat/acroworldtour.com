-- name: UpsertPilotIdentity :exec
INSERT INTO pilots (civlid, name, country, about, civl_link, photo_url, photo_highres_url, background_url, social_links, sponsors, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (civlid) DO UPDATE SET
    name = excluded.name,
    country = excluded.country,
    about = excluded.about,
    civl_link = excluded.civl_link,
    photo_url = excluded.photo_url,
    photo_highres_url = excluded.photo_highres_url,
    background_url = excluded.background_url,
    social_links = excluded.social_links,
    sponsors = excluded.sponsors,
    updated_at = excluded.updated_at;

-- name: UpdatePilotRank :exec
UPDATE pilots SET rank = ? WHERE civlid = ?;

-- name: UpdatePilotGender :exec
UPDATE pilots SET gender = ? WHERE civlid = ?;

-- name: GetActiveTrickByYearName :one
SELECT * FROM tricks WHERE year = ? AND name = ? AND deleted_at IS NULL;

-- name: ListUniqueTricksByTrick :many
SELECT * FROM unique_tricks WHERE trick_id = ? ORDER BY rowid;

-- name: ListCompetitionRankingEntries :many
SELECT * FROM ranking_entries WHERE scope = 'competition' AND scope_key = ? ORDER BY list_key, position;

-- name: GetSeason :one
SELECT * FROM seasons WHERE id = ?;

-- name: ListRescorableCompetitionsOfPilot :many
-- Active, non-frozen competitions a pilot takes part in or flew in: a change
-- of the pilot's AWT status or gender changes their scores or lists.
SELECT c.id, c.start_date FROM competitions c
WHERE c.deleted_at IS NULL AND c.frozen = 0
  AND (EXISTS (SELECT 1 FROM competition_pilots cp WHERE cp.competition_id = c.id AND cp.civlid = sqlc.arg(civlid))
    OR EXISTS (SELECT 1 FROM flights f WHERE f.competition_id = c.id AND f.civlid = sqlc.arg(civlid)))
ORDER BY c.start_date, c.id;

-- name: ListRescorableCompetitionsOfJudge :many
-- Active, non-frozen competitions with marks of a judge: a change of the
-- judge's level changes the mark weights.
SELECT DISTINCT c.id FROM competitions c
JOIN flights f ON f.competition_id = c.id
JOIN flight_marks fm ON fm.flight_id = f.id
WHERE fm.judge_id = ? AND c.deleted_at IS NULL AND c.frozen = 0
ORDER BY c.id;
