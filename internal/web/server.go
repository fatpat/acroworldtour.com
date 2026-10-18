// Package web holds the HTTP server, routing, middleware and error pages.
package web

import (
	"net/http"
	"time"
)

// NewServer builds the HTTP server on deps.Cfg.Addr with every route
// registered (built-ins plus the feature modules).
func NewServer(deps Deps, mods ...Module) *http.Server {
	return &http.Server{
		Addr:              deps.Cfg.Addr,
		Handler:           Routes(deps, mods...),
		ReadHeaderTimeout: 10 * time.Second,
	}
}
