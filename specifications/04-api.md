# 04 — Backend API surface & cross-cutting concerns

> Reverse-engineered from the legacy repo (`legacy/legacy`, last commit `979862e`).
> All paths are relative to that repo root, e.g. `api/routers/public.py:120`.
> Domain model and scoring formulas are covered in another spec. This document covers the **HTTP surface**
> and the **cross-cutting backend concerns** (config, auth, caching, files, exports, live, tests, risks).

---

## 1. Overview

### 1.1 Stack

| Item | Value | Source |
|---|---|---|
| Language | Python 3.10 (`python:3.10-alpine` image) | `api/Dockerfile:1`, `api/Dockerfile.fly.io:2` |
| Web framework | FastAPI + Starlette (ASGI), Pydantic v2 + `pydantic-settings` | `api/requirements.txt` |
| Server (prod) | `gunicorn main:app --workers 1 --worker-class uvicorn.workers.UvicornWorker -b [::]:8080 --forwarded-allow-ips * --access-logfile -` | `api/Dockerfile.fly.io:12` |
| Server (dev) | `uvicorn main:app --host 0.0.0.0 --reload` on port 8000. The venv is created and `pip install` runs **on every container start** | `api/Dockerfile:8` |
| DB driver | `motor` (async MongoDB), `pymongo` for errors and index constants | `api/core/database.py` |
| Templates | Jinja2 (`fastapi.templating.Jinja2Templates`, directory `templates`) | `api/routers/competitions.py:28` |
| Excel | `openpyxl` (reads CIVL ranking exports, writes results `.xlsx`) | `api/controllers/pilots.py`, `api/controllers/competitions.py` |
| Scraping | `httpx` + `lxml`/`cssselect` against civlcomps.org | `api/controllers/pilots.py` |
| Other libs | `pycountry` (ISO-3 country validation), `python-jose[cryptography]` (JWT), `python-multipart` (uploads), `redis` (**imported, never used**) | `api/requirements.txt` |
| Versions | **Nothing is pinned**: `requirements.txt` lists bare package names, so every build installs the latest versions. A rewrite must pin versions. | `api/requirements.txt` |
| App version | `settings.VERSION = "2025.4"`, exposed in OpenAPI, `/` and `/status/` | `api/core/config.py:11` |
| License / contact | WTFPL 2, author contact in OpenAPI metadata | `api/main.py:37-51` |

### 1.2 Deployment topology

| Component | Details | Source |
|---|---|---|
| API on fly.io | app `api-acroworldtour-com`, region `ams`, internal port 8080, HTTP→HTTPS forced, concurrency soft 20 / hard 25 connections, TCP health check every 15 s | `api/fly.toml` |
| API on render.com | second deployment target, triggered by the same GitHub workflow | `.github/workflows/main.yml` |
| Caching reverse proxy | nginx on fly.io (`proxy-acroworldtour-com`) with a 1 GB disk cache on volume `/cache`. It **only exposes `/public/**`**; every other path returns 404 | `proxy/nginx.conf`, `proxy/fly.toml` |
| CI/CD | push to `main` touching `api/**` → `flyctl deploy` and render deploy. Push to `main` also force-pushes `main:training` (training environment) | `.github/workflows/*.yml` |
| Local dev | `docker-compose.yml`: api (8000), manager (3001, `NEXT_PUBLIC_API_URL=http://127.0.0.1:8000`), results (3000, `NEXT_PUBLIC_API_URL=…/public`), www nginx (3002), mongo 5.0, redis (unused) | `docker-compose.yml` |
| Makefile | `make test` runs pytest inside the compose `api` container; `deploy-*` delegate to the parent Makefile, which is not in the repo | `api/Makefile` |

The manager (back-office, Next.js) talks to the API root with a Bearer token. The results site (Next.js, results.acroworldtour.com) talks to **`https://api.acroworldtour.com/public`** (`results/constants.ts`), which goes through the nginx cache.

### 1.3 Startup sequence (`api/main.py`)

1. `import core.logging` → `logging.basicConfig(level=LOG_LEVEL)` (`api/core/logging.py:5`).
2. `core.database` creates one global `AsyncIOMotorClient(MONGODB_URL, timeoutMS=1000)` and `db = client[DATABASE]` at import time (`api/core/database.py:8-9`).
3. `core.security` picks the auth dependency at import time: JWT if both `ADMIN_USER` and `ADMIN_PASS` are set, otherwise **no auth at all** (`api/core/security.py:94-99`).
4. Lifespan (`api/main.py:23-35`):
   - if the string `"test"` appears in `DATABASE` → **drops every collection** (`clean_database`, `api/core/database.py:39-42`);
   - `JudgeCtrl/PilotCtrl/TeamCtrl/TrickCtrl/CompCtrl/SeasonCtrl.start()` → `createIndexes()` on each collection. These calls are not awaited: motor returns futures that are never awaited, so the index builds run fire-and-forget. `EventCtrl.start()` is **not** called.
5. Middlewares (`api/main.py:53-81`):
   - `validate_ip`: no-op. The IP deny-list is commented out; git history shows an IP was blocked and then unblocked.
   - `CORSMiddleware`: see §2.5.
   - `add_process_time_header`: adds `X-Process-Time: <seconds float>` to every response.
6. A single root router (`api/routers/api.py`) is mounted with no global prefix.

FastAPI's default `/docs`, `/redoc` and `/openapi.json` are enabled and public. The import README tells operators to "take the bearer from https://api.acroworldtour.com/docs/" (`import/README.txt`).

### 1.4 Configuration (`api/core/config.py`, `pydantic-settings`, `case_sensitive=True`)

Settings come from environment variables only (no `.env` file is loaded).

| Env var | Type | Default | Meaning / usage |
|---|---|---|---|
| `PROJECT_NAME` | str | `"acroworldtour.com API"` | OpenAPI title, `/`, `/status/` |
| `PROJECT_DESCRIPTION` | str | "API to manage and access Paragliding Aerobatics competitions results" | OpenAPI description |
| `VERSION` | str | `"2025.4"` | OpenAPI version, `/status/` |
| `JWT_SECRET` | str | an insecure placeholder literal (test value, **must be overridden**) | HMAC key for JWT signing |
| `JWT_ALGORITHM` | str | `HS256` | JWT algorithm |
| `ACCESS_TOKEN_EXPIRE_MINUTES` | int | `11520` (8 days) | token lifetime |
| `SERVER_NAME` | str | = PROJECT_NAME | unused |
| `MONGODB_URL` | str | `mongodb://127.0.0.1/` | Mongo connection string (prod: managed MongoDB, `mongodb+srv` with TLS) |
| `DATABASE` | str | `acropyx2-database-developtment` (sic) | database name. If it contains `test`, **all collections are dropped at startup** |
| `TESTDB` | Optional[str] | None | unused |
| `ADMIN_USER` | str | `admin` | the single admin login |
| `ADMIN_PASS` | Optional[str] | None | admin password. **If unset, authentication is disabled** (fail-open) |
| `LOG_LEVEL` | str | `INFO` | root logging level (`DEBUG` in compose) |
| `PRIMARY_REGION` | – | `ams` (fly.toml) | fly.io only, not read by the app |
| `CACHE_EXPIRES` | – | `0` in compose | **not read by any code** (left over from a removed Redis cache) |
| `_REDIS_URL` | – | compose only, prefixed `_` (disabled) | **not read by any code** |

