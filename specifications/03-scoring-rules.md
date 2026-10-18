# 03 - Scoring rules

> **Goal: a rewrite must reproduce legacy scores exactly.** Everything below was extracted from the legacy code (pydantic v2 / Python 3) and **checked by executing the legacy scoring code** (`Competition.calculate_score`, `flight_save`, `results`, `new_run`) against the 2025 trick catalog (`import/2025/tricks/*.json`) with the DB mocked. The numbers in the worked examples are the actual outputs.
>
> Source references are relative to the legacy repo root. The main implementation is `api/models/competitions.py:968-1513` (`calculate_score`).
> Rule references in comments (`§6.3.1.1`, `§6.5.1`, `§6.6.1`, `§7.2.2`, "7B") point to the FAI/CIVL Sporting Code Section 7B (paragliding aerobatics).

---

## 1. Vocabulary

| Term | Meaning |
|---|---|
| Solo / Synchro | Competition type. Solo: 1 pilot per flight. Synchro: a team of 2 pilots per flight; adds a `synchro` mark and tip-touch bonus. |
| AWT / AWQ | Acro World Tour / Acro World Qualifier. A pilot is **AWT for a year Y** iff `Y in pilot.awt_years`; the year is the competition's `start_date.year`. A solo competition can belong to an `awt-YYYY` season, an `awq-YYYY` season, or both ("mixed event"). |
| Mark type | How judges' technical marks are entered for a flight: `null` (classic: one technical mark per judge), `"awt"` or `"awq"` (one technical mark **per trick** per judge, `technical_per_trick`). Chosen by the operator per request (query param `mark_type`), **not** derived from the pilot (`api/routers/competitions.py:363-366`; manager tabs `TabFlightsSoloAWT.js` / `TabFlightsSoloAWQ.js`). |
| Unique trick | A catalog trick variant (direction + bonus combination), e.g. `/LM` = twisted left Misty Flip. See 02 §2.4. |
| Base trick / uniqueness | Repetition identity: two flown tricks are "the same" iff same `base_trick` **and** identical `uniqueness` list (`[direction]` or `["reverse", direction]`, `[]` for direction-less tricks). Twists/flips/etc. do not change identity: `/LM`, `LM/`, `LM\` are all the same as `LM`. |
| Warnings cat 1 / cat 2 | Operator-entered infringements (`flight.warnings`, `flight.warnings2`), plus automatic cat-1 warnings for trick-position violations. |
| DNS | `did_not_start` = true. |
| DSQ | Disqualified: score 0 for this and all later flights once the total warnings reach `warnings_to_dsq`. |

---

## 2. Configuration parameters and defaults

Each run carries its **own copy** of `CompetitionConfig`, taken from the competition config **at run creation** (`api/models/competitions.py:512-522`). Scoring uses `run.config` (or the competition config if the run index is invalid / negative, e.g. the single-flight simulator) (`competitions.py:996-1007`). Changing the competition config later does not affect existing runs; run configs are editable individually (`PATCH /competitions/{id}/runs/{i}/config`).

| Parameter | Default | Source | Used in |
|---|---|---|---|
| `warning` | 0.5 points | `api/core/config.py:58` | §6.4 penalty per cat-1 warning |
| `warning2` | 1 point | `config.py:59` | §6.4 penalty per cat-2 warning |
| `malus_repetition` | 13 (%) | `config.py:60` | §5.3 |
| `warnings_to_dsq` | 3 | `config.py:61` | §6.2 |
| `judge_weights` senior / certified / trainee | 100 / 100 / 20 | `config.py:62-64` | §4 |
| `mark_percentages.solo` technical / choreography / landing | 40 / 40 / 20 | `config.py:65-67` | §5.7 |
| `mark_percentages.synchro` technical / choreography / landing / synchro | **20 / 20 / 20 / 40** | `config.py:68-71` | §5.7 (the OpenAPI example in `competition_configs.py:36-43` wrongly shows 25/25/25/25) |
| `max_bonus_per_run` twist / reverse / flip | 5 / 3 / **2** | `config.py:72-74` | §5.4 (example in `competition_configs.py:68-74` shows flip 1) |

Hard-coded constants (not configurable per competition):

| Constant | Value | Source |
|---|---|---|
| Max tricks per trick **type** per run | `tumbling: 2`, `to infinite: 1` | `api/core/config.py:75-78` |
| Max per run for a bonus type absent from `max_bonus_per_run` (e.g. `other` = wing touch) | 10 | `competitions.py:1355` |
| Tip-touch bonus | 2 % per touch, max 6 % | `competitions.py:1481` |
| Technicity | mean of the 3 highest coefficients, padded with 1.0 | `competitions.py:1466-1467` |
| Choreography direction penalty | −1 point on the averaged choreography mark | `competitions.py:1236-1238` |
| Last-2-manoeuvres flip ban | automatic cat-1 warning | `competitions.py:1126-1127` |
| Starting-order and qualification rules | see §8 | |

---

## 3. Inputs of a flight score

* `flight.tricks`: ordered list of unique tricks (snapshotted from the catalog when the flight is saved: name, acronym, `technical_coefficient`, `bonus` (sum %), `bonus_types`, `base_trick`, `uniqueness`, `solo`, `solo_awt`, `synchro`, `types`).
* `flight.marks`: list of `JudgeMark {judge, technical?, technical_per_trick?, choreography?, landing?, synchro?}`. Input values `technical/choreography/landing/synchro` are rounded to 3 decimals on input (`api/models/marks.py:41-44`). No 0–10 range check (only `>= 0`).
* `flight.did_not_start`, `flight.warnings` (cat 1), `flight.warnings2` (cat 2), `flight.tip_touch_bonuses`.
* Context: competition type, run index, `run.config`, competition-level `repeatable_tricks`, competition seasons (AWT season?), pilot AWT status for `start_date.year`, previous runs' flights of the same pilot/team.
* `mark_type`: `null` | `"awt"` | `"awq"`.

---

## 4. Averaging helpers (exact semantics)

`api/core/utils.py:6-40`

```python
def average(values):                 # plain arithmetic mean
    if len(values) == 0: return 0.0
    return statistics.mean(values)    # (exception -> 0.0)

def weight_average(pairs):           # pairs = [(value, weight)]
    if len(pairs) == 0: return 0.0
    top = bottom = 0.0
    for (v, w) in pairs:
        # NB: legacy code has `if w <= 0: next` which is a no-op (bare builtin), so
        # zero/negative weights are NOT skipped.
        top += v * w
        bottom += w
    if bottom <= 0: return 0.0
    return top / bottom
