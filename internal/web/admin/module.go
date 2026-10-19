// Package admin holds the back-office screens: reference data (pilots,
// teams, judges, tricks, seasons, files), users and the dashboard
// (spec 05 §5). Pages are server-rendered forms that work without
// JavaScript (POST then 303 redirect); htmx only swaps rows in place.
package admin

import (
	"awt/internal/auth"
	"awt/internal/service"
	"awt/internal/web"
)

// Module registers the reference data screens.
type Module struct{}

type handlers struct {
	svc  *service.Service
	auth *auth.Auth
}

// Register implements web.Module. Reference data is admin-only (decision
// 0009); only the dashboard and one's own account page are open to every
// logged-in user.
func (Module) Register(r *web.Router) {
	d := r.Deps()
	h := &handlers{svc: d.Service, auth: d.Auth}
	h.registerDashboard(r)
	h.registerPilots(r)
	h.registerTeams(r)
	h.registerJudges(r)
	h.registerTricks(r)
	h.registerSeasons(r)
	h.registerFiles(r)
	h.registerCompetitions(r)
	h.registerJudging(r)
	h.registerContent(r)
	h.registerUsers(r)
	h.registerAccount(r)
}
