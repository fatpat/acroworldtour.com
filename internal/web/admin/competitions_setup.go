package admin

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/service"
)

// maxCandidates bounds the add-list of the participants picker.
const maxCandidates = 50

// ---- participants (pilots for solo, teams for synchro) ----

type partItem struct {
	ID     string
	Name   string
	Detail string
	AWT    bool
}

type participantsView struct {
	C          service.Competition
	Solo       bool
	Query      string
	Current    []partItem
	Candidates []partItem
	Truncated  bool
	Errors     formErrors
	Flash      string
}

func (h *handlers) participantsRender(w http.ResponseWriter, r *http.Request, c service.Competition, status int, fe formErrors) {
	ctx := r.Context()
	q := strings.TrimSpace(r.FormValue("q"))
	v := participantsView{C: c, Solo: c.Type == domain.TypeSolo, Query: q, Errors: fe, Flash: flash(r)}
	if v.Solo {
		all, err := h.svc.ListPilots(ctx)
		if err != nil {
			h.fail(w, r, err, nil)
			return
		}
		item := func(p service.Pilot) partItem {
			rank := "unranked"
			if p.Rank > 0 {
				rank = "CIVL rank " + itoa(p.Rank)
			}
			return partItem{ID: itoa(p.CivlID), Name: p.Name, Detail: strings.ToUpper(p.Country) + ", " + rank, AWT: slices.Contains(p.AWTYears, c.Year())}
		}
		byID := map[int]service.Pilot{}
		for _, p := range all {
			byID[p.CivlID] = p
		}
		for _, id := range c.Pilots {
			p, ok := byID[id]
			if !ok {
				p = service.Pilot{CivlID: id, Name: "Pilot " + itoa(id)}
			}
			v.Current = append(v.Current, item(p))
		}
		for _, p := range filterPilots(all, q) {
			if slices.Contains(c.Pilots, p.CivlID) {
				continue
			}
			if len(v.Candidates) == maxCandidates {
				v.Truncated = true
				break
			}
			v.Candidates = append(v.Candidates, item(p))
		}
	} else {
		all, err := h.svc.ListTeams(ctx)
		if err != nil {
			h.fail(w, r, err, nil)
			return
		}
		byID := map[string]service.Team{}
		for _, t := range all {
			byID[t.ID] = t
		}
		for _, id := range c.Teams {
			name := "Team " + id
			if t, ok := byID[id]; ok {
				name = t.Name
			}
			v.Current = append(v.Current, partItem{ID: id, Name: name})
		}
		for _, t := range all {
			if slices.Contains(c.Teams, t.ID) || (q != "" && !strings.Contains(fold(t.Name), fold(q))) {
				continue
			}
			if len(v.Candidates) == maxCandidates {
				v.Truncated = true
				break
			}
			v.Candidates = append(v.Candidates, partItem{ID: t.ID, Name: t.Name})
		}
	}
	render(w, r, status, participantsPage(v))
}

func (h *handlers) participantsPage(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok || !h.writable(w, r, c) {
		return
	}
	h.participantsRender(w, r, c, http.StatusOK, formErrors{})
}

func (h *handlers) participantsSave(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PostFormValue("id"))
	op := r.PostFormValue("op")
	redo := func(status int, fe formErrors) { h.participantsRender(w, r, c, status, fe) }
	if op != "add" && op != "remove" || id == "" {
		redo(http.StatusUnprocessableEntity, messageError("Choose a participant to add or remove."))
		return
	}
	pilots, teams := slices.Clone(c.Pilots), slices.Clone(c.Teams)
	if c.Type == domain.TypeSolo {
		n, bad := atoi("id", id)
		if bad != nil {
			redo(http.StatusUnprocessableEntity, validationErrors([]domain.FieldError{*bad}))
			return
		}
		if op == "remove" {
			pilots = slices.DeleteFunc(pilots, func(p int) bool { return p == n })
		} else if !slices.Contains(pilots, n) {
			pilots = append(pilots, n)
		}
	} else if op == "remove" {
		teams = slices.DeleteFunc(teams, func(t string) bool { return t == id })
	} else if !slices.Contains(teams, id) {
		teams = append(teams, id)
	}
	if err := h.svc.SetParticipants(r.Context(), c.ID, pilots, teams); err != nil {
		h.fail(w, r, err, redo)
		return
	}
	to := compHref(c) + "/participants"
	if q := strings.TrimSpace(r.PostFormValue("q")); q != "" {
		to += "?q=" + url.QueryEscape(q)
	}
	redirect(w, r, to, "saved")
}

// ---- judges ----

type judgesPickView struct {
	C       service.Competition
	Judges  []service.Judge
	Checked map[string]bool
	Errors  formErrors
	Flash   string
}

