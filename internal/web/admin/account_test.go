package admin_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"awt/internal/auth"
	"awt/internal/auth/webauthntest"
)

const accountPassword = "correct horse battery"

var (
	pendingField  = regexp.MustCompile(`name="pending" value="([^"]+)"`)
	secretShown   = regexp.MustCompile(`<code id="totp-secret">([A-Z2-7 ]+)</code>`)
	recoveryShown = regexp.MustCompile(`<li>([2-9a-z]{5}-[2-9a-z]{5})</li>`)
)

// accountPosts are every form POST of the account page (the key ceremony
// routes are JSON and tested apart).
var accountPosts = []string{
	"/manager/account/totp/start", "/manager/account/totp/confirm", "/manager/account/totp/remove",
	"/manager/account/keys/begin", "/manager/account/keys/finish", "/manager/account/keys/x/remove",
	"/manager/account/recovery/regenerate", "/manager/account/password",
}

// startTOTP opens the authenticator setup as s and returns the sealed
// pending value and the secret shown on the page.
func (e *env) startTOTP(s *auth.Session) (pending, secret string) {
	e.t.Helper()
	rec := e.postAs(s, "/manager/account/totp/start", nil, true)
	e.wantStatus(rec, http.StatusOK, "/manager/account/totp/qr.png?p=", `autocomplete="one-time-code"`)
	p := pendingField.FindStringSubmatch(rec.Body.String())
	sec := secretShown.FindStringSubmatch(rec.Body.String())
	if p == nil || sec == nil {
		e.t.Fatalf("no pending value or secret in %.800s", rec.Body.String())
	}
	if !regexp.MustCompile(`^([A-Z2-7]{4} )+[A-Z2-7]{1,4}$`).MatchString(sec[1]) {
		e.t.Fatalf("secret %q not grouped by 4", sec[1])
	}
	return html.UnescapeString(p[1]), strings.ReplaceAll(sec[1], " ", "")
}

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	c, err := auth.TOTPCode(secret, at)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// login opens another session for email, as a login elsewhere would.
func (e *env) login(email, password string) auth.Session {
	e.t.Helper()
	s, err := e.deps.Auth.Login(context.Background(), email, password)
	if err != nil {
		e.t.Fatalf("login %s: %v", email, err)
	}
	return s
}

func (e *env) alive(s *auth.Session) bool {
	e.t.Helper()
	return e.do(httptest.NewRequest(http.MethodGet, "/manager/account", nil), s).Code == http.StatusOK
}

func (e *env) status(u string) auth.Status {
	e.t.Helper()
	st, err := e.deps.Auth.MFAStatus(context.Background(), u)
	if err != nil {
		e.t.Fatal(err)
	}
	return st
}

func TestAccountPageForEveryUser(t *testing.T) {
	e := newEnv(t)
	rec := e.do(httptest.NewRequest(http.MethodGet, "/manager/account", nil), &e.scorer)
	e.wantStatus(rec, http.StatusOK, "My account", "Authenticator app", "Security keys", "Change password", `action="/manager/account/password"`)
	if strings.Contains(rec.Body.String(), "admin@example.org") {
		t.Fatal("scorer's account page shows another account")
	}
	e.wantStatus(e.get("/manager"), http.StatusOK, `href="/manager/account"`)
	e.wantRedirect(e.do(httptest.NewRequest(http.MethodGet, "/manager/account", nil), nil), "/manager/login")
}

func TestAccountPostsNeedCSRF(t *testing.T) {
	e := newEnv(t)
	for _, p := range accountPosts {
		if rec := e.postAs(&e.scorer, p, url.Values{"current_password": {accountPassword}}, false); rec.Code != http.StatusForbidden {
			t.Errorf("%s without CSRF = %d, want 403", p, rec.Code)
		}
		if rec := e.do(httptest.NewRequest(http.MethodPost, p, nil), nil); rec.Code != http.StatusSeeOther {
			t.Errorf("%s without session = %d, want 303 to login", p, rec.Code)
		}
	}
}