```

Judge weight = `config.judge_weights[judge.level]`. The judge is looked up by `JudgeMark.judge` id; unknown judge -> HTTP 404. A judge appearing twice in `marks` counts twice. Judges do **not** need to belong to the run/competition judge list.

---

## 5. Flight score algorithm (`calculate_score`)

The steps below are in **exact execution order**; order matters (e.g. repetition detection happens before trick ignoring, direction check uses ignored tricks too, DSQ returns before averaging).

### 5.0 Full pseudo-code

```text
function calculate_score(comp, flight, run_i, is_awt_pilot, mark_type):
  mark = FinalMark(all numbers 0, judges_mark = {judge:"", technical:0, choreography:0, landing:0, synchro:0},
                   warnings = copy(flight.warnings), warnings2 = copy(flight.warnings2),
                   malus = 0, notes = [], mark_type = mark_type)

  # 1. DNS                                                   (competitions.py:992-994)
  if flight.did_not_start: notes += "Got 0 because of DNS"; return mark          # score 0

  # 2. config                                                (996-1007)
  run    = comp.runs[run_i] if 0 <= run_i < len(runs) else None
  config = run.config if run else comp.config

  # 3. judges                                                (1010-1016)
  judges = {m.judge: Judge.get(m.judge) for m in flight.marks}                   # 404 if unknown

  # 4. per-trick technical marks consistency                 (1019-1063)
  n_with_tpt = count(m in marks where m.technical_per_trick is not null)
  for m with tpt: if len(m.technical_per_trick) != len(flight.tricks): 400 "judge X did not mark enough tricks"
  if 0 < n_with_tpt < len(marks): 400 "somes judges sent technical mark per trick while other(s) did not"
  if n_with_tpt > 0:
     for each trick index i:
        vals = [m.technical_per_trick[i] for m in marks]
        if any null:
           set = non-null vals; if empty: 400 "not enough marks for <trick>"
           fill every null with average(set)          # PLAIN mean, not weighted; mutates the stored marks

  # 5. AWT/AWQ per-trick structures                          (1066-1088)
  if mark_type in {awt, awq}:
     for each trick i:
        trick.technical_marks = {judgeId: m.technical_per_trick[i]}   # for marks having tpt
        if empty: 400 "no technical mark for trick <acronym>"
        if mark_type == awt:
           trick.technical_mark = weight_average([(v, weight(judge)) for judge,v in technical_marks])
        else: trick.technical_mark = null

  # 6. position warnings (cat 1, automatic)                  (1094-1127)  -- on ALL flown tricks, index i 0-based, n = len(tricks)
  for i, trick in enumerate(flight.tricks):
     t = catalog Trick(trick.base_trick)                     # current catalog values!
     if t.first_maneuver    > 0 and i >= t.first_maneuver:        warn "must be (one of) the first N"
     if t.no_first_maneuver > 0 and i <  t.no_first_maneuver:     warn "can't be (one of) the first N"
     if t.last_maneuver     > 0 and i >= n - t.last_maneuver:     warn "must be (one of) the last N"   # inverted logic, see open questions
     if t.no_last_maneuver  > 0 and i >= n - t.no_last_maneuver:  warn "can't be (one of) the last N"
     if "flip" in trick.bonus_types and i >= n - 2:               warn "flips can't be performed as one of the last 2 maneuvers"
     (each warn appends a string to mark.warnings)

  # 7. previous warnings and DSQ                             (1136-1157)
  prev1 = prev2 = 0
  if run_i > 0 and run:
     for each run r with index < run_i:
        f = first flight of the same pilot (solo) / team (synchro) in r
        if f: prev1 += len(f.warnings); prev2 += len(f.warnings2)     # STORED operator warnings only (not auto ones)
     if prev1 + prev2 >= config.warnings_to_dsq: notes += "DSQ ... already had N warnings"; return mark   # score 0
  if len(mark.warnings) + len(mark.warnings2) + prev1 + prev2 >= config.warnings_to_dsq:
     notes += "DSQ ... got N warnings"; return mark                                                       # score 0

  # 8. weighted averages of judges' marks                    (1165-1200)
  for m in marks: w = weight(judge(m))
     if m.technical    != null: T  += (m.technical, w)
     if m.choreography != null: C  += (m.choreography, w)
     if m.landing      != null: L  += (m.landing, w)
     if synchro and m.synchro != null: S += (m.synchro, w)
  if T empty and mark_type not in {awt, awq}: 400 "not enough technical marks"
  if C empty: 400 "not enough choreography marks";  if L empty: 400 "not enough landing marks"
  if synchro and S empty: 400 "not enough synchro marks"
  J.technical = weight_average(T); J.choreography = weight_average(C); J.landing = weight_average(L)
  if synchro: J.synchro = weight_average(S)

  # 9. direction balance (solo only)                          (1212-1238)
  if type == solo and len(flight.tricks) > 1:
     dirs = [first element of trick.uniqueness that is in {right,left,opposite}, for each trick having one]   # ALL flown tricks
     if len(dirs) > 1:
        counts = Counter(dirs).values()
        if len(counts) == 1 or max(counts) - min(counts) > 1:
           if J.choreography >= 1: J.choreography -= 1; notes += "choreography mark has been lower by 1 point ..."

  # 10. repetition malus                                      (1253-1313)
  repeatable = [Trick.get(id).name for id in comp.repeatable_tricks]     # COMPETITION-level list (run list ignored)
  first_run = 0
  if type == solo:
     for i in 0..len(comp.runs)-1:                  # ALL runs, including runs after run_i
        p = runs[i].repetitions_reset_policy
        if p == all
           or (p == awt and is_awt_pilot and comp has an awt-* season)
           or (p == awq and not is_awt_pilot and comp has an awq-* season):
           first_run = i
  if len(runs) > 0 and run_i > 0:                    # vs previous runs
     for k, trick in enumerate(flight.tricks, start=1):
        if trick.base_trick in repeatable: continue
        for i in first_run .. run_i-1:
           f = flight of same pilot/team in runs[i]
           if f has t with t.base_trick == trick.base_trick and t.uniqueness == trick.uniqueness:
               mark.malus += config.malus_repetition; note "#k ... already performed in a previous run"; break (next trick)
  for k, trick in enumerate(flight.tricks, start=1): # within this run
     if trick.base_trick in repeatable: continue
     if any earlier trick t (index < k) in this flight with same base_trick and uniqueness:
         mark.malus += config.malus_repetition; note "#k ... already performed in this run"
  # NB: a trick repeated both in a previous run and earlier in this run gets 2 x malus.

  # 11. ignored tricks                                        (1322-1385)
  kept = []; n_bonus = {}; n_type = {}
  for i, trick in enumerate(flight.tricks):
     ignore = False
     if type == solo:
        if not trick.solo: ignore ("not allowed in a solo competition")
        elif is_awt_pilot and comp has an awt-* season and not trick.solo_awt: ignore ("not allowed in AWT solo competition")
     elif type == synchro and not trick.synchro: ignore
     if not ignore:
        for bt in trick.bonus_types:
           n_bonus[bt] += 1
           if n_bonus[bt] > config.max_bonus_per_run.get(bt, 10): ignore ("more than N <bt> tricks")
        for ty in trick.types:
           if ty in MAX_TRICKS_PER_TYPE:
              n_type[ty] += 1
              if n_type[ty] > MAX_TRICKS_PER_TYPE[ty]: ignore ("more than N '<ty>' tricks")
     if not ignore: kept.append(trick)
  # counters are incremented even for a trick that ends up ignored by another counter

  # 12. technical mark per trick                              (1396-1424)
  if mark_type == awt:
     for t in kept: if t.technical_mark is null: 400
     J.technical = average([t.technical_mark for t in kept])            # 0 if kept empty
  elif mark_type == awq:
     per_judge = {judge: [t.technical_marks[judge] for t in kept]}
     J.technical = weight_average([(average(list), weight(judge)) for judge, list in per_judge])

  # 13. technicity and bonus                                  (1431-1474)
  coefs   = [t.technical_coefficient for t in kept]
  bonuses = [t.bonus for t in kept if t.bonus > 0]
  details = [t.bonus * t.technical_mark / 10 for t in kept if t.bonus > 0 and t.technical_mark is not null]   # AWT only
  bonus_total = sum(bonuses)
  if details non-empty and sum(details) != bonus_total:
       note "Initial total bonus of X% has been lowered by technical mark to Y%"; bonus_total = sum(details)
  mark.technicity = average((sorted(coefs, desc) + [1.0, 1.0, 1.0])[0:3])
  mark.bonus_percentage = bonus_total

  # 14. tip touches (synchro)                                 (1480-1483)
  if type == synchro and flight.tip_touch_bonuses > 0:
     tt = min(flight.tip_touch_bonuses * 2, 6); note "tt% tip-touch bonus"; mark.bonus_percentage += tt

  # 15. components                                            (1485-1496)
  P = config.mark_percentages[type]
  mark.technical    = mark.technicity * J.technical * P.technical / 100
  mark.choreography = (J.choreography * P.choreography / 100) if len(kept) > 0 else 0
  mark.landing      = J.landing * P.landing / 100
  mark.synchro      = (J.synchro * P.synchro / 100) if synchro else 0
  mark.bonus        = (mark.technical + mark.choreography) * (mark.bonus_percentage - mark.malus) / 100
  mark.score        = mark.technical + mark.choreography + mark.landing + mark.synchro + mark.bonus

  # 16. warning penalties                                     (1501-1511)
  penalty = 0
  if len(mark.warnings)  > 0: penalty += (len(mark.warnings)  + prev1) * config.warning
  if len(mark.warnings2) > 0: penalty += (len(mark.warnings2) + prev2) * config.warning2
  mark.score = max(0, mark.score - penalty)
  return mark      # NOT rounded here (see §9)
