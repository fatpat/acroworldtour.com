package admin_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"awt/internal/catalog"
	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/service"
)

// judgingEnv is an open 2025 competition (2025 catalog) with run 1 and
// judges[0:2] as run judges.
type judgingEnv struct {
	*env
	c      service.Competition
	judges []string
	base   string // judging URL of run 1
}

// newJudging creates a competition of the given type with seasons; solo
// uses pilots 101-103 (101 AWT in 2025), synchro two teams. openRun leaves
// the run in init when false.
func newJudging(t *testing.T, typ domain.CompetitionType, seasons []string, openRun bool) *judgingEnv {
	e := newEnv(t)
	ctx := context.Background()
	judges := e.setupData(101, 102, 103, 104)
	cat, err := os.ReadFile("../../../internal/catalog/data/2025.json")
	e.mustOK(err)
	_, err = e.svc.ImportCatalog(ctx, 2025, bytes.NewReader(cat))
	e.mustOK(err)
	e.mustOK(e.svc.SetAWTYear(ctx, 101, 2025, true))
	for _, code := range seasons {
		_, err := e.svc.CreateSeason(ctx, service.SeasonInput{Name: code, Code: code, Year: 2025, SortIndex: 1})
		e.mustOK(err)
	}
	c, err := e.svc.CreateCompetition(ctx, service.CompetitionInput{
		Name: "Judging Cup 2025", Type: typ, StartDate: date("2025-06-01"), EndDate: date("2025-06-02"), Location: "Annecy", Seasons: seasons,
	})
	e.mustOK(err)
	if typ == domain.TypeSolo {
		e.mustOK(e.svc.SetParticipants(ctx, c.ID, []int{101, 102, 103}, nil))
	} else {
		t1, err := e.svc.CreateTeam(ctx, service.TeamInput{Name: "Team A", Pilots: []int{101, 102}})
		e.mustOK(err)
		t2, err := e.svc.CreateTeam(ctx, service.TeamInput{Name: "Team B", Pilots: []int{103, 104}})
		e.mustOK(err)
		e.mustOK(e.svc.SetParticipants(ctx, c.ID, nil, []string{t1.ID, t2.ID}))
	}
	e.mustOK(e.svc.SetJudges(ctx, c.ID, judges[:2]))
	e.mustOK(e.svc.OpenCompetition(ctx, c.ID))
	_, err = e.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone)
	e.mustOK(err)
	if openRun {
		e.mustOK(e.svc.OpenRun(ctx, c.ID, 0))
	}
	c, err = e.svc.GetCompetition(ctx, c.ID)
	e.mustOK(err)
	return &judgingEnv{env: e, c: c, judges: judges[:2], base: "/manager/competitions/" + c.Code + "/runs/1/judging"}
}

// hx sends an htmx form POST with the admin session.
func (e *judgingEnv) hx(path string, form url.Values) *httptest.ResponseRecorder {
	form.Set("_csrf", e.admin.CSRF)
	req := newReq(http.MethodPost, path, form)
	req.Header.Set("HX-Request", "true")
	return e.do(req, &e.admin)
}

// classicForm is a flight of LM, RMC with both judges marking technical t.
func (e *judgingEnv) classicForm(t string) url.Values {
	f := url.Values{"trick": {"LM", "RMC"}}
	for _, j := range e.judges {
		f.Set("mark."+j+".technical", t)
		f.Set("mark."+j+".choreography", "6")
		f.Set("mark."+j+".landing", "6")
	}
	return f
}

func (e *judgingEnv) stored(participant string) (service.StoredFlight, bool) {
	e.t.Helper()
	f, ok, err := e.svc.GetFlight(context.Background(), e.c.ID, 0, participant)
	e.mustOK(err)
	return f, ok
}

