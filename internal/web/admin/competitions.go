package admin

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"awt/internal/auth"
	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

// codeFrom is the competition reference of a scorer-guarded route.
func codeFrom(r *http.Request) string { return r.PathValue("code") }

// scorer registers a GET route open to admins and to the scorers assigned to
// the competition named by {code} (read-only pages and exports).
func scorer(r *web.Router, pattern string, fn http.HandlerFunc) {
	r.Scorer("GET "+pattern, codeFrom, fn)
}

func (h *handlers) registerCompetitions(r *web.Router) {
	get(r, "/manager/competitions", h.compList)
	get(r, "/manager/competitions/new", h.compNew)
	post(r, "/manager/competitions", h.compCreate)
	scorer(r, "/manager/competitions/{code}", h.compShow)
	get(r, "/manager/competitions/{code}/edit", h.compEdit)
	post(r, "/manager/competitions/{code}/edit", h.compUpdate)
	get(r, "/manager/competitions/{code}/participants", h.participantsPage)
	post(r, "/manager/competitions/{code}/participants", h.participantsSave)
	get(r, "/manager/competitions/{code}/judges", h.judgesPage)
	post(r, "/manager/competitions/{code}/judges", h.judgesSave)
	get(r, "/manager/competitions/{code}/tricks", h.repeatablePage)
	post(r, "/manager/competitions/{code}/tricks", h.repeatableSave)
	get(r, "/manager/competitions/{code}/config", h.compConfigPage)
	post(r, "/manager/competitions/{code}/config", h.compConfigSave)
	get(r, "/manager/competitions/{code}/state/{action}", h.compStateConfirm)
	post(r, "/manager/competitions/{code}/state/{action}", h.compStateDo)
	get(r, "/manager/competitions/{code}/delete", h.compDeleteConfirm)
	post(r, "/manager/competitions/{code}/delete", h.compDelete)
	h.registerRuns(r)
	h.registerExports(r)
}

// loadComp finds an active competition by code (or id) from the path.
func (h *handlers) loadComp(w http.ResponseWriter, r *http.Request) (service.Competition, bool) {
	ref := r.PathValue("code")
	c, err := h.svc.GetCompetitionByCode(r.Context(), ref)
	if err == nil {
		return c, true
	}
	if byID, err2 := h.svc.GetCompetition(r.Context(), ref); err2 == nil {
		return byID, true
	}
	h.fail(w, r, err, nil)
	return service.Competition{}, false
}

// writable refuses the setup pages of a frozen competition with a
// read-only notice (decision 0003).
func (h *handlers) writable(w http.ResponseWriter, r *http.Request, c service.Competition) bool {
	return h.writableAt(w, r, c, http.StatusOK)
}

// writableAt is writable with the status of the refusal page: a failed POST
// answers 409.
func (h *handlers) writableAt(w http.ResponseWriter, r *http.Request, c service.Competition, status int) bool {
	if c.Frozen {
		render(w, r, status, frozenPage(c))
		return false
	}
	return true
}

func compHref(c service.Competition) string { return "/manager/competitions/" + c.Code }

func isAdmin(r *http.Request) bool {
	s, _ := auth.SessionFromContext(r.Context())
	return s.User.Role == auth.RoleAdmin
}

// ---- list ----

type compRow struct {
	C    service.Competition
	Runs int
}

type compListView struct {
	Years []int
	Year  int
	Rows  []compRow
	Flash string
}