```

### 5.1 Technicity (§6.3.1.1 "Technicity in Solo")

`technicity = mean of the 3 highest technical_coefficient among kept tricks, the list being padded with three virtual 1.0 tricks` (`competitions.py:1466-1467`, commit `a5a0255` "ensure to use 3 tricks for technicity calculation"). Hence:

| Kept coefficients | Technicity |
|---|---|
| 1.8, 1.75, 1.65, 1.6, 1.25 | (1.8+1.75+1.65)/3 = 1.733333 |
| 1.65, 1.65 | (1.65+1.65+1.0)/3 = 1.433333 |
| none (all ignored / no tricks) | 1.0 |

The same rule applies to synchro (no separate synchro technicity rule).

### 5.2 Judges' marks and weights

| Component | Formula | Notes |
|---|---|---|
| `J.technical` (classic) | `weight_average((m.technical, w_judge))` over marks having `technical` | |
| `J.technical` (`awt`) | `average over kept tricks of weight_average over judges of technical_per_trick` | §5.6 |
| `J.technical` (`awq`) | `weight_average over judges of (average over kept tricks of that judge's technical_per_trick)` | §5.6 |
| `J.choreography` | weighted average, then possibly −1 (direction rule §5.5) | |
| `J.landing` | weighted average | |
| `J.synchro` | weighted average (synchro only) | |

Default weights: senior 100, certified 100, trainee 20 -> a trainee counts 1/5 of a certified judge.

### 5.3 Repetitions and repeatable tricks (§6.5.1)

* A trick is a **repetition** if the same pilot (solo) / team (synchro) already flew a trick with the same `base_trick` and identical `uniqueness` (i.e. same direction and same reverse-ness) earlier **in the same run**, or in **any previous run** of the same competition starting from `first_run` (reset policy).
* "Each trick can be performed left/right and reversed without malus during the same competition" — i.e. `LM`, `RM`, `LMHR`, `LMH`, `RMHR`, `RMH` are 6 distinct identities; twisted/flip/etc. variants do not count as different.
* Each repeated trick adds `malus_repetition` (13 %) to `mark.malus` (at most once vs previous runs and at most once vs the current run, so up to 26 % for one trick).
* The malus is subtracted from the **bonus percentage** (not from the score directly): `bonus = (technical + choreography) * (bonus% − malus%) / 100`, which can be negative.
* Tricks whose `base_trick` is the **name** of a trick listed in the competition's `repeatable_tricks` are never counted as repetitions (the check `continue`s on them). Default list at competition creation = all catalog tricks with `repeatable=true` (2025: Stall, Tail Slide, Wingovers, Bro Hook*, Gearbox Damage, Pitch Pendulum Synchro, Wingovers Cruzados/Opposite).
* Repetition detection runs on **all flown tricks before trick ignoring** (commit `d5f32b9` "check repetition before ignoring tricks"): an ignored trick still triggers, and is subject to, repetition malus. Previous runs' flights are compared using all their stored tricks (including tricks ignored there, and DNS flights' tricks).
* **Reset policy** (`Run.repetitions_reset_policy`, solo only): `first_run` = index of the **last** run (scanning all runs of the comp) whose policy applies to this pilot: `all` (3) -> everyone; `awt` (1) -> AWT pilots when the comp has an `awt-*` season; `awq` (2) -> non-AWT pilots when the comp has an `awq-*` season; `none` (0) -> no effect. Runs before `first_run` are ignored for previous-run repetitions. Synchro competitions never reset.

### 5.4 Ignored tricks (not scored)

A kept trick contributes its coefficient to technicity, its bonus to the bonus %, its per-trick technical marks (AWT/AWQ), and makes `choreography` count. An ignored trick contributes nothing of these, **but** still counts for: position warnings (§6.1), direction balance (§5.5), repetition (§5.3), and as an index in notes.

| Reason | Condition | Source |
|---|---|---|
| Not allowed in solo | solo comp and `trick.solo` false (e.g. all `opposite` variants, synchro-only tricks). NB `solo == null` (very old data) is also treated as not allowed. | `competitions.py:1332-1335` |
| Not allowed for AWT | solo comp, pilot is AWT for `start_date.year`, comp has an `awt-*` season, `trick.solo_awt` false (2025: Misty to Misty) | `1337-1339` |
| Not allowed in synchro | synchro comp and `trick.synchro` false | `1341-1343` |
| Too many bonus tricks | for each bonus **type** of the trick, running count (over non-type-ignored tricks, in flown order) `> max_bonus_per_run[type]` (twist 5, reverse 3, flip 2, other 10) | `1349-1363` |
| Too many tricks of a type | for each trick `types` entry in {`tumbling` (2), `to infinite` (1)}, running count `> limit` (§6.5.1.2 2024) | `1372-1379` |

