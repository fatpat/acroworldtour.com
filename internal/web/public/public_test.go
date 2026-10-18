package public_test

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"awt/internal/auth"
	"awt/internal/blob"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/importer"
	"awt/internal/ranking"
	"awt/internal/store"
	"awt/internal/store/gen"
	"awt/internal/web"
	"awt/internal/web/public"
)

const (
	baseURL   = "https://results.test"
	pngID     = "1111111111111111111111111111111111111111111111111111111111111111"
	pdfID     = "2222222222222222222222222222222222222222222222222222222222222222"
	deletedID = "3333333333333333333333333333333333333333333333333333333333333333"
	liveID    = "ffffffffffffffffffffff01"
	hiddenID  = "ffffffffffffffffffffff02"
	emptyID   = "ffffffffffffffffffffff03"
	evilName  = "Eve <b>Bold</b>"
)

// resultsApp seeds a temp DB with the importer's mini export plus direct
// inserts (an open provisional competition with an unpublished flight, an
// unpublished competition, a competition with no runs, an empty season, a
// deleted season, a pilot without results, stored files) and returns the
// app handler.
func resultsApp(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importer.Run(ctx, db, blobs, "../../importer/testdata/mini",
		importer.WithCatalogDir("../../../testdata/catalog")); err != nil {
		t.Fatal(err)
	}
	if err := db.Tx(ctx, func(q *gen.Queries) error { return seedExtra(ctx, t, q, blobs) }); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.ExecContext(ctx, `UPDATE files SET deleted_at = '2024-02-01T00:00:00Z' WHERE id = ?`, deletedID); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: "dev", Addr: ":0", BaseURL: baseURL}
	return web.Routes(web.Deps{Cfg: cfg, Auth: auth.New(db, cfg), DB: db, Purger: cache.NoopPurger{}, Blobs: blobs}, public.Module{})
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func seedExtra(ctx context.Context, t *testing.T, q *gen.Queries, blobs blob.Store) error {
	year := time.Now().Year()
	for _, p := range []struct {
		id     int64
		name   string
		gender string
	}{{2001, evilName, "woman"}, {2002, "Carl Live", "man"}, {3003, "Nobody Results", "man"}} {
		must(t, q.UpsertPilot(ctx, gen.UpsertPilotParams{Civlid: p.id, Name: p.name, Country: "ita", Rank: 9999,
			Gender: p.gender, SocialLinks: "[]", Sponsors: "[]"}))
	}
	must(t, q.AddPilotAwtYear(ctx, gen.AddPilotAwtYearParams{Civlid: 2001, Year: int64(year)}))

	comp := func(id, code, name, state string, published bool) {
		start := strconv.Itoa(year) + "-06-01"
		must(t, q.UpsertCompetition(ctx, gen.UpsertCompetitionParams{ID: id, Code: code, Name: name, Type: "solo",
			State: state, StartDate: start, EndDate: strconv.Itoa(year) + "-06-03", Location: "Lake Test",
			Published: published, Config: "{}"}))
	}
	comp(liveID, "live-open-cup", "Live Open Cup", "open", true)
	comp(hiddenID, "hidden-cup", "Hidden Cup", "closed", false)
	comp(emptyID, "empty-cup", "Empty Cup", "init", true)
	for i, id := range []int64{2001, 2002} {
		must(t, q.AddCompetitionPilot(ctx, gen.AddCompetitionPilotParams{CompetitionID: liveID, Position: int64(i), Civlid: id}))
	}
	must(t, q.AddCompetitionPilot(ctx, gen.AddCompetitionPilotParams{CompetitionID: hiddenID, Position: 0, Civlid: 2001}))

	// Live Open Cup: run 1 has Eve published and Carl unpublished; run 2 has
	// only an unpublished flight of Carl.
	for i := range 2 {
		must(t, q.UpsertRun(ctx, gen.UpsertRunParams{CompetitionID: liveID, Idx: int64(i), State: "open", Config: "{}"}))
	}
	eveMark := `{"judges_mark":{"judge":"","technical":7.5,"choreography":6,"landing":8},"technicity":1.725,` +
		`"bonus_percentage":4.5,"technical":5.1,"choreography":2.4,"landing":0.8,"synchro":0,"bonus":0.44,"score":10,` +
		`"malus":13,"warnings1":["late <i>entry</i>"],"warnings2":["cat2 thing"],"notes":["Note about <script>x</script>"],"mark_type":"awt"}`
	fid, err := q.InsertFlight(ctx, gen.InsertFlightParams{CompetitionID: liveID, RunIdx: 0, Civlid: sql.NullInt64{Int64: 2001, Valid: true},
		Published: true, SaveSeq: 1, FinalMark: sql.NullString{String: eveMark, Valid: true}, Score: sql.NullFloat64{Float64: 10, Valid: true}})
	must(t, err)
	must(t, q.InsertFlightTrick(ctx, gen.InsertFlightTrickParams{FlightID: fid, Position: 0, Name: "Left Misty Flip", Acronym: "LMF",
		TechnicalCoefficient: 1.65, BonusTypes: "[]", Uniqueness: "[]", Bonuses: "[]", Types: "[]"}))
	must(t, q.InsertFlightMark(ctx, gen.InsertFlightMarkParams{FlightID: fid, JudgeID: "bbbbbbbbbbbbbbbbbbbbbbb1",
		Technical: sql.NullFloat64{Float64: 7.5, Valid: true}, Choreography: sql.NullFloat64{Float64: 6, Valid: true},
		Landing: sql.NullFloat64{Float64: 8, Valid: true}}))
	for _, f := range []struct {
		run   int64
		score float64
	}{{0, 20}, {1, 5}} {
		_, err := q.InsertFlight(ctx, gen.InsertFlightParams{CompetitionID: liveID, RunIdx: f.run, Civlid: sql.NullInt64{Int64: 2002, Valid: true},
			Published: false, SaveSeq: 2, FinalMark: sql.NullString{String: `{"score":0}`, Valid: true}, Score: sql.NullFloat64{Float64: f.score, Valid: true}})
		must(t, err)
	}
	eve := ranking.Participant{CivlID: 2001, Name: evilName, Gender: "woman", IsAWT: true}
	carl := ranking.Participant{CivlID: 2002, Name: "Carl Live", Gender: "man"}
	res := ranking.Competition("open", []ranking.RunInput{
		{Type: "solo", Year: year, State: "open", Flights: []ranking.FlightScore{{Participant: eve, Score: 10, Published: true, SaveSeq: 1}, {Participant: carl, Score: 20, SaveSeq: 2}}},
		{Type: "solo", Year: year, State: "open", Flights: []ranking.FlightScore{{Participant: carl, Score: 5, SaveSeq: 3}}},
	}, -1)
	for i, rr := range res.Runs {
		must(t, store.SaveRankings(ctx, q, store.ScopeRun, store.RunScopeKey(liveID, i), rr.Lists))
	}
	must(t, store.SaveRankings(ctx, q, store.ScopeCompetition, liveID, res.Lists))
	// The unpublished competition has a stored ranking with Eve in it.
	must(t, store.SaveRankings(ctx, q, store.ScopeCompetition, hiddenID, ranking.Lists{"overall": {{Participant: eve, Score: 1, Position: 1, Rank: 1}}}))

	must(t, q.UpsertSeason(ctx, gen.UpsertSeasonParams{ID: "s-empty", Code: "empty-" + strconv.Itoa(year), Name: "Empty Season", Year: int64(year), SortIndex: 1}))
	must(t, q.UpsertSeason(ctx, gen.UpsertSeasonParams{ID: "s-gone", Code: "gone-2023", Name: "Gone Season", Year: 2023, SortIndex: 1,
		DeletedAt: sql.NullString{String: "2024-01-01T00:00:00Z", Valid: true}}))

	for _, f := range []struct{ id, ct, name string }{{pngID, "image/png", "a.png"}, {pdfID, "application/pdf", "rules.pdf"}, {deletedID, "image/png", "old.png"}} {
		must(t, blobs.Put(ctx, f.id, f.ct, bytes.NewReader([]byte("content-"+f.name))))
		must(t, q.UpsertFile(ctx, gen.UpsertFileParams{ID: f.id, Filename: f.name, ContentType: f.ct, Size: 10, CreatedAt: "2024-01-01T00:00:00Z"}))
	}
	return nil
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func expectPage(t *testing.T, h http.Handler, path string, want ...string) string {
	t.Helper()
	rec := get(t, h, path)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d\n%s", path, rec.Code, body)
	}
	for _, w := range append([]string{"<title>", `property="og:title"`, `property="og:url" content="` + baseURL}, want...) {
		if !strings.Contains(body, w) {
			t.Errorf("GET %s: body lacks %q", path, w)
		}
	}
	return body
}

