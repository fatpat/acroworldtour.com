# AcroWorldTour

Scoring and results platform for paragliding aerobatics competitions (FAI/CIVL Sporting Code Section 7B). One Go application serves:

- the public results site,
- the institutional site,
- the admin and judging back-office,
- the JSON endpoints and live updates.

This is a rewrite of the legacy stack in `legacy/legacy/`.

**This codebase is written and maintained by AI agents only. No human edits the code.** Humans write specs, make decisions and review behaviour. The specs, the tests and CI are the only safety net, so treat every rule below as mandatory.

## Sources of truth

| What | Where |
|---|---|
| Functional specs | `specifications/` (start with `README.md`) |
| Scoring algorithm (must match exactly) | `specifications/03-scoring-rules.md` |
| Domain model | `specifications/02-domain-model.md` |
| Screens and workflows | `specifications/05-manager.md`, `06-results.md`, `07-www.md` |
| Stack, hosting, open decisions | `specifications/08-rewrite-recommendations.md` |
| Decisions taken | `docs/decisions/NNNN-title.md` (one file per decision) |
| Legacy code (reference, **read-only**) | `legacy/legacy/` |

When the spec and the legacy code disagree, the spec wins. When the spec is silent or marks something as **Decision**, check `docs/decisions/`. If no decision is recorded, **stop and ask the human**. Never guess a business rule.

## Workflow for every change

1. Read the relevant spec sections and decisions before touching code.
2. Keep the change small: one feature or fix per branch and commit series.
3. Write or update the tests first, from the spec's acceptance criteria. Then implement.
4. Run `make check` and make it pass. Never commit with a red check.
5. If behaviour changed, update the spec in the same change. If you discovered a rule, add it to the spec with a source reference.
6. Commit with a Conventional Commit message (`feat(scoring): ...`, `fix(judging): ...`).

## Commands

The `Makefile` is the interface. If a target is missing, add it rather than running ad-hoc commands in CI.

| Command | Does |
|---|---|
| `make dev` | Run the app locally with live reload, a local SQLite file and seed data |
| `make generate` | Run `templ generate` and `sqlc generate`. Generated files are committed. |
| `make test` | Unit tests (`go test ./...` with `-race`) |
| `make golden` | Scoring golden tests against `testdata/golden/` |
| `make e2e` | End-to-end browser tests (`go test ./e2e/...`, playwright-go) |
| `make lint` | `gofmt -l`, `go vet`, `staticcheck`, `govulncheck` |
| `make check` | `generate` (must leave no diff), then `lint`, `test`, `golden`, `e2e`. This is what CI runs. |
| `make migrate` | Apply database migrations |
| `make docker` | Build the production image |

## Stack

- **Go**, latest stable, pinned with the `toolchain` directive in `go.mod`. One module, one binary in `cmd/awt`.
- **HTTP**: standard library `net/http` (method and path patterns). No web framework.
- **HTML**: `templ` components, server-rendered for every page.
- **Interactivity**: htmx with its SSE extension, plus small plain-JS modules in `web/static/js/`. There's no SPA, no npm and no front-end build step. Vendor htmx at a pinned version.
- **CSS**: plain CSS with custom properties in `web/static/css/`.
- **Database**: SQLite in WAL mode through `modernc.org/sqlite` (pure Go, no cgo). Queries live in `internal/store/queries/*.sql`, and `sqlc` generates type-safe Go from them. Migrations are numbered in `internal/store/migrations/` and never edited once merged.
- **Files** (photos, logos, PDFs): S3-compatible object storage (OVHcloud Object Storage), never in the database.
- **Backups**: Litestream replicates the SQLite file to object storage.
- **Logging**: `log/slog` with JSON output in production.
- **Config**: environment variables only, parsed once at startup into a typed struct. The app refuses to start if a required value is missing.

### Dependencies

The standard library comes first. Adding a module requires:
- a one-line justification in the commit message;
- that it be maintained, widely used and permissively licensed.

The current allowed list is `templ`, `sqlc` (tool), `modernc.org/sqlite`, an S3 client, `playwright-go` (tests only), and `golang.org/x/*`. Anything else needs a decision record.

## Repository layout

```
cmd/awt/              main: config, wiring, server start
internal/scoring/     PURE scoring engine (see below)
internal/domain/      entities, state machines, validation (no I/O)
internal/store/       SQLite access: migrations/, queries/, generated code
internal/service/     use cases: transactions, rescoring, ranking rebuild, events
internal/web/         HTTP handlers, middleware, SSE, templ components
  public/             results + institutional pages
  admin/              back-office and judging screens
internal/civl/        CIVL import (pilots, rankings)
internal/export/      CIVL xlsx, starting order, results, overlay SVG
web/static/           css, js, vendored htmx, images
testdata/golden/      scoring golden corpus (generated, never hand-edited)
e2e/                  browser tests of full workflows
docs/decisions/       decision records
specifications/       functional specs
legacy/legacy/        legacy code, read-only
```

