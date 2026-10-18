-- Queries of the institutional site (public www pages, content admin, www importer).
-- Basic upserts and slug lookups live in files_content.sql.

-- name: GetNewsByID :one
SELECT * FROM news WHERE id = ?;

-- name: GetNewsBySlugAny :one
-- Includes soft-deleted rows: the slug is unique across all of them.
SELECT * FROM news WHERE slug = ?;

-- name: ListNewsAdmin :many
SELECT * FROM news WHERE deleted_at IS NULL ORDER BY COALESCE(published_at, '9999') DESC, id;

-- name: ListPublishedNews :many
-- A draft has no published_at; a future date is scheduled, not yet public.
SELECT * FROM news
WHERE deleted_at IS NULL AND published_at IS NOT NULL AND published_at <= sqlc.arg(now)
ORDER BY published_at DESC, id
LIMIT sqlc.arg(max) OFFSET sqlc.arg(skip);

-- name: CountPublishedNews :one
SELECT COUNT(*) FROM news
WHERE deleted_at IS NULL AND published_at IS NOT NULL AND published_at <= sqlc.arg(now);

-- name: GetPublishedNewsBySlug :one
SELECT * FROM news
WHERE slug = ? AND deleted_at IS NULL AND published_at IS NOT NULL AND published_at <= sqlc.arg(now);

-- name: ListEventsAdmin :many
SELECT * FROM events WHERE deleted_at IS NULL ORDER BY season_year DESC, start_date DESC, id;

-- name: GetEventBySeasonSlugAny :one
SELECT * FROM events WHERE season_year = ? AND slug = ? AND deleted_at IS NULL;

-- name: GetEventBySlugLatest :one
-- Slugs are unique per season only: the public URL shows the latest season.
SELECT * FROM events WHERE slug = ? AND deleted_at IS NULL ORDER BY season_year DESC LIMIT 1;

-- name: ListEventsFrom :many
SELECT * FROM events WHERE deleted_at IS NULL AND end_date >= ? AND status <> 'cancelled'
ORDER BY start_date, id LIMIT ?;

-- name: ListEventCompetitionsPublished :many
SELECT c.id, c.code, c.name, c.state
FROM event_competitions ec JOIN competitions c ON c.id = ec.competition_id
WHERE ec.event_id = ? AND c.published = 1 AND c.deleted_at IS NULL
ORDER BY c.start_date, c.id;

-- name: DeleteEventCompetitions :exec
DELETE FROM event_competitions WHERE event_id = ?;

-- name: GetCompetitionIDByCode :one
SELECT id FROM competitions WHERE code = ? AND deleted_at IS NULL;

-- name: ListCompetitionChoices :many
SELECT id, code, name FROM competitions WHERE deleted_at IS NULL ORDER BY start_date DESC, id;

-- name: SoftDeleteNews :exec
UPDATE news SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL;

-- name: SoftDeleteEvent :exec
UPDATE events SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL;

-- name: GetDocument :one
SELECT * FROM documents WHERE id = ?;

-- name: DeleteDocument :execrows
DELETE FROM documents WHERE id = ?;

-- name: ListPages :many
SELECT * FROM pages ORDER BY slug;

-- name: DeletePage :execrows
DELETE FROM pages WHERE slug = ?;

-- name: DeleteRedirect :execrows
DELETE FROM redirects WHERE from_path = ?;

-- name: ListSiteSettings :many
SELECT * FROM site_settings ORDER BY key;

-- name: ListSitemapCompetitions :many
SELECT code, end_date FROM competitions WHERE published = 1 AND deleted_at IS NULL ORDER BY start_date DESC, id;

-- name: ListSitemapSeasons :many
SELECT code FROM seasons WHERE deleted_at IS NULL ORDER BY year DESC, sort_index, name;

-- name: ListSitemapPilots :many
SELECT civlid, name FROM pilots ORDER BY civlid;
