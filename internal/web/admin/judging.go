package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/service"
	"awt/internal/web"
)

// Live judging screen (spec 05 §6.2, §8 "Live scoring"): the participant
// list of a run in starting order and the flight form of one participant.
// The form is rebuilt server side from the posted values, so structural
// edits (removing or moving a trick, removing a warning) keep per-trick
// marks attached to their trick and remove exactly the targeted entry
// (spec 05 §6.4).

const judgingPath = "/admin/competitions/{code}/runs/{n}/judging"

func (h *handlers) registerJudging(r *web.Router) {
	get := func(p string, fn http.HandlerFunc) { r.Scorer("GET "+judgingPath+p, codeFrom, fn) }
	post := func(p string, fn http.HandlerFunc) { r.ScorerMax("POST "+judgingPath+p, formMax, codeFrom, fn) }
	get("", h.judgingPage)
	get("/tricks.json", h.judgingTricks)
	get("/{participant}", h.judgingParticipant)
	post("/{participant}/simulate", h.judgingSimulate)
	post("/{participant}/edit", h.judgingEdit)
	post("/{participant}/save", h.judgingSave)
}

// judgingCtx is what every judging request loads.
type judgingCtx struct {
	C      service.Competition
	Run    service.Run
	Order  []orderRow
	Judges []service.Judge // run judges, in run order
	Status map[string]service.FlightStatus
}

func (jc judgingCtx) Base() string { return runHref(jc.C, jc.Run.Index) + "/judging" }

func (jc judgingCtx) synchro() bool { return jc.C.Type == domain.TypeSynchro }

// index is the position of a participant in the starting order (-1: none).
func (jc judgingCtx) index(id string) int {
	return slices.IndexFunc(jc.Order, func(o orderRow) bool { return o.ID == id })
}

func (h *handlers) judgingLoad(w http.ResponseWriter, r *http.Request) (judgingCtx, bool) {
	c, run, ok := h.loadRun(w, r)
	if !ok {
		return judgingCtx{}, false
	}
	jc := judgingCtx{C: c, Run: run}
	var err error
	if jc.Order, err = h.orderRows(r, c, run); err != nil {
		h.fail(w, r, err, nil)
		return jc, false
	}
	all, err := h.svc.ListJudges(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return jc, false
	}
	for _, id := range run.Judges {
		j := service.Judge{ID: id, Name: id}
		if i := slices.IndexFunc(all, func(j service.Judge) bool { return j.ID == id }); i >= 0 {
			j = all[i]
		}
		jc.Judges = append(jc.Judges, j)
	}
	if jc.Status, err = h.svc.RunFlightStatuses(r.Context(), c.ID, run.Index); err != nil {
		h.fail(w, r, err, nil)
		return jc, false
	}
	return jc, true
}

// participantRow is one line of the navigator.
type participantRow struct {
	orderRow
	Status  string // "none", "saved" or "published"
	DNS     bool
	Score   string
	Current bool
}

func (jc judgingCtx) participants(current string) []participantRow {
	out := make([]participantRow, 0, len(jc.Order))
	for _, o := range jc.Order {
		row := participantRow{orderRow: o, Status: "none", Current: o.ID == current}
		if st, ok := jc.Status[o.ID]; ok {
			row.Status, row.DNS, row.Score = "saved", st.DidNotStart, ftoa(scoring.Round3(st.Score))
			if st.Published {
				row.Status = "published"
			}
		}
		out = append(out, row)
	}
	return out
}

// markRow is one judge's line of the marks grid, as typed.
type markRow struct {
	Judge                                     service.Judge
	Technical, Choreography, Landing, Synchro string
	PerTrick                                  []string // one per trick row
}

// flightForm is the flight form as typed: blank trick rows and blank
// warnings are kept on screen but left out of the scored input.
type flightForm struct {
	Tricks     []string
	Rows       []markRow
	W1, W2     []string
	DNS        bool
	TipTouches string
}

