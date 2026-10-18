package admin_test

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/service"
)

func date(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

// setupData creates two judges and the pilots, and returns the judge ids.
func (e *env) setupData(pilots ...int) []string {
	e.t.Helper()
	ctx := context.Background()
	e.importPilots(pilots...)
	var ids []string
	for _, n := range []string{"Judge One", "Judge Two", "Judge Three"} {
		j, err := e.svc.CreateJudge(ctx, service.JudgeInput{Name: n, Country: "fra", Level: domain.JudgeSenior})
		if err != nil {
			e.t.Fatal(err)
		}
		ids = append(ids, j.ID)
	}
	return ids
}

func (e *env) mustOK(err error) {
	e.t.Helper()
	if err != nil {
		e.t.Fatal(err)
	}
}

// soloComp creates a solo competition through the service.
func (e *env) soloComp(name, start string, seasons ...string) service.Competition {
	e.t.Helper()
	c, err := e.svc.CreateCompetition(context.Background(), service.CompetitionInput{
		Name: name, Type: domain.TypeSolo, StartDate: date(start), EndDate: date(start), Location: "Annecy", Seasons: seasons,
	})
	e.mustOK(err)
	return c
}

func TestCompetitionSetupFlow(t *testing.T) {
	e := newEnv(t)
	judges := e.setupData(101, 102, 103)

	// Creation works without ticking "published" and starts in init.
	rec := e.post("/admin/competitions", url.Values{"name": {"Test Cup 2026"}, "type": {"solo"},
		"start_date": {"2026-06-01"}, "end_date": {"2026-06-03"}, "location": {"Annecy"}})
	e.wantRedirect(rec, "/admin/competitions/test-cup-2026")

	// The list shows the unpublished competition of the year.
	e.wantStatus(e.get("/admin/competitions"), 200, "Test Cup 2026", "unpublished", "init", "2026")

	page := e.get("/admin/competitions/test-cup-2026")
	e.wantStatus(page, 200, "Test Cup 2026", "/state/open")

	// Participants: search, add three pilots, remove one, add it back.
	e.wantStatus(e.get("/admin/competitions/test-cup-2026/participants?q=102"), 200, "Pilot 102")
	for _, id := range []string{"101", "102", "103"} {
		e.wantRedirect(e.post("/admin/competitions/test-cup-2026/participants", url.Values{"op": {"add"}, "id": {id}}),
			"/admin/competitions/test-cup-2026/participants")
	}
	e.wantRedirect(e.post("/admin/competitions/test-cup-2026/participants", url.Values{"op": {"remove"}, "id": {"103"}}), "/admin/competitions/test-cup-2026/participants")
	c, err := e.svc.GetCompetitionByCode(context.Background(), "test-cup-2026")
	e.mustOK(err)
	if len(c.Pilots) != 2 {
		t.Fatalf("pilots = %v, want 2 after the removal", c.Pilots)
	}
	e.post("/admin/competitions/test-cup-2026/participants", url.Values{"op": {"add"}, "id": {"103"}})

	// Judges, repeatable tricks, settings.
	e.wantStatus(e.get("/admin/competitions/test-cup-2026/judges"), 200, "Judge One")
	e.wantRedirect(e.post("/admin/competitions/test-cup-2026/judges", url.Values{"judge": judges[:2]}), "/admin/competitions/test-cup-2026/judges")
	e.wantStatus(e.get("/admin/competitions/test-cup-2026/tricks"), 200, "Repeatable")
	cfg := e.get("/admin/competitions/test-cup-2026/config")
	e.wantStatus(cfg, 200, `name="warning2"`, `name="warnings_to_dsq"`, `name="mark_percentages.synchro.synchro"`, "points deducted per warning")
	form := url.Values{}
	for _, m := range regexp.MustCompile(`name="([^"]+)"[^>]*value="([^"]*)"`).FindAllStringSubmatch(cfg.Body.String(), -1) {
		if m[1] != "_csrf" {
			form.Set(m[1], m[2])
		}
	}
	form.Set("warning2", "1.5")
	e.wantRedirect(e.post("/admin/competitions/test-cup-2026/config", form), "/admin/competitions/test-cup-2026/config")
	c, _ = e.svc.GetCompetitionByCode(context.Background(), "test-cup-2026")
	if !strings.Contains(string(c.Config), `"warning2":1.5`) {
		t.Fatalf("warning2 not saved: %s", c.Config)
	}
	// An invalid config (percentages not summing to 100) is a field error.
	bad := url.Values{}
	for k, v := range form {
		bad[k] = v
	}
	bad.Set("mark_percentages.solo.technical", "10")
	e.wantStatus(e.post("/admin/competitions/test-cup-2026/config", bad), http.StatusUnprocessableEntity, "sum to")

	// Open with confirmation page, then the state flips.
	e.wantStatus(e.get("/admin/competitions/test-cup-2026/state/open"), 200, "Confirm open")
	e.wantRedirect(e.post("/admin/competitions/test-cup-2026/state/open", nil), "/admin/competitions/test-cup-2026")
	e.wantStatus(e.get("/admin/competitions/test-cup-2026"), 200, "badge--open", "New run")
	// Name, code and type are frozen once open; an edit keeps them.
	e.wantStatus(e.get("/admin/competitions/test-cup-2026/edit"), 200, "can no longer change")
	e.wantRedirect(e.post("/admin/competitions/test-cup-2026/edit", url.Values{"name": {"Hacked"}, "start_date": {"2026-06-01"},
		"end_date": {"2026-06-04"}, "location": {"Chamonix"}, "published": {"1"}}), "/admin/competitions/test-cup-2026")
	c, _ = e.svc.GetCompetitionByCode(context.Background(), "test-cup-2026")
	if c.Name != "Test Cup 2026" || c.Location != "Chamonix" || !c.Published {
		t.Fatalf("after edit: %q %q published=%v", c.Name, c.Location, c.Published)
	}

	// New run: form explains the policies, run 1 follows the competition order.
	e.wantStatus(e.get("/admin/competitions/test-cup-2026/runs/new"), 200, "Repetition reset policy", "AWT")
	rec = e.post("/admin/competitions/test-cup-2026/runs", url.Values{"qualify": {"0"}, "reset_policy": {"0"}})
	e.wantRedirect(rec, "/admin/competitions/test-cup-2026/runs/1")
	run := e.get("/admin/competitions/test-cup-2026/runs/1")
	e.wantStatus(run, 200, "Starting order", "/runs/1/judging", "starting-order.html", "Run settings")
	if r0, _ := e.svc.GetRun(context.Background(), c.ID, 0); len(r0.Pilots) != 3 {
		t.Fatalf("run pilots = %v", r0.Pilots)
	}
	// Move the second pilot up with the plain form, then down with htmx.
	r2, _ := e.svc.GetRun(context.Background(), c.ID, 0)
	secondID := r2.Pilots[1]
	e.wantRedirect(e.post("/admin/competitions/test-cup-2026/runs/1/move", url.Values{"id": {strconv.Itoa(secondID)}, "dir": {"up"}}), "/admin/competitions/test-cup-2026/runs/1")
	r3, _ := e.svc.GetRun(context.Background(), c.ID, 0)
	if r3.Pilots[0] != secondID {
		t.Fatalf("pilot %d not moved first: %v", secondID, r3.Pilots)
	}
	req := newReq(http.MethodPost, "/admin/competitions/test-cup-2026/runs/1/move", url.Values{"id": {strconv.Itoa(secondID)}, "dir": {"down"}, "_csrf": {e.admin.CSRF}})
	req.Header.Set("HX-Request", "true")
	hx := e.do(req, &e.admin)
	e.wantStatus(hx, 200, `id="starting-order"`)
	if strings.Contains(hx.Body.String(), "<html") {
		t.Fatal("htmx move returned a full page")
	}
	r4, _ := e.svc.GetRun(context.Background(), c.ID, 0)
	if r4.Pilots[1] != secondID {
		t.Fatalf("pilot %d not moved back down: %v", secondID, r4.Pilots)
	}
	// Moving the first pilot up is refused with a message.
	e.wantStatus(e.post("/admin/competitions/test-cup-2026/runs/1/move", url.Values{"id": {strconv.Itoa(r4.Pilots[0])}, "dir": {"up"}}), 422, "not possible")

	// Run judges and config, then open the run.
	e.wantRedirect(e.post("/admin/competitions/test-cup-2026/runs/1/judges", url.Values{"judge": {judges[0]}}), "/admin/competitions/test-cup-2026/runs/1")
	r5, _ := e.svc.GetRun(context.Background(), c.ID, 0)
	if len(r5.Judges) != 1 || r5.Judges[0] != judges[0] {
		t.Fatalf("run judges = %v", r5.Judges)
	}
	e.wantStatus(e.post("/admin/competitions/test-cup-2026/runs/1/judges", url.Values{"judge": {judges[2]}}), 422, "not a judge of the competition")
	e.wantRedirect(e.post("/admin/competitions/test-cup-2026/runs/1/config", form), "/admin/competitions/test-cup-2026/runs/1")
	e.wantStatus(e.get("/admin/competitions/test-cup-2026/runs/1/state/open"), 200, "Confirm open")
	e.wantRedirect(e.post("/admin/competitions/test-cup-2026/runs/1/state/open", nil), "/admin/competitions/test-cup-2026/runs/1")
	e.wantStatus(e.get("/admin/competitions/test-cup-2026/runs/1"), 200, "badge--open", "Close run")

	// Closing the run is refused with the names of the participants.
	e.wantStatus(e.post("/admin/competitions/test-cup-2026/runs/1/state/close", nil), 409, "without a published flight")
	e.wantStatus(e.get("/admin/competitions/test-cup-2026/runs/9"), 404)
}

func newReq(method, path string, form url.Values) *http.Request {
	r, _ := http.NewRequest(method, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestOpenRefusedWithFewJudges(t *testing.T) {
	e := newEnv(t)
	judges := e.setupData(101, 102)
	c := e.soloComp("Small Cup", "2026-05-01")
	e.mustOK(e.svc.SetParticipants(context.Background(), c.ID, []int{101, 102}, nil))
	e.mustOK(e.svc.SetJudges(context.Background(), c.ID, judges[:1]))

	rec := e.post("/admin/competitions/small-cup/state/open", nil)
	e.wantStatus(rec, http.StatusUnprocessableEntity, "at least 2 judges are needed")
	got, _ := e.svc.GetCompetitionByCode(context.Background(), "small-cup")
	if got.State != domain.CompetitionInit {
		t.Fatalf("state = %s, want init", got.State)
	}
	// An open competition cannot be closed while a run is not closed.
	e.mustOK(e.svc.SetJudges(context.Background(), c.ID, judges[:2]))
	e.wantRedirect(e.post("/admin/competitions/small-cup/state/open", nil), "/admin/competitions/small-cup")
	e.wantRedirect(e.post("/admin/competitions/small-cup/runs", url.Values{"qualify": {"0"}, "reset_policy": {"0"}}), "/admin/competitions/small-cup/runs/1")
	e.wantStatus(e.post("/admin/competitions/small-cup/state/close", nil), http.StatusConflict, "uns not closed: 1")
	// A bad qualify value is a field error.
	e.wantStatus(e.post("/admin/competitions/small-cup/runs", url.Values{"qualify": {"-1"}, "reset_policy": {"0"}}), http.StatusUnprocessableEntity, "must be &gt;= 0")
}

func TestFrozenCompetitionIsReadOnly(t *testing.T) {
	e := newEnv(t)
	judges := e.setupData(101, 102)
	c := e.soloComp("Old Cup", "2026-04-01")
	e.mustOK(e.svc.SetParticipants(context.Background(), c.ID, []int{101, 102}, nil))
	e.mustOK(e.svc.SetJudges(context.Background(), c.ID, judges[:2]))
	e.mustOK(e.svc.OpenCompetition(context.Background(), c.ID))
	_, err := e.svc.NewRun(context.Background(), c.ID, 0, scoring.ResetNone)
	e.mustOK(err)
	if _, err := e.deps.DB.Write.Exec(`UPDATE competitions SET frozen = 1 WHERE id = ?`, c.ID); err != nil {
		t.Fatal(err)
	}
	show := e.get("/admin/competitions/old-cup")
	e.wantStatus(show, 200, "frozen", "read-only")
	e.wantStatus(e.get("/admin/competitions/old-cup/runs/1"), 200, "frozen")
	for _, forbidden := range []string{"/state/open", "/delete", "/runs/new"} {
		if strings.Contains(show.Body.String(), forbidden) {
			t.Fatalf("frozen page offers %s", forbidden)
		}
	}
	for _, p := range []string{"edit", "participants", "judges", "tricks", "config", "state/open", "delete", "runs/new"} {
		e.wantStatus(e.get("/admin/competitions/old-cup/"+p), 200, "frozen")
	}
	list := e.get("/admin/competitions?year=2026")
	e.wantStatus(list, 200, "Old Cup", "frozen")
	for _, p := range []struct {
		path string
		form url.Values
	}{
		{"/admin/competitions/old-cup/participants", url.Values{"op": {"add"}, "id": {"101"}}},
		{"/admin/competitions/old-cup/judges", url.Values{}},
		{"/admin/competitions/old-cup/tricks", url.Values{}},
		{"/admin/competitions/old-cup/edit", url.Values{"name": {"Old Cup"}, "start_date": {"2026-04-01"}, "end_date": {"2026-04-01"}}},
		{"/admin/competitions/old-cup/state/open", nil},
		{"/admin/competitions/old-cup/delete", nil},
		{"/admin/competitions/old-cup/runs", url.Values{"qualify": {"0"}, "reset_policy": {"0"}}},
		{"/admin/competitions/old-cup/config", url.Values{}},
		{"/admin/competitions/old-cup/runs/1/move", url.Values{"id": {"101"}, "dir": {"up"}}},
		{"/admin/competitions/old-cup/runs/1/judges", url.Values{}},
		{"/admin/competitions/old-cup/runs/1/config", url.Values{}},
		{"/admin/competitions/old-cup/runs/1/state/open", nil},
	} {
		e.wantStatus(e.post(p.path, p.form), http.StatusConflict, "frozen")
	}
}

func TestScorerAccess(t *testing.T) {
	e := newEnv(t)
	judges := e.setupData(101, 102)
	ctx := context.Background()
	c := e.soloComp("Scored Cup", "2026-05-01")
	e.mustOK(e.svc.SetParticipants(ctx, c.ID, []int{101, 102}, nil))
	e.mustOK(e.svc.SetJudges(ctx, c.ID, judges[:2]))
	e.mustOK(e.svc.OpenCompetition(ctx, c.ID))
	_, err := e.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone)
	e.mustOK(err)

	sget := func(path string) int {
		return e.do(newReq(http.MethodGet, path, nil), &e.scorer).Code
	}
	// Not assigned: nothing is reachable.
	for _, p := range []string{"/admin/competitions/scored-cup", "/admin/competitions/scored-cup/runs/1",
		"/admin/competitions/scored-cup/export/results.html"} {
		if code := sget(p); code != http.StatusForbidden {
			t.Fatalf("unassigned scorer GET %s = %d, want 403", p, code)
		}
	}
	e.mustOK(e.deps.Auth.AssignCompetition(ctx, "scorer@example.org", "scored-cup"))
	for _, p := range []string{"/admin/competitions/scored-cup", "/admin/competitions/scored-cup/runs/1",
		"/admin/competitions/scored-cup/runs/1/starting-order.html", "/admin/competitions/scored-cup/export/results.html",
		"/admin/competitions/scored-cup/export/results.xlsx", "/admin/competitions/scored-cup/runs/1/export/results.html"} {
		if code := sget(p); code != http.StatusOK {
			t.Fatalf("assigned scorer GET %s = %d, want 200", p, code)
		}
	}
	// The run page is read-only for scorers: no state or order buttons.
	body := e.do(newReq(http.MethodGet, "/admin/competitions/scored-cup/runs/1", nil), &e.scorer).Body.String()
	for _, forbidden := range []string{"/state/", "/move", "Save run judges", "Save settings"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("scorer run page offers %q", forbidden)
		}
	}
	// Setup stays admin-only.
	for _, p := range []string{"/admin/competitions", "/admin/competitions/new", "/admin/competitions/scored-cup/edit",
		"/admin/competitions/scored-cup/participants", "/admin/competitions/scored-cup/judges", "/admin/competitions/scored-cup/config",
		"/admin/competitions/scored-cup/state/close", "/admin/competitions/scored-cup/runs/new", "/admin/competitions/scored-cup/runs/1/state/open"} {
		if code := sget(p); code != http.StatusForbidden {
			t.Fatalf("scorer GET %s = %d, want 403", p, code)
		}
	}
	for _, p := range []string{"/admin/competitions", "/admin/competitions/scored-cup/edit", "/admin/competitions/scored-cup/judges",
		"/admin/competitions/scored-cup/participants", "/admin/competitions/scored-cup/config", "/admin/competitions/scored-cup/state/close",
		"/admin/competitions/scored-cup/runs", "/admin/competitions/scored-cup/runs/1/move", "/admin/competitions/scored-cup/runs/1/judges",
		"/admin/competitions/scored-cup/runs/1/state/open", "/admin/competitions/scored-cup/delete"} {
		if rec := e.postAs(&e.scorer, p, url.Values{}, true); rec.Code != http.StatusForbidden {
			t.Fatalf("scorer POST %s = %d, want 403", p, rec.Code)
		}
	}
}

func TestExports(t *testing.T) {
	e := newEnv(t)
	judges := e.setupData(101, 102)
	ctx := context.Background()
	cat, err := os.ReadFile("../../../testdata/catalog/2025.json")
	e.mustOK(err)
	_, err = e.svc.ImportCatalog(ctx, 2025, bytes.NewReader(cat))
	e.mustOK(err)
	c := e.soloComp("Export Cup 2025", "2025-06-01")
	e.mustOK(e.svc.SetParticipants(ctx, c.ID, []int{101, 102}, nil))
	e.mustOK(e.svc.SetJudges(ctx, c.ID, judges[:2]))
	e.mustOK(e.svc.OpenCompetition(ctx, c.ID))
	_, err = e.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone)
	e.mustOK(err)
	const base = "/admin/competitions/export-cup-2025"

	// Run not final: xlsx is a 400 with the reason, the print page works.
	e.wantStatus(e.get(base+"/runs/1/export/results.xlsx"), http.StatusBadRequest, "not final")
	html := e.get(base + "/runs/1/export/results.html")
	e.wantStatus(html, 200, "run 1 intermediate results")
	if ct := html.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content type %q", ct)
	}
	if csp := html.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "style-src 'unsafe-inline'") || strings.Contains(csp, "script-src") {
		t.Fatalf("print CSP = %q", csp)
	}
	if cc := html.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	e.wantStatus(e.get(base+"/runs/1/starting-order.html"), 200, "Starting Order - run 1", "Pilot 101", "Pilot 102")

	e.mustOK(e.svc.OpenRun(ctx, c.ID, 0))
	// Flights published and run closed: final xlsx, with the safe filename.
	ptr := func(v float64) *float64 { return &v }
	for _, p := range []string{"101", "102"} {
		in := service.FlightInput{Tricks: []string{"LM", "RMC"}}
		for _, j := range judges[:2] {
			in.Marks = append(in.Marks, service.MarkInput{Judge: j, Technical: ptr(7), Choreography: ptr(6), Landing: ptr(6),
				TechnicalPerTrick: []*float64{ptr(7), ptr(7)}})
		}
		_, err := e.svc.SaveFlight(ctx, c.ID, 0, p, in, true, "")
		e.mustOK(err)
	}
	e.mustOK(e.svc.CloseRun(ctx, c.ID, 0))
	x := e.get(base + "/runs/1/export/results.xlsx")
	e.wantStatus(x, 200)
	if ct := x.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("xlsx content type %q", ct)
	}
	if cd := x.Header().Get("Content-Disposition"); cd != `attachment; filename="export-cup-2025-run-1-results.xlsx"` {
		t.Fatalf("content disposition %q", cd)
	}
	if !bytes.HasPrefix(x.Body.Bytes(), []byte("PK")) {
		t.Fatal("xlsx is not a zip")
	}
	comp := e.get(base + "/export/results.xlsx")
	e.wantStatus(comp, 200)
	if cd := comp.Header().Get("Content-Disposition"); cd != `attachment; filename="export-cup-2025-results.xlsx"` {
		t.Fatalf("competition content disposition %q", cd)
	}
	e.wantStatus(e.get(base+"/export/results.html"), 200, "Pilot 101", "Pilot 102")
	e.wantStatus(e.get(base+"/export/results.html?limit_run=1"), 200, "after run 1")
	e.wantStatus(e.get(base+"/export/results.xlsx?limit_run=1"), 200)
	e.wantStatus(e.get(base+"/export/results.html?limit_run=7"), http.StatusBadRequest)
	e.wantStatus(e.get(base+"/export/results.html?limit_run=x"), http.StatusBadRequest)
	e.wantStatus(e.get(base+"/runs/2/export/results.html"), http.StatusNotFound)
	e.wantStatus(e.get("/admin/competitions/nope/export/results.html"), http.StatusNotFound)

	// Unpublished flights stay out of the run exports.
	e.mustOK(e.svc.ReopenRun(ctx, c.ID, 0))
	_, err = e.svc.SaveFlight(ctx, c.ID, 0, "102", service.FlightInput{DidNotStart: true}, false, "1")
	e.mustOK(err)
	out := e.get(base + "/runs/1/export/results.html")
	e.wantStatus(out, 200, "Pilot 101")
	if strings.Contains(out.Body.String(), "Pilot 102") {
		t.Fatal("run export lists an unpublished flight")
	}
	e.wantStatus(e.get(base+"/runs/1/export/results.xlsx"), http.StatusBadRequest, "not final")
}
