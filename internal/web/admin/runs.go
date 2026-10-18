package admin

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/service"
	"awt/internal/web"
)

func (h *handlers) registerRuns(r *web.Router) {
	get(r, "/admin/competitions/{code}/runs/new", h.runNew)
	post(r, "/admin/competitions/{code}/runs", h.runCreate)
	scorer(r, "/admin/competitions/{code}/runs/{n}", h.runShow)
	post(r, "/admin/competitions/{code}/runs/{n}/move", h.runMove)
	post(r, "/admin/competitions/{code}/runs/{n}/judges", h.runJudgesSave)
	post(r, "/admin/competitions/{code}/runs/{n}/config", h.runConfigSave)
	get(r, "/admin/competitions/{code}/runs/{n}/state/{action}", h.runStateConfirm)
	post(r, "/admin/competitions/{code}/runs/{n}/state/{action}", h.runStateDo)
}

// resetPolicies explain the repetition reset policy (spec 03 §5.3).
var resetPolicies = []option{
	{"0", "None: repetitions of previous runs always count"},
	{"1", "AWT: repetitions are reset for AWT pilots (competition with an awt season)"},
	{"2", "AWQ: repetitions are reset for non-AWT pilots (competition with an awq season)"},
	{"3", "All: repetitions are reset for every pilot"},
}

type runNewView struct {
	C       service.Competition
	Qualify string
	Policy  string
	Errors  formErrors
}

func (h *handlers) runNewRender(w http.ResponseWriter, r *http.Request, c service.Competition, qualify, policy string, status int, fe formErrors) {
	render(w, r, status, runNewPage(runNewView{C: c, Qualify: qualify, Policy: policy, Errors: fe}))
}

func (h *handlers) runNew(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok || !h.writable(w, r, c) {
		return
	}
	h.runNewRender(w, r, c, "0", "0", http.StatusOK, formErrors{})
}

func (h *handlers) runCreate(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	qualify, policy := strings.TrimSpace(r.PostFormValue("qualify")), strings.TrimSpace(r.PostFormValue("reset_policy"))
	redo := func(status int, fe formErrors) { h.runNewRender(w, r, c, qualify, policy, status, fe) }
	var errs []domain.FieldError
	q, bad := atoi("qualify", qualify)
	if bad != nil {
		errs = append(errs, *bad)
	}
	p, bad := atoi("reset_policy", policy)
	if bad != nil {
		errs = append(errs, *bad)
	}
	if len(errs) > 0 {
		redo(http.StatusUnprocessableEntity, validationErrors(errs))
		return
	}
	run, err := h.svc.NewRun(r.Context(), c.ID, q, scoring.ResetPolicy(p))
	if err != nil {
		h.fail(w, r, err, redo)
		return
	}
	redirect(w, r, runHref(c, run.Index), "created")
}

func runHref(c service.Competition, idx int) string {
	return compHref(c) + "/runs/" + strconv.Itoa(idx+1)
}

// loadRun resolves {code} and {n} to a competition and one of its runs.
func (h *handlers) loadRun(w http.ResponseWriter, r *http.Request) (service.Competition, service.Run, bool) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return c, service.Run{}, false
	}
	idx, ok := runNumber(r)
	if !ok {
		h.notFound(w, r)
		return c, service.Run{}, false
	}
	run, err := h.svc.GetRun(r.Context(), c.ID, idx)
	if err != nil {
		h.fail(w, r, err, nil)
		return c, run, false
	}
	return c, run, true
}

type orderRow struct {
	Pos    int
	ID     string
	Name   string
	AWT    bool
	Detail string
}

type judgeChoice struct {
	J       service.Judge
	Checked bool
}

type runView struct {
	C       service.Competition
	Run     service.Run
	N       int
	Admin   bool
	CanEdit bool // admin on a competition that is not frozen
	Order   []orderRow
	Judges  []judgeChoice
	Config  configView
	Errors  formErrors // of the section named Section
	Section string
	Flash   string
}

func (v runView) Href() string { return runHref(v.C, v.Run.Index) }