// flightView is the flight panel: form, provisional score and messages.
type flightView struct {
	JC         judgingCtx
	P          orderRow
	MarkType   scoring.MarkType
	F          flightForm
	Errs       map[string]string // by input id
	Summary    string            // refusal or engine message
	Final      *scoring.FinalMark
	Saved      bool // a flight is stored
	Published  bool // the stored flight is published
	StoredType scoring.MarkType
	Flash      string
	FocusTrick int // index of a trick input to focus, -1 none
	Fragment   bool
	SwapList   bool // also swap the participant list (after a save)
}

func (v flightView) Href() string { return v.JC.Base() + "/" + url.PathEscape(v.P.ID) }
func (v flightView) PerTrick() bool {
	return v.MarkType == scoring.MarkAWT || v.MarkType == scoring.MarkAWQ
}
func (v flightView) Synchro() bool        { return v.JC.synchro() }
func (v flightView) Err(id string) string { return v.Errs[id] }

// MarkTypeLabel names the marking scheme shown on the form.
func (v flightView) MarkTypeLabel() string {
	switch v.MarkType {
	case scoring.MarkAWT:
		return "AWT: technical marks per trick"
	case scoring.MarkAWQ:
		return "AWQ: technical marks per trick"
	}
	return "Classic: one technical mark per judge"
}

func markName(judge, col string) string { return "mark." + judge + "." + col }
func markID(j int, col string) string   { return "m" + itoa(j) + "-" + col }
func trickID(k int) string              { return "trick-" + itoa(k) }

// emptyForm is the form of a participant without a stored flight: one
// blank trick row to type into.
func emptyForm(jc judgingCtx) flightForm {
	f := flightForm{Tricks: []string{""}}
	for _, j := range jc.Judges {
		f.Rows = append(f.Rows, markRow{Judge: j, PerTrick: []string{""}})
	}
	return f
}

func markString(v *float64) string {
	if v == nil {
		return ""
	}
	return ftoa(*v)
}

// storedForm fills the form from a stored flight.
func storedForm(jc judgingCtx, in service.FlightInput) flightForm {
	f := flightForm{Tricks: slices.Clone(in.Tricks), W1: slices.Clone(in.Warnings1), W2: slices.Clone(in.Warnings2), DNS: in.DidNotStart}
	if in.TipTouches != 0 {
		f.TipTouches = itoa(in.TipTouches)
	}
	for _, j := range jc.Judges {
		row := markRow{Judge: j, PerTrick: make([]string, len(f.Tricks))}
		if i := slices.IndexFunc(in.Marks, func(m service.MarkInput) bool { return m.Judge == j.ID }); i >= 0 {
			m := in.Marks[i]
			row.Technical, row.Choreography = markString(m.Technical), markString(m.Choreography)
			row.Landing, row.Synchro = markString(m.Landing), markString(m.Synchro)
			for k := range row.PerTrick {
				if k < len(m.TechnicalPerTrick) {
					row.PerTrick[k] = markString(m.TechnicalPerTrick[k])
				}
			}
		}
		f.Rows = append(f.Rows, row)
	}
	return f
}

// postedForm reads the form as typed. Per-trick marks are read by trick
// row index, so the grid always has one column per trick row.
func postedForm(r *http.Request, jc judgingCtx) flightForm {
	f := flightForm{
		Tricks: trimAll(r.PostForm["trick"]), W1: r.PostForm["w1"], W2: r.PostForm["w2"],
		DNS: r.PostFormValue("dns") == "1", TipTouches: strings.TrimSpace(r.PostFormValue("tip_touches")),
	}
	for _, j := range jc.Judges {
		row := markRow{
			Judge:        j,
			Technical:    r.PostFormValue(markName(j.ID, "technical")),
			Choreography: r.PostFormValue(markName(j.ID, "choreography")),
			Landing:      r.PostFormValue(markName(j.ID, "landing")),
			Synchro:      r.PostFormValue(markName(j.ID, "synchro")),
			PerTrick:     make([]string, len(f.Tricks)),
		}
		for k := range f.Tricks {
			row.PerTrick[k] = r.PostFormValue(markName(j.ID, "t."+itoa(k)))
		}
		f.Rows = append(f.Rows, row)
	}
	return f
}

