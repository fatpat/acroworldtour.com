-- Initial schema. Timestamps are RFC 3339 UTC text, JSON columns are text
-- validated with json_valid. SQLite treats NULLs as distinct in UNIQUE
-- constraints, so uniqueness involving deleted_at / nullable participants uses
-- partial unique indexes.

CREATE TABLE rulesets (
    year INTEGER PRIMARY KEY,
    config TEXT NOT NULL CHECK (json_valid(config)),
    max_tricks_per_type TEXT NOT NULL CHECK (json_valid(max_tricks_per_type)),
    tip_touch_pct_per_touch REAL NOT NULL,
    tip_touch_pct_max REAL NOT NULL,
    default_max_bonus REAL NOT NULL
);

CREATE TABLE pilots (
    civlid INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT '',
    about TEXT NOT NULL DEFAULT '',
    civl_link TEXT NOT NULL DEFAULT '',
    photo_url TEXT NOT NULL DEFAULT '',
    photo_highres_url TEXT NOT NULL DEFAULT '',
    background_url TEXT NOT NULL DEFAULT '',
    rank INTEGER NOT NULL DEFAULT 9999,
    gender TEXT NOT NULL DEFAULT 'man' CHECK (gender IN ('man', 'woman', 'none')),
    social_links TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(social_links)),
    sponsors TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(sponsors)),
    updated_at TEXT
);

CREATE TABLE pilot_awt_years (
    civlid INTEGER NOT NULL REFERENCES pilots (civlid) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    PRIMARY KEY (civlid, year)
);

CREATE TABLE teams (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    deleted_at TEXT
);
CREATE UNIQUE INDEX teams_name_active ON teams (name) WHERE deleted_at IS NULL;

CREATE TABLE team_pilots (
    team_id TEXT NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 1),
    civlid INTEGER NOT NULL REFERENCES pilots (civlid),
    PRIMARY KEY (team_id, position)
);

CREATE TABLE judges (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    country TEXT NOT NULL,
    level TEXT NOT NULL CHECK (level IN ('trainee', 'certified', 'senior')),
    civlid INTEGER REFERENCES pilots (civlid),
    deleted_at TEXT
);
CREATE UNIQUE INDEX judges_name_active ON judges (name) WHERE deleted_at IS NULL;

CREATE TABLE tricks (
    id TEXT PRIMARY KEY,
    year INTEGER NOT NULL,
    name TEXT NOT NULL,
    acronym TEXT NOT NULL,
    solo BOOLEAN NOT NULL CHECK (solo IN (0, 1)),
    solo_awt BOOLEAN NOT NULL DEFAULT 1 CHECK (solo_awt IN (0, 1)),
    synchro BOOLEAN NOT NULL CHECK (synchro IN (0, 1)),
    directions TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(directions)),
    technical_coefficient REAL NOT NULL CHECK (technical_coefficient >= 0),
    bonuses TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(bonuses)),
    bonus_constraints TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(bonus_constraints)),
    first_maneuver INTEGER NOT NULL DEFAULT 0 CHECK (first_maneuver >= 0),
    no_first_maneuver INTEGER NOT NULL DEFAULT 0 CHECK (no_first_maneuver >= 0),
    last_maneuver INTEGER NOT NULL DEFAULT 0 CHECK (last_maneuver >= 0),
    no_last_maneuver INTEGER NOT NULL DEFAULT 0 CHECK (no_last_maneuver >= 0),
    repeatable BOOLEAN NOT NULL DEFAULT 0 CHECK (repeatable IN (0, 1)),
    types TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(types)),
    sample_video TEXT,
    deleted_at TEXT
);
CREATE UNIQUE INDEX tricks_year_name_active ON tricks (year, name) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX tricks_year_acronym_active ON tricks (year, acronym) WHERE deleted_at IS NULL;

-- Rule: a trick's unique_tricks rows are deleted when the trick is soft-deleted
-- (otherwise UNIQUE(year, acronym) would block re-creating it).
CREATE TABLE unique_tricks (
    trick_id TEXT NOT NULL REFERENCES tricks (id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    acronym TEXT NOT NULL,
    name TEXT NOT NULL,
    base_trick TEXT NOT NULL DEFAULT '',
    technical_coefficient REAL NOT NULL,
    bonus REAL NOT NULL,
    bonus_types TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(bonus_types)),
    uniqueness TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(uniqueness)),
    bonuses TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(bonuses)),
    solo BOOLEAN NOT NULL CHECK (solo IN (0, 1)),
    solo_awt BOOLEAN NOT NULL CHECK (solo_awt IN (0, 1)),
    synchro BOOLEAN NOT NULL CHECK (synchro IN (0, 1)),
    types TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(types)),
    UNIQUE (year, acronym)
);
CREATE INDEX unique_tricks_trick ON unique_tricks (trick_id);

CREATE TABLE files (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    deleted_at TEXT
);

