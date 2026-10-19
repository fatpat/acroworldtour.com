package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var pdfBytes = []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n")

func TestContentNewsCRUD(t *testing.T) {
	e := newEnv(t)
	e.wantStatus(e.get("/manager/content"), 200, "News", "Events", "Documents", "Pages", "Redirects")
	e.wantStatus(e.get("/manager/content/news/new"), 200, `name="_csrf"`, `hx-post="/manager/content/preview"`, "Preview")

	// Validation: missing title, bad slug, bad date.
	e.wantStatus(e.post("/manager/content/news", url.Values{"title": {""}}), 422, "title: required")
	e.wantStatus(e.post("/manager/content/news", url.Values{"title": {"x"}, "slug": {"Bad Slug"}}), 422, "lowercase letters")
	e.wantStatus(e.post("/manager/content/news", url.Values{"title": {"x"}, "published_at": {"yesterday"}}), 422, "published_at")

	// Create: the slug comes from the title, the hostile text is escaped.
	e.wantRedirect(e.post("/manager/content/news", url.Values{"title": {"Big <b>News</b> Day"}, "author": {"Eve"},
		"published_at": {"2024-05-01"}, "body_md": {"Hello <script>alert(1)</script>"}}), "/manager/content/news?ok=created")
	list := e.get("/manager/content/news")
	e.wantStatus(list, 200, "Big &lt;b&gt;News&lt;/b&gt; Day", "big-bnewsb-day", "2024-05-01")
	if strings.Contains(list.Body.String(), "<script>alert") {
		t.Fatal("list renders an unescaped script tag")
	}
	id := firstID(t, list.Body.String(), "/manager/content/news/")

	// Duplicate slug: 409 with the form re-rendered.
	e.wantStatus(e.post("/manager/content/news", url.Values{"title": {"Other"}, "slug": {"big-bnewsb-day"}}), 409, "already exists")

	// Edit shows the stored values; update as a draft.
	e.wantStatus(e.get("/manager/content/news/"+id+"/edit"), 200, "Eve", "2024-05-01")
	e.wantRedirect(e.post("/manager/content/news/"+id, url.Values{"title": {"Renamed"}, "slug": {"renamed"}, "published_at": {""}}),
		"/manager/content/news?ok=saved")
	e.wantStatus(e.get("/manager/content/news"), 200, "Renamed", "Draft")
	e.wantStatus(e.post("/manager/content/news/ffffffffffffffffffffffff", url.Values{"title": {"x"}}), 404)

	// Delete needs the confirmation POST, then the article is gone.
	e.wantStatus(e.get("/manager/content/news/"+id+"/delete"), 200, "Renamed", "Confirm delete")
	e.wantRedirect(e.post("/manager/content/news/"+id+"/delete", nil), "/manager/content/news?ok=deleted")
	if strings.Contains(e.get("/manager/content/news").Body.String(), "Renamed") {
		t.Fatal("deleted article still listed")
	}
	e.wantStatus(e.get("/manager/content/news/"+id+"/edit"), 404)
}

func TestContentPreviewDropsRawHTML(t *testing.T) {
	e := newEnv(t)
	rec := e.post("/manager/content/preview", url.Values{"body_md": {"# Hi\n\n**bold** <script>alert(1)</script> <img src=x onerror=alert(2)> [x](javascript:alert(3))"}})
	e.wantStatus(rec, 200, "<h1>Hi</h1>", "<strong>bold</strong>")
	for _, bad := range []string{"<script", "<img", "onerror=alert", `href="javascript:`} {
		if strings.Contains(rec.Body.String(), bad) {
			t.Errorf("preview contains %q: %s", bad, rec.Body.String())
		}
	}
}

