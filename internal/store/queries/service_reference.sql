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
