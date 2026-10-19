package admin_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"awt/internal/auth"
	"awt/internal/blob"
	"awt/internal/cache"
	"awt/internal/civl"
	"awt/internal/config"
	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/store"
	"awt/internal/store/gen"
	"awt/internal/web"
	"awt/internal/web/admin"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

type fakeCIVL struct{}

func (fakeCIVL) FetchPilot(_ context.Context, id int) (civl.PilotData, error) {
	if id >= 900000 {
		return civl.PilotData{}, &civl.StatusError{Method: "GET", URL: "x", Status: http.StatusNotFound}
	}
	name := fmt.Sprintf("Pilot %d", id)
	if id == 666 {
		name = "<script>alert(1)</script>"
	}
	return civl.PilotData{CivlID: id, Name: name, Country: "fra",
		CivlLink: fmt.Sprintf("https://civlcomps.org/pilot/%d", id)}, nil
}

func (fakeCIVL) FetchRankings(context.Context) (map[int]int, error) {
	return map[int]int{101: 7, 102: 3}, nil
}

type env struct {
	t      *testing.T
	h      http.Handler
	deps   web.Deps
	svc    *service.Service
	admin  auth.Session
	scorer auth.Session
}

func newEnv(t *testing.T) *env {
	t.Helper()
	t.Parallel()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.NewFS(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: "dev", Addr: ":0"}
	svc := service.New(db, blobs, service.NewBus(), fakeCIVL{})
	deps := web.Deps{Cfg: cfg, Auth: auth.New(db, cfg), DB: db, Purger: cache.NoopPurger{}, Blobs: blobs, Service: svc}
	e := &env{t: t, deps: deps, svc: svc, h: web.Routes(deps, admin.Module{})}
	ctx := context.Background()
	for _, u := range []struct{ email, role string }{{"admin@example.org", auth.RoleAdmin}, {"scorer@example.org", auth.RoleScorer}} {
		if _, err := deps.Auth.CreateUser(ctx, u.email, u.role, u.role, "correct horse battery"); err != nil {
			t.Fatal(err)
		}
	}
	if e.admin, err = deps.Auth.Login(ctx, "admin@example.org", "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	if e.scorer, err = deps.Auth.Login(ctx, "scorer@example.org", "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) do(req *http.Request, s *auth.Session) *httptest.ResponseRecorder {
	if s != nil {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: s.Token})
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func (e *env) get(path string) *httptest.ResponseRecorder {
	return e.do(httptest.NewRequest(http.MethodGet, path, nil), &e.admin)
}

// post sends a form with the admin session and its CSRF token.
func (e *env) post(path string, form url.Values) *httptest.ResponseRecorder {
	return e.postAs(&e.admin, path, form, true)
}

func (e *env) postAs(s *auth.Session, path string, form url.Values, withCSRF bool) *httptest.ResponseRecorder {
	if form == nil {
		form = url.Values{}
	}
	if withCSRF {
		form.Set("_csrf", s.CSRF)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return e.do(req, s)
}

func (e *env) upload(path string, fields map[string]string, filename string, content []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("_csrf", e.admin.CSRF)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if filename != "" {
		fw, _ := mw.CreateFormFile("file", filename)
		_, _ = fw.Write(content)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return e.do(req, &e.admin)
}

func (e *env) wantStatus(rec *httptest.ResponseRecorder, code int, contains ...string) {
	e.t.Helper()
	if rec.Code != code {
		e.t.Fatalf("status = %d, want %d; body: %.400s", rec.Code, code, rec.Body.String())
	}
	for _, c := range contains {
		if !strings.Contains(rec.Body.String(), c) {
			e.t.Fatalf("body lacks %q: %.600s", c, rec.Body.String())
		}
	}
}

func (e *env) wantRedirect(rec *httptest.ResponseRecorder, prefix string) {
	e.t.Helper()
	if rec.Code != http.StatusSeeOther {
		e.t.Fatalf("status = %d, want 303; body: %.400s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, prefix) {
		e.t.Fatalf("Location = %q, want prefix %q", loc, prefix)
	}
}

func (e *env) importPilots(ids ...int) {
	e.t.Helper()
	for _, id := range ids {
		e.wantRedirect(e.post("/manager/pilots/import", url.Values{"civlid": {fmt.Sprint(id)}}), "/manager/pilots")
	}
}

var idInHref = func(prefix string) *regexp.Regexp { return regexp.MustCompile(prefix + `([0-9a-f]{24})`) }

func firstID(t *testing.T, body, prefix string) string {
	t.Helper()
	m := idInHref(prefix).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no %s<id> in %.500s", prefix, body)
	}
	return m[1]
}

func TestPilots(t *testing.T) {
	e := newEnv(t)
	e.importPilots(101, 102)
	list := e.get("/manager/pilots")
	e.wantStatus(list, 200, "Pilot 101", "Pilot 102", "unranked")

	// Search by CIVL id and by (accent-insensitive) name.
	rec := e.get("/manager/pilots?q=102")
	e.wantStatus(rec, 200, "Pilot 102")
	if strings.Contains(rec.Body.String(), "Pilot 101") {
		t.Fatal("search by id returned another pilot")
	}
	e.wantStatus(e.get("/manager/pilots?q=PILÓT+101"), 200) // no crash on accents, no regex
	e.wantStatus(e.get("/manager/pilots?q=%5B"), 200)       // "[" is plain text, not a regex

	// Unknown CIVL id: clear message, 422.
	e.wantStatus(e.post("/manager/pilots/import", url.Values{"civlid": {"900001"}}), 422, "900001 was not found on civlcomps.org")
	e.wantStatus(e.post("/manager/pilots/import", url.Values{"civlid": {"abc"}}), 422, "positive CIVL id")

	// Rankings.
	e.wantRedirect(e.post("/manager/pilots/rankings", nil), "/manager/pilots")
	p, _ := e.svc.GetPilot(context.Background(), 101)
	if p.Rank != 7 {
		t.Fatalf("rank = %d", p.Rank)
	}

	// Gender toggle: set man first (imports leave it unset).
	if err := e.deps.DB.Tx(context.Background(), func(q *gen.Queries) error {
		return q.UpdatePilotGender(context.Background(), gen.UpdatePilotGenderParams{Gender: "man", Civlid: 101})
	}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/manager/pilots/101/gender", strings.NewReader(url.Values{"_csrf": {e.admin.CSRF}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec = e.do(req, &e.admin)
	e.wantStatus(rec, 200, `id="pilot-101"`, "woman")
	if strings.Contains(rec.Body.String(), "<html") {
		t.Fatal("htmx response must be the row only")
	}

	// AWT year toggle for the current and next year, then any year.
	year := time.Now().UTC().Year()
	for _, y := range []int{year, year + 1} {
		rec = e.post("/manager/pilots/101/awt", url.Values{"year": {fmt.Sprint(y)}, "awt": {"1"}})
		e.wantRedirect(rec, "/manager/pilots")
	}
	e.wantRedirect(e.post("/manager/pilots/awt", url.Values{"civlid": {"101"}, "year": {"2022"}, "awt": {"1"}}), "/manager/pilots")
	p, _ = e.svc.GetPilot(context.Background(), 101)
	if len(p.AWTYears) != 3 {
		t.Fatalf("AWT years = %v", p.AWTYears)
	}
	e.wantRedirect(e.post("/manager/pilots/101/awt", url.Values{"year": {fmt.Sprint(year)}, "awt": {"0"}}), "/manager/pilots")
	p, _ = e.svc.GetPilot(context.Background(), 101)
	if len(p.AWTYears) != 2 {
		t.Fatalf("AWT years after removal = %v", p.AWTYears)
	}
	e.wantStatus(e.post("/manager/pilots/999/gender", nil), 404)
}

func TestTeams(t *testing.T) {
	e := newEnv(t)
	e.importPilots(101, 102, 103)
	e.wantStatus(e.get("/manager/teams/new"), 200, `list="pilot-options"`, "Pilot 101 (101)")

	rec := e.post("/manager/teams", url.Values{"name": {"Duo"}, "pilot1": {"Pilot 101 (101)"}, "pilot2": {"102"}})
	e.wantRedirect(rec, "/manager/teams")
	list := e.get("/manager/teams")
	e.wantStatus(list, 200, "Duo", "Pilot 101 &amp; Pilot 102")
	id := firstID(t, list.Body.String(), "/manager/teams/")

	// Validation: same pilot twice, missing pilot, unknown pilot.
	e.wantStatus(e.post("/manager/teams", url.Values{"name": {"Twin"}, "pilot1": {"101"}, "pilot2": {"101"}}), 422, "the two pilots must be different")
	e.wantStatus(e.post("/manager/teams", url.Values{"name": {"Solo"}, "pilot1": {"101"}, "pilot2": {""}}), 422, "pick a pilot")
	e.wantStatus(e.post("/manager/teams", url.Values{"name": {"Ghost"}, "pilot1": {"101"}, "pilot2": {"555"}}), 422, "pilot 555 is unknown")
	e.wantStatus(e.post("/manager/teams", url.Values{"name": {""}, "pilot1": {"101"}, "pilot2": {"102"}}), 422)
	// The typed values are shown again.
	e.wantStatus(e.post("/manager/teams", url.Values{"name": {"Keep me"}, "pilot1": {"101"}, "pilot2": {"101"}}), 422, `value="Keep me"`)
	e.wantStatus(e.post("/manager/teams", url.Values{"name": {"Duo"}, "pilot1": {"101"}, "pilot2": {"103"}}), 409, "already exists")

	// Edit.
	e.wantStatus(e.get("/manager/teams/"+id+"/edit"), 200, `value="Duo"`, "Pilot 102 (102)")
	e.wantRedirect(e.post("/manager/teams/"+id, url.Values{"name": {"Trio"}, "pilot1": {"101"}, "pilot2": {"103"}}), "/manager/teams")
	e.wantStatus(e.get("/manager/teams"), 200, "Trio")

	// Delete needs the in-page confirmation page.
	e.wantStatus(e.get("/manager/teams/"+id+"/delete"), 200, "Confirm delete", "Trio")
	e.wantRedirect(e.post("/manager/teams/"+id+"/delete", nil), "/manager/teams")
	e.wantStatus(e.get("/manager/teams/"+id+"/edit"), 404)
}

func TestJudges(t *testing.T) {
	e := newEnv(t)
	e.importPilots(101)
	rec := e.post("/manager/judges", url.Values{"name": {"Jane Judge"}, "level": {"senior"}, "country": {"FRA"}, "civlid": {"101"}})
	e.wantRedirect(rec, "/manager/judges")
	list := e.get("/manager/judges")
	e.wantStatus(list, 200, "Jane Judge", "fra", "senior")
	id := firstID(t, list.Body.String(), "/manager/judges/")

	e.wantStatus(e.post("/manager/judges", url.Values{"name": {"Bad"}, "level": {"god"}, "country": {"fr"}}), 422, "country", "level")
	e.wantStatus(e.post("/manager/judges", url.Values{"name": {"Bad"}, "level": {"senior"}, "country": {"fra"}, "civlid": {"zz"}}), 422, "positive CIVL id")
	e.wantStatus(e.post("/manager/judges", url.Values{"name": {"Bad"}, "level": {"senior"}, "country": {"fra"}, "civlid": {"777"}}), 422, "pilot 777 is unknown")
	e.wantStatus(e.post("/manager/judges", url.Values{"name": {"Keep"}, "level": {"nope"}, "country": {"fra"}}), 422, `value="Keep"`)

	e.wantStatus(e.get("/manager/judges/"+id+"/edit"), 200, `value="Jane Judge"`, `value="101"`)
	e.wantRedirect(e.post("/manager/judges/"+id, url.Values{"name": {"Jane J."}, "level": {"certified"}, "country": {"che"}}), "/manager/judges")
	e.wantStatus(e.get("/manager/judges"), 200, "Jane J.", "certified", "che")

	e.wantStatus(e.get("/manager/judges/"+id+"/delete"), 200, "Confirm delete")
	e.wantRedirect(e.post("/manager/judges/"+id+"/delete", nil), "/manager/judges")
	e.wantStatus(e.get("/manager/judges/"+id+"/edit"), 404)
}

const catalogJSON = `[{"name":"Misty Flip","acronym":"M","solo":true,"directions":["left","right"],"technical_coefficient":1.65,"bonuses":[{"name":"twisted","bonus":2.5}]},
{"name":"Tumbling","acronym":"T","solo":true,"synchro":true,"directions":[],"technical_coefficient":1.8,"bonuses":[],"repeatable":true}]`

func TestTricks(t *testing.T) {
	e := newEnv(t)
	form := url.Values{"year": {"2026"}, "name": {"Helico"}, "acronym": {"H"}, "solo": {"1"}, "solo_awt": {"1"},
		"technical_coefficient": {"1,5"}, "dir_left": {"1"}, "dir_right": {"1"}, "bonus_on_0": {"1"}, "bonus_val_0": {"2"},
		"first_maneuver": {"1"}, "no_first_maneuver": {"0"}, "last_maneuver": {"0"}, "no_last_maneuver": {"0"}}
	e.wantStatus(e.get("/manager/tricks/new?year=2026"), 200, "Technical coefficient", "reverse", "dir_left")
	e.wantRedirect(e.post("/manager/tricks", form), "/manager/tricks?year=2026")
	list := e.get("/manager/tricks?year=2026")
	e.wantStatus(list, 200, "Helico", "reverse (2)", "left, right", "1.5")
	id := firstID(t, list.Body.String(), "/manager/tricks/")

	// Unique tricks are generated and listed read-only.
	e.wantStatus(e.get("/manager/tricks/unique?year=2026"), 200, "Helico")
	e.wantStatus(e.get("/manager/tricks/unique?year=2026&q=zzz"), 200)

	// Validation re-render with messages.
	bad := url.Values{"year": {"2026"}, "name": {""}, "acronym": {"X"}, "technical_coefficient": {"1"}}
	e.wantStatus(e.post("/manager/tricks", bad), 422, "name: required")
	bad = url.Values{"year": {"2026"}, "name": {"Z"}, "acronym": {"Z"}, "technical_coefficient": {"abc"}}
	e.wantStatus(e.post("/manager/tricks", bad), 422, "must be a number", `value="Z"`)
	bad = url.Values{"year": {"2026"}, "name": {"Z"}, "acronym": {"Z"}, "technical_coefficient": {"1"}, "dir_left": {"1"}}
	e.wantStatus(e.post("/manager/tricks", bad), 422, "at least 2 distinct directions")
	dup := url.Values{"year": {"2026"}, "name": {"Other"}, "acronym": {"H"}, "solo": {"1"}, "technical_coefficient": {"1"}}
	e.wantStatus(e.post("/manager/tricks", dup), 409, "already exists")

	// Edit keeps the catalog year.
	e.wantStatus(e.get("/manager/tricks/"+id+"/edit"), 200, `value="Helico"`, `value="1.5"`)
	form.Set("name", "Helico 2")
	form.Set("year", "1999") // ignored: a trick stays in its year
	e.wantRedirect(e.post("/manager/tricks/"+id, form), "/manager/tricks?year=2026")
	e.wantStatus(e.get("/manager/tricks?year=2026"), 200, "Helico 2")

	// Import a JSON file into 2025, then duplicate 2025 into 2027.
	e.wantRedirect(e.upload("/manager/tricks/import", map[string]string{"year": "2025"}, "catalog.json", []byte(catalogJSON)), "/manager/tricks?year=2025")
	e.wantStatus(e.get("/manager/tricks?year=2025"), 200, "Misty Flip", "Tumbling")
	e.wantStatus(e.upload("/manager/tricks/import", map[string]string{"year": "2025"}, "catalog.json", []byte("not json")), 422, "catalog")
	e.wantStatus(e.upload("/manager/tricks/import", map[string]string{"year": "2025"}, "", nil), 422, "choose a JSON catalog file")
	e.wantRedirect(e.post("/manager/tricks/duplicate", url.Values{"from": {"2025"}, "to": {"2027"}}), "/manager/tricks?year=2027")
	e.wantStatus(e.get("/manager/tricks?year=2027"), 200, "Misty Flip", "Tumbling")
	e.wantStatus(e.post("/manager/tricks/duplicate", url.Values{"from": {"2025"}, "to": {"2025"}}), 422, "must differ")
	e.wantStatus(e.post("/manager/tricks/duplicate", url.Values{"from": {"2030"}, "to": {"2031"}}), 422, "has no tricks")

	// Delete with confirmation.
	e.wantStatus(e.get("/manager/tricks/"+id+"/delete"), 200, "Confirm delete", "Helico 2")
	e.wantRedirect(e.post("/manager/tricks/"+id+"/delete", nil), "/manager/tricks?year=2026")
	e.wantStatus(e.get("/manager/tricks/"+id+"/edit"), 404)
}

func TestSeasons(t *testing.T) {
	e := newEnv(t)
	e.wantStatus(e.get("/manager/seasons/new"), 200, `value="999"`)
	e.wantRedirect(e.post("/manager/seasons", url.Values{"name": {"AWT 2026"}, "code": {"awt-2026"}, "year": {"2026"}, "index": {"1"}}), "/manager/seasons")
	list := e.get("/manager/seasons")
	e.wantStatus(list, 200, "AWT 2026", "awt-2026")
	id := firstID(t, list.Body.String(), "/manager/seasons/")

	e.wantStatus(e.post("/manager/seasons", url.Values{"name": {"X"}, "code": {"BAD"}, "year": {"2026"}, "index": {"1"}}), 422, "code")
	e.wantStatus(e.post("/manager/seasons", url.Values{"name": {"X"}, "code": {"xxx-2026"}, "year": {"20x"}, "index": {"1"}}), 422, "must be a whole number")
	e.wantStatus(e.post("/manager/seasons", url.Values{"name": {"Kept name"}, "code": {"BAD"}, "year": {"2026"}, "index": {"1"}}), 422, `value="Kept name"`)
	e.wantStatus(e.post("/manager/seasons", url.Values{"name": {"Dup"}, "code": {"awt-2026"}, "year": {"2026"}, "index": {"2"}}), 409, "already exists")
	e.wantStatus(e.post("/manager/seasons", url.Values{"name": {"Img"}, "code": {"img-2026"}, "year": {"2026"}, "index": {"2"}, "image": {"nofile"}}), 422, "unknown")

	e.wantStatus(e.get("/manager/seasons/"+id+"/edit"), 200, `value="AWT 2026"`)
	e.wantRedirect(e.post("/manager/seasons/"+id, url.Values{"name": {"AWT Tour 2026"}, "code": {"awt-2026"}, "year": {"2026"}, "index": {"3"}}), "/manager/seasons")
	e.wantStatus(e.get("/manager/seasons"), 200, "AWT Tour 2026")
	// The "new" form is fresh: nothing leaks from a previous season.
	if strings.Contains(e.get("/manager/seasons/new").Body.String(), "AWT Tour") {
		t.Fatal("new season form leaks previous values")
	}

	e.wantStatus(e.get("/manager/seasons/"+id+"/delete"), 200, "Confirm delete", "AWT Tour 2026")
	e.wantRedirect(e.post("/manager/seasons/"+id+"/delete", nil), "/manager/seasons")
	e.wantStatus(e.get("/manager/seasons/"+id+"/edit"), 404)
}

var pngBytes = func() []byte {
	b, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")
	return b
}()

func TestFiles(t *testing.T) {
	e := newEnv(t)
	e.wantRedirect(e.upload("/manager/files", nil, "logo.png", pngBytes), "/manager/files")
	list := e.get("/manager/files")
	e.wantStatus(list, 200, "logo.png", "image/png", `value="/files/`)
	// Rejected types: HTML is never accepted.
	e.wantStatus(e.upload("/manager/files", nil, "evil.png", []byte("<html><script>alert(1)</script></html>")), 422, "must be a PNG")
	e.wantStatus(e.upload("/manager/files", nil, "", nil), 422, "choose a file")

	// An uploaded image can be picked as a season image.
	id := regexp.MustCompile(`value="/files/([0-9a-f]{64})"`).FindStringSubmatch(list.Body.String())[1]
	e.wantStatus(e.get("/manager/seasons/new"), 200, `value="`+id+`"`)
	rec := e.post("/manager/seasons", url.Values{"name": {"Swiss 2026"}, "code": {"swi-2026"}, "year": {"2026"}, "index": {"1"}, "image": {id}, "country": {"swi"}})
	e.wantRedirect(rec, "/manager/seasons")
}

func TestUsers(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.wantStatus(e.get("/manager/users"), 200, "admin@example.org", "scorer@example.org")

	e.wantRedirect(e.post("/manager/users", url.Values{"email": {"new@example.org"}, "name": {"New One"}, "role": {"scorer"}, "password": {"a long enough pass"}}), "/manager/users")
	users, _ := e.deps.Auth.ListUsers(ctx)
	var newID string
	for _, u := range users {
		if u.Email == "new@example.org" {
			newID = u.ID
		}
	}
	if newID == "" {
		t.Fatal("user not created")
	}
	e.wantStatus(e.post("/manager/users", url.Values{"email": {"x@example.org"}, "name": {"X"}, "role": {"scorer"}, "password": {"short"}}), 422, "at least 10 characters")
	e.wantStatus(e.post("/manager/users", url.Values{"email": {"x@example.org"}, "name": {"X"}, "role": {"boss"}, "password": {"a long enough pass"}}), 422, "admin or scorer")
	e.wantStatus(e.post("/manager/users", url.Values{"email": {"not-an-email"}, "name": {"X"}, "role": {"scorer"}, "password": {"a long enough pass"}}), 422, "name@example.org")
	e.wantStatus(e.post("/manager/users", url.Values{"email": {"y@example.org"}, "name": {"  "}, "role": {"scorer"}, "password": {"a long enough pass"}}), 422, "name is required")
	e.wantStatus(e.post("/manager/users", url.Values{"email": {"new@example.org"}, "name": {"X"}, "role": {"scorer"}, "password": {"a long enough pass"}}), 409, "already exists")

	// Assign competitions.
	comp, err := e.svc.CreateCompetition(ctx, service.CompetitionInput{Name: "Cup", Type: domain.TypeSolo,
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC), Location: "Annecy"})
	if err != nil {
		t.Fatal(err)
	}
	e.wantStatus(e.get("/manager/users/"+newID), 200, "Cup", "Reset password")
	e.wantRedirect(e.post("/manager/users/"+newID+"/competitions", url.Values{"competition": {comp.ID, "bogus"}}), "/manager/users/"+newID)
	assigned, _ := e.deps.Auth.UserCompetitions(ctx, newID)
	if len(assigned) != 1 || assigned[0] != comp.ID {
		t.Fatalf("assigned = %v", assigned)
	}
	e.wantStatus(e.get("/manager/users/"+newID), 200, `value="`+comp.ID+`" checked`)

	// Reset password: the new one works, short ones are refused.
	e.wantStatus(e.post("/manager/users/"+newID+"/password", url.Values{"password": {"short"}}), 422, "at least 10 characters")
	e.wantRedirect(e.post("/manager/users/"+newID+"/password", url.Values{"password": {"another long password"}}), "/manager/users/"+newID)
	if _, err := e.deps.Auth.Login(ctx, "new@example.org", "another long password"); err != nil {
		t.Fatalf("login with the new password: %v", err)
	}

	// Disable: cannot log in; cannot disable oneself.
	e.wantRedirect(e.post("/manager/users/"+newID+"/disable", nil), "/manager/users/"+newID)
	if _, err := e.deps.Auth.Login(ctx, "new@example.org", "another long password"); err == nil {
		t.Fatal("disabled user can log in")
	}
	e.wantRedirect(e.post("/manager/users/"+newID+"/enable", nil), "/manager/users/"+newID)
	if _, err := e.deps.Auth.Login(ctx, "new@example.org", "another long password"); err != nil {
		t.Fatalf("re-enabled user cannot log in: %v", err)
	}
	e.wantStatus(e.post("/manager/users/"+e.admin.User.ID+"/disable", nil), 409, "cannot disable your own")
	e.wantStatus(e.get("/manager/users/nope"), 404)
}

func TestDashboard(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.importPilots(101, 102)
	var judges []string
	for _, n := range []string{"J1", "J2"} {
		j, err := e.svc.CreateJudge(ctx, service.JudgeInput{Name: n, Country: "fra", Level: domain.JudgeSenior})
		if err != nil {
			t.Fatal(err)
		}
		judges = append(judges, j.ID)
	}
	comp, err := e.svc.CreateCompetition(ctx, service.CompetitionInput{Name: "Open Cup", Type: domain.TypeSolo,
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC), Location: "Annecy"})
	if err != nil {
		t.Fatal(err)
	}
	e.wantStatus(e.get("/manager"), 200, "<strong>2</strong> pilots", "<strong>2</strong> judges", "No competition is open.")
	if err := errors.Join(e.svc.SetParticipants(ctx, comp.ID, []int{101, 102}, nil), e.svc.SetJudges(ctx, comp.ID, judges), e.svc.OpenCompetition(ctx, comp.ID)); err != nil {
		t.Fatal(err)
	}
	e.wantStatus(e.get("/manager"), 200, "Open Cup", "/manager/competitions/"+comp.Code)
	// Scorers see the dashboard, without the reference data counts.
	rec := e.do(httptest.NewRequest(http.MethodGet, "/manager", nil), &e.scorer)
	e.wantStatus(rec, 200, "No competition is open.") // not assigned yet
	if strings.Contains(rec.Body.String(), "Open Cup") {
		t.Fatal("scorer sees a competition they are not assigned to")
	}
	if err := e.deps.Auth.AssignCompetition(ctx, "scorer@example.org", comp.ID); err != nil {
		t.Fatal(err)
	}
	rec = e.do(httptest.NewRequest(http.MethodGet, "/manager", nil), &e.scorer)
	e.wantStatus(rec, 200, "Open Cup")
	if strings.Contains(rec.Body.String(), "pilots</a>") {
		t.Fatal("scorer sees the admin counts")
	}
}

func TestScorerIsForbiddenOnReferenceData(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/manager/pilots", "/manager/teams", "/manager/teams/new", "/manager/judges", "/manager/tricks",
		"/manager/tricks/unique", "/manager/seasons", "/manager/files", "/manager/users"} {
		rec := e.do(httptest.NewRequest(http.MethodGet, p, nil), &e.scorer)
		if rec.Code != http.StatusForbidden {
			t.Errorf("scorer GET %s = %d, want 403", p, rec.Code)
		}
	}
	for _, p := range []string{"/manager/pilots/import", "/manager/teams", "/manager/judges", "/manager/tricks", "/manager/seasons", "/manager/users"} {
		rec := e.postAs(&e.scorer, p, url.Values{"name": {"x"}}, true)
		if rec.Code != http.StatusForbidden {
			t.Errorf("scorer POST %s = %d, want 403", p, rec.Code)
		}
	}
	if teams, _ := e.svc.ListTeams(context.Background()); len(teams) != 0 {
		t.Fatal("a forbidden request changed data")
	}
}

func TestCSRFIsRequired(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/manager/pilots/import", "/manager/pilots/rankings", "/manager/teams", "/manager/judges", "/manager/tricks",
		"/manager/seasons", "/manager/users", "/manager/tricks/duplicate"} {
		rec := e.postAs(&e.admin, p, url.Values{"name": {"x"}, "civlid": {"101"}}, false)
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST %s without CSRF = %d, want 403", p, rec.Code)
		}
		rec = e.postAs(&e.admin, p, url.Values{"_csrf": {"wrong"}}, false)
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST %s with a wrong CSRF = %d, want 403", p, rec.Code)
		}
	}
	if pilots, _ := e.svc.ListPilots(context.Background()); len(pilots) != 0 {
		t.Fatal("a request without CSRF imported a pilot")
	}
}