func TestContentEvents(t *testing.T) {
	e := newEnv(t)
	form := func(m map[string]string) url.Values {
		v := url.Values{"name": {"Acro Open"}, "start_date": {"2030-08-09"}, "end_date": {"2030-08-15"}, "status": {"planned"},
			"circuits": {"awt, awq"}, "solo": {"1"}, "description_md": {"Fly **high**"}}
		for k, x := range m {
			v.Set(k, x)
		}
		return v
	}
	e.wantStatus(e.get("/manager/content/events/new"), 200, "Linked competitions", `name="solo"`)
	e.wantStatus(e.post("/manager/content/events", form(map[string]string{"end_date": "2030-08-01"})), 422, "must not be before")
	e.wantStatus(e.post("/manager/content/events", form(map[string]string{"status": "bogus"})), 422, "status")
	e.wantStatus(e.post("/manager/content/events", form(map[string]string{"website": "javascript:alert(1)"})), 422, "http(s) URL")
	e.wantStatus(e.post("/manager/content/events", form(map[string]string{"season_year": "abc"})), 422, "season_year")
	bad := form(nil)
	bad.Add("competitions", "nope")
	e.wantStatus(e.post("/manager/content/events", bad), 422, "unknown competition")

	e.wantRedirect(e.post("/manager/content/events", form(nil)), "/manager/content/events?ok=created")
	list := e.get("/manager/content/events")
	e.wantStatus(list, 200, "Acro Open", "2030", "2030-08-09 – 2030-08-15", "planned")
	id := firstID(t, list.Body.String(), "/manager/content/events/")
	e.wantStatus(e.post("/manager/content/events", form(nil)), 409, "already exists") // same slug and season

	rec := e.get("/manager/content/events/" + id + "/edit")
	e.wantStatus(rec, 200, "Acro Open", "AWT, AWQ", `name="solo" value="1" checked`)
	e.wantRedirect(e.post("/manager/content/events/"+id, form(map[string]string{"status": "cancelled", "synchro": "1"})), "/manager/content/events?ok=saved")
	e.wantStatus(e.get("/manager/content/events"), 200, "cancelled")
	e.wantRedirect(e.post("/manager/content/events/"+id+"/delete", nil), "/manager/content/events?ok=deleted")
	e.wantStatus(e.get("/manager/content/events/"+id+"/edit"), 404)
}

func TestContentDocuments(t *testing.T) {
	e := newEnv(t)
	e.wantStatus(e.get("/manager/content/documents/new"), 200, `enctype="multipart/form-data"`)
	e.wantStatus(e.upload("/manager/content/documents", map[string]string{"title": "Rules"}, "", nil), 422, "upload a PDF or give a URL")
	e.wantStatus(e.upload("/manager/content/documents", map[string]string{"title": "Rules"}, "notes.txt", []byte("hello")), 422, "must be a PNG")
	e.wantStatus(e.upload("/manager/content/documents", map[string]string{"title": "Rules", "url": "ftp://x"}, "", nil), 422, "http(s) URL")

	e.wantRedirect(e.upload("/manager/content/documents", map[string]string{"title": "Sporting Code 2026", "year": "2026", "category": "sporting-code"},
		"code.pdf", pdfBytes), "/manager/content/documents?ok=created")
	list := e.get("/manager/content/documents")
	e.wantStatus(list, 200, "Sporting Code 2026", "2026", "sporting-code", "PDF ")
	id := firstID(t, list.Body.String(), "/manager/content/documents/")

	// An external URL document, then edit the first one (keeps its file).
	e.wantRedirect(e.upload("/manager/content/documents", map[string]string{"title": "FAI copy", "url": "https://example.org/code.pdf"}, "", nil),
		"/manager/content/documents?ok=created")
	e.wantRedirect(e.upload("/manager/content/documents/"+id, map[string]string{"title": "Sporting Code 2026 (rev)", "year": "2026"}, "", nil),
		"/manager/content/documents?ok=saved")
	e.wantStatus(e.get("/manager/content/documents"), 200, "Sporting Code 2026 (rev)", "https://example.org/code.pdf")
	docs, err := e.svc.ListDocuments(context.Background())
	if err != nil || len(docs) != 2 {
		t.Fatalf("documents = %+v, %v", docs, err)
	}
	for _, d := range docs {
		if d.Title == "Sporting Code 2026 (rev)" && d.FileID == "" {
			t.Fatal("editing a document dropped its file")
		}
	}
	e.wantRedirect(e.post("/manager/content/documents/"+id+"/delete", nil), "/manager/content/documents?ok=deleted")
	e.wantStatus(e.post("/manager/content/documents/"+id+"/delete", nil), 404)
}

