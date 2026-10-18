# 06 — Public results website (`results/`)

Reverse-engineered specification of the legacy public results site (`results.acroworldtour.com`).
All paths are relative to the legacy repo root (`legacy/legacy/`). The legacy code is read-only; this
document describes **what exists** (behaviour, data, look) and **what the rewrite must keep / fix**.

---

## 1. Overview

| Item | Value |
|---|---|
| Purpose | Public, read-only website: live and past competition results, seasons standings, pilots, teams, judges, tricks catalogue, run score simulator. |
| Audience | Pilots, fans, organisers, judges. Mostly mobile (bottom tab bar) but also desktop (left side bar). |
| Data source | Public API only: `${NEXT_PUBLIC_API_URL}` (default `https://api.acroworldtour.com/public`), router `api/routers/public.py` mounted under `/public` (`api/routers/api.py:28`). |
| `public2` router | `api/routers/public2.py` defines an alternative router but it is **never mounted** (no `include_router(public2)` anywhere) — dead code, ignore. |
| Auth | None. No cookies, no login. |
| Language | English only, hard-coded strings. `<html lang="en">` (`results/pages/_document.tsx:5`). |

---

## 2. Tech stack, dependencies, configuration

### 2.1 Stack

| Layer | Choice | Source |
|---|---|---|
| Framework | Next.js **14.2.4**, **Pages Router** (not App Router) | `results/package.json:24`, `results/README.md:17` |
| Language | TypeScript 5.4.5, `strict: true`, path alias `@/* -> ./*` | `results/tsconfig.json:7,17-19` |
| UI | React 18.3.1 | `results/package.json:26-27` |
| Styling | Tailwind CSS 3.4.4 + `@tailwindcss/forms` (strategy `base`) + `@tailwindcss/typography`; PostCSS + autoprefixer | `results/tailwind.config.js`, `results/postcss.config.js` |
| Component lib | MUI 5 (`@mui/material`, emotion) — **only used by the simulator** (Autocomplete, TextField, MenuItem, IconButton) | `results/pages/simulator/index.tsx:1-3`, `results/components/simulator/*.tsx` |
| Data fetching | SWR 2.2.5, global fetcher via `<SWRConfig>` | `results/pages/_app.tsx:26`, `results/utils/fetcher.ts` |
| Flags | `semantic-ui-flag` CSS sprites (`<i class="fr flag">`) + `i18n-iso-countries` to convert ISO alpha-3 → alpha-2 | `results/pages/_app.tsx:2`, `results/components/pilot/pilotCard.tsx:15` |
| Font | Google font **Montserrat** (latin) via `next/font/google` | `results/pages/_app.tsx:12-14` |
| Utils | `classnames` | everywhere |
| Type generation | `openapi-typescript` 6.7.6 generating `types.d.ts` from `https://api.acroworldtour.com/openapi.json` (`pnpm interface`) | `results/package.json:11` |
| Lint/format | ESLint 9 (`next/core-web-vitals`, `prettier`, `jsx-a11y`, `simple-import-sort`), Prettier 3 + tailwind plugin, single quotes except `.tsx` (double) | `results/.eslintrc.json`, `results/.prettierrc.json` |
| Git hooks | husky `pre-commit` runs `pnpm build` (installed from `manager/.husky`!) | `results/.husky/pre-commit`, `results/package.json:12` |
| Package manager | pnpm (`pnpm-lock.yaml`) | |

### 2.2 Scripts

| Script | Command | Note |
|---|---|---|
| `dev` | `pnpm format && clear && next dev` | formats the whole repo on every dev start |
| `build` | `next build` | |
| `start` | `next start` | Node server needed (see 2.5) |
| `lint` | `next lint --fix` | |
| `format` | `prettier --write .` | |
| `interface` | `openapi-typescript https://api.acroworldtour.com/openapi.json --output types.d.ts` | regenerates the data contract from **prod** |
| `prepare` | `cd .. && husky install manager/.husky` | couples results to the manager hooks dir |

### 2.3 Environment variables

| Variable | Used? | Default | Purpose |
|---|---|---|---|
| `NEXT_PUBLIC_API_URL` | **Yes** (`results/constants.ts:4-5`) | `https://api.acroworldtour.com/public` | Base URL of public API, inlined at build time. Also used to absolutise relative image paths (`results/utils/common.ts:10-14`). |
| `NEXT_CACHE_DURATION` | **No** — set to `1` in `docker-compose.yml:44` but never read by the code | — | Dead config. |
| `NEXT_TELEMETRY_DISABLED` | Next built-in | `1` in compose | |

Other constants: `SPORTING_CODE_MANUAL_URL = https://www.fai.org/sites/default/files/civl/documents/sporting_code_s7_b_-_aerobatics_2025.pdf` (`results/constants.ts:1-2`), hard-coded, must be edited each year.

### 2.4 `next.config.js`

- Adds header `X-Content-Type-Options: nosniff` on every route (`results/next.config.js:4-16`).
- `images.domains` whitelist for `next/image`: `api.acroworldtour.com`, `proxy.acroworldtour.com`, `api.training.acroworldtour.com`, `proxy.training.acroworldtour.com`, `civlcomps.org`, `172.17.0.1`, `localhost`, `127.0.0.1` (`results/next.config.js:17-28`).
- `reactStrictMode: true`.
- No `output: 'export'`, no rewrites, no i18n, no redirects.

### 2.5 Rendering strategy

| Aspect | Legacy behaviour |
|---|---|
| SSR / SSG / ISR | **None for data.** No `getServerSideProps`, `getStaticProps` or `getStaticPaths` anywhere. Pages are statically pre-rendered as empty shells by Next; dynamic routes (`[code]`, `[...civlid]`, `[...teamId]`) are client-resolved (`router.isReady` → `router.query`). README: "The website is static, and the data fetching happens on the client" (`results/README.md:18`). |
| CSR | 100 % client-side fetching with SWR. |
| SWR config | Only `fetcher` set (`results/pages/_app.tsx:26`). **No `refreshInterval` anywhere** (grep confirmed). Defaults apply: revalidate on mount, on window focus, on reconnect; 2 s dedupe. |
| Preloading | `preload()` of list endpoints on nav `onMouseEnter` (`results/components/layout/nav.tsx:31-66`), on team card hover (`results/components/team/teamCard.tsx:22`), at module load for `/competitions/` (`results/pages/index.tsx:15`) and `/teams/` (`results/pages/teams/index.tsx:14`). |
| Server needed? | Yes in practice: `next/image` with remote domains requires the Next image optimiser (`next start` or Vercel). |
| SEO consequence | Crawlers/social previews only see empty `<title>` and no content (title/description are set client-side in `useEffect`). |