func trimAll(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = strings.TrimSpace(v)
	}
	return out
}

// applyOp applies a structural edit ("trick-add", "trick-remove:K",
// "trick-up:K", "trick-down:K", "w1-add", "w1-remove:I", "w2-add",
// "w2-remove:I") and returns the trick input to focus (-1 none). Trick
// edits apply to the per-trick marks of every judge at the same index.
func (f *flightForm) applyOp(op string) (focus int, ok bool) {
	name, arg, hasArg := strings.Cut(op, ":")
	idx := -1
	if hasArg {
		n, err := strconv.Atoi(arg)
		if err != nil {
			return -1, false
		}
		idx = n
	}
	trickOK := idx >= 0 && idx < len(f.Tricks)
	switch {
	case name == "trick-add" && !hasArg:
		f.Tricks = append(f.Tricks, "")
		for i := range f.Rows {
			f.Rows[i].PerTrick = append(f.Rows[i].PerTrick, "")
		}
		return len(f.Tricks) - 1, true
	case name == "trick-remove" && trickOK:
		f.Tricks = slices.Delete(f.Tricks, idx, idx+1)
		for i := range f.Rows {
			f.Rows[i].PerTrick = slices.Delete(f.Rows[i].PerTrick, idx, idx+1)
		}
		return -1, true
	case (name == "trick-up" && trickOK && idx > 0) || (name == "trick-down" && trickOK && idx < len(f.Tricks)-1):
		other := idx - 1
		if name == "trick-down" {
			other = idx + 1
		}
		f.Tricks[idx], f.Tricks[other] = f.Tricks[other], f.Tricks[idx]
		for i := range f.Rows {
			p := f.Rows[i].PerTrick
			p[idx], p[other] = p[other], p[idx]
		}
		return other, true
	case name == "w1-add" && !hasArg:
		f.W1 = append(f.W1, "")
		return -1, true
	case name == "w2-add" && !hasArg:
		f.W2 = append(f.W2, "")
		return -1, true
	case name == "w1-remove" && idx >= 0 && idx < len(f.W1):
		f.W1 = slices.Delete(f.W1, idx, idx+1)
		return -1, true
	case name == "w2-remove" && idx >= 0 && idx < len(f.W2):
		f.W2 = slices.Delete(f.W2, idx, idx+1)
		return -1, true
	}
	return -1, false
}

// markError is the message shown next to a mark input.
func markError(err error) string {
	if errors.Is(err, domain.ErrMarkRange) {
		return "Between 0 and 10"
	}
	return "Not a mark: type 7.5 or 7,5"
}

