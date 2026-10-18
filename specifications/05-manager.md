# 05 — Manager (back-office / judging app)

> Scope: `manager/` in the legacy repo (`acropyx-frontend-manager`). This is the app judges and organizers use to manage reference data (pilots, teams, judges, tricks, seasons), set up competitions and runs, and **enter scores live** during a competition. All paths below are relative to the legacy repo root.
>
> Business rules (scoring formulas, validation) live in the FastAPI backend (`api/`). This document covers the manager's behavior, its API calls, and the parts of the business rules the operator actually sees.

---

## 1. Tech stack, configuration, build/run

### 1.1 Stack

| Concern | Choice | Notes |
|---|---|---|
| Framework | Next.js **14.2.4**, pages router, React 18.3.1 | `manager/package.json` |
| Output | **Static export** (`output: 'export'`) | `manager/next.config.js:4`. No server-side code, no API routes. All data is fetched client-side. |
| UI kit | MUI 5 (`@mui/material`, `@mui/lab` TabContext/TabList/TabPanel, `@mui/x-date-pickers`), `mdi-material-ui` icons | Based on the **Materio** free admin template (`manager/src/@core/**`) |
| Notifications | `notistack` snackbars, top-center, click to dismiss | `manager/src/util/notifications.js`, `manager/src/pages/_app.js:92-98` |
| Drag & drop | `@dnd-kit/core` + `@dnd-kit/sortable` | Starting order only (`manager/src/pages/competitions/run.js:511-522`) |
| "Auth" | `@auth0/nextjs-auth0` **client helpers only** (`UserProvider`, `useUser`, `withPageAuthRequired`), pointed at the custom API instead of Auth0 | `manager/src/pages/_app.js:76`. No Auth0 tenant is involved. |
| Misc libs | `react-country-flag`, `countries-code` (flags on pilot cards), `react-moment` (imported, not used), `jquery` (only to hide `.hideToPrint` elements when printing, `_app.js:66-73`), `nprogress` (route loader) | |
| Unused deps | `apexcharts-clevision`, `react-apexcharts`, `react-datepicker`, `react-perfect-scrollbar` (template only), `@mui/x-data-grid`, `moment` (only via react-moment), `@popperjs/core`/`react-popper` (only the dead UpgradeToProButton), `clsx`, `next-transpile-modules`, `@babel/eslint-parser` in dependencies | |
| Language | Plain JavaScript (no TypeScript, no PropTypes besides the table) | |
| Tests | **None** | |

### 1.2 Environment variables

| Variable | Used in | Purpose |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `manager/src/util/backend.js:24,59`, export links in `TabRunResults.js:68-71`, `TabCompResults.js:66-67`, `run.js:503` | Base URL of the FastAPI backend (e.g. `http://127.0.0.1:8000`, `https://api.acroworldtour.com`). Inlined at build time. |
| `NEXT_PUBLIC_BORDER_COLOR` | `manager/src/@core/theme/globalStyles.js:4` + `ThemeComponent.js:42-45` + `styles/globals.css:26-37` | Paints a colored frame around the whole viewport so operators can tell environments apart (e.g. `blueviolet` for dev/training, none for prod). **Keep this feature.** |
| `NEXT_TELEMETRY_DISABLED` | docker-compose | Next.js telemetry off |

No `.env` file is committed.

### 1.3 Build / run

| Command | Effect |
|---|---|
| `pnpm dev` | `next dev -p 3001` |
| `pnpm build` | `next build`, which writes the static export to `out/` |
| `pnpm check` / `lint` / `format` | eslint / eslint --fix / prettier on `src/**/*.{js,jsx}` |
| docker-compose (`docker-compose.yml:19-31`) | `node:20-alpine`, `pnpm install && pnpm run dev`, port 3001, source mounted |

The repo has no CI or deploy pipeline for the manager. `.github/workflows` only deploys the API and the proxy, so the manager is presumably deployed by hand as static files.

---

## 2. Authentication

### 2.1 Flow