func TestOversizedBodiesAre413(t *testing.T) {
	e := newEnv(t)
	e.importPilots(101, 102)
	big := url.Values{"name": {strings.Repeat("a", 70<<10)}, "pilot1": {"101"}, "pilot2": {"102"}, "_csrf": {e.admin.CSRF}}.Encode()

	// Declared length over the cap: refused up front.
	req := httptest.NewRequest(http.MethodPost, "/manager/teams", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	e.wantStatus(e.do(req, &e.admin), http.StatusRequestEntityTooLarge)

	// Unknown length (chunked): the MaxBytesReader error is mapped to 413, not 403.
	req = httptest.NewRequest(http.MethodPost, "/manager/teams", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = -1
	e.wantStatus(e.do(req, &e.admin), http.StatusRequestEntityTooLarge)
	if teams, _ := e.svc.ListTeams(context.Background()); len(teams) != 0 {
		t.Fatal("an oversized form created a team")
	}

	// An upload over a (small) cap, declared and chunked.
	h := web.Routes(e.deps, web.ModuleFunc(func(r *web.Router) {
		r.AdminMax("POST /manager/smallmax", 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
	}))
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("_csrf", e.admin.CSRF)
	fw, _ := mw.CreateFormFile("file", "big.png")
	_, _ = fw.Write(bytes.Repeat([]byte{1}, 4096))
	_ = mw.Close()
	for _, chunked := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, "/manager/smallmax", bytes.NewReader(buf.Bytes()))
		req.Header.Set("Content-Type", mw.FormDataContentType())
		if chunked {
			req.ContentLength = -1
		}
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: e.admin.Token})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("upload over the cap (chunked=%v) = %d, want 413", chunked, rec.Code)
		}
	}
}

