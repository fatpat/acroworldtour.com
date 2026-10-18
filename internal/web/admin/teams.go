package admin

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

type teamForm struct {
	ID     string // empty when creating
	Name   string
	Pilot1 string
	Pilot2 string
}

type teamsView struct {
	Teams []service.Team
	Names map[int]string // pilot name by CIVL id
	Flash string
}

type teamFormView struct {
	Form   teamForm
	Pilots []service.Pilot
	Errors formErrors
}

func (h *handlers) registerTeams(r *web.Router) {
	get(r, "/admin/teams", h.teamsList)
	get(r, "/admin/teams/new", h.teamNew)
	post(r, "/admin/teams", h.teamCreate)
	get(r, "/admin/teams/{id}/edit", h.teamEdit)
	post(r, "/admin/teams/{id}", h.teamUpdate)
	get(r, "/admin/teams/{id}/delete", h.teamDeleteConfirm)
	post(r, "/admin/teams/{id}/delete", h.teamDelete)
}

func (h *handlers) pilotNames(w http.ResponseWriter, r *http.Request) ([]service.Pilot, map[int]string, bool) {
	pilots, err := h.svc.ListPilots(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return nil, nil, false
	}
	names := make(map[int]string, len(pilots))
	for _, p := range pilots {
		names[p.CivlID] = p.Name
	}
	return pilots, names, true
}

func (h *handlers) teamsList(w http.ResponseWriter, r *http.Request) {
	teams, err := h.svc.ListTeams(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	_, names, ok := h.pilotNames(w, r)
	if !ok {
		return
	}
	render(w, r, http.StatusOK, teamsPage(teamsView{Teams: teams, Names: names, Flash: flash(r)}))
}

func (h *handlers) teamFormPage(w http.ResponseWriter, r *http.Request, status int, f teamForm, fe formErrors) {
	pilots, _, ok := h.pilotNames(w, r)
	if !ok {
		return
	}
	render(w, r, status, teamFormPage(teamFormView{Form: f, Pilots: pilots, Errors: fe}))
}

func (h *handlers) teamNew(w http.ResponseWriter, r *http.Request) {
	h.teamFormPage(w, r, http.StatusOK, teamForm{}, formErrors{})
}

var trailingID = regexp.MustCompile(`\((\d+)\)\s*$`)

// parsePilotRef reads a pilot picker value: "Name (12345)" from the datalist
// or a bare CIVL id.
func parsePilotRef(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if m := trailingID.FindStringSubmatch(raw); m != nil {
		raw = m[1]
	}
	n, err := strconv.Atoi(raw)
	return n, err == nil && n > 0
}

func readTeamForm(r *http.Request) (teamForm, service.TeamInput, []domain.FieldError) {
	f := teamForm{Name: r.PostFormValue("name"), Pilot1: r.PostFormValue("pilot1"), Pilot2: r.PostFormValue("pilot2")}
	in := service.TeamInput{Name: strings.TrimSpace(f.Name)}
	var errs []domain.FieldError
	for i, raw := range []string{f.Pilot1, f.Pilot2} {
		id, ok := parsePilotRef(raw)
		if !ok {
			errs = append(errs, domain.FieldError{Field: "pilot" + strconv.Itoa(i+1), Msg: "pick a pilot from the list or type a CIVL id"})
			continue
		}
		in.Pilots = append(in.Pilots, id)
	}
	return f, in, errs
}

// pilotFieldErrors maps the service's "pilots" field onto the two pickers.
func pilotFieldErrors(fe formErrors) formErrors {
	if m := fe.Fields["pilots"]; m != "" {
		fe.Fields["pilot1"], fe.Fields["pilot2"] = m, m
	}
	return fe
}

func (h *handlers) teamSave(w http.ResponseWriter, r *http.Request, id string) {
	f, in, errs := readTeamForm(r)
	f.ID = id
	redo := func(status int, fe formErrors) { h.teamFormPage(w, r, status, f, pilotFieldErrors(fe)) }
	if len(errs) > 0 {
		redo(http.StatusUnprocessableEntity, validationErrors(errs))
		return
	}
	var err error
	if id == "" {
		_, err = h.svc.CreateTeam(r.Context(), in)
	} else {
		_, err = h.svc.UpdateTeam(r.Context(), id, in)
	}
	if err != nil {
		h.fail(w, r, err, redo)
		return
	}
	flashKey := "created"
	if id != "" {
		flashKey = "saved"
	}
	redirect(w, r, "/admin/teams", flashKey)
}

func (h *handlers) teamCreate(w http.ResponseWriter, r *http.Request) { h.teamSave(w, r, "") }
func (h *handlers) teamUpdate(w http.ResponseWriter, r *http.Request) { h.teamSave(w, r, pathID(r)) }

func (h *handlers) teamEdit(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTeam(r.Context(), pathID(r))
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	_, names, ok := h.pilotNames(w, r)
	if !ok {
		return
	}
	f := teamForm{ID: t.ID, Name: t.Name}
	if len(t.Pilots) == 2 {
		f.Pilot1, f.Pilot2 = pilotLabel(names, t.Pilots[0]), pilotLabel(names, t.Pilots[1])
	}
	h.teamFormPage(w, r, http.StatusOK, f, formErrors{})
}

func pilotLabel(names map[int]string, id int) string {
	return names[id] + " (" + strconv.Itoa(id) + ")"
}

func (h *handlers) teamDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTeam(r.Context(), pathID(r))
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, confirmDelete("teams", "team", t.Name, "/admin/teams/"+t.ID+"/delete", "/admin/teams"))
}

func (h *handlers) teamDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteTeam(r.Context(), pathID(r)); err != nil {
		h.fail(w, r, err, nil)
		return
	}
	redirect(w, r, "/admin/teams", "deleted")
}
