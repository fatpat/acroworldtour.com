# 07 — Institutional website (`www/`)

> Scope: the static marketing / institutional site served at `www.acroworldtour.com` (apex `acroworldtour.com` assumed to point there too).
> Source analysed: `www/` in the legacy repo (read-only). All paths below are relative to the legacy repo root.

## 1. Summary

| Aspect | Today |
|---|---|
| Technology | Hand-written static HTML5 + one CSS file + 5 lines of vanilla JS + Flickity carousel. No build step, no templating, no CMS. |
| Pages | 37 HTML files (5 top-level pages, 9 event pages, 9 news articles, 14 static result pages, 1 × 404). |
| Authoring | Content edited directly in HTML by Luke de Weert (≈54 of the 58 commits on `www/` are by him; `git log -- www`). Nav, footer, `<head>` are copy-pasted in every file. |
| Hosting | Vercel (`www/vercel.json:2` → `cleanUrls`). An Apache `www/.htaccess` duplicates the same clean-URL rule (legacy/alternative host). Local dev: nginx container (`docker-compose.yml:47-52`, `utils/nginx.vhost.www.conf`). |
| Dynamic data | None. Results, events, pilots are all hard-coded copies of data that also exists in the API (`api.acroworldtour.com`) and the results site (`results.acroworldtour.com`). |
| Analytics | Google Analytics 4 (`gtag.js`, measurement id `G-LYV0V2T0BQ`) in the `<head>` of all 37 pages. No cookie banner. |
| Last content update | 2025-06-25 (`git log -- www`: "added brienz"). Home page still shows 2024 standings partly (see §9). |

## 2. Sitemap

URLs are "clean" (no `.html`) thanks to `www/vercel.json:2` / `www/.htaccess:1-4` / `utils/nginx.vhost.www.conf:9` (`try_files $uri $uri.html`).

### 2.1 Top-level pages

| URL | File | Purpose | In nav? | Notes |
|---|---|---|---|---|
| `/` | `www/index.html` | Home: hero, featured YouTube video, "Hot News" (3 items), "Upcoming Events" (4 cards), "Latest Results" (AWT + AWQ top-5 cards), "Discover Our Pilots" carousel, "Learn the rules" (Sporting Code PDF download), footer. | Home | Hero background `assets/img/header.jpg` (850 KB). |
| `/events` | `www/events.html` | Season calendar (6 cards for 2025) + a legend of competition categories (`events__cat-list`, purely decorative, no filtering JS). | Events | Cards are `<div href=…>` — **not clickable** (`www/events.html:82,99,…`). |
| `/pilots` | `www/pilots.html` | Two Flickity carousels: "Acro World Tour" (`pilots.html:76`) and "Acro World Qualifier" (`pilots.html:179`) — 22 pilot cards, each linking to the pilot's CIVL profile (`https://civlcomps.org/pilot/<civlid>`). | Pilots | Photos in `assets/img/athletes/`. |
| `/results` | `www/results.html` | Archive index of the 2022–2023 static result pages, grouped by year. | **No** (orphan) | Nav "Results" goes to `https://results.acroworldtour.com`. Links to missing `/result/2023/sonchaux` (broken). |
| `/404` | `www/404.html` | Not-found page (header "404 not found" + footer). | — | Wired via `www/.htaccess:6` (`ErrorDocument 404 /404`). Vercel serves `404.html` automatically. |
| external | — | `https://results.acroworldtour.com` (Next.js results app). | Results (opens new tab) | Also "View All" links on home → `/seasons`, `/seasons/awt-2024`, `/seasons/awq-2024`. |

### 2.2 Event detail pages (`/event/<slug>`)

None of these is reachable by clicking (the cards linking to them are `<div href>`), and their content is from 2024 while the listings show 2025 dates.

