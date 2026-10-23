# Implementation rulings (2026-10-18/19)

Decisions taken by the AI controller during the Go rewrite, on the owner's behalf, when the spec or plan was silent or ambiguous. Each line: what was decided — why — what it costs if wrong. Review and override any of them with a decision record.

- T1 skips git init; it extends the existing .gitignore — controller already initialised the repo on branch `rewrite` — cost if wrong: none.
- Makefile `golden`/`e2e` targets no-op with a message while their directories are absent — keeps `make check` usable from T1 — cost if wrong: a missing suite could pass silently until T7/T25 (mitigated: T7/T25 make them real).
- catalog.UniqueTrick uses snake_case JSON tags identical to legacy field names (name, acronym, technical_coefficient, bonus, bonus_types, base_trick, uniqueness, bonuses, synchro, solo, solo_awt, types) — golden files need them — cost if wrong: re-tagging.
- mark type derived per pilot (MarkTypeFor) is recorded as part of decision 0006 (replaces three manual tabs, spec 08 §2.2) — cost if wrong: add an operator override later.
- T23 adds subcommand `import-www --from <legacy www dir>` — T27 needs it — cost if wrong: none.
- T25 installs chromium without --with-deps; if the browser cannot launch, e2e reports the launch error (no silent skip) — cost if wrong: e2e must run in CI container only.
- toolchain pinned to go1.26.9 instead of go1.26.4 — govulncheck stdlib CVEs block lint on 1.26.4 — cost if wrong: none (newer patch).
- uniqueness involving nullable columns (deleted_at, civlid/team_id) uses partial unique indexes (WHERE deleted_at IS NULL; WHERE civlid IS NOT NULL / team_id IS NOT NULL) since SQLite treats NULLs as distinct in UNIQUE — cost if wrong: none.
- commit trailers name the model that actually wrote the commit (subagent attribution reminders), not Opus — accuracy — cost if wrong: none.
- unique_tricks keeps UNIQUE(year, acronym); a trick's unique_tricks rows are deleted when the trick is soft-deleted and regenerated on restore (service responsibility, Task 12) — keeps the schema simple — cost if wrong: re-add rows on restore.
- pilots.name not unique (CIVL id is the key; homonyms possible) — cost if wrong: none.
- `awt migrate` subcommand + `make migrate` wired in Task 2 fix round — Global constraints list it — cost if wrong: none.
- testdata/catalog/2022.json must load: drop the two legacy 'mirror' synchro spiral tricks (legacy could not import them either) and document it in the file's README/test — Task 10 imports every year — cost if wrong: two 2022 synchro tricks missing from the 2022 catalog (no stored flight depends on catalog rows).
- Tasks 5 and 6 dispatched as one implementer + one review (one algorithm, steps 1-16 share types and tests) — fewer handoffs — cost if wrong: bigger review diff.
- independent pure packages (internal/domain T9) may be implemented concurrently with the engine (internal/scoring) — disjoint files; implementers git add only their own paths and retry on index.lock — saves wall-clock — cost if wrong: a commit retry.
- during concurrent implementation, make check may be blocked by the other agent's uncommitted files; each task's focused tests + vet/staticcheck on its own paths are accepted, and make check is re-run by the controller when the tree is clean — cost if wrong: a lint failure surfaces one task later.
- Task 14 adds AWT_TRUSTED_PROXIES / AWT_CLIENT_IP_HEADER (default CF-Connecting-IP) — planned deployment is behind Cloudflare+Caddy — cost if wrong: config tweak.
- password minimum length 10 — none in brief, security baseline — cost if wrong: none.
- golden generator (T7) patches legacy `sum` with naive left-to-right float summation — production ran Python 3.10 (naive sum); venv is 3.12 (compensated) — cost if wrong: last-digit diffs in ~1% AWT bonuses.
- note strings use production float formatting ("13.0%", "1.0") — matches stored production notes — cost if wrong: spec 03 examples show int formatting; spec to be updated in T27.
- classic marking keeps stored per-trick technical_mark scaling bonus (legacy behaviour, 78 prod flights); golden inputs carry full trick snapshots — cost if wrong: none for history (frozen).
- "did not mark enough tricks" 400 message names the judge id instead of the judge name (Input carries no names) — UI will map ids to names — cost if wrong: golden error case mismatch would surface in T7.
- file id accepted as sha256 of either decoded bytes or base64 text (legacy hashed the base64 string) — real data — cost if wrong: none.
- Task 19 split — 19a = internal/civl client (independent, now), 19b = admin reference screens (after T15) — parallelism — cost if wrong: none.
- T15 builds the web foundation before the service layer exists: web.Routes takes a Deps struct {Cfg, Auth, Bus-like Purger subscription later}; feature packages register handlers via func Register(mux *http.ServeMux, deps Deps) added by later tasks; the cache purger subscribes to the service Bus in T12/T16 — decouples ordering — cost if wrong: small signature change.
- env example files live at deploy/env*.example (tool permission denies .env* paths) — cost if wrong: rename only.
- golden generator resolves catalog lookups of tricks deleted in 2024 (Full Stall, Super Stall, Tail Slide to Infinity Tumbling) to their last definition instead of legacy 404 — otherwise 751 flights are unscorable — cost if wrong: those cases test a reconstruction, not literal legacy behaviour.
- historical results stay frozen (decision 0003) — confirmed by data: 913/2310 stored scores differ from today's legacy recomputation (rule/catalog drift since 2022).
- deploy SSH = internet-reachable non-default port, key-only, fail2ban, forced-command deploy key — GitHub runners have no fixed IPs — cost if wrong: switch to self-hosted runner/VPN later. Superseded by decision 0016 (pull-based deploy).
- TLS = Cloudflare Origin CA certificate, Full (strict), no ACME — Caddy only accepts Cloudflare traffic — cost if wrong: cert renewal procedure differs.
- golden corpus pseudonymises pilot/team/judge ids (gender/country tied to real CIVL ids = personal data) — cost if wrong: none; NOTE commit b5d81d0 still holds the non-pseudonymised corpus in local history: squash/rewrite before any push (tell the user).
- qualification in mixed AWT/AWQ events keeps the best N by overall competition score, then applies the mixed start order (AWQ first, worst→best) — avoids cutting all AWQ pilots — cost if wrong: owner may want per-category quotas (flag to user).
- importer creates placeholder pilot rows (name "Pilot #<civlid>", empty country, rank 9999, gender none) for CIVL ids referenced by competitions/flights/teams but missing from pilots.json (hard-deleted legacy pilots, e.g. 1234, 51090, 67619 referenced by ~20 competitions), and then imports the previously skipped teams — preserves historical results — cost if wrong: placeholder names visible on old result pages (owner can fix names in admin).
- T22 builds internal/export as pure renderers over its own input structs (CompetitionExport etc.); admin route wiring is deferred to T20 (admin competitions) — service not ready — cost if wrong: none.
- importer fills config keys missing from stored legacy configs (e.g. warning2) from the start-year ruleset — equals legacy defaults — cost if wrong: none for frozen results.
- T24 JSON API reads stored rows (store queries) directly, not via the service layer (read-only, service still in progress) — cost if wrong: small refactor later.
- simulator reset rule follows legacy (0-based i>0 && i%k==0) — matches the legacy public simulator — cost if wrong: off-by-one in reset frequency.
- T16 public pages read stored rows directly through internal/store queries (same as T24), sharing a small read-model package internal/readmodel if both need it — whichever task lands second reuses the first's queries — cost if wrong: duplicate queries to merge in final review.
- GET /files/{id} added to Task 16 (public module) — no task owned it — cost if wrong: none.
- public reads (API + pages) of run-scope rankings show only participants whose flight in that run is published (legacy public run results were published-only); competition/season totals keep decision 0005 — cost if wrong: none.
- no per-run repeatable tricks (scoring uses the competition list, as legacy) — cost if wrong: migration later.
- SetRunOrder refuses closed runs — roster edits follow run state — cost if wrong: reopen first.
- public competition page hides the `overall` list when aw[tqs]-* lists exist (spec 06 §4.5/§11, legacy behaviour) — cost if wrong: one-line change.
- DuplicateCatalog replaces same-name tricks in the target year (like import) — cost if wrong: none.
- overall-tab per-run cells show "—" for unpublished flights and per-run ranks are re-ranked among published flights (consistent with run tabs); totals keep decision 0005 (so a total can include a hidden run score while a run is unpublished — flag to user) — cost if wrong: display only.
- judge marks sorted by judge id before scoring (deterministic save vs rescore; may differ from legacy input order in last bit, new flights only) — cost if wrong: none.
- season rankings rebuilt on competition delete/restore — consistency — cost if wrong: none.
- changing a non-frozen competition's seasons or start year rescores all its flights in run order and rebuilds rankings in the same Tx — compute-on-write consistency — cost if wrong: slower update (rare admin op).
- fragment route is /competitions/{code}/fragment?list=&run= (not /fragment/{list}) — documented — cost if wrong: none.
- export HTML pages may relax CSP to allow inline <style> (no scripts) — print-only pages — cost if wrong: none.
- limit_run is 1-based (results after run N) — human URL convention — cost if wrong: none.
- changing a RUN's config rescores that run's flights (and later runs) + rebuilds rankings; competition config only applies to new runs (legacy copy semantics) — compute-on-write consistency — cost if wrong: none.
- SetRunJudges refuses removing a judge who already has marks on saved flights of the run — keeps stored scores consistent — cost if wrong: admin must delete marks first.
- legacy news without a date get published_at = date of the first git commit that added the file in the legacy repo (when it went online), so legacy URLs keep working — cost if wrong: date approximate (editor can fix).
- import-www overwrites same-slug content — run once at migration — cost if wrong: re-run clobbers edits (documented).
- import-www never resurrects admin soft-deleted news/events (skip + warn) — cost if wrong: none.
- flight_marks store the scorer's RAW marks (nulls kept); engine-filled per-trick values live only in final_mark/technical_marks — re-opening a flight must show what was typed — cost if wrong: none (imported frozen flights keep legacy-filled marks).
- judging save carries an optimistic version (flight updated_at/version); stale save → 409 "flight changed by someone else, reload" — two scorers on the same flight — cost if wrong: none.
- delete flight (unpublished or published) available to scorers in open runs, with rescoring/rankings rebuild — spec 05 must-have — cost if wrong: none.
- no mark-type override UI (derived per pilot, decision 0006); tip touches accept ≥ 0 (bonus capped by rules) — cost if wrong: small UI later.
- tab order judge by judge (row-wise) kept — cost if wrong: template change.
- live fragment route sends `Cache-Control: public, max-age=0, s-maxage=5, must-revalidate` (browser always revalidates; CDN may hold 5 s and is purged on events) — e2e found missed updates with max-age=5 — cost if wrong: slightly more origin traffic.
- playwright driver installed by a repo script (scripts/install-playwright.sh) reproducing the PyPI-wheel workaround, used by `make e2e-deps` and CI — upstream CDN 404 — cost if wrong: switch when upstream fixed.
- reference-data edits that change scores/lists (SetAWTYear, ToggleGender, judge level, SetRepeatableTricks) rescore + rebuild affected non-frozen competitions in the same Tx and publish after commit — compute-on-write — cost if wrong: slower admin edits.
- drop the AWT_SESSION_SECRET requirement (sessions use random DB-backed tokens; nothing reads it) and require AWT_BASE_URL outside dev — cost if wrong: none.

