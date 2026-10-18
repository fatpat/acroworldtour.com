package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"awt/internal/domain"
	"awt/internal/ranking"
	"awt/internal/store/gen"
)

// runSetup loads an editable competition and one of its runs and applies fn
// to the stored run row. A closed run must be reopened first (spec 05 §8:
// the backend enforces the run state on roster and config edits).
func (s *Service) runSetup(ctx context.Context, compID string, idx int, rescore bool, fn func(q *gen.Queries, c Competition, r Run, row *gen.Run) error) error {
	var c Competition
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		var err error
		c, _, err = editable(ctx, q, compID)
		if err != nil {
			return err
		}
		r, row, err := loadRun(ctx, q, compID, idx)
		if err != nil {
			return err
		}
		if r.State == domain.RunClosed {
			return fmt.Errorf("run %d is closed, reopen it first: %w", idx+1, domain.ErrInvalidTransition)
		}
		if err := fn(q, c, r, &row); err != nil {
			return err
		}
		err = q.UpsertRun(ctx, gen.UpsertRunParams{
			CompetitionID: compID, Idx: row.Idx, State: row.State, Config: row.Config,
			ResetPolicy: row.ResetPolicy, ResultsFinal: row.ResultsFinal, UpdatedAt: nullString(now()),
		})
		if err != nil || !rescore {
			return err
		}
		return rescoreFrom(ctx, q, c, idx)
	})
	if err != nil {
		return err
	}
	s.publish(KindRun, competitionTopic(c.Code))
	return nil
}

// SetRunJudges replaces the judges of a run (spec 05 §5.11: for example a
// judge absent today). They must be judges of the competition, listed once.
func (s *Service) SetRunJudges(ctx context.Context, compID string, idx int, judgeIDs []string) error {
	return s.runSetup(ctx, compID, idx, false, func(q *gen.Queries, c Competition, _ Run, _ *gen.Run) error {
		var errs []domain.FieldError
		seen := map[string]bool{}
		for _, j := range judgeIDs {
			if seen[j] {
				errs = append(errs, domain.FieldError{Field: "judges", Msg: "duplicate judges"})
			}
			seen[j] = true
			if !slices.Contains(c.Judges, j) {
				errs = append(errs, domain.FieldError{Field: "judges", Msg: fmt.Sprintf("judge %s is not a judge of the competition", j)})
			}
		}
		if err := invalid(errs...); err != nil {
			return err
		}
		if err := checkJudgesUnmarked(ctx, q, compID, idx, judgeIDs); err != nil {
			return err
		}
		if err := q.DeleteRunJudges(ctx, gen.DeleteRunJudgesParams{CompetitionID: compID, RunIdx: int64(idx)}); err != nil {
			return fmt.Errorf("clear judges of run %d: %w", idx+1, err)
		}
		for i, j := range judgeIDs {
			err := q.AddRunJudge(ctx, gen.AddRunJudgeParams{CompetitionID: compID, RunIdx: int64(idx), Position: int64(i), JudgeID: j})
			if err != nil {
				return fmt.Errorf("add judge to run %d: %w", idx+1, err)
			}
		}
		return nil
	})
}

// SetRunConfig replaces a run's scoring config; it must be complete, like
// the competition config (checkConfig).
func (s *Service) SetRunConfig(ctx context.Context, compID string, idx int, config json.RawMessage) error {
	if err := invalid(checkConfig(config)...); err != nil {
		return err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, config); err != nil {
		return invalid(domain.FieldError{Field: "config", Msg: err.Error()})
	}
	return s.runSetup(ctx, compID, idx, true, func(_ *gen.Queries, _ Competition, _ Run, row *gen.Run) error {
		row.Config = compact.String()
		return nil
	})
}

// checkJudgesUnmarked refuses to drop a judge who already marked a saved
// flight of the run: the stored scores would reference an absent judge.
func checkJudgesUnmarked(ctx context.Context, q *gen.Queries, compID string, idx int, keep []string) error {
	flights, err := q.ListFlightsOfRun(ctx, gen.ListFlightsOfRunParams{CompetitionID: compID, RunIdx: int64(idx)})
	if err != nil {
		return fmt.Errorf("list flights of run %d: %w", idx+1, err)
	}
	var errs []domain.FieldError
	seen := map[string]bool{}
	for _, f := range flights {
		marks, err := q.ListFlightMarks(ctx, f.ID)
		if err != nil {
			return fmt.Errorf("list marks of flight %d: %w", f.ID, err)
		}
		for _, m := range marks {
			if slices.Contains(keep, m.JudgeID) || seen[m.JudgeID] {
				continue
			}
			seen[m.JudgeID] = true
			name := m.JudgeID
			if j, err := loadJudge(ctx, q, m.JudgeID, true); err == nil {
				name = j.Name
			}
			errs = append(errs, domain.FieldError{Field: "judges", Msg: fmt.Sprintf("judge %s already marked saved flights of this run", name)})
		}
	}
	return invalid(errs...)
}

// rescoreFrom rescores the stored flights of run idx and of every later run,
// in run order, then rebuilds the rankings (a run config change changes the
// scores). An engine refusal is an ErrRescore and rolls the transaction back.
func rescoreFrom(ctx context.Context, q *gen.Queries, c Competition, idx int) error {
	env, err := loadScoringEnv(ctx, q, c)
	if err != nil {
		return err
	}
	for i := idx; i < len(env.runs); i++ {
		flights, err := q.ListFlightsOfRun(ctx, gen.ListFlightsOfRunParams{CompetitionID: c.ID, RunIdx: int64(i)})
		if err != nil {
			return fmt.Errorf("list flights of run %d: %w", i+1, err)
		}
		for _, f := range flights {
			p := ranking.Participant{CivlID: int(f.Civlid.Int64), TeamID: f.TeamID.String}
			if err := rescoreFlight(ctx, q, env, i, p, f); err != nil {
				return err
			}
		}
	}
	return rebuildRankings(ctx, q, c.ID, c.State == domain.CompetitionClosed)
}
