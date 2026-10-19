package auth

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"awt/internal/store/gen"
)

func addTestKey(t *testing.T, a *Auth, u User, id string) {
	t.Helper()
	err := a.db.Tx(context.Background(), func(q *gen.Queries) error {
		return q.InsertUserWebAuthnCredential(context.Background(), gen.InsertUserWebAuthnCredentialParams{
			ID: id, UserID: u.ID, Name: "key " + id, Credential: "{}", CreatedAt: a.now().UTC().Format(time.RFC3339)})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestResetMFARemovesEverything(t *testing.T) {
	a, c := newMFAAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "lost@example.org", RoleAdmin)
	other := mustUser(t, a, "other@example.org", RoleAdmin)
	secret := enrolTOTP(t, a, u)
	seedRecovery(t, a, u)
	addTestKey(t, a, u, "k1")
	mustLogin(t, a, other.Email)
	enrolTOTP(t, a, other)
	seedRecovery(t, a, other)

	// A pending login and a lockout, then a live session.
	req := startPending(t, a, u.Email, "192.0.2.1")
	bad := wrongCode(t, secret, c.now())
	for i := 0; i < 5; i++ {
		_, _ = a.CompleteWithTOTP(ctx, req, bad)
	}
	if countRows(t, a, `SELECT count(*) FROM login_challenges WHERE user_id = ?`, u.ID) == 0 {
		t.Fatal("setup: no pending login")
	}
	if err := a.db.Tx(ctx, func(q *gen.Queries) error {
		return q.InsertSession(ctx, gen.InsertSessionParams{TokenHash: "s1", UserID: u.ID, CsrfToken: "c", ExpiresAt: c.now().Add(time.Hour).Format(time.RFC3339), CreatedAt: c.now().Format(time.RFC3339)})
	}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	if err := a.ResetMFA(ctx, u.ID, "cli"); err != nil {
		t.Fatal(err)
	}
	for table, q := range map[string]string{
		"totp": `SELECT count(*) FROM user_totp WHERE user_id = ?`, "keys": `SELECT count(*) FROM user_webauthn_credentials WHERE user_id = ?`,
		"codes": `SELECT count(*) FROM user_recovery_codes WHERE user_id = ?`, "challenges": `SELECT count(*) FROM login_challenges WHERE user_id = ?`,
		"sessions": `SELECT count(*) FROM sessions WHERE user_id = ?`,
	} {
		if n := countRows(t, a, q, u.ID); n != 0 {
			t.Errorf("%s left: %d", table, n)
		}
	}
	var failed int
	var locked sql.NullString
	if err := a.db.Read.QueryRow(`SELECT mfa_failed_count, mfa_locked_until FROM users WHERE id = ?`, u.ID).Scan(&failed, &locked); err != nil || failed != 0 || locked.Valid {
		t.Fatalf("lockout not cleared: %d %v %v", failed, locked, err)
	}
	// The account logs in with its password only.
	if _, err := a.Login(ctx, u.Email, "s3cret-passw0rd"); err != nil {
		t.Fatalf("login after reset: %v", err)
	}
	// Another account is untouched.
	if st, err := a.MFAStatus(ctx, other.ID); err != nil || !st.TOTP || st.RecoveryLeft != 10 || countRows(t, a, `SELECT count(*) FROM sessions WHERE user_id = ?`, other.ID) != 1 {
		t.Fatalf("other account changed: %+v %v", st, err)
	}
	log := buf.String()
	for _, want := range []string{"event=mfa.reset_by_admin", "user_id=" + u.ID, "actor=cli"} {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %q: %s", want, log)
		}
	}
	if strings.Contains(log, secret) {
		t.Fatal("secret logged")
	}
	if err := a.ResetMFA(ctx, "nobody", "cli"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown user = %v", err)
	}
}

func TestMFABadgesBatched(t *testing.T) {
	a, _ := newMFAAuth(t)
	ctx := context.Background()
	none := mustUser(t, a, "none@example.org", RoleScorer)
	app := mustUser(t, a, "app@example.org", RoleScorer)
	keys := mustUser(t, a, "keys@example.org", RoleScorer)
	both := mustUser(t, a, "both@example.org", RoleAdmin)
	enrolTOTP(t, a, app)
	enrolTOTP(t, a, both)
	addTestKey(t, a, keys, "k1")
	addTestKey(t, a, keys, "k2")
	addTestKey(t, a, both, "k3")

	got, err := a.MFABadges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]MFABadge{none.ID: {}, app.ID: {TOTP: true}, keys.ID: {Keys: 2}, both.ID: {TOTP: true, Keys: 1}}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("badge %s = %+v, want %+v", id, got[id], w)
		}
	}
	if l := (MFABadge{TOTP: true, Keys: 2}).Labels(); len(l) != 2 || l[0] != "App" || l[1] != "Key ×2" {
		t.Fatalf("labels = %v", l)
	}
	if l := (MFABadge{Keys: 1}).Labels(); len(l) != 1 || l[0] != "Key" {
		t.Fatalf("labels = %v", l)
	}
	if l := (MFABadge{}).Labels(); len(l) != 0 {
		t.Fatalf("labels = %v", l)
	}
}
