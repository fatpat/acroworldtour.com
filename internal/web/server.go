// Package web holds the HTTP server, routing, middleware and error pages.
package web

import (
	"context"
	"net"
	"net/http"
	"time"
)

// NewServer builds the HTTP server on deps.Cfg.Addr with every route
// registered (built-ins plus the feature modules). Request contexts derive
// from a base context cancelled when Shutdown starts, so long-lived SSE
// streams end promptly instead of blocking the graceful shutdown.
func NewServer(deps Deps, mods ...Module) *http.Server {
	base, cancel := context.WithCancel(context.Background())
	srv := &http.Server{
		Addr:              deps.Cfg.Addr,
		Handler:           Routes(deps, mods...),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return base },
	}
	srv.RegisterOnShutdown(cancel)
	return srv
}
