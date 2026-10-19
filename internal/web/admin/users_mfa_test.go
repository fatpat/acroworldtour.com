package admin_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"awt/internal/auth"
)

// enrolApp sets up an authenticator app for s through the account page.
func (e *env) enrolApp(s *auth.Session) {
	e.t.Helper()
	pending, secret := e.startTOTP(s)
	e.wantStatus(e.postAs(s, "/manager/account/totp/confirm", url.Values{"pending": {pending}, "code": {codeAt(e.t, secret, time.Now())}, "current_password": {accountPassword}}, true), http.StatusOK)
}

func (e *env) addKeyRow(userID, id string) {
	e.t.Helper()
	if _, err := e.deps.DB.Write.Exec(`INSERT INTO user_webauthn_credentials (id, user_id, name, credential, created_at) VALUES (?, ?, 'k', '{}', '2026-01-01T00:00:00Z')`, id, userID); err != nil {
		e.t.Fatal(err)
	}
}

func TestUsersListShowsMFABadges(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	keys, err := e.deps.Auth.CreateUser(ctx, "keys@example.org", "N", auth.RoleScorer, accountPassword)
	if err != nil {
		t.Fatal(err)
	}
	e.enrolApp(&e.scorer)
	e.addKeyRow(keys.ID, "k1")
	e.addKeyRow(keys.ID, "k2")

	body := e.get("/manager/users").Body.String()
	row := func(email string) string {
		i := strings.Index(body, email)
		if i < 0 {
			t.Fatalf("no row for %s", email)
		}
		return body[i : i+strings.Index(body[i:], "</tr>")]
	}
	if r := row("scorer@example.org"); !strings.Contains(r, ">App<") || strings.Contains(r, "Key") {
		t.Errorf("app row: %s", r)
	}
	if r := row("keys@example.org"); !strings.Contains(r, ">Key ×2<") || strings.Contains(r, ">App<") {
		t.Errorf("keys row: %s", r)
	}
	if r := row("admin@example.org"); strings.Contains(r, "badge") {
		t.Errorf("row without a factor has a badge: %s", r)
	}
}

func TestAdminResetsMFAWithConfirmation(t *testing.T) {
	e := newEnv(t)
	scorerID := e.scorer.User.ID
	e.enrolApp(&e.scorer)
	e.addKeyRow(scorerID, "k1")
	path := "/manager/users/" + scorerID + "/reset-mfa"
	e.wantStatus(e.get("/manager/users/"+scorerID), http.StatusOK, `action="`+path+`"`, "Remove second factor")

	// First post asks; nothing changes.
	e.wantStatus(e.post(path, nil), http.StatusOK, "Confirm reset", `name="confirm" value="1"`)
	if st := e.status(scorerID); !st.TOTP || len(st.Keys) != 1 || !e.alive(&e.scorer) {
		t.Fatalf("asking changed something: %+v", st)
	}
	// Second post, with confirm=1, resets.
	e.wantRedirect(e.post(path, url.Values{"confirm": {"1"}}), "/manager/users/"+scorerID+"?ok=mfa_reset")
	if st := e.status(scorerID); st.TOTP || len(st.Keys) != 0 || st.RecoveryLeft != 0 {
		t.Fatalf("factors left: %+v", st)
	}
	if e.alive(&e.scorer) {
		t.Fatal("session survived the reset")
	}
	e.wantStatus(e.get("/manager/users/"+scorerID+"?ok=mfa_reset"), http.StatusOK, "Second factor removed")
	e.login("scorer@example.org", accountPassword) // password only again
}

func TestAdminCannotResetOwnMFA(t *testing.T) {
	e := newEnv(t)
	e.enrolApp(&e.admin)
	path := "/manager/users/" + e.admin.User.ID + "/reset-mfa"
	for _, form := range []url.Values{nil, {"confirm": {"1"}}} {
		e.wantStatus(e.post(path, form), http.StatusConflict, "Use your account page to manage your own MFA")
	}
	if st := e.status(e.admin.User.ID); !st.TOTP || !e.alive(&e.admin) {
		t.Fatalf("self reset changed something: %+v", st)
	}
	e.wantStatus(e.get("/manager/users/"+e.admin.User.ID), http.StatusOK, "own account page")
}

func TestResetMFAUnknownUserAndForbiddenScorer(t *testing.T) {
	e := newEnv(t)
	e.wantStatus(e.post("/manager/users/nope/reset-mfa", url.Values{"confirm": {"1"}}), http.StatusNotFound)
	path := "/manager/users/" + e.admin.User.ID + "/reset-mfa"
	if rec := e.postAs(&e.scorer, path, url.Values{"confirm": {"1"}}, true); rec.Code != http.StatusForbidden {
		t.Fatalf("scorer = %d, want 403", rec.Code)
	}
	if rec := e.postAs(&e.admin, "/manager/users/"+e.scorer.User.ID+"/reset-mfa", url.Values{"confirm": {"1"}}, false); rec.Code != http.StatusForbidden {
		t.Fatalf("no CSRF = %d, want 403", rec.Code)
	}
}