func TestMarkInputParsing(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, nil, true)

	// "7,5" is a mark; the provisional score is shown.
	rec := e.hx(e.base+"/102/simulate", e.classicForm("7,5"))
	e.wantStatus(rec, 200, `id="provisional"`, "Score")
	if strings.Contains(rec.Body.String(), "<html") {
		t.Fatal("htmx simulate returned a full page")
	}

	for _, bad := range []string{"75", "abc", "1e1", "+5", "0x1p3"} {
		form := e.classicForm("7")
		form.Set("mark."+e.judges[1]+".technical", bad)
		// htmx: 422 with the message in the field's error slot (out of band).
		rec := e.hx(e.base+"/102/simulate", form)
		e.wantStatus(rec, http.StatusUnprocessableEntity, `id="err-m1-technical"`)
		if !regexp.MustCompile(`id="err-m1-technical"[^>]*>[^<]+<`).MatchString(rec.Body.String()) {
			t.Fatalf("%q: no message in the field error slot: %s", bad, rec.Body.String())
		}
		// Without JS: the page is re-rendered with the typed value kept.
		form.Del("_csrf")
		rec = e.post(e.base+"/102/simulate", form)
		e.wantStatus(rec, http.StatusUnprocessableEntity, `value="`+bad+`"`, `aria-invalid="true"`)
	}

	// Blank is no mark: judge 1 gives no technical mark, the flight is saved
	// without it.
	form := e.classicForm("7,5")
	form.Set("mark."+e.judges[1]+".technical", " ")
	e.wantStatus(e.hx(e.base+"/102/simulate", form), 200, `id="provisional"`)
	form.Set("publish", "0")
	e.wantStatus(e.hx(e.base+"/102/save", form), 200)
	f, ok := e.stored("102")
	if !ok {
		t.Fatal("flight not saved")
	}
	for _, m := range f.Input.Marks {
		switch m.Judge {
		case e.judges[0]:
			if m.Technical == nil || *m.Technical != 7.5 {
				t.Errorf("judge 0 technical = %v, want 7.5", m.Technical)
			}
		case e.judges[1]:
			if m.Technical != nil {
				t.Errorf("judge 1 technical = %v, want no mark", *m.Technical)
			}
		}
	}
}

func TestJudgingSaveAndPublish(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, nil, true)
	page := e.get(e.base)
	e.wantStatus(page, 200, "Pilot 101", "Pilot 102", "Pilot 103", "not flown", `id="participants"`, `id="flight"`,
		`hx-post="`+e.base+`/101/simulate"`, `hx-sync="this:queue last"`, "js/judging.js")
	if strings.Contains(page.Body.String(), "chip--saved") {
		t.Fatal("a participant is marked saved before any save")
	}

	// htmx save: flash, participant list swapped out of band with the new
	// status, and the next participant's form.
	form := e.classicForm("7")
	form.Set("publish", "0")
	rec := e.hx(e.base+"/101/save", form)
	e.wantStatus(rec, 200, "Saved", `id="participants" hx-swap-oob="true"`, "chip--saved", `hx-post="`+e.base+`/102/simulate"`)
	if st, _ := e.svc.RunFlightStatuses(context.Background(), e.c.ID, 0); st["101"].Published || st["101"].Score == 0 {
		t.Fatalf("status after save = %+v", st["101"])
	}

	// Save & publish without JS (from the reloaded form: its version):
	// redirect to the next participant.
	saved, _ := e.stored("101")
	form.Set("version", saved.Version)
	form.Set("publish", "1")
	form.Del("_csrf")
	e.wantRedirect(e.post(e.base+"/101/save", form), e.base+"/102?ok=published")
	if st, _ := e.svc.RunFlightStatuses(context.Background(), e.c.ID, 0); !st["101"].Published {
		t.Fatal("flight not published")
	}
	next := e.get(e.base + "/102?ok=published")
	e.wantStatus(next, 200, "Published.", "chip--published", `hx-post="`+e.base+`/102/simulate"`)

	// The saved flight is prefilled when selected (htmx fragment).
	req := httptest.NewRequest(http.MethodGet, e.base+"/101", nil)
	req.Header.Set("HX-Request", "true")
	frag := e.do(req, &e.admin)
	e.wantStatus(frag, 200, `value="LM"`, `value="RMC"`, `name="mark.`+e.judges[0]+`.technical"`, "published")
	if strings.Contains(frag.Body.String(), "<html") {
		t.Fatal("htmx participant load returned a full page")
	}

	// The last participant stays selected after a save.
	form.Set("publish", "0")
	form.Del("version")
	e.wantRedirect(e.post(e.base+"/103/save", form), e.base+"/103?ok=saved")

	// Unknown participant.
	e.wantStatus(e.get(e.base+"/999"), 404)
}