### 2.6 Build / deploy

| Item | Finding |
|---|---|
| CI | No GitHub workflow for `results/` (`.github/workflows/main.yml` deploys only `api/**` to fly.io and render.com). |
| Hosting | Not in repo. `.gitignore` contains `.vercel` (`results/.gitignore:30-31`) → most likely Vercel git integration. |
| Local dev | `docker-compose.yml:33-45`: `node:20-alpine`, `npm i -g pnpm && pnpm install && pnpm run dev`, port 3000, `NEXT_PUBLIC_API_URL=http://127.0.0.1:8000/public`. |
| Docker image | None for production. |

---

## 3. Global layout & navigation

### 3.1 App shell

- `results/pages/_app.tsx`: `<Layout>` → `<main>` (Montserrat, `flex col items-center overflow-y-scroll pb-40 pt-4`, desktop `lg:pl-52 lg:pr-4 lg:pt-28 lg:pb-4`) → `<SWRConfig>` → page.
- `results/components/layout/layout.tsx`: React context (`LayoutContext`) holding `pageTitle`, `pageDescription`, `headerTitle`, `headerSubtitle`, `activeNav` and setters. Renders `<Head><title/><meta name="description"/></Head>`, `<Header>`, children, `<Nav>`.
- Every page calls the 5 setters in a `useEffect` (boilerplate duplicated in 11 pages). `headerTitle`/`headerSubtitle` are always set to `""` → the header `hgroup` is always empty (feature unused).

### 3.2 Header (`results/components/layout/header.tsx`)

- Height 96 px (`h-24`), full width, background `awt-dark-950` (slate-950) with decorative `img/lines.svg` right-aligned, white text.
- Left: AWT logo (`img/logo.svg`, white) 44 px high, links to `/`.
- Right: subtitle + title (always empty).
- Desktop (`lg`): `fixed top-0`, z-20. Mobile: scrolls with page.

### 3.3 Navigation (`results/components/layout/nav.tsx`, `navItem.tsx`)

| Label | Route | `activeNav` key | Icon | Preload on hover |
|---|---|---|---|---|
| Home | `/` | `home` | house | `/competitions/` |
| Seasons | `/seasons` | `seasons` | calendar | `/seasons/` |
| Competitions | `/competitions` | `competitions` | trophy | `/competitions/` + `/seasons/` |
| Pilots | `/pilots` | `pilots` | person | `/pilots/` |
| Teams | `/teams` | `teams` | two people | `/teams/` |
| Judges | `/judges` | `judges` | gavel | `/judges/` |
| Tricks | `/tricks` | `tricks` | star | `/tricks/` |
| Simulator | `/simulator` | `simulator` | dice | `/tricks/unique/` — **item has `className="invisible"`**: hidden but still occupies space; route reachable by URL only (`nav.tsx:64-69`). |

- Mobile: fixed **bottom** bar, white, 2 rows of 4 items (icon above label, `text-xs`), shadow.
- Desktop (`lg` ≥ 1024 px): fixed **left** sidebar 192 px wide (`lg:w-48`), full height, items right-aligned, icon + label in a row, active item = `bg-awt-dark-900` white text with left-rounded pill shape (`lg:rounded-r-none`).
- Hover (inactive): `bg-awt-accent-600` (sky-600), white text.
- No hamburger, no footer, no breadcrumb, no back button.

### 3.4 Loading / error / empty states

| Component | Look | File |
|---|---|---|
| `FetchLoading` | "Fetching data..." + animated CSS "cloud with rain" loader | `results/components/ui/fetchLoading.tsx`, CSS `results/styles/globals.css:85-174` |
| `FetchError` | "Couldn't fetch data." + same cloud with a rotated ":(" | `results/components/ui/fetchError.tsx` |
| Not found | plain `<h2>Competition not found</h2>` etc. | each page |

No 404 page (Next default), no retry button.

---

## 4. Sitemap

Endpoint column = path appended to `API_URL` (i.e. `/public/...` on the API).

| Route | File | Purpose | Endpoint(s) | Page title |
|---|---|---|---|---|
| `/` | `results/pages/index.tsx` | Home: About box, current competitions, downloads | `GET /competitions/` | `Acro World Tour \| Home` |
| `/seasons` | `results/pages/seasons/index.tsx` | Seasons by year | `GET /seasons/` | `Acro World Tour \| Seasons` |
| `/seasons/[code]` | `results/pages/seasons/[code].tsx` | Season standings | `GET /seasons/{code}` | season name |
| `/competitions` | `results/pages/competitions/index.tsx` | Competitions by year grouped by season | `GET /seasons/`, `GET /competitions/` | `Acro World Tour \| Competitions` |
| `/competitions/[code]` | `results/pages/competitions/[code].tsx` | Competition details, overall & per-run results | `GET /competitions/{code}` | competition name |
| `/pilots` | `results/pages/pilots/index.tsx` | Pilot directory | `GET /pilots/` | `Acro World Tour \| Pilots` |
| `/pilots/[civlid]/[slug]` (catch-all `[...civlid]`) | `results/pages/pilots/[...civlid].tsx` | Pilot profile & history. Only first segment used; slug = lower-cased name with spaces → `-` (cosmetic). | `GET /pilots/{civlid}` (with results) | pilot name |
| `/teams` | `results/pages/teams/index.tsx` | Synchro teams directory | `GET /teams/` | `Acro World Tour \| Teams` |
| `/teams/[teamId]/[name]` (catch-all `[...teamId]`) | `results/pages/teams/[...teamId].tsx` | Team page | `GET /teams/{id}` | team name |
| `/judges` | `results/pages/judges/index.tsx` | Judges by level | `GET /judges/` + **one `GET /pilots/{civlid}?results=false` per judge having a civlid** (photo) | `Acro World Tour \| Judges` |
| `/tricks` | `results/pages/tricks/index.tsx` | Tricks catalogue | `GET /tricks/` | `Acro World Tour \| Tricks` |
| `/simulator` | `results/pages/simulator/index.tsx` | Multi-run score simulator (hidden from nav) | `GET /tricks/unique/`, `POST /simulate/competition/solo?reset_repetitions_frequency=N` | `Acro World Tour \| Simulator` |

No query parameters are read by any page. All UI state (year selector, accordions) is local or `localStorage` — not reflected in the URL (not shareable).