| URL | Title | Date shown | Location | Categories | Discipline | Official link |
|---|---|---|---|---|---|---|
| `/event/acromax` | Acromax | Aug 9–15 2024 | Lago dei Tre Comuni, Italy | AWT, AWQ, ITC | Solo + Synchro | vololiberofriuli.it |
| `/event/kings-of-the-box` | Kings of the Box | Jun 14–16 2024 | Allevard, France | AWT | Solo | civlcomps.org (commented out) |
| `/event/nest-games` | Air et Aventure's Nest Games | Jun 8–9 2024 | St Hilaire du Touvet, France | AWT | Solo | civlcomps.org (commented out) |
| `/event/roqacro` | Roq'Acro | Nov 3–5 2023 | Roquebrune, France | FRC | Solo | — |
| `/event/sonchaux-acroshow` | Sonchaux Acroshow | Aug 16–18 2024 | Villeneuve, Switzerland | AWT, AWQ, SAT, SWC | Solo + Synchro | acroshow.ch |
| `/event/swiss-acro-tour1` | Swiss Acro Tour | Jul 25–28 2024 | Villeneuve, Switzerland | SAT | — | — |
| `/event/swiss-acro-tour2` | Swiss Acro Tour | Jul 25–28 2024 | Brienz, Switzerland | AWT, AWQ, SAT | Solo + Synchro | swissleague.ch |
| `/event/world-acro-cup-superfinal` | World Acro Cup Superfinal | Sep 1–5 2024 | Oludeniz, Turkey | AWT, AWQ | Solo + Synchro | worldacrocup.com |
| `/event/worldchampionships` | World Championships | Sep 6–15 2024 | Oludeniz, Turkey | WC | Solo + Synchro | worldacrocup.com |

Listing cards re-use detail slugs for unrelated events: "Saisies Acro Masters" and "Nest Games" both → `/event/nest-games`; "New School" and "Swiss Acro Tour (Brienz)" both → `/event/swiss-acro-tour2` (`www/events.html`, `www/index.html:151,165`).

Category acronyms used: AWT (Acro World Tour), AWQ (Acro World Qualifier), WC (World Championships), SAT (Swiss Acro Tour), SWC (Swiss Championships), FRC (French Championships), ITC (Italian Championships).

### 2.3 News articles (`/news/<slug>`)

| URL | Headline (h1) | Images | Embedded result card | Linked from |
|---|---|---|---|---|
| `/news/world-acro-cup` | Luke de Weert is the new Acro World Tour Champion! | 10 | no | home (29 Sep 2023) |
| `/news/sonchaux` | Championship leader Luke de Weert wins the AcroShow | 6 | no | home (22 Aug 2023) |
| `/news/acromax` | Théo de Blic redeems himself and wins AcroMax. | 6 | no | home (15 Aug 2023) |
| `/news/brienz-results` | Florian Landreau wins the first acro world qualifier. | 7 | 1 (subtitle wrongly says "Kings of the Box 2023") | orphan |
| `/news/kings-of-the-box-results` | Dutch pilot wins Acro World Cup for the first time in history | 8 | 1 | orphan |
| `/news/nest-games-results` | Nest Games AWT 01 results | 6 | 1 | orphan |
| `/news/the-awt-2022-overall-standings` | The AWT 2022 Overall Standings | 12 | 2 | orphan |
| `/news/the-awt-2023-is-about-to-start` | The AWT 2023 is about to start! | 2 | 0 | orphan |
| `/news/theo-de-blic-and-luke-the-weert-are-out-of-season` | Theo de Blic and Luke de Weert are out of the season! | 4 | 0 | orphan |

There is **no news index page**; the publication date only exists on the home teaser, not on the article itself.

Article layout (`.default-page`): optional lead image(s) `.default-page__image`, rich text `.default-page__text` (`<p>`, `<strong>`), images interleaved between paragraphs, optional `.results__card` top-5 ranking with a "View All" button.

### 2.4 Static result pages (`/result/<year>/<slug>`)

Hand-typed ranking tables (position, flag, pilot, points) — a frozen copy of results that pre-date or duplicate the API.

| Year | Pages | Rows |
|---|---|---|
| 2022 | `awt-overall-standings`, `awq-overall-standings`, `world-acro-cup`, `synchro-llogora-airgames`, `awq-llogora-airgames`, `awt-llogora-airgames`, `awq-lery-poses`, `awt-lery-poses`, `synchro-lery-poses`, `french-championships` | 5–25 each |
| 2023 | `nest-games`, `kings-of-the-box`, `swiss-championships`, `acromax` (2 cards: AWT stop 03 + AWQ stop 02) | 11–26 each |

Results from late 2023 onwards only exist in `results.acroworldtour.com` (API-driven). The 2022 data probably exists **only** on these pages and must be migrated (see §10).

## 3. Shared layout

Every page repeats (copy-paste, no includes):

