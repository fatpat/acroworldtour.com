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

// MarkInput is one judge's marks for a flight. A nil value means "no mark"
// (decision 0006); values must be within 0-10.
type MarkInput struct {
	Judge                                     string
	Technical, Choreography, Landing, Synchro *float64
	TechnicalPerTrick                         []*float64
}

// FlightInput is a flight as entered on the judging screen. Tricks are
// acronyms or unique trick names of the competition-year catalog.
type FlightInput struct {
	Tricks      []string
	Marks       []MarkInput
	DidNotStart bool
	Warnings1   []string
	Warnings2   []string
	TipTouches  int
}

// MarkTypeFor is the mark type of a participant of a competition
// (decision 0006): "awt" for a solo pilot AWT in the competition year when
// the competition has an awt-* season, "awq" for other solo pilots when it
// has an awq-* season, classic ("") otherwise and for synchro.
func (s *Service) MarkTypeFor(ctx context.Context, comp Competition, participant string) (scoring.MarkType, error) {
	if comp.Type != domain.TypeSolo {
		return scoring.MarkClassic, nil
	}
	env := &scoringEnv{comp: comp, awt: map[int]bool{}}
	env.hasAWT, env.hasAWQ = seasonKinds(comp.Seasons)
	civlid, err := strconv.Atoi(participant)
	if err != nil {
		return "", invalid(domain.FieldError{Field: "participant", Msg: fmt.Sprintf("invalid CIVL id %q", participant)})
	}
	return env.markType(ctx, s.read(), ranking.Participant{CivlID: civlid})
}

// checkMark validates one mark value (decision 0006: 0-10).
func checkMark(field string, v *float64) []domain.FieldError {
	if v == nil || (*v >= 0 && *v <= 10) {
		return nil
	}
	return []domain.FieldError{{Field: field, Msg: domain.ErrMarkRange.Error()}}
}

// round3Mark rounds a judge input mark as legacy did at input validation
// (spec 03 §9).
func round3Mark(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := scoring.Round3(*v)
	return &r
}

// resolveTricks snapshots the flown tricks from the competition-year
// catalog (decision 0008). Unknown tricks give the legacy 400 message.
func resolveTricks(ctx context.Context, q *gen.Queries, year int, names []string) ([]scoring.FlownTrick, error) {
	out := make([]scoring.FlownTrick, 0, len(names))
	var unknown []string
	for _, n := range names {
		row, err := q.GetUniqueTrick(ctx, gen.GetUniqueTrickParams{Year: int64(year), Acronym: n})
		if errors.Is(err, sql.ErrNoRows) {
			row, err = q.GetUniqueTrickByName(ctx, gen.GetUniqueTrickByNameParams{Year: int64(year), Name: n})
		}
		if errors.Is(err, sql.ErrNoRows) {
			if !slices.Contains(unknown, n) {
				unknown = append(unknown, n)
			}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get trick %q: %w", n, err)
		}
		u, err := uniqueTrickFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, scoring.FlownTrick{UniqueTrick: u})
	}
	if len(unknown) > 0 {
		return nil, scoring.ValidationError{Msg: "Unknown trick(s): " + strings.Join(unknown, ", ")}
	}
	return out, nil
}