func TestPublicPages(t *testing.T) {
	h := resultsApp(t)
	expectPage(t, h, "/results", "Live Open Cup", "Live", "Mini Solo 2024")
	expectPage(t, h, "/seasons?year=2024", "Acro World Synchro 2024", "1 competition")
	expectPage(t, h, "/seasons/aws-2024", "<title>Acro World Synchro 2024 | Acro World Tour</title>", "Fictional Duo", "Mini Synchro 2024", "12.346")
	expectPage(t, h, "/competitions?year=2024", "Mini Solo 2024", "Mini Synchro 2024", "Acro World Synchro 2024")
	body := expectPage(t, h, "/competitions/mini-solo-2024", "<title>Mini Solo 2024 | Acro World Tour</title>", "Alice Aerial", "Final", "Run 1", "Run 2")
	if strings.Contains(body, "Provisional") {
		t.Error("final competition shows a provisional badge")
	}
	expectPage(t, h, "/competitions/mini-solo-2024/runs/1", "Alice Aerial", "8.500")
	expectPage(t, h, "/competitions/mini-solo-2024/details", "Jane Judge", "Nowhere", "https://example.org/mini", "Bob Barrel")
	expectPage(t, h, "/competitions/mini-synchro-2024", "Fictional Duo", "12.346")
	expectPage(t, h, "/pilots", "Alice Aerial", "Bob Barrel")
	expectPage(t, h, "/pilots/1001/alice-aerial", "<title>Alice Aerial | Acro World Tour</title>", "Mini Solo 2024", "2024, 2025, 2026",
		`property="og:image" content="https://example.org/a-hr.jpg"`)
	expectPage(t, h, "/teams", "Fictional Duo", "Alice Aerial")
	expectPage(t, h, "/teams/aaaaaaaaaaaaaaaaaaaaaaa1/fictional-duo", "Fictional Duo", "Mini Synchro 2024", "Bob Barrel")
	body = expectPage(t, h, "/judges", "Jane Judge", "Senior judges")
	if strings.Contains(body, "Joe Trainee") {
		t.Error("deleted judge listed")
	}
	expectPage(t, h, "/tricks?year=2024", "Solo and synchro tricks", "Synchro only tricks")
	expectPage(t, h, "/tricks", "Tricks")
}

