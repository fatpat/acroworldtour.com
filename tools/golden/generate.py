"""Generate the scoring golden corpus from a production export.

Runs the legacy Python engine (legacy/legacy/api, read-only) with MongoDB
replaced by an in-memory copy of the export, and writes:

  testdata/golden/flights/<competition code>.json   one case per flight
  testdata/golden/rankings/competition-<code>.json  legacy results()/export()
  testdata/golden/rankings/season-<code>.json       legacy Season.export_public

Usage (see tools/golden/README.md):

  python -I tools/golden/generate.py --backup mongo.backup.tar.gz \
      --legacy legacy/legacy --out testdata/golden

Must run with the legacy virtualenv's interpreter, from a working
directory outside the repository (-I keeps it from importing local files).
"""

import argparse
import asyncio
import json
import logging
import os
import re
import sys
import tarfile
import tempfile
import warnings
from collections import Counter

warnings.filterwarnings("ignore")

# Anonymised operator warning text (global constraints: no free text).
WARNING_TEXT = "w"


def naive_sum(values, start=0):
    """Left-to-right float summation, as Python 3.10's sum() (production).

    Python >= 3.12 uses compensated summation, which changes the last digits
    of AWT bonus totals; controller ruling for Task 7.
    """
    total = start
    for v in values:
        total = total + v
    return total


# --------------------------------------------------------------------------
# In-memory MongoDB replacement (only what the legacy getters use)
# --------------------------------------------------------------------------

def _norm(v):
    # ObjectId -> str so that ids compare like the stored export strings.
    if type(v).__name__ == "ObjectId":
        return str(v)
    return v


def _get_path(doc, path):
    """Values at a dotted path; arrays of sub-documents are flattened."""
    cur = [doc]
    for part in path.split("."):
        nxt = []
        for c in cur:
            if isinstance(c, list):
                for e in c:
                    if isinstance(e, dict) and part in e:
                        nxt.append(e[part])
            elif isinstance(c, dict):
                nxt.append(c.get(part))
        cur = nxt
    return cur


def _match_value(values, cond):
    if isinstance(cond, dict) and "$in" in cond:
        wanted = [_norm(x) for x in cond["$in"]]
        return any(_eq(v, w) for v in values for w in wanted)
    cond = _norm(cond)
    if cond is None:
        # Mongo: {"f": None} matches a null or missing field.
        return not values or any(v is None for v in values)
    return any(_eq(v, cond) for v in values)


def _eq(v, w):
    if isinstance(v, list):
        return any(_eq(e, w) for e in v)
    return v == w


def _match(doc, query):
    for k, cond in query.items():
        if k == "$or":
            if not any(_match(doc, q) for q in cond):
                return False
        elif not _match_value(_get_path(doc, k), cond):
            return False
    return True


class _Cursor:
    def __init__(self, docs):
        self.docs = docs

    def sort(self, key, direction=1):
        self.docs = _sorted(self.docs, [(key, direction)])
        return self

    async def to_list(self, n):
        return [json.loads(json.dumps(d)) for d in self.docs[:n]]


def _sorted(docs, spec):
    for key, direction in reversed(spec or []):
        docs = sorted(docs, key=lambda d: (d.get(key) is not None, d.get(key)), reverse=direction < 0)
    return docs


class FakeCollection:
    def __init__(self, docs):
        self.docs = docs

    async def find_one(self, query):
        for d in self.docs:
            if _match(d, query):
                return json.loads(json.dumps(d))
        return None

    def find(self, filter=None, sort=None):
        return _Cursor(_sorted([d for d in self.docs if _match(d, filter or {})], sort))

    async def _refuse(self, *a, **kw):
        raise RuntimeError("the golden generator never writes")

    insert_one = update_one = delete_one = _refuse


class FakeTrickCollection(FakeCollection):
    """Catalog lookups fall back to deleted tricks.

    Flights of 2022-2023 use tricks deleted from the catalog in 2024 (Full
    Stall, Super Stall, ...). They were scored while those tricks existed;
    today's legacy Trick.get would raise "Trick ... not found" (404), which
    tests nothing. The deleted trick's last definition is used instead.
    """

    async def find_one(self, query):
        doc = await super().find_one(query)
        if doc is None and "deleted" in query:
            doc = await super().find_one({k: v for k, v in query.items() if k != "deleted"})
        return doc


