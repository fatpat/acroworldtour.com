package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"awt/internal/catalog"
	"awt/internal/domain"
	"awt/internal/store/gen"
)

// Trick is a catalog trick of one year (decision 0008); Trick.Tricks holds
// its generated unique tricks (empty when soft-deleted).
type Trick struct {
	ID   string
	Year int
	catalog.Trick
}

// jsonArray encodes v, writing a nil slice as [].
func jsonArray(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	if string(b) == "null" {
		return "[]", nil
	}
	return string(b), nil
}

func decodeJSON(s string, v any) error {
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), v)
}

func trickFromRow(ctx context.Context, q *gen.Queries, r gen.Trick) (Trick, error) {
	t := Trick{ID: r.ID, Year: int(r.Year), Trick: catalog.Trick{
		Name: r.Name, Acronym: r.Acronym, Solo: r.Solo, SoloAWT: r.SoloAwt, Synchro: r.Synchro,
		TechnicalCoefficient: r.TechnicalCoefficient, FirstManeuver: int(r.FirstManeuver), NoFirstManeuver: int(r.NoFirstManeuver),
		LastManeuver: int(r.LastManeuver), NoLastManeuver: int(r.NoLastManeuver), Repeatable: r.Repeatable,
		SampleVideo: r.SampleVideo.String,
	}}
	if err := errors.Join(decodeJSON(r.Directions, &t.Directions), decodeJSON(r.Bonuses, &t.Bonuses),
		decodeJSON(r.BonusConstraints, &t.BonusConstraints), decodeJSON(r.Types, &t.Types)); err != nil {
		return Trick{}, fmt.Errorf("decode trick %s: %w", r.ID, err)
	}
	rows, err := q.ListUniqueTricksByTrick(ctx, r.ID)
	if err != nil {
		return Trick{}, fmt.Errorf("list unique tricks of %s: %w", r.ID, err)
	}
	for _, u := range rows {
		ut, err := uniqueTrickFromRow(u)
		if err != nil {
			return Trick{}, err
		}
		t.Tricks = append(t.Tricks, ut)
	}
	return t, nil
}

func uniqueTrickFromRow(u gen.UniqueTrick) (catalog.UniqueTrick, error) {
	out := catalog.UniqueTrick{
		Name: u.Name, Acronym: u.Acronym, TechnicalCoefficient: u.TechnicalCoefficient, Bonus: u.Bonus,
		BaseTrick: u.BaseTrick, Synchro: u.Synchro, Solo: u.Solo, SoloAWT: u.SoloAwt,
	}
	if err := errors.Join(decodeJSON(u.BonusTypes, &out.BonusTypes), decodeJSON(u.Uniqueness, &out.Uniqueness),
		decodeJSON(u.Bonuses, &out.Bonuses), decodeJSON(u.Types, &out.Types)); err != nil {
		return out, fmt.Errorf("decode unique trick %s: %w", u.Acronym, err)
	}
	return out, nil
}

func loadTrick(ctx context.Context, q *gen.Queries, id string, withDeleted bool) (gen.Trick, error) {
	t, err := q.GetTrick(ctx, id)
	if err != nil {
		return t, notFound(err, "trick "+id)
	}
	if t.DeletedAt.Valid && !withDeleted {
		return t, fmt.Errorf("trick %s: %w", id, ErrNotFound)
	}
	return t, nil
}

// GetTrick returns an active trick with its unique tricks.
func (s *Service) GetTrick(ctx context.Context, id string) (Trick, error) {
	q := s.read()
	r, err := loadTrick(ctx, q, id, false)
	if err != nil {
		return Trick{}, err
	}
	return trickFromRow(ctx, q, r)
}