## Open items for the owner

- Decision 0005 "Decide" items still use legacy behaviour (unpublished flights count in totals, cumulative warning penalty, automatic warnings not carried, synchro 20/20/20/40).
- Verify the CIVL ranking export form encoding against the live site (internal/civl).
- Run the deploy pipeline once against the real VPS / Cloudflare / GHCR.
- Consent-aware analytics for the public site.
- One legacy news article has no git date and stays a draft until dated.

## MFA feature rulings (2026-10-19)

- T3 extends isLoginPath to "/manager/login" and the "/manager/login/mfa" prefix — login second step must be reachable without a session — cost if wrong: none.
- T4 RP origins = origin of AWT_BASE_URL; in dev with empty BaseURL use http://localhost:<port> and http://127.0.0.1:<port> from AWT_ADDR — never derived from the request Host header (spoofable) — cost if wrong: dev keys need BaseURL set.
- T5 updates TestEveryAdminRouteIsGuarded so /manager/account routes are allowed with the User guard (and only those) — cost if wrong: none.
- sealSecret/openSecret take a purpose string used as GCM AAD (totp-secret|user:<id>, totp-enroll|user:<id>); TOTP keys < 16 bytes rejected — cheap defence in depth before Tasks 3/5 bind to the signature — cost if wrong: none.
- the 5th failed factor attempt itself answers 429 (locking failure) — informs the user immediately; 6th is 429 too — cost if wrong: none.
- Config.Origins() is the single origin source for Origin checks and the WebAuthn RP — cost if wrong: none.
- account-form password checks get a per-user limiter (5 wrong current passwords / 15 min → 429), and bcrypt runs before the write Tx; inside the Tx the stored password_hash must still equal the one verified (else retry error) — no 250 ms write lock during live judging — cost if wrong: none.
- failure counter without 15-min window kept (stricter than acceptance wording; matches settled "5 failed attempts") — cost if wrong: an old failure can contribute to a lock.
- Deferred, owner to know: recovery codes are unsalted SHA-256 of ~49.5-bit codes (plan-mandated); a leaked DB allows offline cracking in about a day on one GPU, but the password is still required. Later: 16-character codes.
- Deferred: the factor failure counter has no 15-minute window (failures accumulate until a success, a lock or an admin reset).