`tsconfig.json:25` includes `pages/judges/[...id].tsx` which does **not** exist (removed judge page).

### 4.1 Home `/`

Sections in order:

1. **About** (`results/components/about.tsx`, loaded with `dynamic(..., {ssr:false})` because it reads `localStorage` at init).
   - Toggle button "About" + chevron; collapsed state persisted in `localStorage["home/hideAbout"]` ("1" when hidden).
   - Box `bg-awt-dark-100`, rounded, 2 paragraphs:
     - "This is the official web application of the Acro World Tour. In this app you can find everything you need to know about competition results and your favourite pilots."
     - "To stay up to date about the Acro World Tour, follow our social media down below."
   - Social icons: Instagram `https://www.instagram.com/acroworldtour`, Facebook group `https://www.facebook.com/groups/120757714620928`, YouTube `https://www.youtube.com/@acroworldtour` (`about.tsx:59-70`).
   - When collapsed, the button and icons slide sideways (translate animations).
2. **Current competitions** (`results/components/competition/currentCompetitions.tsx`) — see §5.
3. **Downloads** (`results/components/download.tsx`): single link "📑 FAI Sporting Code" → `SPORTING_CODE_MANUAL_URL` (FAI 2025 PDF), new tab, orange text in light card, `hover:invert`.

### 4.2 Seasons list `/seasons`

- Year selector in heading: "Seasons in `<select>`" / "No seasons in `<select>`". Default = current calendar year. Options = distinct `season.year` ∪ current year, descending.
- Filter `season.year === selectedYear`; sort by `year desc`, `index asc` (null → 999), `name asc`.
- Grid (`.wrapper`: flex-wrap, justify-evenly, gap 32 px) of **SeasonCard** (`results/components/season/seasonCard.tsx`):

| Field | Display |
|---|---|
| `image` (absolutised) | background, darkened (`bg-awt-dark-900/60` multiply); on hover light overlay + lift |
| `name` | h3 |
| `type` | "solo"/"synchro" (capitalised) |
| `number_of_competitions` | "N competition(s)" (hidden if 0) |
| `number_of_pilots` / `number_of_teams` | bottom-right "N pilot(s)" or "N team(s)" according to type |
| link | `/seasons/{code}` |

### 4.3 Season details `/seasons/[code]` (`results/components/season/seasonDetails.tsx`)

Layout: `<h2>{name}</h2>`, "No results at the moment." if `results` empty, then two columns (landscape) / stacked (portrait): summary (max 512 px) + results panel.

**Summary** (`results/components/season/seasonSummary.tsx`):

| Element | Data |
|---|---|
| Header image (16:9) | `season.image` via `next/image` (**not** passed through `relativeToUrl` — bug), else a strip of all competitions' images side by side |
| Country | `country` if set |
| Count | "N pilots" / "N teams" |
| Competitions list | "N Competition(s):" then each competition (sorted by `start_date` asc) as "🔹name" link to `/competitions/{code}` (plain `<a>`, full reload) |
| Pilots | solo only: unique pilots across all competitions (dedupe by civlid), sorted by FAI `rank` asc, small PilotCards |

**Results panel**:
- "Results are updated automatically." note when a category is open (misleading — no polling).
- One accordion per `results[]` category (`SeasonResults.type`, e.g. `overall`, `women`, country codes...): button "`{type} results`". Only one open at a time. Default open = `overall`. State persisted in `localStorage["season/{code}/showCategories"]` (array of booleans by index).
- Category table (`results/components/season/seasonCategoryResults.tsx`), 12-col grid: **Rank** (index+1) | **Pilot** (name, or team name + 2 pilot names for synchro; 🥇🥈🥉 for top 3) | **Score** (3 decimals). Sorted client-side by score desc (in-place).
- Clicking a row expands **per-competition breakdown** (`results/components/season/seasonOverallPilotResults.tsx`): iterates `season.competitions_results` (map competition code → ordered results), finds the pilot (by civlid) / team (by `_id`), rank = position in that list; rows sorted by competition start date: "`{1st|2nd|3rd|Nth}` at `{competition name}` 🥇" (link to competition) + score (3 decimals). Collapse chevron at bottom.

### 4.4 Competitions list `/competitions`

- Year selector "Competitions in `<select>`" (default current year). Years = all start/end years of competitions ∪ current year, desc.
- Competition kept if start **or** end year == selected year.
- Grouped by season: seasons that contain at least one filtered competition (`competition.seasons` contains `season.code`), seasons sorted `year desc, index asc, name asc`; each group `<h3>{season.name}</h3>` + cards sorted by start date asc. A competition in several seasons appears several times.
- "Off Season" group: competitions with `seasons.length === 0`, sorted start date **desc**.
- **CompetitionCard** (`results/components/competition/competitionCard.tsx`), 192 px high, max 512 px wide:

| Field | Display |
|---|---|
| `image` (absolutised) | darkened background |
| `name` | h3, top-left |
| `location` | h4 |
| dates | "`{startDay} {startMonth if ≠ endMonth} {startYear if ≠ endYear} to {endDay} {endMonth} {endYear}`" with `toLocaleString("default", {month:"long"})` → **browser locale** month names |
| count | "N pilots" (solo) / "N teams" (synchro) |
| state style | `state=="closed"` **and** start year == current year → grayscale (colour on hover). Past-year closed comps are not grayscale (inconsistent). |
| link | `/competitions/{code}` |

### 4.5 Competition details `/competitions/[code]` (`results/components/competition/competitionDetails.tsx`)

Data: `CompetitionPublicExportWithResults` (§9).

**Header**: `<h2>{name}</h2>`; "No results at the moment." when no non-empty result list.

**Summary column** (`results/components/competition/competitionSummary.tsx`), hidden in landscape when any result accordion is open:

| Element | Data |
|---|---|
| Image | `image` via `next/image` (absolutised), width 512 |
| "Official Website" | `website` link (new tab, no `rel`) |
| Location | `location` |
| Count | "N pilots" / "N teams" |
| Dates | `start_date - end_date` formatted `en-GB` "1 June 2024" |
| Pilots accordion | solo only, collapsed by default (collapse only applies in landscape!), small PilotCards sorted by FAI rank (`pilots.sort` mutates SWR data) |
| Judges accordion | small JudgeCards (each triggers a pilot fetch for photo) |