CREATE TABLE seasons (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    year INTEGER NOT NULL,
    country TEXT,
    image_file TEXT REFERENCES files (id),
    sort_index INTEGER NOT NULL DEFAULT 999,
    deleted_at TEXT
);

CREATE UNIQUE INDEX seasons_code_active ON seasons (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX seasons_name_active ON seasons (name) WHERE deleted_at IS NULL;

CREATE TABLE competitions (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('solo', 'synchro')),
    state TEXT NOT NULL DEFAULT 'init' CHECK (state IN ('init', 'open', 'closed')),
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL,
    location TEXT NOT NULL DEFAULT '',
    published BOOLEAN NOT NULL DEFAULT 0 CHECK (published IN (0, 1)),
    website TEXT,
    image_file TEXT REFERENCES files (id),
    logo_file TEXT REFERENCES files (id),
    config TEXT NOT NULL CHECK (json_valid(config)),
    frozen BOOLEAN NOT NULL DEFAULT 0 CHECK (frozen IN (0, 1)),
    results_final BOOLEAN NOT NULL DEFAULT 0 CHECK (results_final IN (0, 1)),
    updated_at TEXT,
    deleted_at TEXT
);

CREATE UNIQUE INDEX competitions_code_active ON competitions (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX competitions_name_active ON competitions (name) WHERE deleted_at IS NULL;

CREATE TABLE competition_seasons (
    competition_id TEXT NOT NULL REFERENCES competitions (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    season_code TEXT NOT NULL,
    PRIMARY KEY (competition_id, position),
    UNIQUE (competition_id, season_code)
);

CREATE TABLE competition_pilots (
    competition_id TEXT NOT NULL REFERENCES competitions (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    civlid INTEGER NOT NULL REFERENCES pilots (civlid),
    PRIMARY KEY (competition_id, position),
    UNIQUE (competition_id, civlid)
);

CREATE TABLE competition_teams (
    competition_id TEXT NOT NULL REFERENCES competitions (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    team_id TEXT NOT NULL REFERENCES teams (id),
    PRIMARY KEY (competition_id, position),
    UNIQUE (competition_id, team_id)
);

CREATE TABLE competition_judges (
    competition_id TEXT NOT NULL REFERENCES competitions (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    judge_id TEXT NOT NULL REFERENCES judges (id),
    PRIMARY KEY (competition_id, position),
    UNIQUE (competition_id, judge_id)
);

CREATE TABLE competition_repeatable_tricks (
    competition_id TEXT NOT NULL REFERENCES competitions (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    trick_name TEXT NOT NULL,
    PRIMARY KEY (competition_id, position)
);

CREATE TABLE runs (
    competition_id TEXT NOT NULL REFERENCES competitions (id) ON DELETE CASCADE,
    idx INTEGER NOT NULL CHECK (idx >= 0),
    state TEXT NOT NULL DEFAULT 'init' CHECK (state IN ('init', 'open', 'closed')),
    config TEXT NOT NULL CHECK (json_valid(config)),
    reset_policy INTEGER NOT NULL DEFAULT 0 CHECK (reset_policy BETWEEN 0 AND 3),
    results_final BOOLEAN NOT NULL DEFAULT 0 CHECK (results_final IN (0, 1)),
    updated_at TEXT,
    PRIMARY KEY (competition_id, idx)
);

CREATE TABLE run_participants (
    competition_id TEXT NOT NULL,
    run_idx INTEGER NOT NULL,
    position INTEGER NOT NULL,
    civlid INTEGER REFERENCES pilots (civlid),
    team_id TEXT REFERENCES teams (id),
    PRIMARY KEY (competition_id, run_idx, position),
    FOREIGN KEY (competition_id, run_idx) REFERENCES runs (competition_id, idx) ON DELETE CASCADE,
    CHECK ((civlid IS NULL) <> (team_id IS NULL))
);

CREATE TABLE run_judges (
    competition_id TEXT NOT NULL,
    run_idx INTEGER NOT NULL,
    position INTEGER NOT NULL,
    judge_id TEXT NOT NULL REFERENCES judges (id),
    PRIMARY KEY (competition_id, run_idx, position),
    FOREIGN KEY (competition_id, run_idx) REFERENCES runs (competition_id, idx) ON DELETE CASCADE
);

CREATE TABLE flights (
    id INTEGER PRIMARY KEY,
    competition_id TEXT NOT NULL,
    run_idx INTEGER NOT NULL,
    civlid INTEGER REFERENCES pilots (civlid),
    team_id TEXT REFERENCES teams (id),
    did_not_start BOOLEAN NOT NULL DEFAULT 0 CHECK (did_not_start IN (0, 1)),
    tip_touches INTEGER NOT NULL DEFAULT 0,
    published BOOLEAN NOT NULL DEFAULT 0 CHECK (published IN (0, 1)),
    mark_type TEXT CHECK (mark_type IN ('awt', 'awq')),
    save_seq INTEGER NOT NULL DEFAULT 0,
    final_mark TEXT CHECK (final_mark IS NULL OR json_valid(final_mark)),
    score REAL,
    updated_at TEXT,
    FOREIGN KEY (competition_id, run_idx) REFERENCES runs (competition_id, idx) ON DELETE CASCADE,
    CHECK ((civlid IS NULL) <> (team_id IS NULL))
);
CREATE UNIQUE INDEX flights_run_pilot ON flights (competition_id, run_idx, civlid) WHERE civlid IS NOT NULL;
CREATE UNIQUE INDEX flights_run_team ON flights (competition_id, run_idx, team_id) WHERE team_id IS NOT NULL;

CREATE TABLE flight_tricks (
    flight_id INTEGER NOT NULL REFERENCES flights (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    name TEXT NOT NULL,
    acronym TEXT NOT NULL,
    base_trick TEXT NOT NULL DEFAULT '',
    technical_coefficient REAL NOT NULL,
    bonus REAL NOT NULL,
    bonus_types TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(bonus_types)),
    uniqueness TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(uniqueness)),
    bonuses TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(bonuses)),
    solo BOOLEAN CHECK (solo IN (0, 1)),
    solo_awt BOOLEAN CHECK (solo_awt IN (0, 1)),
    synchro BOOLEAN CHECK (synchro IN (0, 1)),
    types TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(types)),
    technical_mark REAL,
    technical_marks TEXT CHECK (technical_marks IS NULL OR json_valid(technical_marks)),
    PRIMARY KEY (flight_id, position)
);

CREATE TABLE flight_marks (
    flight_id INTEGER NOT NULL REFERENCES flights (id) ON DELETE CASCADE,
    judge_id TEXT NOT NULL,
    technical REAL,
    technical_per_trick TEXT CHECK (technical_per_trick IS NULL OR json_valid(technical_per_trick)),
    choreography REAL,
    landing REAL,
    synchro REAL,
    PRIMARY KEY (flight_id, judge_id)
);

CREATE TABLE flight_warnings (
    flight_id INTEGER NOT NULL REFERENCES flights (id) ON DELETE CASCADE,
    category INTEGER NOT NULL CHECK (category BETWEEN 1 AND 2),
    position INTEGER NOT NULL,
    text TEXT NOT NULL,
    PRIMARY KEY (flight_id, category, position)
);

CREATE TABLE ranking_entries (
    scope TEXT NOT NULL CHECK (scope IN ('run', 'competition', 'season')),
    scope_key TEXT NOT NULL,
    list_key TEXT NOT NULL,
    position INTEGER NOT NULL,
    rank INTEGER NOT NULL,
    civlid INTEGER REFERENCES pilots (civlid),
    team_id TEXT REFERENCES teams (id),
    score REAL NOT NULL,
    details TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(details)),
    PRIMARY KEY (scope, scope_key, list_key, position)
);

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'scorer')),
    disabled BOOLEAN NOT NULL DEFAULT 0 CHECK (disabled IN (0, 1))
);