func TestDuplicateCatalogReplacesSameNameTricks(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.wantRedirect(e.upload("/manager/tricks/import", map[string]string{"year": "2025"}, "c.json", []byte(catalogJSON)), "/manager/tricks")
	// The target year already has an older "Misty Flip" and an unrelated trick.
	old := `[{"name":"Misty Flip","acronym":"MO","solo":true,"directions":[],"technical_coefficient":0.5,"bonuses":[]},
{"name":"Keep Me","acronym":"K","solo":true,"directions":[],"technical_coefficient":1,"bonuses":[]}]`
	e.wantRedirect(e.upload("/manager/tricks/import", map[string]string{"year": "2026"}, "o.json", []byte(old)), "/manager/tricks")
	before, _ := e.svc.ListTricks(ctx, 2026)
	var mistyID string
	for _, tr := range before {
		if tr.Name == "Misty Flip" {
			mistyID = tr.ID
		}
	}
	e.wantRedirect(e.post("/manager/tricks/duplicate", url.Values{"from": {"2025"}, "to": {"2026"}}), "/manager/tricks?year=2026")
	after, err := e.svc.ListTricks(ctx, 2026)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]int{}
	for _, tr := range after {
		names[tr.Name]++
		if tr.Name == "Misty Flip" {
			if tr.ID != mistyID || tr.TechnicalCoefficient != 1.65 || tr.Acronym != "M" {
				t.Fatalf("Misty Flip not replaced in place: %+v", tr)
			}
		}
	}
	if len(after) != 3 || names["Misty Flip"] != 1 || names["Tumbling"] != 1 || names["Keep Me"] != 1 {
		t.Fatalf("2026 catalog = %v", names)
	}
	src, _ := e.svc.ListTricks(ctx, 2025)
	if len(src) != 2 {
		t.Fatalf("source catalog changed: %d tricks", len(src))
	}
	e.wantStatus(e.post("/manager/tricks/duplicate", url.Values{"from": {"2025"}, "to": {"99999"}}), 422, "between 1901 and 2999")
}