**Overall results section**:
- Title "Overall Results" + " (on-going)" when `results.final == false`.
- Text "Results are updated automatically." when an accordion is open.
- One accordion per key of `results.results` with filtering rules (`competitionDetails.tsx:115-121`):
  - keys matching `/^wac-/` are hidden;
  - key `overall` is hidden if any key matches `/^aw[tqs]-/` (i.e. when the competition belongs to an AWT/AWQ season the season-specific ranking replaces the generic one).
  - Typical keys: `overall`, `awt-2024`, `awq-2024`, `<ISO3>-<year>` (national cups), `women` (solo, only when ≥ 3 women), synchro season codes (copy of overall). Generated by `api/models/competitions.py:853-889`.
- Before display, each list is re-sorted client-side by `score desc`, ties broken by pilot/team name (with 3 `console.log` per comparison, `competitionDetails.tsx:22-36`).
- Table (`results/components/competition/competitionOverallResults.tsx` + `competitionOverallHeader.tsx`): **Rank** | **Pilot** (header label is "Pilot" even for synchro; row shows pilot name or team name + both pilot names; 🥇🥈🥉) | **Score** (3 decimals). Click a row → sub-table **Run# | Rank | Score** from `result_per_run[]` (score shown raw, unrounded — `competitionOverallRuns.tsx:17`).

**Per-run sections** (one per `results.runs_results[i]`):
- Title "run N" + " (provisional)" when `run.final == false`.
- Accordions per result type with the same filtering (note: the `overall` hiding test uses the *global* keys).
- Run table (`results/components/competition/competitionRunMain.tsx`): sorted by `final_marks.score desc` (in place). Columns **Rank | Pilot | Score**. Score = 3 decimals or "No score record"; 🔻 if `malus > 0`; ⚠️ if any warnings.
- Click a row → flight detail block:

| Block | Fields |
|---|---|
| Judge's Marks | `final_marks.judges_mark.technical`, `.choreography`, `.landing`, (+ `.synchro` for synchro) — 3 decimals |
| Technicity / Bonus / Malus | `final_marks.technicity` (3 dec), `bonus_percentage` "%" (+🔻 if malus), `malus` "%" |
| Tricks | bullet list of `tricks[].name` (capitalised) |
| Final Marks | `final_marks.technical`, `.choreography`, `.landing`, (+ `.synchro`) — 3 decimals |
| Bonus | `final_marks.bonus` (3 dec) |
| Warnings | `warnings2[]` as "⚠️ X (1pt)" then `warnings[]` as "⚠️ X (0.5pt)" (cat 2 / cat 1 warnings) |
| Notes | `notes[]` as "📝 X" |
| Close | chevron button |

- Accordion open state: single open accordion across overall+runs (`changeResults`), persisted in `localStorage["competitions/{code}/showResults"]`. Keys `overall-{type}` and `run{i}-{type}`.
- **Not displayed** although available: per-judge marks (`marks[]`), `did_not_start`, `tip_touch_bonuses`, `last_update`, `logo`, `number_of_runs`, teams list, `technical_per_trick`, trick bonuses detail, SVG standings exports.

### 4.6 Pilots list `/pilots`

- Two sections:
  - "Current AWT Pilots": `awt_years` contains current calendar year.
  - "AWQ Pilots": `rank >= 9999` **or** no `awt_years` **or** current year not in `awt_years`. (A pilot with AWT year and rank ≥ 9999 appears in both.)
- API order is kept (no sort, no search, no country filter).
- **PilotCard** (`results/components/pilot/pilotCard.tsx`): square photo (`photo`, 192 px; 96 px when `small`), country flag top-right, `name` + medal emoji if FAI rank 1-3, "FAI Rank: #N" (not in small). Link `/pilots/{civlid}/{slug}`.

### 4.7 Pilot profile `/pilots/[civlid]/[slug]` (`results/components/pilot/pilotDetails.tsx`)

| Element | Data |
|---|---|
| Big square photo | `photo_highres` or `photo`; links to `civl_link` (CIVL page) new tab; desktop: fixed on the right 5/12 width |
| Name line | `name` + medal if rank ≤ 3 + flag; "FAI Rank #`rank`" — whole line links to `civl_link` |
| AWT years | "AWT pilot in 2022, 2023" (`awt_years` sorted) |
| About | `about` text |
| Social links | `social_links[]` `{name, link}` → icon by name (facebook, instagram, tiktok, twitter, wikipedia, youtube, else globe) |
| Sponsors | `sponsors[]` `{name, link?, img}` logos 80 px (linked if `link`) |
| Competition results table | `competitions_results[]` `{competition, rank}` sorted by competition start date desc then rank: "Competition" name + medal → `/competitions/{code}`, "Rank" `#N` |
| Season results table | `seasons_results[]` `{season, rank}` sorted year desc: name + medal → `/seasons/{code}`, `#rank` (header "Season" only, missing "Rank" header) |

Note: page types the response as `Pilot` but the component expects `PilotWithResults` (API returns results by default, `api/routers/public.py:54`).

### 4.8 Teams `/teams` and `/teams/[id]/[name]`

- List "All Teams" (API order). **TeamCard** (`results/components/team/teamCard.tsx`): 192 px card, team `name`, background = the 2 pilots' photos side by side (overlay blend), footer per pilot: name + medal + flag, "FAI Rank: #N"; hover radial sky gradient. Link `/teams/{_id}/{name}` (name not slugified/encoded).
- Team page (`results/components/team/teamDetails.tsx`): `<h2>{name}</h2>` + full PilotCards of members. No results history (gap). `console.log(team)` left in page (`results/pages/teams/[...teamId].tsx:58`).

### 4.9 Judges `/judges`

- Three sections: "Senior Judges", "Certified Judges", "Trainee Judges" (`level`), each sorted by name, separated by `<hr>`.
- **JudgeCard** (`results/components/judge/judgeCard.tsx`): square photo from the linked pilot profile (only if `civlid`; fetched individually), flag (`country` alpha-3), `name`, `level`. Links to `/pilots/{civlid}/{slug}`; disabled (`pointer-events-none`) if no civlid. While each judge's pilot fetch is loading, the card renders a full-size `FetchLoading` (layout jumps); on error a full `FetchError`.

### 4.10 Tricks `/tricks` (`results/components/trick/tricksTable.tsx`)