| Block | Markup | Content |
|---|---|---|
| `<head>` | lines 1–40 of each file | GA4 snippet, charset/viewport, `<title>`, legacy meta (`Generator`, `author`, `copyright`, `reply-to: info[at]acroworldtour.com`, `Keywords`, `Description`), favicon set, `style.css` (+ `flickity.min.css` on pages with carousels). |
| Nav | `nav.nav` | Logo (`/assets/img/logo.svg`) → `/`; items Home, Events, Pilots, Results (external, `target=_blank`). Active item set by hard-coded class `nav__link--active`. Each item has an SVG icon via `::before` (`home-icon.svg`, `events-icon.svg`, `pilots-icon.svg`, `acropyx-icon.svg`; `results-icon.svg` defined but unused). |
| Header | `header.header` | Home: full-height hero (`header.jpg`, 50vh / 80vh ≥800px) with pre-title, h1, subtitle, social icons. Other pages: `header--small` with only an h1. |
| Footer | `footer.footer` | "© Copyright <year> Acro World Tour. All Rights Reserved." (year replaced by JS) + Instagram / Facebook group / YouTube icons. |
| Scripts | end of body | `/assets/js/main.js` everywhere; `/assets/js/flickity.pkgd.min.js` on `index`, `pilots`, `results`. |

Social links (hard-coded in nav hero + footer): `https://instagram.com/acroworldtour/`, `https://www.facebook.com/groups/120757714620928`, `https://www.youtube.com/@acroworldtour`.

### Navigation behaviour (CSS only, `www/assets/css/style.css:200-300`)
- Mobile (<800px): logo bar at top; the menu is a **fixed bottom tab bar** (rounded, dark, icons above labels).
- Desktop (≥800px): fixed top bar, max-width 1400px, centred, with angled side decorations (`nav-angle.svg` via `::before/::after`).
- No hamburger / JS menu (`menu.svg` exists but is unused).

## 4. JavaScript behaviour

| File | Behaviour |
|---|---|
| `www/assets/js/main.js:1-5` | Replaces the content of `.js-current-year` with the current year (footer copyright). That's all. |
| `www/assets/js/flickity.pkgd.min.js` + `assets/css/flickity.min.css` | Flickity (Metafizzy, vendored, minified) — initialised declaratively through `data-flickity='{ "cellAlign": "left", "contain": true, "draggable": true, "prevNextButtons": false }'` on `.main-carousel` (`www/index.html:354-355`, `www/pilots.html`). Draggable horizontal pilot carousel with page dots. Note: Flickity's GPL/commercial licence should be checked for a rewrite. |
| GA4 inline snippet | `gtag('config','G-LYV0V2T0BQ')` on every page. |
| YouTube iframe | `www/index.html:94` — embedded video "Acro World Tour 2024 \| Kings of the Box". |

No other interactivity: the category list on `/events` is not a filter; event cards are not links.

## 5. Visual identity

| Token | Value | Source |
|---|---|---|
| Base / dark | `#171717` (nav, text) | `style.css:52-53` |
| Primary (orange) | `#FB8B24`, hover `#dc7a1f` | `style.css:54,57`; also `::selection` |
| Secondary grey | `#bababa` | `style.css:55` |
| Backgrounds | `#f6f6f6`, `#F8F8F8`, `#F5F5F5`, `#e9e9e9`, white | `style.css` |
| Overlay / shadow | `rgba(150,170,180,.5)`, `rgba(12,27,65,.6)` | `style.css` |
| Radius | `--border-radius: 20px` | `style.css:56` |
| Typeface | **Exo** (Google font, self-hosted woff/woff2: 400, 500, 600, 700, 900; `font-display: swap`) | `style.css:1-49`, `www/assets/fonts/` |
| Nav labels | 12px, 600, uppercase | `style.css` `.nav__link` |
| Section titles | 48px ≥800px | `style.css:178-186` |
| Breakpoints | 650, 680, 700, 800, 900, 1000, 1100, 1200 px (mobile-first `min-width`) | `style.css` |
| Content width | `.wrapper` max 1440px; nav max 1400px | `style.css:119-121,266` |
| Logo | `www/assets/img/logo.svg` — 129×57, white paths on transparent (designed for the dark nav). No dark-on-light variant. | |
| Favicons | `favicon.ico`, 16/32 PNG, `apple-touch-icon.png`, `android-chrome-192/512`, `mstile-150x150.png`, `safari-pinned-tab.svg` (mask colour `#5bbad5`), `site.webmanifest` (name/short_name **empty**, theme `#ffffff`), `browserconfig.xml`. | root of `www/` |

The results site uses Tailwind with an `awt-dark` alias (`results/README.md`), the manager uses MUI with a different theme — there is no shared design system across the three front-ends.