# --------------------------------------------------------------------------
# Pseudonymisation
# --------------------------------------------------------------------------

# Pilots get 1000001, 1000002, ... Teams and judges keep ObjectId-shaped
# pseudonyms while legacy runs (its models validate ObjectIds); the output
# writer turns them into "t0001" / "j0001".
PILOT_BASE = 1000000
_HEX_PSEUDONYM = re.compile(r"0{18}([ab])(\d{5})")


def _hex_pseudonym(kind, n):
    return "0" * 18 + kind + f"{n:05d}"


def _readable_ids(text):
    return _HEX_PSEUDONYM.sub(lambda m: ("j" if m.group(1) == "a" else "t") + f"{int(m.group(2)):04d}", text)


def pseudonymise(data):
    """Replace CIVL ids, team ids and judge ids everywhere in the export.

    Ids are numbered in first-appearance order (competitions by code, then
    their flights, participants and judges, then the rest of the export).
    The mapping stays in memory. Returns the new data and the real ids,
    for the leak guard.
    """
    pilots, teams, judges = {}, {}, {}

    def pilot(i):
        if i not in pilots:
            pilots[i] = PILOT_BASE + len(pilots) + 1
        return pilots[i]

    def team(i):
        if i not in teams:
            teams[i] = _hex_pseudonym("b", len(teams) + 1)
        return teams[i]

    def judge(i):
        if i not in judges:
            judges[i] = _hex_pseudonym("a", len(judges) + 1)
        return judges[i]

    for c in sorted(data["competitions"], key=lambda c: c["code"]):
        solo = c["type"] == "solo"
        for run in c["runs"]:
            for f in run["flights"]:
                if solo:
                    pilot(f["pilot"])
                elif f.get("team"):
                    team(f["team"])
                for m in f["marks"]:
                    judge(m["judge"])
                for t in f["tricks"]:
                    for i in t.get("technical_marks") or {}:
                        judge(i)
        for run in [c] + c["runs"]:
            for i in run.get("pilots") or []:
                pilot(i)
            for i in run.get("teams") or []:
                team(i)
            for i in run.get("judges") or []:
                judge(i)
    for t in data["teams"]:
        team(t["_id"])
        for i in t["pilots"]:
            pilot(i)
    for p in data["pilots"]:
        pilot(p["_id"])
    for j in data["judges"]:
        judge(j["_id"])

    out = json.loads(json.dumps(data))
    for p in out["pilots"]:
        p["_id"] = p["civlid"] = pilots[p["_id"]]
    for t in out["teams"]:
        t["_id"] = teams[t["_id"]]
        t["pilots"] = [pilots[i] for i in t["pilots"]]
    for j in out["judges"]:
        j["_id"] = judges[j["_id"]]
        j["civlid"] = pilots.get(j.get("civlid"))
    for c in out["competitions"]:
        for run in [c] + c["runs"]:
            run["pilots"] = [pilots[i] for i in run.get("pilots") or []]
            run["teams"] = [teams[i] for i in run.get("teams") or []]
            run["judges"] = [judges[i] for i in run.get("judges") or []]
        for run in c["runs"]:
            for f in run["flights"]:
                f["pilot"] = pilots.get(f["pilot"], f["pilot"])
                if f.get("team"):
                    f["team"] = teams[f["team"]]
                for m in f["marks"]:
                    m["judge"] = judges[m["judge"]]
                for t in f["tricks"]:
                    if t.get("technical_marks"):
                        t["technical_marks"] = {judges[i]: v for i, v in t["technical_marks"].items()}
    real_ids = {str(i) for i in pilots} | {str(j["civlid"]) for j in data["judges"] if j.get("civlid")}
    real_hex = set(teams) | set(judges)
    return out, real_ids, real_hex


def load_export(path):
    data = {}
    with tempfile.TemporaryDirectory() as tmp:
        with tarfile.open(path) as tar:
            tar.extractall(tmp, filter="data")
        for name in ("competitions", "pilots", "teams", "judges", "seasons", "tricks"):
            with open(os.path.join(tmp, name + ".json")) as f:
                data[name] = [json.loads(line) for line in f if line.strip()]
    return data