// flightFromInput validates the input and builds the flight to score.
// Marks are ordered by judge id, the order they are stored in, so that a
// later rescoring sums them in the same order.
func flightFromInput(ctx context.Context, q *gen.Queries, c Competition, r Run, in FlightInput) (scoring.Flight, error) {
	var errs []domain.FieldError
	if in.TipTouches < 0 {
		errs = append(errs, domain.FieldError{Field: "tip_touches", Msg: "must be >= 0"})
	}
	seen := map[string]bool{}
	marks := make([]scoring.JudgeMark, 0, len(in.Marks))
	for i, m := range in.Marks {
		field := fmt.Sprintf("marks[%d]", i)
		switch {
		case seen[m.Judge]:
			errs = append(errs, domain.FieldError{Field: field, Msg: fmt.Sprintf("judge %s is listed twice", m.Judge)})
		case !slices.Contains(r.Judges, m.Judge):
			errs = append(errs, domain.FieldError{Field: field, Msg: fmt.Sprintf("judge %s is not a judge of this run", m.Judge)})
		}
		seen[m.Judge] = true
		errs = append(errs, checkMark(field+".technical", m.Technical)...)
		errs = append(errs, checkMark(field+".choreography", m.Choreography)...)
		errs = append(errs, checkMark(field+".landing", m.Landing)...)
		errs = append(errs, checkMark(field+".synchro", m.Synchro)...)
		for k, v := range m.TechnicalPerTrick {
			errs = append(errs, checkMark(fmt.Sprintf("%s.technical_per_trick[%d]", field, k), v)...)
		}
		marks = append(marks, scoring.JudgeMark{
			Judge: m.Judge, Technical: round3Mark(m.Technical), Choreography: round3Mark(m.Choreography),
			Landing: round3Mark(m.Landing), Synchro: round3Mark(m.Synchro), TechnicalPerTrick: slices.Clone(m.TechnicalPerTrick),
		})
	}
	if err := invalid(errs...); err != nil {
		return scoring.Flight{}, err
	}
	slices.SortStableFunc(marks, func(a, b scoring.JudgeMark) int { return strings.Compare(a.Judge, b.Judge) })
	tricks, err := resolveTricks(ctx, q, c.Year(), in.Tricks)
	if err != nil {
		return scoring.Flight{}, err
	}
	nonNil := func(s []string) []string { return append([]string{}, s...) }
	return scoring.Flight{
		DidNotStart: in.DidNotStart, Tricks: tricks, Marks: marks,
		Warnings1: nonNil(in.Warnings1), Warnings2: nonNil(in.Warnings2), TipTouches: in.TipTouches,
	}, nil
}

// SimulateFlight scores a flight without saving it (live provisional score
// of the judging screen). It reads one consistent snapshot on a single read
// connection.
func (s *Service) SimulateFlight(ctx context.Context, compID string, run int, participant string, in FlightInput) (scoring.Result, error) {
	tx, err := s.db.Read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return scoring.Result{}, fmt.Errorf("begin read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := gen.New(tx)
	c, _, err := loadCompetition(ctx, q, compID, false)
	if err != nil {
		return scoring.Result{}, err
	}
	env, _, p, flight, err := prepareFlight(ctx, q, c, run, participant, in)
	if err != nil {
		return scoring.Result{}, err
	}
	input, err := env.input(ctx, q, run, p, flight)
	if err != nil {
		return scoring.Result{}, err
	}
	return scoring.Score(input)
}

// prepareFlight loads what scoring a new flight input needs and validates
// the participant and the input.
func prepareFlight(ctx context.Context, q *gen.Queries, c Competition, run int, participant string, in FlightInput) (*scoringEnv, Run, ranking.Participant, scoring.Flight, error) {
	r, _, err := loadRun(ctx, q, c.ID, run)
	if err != nil {
		return nil, r, ranking.Participant{}, scoring.Flight{}, err
	}
	env, err := loadScoringEnv(ctx, q, c)
	if err != nil {
		return nil, r, ranking.Participant{}, scoring.Flight{}, err
	}
	p, err := env.participant(r, participant)
	if err != nil {
		return nil, r, p, scoring.Flight{}, err
	}
	flight, err := flightFromInput(ctx, q, c, r, in)
	return env, r, p, flight, err
}

// SaveFlight scores and stores a flight in an open run, then, in the same
// transaction (CLAUDE.md "compute on write"): rescores the participant's
// flights of later runs in order (decision 0006 #14) and rebuilds the
// stored run, competition and season rankings. Live events are published
// after commit.
func (s *Service) SaveFlight(ctx context.Context, compID string, run int, participant string, in FlightInput, publish bool) (scoring.Result, error) {
	var res scoring.Result
	var c Competition
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		var err error
		if c, _, err = editable(ctx, q, compID); err != nil {
			return err
		}
		env, r, p, flight, err := prepareFlight(ctx, q, c, run, participant, in)
		if err != nil {
			return err
		}
		if err := domain.CanSaveFlight(r.State); err != nil {
			return fmt.Errorf("run %d: %w", run+1, err)
		}
		if res, err = scoreAndStore(ctx, q, env, run, p, flight, &publish); err != nil {
			return err
		}
		for later := run + 1; later < len(env.runs); later++ {
			if err := rescoreFlight(ctx, q, env, later, p); err != nil {
				return err
			}
		}
		return RebuildRankings(ctx, q, compID)
	})
	if err != nil {
		return scoring.Result{}, err
	}
	s.publish(KindFlight, c.topics()...)
	return res, nil
}