// perTrickValue reads the value of a judge's per-trick mark input.
func perTrickValue(t *testing.T, body, judge string, k int) string {
	t.Helper()
	m := regexp.MustCompile(`name="mark\.` + judge + `\.t\.` + strconv.Itoa(k) + `"[^>]*value="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		return "<none>"
	}
	return m[1]
}

func trickValues(body string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`name="trick"[^>]*value="([^"]*)"`).FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestPerTrickMarksStayAligned(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, []string{"awt-2025"}, true)
	j0, j1 := e.judges[0], e.judges[1]

	// Pilot 101 is AWT: one technical column per trick, no global technical.
	page := e.get(e.base + "/101")
	e.wantStatus(page, 200, "per trick")
	if strings.Contains(page.Body.String(), `name="mark.`+j0+`.technical"`) {
		t.Fatal("AWT grid has a global technical column")
	}

	form := url.Values{"trick": {"LM", "RMC", "LS"}, "op": {"trick-remove:1"}}
	for k, v := range []string{"1", "2", "3"} {
		form.Set("mark."+j0+".t."+strconv.Itoa(k), v)
	}
	for k, v := range []string{"4", "5", "6"} {
		form.Set("mark."+j1+".t."+strconv.Itoa(k), v)
	}
	rec := e.hx(e.base+"/101/edit", form)
	e.wantStatus(rec, 200)
	body := rec.Body.String()
	if got := trickValues(body); !slices.Equal(got, []string{"LM", "LS"}) {
		t.Fatalf("tricks = %v, want [LM LS]", got)
	}
	want := map[string][]string{j0: {"1", "3", "<none>"}, j1: {"4", "6", "<none>"}}
	for j, vals := range want {
		for k, v := range vals {
			if got := perTrickValue(t, body, j, k); got != v {
				t.Errorf("judge %s trick %d = %q, want %q", j, k, got, v)
			}
		}
	}

	// Moving a trick moves its column; adding one adds an empty column.
	form = url.Values{"trick": {"LM", "LS"}, "op": {"trick-down:0"}}
	form.Set("mark."+j0+".t.0", "1")
	form.Set("mark."+j0+".t.1", "3")
	rec = e.hx(e.base+"/101/edit", form)
	if got := trickValues(rec.Body.String()); !slices.Equal(got, []string{"LS", "LM"}) {
		t.Fatalf("tricks after move = %v", got)
	}
	if a, b := perTrickValue(t, rec.Body.String(), j0, 0), perTrickValue(t, rec.Body.String(), j0, 1); a != "3" || b != "1" {
		t.Fatalf("judge 0 marks after move = %q %q, want 3 1", a, b)
	}
	form = url.Values{"trick": {"LM"}, "op": {"trick-add"}}
	form.Set("mark."+j0+".t.0", "1")
	rec = e.hx(e.base+"/101/edit", form)
	if got := trickValues(rec.Body.String()); !slices.Equal(got, []string{"LM", ""}) {
		t.Fatalf("tricks after add = %v", got)
	}
	if got := perTrickValue(t, rec.Body.String(), j0, 1); got != "" {
		t.Fatalf("new column = %q, want empty", got)
	}

	// Without JS the edit re-renders the page.
	form = url.Values{"trick": {"LM", "RMC"}, "op": {"trick-remove:0"}}
	e.wantStatus(e.post(e.base+"/101/edit", form), 200, "<html", `value="RMC"`)

	// Saving the aligned grid stores the per-trick marks in trick order.
	form = url.Values{"trick": {"LM", "LS"}, "publish": {"0"}}
	for _, j := range e.judges {
		form.Set("mark."+j+".t.0", "1")
		form.Set("mark."+j+".t.1", "3")
		form.Set("mark."+j+".choreography", "6")
		form.Set("mark."+j+".landing", "6")
	}
	e.wantStatus(e.hx(e.base+"/101/save", form), 200, "Saved")
	f, _ := e.stored("101")
	for _, m := range f.Input.Marks {
		if len(m.TechnicalPerTrick) != 2 || *m.TechnicalPerTrick[0] != 1 || *m.TechnicalPerTrick[1] != 3 {
			t.Fatalf("stored per-trick marks of %s = %v", m.Judge, m.TechnicalPerTrick)
		}
	}

	// Unknown acronyms are flagged on their input.
	form = url.Values{"trick": {"LM", "NOPE"}}
	rec = e.hx(e.base+"/101/simulate", form)
	e.wantStatus(rec, http.StatusUnprocessableEntity, "Unknown trick(s): NOPE")
	if !regexp.MustCompile(`id="err-trick-1"[^>]*>[^<]+<`).MatchString(rec.Body.String()) {
		t.Fatalf("unknown trick not flagged: %s", rec.Body.String())
	}
}

