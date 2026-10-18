package admin

import (
	"net/http"
	"slices"
	"time"

	"awt/internal/auth"
	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

type dashboardView struct {
	Admin                                        bool
	Pilots, Teams, Judges, Seasons, Competitions int
	Tricks, Year                                 int
	Open                                         []service.Competition
}

// registerDashboard owns GET /admin: counts for admins, the open
// competitions for everybody (spec 05 §4 "Dashboard").
func (h *handlers) registerDashboard(r *web.Router) {
	r.User("GET /admin", http.HandlerFunc(h.dashboard))
}

func (h *handlers) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, _ := auth.SessionFromContext(ctx)
	v := dashboardView{Admin: sess.User.Role == auth.RoleAdmin, Year: time.Now().UTC().Year()}
	comps, err := h.svc.ListCompetitions(ctx)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	v.Competitions = len(comps)
	var mine []string
	if !v.Admin { // scorers only see the competitions they are assigned to
		if mine, err = h.auth.UserCompetitions(ctx, sess.User.ID); err != nil {
			h.fail(w, r, err, nil)
			return
		}
	}
	for _, c := range comps {
		if c.State == domain.CompetitionOpen && (v.Admin || slices.Contains(mine, c.ID)) {
			v.Open = append(v.Open, c)
		}
	}
	if v.Admin {
		pilots, e1 := h.svc.ListPilots(ctx)
		teams, e2 := h.svc.ListTeams(ctx)
		judges, e3 := h.svc.ListJudges(ctx)
		seasons, e4 := h.svc.ListSeasons(ctx)
		tricks, e5 := h.svc.ListTricks(ctx, v.Year)
		for _, e := range []error{e1, e2, e3, e4, e5} {
			if e != nil {
				h.fail(w, r, e, nil)
				return
			}
		}
		v.Pilots, v.Teams, v.Judges, v.Seasons, v.Tricks = len(pilots), len(teams), len(judges), len(seasons), len(tricks)
	}
	render(w, r, http.StatusOK, dashboardPage(v))
}
