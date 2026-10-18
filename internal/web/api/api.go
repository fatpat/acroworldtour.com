// Package api serves the read-only public JSON API under /api/v1. It reads
// stored rows only (competitions must be published and not deleted, runs list
// published flights only) and never exposes gender or internal state.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"awt/internal/store"
	"awt/internal/web"
)

// Cache lifetimes: live data (competitions, runs, seasons, one pilot's
// results) is short, reference lists are cached for an hour.
const (
	liveMaxAge = 30 * time.Second
	liveSWR    = 60 * time.Second
	refMaxAge  = time.Hour
	refSWR     = 60 * time.Second

	prefix = "/api/v1"
)

// Module registers the API routes.
type Module struct{}

// Register implements web.Module.
func (Module) Register(r *web.Router) {
	s := &server{db: r.Deps().DB}
	live := func(pattern string, h http.HandlerFunc) {
		r.PublicCached("GET "+prefix+pattern, s.wrap(h), liveMaxAge, liveSWR)
	}
	ref := func(pattern string, h http.HandlerFunc) {
		r.PublicCached("GET "+prefix+pattern, s.wrap(h), refMaxAge, refSWR)
	}
	live("/competitions", s.competitions)
	live("/competitions/{code}", s.competition)
	live("/competitions/{code}/runs/{i}", s.run)
	live("/seasons", s.seasons)
	live("/seasons/{code}", s.season)
	live("/pilots/{civlid}", s.pilot)
	ref("/pilots", s.pilots)
	ref("/teams", s.teams)
	ref("/judges", s.judges)
	ref("/tricks", s.tricks)
	// Everything else under the prefix: CORS preflight, JSON 404 and 405.
	r.Public(prefix+"/", s.wrap(s.fallback))
}

type server struct{ db *store.DB }

var errNotFound = errors.New("not found")

// wrap adds CORS and the JSON content type.
func (s *server) wrap(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("Access-Control-Allow-Origin", "*")
		hd.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		hd.Set("Content-Type", "application/json; charset=utf-8")
		h(w, r)
	})
}

func (s *server) fallback(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.WriteHeader(http.StatusNoContent)
	case http.MethodGet, http.MethodHead:
		fail(w, errNotFound)
	default:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":"method not allowed"}` + "\n"))
	}
}

type badRequest string

func (b badRequest) Error() string { return string(b) }

// fail writes a JSON error. Internal errors are logged, never leaked.
func fail(w http.ResponseWriter, err error) {
	w.Header().Set("Cache-Control", "no-store")
	var br badRequest
	switch {
	case errors.Is(err, errNotFound), errors.Is(err, sql.ErrNoRows):
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}` + "\n"))
	case errors.As(err, &br):
		w.WriteHeader(http.StatusBadRequest)
		b, _ := json.Marshal(map[string]string{"error": string(br)})
		_, _ = w.Write(append(b, '\n'))
	default:
		slog.Error("api", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}` + "\n"))
	}
}

func send(w http.ResponseWriter, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fail(w, err)
		return
	}
	_, _ = w.Write(append(b, '\n'))
}

func sendList[T any](w http.ResponseWriter, items []T) {
	if items == nil {
		items = []T{}
	}
	send(w, map[string]any{"items": items})
}
