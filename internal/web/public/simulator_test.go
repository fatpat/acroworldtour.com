package public_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"awt/internal/auth"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/store"
	"awt/internal/store/gen"
	"awt/internal/web"
	"awt/internal/web/public"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

// newApp builds the app handler on a migrated temp DB with a catalog of two
// solo tricks (LM, RM) and one synchro-only trick (SY) for the current year.
func newApp(t *testing.T) http.Handler {
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
	year := int64(time.Now().Year())
	q := gen.New(db.Write)
	for _, tr := range []struct {
		id, name, acr string
		solo          bool
	}{{"t1", "left Misty Flip", "LM", true}, {"t2", "right Misty Flip", "RM", true}, {"t3", "Synchro Thing", "SY", false}} {
		if err := q.UpsertTrick(ctx, gen.UpsertTrickParams{ID: tr.id, Year: year, Name: tr.name, Acronym: tr.acr,
			Solo: tr.solo, SoloAwt: true, Synchro: !tr.solo, Directions: "[]", TechnicalCoefficient: 1.65,
			Bonuses: "[]", BonusConstraints: "[]", Types: "[]"}); err != nil {
			t.Fatal(err)
		}
		if err := q.InsertUniqueTrick(ctx, gen.InsertUniqueTrickParams{TrickID: tr.id, Year: year, Acronym: tr.acr,
			Name: tr.name, BaseTrick: "Misty Flip", TechnicalCoefficient: 1.65, BonusTypes: "[]", Uniqueness: "[]",
			Bonuses: "[]", Solo: tr.solo, SoloAwt: true, Synchro: !tr.solo, Types: "[]"}); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Config{Env: "dev", Addr: ":0"}
	return web.Routes(web.Deps{Cfg: cfg, Auth: auth.New(db, cfg), DB: db, Purger: cache.NoopPurger{}}, public.Module{})
}

func post(h http.Handler, form url.Values, htmx bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/simulator", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if htmx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func twoRuns(extra url.Values) url.Values {
	f := url.Values{
		"tricks1": {"LM"}, "technical1": {"7"}, "choreography1": {"7"}, "landing1": {"7"},
		"tricks2": {"LM"}, "technical2": {"7"}, "choreography2": {"7"}, "landing2": {"7"},
	}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

func TestRepetitionMalusAcrossRuns(t *testing.T) {
	rec := post(newApp(t), twoRuns(nil), false)
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, body)
	}
	for _, want := range []string{"<html", "Adding a 13.0% malus.", "has already been performed in a previous run", "<td>13</td>"} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
}

func TestResetEveryRunClearsMalus(t *testing.T) {
	body := post(newApp(t), twoRuns(url.Values{"reset": {"1"}}), false).Body.String()
	if strings.Contains(body, "malus.") || strings.Contains(body, "<td>13</td>") {
		t.Errorf("malus applied despite reset:\n%s", body)
	}
	if !strings.Contains(body, "Total") {
		t.Errorf("no total:\n%s", body)
	}
}

func TestUnknownTrickIsReportedPerRun(t *testing.T) {
	f := twoRuns(url.Values{"tricks2": {"LM NOPE"}})
	rec := post(newApp(t), f, false)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, "Unknown trick(s): NOPE") || !strings.Contains(body, "Run 2") {
		t.Errorf("status %d, body:\n%s", rec.Code, body)
	}
}

func TestSynchroOnlyTrickIsUnknown(t *testing.T) {
	body := post(newApp(t), twoRuns(url.Values{"tricks1": {"SY"}}), false).Body.String()
	if !strings.Contains(body, "Unknown trick(s): SY") {
		t.Errorf("synchro trick accepted:\n%s", body)
	}
}

func TestInvalidMark(t *testing.T) {
	body := post(newApp(t), twoRuns(url.Values{"technical1": {"11"}}), false).Body.String()
	if !strings.Contains(body, "Invalid technical mark") {
		t.Errorf("body:\n%s", body)
	}
}

func TestHTMXReturnsOnlyResultsPanel(t *testing.T) {
	rec := post(newApp(t), twoRuns(nil), true)
	body := rec.Body.String()
	if strings.Contains(body, "<html") || !strings.Contains(body, `id="results"`) || !strings.Contains(body, "Total") {
		t.Errorf("not a partial:\n%s", body)
	}
}

func TestPageAndTricksJSON(t *testing.T) {
	h := newApp(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/simulator", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `name="tricks5"`) || !strings.Contains(rec.Body.String(), "trick-input.js") {
		t.Fatalf("page: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/simulator/tricks.json", nil))
	var got []struct{ Acronym, Name string }
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got) != 2 {
		t.Fatalf("tricks.json: %v %s", err, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "max-age") {
		t.Errorf("not cacheable: %q", rec.Header().Get("Cache-Control"))
	}
}