// ListTricks returns the active tricks of a year's catalog by name.
func (s *Service) ListTricks(ctx context.Context, year int) ([]Trick, error) {
	q := s.read()
	rows, err := q.ListTricksByYear(ctx, int64(year))
	if err != nil {
		return nil, fmt.Errorf("list tricks %d: %w", year, err)
	}
	out := make([]Trick, 0, len(rows))
	for _, r := range rows {
		t, err := trickFromRow(ctx, q, r)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// ListUniqueTricks returns the unique tricks of a year's catalog by acronym.
func (s *Service) ListUniqueTricks(ctx context.Context, year int) ([]catalog.UniqueTrick, error) {
	rows, err := s.read().ListUniqueTricksByYear(ctx, int64(year))
	if err != nil {
		return nil, fmt.Errorf("list unique tricks %d: %w", year, err)
	}
	out := make([]catalog.UniqueTrick, 0, len(rows))
	for _, r := range rows {
		u, err := uniqueTrickFromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

var directions = map[string]bool{"left": true, "right": true, "opposite": true}

// validateTrick checks a catalog trick (spec 02 §2.4.1-2.4.2).
func validateTrick(year int, t catalog.Trick) []domain.FieldError {
	var errs []domain.FieldError
	add := func(f, m string) { errs = append(errs, domain.FieldError{Field: f, Msg: m}) }
	if year <= 1900 {
		add("year", "must be after 1900")
	}
	if strings.TrimSpace(t.Name) == "" {
		add("name", "required")
	}
	if strings.TrimSpace(t.Acronym) == "" {
		add("acronym", "required")
	}
	seen := map[string]bool{}
	for _, d := range t.Directions {
		if !directions[d] {
			add("directions", fmt.Sprintf("unknown direction %q", d))
		}
		seen[d] = true
	}
	if len(t.Directions) > 0 && len(seen) < 2 {
		add("directions", "none, or at least 2 distinct directions")
	}
	if t.TechnicalCoefficient < 0 {
		add("technical_coefficient", "must be >= 0")
	}
	for _, b := range t.Bonuses {
		if _, ok := catalog.LookupBonus(b.Name); !ok {
			add("bonuses", fmt.Sprintf("unknown bonus %q", b.Name))
		}
		if b.Bonus < 0 {
			add("bonuses", fmt.Sprintf("bonus %q must be >= 0", b.Name))
		}
	}
	for _, c := range t.BonusConstraints {
		if len(c) != 2 {
			add("bonus_constraints", "each constraint is a pair of bonus names")
		}
	}
	if t.FirstManeuver < 0 || t.NoFirstManeuver < 0 || t.LastManeuver < 0 || t.NoLastManeuver < 0 {
		add("maneuvers", "position rules must be >= 0")
	}
	return errs
}

// saveTrick writes the trick and regenerates its unique tricks.
func saveTrick(ctx context.Context, q *gen.Queries, id string, year int, t catalog.Trick) error {
	var cols [4]string
	for i, v := range []any{t.Directions, t.Bonuses, t.BonusConstraints, t.Types} {
		s, err := jsonArray(v)
		if err != nil {
			return err
		}
		cols[i] = s
	}
	err := q.UpsertTrick(ctx, gen.UpsertTrickParams{
		ID: id, Year: int64(year), Name: t.Name, Acronym: t.Acronym, Solo: t.Solo, SoloAwt: t.SoloAWT, Synchro: t.Synchro,
		Directions: cols[0], TechnicalCoefficient: t.TechnicalCoefficient, Bonuses: cols[1], BonusConstraints: cols[2],
		FirstManeuver: int64(t.FirstManeuver), NoFirstManeuver: int64(t.NoFirstManeuver),
		LastManeuver: int64(t.LastManeuver), NoLastManeuver: int64(t.NoLastManeuver),
		Repeatable: t.Repeatable, Types: cols[3], SampleVideo: nullString(t.SampleVideo),
	})
	if err != nil {
		return conflict(err, fmt.Sprintf("trick %q (%d)", t.Name, year))
	}
	return generateUniqueTricks(ctx, q, id, year, t)
}

func generateUniqueTricks(ctx context.Context, q *gen.Queries, id string, year int, t catalog.Trick) error {
	if err := q.DeleteUniqueTricksByTrick(ctx, id); err != nil {
		return fmt.Errorf("delete unique tricks of %s: %w", id, err)
	}
	for _, u := range catalog.Generate(t) {
		var cols [4]string
		for i, v := range []any{u.BonusTypes, u.Uniqueness, u.Bonuses, u.Types} {
			s, err := jsonArray(v)
			if err != nil {
				return err
			}
			cols[i] = s
		}
		err := q.InsertUniqueTrick(ctx, gen.InsertUniqueTrickParams{
			TrickID: id, Year: int64(year), Acronym: u.Acronym, Name: u.Name, BaseTrick: u.BaseTrick,
			TechnicalCoefficient: u.TechnicalCoefficient, Bonus: u.Bonus, BonusTypes: cols[0], Uniqueness: cols[1],
			Bonuses: cols[2], Solo: u.Solo, SoloAwt: u.SoloAWT, Synchro: u.Synchro, Types: cols[3],
		})
		if err != nil {
			return conflict(err, fmt.Sprintf("unique trick %q (%d)", u.Acronym, year))
		}
	}
	return nil
}

func (s *Service) writeTrick(ctx context.Context, id string, year int, t catalog.Trick, mustExist bool) (Trick, error) {
	var out Trick
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		if mustExist {
			row, err := loadTrick(ctx, q, id, false)
			if err != nil {
				return err
			}
			year = int(row.Year) // a trick stays in its catalog year
		}
		if err := invalid(validateTrick(year, t)...); err != nil {
			return err
		}
		if err := saveTrick(ctx, q, id, year, t); err != nil {
			return err
		}
		row, err := q.GetTrick(ctx, id)
		if err != nil {
			return err
		}
		out, err = trickFromRow(ctx, q, row)
		return err
	})
	return out, err
}

// CreateTrick adds a trick to a year's catalog and generates its variants.
func (s *Service) CreateTrick(ctx context.Context, year int, t catalog.Trick) (Trick, error) {
	id, err := newID()
	if err != nil {
		return Trick{}, err
	}
	return s.writeTrick(ctx, id, year, t, false)
}

// UpdateTrick replaces an active trick and regenerates its variants. Flights
// keep their own snapshot (spec 02 §2.4.5).
func (s *Service) UpdateTrick(ctx context.Context, id string, t catalog.Trick) (Trick, error) {
	return s.writeTrick(ctx, id, 0, t, true)
}

// DeleteTrick soft-deletes a trick and deletes its unique tricks, so its
// acronyms can be reused.
func (s *Service) DeleteTrick(ctx context.Context, id string) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		r, err := loadTrick(ctx, q, id, false)
		if err != nil {
			return err
		}
		r.DeletedAt = deletedAt(true)
		if err := upsertTrickRow(ctx, q, r); err != nil {
			return err
		}
		return q.DeleteUniqueTricksByTrick(ctx, id)
	})
}

