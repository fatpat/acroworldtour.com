# Golden corpus generator

`generate.py` runs the legacy Python engine (`legacy/legacy/api`, read-only)
on a production MongoDB export and writes `testdata/golden/`. The corpus is
generated, never hand-edited (CLAUDE.md "Never").

```
make golden-regen LEGACY_VENV=<legacy virtualenv> BACKUP=<mongo export .tar.gz>
make golden        # TestGoldenFlights + ranking corpus decoding
```

The target runs in a temporary directory that it removes afterwards.
`LEGACY_VENV` is a Python 3 virtualenv with the legacy API dependencies
(`legacy/legacy/api/requirements.txt`), kept outside the repository. The
target runs the generator from a temporary directory with `python -I`.
The export is a tar of JSON-lines files (`competitions.json`, `pilots.json`,
`teams.json`, `judges.json`, `seasons.json`, `tricks.json`).

## How legacy is run

- MongoDB is replaced by an in-memory copy of the export (`FakeCollection`
  implements the `find_one`/`find` queries of the legacy getters); nothing is
  ever written.
- `sum` in `models/competitions.py` is replaced by a naive left-to-right float
  sum: production ran Python 3.10, whose `sum()` is naive; Python >= 3.12
  compensates and changes the last digits of AWT bonus totals.
- Every flight of run `i` is scored on a fresh copy of its competition with
  `runs` truncated to `i+1` (decision 0004), with
  `mark_type = flight.final_marks.mark_type` and `is_awt_pilot` =
  pilot AWT for the competition start year (`False` for synchro), as
  `flight_save` does.
- Catalog lookups (position rules, repeatable tricks) read the current
  catalog. Tricks deleted from the catalog since (Full Stall, Super Stall,
  Tail Slide to Infinity Tumbling, deleted in 2024) fall back to their
  last (deleted) definition: the flights using them were scored while they
  existed, and today's legacy would only answer "Trick ... not found" (751
  flights).
- Stored trick snapshots are scored as stored, including stored
  `technical_mark`/`technical_marks` and null `solo`/`synchro`/`solo_awt`
  flags.

## Anonymisation

The corpus holds pseudonymous ids, competition ids and codes (public
events), numbers, countries, genders and trick names/acronyms only.

- Pilot CIVL ids, team ids and judge ids are replaced by pseudonyms in the
  export before legacy runs, so inputs, expected outputs, error messages and
  ranking files agree: pilots `1000001`, `1000002`, ... teams `t0001`, ...
  judges `j0001`, ..., numbered in first-appearance order (competitions by
  code, their flights, participants and judges, then the rest of the
  export). Gender and country stay attached to a pseudonym only. The mapping
  is never written. While legacy runs, teams and judges carry ObjectId-shaped
  pseudonyms (`000000000000000000b00001`, legacy validates ObjectIds); the
  writer turns them into `t0001`/`j0001`.
- Operator warning texts are replaced by `"w"` before scoring, so input and
  expected output agree (legacy notes only count warnings).
- Judge names are replaced by the judge's (pseudonymous) id in the
  in-memory judges collection: the legacy error `judge '<name>' did not mark
  enough tricks (...)` then carries the id, which is the engine's wording
  (controller ruling).
- Pilot and team names are never written; ranking files carry a
  `name_order` instead (see below).
- Leak guard: generation fails if any output file contains, as a plain
  substring, a pilot or judge name (3 characters or more) or a real team or
  judge id, or a real CIVL id as a whole number.

## Flight cases: `testdata/golden/flights/<competition code>.json`

```
{"competition": "<id>", "code": "<code>", "cases": [
 {"run": i, "participant": "<civl id | team id>",
  "input": <scoring.Input>,                 // internal/scoring/doc.go
  "expected": <scoring.FinalMark> | null,   // full precision legacy floats
  "expected_tricks": [<FlownTrick>] | null, // flight tricks after scoring
  "expected_marks": [<JudgeMark>] | null,   // marks after per-trick fill-in
  "error": "<legacy HTTPException detail>" | ""},
 ...]}
```

