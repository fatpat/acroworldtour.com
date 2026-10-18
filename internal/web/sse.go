package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"awt/internal/service"
)

// Defaults of the live stream: Cloudflare closes idle connections after
// about 100 s, so a comment ping goes out every 30 s (spec 08 §6.4).
const (
	DefaultSSEPing        = 30 * time.Second
	DefaultSSEMaxConns    = 2000
	sseCompetitionTopicPf = "competition:"
)

// SSE streams the events of one competition to public pages (htmx SSE
// extension). It holds no database connection while a client is connected:
// the competition is looked up once, before the stream starts.
type SSE struct {
	Bus *service.Bus
	// Exists reports whether code names a published competition.
	Exists func(ctx context.Context, code string) (bool, error)
	// Ping is the interval between `: ping` comments; zero means the default.
	Ping time.Duration
	// Drain, when closed, ends every stream (graceful shutdown); nil never fires.
	Drain <-chan struct{}
	// MaxConns caps concurrent streams per process; zero means the default.
	MaxConns int64

	active atomic.Int64
}

// Active is the number of streams currently open.
func (s *SSE) Active() int64 { return s.active.Load() }

// ServeHTTP implements http.Handler for GET /live/competitions/{code}.
func (s *SSE) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	rc := http.NewResponseController(w)
	if s.Bus == nil || s.Exists == nil {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "live updates unavailable", http.StatusServiceUnavailable)
		return
	}
	limit := s.MaxConns
	if limit <= 0 {
		limit = DefaultSSEMaxConns
	}
	if s.active.Add(1) > limit {
		s.active.Add(-1)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "30")
		http.Error(w, "too many live connections", http.StatusServiceUnavailable)
		return
	}
	defer s.active.Add(-1)

	ok, err := s.Exists(r.Context(), code)
	if err != nil {
		slog.Error("live: lookup", "code", code, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}

	// A stream outlives any server-wide write timeout; make sure none applies
	// (the error is ignored: not every ResponseWriter supports deadlines).
	_ = rc.SetWriteDeadline(time.Time{})

	// Events are signals to reload the current state, so one dropped by the
	// Bus for a slow client (full buffer) is harmless: the next event, or the
	// next page load, catches up.
	events, unsubscribe := s.Bus.Subscribe(sseCompetitionTopicPf + code)
	defer unsubscribe()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	h.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	// Tell the client how long to wait before reconnecting, then flush the
	// headers so the connection is established right away.
	if _, err := fmt.Fprint(w, "retry: 3000\n\n"); err != nil || rc.Flush() != nil {
		return
	}

	ping := s.Ping
	if ping <= 0 {
		ping = DefaultSSEPing
	}
	ticker := time.NewTicker(ping)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.Drain:
			return
		case e, open := <-events:
			if !open {
				return
			}
			if _, err := fmt.Fprintf(w, "event: update\ndata: %s\n\n", e.Kind); err != nil {
				return
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
		}
		if rc.Flush() != nil {
			return
		}
	}
}
