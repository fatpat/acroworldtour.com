package importer_test

import (
	"context"
	"strings"
	"testing"

	"awt/internal/importer"
	"awt/internal/store/gen"
)

const wwwDir = "testdata/www"

func TestWWWImportsNewsEventsDocumentsAndRedirects(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	rep, err := importer.WWW(ctx, db, blobs, wwwDir)
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]int{"news": 1, "events": 1, "documents": 1, "files": 4, "redirects": 8} {
		if rep.Counts[k] != want {
			t.Errorf("Counts[%s] = %d, want %d (report %v)", k, rep.Counts[k], want, rep)
		}
	}
	if !strings.Contains(strings.Join(rep.Warnings, "\n"), "missing.png") {
		t.Errorf("warnings %v do not mention the missing image", rep.Warnings)
	}

	q := gen.New(db.Read)
	n, err := q.GetNewsBySlug(ctx, "sample-news")
	if err != nil {
		t.Fatal(err)
	}
	if n.Title != "Sample news headline" {
		t.Errorf("title = %q", n.Title)
	}
	if n.PublishedAt.String != "2023-08-15T00:00:00Z" {
		t.Errorf("published_at = %q, want the home teaser date", n.PublishedAt.String)
	}
	if !n.CoverFile.Valid {
		t.Error("the first image should become the cover")
	}
	for _, want := range []string{
		"**Lead paragraph** with a [link](/events) & *style*.",
		"### A heading",
		`Second paragraph with a \*star\* and`,
		"- one\n- two",
		"![Second picture](/files/",
		"| Pos. | Pilot | Pts. |",
		"| 1 | Pilot One | 39.757 |",
		"### Acro World Tour",
	} {
		if !strings.Contains(n.BodyMd, want) {
			t.Errorf("body lacks %q:\n%s", want, n.BodyMd)
		}
	}
	if strings.Contains(n.BodyMd, "lead.png") || strings.Contains(n.BodyMd, "<script") {
		t.Errorf("body keeps the cover image or raw HTML:\n%s", n.BodyMd)
	}
	if n.Excerpt == "" {
		t.Error("excerpt should be derived from the body")
	}

	e, err := q.GetEventBySlugLatest(ctx, "sample-event")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Sample Event" || e.StartDate != "2030-08-30" || e.EndDate != "2030-09-02" || e.SeasonYear != 2030 {
		t.Errorf("event = %+v", e)
	}
	if e.Site != "Lago Example" || e.Country != "ita" || e.Circuits != `["AWT","AWQ"]` || e.Disciplines != `["solo","synchro"]` {
		t.Errorf("event details = %+v", e)
	}
	if e.Website.String != "https://example.org/" || e.Status != "confirmed" || !e.CoverFile.Valid {
		t.Errorf("event links = %+v", e)
	}

	docs, err := q.ListDocuments(ctx)
	if err != nil || len(docs) != 1 || docs[0].Year.Int64 != 2025 || docs[0].Category != "sporting-code" || !docs[0].FileID.Valid {
		t.Errorf("documents = %+v, err %v", docs, err)
	}
	if s, err := q.GetSiteSetting(ctx, "sporting_code_document"); err != nil || s.Value != `"`+docs[0].ID+`"` {
		t.Errorf("sporting_code_document = %+v, err %v", s, err)
	}

	for from, to := range map[string]string{
		"/news/sample-news.html":   "/news/sample-news",
		"/event/sample-event.html": "/event/sample-event",
		"/results.html":            "/results",
		"/pilots.html":             "/pilots",
		"/events.html":             "/events",
		"/index.html":              "/",
		"/result/2023/sample.html": "/result/2023/sample",
		"/assets/files/sporting_code_s7_b_-_aerobatics_2025.pdf": "/files/" + docs[0].FileID.String,
	} {
		r, err := q.GetRedirect(ctx, from)
		if err != nil || r.ToUrl != to || !r.Permanent {
			t.Errorf("redirect %s = %+v, err %v; want permanent to %s", from, r, err, to)
		}
	}
}

func TestWWWIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	if _, err := importer.WWW(ctx, db, blobs, wwwDir); err != nil {
		t.Fatal(err)
	}
	before := map[string]int{}
	tables := []string{"news", "events", "documents", "redirects", "files", "site_settings"}
	for _, tb := range tables {
		before[tb] = count(t, db, "SELECT COUNT(*) FROM "+tb)
	}
	var newsID string
	if err := db.Read.QueryRow("SELECT id FROM news").Scan(&newsID); err != nil {
		t.Fatal(err)
	}
	if _, err := importer.WWW(ctx, db, blobs, wwwDir); err != nil {
		t.Fatal(err)
	}
	for _, tb := range tables {
		if got := count(t, db, "SELECT COUNT(*) FROM "+tb); got != before[tb] {
			t.Errorf("%s: %d rows after the second run, %d after the first", tb, got, before[tb])
		}
	}
	var again string
	if err := db.Read.QueryRow("SELECT id FROM news").Scan(&again); err != nil || again != newsID {
		t.Errorf("news id changed on re-import: %q -> %q (%v)", newsID, again, err)
	}
}

func TestWWWRejectsANonLegacyDirectory(t *testing.T) {
	db, blobs := setup(t)
	if _, err := importer.WWW(context.Background(), db, blobs, t.TempDir()); err == nil {
		t.Fatal("want an error for a directory without news/ and event/")
	}
}
