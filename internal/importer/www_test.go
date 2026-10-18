package importer_test

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
		`2024\. A year, not a list`,
		`\= not a setext line`,
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

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWWWUndatedNewsTakeTheirFirstCommitDate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	www := filepath.Join(repo, "www")
	copyTree(t, wwwDir, www)
	if err := os.Remove(filepath.Join(www, "index.html")); err != nil { // no teaser date
		t.Fatal(err)
	}
	git := func(date string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=T", "-c", "user.email=t@example.org", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("2022-03-04T05:06:07+01:00", "init", "-q")
	git("2022-03-04T05:06:07+01:00", "add", ".")
	git("2022-03-04T05:06:07+01:00", "commit", "-q", "-m", "add")
	// A later edit must not change the date: only the commit that added the file counts.
	f := filepath.Join(www, "news", "sample-news.html")
	b, _ := os.ReadFile(f)
	if err := os.WriteFile(f, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	git("2024-01-01T00:00:00+00:00", "commit", "-q", "-am", "edit")

	ctx := context.Background()
	db, blobs := setup(t)
	if _, err := importer.WWW(ctx, db, blobs, www); err != nil {
		t.Fatal(err)
	}
	n, err := gen.New(db.Read).GetNewsBySlug(ctx, "sample-news")
	if err != nil || n.PublishedAt.String != "2022-03-04T04:06:07Z" {
		t.Errorf("published_at = %q, err %v; want the first commit date in UTC", n.PublishedAt.String, err)
	}
}

// Not parallel: it changes the working directory.
func TestWWWUndatedNewsDateWorksWithARelativeFrom(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	copyTree(t, wwwDir, filepath.Join(repo, "www"))
	if err := os.Remove(filepath.Join(repo, "www", "index.html")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"commit", "-q", "-m", "add"}} {
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=T", "-c", "user.email=t@example.org", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2021-02-03T04:05:06+00:00", "GIT_COMMITTER_DATE=2021-02-03T04:05:06+00:00")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// Dir -> db setup first: setup() uses absolute temp paths.
	ctx := context.Background()
	db, blobs := setup(t)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if _, err := importer.WWW(ctx, db, blobs, "www"); err != nil {
		t.Fatal(err)
	}
	n, err := gen.New(db.Read).GetNewsBySlug(ctx, "sample-news")
	if err != nil || n.PublishedAt.String != "2021-02-03T04:05:06Z" {
		t.Errorf("published_at = %q, err %v; want the commit date with a relative --from", n.PublishedAt.String, err)
	}
}

func TestWWWUndatedNewsWithoutHistoryStayDrafts(t *testing.T) {
	dir := t.TempDir() // outside any git repository
	copyTree(t, wwwDir, dir)
	_ = os.Remove(filepath.Join(dir, "index.html"))
	ctx := context.Background()
	db, blobs := setup(t)
	rep, err := importer.WWW(ctx, db, blobs, dir)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := gen.New(db.Read).GetNewsBySlug(ctx, "sample-news")
	if n.PublishedAt.Valid || !strings.Contains(strings.Join(rep.Warnings, "\n"), "imported as a draft") {
		t.Errorf("published_at = %+v, warnings %v; want a draft with a warning", n.PublishedAt, rep.Warnings)
	}
}

func TestWWWDoesNotResurrectDeletedContent(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	if _, err := importer.WWW(ctx, db, blobs, wwwDir); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE news SET deleted_at = '2025-01-01T00:00:00Z' WHERE slug = 'sample-news'`,
		`UPDATE events SET deleted_at = '2025-01-01T00:00:00Z' WHERE slug = 'sample-event'`,
	} {
		if _, err := db.Write.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := importer.WWW(ctx, db, blobs, wwwDir)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Counts["news"] != 0 || rep.Counts["events"] != 0 {
		t.Errorf("counts = %v, want no news or events written", rep.Counts)
	}
	for _, tb := range []string{"news", "events"} {
		if n := count(t, db, "SELECT COUNT(*) FROM "+tb); n != 1 {
			t.Errorf("%s has %d rows, want the single deleted one", tb, n)
		}
		if n := count(t, db, "SELECT COUNT(*) FROM "+tb+" WHERE deleted_at IS NULL"); n != 0 {
			t.Errorf("%s was restored", tb)
		}
	}
	if w := strings.Join(rep.Warnings, "\n"); !strings.Contains(w, "not restored") {
		t.Errorf("warnings %v lack the skip notice", rep.Warnings)
	}
}

func TestParseEventDatesAcrossNewYear(t *testing.T) {
	// exercised through a fixture to keep parseEventDates unexported
	dir := t.TempDir()
	copyTree(t, wwwDir, dir)
	p := filepath.Join(dir, "event", "sample-event.html")
	b, _ := os.ReadFile(p)
	b = []byte(strings.Replace(string(b), "August 30th - September 2nd 2030", "December 30th - January 2nd 2030", 1))
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db, blobs := setup(t)
	if _, err := importer.WWW(ctx, db, blobs, dir); err != nil {
		t.Fatal(err)
	}
	e, err := gen.New(db.Read).GetEventBySlugLatest(ctx, "sample-event")
	if err != nil || e.StartDate != "2030-12-30" || e.EndDate != "2031-01-02" {
		t.Errorf("event dates = %q..%q, err %v", e.StartDate, e.EndDate, err)
	}
}

func TestWWWFailsOnDatabaseErrorDuringUpload(t *testing.T) {
	ctx := context.Background()
	db, blobs := setup(t)
	if _, err := db.Write.ExecContext(ctx, `DROP TABLE files`); err != nil {
		t.Fatal(err)
	}
	if _, err := importer.WWW(ctx, db, blobs, wwwDir); err == nil {
		t.Fatal("want an error when files cannot be written")
	}
}