func TestContentPages(t *testing.T) {
	e := newEnv(t)
	e.wantStatus(e.post("/manager/content/pages", url.Values{"slug": {"events"}, "title": {"x"}}), 422, "reserved")
	e.wantStatus(e.post("/manager/content/pages", url.Values{"slug": {"live"}, "title": {"x"}}), 422, "reserved")
	e.wantStatus(e.post("/manager/content/pages", url.Values{"slug": {"Bad Slug"}, "title": {"x"}}), 422, "lowercase letters")
	e.wantStatus(e.post("/manager/content/pages", url.Values{"slug": {"about"}, "title": {""}}), 422, "title: required")
	e.wantRedirect(e.post("/manager/content/pages", url.Values{"slug": {"about"}, "title": {"About"}, "body_md": {"We fly."}}), "/manager/content/pages?ok=created")
	e.wantStatus(e.post("/manager/content/pages", url.Values{"slug": {"about"}, "title": {"Again"}}), 409, "already exists")
	e.wantStatus(e.get("/manager/content/pages"), 200, "About", "/about")
	e.wantStatus(e.get("/manager/content/pages/about/edit"), 200, "We fly.", "<code>/about</code>")
	e.wantRedirect(e.post("/manager/content/pages/about", url.Values{"title": {"About us"}, "body_md": {"New text"}}), "/manager/content/pages?ok=saved")
	p, err := e.svc.GetPage(context.Background(), "about")
	if err != nil || p.Title != "About us" || p.BodyMD != "New text" {
		t.Fatalf("page = %+v, %v", p, err)
	}
	e.wantStatus(e.post("/manager/content/pages/missing", url.Values{"title": {"x"}}), 404)
	e.wantRedirect(e.post("/manager/content/pages/about/delete", nil), "/manager/content/pages?ok=deleted")
	e.wantStatus(e.get("/manager/content/pages/about/edit"), 404)
}

func TestContentRedirects(t *testing.T) {
	e := newEnv(t)
	for _, c := range []struct{ from, to, msg string }{
		{"old", "/new", "from:"},
		{"/old", "//evil.example", "to:"},
		{"/old", "javascript:alert(1)", "to:"},
		{"/old", "/old", "must differ"},
		{"/manager/users", "/", "admin paths"},
		{"/old?x=1", "/new", "from:"},
	} {
		e.wantStatus(e.post("/manager/content/redirects", url.Values{"from": {c.from}, "to": {c.to}}), 422, c.msg)
	}
	e.wantRedirect(e.post("/manager/content/redirects", url.Values{"from": {"/old"}, "to": {"/new"}}), "/manager/content/redirects?ok=saved")
	e.wantRedirect(e.post("/manager/content/redirects", url.Values{"from": {"/temp"}, "to": {"https://example.org/x"}, "temporary": {"1"}}), "/manager/content/redirects?ok=saved")
	e.wantStatus(e.get("/manager/content/redirects"), 200, "/old", "/new", "301", "302", "https://example.org/x")
	rs, _ := e.svc.ListRedirects(context.Background())
	if len(rs) != 2 {
		t.Fatalf("redirects = %+v", rs)
	}
	e.wantRedirect(e.post("/manager/content/redirects/delete", url.Values{"from": {"/old"}}), "/manager/content/redirects?ok=deleted")
	e.wantStatus(e.post("/manager/content/redirects/delete", url.Values{"from": {"/old"}}), 404)
}

