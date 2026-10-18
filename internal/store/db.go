// Package store is the SQLite access layer: connection pools, migrations and
// the sqlc generated queries.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	"awt/internal/store/gen"

	_ "modernc.org/sqlite" // pure Go SQLite driver (decision 0001)
)

// DB holds one write connection and a separate read pool on the same file.
type DB struct {
	Read  *sql.DB
	Write *sql.DB
}

const pragmas = "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"

func dsn(path string, readOnly bool) string {
	d := "file:" + (&url.URL{Path: path}).EscapedPath() + "?" + pragmas
	if readOnly {
		d += "&_pragma=query_only(1)"
	}
	return d
}

// Open opens the database file, creating it when missing. It does not migrate.
func Open(path string) (*DB, error) {
	w, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, fmt.Errorf("open write pool: %w", err)
	}
	w.SetMaxOpenConns(1)
	if err := w.Ping(); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	r, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("open read pool: %w", err)
	}
	if err := r.Ping(); err != nil {
		_ = w.Close()
		_ = r.Close()
		return nil, fmt.Errorf("open read pool %s: %w", path, err)
	}
	return &DB{Read: r, Write: w}, nil
}

// Close closes both pools.
func (d *DB) Close() error {
	return errors.Join(d.Read.Close(), d.Write.Close())
}

// Tx runs fn inside a BEGIN IMMEDIATE transaction on the write connection and
// rolls back when fn fails.
func (d *DB) Tx(ctx context.Context, fn func(q *gen.Queries) error) (err error) {
	conn, err := d.Write.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire write connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			// Never hand a connection with an open transaction back to the pool.
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
			panic(p)
		}
		if err != nil {
			// The context may be the reason we are failing: roll back regardless.
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()
	if err = fn(gen.New(conn)); err != nil {
		return err
	}
	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
