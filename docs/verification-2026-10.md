# Verification against production data, October 2026

Run on 2026-10-19, branch `rewrite`, with the production export `mongo.backup.20261018-2128.tar.gz` and the legacy `legacy/legacy/www` tree. The temporary database and blobs were deleted afterwards (they hold personal data). This report contains competition codes, counts and scores only.

Verdict: the import, the golden corpus, `make check` and the public pages all pass. One defect was found: the JSON API is not registered in the server binary (section 7).

## 1. Commands

```
go build -o awt ./cmd/awt
export AWT_ENV=dev AWT_DB_PATH=<tmp>/awt.db AWT_BLOB_DIR=<tmp>/blobs
awt migrate
awt import --from mongo.backup.20261018-2128.tar.gz
awt import-www --from legacy/legacy/www
awt golden-verify
make e2e-deps && make check
AWT_ADDR=127.0.0.1:18457 awt serve     # then curl and a comparison script
```

## 2. Import counts

| Entity | Count |
|---|---|
| pilots | 246 (244 imported + 2 placeholders for pilots referenced but absent from the export) |
| teams | 40 |
| judges | 11 |
| competitions | 48 |
| runs | 142 |
| flights | 2310 |
| flight marks / tricks / warnings | 6861 / 13932 / 46 |
| seasons | 24 (1 soft-deleted duplicate of `aws-2024` skipped) |
| season rankings | 20 |
| ranking entries stored | 6429 (competition 1455, run 4607, season 367) |
| catalog tricks / unique tricks | 257 / 3696 (4 deleted tricks skipped) |
| news | 9 (1 imported as a draft: no publication date found) |
| events | 9 |
| documents | 3 |
| redirects | 39 |
| files (export / www) | 30 / 40 |

Importer warnings: 2 referenced pilots missing from `pilots.json` (placeholders created), 1 soft-deleted duplicate season skipped, 1 news without date kept as draft. All are expected data-quality notes, not failures.

## 3. golden-verify

```
48 competitions, 20 seasons OK
```

Exit code 0.

## 4. make check

PASS. `generate` left no diff; `lint` (gofmt, vet, staticcheck, govulncheck: no vulnerability reachable, no `math.Round(`), `test -race ./...` (all 23 packages with tests ok), `golden` ok, `e2e` ok (`ok awt/e2e 32.4s`).

## 5. Comparison with the legacy values (golden corpus)

Method: for each list the multiset of totals (3 decimals) and the number of entries of the served result are compared with `testdata/golden/rankings/{competition,season}-<code>.json` (pseudonymised ids, so ids are not compared). Two sources were compared: the public HTML pages (`/competitions/<code>?list=<key>`, `/seasons/<code>`) from the real binary, and the JSON API (`/api/v1/...`) from a scratch build of the same code with the API module registered (see section 7).

### 5.1 Competitions

| Competition | Kind | List | Entries (legacy) | Entries (served) | Totals |
|---|---|---|---|---|---|
| world-acro-cup-2024 | solo, awt+awq+tur | awt-2024 | 9 | 9 | match |
| | | awq-2024 | 25 | 25 | match |
| | | tur-2024 (national) | 7 | 7 | match |
| | | women | 5 | 5 | match |
| | | overall (API/DB only) | 34 | 34 | match |
| acromax-2023 | solo, awt+awq+ita | awt-2023 | 11 | 11 | match |
| | | awq-2023 | 15 | 15 | match |
| | | ita-2023 (national) | 2 | 2 | match |
| | | women | 4 | 4 | match |
| | | overall (API/DB only) | 26 | 26 | match |
| sonchaux-synchro-2024 | synchro | aws-2024 | 5 | 5 | match |
| | | overall (API/DB only) | 5 | 5 | match |
| sonchaux-2024 | solo, awt+awq+che | awt-2024 | 9 | 9 | match |
| | | awq-2024 | 32 | 32 | match |
| | | che-2024 (national) | 12 | 12 | match |
| | | women (7 women) | 7 | 7 | match |
| | | overall (API/DB only) | 41 | 41 | match |
| acromax-2024 (API only) | solo, awt+awq | awt-2024 / awq-2024 / women / overall | 9 / 17 / 3 / 26 | 9 / 17 / 3 / 26 | match |
| roq-acro-2023 | solo, fra-2023 | all lists | 0 | 0 | match (empty in production too: closed, no participants) |

Coverage of the brief: 2 solo with awt-* and awq-* seasons (world-acro-cup-2024, acromax-2023, plus sonchaux-2024 and acromax-2024), 1 synchro, nationals (tur, ita, che), women lists with 4, 5 and 7 women.

The generic `overall` list is hidden on the HTML page when the competition belongs to an AWT/AWQ/AWS season (spec 06: the season-specific ranking replaces it), which is why the HTML column only has the season lists for those. It is stored and served by the API, and matches.

### 5.2 Seasons (API, overall list; HTML overall also compared for the first four)

| Season | Entries (legacy) | Entries (served) | Totals |
|---|---|---|---|
| awt-2024 | 9 | 9 | match (top 265.803) |
| awq-2023 | 41 | 41 | match (top 130.296); women 9/9 match |
| aws-2023 | 8 | 8 | match (top 59.012) |
| che-2024 | 12 | 12 | match (top 25.499); women 3/3 match |
| tur-2024 (API) | 7 | 7 | match |