func TestContentSettings(t *testing.T) {
	e := newEnv(t)
	e.wantStatus(e.get("/manager/content/settings"), 200, "Hero: title", "Seasons whose top 5")
	e.wantStatus(e.post("/manager/content/settings", url.Values{"featured_seasons": {"AWT 2026!"}}), 422, "season codes")
	e.wantStatus(e.post("/manager/content/settings", url.Values{"featured_video": {"javascript:x"}}), 422, "http(s) URL")
	e.wantStatus(e.post("/manager/content/settings", url.Values{"contact_email": {"nope"}}), 422, "email")
	e.wantStatus(e.post("/manager/content/settings", url.Values{"hero_image": {strings.Repeat("a", 64)}}), 422, "is unknown")
	e.wantStatus(e.post("/manager/content/settings", url.Values{"sporting_code_document": {"nope"}}), 422, "unknown document")
	e.wantRedirect(e.post("/manager/content/settings", url.Values{"hero_title": {"Fly the Tour"}, "hero_subtitle": {"Best pilots"},
		"featured_seasons": {"awt-2026, awq-2026"}, "contact_email": {"info@example.org"}}), "/manager/content/settings?ok=saved")
	e.wantStatus(e.get("/manager/content/settings"), 200, "Fly the Tour", "awt-2026, awq-2026", "info@example.org")
	st, err := e.svc.GetSettings(context.Background())
	if err != nil || st.HeroTitle != "Fly the Tour" || len(st.FeaturedSeasons) != 2 {
		t.Fatalf("settings = %+v, %v", st, err)
	}
	var raw string
	if err := e.deps.DB.Read.QueryRow(`SELECT value FROM site_settings WHERE key = 'featured_seasons'`).Scan(&raw); err != nil || raw != `["awt-2026","awq-2026"]` {
		t.Fatalf("stored featured_seasons = %q, %v", raw, err)
	}
}

func TestContentRequiresCSRFAndAdmin(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/manager/content/news", "/manager/content/events", "/manager/content/documents", "/manager/content/pages",
		"/manager/content/redirects", "/manager/content/redirects/delete", "/manager/content/settings", "/manager/content/preview", "/manager/content/news/abc"} {
		if rec := e.postAs(&e.admin, p, url.Values{"title": {"x"}}, false); rec.Code != http.StatusForbidden {
			t.Errorf("POST %s without CSRF = %d, want 403", p, rec.Code)
		}
		if rec := e.postAs(&e.admin, p, url.Values{"_csrf": {"wrong"}}, false); rec.Code != http.StatusForbidden {
			t.Errorf("POST %s with a wrong CSRF = %d, want 403", p, rec.Code)
		}
		// A scorer is refused even with a valid token.
		if rec := e.postAs(&e.scorer, p, url.Values{"title": {"x"}}, true); rec.Code != http.StatusForbidden {
			t.Errorf("scorer POST %s = %d, want 403", p, rec.Code)
		}
	}
	for _, p := range []string{"/manager/content", "/manager/content/news", "/manager/content/news/new", "/manager/content/events", "/manager/content/documents",
		"/manager/content/pages", "/manager/content/redirects", "/manager/content/settings"} {
		if rec := e.do(httptest.NewRequest(http.MethodGet, p, nil), &e.scorer); rec.Code != http.StatusForbidden {
			t.Errorf("scorer GET %s = %d, want 403", p, rec.Code)
		}
		if rec := e.do(httptest.NewRequest(http.MethodGet, p, nil), nil); rec.Code != http.StatusSeeOther {
			t.Errorf("anonymous GET %s = %d, want 303 to the login", p, rec.Code)
		}
	}
	// Oversized bodies are refused before parsing.
	big := url.Values{"title": {"x"}, "body_md": {strings.Repeat("a", 300<<10)}}
	if rec := e.postAs(&e.admin, "/manager/content/news", big, true); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized news body = %d, want 413", rec.Code)
	}
}