func TestRemoveWarningByIndex(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, nil, true)
	form := e.classicForm("7")
	form["w1"] = []string{"first", "second", "third"}
	form["w2"] = []string{"keep"}
	form.Set("op", "w1-remove:1")
	rec := e.hx(e.base+"/101/edit", form)
	e.wantStatus(rec, 200)
	var w1 []string
	for _, m := range regexp.MustCompile(`name="w1"[^>]*value="([^"]*)"`).FindAllStringSubmatch(rec.Body.String(), -1) {
		w1 = append(w1, m[1])
	}
	if !slices.Equal(w1, []string{"first", "third"}) {
		t.Fatalf("cat 1 warnings = %v, want [first third]", w1)
	}
	if !strings.Contains(rec.Body.String(), `value="keep"`) {
		t.Fatal("cat 2 warning lost")
	}

	// Saved: the remaining warnings, in order.
	form["w1"] = w1
	form.Del("op")
	form.Set("publish", "0")
	e.wantStatus(e.hx(e.base+"/101/save", form), 200, "Saved")
	f, _ := e.stored("101")
	if !slices.Equal(f.Input.Warnings1, []string{"first", "third"}) || !slices.Equal(f.Input.Warnings2, []string{"keep"}) {
		t.Fatalf("stored warnings = %v / %v", f.Input.Warnings1, f.Input.Warnings2)
	}
}

func TestSynchroDNSAndTipTouchesSaved(t *testing.T) {
	e := newJudging(t, domain.TypeSynchro, nil, true)
	ctx := context.Background()
	run, err := e.svc.GetRun(ctx, e.c.ID, 0)
	e.mustOK(err)
	team := run.Teams[0]
	uts, err := e.svc.ListUniqueTricks(ctx, 2025)
	e.mustOK(err)
	i := slices.IndexFunc(uts, func(u catalog.UniqueTrick) bool { return u.Synchro })
	if i < 0 {
		t.Fatal("no synchro trick in the 2025 catalog")
	}

	page := e.get(e.base + "/" + team)
	e.wantStatus(page, 200, "Team A", `name="tip_touches"`, `name="mark.`+e.judges[0]+`.synchro"`)

	form := url.Values{"trick": {uts[i].Acronym}, "dns": {"1"}, "tip_touches": {"2"}, "publish": {"1"}}
	for _, j := range e.judges {
		for _, col := range []string{"technical", "choreography", "landing", "synchro"} {
			form.Set("mark."+j+"."+col, "7")
		}
	}
	e.wantStatus(e.hx(e.base+"/"+team+"/simulate", form), 200, `id="provisional"`)
	e.wantStatus(e.hx(e.base+"/"+team+"/save", form), 200, "Published")
	f, ok := e.stored(team)
	if !ok || !f.Input.DidNotStart || f.Input.TipTouches != 2 || !f.Published {
		t.Fatalf("stored synchro flight: ok=%v dns=%v tip=%d published=%v", ok, f.Input.DidNotStart, f.Input.TipTouches, f.Published)
	}
	again := e.get(e.base + "/" + team)
	e.wantStatus(again, 200, `name="tip_touches" type="number" value="2"`, "DNS")
	if !regexp.MustCompile(`name="dns" value="1" checked`).MatchString(again.Body.String()) {
		t.Fatal("DNS not prefilled")
	}
	// A bad tip touches count is a field error.
	form.Set("tip_touches", "x")
	e.wantStatus(e.hx(e.base+"/"+team+"/simulate", form), http.StatusUnprocessableEntity, `id="err-tip_touches"`)
}

