// Package web holds the HTTP server and handlers.
package web

import (
	"net/http"
	"time"

	"awt/internal/config"
)

// Deps carries the services handlers need. Empty until later tasks add them.
type Deps struct{}

// NewServer builds the HTTP server with all routes registered.
func NewServer(cfg config.Config, deps Deps) *http.Server {
	_ = deps
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
}
