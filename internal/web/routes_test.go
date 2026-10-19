package web_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"awt/internal/auth"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/store"
	"awt/internal/web"
	"awt/internal/web/layout"
	"awt/web/static"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func newDeps(t *testing.T) web.Deps {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: "dev", Addr: ":0"}
	return web.Deps{Cfg: cfg, Auth: auth.New(db, cfg), DB: db, Purger: cache.NoopPurger{}}
}

func ok(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }

// sample registers one route per guard level, as feature modules will.
var sample = web.ModuleFunc(func(r *web.Router) {
	r.Public("GET /pilots", http.HandlerFunc(ok))
	r.User("GET /manager/me", http.HandlerFunc(ok))
	r.Admin("GET /manager/users", http.HandlerFunc(ok))
	r.Admin("POST /manager/users", http.HandlerFunc(ok))
	r.Scorer("GET /manager/competitions/{code}/flights", func(r *http.Request) string { return r.PathValue("code") }, http.HandlerFunc(ok))
	r.Public("GET /boom", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("kaboom") }))
})

func get(h http.Handler, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

var wildcard = regexp.MustCompile(`\{[^}]*\}`)

func TestAdminRoutesAreGuarded(t *testing.T) {
	deps := newDeps(t)
	table := web.RouteTable(deps, sample)
	h := web.Routes(deps, sample)
	adminRoutes := 0
	for _, rt := range table {
		p := rt.Path()
		if p != "/manager" && !strings.HasPrefix(p, "/manager/") {
			continue
		}
		adminRoutes++
		if p == "/manager/login" || p == "/manager/login/mfa" || strings.HasPrefix(p, "/manager/login/mfa/") {
			continue // the login steps are the only anonymous admin URLs
		}
		if rt.Guard == web.GuardPublic {
			t.Errorf("%s is under /manager but public", rt.Pattern)
			continue
		}
		// Behaviour, not just the table: anonymous requests never get through.
		method, _, _ := strings.Cut(rt.Pattern, " ")
		req := httptest.NewRequest(method, wildcard.ReplaceAllString(p, "x"), nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK || rec.Code == http.StatusNotFound {
			t.Errorf("%s answered %d without a session", rt.Pattern, rec.Code)
		}
	}
	if adminRoutes < 6 {
		t.Fatalf("only %d admin routes walked", adminRoutes)
	}
}

func TestPublicRouteUnderAdminIsRefused(t *testing.T) {
	deps := newDeps(t)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	web.Routes(deps, web.ModuleFunc(func(r *web.Router) {
		r.Public("GET /manager/secret", http.HandlerFunc(ok))
	}))
}

func TestStaticFilesLongCacheWithHash(t *testing.T) {
	h := web.Routes(newDeps(t))
	u := static.URL("css/app.css")
	if !regexp.MustCompile(`^/static/css/app\.css\?v=[0-9a-f]{12}$`).MatchString(u) {
		t.Fatalf("URL = %q", u)
	}
	rec := get(h, u)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("versioned Cache-Control = %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty body")
	}

	if cc := get(h, "/static/css/app.css").Header().Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Fatalf("unversioned must not be immutable: %q", cc)
	}
	if ct := get(h, static.URL("fonts/exo-v20-latin-700.woff2")).Header().Get("Content-Type"); ct != "font/woff2" {
		t.Fatalf("font Content-Type = %q", ct)
	}
	if code := get(h, "/static/css/nope.css").Code; code != 404 {
		t.Fatalf("missing file status %d", code)
	}
	if code := get(h, "/static/css/").Code; code != 404 {
		t.Fatalf("directory listing status %d", code)
	}

	req := httptest.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("conditional status %d", rec2.Code)
	}
}