"If more than 5 manoeuvres are flown twisted, the extra manoeuvres will not be scored and their coefficients not taken into account for the determination of the average coefficient" (7B, comment at `1448-1451`).

### 5.5 Direction balance (solo only)

`competitions.py:1212-1238` (commit `627b207`, `e48d2cc` "ignore tricks direction evenness for synchro").

Judges always give the choreography mark as if directions were balanced; the system then lowers the **averaged** choreography mark `J.choreography` by exactly 1 point if:

* solo competition, more than 1 trick flown, more than 1 trick having a direction, and
* either all directional tricks go the same way, or `max(count per direction) − min(count per direction) > 1`;
* and only if `J.choreography >= 1`.

Direction of a trick = first element of `uniqueness` among `right/left/opposite` (all flown tricks, including ignored ones). Directionless tricks (Stall, MacFly, ...) are not counted. Note "min" is over directions present: L,L,L,R -> counts (3,1) -> diff 2 -> penalty; L,L,R -> (2,1) -> no penalty; L,L -> one direction -> penalty.

### 5.6 AWT / AWQ per-trick technical marking (§6.6.1 "Bonus calculation during AWT and CAT1 events")

Introduced by commits `2c87158`, `d30506c`, `d73fd88`. With `mark_type ∈ {awt, awq}` each judge sends `technical_per_trick` (one value per flown trick, `null` allowed) instead of `technical`:

1. Every judge must send a list of exactly `len(tricks)` entries (or nobody sends one).
2. Missing entries are replaced by the **plain** mean of the other judges' values for that trick (the filled values are persisted in the flight's marks).
3. `awt`: per-trick technical mark `TM_i = weight_average_j(tpt[j][i], w_j)`; `J.technical = mean_i∈kept(TM_i)`; **and each bonus is scaled by the trick's technical mark**: `bonus% = Σ_kept bonus_i × TM_i / 10` (only replaces the plain sum if different).
4. `awq`: `J.technical = weight_average_j(mean_i∈kept(tpt[j][i]), w_j)`; bonuses are **not** scaled.

Since all judges have a value for every trick after step 2, the two `J.technical` formulas are mathematically equal (both linear); **the only practical difference between `awt` and `awq` is the bonus scaling**. With `mark_type = null`, any `technical_per_trick` sent is filled/validated but otherwise unused, and a `technical` value is mandatory.

### 5.7 Final components and score

| Component | Solo | Synchro |
|---|---|---|
| technical | `technicity × J.technical × 40/100` | `technicity × J.technical × 20/100` |
| choreography | `J.choreography × 40/100` (0 if no kept trick) | `J.choreography × 20/100` (idem) |
| landing | `J.landing × 20/100` | `J.landing × 20/100` |
| synchro | 0 | `J.synchro × 40/100` |
| bonus | `(technical + choreography) × (bonus% − malus%) / 100` | same, bonus% includes tip touches |
| score | `max(0, technical + choreography + landing + synchro + bonus − penalty)` | same |

(Percentages = defaults of `run.config.mark_percentages[type]`.)

### 5.8 Tip touch (synchro, §6.6)

"In Synchro a bonus will be awarded each time the glider of a team touch each other. The said bonus will be 2%/touch with a maximum of 6% bonus earned during a run." `tip_touch_pct = min(2 × tip_touch_bonuses, 6)` added to `bonus_percentage` (`competitions.py:1480-1483`, commit `a62b813`). Ignored for solo.

---

## 6. Warnings, penalties, DSQ, DNS

### 6.1 Automatic cat-1 warnings (position rules)

Computed on every scoring from the **current catalog** trick (`Trick.get(trick.base_trick)`), with `i` = 0-based position among **all** flown tricks and `n` = number of flown tricks (`competitions.py:1094-1127`):

| Catalog field | Warning when | Message |
|---|---|---|
| `first_maneuver = N > 0` | `i >= N` | "`<name>` must be the first maneuver" / "must be one of the first N maneuvers" |
| `no_first_maneuver = N > 0` | `i < N` | "can't be the first maneuver" / "can't be one of the first N maneuvers" |
| `last_maneuver = N > 0` | `i >= n − N` (**i.e. when it IS in the last N — inverted**) | "must be the last maneuver" / "must be one of the last N maneuvers" |
| `no_last_maneuver = N > 0` | `i >= n − N` | "can't be the last maneuver" / "can't be one of the last N maneuvers" |
| any trick whose `bonus_types` contains `flip` | `i >= n − 2` | "flips can't be performed as one of the last 2 maneuvers (`<name>`)" (commit `24069bc`) |

2025 catalog usage: all `tumbling`-type tricks have `no_last_maneuver = 2`; Flat Stall to Infinity Tumbling has `first_maneuver = 1`; no trick uses `last_maneuver`. A tumbling-with-flip trick in the last 2 gets **two** warnings.

These automatic warnings are added to `final_marks.warnings` only — **not** to `flight.warnings` — so they count for this flight's penalty and DSQ check but **never as "previous warnings"** in later runs.

### 6.2 DSQ

`warnings_to_dsq` (default 3) counts cat-1 + cat-2 together:

* If the stored operator warnings of the pilot's/team's flights in all previous runs (`prev1 + prev2`) reach the threshold -> score 0, note "Pilot has been DSQ because he/she already had 3 warnings".
* Else if `len(final.warnings) (operator + automatic) + len(warnings2) + prev1 + prev2 >= threshold` -> score 0, note "Pilot has been DSQ he/she's got 3 warnings".

A DSQ flight still appears in rankings with score 0 (no special status). The DSQ return happens **before** averaging, so `judges_mark`, technicity, etc. are all 0 and no mark validation errors are raised.

### 6.3 DNS

`did_not_start = true` -> immediate return, score 0, note "Got 0 because of DNS"; marks are not validated. The DNS flight's operator warnings still count in later runs.

### 6.4 Penalty points (§7.2.2)

```
penalty = 0
if this flight has >= 1 cat-1 warning (operator or automatic):
    penalty += (n_cat1_this_flight + prev1) * warning        # 0.5
if this flight has >= 1 cat-2 warning:
    penalty += (n_cat2_this_flight + prev2) * warning2       # 1
score = max(0, score - penalty)
```

The deduction is **cumulative**: when a pilot gets a new warning, all his previous warnings of the same category are deducted again in this run (commit `ffca768` "fix 2nd warning point deduction" changed `- previous` into `+ previous`). A run without a new warning of a category gets no deduction for that category. The notes text for cat 2 prints the cumulated `penalty` (cat 1 + cat 2).

---

## 7. Run results and ranking

`Competition.run_results(run_i, published_only=True)` (`competitions.py:625-657`):

1. Take the run's flights (in stored order = order of first save); if `published_only`, skip unpublished flights. One entry per pilot/team.
2. Sort **ascending** by `final_marks.score` (stable), then **reverse** -> descending; **ties end up in reverse flight order** (the later-saved flight ranks first among equals).
3. `results = {"overall": [...]}` + sub-rankings (§7.1).
4. `final = run.state == closed and all flights published` (computed over all flights even if `published_only`).