// input builds the scored flight from the form, with the field errors by
// input id. Only the columns of the mark type are read.
func (f flightForm) input(mt scoring.MarkType, synchro bool) (service.FlightInput, map[string]string) {
	errs := map[string]string{}
	in := service.FlightInput{DidNotStart: f.DNS}
	perTrick := mt == scoring.MarkAWT || mt == scoring.MarkAWQ
	var kept []int // trick rows that are not blank
	for k, t := range f.Tricks {
		if t != "" {
			in.Tricks = append(in.Tricks, t)
			kept = append(kept, k)
		}
	}
	parse := func(id, raw string) *float64 {
		v, err := domain.ValidateMark(raw)
		if err != nil {
			errs[id] = markError(err)
		}
		return v
	}
	for j, row := range f.Rows {
		m := service.MarkInput{
			Judge:        row.Judge.ID,
			Choreography: parse(markID(j, "choreography"), row.Choreography),
			Landing:      parse(markID(j, "landing"), row.Landing),
		}
		if perTrick {
			m.TechnicalPerTrick = make([]*float64, 0, len(kept))
			for _, k := range kept {
				m.TechnicalPerTrick = append(m.TechnicalPerTrick, parse(markID(j, "t"+itoa(k)), row.PerTrick[k]))
			}
		} else {
			m.Technical = parse(markID(j, "technical"), row.Technical)
		}
		if synchro {
			m.Synchro = parse(markID(j, "synchro"), row.Synchro)
		}
		in.Marks = append(in.Marks, m)
	}
	in.Warnings1, in.Warnings2 = nonBlank(f.W1), nonBlank(f.W2)
	if synchro && f.TipTouches != "" {
		n, err := strconv.Atoi(f.TipTouches)
		if err != nil || n < 0 {
			errs["tip_touches"] = "A whole number, 0 or more"
		}
		in.TipTouches = n
	}
	return in, errs
}