func TestTamperedYearIs422(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/manager/tricks?year=abc", "/manager/tricks?year=99999", "/manager/tricks/unique?year=-1", "/manager/tricks/new?year=x"} {
		e.wantStatus(e.get(p), 422, "between 1901 and 2999")
	}
	e.wantStatus(e.upload("/manager/tricks/import", map[string]string{"year": "abc"}, "c.json", []byte(catalogJSON)), 422, "year")
	e.wantStatus(e.post("/manager/tricks", url.Values{"year": {"abc"}, "name": {"Z"}, "acronym": {"Z"}, "technical_coefficient": {"1"}}), 422, "year")
}

func TestNamesAreEscaped(t *testing.T) {
	e := newEnv(t)
	e.importPilots(666, 101)
	e.wantRedirect(e.post("/manager/teams", url.Values{"name": {"<script>alert(2)</script>"}, "pilot1": {"666"}, "pilot2": {"101"}}), "/manager/teams")
	for _, p := range []string{"/manager/pilots", "/manager/teams", "/manager/teams/new", "/manager/judges/new"} {
		body := e.get(p).Body.String()
		if strings.Contains(body, "<script>alert") {
			t.Fatalf("%s renders an unescaped script tag", p)
		}
	}
	e.wantStatus(e.get("/manager/pilots"), 200, "&lt;script&gt;alert(1)&lt;/script&gt;")
	e.wantStatus(e.get("/manager/teams"), 200, "&lt;script&gt;alert(2)&lt;/script&gt;")
}