One case per line, in run then stored flight order. Floats are written by
Python's `json` (repr, shortest round-trip), so Go parses the exact values.

`input` is built from the same pydantic objects legacy scored: run config,
`reset_policies` of runs `0..i`, `previous` = the participant's first
flight of each run `< i` (legacy `break`, `competitions.py:1134`; the
generator fails if a run holds two flights of one participant) with its
stored tricks and operator warning counts, judge levels,
position rules by base trick, repeatable base trick names, season flags,
`legacy_last_maneuver: true`, and the flight (marks after pydantic
validation, i.e. explicit null marks are 0).

`TestGoldenFlights` (internal/scoring/golden_test.go) requires, for every
case: the same `ValidationError` message when `error` is set; otherwise
every float equal after `Round3` and within 1e-9 raw, identical notes,
warnings and mark type, identical per-trick technical marks and filled
judge marks.

## Ranking cases: `testdata/golden/rankings/`

Go types: `internal/ranking/golden_types.go`.

`competition-<code>.json` (one per non-deleted competition):

```
{"scope": "competition", "code": "<code>",
 "input": {"type", "state", "published", "year", "seasons": [...],
   "participants": [{"id", "country", "gender", "awt", "name_order"}   // solo
                    {"id", "pilots": [civl ids], "name_order"}],        // synchro
   "runs": [{"state", "flights": [{"participant", "score", "published"}]}]},
 "expected": {"final",
   "results": {"<key>": [{"participant", "score", "result_per_run": [{"rank", "score"}]}]},
   "internal_results": {"<key>": ["<participant>", ...]},
   "runs_results": [{"final", "results": {"<key>": [{"participant", "score"}]}}]}}
```

- Flights are in stored (first save) order; `score` is the stored score as
  legacy reloads it (`Round3`).
- `name_order` is the participant's position when sorted by display name;
  it replaces the name tie-break of `CompetitionResults.export`
  (`(-score, name)`).
- `results` is `results()` then `export()` (keys `overall`, season codes,
  `women`); `internal_results` is the order before export;
  `runs_results` are the run rankings (unpublished flights included).

`season-<code>.json` (one per non-deleted season, `Season.export_public`):

```
{"scope": "season", "code": "<code>",
 "input": {"code", "country", "competitions": ["<code>", ...]},  // legacy order (by name)
 "expected": {"type", "number_of_pilots", "number_of_teams",
   "results": {"overall": [{"participant", "score"}], "women": [...]},
   "competitions_results": {"<competition code>": [{"participant", "score"}]}} | null,
 "error": ""}
```

Competition data (state, published, participants) is in the competition
files. Season scores are legacy's unrounded float sums.

## Last generation (export 2026-10-18 21:28)

- 2310 flight cases (48 competitions), 9 legacy errors (8 "did not mark
  enough tricks", 1 "somes judges sent technical mark per trick while
  other(s) did not"); `TestGoldenFlights`: 0 mismatches.
- 48 competition and 20 season ranking files, no season error.
- Informational: 913 recomputed scores differ from the stored
  `final_marks.score` after `Round3`. Stored scores come from the legacy
  code of their time; causes (first matching, best effort):

| Cause | Flights |
|---|---|
| Pre-2024 trick snapshots without `solo`/`synchro` flags: today's legacy ignores every trick | 750 |
| Per-trick marks stored with `mark_type` null (kob-2024...): scored as classic now, technical 0 | 76 |
| Judges' marks averages differ (marks edited after scoring, judge level/weights) | 39 |
| Bonus percentage differs (stored per-trick technical marks, ignored tricks) | 27 |
| Technicity differs (ignored tricks) | 9 |
| Other note differences | 6 |
| Repetition malus differs (earlier flights edited afterwards) | 5 |
| Number of warnings differs (catalog position rules changed) | 1 |

Historical results are frozen on import (decision 0003); these differences
only matter if such flights were ever rescored.