func TestAccountEnrollTOTP(t *testing.T) {
	e := newEnv(t)
	scorerID := e.scorer.User.ID
	elsewhere := e.login("scorer@example.org", accountPassword)
	pending, secret := e.startTOTP(&e.scorer)

	// QR code: logged in, own and live pending value only.
	qr := e.do(httptest.NewRequest(http.MethodGet, "/manager/account/totp/qr.png?p="+url.QueryEscape(pending), nil), &e.scorer)
	if qr.Code != http.StatusOK || qr.Header().Get("Content-Type") != "image/png" || qr.Header().Get("Cache-Control") != "no-store" || !strings.HasPrefix(qr.Body.String(), "\x89PNG") {
		t.Fatalf("qr = %d %q %q", qr.Code, qr.Header().Get("Content-Type"), qr.Header().Get("Cache-Control"))
	}
	e.wantRedirect(e.do(httptest.NewRequest(http.MethodGet, "/manager/account/totp/qr.png?p="+url.QueryEscape(pending), nil), nil), "/manager/login")
	for _, s := range []*auth.Session{&e.admin} {
		if rec := e.do(httptest.NewRequest(http.MethodGet, "/manager/account/totp/qr.png?p="+url.QueryEscape(pending), nil), s); rec.Code != http.StatusNotFound {
			t.Fatalf("another user's pending QR = %d, want 404", rec.Code)
		}
	}
	if rec := e.do(httptest.NewRequest(http.MethodGet, "/manager/account/totp/qr.png?p=garbage", nil), &e.scorer); rec.Code != http.StatusNotFound {
		t.Fatalf("garbage pending QR = %d, want 404", rec.Code)
	}

	confirm := func(code, password string) *httptest.ResponseRecorder {
		return e.postAs(&e.scorer, "/manager/account/totp/confirm", url.Values{"pending": {pending}, "code": {code}, "current_password": {password}}, true)
	}
	// Wrong password, then a wrong code: 422, the setup is shown again, nothing stored.
	e.wantStatus(confirm(codeAt(t, secret, time.Now()), "wrong-password"), http.StatusUnprocessableEntity, "Current password is wrong", "/manager/account/totp/qr.png?p=")
	e.wantStatus(confirm("000000", accountPassword), http.StatusUnprocessableEntity, "does not match", "/manager/account/totp/qr.png?p=")
	if st := e.status(scorerID); st.TOTP || st.RecoveryLeft != 0 || !e.alive(&elsewhere) {
		t.Fatalf("refused confirmation changed something: %+v", st)
	}

	rec := confirm(codeAt(t, secret, time.Now()), accountPassword)
	e.wantStatus(rec, http.StatusOK, "Recovery codes", "I saved them", "data-copy-list")
	codes := recoveryShown.FindAllStringSubmatch(rec.Body.String(), -1)
	if len(codes) != 10 {
		t.Fatalf("%d recovery codes shown, want 10", len(codes))
	}
	if st := e.status(scorerID); !st.TOTP || st.RecoveryLeft != 10 {
		t.Fatalf("status = %+v", st)
	}
	if !e.alive(&e.scorer) || e.alive(&elsewhere) {
		t.Fatal("want the current session kept and the other revoked")
	}
	// Shown once: the page afterwards has the count only.
	page := e.do(httptest.NewRequest(http.MethodGet, "/manager/account", nil), &e.scorer)
	e.wantStatus(page, http.StatusOK, "10 recovery codes left", `action="/manager/account/totp/remove"`)
	if strings.Contains(page.Body.String(), codes[0][1]) {
		t.Fatal("recovery code shown again")
	}
	// The login now asks for the second factor.
	if _, err := e.deps.Auth.Login(context.Background(), "scorer@example.org", accountPassword); err != auth.ErrMFARequired {
		t.Fatalf("login after enrolment = %v, want ErrMFARequired", err)
	}
}

func TestAccountRemoveLastFactorDisablesMFA(t *testing.T) {
	e := newEnv(t)
	pending, secret := e.startTOTP(&e.scorer)
	e.wantStatus(e.postAs(&e.scorer, "/manager/account/totp/confirm", url.Values{"pending": {pending}, "code": {codeAt(t, secret, time.Now())}, "current_password": {accountPassword}}, true), http.StatusOK)

	e.wantStatus(e.postAs(&e.scorer, "/manager/account/totp/remove", url.Values{"current_password": {"wrong-password"}}, true), http.StatusUnprocessableEntity, "Current password is wrong")
	e.wantRedirect(e.postAs(&e.scorer, "/manager/account/totp/remove", url.Values{"current_password": {accountPassword}}, true), "/manager/account?ok=")
	if st := e.status(e.scorer.User.ID); st.TOTP || st.RecoveryLeft != 0 {
		t.Fatalf("status = %+v", st)
	}
	e.login("scorer@example.org", accountPassword) // password only again
}

