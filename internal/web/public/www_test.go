package public_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"awt/internal/auth"
	"awt/internal/blob"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/importer"
	"awt/internal/store"
	"awt/internal/store/gen"
	"awt/internal/web"
	"awt/internal/web/public"
)

const wwwBase = "https://www.test"

// wwwApp builds the app on a temp DB holding the importer's mini results
// export, the www importer's fixture site and a few editor-written rows (a
// page with hostile Markdown, a draft, a scheduled article, settings).
func wwwApp(t *testing.T) http.Handler {
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
	if _, err := importer.Run(ctx, db, blobs, "../../importer/testdata/mini", importer.WithCatalogDir("../../../internal/catalog/data")); err != nil {
		t.Fatal(err)
	}
	if _, err := importer.WWW(ctx, db, blobs, "../../importer/testdata/www"); err != nil {
		t.Fatal(err)
	}
	err = db.Tx(ctx, func(q *gen.Queries) error {
		must := func(err error) {
			if err != nil {
				t.Fatal(err)
			}
		}
		set := func(k, v string) {
			b, _ := json.Marshal(v)
			must(q.UpsertSiteSetting(ctx, gen.UpsertSiteSettingParams{Key: k, Value: string(b)}))
		}
		set("hero_title", "Fly the Tour")
		set("hero_pre_title", "Season opener")
		set("featured_video", "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
		must(q.UpsertSiteSetting(ctx, gen.UpsertSiteSettingParams{Key: "featured_seasons", Value: `["aws-2024","nope-2024"]`}))
		must(q.UpsertPage(ctx, gen.UpsertPageParams{Slug: "about", Title: "About us",
			BodyMd: "We fly.\n\n<script>alert('page')</script>\n\n[bad](javascript:alert(1))"}))
		// A page whose slug equals a literal route must never shadow it.
		must(q.UpsertPage(ctx, gen.UpsertPageParams{Slug: "events", Title: "Shadow", BodyMd: "shadow"}))
		must(q.UpsertNews(ctx, gen.UpsertNewsParams{ID: "n-draft", Slug: "draft-news", Title: "Draft news", BodyMd: "secret"}))
		must(q.UpsertNews(ctx, gen.UpsertNewsParams{ID: "n-future", Slug: "future-news", Title: "Future news", BodyMd: "later",
			PublishedAt: sql.NullString{String: "2999-01-01T00:00:00Z", Valid: true}}))
		must(q.UpsertNews(ctx, gen.UpsertNewsParams{ID: "n-ld", Slug: "ld-news", Title: "</script><script>alert(9)</script>",
			BodyMd: "text", PublishedAt: sql.NullString{String: "2024-05-02T10:00:00Z", Valid: true}}))
		must(q.UpsertNews(ctx, gen.UpsertNewsParams{ID: "n-xss", Slug: "xss-news", Title: "Hostile <b>title</b>", Author: "Eve",
			BodyMd:      "Hello <script>alert(1)</script> <img src=x onerror=alert(2)>\n\n[x](javascript:alert(3))",
			PublishedAt: sql.NullString{String: "2024-05-01T10:00:00Z", Valid: true}}))
		for _, e := range []struct{ id, slug, name, start, end, circuits string }{
			{"e-2024", "swiss-acro-tour", "Swiss Acro Tour", "2024-07-25", "2024-07-28", `["SAT"]`},
			{"e-2023", "kings-of-the-box", "Kings of the Box", "2023-06-17", "2023-06-18", `["AWT"]`},
		} {
			must(q.UpsertEvent(ctx, gen.UpsertEventParams{ID: e.id, Slug: e.slug, SeasonYear: int64(year(e.start)), Name: e.name,
				StartDate: e.start, EndDate: e.end, Site: "Allevard", Country: "fra", Circuits: e.circuits, Disciplines: "[]", Status: "finished"}))
		}
		must(q.AddEventCompetition(ctx, gen.AddEventCompetitionParams{EventID: eventID(t, db), CompetitionID: "eeeeeeeeeeeeeeeeeeeeeee1"}))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Env: "dev", Addr: ":0", BaseURL: wwwBase}
	return web.Routes(web.Deps{Cfg: cfg, Auth: auth.New(db, cfg), DB: db, Purger: cache.NoopPurger{}, Blobs: blobs}, public.Module{})
}

func year(date string) int {
	y, _ := strconv.Atoi(date[:4])
	return y
}

func eventID(t *testing.T, db *store.DB) string {
	t.Helper()
	var id string
	if err := db.Read.QueryRow(`SELECT id FROM events WHERE slug = 'sample-event'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func wwwGet(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	return get(t, h, path)
}

func mustContain(t *testing.T, what, body string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("%s lacks %q", what, w)
		}
	}
}

func mustNotContain(t *testing.T, what, body string, bad ...string) {
	t.Helper()
	for _, b := range bad {
		if strings.Contains(body, b) {
			t.Errorf("%s contains %q", what, b)
		}
	}
}

var ldRe = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)

func jsonLD(t *testing.T, body string) map[string]any {
	t.Helper()
	m := ldRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no JSON-LD block")
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(m[1]), &v); err != nil {
		t.Fatalf("JSON-LD is not valid JSON: %v\n%s", err, m[1])
	}
	return v
}

func TestWWWHome(t *testing.T) {
	h := wwwApp(t)
	rec := wwwGet(t, h, "/")
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, body)
	}
	mustContain(t, "home", body,
		"Fly the Tour", "Season opener",
		"Sample Event", "30 August – 2 September 2030", // upcoming events
		"Sample news headline", `href="/news/sample-news"`, // latest news
		"Acro World Synchro 2024", "Fictional Duo", `href="/seasons/aws-2024"`, // featured standings
		`src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"`,
		`href="/results"`, // results nav kept
		"Learn the rules", "FAI Sporting Code Section 7B Aerobatics 2025",
		`<link rel="canonical" href="`+wwwBase+`/"`, `property="og:title"`, `<meta name="description"`)
	mustNotContain(t, "home", body, "Draft news", "Future news", "nope-2024")
}

func TestWWWNews(t *testing.T) {
	h := wwwApp(t)
	list := wwwGet(t, h, "/news").Body.String()
	mustContain(t, "news list", list, "Sample news headline", "Hostile &lt;b&gt;title&lt;/b&gt;", `<link rel="canonical" href="`+wwwBase+`/news"`)
	mustNotContain(t, "news list", list, "Draft news", "Future news")

	rec := wwwGet(t, h, "/news/sample-news")
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("article status %d", rec.Code)
	}
	mustContain(t, "article", body, "<title>Sample news headline | Acro World Tour</title>",
		`property="og:title" content="Sample news headline | Acro World Tour"`, `property="og:type" content="article"`,
		`property="og:description"`, `property="og:image" content="`+wwwBase+`/files/`, `<link rel="canonical" href="`+wwwBase+`/news/sample-news"`,
		"<strong>Lead paragraph</strong>", "<table>", "Pilot One")
	ld := jsonLD(t, body)
	if ld["@type"] != "NewsArticle" || ld["headline"] != "Sample news headline" || ld["datePublished"] != "2023-08-15T00:00:00Z" ||
		ld["mainEntityOfPage"] != wwwBase+"/news/sample-news" {
		t.Errorf("NewsArticle JSON-LD = %v", ld)
	}

	for _, slug := range []string{"draft-news", "future-news", "missing"} {
		if rec := wwwGet(t, h, "/news/"+slug); rec.Code != http.StatusNotFound {
			t.Errorf("/news/%s status %d, want 404", slug, rec.Code)
		}
	}
}

func TestWWWMarkdownCannotInjectScripts(t *testing.T) {
	h := wwwApp(t)
	for _, path := range []string{"/news/xss-news", "/about"} {
		body := wwwGet(t, h, path).Body.String()
		mustNotContain(t, path, body, "<script>alert", "<img src=x", `<img src="x"`, `href="javascript:`)
	}
	// The title is escaped by templ, and the JSON-LD cannot be closed early.
	body := wwwGet(t, h, "/news/xss-news").Body.String()
	mustContain(t, "xss article", body, "Hostile &lt;b&gt;title&lt;/b&gt;")
	if ld := jsonLD(t, body); ld["headline"] != "Hostile <b>title</b>" {
		t.Errorf("headline = %v", ld["headline"])
	}
}

func TestWWWEvents(t *testing.T) {
	h := wwwApp(t)
	body := wwwGet(t, h, "/events").Body.String()
	mustContain(t, "events", body, "Upcoming", "Sample Event", "Swiss Acro Tour", "Kings of the Box", "Allevard, FRA", `href="/events?circuit=SAT"`)
	// Past events are grouped by year, latest first.
	if strings.Index(body, ">2024<") > strings.Index(body, ">2023<") {
		t.Error("2024 must come before 2023")
	}
	filtered := wwwGet(t, h, "/events?circuit=SAT").Body.String()
	mustContain(t, "SAT events", filtered, "Swiss Acro Tour")
	mustNotContain(t, "SAT events", filtered, "Sample Event", "Kings of the Box")
	mustContain(t, "SAT canonical", filtered, `<link rel="canonical" href="`+wwwBase+`/events?circuit=SAT"`)

	rec := wwwGet(t, h, "/event/sample-event")
	page := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("event status %d", rec.Code)
	}
	mustContain(t, "event", page, "<title>Sample Event 2030 | Acro World Tour</title>", "The event description.", `href="https://example.org/"`,
		`href="/competitions/mini-solo-2024"`, "Mini Solo 2024", "solo + synchro", `property="og:image" content="`+wwwBase+`/files/`)
	mustNotContain(t, "event", page, "commented.example")
	ld := jsonLD(t, page)
	loc, _ := ld["location"].(map[string]any)
	if ld["@type"] != "SportsEvent" || ld["name"] != "Sample Event" || ld["startDate"] != "2030-08-30" || ld["endDate"] != "2030-09-02" ||
		ld["url"] != wwwBase+"/event/sample-event" || loc["name"] != "Lago Example, ITA" {
		t.Errorf("SportsEvent JSON-LD = %v", ld)
	}
	if rec := wwwGet(t, h, "/event/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown event status %d", rec.Code)
	}
}

