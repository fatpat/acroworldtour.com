package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"awt/internal/domain"
	"awt/internal/store/gen"
)

// Season is a season (spec 02 §2.11).
type Season struct {
	ID        string
	Code      string
	Name      string
	Year      int
	Country   string
	ImageFile string
	SortIndex int
}

// SeasonInput is the editable part of a season. Code is forced to
// {country}-{year} when Country is set.
type SeasonInput struct {
	Name      string
	Code      string
	Year      int
	Country   string
	ImageFile string
	SortIndex int
}

func seasonFromRow(r gen.Season) Season {
	return Season{ID: r.ID, Code: r.Code, Name: r.Name, Year: int(r.Year), Country: r.Country.String,
		ImageFile: r.ImageFile.String, SortIndex: int(r.SortIndex)}
}

// GetSeason returns an active season by code.
func (s *Service) GetSeason(ctx context.Context, code string) (Season, error) {
	r, err := s.read().GetSeasonByCode(ctx, code)
	if err != nil {
		return Season{}, notFound(err, "season "+code)
	}
	return seasonFromRow(r), nil
}

// ListSeasons returns the active seasons, latest year first.
func (s *Service) ListSeasons(ctx context.Context) ([]Season, error) {
	rows, err := s.read().ListSeasons(ctx)
	if err != nil {
		return nil, fmt.Errorf("list seasons: %w", err)
	}
	out := make([]Season, 0, len(rows))
	for _, r := range rows {
		out = append(out, seasonFromRow(r))
	}
	return out, nil
}

func (s *Service) saveSeason(ctx context.Context, id string, in SeasonInput, mustExist bool) (Season, error) {
	d := domain.Season{Name: in.Name, Code: in.Code, Year: in.Year, Country: in.Country}
	in.Code = d.EffectiveCode()
	var out gen.Season
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		if mustExist {
			r, err := q.GetSeason(ctx, id)
			if err != nil || r.DeletedAt.Valid {
				return fmt.Errorf("season %s: %w", id, ErrNotFound)
			}
		}
		errs := domain.ValidateSeason(d)
		if in.ImageFile != "" {
			if err := checkFile(ctx, q, in.ImageFile); err != nil {
				errs = append(errs, domain.FieldError{Field: "image", Msg: err.Error()})
			}
		}
		if err := invalid(errs...); err != nil {
			return err
		}
		out = gen.Season{ID: id, Code: in.Code, Name: in.Name, Year: int64(in.Year), Country: nullString(in.Country),
			ImageFile: nullString(in.ImageFile), SortIndex: int64(in.SortIndex)}
		return upsertSeason(ctx, q, out)
	})
	return seasonFromRow(out), err
}

func upsertSeason(ctx context.Context, q *gen.Queries, r gen.Season) error {
	err := q.UpsertSeason(ctx, gen.UpsertSeasonParams(r))
	if err != nil {
		return conflict(err, "season "+r.Code)
	}
	return nil
}

// CreateSeason creates a season.
func (s *Service) CreateSeason(ctx context.Context, in SeasonInput) (Season, error) {
	id, err := newID()
	if err != nil {
		return Season{}, err
	}
	return s.saveSeason(ctx, id, in, false)
}

// UpdateSeason replaces an active season's fields. Competitions reference
// seasons by code, so a code change does not follow into competitions (as
// in legacy).
func (s *Service) UpdateSeason(ctx context.Context, id string, in SeasonInput) (Season, error) {
	return s.saveSeason(ctx, id, in, true)
}

// DeleteSeason soft-deletes a season.
func (s *Service) DeleteSeason(ctx context.Context, id string) error {
	return s.setSeasonDeleted(ctx, id, true)
}

// RestoreSeason restores a soft-deleted season.
func (s *Service) RestoreSeason(ctx context.Context, id string) error {
	return s.setSeasonDeleted(ctx, id, false)
}

func (s *Service) setSeasonDeleted(ctx context.Context, id string, deleted bool) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		r, err := q.GetSeason(ctx, id)
		if err != nil {
			return notFound(err, "season "+id)
		}
		if r.DeletedAt.Valid == deleted {
			return fmt.Errorf("season %s: %w", id, ErrNotFound)
		}
		r.DeletedAt = deletedAt(deleted)
		return upsertSeason(ctx, q, r)
	})
}

// checkFile reports a missing or deleted file.
func checkFile(ctx context.Context, q *gen.Queries, id string) error {
	f, err := q.GetFile(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && f.DeletedAt.Valid) {
		return fmt.Errorf("file %s is unknown", id)
	}
	return err
}
