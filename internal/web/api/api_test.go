package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"awt/internal/auth"
	"awt/internal/blob"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/importer"
	"awt/internal/store"
	"awt/internal/web"
	"awt/internal/web/api"
)

const (
	miniDir    = "../../importer/testdata/mini"
	catalogDir = "../../../testdata/catalog"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

// newApp imports the mini fixture into a temp DB and returns the handler and the DB.
func newApp(t *testing.T) (http.Handler, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importer.Run(ctx, db, blobs, miniDir, importer.WithCatalogDir(catalogDir)); err != nil {
		t.Fatalf("import: %v", err)
	}
	cfg := config.Config{Env: "dev", Addr: ":0"}
	return web.Routes(web.Deps{Cfg: cfg, Auth: auth.New(db, cfg), DB: db, Purger: cache.NoopPurger{}}, api.Module{}), db
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func getJSON(t *testing.T, h http.Handler, path string, want int) map[string]any {
	t.Helper()
	rec := do(h, http.MethodGet, path)
	if rec.Code != want {
		t.Fatalf("GET %s = %d, want %d: %s", path, rec.Code, want, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("GET %s content type %q", path, ct)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("GET %s: not a JSON object: %v: %s", path, err, rec.Body)
	}
	return out
}

func getList(t *testing.T, h http.Handler, path string) []any {
	t.Helper()
	out := getJSON(t, h, path, 200)
	l, ok := out["items"].([]any)
	if !ok {
		t.Fatalf("GET %s: no items array: %v", path, out)
	}
	return l
}

func TestCompetitionsList(t *testing.T) {
	h, _ := newApp(t)
	items := getList(t, h, "/api/v1/competitions")
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	// newest first
	c := items[0].(map[string]any)
	if c["code"] != "mini-synchro-2024" || c["type"] != "synchro" || c["start_date"] != "2024-09-01" ||
		c["state"] != "closed" || c["number_of_teams"] != 1.0 || c["number_of_runs"] != 1.0 {
		t.Errorf("competition = %v", c)
	}
	solo := items[1].(map[string]any)
	if solo["number_of_pilots"] != 3.0 || solo["number_of_judges"] != 2.0 || solo["website"] != "https://example.org/mini" {
		t.Errorf("solo = %v", solo)
	}
	if seasons := solo["seasons"].([]any); len(seasons) != 1 || seasons[0] != "awt-2024" {
		t.Errorf("seasons = %v", seasons)
	}
	if _, ok := solo["published"]; ok {
		t.Error("published must not be exposed")
	}
}

func TestCompetitionDetail(t *testing.T) {
	h, _ := newApp(t)
	c := getJSON(t, h, "/api/v1/competitions/mini-solo-2024", 200)
	if c["name"] != "Mini Solo 2024" || c["final"] != true {
		t.Errorf("competition = %v", c)
	}
	pilots := c["pilots"].([]any)
	if len(pilots) != 3 {
		t.Fatalf("pilots = %d", len(pilots))
	}
	p := pilots[0].(map[string]any)
	for _, k := range []string{"civlid", "name", "country", "photo", "slug"} {
		if _, ok := p[k]; !ok {
			t.Errorf("pilot lacks %q: %v", k, p)
		}
	}
	if _, ok := p["gender"]; ok {
		t.Error("gender must not be exposed")
	}
	if len(c["judges"].([]any)) != 2 {
		t.Errorf("judges = %v", c["judges"])
	}
	runs := c["runs"].([]any)
	if len(runs) != 2 || runs[0].(map[string]any)["index"] != 0.0 || runs[0].(map[string]any)["number_of_flights"] != 3.0 {
		t.Errorf("runs = %v", runs)
	}
	overall := c["results"].(map[string]any)["overall"].([]any)
	if len(overall) == 0 {
		t.Fatalf("no overall results: %v", c["results"])
	}
	first := overall[0].(map[string]any)
	if first["rank"] != 1.0 || first["position"] != 1.0 || first["pilot"] == nil {
		t.Errorf("first = %v", first)
	}
	if cells := first["result_per_run"].([]any); len(cells) != 2 {
		t.Errorf("result_per_run = %v", cells)
	}

	s := getJSON(t, h, "/api/v1/competitions/mini-synchro-2024", 200)
	teams := s["teams"].([]any)
	if len(teams) != 1 || len(teams[0].(map[string]any)["pilots"].([]any)) != 2 {
		t.Errorf("teams = %v", teams)
	}
}

func TestRunDetail(t *testing.T) {
	h, _ := newApp(t)
	r := getJSON(t, h, "/api/v1/competitions/mini-solo-2024/runs/0", 200)
	if r["index"] != 0.0 || r["final"] != true {
		t.Errorf("run = %v", r)
	}
	flights := r["flights"].([]any)
	if len(flights) != 3 {
		t.Fatalf("flights = %d", len(flights))
	}
	f := flights[0].(map[string]any)
	if f["score"] != 8.5 || f["did_not_start"] != false || f["pilot"].(map[string]any)["civlid"] != 1001.0 {
		t.Errorf("flight = %v", f)
	}
	if len(f["tricks"].([]any)) != 2 || len(f["marks"].([]any)) != 2 {
		t.Errorf("tricks/marks = %v / %v", f["tricks"], f["marks"])
	}
	fm := f["final_marks"].(map[string]any)
	if fm["score"] != 8.5 || fm["technicity"] != 1.65 || fm["mark_type"] != "awt" {
		t.Errorf("final_marks = %v", fm)
	}
	if w := f["warnings"].([]any); len(w) != 1 || w[0] != "w cat1" {
		t.Errorf("warnings = %v", w)
	}
	mark := f["marks"].([]any)[0].(map[string]any)
	if mark["judge"].(map[string]any)["name"] == "" || mark["choreography"] != 5.0 {
		t.Errorf("mark = %v", mark)
	}
	if got := runRanking(r); got != "1:1001 2:1002 3:77777" {
		t.Errorf("run ranking = %s", got)
	}

	// synchro: team flight, warnings2, tip touch, 3-decimal rounding already applied
	sf := getJSON(t, h, "/api/v1/competitions/mini-synchro-2024/runs/0", 200)["flights"].([]any)[0].(map[string]any)
	if sf["team"] == nil || sf["pilot"] != nil || sf["score"] != 12.346 || sf["tip_touches"] != 1.0 {
		t.Errorf("synchro flight = %v", sf)
	}
	if w := sf["warnings2"].([]any); len(w) != 1 || w[0] != "w cat2" {
		t.Errorf("warnings2 = %v", w)
	}
}

// runRanking renders the overall list of a run as "rank:civlid ...".
func runRanking(run map[string]any) string {
	var parts []string
	for _, e := range run["results"].(map[string]any)["overall"].([]any) {
		m := e.(map[string]any)
		parts = append(parts, strconv.Itoa(int(m["rank"].(float64)))+":"+strconv.Itoa(int(m["pilot"].(map[string]any)["civlid"].(float64))))
	}
	return strings.Join(parts, " ")
}

func TestNotFound(t *testing.T) {
	h, _ := newApp(t)
	for _, p := range []string{
		"/api/v1/competitions/nope", "/api/v1/competitions/mini-solo-2024/runs/9", "/api/v1/competitions/mini-solo-2024/runs/x",
		"/api/v1/competitions/mini-solo-2024/runs/-1", "/api/v1/seasons/nope", "/api/v1/pilots/424242", "/api/v1/pilots/abc",
		"/api/v1/nothing",
	} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != 404 || strings.TrimSpace(rec.Body.String()) != `{"error":"not found"}` {
			t.Errorf("GET %s = %d %q", p, rec.Code, rec.Body)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("GET %s content type %q", p, ct)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("GET %s lacks CORS on 404", p)
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("GET %s 404 Cache-Control %q", p, cc)
		}
	}
}

func TestUnpublishedCompetitionHidden(t *testing.T) {
	h, db := newApp(t)
	if _, err := db.Write.Exec(`UPDATE competitions SET published = 0 WHERE code = 'mini-solo-2024'`); err != nil {
		t.Fatal(err)
	}
	if items := getList(t, h, "/api/v1/competitions"); len(items) != 1 {
		t.Errorf("list = %d items, want 1", len(items))
	}
	getJSON(t, h, "/api/v1/competitions/mini-solo-2024", 404)
	getJSON(t, h, "/api/v1/competitions/mini-solo-2024/runs/0", 404)
	// deleted too
	if _, err := db.Write.Exec(`UPDATE competitions SET deleted_at = '2025-01-01T00:00:00Z' WHERE code = 'mini-synchro-2024'`); err != nil {
		t.Fatal(err)
	}
	if items := getList(t, h, "/api/v1/competitions"); len(items) != 0 {
		t.Errorf("list = %d items, want 0", len(items))
	}
	// the season no longer lists it
	s := getJSON(t, h, "/api/v1/seasons/aws-2024", 200)
	if c := s["competitions"].([]any); len(c) != 0 {
		t.Errorf("season competitions = %v", c)
	}
}

func TestUnpublishedFlightHidden(t *testing.T) {
	h, db := newApp(t)
	if _, err := db.Write.Exec(`UPDATE flights SET published = 0 WHERE competition_id IN
		(SELECT id FROM competitions WHERE code = 'mini-solo-2024') AND run_idx = 0 AND civlid = 1001`); err != nil {
		t.Fatal(err)
	}
	flights := getJSON(t, h, "/api/v1/competitions/mini-solo-2024/runs/0", 200)["flights"].([]any)
	if len(flights) != 2 {
		t.Fatalf("flights = %d, want 2", len(flights))
	}
	for _, f := range flights {
		if f.(map[string]any)["pilot"].(map[string]any)["civlid"] == 1001.0 {
			t.Error("unpublished flight exposed")
		}
	}
	run := getJSON(t, h, "/api/v1/competitions/mini-solo-2024/runs/0", 200)
	if got := runRanking(run); got != "1:1002 2:77777" {
		t.Errorf("run ranking = %s, want the unpublished pilot dropped and ranks renumbered", got)
	}
	runs := getJSON(t, h, "/api/v1/competitions/mini-solo-2024", 200)["runs"].([]any)
	if runs[0].(map[string]any)["number_of_flights"] != 2.0 {
		t.Errorf("runs = %v", runs)
	}
}

func TestPilotResultsSkipHiddenCompetitions(t *testing.T) {
	h, db := newApp(t)
	count := func() int {
		p := getJSON(t, h, "/api/v1/pilots/1001", 200)
		return len(p["competitions_results"].([]any))
	}
	if count() != 1 {
		t.Fatalf("competitions_results = %d, want 1", count())
	}
	if _, err := db.Write.Exec(`UPDATE competitions SET published = 0 WHERE code = 'mini-solo-2024'`); err != nil {
		t.Fatal(err)
	}
	if count() != 0 {
		t.Error("unpublished competition in competitions_results")
	}
	if _, err := db.Write.Exec(`UPDATE competitions SET published = 1, deleted_at = '2025-01-01T00:00:00Z' WHERE code = 'mini-solo-2024'`); err != nil {
		t.Fatal(err)
	}
	if count() != 0 {
		t.Error("deleted competition in competitions_results")
	}
	if _, err := db.Write.Exec(`UPDATE seasons SET deleted_at = '2025-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	if n := len(getJSON(t, h, "/api/v1/pilots/1001", 200)["seasons_results"].([]any)); n != 0 {
		t.Errorf("deleted seasons in seasons_results: %d", n)
	}
}

func TestSeasonDetailExposesNoPerCompetitionBreakdown(t *testing.T) {
	h, db := newApp(t)
	if _, err := db.Write.Exec(`UPDATE competitions SET published = 0 WHERE code = 'mini-synchro-2024'`); err != nil {
		t.Fatal(err)
	}
	d := getJSON(t, h, "/api/v1/seasons/aws-2024", 200)
	if _, ok := d["competitions_results"]; ok {
		t.Error("season detail must not embed per-competition results")
	}
	if c := d["competitions"].([]any); len(c) != 0 {
		t.Errorf("unpublished competition listed: %v", c)
	}
	if strings.Contains(do(h, http.MethodGet, "/api/v1/seasons/aws-2024").Body.String(), "mini-synchro-2024") {
		t.Error("unpublished competition code leaked in season detail")
	}
}

func TestSeasons(t *testing.T) {
	h, db := newApp(t)
	items := getList(t, h, "/api/v1/seasons")
	if len(items) == 0 {
		t.Fatal("no seasons")
	}
	codes := map[string]map[string]any{}
	for _, it := range items {
		m := it.(map[string]any)
		codes[m["code"].(string)] = m
	}
	aws, ok := codes["aws-2024"]
	if !ok || aws["name"] != "Acro World Synchro 2024" || aws["year"] != 2024.0 || aws["number_of_competitions"] != 1.0 {
		t.Errorf("aws-2024 = %v (all %v)", aws, codes)
	}
	if aws["image"] == nil || !strings.HasPrefix(aws["image"].(string), "/files/") {
		t.Errorf("image = %v", aws["image"])
	}
	d := getJSON(t, h, "/api/v1/seasons/aws-2024", 200)
	if c := d["competitions"].([]any); len(c) != 1 || c[0].(map[string]any)["code"] != "mini-synchro-2024" {
		t.Errorf("competitions = %v", d["competitions"])
	}
	if _, ok := d["results"].(map[string]any); !ok {
		t.Errorf("results = %v", d["results"])
	}
	// deleted seasons are not listed
	if _, err := db.Write.Exec(`UPDATE seasons SET deleted_at = '2025-01-01T00:00:00Z' WHERE code = 'aws-2024'`); err != nil {
		t.Fatal(err)
	}
	getJSON(t, h, "/api/v1/seasons/aws-2024", 404)
}

func TestPilots(t *testing.T) {
	h, _ := newApp(t)
	items := getList(t, h, "/api/v1/pilots")
	if len(items) < 2 {
		t.Fatalf("pilots = %d", len(items))
	}
	for _, it := range items {
		if _, ok := it.(map[string]any)["gender"]; ok {
			t.Fatal("gender must not be exposed")
		}
	}
	p := getJSON(t, h, "/api/v1/pilots/1001", 200)
	if p["civlid"] != 1001.0 || p["slug"] == "" {
		t.Errorf("pilot = %v", p)
	}
	cr := p["competitions_results"].([]any)
	if len(cr) != 1 || cr[0].(map[string]any)["competition"].(map[string]any)["code"] != "mini-solo-2024" || cr[0].(map[string]any)["rank"] == nil {
		t.Errorf("competitions_results = %v", cr)
	}
	if _, ok := p["seasons_results"].([]any); !ok {
		t.Errorf("seasons_results = %v", p["seasons_results"])
	}
}

func TestTeamsAndJudges(t *testing.T) {
	h, _ := newApp(t)
	teams := getList(t, h, "/api/v1/teams")
	if len(teams) == 0 || len(teams[0].(map[string]any)["pilots"].([]any)) != 2 || teams[0].(map[string]any)["id"] == "" {
		t.Errorf("teams = %v", teams)
	}
	judges := getList(t, h, "/api/v1/judges")
	if len(judges) == 0 {
		t.Fatal("no judges")
	}
	j := judges[0].(map[string]any)
	for _, k := range []string{"id", "name", "country", "level"} {
		if _, ok := j[k]; !ok {
			t.Errorf("judge lacks %q: %v", k, j)
		}
	}
}

func TestTricks(t *testing.T) {
	h, db := newApp(t)
	year := 2031
	if _, err := db.Write.Exec(`INSERT INTO tricks (id, year, name, acronym, solo, solo_awt, synchro, technical_coefficient)
		VALUES ('trick-api-1', ?, 'API Test Trick', 'ATT', 1, 1, 0, 1.5)`, year); err != nil {
		t.Fatal(err)
	}
	items := getList(t, h, "/api/v1/tricks?year="+strconv.Itoa(year))
	if len(items) == 0 {
		t.Fatal("no tricks")
	}
	tr := items[0].(map[string]any)
	for _, k := range []string{"name", "acronym", "solo", "synchro", "technical_coefficient", "bonuses", "directions", "types"} {
		if _, ok := tr[k]; !ok {
			t.Errorf("trick lacks %q: %v", k, tr)
		}
	}
	if len(getList(t, h, "/api/v1/tricks?year=1900")) != 0 {
		t.Error("year 1900 should be empty")
	}
	rec := do(h, http.MethodGet, "/api/v1/tricks?year=abc")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Errorf("bad year = %d %s", rec.Code, rec.Body)
	}
	// default year is the current one: answers 200
	getJSON(t, h, "/api/v1/tricks", 200)
}

func TestCORSAndCacheHeaders(t *testing.T) {
	h, _ := newApp(t)
	for path, cc := range map[string]string{
		"/api/v1/competitions":                       "public, max-age=30, stale-while-revalidate=60",
		"/api/v1/competitions/mini-solo-2024":        "public, max-age=30, stale-while-revalidate=60",
		"/api/v1/competitions/mini-solo-2024/runs/0": "public, max-age=30, stale-while-revalidate=60",
		"/api/v1/seasons":                            "public, max-age=30, stale-while-revalidate=60",
		"/api/v1/pilots":                             "public, max-age=3600, stale-while-revalidate=60",
		"/api/v1/teams":                              "public, max-age=3600, stale-while-revalidate=60",
		"/api/v1/judges":                             "public, max-age=3600, stale-while-revalidate=60",
		"/api/v1/tricks":                             "public, max-age=3600, stale-while-revalidate=60",
	} {
		rec := do(h, http.MethodGet, path)
		if got := rec.Header().Get("Cache-Control"); got != cc {
			t.Errorf("%s Cache-Control = %q, want %q", path, got, cc)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("%s: no ACAO *", path)
		}
		if rec.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Errorf("%s: credentials allowed", path)
		}
	}
	if rec := do(h, http.MethodHead, "/api/v1/competitions"); rec.Code != 200 {
		t.Errorf("HEAD = %d", rec.Code)
	}
	rec := do(h, http.MethodOptions, "/api/v1/competitions")
	if rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Origin") != "*" ||
		rec.Header().Get("Access-Control-Allow-Methods") != "GET, HEAD, OPTIONS" {
		t.Errorf("OPTIONS = %d %v", rec.Code, rec.Header())
	}
	rec = do(h, http.MethodPost, "/api/v1/competitions")
	if rec.Code != 405 || !strings.Contains(rec.Header().Get("Allow"), "GET") {
		t.Errorf("POST = %d %v", rec.Code, rec.Header())
	}
}

func TestRoutesArePublic(t *testing.T) {
	cfg := config.Config{Env: "dev", Addr: ":0"}
	for _, r := range web.RouteTable(web.Deps{Cfg: cfg}, api.Module{}) {
		if strings.Contains(r.Pattern, "/api/v1") && r.Guard != web.GuardPublic {
			t.Errorf("%s guard = %s", r.Pattern, r.Guard)
		}
	}
}
