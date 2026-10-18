package public

import (
	"database/sql"
	"net/http"
)

type teamsView struct {
	Teams []participant
}

// teams lists the synchro teams with their pilots (spec 06 §4.8).
func (p *results) teams(w http.ResponseWriter, r *http.Request) {
	teams, err := p.q.ListTeams(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}
	members, err := p.teamMembers(r)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var v teamsView
	for _, t := range teams {
		v.Teams = append(v.Teams, participant{TeamID: t.ID, Name: t.Name, Members: members[t.ID]})
	}
	renderPage(w, r, TeamsPage(p.meta("Teams", "Acro World Tour synchro teams and their pilots.", "/teams", ""), v))
}

type teamView struct {
	T       participant
	Comps   []compResult
	Seasons []seasonResult
}

// team is a team's pilots and results history (spec 06 §10.3: the legacy
// team page had none). A deleted team keeps its page: past results link to it.
func (p *results) team(w http.ResponseWriter, r *http.Request) {
	t, err := p.q.GetTeam(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	href := teamHref(t.ID, t.Name)
	if r.URL.Path != href {
		http.Redirect(w, r, href, http.StatusMovedPermanently)
		return
	}
	members, err := p.teamMembers(r)
	if err != nil {
		serverError(w, r, err)
		return
	}
	v := teamView{T: participant{TeamID: t.ID, Name: t.Name, Members: members[t.ID]}}
	if v.Comps, v.Seasons, err = p.history(r, sql.NullInt64{}, sql.NullString{String: t.ID, Valid: true}); err != nil {
		serverError(w, r, err)
		return
	}
	image := ""
	if len(v.T.Members) > 0 {
		image = v.T.Members[0].Photo
	}
	renderPage(w, r, TeamPage(p.meta(t.Name, t.Name+": Acro World Tour synchro team and results.", href, image), v))
}