# --------------------------------------------------------------------------
# Legacy setup
# --------------------------------------------------------------------------

class Legacy:
    def __init__(self, legacy_dir, data):
        sys.path.insert(0, os.path.join(legacy_dir, "api"))
        logging.disable(logging.CRITICAL)
        import models.competitions as competitions
        import models.judges as judges
        import models.pilots as pilots
        import models.seasons as seasons
        import models.teams as teams
        import models.tricks as tricks
        from core.config import settings
        from fastapi import HTTPException

        # Judge names only appear in legacy error messages; the engine and
        # the corpus use judge ids (anonymisation, controller ruling).
        judge_docs = [dict(j, name=str(j["_id"])) for j in data["judges"]]
        competitions.collection = FakeCollection(data["competitions"])
        judges.collection = FakeCollection(judge_docs)
        pilots.collection = FakeCollection(data["pilots"])
        seasons.collection = FakeCollection(data["seasons"])
        teams.collection = FakeCollection(data["teams"])
        tricks.collection = FakeTrickCollection(data["tricks"])

        async def no_save(self):
            raise RuntimeError("the golden generator never writes")

        competitions.Competition.save = no_save
        # Production ran Python 3.10 (naive float sum), see naive_sum.
        competitions.sum = naive_sum

        self.Competition = competitions.Competition
        self.CompetitionType = competitions.CompetitionType
        self.Season = seasons.Season
        self.Pilot = pilots.Pilot
        self.Team = teams.Team
        self.Judge = judges.Judge
        self.Trick = tricks.Trick
        self.HTTPException = HTTPException
        self.settings = settings


# --------------------------------------------------------------------------
# Flight cases
# --------------------------------------------------------------------------

def anonymise_competition(raw):
    """Replace operator warning texts by WARNING_TEXT (counts unchanged)."""
    raw = json.loads(json.dumps(raw))
    for run in raw["runs"]:
        for f in run["flights"]:
            f["warnings"] = [WARNING_TEXT for _ in f.get("warnings") or []]
            f["warnings2"] = [WARNING_TEXT for _ in f.get("warnings2") or []]
    return raw


def dump_trick(t):
    return t.model_dump(mode="json")


def dump_mark(m):
    out = {"judge": m.judge}
    for k in ("technical", "choreography", "landing", "synchro"):
        v = getattr(m, k)
        if v is not None:
            out[k] = v
    if m.technical_per_trick is not None:
        out["technical_per_trick"] = list(m.technical_per_trick)
    return out


def dump_final(mark):
    jm = mark.judges_mark
    return {
        "judges_mark": {
            "judge": jm.judge,
            "technical": jm.technical,
            "choreography": jm.choreography,
            "landing": jm.landing,
            "synchro": jm.synchro,
        },
        "technicity": mark.technicity,
        "bonus_percentage": mark.bonus_percentage,
        "technical": mark.technical,
        "choreography": mark.choreography,
        "landing": mark.landing,
        "synchro": mark.synchro,
        "bonus": mark.bonus,
        "score": mark.score,
        "malus": mark.malus,
        "warnings1": list(mark.warnings),
        "warnings2": list(mark.warnings2),
        "notes": list(mark.notes),
        "mark_type": mark.mark_type or "",
    }


def same_participant(comp, a, b, L):
    if comp.type == L.CompetitionType.solo:
        return a.pilot == b.pilot
    return a.team == b.team