func TestAnonymousIsRedirectedToLogin(t *testing.T) {
	e := newEnv(t)
	rec := e.do(httptest.NewRequest(http.MethodGet, "/manager/pilots", nil), nil)
	e.wantRedirect(rec, "/manager/login?next=")
	req := httptest.NewRequest(http.MethodPost, "/manager/pilots/gender", nil)
	req.Header.Set("HX-Request", "true")
	if rec := e.do(req, nil); rec.Header().Get("HX-Redirect") == "" && rec.Code == http.StatusOK {
		t.Fatal("anonymous htmx request passed")
	}
}

// scorerAllowed are the exact routes open to assigned scorers (read-only).
var scorerAllowed = map[string]bool{
	"GET /manager/competitions/{code}":                              true,
	"GET /manager/competitions/{code}/runs/{n}":                     true,
	"GET /manager/competitions/{code}/export/results.xlsx":          true,
	"GET /manager/competitions/{code}/export/results.html":          true,
	"GET /manager/competitions/{code}/runs/{n}/export/results.xlsx": true,
	"GET /manager/competitions/{code}/runs/{n}/export/results.html": true,
	"GET /manager/competitions/{code}/runs/{n}/starting-order.html": true,
}

var wildcard = regexp.MustCompile(`\{[^}]*\}`)