## 6. Assets

| Folder | Content | Size |
|---|---|---|
| `www/assets/img/news/` | 30 article photos, original camera files (`_DSC5778.jpg`, `IMG_2342.jpeg`, …) | **46 MB** — unoptimised, no responsive variants |
| `www/assets/img/events/` | 10 event cover photos | 5.9 MB |
| `www/assets/img/athletes/` | 16 pilot portraits (`<firstname><lastname>.jpg`) | 2.8 MB |
| `www/assets/img/flags/` | 24 country flags PNG named by 3-letter code (`fra.png`, `che.png`, `blk.png` for "black/neutral", `chi.png` for Chile — non-ISO) | 404 KB |
| `www/assets/img/*.svg` | logo, nav icons, social icons, arrow, calendar | small |
| `www/assets/files/` | FAI Sporting Code Section 7B Aerobatics PDFs 2023, 2024, 2025 (home links 2025) | 1.5 MB |
| `www/assets/fonts/` | Exo woff/woff2 | 120 KB |

Images are placed either as `<img>` (no `alt`, no `width/height`, no `loading=lazy`) or as inline `style="background-image:url(…)"` on `<i>` elements (not accessible, not lazy-loaded). Broken asset: `/assets/img/flags/czew.png` referenced in `www/result/2023/acromax.html`.

## 7. SEO / meta

| Item | State |
|---|---|
| `<title>` | Present and unique per page (`"<Page> \| Acro World Tour"`). |
| `meta description` | Same generic text on all pages ("The world's best pilots fighting for the most prestigious title"). Uses non-standard capitalised `name="Description"`. |
| `meta keywords`, `Generator`, `copyright`, `reply-to` | Obsolete, ignored by search engines. |
| Open Graph / Twitter cards | **None** on any page — shared links on social media get no image/title preview. |
| Canonical URL | None. |
| `sitemap.xml` / `robots.txt` | None. |
| Structured data (JSON-LD `SportsEvent`, `NewsArticle`) | None. |
| `lang` | `en` on all pages. Single language. |
| Accessibility | Social links have visually-hidden labels (good); images lack `alt`; invalid HTML (`<div …>` closed by `</a>` in every ranking row, nested `.carousel-cell` at `www/index.html:428-437`). |

## 8. Redirects / routing / hosting config