func TestVendoredHTMXServed(t *testing.T) {
	h := web.Routes(newDeps(t))
	for _, p := range []string{"vendor/htmx-2.0.4.min.js", "vendor/htmx-ext-sse-2.2.2.js", "js/app.js"} {
		rec := get(h, static.URL(p))
		if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") {
			t.Errorf("%s: %d %q", p, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
}

func TestNotFoundPage(t *testing.T) {
	h := web.Routes(newDeps(t))
	rec := get(h, "/no/such/page")
	if rec.Code != 404 {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Page not found", "<title>Page not found | Acro World Tour</title>", `href="/pilots"`, `noindex`} {
		if !strings.Contains(body, want) {
			t.Errorf("404 body missing %q", want)
		}
	}
	for k, v := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q", k, got)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"frame-ancestors 'none'", "script-src 'self'", "style-src 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q missing %q", csp, want)
		}
	}
	if strings.Contains(csp, "unsafe-inline") {
		t.Errorf("CSP allows inline: %q", csp)
	}
}

func TestRedirectTable(t *testing.T) {
	deps := newDeps(t)
	for _, r := range []struct {
		from, to string
		perm     int
	}{
		{"/news/acromax.html", "/news/acromax", 1},
		{"/old/temp", "https://example.org/x", 0},
		{"/evil", "javascript:alert(1)", 1},
		{"/proto", "//evil.com", 1},
		{"/back", "/\\evil.com", 1},
	} {
		if _, err := deps.DB.Write.Exec(`INSERT INTO redirects (from_path, to_url, permanent) VALUES (?, ?, ?)`, r.from, r.to, r.perm); err != nil {
			t.Fatal(err)
		}
	}
	h := web.Routes(deps)

	rec := get(h, "/news/acromax.html")
	if rec.Code != 301 || rec.Header().Get("Location") != "/news/acromax" {
		t.Fatalf("permanent: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = get(h, "/old/temp/") // trailing slash tolerated
	if rec.Code != 302 || rec.Header().Get("Location") != "https://example.org/x" {
		t.Fatalf("temporary: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if code := get(h, "/evil").Code; code != 404 {
		t.Fatalf("unsafe target must 404, got %d", code)
	}
	for _, p := range []string{"/proto", "/back"} {
		if code := get(h, p).Code; code != 404 {
			t.Fatalf("%s: protocol-relative target must 404, got %d", p, code)
		}
	}
	if code := get(h, "/unknown").Code; code != 404 {
		t.Fatalf("unknown path %d", code)
	}
}

func TestPanicRecoveryAndRequestLog(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	h := web.Routes(newDeps(t), sample)
	rec := get(h, "/boom")
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "Something went wrong") {
		t.Fatalf("panic response %d %q", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatal("panic value leaked to the client")
	}
	log := buf.String()
	for _, want := range []string{"panic", "kaboom", "method=GET", "path=/boom", "status=500", "duration=", "ip="} {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %q:\n%s", want, log)
		}
	}
}

func TestPublicLayout(t *testing.T) {
	rec := get(web.Routes(newDeps(t)), "/nope")
	body := rec.Body.String()
	for _, want := range []string{
		">Home<", ">Events<", ">News<", ">Results<", ">Seasons<", ">Competitions<", ">Pilots<", ">Teams<", ">Judges<", ">Tricks<", ">Simulator<",
		"Copyright " + time.Now().Format("2006"),
		`property="og:title"`, `property="og:site_name"`,
		"/static/vendor/htmx-2.0.4.min.js?v=", "/static/css/app.css?v=",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("layout missing %q", want)
		}
	}
	if strings.Contains(body, "csrf-token") {
		t.Error("anonymous page must not carry a CSRF token")
	}
}

func TestLoginAdminAndLogoutFlow(t *testing.T) {
	deps := newDeps(t)
	if _, err := deps.Auth.CreateUser(context.Background(), "boss@example.org", "Boss", auth.RoleAdmin, "s3cret-passw0rd"); err != nil {
		t.Fatal(err)
	}
	h := web.Routes(deps)

	rec := get(h, "/manager")
	if rec.Code != 303 || !strings.HasPrefix(rec.Header().Get("Location"), "/manager/login?next=") {
		t.Fatalf("anonymous /manager: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = get(h, "/manager/login?next=%2Fmanager%2Fpilots")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `action="/manager/login"`) || !strings.Contains(rec.Body.String(), `value="/manager/pilots"`) {
		t.Fatalf("login page: %d\n%s", rec.Code, rec.Body.String())
	}

	post := func(path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec = post("/manager/login", url.Values{"email": {"boss@example.org"}, "password": {"wrong"}})
	if rec.Code != 401 || !strings.Contains(rec.Body.String(), "invalid email or password") || !strings.Contains(rec.Body.String(), "<form") {
		t.Fatalf("bad login: %d %q", rec.Code, rec.Body.String())
	}

	rec = post("/manager/login", url.Values{"email": {"boss@example.org"}, "password": {"s3cret-passw0rd"}, "next": {"/manager"}})
	if rec.Code != 303 || rec.Header().Get("Location") != "/manager" {
		t.Fatalf("login: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v", cookies)
	}

	rec = get(h, "/manager", cookies[0])
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("/manager: %d", rec.Code)
	}
	m := regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no csrf meta in:\n%s", body)
	}
	token := m[1]
	if !strings.Contains(body, "hx-headers=") || !strings.Contains(body, "X-CSRF-Token") {
		t.Error("htmx hx-headers wiring missing")
	}
	for _, want := range []string{">Pilots<", ">Teams<", ">Judges<", ">Tricks<", ">Seasons<", ">Competitions<", ">Files<", ">Users<", ">Content<", "Boss", `action="/manager/logout"`} {
		if !strings.Contains(body, want) {
			t.Errorf("admin layout missing %q", want)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("admin Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}

	if rec = post("/manager/logout", nil, cookies[0]); rec.Code != 403 {
		t.Fatalf("logout without CSRF: %d", rec.Code)
	}
	rec = post("/manager/logout", url.Values{"_csrf": {token}}, cookies[0])
	if rec.Code != 303 || rec.Header().Get("Location") != "/manager/login" {
		t.Fatalf("logout: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec = get(h, "/manager", cookies[0]); rec.Code != 303 {
		t.Fatalf("session still valid after logout: %d", rec.Code)
	}
}

func TestPublicNavVisibleWithoutScriptAndToggleAccessible(t *testing.T) {
	body := get(web.Routes(newDeps(t)), "/nope").Body.String()
	if strings.Contains(body, "<details") {
		t.Error("nav must not rely on <details>")
	}
	list := strings.Index(body, `id="nav-list"`)
	if list < 0 || !strings.Contains(body[:list], `aria-controls="nav-list"`) || !strings.Contains(body, `aria-expanded="false"`) {
		t.Error("toggle button needs aria-controls=nav-list and aria-expanded")
	}
	if !strings.Contains(body[list:], ">Home<") {
		t.Error("nav list missing")
	}
}

func TestAdminLayoutWithoutSessionDoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	if err := layout.AdminHome().Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "/manager/logout") {
		t.Error("no session: no user menu expected")
	}
}

// The anonymous login POST caps its body before parsing the form.
func TestLoginBodyCapped(t *testing.T) {
	h := web.Routes(newDeps(t))
	big := url.Values{"email": {"a@b.c"}, "password": {strings.Repeat("x", 64<<10)}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/manager/login", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized login = %d, want 413", rec.Code)
	}
	// Chunked (no Content-Length): the reader cap applies.
	req = httptest.NewRequest(http.MethodPost, "/manager/login", io.MultiReader(strings.NewReader(big)))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized chunked login = %d, want 413", rec.Code)
	}
}

// Only the login steps escape the /manager guard: a sibling path is refused.
func TestPublicLoginPathsAreExact(t *testing.T) {
	deps := newDeps(t)
	for _, p := range []string{"/manager/login/other", "/manager/loginx", "/manager/login/mfax"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("public %s accepted", p)
				}
			}()
			web.Routes(deps, web.ModuleFunc(func(r *web.Router) { r.Public("GET "+p, http.HandlerFunc(ok)) }))
		}()
	}
}

