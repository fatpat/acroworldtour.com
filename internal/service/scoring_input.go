package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"awt/internal/domain"
	"awt/internal/ranking"
	"awt/internal/scoring"
	"awt/internal/store/gen"
)

// scoringEnv is the competition-level context of scoring.Input, loaded once
// per transaction and shared by every flight scored in it (the saved flight
// and the rescored later flights).
type scoringEnv struct {
	comp   Competition
	runs   []gen.Run
	rules  scoring.Rules
	hasAWT bool
	hasAWQ bool
	// positions holds the position rules of the competition-year catalog
	// by base trick name (decision 0008, spec 03 §6.1).
	positions map[string]scoring.PositionRule
	levels    map[string]scoring.JudgeLevel
	awt       map[int]bool
}

func loadScoringEnv(ctx context.Context, q *gen.Queries, c Competition) (*scoringEnv, error) {
	env := &scoringEnv{
		comp: c, positions: map[string]scoring.PositionRule{}, levels: map[string]scoring.JudgeLevel{}, awt: map[int]bool{},
	}
	env.hasAWT, env.hasAWQ = seasonKinds(c.Seasons)
	var err error
	if env.runs, err = q.ListRuns(ctx, c.ID); err != nil {
		return nil, fmt.Errorf("list runs of %s: %w", c.ID, err)
	}
	rs, err := q.GetRuleset(ctx, int64(c.Year()))
	if err != nil {
		return nil, fmt.Errorf("ruleset %d: %w", c.Year(), err)
	}
	env.rules = scoring.Rules{
		TipTouchPctPerTouch: rs.TipTouchPctPerTouch, TipTouchPctMax: rs.TipTouchPctMax, DefaultMaxBonus: rs.DefaultMaxBonus,
	}
	if err := json.Unmarshal([]byte(rs.MaxTricksPerType), &env.rules.MaxTricksPerType); err != nil {
		return nil, fmt.Errorf("ruleset %d max_tricks_per_type: %w", c.Year(), err)
	}
	tricks, err := q.ListTricksByYear(ctx, int64(c.Year()))
	if err != nil {
		return nil, fmt.Errorf("list tricks %d: %w", c.Year(), err)
	}
	for _, t := range tricks {
		env.positions[t.Name] = scoring.PositionRule{
			First: int(t.FirstManeuver), NoFirst: int(t.NoFirstManeuver), Last: int(t.LastManeuver), NoLast: int(t.NoLastManeuver),
		}
	}
	return env, nil
}

// seasonKinds tells whether a competition has an awt-* and an awq-*
// season (legacy is_awt_season / is_awq_season).
func seasonKinds(seasons []string) (hasAWT, hasAWQ bool) {
	for _, s := range seasons {
		hasAWT = hasAWT || strings.HasPrefix(s, "awt-")
		hasAWQ = hasAWQ || strings.HasPrefix(s, "awq-")
	}
	return hasAWT, hasAWQ
}

// markType derives the mark type (decision 0006, spec 08 §2.2): solo AWT
// pilots of a competition with an awt-* season are marked "awt", other solo
// pilots "awq" when it has an awq-* season, everyone else classic.
func markType(typ domain.CompetitionType, isAWT, hasAWT, hasAWQ bool) scoring.MarkType {
	switch {
	case typ != domain.TypeSolo:
		return scoring.MarkClassic
	case isAWT && hasAWT:
		return scoring.MarkAWT
	case hasAWQ:
		return scoring.MarkAWQ
	}
	return scoring.MarkClassic
}

// isAWT is the pilot's AWT status for the competition year (false for
// teams).
func (e *scoringEnv) isAWT(ctx context.Context, q *gen.Queries, p ranking.Participant) (bool, error) {
	if p.TeamID != "" {
		return false, nil
	}
	if v, ok := e.awt[p.CivlID]; ok {
		return v, nil
	}
	years, err := q.ListPilotAwtYears(ctx, int64(p.CivlID))
	if err != nil {
		return false, fmt.Errorf("awt years of pilot %d: %w", p.CivlID, err)
	}
	e.awt[p.CivlID] = slices.Contains(years, int64(e.comp.Year()))
	return e.awt[p.CivlID], nil
}

func (e *scoringEnv) markType(ctx context.Context, q *gen.Queries, p ranking.Participant) (scoring.MarkType, error) {
	awt, err := e.isAWT(ctx, q, p)
	if err != nil {
		return "", err
	}
	return markType(e.comp.Type, awt, e.hasAWT, e.hasAWQ), nil
}