| File | Rule | Effect |
|---|---|---|
| `www/vercel.json:2` | `"cleanUrls": true` | Vercel serves `/events` from `events.html` and 308-redirects `/events.html` → `/events`. |
| `www/.htaccess:1-4` | `RewriteCond %{REQUEST_FILENAME}.html -f` → `RewriteRule ^(.*)$ $1.html` | Same clean URLs on Apache (no redirect of `.html` URLs). |
| `www/.htaccess:6` | `ErrorDocument 404 /404` | Custom 404. |
| `utils/nginx.vhost.www.conf:7-10` | `try_files $uri $uri.html $uri/ =404` | Same behaviour for the local nginx container (port 3002). No custom 404 there (`error_page` only for 5xx → `/50x.html`, which doesn't exist). |

No redirects for renamed/removed content, no `www` ↔ apex rules in the repo (handled at DNS/Vercel level, not versioned).

## 9. Content-quality issues observed (to fix during migration)

| # | Issue | Location |
|---|---|---|
| 1 | Event cards use `<div href>` → not clickable; event detail pages unreachable. | `www/index.html:151-196`, `www/events.html:82-…` |
| 2 | Event detail pages are 2024 data, listings are 2025; two listing cards point at unrelated detail pages. | `www/event/*.html` vs `www/events.html` |
| 3 | Home "Latest Results" AWT card subtitle "Les Saisies 2025" but "View All" → `/seasons/awt-2024`; AWQ card is 2024. | `www/index.html:225,281,285,341` |
| 4 | `/results` page orphaned and has a broken link to `/result/2023/sonchaux`. | `www/results.html` |
| 5 | Footer year placeholder hard-coded to 2023 (relies on JS). | all pages |
| 6 | Pilot list (AWT/AWQ roster) duplicated between `index.html` and `pilots.html`; pilot data also lives in the API (`/public/pilots`, CIVL ids). | |
| 7 | 6 of 9 news articles orphaned (only 3 newest in home, no news index). | `www/news/` |
| 8 | 46 MB of unoptimised photos. | `www/assets/img/news/` |
| 9 | Sporting-code PDF self-hosted although the results site links FAI's copy (`results/constants.ts:2`). | |

## 10. Proposed content model for the rewrite

Goal: editors (non-developers) manage news, events, pilots, pages and media through a CMS / admin UI; everything that is *competition data* (results, standings, pilot identity, seasons) is **read from the scoring API** rather than retyped.

### 10.1 Collections

#### `SiteSettings` (singleton)
| Field | Type | Notes |
|---|---|---|
| `siteName` | string | "Acro World Tour" |
| `tagline` | string | "The world's best pilots fighting for the most prestigious title." |
| `defaultSeoDescription` | text | |
| `defaultOgImage` | media | |
| `contactEmail` | email | info@acroworldtour.com |
| `socialLinks` | list<{network: enum(instagram, facebook, youtube, …), url}> | used in hero + footer |
| `navigation` | list<{label, url \| pageRef, external: bool, icon}> | Home, Events, Pilots, Results… |
| `resultsSiteUrl` | url | `https://results.acroworldtour.com` |
| `analyticsId` | string | GA4 / Plausible id (consent-aware) |
| `currentSportingCode` | ref → `Document` | drives "Download Sporting Code" |

#### `HomePage` (singleton)
| Field | Type |
|---|---|
| `heroPreTitle`, `heroTitle`, `heroSubtitle` | string |
| `heroImage` | media |
| `featuredVideo` | {title, youtubeUrl} |
| `featuredNews` | list<ref → `NewsArticle`> (max 3) — or "latest N" automatic |
| `upcomingEventsCount` | int (auto from `Event` where `endDate >= today`) |
| `featuredSeasons` | list<{label, seasonCode (API, e.g. `awt-2025`), topN}> — rendered live from API standings |
| `featuredPilots` | list<ref → `PilotProfile`> or "all pilots of season X" |
| `rulesBlock` | {title, body (rich text), document ref → `Document`} |

#### `NewsArticle`
| Field | Type | Notes |
|---|---|---|
| `title` | string | |
| `slug` | slug | keep legacy slugs for redirects |
| `publishedAt` | datetime | (missing today) |
| `author` | string / ref | |
| `excerpt` | text | teaser + meta description |
| `coverImage` | media (+ alt) | |
| `body` | rich text / block content | blocks: paragraph, heading, image, gallery, YouTube embed, quote, **ResultsEmbed** |
| `ResultsEmbed` block | {competitionCode or seasonCode (API), category (solo/synchro/women…), topN} | replaces hand-typed `.results__card` |
| `relatedEvent` | ref → `Event` | |
| `tags` | list<enum: AWT, AWQ, Synchro, Women…> | |
| `seo` | {title, description, ogImage} | |

#### `Event`
| Field | Type | Notes |
|---|---|---|
| `name` | string | |
| `slug` | slug | unique per season (avoid reuse seen today) |
| `season` | int | 2025 |
| `startDate`, `endDate` | date | rendered as "July 24th – 27th 2025" |
| `location` | {site, country (ISO-3166 alpha-3)} | flag derived from ISO code — no hand-made PNGs |
| `circuits` | list<ref → `Circuit`> | AWT, AWQ, SAT… |
| `disciplines` | list<enum: solo, synchro> | |
| `coverImage` | media | |
| `description` | rich text | |
| `officialWebsite`, `registrationUrl`, `civlUrl`, `liveStreamUrl` | url | |
| `status` | enum(planned, confirmed, cancelled, finished) | |
| `competitionCode` | string (optional) | link to API competition → live results / "results" button |

#### `Circuit` (competition category)
| Field | Type |
|---|---|
| `code` | string (AWT, AWQ, WC, SAT, SWC, FRC, ITC) |
| `name` | string |
| `description` | rich text |
| `color` / `badgeStyle` | string |
| `order` | int |

#### `PilotProfile` (editorial overlay on API pilots)
| Field | Type | Notes |
|---|---|---|
| `civlId` | int | **key** — name, country, ranking pulled from API `/public/pilots/{civlid}` |
| `displayName` | string (optional override) | e.g. "Egor Hardcore" |
| `portrait` | media | |
| `bio` | rich text | |
| `rosters` | list<{season, circuit: ref → `Circuit`}> | replaces the AWT/AWQ carousels |
| `socialLinks` | list<url> | |
| `sponsors` | list<string> | |

#### `Document`
| Field | Type |
|---|---|
| `title` | string ("FAI Sporting Code S7B Aerobatics 2025") |
| `year` | int |
| `file` | media (PDF) or external URL |
| `category` | enum(sporting-code, local-regulation, press-kit…) |

#### `Page` (generic static page — about, contact, legal notice, privacy)
| Field | Type |
|---|---|
| `title`, `slug`, `body` (block content), `seo` | |

#### `LegacyResult` (one-off archive, read-only)
For the 2022–2023 hand-typed ranking pages: either import them into the scoring API as historical competitions (preferred), or keep a minimal collection `{year, title, circuit, location, rows: list<{rank, country, pilotName, civlId?, points}>}` rendered under `/result/<year>/<slug>`.

#### `Redirect`
`{from, to, permanent: bool}` — editable redirects (legacy `.html` URLs, removed event slugs, `/results` → results site).

### 10.2 Rendering / functional requirements
- Static generation (SSG/ISR) of all CMS content; API-driven blocks (standings, rosters) refreshed at build or with ISR/client fetch through the cached `/public` API.
- Shared layout component (nav/footer/head) — no duplication.
- Automatic: news index with pagination, events grouped by season (upcoming / past), per-circuit filter on `/events` (the current decorative legend becomes a real filter), current-year footer server-side.
- SEO: per-entry title/description, Open Graph + Twitter cards, canonical, `sitemap.xml`, `robots.txt`, JSON-LD `SportsEvent` / `NewsArticle`.
- Image pipeline: responsive variants, WebP/AVIF, lazy-loading, mandatory `alt`.
- Keep URL compatibility: `/`, `/events`, `/event/<slug>`, `/news/<slug>`, `/pilots`, `/result/<year>/<slug>`, `/404`; 301 for any change.
- Consent-aware analytics (GDPR — current GA4 loads without consent).
- Optional i18n readiness (`lang` per locale) — today English only.

## 11. Rewrite implementation notes (task 23)

What the Go app implements of §10, trimmed to the tables of `0001_init.sql` (`news`, `events`, `event_competitions`, `documents`, `pages`, `redirects`, `site_settings`). Pilot profiles, circuits and `LegacyResult` collections are not tables: pilots come from the results pages, circuits are free codes on the event, and the 2022-2023 result pages are served as the frozen HTML below.

| Topic | Behaviour |
|---|---|
| URLs | `/` (hero, upcoming events, latest 3 news, top 5 of each `featured_seasons` season read from stored `ranking_entries`, rules link), `/events` (upcoming first, then past by year, `?circuit=AWT` filter), `/event/{slug}` (latest season when a slug exists in several), `/news` (9 per page, `?page=`), `/news/{slug}`, `/documents`, `/{page-slug}` (stored pages only; literal routes always win, reserved slugs are refused in the admin), `/sitemap.xml`, `/robots.txt`, `/result/{year}/{slug}`. |
| News visibility | A news row is public when `published_at` is set and not in the future. No date = draft; a future date = scheduled. |
| Markdown | goldmark with raw HTML disabled: `<script>`, `<img onerror>`, `javascript:` links are never rendered (decision 0010). No embeds in Markdown. Only the home `featured_video` YouTube URL becomes an iframe (`youtube-nocookie.com`, allowed by the CSP `frame-src`). |
| Site settings keys | `hero_pre_title`, `hero_title`, `hero_subtitle`, `hero_image` (file id), `featured_video`, `featured_seasons` (JSON array of season codes), `default_description`, `default_og_image` (file id), `contact_email`, `sporting_code_document` (document id). Each value is JSON. |
| SEO | Per-page title, description, canonical, `og:*`; JSON-LD `NewsArticle` and `SportsEvent` marshalled with `encoding/json`. |
| Redirects | The `redirects` table is consulted for every unknown path and every unknown slug (`/news/x.html`, `/events.html`). `import-www` seeds a 301 for each legacy `.html` path and for the self-hosted sporting code PDFs. |
| Archive | The 14 result pages of 2022-2023 are embedded (`web/archive`) with the CSS, fonts and flags they use. The Google Analytics snippet is stripped at start-up. |
| Importer | `awt import-www --from legacy/legacy/www` is idempotent (upsert by slug; events by season and slug). Articles carry no date: it is read from the home teasers, else from the first git commit that added the file in the legacy repository, else the article is imported as a draft. Re-running overwrites news and events with the same slug but never restores ones deleted in the admin (skipped with a warning). |
| Analytics | None for now. Consent-aware analytics is an owner decision. |
