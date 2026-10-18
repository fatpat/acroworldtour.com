// Package web holds the HTTP server, routing, middleware and error pages.
package web

import (
	"net/http"
	"time"
)

// NewServer builds the HTTP server on deps.Cfg.Addr with every route
// registered (built-ins plus the feature modules). When Shutdown starts it
// closes deps.Draining, which ends long-lived SSE streams promptly while
// ordinary in-flight requests run to completion.
func NewServer(deps Deps, mods ...Module) *http.Server {
	drain := make(chan struct{})
	deps.Draining = drain
	srv := &http.Server{
		Addr:              deps.Cfg.Addr,
		Handler:           Routes(deps, mods...),
		ReadHeaderTimeout: 10 * time.Second,
	}
	srv.RegisterOnShutdown(func() { close(drain) })
	return srv
}