func nonBlank(s []string) []string {
	out := []string{}
	for _, v := range s {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ---- handlers ----

func (h *handlers) judgingPage(w http.ResponseWriter, r *http.Request) {
	jc, ok := h.judgingLoad(w, r)
	if !ok {
		return
	}
	if len(jc.Order) == 0 {
		render(w, r, http.StatusOK, judgingPage(jc, nil))
		return
	}
	// Start on the first participant without a saved flight.
	p := jc.Order[0]
	for _, o := range jc.Order {
		if _, saved := jc.Status[o.ID]; !saved {
			p = o
			break
		}
	}
	h.judgingShow(w, r, jc, p, judgingFlash(r))
}

// judgingParticipant loads a participant's form, prefilled with the stored
// flight (htmx: the flight panel only).
func (h *handlers) judgingParticipant(w http.ResponseWriter, r *http.Request) {
	jc, p, ok := h.judgingParticipantCtx(w, r)
	if !ok {
		return
	}
	h.judgingShow(w, r, jc, p, judgingFlash(r))
}

// judgingShow shows a participant's form prefilled with the stored flight
// (htmx: the flight panel, with the participant list out of band).
func (h *handlers) judgingShow(w http.ResponseWriter, r *http.Request, jc judgingCtx, p orderRow, flashMsg string) {
	v, ok := h.flightViewFor(w, r, jc, p)
	if !ok {
		return
	}
	stored, saved, err := h.svc.GetFlight(r.Context(), jc.C.ID, jc.Run.Index, p.ID)
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	v.F = emptyForm(jc)
	if saved {
		v.F, v.Saved, v.Published, v.StoredType = storedForm(jc, stored.Input), true, stored.Published, stored.Final.MarkType
		v.Final = &stored.Final
	}
	v.Flash, v.SwapList, v.FocusTrick = flashMsg, v.Fragment, 0
	h.judgingRender(w, r, http.StatusOK, v)
}

func judgingFlash(r *http.Request) string {
	if r.URL.Query().Get("ok") == "published" {
		return "Published."
	}
	return flash(r)
}

func (h *handlers) judgingParticipantCtx(w http.ResponseWriter, r *http.Request) (judgingCtx, orderRow, bool) {
	jc, ok := h.judgingLoad(w, r)
	if !ok {
		return jc, orderRow{}, false
	}
	i := jc.index(r.PathValue("participant"))
	if i < 0 {
		h.notFound(w, r)
		return jc, orderRow{}, false
	}
	return jc, jc.Order[i], true
}

// flightViewFor starts the view of a participant: mark type and the
// stored flight's state.
func (h *handlers) flightViewFor(w http.ResponseWriter, r *http.Request, jc judgingCtx, p orderRow) (flightView, bool) {
	mt, err := h.svc.MarkTypeFor(r.Context(), jc.C, p.ID)
	if err != nil {
		h.fail(w, r, err, nil)
		return flightView{}, false
	}
	return flightView{JC: jc, P: p, MarkType: mt, FocusTrick: -1, Fragment: isHTMX(r), Errs: map[string]string{}}, true
}

// judgingPosted reads a posted flight form with the stored flight's state.
func (h *handlers) judgingPosted(w http.ResponseWriter, r *http.Request) (flightView, bool) {
	jc, p, ok := h.judgingParticipantCtx(w, r)
	if !ok {
		return flightView{}, false
	}
	v, ok := h.flightViewFor(w, r, jc, p)
	if !ok {
		return v, false
	}
	if st, saved := jc.Status[p.ID]; saved {
		v.Saved, v.Published = true, st.Published
	}
	v.F = postedForm(r, jc)
	return v, true
}

// simulate scores the form without writing; it fills Final, or Errs and
// Summary, and returns the status to answer.
func (h *handlers) simulate(w http.ResponseWriter, r *http.Request, v *flightView) (int, bool) {
	in, errs := v.F.input(v.MarkType, v.Synchro())
	if len(errs) > 0 {
		v.Errs = errs
		v.Summary = "Some values are not valid; the score is not updated."
		return http.StatusUnprocessableEntity, true
	}
	res, err := h.svc.SimulateFlight(r.Context(), v.JC.C.ID, v.JC.Run.Index, v.P.ID, in)
	if err == nil {
		v.Final = &res.Final
		return http.StatusOK, true
	}
	if status, ok := h.flightError(w, r, v, err); ok {
		return status, true
	}
	return 0, false
}

// flightError puts a refused flight's messages on the view; other errors
// are answered (ok false).
func (h *handlers) flightError(w http.ResponseWriter, r *http.Request, v *flightView, err error) (int, bool) {
	var sve scoring.ValidationError
	var ve *service.ValidationError
	switch {
	case errors.As(err, &sve):
		v.Summary = sve.Msg
		if rest, found := strings.CutPrefix(sve.Msg, "Unknown trick(s): "); found {
			unknown := strings.Split(rest, ", ")
			for k, t := range v.F.Tricks {
				if t != "" && slices.Contains(unknown, t) {
					v.Errs[trickID(k)] = "Not in the " + itoa(v.JC.C.Year()) + " catalog"
				}
			}
		}
		return http.StatusUnprocessableEntity, true
	case errors.As(err, &ve):
		fe := validationErrors(ve.Fields)
		if m := fe.Msg("tip_touches"); m != "" {
			v.Errs["tip_touches"] = m
		}
		v.Summary = fe.Summary + " " + strings.Join(fe.Messages, "; ")
		return http.StatusUnprocessableEntity, true
	case errors.Is(err, domain.ErrInvalidTransition):
		v.Summary = refusalMessage(err)
		return http.StatusConflict, true
	case errors.Is(err, service.ErrConflict), errors.Is(err, service.ErrFrozen), errors.Is(err, service.ErrRescore):
		v.Summary = capitalize(err.Error())
		return http.StatusConflict, true
	}
	h.fail(w, r, err, nil)
	return 0, false
}

// judgingSimulate answers the live provisional score: the result panel and
// the field error slots (htmx), or the page (no JS).
func (h *handlers) judgingSimulate(w http.ResponseWriter, r *http.Request) {
	v, ok := h.judgingPosted(w, r)
	if !ok {
		return
	}
	status, ok := h.simulate(w, r, &v)
	if !ok {
		return
	}
	if v.Fragment {
		render(w, r, status, provisionalResponse(v))
		return
	}
	h.judgingRender(w, r, status, v)
}

// judgingEdit applies a structural edit and re-renders the form with a
// fresh provisional score.
func (h *handlers) judgingEdit(w http.ResponseWriter, r *http.Request) {
	v, ok := h.judgingPosted(w, r)
	if !ok {
		return
	}
	focus, ok := v.F.applyOp(r.PostFormValue("op"))
	if !ok {
		v.Summary = "That change is not possible."
		h.judgingRender(w, r, http.StatusUnprocessableEntity, v)
		return
	}
	v.FocusTrick = focus
	// The edit itself succeeded: an incomplete flight only shows its
	// messages, with a 200.
	if _, ok := h.simulate(w, r, &v); ok {
		h.judgingRender(w, r, http.StatusOK, v)
	}
}

// judgingSave stores the flight (publish=1 publishes it), then moves to the
// next participant in starting order.
func (h *handlers) judgingSave(w http.ResponseWriter, r *http.Request) {
	v, ok := h.judgingPosted(w, r)
	if !ok {
		return
	}
	publish := r.PostFormValue("publish") == "1"
	in, errs := v.F.input(v.MarkType, v.Synchro())
	if len(errs) > 0 {
		v.Errs, v.Summary = errs, "Some values are not valid; the flight was not saved."
		h.judgingRender(w, r, http.StatusUnprocessableEntity, v)
		return
	}
	if _, err := h.svc.SaveFlight(r.Context(), v.JC.C.ID, v.JC.Run.Index, v.P.ID, in, publish); err != nil {
		status, ok := h.flightError(w, r, &v, err)
		if ok {
			h.judgingRender(w, r, status, v)
		}
		return
	}
	key, msg := "saved", flashMessages["saved"]
	if publish {
		key, msg = "published", "Published."
	}
	next := v.P
	if i := v.JC.index(v.P.ID); i+1 < len(v.JC.Order) {
		next = v.JC.Order[i+1]
	}
	href := v.JC.Base() + "/" + url.PathEscape(next.ID)
	if !v.Fragment {
		redirect(w, r, href, key)
		return
	}
	jc, ok := h.judgingLoad(w, r) // fresh statuses
	if !ok {
		return
	}
	w.Header().Set("HX-Push-Url", href)
	h.judgingShow(w, r, jc, next, msg)
}

// judgingRender answers with the flight panel (htmx) or the whole page.
func (h *handlers) judgingRender(w http.ResponseWriter, r *http.Request, status int, v flightView) {
	if v.Fragment {
		render(w, r, status, flightPanel(v))
		return
	}
	render(w, r, status, judgingPage(v.JC, &v))
}

// judgingTricks lists the competition-year unique tricks of the
// competition type, for the acronym autocomplete.
func (h *handlers) judgingTricks(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadComp(w, r)
	if !ok {
		return
	}
	all, err := h.svc.ListUniqueTricks(r.Context(), c.Year())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	type item struct {
		Acronym string `json:"acronym"`
		Name    string `json:"name"`
	}
	out := []item{}
	for _, t := range all {
		if (c.Type == domain.TypeSynchro && t.Synchro) || (c.Type == domain.TypeSolo && (t.Solo || t.SoloAWT)) {
			out = append(out, item{Acronym: t.Acronym, Name: t.Name + " (" + ftoa(t.TechnicalCoefficient) + ")"})
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(out)
}

// Preset warnings of the legacy screen (spec 05 §6.2), offered as
// suggestions; any free text is accepted.
var (
	warningPresets1 = []string{"flight over the public", "disregard of the flightbox", "missing or late at briefing",
		"missing or late at shuttle", "not ready before deadline"}
	warningPresets2 = []string{"dangerous flying", "not using the rescue", "unsportmanship behavior"}
)

func statusLabel(s string) string {
	switch s {
	case "saved":
		return "saved"
	case "published":
		return "published"
	}
	return "not flown"
}

func markTypeName(mt scoring.MarkType) string {
	if mt == scoring.MarkClassic {
		return "classic"
	}
	return strings.ToUpper(string(mt))
}

// num shows a score component rounded as stored (spec 03 §9).
func num(f float64) string { return ftoa(scoring.Round3(f)) }

func pathEscape(s string) string { return url.PathEscape(s) }
