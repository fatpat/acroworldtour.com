package main

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"awt/internal/auth"
	"awt/internal/config"
	"awt/internal/store"
)

func TestUserCmdValidatesArgsBeforeOpeningDB(t *testing.T) {
	t.Setenv("AWT_DB_PATH", "/nonexistent-dir/awt.db") // would fail if opened
	cases := map[string][]string{
		"no subcommand":       {},
		"unknown subcommand":  {"frobnicate", "--email", "a@b.c"},
		"missing email":       {"add", "--name", "N", "--role", "admin"},
		"missing role":        {"add", "--email", "a@b.c", "--name", "N"},
		"missing competition": {"assign", "--email", "a@b.c"},
		"reset without email": {"reset-mfa"},
	}
	for name, args := range cases {
		err := userCmd(args, nil)
		if err == nil {
			t.Errorf("%s: expected an error", name)
		} else if strings.Contains(err.Error(), "nonexistent") || strings.Contains(err.Error(), "unable to open") {
			t.Errorf("%s: DB opened before validation: %v", name, err)
		}
	}
}

func TestUserResetMFA(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "awt.db")
	t.Setenv("AWT_DB_PATH", dbPath)
	t.Setenv("AWT_ENV", "dev")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	a := auth.New(db, config.Config{Env: "dev"})
	u, err := a.CreateUser(ctx, "lost@example.org", "Lost", auth.RoleAdmin, "a long enough pass")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`INSERT INTO user_totp (user_id, secret_enc, created_at, last_step) VALUES (?, 'x', '2026-01-01T00:00:00Z', 0)`, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`INSERT INTO user_recovery_codes (user_id, code_hash) VALUES (?, 'h')`, u.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	if err := userCmd([]string{"reset-mfa", "--email", "Lost@Example.org"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := userCmd([]string{"reset-mfa", "--email", "nobody@example.org"}, nil); err == nil || !strings.Contains(err.Error(), "nobody@example.org") {
		t.Fatalf("unknown email = %v", err)
	}

	db, err = store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st, err := auth.New(db, config.Config{Env: "dev"}).MFAStatus(ctx, u.ID)
	if err != nil || st.TOTP || st.RecoveryLeft != 0 {
		t.Fatalf("status after reset = %+v, %v", st, err)
	}
	if log := buf.String(); !strings.Contains(log, "event=mfa.reset_by_admin") || !strings.Contains(log, "actor=cli") || !strings.Contains(log, "user_id="+u.ID) {
		t.Fatalf("log = %s", log)
	}
}
