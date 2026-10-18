package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"awt/internal/domain"
	"awt/internal/store/gen"
)

// Judge is a judge (spec 02 §2.3). CivlID is 0 when the judge is not linked
// to a pilot.
type Judge struct {
	ID      string
	Name    string
	Country string
	Level   domain.JudgeLevel
	CivlID  int
}

// JudgeInput is the editable part of a judge.
type JudgeInput struct {
	Name    string
	Country string // alpha-3, stored lower-case (spec 05 §5.3)
	Level   domain.JudgeLevel
	CivlID  int
}

func judgeFromRow(j gen.Judge) Judge {
	return Judge{ID: j.ID, Name: j.Name, Country: j.Country, Level: domain.JudgeLevel(j.Level), CivlID: int(j.Civlid.Int64)}
}

func loadJudge(ctx context.Context, q *gen.Queries, id string, withDeleted bool) (gen.Judge, error) {
	j, err := q.GetJudge(ctx, id)
	if err != nil {
		return j, notFound(err, "judge "+id)
	}
	if j.DeletedAt.Valid && !withDeleted {
		return j, fmt.Errorf("judge %s: %w", id, ErrNotFound)
	}
	return j, nil
}

// GetJudge returns an active judge.
func (s *Service) GetJudge(ctx context.Context, id string) (Judge, error) {
	j, err := loadJudge(ctx, s.read(), id, false)
	return judgeFromRow(j), err
}

// ListJudges returns the active judges by name.
func (s *Service) ListJudges(ctx context.Context) ([]Judge, error) {
	rows, err := s.read().ListJudges(ctx)
	if err != nil {
		return nil, fmt.Errorf("list judges: %w", err)
	}
	out := make([]Judge, 0, len(rows))
	for _, r := range rows {
		out = append(out, judgeFromRow(r))
	}
	return out, nil
}

func (s *Service) saveJudge(ctx context.Context, id string, in JudgeInput, mustExist bool) (Judge, error) {
	in.Country = strings.ToLower(in.Country)
	var out gen.Judge
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		if mustExist {
			if _, err := loadJudge(ctx, q, id, false); err != nil {
				return err
			}
		}
		errs := domain.ValidateJudge(domain.Judge{Name: in.Name, Country: in.Country, Level: in.Level})
		civlid := sql.NullInt64{Int64: int64(in.CivlID), Valid: in.CivlID != 0}
		if civlid.Valid {
			if _, err := q.GetPilot(ctx, civlid.Int64); err != nil {
				errs = append(errs, domain.FieldError{Field: "civlid", Msg: fmt.Sprintf("pilot %d is unknown", in.CivlID)})
			}
		}
		if err := invalid(errs...); err != nil {
			return err
		}
		out = gen.Judge{ID: id, Name: in.Name, Country: in.Country, Level: string(in.Level), Civlid: civlid}
		return upsertJudge(ctx, q, out)
	})
	return judgeFromRow(out), err
}

func upsertJudge(ctx context.Context, q *gen.Queries, j gen.Judge) error {
	err := q.UpsertJudge(ctx, gen.UpsertJudgeParams(j))
	if err != nil {
		return conflict(err, "judge "+j.Name)
	}
	return nil
}

// CreateJudge creates a judge.
func (s *Service) CreateJudge(ctx context.Context, in JudgeInput) (Judge, error) {
	id, err := newID()
	if err != nil {
		return Judge{}, err
	}
	return s.saveJudge(ctx, id, in, false)
}

// UpdateJudge replaces an active judge's fields.
func (s *Service) UpdateJudge(ctx context.Context, id string, in JudgeInput) (Judge, error) {
	return s.saveJudge(ctx, id, in, true)
}

// DeleteJudge soft-deletes a judge.
func (s *Service) DeleteJudge(ctx context.Context, id string) error {
	return s.setJudgeDeleted(ctx, id, true)
}

// RestoreJudge restores a soft-deleted judge.
func (s *Service) RestoreJudge(ctx context.Context, id string) error {
	return s.setJudgeDeleted(ctx, id, false)
}

func (s *Service) setJudgeDeleted(ctx context.Context, id string, deleted bool) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		j, err := loadJudge(ctx, q, id, true)
		if err != nil {
			return err
		}
		if j.DeletedAt.Valid == deleted {
			return fmt.Errorf("judge %s: %w", id, ErrNotFound)
		}
		j.DeletedAt = deletedAt(deleted)
		return upsertJudge(ctx, q, j)
	})
}