- Two tables: "Solo and Synchro Tricks" (`solo == true`), "Synchro only Tricks" (`synchro && !solo`). Sorted by `technical_coefficient` asc.
- Columns: **Trick** (name, linked to `sample_video` with 📺 if present) | **Acronym** | **Properties** (icons) | **Bonuses** (each `name (bonus%)` + 📺 link to bonus `sample_video`, sorted by bonus asc) | **Technical Coefficient** (2 decimals).
- Property icons: ⬅️ `directions` has `left`, ➡️ `right` (**tooltips are swapped**: ⬅️ says "Right", ➡️ says "Left"), ↔️ `opposite`/`mirror`, 🔁 `repeatable`, ① `first_maneuver > 0` ("must be performed as a first trick only"), ⨂ `no_last_maneuver > 0` ("cannot be performed as a last N maneuver(s)").
- Not shown: `no_first_maneuver`, `last_maneuver`, `bonus_constraints`, `tricks_constraints`, `solo_awt`, `types`.
- `trickCard.tsx` is unused.

---

## 5. Live mode

| Question | Legacy answer |
|---|---|
| How is a "current competition" detected? | `competition.state === "open"` on the `/competitions/` list (`results/components/competition/currentCompetitions.tsx:31-33`). Dates are ignored. Heading: "Current Competition(s)" or "There are no ongoing competitions." |
| How are live runs detected? | Per run: `runs_results[i].final == false` → "(provisional)"; competition: `results.final == false` → "(on-going)". |
| Refresh / polling | **No polling at all.** No `refreshInterval`. Data only refreshes through SWR defaults: on window focus, on network reconnect, on re-mount (navigation). The UI text "Results are updated automatically." (`competitionDetails.tsx:108-112`, `seasonDetails.tsx:71-75`) is therefore only true when the user switches tab/app. |
| Server side caching | API builds a fresh in-memory `Cache()` per request (`api/routers/public.py:152-161`): each competition GET recomputes the full results from MongoDB. No HTTP cache headers from the site. |
| Live overlays | API offers `/public/live/overlay/...` and SVG standings (`api/routers/public.py:166-283,320-398`) but **the results site does not use them** (they are for broadcast tools). |

Rewrite requirement: real live updates (SSE/WebSocket or polling ~10-15 s while a competition is `open` and the tab is visible, slower otherwise), a visible "last updated" timestamp (`last_update` is in the payload), and highlight of the run in progress / newly published flights.

---

## 6. Simulator `/simulator`

Hidden from navigation (`invisible` nav item) but functional.

### 6.1 Behaviour

| Item | Detail | Source |
|---|---|---|
| Trick list | `GET /tricks/unique/` (defaults `solo=true, synchro=false`) → `UniqueTrick[]` (every trick × direction × bonus combination with its own acronym/coefficient/bonus) | `pages/simulator/index.tsx:233-237` |
| Number of runs | select 1..5 (`maxNumberOfRuns=5`, default 1) | `:43-45,268-280` |
| Repetition reset | select: `0` never, `1` every run, `2` "at runs 3 and 5" (default), `4` "at run 5" | `:283-294` |
| Per run | 10 trick slots (MUI Autocomplete, options grouped by `base_trick`, label "`ACRONYM - name`", filter: acronym prefix first, fallback acronym/name substring) + Technical / Choreography / Landing marks: `<select>` 0..10 step 0.5, default 5 (clamped 0–10) | `components/simulator/simulatorRun.tsx:29-33,210-270`, `simulatorMarkSelect.tsx` |
| Reset run | trash icon + `confirm()` → defaults | `simulatorRun.tsx:196-206` |
| Reset all | trash icon next to title + `confirm()` → deletes `localStorage` keys of the simulation and reloads page | `pages/simulator/index.tsx:148-155,249-256` |
| Trigger | "Click to get result !" button (becomes "Final Score: X.XXX"), plus double-dice icon buttons (×3). Also auto-recomputes when reset frequency changes. Not recomputed automatically on trick/mark change. | `:64-137,296-336` |
| Persistence | `localStorage`: `simulator/competitionName` (always "default"), `simulator/competition/{name}/numberOfRuns`, `.../resetRepetitionsFrequency`, `.../run/{i}/tricks|technical|choreography|landing` | |

### 6.2 Request

`POST {API_URL}/simulate/competition/solo?reset_repetitions_frequency={0|1|2|4}` with body = array of 5 `FlightNew` (always 5, unused runs sent as empty):

```json
[{ "tricks": ["<unique trick name>", ...], "marks": [{"judge":"simulator","technical":5,"choreography":5,"landing":5}], "did_not_start": false, "warnings": [] }, ...]
```

Response: `Flight[]` (one per run) with `final_marks` and resolved `tricks` (UniqueTrick with `bonuses`).

### 6.3 Scoring logic

**No scoring formula is duplicated client-side.** All computation is server-side (`api/controllers/scores.py:75-136`): the API builds an in-memory solo competition (2 fake pilots, judges "simulator1/2", `CompetitionConfig()` defaults, repeatable tricks from DB), creates one run per flight with repetition reset policy `all` when `i>0 && freq>0 && i % freq == 0`, saves the flight without DB writes, then returns `runs_results[i].results['overall'][0]`. Scoring formulas belong to the API spec, not this one.

Client-side only:
- Final score = **sum of `final_marks.score`** over runs having ≥ 1 trick (`pages/simulator/index.tsx:119-134`).
- Display per run: Simulated Score (3 dec); Technicity (3 dec), Bonus % (1 dec), Malus % (1 dec); Final marks: technicity, choreography, landing, bonus (3 dec) — note "Technicity" is shown where "Technical" final mark is expected; "3 best tricks" = top 3 tricks by `technical_coefficient`; bonus tricks columns: **Twisted** (bonus name contains `twist`, `hardcore`, `cab slide`, max 5 shown), **Reversed** (`reverse`, max 3), **Flipped** (`flip`, max 2) — these caps mirror the sporting-code bonus limits but are display-only; warnings2 "(1pt)" / warnings "(0.5pt)"; notes.
- Errors: non-2xx (e.g. 400 "Some tricks are unknown...") only `console.log`ged — no user feedback.

---

## 7. Downloads, about, social, SEO, PWA, analytics

