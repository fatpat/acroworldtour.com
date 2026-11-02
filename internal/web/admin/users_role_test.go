package admin_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"awt/internal/auth"
)

func (e *env) roleOf(id string) string {
	e.t.Helper()
	u, err := e.deps.Auth.GetUser(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return u.Role
}

func TestAdminChangesAnotherUsersRole(t *testing.T) {
	e := newEnv(t)
	id := e.scorer.User.ID
	path := "/manager/users/" + id + "/role"
	e.wantStatus(e.get("/manager/users/"+id), http.StatusOK, `action="`+path+`"`, "Change role", "assignments are kept")

	e.wantRedirect(e.post(path, url.Values{"role": {auth.RoleAdmin}}), "/manager/users/"+id+"?ok=role")
	if got := e.roleOf(id); got != auth.RoleAdmin {
		t.Fatalf("role = %s", got)
	}
	if e.alive(&e.scorer) {
		t.Fatal("the promoted user's session survived")
	}
	e.wantStatus(e.get("/manager/users/"+id+"?ok=role"), http.StatusOK, "Role changed")

	// Back to scorer; the same role again is a no-op.
	e.wantRedirect(e.post(path, url.Values{"role": {auth.RoleScorer}}), "/manager/users/"+id+"?ok=role")
	e.wantRedirect(e.post(path, url.Values{"role": {auth.RoleScorer}}), "/manager/users/"+id+"?ok=role_same")
	e.wantStatus(e.get("/manager/users/"+id+"?ok=role_same"), http.StatusOK, "Role unchanged")
	if got := e.roleOf(id); got != auth.RoleScorer {
		t.Fatalf("role = %s", got)
	}
	e.wantStatus(e.post(path, url.Values{"role": {"boss"}}), http.StatusUnprocessableEntity, "admin or scorer")
	e.wantStatus(e.post("/manager/users/nope/role", url.Values{"role": {auth.RoleAdmin}}), http.StatusNotFound)
}

func TestScorerCannotChangeRoles(t *testing.T) {
	e := newEnv(t)
	path := "/manager/users/" + e.scorer.User.ID + "/role"
	if rec := e.postAs(&e.scorer, path, url.Values{"role": {auth.RoleAdmin}}, true); rec.Code != http.StatusForbidden {
		t.Fatalf("scorer = %d, want 403", rec.Code)
	}
	if rec := e.postAs(&e.admin, path, url.Values{"role": {auth.RoleAdmin}}, false); rec.Code != http.StatusForbidden {
		t.Fatalf("no CSRF = %d, want 403", rec.Code)
	}
	if got := e.roleOf(e.scorer.User.ID); got != auth.RoleScorer {
		t.Fatalf("role = %s", got)
	}
}

func TestAdminCannotChangeOwnRole(t *testing.T) {
	e := newEnv(t)
	if _, err := e.deps.Auth.CreateUser(context.Background(), "second@example.org", "Second", auth.RoleAdmin, accountPassword); err != nil {
		t.Fatal(err)
	}
	id := e.admin.User.ID
	e.wantStatus(e.get("/manager/users/"+id), http.StatusOK, "You cannot change your own role")
	e.wantStatus(e.post("/manager/users/"+id+"/role", url.Values{"role": {auth.RoleScorer}}), http.StatusConflict, "You cannot change your own role")
	if got := e.roleOf(id); got != auth.RoleAdmin || !e.alive(&e.admin) {
		t.Fatalf("self change went through: role %s", got)
	}
}

// Through the UI the actor is always an enabled admin other than the target,
// so the last-admin guard only fires when another admin demotes the actor at
// the same instant. A test-only trigger plays that concurrent demotion inside
// the role change's transaction.
func TestLastAdminCannotBeDemoted(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	other, err := e.deps.Auth.CreateUser(ctx, "second@example.org", "Second", auth.RoleAdmin, accountPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.deps.DB.Write.Exec(`CREATE TRIGGER race_demote AFTER UPDATE OF role ON users WHEN NEW.id = '` + other.ID + `'
		BEGIN UPDATE users SET role = 'scorer' WHERE id = '` + e.admin.User.ID + `'; END`); err != nil {
		t.Fatal(err)
	}
	e.wantStatus(e.post("/manager/users/"+other.ID+"/role", url.Values{"role": {auth.RoleScorer}}), http.StatusConflict, "last enabled admin")
	if e.roleOf(other.ID) != auth.RoleAdmin || e.roleOf(e.admin.User.ID) != auth.RoleAdmin {
		t.Fatal("a refused demotion was not rolled back")
	}
}
