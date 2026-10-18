// Package importer loads the production MongoDB export (mongoexport JSON
// lines, one file per collection, optionally inside a .tar.gz) into the SQLite
// schema. Reference data lives in reference.go; competitions are imported in a
// separate step.
package importer

import (
	"context"
	"errors"
	"fmt"

	"awt/internal/blob"
	"awt/internal/store"
	"awt/internal/store/gen"
)

// ErrNotEmpty is returned when the target database already holds imported data.
var ErrNotEmpty = errors.New("database is not empty: use replace to overwrite it")

// Report summarises an import: rows written per table and everything that was
// skipped or resolved by a rule.
type Report struct {
	Counts   map[string]int
	Warnings []string
}

func (r *Report) warnf(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

type options struct {
	catalogDir string
	replace    bool
}

// Option tunes Run.
type Option func(*options)

// WithCatalogDir sets the directory holding the 2022-2025 trick catalogs
// (<year>.json). Default "testdata/catalog".
func WithCatalogDir(dir string) Option { return func(o *options) { o.catalogDir = dir } }

// WithReplace deletes the rows of every imported table before importing,
// instead of refusing a non-empty database.
func WithReplace() Option { return func(o *options) { o.replace = true } }

// Run imports the export at src (a .tar.gz or a directory) in one
// transaction. File contents go to blobs under their sha256 id.
func Run(ctx context.Context, db *store.DB, blobs blob.Store, src string, opts ...Option) (Report, error) {
	o := options{catalogDir: "testdata/catalog"}
	for _, opt := range opts {
		opt(&o)
	}
	rep := Report{Counts: map[string]int{}}
	s, err := openSource(src)
	if err != nil {
		return rep, err
	}
	err = db.Tx(ctx, func(q *gen.Queries) error {
		if err := prepare(ctx, q, o.replace); err != nil {
			return err
		}
		return importReference(ctx, q, blobs, s, o, &rep)
	})
	if err != nil {
		return Report{Counts: map[string]int{}}, err
	}
	return rep, nil
}

// prepare enforces the empty-database rule, or clears the imported tables.
func prepare(ctx context.Context, q *gen.Queries, replace bool) error {
	if replace {
		for _, del := range []func(context.Context) error{
			q.DeleteAllRankingEntries, q.DeleteAllCompetitions, q.DeleteAllTeams, q.DeleteAllJudges,
			q.DeleteAllPilots, q.DeleteAllSeasons, q.DeleteAllTricks, q.DeleteAllFiles,
		} {
			if err := del(ctx); err != nil {
				return fmt.Errorf("clear tables: %w", err)
			}
		}
		return nil
	}
	pilots, err := q.CountPilots(ctx)
	if err != nil {
		return fmt.Errorf("count pilots: %w", err)
	}
	comps, err := q.CountCompetitions(ctx)
	if err != nil {
		return fmt.Errorf("count competitions: %w", err)
	}
	if pilots > 0 || comps > 0 {
		return ErrNotEmpty
	}
	return nil
}