| Topic | Legacy | Source |
|---|---|---|
| Downloads | Only link to FAI Sporting Code Section 7B 2025 (external). Local PDFs `public/files/sporting_code_s7_b_-_aerobatics_2022.pdf` and `..._2023_v1.pdf` are shipped but **not linked**. No CSV/PDF/SVG export of results on the site. | `results/components/download.tsx`, `results/constants.ts:1` |
| API exports usable by rewrite | `GET /public/competitions/{id}/standings/{result_type}/svg`, `/run/{run}/svg`, `GET /public/seasons/{id}/standings/svg` (`?download=true`) — not used by legacy site | `api/routers/public.py:166-283` |
| About | Collapsible box on home (text in §4.1) | `results/components/about.tsx` |
| Social | Instagram, Facebook group, YouTube (home). Pilot social links. | |
| SEO/meta | Only `<title>` and `<meta name="description">`, set client-side → absent from initial HTML. No Open Graph/Twitter cards, no canonical, no sitemap.xml, no robots.txt, no structured data. | `results/components/layout/layout.tsx:35-38` |
| Favicons | `public/favicon.ico`, `favicon-16x16.png`, `favicon-32x32.png`, `apple-touch-icon.png`, `android-chrome-192/256.png`, `mstile-150x150.png`, `safari-pinned-tab.svg` — **none linked** in `<Head>` (only `/favicon.ico` works by browser convention). | `results/public/` |
| PWA manifest | `public/site.webmanifest` with empty `name`/`short_name`, white theme/background, `display: standalone` — **not linked**; no service worker. | `results/public/site.webmanifest` |
| Analytics | None (no GA, Plausible, Matomo, Vercel Analytics). | grep |
| Security headers | only `X-Content-Type-Options: nosniff` | `results/next.config.js` |

### 7.1 Assets inventory (binary, not read)

| Path | Used? |
|---|---|
| `results/img/logo.svg` (white AWT logo) | yes, header |
| `results/img/lines.svg` (decorative lines) | yes, header background |
| `results/img/arrow.svg`, `back.svg`, `competitions-icon.svg`, `home-icon.svg`, `pilots-icon.svg`, `facebook.svg`, `instagram.svg`, `youtube.svg`, `results.svg`, `red-card.svg`, `yellow-card.svg` | no (icons are inline SVG in `components/ui/icons.tsx`) |
| `results/img/competition-small-img.jpg` (637×387), `results/img/pilot.jpg` (286×286) | no |
| `results/public/pilot.jpg` (286×286), `results/public/martin-wyall-RYAUYkia-cI-unsplash.jpg` (2400×3598, Unsplash) | no |
| `results/public/files/sporting_code_s7_b_-_aerobatics_2022.pdf`, `sporting_code_s7_b_aerobatics_2023_v1.pdf` | no |
| favicons / manifest (see above) | not linked |

---

## 8. Visual design notes

| Aspect | Legacy |
|---|---|
| Palette | Tailwind aliases: `awt-dark` = **slate**, `awt-accent` = **sky** (`results/tailwind.config.js:12-14`). Body text slate-700; header slate-950; active nav slate-900; panels slate-50 with `shadow-inner`; table headers slate-900 / slate-700 / slate-500 with white text; hover sky-600 white. Downloads link orange-500. White background everywhere (no dark mode). |
| Typography | Montserrat. Headings centred, extra-bold, `leading-loose`, subtle drop-shadow; small sizes (h1 18→20 px, h2 16→18 px, body `p` 12→14 px) (`results/styles/globals.css:21-60`). |
| Motion | **Global** `* { transition-all 300ms ease-in-out }` (`globals.css:6-7`) — every property animates (cards lift `-translate-y-2` on hover, chevrons rotate, accordions). |
| Cards | rounded-xl (12 px), shadow slate-400, image backgrounds with multiply/screen blend modes; medals 🥇🥈🥉 emojis for top 3; flags via CSS sprites. |
| Tables | CSS grid 12 columns built with headings/paragraphs (not semantic `<table>` except on pilot page), 1 px borders. |
| Responsive | Breakpoint `lg` (1024 px) switches bottom tab bar ↔ left sidebar and fixed header; `portrait:`/`landscape:` variants used to stack summary/results and to hide summary when results are open (landscape only). Mobile: 160 px bottom padding for the tab bar. |
| Icons | Inline Font Awesome-like SVG paths (`results/components/ui/icons.tsx`). |
| Look & feel | Sober, light, "app-like" (tab bar), emoji-heavy for status (🔻 ⚠️ 📝 📺 🔁). Hidden content via accordions (everything collapsed by default except season `overall`). |

---

## 9. Public data contract (`results/types.d.ts` summarised)

`types.d.ts` is the full OpenAPI dump (4342 lines, includes private endpoints). Only these schemas are used by the site:

### 9.1 Endpoints used

| Method & path (under `/public`) | Response schema | Query params |
|---|---|---|
| `GET /competitions/` | `CompetitionPublicExport[]` (page types it `CompetitionPublicExportWithResults[]` on home — wrong but harmless) | — |
| `GET /competitions/{code}` | `CompetitionPublicExportWithResults` | — (`{id}` accepts code) |
| `GET /seasons/` | `SeasonExportLight[]` | `deleted=false` |
| `GET /seasons/{code}` | `SeasonPublicExport` | `deleted=false` |
| `GET /pilots/` | `Pilot[]` | — |
| `GET /pilots/{civlid}` | `PilotWithResults` (or `Pilot` when `results=false`) | `results=true` |
| `GET /teams/`, `GET /teams/{id}` | `TeamExport[]`, `TeamExport` | — |
| `GET /judges/` | `Judge[]` | — |
| `GET /tricks/` | `Trick[]` | `repeatable` |
| `GET /tricks/unique/` | `UniqueTrick[]` | `solo=true`, `synchro=false` |
| `POST /simulate/competition/{solo|synchro}` | `Flight[]` | `reset_repetitions_frequency` (int) ; body `FlightNew[]` |
| `GET /files/{id}` | binary (images referenced as relative `/files/{id}` → absolutised with `API_URL`) | `download` |

### 9.2 Schemas

