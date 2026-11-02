package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestUpdateProfileByAdmin(t *testing.T) {
	a := newAuth(t, "dev")
	ctx := context.Background()
	admin := mustUser(t, a, "admin@example.org", RoleAdmin)
	s := mustUser(t, a, "scorer@example.org", RoleScorer)
	mustUser(t, a, "taken@example.org", RoleScorer)
	sess := mustLogin(t, a, s.Email)
	log := captureLog(t)

	changed, err := a.UpdateProfile(ctx, admin.ID, s.ID, "  New@Example.org ", " New Name ")
	if err != nil || strings.Join(changed, ",") != "email,name" {
		t.Fatalf("UpdateProfile = %v, %v", changed, err)
	}
	u, _ := a.GetUser(ctx, s.ID)
	if u.Email != "new@example.org" || u.Name != "New Name" {
		t.Fatalf("stored %q %q", u.Email, u.Name)
	}
	// The email only identifies the account: sessions stay, login uses the new one.
	if !sessionAlive(a, sess) {
		t.Fatal("the user's session was closed")
	}
	if _, err := a.Login(ctx, s.Email, "s3cret-passw0rd"); err == nil {
		t.Fatal("the old email still logs in")
	}
	mustLogin(t, a, "new@example.org")
	for _, want := range []string{"event=user.profile_changed", "actor=" + admin.ID, "user_id=" + s.ID, "fields=email,name"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log missing %q: %s", want, log)
		}
	}

	if changed, err := a.UpdateProfile(ctx, admin.ID, s.ID, "new@example.org", "New Name"); err != nil || len(changed) != 0 {
		t.Fatalf("same values = %v, %v", changed, err)
	}
	var exists *ExistsError
	if _, err := a.UpdateProfile(ctx, admin.ID, s.ID, "TAKEN@example.org", "x"); !errors.As(err, &exists) {
		t.Fatalf("taken email = %v", err)
	}
	var in *InputError
	for _, c := range []struct{ email, name, field string }{
		{"", "x", "email"}, {"nope", "x", "email"}, {"ok@example.org", "  ", "name"},
	} {
		if _, err := a.UpdateProfile(ctx, admin.ID, s.ID, c.email, c.name); !errors.As(err, &in) || in.Field != c.field {
			t.Errorf("%q %q = %v, want an error on %s", c.email, c.name, err, c.field)
		}
	}
	if _, err := a.UpdateProfile(ctx, admin.ID, admin.ID, "boss@example.org", "Boss"); !errors.Is(err, ErrOwnProfile) {
		t.Fatalf("own profile = %v", err)
	}
	if u, _ := a.GetUser(ctx, s.ID); u.Email != "new@example.org" || u.Name != "New Name" {
		t.Fatalf("a refused change was stored: %q %q", u.Email, u.Name)
	}
}

func TestUpdateOwnProfile(t *testing.T) {
	a := newAuth(t, "dev")
	ctx := context.Background()
	u := mustUser(t, a, "me@example.org", RoleScorer)
	mustUser(t, a, "taken@example.org", RoleScorer)
	here := mustLogin(t, a, u.Email)
	elsewhere := mustLogin(t, a, u.Email)

	// A name alone needs no password and keeps every session.
	if changed, err := a.UpdateOwnProfile(ctx, u.ID, "me@example.org", "Renamed", "", here.Token); err != nil || strings.Join(changed, ",") != "name" {
		t.Fatalf("rename = %v, %v", changed, err)
	}
	if !sessionAlive(a, elsewhere) {
		t.Fatal("a rename closed a session")
	}
	// A new email needs the current password.
	if _, err := a.UpdateOwnProfile(ctx, u.ID, "me2@example.org", "Renamed", "wrong", here.Token); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password = %v", err)
	}
	var exists *ExistsError
	if _, err := a.UpdateOwnProfile(ctx, u.ID, "taken@example.org", "Renamed", "s3cret-passw0rd", here.Token); !errors.As(err, &exists) {
		t.Fatalf("taken email = %v", err)
	}
	if !sessionAlive(a, elsewhere) {
		t.Fatal("a refused change closed a session")
	}
	changed, err := a.UpdateOwnProfile(ctx, u.ID, "me2@example.org", "Me", "s3cret-passw0rd", here.Token)
	if err != nil || strings.Join(changed, ",") != "email,name" {
		t.Fatalf("new email = %v, %v", changed, err)
	}
	if !sessionAlive(a, here) || sessionAlive(a, elsewhere) {
		t.Fatal("want the current session kept and the other closed")
	}
	if got, _ := a.GetUser(ctx, u.ID); got.Email != "me2@example.org" || got.Name != "Me" {
		t.Fatalf("stored %q %q", got.Email, got.Name)
	}
}