// loginSteps are the only anonymous /manager routes: the password step and
// the second-factor step, both before any session exists.
var loginSteps = map[string]bool{
	"GET /manager/login":               true,
	"POST /manager/login":              true,
	"GET /manager/login/mfa":           true,
	"POST /manager/login/mfa/totp":     true,
	"POST /manager/login/mfa/recovery": true,
}

func TestEveryAdminRouteIsGuarded(t *testing.T) {
	e := newEnv(t)
	table := web.RouteTable(e.deps, admin.Module{})
	n := 0
	for _, rt := range table {
		p := rt.Path()
		if p != "/manager" && !strings.HasPrefix(p, "/manager/") || loginSteps[rt.Pattern] {
			continue
		}
		n++
		if rt.Guard == web.GuardPublic {
			t.Errorf("%s is public", rt.Pattern)
			continue
		}
		method, _, _ := strings.Cut(rt.Pattern, " ")
		rec := e.do(httptest.NewRequest(method, wildcard.ReplaceAllString(p, "x"), nil), nil)
		if rec.Code == http.StatusOK || rec.Code == http.StatusNotFound {
			t.Errorf("%s answered %d without a session", rt.Pattern, rec.Code)
		}
		// Reference data and users are admin-only; only the dashboard is user-level.
		// Only the listed read-only GETs (and the judging routes of Task 21)
		// may be scorer-guarded; everything else is admin-only.
		if rt.Pattern != "GET /manager" && rt.Pattern != "POST /manager/logout" && rt.Guard != web.GuardAdmin &&
			!scorerAllowed[rt.Pattern] && !(rt.Guard == web.GuardScorer && strings.Contains(p, "/judging")) {
			t.Errorf("%s has guard %s, want admin", rt.Pattern, rt.Guard)
		}
		if method == "POST" && rt.Guard == web.GuardScorer && !strings.Contains(p, "/judging") {
			t.Errorf("%s: only judging POSTs may be scorer-guarded", rt.Pattern)
		}
		if scorerAllowed[rt.Pattern] && rt.Guard != web.GuardScorer {
			t.Errorf("%s should be scorer-guarded", rt.Pattern)
		}
	}
	if n < 30 {
		t.Fatalf("only %d admin routes walked", n)
	}
}