var mfaKey = bytes.Repeat([]byte{7}, 32)

// sealTOTP mirrors auth's sealed format (v1:<keyid>:<base64(nonce|ct)>, the
// purpose as GCM additional data): Task 5 builds the enrolment that stores
// it, and auth keeps sealSecret unexported.
func sealTOTP(t *testing.T, userID, secret string) string {
	t.Helper()
	block, err := aes.NewCipher(mfaKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	ct := gcm.Seal(nonce, nonce, []byte(secret), []byte(auth.PurposeTOTPSecret+userID))
	id := sha256.Sum256(mfaKey)
	return "v1:" + hex.EncodeToString(id[:4]) + ":" + base64.StdEncoding.EncodeToString(ct)
}

type mfaEnv struct {
	t       *testing.T
	h       http.Handler
	deps    web.Deps
	secret  string
	recover []string
}

// newMFAEnv creates mfa@example.org with a TOTP factor and recovery codes.
func newMFAEnv(t *testing.T) *mfaEnv {
	t.Helper()
	deps := newDeps(t)
	deps.Cfg.MFAKey = mfaKey
	deps.Auth = auth.New(deps.DB, deps.Cfg)
	ctx := context.Background()
	u, err := deps.Auth.CreateUser(ctx, "mfa@example.org", "Mfa", auth.RoleAdmin, "s3cret-passw0rd")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.DB.Write.Exec(`INSERT INTO user_totp (user_id, secret_enc, created_at) VALUES (?, ?, ?)`, u.ID, sealTOTP(t, u.ID, secret), time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	display, hashes, err := auth.NewRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hashes {
		if _, err := deps.DB.Write.Exec(`INSERT INTO user_recovery_codes (user_id, code_hash) VALUES (?, ?)`, u.ID, h); err != nil {
			t.Fatal(err)
		}
	}
	return &mfaEnv{t: t, h: web.Routes(deps), deps: deps, secret: secret, recover: display}
}

func (e *mfaEnv) post(path string, form url.Values, origin string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

// password runs the first step and returns the pending cookie.
func (e *mfaEnv) password(next string) *http.Cookie {
	e.t.Helper()
	rec := e.post("/manager/login", url.Values{"email": {"mfa@example.org"}, "password": {"s3cret-passw0rd"}, "next": {next}}, "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/manager/login/mfa" {
		e.t.Fatalf("password step: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.PendingCookieName {
			return c
		}
		if c.Name == auth.CookieName {
			e.t.Fatal("session cookie issued before the second factor")
		}
	}
	e.t.Fatal("no pending cookie")
	return nil
}

func (e *mfaEnv) code() string {
	e.t.Helper()
	c, err := auth.TOTPCode(e.secret, time.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	return c
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestMFALoginWithTOTP(t *testing.T) {
	e := newMFAEnv(t)
	pending := e.password("/manager/pilots")

	// The pending cookie opens nothing under /manager.
	if rec := get(e.h, "/manager", pending, &http.Cookie{Name: auth.CookieName, Value: pending.Value}); rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/manager/login") {
		t.Fatalf("pending cookie on /manager: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	rec := get(e.h, "/manager/login/mfa", pending)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("second step page: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	for _, want := range []string{`action="/manager/login/mfa/totp"`, `action="/manager/login/mfa/recovery"`, `autocomplete="one-time-code"`, "Use a recovery code"} {
		if !strings.Contains(body, want) {
			t.Errorf("second step page missing %q", want)
		}
	}
	if strings.Contains(body, "csrf-token") || strings.Contains(body, "s3cret") {
		t.Error("second step page leaks session data")
	}

	if rec = e.post("/manager/login/mfa/totp", url.Values{"code": {e.code()}}, "https://evil.example", pending); rec.Code != http.StatusForbidden {
		t.Fatalf("foreign Origin: %d", rec.Code)
	}
	rec = e.post("/manager/login/mfa/totp", url.Values{"code": {"12345"}}, "", pending)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Invalid code") || !strings.Contains(rec.Body.String(), `action="/manager/login/mfa/totp"`) {
		t.Fatalf("wrong code: %d\n%s", rec.Code, rec.Body.String())
	}

	rec = e.post("/manager/login/mfa/totp", url.Values{"code": {e.code()}}, "http://localhost:0", pending)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/manager/pilots" {
		t.Fatalf("correct code: %d %q\n%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
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
	rec = e.post("/manager/login/mfa/totp", url.Values{"code": {e.code()}}, "", pending)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Your login expired, please sign in again") || !strings.Contains(rec.Body.String(), `action="/manager/login"`) {
		t.Fatalf("spent pending: %d\n%s", rec.Code, rec.Body.String())
	}
	if rec = get(e.h, "/manager/login/mfa"); rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Your login expired") {
		t.Fatalf("page without pending: %d", rec.Code)
	}
}

func TestMFALoginWithRecoveryCode(t *testing.T) {
	e := newMFAEnv(t)
	pending := e.password("/manager/teams")
	rec := e.post("/manager/login/mfa/recovery", url.Values{"code": {strings.ToUpper(e.recover[0])}}, "", pending)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "9 recovery codes left") || !strings.Contains(body, `href="/manager/teams"`) {
		t.Fatalf("recovery: %d\n%s", rec.Code, body)
	}
	if cookieNamed(rec, auth.CookieName) == nil {
		t.Fatal("no session cookie after recovery code")
	}
	pending = e.password("/manager")
	rec = e.post("/manager/login/mfa/recovery", url.Values{"code": {e.recover[0]}}, "", pending)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Invalid code") {
		t.Fatalf("reused recovery code: %d", rec.Code)
	}
}

func TestMFALockoutAnswers429(t *testing.T) {
	e := newMFAEnv(t)
	pending := e.password("/manager")
	for i := 1; i <= 4; i++ {
		if rec := e.post("/manager/login/mfa/recovery", url.Values{"code": {"aaaaa-aaaaa"}}, "", pending); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, rec.Code)
		}
	}
	rec := e.post("/manager/login/mfa/recovery", url.Values{"code": {"aaaaa-aaaaa"}}, "", pending)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("5th attempt: %d", rec.Code)
	}
	// Re-entering the password does not lift the lock, and the right code is refused.
	pending = e.password("/manager")
	rec = e.post("/manager/login/mfa/totp", url.Values{"code": {e.code()}}, "", pending)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "Too many attempts, try again in 15 minutes") || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("locked: %d %q\n%s", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
	}
	if cookieNamed(rec, auth.CookieName) != nil {
		t.Fatal("session issued while locked")
	}
}

func TestMFAStepBodyCapped(t *testing.T) {
	e := newMFAEnv(t)
	pending := e.password("/manager")
	rec := e.post("/manager/login/mfa/totp", url.Values{"code": {strings.Repeat("1", 64<<10)}}, "", pending)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized: %d", rec.Code)
	}
}

// With AWT_BASE_URL set, only its origin passes the second-step Origin check.
func TestMFAOriginFromBaseURL(t *testing.T) {
	deps := newDeps(t)
	deps.Cfg.BaseURL = "https://Manager.Example.org/"
	h := web.Routes(deps)
	for origin, want := range map[string]int{
		"https://manager.example.org": http.StatusUnauthorized, // passes the check, no pending login
		"http://localhost:0":          http.StatusForbidden,
		"null":                        http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodPost, "/manager/login/mfa/recovery", strings.NewReader("code=x"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Origin %s: %d, want %d", origin, rec.Code, want)
		}
	}
}