func (h *handlers) judgesRender(w http.ResponseWriter, r *http.Request, c service.Competition, checked []string, status int, fe formErrors) {
	judges, err := h.svc.ListJudges(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	v := judgesPickView{C: c, Judges: judges, Checked: map[string]bool{}, Errors: fe, Flash: flash(r)}
	for _, j := range checked {
		v.Checked[j] = true
	}
	render(w, r, status, judgesPickPage(v))
}

func (h *handlers) judgesPage(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok || !h.writable(w, r, c) {
		return
	}
	h.judgesRender(w, r, c, c.Judges, http.StatusOK, formErrors{})
}

func (h *handlers) judgesSave(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	_ = r.ParseForm()
	ids := r.PostForm["judge"]
	if err := h.svc.SetJudges(r.Context(), c.ID, ids); err != nil {
		h.fail(w, r, err, func(status int, fe formErrors) { h.judgesRender(w, r, c, ids, status, fe) })
		return
	}
	redirect(w, r, compHref(c)+"/judges", "saved")
}

// ---- repeatable tricks ----

type repeatableView struct {
	C       service.Competition
	Tricks  []service.Trick
	Checked map[string]bool
	Errors  formErrors
	Flash   string
}

func (h *handlers) repeatableRender(w http.ResponseWriter, r *http.Request, c service.Competition, checked []string, status int, fe formErrors) {
	tricks, err := h.svc.ListTricks(r.Context(), c.Year())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	v := repeatableView{C: c, Tricks: tricks, Checked: map[string]bool{}, Errors: fe, Flash: flash(r)}
	for _, t := range checked {
		v.Checked[t] = true
	}
	render(w, r, status, repeatablePage(v))
}

func (h *handlers) repeatablePage(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok || !h.writable(w, r, c) {
		return
	}
	h.repeatableRender(w, r, c, c.RepeatableTricks, http.StatusOK, formErrors{})
}

func (h *handlers) repeatableSave(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	_ = r.ParseForm()
	names := r.PostForm["trick"]
	if err := h.svc.SetRepeatableTricks(r.Context(), c.ID, names); err != nil {
		h.fail(w, r, err, func(status int, fe formErrors) { h.repeatableRender(w, r, c, names, status, fe) })
		return
	}
	redirect(w, r, compHref(c)+"/tricks", "saved")
}

// ---- scoring config (competition and run) ----

type cfgField struct {
	Name, Label, Unit string
	Int               bool
}

type cfgGroup struct {
	Title, Hint string
	Fields      []cfgField
}

// cfgGroups lists every field of the scoring config (spec 02 §2.6) with its
// unit. The input names are the config paths, as in service validation
// errors.
var cfgGroups = []cfgGroup{
	{"Warnings and penalties", "Category 1 warnings are the operator's usual warnings, category 2 the more serious ones.", []cfgField{
		{"warning", "Category 1 warning", "points deducted per warning", false},
		{"warning2", "Category 2 warning", "points deducted per warning", false},
		{"malus_repetition", "Repetition malus", "% of choreography per repeated trick", false},
		{"warnings_to_dsq", "Warnings before disqualification", "warnings (threshold for DSQ)", true},
	}},
	{"Judge weights", "Relative weight of a judge's marks by level.", []cfgField{
		{"judge_weights.senior", "Senior judge", "weight", false},
		{"judge_weights.certified", "Certified judge", "weight", false},
		{"judge_weights.trainee", "Trainee judge", "weight", false},
	}},
	{"Maximum bonus per run", "Bonus cap per bonus type (maximum number counted in a run).", []cfgField{
		{"max_bonus_per_run.twist", "Twists", "per run", false},
		{"max_bonus_per_run.reverse", "Reverses", "per run", false},
		{"max_bonus_per_run.flip", "Flips", "per run", false},
	}},
	{"Mark percentages, solo", "Share of each component in the final mark. Must add up to 100.", []cfgField{
		{"mark_percentages.solo.technical", "Technical", "%", false},
		{"mark_percentages.solo.choreography", "Choreography", "%", false},
		{"mark_percentages.solo.landing", "Landing", "%", false},
	}},
	{"Mark percentages, synchro", "Must add up to 100.", []cfgField{
		{"mark_percentages.synchro.technical", "Technical", "%", false},
		{"mark_percentages.synchro.choreography", "Choreography", "%", false},
		{"mark_percentages.synchro.landing", "Landing", "%", false},
		{"mark_percentages.synchro.synchro", "Synchro", "%", false},
	}},
}

// cfgAccess reads and writes a config field by its path.
func cfgAccess(cfg *scoring.Config, name string) (get func() float64, set func(float64)) {
	if cfg.JudgeWeights == nil {
		cfg.JudgeWeights = map[scoring.JudgeLevel]float64{}
	}
	if cfg.MaxBonusPerRun == nil {
		cfg.MaxBonusPerRun = map[string]float64{}
	}
	if cfg.MarkPercentages == nil {
		cfg.MarkPercentages = map[scoring.CompetitionType]map[string]float64{}
	}
	switch name {
	case "warning":
		return func() float64 { return cfg.Warning }, func(f float64) { cfg.Warning = f }
	case "warning2":
		return func() float64 { return cfg.Warning2 }, func(f float64) { cfg.Warning2 = f }
	case "malus_repetition":
		return func() float64 { return cfg.MalusRepetition }, func(f float64) { cfg.MalusRepetition = f }
	case "warnings_to_dsq":
		return func() float64 { return float64(cfg.WarningsToDSQ) }, func(f float64) { cfg.WarningsToDSQ = int(f) }
	}
	parts := strings.Split(name, ".")
	switch parts[0] {
	case "judge_weights":
		k := scoring.JudgeLevel(parts[1])
		return func() float64 { return cfg.JudgeWeights[k] }, func(f float64) { cfg.JudgeWeights[k] = f }
	case "max_bonus_per_run":
		return func() float64 { return cfg.MaxBonusPerRun[parts[1]] }, func(f float64) { cfg.MaxBonusPerRun[parts[1]] = f }
	}
	typ := scoring.CompetitionType(parts[1])
	if cfg.MarkPercentages[typ] == nil {
		cfg.MarkPercentages[typ] = map[string]float64{}
	}
	return func() float64 { return cfg.MarkPercentages[typ][parts[2]] }, func(f float64) { cfg.MarkPercentages[typ][parts[2]] = f }
}

// configValues renders a stored config into form values.
func configValues(raw json.RawMessage) map[string]string {
	var cfg scoring.Config
	_ = json.Unmarshal(raw, &cfg)
	out := map[string]string{}
	for _, g := range cfgGroups {
		for _, f := range g.Fields {
			get, _ := cfgAccess(&cfg, f.Name)
			out[f.Name] = ftoa(get())
		}
	}
	return out
}

// parseConfig applies the posted fields over base (keeping what the form
// does not show, such as extra bonus types) and returns the config JSON.
func parseConfig(r *http.Request, base json.RawMessage) (json.RawMessage, map[string]string, []domain.FieldError) {
	var cfg scoring.Config
	_ = json.Unmarshal(base, &cfg)
	posted := map[string]string{}
	var errs []domain.FieldError
	for _, g := range cfgGroups {
		for _, f := range g.Fields {
			raw := r.PostFormValue(f.Name)
			posted[f.Name] = raw
			_, set := cfgAccess(&cfg, f.Name)
			if f.Int {
				n, bad := atoi(f.Name, raw)
				if bad != nil {
					errs = append(errs, *bad)
					continue
				}
				set(float64(n))
				continue
			}
			n, bad := atof(f.Name, raw)
			if bad != nil {
				errs = append(errs, *bad)
				continue
			}
			set(n)
		}
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		errs = append(errs, domain.FieldError{Field: "config", Msg: err.Error()})
	}
	return b, posted, errs
}

// stripConfigPrefix maps the service's "config.x" field errors to the form
// input names.
func stripConfigPrefix(fe formErrors) formErrors {
	out := formErrors{Summary: fe.Summary, Fields: map[string]string{}}
	for k, v := range fe.Fields {
		out.Fields[strings.TrimPrefix(k, "config.")] = v
	}
	for _, m := range fe.Messages {
		out.Messages = append(out.Messages, strings.TrimPrefix(m, "config."))
	}
	return out
}

type configView struct {
	C       service.Competition
	Heading string
	Action  string
	Back    string
	Values  map[string]string
	Errors  formErrors
	Flash   string
	Run     bool // inside the run page: no heading, no tabs
}

func (h *handlers) compConfigRender(w http.ResponseWriter, r *http.Request, c service.Competition, vals map[string]string, status int, fe formErrors) {
	render(w, r, status, compConfigPage(configView{C: c, Heading: "Settings", Action: compHref(c) + "/config", Back: compHref(c),
		Values: vals, Errors: stripConfigPrefix(fe), Flash: flash(r)}))
}

func (h *handlers) compConfigPage(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok || !h.writable(w, r, c) {
		return
	}
	h.compConfigRender(w, r, c, configValues(c.Config), http.StatusOK, formErrors{})
}

func (h *handlers) compConfigSave(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	raw, posted, errs := parseConfig(r, c.Config)
	redo := func(status int, fe formErrors) { h.compConfigRender(w, r, c, posted, status, fe) }
	if len(errs) > 0 {
		redo(http.StatusUnprocessableEntity, validationErrors(errs))
		return
	}
	if err := h.svc.SetConfig(r.Context(), c.ID, raw); err != nil {
		h.fail(w, r, err, redo)
		return
	}
	redirect(w, r, compHref(c)+"/config", "saved")
}

// runNumber parses a 1-based run number from the path.
func runNumber(r *http.Request) (int, bool) {
	n, err := strconv.Atoi(r.PathValue("n"))
	return n - 1, err == nil && n >= 1
}