func TestAccountRegenerateRecoveryCodes(t *testing.T) {
	e := newEnv(t)
	e.wantStatus(e.postAs(&e.scorer, "/manager/account/recovery/regenerate", url.Values{"current_password": {accountPassword}}, true), http.StatusConflict, "authenticator app or a security key first")
	pending, secret := e.startTOTP(&e.scorer)
	first := e.postAs(&e.scorer, "/manager/account/totp/confirm", url.Values{"pending": {pending}, "code": {codeAt(t, secret, time.Now())}, "current_password": {accountPassword}}, true)
	old := recoveryShown.FindAllStringSubmatch(first.Body.String(), -1)

	e.wantStatus(e.postAs(&e.scorer, "/manager/account/recovery/regenerate", url.Values{"current_password": {"wrong-password"}}, true), http.StatusUnprocessableEntity, "Current password is wrong")
	rec := e.postAs(&e.scorer, "/manager/account/recovery/regenerate", url.Values{"current_password": {accountPassword}}, true)
	e.wantStatus(rec, http.StatusOK, "I saved them")
	fresh := recoveryShown.FindAllStringSubmatch(rec.Body.String(), -1)
	if len(fresh) != 10 || len(old) != 10 || fresh[0][1] == old[0][1] {
		t.Fatalf("regenerated %d codes (old %d)", len(fresh), len(old))
	}
}

func TestAccountChangePassword(t *testing.T) {
	e := newEnv(t)
	elsewhere := e.login("scorer@example.org", accountPassword)
	change := func(current, next string) *httptest.ResponseRecorder {
		return e.postAs(&e.scorer, "/manager/account/password", url.Values{"current_password": {current}, "new_password": {next}}, true)
	}
	e.wantStatus(change("wrong-password", "a-brand-new-one"), http.StatusUnprocessableEntity, "Current password is wrong")
	e.wantStatus(change(accountPassword, "short"), http.StatusUnprocessableEntity, "at least 10 characters")
	if !e.alive(&elsewhere) {
		t.Fatal("refused change revoked a session")
	}
	e.wantRedirect(change(accountPassword, "a-brand-new-one"), "/manager/account?ok=")
	if !e.alive(&e.scorer) || e.alive(&elsewhere) {
		t.Fatal("want the current session kept and the other revoked")
	}
	e.login("scorer@example.org", "a-brand-new-one")
}

// postJSON sends a JSON body as s with the CSRF header webauthn.js sets.
func (e *env) postJSON(s *auth.Session, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", s.CSRF)
	return e.do(req, s)
}

func (e *env) addKey(s *auth.Session, key *webauthntest.Key, name, password string) *httptest.ResponseRecorder {
	e.t.Helper()
	begin := e.postJSON(s, "/manager/account/keys/begin", "{}")
	if begin.Code != http.StatusOK || begin.Header().Get("Content-Type") != "application/json" {
		e.t.Fatalf("begin = %d %s", begin.Code, begin.Body.String())
	}
	var opts struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(begin.Body.Bytes(), &opts); err != nil || opts.Token == "" {
		e.t.Fatalf("begin answer %s: %v", begin.Body.String(), err)
	}
	cred, err := key.Create(begin.Body.Bytes(), "http://localhost:0")
	if err != nil {
		e.t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"name": name, "password": password, "token": opts.Token, "credential": json.RawMessage(cred)})
	return e.postJSON(s, "/manager/account/keys/finish", string(body))
}

const backupName = "Backup <script>alert(1)</script>"