func TestJudgingGuards(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, nil, false)
	ctx := context.Background()

	// Unassigned scorer: 403 on every judging route.
	if rec := e.do(httptest.NewRequest(http.MethodGet, e.base, nil), &e.scorer); rec.Code != http.StatusForbidden {
		t.Fatalf("unassigned scorer GET = %d", rec.Code)
	}
	if rec := e.postAs(&e.scorer, e.base+"/101/simulate", e.classicForm("7"), true); rec.Code != http.StatusForbidden {
		t.Fatalf("unassigned scorer simulate = %d", rec.Code)
	}
	e.mustOK(e.deps.Auth.AssignCompetition(ctx, "scorer@example.org", e.c.ID))
	e.wantStatus(e.do(httptest.NewRequest(http.MethodGet, e.base, nil), &e.scorer), 200, "Pilot 101")

	// CSRF token required.
	if rec := e.postAs(&e.scorer, e.base+"/101/save", e.classicForm("7"), false); rec.Code != http.StatusForbidden {
		t.Fatalf("save without CSRF = %d", rec.Code)
	}

	// Simulate does not write.
	e.wantStatus(e.postAs(&e.scorer, e.base+"/101/simulate", e.classicForm("7"), true), 200, "Score")
	if st, _ := e.svc.RunFlightStatuses(ctx, e.c.ID, 0); len(st) != 0 {
		t.Fatalf("simulate stored flights: %v", st)
	}

	// Run not open: the domain message, typed values kept.
	form := e.classicForm("7")
	form.Set("publish", "0")
	rec := e.postAs(&e.scorer, e.base+"/101/save", form, true)
	e.wantStatus(rec, http.StatusConflict, "only be saved in an open run", `value="LM"`)
	rec = e.hx(e.base+"/101/save", form)
	e.wantStatus(rec, http.StatusConflict, "only be saved in an open run", `value="LM"`)

	// Body limit.
	big := url.Values{"trick": {strings.Repeat("x", 70<<10)}}
	if rec := e.postAs(&e.scorer, e.base+"/101/simulate", big, true); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body = %d", rec.Code)
	}
}

func TestJudgingRequestSync(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, nil, true)
	form := e.classicForm("7")
	form.Set("publish", "0")
	e.wantStatus(e.hx(e.base+"/101/save", form), 200)
	body := e.get(e.base + "/101").Body.String()
	// The live simulate queues on the form's queue; it never aborts a save.
	if !regexp.MustCompile(`<form id="flight-form"[^>]*hx-sync="this:queue last"`).MatchString(body) {
		t.Fatal("form does not queue its simulate requests")
	}
	// Save, edit and delete abort a pending simulate, and set hx-sync
	// themselves (an inherited "queue last" would make them wait).
	for _, re := range []string{
		`name="publish" value="0" hx-post="[^"]*/save"[^>]*hx-sync="closest form:replace"`,
		`name="publish" value="1" hx-post="[^"]*/save"[^>]*hx-sync="closest form:replace"`,
		`name="op" value="trick-add"[^>]*hx-sync="closest form:replace"`,
		`hx-post="[^"]*/delete"[^>]*hx-sync="closest form:replace"`,
		`hx-post="[^"]*/simulate"[^>]*hx-sync="closest form:queue last"`,
	} {
		if !regexp.MustCompile(re).MatchString(body) {
			t.Errorf("no %s", re)
		}
	}
	// Every "replace" request disables every enabled button of the form
	// while it runs, so no other button (a trick op, a shortcut) can abort
	// an in-flight save or delete.
	replace := 0
	for _, b := range regexp.MustCompile(`<button[^>]*>`).FindAllString(body, -1) {
		if !strings.Contains(b, `hx-sync="closest form:replace"`) {
			continue
		}
		replace++
		if !strings.Contains(b, `hx-disabled-elt="#flight-form button:not([disabled])"`) {
			t.Errorf("button does not disable the form's buttons while it runs: %s", b)
		}
	}
	if replace < 5 {
		t.Errorf("only %d replace buttons found", replace)
	}
}

func TestJudgingStaleSaveRefused(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, nil, true)
	// Two scorers load the new flight (no version) and both save.
	form := e.classicForm("7")
	form.Set("publish", "0")
	e.wantStatus(e.hx(e.base+"/101/save", form), 200, "Saved")
	stale := e.classicForm("9")
	stale.Set("publish", "0")
	rec := e.hx(e.base+"/101/save", stale)
	e.wantStatus(rec, http.StatusConflict, "This flight was changed by someone else", `value="9"`)

	// Two saves from the same loaded form (version 1): the second is refused.
	v1, _ := e.stored("101")
	for i, want := range []int{200, http.StatusConflict} {
		f := e.classicForm("8")
		f.Set("publish", "0")
		f.Set("version", v1.Version)
		f.Del("_csrf")
		if rec := e.post(e.base+"/101/save", f); rec.Code != want && !(want == 200 && rec.Code == http.StatusSeeOther) {
			t.Fatalf("save %d = %d, want %d: %.300s", i, rec.Code, want, rec.Body.String())
		}
	}
	got, _ := e.stored("101")
	if got.Version != "2" || *got.Input.Marks[0].Technical != 8 {
		t.Fatalf("stored version %s technical %v", got.Version, *got.Input.Marks[0].Technical)
	}
}