// rescoreFlight rescores the participant's stored flight of run idx, if
// any, from its stored input; earlier runs are read from the transaction,
// so they include the flights just saved or rescored.
func rescoreFlight(ctx context.Context, q *gen.Queries, env *scoringEnv, idx int, p ranking.Participant) error {
	f, ok, err := storedFlight(ctx, q, env.comp.ID, idx, p)
	if err != nil || !ok {
		return err
	}
	flight, err := flightFromStore(ctx, q, f)
	if err != nil {
		return err
	}
	if _, err := scoreAndStore(ctx, q, env, idx, p, flight, nil); err != nil {
		return fmt.Errorf("rescore flight of run %d: %w", idx+1, err)
	}
	return nil
}

// scoreAndStore scores a flight and writes it with its tricks, marks and
// warnings. publish nil keeps the stored published flag (rescoring).
func scoreAndStore(ctx context.Context, q *gen.Queries, env *scoringEnv, idx int, p ranking.Participant, flight scoring.Flight, publish *bool) (scoring.Result, error) {
	input, err := env.input(ctx, q, idx, p, flight)
	if err != nil {
		return scoring.Result{}, err
	}
	res, err := scoring.Score(input)
	if err != nil {
		return scoring.Result{}, err
	}
	final, err := json.Marshal(res.Final)
	if err != nil {
		return res, fmt.Errorf("encode final mark: %w", err)
	}
	cur, exists, err := storedFlight(ctx, q, env.comp.ID, idx, p)
	if err != nil {
		return res, err
	}
	published := cur.Published
	if publish != nil {
		published = *publish
	}
	mt := sql.NullString{String: string(res.Final.MarkType), Valid: res.Final.MarkType != scoring.MarkClassic}
	score := sql.NullFloat64{Float64: scoring.Round3(res.Final.Score), Valid: true}
	id := cur.ID
	if exists {
		err = q.UpdateFlight(ctx, gen.UpdateFlightParams{
			DidNotStart: flight.DidNotStart, TipTouches: int64(flight.TipTouches), Published: published, MarkType: mt,
			SaveSeq: cur.SaveSeq, FinalMark: nullString(string(final)), Score: score, UpdatedAt: nullString(now()), ID: id,
		})
	} else {
		var seq int64
		seq, err = q.MaxFlightSaveSeq(ctx, gen.MaxFlightSaveSeqParams{CompetitionID: env.comp.ID, RunIdx: int64(idx)})
		if err != nil {
			return res, fmt.Errorf("next save sequence of run %d: %w", idx+1, err)
		}
		id, err = q.InsertFlight(ctx, gen.InsertFlightParams{
			CompetitionID: env.comp.ID, RunIdx: int64(idx), Civlid: sql.NullInt64{Int64: int64(p.CivlID), Valid: p.TeamID == ""},
			TeamID: nullString(p.TeamID), DidNotStart: flight.DidNotStart, TipTouches: int64(flight.TipTouches),
			Published: published, MarkType: mt, SaveSeq: seq + 1, FinalMark: nullString(string(final)), Score: score,
			UpdatedAt: nullString(now()),
		})
	}
	if err != nil {
		return res, fmt.Errorf("save flight of run %d: %w", idx+1, err)
	}
	return res, writeFlightChildren(ctx, q, id, res, flight)
}