func TestAccountSecurityKeys(t *testing.T) {
	e := newEnv(t)
	elsewhere := e.login("scorer@example.org", accountPassword)
	key, err := webauthntest.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	rec := e.addKey(&e.scorer, key, "YubiKey", "wrong-password")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Current password is wrong") {
		t.Fatalf("wrong password = %d %s", rec.Code, rec.Body.String())
	}
	rec = e.addKey(&e.scorer, key, "YubiKey", accountPassword)
	var done struct {
		RecoveryCodes []string `json:"recoveryCodes"`
		Redirect      string   `json:"redirect"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &done); rec.Code != http.StatusOK || err != nil || len(done.RecoveryCodes) != 10 {
		t.Fatalf("first key = %d %s", rec.Code, rec.Body.String())
	}
	if !e.alive(&e.scorer) || e.alive(&elsewhere) {
		t.Fatal("want the current session kept and the other revoked")
	}
	// The same key again is refused; a second key just redirects.
	if rec = e.addKey(&e.scorer, key, "Again", accountPassword); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("same key again = %d %s", rec.Code, rec.Body.String())
	}
	key2, _ := webauthntest.NewKey()
	rec = e.addKey(&e.scorer, key2, backupName, accountPassword)
	done.RecoveryCodes = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &done); rec.Code != http.StatusOK || err != nil || done.Redirect != "/manager/account?ok=key_added" || done.RecoveryCodes != nil {
		t.Fatalf("second key = %d %s", rec.Code, rec.Body.String())
	}
	page := e.do(httptest.NewRequest(http.MethodGet, "/manager/account", nil), &e.scorer)
	e.wantStatus(page, http.StatusOK, "YubiKey", "Backup &lt;script&gt;alert(1)&lt;/script&gt;", "never used")
	if strings.Contains(page.Body.String(), "<script>alert(1)") {
		t.Fatal("key name not escaped")
	}

	// Removal: own keys only, password required.
	id := base64.RawURLEncoding.EncodeToString(key.ID)
	adminKey, _ := webauthntest.NewKey()
	if rec := e.addKey(&e.admin, adminKey, "Admin key", accountPassword); rec.Code != http.StatusOK {
		t.Fatalf("admin key = %d", rec.Code)
	}
	adminID := base64.RawURLEncoding.EncodeToString(adminKey.ID)
	if rec := e.postAs(&e.scorer, "/manager/account/keys/"+adminID+"/remove", url.Values{"current_password": {accountPassword}}, true); rec.Code != http.StatusNotFound {
		t.Fatalf("removing another user's key = %d, want 404", rec.Code)
	}
	e.wantStatus(e.postAs(&e.scorer, "/manager/account/keys/"+id+"/remove", url.Values{"current_password": {"wrong-password"}}, true), http.StatusUnprocessableEntity, "Current password is wrong")
	e.wantRedirect(e.postAs(&e.scorer, "/manager/account/keys/"+id+"/remove", url.Values{"current_password": {accountPassword}}, true), "/manager/account?ok=")
	if st := e.status(e.scorer.User.ID); len(st.Keys) != 1 || st.Keys[0].Name != backupName || st.RecoveryLeft != 10 {
		t.Fatalf("status = %+v", st)
	}
	if st := e.status(e.admin.User.ID); len(st.Keys) != 1 {
		t.Fatalf("admin's key touched: %+v", st)
	}
}

func TestAccountWrongPasswordsLimited(t *testing.T) {
	e := newEnv(t)
	change := func(current string) *httptest.ResponseRecorder {
		return e.postAs(&e.scorer, "/manager/account/password", url.Values{"current_password": {current}, "new_password": {"a-brand-new-one"}}, true)
	}
	for range 5 {
		e.wantStatus(change("wrong-password"), http.StatusUnprocessableEntity, "Current password is wrong")
	}
	rec := change(accountPassword)
	e.wantStatus(rec, http.StatusTooManyRequests, "Too many wrong passwords, try again in 15 minutes")
	if ra := rec.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Fatalf("Retry-After = %q", ra)
	}
	key, _ := webauthntest.NewKey()
	rec = e.addKey(&e.scorer, key, "Key", accountPassword)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || !strings.Contains(rec.Body.String(), "Too many wrong passwords") {
		t.Fatalf("key finish while limited = %d %s", rec.Code, rec.Body.String())
	}
	// Only this account is limited.
	e.wantRedirect(e.postAs(&e.admin, "/manager/account/password", url.Values{"current_password": {accountPassword}, "new_password": {"a-brand-new-one"}}, true), "/manager/account?ok=")
}

func TestTOTPEnrollmentWithoutMFAKeyShowsMessage(t *testing.T) {
	e := newEnvNoMFAKey(t)
	e.wantStatus(e.postAs(&e.scorer, "/manager/account/totp/start", nil, true), http.StatusServiceUnavailable,
		"Authenticator apps are not available: the server has no MFA key configured")
}