## 6. URL checks (real binary, `AWT_ENV=dev`, 127.0.0.1)

| URL | status | time_total (s) | redirect |
|---|---|---|---|
| / | 200 | 0.003834 |  |
| /results | 200 | 0.002605 |  |
| /seasons | 200 | 0.005154 |  |
| /competitions | 200 | 0.003806 |  |
| /competitions/world-acro-cup-2024 | 200 | 0.007337 |  |
| /competitions/acromax-2023 | 200 | 0.003790 |  |
| /competitions/sonchaux-synchro-2024 | 200 | 0.003383 |  |
| /competitions/sonchaux-2024 | 200 | 0.003346 |  |
| /competitions/roq-acro-2023 | 200 | 0.002084 |  |
| /competitions/world-acro-cup-2024/runs/1 | 200 | 0.020211 |  |
| /competitions/world-acro-cup-2024/details | 200 | 0.001774 |  |
| /seasons/awt-2024 | 200 | 0.005532 |  |
| /seasons/awq-2023 | 200 | 0.005523 |  |
| /seasons/aws-2023 | 200 | 0.003801 |  |
| /pilots | 200 | 0.005654 |  |
| /pilots/26069/theo-de-blic | 200 | 0.004000 |  |
| /pilots/67619 | 301 | 0.001200 | http://localhost:18457/pilots/67619/luke-de-weert |
| /pilots/46598 | 301 | 0.001356 | http://localhost:18457/pilots/46598/bicho-carrera |
| /teams | 200 | 0.003029 |  |
| /teams/64d5ef3f13068cd8790874b5 | 301 | 0.001116 | http://localhost:18457/teams/64d5ef3f13068cd8790874b5/team-back2back |
| /judges | 200 | 0.001565 |  |
| /tricks | 200 | 0.004030 |  |
| /simulator | 200 | 0.012666 |  |
| /events | 200 | 0.002193 |  |
| /event/acromax | 200 | 0.002072 |  |
| /news | 200 | 0.002393 |  |
| /news/acromax | 200 | 0.002017 |  |
| /documents | 200 | 0.001574 |  |
| /sitemap.xml | 200 | 0.004581 |  |
| /robots.txt | 200 | 0.000853 |  |
| /healthz | 200 | 0.000900 |  |
| /news/acromax.html | 301 | 0.001458 | http://localhost:18457/news/acromax |
| /event/acromax.html | 301 | 0.001330 | http://localhost:18457/event/acromax |
| /assets/files/sporting_code_s7_b_-_aerobatics_2024.pdf | 301 | 0.001235 | http://localhost:18457/files/1b3cd99846ee03016ca8192c525bb03071286381b65afe43d9e0c50c63b609a8 |
| /nonexistent-page | 404 | 0.001180 |  |
| /api/v1/competitions (main binary) | 404 | 0.001425 | |
| /api/v1/competitions (api wired in scratch build) | 200 | 0.004553 | |

No 5xx and no WARN/ERROR line in the server log (all requests logged at INFO). The 301 on `/pilots/<id>` and `/teams/<id>` is the canonical slug redirect; `/news/<slug>.html`, `/event/<slug>.html` and the legacy PDF path are the imported legacy redirects. `/api/v1/competitions` answers 404 on the real binary: see section 7.

## 7. Discrepancy found

**D1: the JSON API is not served by `awt serve`.** `cmd/awt/main.go` builds the server with `public.Module{}, admin.Module{}` only; `api.Module{}` (`internal/web/api`) is never registered, so every `/api/v1/*` URL returns the HTML 404 page. The module is complete and its own tests pass; in a scratch copy of the repository with `api.Module{}` appended to the module list, `/api/v1/competitions` answers 200 and all API comparisons of section 5 match. Fix for the controller: add `api.Module{}` to the `web.NewServer` call (and an e2e or server-level test that `/api/v1/competitions` answers 200 from the real wiring). No product code was changed in this task.

No scoring or ranking discrepancy: every compared list matches the legacy totals and entry counts.

## 8. Performance smoke (single run, local, SQLite WAL)

| Metric | Value |
|---|---|
| `awt migrate` | under 0.1 s |
| `awt import` (wall clock) | 1.16 s |
| `awt import-www` (wall clock) | 0.52 s |
| database file after import | 8.2 MB (8,187,904 bytes) |
| busiest competition page `/competitions/world-acro-cup-2024` (34 entries, 5 runs), 20 requests | p50 4.9 ms, max 6.3 ms |
| `/competitions/sonchaux-2024` (41 entries), 20 requests | p50 4.2 ms, max 6.1 ms |
| `/competitions/world-acro-cup-2024?list=awq-2024`, 20 requests | p50 6.1 ms, max 9.4 ms |
| slowest page of section 6 | `/competitions/world-acro-cup-2024/runs/1`, 20 ms (first request) |

## 9. Specifications

`specifications/03-scoring-rules.md`: added a note after the §5.0 algorithm that numbers in notes use Python `str(float)` formatting (`2.0%`, not `2%`), as implemented by `pyFloat`.