async def build_input(L, comp, run_i, flight, is_awt, mark_type):
    run = comp.runs[run_i]
    s = L.settings
    previous = []
    for k in range(run_i):
        # Legacy counts the first matching flight of each earlier run
        # (competitions.py:1134 break); the export has at most one.
        matches = [f for f in comp.runs[k].flights if same_participant(comp, flight, f, L)]
        if len(matches) > 1:
            raise SystemExit(f"{comp.code}: run {k} has {len(matches)} flights of one participant")
        for f in matches[:1]:
            previous.append({
                "run_index": k,
                "tricks": [dump_trick(t) for t in f.tricks],
                "warnings1": len(f.warnings),
                "warnings2": len(f.warnings2),
            })
    judge_levels = {}
    for m in flight.marks:
        try:
            judge_levels[m.judge] = (await L.Judge.get(m.judge)).level.value
        except L.HTTPException:
            pass
    position_rules = {}
    for t in flight.tricks:
        try:
            ct = await L.Trick.get(t.base_trick)
        except L.HTTPException:
            continue
        position_rules[t.base_trick] = {
            "first_maneuver": ct.first_maneuver,
            "no_first_maneuver": ct.no_first_maneuver,
            "last_maneuver": ct.last_maneuver,
            "no_last_maneuver": ct.no_last_maneuver,
        }
    repeatable = []
    for tid in comp.repeatable_tricks:
        try:
            repeatable.append((await L.Trick.get(tid)).name)
        except L.HTTPException:
            pass
    return {
        "type": comp.type.value,
        "config": run.config.model_dump(mode="json"),
        "rules": {
            "max_tricks_per_type": dict(s.competitions.max_tricks_per_type),
            "tip_touch_pct_per_touch": 2,
            "tip_touch_pct_max": 6,
            "default_max_bonus": 10,
        },
        "run_index": run_i,
        "mark_type": mark_type or "",
        "is_awt_pilot": is_awt,
        "has_awt_season": comp.is_awt_season(),
        "has_awq_season": comp.is_awq_season(),
        "repeatable_base_tricks": repeatable,
        "reset_policies": [int(r.repetitions_reset_policy) for r in comp.runs[: run_i + 1]],
        "previous": previous,
        "judge_levels": judge_levels,
        "position_rules": position_rules,
        "legacy_last_maneuver": True,
        "flight": {
            "did_not_start": flight.did_not_start,
            "tricks": [dump_trick(t) for t in flight.tricks],
            "marks": [dump_mark(m) for m in flight.marks],
            "warnings1": list(flight.warnings),
            "warnings2": list(flight.warnings2),
            "tip_touches": flight.tip_touch_bonuses,
        },
    }


def py_round3(x):
    return round(x, 3)


async def flight_cases(L, raw, stats):
    anon = anonymise_competition(raw)
    cases = []
    for run_i, run in enumerate(raw["runs"]):
        for j, raw_flight in enumerate(run["flights"]):
            # Fresh objects for every flight: calculate_score mutates the
            # flight, and the competition is truncated to run_i+1 runs
            # (decision 0004).
            comp = L.Competition.model_validate(anon)
            comp.runs = comp.runs[: run_i + 1]
            flight = comp.runs[run_i].flights[j]
            mark_type = flight.final_marks.mark_type if flight.final_marks else None
            is_awt = False
            if comp.type == L.CompetitionType.solo:
                is_awt = (await L.Pilot.get(flight.pilot)).is_awt(comp.start_date.year)
                participant = str(flight.pilot)
            else:
                participant = flight.team
            inp = await build_input(L, comp, run_i, flight, is_awt, mark_type)
            case = {"run": run_i, "participant": participant, "input": inp}
            try:
                mark = await comp.calculate_score(flight, run_i, is_awt, mark_type=mark_type)
            except L.HTTPException as e:
                stats["legacy errors"] += 1
                case.update(expected=None, expected_tricks=None, expected_marks=None, error=str(e.detail))
                cases.append(case)
                continue
            case.update(
                expected=dump_final(mark),
                expected_tricks=[dump_trick(t) for t in flight.tricks],
                expected_marks=[dump_mark(m) for m in flight.marks],
                error="",
            )
            cases.append(case)
            stored = (raw_flight.get("final_marks") or {}).get("score")
            if stored is None or py_round3(stored) != py_round3(mark.score):
                stats["score differs from stored"] += 1
                classify_difference(stats, raw_flight, mark)
    return cases