func TestJudgingDeleteFlight(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, nil, true)
	ctx := context.Background()
	form := e.classicForm("7")
	form.Set("publish", "1")
	e.wantStatus(e.hx(e.base+"/101/save", form), 200)
	f, _ := e.stored("101")
	e.wantStatus(e.get(e.base+"/101"), 200, "Delete flight")

	del := url.Values{"version": {f.Version}}
	// CSRF required.
	if rec := e.postAs(&e.admin, e.base+"/101/delete", del, false); rec.Code != http.StatusForbidden {
		t.Fatalf("delete without CSRF = %d", rec.Code)
	}
	// First post asks for confirmation, nothing deleted.
	e.wantStatus(e.hx(e.base+"/101/delete", url.Values{"version": {f.Version}}), 200, "Confirm delete")
	if _, ok := e.stored("101"); !ok {
		t.Fatal("deleted without confirmation")
	}
	// A stale version is refused.
	e.wantStatus(e.hx(e.base+"/101/delete", url.Values{"version": {"99"}, "confirm": {"1"}}), http.StatusConflict, "changed by someone else")
	// Confirmed: deleted, participant shown again as not flown.
	rec := e.hx(e.base+"/101/delete", url.Values{"version": {f.Version}, "confirm": {"1"}})
	e.wantStatus(rec, 200, "Flight deleted.", `id="participants" hx-swap-oob="true"`)
	if _, ok := e.stored("101"); ok {
		t.Fatal("flight not deleted")
	}
	if st, _ := e.svc.RunFlightStatuses(ctx, e.c.ID, 0); len(st) != 0 {
		t.Fatalf("statuses after delete: %v", st)
	}

	// Closed run: refused with the domain message.
	for _, p := range []string{"101", "102", "103"} {
		form := e.classicForm("7")
		form.Set("publish", "1")
		e.wantStatus(e.hx(e.base+"/"+p+"/save", form), 200)
	}
	e.mustOK(e.svc.CloseRun(ctx, e.c.ID, 0))
	f, _ = e.stored("101")
	e.wantStatus(e.hx(e.base+"/101/delete", url.Values{"version": {f.Version}, "confirm": {"1"}}), http.StatusConflict, "open run")
	if _, ok := e.stored("101"); !ok {
		t.Fatal("flight deleted in a closed run")
	}
}

func TestJudgingRawPerTrickMarks(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, []string{"awt-2025"}, true)
	j0, j1 := e.judges[0], e.judges[1]
	form := url.Values{"trick": {"LM", "RMC"}, "publish": {"0"}}
	for _, j := range e.judges {
		form.Set("mark."+j+".t.0", "7")
		form.Set("mark."+j+".choreography", "6")
		form.Set("mark."+j+".landing", "6")
	}
	form.Set("mark."+j0+".t.1", "8")
	form.Set("mark."+j1+".t.1", "") // blank: guessed by the engine
	e.wantStatus(e.hx(e.base+"/101/save", form), 200, "Saved")
	page := e.get(e.base + "/101").Body.String()
	if got := perTrickValue(t, page, j1, 1); got != "" {
		t.Fatalf("blank cell reopened as %q", got)
	}
	if got := perTrickValue(t, page, j0, 1); got != "8" {
		t.Fatalf("typed cell reopened as %q", got)
	}

	// Junk under a blank trick row is flagged instead of dropped.
	junk := url.Values{"trick": {"LM", ""}}
	junk.Set("mark."+j0+".t.1", "7")
	rec := e.hx(e.base+"/101/simulate", junk)
	e.wantStatus(rec, http.StatusUnprocessableEntity)
	if !regexp.MustCompile(`id="err-m0-t1"[^>]*>No trick in this row<`).MatchString(rec.Body.String()) {
		t.Fatalf("junk cell not flagged: %s", rec.Body.String())
	}
}

func TestJudgingCSRFAndUnknownParticipant(t *testing.T) {
	e := newJudging(t, domain.TypeSolo, nil, true)
	for _, p := range []string{"/101/simulate", "/101/edit", "/101/save", "/101/delete"} {
		if rec := e.postAs(&e.admin, e.base+p, e.classicForm("7"), false); rec.Code != http.StatusForbidden {
			t.Errorf("POST %s without CSRF = %d", p, rec.Code)
		}
	}
	form := e.classicForm("7")
	form.Set("publish", "1")
	e.wantStatus(e.post(e.base+"/999/save", form), http.StatusNotFound)
	if st, _ := e.svc.RunFlightStatuses(context.Background(), e.c.ID, 0); len(st) != 0 {
		t.Fatalf("unknown participant wrote flights: %v", st)
	}
}
