package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"awt/internal/store/gen"
)

type ctxKey struct{}

// SessionFromContext returns the session stored by RequireUser.
func SessionFromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(ctxKey{}).(Session)
	return s, ok
}

// CSRFToken returns the session's CSRF token, for templates (the `_csrf` form
// field and the <meta> htmx reads).
func CSRFToken(ctx context.Context) string {
	s, _ := SessionFromContext(ctx)
	return s.CSRF
}

func loginURL(r *http.Request) string {
	u := "/manager/login"
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		u += "?next=" + url.QueryEscape(r.URL.RequestURI())
	}
	return u
}

// RequireUser lets any enabled, logged-in user through.
func (a *Auth) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, err := a.UserFromRequest(r)
		if err != nil {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", loginURL(r))
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, loginURL(r), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, s)))
	})
}

// RequireAdmin lets admins through; other users get 403.
func (a *Auth) RequireAdmin(next http.Handler) http.Handler {
	return a.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s, _ := SessionFromContext(r.Context()); s.User.Role != RoleAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// RequireScorer lets admins through, and scorers assigned to the competition
// that compIDFrom extracts from the request.
func (a *Auth) RequireScorer(compIDFrom func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return a.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, _ := SessionFromContext(r.Context())
			if s.User.Role != RoleAdmin {
				ref := compIDFrom(r)
				assigned := false
				if s.User.Role == RoleScorer && ref != "" {
					q := gen.New(a.db.Read)
					id, err := resolveCompetition(r.Context(), q, ref)
					if err == nil && id != "" {
						assigned, err = q.IsUserAssigned(r.Context(), gen.IsUserAssignedParams{UserID: s.User.ID, CompetitionID: id})
					}
					if err != nil {
						http.Error(w, "internal error", http.StatusInternalServerError)
						return
					}
				}
				if !assigned {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// resolveCompetition maps a competition id or code to its id ("" when unknown).
func resolveCompetition(ctx context.Context, q *gen.Queries, ref string) (string, error) {
	c, err := q.GetCompetition(ctx, ref)
	if errors.Is(err, sql.ErrNoRows) {
		c, err = q.GetCompetitionByCode(ctx, ref)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return c.ID, err
}

// safeNext only accepts local absolute paths, to avoid open redirects.
func safeNext(n string) string {
	const fallback = "/manager"
	if !strings.HasPrefix(n, "/") || strings.HasPrefix(n, "//") || strings.HasPrefix(n, "/\\") {
		return fallback
	}
	u, err := url.Parse(n)
	if err != nil || u.Scheme != "" || u.Host != "" || strings.HasPrefix(u.Path, "//") {
		return fallback
	}
	// Check both the raw and the decoded form for control characters and
	// backslashes (browsers strip tabs and newlines, and treat \ as /).
	for _, v := range []string{n, u.Path} {
		for _, c := range v {
			if c < 0x20 || c == 0x7f || c == '\\' {
				return fallback
			}
		}
	}
	return n
}

// HandleLogin handles the login form POST (fields email, password, next). The
// failure limiter answers 429 once an email+IP pair failed 5 times in 15 min.
func (a *Auth) HandleLogin(w http.ResponseWriter, r *http.Request) {
	email, password := r.PostFormValue("email"), r.PostFormValue("password")
	key := limiterKey(email, a.ClientIP(r))
	if !a.limiter.Attempt(key) {
		w.Header().Set("Retry-After", "900")
		http.Error(w, "too many failed attempts, try again later", http.StatusTooManyRequests)
		return
	}
	s, err := a.Login(r.Context(), email, password)
	if errors.Is(err, ErrInvalidCredentials) {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	a.limiter.Reset(key)
	a.SetCookie(w, s)
	http.Redirect(w, r, safeNext(r.PostFormValue("next")), http.StatusSeeOther)
}