func TestWWWPagesAndDocuments(t *testing.T) {
	h := wwwApp(t)
	body := wwwGet(t, h, "/about").Body.String()
	mustContain(t, "page", body, "<title>About us | Acro World Tour</title>", "We fly.", `<link rel="canonical" href="`+wwwBase+`/about"`)
	// A stored page named like a literal route never shadows it.
	if ev := wwwGet(t, h, "/events").Body.String(); strings.Contains(ev, "shadow") || !strings.Contains(ev, "Upcoming") {
		t.Error("/events was shadowed by a page")
	}
	rec := wwwGet(t, h, "/no-such-page")
	if rec.Code != http.StatusNotFound || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("unknown page: status %d, cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	docs := wwwGet(t, h, "/documents").Body.String()
	mustContain(t, "documents", docs, "FAI Sporting Code Section 7B Aerobatics 2025", `href="/files/`, "sporting-code")
}

func TestWWWSitemapAndRobots(t *testing.T) {
	h := wwwApp(t)
	rec := wwwGet(t, h, "/sitemap.xml")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/xml") {
		t.Fatalf("sitemap: status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var set struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatalf("sitemap is not valid XML: %v", err)
	}
	have := map[string]bool{}
	for _, u := range set.URLs {
		have[u.Loc] = true
	}
	for _, p := range []string{"/", "/events", "/news", "/news/sample-news", "/news/xss-news", "/event/sample-event", "/event/swiss-acro-tour",
		"/competitions/mini-solo-2024", "/seasons/aws-2024", "/pilots/1001/alice-aerial", "/about", "/result/2023/nest-games", "/result/2022/world-acro-cup"} {
		if !have[wwwBase+p] {
			t.Errorf("sitemap lacks %s", p)
		}
	}
	for _, p := range []string{"/news/draft-news", "/news/future-news", "/competitions/hidden-cup"} {
		if have[wwwBase+p] {
			t.Errorf("sitemap lists non-public %s", p)
		}
	}
	robots := wwwGet(t, h, "/robots.txt").Body.String()
	mustContain(t, "robots", robots, "Disallow: /admin/", "Sitemap: "+wwwBase+"/sitemap.xml")
}

func TestWWWLegacyRedirects(t *testing.T) {
	h := wwwApp(t)
	var pdf string
	for from, to := range map[string]string{
		"/news/sample-news.html":   "/news/sample-news",
		"/event/sample-event.html": "/event/sample-event",
		"/events.html":             "/events",
		"/pilots.html":             "/pilots",
		"/results.html":            "/results",
		"/index.html":              "/",
		"/result/2023/sample.html": "/result/2023/sample",
	} {
		rec := wwwGet(t, h, from)
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != to {
			t.Errorf("GET %s = %d -> %q, want 301 -> %s", from, rec.Code, rec.Header().Get("Location"), to)
		}
	}
	rec := wwwGet(t, h, "/assets/files/sporting_code_s7_b_-_aerobatics_2025.pdf")
	if pdf = rec.Header().Get("Location"); rec.Code != http.StatusMovedPermanently || !strings.HasPrefix(pdf, "/files/") {
		t.Errorf("legacy PDF = %d -> %q", rec.Code, pdf)
	}
	if rec := wwwGet(t, h, "/news/never-existed.html"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown .html status %d", rec.Code)
	}
}

func TestWWWArchivedResults(t *testing.T) {
	h := wwwApp(t)
	rec := wwwGet(t, h, "/result/2023/nest-games")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Theo de Blic") || !strings.Contains(body, "AWT Nest Games 2023") {
		t.Fatalf("archived page: status %d\n%.300s", rec.Code, body)
	}
	mustNotContain(t, "archived page", body, "googletagmanager", "gtag(")
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=86400") {
		t.Errorf("archive Cache-Control = %q", cc)
	}
	css := wwwGet(t, h, "/assets/css/style.css")
	if css.Code != http.StatusOK || !strings.HasPrefix(css.Header().Get("Content-Type"), "text/css") {
		t.Errorf("archive css: status %d, type %q", css.Code, css.Header().Get("Content-Type"))
	}
	if rec := wwwGet(t, h, "/result/2023/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown archive page status %d", rec.Code)
	}
	if rec := wwwGet(t, h, "/assets/../etc/passwd"); rec.Code == http.StatusOK {
		t.Error("asset traversal served a file")
	}
}