// participant parses a participant id (CIVL id for solo, team id for
// synchro) and checks it flies in the run and the competition (legacy
// flight_save).
func (e *scoringEnv) participant(r Run, id string) (ranking.Participant, error) {
	field := func(msg string) error { return invalid(domain.FieldError{Field: "participant", Msg: msg}) }
	if e.comp.Type == domain.TypeSynchro {
		if !slices.Contains(r.Teams, id) || !slices.Contains(e.comp.Teams, id) {
			return ranking.Participant{}, field(fmt.Sprintf("team %s does not fly in run %d", id, r.Index+1))
		}
		return ranking.Participant{TeamID: id}, nil
	}
	civlid, err := strconv.Atoi(id)
	if err != nil || !slices.Contains(r.Pilots, civlid) || !slices.Contains(e.comp.Pilots, civlid) {
		return ranking.Participant{}, field(fmt.Sprintf("pilot %s does not fly in run %d", id, r.Index+1))
	}
	return ranking.Participant{CivlID: civlid}, nil
}

// storedFlight returns the participant's flight in run idx, or ok false.
func storedFlight(ctx context.Context, q *gen.Queries, compID string, idx int, p ranking.Participant) (gen.Flight, bool, error) {
	var f gen.Flight
	var err error
	if p.TeamID != "" {
		f, err = q.GetTeamFlight(ctx, gen.GetTeamFlightParams{CompetitionID: compID, RunIdx: int64(idx), TeamID: nullString(p.TeamID)})
	} else {
		f, err = q.GetPilotFlight(ctx, gen.GetPilotFlightParams{
			CompetitionID: compID, RunIdx: int64(idx), Civlid: sql.NullInt64{Int64: int64(p.CivlID), Valid: true},
		})
	}
	if errors.Is(err, sql.ErrNoRows) {
		return f, false, nil
	}
	if err != nil {
		return f, false, fmt.Errorf("get flight of run %d: %w", idx+1, err)
	}
	return f, true, nil
}

