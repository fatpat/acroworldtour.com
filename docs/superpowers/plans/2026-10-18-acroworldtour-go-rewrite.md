# AcroWorldTour Go Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the legacy AcroWorldTour stack (FastAPI API, Next.js manager, Next.js results site, static www site, nginx proxy) with one Go binary backed by SQLite, migrated from the production MongoDB export, whose scoring engine matches the legacy engine on every production flight.

**Architecture:** A pure scoring engine (`internal/scoring`) and a pure ranking engine (`internal/ranking`) sit under a service layer (`internal/service`) that writes to SQLite in transactions, scoring on write and storing all rankings. A single `net/http` server renders public, institutional and admin pages with `templ` and htmx, and pushes live updates over SSE. A one-shot importer loads the Mongo export. A Python generator run against the legacy code produces the golden corpus the Go engine must match.

**Tech Stack:** Go 1.26 (`toolchain go1.26.4`), `net/http`, `templ`, htmx 2 (vendored), `modernc.org/sqlite`, `sqlc`, `golang.org/x/crypto` (bcrypt), `golang.org/x/net/html`, `github.com/yuin/goldmark`, `github.com/minio/minio-go/v7`, `github.com/xuri/excelize/v2`, `playwright-go` (tests only), Litestream, Caddy, Docker Compose.

**Spec:** `CLAUDE.md` (project rules, mandatory) and `specifications/01..08`. The scoring spec `specifications/03-scoring-rules.md` is normative for Tasks 3–8.

## Global Constraints

- Every rule in `CLAUDE.md` applies, in particular "Never" and "Scoring engine rules".
- Go module path: `awt`. Binary: `cmd/awt`. Subcommands: `serve`, `migrate`, `import`, `user`, `golden-verify`.
- SQLite opened with `_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)`; one write connection (`SetMaxOpenConns(1)` on the writer pool), a separate read pool.
- All timestamps stored as RFC 3339 UTC text. IDs: legacy string ids kept as-is on import; new ids are 24-char lowercase hex (same shape as Mongo ObjectIds).
- Rounding: only `scoring.Round3`. No `math.Round(x*1000)/1000` anywhere (enforced by a `staticcheck`-independent grep in `make lint`).
- Allowed dependencies are exactly those in Tech Stack plus `golang.org/x/*`. Each non-`CLAUDE.md` dependency (goldmark, minio-go, excelize) gets a decision record in Task 1.
- The production export lives at `./mongo.backup.20261018-2128.tar.gz` (git-ignored). Nothing derived from it is committed except the anonymised golden corpus (ids, numbers and trick acronyms only: no names, no photos, no free-text warnings — warning texts replaced by `"w"`).
- Default scoring config values (spec 03 §2) are the seed of the 2022–2026 rulesets; they are data in the `rulesets` table, never Go constants outside `internal/store/seed`.
- UI copy is English, matches the legacy wording for scoring notes (notes strings are part of the golden comparison).

## Review Focus

1. **Two scorers saving flights in the same competition at the same time** must both be persisted with consistent rankings (legacy loses one). Test in Task 13: `TestConcurrentFlightSaves`.
2. **A judge typing `7,5`, `75` or leaving a mark blank** must get a clear validation message, never a silent `/10` or a 0 (legacy bugs). Test in Task 21: `TestMarkInputParsing`.
3. **Re-saving an earlier run's flight** (adding a warning, changing a trick) must update later runs' malus, penalties and DSQ. Test in Task 13: `TestResaveEarlierRunRescoresLaterRuns`.
4. **Empty states**: a competition with no runs, a season with no closed competition, a pilot with no results, a run with no published flight must render a page, not a 500. Test in Task 16: `TestPublicPagesEmptyStates`.
5. **Unicode names** (`Élite`, `Şahin`, apostrophes) must produce correct slugs, survive xlsx export intact and be HTML-escaped in pages and the overlay SVG. Tests in Task 9 (`TestSlug`) and Task 22 (`TestExportsUnicodeAndEscaping`).

---

## File Structure

```
.gitignore  Makefile  go.mod  go.sum  sqlc.yaml  Dockerfile  compose.yaml  Caddyfile  litestream.yml
.github/workflows/ci.yml  .github/workflows/deploy.yml
cmd/awt/main.go                       subcommand dispatch
internal/config/config.go             env → Config
internal/store/db.go                  Open, pools, Tx helper
internal/store/migrations/*.sql       numbered migrations (embedded)
internal/store/migrate.go
internal/store/queries/*.sql          sqlc input, one file per aggregate
internal/store/gen/                   sqlc output (committed)
internal/store/seed/rulesets.go       default rulesets 2022–2026
internal/blob/blob.go fs.go s3.go     file storage interface + backends
internal/scoring/round.go average.go types.go engine.go steps_*.go
internal/catalog/variants.go          unique-trick generation
internal/ranking/run.go competition.go season.go startorder.go
internal/domain/*.go                  entities, state machines, slug, validation
internal/service/*.go                 use cases (one file per aggregate) + events.go
internal/auth/*.go                    users, sessions, csrf, middleware, ratelimit
internal/civl/client.go               CIVL scraping
internal/export/xlsx.go html.go svg.go
internal/importer/*.go                Mongo export → SQLite
internal/web/server.go routes.go middleware.go sse.go cache.go
internal/web/layout/*.templ           shared layouts
internal/web/public/*.go *.templ      results + simulator + www pages
internal/web/admin/*.go *.templ       back-office + judging
internal/web/api/*.go                 JSON read API /api/v1
web/static/{css,js,img,vendor}/
tools/golden/generate.py              legacy-oracle corpus generator (runs in legacy venv)
testdata/golden/flights/*.json        flight cases (one file per competition)
testdata/golden/rankings/*.json       ranking cases (one file per competition / season)
testdata/catalog/*.json               trick catalogs per year (public data)
e2e/*_test.go                         playwright-go workflows
docs/decisions/NNNN-*.md  docs/ops.md
```

---

### Task 1: Repository bootstrap, decision records, CI skeleton

