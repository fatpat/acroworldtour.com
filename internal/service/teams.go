package service

import (
	"context"
	"database/sql"
	"fmt"

	"awt/internal/domain"
	"awt/internal/store/gen"
)

// Team is a synchro team of exactly two pilots, in stored order.
type Team struct {
	ID     string
	Name   string
	Pilots []int
}

// TeamInput is the editable part of a team.
type TeamInput struct {
	Name   string
	Pilots []int
}

func loadTeam(ctx context.Context, q *gen.Queries, id string, withDeleted bool) (Team, gen.Team, error) {
	t, err := q.GetTeam(ctx, id)
	if err != nil {
		return Team{}, t, notFound(err, "team "+id)
	}
	if t.DeletedAt.Valid && !withDeleted {
		return Team{}, t, fmt.Errorf("team %s: %w", id, ErrNotFound)
	}
	rows, err := q.ListTeamPilots(ctx, id)
	if err != nil {
		return Team{}, t, fmt.Errorf("list pilots of team %s: %w", id, err)
	}
	out := Team{ID: t.ID, Name: t.Name}
	for _, r := range rows {
		out.Pilots = append(out.Pilots, int(r.Civlid))
	}
	return out, t, nil
}

// GetTeam returns an active team.
func (s *Service) GetTeam(ctx context.Context, id string) (Team, error) {
	t, _, err := loadTeam(ctx, s.read(), id, false)
	return t, err
}

// ListTeams returns the active teams by name.
func (s *Service) ListTeams(ctx context.Context) ([]Team, error) {
	q := s.read()
	rows, err := q.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	out := make([]Team, 0, len(rows))
	for _, r := range rows {
		t, _, err := loadTeam(ctx, q, r.ID, false)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// validateTeam checks spec 02 §2.2 plus distinct pilots (spec 05 §8).
func validateTeam(ctx context.Context, q *gen.Queries, in TeamInput) error {
	errs := domain.ValidateTeam(domain.Team{Name: in.Name, Pilots: in.Pilots})
	if len(in.Pilots) == 2 && in.Pilots[0] == in.Pilots[1] {
		errs = append(errs, domain.FieldError{Field: "pilots", Msg: "the two pilots must be different"})
	}
	for _, p := range in.Pilots {
		if _, err := q.GetPilot(ctx, int64(p)); err != nil {
			errs = append(errs, domain.FieldError{Field: "pilots", Msg: fmt.Sprintf("pilot %d is unknown", p)})
		}
	}
	return invalid(errs...)
}

func saveTeam(ctx context.Context, q *gen.Queries, id string, in TeamInput) error {
	if err := q.UpsertTeam(ctx, gen.UpsertTeamParams{ID: id, Name: in.Name}); err != nil {
		return conflict(err, "team "+in.Name)
	}
	for pos, p := range in.Pilots {
		if err := q.SetTeamPilot(ctx, gen.SetTeamPilotParams{TeamID: id, Position: int64(pos), Civlid: int64(p)}); err != nil {
			return fmt.Errorf("save pilots of team %s: %w", id, err)
		}
	}
	return nil
}

// CreateTeam creates a team.
func (s *Service) CreateTeam(ctx context.Context, in TeamInput) (Team, error) {
	id, err := newID()
	if err != nil {
		return Team{}, err
	}
	var out Team
	err = s.db.Tx(ctx, func(q *gen.Queries) error {
		if err := validateTeam(ctx, q, in); err != nil {
			return err
		}
		if err := saveTeam(ctx, q, id, in); err != nil {
			return err
		}
		out, _, err = loadTeam(ctx, q, id, false)
		return err
	})
	return out, err
}

// UpdateTeam replaces an active team's name and pilots.
func (s *Service) UpdateTeam(ctx context.Context, id string, in TeamInput) (Team, error) {
	var out Team
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		if _, _, err := loadTeam(ctx, q, id, false); err != nil {
			return err
		}
		if err := validateTeam(ctx, q, in); err != nil {
			return err
		}
		if err := saveTeam(ctx, q, id, in); err != nil {
			return err
		}
		var err error
		out, _, err = loadTeam(ctx, q, id, false)
		return err
	})
	return out, err
}

// DeleteTeam soft-deletes a team.
func (s *Service) DeleteTeam(ctx context.Context, id string) error {
	return s.setTeamDeleted(ctx, id, true)
}

// RestoreTeam restores a soft-deleted team.
func (s *Service) RestoreTeam(ctx context.Context, id string) error {
	return s.setTeamDeleted(ctx, id, false)
}

func (s *Service) setTeamDeleted(ctx context.Context, id string, deleted bool) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		_, row, err := loadTeam(ctx, q, id, true)
		if err != nil {
			return err
		}
		if row.DeletedAt.Valid == deleted {
			return fmt.Errorf("team %s: %w", id, ErrNotFound)
		}
		row.DeletedAt = deletedAt(deleted)
		if err := q.UpsertTeam(ctx, gen.UpsertTeamParams(row)); err != nil {
			return conflict(err, "team "+row.Name)
		}
		return nil
	})
}

// deletedAt is the deleted_at value for a soft delete (now) or a restore.
func deletedAt(deleted bool) sql.NullString {
	if deleted {
		return nullString(now())
	}
	return sql.NullString{}
}