func flownTricks(ctx context.Context, q *gen.Queries, flightID int64) ([]scoring.FlownTrick, error) {
	rows, err := q.ListFlightTricks(ctx, flightID)
	if err != nil {
		return nil, fmt.Errorf("list tricks of flight %d: %w", flightID, err)
	}
	out := make([]scoring.FlownTrick, 0, len(rows))
	for _, r := range rows {
		t := scoring.FlownTrick{
			Name: r.Name, Acronym: r.Acronym, BaseTrick: r.BaseTrick, TechnicalCoefficient: r.TechnicalCoefficient, Bonus: r.Bonus,
			Solo: r.Solo.Bool, SoloAWT: r.SoloAwt.Bool, Synchro: r.Synchro.Bool}
		if err := errors.Join(decodeJSON(r.BonusTypes, &t.BonusTypes), decodeJSON(r.Uniqueness, &t.Uniqueness),
			decodeJSON(r.Bonuses, &t.Bonuses), decodeJSON(r.Types, &t.Types)); err != nil {
			return nil, fmt.Errorf("decode trick %d of flight %d: %w", r.Position, flightID, err)
		}
		if r.TechnicalMark.Valid {
			t.TechnicalMark = &r.TechnicalMark.Float64
		}
		if r.TechnicalMarks.Valid {
			if err := json.Unmarshal([]byte(r.TechnicalMarks.String), &t.TechnicalMarks); err != nil {
				return nil, fmt.Errorf("decode technical marks of flight %d: %w", flightID, err)
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// operatorWarnings returns a flight's stored cat-1 and cat-2 warnings.
func operatorWarnings(ctx context.Context, q *gen.Queries, flightID int64) (w1, w2 []string, err error) {
	rows, err := q.ListFlightWarnings(ctx, flightID)
	if err != nil {
		return nil, nil, fmt.Errorf("list warnings of flight %d: %w", flightID, err)
	}
	w1, w2 = []string{}, []string{}
	for _, r := range rows {
		if r.Category == 1 {
			w1 = append(w1, r.Text)
		} else {
			w2 = append(w2, r.Text)
		}
	}
	return w1, w2, nil
}

// flightFromStore rebuilds the scoring input of a stored flight for
// rescoring: its trick snapshot (the per-trick technical marks are
// recomputed, as on a fresh save), its marks and its operator warnings.
func flightFromStore(ctx context.Context, q *gen.Queries, f gen.Flight) (scoring.Flight, error) {
	out := scoring.Flight{DidNotStart: f.DidNotStart, TipTouches: int(f.TipTouches)}
	var err error
	if out.Tricks, err = flownTricks(ctx, q, f.ID); err != nil {
		return out, err
	}
	for i := range out.Tricks {
		out.Tricks[i].TechnicalMark, out.Tricks[i].TechnicalMarks = nil, nil
	}
	if out.Warnings1, out.Warnings2, err = operatorWarnings(ctx, q, f.ID); err != nil {
		return out, err
	}
	marks, err := q.ListFlightMarks(ctx, f.ID)
	if err != nil {
		return out, fmt.Errorf("list marks of flight %d: %w", f.ID, err)
	}
	for _, m := range marks {
		jm := scoring.JudgeMark{
			Judge: m.JudgeID, Technical: floatPtr(m.Technical), Choreography: floatPtr(m.Choreography),
			Landing: floatPtr(m.Landing), Synchro: floatPtr(m.Synchro),
		}
		if m.TechnicalPerTrick.Valid {
			if err := json.Unmarshal([]byte(m.TechnicalPerTrick.String), &jm.TechnicalPerTrick); err != nil {
				return out, fmt.Errorf("decode per-trick marks of flight %d: %w", f.ID, err)
			}
		}
		out.Marks = append(out.Marks, jm)
	}
	return out, nil
}

func floatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

// input builds the scoring.Input of a flight of participant p in run idx
// (spec 03 §3, golden corpus contract in tools/golden/README.md).
func (e *scoringEnv) input(ctx context.Context, q *gen.Queries, idx int, p ranking.Participant, flight scoring.Flight) (scoring.Input, error) {
	if idx < 0 || idx >= len(e.runs) {
		return scoring.Input{}, fmt.Errorf("run %d of competition %s: %w", idx+1, e.comp.ID, ErrNotFound)
	}
	in := scoring.Input{
		Type: scoring.CompetitionType(e.comp.Type), Rules: e.rules, RunIndex: idx,
		HasAWTSeason: e.hasAWT, HasAWQSeason: e.hasAWQ, RepeatableBaseTricks: e.comp.RepeatableTricks,
		PositionRules: e.positions, JudgeLevels: map[string]scoring.JudgeLevel{}, Flight: flight,
	}
	if err := json.Unmarshal([]byte(e.runs[idx].Config), &in.Config); err != nil {
		return in, fmt.Errorf("run %d config: %w", idx+1, err)
	}
	var err error
	if in.IsAWTPilot, err = e.isAWT(ctx, q, p); err != nil {
		return in, err
	}
	in.MarkType = markType(e.comp.Type, in.IsAWTPilot, e.hasAWT, e.hasAWQ)
	// decision 0004: only runs 0..idx count for the reset policy
	for _, r := range e.runs[:idx+1] {
		in.ResetPolicies = append(in.ResetPolicies, scoring.ResetPolicy(r.ResetPolicy))
	}
	for i := range idx {
		f, ok, err := storedFlight(ctx, q, e.comp.ID, i, p)
		if err != nil {
			return in, err
		}
		if !ok {
			continue
		}
		prev := scoring.PreviousFlight{RunIndex: i}
		if prev.Tricks, err = flownTricks(ctx, q, f.ID); err != nil {
			return in, err
		}
		w1, w2, err := operatorWarnings(ctx, q, f.ID)
		if err != nil {
			return in, err
		}
		prev.Warnings1, prev.Warnings2 = len(w1), len(w2)
		in.Previous = append(in.Previous, prev)
	}
	for _, m := range flight.Marks {
		level, err := e.judgeLevel(ctx, q, m.Judge)
		if err != nil {
			return in, err
		}
		in.JudgeLevels[m.Judge] = level
	}
	return in, nil
}

func (e *scoringEnv) judgeLevel(ctx context.Context, q *gen.Queries, id string) (scoring.JudgeLevel, error) {
	if l, ok := e.levels[id]; ok {
		return l, nil
	}
	j, err := q.GetJudge(ctx, id)
	if err != nil {
		return "", notFound(err, "judge "+id)
	}
	e.levels[id] = scoring.JudgeLevel(j.Level)
	return e.levels[id], nil
}