Hard-coded, non-env "settings" are nested classes in `Settings`. They are business constants and the other spec details them:
- `settings.pilots`: CIVL URLs (`civl_link_all_pilots`, `civl_link_one_pilot`).
- `settings.tricks.available_bonuses` / `available_directions`: served by `/tricks/bonuses` and `/tricks/directions`.
- `settings.competitions.*`: warnings, malus, judge weights, mark percentages, bonus caps, `max_tricks_per_type`.
- `settings.runs.sample`: unused.

### 1.5 Database connection

- One global motor client per process. `timeoutMS=1000` (1 s client-side operation timeout) (`api/core/database.py:8`).
- Collections: `pilots`, `teams`, `judges`, `tricks`, `competitions` (runs and flights are **embedded**), `seasons`, `files`, `events` (unused).
- IDs: `PyObjectId` serialised as **string**. `jsonable_encoder` stores `_id` as a string, not a BSON ObjectId. Pilots use the int CIVL ID as `_id`. Files use the SHA-256 of their base64 content.
- Soft delete: `deleted: datetime|null` on every entity except pilots (hard delete).
- Indexes created at startup (fire-and-forget):

| Collection | Index | Source |
|---|---|---|
| pilots | `civlid` unique, `name` unique | `api/models/pilots.py:177-179` |
| judges | `(name, deleted)` unique | `api/models/judges.py:80-81` |
| teams | `(name, deleted)` unique | `api/models/teams.py:88-89` |
| tricks | `(name, deleted)` unique, `(acronym, deleted)` unique | `api/models/tricks.py:175-177` |
| competitions | `(name, deleted)` unique, `(code, deleted)` unique | `api/models/competitions.py:899-901` |
| seasons | `(name, deleted)` unique, `(code, deleted)` unique | `api/models/seasons.py:157-159` |
| events | `(name, deleted)`, `(code, deleted)` unique (**never created**) | `api/models/events.py:132-134` |
| files | none besides `_id` | – |

### 1.6 Logging

- `logging.basicConfig(level=LOG_LEVEL)` writes plain text to stderr. There is no structured logging and no request ID. `pymongo` is forced to INFO (`api/core/database.py:6`).
- Gunicorn access log goes to stdout (`--access-logfile -`).
- Some modules use `print()` (`api/routers/competitions.py:452`, `api/controllers/competitions.py:147`).
- Several logging calls are wrong: `log.errorf` (`api/controllers/pilots.py:139`), and `log.error(msg, exc)` with no `%s` placeholder.

### 1.7 Error format

FastAPI defaults:
- `{"detail": "<message>"}` for `HTTPException`.
- 422 `{"detail":[...]}` for validation errors.

Business rule violations raise **400** with a human-readable English message, e.g. "Can't close a run while some flights are not published (…)". Missing entities raise **404** ("Competition X not found"). Some model inconsistencies raise **500** (seasons mixing solo and synchro). Unhandled Python errors give a bare 500. There are no error codes, only free text, and the manager shows that text to the user.

---

## 2. Authentication & authorization

### 2.1 Model

- **Single shared admin account** defined by env (`ADMIN_USER` / `ADMIN_PASS`). There is no users collection, no roles, no per-competition permissions, and judges have no accounts. Everyone who uses the manager shares the same credentials.
- `check_credentials` uses `secrets.compare_digest` for both username and password, so the comparison is constant-time (`api/core/security.py:17-22`).
- **Fail-open**: if `ADMIN_USER` or `ADMIN_PASS` is None, `check_credentials` always returns True **and** `auth = no_auth`, so every protected route is open (`api/core/security.py:18-19`, `94-99`). `ADMIN_PASS` defaults to None.

### 2.2 Login

| | |
|---|---|
| Endpoint | `POST /auth/login` (`api/routers/auth.py:9-24`) |
| Body | `application/x-www-form-urlencoded` OAuth2 password form: `username`, `password` (plus optional `scope`, `grant_type`, `client_id`, `client_secret`, which are ignored) |
| 200 | `{"access_token": "<jwt>", "token_type": "bearer"}` |
| 401 | `{"detail":"Incorrect username or password"}` |

### 2.3 Token

- JWT HS256 signed with `JWT_SECRET`. Claims: `type: "access_token"`, `exp` (now + `ACCESS_TOKEN_EXPIRE_MINUTES`, default 8 days), `iat`, `sub` (username) (`api/core/security.py:25-53`).
- Validation (`auth_jwt`, `api/core/security.py:67-89`) checks the signature, `exp`, and that `sub` is present. It does **not** check `type`, does not compare `sub` with `ADMIN_USER`, and disables audience verification. The function returns `ADMIN_USER`.
- Transport: `Authorization: Bearer <token>`. FastAPI's `OAuth2PasswordBearer(tokenUrl="/auth/login")` returns 401 with `WWW-Authenticate: Bearer` if the header is missing.
- There is no refresh token, no logout or revocation, and no rotation. The manager stores the token in `localStorage` (`manager/src/pages/login.js:58`, `manager/src/util/backend.js:17-20`).
- `GET /auth/me` returns a static profile: `{name: ADMIN_USER, nickname: ADMIN_USER, email: "<ADMIN_USER>@acropyx", org_id: "acropyx"}` (`api/routers/auth.py:26-38`).
- Dead code: `HTTPBasic` / `auth_http_basic` is defined but never used (`api/core/security.py:11,58-65`).

### 2.4 Protected vs. unprotected

Protection is applied **per route** with `dependencies=[Depends(auth)]`. There is no router-level guard.
- Everything under `/public/**`, plus `/`, `/status/`, `/auth/login` and the OpenAPI docs, is public by design.
- **Unprotected routes on the private routers.** These are opened in a new browser tab by the manager, where no Bearer header can be sent:
  - `GET /files/{id}` (`api/routers/files.py:21-36`)
  - `GET /competitions/{id}/results/export` (`api/routers/competitions.py:382-388`, `# dependencies` commented out)
  - `GET /competitions/{id}/results/{i}/export` (`api/routers/competitions.py:435-441`)
  - `GET /competitions/{id}/starting_order/{i}/export` (`api/routers/competitions.py:477-483`)
- In production the nginx proxy only forwards `/public`. The fly.io and render apps themselves are still reachable directly (`api.acroworldtour.com`), and that is how the manager reaches the private API.

### 2.5 CORS

`CORSMiddleware(allow_origins=["*"], allow_credentials=True, allow_methods=["*"], allow_headers=["*"], expose_headers=["Content-Disposition"])` (`api/main.py:65-72`). The nginx proxy also strips the upstream `Access-Control-Allow-Origin` and adds `*` itself on `/public/*` (`proxy/nginx.conf`).

---

## 3. Endpoint catalogue

### 3.0 Conventions