Ranks are purely positional (index + 1); **no shared ranks for ties**. No tie-break other than the ordering above (run results are not re-sorted at export; the HTML export re-sorts by `-score` only, the xlsx export by `(-score, pilot/team name)`).

The admin endpoint `GET /competitions/{id}/results/{i}` defaults to `published_only=true`; competition results always use `published_only=false` (§8).

### 7.1 Sub-rankings (`create_sub_results`, `competitions.py:853-890`)

For each season code `s` in `competition.seasons` (in that order), a sub-list `results[s]` is built from `results["overall"]`:

| Season code | Solo | Synchro |
|---|---|---|
| `awt-YYYY` (regex `^awt-\d{4}$`) | pilots with `is_awt(comp.start_date.year)` | `results[s] = results["overall"]` (same list) |
| `awq-YYYY` | pilots **not** AWT that year | same as overall |
| any other `^\w\w\w-\d{4}$` (e.g. `fra-2025`) | pilots whose `country == s[0:3]` | same as overall |

Each solo sub-list is re-sorted ascending by score and reversed (ties reversed again relative to the overall order). Note the year used is the **competition** year, not the season's `year` field.

**Women ranking**: for solo only, if at least **3** of the pilots present in the overall results have `gender == woman`, `results["women"]` = women in score order. (Same rule at season level, §10.)

---

## 8. Competition results (overall)

`Competition.results(limit=-1)` (`competitions.py:762-851`):

```text
final = (comp.state == closed)
overall = {}                                   # insertion-ordered dict
for i, run in enumerate(runs):
   if limit >= 0 and i > limit: break          # limit = last run index to include (export "after run N")
   rr = run_results(i, published_only=False)   # UNPUBLISHED flights are included
   if not rr.final: final = False
   for j, flight in enumerate(rr.results["overall"]):           # descending order
       entry = overall.setdefault(pilot_or_team, {score: 0, result_per_run: []})
       entry.score = round(entry.score + flight.final_marks.score, 3)       # rounding at EVERY addition
       entry.result_per_run.append({rank: len(rr.overall) - j, score: flight.score})   # see open question (inverted)
overall_list = sort ascending by score (stable) then reverse
results = {"overall": overall_list} + sub-rankings (§7.1) with pilots = those of overall_list
for each run result: recompute its sub-rankings with the same competition-level pilot list
for each non-overall key k, for each entry e in results[k]:
    e.result_per_run = [ {rank: position of e in run.results[k] + 1, score} for each run where present ]
```

Key properties:

* **All runs count**: the competition score is the plain **sum of all run scores** (no discard, no best-N). A pilot missing from a run simply gets nothing for it (and has fewer `result_per_run` entries).
* **Unpublished flights are counted** in competition totals (only `final` signals incompleteness), including in public exports.
* **Rounding**: `score_n = round(score_{n−1} + flight_score_n, 3)` (commit `02aa0dc`), using Python `round` (§9).
* **Ordering**: internal order = descending score, ties in reverse insertion order. **At export** (`CompetitionResults.export`, `api/models/results.py:150-154`) every list in `results` is re-sorted by `(−score, pilot.name)` (solo) or `(−score, team.name)` (synchro): **the only tie-break is alphabetical name**. Ranks shown to users = position in that exported list (e.g. `PilotWithResults`, `api/models/pilots_with_results.py:61-74`).
* Sub-ranking objects are the **same objects** as in overall; therefore the per-run ranks displayed in the overall table are overwritten by the per-run ranks of the last sub-ranking containing the pilot (season order, then `women`). Only pilots in no sub-ranking keep the (inverted) `len − j` value.

### 8.1 Starting order and qualification for a new run

`Competition.new_run(pilots_to_qualify=0, repetitions_reset_policy=none)` (`competitions.py:464-526`):

| Case | Participants (in starting order) |
|---|---|
| First run, solo | `competition.pilots`, which `check()` keeps sorted by CIVL `rank` **descending** (worst-ranked / unranked 9999 first, best-ranked last) (`competitions.py:172-173`). |
| First run, synchro | `competition.teams` as stored (manual order). |
| Later runs | participants of the **previous run**, sorted by their position `p` in the current overall results **descending** (`key = −p`): the current leader starts **last**. Participants with no result (`p = −1`) go **last**. |
| Later runs, solo comp with exactly one `awt-*` and one `awq-*` sub-ranking ("mixed event") | position computed in the concatenated list `awt_results + awq_results`, hence AWQ pilots fly first (worst to best), then AWT pilots (worst to best) — never mixed (commit `6841d9a`). |
| `pilots_to_qualify = N > 0` | keep `participants[0:N]` of the list above. |

Overall positions used here are the internal (non-name-tie-broken) order and include unpublished flights.

### 8.2 Worked example — competition with mixed AWT/AWQ (executed)

Comp seasons `["awt-2025","awq-2025"]`, pilots 1 (AWT, CIVL rank 5), 2 (AWQ, rank 1), 3 (AWT, rank 3), 4 (AWQ, rank 2). Two senior/certified judges give technical `t`, choreography 6, landing 6; tricks `LM, RMC, LS` (run 1) and `RM, LMC, RS` (run 2) -> technicity (1.7+1.65+1.25)/3 = 1.533333, no bonus, directions balanced, no repetition (directions differ) -> `score = 1.533333 × t × 0.4 + 6×0.4 + 6×0.2 = 0.613333 t + 3.6`.

| | t run 1 | score run 1 | t run 2 | score run 2 (stored, 3 d.) | Overall `round(Σ,3)` |
|---|---|---|---|---|---|
| P1 (AWT) | 7 | 7.893 | 8 | 8.507 | 16.400 |
| P2 (AWQ) | 8 | 8.507 | 6 | 7.280 | 15.787 |
| P4 (AWQ) | 7 | 7.893 | 7 | 7.893 (unpublished) | 15.786 |
| P3 (AWT) | 6 | 7.280 | 7 | 7.893 | 15.173 |

* Run 1 starting order (`check()` sorts by rank desc): `[1(rank 5), 3(rank 3), 4(rank 2), 2(rank 1)]`.
* Run 1 overall: `[2, 4, 1, 3]` (P1 and P4 tie at 7.893: P4 saved later -> first).
* Run 2 starting order: `awt=[1,3]` + `awq=[2,4]` -> positions 1,2,3,4 -> sorted by −position -> `[4, 2, 3, 1]`.
* P4's unpublished run-2 flight counts in the overall (15.786) but `final = false`; `GET .../results/1` (published only) lists `[1, 3, 2]`.
* P2 15.787 vs P4 15.786: identical sums 8.506667+7.28 vs 7.893333+7.893333 differ only through per-step rounding (8.507 + 7.28 = 15.787; 7.893 + 7.893 = 15.786).
* Then `new_run(pilots_to_qualify=3)` returns `[4, 2, 3]` — the 3 **worst**, see open questions.

---

## 9. Rounding rules (critical for exact reproduction)

