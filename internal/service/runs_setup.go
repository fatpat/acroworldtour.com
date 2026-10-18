package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"awt/internal/domain"
	"awt/internal/store/gen"
)

// runSetup loads an editable competition and one of its runs and applies fn
// to the stored run row. A closed run must be reopened first (spec 05 §8:
// the backend enforces the run state on roster and config edits).
func (s *Service) runSetup(ctx context.Context, compID string, idx int, fn func(q *gen.Queries, c Competition, r Run, row *gen.Run) error) error {
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
		return q.UpsertRun(ctx, gen.UpsertRunParams{
			CompetitionID: compID, Idx: row.Idx, State: row.State, Config: row.Config,
			ResetPolicy: row.ResetPolicy, ResultsFinal: row.ResultsFinal, UpdatedAt: nullString(now()),
		})
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
	return s.runSetup(ctx, compID, idx, func(q *gen.Queries, c Competition, _ Run, _ *gen.Run) error {
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
	return s.runSetup(ctx, compID, idx, func(_ *gen.Queries, _ Competition, _ Run, row *gen.Run) error {
		row.Config = compact.String()
		return nil
	})
}
