package auth

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func roleOf(t *testing.T, a *Auth, id string) string {
	t.Helper()
	u, err := a.GetUser(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return u.Role
}

func sessionAlive(a *Auth, s Session) bool {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(cookieFor(s))
	_, err := a.UserFromRequest(r)
	return err == nil
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func TestSetRolePromotesAndRevokesSessions(t *testing.T) {
	a := newAuth(t, "dev")
	ctx := context.Background()
	admin := mustUser(t, a, "admin@example.org", RoleAdmin)
	s := mustUser(t, a, "scorer@example.org", RoleScorer)
	addComp(t, a, "c1", "CUP")
	if err := a.AssignCompetition(ctx, s.Email, "c1"); err != nil {
		t.Fatal(err)
	}
	sess := mustLogin(t, a, s.Email)
	adminSess := mustLogin(t, a, admin.Email)
	log := captureLog(t)

	changed, err := a.SetRole(ctx, admin.ID, s.ID, RoleAdmin)
	if err != nil || !changed {
		t.Fatalf("SetRole = %v, %v", changed, err)
	}
	if got := roleOf(t, a, s.ID); got != RoleAdmin {
		t.Fatalf("role = %s", got)
	}
	if sessionAlive(a, sess) {
		t.Fatal("the promoted user's session survived")
	}
	if !sessionAlive(a, adminSess) {
		t.Fatal("the actor's session was closed")
	}
	// Assignments are kept: they apply again after a demotion.
	if comps, err := a.UserCompetitions(ctx, s.ID); err != nil || len(comps) != 1 || comps[0] != "c1" {
		t.Fatalf("assignments = %v, %v", comps, err)
	}
	for _, want := range []string{"event=user.role_changed", "actor=" + admin.ID, "user_id=" + s.ID, "old_role=scorer", "new_role=admin"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log missing %q: %s", want, log)
		}
	}
	if strings.Contains(log.String(), s.PasswordHash) {
		t.Fatal("log holds the password hash")
	}
}

func TestSetRoleDemotes(t *testing.T) {
	a := newAuth(t, "dev")
	ctx := context.Background()
	mustUser(t, a, "admin@example.org", RoleAdmin)
	other := mustUser(t, a, "other@example.org", RoleAdmin)
	sess := mustLogin(t, a, other.Email)

	changed, err := a.SetRole(ctx, "cli", other.ID, RoleScorer)
	if err != nil || !changed {
		t.Fatalf("SetRole = %v, %v", changed, err)
	}
	if got := roleOf(t, a, other.ID); got != RoleScorer {
		t.Fatalf("role = %s", got)
	}
	if sessionAlive(a, sess) {
		t.Fatal("the demoted user's session survived")
	}
}

func TestSetRoleRefusesLastEnabledAdmin(t *testing.T) {
	a := newAuth(t, "dev")
	ctx := context.Background()
	admin := mustUser(t, a, "admin@example.org", RoleAdmin)
	off := mustUser(t, a, "off@example.org", RoleAdmin)
	if err := a.SetDisabled(ctx, off.ID, true); err != nil {
		t.Fatal(err)
	}
	sess := mustLogin(t, a, admin.Email)

	// A disabled admin does not count: admin is the last enabled one.
	if _, err := a.SetRole(ctx, "cli", admin.ID, RoleScorer); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("err = %v, want ErrLastAdmin", err)
	}
	if roleOf(t, a, admin.ID) != RoleAdmin || !sessionAlive(a, sess) {
		t.Fatal("a refused demotion changed something")
	}
	// Demoting the disabled admin leaves the enabled one: allowed.
	if _, err := a.SetRole(ctx, "cli", off.ID, RoleScorer); err != nil {
		t.Fatal(err)
	}
}

func TestSetRoleRefusesOwnRole(t *testing.T) {
	a := newAuth(t, "dev")
	ctx := context.Background()
	admin := mustUser(t, a, "admin@example.org", RoleAdmin)
	mustUser(t, a, "other@example.org", RoleAdmin)
	if _, err := a.SetRole(ctx, admin.ID, admin.ID, RoleScorer); !errors.Is(err, ErrOwnRole) {
		t.Fatalf("err = %v, want ErrOwnRole", err)
	}
	if roleOf(t, a, admin.ID) != RoleAdmin {
		t.Fatal("own role changed")
	}
}

func TestSetRoleInvalidRoleAndUnknownUser(t *testing.T) {
	a := newAuth(t, "dev")
	ctx := context.Background()
	s := mustUser(t, a, "scorer@example.org", RoleScorer)
	var ie *InputError
	if _, err := a.SetRole(ctx, "cli", s.ID, "boss"); !errors.As(err, &ie) || ie.Field != "role" {
		t.Fatalf("err = %v, want a role InputError", err)
	}
	if _, err := a.SetRole(ctx, "cli", "nope", RoleAdmin); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want sql.ErrNoRows", err)
	}
}

func TestSetRoleSameRoleIsNoop(t *testing.T) {
	a := newAuth(t, "dev")
	ctx := context.Background()
	admin := mustUser(t, a, "admin@example.org", RoleAdmin)
	sess := mustLogin(t, a, admin.Email)
	log := captureLog(t)
	// Even the last admin may be "set" to admin.
	changed, err := a.SetRole(ctx, "cli", admin.ID, RoleAdmin)
	if err != nil || changed {
		t.Fatalf("SetRole = %v, %v; want a no-op", changed, err)
	}
	if !sessionAlive(a, sess) {
		t.Fatal("a no-op closed the sessions")
	}
	if strings.Contains(log.String(), "user.role_changed") {
		t.Fatalf("a no-op logged a change: %s", log)
	}
}
