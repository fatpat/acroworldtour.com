package web

import (
	"bytes"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"awt/internal/store/gen"
	"awt/internal/web/layout"
	"awt/web/static"

	"github.com/a-h/templ"
)

// render writes a templ component with the given status. It renders to a
// buffer first so a template error cannot leave a half-written page.
func render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	var buf bytes.Buffer
	if err := c.Render(r.Context(), &buf); err != nil {
		slog.Error("render", "path", r.URL.Path, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (d Deps) serverError(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	render(w, r, http.StatusInternalServerError, layout.ErrorPage(500,
		"Something went wrong", "An unexpected error occurred. Please try again in a moment."))
}

// notFound is the catch-all route. The redirects table is consulted before
// answering 404 (legacy URLs, spec 07 §8).
func (d Deps) notFound(w http.ResponseWriter, r *http.Request) {
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && d.DB != nil {
		if to, permanent, ok := d.lookupRedirect(r); ok {
			code := http.StatusFound
			if permanent {
				code = http.StatusMovedPermanently
			}
			http.Redirect(w, r, to, code)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	render(w, r, http.StatusNotFound, layout.ErrorPage(404,
		"Page not found", "The page you are looking for does not exist or has moved."))
}

func (d Deps) lookupRedirect(r *http.Request) (to string, permanent, ok bool) {
	q := gen.New(d.DB.Read)
	p := r.URL.Path
	for _, cand := range []string{p, strings.TrimSuffix(p, "/")} {
		if cand == "" {
			continue
		}
		row, err := q.GetRedirect(r.Context(), cand)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			slog.Error("redirect lookup", "path", p, "err", err)
			return "", false, false
		}
		// The table is admin-edited, but an unsafe target must never be emitted.
		if !safeRedirectTarget(row.ToUrl) {
			slog.Warn("redirect with unsafe target ignored", "from", cand)
			continue
		}
		return row.ToUrl, row.Permanent, true
	}
	return "", false, false
}

func serveFavicon(w http.ResponseWriter, r *http.Request) {
	b, err := static.Open("img/favicon.ico")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(b)
}

func adminHome(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, layout.AdminHome())
}

func (d Deps) loginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if _, err := d.Auth.UserFromRequest(r); err == nil {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	render(w, r, http.StatusOK, layout.LoginPage(r.URL.Query().Get("next"), ""))
}

// capture buffers a handler's response so failures can be re-rendered in the
// page layout while successes (cookie + redirect) pass through untouched.
type capture struct {
	h      http.Header
	status int
	body   bytes.Buffer
}

func (c *capture) Header() http.Header         { return c.h }
func (c *capture) WriteHeader(code int)        { c.status = code }
func (c *capture) Write(b []byte) (int, error) { return c.body.Write(b) }

func (d Deps) loginSubmit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	c := &capture{h: http.Header{}, status: http.StatusOK}
	d.Auth.HandleLogin(c, r)
	if c.status >= 300 && c.status < 400 {
		for k, v := range c.h {
			w.Header()[k] = v
		}
		w.WriteHeader(c.status)
		return
	}
	if v := c.h.Get("Retry-After"); v != "" {
		w.Header().Set("Retry-After", v)
	}
	msg := strings.TrimSpace(c.body.String())
	if c.status >= 500 {
		msg = "Something went wrong, please try again."
	}
	render(w, r, c.status, layout.LoginPage(r.PostFormValue("next"), msg))
}

func (d Deps) logout(w http.ResponseWriter, r *http.Request) {
	if err := d.Auth.Logout(w, r); err != nil {
		slog.Error("logout", "err", err)
	}
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// safeRedirectTarget accepts a local absolute path or an http(s) URL. It
// refuses protocol-relative ("//host", "/\\host") targets and control
// characters, which browsers normalise into another origin.
func safeRedirectTarget(to string) bool {
	for _, c := range to {
		if c < 0x20 || c == 0x7f || c == '\\' {
			return false
		}
	}
	switch {
	case strings.HasPrefix(to, "//"):
		return false
	case strings.HasPrefix(to, "/"):
		return true
	}
	return strings.HasPrefix(to, "https://") || strings.HasPrefix(to, "http://")
}