| Schema | Fields |
|---|---|
| `CompetitionType` | `"solo" \| "synchro"` |
| `CompetitionState` | `"init" \| "open" \| "closed"` |
| `JudgeLevel` | `"trainee" \| "certified" \| "senior"` |
| `GenderEnum` | `"man" \| "woman" \| "none"` |
| `CompetitionPublicExport` | `_id, name, code, start_date, end_date, location, published, type, state, number_of_pilots, number_of_teams, number_of_judges, number_of_runs, image?, logo?, website?, seasons: string[] (season codes), last_update?` |
| `CompetitionPublicExportWithResults` | above + `results: CompetitionResultsExport, pilots: Pilot[], teams: TeamExport[], judges: Judge[]` |
| `CompetitionResultsExport` | `final: bool, type, results: {[resultType]: CompetitionPilotResultsExport[]}, runs_results: RunResultsExport[]` |
| `CompetitionPilotResultsExport` | `pilot?: Pilot, team?: TeamExport, result_per_run: RunResultSummary[], score` |
| `RunResultSummary` | `rank, score` |
| `RunResultsExport` | `final: bool, type, results: {[resultType]: FlightExport[]}, last_update?` |
| `FlightExport` | `pilot?, team?, tricks: UniqueTrick[], marks: JudgeMarkExport[], did_not_start?, final_marks?: FinalMarkExport, published?, warnings: string[], warnings2?: string[], tip_touch_bonuses?, last_update?` |
| `FinalMarkExport` / `FinalMark` | `judges_mark, technicity, bonus_percentage, technical, choreography, landing, synchro, bonus, score, warnings[], warnings2?[], malus, notes[], mark_type?` |
| `JudgeMarkExport` / `JudgeMark` | `judge (Judge obj / string), technical?, technical_per_trick?[], choreography?, landing?, synchro?` |
| `Pilot` | `_id, civlid, name, civl_link, country (ISO alpha-3), about, social_links: Link[], sponsors: Sponsor[], photo, photo_highres?, background_picture, last_update?, rank (FAI rank; ≥ 9999 = unranked), gender?, awt_years?: number[]` |
| `PilotWithResults` | `Pilot` + `competitions_results?: {competition: CompetitionPublicExport, rank}[]`, `seasons_results?: {season: Season, rank}[]` |
| `Link` / `Sponsor` | `{name, link}` / `{name, link?, img}` |
| `TeamExport` | `_id, name, pilots: Pilot[]` (2) |
| `Judge` | `_id?, name, country, level, civlid?, deleted?` |
| `SeasonExportLight` | `_id, name, code, year, image?, country?, index?, type, number_of_pilots, number_of_teams, number_of_competitions` |
| `SeasonPublicExport` | `_id, name, code, year, image?, country?, index?, type, number_of_pilots, number_of_teams, competitions: CompetitionPublicExportWithResults[], results: SeasonResults[], competitions_results: {[competitionCode]: CompetitionPilotResultsExport[]}` |
| `SeasonResults` | `type (category), results: {pilot?, team?, score}[]` |
| `Trick` | `_id?, name, acronym, types?, solo, solo_awt?, synchro, directions: string[], technical_coefficient, bonuses: Bonus[], bonus_constraints?, tricks_constraints?, first_maneuver?, no_first_maneuver?, last_maneuver?, no_last_maneuver?, tricks?: UniqueTrick[], repeatable?, deleted?, sample_video?` |
| `Bonus` | `name, bonus (%), sample_video?` |
| `UniqueTrick` | `name, acronym, technical_coefficient, bonus, bonus_types[], base_trick, uniqueness[], bonuses?, synchro?, solo?, solo_awt?, types?, technical_mark?, technical_marks?` |
| `Flight` | `pilot (int), team?, tricks: UniqueTrick[], marks: JudgeMark[], did_not_start?, final_marks?: FinalMark, published?, warnings[], warnings2?[], tip_touch_bonuses?, last_update?` |
| `FlightNew` | `tricks: string[] (unique trick names), marks: JudgeMark[], did_not_start?, warnings?, warnings2?, tip_touch_bonuses?` |

Note: `SeasonPublicExport.competitions` embeds every competition **with full results** → very heavy payload (see §10).

---

## 10. Performance issues, bugs, UX pain points

### 10.1 Performance

| # | Issue | Where |
|---|---|---|
| P1 | No SSR/SSG: blank page + loader on every navigation; nothing indexable; slow on mobile. | all pages |
| P2 | `/seasons/{code}` returns all competitions with full results + `competitions_results` (duplicate data). Heavy JSON. | `api/routers/public.py:242-256` |
| P3 | API recomputes full results per request (no cache), multiplied by focus revalidation. | `api/routers/public.py:147-161` |
| P4 | Judges page: N+1 requests (one `/pilots/{civlid}` per judge), also in each competition summary. | `results/components/judge/judgeCard.tsx:24-30` |
| P5 | `/pilots/` returns all pilots ever (no pagination) with about/sponsors. | |
| P6 | MUI + emotion shipped just for the hidden simulator page (bundle size). | |
| P7 | Global `* { transition-all }` makes every layout change animate (jank on big tables). | `results/styles/globals.css:6` |
| P8 | Background images as CSS (not optimised); `next/image` used inconsistently. | cards |
| P9 | `console.log` in sort comparator (3 logs per comparison) on competition page. | `competitionDetails.tsx:28-31` |

### 10.2 Bugs

| # | Bug | Where |
|---|---|---|
| B1 | "Results are updated automatically." but no polling. | `competitionDetails.tsx:108`, `seasonDetails.tsx:71` |
| B2 | `changeResults` builds new state from `Object.values` (booleans) instead of keys → junk `"true"/"false"` keys persisted in localStorage. Works by accident. | `competitionDetails.tsx:56-62` |
| B3 | Persisted accordion state indexed by position/keys survives data changes (new run / categories) → wrong panel open; season state is an index array. | `seasonDetails.tsx:25-29` |
| B4 | Season image not passed through `relativeToUrl` → relative `/files/..` breaks. | `seasonSummary.tsx:45-51` |
| B5 | Trick direction tooltips swapped (⬅️ "Right", ➡️ "Left"). | `tricksTable.tsx:43-51` |
| B6 | Invalid HTML: `<p>` inside `<p>` (bonuses), fragments without `key` in maps, `py-2u` typo, `arai-label` typo (accessibility label missing), duplicated `col-span-1 col-span-full`. | `tricksTable.tsx:29,87-102`, `simulatorRefreshButton.tsx:12`, `simulatorRun.tsx:414` |
| B7 | In-place sorting of SWR data (`pilots.sort`, `results?.sort`, `competitions.sort`) mutates cache. | `competitionSummary.tsx:38`, `competitionRunMain.tsx:30`, `seasonSummary.tsx:33` |
| B8 | Ranks computed from array index → ties get different ranks; season breakdown rank depends on API ordering. | `competitionOverallResults.tsx:28`, `seasonOverallPilotResults.tsx:39-46` |
| B9 | Pilots page: pilot can appear in both AWT and AWQ lists (rank ≥ 9999 and AWT year). "Current" uses browser year, not season. | `pages/pilots/index.tsx:46-55` |
| B10 | Competition card grayscale only for closed competitions of the **current** year. | `competitionCard.tsx:55-57` |
| B11 | Month names use browser locale (`"default"`) while summary uses `en-GB` → mixed languages. Dates parsed as UTC midnight → may shift a day in negative-offset zones. | `competitionCard.tsx:31-38` |
| B12 | Synchro tables header says "Pilot"; team pilots accessed as `pilots[0]`/`pilots[1]` without guard. | `competitionOverallResults.tsx:17`, `competitionOverallHeader.tsx:37-38` |
| B13 | Run "overall" filtering checks global keys, not run keys. | `competitionDetails.tsx:176-181` |
| B14 | Pilots/Judges accordions in competition summary only collapse in landscape (`landscape:hidden`); in portrait always expanded although chevron says collapsed. | `competitionPilots.tsx:31-34` |
| B15 | Simulator: errors only logged; "Technicity" label for technical final mark; `cleanCompetition` reloads inside the loop; `flew` typo class; nav item invisible. | `pages/simulator/index.tsx:108-116,148-155` |
| B16 | Team URL uses raw team name (spaces/accents unencoded). | `teamCard.tsx:20` |
| B17 | Internal links sometimes `<a href>` (full reload) instead of `Link`. | `pilotDetails.tsx:153,181`, `seasonSummary.tsx:96` |
| B18 | `Official Website` link `target=_blank` without `rel`. | `competitionSummary.tsx:54` |
| B19 | Favicons/manifest not linked; manifest name empty. | `pages/_document.tsx` |
| B20 | Sporting code URL hard-coded to 2025; local PDFs outdated & unused. | `constants.ts:1` |
| B21 | `tsconfig.json` references non-existent `pages/judges/[...id].tsx`; `NEXT_CACHE_DURATION` unused. | |

