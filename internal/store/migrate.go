package store

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"awt/internal/store/seed"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate applies the embedded migrations not yet recorded in
// schema_migrations, then inserts the default rulesets that are missing.
// Existing rulesets are never overwritten: they are edited data.
func Migrate(ctx context.Context, db *DB) error {
	if _, err := db.Write.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	names, err := migrationNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		var done int
		if err := db.Write.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE version = ?`, version).Scan(&done); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if done > 0 {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}
		if err := applyMigration(ctx, db, version, string(body)); err != nil {
			return err
		}
	}
	return seedRulesets(ctx, db)
}

// migrationNames lists the embedded migration files in order.
func migrationNames() ([]string, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// UnknownMigrations returns the versions recorded in schema_migrations that
// this binary does not embed, sorted: the database comes from a newer
// release. Migrate ignores them; the training refresh refuses them
// (decision 0018), as their tables may hold auth state it does not clear.
func UnknownMigrations(ctx context.Context, db *DB) ([]string, error) {
	names, err := migrationNames()
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, n := range names {
		known[strings.TrimSuffix(n, ".sql")] = true
	}
	rows, err := db.Read.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("list schema_migrations: %w", err)
	}
	defer rows.Close()
	var unknown []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("list schema_migrations: %w", err)
		}
		if !known[v] {
			unknown = append(unknown, v)
		}
	}
	return unknown, rows.Err()
}

func applyMigration(ctx context.Context, db *DB, version, body string) error {
	conn, err := db.Write.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migration %s: %w", version, err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("migration %s: begin: %w", version, err)
	}
	_, err = conn.ExecContext(ctx, body)
	if err == nil {
		_, err = conn.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, version)
	}
	if err != nil {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		return fmt.Errorf("migration %s: %w", version, err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("migration %s: commit: %w", version, err)
	}
	return nil
}

func seedRulesets(ctx context.Context, db *DB) error {
	for _, r := range seed.Rulesets() {
		_, err := db.Write.ExecContext(ctx, `INSERT OR IGNORE INTO rulesets
			(year, config, max_tricks_per_type, tip_touch_pct_per_touch, tip_touch_pct_max, default_max_bonus)
			VALUES (?, ?, ?, ?, ?, ?)`,
			r.Year, r.Config, r.MaxTricksPerType, r.TipTouchPctPerTouch, r.TipTouchPctMax, r.DefaultMaxBonus)
		if err != nil {
			return fmt.Errorf("seed ruleset %d: %w", r.Year, err)
		}
	}
	return nil
}