def classify_difference(stats, raw_flight, mark):
    """Best-effort cause of a recomputed score differing from the stored one."""
    fm = raw_flight.get("final_marks") or {}
    notes_old, notes_new = fm.get("notes") or [], list(mark.notes)
    w_old = len(fm.get("warnings") or []) + len(fm.get("warnings2") or [])
    w_new = len(mark.warnings) + len(mark.warnings2)
    if any(t.get("solo") is None or t.get("synchro") is None for t in raw_flight.get("tricks") or []):
        cause = "pre-2024 trick snapshot without solo/synchro flags (tricks now ignored)"
    elif not fm.get("mark_type") and any(m.get("technical_per_trick") for m in raw_flight.get("marks") or []):
        cause = "per-trick marks stored without mark_type (scored as classic now)"
    elif w_old != w_new:
        cause = "different number of warnings (catalog position rules or warnings edited later)"
    elif any("DSQ" in n for n in notes_old) != any("DSQ" in n for n in notes_new):
        cause = "DSQ status differs"
    elif any("previous run" in n for n in notes_old) != any("previous run" in n for n in notes_new) or \
            fm.get("malus") != mark.malus:
        cause = "repetition malus differs (later runs / reset policy / edited earlier flights)"
    elif round(fm.get("technicity", 0), 3) != round(mark.technicity, 3):
        cause = "technicity differs (trick snapshot/ignored tricks)"
    elif round(fm.get("bonus_percentage", 0), 3) != round(mark.bonus_percentage, 3):
        cause = "bonus percentage differs (stored per-trick technical marks / ignored tricks)"
    elif notes_old != notes_new:
        cause = "other note differences"
    else:
        cause = "judges marks average differs (judge level or weights changed)"
    stats["diff: " + cause] += 1


# --------------------------------------------------------------------------
# Ranking cases
# --------------------------------------------------------------------------

async def participant_info(L, comp, pid):
    if comp.type == L.CompetitionType.solo:
        p = await L.Pilot.get(int(pid))
        return {"id": str(p.civlid), "country": p.country, "gender": p.gender.value,
                "awt": p.is_awt(comp.start_date.year)}, p.name
    t = await L.Team.get(pid, deleted=True)
    return {"id": str(t.id), "pilots": [str(p) for p in t.pilots]}, t.name


async def competition_ranking(L, raw):
    comp = L.Competition.model_validate(raw)
    solo = comp.type == L.CompetitionType.solo
    ids = []
    for run in comp.runs:
        for f in run.flights:
            ids.append(f.pilot if solo else f.team)
    ids += comp.pilots if solo else comp.teams
    participants, names = {}, {}
    for pid in ids:
        if str(pid) in participants:
            continue
        info, name = await participant_info(L, comp, pid)
        participants[info["id"]] = info
        names[info["id"]] = name
    # name_order: position in the participants sorted by display name, so
    # that the ranking engine can reproduce name tie-breaks without names.
    for i, pid in enumerate(sorted(names, key=lambda k: (names[k], k))):
        participants[pid]["name_order"] = i

    inp = {
        "type": comp.type.value,
        "state": comp.state.value,
        "published": comp.published,
        "year": comp.start_date.year,
        "seasons": list(comp.seasons),
        "participants": sorted(participants.values(), key=lambda p: p["id"]),
        "runs": [{
            "state": run.state.value,
            "flights": [{
                "participant": str(f.pilot if solo else f.team),
                "score": f.final_marks.score,
                "published": f.published,
            } for f in run.flights],
        } for run in comp.runs],
    }

    def pid(r):
        return str(r.pilot if solo else r.team)

    res = await comp.results()
    exp = await res.export()
    expected = {
        "final": res.final,
        "results": {k: [{
            "participant": str(e.pilot.civlid) if solo else str(e.team.id),
            "score": e.score,
            "result_per_run": [{"rank": x.rank, "score": x.score} for x in e.result_per_run],
        } for e in v] for k, v in exp.results.items()},
        "internal_results": {k: [pid(e) for e in v] for k, v in res.results.items()},
        "runs_results": [{
            "final": rr.final,
            "results": {k: [{"participant": pid(f), "score": f.final_marks.score} for f in v]
                        for k, v in rr.results.items()},
        } for rr in res.runs_results],
    }
    return {"scope": "competition", "code": comp.code, "input": inp, "expected": expected}


