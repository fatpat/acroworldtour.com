-- name: GetPilot :one
SELECT * FROM pilots WHERE civlid = ?;

-- name: ListPilots :many
SELECT * FROM pilots ORDER BY rank, name;

-- name: UpsertPilot :exec
INSERT INTO pilots (civlid, name, country, about, civl_link, photo_url, photo_highres_url, background_url, rank, gender, social_links, sponsors, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (civlid) DO UPDATE SET
    name = excluded.name,
    country = excluded.country,
    about = excluded.about,
    civl_link = excluded.civl_link,
    photo_url = excluded.photo_url,
    photo_highres_url = excluded.photo_highres_url,
    background_url = excluded.background_url,
    rank = excluded.rank,
    gender = excluded.gender,
    social_links = excluded.social_links,
    sponsors = excluded.sponsors,
    updated_at = excluded.updated_at;

-- name: ListPilotAwtYears :many
SELECT year FROM pilot_awt_years WHERE civlid = ? ORDER BY year;

-- name: AddPilotAwtYear :exec
INSERT OR IGNORE INTO pilot_awt_years (civlid, year) VALUES (?, ?);

-- name: RemovePilotAwtYear :exec
DELETE FROM pilot_awt_years WHERE civlid = ? AND year = ?;