CREATE TABLE user_competitions (
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    competition_id TEXT NOT NULL REFERENCES competitions (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, competition_id)
);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    csrf_token TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE news (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    excerpt TEXT NOT NULL DEFAULT '',
    body_md TEXT NOT NULL DEFAULT '',
    cover_file TEXT REFERENCES files (id),
    author TEXT NOT NULL DEFAULT '',
    published_at TEXT,
    deleted_at TEXT
);

CREATE TABLE events (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL,
    season_year INTEGER NOT NULL,
    name TEXT NOT NULL,
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL,
    site TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    circuits TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(circuits)),
    disciplines TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(disciplines)),
    cover_file TEXT REFERENCES files (id),
    description_md TEXT NOT NULL DEFAULT '',
    website TEXT,
    registration_url TEXT,
    civl_url TEXT,
    live_stream_url TEXT,
    status TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'confirmed', 'cancelled', 'finished')),
    deleted_at TEXT
);
CREATE UNIQUE INDEX events_season_slug_active ON events (season_year, slug) WHERE deleted_at IS NULL;

CREATE TABLE event_competitions (
    event_id TEXT NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    competition_id TEXT NOT NULL REFERENCES competitions (id),
    PRIMARY KEY (event_id, competition_id)
);

CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    year INTEGER,
    category TEXT NOT NULL DEFAULT '',
    file_id TEXT REFERENCES files (id),
    url TEXT
);

CREATE TABLE pages (
    slug TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body_md TEXT NOT NULL DEFAULT ''
);

CREATE TABLE redirects (
    from_path TEXT PRIMARY KEY,
    to_url TEXT NOT NULL,
    permanent BOOLEAN NOT NULL DEFAULT 1 CHECK (permanent IN (0, 1))
);

CREATE TABLE site_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL CHECK (json_valid(value))
);