func TestCompetitionCacheHeaders(t *testing.T) {
	h := resultsApp(t)
	if cc := get(t, h, "/competitions/mini-solo-2024").Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=30") {
		t.Errorf("competition Cache-Control = %q", cc)
	}
	if cc := get(t, h, "/pilots").Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=600") {
		t.Errorf("pilots Cache-Control = %q", cc)
	}
	if cc := get(t, h, "/competitions/hidden-cup").Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("404 Cache-Control = %q", cc)
	}
}

func TestProvisionalCompetition(t *testing.T) {
	h := resultsApp(t)
	body := expectPage(t, h, "/competitions/live-open-cup", "Provisional", "Carl Live", "20.000", "10.000")
	// overall keeps the unpublished flight's total (decision 0005)
	if strings.Index(body, "Carl Live") > strings.Index(body, "Eve &lt;b&gt;") {
		t.Error("Carl (20) should rank above Eve (10) in the overall")
	}
}

// Run tables list only published flights, re-ranked (controller ruling).
func TestRunHidesUnpublishedFlights(t *testing.T) {
	h := resultsApp(t)
	body := expectPage(t, h, "/competitions/live-open-cup/runs/1", "Provisional", "Eve &lt;b&gt;Bold&lt;/b&gt;",
		"Left Misty Flip", "Jane Judge", "Technicity", "1.725", "13%", "4.5%", "Note about &lt;script&gt;",
		"late &lt;i&gt;entry&lt;/i&gt; (0.5 pt)", "cat2 thing (1 pt)", `<td class="num">1</td>`)
	if strings.Contains(body, "Carl Live") {
		t.Error("unpublished flight listed in the run table")
	}
}

