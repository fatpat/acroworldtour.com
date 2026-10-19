package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"awt/internal/config"
	"awt/internal/store"
	"awt/internal/store/gen"
)

func newAuth(t *testing.T, env string) *Auth {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	a := New(db, config.Config{Env: env})
	a.cost = bcrypt.MinCost // bcrypt cost 12 is too slow for tests
	return a
}

func TestDefaultBcryptCost(t *testing.T) {
	if a := New(nil, config.Config{}); a.cost != 12 {
		t.Fatalf("cost = %d", a.cost)
	}
}

func addComp(t *testing.T, a *Auth, id, code string) {
	t.Helper()
	_, err := a.db.Write.Exec(`INSERT INTO competitions (id, code, name, type, start_date, end_date, config) VALUES (?, ?, ?, 'solo', '2026-01-01', '2026-01-02', '{}')`, id, code, code)
	if err != nil {
		t.Fatal(err)
	}
}

func mustUser(t *testing.T, a *Auth, email, role string) User {
	t.Helper()
	u, err := a.CreateUser(context.Background(), email, "Name", role, "s3cret-passw0rd")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func mustLogin(t *testing.T, a *Auth, email string) Session {
	t.Helper()
	s, err := a.Login(context.Background(), email, "s3cret-passw0rd")
	if err != nil {
		t.Fatalf("login %s: %v", email, err)
	}
	return s
}

func cookieFor(s Session) *http.Cookie { return &http.Cookie{Name: CookieName, Value: s.Token} }

func do(h http.Handler, method, target string, c *http.Cookie, hdr map[string]string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c != nil {
		req.AddCookie(c)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

var ok = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

func TestLoginSuccessAndFailure(t *testing.T) {
	a := newAuth(t, "dev")
	mustUser(t, a, "Admin@Example.com", "admin")
	ctx := context.Background()

	s, err := a.Login(ctx, " admin@example.com ", "s3cret-passw0rd")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if s.Token == "" || s.CSRF == "" || s.User.Role != "admin" {
		t.Fatalf("bad session %+v", s)
	}
	// Only the hash is stored.
	count := func(hash string) int {
		var n int
		if err := a.db.Read.QueryRow(`SELECT count(*) FROM sessions WHERE token_hash = ?`, hash).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count(s.Token) != 0 {
		t.Fatal("raw token stored")
	}
	if count(hashToken(s.Token)) != 1 {
		t.Fatal("hashed token not stored")
	}
	if _, err := a.Login(ctx, "admin@example.com", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := a.Login(ctx, "nobody@example.com", "x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown user: %v", err)
	}
	s2 := mustLogin(t, a, "admin@example.com")
	if s2.Token == s.Token {
		t.Fatal("token not rotated")
	}
}

func TestDisabledUserRefused(t *testing.T) {
	a := newAuth(t, "dev")
	u := mustUser(t, a, "a@example.com", "admin")
	s, err := a.Login(context.Background(), "a@example.com", "s3cret-passw0rd")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Write.Exec(`UPDATE users SET disabled = 1 WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Login(context.Background(), "a@example.com", "s3cret-passw0rd"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("disabled login: %v", err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookieFor(s))
	if _, err := a.UserFromRequest(req); err == nil {
		t.Fatal("disabled user keeps session")
	}
}

func TestSessionExpiryAndLogout(t *testing.T) {
	a := newAuth(t, "dev")
	mustUser(t, a, "a@example.com", "admin")
	s := mustLogin(t, a, "a@example.com")
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookieFor(s))
	if _, err := a.UserFromRequest(req); err != nil {
		t.Fatalf("fresh session: %v", err)
	}
	a.now = func() time.Time { return time.Now().Add(15 * 24 * time.Hour) }
	if _, err := a.UserFromRequest(req); err == nil {
		t.Fatal("expired session accepted")
	}
	a.now = time.Now
	rec := httptest.NewRecorder()
	if err := a.Logout(rec, req); err != nil {
		t.Fatal(err)
	}
	if _, err := a.UserFromRequest(req); err == nil {
		t.Fatal("session survives logout")
	}
	if c := rec.Result().Cookies(); len(c) != 1 || c[0].MaxAge >= 0 {
		t.Fatalf("cookie not cleared: %v", c)
	}
}

func TestCookieAttributes(t *testing.T) {
	for env, secure := range map[string]bool{"dev": false, "prod": true} {
		a := newAuth(t, env)
		rec := httptest.NewRecorder()
		a.SetCookie(rec, Session{Token: "tok", ExpiresAt: time.Now().Add(14 * 24 * time.Hour)})
		c := rec.Result().Cookies()[0]
		if c.Name != "awt_session" || !c.HttpOnly || c.Secure != secure || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 14*24*3600 || c.Path != "/" {
			t.Errorf("%s: cookie %+v", env, c)
		}
	}
}

func TestRequireUserRedirects(t *testing.T) {
	a := newAuth(t, "dev")
	h := a.RequireUser(ok)
	rec := do(h, "GET", "/manager/pilots?x=1", nil, nil, "")
	if rec.Code != 303 || rec.Header().Get("Location") != "/manager/login?next="+url.QueryEscape("/manager/pilots?x=1") {
		t.Fatalf("redirect: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = do(h, "GET", "/manager/pilots", nil, map[string]string{"HX-Request": "true"}, "")
	if rec.Code != 401 || rec.Header().Get("HX-Redirect") != "/manager/login?next=%2Fmanager%2Fpilots" {
		t.Fatalf("htmx: %d %q", rec.Code, rec.Header().Get("HX-Redirect"))
	}
}

func TestRequireAdminAndScorer(t *testing.T) {
	a := newAuth(t, "dev")
	addComp(t, a, "compa", "A")
	addComp(t, a, "compb", "B")
	mustUser(t, a, "admin@example.com", "admin")
	sc := mustUser(t, a, "sc@example.com", "scorer")
	if err := a.AssignCompetition(context.Background(), "sc@example.com", "A"); err != nil { // by code
		t.Fatal(err)
	}
	_ = sc
	as := mustLogin(t, a, "admin@example.com")
	ss := mustLogin(t, a, "sc@example.com")

	if rec := do(a.RequireAdmin(ok), "GET", "/", cookieFor(ss), nil, ""); rec.Code != 403 {
		t.Fatalf("scorer on admin: %d", rec.Code)
	}
	if rec := do(a.RequireAdmin(ok), "GET", "/", cookieFor(as), nil, ""); rec.Code != 200 {
		t.Fatalf("admin on admin: %d", rec.Code)
	}
	mux := http.NewServeMux()
	mux.Handle("/comp/{id}", a.RequireScorer(func(r *http.Request) string { return r.PathValue("id") })(ok))
	if rec := do(mux, "GET", "/comp/compa", cookieFor(ss), nil, ""); rec.Code != 200 {
		t.Fatalf("scorer on A: %d", rec.Code)
	}
	if rec := do(mux, "GET", "/comp/compb", cookieFor(ss), nil, ""); rec.Code != 403 {
		t.Fatalf("scorer on B: %d", rec.Code)
	}
	if rec := do(mux, "GET", "/comp/A", cookieFor(ss), nil, ""); rec.Code != 200 {
		t.Fatalf("scorer on A by code: %d", rec.Code)
	}
	if rec := do(mux, "GET", "/comp/B", cookieFor(ss), nil, ""); rec.Code != 403 {
		t.Fatalf("scorer on B by code: %d", rec.Code)
	}
	if rec := do(mux, "GET", "/comp/nope", cookieFor(ss), nil, ""); rec.Code != 403 {
		t.Fatalf("scorer on unknown: %d", rec.Code)
	}
	if rec := do(mux, "GET", "/comp/compb", cookieFor(as), nil, ""); rec.Code != 200 {
		t.Fatalf("admin on B: %d", rec.Code)
	}
	if rec := do(mux, "GET", "/comp/compb", nil, nil, ""); rec.Code != 303 {
		t.Fatalf("anon: %d", rec.Code)
	}
}

func TestCSRF(t *testing.T) {
	a := newAuth(t, "dev")
	mustUser(t, a, "a@example.com", "admin")
	s := mustLogin(t, a, "a@example.com")
	h := a.RequireUser(a.CSRF(ok))
	c := cookieFor(s)
	if rec := do(h, "GET", "/", c, nil, ""); rec.Code != 200 {
		t.Fatalf("GET: %d", rec.Code)
	}
	if rec := do(h, "POST", "/", c, nil, "a=b"); rec.Code != 403 {
		t.Fatalf("missing token: %d", rec.Code)
	}
	if rec := do(h, "POST", "/", c, map[string]string{"X-CSRF-Token": "nope"}, ""); rec.Code != 403 {
		t.Fatalf("bad token: %d", rec.Code)
	}
	if rec := do(h, "POST", "/", c, map[string]string{"X-CSRF-Token": s.CSRF}, ""); rec.Code != 200 {
		t.Fatalf("header token: %d", rec.Code)
	}
	if rec := do(h, "POST", "/", c, nil, "_csrf="+url.QueryEscape(s.CSRF)); rec.Code != 200 {
		t.Fatalf("form token: %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/", c, nil, ""); rec.Code != 403 {
		t.Fatalf("DELETE: %d", rec.Code)
	}
}

func TestLimiterBlocksSixthAttempt(t *testing.T) {
	a := newAuth(t, "dev")
	mustUser(t, a, "a@example.com", "admin")
	login := func(pw string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/manager/login", strings.NewReader(url.Values{"email": {"A@example.com"}, "password": {pw}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "1.2.3.4:5555"
		rec := httptest.NewRecorder()
		a.HandleLogin(rec, req)
		return rec
	}
	for i := 0; i < 5; i++ {
		if rec := login("bad"); rec.Code != 401 {
			t.Fatalf("attempt %d: %d", i+1, rec.Code)
		}
	}
	if rec := login("bad"); rec.Code != 429 {
		t.Fatalf("6th: %d", rec.Code)
	}
	if rec := login("s3cret-passw0rd"); rec.Code != 429 {
		t.Fatalf("correct password while blocked: %d", rec.Code)
	}
}

func TestLimiterWindowAndSuccessReset(t *testing.T) {
	now := time.Now()
	l := NewLoginLimiter(5, 15*time.Minute)
	l.now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		if !l.Attempt("k") {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	if l.Attempt("k") {
		t.Fatal("should block")
	}
	if !l.Attempt("other") {
		t.Fatal("other key blocked")
	}
	now = now.Add(16 * time.Minute)
	if !l.Attempt("k") {
		t.Fatal("window not expired")
	}
	l.Reset("k")
	for i := 0; i < 5; i++ {
		if !l.Attempt("k") {
			t.Fatal("reset ignored")
		}
	}
}

func TestLimiterSweepsExpiredKeys(t *testing.T) {
	now := time.Now()
	l := NewLoginLimiter(5, time.Minute)
	l.now = func() time.Time { return now }
	for i := 0; i <= maxLimiterKeys; i++ {
		l.Attempt(fmt.Sprint("k", i))
	}
	now = now.Add(2 * time.Minute)
	l.Attempt("fresh")
	if len(l.failures) != 1 {
		t.Fatalf("%d keys kept", len(l.failures))
	}
}

func TestLimiterConcurrentAttempts(t *testing.T) {
	a := newAuth(t, "dev")
	mustUser(t, a, "a@example.com", "admin")
	var wg sync.WaitGroup
	var reached, blocked atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/manager/login", strings.NewReader(url.Values{"email": {"a@example.com"}, "password": {"bad"}}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			a.HandleLogin(rec, req)
			if rec.Code == 429 {
				blocked.Add(1)
			} else {
				reached.Add(1)
			}
		}()
	}
	wg.Wait()
	if reached.Load() != 5 || blocked.Load() != 15 {
		t.Fatalf("reached %d, blocked %d", reached.Load(), blocked.Load())
	}
}

func TestClientIP(t *testing.T) {
	cfg := func(header string) config.Config {
		return config.Config{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, ClientIPHeader: header}
	}
	req := func(remote, hdr, val string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if hdr != "" {
			r.Header.Set(hdr, val)
		}
		return r
	}
	cf := &Auth{cfg: cfg("CF-Connecting-IP")}
	xff := &Auth{cfg: cfg("X-Forwarded-For")}
	none := &Auth{}
	cases := []struct {
		name string
		a    *Auth
		r    *http.Request
		want string
	}{
		{"trusted cf", cf, req("10.1.2.3:99", "CF-Connecting-IP", "203.0.113.7"), "203.0.113.7"},
		{"untrusted peer ignores header", cf, req("198.51.100.1:99", "CF-Connecting-IP", "203.0.113.7"), "198.51.100.1"},
		{"trusted xff first entry", xff, req("10.1.2.3:99", "X-Forwarded-For", "203.0.113.7, 10.9.9.9"), "203.0.113.7"},
		{"trusted but no header", cf, req("10.1.2.3:99", "", ""), "10.1.2.3"},
		{"trusted garbage header", cf, req("10.1.2.3:99", "CF-Connecting-IP", "junk"), "10.1.2.3"},
		{"no proxies configured", none, req("10.1.2.3:99", "CF-Connecting-IP", "203.0.113.7"), "10.1.2.3"},
	}
	for _, c := range cases {
		if got := c.a.ClientIP(c.r); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPasswordMinimumLength(t *testing.T) {
	a := newAuth(t, "dev")
	_, err := a.CreateUser(context.Background(), "s@example.com", "S", "admin", "123456789")
	if err == nil || err.Error() != "password must be at least 10 characters" {
		t.Fatalf("add: %v", err)
	}
	mustUser(t, a, "s@example.com", "admin")
	err = a.SetPassword(context.Background(), "s@example.com", "short")
	if err == nil || err.Error() != "password must be at least 10 characters" {
		t.Fatalf("passwd: %v", err)
	}
}

func TestHandleLoginSuccessAndNext(t *testing.T) {
	a := newAuth(t, "dev")
	mustUser(t, a, "a@example.com", "admin")
	post := func(next string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/manager/login", strings.NewReader(url.Values{"email": {"a@example.com"}, "password": {"s3cret-passw0rd"}, "next": {next}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		a.HandleLogin(rec, req)
		return rec
	}
	rec := post("/manager/pilots")
	if rec.Code != 303 || rec.Header().Get("Location") != "/manager/pilots" || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, bad := range []string{"https://evil.com", "//evil.com", `/\evil.com`, "/\t/evil.com", "/\n/evil.com", "/%09/evil.com", "/\r/evil.com", "/\x7f/x"} {
		if loc := post(bad).Header().Get("Location"); loc != "/manager" {
			t.Errorf("open redirect %q -> %q", bad, loc)
		}
	}
}

func TestUserManagement(t *testing.T) {
	a := newAuth(t, "dev")
	ctx := context.Background()
	addComp(t, a, "abcdef012345abcdef012345", "WC26")
	u := mustUser(t, a, "x@example.com", "scorer")
	if len(u.ID) != 24 || strings.ToLower(u.ID) != u.ID {
		t.Fatalf("id %q", u.ID)
	}
	if _, err := a.CreateUser(ctx, "x@example.com", "N", "scorer", "long-enough-pw"); err == nil {
		t.Fatal("duplicate email accepted")
	}
	if _, err := a.CreateUser(ctx, "y@example.com", "N", "root", "long-enough-pw"); err == nil {
		t.Fatal("bad role accepted")
	}
	if err := a.SetPassword(ctx, "x@example.com", "new-password-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Login(ctx, "x@example.com", "new-password-1"); err != nil {
		t.Fatalf("new password: %v", err)
	}
	if err := a.AssignCompetition(ctx, "x@example.com", "abcdef012345abcdef012345"); err != nil { // by id
		t.Fatal(err)
	}
	if err := a.AssignCompetition(ctx, "x@example.com", "nope"); err == nil {
		t.Fatal("unknown competition accepted")
	}
	ids, _ := gen.New(a.db.Read).ListUserCompetitions(ctx, u.ID)
	if len(ids) != 1 {
		t.Fatalf("assignments %v", ids)
	}
}
