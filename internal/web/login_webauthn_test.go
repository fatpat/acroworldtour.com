package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"awt/internal/auth"
	"awt/internal/auth/webauthntest"
	"awt/internal/web"
)

const keyOrigin = "http://localhost:0" // dev origin of AWT_ADDR=:0

// newKeyEnv creates mfa@example.org with one registered software security key.
func newKeyEnv(t *testing.T) (*mfaEnv, *webauthntest.Key) {
	t.Helper()
	deps := newDeps(t)
	deps.Cfg.WebAuthnRPID = "localhost"
	deps.Auth = auth.New(deps.DB, deps.Cfg)
	ctx := context.Background()
	u, err := deps.Auth.CreateUser(ctx, "mfa@example.org", "Mfa", auth.RoleAdmin, "s3cret-passw0rd")
	if err != nil {
		t.Fatal(err)
	}
	key, err := webauthntest.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	opts, token, err := deps.Auth.BeginWebAuthnRegistration(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	body, err := key.Create(opts, keyOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.Auth.FinishWebAuthnRegistration(ctx, u.ID, token, "Key", "s3cret-passw0rd", "", body); err != nil {
		t.Fatal(err)
	}
	return &mfaEnv{t: t, h: web.Routes(deps), deps: deps}, key
}

func (e *mfaEnv) postJSON(path, body, origin string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

type keyAnswer struct {
	Redirect string `json:"redirect"`
	Error    string `json:"error"`
}

func decodeAnswer(t *testing.T, rec *httptest.ResponseRecorder) keyAnswer {
	t.Helper()
	var a keyAnswer
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type %q, body %s", ct, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("answer %q: %v", rec.Body.String(), err)
	}
	return a
}

func TestMFALoginWithSecurityKey(t *testing.T) {
	e, key := newKeyEnv(t)
	pending := e.password("/manager/pilots")

	rec := get(e.h, "/manager/login/mfa", pending)
	body := rec.Body.String()
	for _, want := range []string{`data-begin-url="/manager/login/mfa/webauthn/begin"`, `data-finish-url="/manager/login/mfa/webauthn/finish"`, `data-webauthn-error`, `/static/js/webauthn.js`} {
		if !strings.Contains(body, want) {
			t.Errorf("second step page missing %q", want)
		}
	}
	if strings.Contains(body, "data-webauthn-login disabled") {
		t.Error("security key button still disabled")
	}

	if rec = e.postJSON("/manager/login/mfa/webauthn/begin", "", "https://evil.example", pending); rec.Code != http.StatusForbidden {
		t.Fatalf("begin from a foreign Origin: %d", rec.Code)
	}
	rec = e.postJSON("/manager/login/mfa/webauthn/begin", "", keyOrigin, pending)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"allowCredentials"`) {
		t.Fatalf("begin: %d %q\n%s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}
	key.SignCount = 1
	assertion, err := key.Get(rec.Body.Bytes(), keyOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if rec = e.postJSON("/manager/login/mfa/webauthn/finish", string(assertion), "https://evil.example", pending); rec.Code != http.StatusForbidden {
		t.Fatalf("finish from a foreign Origin: %d", rec.Code)
	}
	rec = e.postJSON("/manager/login/mfa/webauthn/finish", string(assertion), keyOrigin, pending)
	if rec.Code != http.StatusOK || decodeAnswer(t, rec).Redirect != "/manager/pilots" {
		t.Fatalf("finish: %d\n%s", rec.Code, rec.Body.String())
	}
	session := cookieNamed(rec, auth.CookieName)
	if session == nil {
		t.Fatal("no session cookie")
	}
	if c := cookieNamed(rec, auth.PendingCookieName); c == nil || c.MaxAge >= 0 {
		t.Fatalf("pending cookie not cleared: %v", c)
	}
	if rec = get(e.h, "/manager", session); rec.Code != http.StatusOK {
		t.Fatalf("/manager with session: %d", rec.Code)
	}

	// The pending login is spent.
	rec = e.postJSON("/manager/login/mfa/webauthn/begin", "", keyOrigin, pending)
	if rec.Code != http.StatusUnauthorized || decodeAnswer(t, rec).Error != "Your login expired, please sign in again" {
		t.Fatalf("begin on a spent pending: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMFASecurityKeyErrors(t *testing.T) {
	e, key := newKeyEnv(t)
	pending := e.password("/manager")
	rec := e.postJSON("/manager/login/mfa/webauthn/begin", "", keyOrigin, pending)
	if rec.Code != http.StatusOK {
		t.Fatalf("begin: %d", rec.Code)
	}
	key.SignCount = 1
	wrong, _ := key.Get(rec.Body.Bytes(), "http://127.0.0.2:0")
	rec = e.postJSON("/manager/login/mfa/webauthn/finish", string(wrong), keyOrigin, pending)
	if rec.Code != http.StatusUnauthorized || decodeAnswer(t, rec).Error != "This security key was not accepted" || cookieNamed(rec, auth.CookieName) != nil {
		t.Fatalf("rejected key: %d %s", rec.Code, rec.Body.String())
	}
	if rec = e.postJSON("/manager/login/mfa/webauthn/finish", strings.Repeat("x", 64<<10), keyOrigin, pending); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized: %d", rec.Code)
	}
	for i := 2; i <= 4; i++ {
		if rec = e.postJSON("/manager/login/mfa/webauthn/finish", "{}", keyOrigin, pending); rec.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d: %d", i, rec.Code)
		}
	}
	rec = e.postJSON("/manager/login/mfa/webauthn/finish", "{}", keyOrigin, pending)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || decodeAnswer(t, rec).Error != "Too many attempts, try again in 15 minutes" {
		t.Fatalf("5th failure: %d %s", rec.Code, rec.Body.String())
	}
	if rec = e.postJSON("/manager/login/mfa/webauthn/begin", "", keyOrigin, pending); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("begin while locked: %d", rec.Code)
	}
	if rec = e.postJSON("/manager/login/mfa/webauthn/begin", "", keyOrigin); rec.Code != http.StatusUnauthorized {
		t.Fatalf("begin without pending: %d", rec.Code)
	}
}