// orderRows names the run's participants in starting order.
func (h *handlers) orderRows(r *http.Request, c service.Competition, run service.Run) ([]orderRow, error) {
	var rows []orderRow
	if c.Type == domain.TypeSolo {
		all, err := h.svc.ListPilots(r.Context())
		if err != nil {
			return nil, err
		}
		byID := map[int]service.Pilot{}
		for _, p := range all {
			byID[p.CivlID] = p
		}
		for i, id := range run.Pilots {
			p, ok := byID[id]
			if !ok {
				p = service.Pilot{CivlID: id, Name: "Pilot " + itoa(id)}
			}
			rows = append(rows, orderRow{Pos: i + 1, ID: itoa(id), Name: p.Name, AWT: slices.Contains(p.AWTYears, c.Year()), Detail: strings.ToUpper(p.Country)})
		}
		return rows, nil
	}
	all, err := h.svc.ListTeams(r.Context())
	if err != nil {
		return nil, err
	}
	byID := map[string]service.Team{}
	for _, t := range all {
		byID[t.ID] = t
	}
	for i, id := range run.Teams {
		name := "Team " + id
		if t, ok := byID[id]; ok {
			name = t.Name
		}
		rows = append(rows, orderRow{Pos: i + 1, ID: id, Name: name})
	}
	return rows, nil
}

