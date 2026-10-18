// Package public holds the public site pages.
package public

import "awt/internal/web"

// Module registers the public pages.
type Module struct{}

// Register implements web.Module.
func (Module) Register(r *web.Router) {
	registerSimulator(r)
	registerResults(r)
	registerFiles(r)
}
