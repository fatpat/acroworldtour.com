package web

import (
	"net/http"
	"strings"
	"time"

	"awt/internal/auth"
	"awt/internal/blob"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/service"
	"awt/internal/store"
	"awt/web/static"
)

// Deps carries what handlers need. Feature modules receive it in Register.
type Deps struct {
	Cfg    config.Config
	Auth   *auth.Auth
	DB     *store.DB
	Purger cache.Purger
	// Blobs holds uploaded files (served by GET /files/{id}).
	Blobs blob.Store
	// Service holds the back-office use cases.
	Service *service.Service
	// Bus carries the live-update events (SSE stream, CDN purge); nil
	// disables the live routes' event source.
	Bus *service.Bus
	// Draining is closed when the server starts shutting down (set by
	// NewServer); long-lived streams end on it.
	Draining <-chan struct{}
}

// Guard is the access level a route requires.
type Guard string

// Guard levels. GuardScorer admits admins and scorers assigned to the
// competition the route names.
const (
	GuardPublic Guard = "public"
	GuardUser   Guard = "user"
	GuardAdmin  Guard = "admin"
	GuardScorer Guard = "scorer"
)

// Route is one entry of the route table.
type Route struct {
	Pattern string // as given to ServeMux, e.g. "GET /manager/pilots"
	Guard   Guard
}

// Path returns the URL path part of the pattern.
func (r Route) Path() string {
	p := r.Pattern
	if _, rest, ok := strings.Cut(p, " "); ok {
		p = rest
	}
	if i := strings.IndexByte(p, '/'); i > 0 { // host-qualified pattern
		p = p[i:]
	}
	return p
}

// Module is a feature package's hook: it registers its handlers on the Router.
// cmd/awt composes the modules, so feature packages never import each other.
type Module interface {
	Register(r *Router)
}

// ModuleFunc adapts a function to Module.
type ModuleFunc func(r *Router)

// Register implements Module.
func (f ModuleFunc) Register(r *Router) { f(r) }

// Router wraps ServeMux and records every route with its guard, so tests can
// walk the table (CLAUDE.md: a test fails if an admin route is unguarded).
type Router struct {
	mux    *http.ServeMux
	deps   Deps
	routes []Route
}

// Deps returns the dependencies given to Routes.
func (r *Router) Deps() Deps { return r.deps }

// Table returns a copy of the recorded routes.
func (r *Router) Table() []Route { return append([]Route(nil), r.routes...) }

func (r *Router) add(pattern string, g Guard, h http.Handler) {
	r.routes = append(r.routes, Route{Pattern: pattern, Guard: g})
	r.mux.Handle(pattern, h)
}

// Public registers an unauthenticated route. Anything under /manager/ is
// refused (panics at startup: a programming error) except the login endpoints.
func (r *Router) Public(pattern string, h http.Handler) {
	if p := (Route{Pattern: pattern}).Path(); isAdminPath(p) && !isLoginPath(p) {
		panic("web: public route under /manager: " + pattern)
	}
	r.add(pattern, GuardPublic, h)
}

// PublicCached registers a public route with CDN cache headers.
func (r *Router) PublicCached(pattern string, h http.Handler, maxAge, swr time.Duration) {
	r.Public(pattern, cache.Public(maxAge, swr)(h))
}

// User registers a route for any logged-in user (CSRF checked on writes).
func (r *Router) User(pattern string, h http.Handler) {
	r.add(pattern, GuardUser, cache.NoStore(r.deps.Auth.RequireUser(r.deps.Auth.CSRF(h))))
}

// UserMax is User with the request body capped at max bytes before any
// parsing, CSRF check included.
func (r *Router) UserMax(pattern string, max int64, h http.Handler) {
	r.add(pattern, GuardUser, cache.NoStore(limitBody(max, r.deps.Auth.RequireUser(r.deps.Auth.CSRF(h)))))
}

// Admin registers a route for admins only (CSRF checked on writes).
func (r *Router) Admin(pattern string, h http.Handler) {
	r.add(pattern, GuardAdmin, cache.NoStore(r.deps.Auth.RequireAdmin(r.deps.Auth.CSRF(h))))
}