**Files:**
- Create: `.gitignore`, `go.mod`, `Makefile`, `cmd/awt/main.go`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/web/server.go`, `internal/web/server_test.go`, `.github/workflows/ci.yml`, `docs/decisions/0000-template.md`, `docs/decisions/0001..0012-*.md`

**Interfaces:**
- Produces: `config.Load(getenv func(string) string) (Config, error)`; `Config{Addr, DBPath, BlobBackend ("fs"|"s3"), BlobDir, S3Endpoint, S3Bucket, S3AccessKey, S3SecretKey, BaseURL, Env ("dev"|"training"|"prod"), CloudflareZoneID, CloudflareToken, SessionSecret}`; `web.NewServer(cfg config.Config, deps Deps) *http.Server` with `GET /healthz` → `200 ok`.

- [ ] **Step 1:** `git init`; `.gitignore` covers `legacy/`, `mongo.backup.*`, `*.db*`, `.env*`, `/data/`, `/bin/`, `e2e/.artifacts/`.
- [ ] **Step 2:** `go mod init awt`, `go 1.26`, `toolchain go1.26.4`; add tools with `go get -tool github.com/a-h/templ/cmd/templ github.com/sqlc-dev/sqlc/cmd/sqlc honnef.co/go/tools/cmd/staticcheck golang.org/x/vuln/cmd/govulncheck` (pin current releases).
- [ ] **Step 3:** Write `TestLoadRequiresSessionSecretOutsideDev` (Env=prod, SessionSecret="" → error mentioning `AWT_SESSION_SECRET`), `TestLoadDefaults` (Env=dev → Addr ":8080", DBPath "data/awt.db", BlobBackend "fs", BlobDir "data/blobs"). Env var names: `AWT_` + upper snake of field.
- [ ] **Step 4:** Implement `config.Load`; run `go test ./internal/config/` → PASS.
- [ ] **Step 5:** Write `TestHealthz` (httptest, expects 200 and body `ok`), implement `web.NewServer`, `cmd/awt serve`.
- [ ] **Step 6:** Makefile targets exactly as listed in `CLAUDE.md` "Commands"; `lint` also runs `! grep -rn 'math.Round(' --include=*.go internal/ cmd/`; `check` fails if `git status --porcelain` is non-empty after `generate`. CI runs `make check` on push and PR.
- [ ] **Step 7:** Decision records (context / decision / consequences, ≤ 20 lines each):
  - 0001 SQLite + Litestream (08 §6.2). 0002 Go + templ + htmx (08 §7).
  - 0003 Historical results are frozen: imported `final_marks` and legacy-computed rankings are kept; flights of imported competitions are never rescored.
  - 0004 Repetition reset policy only considers runs `0..run_i` (fixes 03 §14 #9). Golden cases are generated with the competition truncated to `run_i+1` runs, which is how flights were originally scored.
  - 0005 Kept legacy behaviour (spec 08 §2.1 "Decide" items, until the owner decides): unpublished flights count in competition totals (#5), cumulative warning penalty (#7), automatic warnings not carried to later runs (#8), synchro percentages 20/20/20/40 (#18).
  - 0006 Fixed: `pilots_to_qualify` keeps the best N (#1); previous run order not mutated (#2); per-run ranks stored per ranking list (#3–4); `last_maneuver` warns when the trick is *not* in the last N (#6); later runs rescored on save (#14); flights only saved in an `open` run (#15); explicit null mark = no mark and marks must be within 0–10 (#12–13); position rules read from the run's catalog snapshot year (#16).
  - 0007 Ties: every ranking list ordered by `(score desc, display name asc, id asc)`; displayed rank is shared for equal Round3 scores ("1, 2, 2, 4").
  - 0008 Trick catalog versioned by year; a competition uses the catalog of `start_date.year`.
  - 0009 Accounts: roles `admin` and `scorer`; scorers are assigned per competition.
  - 0010 goldmark for Markdown content. 0011 minio-go for object storage. 0012 excelize for CIVL xlsx.
- [ ] **Step 8:** `make lint test` → PASS. Commit `chore: bootstrap repository, config, healthz, decisions`.

---

### Task 2: Database schema, migrations, store plumbing, blob storage

**Files:**
- Create: `internal/store/db.go`, `internal/store/migrate.go`, `internal/store/migrations/0001_init.sql`, `sqlc.yaml`, `internal/store/queries/*.sql`, `internal/store/gen/*`, `internal/store/seed/rulesets.go`, `internal/store/store_test.go`, `internal/blob/{blob.go,fs.go,s3.go,fs_test.go}`

**Interfaces:**
- Produces: `store.Open(path string) (*DB, error)`; `DB.Read *sql.DB`, `DB.Write *sql.DB`; `DB.Tx(ctx, func(q *gen.Queries) error) error` (BEGIN IMMEDIATE, rollback on error); `store.Migrate(ctx, db) error` (embedded files, `schema_migrations(version)`); `blob.Store` interface `{Put(ctx, key, contentType string, r io.Reader) error; Get(ctx, key) (io.ReadCloser, string, error); Delete(ctx, key) error}`; `blob.NewFS(dir)`, `blob.NewS3(cfg)`.

Tables (columns abbreviated; every table with soft delete has `deleted_at TEXT NULL`):

| Table | Key columns |
|---|---|
| `rulesets` | `year PK`, `config JSON` (spec 02 §2.6 shape), `max_tricks_per_type JSON`, `tip_touch_pct_per_touch REAL`, `tip_touch_pct_max REAL`, `default_max_bonus REAL` |
| `pilots` | `civlid PK`, `name`, `country`, `about`, `civl_link`, `photo_url`, `photo_highres_url`, `background_url`, `rank`, `gender CHECK(man,woman,none)`, `social_links JSON`, `sponsors JSON`, `updated_at` |
| `pilot_awt_years` | `(civlid, year) PK` |
| `teams` / `team_pilots` | `id PK, name, deleted_at` / `(team_id, position 0..1) PK, civlid FK` |
| `judges` | `id PK, name, country, level CHECK(trainee,certified,senior), civlid NULL FK, deleted_at` |
| `tricks` | `id PK, year, name, acronym, solo, solo_awt, synchro, directions JSON, technical_coefficient, bonuses JSON, bonus_constraints JSON, first_maneuver, no_first_maneuver, last_maneuver, no_last_maneuver, repeatable, types JSON, sample_video, deleted_at, UNIQUE(year,name,deleted_at)` |
| `unique_tricks` | `trick_id FK, year, acronym, name, technical_coefficient, bonus, bonus_types JSON, uniqueness JSON, bonuses JSON, solo, solo_awt, synchro, types JSON, UNIQUE(year, acronym)` |
| `seasons` | `id PK, code UNIQUE, name, year, country NULL, image_file NULL, sort_index, deleted_at` |
| `competitions` | `id PK, code UNIQUE, name, type, state, start_date, end_date, location, published, website, image_file, logo_file, config JSON, frozen, results_final, updated_at, deleted_at` |
| `competition_seasons` / `_pilots` / `_teams` / `_judges` / `_repeatable_tricks` | `(competition_id, position)` + `season_code` / `civlid` / `team_id` / `judge_id` / `trick_name` |
| `runs` | `(competition_id, idx) PK, state, config JSON, reset_policy, results_final, updated_at` |
| `run_participants` / `run_judges` | `(competition_id, run_idx, position)` + `civlid NULL, team_id NULL` / `judge_id` |
| `flights` | `id INTEGER PK, competition_id, run_idx, civlid NULL, team_id NULL, did_not_start, tip_touches, published, mark_type, save_seq, final_mark JSON, score REAL, updated_at, UNIQUE(competition_id, run_idx, civlid, team_id)` |
| `flight_tricks` | `(flight_id, position) PK`, full unique-trick snapshot columns, `technical_mark NULL`, `technical_marks JSON NULL` |
| `flight_marks` | `(flight_id, judge_id) PK, technical NULL, technical_per_trick JSON NULL, choreography NULL, landing NULL, synchro NULL` |
| `flight_warnings` | `(flight_id, category 1..2, position) PK, text` |
| `ranking_entries` | `scope ('run','competition','season'), scope_key ('<comp_id>:<idx>' / '<comp_id>' / '<season_code>'), list_key ('overall', season code, 'women'), position, rank, civlid NULL, team_id NULL, score, details JSON` ; PK `(scope, scope_key, list_key, position)` |
| `users` / `user_competitions` / `sessions` | `id, email UNIQUE, name, password_hash, role, disabled` / `(user_id, competition_id)` / `token_hash PK, user_id, csrf_token, expires_at, created_at` |
| `files` | `id PK (sha256 hex), filename, content_type, size, created_at, deleted_at` (bytes live in blob store under key = id) |
| `news`, `events`, `event_competitions`, `documents`, `pages`, `redirects`, `site_settings` | per spec 07 §10.1 trimmed: news `(id, slug UNIQUE, title, excerpt, body_md, cover_file, author, published_at NULL)`; events `(id, slug, season_year, name, start_date, end_date, site, country, circuits JSON, disciplines JSON, cover_file, description_md, website, registration_url, civl_url, live_stream_url, status)`; documents `(id, title, year, category, file_id NULL, url NULL)`; pages `(slug PK, title, body_md)`; redirects `(from_path PK, to_url, permanent)`; site_settings `(key PK, value JSON)` |

- [ ] **Step 1:** Write `TestMigrateCreatesSchemaAndEnforcesFKs` (open temp file, migrate, insert `team_pilots` with unknown civlid → FK error), `TestMigrateIsIdempotent`, `TestTxRollsBackOnError`.
- [ ] **Step 2:** Write migration + `db.go` + `migrate.go`; run tests → PASS.
- [ ] **Step 3:** `seed.Rulesets()` returns 2022–2026 rows with spec 03 §2 defaults (warning 0.5, warning2 1, malus 13, dsq 3, weights 100/100/20, solo 40/40/20, synchro 20/20/20/40, max bonus twist 5 / reverse 3 / flip 2, max per type tumbling 2 / "to infinite" 1, tip touch 2 %/6 %, default max bonus 10). `store.Migrate` inserts missing rulesets. Test `TestSeedRulesets2025`.
- [ ] **Step 4:** sqlc config (engine sqlite, emit JSON tags); minimal queries per table (get/list/upsert); `make generate` clean.
- [ ] **Step 5:** `blob.FS` with `TestFSPutGetDelete`; `blob.S3` compiled but tested only behind `AWT_S3_TEST=1`.
- [ ] **Step 6:** `make check` (without e2e/golden yet passing trivially) → PASS. Commit `feat(store): schema, migrations, rulesets, blob storage`.

---

### Task 3: Rounding and averaging helpers

**Files:** Create `internal/scoring/round.go`, `internal/scoring/average.go`, `internal/scoring/round_test.go`, `internal/scoring/average_test.go`

**Interfaces:**
- Produces: `Round3(x float64) float64`; `Average(xs []float64) float64` (Python `statistics.mean`: exact rational sum via `math/big.Rat`, divided by n, converted once with `Rat.Float64()`; empty → 0); `WeightedAverage(pairs []Weighted) float64` with `type Weighted struct{ Value, Weight float64 }` (naive left-to-right `top += v*w; bottom += w`, zero/negative weights NOT skipped, `bottom <= 0` → 0, spec 03 §4).

- [ ] **Step 1:** Tests: `Round3(0.0625)==0.062`, `Round3(2.0005)==2.001`, `Round3(1.0005)==1.0`, `Round3(7.8933333)==7.893`, `Round3(-0.0004)==-0.0`; `Average([]float64{0.1,0.2,0.3})==0.2` (naive sum/3 gives 0.20000000000000004), `Average(nil)==0`; `WeightedAverage({{7,100},{8,20}})==7.166666666666667`, `WeightedAverage({{5,0}})==0`.
- [ ] **Step 2:** Run → FAIL. **Step 3:** Implement (`Round3` = `strconv.ParseFloat(strconv.FormatFloat(x,'f',3,64),64)`). **Step 4:** Run → PASS. **Step 5:** Commit `feat(scoring): python-compatible rounding and averages`.

---

### Task 4: Trick catalog and unique-trick generation

**Files:** Create `internal/catalog/variants.go`, `internal/catalog/bonuses.go`, `internal/catalog/variants_test.go`, `testdata/catalog/2022.json` … `2025.json` (from `legacy/legacy/import/<year>/tricks/*.json`, concatenated), `testdata/catalog/2026.json` (from the export's `tricks.json`, including its stored `tricks` variants as the expected output)

**Interfaces:**
- Produces: `catalog.Trick` (fields of spec 02 §2.4.2), `catalog.Bonus{Name string; Bonus float64}`, `catalog.UniqueTrick` (spec 02 §2.4.3 minus `technical_mark(s)`), `catalog.Generate(t Trick) []UniqueTrick` (algorithm spec 02 §2.4.5, bonus table §2.4.4 as a Go table in `bonuses.go`: name, acronym, prefix bool, type); `catalog.LoadJSON(r io.Reader) ([]Trick, error)`.

- [ ] **Step 1:** `TestGenerateMatchesProductionCatalog`: for every trick in `testdata/catalog/2026.json`, `Generate(t)` equals the stored variants exactly (order, name, acronym, coefficient, bonus, bonus_types, uniqueness, solo, solo_awt, synchro, types).
- [ ] **Step 2:** `TestGenerateMistyFlip`: the 15 variants listed in spec 02 §2.4.5 example, in that order.
- [ ] **Step 3:** Run → FAIL; implement; run → PASS. Bonus sum uses left-to-right float addition. Commit `feat(catalog): unique trick generation`.

---

### Task 5: Scoring engine — types and steps 1–9

**Files:** Create `internal/scoring/types.go`, `internal/scoring/engine.go`, `internal/scoring/steps_marks.go`, `internal/scoring/steps_warnings.go`, `internal/scoring/engine_test.go`

**Interfaces:**
- Consumes: `Round3`, `Average`, `WeightedAverage` (Task 3); `catalog.UniqueTrick` (Task 4).
- Produces (JSON tags in snake_case, identical to golden files):

```go
type CompetitionType string // "solo" | "synchro"
type MarkType string        // "" | "awt" | "awq"
type JudgeLevel string      // "trainee" | "certified" | "senior"
type ResetPolicy int        // 0 none, 1 awt, 2 awq, 3 all

type Config struct { Warning, Warning2, MalusRepetition float64; WarningsToDSQ int
  JudgeWeights map[JudgeLevel]float64; MarkPercentages map[CompetitionType]map[string]float64
  MaxBonusPerRun map[string]float64 }
type Rules struct { MaxTricksPerType map[string]int; TipTouchPctPerTouch, TipTouchPctMax, DefaultMaxBonus float64 }
type PositionRule struct { First, NoFirst, Last, NoLast int }
type FlownTrick struct { catalog.UniqueTrick; TechnicalMark *float64 `json:"technical_mark"`; TechnicalMarks map[string]float64 `json:"technical_marks"` }
type JudgeMark struct { Judge string; Technical, Choreography, Landing, Synchro *float64; TechnicalPerTrick []*float64 }
type Flight struct { DidNotStart bool; Tricks []FlownTrick; Marks []JudgeMark; Warnings1, Warnings2 []string; TipTouches int }
type PreviousFlight struct { RunIndex int; Tricks []FlownTrick; Warnings1, Warnings2 int }
type Input struct {
  Type CompetitionType; Config Config; Rules Rules; RunIndex int; MarkType MarkType
  IsAWTPilot, HasAWTSeason, HasAWQSeason bool
  RepeatableBaseTricks []string           // base trick names
  ResetPolicies []ResetPolicy             // runs 0..RunIndex
  Previous []PreviousFlight               // same pilot/team, runs < RunIndex, ascending
  JudgeLevels map[string]JudgeLevel
  PositionRules map[string]PositionRule   // by base trick name
  LegacyLastManeuver bool                 // true only in golden tests (decision 0006 #6)
  Flight Flight }
type FinalMark struct { JudgesMark JudgeMark; Technicity, BonusPercentage, Technical, Choreography, Landing, Synchro, Bonus, Score, Malus float64
  Warnings1, Warnings2, Notes []string; MarkType MarkType }
type Result struct { Final FinalMark; Tricks []FlownTrick; Marks []JudgeMark } // tricks/marks after steps 4–5 mutations
type ValidationError struct{ Msg string }
func Score(in Input) (Result, error)
```

- [ ] **Step 1:** Write tests for steps 1–9 using spec 03 §12 Examples A, B2 (DSQ), B3 (unbalanced directions), C (AWT vs AWQ marking), G is out of scope (null quirk is now rejected upstream). Each test asserts every `FinalMark` field after `Round3` and the exact `Notes` strings from the spec.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement `Score` steps 1–9 in spec 03 §5.0 order, with `// step N:` comments. Error messages are the legacy texts quoted in §5.0 (e.g. `"not enough choreography marks"`). Unknown judge in `Marks` → `ValidationError{"unknown judge <id>"}`.
- [ ] **Step 4:** Run → PASS for the step 1–9 assertions (tests needing steps 10–16 marked `t.Skip("task 6")`). Commit `feat(scoring): engine steps 1-9`.

---

### Task 6: Scoring engine — steps 10–16

**Files:** Create `internal/scoring/steps_tricks.go`, `internal/scoring/steps_final.go`; Modify `internal/scoring/engine_test.go`

- [ ] **Step 1:** Remove the skips; add Examples B (repetitions + cumulative warning), D (limits, in-run repetition), E (synchro + tip touch), F (AWT-forbidden trick, reset policy) from spec 03 §12, plus `TestTechnicityPadding` (table of spec 03 §5.1) and `TestLastManeuverFixed` (decision 0006: with `LegacyLastManeuver=false` a `last_maneuver=1` trick placed first warns, placed last does not; with `true` the legacy inverted condition applies).
- [ ] **Step 2:** Run → FAIL. **Step 3:** Implement steps 10–16. Bonus sums are left-to-right float additions; `Technicity` uses `Average` of the top 3 padded coefficients. **Step 4:** Run → PASS.
- [ ] **Step 5:** Add `FuzzScore` (random valid inputs: score ≥ 0, technicity ≥ 1, no panic). Run `go test -fuzz=FuzzScore -fuzztime=30s ./internal/scoring/` → no failure. Commit `feat(scoring): engine steps 10-16`.

---

### Task 7: Golden corpus generator and golden test

**Files:** Create `tools/golden/generate.py`, `tools/golden/README.md`, `testdata/golden/flights/*.json`, `testdata/golden/rankings/*.json`, `internal/scoring/golden_test.go`, `internal/ranking/golden_types.go`; Modify `Makefile` (`golden`, `golden-regen`)

**Interfaces:**
- Produces: flight case file format `{"competition":"<id>","cases":[{"run":i,"participant":"<civlid|team id>","input":<scoring.Input JSON>,"expected":<FinalMark JSON, full precision>,"expected_tricks":[...],"expected_marks":[...],"error":"<legacy HTTPException detail or empty>"}]}`; ranking case format `{"scope":"competition","code":"<code>","input":{...ranking input...},"expected":{...legacy CompetitionResults.export() reduced to ids and scores...}}` and the same for seasons (`Season.export_public`).

- [ ] **Step 1:** `generate.py` (run with the legacy venv: `make golden-regen LEGACY_VENV=<path> BACKUP=<tar.gz>`): extract the export to a temp dir; monkeypatch `Trick.get`/`get_unique_trick`/`getall`, `Judge.get`, `Pilot.get/getall`, `Team.get`, `Season` lookups and `Competition.save` to read from the export (pattern: `scratchpad` harness described in spec 03 header); for every non-deleted competition, every run `i`, every flight: load the competition with `Competition.model_validate`, truncate `runs` to `i+1` (decision 0004), call `calculate_score(flight, i, is_awt_pilot, mark_type=flight.final_marks.mark_type)`, and write `input` built from the same objects (position rules from the current catalog, `LegacyLastManeuver: true`), anonymised per Global Constraints.
- [ ] **Step 2:** Also emit ranking cases: for each competition, `results()` and `export()` (ids + scores + per-run rank/score + `final`); for each season, `export_public` (ids + scores, overall and women).
- [ ] **Step 3:** Run the generator; commit the corpus. Print a summary: number of cases, number of legacy errors, and how many recomputed scores differ from the stored `final_marks.score` after `Round3` (informational, goes in `tools/golden/README.md`).
- [ ] **Step 4:** `TestGoldenFlights`: for each case, `scoring.Score(input)`: if `error` non-empty, expect a `ValidationError` with that message; else every float field equal after `Round3` and `|go-py| ≤ 1e-9` raw, `Notes`, `Warnings1/2`, filled marks and per-trick technical marks equal. Failures list competition, run, participant and the first differing field.
- [ ] **Step 5:** Run `make golden` → PASS on all 2310 cases (fix the engine, never the corpus). Commit `test(scoring): golden corpus from production data`.

---

### Task 8: Ranking engine

**Files:** Create `internal/ranking/{types.go,run.go,competition.go,season.go,startorder.go,ranking_test.go,golden_test.go}`

**Interfaces:**
- Consumes: `scoring.Round3`.
- Produces:

```go
type Participant struct { CivlID int; TeamID string; Name, Country string; Gender string; IsAWT bool }
type FlightScore struct { Participant Participant; Score float64; Published bool; SaveSeq int }
type RunInput struct { Type string; Seasons []string; Year int; State string; Flights []FlightScore; Participants []Participant }
type Entry struct { Participant Participant; Score float64; Position, Rank int; PerRun []RunCell }
type RunCell struct { RunIndex, Rank int; Score float64 }
type Lists map[string][]Entry // "overall", season codes, "women"
type RunResult struct { Lists Lists; Final bool }
type CompetitionResult struct { Lists Lists; Runs []RunResult; Final bool }
func Run(in RunInput, publishedOnly bool) RunResult
func Competition(state string, runs []RunInput, limit int) CompetitionResult
type SeasonInput struct { Code, Country string; Type string; Competitions []SeasonComp }
type SeasonComp struct { Code string; Published, Closed bool; Result CompetitionResult }
func Season(in SeasonInput) Lists                        // "overall" and "women" (≥3 women)
func NextStartOrder(comp CompetitionResult, prev []Participant, seasons []string, qualify int) []Participant
```

- [ ] **Step 1:** Unit tests from spec 03 §7, §7.1 (sub-rankings by awt/awq/country code, women ≥ 3), §8 (sum with `Round3` at each addition, unpublished counted per decision 0005, `final` flag), §8.1/§8.2 (start order: leader last, no-result last, mixed AWQ-then-AWT, qualify keeps best N per decision 0006), §10 (season: only published+closed+final comps, country filter, unrounded sum, women). Ordering and shared ranks per decision 0007: `TestTiesShareRank` (scores 10, 9.5, 9.5, 8 → ranks 1, 2, 2, 4, order by name).
- [ ] **Step 2:** `TestGoldenRankings`: compare scores per participant and membership of each list with the ranking corpus; positions compared as multisets of (rank group, ids) to tolerate the tie rule change.
- [ ] **Step 3:** FAIL → implement → PASS. Commit `feat(ranking): run, competition, season rankings and start order`.

---

### Task 9: Domain model, state machines, validation

**Files:** Create `internal/domain/{competition.go,run.go,flight.go,season.go,slug.go,validate.go,domain_test.go}`

**Interfaces:**
- Produces: `domain.Slug(name string) string` (spec 02 §2.5.2 algorithm); `domain.CompetitionState`, `RunState`; `domain.CanTransition(from, to CompetitionState) error`, `domain.RunTransition(from, to RunState) error`; `domain.ValidateCompetition(c Competition) []FieldError`; `domain.ValidateMark(raw string) (*float64, error)` (accepts `7.5` and `7,5`, rejects outside 0–10, blank → nil); `domain.CloseRunCheck(participants, flights) error` (every participant has a published flight, error lists names); `FieldError{Field, Msg string}`.

- [ ] **Step 1:** Tests: `TestSlug` (`"Coupe Icare 2025 – Acro Élite!"` → `coupe-icare-2025--acro-elite`, `"Şahin Cup"` → `sahin-cup`), every transition of spec 02 §5.1/§5.2 tables (allowed and refused), open requires ≥ 2 pilots/teams and ≥ 2 judges, `start_date ≤ end_date`, season code `^[a-z]{3}-[0-9]{4}$` forced to `{country}-{year}` when country set, `ValidateMark("7,5")==7.5`, `ValidateMark("75")` error `"mark must be between 0 and 10"`, `ValidateMark("")==nil`.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(domain): entities, state machines, validation`.

---

### Task 10: Importer — reference data

**Files:** Create `internal/importer/{importer.go,mongo.go,reference.go,reference_test.go}`, `internal/importer/testdata/mini/*.json` (hand-written 2-pilot / 1-team / 2-judge / 2-trick / 1-season / 1-file mini export, fictional names); Modify `cmd/awt/main.go` (`import --from <tar.gz|dir>`)

**Interfaces:**
- Produces: `importer.Run(ctx, db *store.DB, blobs blob.Store, src string) (Report, error)`; `Report{Counts map[string]int; Warnings []string}`.

- [ ] **Step 1:** `TestImportReference` on the mini export: pilots with awt years and gender, teams with ordered pilots, judges, seasons (legacy duplicate `aws-2024` codes: keep the non-deleted one, report the other), files decoded from base64 into the blob store with sha256 id verified, tricks imported for every year from `testdata/catalog/<year>.json` and the export's catalog as 2026, variants via `catalog.Generate`.
- [ ] **Step 2:** FAIL → implement (reads mongoexport JSON lines; refuses a non-empty database unless `--replace`) → PASS. Commit `feat(importer): reference data`.

---

### Task 11: Importer — competitions, flights, frozen results

**Files:** Create `internal/importer/{competitions.go,competitions_test.go}`; Modify `internal/importer/testdata/mini/competitions.json`

- [ ] **Step 1:** `TestImportCompetitions` (mini export with one solo and one synchro competition): runs, participants in stored order, flights with tricks snapshot, marks (null `technical_per_trick` entries preserved), warnings cat 1/2, `final_mark` JSON copied verbatim, `score = Round3(final_marks.score)`, `frozen = 1`; rankings rebuilt with `ranking.*` and stored in `ranking_entries`; `results_final` set.
- [ ] **Step 2:** FAIL → implement → PASS.
- [ ] **Step 3:** `cmd/awt golden-verify --db <path>`: recompute every imported competition's rankings and compare with `testdata/golden/rankings` (same tolerance as Task 8). Run against the real export locally: `go run ./cmd/awt import --from mongo.backup.20261018-2128.tar.gz --db /tmp/awt-verify.db && go run ./cmd/awt golden-verify --db /tmp/awt-verify.db` → `48 competitions, 25 seasons OK`. Commit `feat(importer): competitions and frozen results`.

---

### Task 12: Service — reference data and competition lifecycle

**Files:** Create `internal/service/{service.go,events.go,pilots.go,teams.go,judges.go,tricks.go,seasons.go,competitions.go,runs.go,service_test.go}`

**Interfaces:**
- Produces: `service.New(db *store.DB, blobs blob.Store, bus *Bus, civl CIVLClient) *Service`; `Bus.Subscribe(topic string) (<-chan Event, func())`, `Bus.Publish(Event)`, `Event{Topic string ("competition:<code>", "season:<code>"); Kind string ("flight","run","competition"); At time.Time}`; methods: `CreateCompetition(ctx, CompetitionInput) (Competition, error)`, `UpdateCompetition`, `SetParticipants`, `SetJudges`, `SetRepeatableTricks`, `SetConfig`, `OpenCompetition`, `CloseCompetition`, `ReopenCompetition`, `DeleteCompetition`, `NewRun(ctx, compID string, qualify int, policy scoring.ResetPolicy) (Run, error)`, `SetRunOrder`, `OpenRun`, `CloseRun`, `ReopenRun`; reference CRUD for pilots (incl. `ImportPilotFromCIVL`, `ToggleGender`, `SetAWTYear`), teams, judges, tricks (per year, `ImportCatalog(year, json)`), seasons, files (`UploadFile` with type allowlist `image/png,image/jpeg,image/webp,application/pdf` and 10 MB cap).

- [ ] **Step 1:** Tests on a temp DB: full lifecycle init → open → new run (start order from CIVL rank desc for run 1) → open run → close run refused while a participant has no published flight → close comp refused while a run is open; frozen competitions refuse every mutation with `ErrFrozen`; config of a new competition copied from the ruleset of `start_date.year`; `NewRun` start order via `ranking.NextStartOrder`, previous run order unchanged (decision 0006 #2).
- [ ] **Step 2:** FAIL → implement (each method one `DB.Tx`) → PASS. Commit `feat(service): reference data and competition lifecycle`.

---

### Task 13: Service — flight scoring, rescoring, ranking rebuild

**Files:** Create `internal/service/{flights.go,scoring_input.go,rankings.go,flights_test.go}`

**Interfaces:**
- Consumes: `scoring.Score`, `ranking.*`, `Bus`.
- Produces: `FlightInput{Tricks []string (acronyms or names); Marks []MarkInput{Judge string; Technical, Choreography, Landing, Synchro *float64; TechnicalPerTrick []*float64}; DidNotStart bool; Warnings1, Warnings2 []string; TipTouches int}`; `Service.SimulateFlight(ctx, compID string, run int, participant string, in FlightInput) (scoring.Result, error)` (no write); `Service.SaveFlight(ctx, compID string, run int, participant string, in FlightInput, publish bool) (scoring.Result, error)`; `Service.MarkTypeFor(comp, participant) scoring.MarkType` (solo: `awt` if pilot is AWT for the year and the comp has an `awt-*` season, `awq` if the comp has an `awq-*` season, else ""; synchro: ""); `Service.RebuildRankings(ctx, q, compID) error`.

- [ ] **Step 1:** Tests: `TestSaveFlightScoresAndRanks` (two pilots, scores match a hand-checked spec 03 §8.2 run-1 value 7.893); `TestSaveRefusedWhenRunNotOpen`; `TestUnknownTrickError` (`"Unknown trick(s): XYZ"`); `TestResaveEarlierRunRescoresLaterRuns` (pilot repeats `LM` in run 2 → malus 13; re-save run 1 without `LM` → run 2 malus 0 and competition total updated); `TestConcurrentFlightSaves` (20 goroutines saving different pilots of one run → 20 flights, ranking has 20 entries, no `SQLITE_BUSY` leak); `TestSavePublishesEvent`.
- [ ] **Step 2:** FAIL → implement: `scoring_input.go` builds `scoring.Input` from DB rows (previous flights of same participant in runs `< run`, position rules from the competition-year catalog, repeatable base tricks from `competition_repeatable_tricks`); `SaveFlight` = one `BEGIN IMMEDIATE` tx: score, upsert flight (+ tricks/marks/warnings, `save_seq` = max+1 on insert), rescore this participant's flights in runs `> run` (decision 0006 #14), `RebuildRankings` for the competition and each of its seasons, commit, then publish → PASS.
- [ ] **Step 3:** Commit `feat(service): flight scoring with rescoring and stored rankings`.

---

### Task 14: Authentication and authorisation

**Files:** Create `internal/auth/{users.go,sessions.go,csrf.go,middleware.go,ratelimit.go,auth_test.go}`; Modify `cmd/awt/main.go` (`user add --email --name --role`, `user passwd`, `user assign --email --competition`)

**Interfaces:**
- Produces: `auth.Login(ctx, email, password) (Session, error)` (bcrypt cost 12); cookie `awt_session` (HttpOnly, Secure unless dev, SameSite=Lax, 14 days, rotated on login); `auth.RequireUser`, `auth.RequireAdmin`, `auth.RequireScorer(compIDFrom func(*http.Request) string)` middleware; `auth.CSRF` middleware (token in session, checked on every non-GET via form field `_csrf` or header `X-CSRF-Token`, htmx sends the header from a `<meta>`); `auth.LoginLimiter` (5 failures per email+IP per 15 min → 429).

- [ ] **Step 1:** Tests: login success/failure, disabled user refused, session expiry, CSRF missing → 403, limiter → 429 on 6th failure, scorer of competition A refused on competition B → 403.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(auth): accounts, sessions, csrf, roles`.

---

### Task 15: Web foundation

**Files:** Create `internal/web/{routes.go,middleware.go,cache.go,errors.go,routes_test.go}`, `internal/web/layout/{base.templ,public.templ,admin.templ}`, `web/static/css/app.css`, `web/static/vendor/htmx-2.0.4.min.js`, `web/static/vendor/htmx-ext-sse-2.2.2.js`, `web/static/img/*` (logo and favicons from `legacy/legacy/www/assets` and `results/public`)

**Interfaces:**
- Produces: `web.Routes(s *service.Service, a *auth.Auth, cfg config.Config) http.Handler`; `cache.Public(maxAge, swr time.Duration)` middleware setting `Cache-Control: public, max-age=..., stale-while-revalidate=...`; `cache.Purger` interface `{Purge(ctx, urls []string) error}` with Cloudflare and no-op implementations, subscribed to the `Bus` (purges competition, season, pilot and home URLs after each event); request logging with `slog`, panic recovery → 500 page; `TestAdminRoutesAreGuarded` walks the route table and fails if any `/admin/` route lacks `RequireUser`.
- Visual identity: colours, Exo font (self-hosted from `legacy/legacy/www/assets/fonts`) and logo from spec 07 §5; layout mobile-first; public nav = Home, Events, News, Results (Seasons, Competitions), Pilots, Teams, Judges, Tricks, Simulator.

- [ ] **Step 1:** Tests: guard test; static files served with long cache and content hash query; 404 page; redirect table applied (`redirects` rows → 301/302).
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(web): routing, layouts, static assets, caching`.

---

### Task 16: Public results pages

**Files:** Create `internal/web/public/{home.go,seasons.go,competitions.go,pilots.go,teams.go,judges.go,tricks.go,*.templ,public_test.go}`

URLs (keep legacy results URLs): `/results` (home of results: current competitions = state open, latest closed), `/seasons`, `/seasons/{code}`, `/competitions`, `/competitions/{code}` (tabs: overall per list key, each run per list key, details, pilots, judges), `/pilots`, `/pilots/{civlid}/{slug}`, `/teams`, `/teams/{id}/{slug}`, `/judges`, `/tricks?year=`. Every displayed field per spec 06 §4.1–4.10 (with `final`/provisional badge, flight detail: tricks, judges' marks, notes, malus, bonus, technicity). Data read only from stored rows and `ranking_entries`.

- [ ] **Step 1:** Tests with a seeded DB (fixture builder `public_test.go`): each page 200 with expected text (competition winner name, provisional badge when `results_final=0`, `<title>` and `og:title`), `TestPublicPagesEmptyStates`, unpublished competition → 404, HTML escaping of names.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(public): results pages`.

---

### Task 17: Live updates and overlay

**Files:** Create `internal/web/sse.go`, `internal/web/public/live.go`, `internal/export/svg.go`, `internal/web/sse_test.go`

**Interfaces:**
- Produces: `GET /live/competitions/{code}` SSE stream (`event: update` with `data: <kind>`, `: ping` every 30 s, `X-Accel-Buffering: no`); competition page uses `hx-ext="sse" sse-connect=... ` and `hx-trigger="sse:update"` to reload the ranking fragment `GET /competitions/{code}/fragment/{list}`; `GET /live/overlay/competitions/{code}/runs/{i}.svg` and `/live/overlay/competitions/{code}/current.svg` (legacy overlay: pilot name, run, rank, score; all text XML-escaped).

- [ ] **Step 1:** Tests: SSE receives an event within 1 s after `Bus.Publish`; ping sent (use 50 ms interval in test); overlay escapes `<`/`&`.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(live): SSE updates and streaming overlay`.

---

### Task 18: Public simulator

**Files:** Create `internal/web/public/simulator.go`, `simulator.templ`, `simulator_test.go`, `web/static/js/trick-input.js`

Behaviour from spec 03 §13 (public variant) and spec 06 §6: up to 5 runs, tricks typed by acronym with autocomplete from the current-year catalog, marks per run, optional repetition reset every k runs; computed in memory with `scoring.Score` (default ruleset of current year, senior "simulator" judge); result shows per-run final marks and the total.

- [ ] **Step 1:** Test: two runs repeating `LM` → run 2 malus 13 and note text; reset every 1 run → malus 0.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(public): simulator`.

---

### Task 19: Admin — reference data screens

**Files:** Create `internal/web/admin/{dashboard.go,pilots.go,teams.go,judges.go,tricks.go,seasons.go,files.go,users.go,*.templ,admin_ref_test.go}`; Create `internal/civl/{client.go,client_test.go,testdata/*.html}`

**Interfaces:**
- Produces: `civl.Client.FetchPilot(ctx, civlid int) (PilotData, error)`, `FetchRankings(ctx) (map[int]int, error)` (parse with `golang.org/x/net/html`; fixtures saved from the legacy parsing logic in `api/controllers/pilots.py:35-286`, tested offline only).
- Screens (spec 05 §5.1–5.8): pilots list + import by CIVL id + refresh rankings + gender toggle + AWT year toggle; teams CRUD (2 pilots); judges CRUD (level, country, civlid); tricks per year (list, edit, import JSON file, duplicate year); seasons CRUD; files upload; users (admin only).

- [ ] **Step 1:** Handler tests (httptest with an admin session): create/edit/delete each entity, validation errors re-render the form with messages, CIVL parsing fixtures.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(admin): reference data management`.

---

### Task 20: Admin — competitions and runs management

**Files:** Create `internal/web/admin/{competitions.go,runs.go,*.templ,admin_comp_test.go}`

Screens (spec 05 §5.9–5.11 minus scoring): competition list by year (published and unpublished), create/edit form, participants picker (pilots or teams), judges, repeatable tricks, config (every field of spec 02 §2.6, including `warning2`), state buttons with in-page confirmation, runs list, new run (qualify N, reset policy), run starting order (move up/down buttons, htmx), run judges, run open/close/reopen, starting order print view link.

- [ ] **Step 1:** Tests: full setup flow through HTTP handlers; open refused with < 2 judges shows the message; frozen competition shows read-only pages.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(admin): competitions and runs`.

---

### Task 21: Admin — judging screen

**Files:** Create `internal/web/admin/{judging.go,judging.templ,judging_test.go}`, `web/static/js/judging.js`

One screen per run: participant list in starting order with status (not flown / saved / published); selecting a participant opens the flight form: trick inputs (acronym with autocomplete, add/remove/reorder), marks grid (one row per run judge; columns technical or per-trick technical depending on `MarkTypeFor`, choreography, landing, synchro for synchro), warnings cat 1 / cat 2 lists (add/remove by index), DNS, tip touches (synchro). Every change triggers `POST .../simulate` (htmx, `hx-trigger="input changed delay:300ms"`, `hx-sync="this:replace"` so out-of-order responses are dropped) rendering the provisional `FinalMark` (components, notes, warnings). Buttons: Save (unpublished), Save & publish. Marks parsed with `domain.ValidateMark`.

- [ ] **Step 1:** Tests: `TestMarkInputParsing` (`7,5` accepted, `75` and `abc` re-render with field error, blank = no mark); `TestJudgingSaveAndPublish`; `TestPerTrickMarksStayAligned` (removing trick 2 of 3 removes column 2 for every judge); `TestRemoveWarningByIndex`; `TestSynchroDNSAndTipTouchesSaved`.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(admin): live judging screen`.

---

### Task 22: Exports

**Files:** Create `internal/export/{xlsx.go,html.go,export_test.go}`, `internal/export/templates/*.templ`; Modify admin routes

**Interfaces:**
- Produces: `export.CompetitionXLSX(w io.Writer, data CompetitionExport) error` and `RunXLSX` (CIVL format, spec 03 §11 and 04 §7.2: columns, 100/99/… points, AWQ restart at 80, name split on first space, gender M/F, synchro one row per pilot; run export refused unless final); `export.ResultsHTML`, `RunResultsHTML`, `StartingOrderHTML` (print layout, spec 04 §7.1, optional `after run N`).
- Routes (admin and scorer): `/admin/competitions/{code}/export/results.xlsx|.html?limit_run=`, `/admin/competitions/{code}/runs/{i}/export/results.xlsx|.html`, `/admin/competitions/{code}/runs/{i}/starting-order.html`.

- [ ] **Step 1:** Tests: xlsx cell values for a mixed AWT/AWQ fixture (points 100, 99, then 80 for first AWQ); `TestExportsUnicodeAndEscaping`; run export of a non-final run → 400 message.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(export): CIVL xlsx and printable HTML`.

---

### Task 23: Institutional site (www) and content admin

**Files:** Create `internal/web/public/{www.go,news.go,events.go,pages.go,sitemap.go,*.templ,www_test.go}`, `internal/web/admin/content.go`, `content.templ`, `internal/importer/www.go`, `internal/importer/www_test.go`

- Public URLs (spec 07 §2, §10.2): `/` (hero, upcoming events, latest 3 news, featured season standings top 5 from `site_settings.featured_seasons`), `/events` (by season, upcoming/past, circuit filter), `/event/{slug}`, `/news`, `/news/{slug}`, `/pilots` keeps the results pilots page, `/documents`, `/{page-slug}`, `/sitemap.xml`, `/robots.txt`; legacy `.html` URLs redirected (301) via seeded `redirects`; archived 2022–2023 result pages from `legacy/legacy/www/result/` served as static files under `/result/{year}/{slug}`.
- Admin: CRUD for news, events (with optional competition link), documents (upload PDF), pages, redirects, site settings; Markdown with preview.
- `importer.WWW(ctx, legacyDir)`: parse `legacy/legacy/www/news/*.html`, `event/*.html` with `x/net/html` into news/events (title, date when present, body converted to Markdown paragraphs and images uploaded to blob store), sporting code PDFs into documents, seed redirects for every legacy path.

- [ ] **Step 1:** Tests: WWW importer on two fixture HTML files; pages render with OG tags and JSON-LD; sitemap lists news, events, competitions, pilots; redirect `/news/acromax.html` → 301 `/news/acromax`.
- [ ] **Step 2:** FAIL → implement → PASS. Commit `feat(www): institutional site and content admin`.

---

### Task 24: Read-only JSON API

**Files:** Create `internal/web/api/{api.go,api_test.go}`

`GET /api/v1/competitions`, `/competitions/{code}` (with lists and runs), `/competitions/{code}/runs/{i}`, `/seasons`, `/seasons/{code}`, `/pilots`, `/pilots/{civlid}`, `/teams`, `/judges`, `/tricks?year=`; JSON from stored rows only, same cache headers as public pages, CORS `*` without credentials.

- [ ] **Step 1:** Tests: shapes and 404s; unpublished competition hidden. **Step 2:** FAIL → implement → PASS. Commit `feat(api): public read API`.

---

### Task 25: End-to-end tests

**Files:** Create `e2e/{main_test.go,judging_test.go,public_test.go}`; Modify `Makefile` (`e2e` installs browsers via `go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium` once)

- [ ] **Step 1:** `TestMain` builds and starts the binary on a temp DB with an admin user and the mini reference data.
- [ ] **Step 2:** `TestJudgingWorkflow`: log in, create a solo competition with 2 pilots and 2 judges, open it, create and open run 1, score both pilots (live provisional score appears before saving), publish, close run, close competition; public competition page shows the winner and a final badge; a second browser on the public page receives the SSE update without reload.
- [ ] **Step 3:** `TestSynchroScoring` (DNS and tip touches persist); `TestPublicNavigation` (every nav link 200).
- [ ] **Step 4:** `make e2e` → PASS. Commit `test(e2e): judging and public workflows`.

---

### Task 26: Packaging, deployment, operations

**Files:** Create `Dockerfile` (multi-stage, `CGO_ENABLED=0`, distroless static, Litestream binary copied from its official image, entrypoint `litestream restore -if-db-not-exists -if-replica-exists $DB && exec litestream replicate -exec "awt serve"`), `litestream.yml`, `compose.yaml` (caddy, awt-prod, awt-training, volumes), `Caddyfile` (two hosts, `flush_interval -1` for `/live/*`, Cloudflare-only origin), `.github/workflows/deploy.yml` (on main after CI: build, push to GHCR, ssh `docker compose pull && up -d awt-training`, smoke test `/healthz`, then same for prod; rollback to previous tag on failed health check), `docs/ops.md` (provision VPS, firewall, secrets in `.env`, restore procedure, weekly restore test cron, first import procedure); Modify `Makefile` (`docker`, `backup-restore-test`)

- [ ] **Step 1:** `make docker` builds; `docker run` with an empty volume and no replica starts and serves `/healthz`.
- [ ] **Step 2:** Local restore test with MinIO in compose profile `test`: write data, stop, delete volume, start → data restored (`make backup-restore-test` → PASS).
- [ ] **Step 3:** Commit `chore(ops): docker, compose, litestream, deploy pipeline`.

---

### Task 27: Final verification against production data

- [ ] **Step 1:** Fresh DB: `awt migrate`, `awt import --from mongo.backup.20261018-2128.tar.gz`, `awt import-www --from legacy/legacy/www`, `awt golden-verify` → all OK.
- [ ] **Step 2:** `make check` → PASS (lint, unit, golden, e2e).
- [ ] **Step 3:** Start the server on the imported DB; for 5 competitions (2 solo AWT/AWQ mixed, 1 synchro, 1 national, 1 with women ranking) and 3 seasons, compare displayed totals with the legacy public API values recorded in the golden ranking corpus; record results in `docs/verification-2026-10.md`.
- [ ] **Step 4:** Update `specifications/` where implementation decisions refined behaviour; commit `docs: verification report`.