### 10.3 UX pain points

- Everything hidden behind accordions; only one open at a time; no deep link to a run/category/flight (state not in URL).
- No search (pilots, competitions), no filters (country, gender), no sort.
- No per-judge marks breakdown, no DNS display, no trick-by-trick technical marks, no "last updated".
- No live indicator (which run is in progress, who is flying next / starting order).
- Team page has no results history; judges have no page of their own.
- No footer/legal/contact; no link to the main site `acroworldtour.com`.
- No dark mode, small font sizes (12 px body on mobile), centred headings everywhere.
- Year selector defaults to calendar year → empty page in January before seasons exist.
- Loading states replace the whole page (no skeletons); per-judge loaders cause layout shifts.

---

## 11. Must-have features for the rewrite (checklist)

### Data & rendering
- [ ] Server-render (SSR/ISR) every public page with real HTML content and per-page `<title>`, description, Open Graph/Twitter image (competition/season image, pilot photo).
- [ ] Short revalidation (ISR/cache) for closed competitions; live client updates for `open` ones.
- [ ] Single configurable API base URL (`API_URL` env, server and client), image domain whitelist / proxy for `/files/{id}`.
- [ ] Typed API client generated from OpenAPI (public endpoints only).
- [ ] Lighter season endpoint (no embedded full competitions) and no N+1 judge photo fetches (embed photo in judge payload).

### Live
- [ ] Detect current competitions (`state == open`) on home with a prominent "LIVE" badge.
- [ ] Auto-refresh results of open competitions (SSE/WebSocket or polling 10–15 s when tab visible), show `last_update`, mark runs "provisional" and competition "on-going".
- [ ] Highlight newly published flights; show current run.

### Pages (parity)
- [ ] Home: about text, social links (Instagram, Facebook group, YouTube), current competitions, downloads (FAI Sporting Code S7B, configurable per year).
- [ ] Seasons list by year (sorted year desc / index / name) with cards (image, name, type, #competitions, #pilots|#teams).
- [ ] Season page: summary (image or competition mosaic, country, counts, competitions list, pilots), standings per category (`overall`, `women`, national…), expandable per-competition breakdown with ordinal rank and score.
- [ ] Competitions list by year grouped by season + "Off Season", cards with dates/location/count/state.
- [ ] Competition page: summary (image, website, location, dates, counts, pilots, judges), overall standings per result type with filtering rules (hide `wac-*`; hide `overall` when `aw[tqs]-*` exists), per-run breakdown (rank/score), per-run tables with flight detail (judges' marks, technicity, bonus %, malus %, tricks, final marks, bonus, warnings cat1 0.5 pt / cat2 1 pt, notes), solo and synchro variants (synchro mark).
- [ ] Pilots list split "Current AWT pilots" / "AWQ pilots" (fix overlap), cards with photo, flag, name, FAI rank, medals.
- [ ] Pilot page: high-res photo, CIVL link, flag, FAI rank, AWT years, about, social links, sponsors, competition history (rank, link), season history (rank, link).
- [ ] Teams list and team page (add results history).
- [ ] Judges grouped by level (senior, certified, trainee) with photo, flag, link to pilot profile.
- [ ] Tricks catalogue: solo(+synchro) and synchro-only tables, acronym, properties (directions, repeatable, first/last maneuver constraints), bonuses with %, technical coefficient, video links.
- [ ] Simulator (decide whether public): up to 5 runs × 10 tricks with searchable acronym picker grouped by base trick, 3 marks 0–10 step 0.5, repetition reset policy (never / every run / runs 3 & 5 / run 5), server-side simulation via `POST /public/simulate/competition/{type}`, per-run and total score, bonus breakdown, warnings/notes, local persistence, clear error messages; optionally synchro.

### Improvements
- [ ] Shareable URLs for state: `?year=`, `?category=`, `#run-2`, flight anchors.
- [ ] Tie-aware ranking (rank from API, not array index).
- [ ] Search / filters (name, country, gender, AWT/AWQ).
- [ ] Downloads: SVG/PNG standings (API already has `/standings/.../svg`), optional CSV/PDF results.
- [ ] Accessible semantic tables, keyboard navigation, correct tooltips, i18n-ready dates (single locale), dark mode optional.
- [ ] Linked favicons + complete web manifest (name "Acro World Tour"), robots.txt, sitemap.xml.
- [ ] Privacy-friendly analytics (optional), security headers (CSP, HSTS, nosniff).
- [ ] Footer with link to acroworldtour.com, contact, social links.
- [ ] Keep the visual identity: slate (dark) + sky (accent) palette, Montserrat, dark header with logo and line pattern, bottom tab bar on mobile / left sidebar on desktop, image cards with dark overlay, medal emojis for podium.
- [ ] CI/CD for the site (build, lint, type-check, deploy) — none exists today.