| Where | Rule | Source |
|---|---|---|
| Judge input marks `technical`, `choreography`, `landing`, `synchro` | `round(v, 3)` at input validation; explicit `null` becomes **0** | `api/models/marks.py:41-44`, `api/core/utils.py:37-40` |
| `technical_per_trick` values | not rounded | |
| Scoring computation | **full float precision**, no intermediate rounding | `competitions.py:968-1513` |
| `FinalMark` fields (`technicity`, `bonus_percentage`, `technical`, `choreography`, `landing`, `synchro`, `bonus`, `score`, and `judges_mark.*`) | `round(v, 3)` **only when the model is validated**: i.e. in the API response (`FinalMarkExport`) and **when the competition is re-loaded from MongoDB**. Values are *stored* with full precision. | `marks.py:114-121`, `69-95` |
| Consequence | every consumer that reads a competition from the DB (results, rankings, next-run computations) sees **flight scores rounded to 3 decimals** | |
| Competition total | `round(previous_total + flight_score, 3)` at each run | `competitions.py:793, 805` |
| `RunResultSummary.score`, `CompetitionPilotResults.score` | `round(v, 3)` at construction | `api/models/results.py:59, 82` |
| Season total | plain float sum of competition totals, **not rounded** (may show `x.y00000000001`) | `api/models/seasons.py:302-304` |
| Displays | SVG/HTML: `"%.3f"` | `api/controllers/competitions.py:250` |

`round` is Python's built-in: correctly rounded on the exact binary value, ties-to-even on exact halves (e.g. `round(0.0625, 3) = 0.062`, `round(2.0005, 3) = 2.001` because 2.0005 is slightly above in binary). A rewrite in another language must reproduce this (do **not** use `toFixed` / half-up decimal rounding blindly).

Recommendation for an exact port: compute in IEEE-754 doubles in the same operation order, then apply Python-equivalent `round(x, 3)` at the same points (flight score when persisted/read, running sum per run, etc.).

---

## 10. Season standings

`Season.export_public` (`api/models/seasons.py:244-343`):

```text
competitions = all non-deleted competitions whose seasons[] contains season.code (sorted by name)
all competitions must share the same type, else HTTP 500
for comp in competitions:
   cr = comp.export_public_with_results()          # raises 404 if comp unpublished (see open questions)
   if comp.published and comp.state == closed and cr.results.final:
      for res in cr.results.results[season.code]:   # the comp's sub-ranking for this season, export order (−score, name)
          if solo and season.country and res.pilot.country != season.country: skip
          total[pilot_or_team] += res.score          # competition total (3 d.), plain float sum
overall = [(pilot_or_team, total)] in first-appearance order, sorted by total DESC (stable: ties keep first-appearance order)
results = [overall] if non-empty
women = pilots of overall with gender == woman, if >= 3 -> results.append({type: "women", ...})   (Season.women_results, 236-242)
```

* Only **published, closed and final** competitions count; **every** such competition counts fully (no best-N, no discard, no coefficient per competition).
* For a solo season the per-competition contribution is the pilot's **competition total** within that comp's sub-ranking for the season code: an `awt-YYYY` season only receives AWT pilots' totals, `awq-YYYY` non-AWT pilots', a country season only pilots of that country.
* Synchro: the team total is added for each competition (sub-ranking = overall).
* Women ranking (commit `979862e`): exists only if >= 3 women in the season overall; women keep their overall order and scores (`api/tests/test_season_women.py`). Never for synchro.
* `competitions_results[comp.code]` = the list of `(pilot|team, score)` contributing from each counted comp.
* `number_of_pilots` = distinct pilots counted (synchro: both team members), `number_of_teams` = distinct teams.
* Season ranks shown (e.g. on a pilot page) = position in the sorted list (`api/models/pilots_with_results.py:80-99`); ties have different ranks.