// RestoreTrick restores a soft-deleted trick and regenerates its unique
// tricks.
func (s *Service) RestoreTrick(ctx context.Context, id string) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		r, err := loadTrick(ctx, q, id, true)
		if err != nil {
			return err
		}
		if !r.DeletedAt.Valid {
			return fmt.Errorf("deleted trick %s: %w", id, ErrNotFound)
		}
		r.DeletedAt = sql.NullString{}
		if err := upsertTrickRow(ctx, q, r); err != nil {
			return err
		}
		t, err := trickFromRow(ctx, q, r)
		if err != nil {
			return err
		}
		return generateUniqueTricks(ctx, q, id, t.Year, t.Trick)
	})
}

func upsertTrickRow(ctx context.Context, q *gen.Queries, r gen.Trick) error {
	err := q.UpsertTrick(ctx, gen.UpsertTrickParams(r))
	if err != nil {
		return conflict(err, fmt.Sprintf("trick %q (%d)", r.Name, r.Year))
	}
	return nil
}

// ImportCatalog loads a JSON array of tricks into a year's catalog, updating
// the active trick of the same name or creating it. It returns the number of
// tricks imported.
func (s *Service) ImportCatalog(ctx context.Context, year int, r io.Reader) (int, error) {
	tricks, err := catalog.LoadJSON(r)
	if err != nil {
		return 0, invalid(domain.FieldError{Field: "catalog", Msg: err.Error()})
	}
	var errs []domain.FieldError
	for _, t := range tricks {
		for _, e := range validateTrick(year, t) {
			errs = append(errs, domain.FieldError{Field: e.Field, Msg: fmt.Sprintf("%q: %s", t.Name, e.Msg)})
		}
	}
	if err := invalid(errs...); err != nil {
		return 0, err
	}
	err = s.db.Tx(ctx, func(q *gen.Queries) error {
		for _, t := range tricks {
			id, err := activeTrickID(ctx, q, year, t.Name)
			if err != nil {
				return err
			}
			if err := saveTrick(ctx, q, id, year, t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(tricks), nil
}

// activeTrickID returns the id of the active trick (year, name), or a new id.
func activeTrickID(ctx context.Context, q *gen.Queries, year int, name string) (string, error) {
	t, err := q.GetActiveTrickByYearName(ctx, gen.GetActiveTrickByYearNameParams{Year: int64(year), Name: name})
	if errors.Is(err, sql.ErrNoRows) {
		return newID()
	}
	if err != nil {
		return "", fmt.Errorf("get trick %q (%d): %w", name, year, err)
	}
	return t.ID, nil
}
