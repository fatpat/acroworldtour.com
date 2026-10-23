package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"testing"

	"awt/internal/store"
)

// authTables are emptied on training (decision 0018).
var authTables = []string{"sessions", "login_challenges", "user_totp", "user_webauthn_credentials", "user_recovery_codes"}

// fingerprint hashes every row of every table except the auth tables and
// the users' MFA failure columns, so a cleanup that touches anything else
// changes it.
func fingerprint(t *testing.T, db *store.DB) map[string]string {
	t.Helper()
	ctx := context.Background()
	rows, err := db.Read.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(authTables, n) {
			tables = append(tables, n)
		}
	}
	rows.Close()
	out := map[string]string{}
	for _, tbl := range tables {
		q := `SELECT * FROM "` + tbl + `"`
		if tbl == "users" {
			q = `SELECT id, email, name, password_hash, role, disabled FROM users`
		}
		r, err := db.Read.QueryContext(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := r.Columns()
		var lines []string
		for r.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := r.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			lines = append(lines, fmt.Sprintf("%q", vals))
		}
		r.Close()
		slices.Sort(lines)
		h := sha256.New()
		for _, l := range lines {
			h.Write([]byte(l + "\n"))
		}
		out[tbl] = fmt.Sprintf("%d %s", len(lines), hex.EncodeToString(h.Sum(nil)))
	}
	return out
}

func count(t *testing.T, db *store.DB, q string) int {
	t.Helper()
	var n int
	if err := db.Read.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func TestResetTrainingAuth(t *testing.T) {
	db := openMigrated(t)
	ctx := context.Background()
	for _, q := range []string{
		`INSERT INTO users (id, email, name, password_hash, role) VALUES ('u1', 'a@x.org', 'A', 'h1', 'admin'), ('u2', 's@x.org', 'S', 'h2', 'scorer')`,
		`UPDATE users SET mfa_failed_count = 4, mfa_locked_until = '2026-10-23T10:00:00Z' WHERE id = 'u2'`,
		`INSERT INTO files (id, filename, content_type, size, created_at) VALUES ('f1', 'a.png', 'image/png', 3, '2026-01-01T00:00:00Z')`,
		`INSERT INTO competitions (id, code, name, type, start_date, end_date, config, logo_file) VALUES ('c1', 'awt-1', 'AWT 1', 'solo', '2026-06-01', '2026-06-03', '{}', 'f1')`,
		`INSERT INTO user_competitions (user_id, competition_id) VALUES ('u2', 'c1')`,
		`INSERT INTO news (id, slug, title, cover_file) VALUES ('n1', 'hello', 'Hello', 'f1')`,
		`INSERT INTO sessions (token_hash, user_id, csrf_token, expires_at, created_at) VALUES ('s1', 'u1', 'c', '2099-01-01T00:00:00Z', '2026-01-01T00:00:00Z'), ('s2', 'u2', 'c', '2099-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		`INSERT INTO login_challenges (token_hash, user_id, kind, expires_at) VALUES ('l1', 'u1', 'pending_login', '2099-01-01T00:00:00Z')`,
		`INSERT INTO user_totp (user_id, secret_enc, created_at) VALUES ('u1', 'enc', '2026-01-01T00:00:00Z')`,
		`INSERT INTO user_webauthn_credentials (id, user_id, name, credential, created_at) VALUES ('k1', 'u1', 'Key', '{}', '2026-01-01T00:00:00Z'), ('k2', 'u2', 'Key', '{}', '2026-01-01T00:00:00Z')`,
		`INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ('u1', 'r1'), ('u1', 'r2'), ('u2', 'r3')`,
	} {
		if _, err := db.Write.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	before := fingerprint(t, db)

	got, err := store.ResetTrainingAuth(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	want := store.TrainingAuthReset{Sessions: 2, LoginChallenges: 1, TOTP: 1, SecurityKeys: 2, RecoveryCodes: 3, MFALocks: 1}
	if got != want {
		t.Errorf("reset = %+v, want %+v", got, want)
	}
	for _, tbl := range authTables {
		if n := count(t, db, `SELECT count(*) FROM "`+tbl+`"`); n != 0 {
			t.Errorf("%s: %d rows left", tbl, n)
		}
	}
	if n := count(t, db, `SELECT count(*) FROM users WHERE mfa_failed_count <> 0 OR mfa_locked_until IS NOT NULL`); n != 0 {
		t.Errorf("%d users keep an MFA failure count or lock", n)
	}
	after := fingerprint(t, db)
	for tbl, b := range before {
		if after[tbl] != b {
			t.Errorf("table %s changed: %s -> %s", tbl, b, after[tbl])
		}
	}
	if before["users"][:2] != "2 " || before["news"][:2] != "1 " {
		t.Fatalf("fixture not loaded: %v", before)
	}

	sum, err := store.CloneSummary(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Users != 2 || sum.Competitions != 1 || sum.News != 1 || sum.Files != 1 {
		t.Errorf("summary %+v", sum)
	}

	// Again on a clean database: nothing to remove, no error.
	if got, err := store.ResetTrainingAuth(ctx, db); err != nil || got != (store.TrainingAuthReset{}) {
		t.Errorf("second reset = %+v, %v", got, err)
	}
}