Dependency direction: `web` → `service` → `domain`, `scoring`, `store`. `scoring` and `domain` import nothing from this repo except each other, and they do no I/O.

## Scoring engine rules (`internal/scoring`)

This is the most critical code in the project.

- **Pure functions only**: `Score(input) (FinalMark, error)`. No database, no clock, no globals, no logging. Every input is passed in explicitly: the flight, the run config, previous flights, catalog snapshots, judge levels and the pilot's AWT status.
- **Follow `03-scoring-rules.md` §5.0 step by step, in the same order.** Keep the same step numbers in code comments (`// step 10: repetition malus`) so the code can be checked against the spec.
- **Use `float64` and the same operation order** as the spec. Don't simplify formulas algebraically; that changes floating-point results.
- **Rounding**: use the single `scoring.Round3` helper, which implements Python's `round(x, 3)` via `strconv.FormatFloat(x, 'f', 3, 64)`. Call it only at the points listed in spec §9. Never use `math.Round(x*1000)/1000`.
- **Rules are data**: percentages, warning points, malus, bonus caps and the DSQ threshold come from the season's ruleset, never from constants in code.
- **Legacy quirks**: reproduce legacy behaviour unless `docs/decisions/` records a fix (see the list in spec 08 §2.1). Each deliberate difference has a decision record and a golden-test exception that names it.
- **Golden tests**: `testdata/golden/` holds input and expected-output pairs generated from production data by the legacy engine. **Never edit, regenerate or delete golden files to make a test pass.** A mismatch means the engine is wrong, or a decision is missing, in which case ask.
- Add a table-driven test for every rule in the spec, using the worked examples in spec §12, and a fuzz test checking invariants (score ≥ 0, technicity ≥ 1).

## Data and domain rules

- Competitions, runs, flights and marks are relational tables with foreign keys (`PRAGMA foreign_keys=ON`). Open connections with `busy_timeout` and `journal_mode=WAL`.
- **Compute on write.** Saving a flight runs in one transaction that:
  1. rescores every dependent flight of that pilot or team (later runs);
  2. rebuilds and stores the run, competition and season rankings;
  3. then publishes an SSE event after commit.

  Public reads never compute scores.
- Enforce state machines in `internal/domain` (spec 02 §5). A flight can't be saved in a closed run without reopening it first.
- Historical results imported from legacy are frozen: never rescore them.
- Use soft delete where the spec says so. Write timestamps in UTC.

## Web rules

- Every public page is server-rendered with real `<title>`, meta and Open Graph tags, and works without JavaScript.
- Use htmx for partial updates. Debounce the live provisional score request (about 300 ms) and ignore out-of-order responses.
- SSE: send a heartbeat every 30 s and disable response buffering. On reconnect, the client reloads the affected fragment.
- Auth: per-person accounts with roles (`admin`, `scorer` per competition), sessions in HTTP-only `Secure` `SameSite=Lax` cookies, CSRF tokens on every form, and a rate limit on login. Every admin route sits behind the auth middleware. Add a test that lists the routes and fails if any admin route is unguarded.
- Set `Cache-Control` on public responses for the CDN, and purge the affected URLs after a publish.
- Uploads: check the type and size, store them in object storage, and serve them with `Content-Disposition` and the right content type. Never serve user-uploaded HTML or SVG inline.

## Code conventions

- Format with `gofmt`. Use the standard project layout above, with small packages named for what they provide.
- Wrap errors with context (`fmt.Errorf("save flight %d: %w", id, err)`), and define domain errors as sentinel values or typed errors. Handlers map them to HTTP codes in one place.
- `context.Context` is the first parameter of anything that does I/O.
- Don't use `panic` outside `main` and impossible states. Don't use `init()` side effects or global mutable state.
- Comments explain *why* and point to the spec (`// spec 03 §5.5`). Don't add comments that restate the code.
- Name things with domain terms from the specs: run, flight, trick, unique trick, technicity, malus, warning, DNS, DSQ.

## Never

- Never modify anything under `legacy/`.
- Never commit secrets, `.env` files or production data. Note that `legacy/legacy/docker-compose.yml` contains a real credential: never copy it anywhere.
- Never edit golden files, skip, disable or weaken tests, or lower the lint configuration to get green.
- Never invent a business rule, a scoring constant or a resolution to an open **Decision**.
- Never add a JS framework, a bundler or npm.
- Never run destructive commands against a non-local database.
- Never push straight to `main`. Every change goes through CI.

## Legacy reference

`legacy/legacy/` is the original Python/FastAPI + Next.js stack at commit `979862e`. Use it to answer "what did the old system do?" and to regenerate the golden corpus. Its scoring engine is `api/models/competitions.py` (`calculate_score`, lines 968–1513). It can run with the database mocked in a separate Python virtualenv kept outside the repo; see spec 03 for how the worked examples were produced. Read it, run it, never change it.