func TestWWWJSONLDCannotBeClosedEarly(t *testing.T) {
	h := wwwApp(t)
	body := wwwGet(t, h, "/news/ld-news").Body.String()
	if strings.Contains(body, "<script>alert(9)") || strings.Count(body, `type="application/ld+json"`) != 1 {
		t.Fatal("a news title injected a script element")
	}
	if ld := jsonLD(t, body); ld["headline"] != "</script><script>alert(9)</script>" {
		t.Errorf("headline = %v", ld["headline"])
	}
}

func TestWWWCircuitLinksAreEscaped(t *testing.T) {
	h := wwwApp(t)
	body := wwwGet(t, h, "/events?circuit=A%26B%22").Body.String()
	mustContain(t, "events", body, `circuit=A%26B%22`)
	mustNotContain(t, "events", body, `circuit=A&B"`)
}

func TestWWWArchiveUsesAppIconsAndShipsNoMissingAssets(t *testing.T) {
	h := wwwApp(t)
	body := wwwGet(t, h, "/result/2023/nest-games").Body.String()
	mustContain(t, "archive", body, `href="/static/img/favicon-32x32.png?v=`)
	mustNotContain(t, "archive", body, `href="/favicon-32x32.png"`, "mask-icon", "safari-pinned-tab")
	css := wwwGet(t, h, "/assets/css/style.css").Body.String()
	mustNotContain(t, "archive css", css, "header.jpg")
}