// AdminMax is Admin with the request body capped at max bytes before any
// parsing, CSRF check included (forms 64 KB, uploads 12 MB).
func (r *Router) AdminMax(pattern string, max int64, h http.Handler) {
	r.add(pattern, GuardAdmin, cache.NoStore(limitBody(max, r.deps.Auth.RequireAdmin(r.deps.Auth.CSRF(h)))))
}

func limitBody(max int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > max {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, max)
		next.ServeHTTP(w, r)
	})
}

// has reports whether a route with exactly this pattern is registered.
func (r *Router) has(pattern string) bool {
	for _, rt := range r.routes {
		if rt.Pattern == pattern {
			return true
		}
	}
	return false
}

// Scorer registers a route for admins and for scorers assigned to the
// competition that compFrom extracts from the request (id or code).
func (r *Router) Scorer(pattern string, compFrom func(*http.Request) string, h http.Handler) {
	r.add(pattern, GuardScorer, cache.NoStore(r.deps.Auth.RequireScorer(compFrom)(r.deps.Auth.CSRF(h))))
}

// ScorerMax is Scorer with the request body capped at max bytes before any
// parsing, CSRF check included.
func (r *Router) ScorerMax(pattern string, max int64, compFrom func(*http.Request) string, h http.Handler) {
	r.add(pattern, GuardScorer, cache.NoStore(limitBody(max, r.deps.Auth.RequireScorer(compFrom)(r.deps.Auth.CSRF(h)))))
}

// loginMax caps the anonymous login form (email, password, next).
const loginMax = 16 << 10

func isAdminPath(p string) bool { return p == "/manager" || strings.HasPrefix(p, "/manager/") }

// isLoginPath: the password step and the second step run before any session.
func isLoginPath(p string) bool {
	return p == "/manager/login" || p == "/manager/login/mfa" || strings.HasPrefix(p, "/manager/login/mfa/")
}

// newRouter builds the router with the built-in routes and the modules.
func newRouter(deps Deps, mods ...Module) *Router {
	r := &Router{mux: http.NewServeMux(), deps: deps}
	r.Public("GET /healthz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	}))
	r.Public("GET "+static.Prefix, static.Handler())
	r.Public("GET /favicon.ico", http.HandlerFunc(serveFavicon))

	r.Public("GET /manager/login", http.HandlerFunc(deps.loginPage))
	r.Public("POST /manager/login", limitBody(loginMax, http.HandlerFunc(deps.loginSubmit)))
	r.Public("GET /manager/login/mfa", http.HandlerFunc(deps.mfaPage))
	r.Public("POST /manager/login/mfa/totp", deps.checkOrigin(limitBody(loginMax, deps.mfaTOTP())))
	r.Public("POST /manager/login/mfa/recovery", deps.checkOrigin(limitBody(loginMax, deps.mfaRecovery())))
	r.Public("POST /manager/login/mfa/webauthn/begin", deps.checkOrigin(limitBody(loginMax, http.HandlerFunc(deps.mfaKeyBegin))))
	r.Public("POST /manager/login/mfa/webauthn/finish", deps.checkOrigin(limitBody(loginMax, http.HandlerFunc(deps.mfaKeyFinish))))
	r.User("POST /manager/logout", http.HandlerFunc(deps.logout))

	for _, m := range mods {
		m.Register(r)
	}
	// The dashboard module may own GET /manager; otherwise the placeholder.
	if !r.has("GET /manager") {
		r.User("GET /manager", http.HandlerFunc(adminHome))
	}
	// Anything unmatched: redirects table first, then the 404 page.
	r.Public("/", http.HandlerFunc(deps.notFound))
	return r
}

// RouteTable returns the full route table (built-ins and modules), for the
// guard test of every module.
func RouteTable(deps Deps, mods ...Module) []Route { return newRouter(deps, mods...).Table() }

// Routes builds the application handler: routes, then the middleware chain
// (recovery, logging, security headers).
func Routes(deps Deps, mods ...Module) http.Handler {
	r := newRouter(deps, mods...)
	var h http.Handler = r.mux
	h = deps.securityHeaders(h)
	h = deps.recoverPanic(h)
	h = deps.logRequests(h) // outermost: it logs the 500 the recovery wrote
	return h
}