async def season_ranking(L, raw_season, data):
    season = L.Season.model_validate(raw_season)
    comps = [c for c in data["competitions"] if season.code in (c.get("seasons") or []) and c.get("deleted") is None]
    comps.sort(key=lambda c: c["name"])
    inp = {
        "code": season.code,
        "country": season.country,
        "competitions": [c["code"] for c in comps],
    }
    try:
        exp = await season.export_public()
    except L.HTTPException as e:
        return {"scope": "season", "code": season.code, "input": inp, "expected": None, "error": str(e.detail)}

    def pid(r):
        return str(r.pilot.civlid) if r.pilot is not None else str(r.team.id)

    expected = {
        "type": exp.type.value if hasattr(exp.type, "value") else exp.type,
        "number_of_pilots": exp.number_of_pilots,
        "number_of_teams": exp.number_of_teams,
        "results": {r.type: [{"participant": pid(e), "score": e.score} for e in r.results] for r in exp.results},
        "competitions_results": {code: [{"participant": pid(e), "score": e.score} for e in v]
                                 for code, v in exp.competitions_results.items()},
    }
    return {"scope": "season", "code": season.code, "input": inp, "expected": expected, "error": ""}


# --------------------------------------------------------------------------
# Output
# --------------------------------------------------------------------------

def write_json(path, head, items_key, items):
    """One item per line: readable diffs, repr-exact floats."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        if items_key is None:
            f.write(_readable_ids(json.dumps(head, allow_nan=False)) + "\n")
            return
        f.write(_readable_ids(json.dumps(head, allow_nan=False)[:-1]) + ', "' + items_key + '": [\n')
        f.write(",\n".join(_readable_ids(json.dumps(i, allow_nan=False)) for i in items))
        f.write("\n]}\n")


def check_no_personal_data(out_dir, data, real_ids, real_hex):
    """Fail if a person's name or a real id appears in the corpus.

    Names (pilots, judges) and real team/judge ids: plain substring search.
    Real CIVL ids: as whole numbers (not inside a float or a longer id).
    """
    needles = {p["name"] for p in data["pilots"]} | {j["name"] for j in data["judges"]} | real_hex
    needles = {n for n in needles if len(n) >= 3}
    numbers = re.compile(r"(?<![\w.])(?:" + "|".join(sorted(real_ids)) + r")(?![\w.])")
    for root, _, files in os.walk(out_dir):
        for fn in files:
            text = open(os.path.join(root, fn)).read()
            for n in needles:
                if n in text:
                    raise SystemExit(f"personal data leaked into {fn}: {n!r}")
            m = numbers.search(text)
            if m:
                raise SystemExit(f"real CIVL id leaked into {fn}: {m.group(0)}")


async def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--backup", required=True)
    ap.add_argument("--legacy", required=True, help="legacy/legacy directory")
    ap.add_argument("--out", required=True, help="testdata/golden directory")
    args = ap.parse_args()

    real = load_export(args.backup)
    data, real_ids, real_hex = pseudonymise(real)
    L = Legacy(os.path.abspath(args.legacy), data)
    stats = Counter()

    flights_dir = os.path.join(args.out, "flights")
    rankings_dir = os.path.join(args.out, "rankings")
    for d in (flights_dir, rankings_dir):
        os.makedirs(d, exist_ok=True)
        for fn in os.listdir(d):
            if fn.endswith(".json"):
                os.remove(os.path.join(d, fn))

    for raw in sorted(data["competitions"], key=lambda c: c["code"]):
        if raw.get("deleted") is not None:
            continue
        cases = await flight_cases(L, raw, stats)
        stats["cases"] += len(cases)
        write_json(os.path.join(flights_dir, raw["code"] + ".json"),
                   {"competition": str(raw["_id"]), "code": raw["code"]}, "cases", cases)
        rk = await competition_ranking(L, raw)
        write_json(os.path.join(rankings_dir, f"competition-{raw['code']}.json"), rk, None, None)
        stats["competition rankings"] += 1

    for raw in sorted(data["seasons"], key=lambda s: s["code"]):
        if raw.get("deleted") is not None:
            continue
        rk = await season_ranking(L, raw, data)
        write_json(os.path.join(rankings_dir, f"season-{raw['code']}.json"), rk, None, None)
        stats["season rankings"] += 1
        if rk["error"]:
            stats["season errors"] += 1

    check_no_personal_data(args.out, real, real_ids, real_hex)
    for k in sorted(stats):
        print(f"{k}: {stats[k]}")


if __name__ == "__main__":
    asyncio.run(main())