func TestPublicPagesEmptyStates(t *testing.T) {
	h := resultsApp(t)
	year := strconv.Itoa(time.Now().Year())
	expectPage(t, h, "/competitions/empty-cup", "No results at the moment")
	expectPage(t, h, "/competitions/empty-cup/details", "Empty Cup")
	expectPage(t, h, "/seasons/empty-"+year, "No results at the moment")
	expectPage(t, h, "/pilots/3003/nobody-results", "No competition results")
	expectPage(t, h, "/competitions/live-open-cup/runs/2", "No published flight")
	expectPage(t, h, "/tricks?year=1990", "No tricks")
	expectPage(t, h, "/competitions?year=1990", "No competitions")
	expectPage(t, h, "/seasons?year=1990", "No seasons")
}

func TestPublicNotFound(t *testing.T) {
	h := resultsApp(t)
	for _, p := range []string{
		"/competitions/hidden-cup", "/competitions/hidden-cup/details", "/competitions/hidden-cup/runs/1",
		"/competitions/nope", "/competitions/live-open-cup/runs/9", "/competitions/live-open-cup/runs/x",
		"/seasons/gone-2023", "/seasons/nope", "/pilots/999999/x", "/pilots/abc/x", "/teams/nope/x",
	} {
		if rec := get(t, h, p); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404", p, rec.Code)
		}
	}
}

func TestPilotSlugRedirect(t *testing.T) {
	h := resultsApp(t)
	for _, p := range []string{"/pilots/1001", "/pilots/1001/wrong"} {
		rec := get(t, h, p)
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/pilots/1001/alice-aerial" {
			t.Errorf("GET %s: %d %q", p, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestNamesAreEscaped(t *testing.T) {
	h := resultsApp(t)
	for _, p := range []string{"/competitions/live-open-cup", "/competitions/live-open-cup/details", "/pilots", "/pilots/2001/eve-bboldb"} {
		body := expectPage(t, h, p, "Eve &lt;b&gt;Bold&lt;/b&gt;")
		if strings.Contains(body, "<b>Bold") {
			t.Errorf("GET %s: unescaped name", p)
		}
	}
}

func TestPilotHistoryHidesUnpublishedCompetition(t *testing.T) {
	h := resultsApp(t)
	body := expectPage(t, h, "/pilots/2001/eve-bboldb", "Live Open Cup")
	if strings.Contains(body, "Hidden Cup") {
		t.Error("unpublished competition in pilot history")
	}
}

func TestFiles(t *testing.T) {
	h := resultsApp(t)
	rec := get(t, h, "/files/"+pngID)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" || rec.Body.String() != "content-a.png" {
		t.Fatalf("png: %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Errorf("png Content-Disposition = %q", cd)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}
	rec = get(t, h, "/files/"+pdfID)
	if cd := rec.Header().Get("Content-Disposition"); rec.Code != 200 || !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "rules.pdf") {
		t.Errorf("pdf: %d %q", rec.Code, cd)
	}
	for _, id := range []string{deletedID, strings.Repeat("4", 64), strings.Repeat("A", 64), pngID + ".png", "abc"} {
		if rec := get(t, h, "/files/"+id); rec.Code != http.StatusNotFound {
			t.Errorf("GET /files/%s: %d, want 404", id, rec.Code)
		}
	}
}
