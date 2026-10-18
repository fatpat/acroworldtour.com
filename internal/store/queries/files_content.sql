-- name: GetFile :one
SELECT * FROM files WHERE id = ?;

-- name: UpsertFile :exec
INSERT INTO files (id, filename, content_type, size, created_at, deleted_at) VALUES (?, ?, ?, ?, ?, NULL)
ON CONFLICT (id) DO UPDATE SET
    filename = excluded.filename,
    content_type = excluded.content_type,
    size = excluded.size,
    deleted_at = NULL;

-- name: GetNewsBySlug :one
SELECT * FROM news WHERE slug = ? AND deleted_at IS NULL;

-- name: ListNews :many
SELECT * FROM news WHERE deleted_at IS NULL ORDER BY published_at DESC, id;

-- name: UpsertNews :exec
INSERT INTO news (id, slug, title, excerpt, body_md, cover_file, author, published_at, deleted_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    slug = excluded.slug,
    title = excluded.title,
    excerpt = excluded.excerpt,
    body_md = excluded.body_md,
    cover_file = excluded.cover_file,
    author = excluded.author,
    published_at = excluded.published_at,
    deleted_at = excluded.deleted_at;

-- name: GetEvent :one
SELECT * FROM events WHERE id = ?;

-- name: ListEventsByYear :many
SELECT * FROM events WHERE season_year = ? AND deleted_at IS NULL ORDER BY start_date;

-- name: UpsertEvent :exec
INSERT INTO events (id, slug, season_year, name, start_date, end_date, site, country, circuits, disciplines, cover_file,
    description_md, website, registration_url, civl_url, live_stream_url, status, deleted_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    slug = excluded.slug,
    season_year = excluded.season_year,
    name = excluded.name,
    start_date = excluded.start_date,
    end_date = excluded.end_date,
    site = excluded.site,
    country = excluded.country,
    circuits = excluded.circuits,
    disciplines = excluded.disciplines,
    cover_file = excluded.cover_file,
    description_md = excluded.description_md,
    website = excluded.website,
    registration_url = excluded.registration_url,
    civl_url = excluded.civl_url,
    live_stream_url = excluded.live_stream_url,
    status = excluded.status,
    deleted_at = excluded.deleted_at;

-- name: ListEventCompetitions :many
SELECT competition_id FROM event_competitions WHERE event_id = ? ORDER BY competition_id;

-- name: AddEventCompetition :exec
INSERT OR IGNORE INTO event_competitions (event_id, competition_id) VALUES (?, ?);

-- name: ListDocuments :many
SELECT * FROM documents ORDER BY year DESC, title;

-- name: UpsertDocument :exec
INSERT INTO documents (id, title, year, category, file_id, url) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    title = excluded.title,
    year = excluded.year,
    category = excluded.category,
    file_id = excluded.file_id,
    url = excluded.url;

-- name: GetPage :one
SELECT * FROM pages WHERE slug = ?;

-- name: UpsertPage :exec
INSERT INTO pages (slug, title, body_md) VALUES (?, ?, ?)
ON CONFLICT (slug) DO UPDATE SET title = excluded.title, body_md = excluded.body_md;

-- name: GetRedirect :one
SELECT * FROM redirects WHERE from_path = ?;

-- name: ListRedirects :many
SELECT * FROM redirects ORDER BY from_path;

-- name: UpsertRedirect :exec
INSERT INTO redirects (from_path, to_url, permanent) VALUES (?, ?, ?)
ON CONFLICT (from_path) DO UPDATE SET to_url = excluded.to_url, permanent = excluded.permanent;

-- name: GetSiteSetting :one
SELECT * FROM site_settings WHERE key = ?;

-- name: UpsertSiteSetting :exec
INSERT INTO site_settings (key, value) VALUES (?, ?)
ON CONFLICT (key) DO UPDATE SET value = excluded.value;
