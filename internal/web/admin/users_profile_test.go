package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"awt/internal/auth"
)

func (e *env) user(id string) auth.User {
	e.t.Helper()
	u, err := e.deps.Auth.GetUser(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

func TestAdminEditsAnotherUsersProfile(t *testing.T) {
	e := newEnv(t)
	id := e.scorer.User.ID
	path := "/manager/users/" + id + "/profile"
	e.wantStatus(e.get("/manager/users/"+id), http.StatusOK, `action="`+path+`"`, `value="scorer@example.org"`)

	e.wantRedirect(e.post(path, url.Values{"email": {"Judge@Example.org"}, "name": {"Judge Dredd"}}), "/manager/users/"+id+"?ok=profile")
	if u := e.user(id); u.Email != "judge@example.org" || u.Name != "Judge Dredd" {
		t.Fatalf("stored %q %q", u.Email, u.Name)
	}
	if !e.alive(&e.scorer) {
		t.Fatal("the user's session was closed")
	}
	e.wantStatus(e.get("/manager/users/"+id+"?ok=profile"), http.StatusOK, "Email and name saved")
	e.login("judge@example.org", accountPassword)

	e.wantRedirect(e.post(path, url.Values{"email": {"judge@example.org"}, "name": {"Judge Dredd"}}), "/manager/users/"+id+"?ok=profile_same")
	// A refused change shows the posted values back.
	e.wantStatus(e.post(path, url.Values{"email": {e.admin.User.Email}, "name": {"Other"}}), http.StatusConflict, "already exists", `value="Other"`)
	e.wantStatus(e.post(path, url.Values{"email": {"not-an-email"}, "name": {"Judge"}}), http.StatusUnprocessableEntity, "must look like", `value="not-an-email"`)
	e.wantStatus(e.post(path, url.Values{"email": {"judge@example.org"}, "name": {" "}}), http.StatusUnprocessableEntity, "name is required")
	e.wantStatus(e.post("/manager/users/nope/profile", url.Values{"email": {"x@example.org"}, "name": {"X"}}), http.StatusNotFound)
	if u := e.user(id); u.Email != "judge@example.org" || u.Name != "Judge Dredd" {
		t.Fatalf("a refused change was stored: %q %q", u.Email, u.Name)
	}
}

func TestAdminEditsOwnProfileOnAccountPage(t *testing.T) {
	e := newEnv(t)
	id := e.admin.User.ID
	e.wantStatus(e.get("/manager/users/"+id), http.StatusOK, "on your <a href=\"/manager/account\">account page</a>")
	e.wantStatus(e.post("/manager/users/"+id+"/profile", url.Values{"email": {"boss@example.org"}, "name": {"Boss"}}), http.StatusConflict, "Use your account page")
	if u := e.user(id); u.Email == "boss@example.org" {
		t.Fatal("own email changed without a password check")
	}
}

func TestScorerCannotEditProfiles(t *testing.T) {
	e := newEnv(t)
	path := "/manager/users/" + e.admin.User.ID + "/profile"
	// postAs adds the token to the form it gets: one form per request.
	form := func() url.Values { return url.Values{"email": {"pwned@example.org"}, "name": {"Pwned"}} }
	if rec := e.postAs(&e.scorer, path, form(), true); rec.Code != http.StatusForbidden {
		t.Fatalf("scorer = %d, want 403", rec.Code)
	}
	if rec := e.postAs(&e.admin, "/manager/users/"+e.scorer.User.ID+"/profile", form(), false); rec.Code != http.StatusForbidden {
		t.Fatalf("no CSRF = %d, want 403", rec.Code)
	}
	if rec := e.postAs(&e.scorer, "/manager/account/profile", form(), false); rec.Code != http.StatusForbidden {
		t.Fatalf("account without CSRF = %d, want 403", rec.Code)
	}
}

func TestAccountEditsOwnProfile(t *testing.T) {
	e := newEnv(t)
	id := e.scorer.User.ID
	elsewhere := e.login("scorer@example.org", accountPassword)
	post := func(email, name, password string) url.Values {
		return url.Values{"email": {email}, "name": {name}, "current_password": {password}}
	}
	e.wantStatus(e.do(httptest.NewRequest(http.MethodGet, "/manager/account", nil), &e.scorer), http.StatusOK, `action="/manager/account/profile"`, `value="scorer@example.org"`)

	// A name alone needs no password and closes nothing.
	e.wantRedirect(e.postAs(&e.scorer, "/manager/account/profile", post("scorer@example.org", "Renamed", ""), true), "/manager/account?ok=own_profile")
	if e.user(id).Name != "Renamed" || !e.alive(&elsewhere) {
		t.Fatal("rename not stored, or it closed a session")
	}
	e.wantRedirect(e.postAs(&e.scorer, "/manager/account/profile", post("scorer@example.org", "Renamed", ""), true), "/manager/account?ok=profile_same")

	// A new email needs the current password.
	e.wantStatus(e.postAs(&e.scorer, "/manager/account/profile", post("me@example.org", "Renamed", ""), true), http.StatusUnprocessableEntity, "Current password is wrong", `value="me@example.org"`)
	e.wantStatus(e.postAs(&e.scorer, "/manager/account/profile", post("admin@example.org", "Renamed", accountPassword), true), http.StatusConflict, "Another account uses this email")
	e.wantStatus(e.postAs(&e.scorer, "/manager/account/profile", post("nope", "Renamed", accountPassword), true), http.StatusUnprocessableEntity, "must look like")
	if e.user(id).Email != "scorer@example.org" || !e.alive(&elsewhere) {
		t.Fatal("a refused change was stored or closed a session")
	}
	e.wantRedirect(e.postAs(&e.scorer, "/manager/account/profile", post("me@example.org", "Me", accountPassword), true), "/manager/account?ok=own_email")
	if u := e.user(id); u.Email != "me@example.org" || u.Name != "Me" {
		t.Fatalf("stored %q %q", u.Email, u.Name)
	}
	if !e.alive(&e.scorer) || e.alive(&elsewhere) {
		t.Fatal("want the current session kept and the other closed")
	}
	e.login("me@example.org", accountPassword)
}
