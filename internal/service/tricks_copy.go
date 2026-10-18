package service

import (
	"context"
	"fmt"

	"awt/internal/domain"
	"awt/internal/store/gen"
)

// DuplicateCatalog copies the active tricks of year from into year to, like
// an import: a trick whose name already exists in the target year is
// replaced, the others are created (decision 0008: a new season starts from
// the previous catalog). It returns the number of tricks copied.
func (s *Service) DuplicateCatalog(ctx context.Context, from, to int) (int, error) {
	var errs []domain.FieldError
	for _, y := range []struct {
		field string
		year  int
	}{{"from", from}, {"to", to}} {
		if y.year <= 1900 || y.year >= 3000 {
			errs = append(errs, domain.FieldError{Field: y.field, Msg: "must be a year between 1901 and 2999"})
		}
	}
	if from == to {
		errs = append(errs, domain.FieldError{Field: "to", Msg: "must differ from the source year"})
	}
	if err := invalid(errs...); err != nil {
		return 0, err
	}
	src, err := s.ListTricks(ctx, from)
	if err != nil {
		return 0, err
	}
	if len(src) == 0 {
		return 0, invalid(domain.FieldError{Field: "from", Msg: fmt.Sprintf("the %d catalog has no tricks", from)})
	}
	err = s.db.Tx(ctx, func(q *gen.Queries) error {
		for _, t := range src {
			id, err := activeTrickID(ctx, q, to, t.Name)
			if err != nil {
				return err
			}
			if err := saveTrick(ctx, q, id, to, t.Trick); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(src), nil
}