// runRender shows the run page; section and fe carry the errors of the form
// that was just submitted, vals the config values to show (nil: stored).
func (h *handlers) runRender(w http.ResponseWriter, r *http.Request, c service.Competition, run service.Run, status int, section string, fe formErrors, vals map[string]string) {
	order, err := h.orderRows(r, c, run)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	judges, err := h.svc.ListJudges(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	v := runView{C: c, Run: run, N: run.Index + 1, Admin: isAdmin(r), Order: order, Section: section, Errors: fe, Flash: flash(r)}
	v.CanEdit = v.Admin && !c.Frozen
	for _, j := range judges {
		if slices.Contains(c.Judges, j.ID) {
			v.Judges = append(v.Judges, judgeChoice{J: j, Checked: slices.Contains(run.Judges, j.ID)})
		}
	}
	if vals == nil {
		vals = configValues(run.Config)
	}
	cfgErr := formErrors{}
	if section == "config" {
		cfgErr = stripConfigPrefix(fe)
	}
	v.Config = configView{C: c, Action: v.Href() + "/config", Values: vals, Errors: cfgErr, Run: true}
	render(w, r, status, runShowPage(v))
}

func (h *handlers) runShow(w http.ResponseWriter, r *http.Request) {
	c, run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	h.runRender(w, r, c, run, http.StatusOK, "", formErrors{}, nil)
}

// ---- starting order ----

type orderView struct {
	C       service.Competition
	Run     service.Run
	Order   []orderRow
	CanEdit bool
	Errors  formErrors
}

func (v orderView) Href() string { return runHref(v.C, v.Run.Index) }

// runMove moves one participant up or down (htmx swaps the list, the plain
// form posts and redirects).
func (h *handlers) runMove(w http.ResponseWriter, r *http.Request) {
	c, run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	if c.Frozen {
		h.fail(w, r, service.ErrFrozen, nil)
		return
	}
	id, dir := r.PostFormValue("id"), r.PostFormValue("dir")
	pilots, teams := slices.Clone(run.Pilots), slices.Clone(run.Teams)
	keys := make([]string, 0, len(pilots)+len(teams))
	for _, p := range pilots {
		keys = append(keys, itoa(p))
	}
	keys = append(keys, teams...)
	i := slices.Index(keys, id)
	j := i - 1
	if dir == "down" {
		j = i + 1
	}
	redo := func(status int, fe formErrors) {
		if isHTMX(r) { // htmx does not swap error statuses: show the message in the list
			order, err := h.orderRows(r, c, run)
			if err != nil {
				h.fail(w, r, err, nil)
				return
			}
			render(w, r, http.StatusOK, startingOrder(orderView{C: c, Run: run, Order: order, CanEdit: true, Errors: fe}))
			return
		}
		h.runRender(w, r, c, run, status, "order", fe, nil)
	}
	if i < 0 || j < 0 || j >= len(keys) || (dir != "up" && dir != "down") {
		redo(http.StatusUnprocessableEntity, messageError("That move is not possible."))
		return
	}
	keys[i], keys[j] = keys[j], keys[i]
	pilots, teams = pilots[:0], teams[:0]
	for _, k := range keys {
		if c.Type == domain.TypeSolo {
			n, _ := strconv.Atoi(k)
			pilots = append(pilots, n)
		} else {
			teams = append(teams, k)
		}
	}
	if err := h.svc.SetRunOrder(r.Context(), c.ID, run.Index, pilots, teams); err != nil {
		h.fail(w, r, err, redo)
		return
	}
	if isHTMX(r) {
		run, err := h.svc.GetRun(r.Context(), c.ID, run.Index)
		if err != nil {
			h.fail(w, r, err, nil)
			return
		}
		order, err := h.orderRows(r, c, run)
		if err != nil {
			h.fail(w, r, err, nil)
			return
		}
		render(w, r, http.StatusOK, startingOrder(orderView{C: c, Run: run, Order: order, CanEdit: true}))
		return
	}
	redirect(w, r, runHref(c, run.Index)+"#order", "saved")
}

// ---- judges and config ----

func (h *handlers) runJudgesSave(w http.ResponseWriter, r *http.Request) {
	c, run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	_ = r.ParseForm()
	ids := r.PostForm["judge"]
	if err := h.svc.SetRunJudges(r.Context(), c.ID, run.Index, ids); err != nil {
		h.fail(w, r, err, func(status int, fe formErrors) {
			run.Judges = ids
			h.runRender(w, r, c, run, status, "judges", fe, nil)
		})
		return
	}
	redirect(w, r, runHref(c, run.Index)+"#judges", "saved")
}

func (h *handlers) runConfigSave(w http.ResponseWriter, r *http.Request) {
	c, run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	if !h.writableAt(w, r, c, http.StatusConflict) {
		return
	}
	raw, posted, errs := parseConfig(r, run.Config)
	redo := func(status int, fe formErrors) { h.runRender(w, r, c, run, status, "config", fe, posted) }
	if len(errs) > 0 {
		redo(http.StatusUnprocessableEntity, validationErrors(errs))
		return
	}
	if err := h.svc.SetRunConfig(r.Context(), c.ID, run.Index, raw); err != nil {
		h.fail(w, r, err, redo)
		return
	}
	redirect(w, r, runHref(c, run.Index)+"#config", "saved")
}

// ---- run state ----

var runStateActions = map[string]stateAction{
	"open":   {"open", "Open", "Opening the run is informational: scoring does not need it, but it shows the run is live."},
	"close":  {"close", "Close", "Closing needs every participant to have a saved and published flight. The run results become final and the next run's starting order can be built."},
	"reopen": {"reopen", "Reopen", "Reopening lets you change flights and the roster again. Close the run again when done."},
}

func (h *handlers) runStatePage(w http.ResponseWriter, r *http.Request, status int, fe formErrors) {
	c, run, ok := h.loadRun(w, r)
	if !ok || !h.writableAt(w, r, c, status) {
		return
	}
	a, ok := runStateActions[r.PathValue("action")]
	if !ok {
		h.notFound(w, r)
		return
	}
	render(w, r, status, stateConfirmPage(stateView{C: c, Action: a, Errors: fe, Post: runHref(c, run.Index) + "/state/" + a.Key,
		Back: runHref(c, run.Index), Subject: "run " + itoa(run.Index+1) + " of " + c.Name}))
}

func (h *handlers) runStateConfirm(w http.ResponseWriter, r *http.Request) {
	h.runStatePage(w, r, http.StatusOK, formErrors{})
}

func (h *handlers) runStateDo(w http.ResponseWriter, r *http.Request) {
	c, run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	redo := func(status int, fe formErrors) { h.runStatePage(w, r, status, fe) }
	var err error
	key := ""
	switch r.PathValue("action") {
	case "open":
		err, key = h.svc.OpenRun(r.Context(), c.ID, run.Index), "opened"
	case "close":
		err, key = h.svc.CloseRun(r.Context(), c.ID, run.Index), "closed"
	case "reopen":
		err, key = h.svc.ReopenRun(r.Context(), c.ID, run.Index), "reopened"
	default:
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.fail(w, r, err, redo)
		return
	}
	redirect(w, r, runHref(c, run.Index), key)
}