| Step | Behavior | Ref |
|---|---|---|
| Page guard | Pages are wrapped with `withPageAuthRequired` (client-side HOC). When no user is resolved, the browser is redirected to `/login?returnTo=<current path>`. | `_app.js:76` (`loginUrl="/login"`) |
| User resolution | `UserProvider` calls `profileUrl="/auth/me"` through a custom `fetcher` that uses `APIRequest` (adds the bearer token). Any non-200 response throws `RequestError`. | `_app.js:60-64,76` → API `api/routers/auth.py:27-38` |
| Login form | `/login`: Username + Password fields. Submit sends `POST /auth/login` as `application/x-www-form-urlencoded` with `grand__type=password` (typo for `grant_type`; harmless because FastAPI's `OAuth2PasswordRequestForm` makes it optional), `username`, `password`. | `manager/src/pages/login.js:39-63` |
| Token storage | On success, `access_token` is stored in `localStorage['token']`. A success toast is shown and the page does a full reload to `returnTo` or `/`. | `login.js:57-62` |
| Token use | Every `APIRequest` adds `Authorization: Bearer <token>` if a token is present. | `util/backend.js:17-20` |
| Token lifetime | JWT, 8 days (`ACCESS_TOKEN_EXPIRE_MINUTES = 60*24*8`). No refresh token. | `api/core/config.py:15` |
| Users | One shared admin account (`ADMIN_USER`/`ADMIN_PASS` env on the API). If those are unset, the API **disables auth completely** and any login succeeds. | `api/core/security.py:20-25,113-118` |
| `/auth/me` payload | `{name, nickname, email: "<user>@acropyx", org_id: "acropyx"}`, all derived from `ADMIN_USER` | `api/routers/auth.py:33-38` |
| Logout | The user menu (avatar initial + "Admin" label) has a **Logout** item that navigates to `/logout`. That page removes `localStorage['token']`, toasts, and redirects to `/login`. | `@core/layouts/components/shared-components/UserDropdown.js:155`, `pages/logout.js:10-14` |

### 2.2 401 handling

* **No global interceptor.** A 401 from any business call is shown as a generic error toast, e.g. `wrong status code (received 401 while expecting 200): {"detail":"Could not validate credentials"}`. The user is not redirected.
* An expired or invalid token is only detected when a guarded page mounts and `/auth/me` fails.
* **Unguarded pages**: `competitions/index`, `competitions/show`, `competitions/run`, `seasons/index` and `seasons/show` are exported **without** `withPageAuthRequired` (`pages/competitions/show.js:709`, `run.js:537`, `competitions/index.js:305`, `seasons/index.js` last line, `seasons/show.js:448`). An anonymous visitor sees the page skeleton and a stream of 401 toasts.
* Export endpoints (CIVL XLSX, HTML results, starting-order HTML) are opened as plain links **without the token**. They only work because the API leaves them unauthenticated (`api/routers/competitions.py:384,437,480`, where `dependencies=[Depends(auth)]` is commented out).
* `util/auth.js` (`useAuth`, which stores `user` in localStorage) is dead code.

---

## 3. Shared plumbing

### 3.1 `APIRequest(route, props)` (`manager/src/util/backend.js:7-54`)

* Returns a tuple `[err, body, headers, status]`. It never throws on HTTP errors.
* `props.expected_status` (number or array, default `[200]`) and `props.expect_json`.
* Parses JSON when the content-type is JSON, and a Blob for `application/x-tar` (the backup).
* **Bugs**:
  * `expect_json` has no effect: `err` is set at line 41 and then reset by `var err = null` at line 44.
  * `expected_status > 0` (line 46) compares an array with a number. This works for `[200]`, because `"200" > 0`. For multi-value arrays like `[200, 404]`, `"200,404" > 0` is `false`, **so the status check is skipped entirely**. For example, a 500 or 401 on `GET flight` is then treated as success.
  * Logs every request and response to the console.
* Error messages shown to users are raw strings: `wrong status code (received X while expecting Y): <json body>`.

### 3.2 Data hooks (`util/backend.js:67-208`)

`usePilots`, `useTeams`, `useJudges`, `useTricks` and `useUniqueTricks(type)` each load once on mount (`GET /pilots`, `/teams`, `/judges`, `/tricks`, `/tricks/uniques?synchro=<type=='synchro'>`). They normalize `id` (`civlid` for pilots, `_id` otherwise) and toast on error. There is no cache: every page and tab remount refetches.

### 3.3 Reusable components

| Component | Behavior | Ref |
|---|---|---|
| `EnhancedTable` | MUI table copied from the MUI demo. Client-side sort by clicking column headers; optional pagination (disabled everywhere). Column types: default, `BOOLEAN` (disabled checkbox), `LINK` (next/link with `href(value,row)`), `ACTION` (button calling `func(row.id)`, or link `path/row.id`). Each column accepts `rewrite(value,row)`. The labels are the column ids lower-cased with `_` replaced by spaces. Selection/toolbar code is dead. | `views/tables/EnhancedTable.js` |
| `Editable` | Click-to-edit label: shows `Title: <strong>value</strong>`; clicking switches to the child input. **Enter or Tab** calls `onChange` (which saves to the API immediately), **Escape** calls `onCancel`. **Blur only exits edit mode, without saving or reverting**, so the label then shows an unsaved value. | `components/Editable.js:21-25,43` |
| `InputMark` | Judge mark input. The operator types an **integer 0–100**, and the value is stored as `int/10`, giving a 0.0–10.0 mark with 0.1 resolution. Uses `parseInt`, so `7.5` is read as `7`, i.e. **0.7**. Typing `75` gives 7.5. Values are clamped to [0,100]. A non-number yields `NaN`, which deletes the mark. Uncontrolled (`defaultValue`). Supports `tabindex`. | `components/InputMark.js:4-16` |
| `ResponsiveDatePicker` | MUI DatePicker. Overrides `toJSON` on the Date object so it serializes as `YYYY-MM-DD` (local time). | `components/ResponsiveDatePicker.js:19-24` |
| `SortablePilotStartingOrder` | One draggable `ListItem` with a drag handle (hidden when printing). | `views/runs/SortablePilotStartingOrder.js` |
| `CardPilot` | Pilot card: background picture, avatar photo, name linked to the CIVL page, gender emoji (click toggles), country flag, CIVL ID, rank (`9999` shown as "unranked"), one line per year from 2022 to the current year showing "⭐AWT"/"🌑AWQ" (click toggles), and an **Update pilot** button. | `views/cards/CardPilot.js` |

### 3.4 Layout

* Materio vertical layout with a left drawer menu (section "Main Menu"; `navigation/vertical/index.js`) and an app bar containing a light/dark **ModeToggler** (not persisted) and the user dropdown.
* App title: `🍑 Acro Scoring System 🍑` (`configs/themeConfig.js:3`). Browser `<title>` is `"… - 2022"` (hard-coded year) and the meta description is the Materio marketing text (`_app.js:78-83`).
* Loading pattern: most pages replace their **whole content** with a `LinearProgress` + "Loading" while any request runs (scroll position and tab focus are lost).
* Print support: elements with class `hideToPrint` are hidden during `window.print()` (used to print starting orders and results from the browser).

---

## 4. Sitemap

| Route | Menu entry | Auth guard | Purpose |
|---|---|---|---|
| `/` | Dashboard | yes | Shows "Welcome {user.name}!" and nothing else (`pages/index.js`) |
| `/login` | — | no | Login form |
| `/logout` | user menu | no | Clears token |
| `/pilots` | Pilots | yes | Pilot directory synced from CIVL |
| `/teams` | Teams | yes | Synchro teams CRUD |
| `/judges` | Judges | yes | Judges CRUD |
| `/tricks` | Tricks | yes | Base tricks list + edit modal (create disabled) |
| `/unique-tricks` | UniqueTricks | yes | Read-only list of the expanded "unique tricks" (trick + direction + bonuses) |
| `/competitions` | Competitions | **no** | List by year + create |
| `/competitions/show?cid=<code>` | — | **no** | Competition detail with 7 tabs |
| `/competitions/run?cid=<code>&rid=<index>` | — | **no** | Run detail with tabs, including **score entry** |
| `/seasons` | Seasons | **no** | List by year + create |
| `/seasons/show?sid=<code>` | — | **no** | Season detail with 4 tabs |
| `/utils` | Utils | yes | DB backup download, cleanup unused pilots |
| `/401`, `/404`, `/500` | — | — | Materio error pages (BlankLayout, template images) |

IDs live in query strings (`?cid=`, `?rid=`, `?sid=`) instead of path segments because of the static export. `rid` is the **0-based index** of the run in `competition.runs[]`; the UI shows `Run #rid+1`.

---

## 5. Screens

### 5.1 Pilots — `/pilots` (`manager/src/pages/pilots/index.js`)

| Element | Behavior | API |
|---|---|---|
| Load | Shows all pilots as `CardPilot` grid, sorted by `rank` ascending (`:191`) | `GET /pilots/` |
| Refresh icon | Reloads | same |
| "Filter pilots" | Client-side filter. A positive integer matches `civlid` exactly; anything else is a case- and accent-insensitive **regex** on the name (an invalid regex throws) (`:144-158`) | — |
| "CIVL ID" + **Add or Update pilot from CIVL** | Imports or refreshes one pilot from CIVL. Accepts IDs ≥1 only. Uses the global `event` instead of the handler argument (`:54-57`), which only works in browsers that expose `window.event`. | `POST /pilots/{civlid}` → 201 |
| **Update rankings** | Refreshes CIVL world rankings for all pilots | `POST /pilots/update_rankings` → 201 |
| **Synchronize from CIVL** | Confirmation "Are you sure to update all pilots ?". Then loops over **every** pilot client-side, calling `POST /pilots/{civlid}` with a 500 ms pause between calls. Progress % is shown in the loader and **one toast is shown per pilot**. Finally reloads. (`:105-130`). The API has `POST /pilots/update_all`, which is not used. | n × `POST /pilots/{civlid}` |
| Card: gender emoji | Toggles man/woman, toast "gender changed to X" | `PATCH /pilots/{civlid}/gender` |
| Card: year line | Toggles AWT membership for that year (2022 to current year) | `PATCH /pilots/{civlid}/awt?year=YYYY` |
| Card: **Update pilot** | Same as single import | `POST /pilots/{civlid}` |

There is no manual pilot creation or editing (name, country, and so on all come from CIVL) and no deletion (only the "cleanup unused pilots" util).

### 5.2 Teams — `/teams` (`manager/src/pages/teams/index.js`)

| Element | Behavior | API |
|---|---|---|
| Table | Columns: name, pilots (2 avatars + links; **both links point to pilot 0's page**, `:176-177`), update, delete | `GET /teams` (+ `GET /pilots` for the picker) |
| Search | Client-side regex on name; toasts "N teams filtered over M" **on every keystroke** | — |
| **new team** / **update** | Modal with **Name** (text) and **Pilots** (multi-select autocomplete "Name (civlid)"). No client-side validation; the API requires exactly 2 pilots. | create `POST /teams/new` body `{name, pilots:[civlid,…]}` → 201; update `PUT /teams/{_id}` (full object) → 204 |
| **delete** | Confirmation (`${name}` refers to the global `window.name`, so the message shows a blank name) | `DELETE /teams/{_id}` → 204 |

### 5.3 Judges — `/judges` (`manager/src/pages/judges/index.js`)

| Element | Behavior | API |
|---|---|---|
| Table | name (avatar + CIVL link if the judge has a `civlid` that matches a known pilot), country, level, civlid, update, delete | `GET /judges`, `GET /judges/levels`, `GET /pilots` |
| Modal fields | **Name** (text), **Level** (autocomplete from `/judges/levels`: `trainee`/`certified`/`senior`), **Country** (autocomplete over ISO list, stored as lower-case alpha-3), **CIVL ID** (optional; empty becomes `null`; sent as a string) | create `POST /judges/new` → 201; update `PUT /judges/{_id}` → 204 |
| delete | Confirmation (same `name` bug) | `DELETE /judges/{_id}` → 204 |

### 5.4 Tricks — `/tricks` (`manager/src/pages/tricks/index.js`)

| Element | Behavior | API |
|---|---|---|
| Table | name, acronym, directions, technical_coefficient, solo, repeatable, bonuses ("name (bonus)"), synchro, first_maneuver, no_first_maneuver, last_maneuver, no_last_maneuver, update, delete. Default sort by coefficient. | `GET /tricks`, `/tricks/bonuses`, `/tricks/directions` |
| Search | Regex on name or acronym | — |
| **new trick** | **Commented out** (`:271-277`). Tricks are seeded through `POST /tricks/import` (not exposed in the UI). | — |
| **update** modal | Name, Acronym, Directions (multi), "Bonuses ##TODO##" (**not editable**, `:325`), Solo?/Synchro?/Repeatable? checkboxes, Technical Coefficient (sent as a **string**), First/No First/Last/No Last Maneuver (numbers). The default for `no_last_maneuver` reads the misspelled `no_last_manoeuver` (`:410`), so it always displays 0. | `PUT /tricks/{_id}` → 204 |
| delete | Confirmation | `DELETE /tricks/{_id}` → 204 |

### 5.5 Unique tricks — `/unique-tricks` (`manager/src/pages/unique-tricks/index.js`)

Read-only table (page title says "Tricks"): name, acronym, technical_coefficient, bonus (%), bonus_types, base_trick. Search by name or acronym. `GET /tricks/uniques`.

### 5.6 Utils — `/utils` (`manager/src/pages/utils.js`)

| Action | Behavior | API |
|---|---|---|
| Download Database Backup | Fetches a tar.gz blob and triggers a download using the filename from `Content-Disposition` | `GET /utils/backup` |
| Cleanup unused pilots | Confirmation, then cleanup | `POST /utils/cleanup_pilots` → 204 |

### 5.7 Seasons list — `/seasons` (`manager/src/pages/seasons/index.js`)

* One table per `year`, years sorted descending. Each table has columns name (link to show), code, type, year, country, index, number_of_competitions, number_of_pilots, number_of_teams, sorted by `index` ascending. `GET /seasons`.
* **New Season** modal fields: **Name**, **Code** (the API enforces `^[a-z]{3}-[0-9]{4}$`, e.g. `awt-2024`, with no client hint), **Year** (year-only date picker, default current year), **Country** (optional, alpha-3), **Index** (display order, default 999). `POST /seasons/new` → 201.
* Bug: `emptySeason` is a module-level object that is mutated in place (`:45,109`), so the "new" form keeps the previous values.

### 5.8 Season detail — `/seasons/show?sid=<code>` (`manager/src/pages/seasons/show.js`)

**Load**: `GET /seasons/{sid}/export`. The page then **builds client-side** the de-duplicated union of pilots (`civlid`) and teams (`name`) across all competitions in the season (`:135-145`), and an acronym from the name initials.

**Header**:

| Element | Behavior | API |
|---|---|---|
| Avatar (image or acronym) | Click opens a file picker (images only; other types toast an error). The file is uploaded, then the season is saved with the new image id | `POST /files/new` (multipart `file`) → `{id}`; then `PUT /seasons/{sid}` |
| Editable Name / Code / Year / Country / index | Enter or Tab saves the whole season: `{name, code, year, country, index, image}` (image = last path segment of the URL). If the code changed, the page navigates to the new `sid`. | `PUT /seasons/{sid}` → 204 |
| **Delete** | Confirmation "delete Season X ?", then redirect to `/seasons` (toast wrongly says "Competition … deleted") | `DELETE /seasons/{_id}` → 204 |

**Tabs** (default: Results):

| Tab | Content |
|---|---|
| Pilots | Reuses `views/competitions/TabPilots` **without** `allPilots` and `update`. The add box is hidden, but the delete icon is still shown and **crashes** when clicked (`allPilots` undefined). |
| Teams | Same issue with `TabTeams`. |
| Competitions | Table: name (link to competition), type, start_date, location, number_of_runs (`views/seasons/TabCompetitions.js`). |
| Results | One table per `season.results[]` entry (`results.type` such as overall/women). Rank = array index + 1, sorted by score desc. Solo: rank, pilot (avatar+link), score (3 decimals). Synchro: rank, team, pilots (link bug, `TabSeasonResults.js:56-57`), score (not rounded). |

### 5.9 Competitions list — `/competitions` (`manager/src/pages/competitions/index.js`)

* `GET /competitions` returns `CompetitionPublicExport[]`. The page renders one table per start year (descending), with columns name (link to `/competitions/show?cid=<code>`), type, seasons, state, start_date, location, number_of_runs, code, published (checkbox). Default sort `start_date` ascending (the `orderId='desc'` prop at `:296` is a typo and is ignored).
* **Unpublished competitions do not appear in this list.** The API list endpoint builds each row with `export_public()`, which returns `None` for unpublished competitions, and those rows are skipped (`api/routers/competitions.py:40-44`, `api/models/competitions.py:287-289`). An unpublished competition (including a freshly created one with published=false) can only be reached by typing `/competitions/show?cid=<code>`. As a result, the "published" column is always checked.
* **new competition** modal:

| Field | Widget | API rule (`api/models/competitions.py:96-108`) |
|---|---|---|
| Name | text | required |
| Type | autocomplete `solo` / `synchro` | required, immutable once the competition is open |
| Start Date / End Date | date pickers (`YYYY-MM-DD`) | required |
| Location | text | required |
| Code | text (empty means omitted) | optional, `^[a-z][a-z0-9-]*[a-z0-9]`. If omitted, it is derived from the name (slugified). |
| Published | checkbox | **required boolean**. Bug: `defaultCHecked` typo (`:269`), so the box renders unchecked and `published` is **missing** unless the user toggles it, which makes the API reject the request with 422. |

  `POST /competitions/new` → 201. On error, a toast shows the raw 422 JSON. The new competition starts in state `init` with the default config and **all repeatable tricks pre-selected** (server).

### 5.10 Competition detail — `/competitions/show?cid=<code>` (`manager/src/pages/competitions/show.js`)

**Load**: `GET /competitions/{cid}` (`CompetitionExport`, with pilots, teams, judges, repeatable_tricks, config and runs embedded) plus the global `usePilots`, `useTeams`, `useJudges` and `useTricks`.

**Header area**

| Element | Behavior | API |
|---|---|---|
| Logo avatar | Click to upload (images only); ✕ removes it (with confirmation) | `POST /files/new`, then PATCH |
| Image (square avatar, right column) | Same: upload / ✕ remove | idem |
| Editable **Name**, **Code**, **Seasons** (free text, comma or space separated season codes), **Start date**, **End date** (free text, **not** a date picker), **Location**, **Website** (empty means null; the API requires a valid URL) | Enter or Tab saves the **full** competition object `{name, code, start_date, end_date, location, published, type, image, logo, website, seasons}`. A code change navigates to the new cid. The server rejects name, code and type changes unless state is `init`, and any change on a `closed` competition. | `PATCH /competitions/{cid}` → 204 |
| **Published** checkbox | Confirmation "publish/unpublish the competition?", then PATCH | idem |
| **Status** + state button | `init`: **Open**; `open`: **Close**; `closed`: **Reopen**. Each asks for confirmation. The server refuses Close while any run is not closed. | `POST /competitions/{cid}/open`, `/close`, `/reopen` → 204 |
| Type | read-only | — |
| Delete competition | `deleteCompetition` exists but only shows `alert('No yet implemented! #TODO')` and **is not rendered** (`:362-378`). | (API has `DELETE /competitions/{id}`) |

**Tabs**. The initial tab value `'actions'` matches no tab, so **the tab area is empty until a tab is clicked** (`:110`).

| Tab | Visible when | Display | Actions → API |
|---|---|---|---|
| **Pilots** (`views/competitions/TabPilots.js`) | type = solo | Table: name (avatar + CIVL link), country, rank, delete icon. Sort by rank. | Multi-select autocomplete of all pilots not yet registered, then **+** button or Enter: `PATCH /competitions/{cid}/pilots` with the **full list** `[civlid…]` (new ones prepended). Delete icon: confirmation, then PATCH without that pilot. |
| **Teams** (`TabTeams.js`) | type = synchro | name, pilots (link bug), delete | Same pattern: `PATCH /competitions/{cid}/teams` `[team_id…]` |
| **Judges** (`TabJudges.js`) | always | name, country, level, delete. Sort by level. | Same pattern: `PATCH /competitions/{cid}/judges` `[judge_id…]` |
| **Repeatables tricks** (`TabRepeatableTricks.js`) | always | name, acronym, delete | Same pattern over **all** base tricks: `PATCH /competitions/{cid}/repeatable_tricks` `[trick_id…]` |
| **Competition Settings** (`TabConfig.js`) | always | Editable fields (see table below) | Enter or Tab on any field sends the whole config: `PATCH /competitions/{cid}/config` → 204 |
| **Runs** (`TabRuns.js`) | always | Table of runs: "run #n" (link to run page), state, repetitions_reset_policy | **New Run** dialog, see below |
| **Results** (`TabCompResults.js`) | always | Overall results, see §5.12 | Export links |

Every successful mutation reloads the whole competition, which shows the full-page loader.

**Competition Settings fields** (`views/competitions/TabConfig.js`). The values are sent as **strings** and coerced by pydantic. There is no client-side validation.

| Field | Config key | Default (API settings) |
|---|---|---|
| Warning | `warning` (points deducted per cat-1 warning) | 0.5 |
| Malus Repetition | `malus_repetition` (% choreography malus per repetition) | 13 |
| Warnings to DSQ | `warnings_to_dsq` | 3 |
| Senior / Certified / Trainee Judge Weight | `judge_weights.{senior,certified,trainee}` | 100 / 100 / 20 |
| Max Twists / Reverses / Flips per run | `max_bonus_per_run.{twist,reverse,flip}` | 5 / 3 / 1 |
| Mark % Technical / Choreography / Landing (+ Synchro for synchro) | `mark_percentages[type].*` | solo 40/40/20; synchro 25/25/25/25 |
| **(missing)** | `warning2` (cat-2 warning points) exists in `api/models/competition_configs.py` but is **not editable** in the UI | 1 |

`TabConfig` mutates the `config` prop object in place (`value` is the same reference). Escape does not really revert.

**New Run dialog** (`TabRuns.js:82-122`)

| Field | Meaning | Default |
|---|---|---|
| Pilots to Qualify (number) | If > 0, only the first N of the computed starting list are kept (cut after a qualification run) | 0 = everybody |
| Repetition Reset Policy (select) | `0 None`, `1 Only for AWT pilots`, `2 Only for AWQ pilots`, `3 For all pilots`. Controls whether trick repetitions counted in previous runs are reset for this run. | Client heuristic (`:35-39`): +1 if the competition has an `awt-*` season and `runs.length > 3 && runs.length % 5 == 4` (i.e. runs 5, 10…), +2 if it has an `awq-*` season and `runs.length > 1 && runs.length % 2 == 0` (i.e. runs 3, 5…) |

**Ok** sends `POST /competitions/{cid}/runs/new?pilots_to_qualify=N&repetitions_reset_policy=P` → 201. The server requires the competition to be `open`. The server copies the competition judges, config and repeatable tricks into the run, and computes the **starting order**:
* first run: competition pilot/team list order;
* later runs: the previous run's list sorted by reverse current overall rank (best flies last). In mixed AWT+AWQ events, the AWQ block comes first and the AWT block after, never interleaved.

There is no run deletion in the UI (`run.js:274-290` is dead code).

### 5.11 Run detail — `/competitions/run?cid=<code>&rid=<i>` (`manager/src/pages/competitions/run.js`)

**Load**: `GET /competitions/{cid}` (the whole competition; the run is `data.runs[rid]`). There is no dedicated `GET /competitions/{cid}/runs/{rid}` call, although the API has one.

**Header**: link back to the competition, "Run #n" + refresh, **Status** with Open / Close / Reopen buttons (each with confirmation). They call `POST /competitions/{cid}/runs/{rid}/open|close|reopen` → 204. Server rules:
* open requires state `init`;
* close requires state `open` **and every participant has a saved and published flight**. Otherwise a 400 lists the missing or unpublished names.
* reopen requires `closed`.

**Tabs** (again the initial value `'actions'` shows nothing):

| Tab | Visible | Content / actions | API |
|---|---|---|---|
| Pilots | solo | `TabPilots` with `allPilots = comp.pilots`. Adding prepends to the list, which changes the starting order. | `PATCH /competitions/{cid}/runs/{rid}/pilots` `[civlid…]` |
| Teams | synchro | `TabTeams` with `allTeams = comp.teams` | `PATCH …/runs/{rid}/teams` |
| Judges | always | `TabJudges` with `allJudges = comp.judges`. These are the judges whose mark columns appear in score entry. | `PATCH …/runs/{rid}/judges` |
| Repeatables tricks | always | `TabRepeatableTricks` with all tricks | `PATCH …/runs/{rid}/repeatable_tricks` |
| Run Settings | always | `TabConfig` on `run.config` | `PATCH …/runs/{rid}/config` |
| **Flights (old)** | always | Solo: `TabFlightsSolo`; synchro: `TabFlightsSynchro` (one global technical mark per judge) | see §6 |
| **Flights (AWQ)** | solo | `TabFlightsSoloAWQ` (technical mark **per trick** per judge, `mark_type=awq`) | see §6 |
| **Flights (AWT)** | solo | `TabFlightsSoloAWT` (same UI, `mark_type=awt`) | see §6 |
| **Starting Order** | always | Title "Run N Starting Order", a draggable list `"<pos> <name>⭐"` (⭐ when the pilot's `awt_years` includes the competition start year), and a **PDF Starting Orders** link that opens the API HTML page in a new tab. Each drop immediately sends the **full reordered list**. | `PATCH …/runs/{rid}/pilots` or `/teams`; link `GET /competitions/{cid}/starting_order/{rid}/export` (HTML, unauthenticated) |
| **Results** | always | `TabRunResults`, see §5.12 | |

The server applies **no run-state guard** to run pilots/teams/judges/config updates, or to flight saves and deletes. A closed run can be modified, and an `init` run can be scored.

### 5.12 Results tabs

**Run results** (`views/competitions/TabRunResults.js`). `GET /competitions/{cid}/results/{rid}?published_only=false`.

* Title: "Final" if `results.final` (the run is closed and all flights are published), else "Intermediate". Then "Run N Results".
* Only `results.results["overall"]` is shown, re-sorted client-side by `final_marks.score` desc, with rank = position.
* Columns:
  * Rank;
  * "Team" (also for solo: pilot name; for synchro: team name + both pilots);
  * Tricks (`name (coef, bonus%)`);
  * Warnings (count + list, Malus %, notes, "DID NOT START");
  * Technicity + Bonus %;
  * Judges marks (technical, choreography, landing, synchro);
  * Final marks;
  * Bonus;
  * **Score**;
  * Published (checkbox).
* Export buttons (plain links, no auth):

| Button | URL |
|---|---|
| CIVL Run Export Run Results | `GET /competitions/{cid}/results/{rid}/export` (XLSX) |
| CIVL Export Overall Results after Run | `GET /competitions/{cid}/results/export?limit_run={rid}` (XLSX) |
| PDF Run Results | `…/results/{rid}/export?filetype=html` (new tab; the user prints it to PDF) |
| PDF Overall Results after Run | `…/results/export?limit_run={rid}&filetype=html` (new tab) |

**Competition results** (`views/competitions/TabCompResults.js`). `GET /competitions/{cid}/results`.

* One table per result type key: `overall`, `women`, `<season code>` such as `awt-2024` / `awq-2024`, and so on.
* Each title is prefixed "Final" or "Intermediate".
* Columns: Rank, Pilot (or team + pilots), Run (one line per run), Score (per-run scores), Score (total). The two headers are both labeled "Score".
* Exports: **Download CIVL Export** (`/results/export`, XLSX) and **PDF Export** (`/results/export?filetype=html`).

---

## 6. Scoring / judging workflow

### 6.1 User story: from zero to published results

| # | Actor | Step | Screen | API |
|---|---|---|---|---|
| 0 | Admin | Reference data is ready: pilots imported from CIVL (**Add or Update pilot from CIVL**), their **AWT years** flagged on the pilot card, rankings refreshed; judges created with a **level** (weights depend on it); teams created for synchro; tricks seeded (repeatable flags set); season(s) created (`awt-YYYY`, `awq-YYYY`, …). | Pilots / Judges / Teams / Seasons | §5.1–5.8 |
| 1 | Organizer | **Create the competition**: name, type solo/synchro, dates, location, code, published (must tick). State is `init`. | Competitions → new | `POST /competitions/new` |
| 2 | Organizer | Open the competition page and set **Seasons** (e.g. `awt-2024, awq-2024`). Seasons determine the result categories and the AWT/AWQ logic. Optionally upload logo/image and set the website. | Competition header | `PATCH /competitions/{cid}` |
| 3 | Organizer | Register **pilots** (solo) or **teams** (synchro), **judges**, and adjust the **repeatable tricks** list and **settings** (weights, %, malus, max bonuses). | Competition tabs | `PATCH …/pilots|teams|judges|repeatable_tricks|config` |
| 4 | Organizer | **Open** the competition (`init` → `open`). Name, code and type are now frozen. | Competition header | `POST /competitions/{cid}/open` |
| 5 | Organizer | **Create run** (New Run dialog: pilots to qualify, repetition reset policy). The run is created in state `init` with a server-computed starting order and a snapshot of judges, config and repeatable tricks. | Runs tab | `POST /competitions/{cid}/runs/new?…` |
| 6 | Organizer | On the run page: adjust **pilots/teams**, **judges** (for example a judge absent today), **settings**, and drag & drop the **Starting Order**. Print it (**PDF Starting Orders**). | Run tabs | `PATCH …/runs/{rid}/…`, export link |
| 7 | Organizer | **Open** the run (`init` → `open`). This is informational only, because scoring is not blocked in `init`. | Run header | `POST …/runs/{rid}/open` |
| 8 | Scorer (one operator at the judges' table, typing what the judges say or write) | For each pilot in starting order, enter the flight (see §6.2), then **Save & Publish & Next**. | Flights (old / AWQ / AWT) | `POST …/flights/{id}/new?save=false` on each change; `…?save=true&published=…` on save |
| 9 | Scorer | Exceptional cases: **Did not start**, cat-1 / cat-2 **warnings**, **Delete flight** (re-score from scratch), synchro **tip touches**. | Flights | same |
| 10 | Organizer | Check the **Results** tab of the run (intermediate). Export CIVL XLSX / print the HTML. | Run → Results | `GET …/results/{rid}?published_only=false`, exports |
| 11 | Organizer | **Close** the run. Refused while any participant has no saved flight or an unpublished one. Results become "Final". | Run header | `POST …/runs/{rid}/close` |
| 12 | Organizer | Repeat 5–11 for each run. The next run's starting order is based on the overall ranking so far. A qualification cut is applied with "Pilots to Qualify". Repetition resets follow the policy. | | |
| 13 | Organizer | **Close** the competition (all runs closed). Competition results become "Final". Export the CIVL file for the federation ranking. | Competition header / Results | `POST /competitions/{cid}/close`, `GET …/results/export` |
| 14 | Organizer | Set **Published** on the competition so the public site shows it (the public API filters on it). Individual flights must also be published (step 8) to appear publicly while the run is open. | Competition header | `PATCH /competitions/{cid}` |
| 15 | — | Fixes after closing require **Reopen** (competition, then run), then edit, then close again. | | `…/reopen` |

### 6.2 Flight entry screens in detail

All four flight components share the same skeleton (they are near copy-paste duplicates):

**Navigation**
* `◀` / `▶` buttons and an autocomplete "Pilot" (`Name (civlid)`) or "Team" over `run.pilots` / `run.teams`, **in starting order**.
* On mount the tab loads **participant #0**. Switching tabs remounts the component and goes back to #0.
* Loading a participant calls `GET /competitions/{cid}/runs/{rid}/flights/{civlid|team_id}`:
  * 200: the saved `Flight` is loaded. `result` is set to its `final_marks`, the save buttons are enabled, and the published flag is shown.
  * 404: an empty flight is loaded and the save buttons are disabled until a successful simulation.
  * Errors are only `console.log`ged. Because of the `[200,404]` bug in §3.1, any status is treated as success.

**Tricks column**
* **21 fixed slots** (indices 0–20). Each slot is an autocomplete over `useUniqueTricks(comp.type)` (`GET /tricks/uniques?synchro=<bool>`), grouped by `base_trick`, labeled `Name (ACRONYM) (coef)`.
* Equality is by acronym (solo tabs only).
* Empty slots are dropped on send: `tricks: data.tricks.filter(t => t != null).map(t => t.name)`. The API resolves names to unique tricks and returns 400 `Unknown trick(s)` otherwise.

**Marks**

| Tab | Mark grid | Stored as |
|---|---|---|
| Flights (old), solo `TabFlightsSolo.js:349-377` | Per judge: Technical, Choreography, Landing | `marks: [{judge: <id>, technical, choreography, landing}]` |
| Flights (old), synchro `TabFlightsSynchro.js` | Per judge: Technical, Choreography, Landing, **Synchro** | `+ synchro` |
| Flights (AWQ / AWT), solo `TabFlightsSoloAWQ.js:307-358, 363-405` | **Matrix: one row per trick slot × one column per judge** for the technical mark, plus a separate per-judge Choreography / Landing table | `marks: [{judge, technical_per_trick: [m0, m1, …], choreography, landing}]` |

* Every mark uses `InputMark` (type 0–100 → 0.0–10.0).
* AWQ/AWT keyboard order is defined by explicit `tabIndex`s:
  1. pilot picker (1);
  2. trick slots (100+i);
  3. judge 1's column down all tricks, then judge 2's column, and so on (1000·(j+1)+i);
  4. choreography/landing per judge (5000+10j, 5000+10j+1);
  5. buttons (10000+).

  This lets the scorer enter the whole sheet without the mouse. **Keep this.**
* A per-trick mark can only be entered once the trick slot is filled (`setMarkTechnicalDetail` guard, `TabFlightsSoloAWQ.js:164-165`).
* The old solo tab refuses to edit a flight that already has `technical_per_trick` marks and tells the user to use the AWQ/AWT tab (`TabFlightsSolo.js:249-253,289`).

**Which tab to use**
* The scorer must pick **AWT** for World-Tour pilots and **AWQ** for qualifier pilots (⭐ in the starting order). Nothing enforces this.
* The selected tab sets `mark_type`, and the score header shows a ⚠ icon when the loaded flight was scored with a different `mark_type` (`TabFlightsSoloAWQ.js:411`).
* Server semantics (`api/models/competitions.py:1066-1088, 1396-1425`):
  * **AWT**: for each trick, compute the weighted average of the judges' marks (weighted by judge level); the technical mark is the plain average over tricks.
  * **AWQ**: for each judge, average the marks over tricks; the technical mark is the weighted average over judges.
  * In both modes, a judge's missing per-trick mark is **guessed** as the average of the other judges' marks for that trick (`:1036-1063`). A trick with no mark at all returns 400. Mixing judges with and without per-trick marks returns 400.

**Live provisional score**
* **Every change** (trick picked, mark keystroke, warning added or removed) calls:
  ```
  POST /competitions/{cid}/runs/{rid}/flights/{id}/new?save=false[&mark_type=awq|awt]
  body: { tricks: [names], marks: [...], did_not_start, warnings: [..], warnings2: [..] [, tip_touch_bonuses] }
  ```
  The response is a `FinalMarkExport`, shown in the **Scores** panel:
  * Technicity, % Bonus, Published?;
  * Judge's marks: technical, choreography, landing (+ synchro);
  * Final marks: technical, choreography, landing (+ synchro);
  * Bonuses, **Final Score**;
  * **notes** (server explanations: repetitions, ignored tricks, max bonus reached, guessed marks, …);
  * cat-1 warnings list and cat-2 warnings list, each with a delete icon.
* On a simulation error, the error text is shown as a note and **all save buttons are disabled** (`resultsOK=false`). The old solo tab also toasts each error.
* No debounce and no request ordering: a slow earlier response can overwrite a newer one.

**Actions**

| Button | Effect | API |
|---|---|---|
| **Save** | Persists the flight, unpublished, and stays on this pilot | `POST …/new?published=false&save=true[&mark_type]` → returns `FinalMark`; toast "`<name>`'s flights saved with a `<score>` score" |
| **Save & Next** | Same, then loads the next participant | |
| **Save & Publish** | Persists with `published=true` (visible on the public site / counts toward "Final") | `…published=true&save=true` |
| **Save & Publish & Next** | Same, then next. This is the main button during a competition. | |
| **Did not start** | Confirmation "publish a DNS flight?", then `did_not_start=true` and saved **published**. There is no way to un-DNS a loaded flight except **Delete flight**, because `did_not_start` stays true in the loaded data. | idem |
| **Add cat 1 warning (0.5 pts)** / **Add cat 2 warning (1 pts)** | `prompt()` for free text, appended to `warnings` / `warnings2`, then re-simulates. The labels hard-code 0.5/1 pts even though the config may differ. | simulate |
| Preset warnings | cat 1: "flight over the public", "disregard of the flightbox", "missing or late at briefing", "missing or late at shuttle", "not ready before deadline". cat 2: "dangerous flying", "not using the rescue", "unsportmanship behavior". | simulate |
| Delete icon on a warning | Removes it from the local list and re-simulates. **Bug**: in the AWQ, AWT and synchro tabs the cat-1 delete calls `removeWarning(i)` instead of `removeWarning(1, i)` (`TabFlightsSoloAWQ.js:470`, `…AWT.js:470`, `TabFlightsSynchro.js:466`), so it removes the wrong warning (usually the first one). | simulate |
| **Tip Touch Bonuses** (synchro only) | `prompt("How many tip touches ?")`. Values other than 1–3 become 0. | simulate |
| **Delete flight** (AWT tab label: "Delete") | Double confirmation, then deletes the saved flight and reloads the participant | `DELETE /competitions/{cid}/runs/{rid}/flights/{id}` → 204 |

Warnings accumulate across runs. When their count reaches `warnings_to_dsq`, the pilot is disqualified server-side, which shows up as notes and score.

### 6.3 Scoring logic on the client side

The manager does **not** compute scores. All scoring goes through the API's simulate (`save=false`) endpoint. The client only does the following:

| Client logic | Where | Notes for the rewrite |
|---|---|---|
| Mark scaling: integer 0–100 divided by 10, clamped | `components/InputMark.js:8-11` | Make it explicit (decimal input with 0.1 or 0.5 step, validation 0–10) |
| Ranking of run results by `final_marks.score` desc, rank = index | `TabRunResults.js:97` | The server already sorts/ranks. Remove it and handle ties server-side. |
| Ranking of season results by array index | `TabSeasonResults.js:76` | idem |
| Season pilots/teams aggregation | `pages/seasons/show.js:135-145` | Move it to the API |
| Default repetition-reset policy heuristic | `TabRuns.js:35-39` | A rule ("every 5 AWT runs / every 2 AWQ runs") that belongs in the backend or the config |
| AWT ⭐ = `awt_years.includes(year(start_date))` | `run.js:513` | Same rule as `Pilot.is_awt` server-side |
| Tip touches clamp 1–3 | `TabFlightsSynchro.js:229-235` | Belongs in API validation |
| Warning point labels (0.5/1) | flight tabs | Read them from the config |

### 6.4 Correctness bugs in the scoring path (must not be reproduced)

| Severity | Bug | Ref |
|---|---|---|
| **High** | Synchro **DNS never sent**: the key is misspelled `did_no_start` in both the simulate and save bodies. | `TabFlightsSynchro.js:115,164` |
| **High** | Synchro **tip touches never saved**: `tip_touch_bonuses` is only in the simulate body, not in the save body. | `TabFlightsSynchro.js:118` vs `:160-166` |
| **High** | Synchro simulation sends `warnings2: data.warnings`, so the provisional score differs from the saved score. | `TabFlightsSynchro.js:117` |
| **High** | AWQ/AWT: `technical_per_trick[i]` is indexed by **slot**, but `tricks` is sent with empty slots removed. A gap between tricks misaligns marks and tricks. Clearing a trick does not clear its marks. | `TabFlightsSoloAWQ.js:117,162-179` |
| High | Wrong warning removed (see §6.2) | AWQ/AWT/synchro `:470/466` |
| High | The `[200,404]` status array disables error detection when loading a flight | `util/backend.js:46` |
| Medium | Synchro `didNotStart` calls `saveResults(true)` with `next` undefined, so `loadTeam(NaN)` throws | `TabFlightsSynchro.js:201` |
| Medium | Race conditions: un-debounced simulate on each keystroke, last response wins | all flight tabs |
| Medium | Server: no run-state guard on flight save/delete or on run roster/config changes | `api/models/competitions.py:534-567,718-760` |
| Low | Server export bug: `pilot.awt_year` (typo) in the AWT starting-order export | `api/routers/competitions.py:507` |

---

## 7. UX pain points, bugs, dead code, template leftovers

### 7.1 UX pain points

* **No real-time or multi-user support.** Nothing refreshes automatically. Two operators can overwrite each other's flights, since the last save wins with no version check.
* Every action reloads the whole entity and shows a full-page loader. Scroll position is lost, and the default tab on competition and run pages is **empty** (`tabContext='actions'`).
* Switching between flight tabs resets to the first pilot. There is no "next unscored pilot" and no per-pilot status in the navigator (saved, published, DNS).
* The scorer has to choose the AWQ or AWT tab manually per pilot. There are three overlapping flight tabs, one of them labeled "(old)".
* `prompt()`/`confirm()`/`alert()` are used for warnings, tip touches, deletes and state changes.
* `Editable` fields save on Enter/Tab but silently keep unsaved values on blur. Dates are free text on the competition page and use pickers elsewhere.
* Error messages are raw `wrong status code … {json}` strings. 422 validation details are not mapped to form fields. No client-side validation exists anywhere.
* Search fields fire an "N filtered over M" toast on every keystroke and use raw regexes (special characters break them).
* **Synchronize from CIVL** produces hundreds of toasts.
* The 21 fixed trick slots are not dynamic. The mark input convention (type `75` for 7.5) is not discoverable.
* Exports are plain links to unauthenticated endpoints. "PDF" buttons actually open HTML for browser printing.
* No delete for competitions or runs, no trick creation, and bonuses are not editable.
* Not mobile-friendly (tables with `minWidth: 750`, wide matrix).
* No audit trail of who changed a score, and only one shared admin account.

### 7.2 Other functional bugs

| Bug | Ref |
|---|---|
| Competition create fails unless "Published" is toggled (`defaultCHecked` typo) | `pages/competitions/index.js:269` |
| Unpublished competitions are invisible in the manager list (API uses the public export) | `api/models/competitions.py:287-289` |
| `orderId` typo, so the sort order is ignored | `pages/competitions/index.js:296` |
| Season "new" form state leaks (shared mutated `emptySeason`) | `pages/seasons/index.js:45,109` |
| Delete confirmations show `window.name` instead of the entity name | `teams/index.js:131`, `judges/index.js:139`, `tricks/index.js:147` |
| The second team pilot links to the first pilot's page | `teams/index.js:176-177`, `TabTeams.js:40-41`, `TabSeasonResults.js:56-57` |
| Season Pilots/Teams tabs: delete icon crashes (no `allPilots`) | `pages/seasons/show.js:430-434` |
| Season delete toast says "Competition" | `pages/seasons/show.js:235` |
| `no_last_manoeuver` typo in the trick edit default | `pages/tricks/index.js:410` |
| `handleSubmitUpdatePilot` uses the global `event` | `pages/pilots/index.js:54-55` |
| Run results header says "Team" for solo; comp results has two "Score" headers | `TabRunResults.js:85`, `TabCompResults.js:85-86` |
| `warning2` is not configurable | `TabConfig.js` |
| `utils.js` sets `setLoading(true)` after a successful download and never resets it (harmless, unused) | `pages/utils.js:60` |
| Five business pages lack the auth guard | §2.2 |
| Season results synchro score not rounded | `TabSeasonResults.js:63-64` |
| Lists keyed by index or missing `key` | flight tabs |

### 7.3 Dead code

| Item | Ref |
|---|---|
| `views/runs/TabPilots.js`, `TabTeams.js`, `TabJudges.js`, `TabReapeatableTricks.js` (older run tabs, never imported) | `manager/src/views/runs/` |
| `util/auth.js` (`useAuth`) | |
| `deleteCompetition`, `deleteRun` (alert "not yet implemented", not rendered) | `show.js:362`, `run.js:274` |
| Unused refs, imports (`CardPilot`, `Moment`, `Editable` in flight tabs, `Breadcrumbs`, …), `headCells` in `TabFlightsSynchro.js:184-197`, `nameRef`, etc. | everywhere |
| EnhancedTable selection/toolbar/dense code | `views/tables/EnhancedTable.js:440-480,496-533` |
| Commented-out search box and notifications in the app bar; commented profile/inbox/pricing items in the user menu | `AppBarContent.js:36-55`, `UserDropdown.js:115-154` |
| ~15 `console.log` calls (including one per mark row per render) | |

### 7.4 Materio template leftovers

| Leftover | Ref |
|---|---|
| `UpgradeToProButton.js` ("Upgrade To Pro", themeselection links) | `manager/src/layouts/components/UpgradeToProButton.js` |
| `@core/components/card-statistics`, `react-apexcharts` wrapper, apexcharts style libs, `react-datepicker` styles | `manager/src/@core/components/**`, `@core/styles/libs/**` |
| `NotificationDropdown` (imported, commented) | `@core/layouts/components/shared-components/NotificationDropdown.js` |
| 401/404/500 Materio pages + `views/pages/misc/FooterIllustrations.js`; `views/pages/auth/FooterIllustration.js` unused | `pages/401.js`, `404.js`, `500.js` |
| Demo images: avatars, cards (iPhone, paypal…), bank logos, `vercel.svg`, `coollogo_com-*.png` | `manager/public/images/**` |
| Meta description / keywords promoting the Materio template; "- 2022" title | `_app.js:78-83` |
| `.eslintrc.js` references `babel-eslint` (not installed) | |
| Still used from @core: `VerticalLayout`, `BlankLayout`, navigation drawer, app bar, `ModeToggler` (dark mode, not persisted), `UserDropdown` (customized), `ThemeComponent` (+ env border), `settingsContext`, `ScrollToTop`, footer ("© year - Acro Scoring System") | |

---

## 8. Must-have features for the rewrite

### Platform and security
- [ ] Real authentication with **per-user accounts and roles** (at least: admin, organizer for a competition, scorer/judge). The legacy app has one shared admin account.
- [ ] Every back-office route is guarded. **A 401 anywhere redirects to login with return-to.** Token refresh or a session approach. Logout.
- [ ] Authenticated (or signed-URL) exports. Do not rely on unauthenticated API endpoints.
- [ ] Configurable API base URL per environment, plus a **visible environment marker** (the colored border from `NEXT_PUBLIC_BORDER_COLOR`).
- [ ] A typed API client with consistent error handling (validation errors mapped to form fields, human-readable messages).
- [ ] Audit log: who created, changed, published or deleted each flight and state change.

### Reference data
- [ ] Pilots: import/refresh one pilot from CIVL by ID; **bulk sync as a server-side job** with a progress indicator (not N client calls); refresh rankings; toggle gender; **toggle AWT membership per year**; search by name (accent-insensitive) or CIVL ID; card/list view with photo, flag, rank.
- [ ] Teams CRUD (exactly 2 distinct pilots, validated in the form).
- [ ] Judges CRUD (name, level trainee/certified/senior, country alpha-3, optional CIVL ID linked to a pilot).
- [ ] Tricks: list, **create**, edit including **bonuses**, directions, solo/synchro/repeatable flags, coefficient, first/last-maneuver constraints; bulk import; view of the derived unique tricks.
- [ ] Seasons CRUD (name, code `xxx-YYYY`, year, country, display index, image upload) with a detail view: competitions, pilots, teams and season standings per category.
- [ ] Utilities: DB backup download, cleanup of unused pilots (admin only).

### Competition setup
- [ ] Create a competition with validation (name, type solo/synchro, start ≤ end dates, location, optional code slug, published flag, website, seasons **picked from existing seasons**, logo and image upload/remove).
- [ ] State machine **init → open → closed (→ reopen)**, with buttons that explain why an action is refused (open runs, …). Freeze name, code and type once open.
- [ ] Manage participants (pilots or teams), judges, repeatable tricks, and the full **config** including `warning2`, with typed and validated inputs.
- [ ] Publish/unpublish the competition (public visibility). The back-office list must show **all** competitions, including unpublished and draft ones, with filters.
- [ ] Delete or archive a competition (soft delete; the API supports `restore`).

### Runs
- [ ] Create a run with "pilots to qualify" and the **repetition reset policy**, with a default computed by the backend rule.
- [ ] A run detail page that **opens on a useful default tab**: roster, judges, repeatable tricks, config (per run), starting order.
- [ ] **Starting order drag & drop** with AWT/AWQ markers, explicit insertion position for added pilots, and a print/export view.
- [ ] Run state machine init → open → closed (→ reopen), with clear blocking reasons (missing or unpublished flights listed with names). **The backend must enforce the run state on scoring and roster edits.**
- [ ] Delete a run (with safeguards) if allowed by the rules.

### Live scoring (the critical part)
- [ ] Participant navigator in **starting order** with per-participant status (not scored / saved / published / DNS), prev/next, and "next unscored".
- [ ] **One unified flight entry screen** that selects the marking scheme automatically (solo AWT vs AWQ from the pilot's AWT status for the competition year, synchro). An admin can override, with a warning when the stored `mark_type` differs.
- [ ] A dynamic trick list (add/remove/reorder, no fixed 21 slots), searchable by name or acronym, grouped by base trick, showing the coefficient. **Per-trick technical marks stay attached to their trick** when tricks are reordered or removed.
- [ ] Mark grid per judge (technical global or per trick; choreography; landing; synchro for synchro). Explicit 0–10 decimal input (0.1 step) with validation and a fast typing shortcut. **Full keyboard flow** (tab order trick list → each judge's column → choreography/landing → save).
- [ ] **Provisional score** computed by the backend simulation endpoint, **debounced and race-safe** (ignore stale responses). Show technicity, bonus %, judges' marks, final marks, bonus, malus, score, **notes** (repetitions, guessed marks, ignored tricks), and warnings.
- [ ] Warnings cat-1 / cat-2 with presets and free text, removable by index (correctly). Point values come from the config. DSQ threshold visible.
- [ ] DNS toggle (reversible), synchro tip touches (validated 0–3), **both persisted on save**.
- [ ] Save (draft), Save & Publish, plus "& Next" variants. Delete flight with confirmation. Optimistic concurrency (version/`last_update` check) so two scorers cannot silently overwrite each other.
- [ ] Optional: live updates (WebSocket/SSE) of flights and results across operator screens; offline-tolerant entry with retry.

### Results and exports
- [ ] Run results (intermediate/final; including unpublished flights for staff) with the full breakdown (tricks, warnings, malus, notes, DNS, judges' and final marks, bonus, score, published flag). **Ranking computed by the backend.**
- [ ] Competition results per category (overall, women, each season such as AWT/AWQ), with per-run scores.
- [ ] Exports: CIVL XLSX (run, overall, overall after run N), printable/PDF run results, overall results after run N, starting order.
- [ ] Season standings.

### Quality
- [ ] TypeScript, shared API types generated from the backend OpenAPI spec, unit tests for the mark input and trick/mark alignment, and E2E tests for the full workflow in §6.1.
- [ ] No template leftovers. Responsive layout usable on a laptop at the judges' table (and ideally on a tablet).