- **Auth** column: 🔒 = Bearer JWT required (when auth is enabled), 🌐 = public.
- `{id}` for competitions, seasons and tricks accepts **either the DB `_id` or the `code` (or `name` for tricks)**. Teams and judges accept `_id` only. Pilots use the int CIVL ID.
- Many routes are declared with a trailing slash (`"/"`). Calls without it (e.g. the manager's `GET /competitions`) get a **307 redirect** to the slash version (Starlette `redirect_slashes`).
- `deleted=true` query flag: include soft-deleted entities. `restore=true` on DELETE: undelete.
- Mutations return **204 No Content** unless stated. Create endpoints return **201** with the created object.
- Response schema names refer to Pydantic models in `api/models/*.py` (summarised in §3.4).

### 3.1 Root mounting (`api/routers/api.py`)

| Prefix | Router file | Mounted? |
|---|---|---|
| `/auth` | `routers/auth.py` | yes |
| `/status` | `routers/status.py` | yes |
| `/pilots` | `routers/pilots.py` | yes |
| `/judges` | `routers/judges.py` | yes |
| `/teams` | `routers/teams.py` | yes |
| `/tricks` | `routers/tricks.py` | yes |
| `/competitions` | `routers/competitions.py` | yes |
| `/scores` | `routers/scores.py` | yes |
| `/public` | `routers/public.py` | yes |
| `/utils` | `routers/utils.py` | yes |
| `/files` | `routers/files.py` | yes |
| `/seasons` | `routers/seasons.py` | yes |
| — | `routers/events.py` | **NO** (dead code) |
| — | `routers/public2.py` | **NO** (dead code, see §3.3) |

### 3.2 PRIVATE / ADMIN API (used by the manager)

#### Misc

| Method | Path | Auth | Params | Body | Response | Notes | Src |
|---|---|---|---|---|---|---|---|
| GET | `/` | 🌐 | – | – | HTML page with project name and version | | `api/routers/api.py:33-49` |
| GET | `/status/` | 🌐 | – | – | `Status{project, version}` | Health or version probe (the only API test) | `api/routers/status.py:8-17` |
| POST | `/auth/login` | 🌐 | – | OAuth2 form | `{access_token, token_type}` | §2.2 | `api/routers/auth.py:9` |
| GET | `/auth/me` | 🔒 | – | – | `{name,nickname,email,org_id}` | static | `api/routers/auth.py:26` |

#### Pilots (`/pilots`, `api/routers/pilots.py`)

Pilots are **never created by hand**. They are scraped from civlcomps.org.

| Method | Path | Auth | Params | Body | Response | Side effects / errors |
|---|---|---|---|---|---|---|
| GET | `/pilots/` | 🔒 | – | – | `List[Pilot]` sorted by `rank` then `name` | `:13-24` |
| GET | `/pilots/{civlid}` | 🔒 | civlid:int | – | `Pilot` | 404 if unknown or ≤0. Negative IDs below −999999 return a synthetic "simulator" pilot (`api/models/pilots.py:106-122`) |
| POST | `/pilots/update_all` | 🔒 | – | – | 201 empty | Downloads the CIVL aerobatics ranking xlsx, extracts every CIVL ID in column B (shuffled), then scrapes each pilot page (**synchronously within the request**, can take minutes). Errors are logged and swallowed. Not used by the manager UI. `:39-48` |
| POST | `/pilots/update_rankings` | 🔒 | – | – | 201 empty | Gets the CIVL ranking page, extracts the export URL and CSRF token, POSTs for the xlsx, then updates `rank` for known pilots, imports unknown ones, and sets rank 9999 for pilots no longer ranked. Bug: uses `%M` (minutes) instead of `%m` (month) when building the date (`api/controllers/pilots.py:72,76`). `:52-61` |
| POST | `/pilots/{civlid}` | 🔒 | civlid:int | – | 201 `Pilot` | Create or refresh **one** pilot by scraping `civlcomps.org/pilot/{id}` and `/ranking?discipline_id=5`. Keeps the existing `gender` and `awt_years`. 404 if CIVL returns 404, 500 on CIVL errors. Upsert (`insert_one`, then `update_one` on duplicate). `:64-74` |
| PATCH | `/pilots/{civlid}/gender` | 🔒 | civlid | – | 200 `Pilot` | **Toggles** man↔woman (`none` stays `none`). `:79-90` |
| PATCH | `/pilots/{civlid}/awt` | 🔒 | civlid, `year:int` (default = current year **evaluated at process start**) | – | 200 `Pilot` | Toggles `year` in `awt_years` (AWT tour membership). `:95-105` |

There is no DELETE endpoint for pilots. Pilots are deleted only through `/utils/cleanup_pilots`.

#### Judges (`/judges`, `api/routers/judges.py`)

| Method | Path | Auth | Params | Body | Response | Notes |
|---|---|---|---|---|---|---|
| GET | `/judges/` | 🔒 | `deleted:bool=false` | – | `List[Judge]` | |
| GET | `/judges/levels` | 🔒 | – | – | `["trainee","certified","senior"]` | |
| GET | `/judges/{id}` | 🔒 | `deleted` | – | `Judge` | 404 |
| POST | `/judges/new` | 🔒 | – | `Judge{name(≥2), country(ISO-3, validated with pycountry), level, civlid?}` | 201 `Judge` | 400 on duplicate name. If `civlid` is set, the pilot must exist |
| PUT | `/judges/{id}` | 🔒 | – | `Judge` (full replacement) | 204 | |
| DELETE | `/judges/{id}` | 🔒 | `restore:bool=false` | – | 204 | soft delete or restore. 400 if already in the target state |

#### Teams (`/teams`, `api/routers/teams.py`)

| Method | Path | Auth | Params | Body | Response | Notes |
|---|---|---|---|---|---|---|
| GET | `/teams/` | 🔒 | `deleted` | – | `List[TeamExport]` (pilots expanded) | |
| GET | `/teams/{id}` | 🔒 | `deleted` | – | `TeamExport` | |
| POST | `/teams/new` | 🔒 | – | `Team{name, pilots:[civlid,civlid]}` (exactly 2 known pilots) | 201 `TeamExport` | 400 on duplicate name |
| PUT | `/teams/{id}` | 🔒 | – | `Team` | 204 | |
| DELETE | `/teams/{id}` | 🔒 | `restore` | – | 204 | soft delete |

#### Tricks (`/tricks`, `api/routers/tricks.py`)

| Method | Path | Auth | Params | Body | Response | Notes |
|---|---|---|---|---|---|---|
| GET | `/tricks/` | 🔒 | `deleted`, `repeatable:bool?` | – | `List[Trick]` | |
| GET | `/tricks/uniques` | 🔒 | `solo=true`, `synchro=true` | – | `List[UniqueTrick]` (all generated variants, sorted by technical coefficient) | |
| GET | `/tricks/bonuses` | 🔒 | – | – | `settings.tricks.available_bonuses` | static |
| GET | `/tricks/directions` | 🔒 | – | – | `settings.tricks.available_directions` | static |
| GET | `/tricks/unique/{id:path}` | 🔒 | id = variant name or acronym (may contain `/` or `\`, hence `:path`) | – | `UniqueTrick` | 404 |
| GET | `/tricks/{id}` | 🔒 | id or name, `deleted` | – | `Trick` | |
| POST | `/tricks/new` | 🔒 | – | `Trick` | 201 `Trick` | Server **generates `tricks[]` variants** (directions × bonus combinations) via `TrickCtrl.generate_tricks` before insert |
| POST | `/tricks/import` | 🔒 | – | `List[Trick]` | 201 string message | Bulk create. Collects errors and returns 400 with the concatenated messages if any insert failed. **Partial success, no rollback.** Not used by the manager |
| PUT | `/tricks/{id}` | 🔒 | id or **name** | `Trick` | 204 | Regenerates variants. The documented bulk-import procedure uses `PUT /tricks/<name>` (`import/README.txt`) |
| DELETE | `/tricks/{id}` | 🔒 | `restore` | – | 204 | soft delete |

#### Seasons (`/seasons`, `api/routers/seasons.py`)

| Method | Path | Auth | Params | Body | Response | Notes |
|---|---|---|---|---|---|---|
| GET | `/seasons/` | 🔒 | `deleted` | – | `List[SeasonExportLight]` | counts pilots, teams and competitions |
| GET | `/seasons/{id}` | 🔒 | `deleted` | – | `Season` (raw) | |
| GET | `/seasons/{id}/export` | 🔒 | `deleted` | – | `SeasonExport` | **Recomputes season standings** (full results of every competition in the season) |
| POST | `/seasons/new` | 🔒 | – | `Season{name, code ~ ^[a-z]{3}-[0-9]{4}$, year>1900, image?(file id), country?(ISO-3 lowercase), index=999}` | 201 `Season` | 400 on duplicate |
| PUT | `/seasons/{id}` | 🔒 | – | `Season` | 204 | if `image` is a URL, it is truncated to the last path segment (file id) (`api/models/seasons.py:207-214`) |
| DELETE | `/seasons/{id}` | 🔒 | `restore` | – | 204 | |

The season code prefix carries meaning: `awt-YYYY` is the World Tour ranking, `awq-YYYY` is qualifiers (non-AWT pilots), and `xxx-YYYY` with an ISO-3 country is a national ranking (`api/models/competitions.py:853-889`).

#### Competitions (`/competitions`, `api/routers/competitions.py`)

| Method | Path | Auth | Params | Body | Response | Side effects / errors |
|---|---|---|---|---|---|---|
| GET | `/competitions/` | 🔒 | – | – | `List[CompetitionPublicExport]` | ⚠ Uses `export_public()`, which returns None for **unpublished** comps, so they are **missing from this admin list** (`:33-45`). Verify manager expectations |
| GET | `/competitions/{id}` | 🔒 | `deleted` | – | `CompetitionExport` (full: pilots, teams, judges, repeatable tricks, config, runs with flights and marks) | `:50-59` |
| POST | `/competitions/new` | 🔒 | – | `CompetitionNew{name, code? ~ ^[a-z][a-z0-9-]*[a-z0-9], start_date, end_date, location, published, type: solo\|synchro, image?, logo?, website?, seasons:[code]}` | 201 `CompetitionExport` | state=`init`, `repeatable_tricks` defaults to all tricks flagged repeatable. 400 on duplicate name or code, or end<start |
| PATCH | `/competitions/{id}` | 🔒 | – | `CompetitionNew` | 204 | 400 if closed. Cannot change type, name or code once open (`api/models/competitions.py:358-387`) |
| PATCH | `/competitions/{id}/pilots` | 🔒 | – | `List[int]` (civlids, **order = starting order of run 1**) | 204 | solo only, not closed |
| PATCH | `/competitions/{id}/teams` | 🔒 | – | `List[str]` | 204 | synchro only |
| PATCH | `/competitions/{id}/judges` | 🔒 | – | `List[str]` | 204 | |
| PATCH | `/competitions/{id}/repeatable_tricks` | 🔒 | – | `List[str]` | 204 | |
| PATCH | `/competitions/{id}/config` | 🔒 | – | `CompetitionConfig` | 204 | |
| DELETE | `/competitions/{id}` | 🔒 | `restore` | – | 204 | soft delete |
| POST | `/competitions/{id}/open` | 🔒 | – | – | 204 | init→open. Requires ≥2 pilots or teams and ≥2 judges (`check()`) |
| POST | `/competitions/{id}/close` | 🔒 | – | – | 204 | open→closed. 400 if any run is not closed |
| POST | `/competitions/{id}/reopen` | 🔒 | – | – | 204 | closed→open |
| POST | `/competitions/{id}/runs/new` | 🔒 | `pilots_to_qualify:int=0`, `repetitions_reset_policy:int=0` (0 none, 1 awt, 2 awq, 3 all) | – | 201 `RunExport` | Comp must be open. **Generates the starting order** (§7.3) |
| GET | `/competitions/{cid}/runs/{rid}` | 🔒 | rid: 0-based index | – | `RunExport` | 404 |
| PATCH | `/competitions/{id}/runs/{i}/pilots` | 🔒 | – | `List[int]` | 204 | manual reorder or edit of the run starting list |
| PATCH | `/competitions/{id}/runs/{i}/teams` | 🔒 | – | `List[str]` | 204 | |
| PATCH | `/competitions/{id}/runs/{i}/judges` | 🔒 | – | `List[str]` | 204 | |
| PATCH | `/competitions/{id}/runs/{i}/repeatable_tricks` | 🔒 | – | `List[str]` | 204 | |
| PATCH | `/competitions/{id}/runs/{i}/config` | 🔒 | – | `CompetitionConfig` | 204 | |
| POST | `/competitions/{id}/runs/{i}/open` | 🔒 | – | – | 204 | init→open |
| POST | `/competitions/{id}/runs/{i}/close` | 🔒 | – | – | 204 | open→closed. 400 listing participant names whose flight is missing or unpublished |
| POST | `/competitions/{id}/runs/{i}/reopen` | 🔒 | – | – | 204 | closed→open |
| GET | `/competitions/{id}/runs/{i}/flights/{pilot_team_id}` | 🔒 | civlid (solo) or team id | – | `Flight` | 400 if not a participant, 404 if no flight |
| DELETE | `/competitions/{id}/runs/{i}/flights/{pilot_team_id}` | 🔒 | | – | 204 | |
| POST | `/competitions/{id}/runs/{i}/flights/{pilot_team_id}/new` | 🔒 | **`save:bool` (required)**, `published:bool=false`, `mark_type:str?` (`awt`/`awq`) | `FlightNew{tricks:[unique trick name or acronym], marks:[JudgeMark{judge, technical?, technical_per_trick?, choreography?, landing?, synchro?}], did_not_start, warnings[], warnings2[], tip_touch_bonuses}` | `FinalMarkExport` | **Core scoring endpoint.** `save=false` gives a dry-run preview of the score. `save=true` stores or replaces the flight, sets `published`, and updates `last_update` on the flight, run and competition. 400 for unknown tricks, missing judge marks, or a non-participant. The manager calls it with `mark_type=awt` or `awq` for mixed events |
| GET | `/competitions/{id}/results` | 🔒 | – | – | `CompetitionResultsExport` (overall plus per-season, women and per-run sub-rankings) | full recompute |
| GET | `/competitions/{id}/results/export` | 🌐 (!) | `limit_run:int=-1` (0-based last run to include), `filetype=xls\|html` | – | xlsx (FileResponse, name `{id}-overall-results[-after-runN]-{ts}.xlsx`) or HTML (`comp_results.html`) | For AW[TQS] tours HTML drops `overall`. With no seasons, only overall and women. 400 for other filetypes |
| GET | `/competitions/{id}/results/{i}` | 🔒 | `published_only:bool=true` | – | `RunResultsExport` | |
| GET | `/competitions/{id}/results/{i}/export` | 🌐 (!) | `filetype=xls\|html` | – | xlsx (`{id}-run{i+1}-results-{ts}.xlsx`) or HTML (`run_results.html`) | xlsx gives 400 unless the run is final |
| GET | `/competitions/{id}/starting_order/{i}/export` | 🌐 (!) | `filetype` (ignored, always HTML) | – | HTML (`run_starting_order.html`) | AWT/AWQ comps are split per season. **Bug: `pilot.awt_year` typo raises a 500 for AWT seasons** (`:507`). Out-of-range `i` gives a 500 (IndexError) |

#### Scores (`/scores`, `api/routers/scores.py`)

| Method | Path | Auth | Params | Body | Response | Notes |
|---|---|---|---|---|---|---|
| POST | `/scores/simulate/{t}` | 🔒 | t = solo/synchro | `FlightNew` | `FinalMark` | **Broken**: calls `Trick.get_unique_trick(name, solo=…, synchro=…)`, but that function takes only `id`, so it raises TypeError → 500 (`api/controllers/scores.py:31-35` vs `api/models/tricks.py:220`). Unused by the frontends |

#### Files (`/files`, `api/routers/files.py`)

| Method | Path | Auth | Params | Body | Response | Notes |
|---|---|---|---|---|---|---|
| GET | `/files/{id}` | 🌐 | `download:bool=false` | – | raw bytes, `Content-Type` = stored content type, `Content-Disposition: inline` or `attachment; filename="…"` | same as `/public/files/{id}` |
| POST | `/files/new` | 🔒 | – | `multipart/form-data` field `file` | `FileID{id}` | §6 |
| DELETE | `/files/{id}` | 🔒 | `restore` | – | 204 | soft delete |

#### Utils (`/utils`, `api/routers/utils.py`)

| Method | Path | Auth | Response | Notes |
|---|---|---|---|---|
| GET | `/utils/backup` | 🔒 | `.tar.gz` named `backup-{db}-{ts}.tar.gz` with one `{db}/{collection}.json` (BSON extended JSON via `bson.json_util.dumps`) per collection | Built in memory and in a temp file, deleted by a background task. **Each collection is capped at 1000 documents** (`to_list(1000)`) (`api/controllers/utils.py:32-46`) |
| POST | `/utils/cleanup_pilots` | 🔒 | 204 | **Hard-deletes** every pilot not referenced by a competition, team or judge (`api/controllers/utils.py:48-52`, `api/controllers/pilots.py:288-313`). Both handlers are named `backup` |

### 3.3 PUBLIC API

#### `public.py` vs `public2.py`

| | `routers/public.py` | `routers/public2.py` |
|---|---|---|
| Mounted | **Yes**, at `/public` | **No.** Not imported in `api/routers/api.py` |
| Status | Live. Used by results.acroworldtour.com (`NEXT_PUBLIC_API_URL=https://api.acroworldtour.com/public`), cached by the nginx proxy | Abandoned "v2" experiment. It calls methods that **do not exist** (`Competition.export_public2_with_results`, `Season.export_public2`) and an outdated `CompCtrl.svg_overall/svg_run` signature, so it would 500 if mounted |
| Shape differences | Teams expanded as `TeamExport`. Includes tricks/unique, simulate, live overlays, files, `result_type` SVG standings | Teams returned raw (`Team`, pilot IDs only). Seasons list returned as full `SeasonPublicExport` (with results) instead of light. No simulate, overlay or files routes |

**Rewrite guidance:** only `public.py` needs to be specified. `public2.py` is dead code with no consumers.

#### `/public` endpoints (`api/routers/public.py`)

All are 🌐 and GET unless noted. "Pre-warm" means the handler first loads whole collections into the per-request `Cache` (§5).

| Method | Path | Params | Response | Used by results site? | Notes / src |
|---|---|---|---|---|---|
| GET | `/public/pilots/` | – | `List[Pilot]` | yes | `:37-44` |
| GET | `/public/pilots/{civlid}` | `results:bool=true` | `PilotWithResults` (Pilot + `competitions_results[{competition, rank}]` + `seasons_results[{season, rank}]`) or `Pilot` if `results=false` | yes (both variants) | **Very heavy**: recomputes results of every published competition, then exports every season of those competitions (`api/models/pilots_with_results.py`). `:49-67` |
| GET | `/public/teams/` | – | `List[TeamExport]` | yes | `:72-85` |
| GET | `/public/teams/{id}` | – | `TeamExport` | yes | `:90-101` |
| GET | `/public/judges/` | – | `List[Judge]` | yes | `:106-113` |
| GET | `/public/judges/{id}` | – | `Judge` | no | `:118-125` |
| GET | `/public/competitions/` | – | `List[CompetitionPublicExport]` (published only, sorted by name) | yes | `:130-142` |
| GET | `/public/competitions/{id}` | id or code | `CompetitionPublicExportWithResults` (summary + pilots, teams, judges + full `results` with per-run flights, **judge marks stripped**) | yes (main page) | 404 if unpublished. Pre-warms pilots, teams, judges and tricks. `:147-161` |
| GET | `/public/competitions/{id}/standings/{result_type}/svg` | `download=false`, `animated:int=-1` (start delay in seconds; ≥0 enables a looping vertical scroll) | `image/svg+xml` 500 px wide ranking table (max 10 visible lines, flags embedded as base64 from `api/flags/`) | no (broadcast/OBS use) | `result_type`: overall, awt, awq, women, or season code. **`download=true` → NameError 500** (`season` undefined, `:185`). `:166-189` |
| GET | `/public/competitions/{id}/standings/run/{run}/svg` and `/public/competitions/{id}/standings/{result_type}/run/{run}/svg` | `run` is **1-based**, `download`, `animated` | `image/svg+xml` run ranking | no | same download bug (`:218`). `:194-222` |
| GET | `/public/seasons/` | `deleted=false` | `List[SeasonExportLight]` | yes | `:227-237` |
| GET | `/public/seasons/{id}` | `deleted=false` | `SeasonPublicExport` (competitions with results, overall + women standings, `competitions_results` per comp) | yes | Recomputes all competitions of the season. `:242-256` |
| GET | `/public/seasons/{id}/standings/svg` | `download` | `image/svg+xml` | no | `:261-283` |
| GET | `/public/tricks/` | `repeatable:bool?` | `List[Trick]` | yes | `:288-294` |
| GET | `/public/tricks/unique/` | `solo=true`, `synchro=false` | `List[UniqueTrick]` | yes (simulator) | `:299-305` |
| POST | `/public/simulate/competition/{t}` | t=solo/synchro, `reset_repetitions_frequency:int=0` | body `List[FlightNew]` → `List[Flight]` (one scored flight per simulated run) | yes (`results/pages/simulator`) | Builds an in-memory competition with 2 fake pilots (ids −1000000 and −1000001) and judges `simulator`. No DB writes. CPU-heavy and unauthenticated. `:310-315` |
| GET | `/public/live/overlay/pilot/{civlid}/run/{run}` | `download` | `image/svg+xml` 1920×1080 overlay | no (broadcast) | §4. `:320-335` |
| GET | `/public/live/overlay/team/{team}/run/{run}` | `download` | `image/svg+xml` | no | `:340-355` |
| GET | `/public/live/overlay/competition/{id}` | `n_run:int=10` | `application/zip` of overlays for every pilot or team × runs 1..n_run | no | unbounded `n_run`. `:360-395` |
| GET | `/public/files/{id}` | `download` | raw file | yes (images: `/files/<id>` relative URLs in payloads are prefixed with `API_URL` by the results site) | `:399-412` |

#### nginx cache in front of `/public` (`proxy/nginx.conf`)

| Location | Cache TTL (nginx and `Cache-Control max-age`) |
|---|---|
| `/public` (default) | 5 min |
| `/public/pilots`, `/teams`, `/judges`, `/tricks` | 1 h |
| `/public/seasons` | 10 min |
| `/public/competitions` | **1 min** (the "live" results latency) |
| `/public/live`, `/public/simulate` | no cache |
| `/public/files` | 1 year (files are content-addressed) |

Cache key is `$proxy$request_uri`. The cache serves stale content on errors and while updating, and upstream `Cache-Control` and `Expires` headers are ignored. A `proxy.training.acroworldtour.com` host maps to the training API.

### 3.4 Main response schemas (abridged; full field lists in the domain spec)

| Schema | Key fields | Src |
|---|---|---|
| `Pilot` | `_id`(=civlid), civlid, name, civl_link, country(ISO-3 lower), about, social_links[{name,link}], sponsors[{name,link?,img}], photo, photo_highres?, background_picture, last_update, rank (9999 = unranked), gender(man/woman/none), awt_years[int] | `api/models/pilots.py:34-49` |
| `Judge` | `_id`, name, country, level, civlid?, deleted | `api/models/judges.py:29-35` |
| `Team` / `TeamExport` | `_id`, name, pilots [civlid] / [Pilot] | `api/models/teams.py:17-26` |
| `Trick` / `UniqueTrick` | see domain spec. `UniqueTrick` = one generated variant (name, acronym, technical_coefficient, bonus, bonus_types, base_trick, uniqueness…) | `api/models/tricks.py` |
| `CompetitionPublicExport` | `_id`, name, code, start/end_date, location, published, type, state, number_of_{pilots,teams,judges,runs}, image/logo (`/files/<id>`), website, seasons[code], last_update | `api/models/competitions.py:67-85` |
| `CompetitionExport` | above + pilots[Pilot], teams[TeamExport], judges[Judge], repeatable_tricks[Trick], config, runs[RunExport] | `:41-61` |
| `CompetitionPublicExportWithResults` | public export + pilots, teams, judges + `results: CompetitionResultsExport` | `:90-94` |
| `RunExport` | state, pilots[Pilot] (**ordered = starting order**), teams, judges, repeatable_tricks, config, flights[FlightExport], repetitions_reset_policy, last_update | `api/models/runs.py:33-45` |
| `FlightExport` | pilot?, team?, tricks[UniqueTrick], marks[JudgeMarkExport], did_not_start, final_marks?, published, warnings, warnings2, tip_touch_bonuses, last_update | `api/models/flights.py:20-31` |
| `FinalMarkExport` | judges_mark, technicity, bonus_percentage, technical, choreography, landing, synchro, bonus, score, warnings, warnings2, malus, notes[], mark_type | `api/models/marks.py:69-83` |
| `CompetitionResultsExport` | final, type, `results: {"overall"\|season code\|"women": [CompetitionPilotResultsExport{pilot?, team?, result_per_run[{rank,score}], score}]}`, runs_results[RunResultsExport] | `api/models/results.py:119-123` |
| `RunResultsExport` | final, type, results {key: [FlightExport]}, last_update | `api/models/results.py:20-27` |
| `SeasonExportLight` / `SeasonPublicExport` / `SeasonExport` | id, name, code, year, image, country, index, type, number_of_pilots/teams, (+number_of_competitions \| competitions + results[{type, results[{pilot?,team?,score}]}] + competitions_results) | `api/models/seasons.py:31-97` |

---

## 4. Real-time / live features

**There is no real-time transport.** There are no WebSockets, no SSE, no Redis pub/sub, and no background workers.

| Aspect | How it works today |
|---|---|
| Live result propagation | Judges' marks are entered in the manager, which calls `POST …/flights/{id}/new?save=true&published=true`. The result is visible on the next read of `/public/competitions/{id}`, which **recomputes everything on each request**. |
| Latency | Bounded by the nginx cache TTL on `/public/competitions` (**1 min**, with stale-while-updating). The results site uses SWR (default revalidate-on-focus/reconnect, **no `refreshInterval`**, `results/pages/_app.tsx`), so users must refocus or reload the page. |
| Change detection | `last_update` timestamps on flight, run and competition (set in `flight_save`, `api/models/competitions.py:745-757`) are exposed in exports and HTML ("Last update: … UTC"). No ETag or `If-Modified-Since`. |
| `events` router | **Not** live events: it is CRUD for "Events" (an umbrella entity grouping several competitions: name, code, dates, country, location, website, image/logo file ids, streaming_url, competitions[]). Only `/events/` GET (list), `/{id}` GET, `/new` POST, `/{id}` PATCH and `/{id}` DELETE are defined, all 🔒 (`api/routers/events.py`). **Never mounted, indexes never created, and `Cache` has no `events` bucket, so it would KeyError with a cache.** Treat it as an unimplemented feature idea. |
| Broadcast overlays | `templates/live-overlay-run-pilot.svg` is a 1920×1080 Inkscape design for the live video stream (lower-third). `LiveCtrl` reads it **from disk on each call** and does plain string replacement of `%{RUN}` → `RUN {n}` and `%{PILOT}` → upper-cased pilot or team name (`api/controllers/live.py`). It contains no scores. Served via `/public/live/overlay/...`. The competition variant returns a ZIP for pre-generation (filenames are ASCII-folded). The same template is used for teams. |
| Broadcast standings | `/public/.../standings/.../svg` (§3.3) produces 500 px ranking tables, optionally animated (`animateTransform` loop, `animated` = begin delay) for OBS-style overlays (`api/controllers/utils.py:55-117`). Fonts: "Exo". |

**Rewrite note:** a real push channel (SSE or WebSocket keyed by competition, emitted on flight save, publish and run state change) and a live overlay fed with actual scores would be new features. The legacy behaviour to preserve is "public results reflect published flights within ≈1 min".

---

## 5. Caching layer

### 5.1 In-process per-request `Cache` (`api/models/cache.py`)

- **This is not a shared cache.** A new `Cache()` is instantiated **per HTTP request** in routers and passed down to models. It is a request-scoped identity map that avoids reloading the same entity many times while building nested exports.
- Buckets: `pilots, teams, judges, tricks, competitions, seasons, files`, each holding two lists:
  - `cache[type]`: entities added one by one.
  - `all[type]`: the result of a full `getall()`. When present, it short-circuits later `getall()` calls.
- Lookup keys are matched by **linear scan**:

| Bucket | Matched on |
|---|---|
| pilots | `id` (int) or `name` |
| teams, judges | `id` or `name` |
| tricks | `id`, `name` or `acronym` |
| competitions, seasons | `id`, `name` or `code` |
| files | `id` |

- Invalidation: none needed (dies with the request). `clean(type)` exists but is unused.
- Pattern: public routes "pre-warm" with `asyncio.gather(Pilot.getall(cache), Team.getall(cache), Judge.getall(cache), Trick.getall(cache))` so later `X.get(id, cache)` calls are served from memory (`api/routers/public.py:154-160`).
- Pitfalls: `Cache.get` is O(n), so exporting n entities is O(n²). Cached objects are **shared mutable instances**: some code mutates exports in place (e.g. `CompCtrl.svg_overall` adds `awt/awq/women` keys to `results`). `Competition.results()` and `run_results()` call `Pilot.getall(list)` **without** the cache.

### 5.2 Redis / `CACHE_EXPIRES`

- `from redis import asyncio as aioredis` in `api/main.py:9` is **unused**. `CACHE_EXPIRES` and `_REDIS_URL` appear only in `docker-compose.yml:14-15` and are read by nothing. The compose `redis` service is unused. These are remnants of a removed Redis response cache.

### 5.3 Effective HTTP caching

Only the nginx proxy in front of `/public` (§3.3 table). The API itself sets no `Cache-Control` or ETag. The private API is uncached.

---

## 6. File handling

| Aspect | Behaviour | Src |
|---|---|---|
| Upload | `POST /files/new` (🔒), multipart field `file`. The whole file is read into memory, base64-encoded, and stored in Mongo `files` as `{_id, filename, content_type, content(base64 str), deleted}` | `api/routers/files.py:41-56` |
| ID | `_id = sha256(base64 content)` hex, which makes it **content-addressed and deduplicated**. Re-uploading the same bytes hits DuplicateKeyError → `save()` (updates filename and content type) | `api/models/files.py:39-48` |
| Validation | **None**: no size limit (bounded only by Mongo's 16 MB document limit, ≈12 MB raw), no MIME sniffing, and the client-supplied `content_type` is trusted | |
| Serving | `GET /files/{id}` and `GET /public/files/{id}` decode the base64 and return it with the stored content type. `Content-Disposition: inline` by default, or `attachment; filename="<stored name>"` (unescaped) with `?download=true`. Soft-deleted files return 404 | `api/routers/files.py:21-36`, `api/routers/public.py:399-412` |
| References | competitions `image` and `logo`, season `image`, and event `image`/`logo` store the **file id**. Exports turn it into the relative URL `/files/<id>`; clients prefix it with their API base (the results site uses `…/public`, giving `/public/files/<id>`, cached 1 year) | `api/models/competitions.py:227-235`, `api/models/seasons.py:231-234` |
| Delete | soft delete only. Nothing garbage-collects orphan files | |
| Manager usage | competition and season image upload (`manager/src/pages/competitions/show.js:172`, `seasons/show.js:173`) | |
| Static assets | `api/flags/*.svg` (ISO-2 country flags) are read from disk and inlined as base64 into SVG standings | `api/controllers/utils.py:115-118` |

---

## 7. Templates, exports & starting order

### 7.1 HTML templates (Jinja2, `api/templates/`)

| Template | Used by | Content |
|---|---|---|
| `base.html` | all HTML exports | print-oriented layout, Bootstrap 3.4.1 and Google Font Exo from CDN, page-break CSS, AWT branding |
| `comp_results.html` | `GET /competitions/{id}/results/export?filetype=html[&limit_run=N]` | one table per result type (season name or capitalised key), with rank, country, pilot or team, per-run rank and score (rowspan), and total. Title "final" or "intermediate results [after run N]", plus "Last update" |
| `run_results.html` | `GET /competitions/{id}/results/{i}/export?filetype=html` | per result type, sorted by run score. Pilot names link to `https://results.acroworldtour.com/pilots/{civlid}/{name}` |
| `run_starting_order.html` | `GET /competitions/{id}/starting_order/{i}/export` | one table per season (AWT/AWQ split) or "overall": starting position, country, pilot or team, and the generation timestamp |
| `live-overlay-run-pilot.svg` | `/public/live/overlay/*` | §4 |

Jinja2Templates autoescapes `.html`, so names are escaped there. SVG standings are built by string concatenation **without escaping** (`api/controllers/utils.py`).

### 7.2 XLSX exports (`api/controllers/competitions.py:21-222`)

- `comp_to_xlsx` / `run_to_xlsx` produce the **CIVL/FAI results-upload format**.
  - Solo columns: Rank, Number, First Name, Last Name, Nat, Gender, Glider, Sponsor, CIVL ID, Score.
  - Synchro adds a Team column and writes one row per team pilot.
- "Score" is **not the competition score**. It is the CIVL ranking points sequence: 100, 99, 98… For AW[TQS] tours the AWT pilots come first, and the first AWQ pilot restarts at 80.
- Names are split on the first space into first and last name. Gender is M/F (synchro is always "M").
- `run_to_xlsx` raises 400 if the run is not final. Files are written to a temp file and deleted after the response by a BackgroundTask.

### 7.3 Starting-order generation (`Competition.new_run`, `api/models/competitions.py:464-526`)

| Run | Order rule |
|---|---|
| First run | `competition.pilots` (or `teams`) **in stored order**, i.e. the order set in the manager via `PATCH /competitions/{id}/pilots` |
| Following runs | Participants of the **previous run**, sorted by **descending current overall rank** (worst ranked flies first, leader last). Participants missing from results get rank −1 and fly **last**. Mixed AWT+AWQ solo events rank AWT then AWQ in one list, so **AWQ pilots fly before AWT pilots** |
| Qualification cut | `pilots_to_qualify > 0` keeps only the first N of the sorted list. ⚠ Because the list is sorted worst-first, slicing `[0:N]` keeps the N **worst** pilots. Verify the intended semantics before reproducing |
| Run creation | The run copies the competition's judges, config and repeatable tricks, with state `init` and no flights |
| Manual changes | `PATCH …/runs/{i}/pilots|teams` replaces the order |

⚠ Side effect: `pilots = self.runs[-1].pilots; pilots.sort(...)` sorts the **previous run's list in place**, so the previous run's stored starting order is rewritten too (`api/models/competitions.py:494-499`).

---

## 8. Tests

| File | Type | Covers |
|---|---|---|
| `api/tests/test_status.py` | HTTP (TestClient on `main.app`) | `GET /status` returns `project` and `version`. Needs a reachable Mongo (lifespan) |
| `api/tests/test_close.py` | unit (pytest + anyio, `Competition.save`, `Pilot.get` and `Team.get` patched) | `run_close` succeeds only when every participant has a saved **and** published flight (solo and synchro). Error message lists participant names, falling back to the ID when unknown. Competition `close` is refused while runs are not closed (message lists run indexes) |
| `api/tests/test_season_women.py` | unit | `Season.women_results`: a women ranking exists only with ≥3 women in a solo overall (none for synchro) |

- Run via `make test` (docker-compose exec … pytest). `pytest-env` and `mongomock` are in requirements but **unused**, and there is no `pytest.ini` or conftest.
- **Not covered:** scoring engine, results aggregation, auth, any CRUD endpoint, exports, public API, starting-order generation, CIVL scraping.
- `api/samples/*.yml|json` hold sample competitions and flights (manual fixtures, not wired into tests).

---

## 9. Performance concerns

| # | Issue | Where |
|---|---|---|
| P1 | **Full results recomputation on every read**: `/public/competitions/{id}`, `/competitions/{id}/results`, SVG standings and exports all rescore every run and flight. `new_run` also recomputes the overall | `api/models/competitions.py:762-851` |
| P2 | **Cascading recomputation**: `/public/seasons/{id}` recomputes every competition of the season. `/public/pilots/{id}` (default `results=true`) recomputes **every published competition**, then calls `season.export()` for each season, which recomputes those competitions again (export calls export_public, then exports the comps again) | `api/models/pilots_with_results.py`, `api/models/seasons.py:244-361` |
| P3 | **N+1 queries**: `Competition.check()` on every `save()` fetches each pilot, team and judge individually. `flight_convert` does one `find_one` per trick (`tricks.name`/`tricks.acronym`, **unindexed** array fields). Starting-order export calls `Pilot.get`/`Season.get` per pilot. Overlay ZIP does `Pilot.get` per pilot × run | `api/models/competitions.py:161-201,686-698`, `api/routers/competitions.py:491-521` |
| P4 | **Whole-document rewrites**: the competition document embeds all runs, flights and marks. Every flight save `$set`s the whole document after a read and compare. **No optimistic locking**, so two concurrent saves (e.g. two operators) give a lost update | `api/models/competitions.py:215-225` |
| P5 | O(n²) request cache lookups (linear scan) | `api/models/cache.py:25-48` |
| P6 | Pre-warming loads **entire collections** (all pilots ≈ CIVL aerobatics list, all tricks with their generated variants) on most public requests | `api/routers/public.py` |
| P7 | `to_list(1000)` everywhere silently truncates collections over 1000 docs (pilots may approach this). Backup is truncated the same way | models, `api/controllers/utils.py:37` |
| P8 | Missing indexes: `deleted` (every query filters `deleted: None`), `competitions.seasons`, `tricks.tricks.name`/`tricks.tricks.acronym`. Index builds are fire-and-forget | §1.5 |
| P9 | One gunicorn worker. CPU-bound scoring and openpyxl run on the event loop (blocking). `/pilots/update_all` scrapes hundreds of pages inside one request | `api/Dockerfile.fly.io:12`, `api/controllers/pilots.py:141-171` |
| P10 | Files stored base64 in Mongo (+33 %) and served through Python. Overlay template and flag SVGs are re-read from disk per request | §6, `api/controllers/live.py` |
| P11 | Unpinned dependencies, and the dev container reinstalls requirements on every start | `api/requirements.txt`, `api/Dockerfile:8` |

---

## 10. Security concerns

| # | Issue | Where |
|---|---|---|
| S1 | **Auth fail-open**: if `ADMIN_PASS` is unset (the default), every admin route is open and `/auth/login` accepts any credentials | `api/core/security.py:18-19,94-99` |
| S2 | Insecure default `JWT_SECRET` literal in code. If not overridden, anyone can forge tokens | `api/core/config.py:12` |
| S3 | Single shared admin account: no per-user identity, roles, audit trail or revocation. 8-day tokens in `localStorage` | §2 |
| S4 | **A production-looking MongoDB connection string with credentials is committed** in `docker-compose.yml:12` (env var prefixed `__`, so disabled, but the secret is in git history). It must be rotated. `utils/Makefile` includes a `../mongo.pass.env` kept outside the repo | `docker-compose.yml:12` |
| S5 | Unauthenticated private-API exports (`results/export`, `results/{i}/export`, `starting_order/{i}/export`) expose unpublished results to anyone who knows the competition code | `api/routers/competitions.py:382-484` |
| S6 | Uploads: no size or type validation, client-controlled `Content-Type`, served `inline` from the API origin. An HTML or SVG upload gives **stored XSS** on api.acroworldtour.com. The filename is put unescaped into `Content-Disposition` | §6 |
| S7 | SVG standings and overlays interpolate pilot and team names (scraped from CIVL) without XML escaping, which allows SVG injection | `api/controllers/utils.py:113-122`, `api/controllers/live.py:18,25` |
| S8 | No rate limiting or brute-force protection on `/auth/login`. Unauthenticated CPU-heavy endpoints (`/public/simulate/...`, `/public/pilots/{id}`) and an unbounded `n_run` on the overlay ZIP are DoS vectors (partly mitigated by the nginx cache for GETs) | |
| S9 | CORS `*` with `allow_credentials=True`. Tolerable only because auth uses a header, not cookies | `api/main.py:65-72` |
| S10 | `/docs` and `/openapi.json` public in production. Operators are told to grab tokens from there | `import/README.txt` |
| S11 | Startup drops the whole DB if the DB name contains "test", a dangerous implicit rule | `api/main.py:25-27` |
| S12 | JWT validation ignores `type` and `sub` | `api/core/security.py:67-89` |

---

## 11. Dead, unused and broken code

| Item | Status | Where |
|---|---|---|
| `routers/public2.py` | not mounted, references nonexistent methods | `api/routers/public2.py` |
| `routers/events.py`, `models/events.py`, `controllers/events.py` | not mounted, indexes never created. Bugs: `logo_url` uses `self.image` (`api/models/events.py:125`), `get(deleted=True)` adds a bogus `id:"deleted"` filter (`:148-149`), Cache has no `events` bucket | |
| Redis import, `CACHE_EXPIRES`, `_REDIS_URL`, compose redis | unused | `api/main.py:9`, `docker-compose.yml` |
| `validate_ip` middleware | no-op | `api/main.py:53-63` |
| `auth_http_basic`, `HTTPBasic` | unused | `api/core/security.py` |
| `POST /scores/simulate/{t}` | broken (TypeError → 500), unused | `api/controllers/scores.py:31-35` |
| `POST /tricks/import`, `POST /pilots/update_all` | not called by the manager (manual or ops use) | |
| SVG standings, live overlays | not used by the frontends (external broadcast tooling) | `api/routers/public.py:166-395` |
| `File.getall`, `File.update`, `Cache.clean`, `settings.runs.sample`, `TESTDB`, `SERVER_NAME`, `mongomock`, `pytest-env` | unused | |
| Duplicate handler names (`close` ×2 `api/routers/competitions.py:189,200`, `run_get_results` ×2 `:429,442`, `list` ×2 `api/routers/public.py:293,304`, `backup` ×2 `api/routers/utils.py`) | work because routes register at decoration time, but they make OpenAPI operation IDs confusing | |
| Known bugs to **not** reproduce | `download=true` NameError on SVG standings (`api/routers/public.py:185,218`). `awt_year` typo (`api/routers/competitions.py:507`). `%M` month bug (`api/controllers/pilots.py:72,76`). `res.status_code` NameError (`api/controllers/pilots.py:148`). `log.errorf` (`:139`). Non-f-string 404 message in `run_get` (`api/models/competitions.py:532`). `year` default frozen at import (`api/routers/pilots.py:102`, `api/models/pilots.py:103`). In-place sort of the previous run's pilots (§7.3). Private `GET /competitions/` hides unpublished competitions (§3.2) | |

---

## 12. Rewrite checklist (API-level)

1. One versioned public read API (`/public/v1/...`), mirroring `public.py` only. One admin API behind real auth (per-user accounts, roles such as admin and judge/scorer, short-lived tokens with refresh, audit log).
2. Persist computed results (or cache them, invalidated on flight, run or competition mutation) instead of recomputing on every read. Expose ETag / `last_update`.
3. Push channel (SSE/WebSocket) per competition for live results and broadcast overlays.
4. Normalise runs and flights out of the competition document, or use atomic per-flight updates with optimistic concurrency.
5. Object storage for files with validation, safe `Content-Disposition`, and `nosniff`. Serve user content from a separate domain.
6. Keep feature parity: CIVL pilot import and ranking refresh, xlsx CIVL export, HTML print exports (results, run results, starting order), SVG standings and overlays, public score simulator, DB backup.
7. Pin dependencies, add structured logging, a real `/healthz` (DB ping), and tests for scoring, results, starting order, auth and exports.
