package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"awt/internal/store"
	"awt/internal/store/gen"
	"awt/internal/store/seed"
)

func openMigrated(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "awt.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func isConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "constraint")
}

func TestMigrateCreatesSchemaAndEnforcesFKs(t *testing.T) {
	db := openMigrated(t)
	ctx := context.Background()

	var n int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('pilots','flights','schema_migrations')`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("tables present = %d, err %v", n, err)
	}

	_, err := db.Write.ExecContext(ctx, `INSERT INTO teams (id, name) VALUES ('t1', 'Team')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Write.ExecContext(ctx, `INSERT INTO team_pilots (team_id, position, civlid) VALUES ('t1', 0, 424242)`)
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("expected FK error, got %v", err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := openMigrated(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var versions, rulesets int
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM rulesets`).Scan(&rulesets); err != nil {
		t.Fatal(err)
	}
	if versions != 3 || rulesets != 5 { // one row per migration file
		t.Fatalf("versions=%d rulesets=%d, want 3 and 5", versions, rulesets)
	}
}

func TestMigrateKeepsEditedRulesets(t *testing.T) {
	db := openMigrated(t)
	ctx := context.Background()
	if _, err := db.Write.ExecContext(ctx, `UPDATE rulesets SET default_max_bonus = 99 WHERE year = 2025`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM rulesets WHERE year = 2022`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var v float64
	var n int
	if err := db.Read.QueryRowContext(ctx, `SELECT default_max_bonus FROM rulesets WHERE year = 2025`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRowContext(ctx, `SELECT count(*) FROM rulesets`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if v != 99 || n != 5 {
		t.Fatalf("edited=%v count=%d, want 99 and 5", v, n)
	}
}

func TestTxRollsBackOnError(t *testing.T) {
	db := openMigrated(t)
	ctx := context.Background()
	boom := errors.New("boom")

	err := db.Tx(ctx, func(q *gen.Queries) error {
		if err := q.UpsertPilot(ctx, gen.UpsertPilotParams{Civlid: 1, Name: "A", Gender: "man", SocialLinks: "[]", Sponsors: "[]"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if _, err := gen.New(db.Read).GetPilot(ctx, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("pilot survived rollback: %v", err)
	}

	err = db.Tx(ctx, func(q *gen.Queries) error {
		return q.UpsertPilot(ctx, gen.UpsertPilotParams{Civlid: 2, Name: "B", Gender: "man", SocialLinks: "[]", Sponsors: "[]"})
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := gen.New(db.Read).GetPilot(ctx, 2)
	if err != nil || p.Name != "B" {
		t.Fatalf("committed pilot: %+v %v", p, err)
	}
}

func TestReadPoolIsReadOnly(t *testing.T) {
	db := openMigrated(t)
	_, err := db.Read.ExecContext(context.Background(), `INSERT INTO pages (slug, title) VALUES ('a', 'b')`)
	if err == nil {
		t.Fatal("write through read pool succeeded")
	}
}

func TestSeedRulesets2025(t *testing.T) {
	db := openMigrated(t)
	r, err := gen.New(db.Read).GetRuleset(context.Background(), 2025)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(r.Config), &cfg); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"warning": 0.5, "warning2": 1.0, "malus_repetition": 13.0, "warnings_to_dsq": 3.0,
		"judge_weights":     map[string]any{"senior": 100.0, "certified": 100.0, "trainee": 20.0},
		"mark_percentages":  map[string]any{"solo": map[string]any{"technical": 40.0, "choreography": 40.0, "landing": 20.0}, "synchro": map[string]any{"technical": 20.0, "choreography": 20.0, "landing": 20.0, "synchro": 40.0}},
		"max_bonus_per_run": map[string]any{"twist": 5.0, "reverse": 3.0, "flip": 2.0},
	}
	gotJSON, _ := json.Marshal(cfg)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("config\n got %s\nwant %s", gotJSON, wantJSON)
	}
	if r.MaxTricksPerType != `{"tumbling":2,"to infinite":1}` {
		t.Fatalf("max_tricks_per_type = %s", r.MaxTricksPerType)
	}
	if r.TipTouchPctPerTouch != 2 || r.TipTouchPctMax != 6 || r.DefaultMaxBonus != 10 {
		t.Fatalf("tip touch/default bonus = %v %v %v", r.TipTouchPctPerTouch, r.TipTouchPctMax, r.DefaultMaxBonus)
	}
	years := []int{}
	for _, s := range seed.Rulesets() {
		years = append(years, s.Year)
	}
	if len(years) != 5 || years[0] != 2022 || years[4] != 2026 {
		t.Fatalf("years = %v", years)
	}
}

func exec(t *testing.T, db *store.DB, q string, args ...any) error {
	t.Helper()
	_, err := db.Write.ExecContext(context.Background(), q, args...)
	return err
}

func TestSoftDeletedTrickDoesNotBlockRecreation(t *testing.T) {
	db := openMigrated(t)
	ins := `INSERT INTO tricks (id, year, name, acronym, solo, synchro, technical_coefficient, deleted_at) VALUES (?, 2025, 'Misty Flip', 'M', 1, 1, 1.65, ?)`
	if err := exec(t, db, ins, "a", nil); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, db, ins, "b", nil); !isConstraint(err) {
		t.Fatalf("duplicate active trick accepted: %v", err)
	}
	if err := exec(t, db, `UPDATE tricks SET deleted_at = '2025-01-01T00:00:00Z' WHERE id = 'a'`); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, db, ins, "b", nil); err != nil {
		t.Fatalf("recreation after soft delete rejected: %v", err)
	}
	if err := exec(t, db, ins, "c", "2025-02-01T00:00:00Z"); err != nil {
		t.Fatalf("second soft-deleted row rejected: %v", err)
	}
}

func TestFlightUniquenessPerParticipant(t *testing.T) {
	db := openMigrated(t)
	for _, s := range []string{
		`INSERT INTO pilots (civlid, name) VALUES (1, 'A')`,
		`INSERT INTO teams (id, name) VALUES ('t1', 'T')`,
		`INSERT INTO competitions (id, code, name, type, start_date, end_date, config) VALUES ('c1', 'c', 'C', 'solo', '2025-01-01', '2025-01-02', '{}')`,
		`INSERT INTO runs (competition_id, idx, config) VALUES ('c1', 0, '{}')`,
	} {
		if err := exec(t, db, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	pilotFlight := `INSERT INTO flights (competition_id, run_idx, civlid) VALUES ('c1', 0, 1)`
	if err := exec(t, db, pilotFlight); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, db, pilotFlight); !isConstraint(err) {
		t.Fatalf("duplicate pilot flight accepted: %v", err)
	}
	teamFlight := `INSERT INTO flights (competition_id, run_idx, team_id) VALUES ('c1', 0, 't1')`
	if err := exec(t, db, teamFlight); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, db, teamFlight); !isConstraint(err) {
		t.Fatalf("duplicate team flight accepted: %v", err)
	}
	if err := exec(t, db, `INSERT INTO flights (competition_id, run_idx) VALUES ('c1', 0)`); !isConstraint(err) {
		t.Fatalf("flight without participant accepted: %v", err)
	}
}

func TestJSONColumnsAreValidated(t *testing.T) {
	db := openMigrated(t)
	err := exec(t, db, `INSERT INTO site_settings (key, value) VALUES ('k', 'not json')`)
	if !isConstraint(err) {
		t.Fatalf("invalid JSON accepted: %v", err)
	}
}

func TestTxPanicRollsBackAndKeepsWriterUsable(t *testing.T) {
	db := openMigrated(t)
	ctx := context.Background()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic was swallowed")
			}
		}()
		_ = db.Tx(ctx, func(q *gen.Queries) error {
			if err := q.UpsertPage(ctx, gen.UpsertPageParams{Slug: "p", Title: "t"}); err != nil {
				return err
			}
			panic("boom")
		})
	}()
	tctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := db.Tx(tctx, func(q *gen.Queries) error {
		return q.UpsertPage(tctx, gen.UpsertPageParams{Slug: "q", Title: "t"})
	})
	if err != nil {
		t.Fatalf("writer poisoned: %v", err)
	}
	if _, err := gen.New(db.Read).GetPage(ctx, "p"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("panicked tx survived: %v", err)
	}
}

func TestCompetitionAndSeasonCodeNameReuseAfterSoftDelete(t *testing.T) {
	db := openMigrated(t)
	comp := `INSERT INTO competitions (id, code, name, type, start_date, end_date, config, deleted_at) VALUES (?, ?, ?, 'solo', '2025-01-01', '2025-01-02', '{}', ?)`
	season := `INSERT INTO seasons (id, code, name, year, deleted_at) VALUES (?, ?, ?, 2025, ?)`
	for _, c := range []struct {
		table, q string
		args     func(id string, del any) []any
	}{
		{"competition", comp, func(id string, del any) []any { return []any{id, "cc", "Comp", del} }},
		{"season", season, func(id string, del any) []any { return []any{id, "awt-2025", "AWT", del} }},
	} {
		if err := exec(t, db, c.q, c.args("a", nil)...); err != nil {
			t.Fatal(err)
		}
		if err := exec(t, db, c.q, c.args("b", nil)...); !isConstraint(err) {
			t.Fatalf("%s: duplicate active code/name accepted: %v", c.table, err)
		}
	}
	// name alone and code alone are each protected
	if err := exec(t, db, comp, "n", "other", "Comp", nil); !isConstraint(err) {
		t.Fatalf("duplicate competition name accepted: %v", err)
	}
	if err := exec(t, db, season, "n", "awt-2025", "Other", nil); !isConstraint(err) {
		t.Fatalf("duplicate season code accepted: %v", err)
	}
	if err := exec(t, db, season, "n", "xxx-2025", "AWT", nil); !isConstraint(err) {
		t.Fatalf("duplicate season name accepted: %v", err)
	}
	for _, q := range []string{`UPDATE competitions SET deleted_at = '2025-01-01T00:00:00Z' WHERE id = 'a'`, `UPDATE seasons SET deleted_at = '2025-01-01T00:00:00Z' WHERE id = 'a'`} {
		if err := exec(t, db, q); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec(t, db, comp, "b", "cc", "Comp", nil); err != nil {
		t.Fatalf("competition recreation rejected: %v", err)
	}
	if err := exec(t, db, season, "b", "awt-2025", "AWT", nil); err != nil {
		t.Fatalf("season recreation rejected: %v", err)
	}
	s, err := gen.New(db.Read).GetSeasonByCode(context.Background(), "awt-2025")
	if err != nil || s.ID != "b" {
		t.Fatalf("GetSeasonByCode = %+v, %v; want the active season b", s, err)
	}
}

func TestTrickAcronymUniqueAmongActive(t *testing.T) {
	db := openMigrated(t)
	ins := `INSERT INTO tricks (id, year, name, acronym, solo, synchro, technical_coefficient) VALUES (?, 2025, ?, 'M', 1, 1, 1.65)`
	if err := exec(t, db, ins, "a", "Misty"); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, db, ins, "b", "Other"); !isConstraint(err) {
		t.Fatalf("duplicate active acronym accepted: %v", err)
	}
}

func TestTrickRecreationAfterUniqueTricksCleanup(t *testing.T) {
	db := openMigrated(t)
	ctx := context.Background()
	trick := `INSERT INTO tricks (id, year, name, acronym, solo, synchro, technical_coefficient) VALUES (?, 2025, 'Misty', 'M', 1, 1, 1.65)`
	variant := `INSERT INTO unique_tricks (trick_id, year, acronym, name, technical_coefficient, bonus, solo, solo_awt, synchro) VALUES (?, 2025, '/LM', 'twisted left Misty', 1.65, 2.5, 1, 1, 1)`
	if err := exec(t, db, trick, "a"); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, db, variant, "a"); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, db, trick, "b"); !isConstraint(err) {
		t.Fatalf("duplicate trick accepted: %v", err)
	}
	if err := db.Tx(ctx, func(q *gen.Queries) error {
		return q.DeleteUniqueTricksByTrick(ctx, "a")
	}); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, db, `UPDATE tricks SET deleted_at = '2025-01-01T00:00:00Z' WHERE id = 'a'`); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, db, trick, "b"); err != nil {
		t.Fatalf("trick recreation rejected: %v", err)
	}
	if err := exec(t, db, variant, "b"); err != nil {
		t.Fatalf("variant recreation rejected: %v", err)
	}
}
