# AcroWorldTour: specifications for the rewrite

These documents describe the legacy AcroWorldTour stack in enough detail to rebuild it from zero. They were reverse-engineered from the legacy monorepo (`legacy/legacy`, commit `979862e`, 502 commits from 2023-05 to 2026-10).

All source references (`api/models/competitions.py:1094`) are relative to the legacy repo root.

## What the system does

AcroWorldTour runs the scoring and publication of paragliding aerobatics ("acro") competitions under FAI/CIVL Sporting Code Section 7B:

- **Organisers and judges** use the **manager** to set up seasons, competitions, runs and starting orders, then type in each flight's tricks and judges' marks live. The API computes the score server-side.
- **The public** follows live and past results on the **results** site: competitions, runs, seasons (AWT / AWQ / national), pilots and teams.
- The **www** site is the hand-written institutional site: news, events, sporting code PDFs.

## Documents

| # | File | Content | Read it if you build... |
|---|---|---|---|
| 01 | [01-current-architecture.md](01-current-architecture.md) | Components, hosting (fly.io, render.com, Vercel, OVHcloud Mongo), nginx cache proxy, CI/CD, env vars, data import, operational pain points, **secrets to rotate** | infra, deployment |
| 02 | [02-domain-model.md](02-domain-model.md) | Every entity and field, Mongo collections, ER diagram, competition/run/flight state machines, stored vs export models | the data model |
| 03 | [03-scoring-rules.md](03-scoring-rules.md) | **Exact scoring algorithm**: pseudo-code in execution order, config defaults, repetitions, warnings/DSQ/DNS, AWT vs AWQ marking, run/competition/season rankings, rounding, executed worked examples, 34 open questions | the scoring engine (critical) |
| 04 | [04-api.md](04-api.md) | Full endpoint catalogue (admin and public), auth, caching, files, exports (xlsx/HTML/SVG), starting order, performance and security issues | the backend |
| 05 | [05-manager.md](05-manager.md) | Back-office screens and tabs, judging workflow, client-side bugs, must-have checklist | the judging / admin UI |
| 06 | [06-results.md](06-results.md) | Public results sitemap, displayed data, live mode, simulator, public data contract, must-have checklist | the public results site |
| 07 | [07-www.md](07-www.md) | Institutional site sitemap, visual identity, SEO, proposed content model | the institutional site |
| 08 | [08-rewrite-recommendations.md](08-rewrite-recommendations.md) | Consolidated issues, decisions to make, proposed target architecture, migration plan, low-cost hosting, tech stack and AI-driven development | the plan |

## The 10 things to know first

1. **The scoring engine is the product.** Everything else is CRUD and display. `03-scoring-rules.md` gives the algorithm in exact execution order. A port must reproduce IEEE-754 doubles and Python's `round(x, 3)` at the same points.
2. **There are no scoring tests in the legacy code.** Build a golden corpus from production data first: recompute every stored flight and compare with its stored `final_marks`.
3. **Several legacy behaviours look like bugs**, for example `pilots_to_qualify` keeps the *worst* pilots, the `last_maneuver` rule is inverted, unpublished flights count in public totals, and per-run ranks are inverted. Each needs an explicit decision: reproduce or fix (see 08 §2).
4. **A competition is one big Mongo document** (runs → flights → marks embedded), rewritten whole on every flight save with no locking. Concurrent scorers can overwrite each other.
5. **Nothing is precomputed.** Every public read recomputes full results. A pilot page cascades this across every competition and season the pilot is in. The only mitigation is the nginx proxy (1 min to 1 year TTL, no purge).
6. **There is no real live mode.** The results site has no polling interval and the API has no push (SSE/WebSocket). "Live" means a 1-minute cache plus a user refresh.
7. **Auth is a single shared admin account** with an 8-day JWT in `localStorage`. Auth is switched off entirely if `ADMIN_PASS` is unset, and several manager pages and export endpoints have no guard.
8. **A production MongoDB password is committed** in `docker-compose.yml:12`. Rotate it now, whatever happens with the rewrite.
9. **Four hosting providers** and three of five components have no CI. The www site duplicates by hand data the API already has.
10. **Rules change every year** (sporting code 2022 to 2025), but rule constants are hard-coded and the trick catalog is global. Importing a new year overwrites the definitions used by past competitions.
