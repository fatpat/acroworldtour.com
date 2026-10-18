package public_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestLiveCompetitionPageConnectsToStream(t *testing.T) {
	h := resultsApp(t)
	body := expectPage(t, h, "/competitions/live-open-cup?list=overall",
		`hx-ext="sse"`, `sse-connect="/live/competitions/live-open-cup"`, `hx-trigger="sse:update"`,
		`hx-get="/competitions/live-open-cup/fragment?list=overall"`)
	if strings.Contains(body, "<script>") {
		t.Error("inline script on the live page")
	}
	run := expectPage(t, h, "/competitions/live-open-cup/runs/1", `sse-connect=`, `fragment?list=`, `run=1`)
	if !strings.Contains(run, "Eve") {
		t.Error("run table missing")
	}
	// closed competitions do not connect
	closed := expectPage(t, h, "/competitions/mini-solo-2024")
	if strings.Contains(closed, "sse-connect") || strings.Contains(closed, "sse:update") {
		t.Error("closed competition connects to the live stream")
	}
}

func TestFragmentReturnsOnlyTheTable(t *testing.T) {
	h := resultsApp(t)
	rec := get(t, h, "/competitions/live-open-cup/fragment?list=overall")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"<table", "Carl Live", "25.000", "Eve &lt;b&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment lacks %q", want)
		}
	}
	for _, bad := range []string{"<html", "<title>", "<nav class=\"tabs\""} {
		if strings.Contains(body, bad) {
			t.Errorf("fragment contains %q", bad)
		}
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=0, s-maxage=5, must-revalidate" {
		t.Errorf("Cache-Control = %q", cc)
	}
	run := get(t, h, "/competitions/live-open-cup/fragment?run=1&list=overall").Body.String()
	if !strings.Contains(run, "Left Misty Flip") || strings.Contains(run, "Carl Live") {
		t.Errorf("run fragment wrong:\n%s", run)
	}
	for path, code := range map[string]int{
		"/competitions/hidden-cup/fragment":              404,
		"/competitions/live-open-cup/fragment?run=9":     404,
		"/competitions/live-open-cup/fragment?run=x":     404,
		"/competitions/live-open-cup/fragment?run=0":     404,
		"/competitions/live-open-cup/fragment?list=nope": 200,
	} {
		if got := get(t, h, path).Code; got != code {
			t.Errorf("GET %s = %d, want %d", path, got, code)
		}
	}
}

func TestLiveStreamUnpublishedIs404(t *testing.T) {
	h := resultsApp(t)
	for _, p := range []string{"/live/competitions/hidden-cup", "/live/competitions/nope"} {
		if got := get(t, h, p).Code; got != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, got)
		}
	}
}

func TestOverlayRunEscapesAndShowsRankScore(t *testing.T) {
	h := resultsApp(t)
	rec := get(t, h, "/live/overlay/competitions/live-open-cup/runs/1.svg")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	body := rec.Body.String()
	// Eve <b>Bold</b> is the latest published flight of run 1
	for _, want := range []string{"EVE &lt;B&gt;BOLD&lt;/B&gt;", "RUN 1", "#1", "10.000", "ITA", `width="1920"`} {
		if !strings.Contains(body, want) {
			t.Errorf("overlay lacks %q", want)
		}
	}
	if strings.Contains(body, "<B>") || strings.Contains(body, "<b>") || strings.Contains(body, "%{") {
		t.Error("overlay has raw markup or an unfilled placeholder")
	}
	if strings.Contains(body, "CARL") {
		t.Error("unpublished flight on the overlay")
	}
}

func TestOverlayCurrentAndEdges(t *testing.T) {
	h := resultsApp(t)
	// both runs are open: current is run 2, which has no published flight
	rec := get(t, h, "/live/overlay/competitions/live-open-cup/current.svg")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<svg") || strings.Contains(rec.Body.String(), "RUN") {
		t.Errorf("current overlay: %d %s", rec.Code, rec.Body.String())
	}
	for path, code := range map[string]int{
		"/live/overlay/competitions/hidden-cup/runs/1.svg":    404,
		"/live/overlay/competitions/live-open-cup/runs/9.svg": 404,
		"/live/overlay/competitions/live-open-cup/runs/x.svg": 404,
		"/live/overlay/competitions/live-open-cup/runs/1":     404,
		"/live/overlay/competitions/hidden-cup/current.svg":   404,
	} {
		if got := get(t, h, path).Code; got != code {
			t.Errorf("GET %s = %d, want %d", path, got, code)
		}
	}
}