Illustrative example (hand-computed, solo AWT season, two counted comps): A: 16.4 (comp 1) + 15.173 (comp 2) = 31.573; B: 15.787 + 16.0 = 31.787 -> B first. If two pilots tie exactly, the one that appeared first (earlier comp by name, then higher in that comp's exported order) ranks first.

---

## 11. Exports with derived points (xlsx)

`CompCtrl.comp_to_xlsx` / `run_to_xlsx` (`api/controllers/competitions.py:21-222`), used for CIVL/ranking submissions:

* Solo comp in any `aw[tqs]*` season: rows = AWT pilots sorted `(−score, name)` then AWQ pilots sorted `(−score, name)`; otherwise overall sorted `(−score, name)`.
* "Rank" column = 1, 2, 3... over the whole concatenated list.
* "Score" column = **ranking points, not the competition score**: starts at **100** for the first row and decreases by 1 per row; in an `aw[tqs]` solo comp, the **first AWQ pilot resets to 80** (then 79, 78...).
* Run export requires the run to be final (400 otherwise).
* `GET /competitions/{id}/results/export?limit_run=k` limits to runs `0..k`; non-xls exports drop `overall` for tour comps; comps without seasons keep only `overall` and `women`.

---

## 12. Worked flight examples (executed against legacy code)

Common setup unless stated: judges **S** (senior, w=100), **C** (certified, w=100), **T** (trainee, w=20); default config; solo comp in season `awt-2025`; pilot 1 is AWT 2025.

Marks: S = (tech 7, choreo 6.5, landing 8), C = (7.5, 7, 9), T = (6, 5, 5).

```
J.technical    = (7×100 + 7.5×100 + 6×20) / 220 = 1570/220 = 7.136364
J.choreography = (6.5×100 + 7×100 + 5×20) / 220 = 1450/220 = 6.590909
J.landing      = (8×100 + 9×100 + 5×20) / 220 = 1800/220 = 8.181818
```

### Example A — run 1, classic marking, 1 operator warning

Tricks: `/LM` (twisted left Misty Flip, 1.65, +2.5 %), `RMHR` (right Misty to Helicopter reverse, 1.75, +3 %), `LT` (left Tumbling, 1.8), `RS` (right SAT, 1.25), `FS` (Stall, 1.6, repeatable); warnings = `["late at briefing"]`.

| Step | Value |
|---|---|
| Position rules | LT at index 2 of 5, `no_last_maneuver=2` -> OK (warns only for i >= 3) |
| Directions | L, R, L, R -> balanced, no choreo change |
| Technicity | (1.8 + 1.75 + 1.65)/3 = 1.733333 |
| technical | 1.733333 × 7.136364 × 0.40 = 4.947879 |
| choreography | 6.590909 × 0.40 = 2.636364 |
| landing | 8.181818 × 0.20 = 1.636364 |
| bonus % | 2.5 + 3 = 5.5 ; malus 0 |
| bonus | (4.947879 + 2.636364) × 5.5 / 100 = 0.417133 |
| subtotal | 9.637739 |
| penalty | 1 cat-1 warning × 0.5 = 0.5 |
| **score** | **9.137739** -> stored/exported **9.138** |

### Example B — run 2, repetitions + cumulative warning

Tricks: `LM, RMHR, RT, LS, FS, /LMC`; warnings = `["box"]`; previous run had 1 cat-1 warning.

| Step | Value |
|---|---|
| Repetitions vs run 1 | `LM` = `/LM` (Misty Flip, [left]) -> +13 %; `RMHR` -> +13 %; `RT`≠`LT`, `LS`≠`RS`; `FS` repeatable; `/LMC` new -> malus 26 % |
| Directions | L, R, R, L, L -> counts (3, 2) -> diff 1 -> OK |
| Technicity | (1.8 + 1.75 + 1.7)/3 = 1.75 |
| technical | 1.75 × 7.136364 × 0.4 = 4.995455 |
| bonus % | 3 (RMHR) + 3.5 (/LMC) = 6.5 |
| bonus | (4.995455 + 2.636364) × (6.5 − 26)/100 = −1.488205 |
| subtotal | 4.995455 + 2.636364 + 1.636364 − 1.488205 = 7.779977 |
| penalty | (1 current + 1 previous) × 0.5 = 1.0 |
| **score** | **6.779977** -> 6.780 |

### Example B2 — DSQ

Same pilot, run 2, with 2 warnings `["box","x"]`: 2 + 1 previous = 3 >= 3 -> **score 0**, note "Pilot has been DSQ he/she's got 3 warnings", all components 0.

### Example B3 — unbalanced directions

Pilot 2 (AWQ), run 1, tricks `LM, LMC, LS` (all left): choreography 6.590909 − 1 = 5.590909; technicity (1.7+1.65+1.25)/3 = 1.533333; technical 4.376970; choreography 2.236364; landing 1.636364; **score 8.249697**.

### Example C — AWT vs AWQ marking (same input)

Tricks `/LM` (+2.5 %), `RMHR` (+3 %), `LT` (last position -> automatic warning "left Tumbling can't be one of the last 2 maneuvers"). Marks: S `tpt=[8,6,7]`, choreo 7, landing 8; C `tpt=[7,null,8]`, choreo 6, landing 7; T `tpt=[5,5,5]`, choreo 5, landing 5.

* Fill-in: C's missing value for RMHR = mean(6, 5) = **5.5** (plain mean).
* Per-trick weighted marks: /LM (800+700+100)/220 = 7.272727; RMHR (600+550+100)/220 = 5.681818; LT (700+800+100)/220 = 7.272727.
* `J.technical` = 6.742424 in both modes (AWT: mean of per-trick; AWQ: per-judge means 7, 6.833333, 5 weighted).
* J.choreography = (700+600+100)/220 = 6.363636; J.landing = (800+700+100)/220 = 7.272727; technicity 1.733333.

| | AWT | AWQ |
|---|---|---|
| bonus % | 2.5×7.272727/10 + 3×5.681818/10 = **3.522727** (note "Initial total bonus of 5.5% has been lowered by technical mark to 3.523%") | **5.5** |
| technical / choreo / landing | 4.674747 / 2.545455 / 1.454545 | same |
| bonus | 0.254348 | 0.397111 |
| penalty | 0.5 (automatic warning) | 0.5 |
| **score** | **8.429096** | **8.571859** |

With `mark_type = null` and only `technical_per_trick`: HTTP 400 "not enough technical marks".

### Example D — limits and in-run repetition

One senior judge (7, 7, 7). Tricks: `MCF` (MacFly 2.1, to infinite+tumbling), `/LM`, `/RM`, `LM/`, `RM/`, `/LMC`, `/RMC`, `RTF` (right Tumbling flip 1.8, +5 %, flip, tumbling).

* In-run repetitions: `LM/` repeats `/LM`, `RM/` repeats `/RM` -> malus 26 %.
* Twist count: `/LM`1, `/RM`2, `LM/`3, `RM/`4, `/LMC`5, `/RMC`**6 > 5 -> ignored**.
* Tumbling count: MCF 1, RTF 2 -> OK; to infinite 1 -> OK.
* Automatic warnings: `RTF` at last position -> "can't be one of the last 2" + "flips can't be ... last 2" = 2 warnings -> penalty 1.0 (2 < 3, no DSQ).
* Technicity (kept): 2.1, 1.8, 1.7 -> 1.866667. Bonus % = 2.5+2.5+3.5+3.5+3.5+5 = 20.5.
* technical 5.226667, choreo 2.8, landing 1.4, bonus (8.026667)×(20.5−26)/100 = −0.441467 -> **score 7.9852**.

### Example E — synchro

Synchro comp, judges S (7, 6, 8, synchro 7) and C (8, 7, 6, synchro 8); 4 tip touches. Tricks `OMH` (opposite Misty to Helicopter 1.75), `/LM` (+2.5 %), `LM`, `FU` (Fusion 2.2), `LMC`.

* J = technical 7.5, choreography 6.5, landing 7, synchro 7.5; no direction rule in synchro.
* Repetition: `LM` after `/LM` -> 13 %.
* Technicity: (2.2 + 1.75 + 1.7)/3 = 1.883333. Bonus % = 2.5 + min(4×2, 6) = 8.5.
* technical 1.883333×7.5×0.2 = 2.825; choreography 1.3; landing 1.4; synchro 7.5×0.4 = 3.0; bonus (4.125)×(8.5−13)/100 = −0.185625 -> **score 8.339375**.

### Example F — AWT-forbidden trick and reset policy

Comp seasons `awt-2025, awq-2025`; P1 AWT, P2 AWQ; one senior judge (7,7,7).

* Run 1, tricks `LM, RM, MM` (Misty to Misty, `solo_awt=false`, 1.75): P1 -> MM ignored, technicity (1.65+1.65+1)/3 = 1.433333, score 8.213333. P2 -> MM kept, technicity 1.683333, 1 cat-2 warning -> 8.913333 − 1 = 7.913333.
* Run 2 created with `repetitions_reset_policy = awt`, tricks `LM, RM`: P1 (AWT) -> no repetition malus (history reset); with 1 cat-1 warning -> 8.213333 − 0.5 = 7.713333. P2 (AWQ) would get 26 % malus, but with `warnings=["w1"]`, `warnings2=["c2"]` its total is 1 + 1 + 1 previous = 3 -> DSQ, 0.

### Example G — explicit null mark quirk

Marks S (technical 8) and C (`"technical": null` sent explicitly): `J.technical = (8×100 + 0×100)/200 = 4.0`. If C omits the key: `J.technical = 8.0`.

---

## 13. Simulators

| Endpoint | Behaviour | Source |
|---|---|---|
| `POST /scores/simulate/{type}` (auth) | Intended: score one flight with a throw-away competition. **Broken**: calls `Trick.get_unique_trick(name, solo=, synchro=)` whose signature has no such kwargs -> TypeError/500. | `api/controllers/scores.py:27-72` |
| `POST /public/simulate/competition/{type}?reset_repetitions_frequency=k` | Builds an in-memory open comp (no seasons, default config, repeatable tricks from catalog), simulator pilot −1000000 (even id => AWT for the current year, but no AWT season so irrelevant), one new run per submitted flight (every k-th run gets reset policy `all`), all marks re-attributed to judge `"simulator"` (senior); returns the flight of each run with `pilot = 0`. Useful as a reference oracle. | `api/controllers/scores.py:75-137` |

---

## 14. Open questions, apparent bugs and dead code

| # | Topic | Observation | Source |
|---|---|---|---|
| 1 | `pilots_to_qualify` keeps the **worst** pilots | New-run participants are sorted with the leader **last** (starting order), then `[0:N]` keeps the first N = the N worst-ranked, contrary to the comment "only keep the N first (best)". Verified by execution. Likely the UI/operators work around it (or never use it). | `competitions.py:494-510` |
| 2 | Previous run's starting order is mutated | `pilots = self.runs[-1].pilots; pilots.sort(...)` sorts the previous run's list **in place**, so the stored starting order of run n−1 is overwritten by run n's order when the comp is saved. | `competitions.py:495-499` |
| 3 | Inverted per-run rank in overall | `RunResultSummary.rank = len − j` gives rank N to the winner; masked when the pilot is in a sub-ranking (objects shared, ranks overwritten by the sub-ranking's ranks), visible for comps without seasons/women. | `competitions.py:779-781, 825-844` |
| 4 | Overall per-run ranks overwritten by sub-rankings | Same objects in `overall` and sub-lists -> the overall table shows AWT/AWQ/women ranks per run, not overall ranks. | `competitions.py:825-844` |
| 5 | Unpublished flights in competition totals | `results()` uses `published_only=False`; public competition and season exports therefore expose unpublished scores (only `final=false` flags it). Intentional? | `competitions.py:772`, `316` |
| 6 | `last_maneuver` logic inverted | Warns when the trick **is** in the last N (same condition as `no_last_maneuver`). Unused in 2025 catalog. | `competitions.py:1112` |
| 7 | Cumulative warning deduction | Each new warning re-deducts all previous ones of the same category (0.5, then 1.0, ...). Matches commit `ffca768` intent but should be confirmed against 7B §7.2.2. Cat-2 note prints the cat1+cat2 total. | `competitions.py:1501-1507` |
| 8 | Automatic warnings not carried over | Position warnings are not stored in `flight.warnings`, so they never count as previous warnings / toward a later DSQ. Intentional? | `competitions.py:985, 1147-1148` |
| 9 | Reset policy scans **all** runs | `first_run` may be set by a run created **after** `run_i`; re-saving an earlier flight then ignores previous-run repetitions. | `competitions.py:1261-1272` |
| 10 | Run-level `repeatable_tricks` unused | Editable per run but scoring uses the competition list. A deleted trick in the comp list makes `Trick.get` raise 404 (the `is not None` guard is useless). | `competitions.py:1253-1257` |
| 11 | Run-level `judges` unused | Marks from any existing judge are accepted; duplicates counted twice; no check that every run judge marked. | `competitions.py:1010-1016` |
| 12 | Explicit `null` mark becomes 0 | Pydantic v1-style validator on Optional fields converts explicit `null` to 0 (counted). Only omitted keys mean "no mark". The manager deletes keys (OK), other clients may not. | `marks.py:41-44`, `utils.py:37-40` |
| 13 | No mark range validation | Only `>= 0`; > 10 accepted. | `marks.py:35-39` |
| 14 | Scores not recomputed downstream | Editing an earlier flight (warnings, tricks) does not update later runs' malus/penalty/DSQ. | `competitions.py:718-760` |
| 15 | Flights savable in any run/comp state | `flight_save` checks neither run state nor comp state (a closed comp can be changed). | `competitions.py:718-760` |
| 16 | Position rules use the **current** catalog | `Trick.get(trick.base_trick)` at scoring time; also fails (404) if the catalog trick was deleted/renamed, whereas coefficients/bonuses come from the flight snapshot. | `competitions.py:1095` |
| 17 | `weight_average` `next` no-op | Zero/negative weights not skipped (harmless for 0, wrong for negative). | `utils.py:22-23` |
| 18 | Synchro default percentages | Code 20/20/20/40 vs documented example 25/25/25/25; flip max 2 vs example 1. Which is the rule? | `config.py:68-74`, `competition_configs.py:31-74` |
| 19 | Choreography reduction | Lowers the **averaged** mark by 1 only if `>= 1` (never below 0); computed before trick ignoring and counting ignored tricks' directions. | `competitions.py:1212-1238` |
| 20 | Twist cap also caps bonus count per type, but the counter increments for tricks already ignored by another type limit | Order-dependent; e.g. a 6th twisted trick that is also a flip still increments the flip counter. | `competitions.py:1349-1363` |
| 21 | Season export crashes on unpublished comps | `export_public_with_results` raises 404 for an unpublished comp, and season export iterates all comps of the season (published or not). | `seasons.py:256-257`, `competitions.py:313-314` |
| 22 | Non-AWT/AWQ season codes treated as countries | Any `aaa-YYYY` code other than awt/awq filters pilots by `country == aaa` (e.g. a hypothetical `wch-2025` would be empty). | `competitions.py:875-881` |
| 23 | AWT/AWQ by competition year | Sub-rankings use `comp.start_date.year`, not `season.year`. | `competitions.py:862, 869` |
| 24 | Ties | No shared ranks anywhere; tie-breaks differ by place (reverse save order in runs, alphabetical name in exported competition results, first appearance in seasons). | §7, §8, §10 |
| 25 | `ScoreCtrl.simulate_score` broken | Wrong `get_unique_trick` call signature. | `api/controllers/scores.py:33-36` |
| 26 | `Trick.tricks_constraints` | Declared ("tricks that cannot be performed during the same competition"), never enforced. | `api/models/tricks.py:77` |
| 27 | Duplicate `wing touch` bonus definition | `T` and `WT`; the first wins in generation. | `api/core/config.py:47-48` |
| 28 | `hardcore` vs `hardcore entry` | Same acronym `HC`; legacy `hardcore` kept "until migration". | `api/core/config.py:43-44` |
| 29 | Starting-order HTML export typo | `pilot.awt_year` (AttributeError) for `awt-*` seasons. | `api/routers/competitions.py:507` |
| 30 | `Event.logo_url` built from `image`; `Event.get(deleted=True)` adds a bogus `id: "deleted"` filter and non-deleted lookups do not filter `deleted` | Cosmetic / data bugs. | `api/models/events.py:125, 148-149` |
| 31 | `CompetitionNew.name/location` `min_len` and `Run` `min_len` | Not pydantic v2 constraints -> not enforced. | `competitions.py:97, 101`, `runs.py:49-51` |
| 32 | Pilot `is_awt` default year | Default argument evaluated once at import (`datetime.now().year`); all domain calls pass the year explicitly, except `PilotCtrl`/routers defaults. | `api/models/pilots.py:103` |
| 33 | Samples are obsolete | `api/samples/*.yml|json` describe a pre-implementation format (`Runs`, `nb_pilots_to_keep_for_next_run`, `apply_penalties`) that the code does not support. | `api/samples/` |
| 34 | Tests | Only run/comp close preconditions (`test_close.py`) and season women ranking (`test_season_women.py`) are tested; **no scoring test exists**. The rewrite should build a golden-test corpus from production data (recompute every stored flight with the new engine and compare with stored `final_marks`, after Python-style 3-decimal rounding). | `api/tests/` |