// writeFlightChildren replaces a flight's tricks and marks (as computed by
// the engine: per-trick technical marks, filled per-trick judge marks) and
// its operator warnings.
func writeFlightChildren(ctx context.Context, q *gen.Queries, id int64, res scoring.Result, flight scoring.Flight) error {
	if err := q.DeleteFlightTricks(ctx, id); err != nil {
		return fmt.Errorf("clear tricks of flight %d: %w", id, err)
	}
	if err := q.DeleteFlightMarks(ctx, id); err != nil {
		return fmt.Errorf("clear marks of flight %d: %w", id, err)
	}
	if err := q.DeleteFlightWarnings(ctx, id); err != nil {
		return fmt.Errorf("clear warnings of flight %d: %w", id, err)
	}
	for i, t := range res.Tricks {
		p, err := flightTrickParams(id, i, t)
		if err != nil {
			return err
		}
		if err := q.InsertFlightTrick(ctx, p); err != nil {
			return fmt.Errorf("save trick %d of flight %d: %w", i, id, err)
		}
	}
	for _, m := range res.Marks {
		p := gen.InsertFlightMarkParams{
			FlightID: id, JudgeID: m.Judge, Technical: nullFloat(m.Technical), Choreography: nullFloat(m.Choreography),
			Landing: nullFloat(m.Landing), Synchro: nullFloat(m.Synchro),
		}
		if m.TechnicalPerTrick != nil {
			b, err := json.Marshal(m.TechnicalPerTrick)
			if err != nil {
				return fmt.Errorf("encode per-trick marks: %w", err)
			}
			p.TechnicalPerTrick = nullString(string(b))
		}
		if err := q.InsertFlightMark(ctx, p); err != nil {
			return fmt.Errorf("save mark of judge %s on flight %d: %w", m.Judge, id, err)
		}
	}
	for cat, texts := range [][]string{flight.Warnings1, flight.Warnings2} {
		for i, text := range texts {
			err := q.InsertFlightWarning(ctx, gen.InsertFlightWarningParams{FlightID: id, Category: int64(cat + 1), Position: int64(i), Text: text})
			if err != nil {
				return fmt.Errorf("save warning %d.%d of flight %d: %w", cat+1, i, id, err)
			}
		}
	}
	return nil
}

func flightTrickParams(id int64, pos int, t scoring.FlownTrick) (gen.InsertFlightTrickParams, error) {
	p := gen.InsertFlightTrickParams{
		FlightID: id, Position: int64(pos), Name: t.Name, Acronym: t.Acronym, BaseTrick: t.BaseTrick,
		TechnicalCoefficient: t.TechnicalCoefficient, Bonus: t.Bonus,
		Solo: sql.NullBool{Bool: t.Solo, Valid: true}, SoloAwt: sql.NullBool{Bool: t.SoloAWT, Valid: true},
		Synchro: sql.NullBool{Bool: t.Synchro, Valid: true}, TechnicalMark: nullFloat(t.TechnicalMark),
	}
	var err error
	enc := func(dst *string, v any) {
		if err == nil {
			*dst, err = jsonArray(v)
		}
	}
	enc(&p.BonusTypes, t.BonusTypes)
	enc(&p.Uniqueness, t.Uniqueness)
	enc(&p.Bonuses, t.Bonuses)
	enc(&p.Types, t.Types)
	if err != nil {
		return p, fmt.Errorf("encode trick %s: %w", t.Acronym, err)
	}
	if t.TechnicalMarks != nil {
		b, err := json.Marshal(t.TechnicalMarks)
		if err != nil {
			return p, fmt.Errorf("encode technical marks of %s: %w", t.Acronym, err)
		}
		p.TechnicalMarks = nullString(string(b))
	}
	return p, nil
}

func nullFloat(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}
