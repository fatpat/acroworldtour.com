package web_test

import (
	"bytes"
	"context"
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
	r.User("GET /admin/me", http.HandlerFunc(ok))
	r.Admin("GET /admin/users", http.HandlerFunc(ok))
	r.Admin("POST /admin/users", http.HandlerFunc(ok))
	r.Scorer("GET /admin/competitions/{code}/flights", func(r *http.Request) string { return r.PathValue("code") }, http.HandlerFunc(ok))
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
		if p != "/admin" && !strings.HasPrefix(p, "/admin/") {
			continue
		}
		adminRoutes++
		if p == "/admin/login" {
			continue // the login form and its POST are the only anonymous admin URLs
		}
		if rt.Guard == web.GuardPublic {
			t.Errorf("%s is under /admin but public", rt.Pattern)
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
		r.Public("GET /admin/secret", http.HandlerFunc(ok))
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

	rec := get(h, "/admin")
	if rec.Code != 303 || !strings.HasPrefix(rec.Header().Get("Location"), "/admin/login?next=") {
		t.Fatalf("anonymous /admin: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = get(h, "/admin/login?next=%2Fadmin%2Fpilots")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `action="/admin/login"`) || !strings.Contains(rec.Body.String(), `value="/admin/pilots"`) {
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

	rec = post("/admin/login", url.Values{"email": {"boss@example.org"}, "password": {"wrong"}})
	if rec.Code != 401 || !strings.Contains(rec.Body.String(), "invalid email or password") || !strings.Contains(rec.Body.String(), "<form") {
		t.Fatalf("bad login: %d %q", rec.Code, rec.Body.String())
	}

	rec = post("/admin/login", url.Values{"email": {"boss@example.org"}, "password": {"s3cret-passw0rd"}, "next": {"/admin"}})
	if rec.Code != 303 || rec.Header().Get("Location") != "/admin" {
		t.Fatalf("login: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v", cookies)
	}

	rec = get(h, "/admin", cookies[0])
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("/admin: %d", rec.Code)
	}
	m := regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no csrf meta in:\n%s", body)
	}
	token := m[1]
	if !strings.Contains(body, "hx-headers=") || !strings.Contains(body, "X-CSRF-Token") {
		t.Error("htmx hx-headers wiring missing")
	}
	for _, want := range []string{">Pilots<", ">Teams<", ">Judges<", ">Tricks<", ">Seasons<", ">Competitions<", ">Files<", ">Users<", ">Content<", "Boss", `action="/admin/logout"`} {
		if !strings.Contains(body, want) {
			t.Errorf("admin layout missing %q", want)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("admin Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}

	if rec = post("/admin/logout", nil, cookies[0]); rec.Code != 403 {
		t.Fatalf("logout without CSRF: %d", rec.Code)
	}
	rec = post("/admin/logout", url.Values{"_csrf": {token}}, cookies[0])
	if rec.Code != 303 || rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("logout: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec = get(h, "/admin", cookies[0]); rec.Code != 303 {
		t.Fatalf("session still valid after logout: %d", rec.Code)
	}
}