func (h *handlers) compList(w http.ResponseWriter, r *http.Request) {
	comps, err := h.svc.ListCompetitions(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	v := compListView{Flash: flash(r)}
	for _, c := range comps {
		if y := c.Year(); !slices.Contains(v.Years, y) {
			v.Years = append(v.Years, y)
		}
	}
	slices.SortFunc(v.Years, func(a, b int) int { return b - a })
	if len(v.Years) > 0 {
		v.Year = v.Years[0]
	}
	if y, err := strconv.Atoi(r.URL.Query().Get("year")); err == nil && slices.Contains(v.Years, y) {
		v.Year = y
	}
	// every competition of the year, published or not (spec 05 §7.2)
	for _, c := range comps {
		if c.Year() != v.Year {
			continue
		}
		runs, err := h.svc.ListRuns(r.Context(), c.ID)
		if err != nil {
			h.fail(w, r, err, nil)
			return
		}
		v.Rows = append(v.Rows, compRow{C: c, Runs: len(runs)})
	}
	slices.SortStableFunc(v.Rows, func(a, b compRow) int { return a.C.StartDate.Compare(b.C.StartDate) })
	render(w, r, http.StatusOK, compListPage(v))
}

// ---- create / edit ----

type compForm struct {
	Code      string // current code ("" when new)
	Name      string
	NewCode   string
	Type      string
	StartDate string
	EndDate   string
	Location  string
	Website   string
	Published bool
	Seasons   []string
	Image     string
	Logo      string
	Locked    bool // name, code and type are frozen once the competition is open
	State     string
}

type compFormView struct {
	Form    compForm
	Seasons []service.Season
	Images  []service.File
	Errors  formErrors
}

func (h *handlers) compFormPage(w http.ResponseWriter, r *http.Request, status int, f compForm, fe formErrors) {
	seasons, err := h.svc.ListSeasons(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	images, ok := h.imageFiles(w, r)
	if !ok {
		return
	}
	render(w, r, status, compFormPageT(compFormView{Form: f, Seasons: seasons, Images: images, Errors: fe}))
}

func (h *handlers) compNew(w http.ResponseWriter, r *http.Request) {
	today := time.Now().UTC().Format(time.DateOnly)
	h.compFormPage(w, r, http.StatusOK, compForm{Type: string(domain.TypeSolo), StartDate: today, EndDate: today}, formErrors{})
}

func (h *handlers) compEdit(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok || !h.writable(w, r, c) {
		return
	}
	h.compFormPage(w, r, http.StatusOK, compForm{
		Code: c.Code, Name: c.Name, NewCode: c.Code, Type: string(c.Type),
		StartDate: c.StartDate.Format(time.DateOnly), EndDate: c.EndDate.Format(time.DateOnly),
		Location: c.Location, Website: c.Website, Published: c.Published, Seasons: c.Seasons,
		Image: c.ImageFile, Logo: c.LogoFile, Locked: c.State != domain.CompetitionInit, State: string(c.State),
	}, formErrors{})
}

// compSave reads the form and creates (cur nil) or updates a competition.
// Name, code and type are not posted by a locked form: they keep their value.
func (h *handlers) compSave(w http.ResponseWriter, r *http.Request, cur *service.Competition) {
	f := compForm{Name: r.PostFormValue("name"), NewCode: r.PostFormValue("code"), Type: r.PostFormValue("type"),
		StartDate: r.PostFormValue("start_date"), EndDate: r.PostFormValue("end_date"), Location: r.PostFormValue("location"),
		Website: r.PostFormValue("website"), Published: r.PostFormValue("published") == "1",
		Seasons: r.PostForm["seasons"], Image: r.PostFormValue("image"), Logo: r.PostFormValue("logo")}
	if cur != nil {
		f.Code, f.State = cur.Code, string(cur.State)
		if f.Locked = cur.State != domain.CompetitionInit; f.Locked {
			f.Name, f.NewCode, f.Type = cur.Name, cur.Code, string(cur.Type)
		}
	}
	redo := func(status int, fe formErrors) { h.compFormPage(w, r, status, f, fe) }
	in := service.CompetitionInput{Name: strings.TrimSpace(f.Name), Code: strings.TrimSpace(f.NewCode),
		Type: domain.CompetitionType(f.Type), Location: strings.TrimSpace(f.Location), Website: strings.TrimSpace(f.Website),
		Published: f.Published, ImageFile: f.Image, LogoFile: f.Logo, Seasons: slices.DeleteFunc(slices.Clone(f.Seasons), func(s string) bool { return s == "" })}
	var errs []domain.FieldError
	var err error
	if in.StartDate, err = time.Parse(time.DateOnly, strings.TrimSpace(f.StartDate)); err != nil {
		errs = append(errs, domain.FieldError{Field: "start_date", Msg: "use the format YYYY-MM-DD"})
	}
	if in.EndDate, err = time.Parse(time.DateOnly, strings.TrimSpace(f.EndDate)); err != nil {
		errs = append(errs, domain.FieldError{Field: "end_date", Msg: "use the format YYYY-MM-DD"})
	}
	if len(errs) > 0 {
		redo(http.StatusUnprocessableEntity, validationErrors(errs))
		return
	}
	var c service.Competition
	key := "created"
	if cur == nil {
		c, err = h.svc.CreateCompetition(r.Context(), in)
	} else {
		c, err = h.svc.UpdateCompetition(r.Context(), cur.ID, in)
		key = "saved"
	}
	if err != nil {
		h.fail(w, r, err, redo)
		return
	}
	redirect(w, r, compHref(c), key)
}

func (h *handlers) compCreate(w http.ResponseWriter, r *http.Request) { h.compSave(w, r, nil) }

func (h *handlers) compUpdate(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	h.compSave(w, r, &c)
}

// ---- detail ----

type compShowView struct {
	C       service.Competition
	Admin   bool
	Runs    []service.Run
	Seasons []service.Season
	Flash   string
}

func (h *handlers) compShow(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	runs, err := h.svc.ListRuns(r.Context(), c.ID)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, compShowPage(compShowView{C: c, Admin: isAdmin(r), Runs: runs, Flash: flash(r)}))
}

// ---- state buttons ----

type stateAction struct {
	Key, Verb, Explain string
}

var compStateActions = map[string]stateAction{
	"open":   {"open", "Open", "Opening needs at least 2 pilots (solo) or 2 teams (synchro) and at least 2 judges. Once open, the name, code and type can no longer change, and runs can be created."},
	"close":  {"close", "Close", "Closing needs every run to be closed. The competition results become final."},
	"reopen": {"reopen", "Reopen", "Reopening lets you fix results again. Close the competition again when done."},
}

type stateView struct {
	C       service.Competition
	Action  stateAction
	Errors  formErrors
	Post    string
	Back    string
	Subject string
}

func (h *handlers) compStateConfirm(w http.ResponseWriter, r *http.Request) {
	h.compStatePage(w, r, http.StatusOK, formErrors{})
}

func (h *handlers) compStatePage(w http.ResponseWriter, r *http.Request, status int, fe formErrors) {
	c, ok := h.loadComp(w, r)
	if !ok || !h.writableAt(w, r, c, status) {
		return
	}
	a, ok := compStateActions[r.PathValue("action")]
	if !ok {
		h.notFound(w, r)
		return
	}
	render(w, r, status, stateConfirmPage(stateView{C: c, Action: a, Errors: fe, Post: compHref(c) + "/state/" + a.Key,
		Back: compHref(c), Subject: "competition " + c.Name}))
}

func (h *handlers) compStateDo(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	redo := func(status int, fe formErrors) { h.compStatePage(w, r, status, fe) }
	var err error
	key := ""
	switch r.PathValue("action") {
	case "open":
		err, key = h.svc.OpenCompetition(r.Context(), c.ID), "opened"
	case "close":
		err, key = h.svc.CloseCompetition(r.Context(), c.ID), "closed"
	case "reopen":
		err, key = h.svc.ReopenCompetition(r.Context(), c.ID), "reopened"
	default:
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.fail(w, r, err, redo)
		return
	}
	redirect(w, r, compHref(c), key)
}

func (h *handlers) compDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok || !h.writable(w, r, c) {
		return
	}
	render(w, r, http.StatusOK, confirmDelete("competitions", "competition", c.Name+" ("+c.Code+")", compHref(c)+"/delete", compHref(c)))
}

func (h *handlers) compDelete(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteCompetition(r.Context(), c.ID); err != nil {
		h.fail(w, r, err, nil)
		return
	}
	redirect(w, r, "/manager/competitions", "deleted")
}
