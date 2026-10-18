package public

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"awt/internal/markdown"
	"awt/internal/store/gen"
)

type pageView struct {
	HTML string
}

// page serves a stored generic page (about, contact, legal notice, ...) at
// /{slug}. Unknown slugs answer 404 or the slug's redirect (legacy
// /events.html and friends).
func (w *www) page(rw http.ResponseWriter, r *http.Request) {
	p, err := w.q.GetPage(r.Context(), r.PathValue("slug"))
	if err != nil {
		w.fail(rw, r, err)
		return
	}
	desc := markdown.Plain(p.BodyMd, 180)
	renderPage(rw, r, ContentPage(w.meta(p.Title, w.describe(w.settings(r.Context()), desc), "/"+p.Slug, ""), pageView{HTML: markdown.HTML(p.BodyMd)}))
}

type docItem struct {
	Title, Category string
	Year            int64
	Href            string
	External        bool
}

type documentsView struct{ Docs []docItem }

func docHref(d gen.Document) (href string, external bool) {
	if d.FileID.Valid && d.FileID.String != "" {
		return "/files/" + d.FileID.String, false
	}
	return d.Url.String, true
}

// documents lists the downloadable documents (sporting code, regulations).
func (w *www) documents(rw http.ResponseWriter, r *http.Request) {
	rows, err := w.q.ListDocuments(r.Context())
	if err != nil {
		serverError(rw, r, err)
		return
	}
	var v documentsView
	for _, d := range rows {
		href, ext := docHref(d)
		v.Docs = append(v.Docs, docItem{Title: d.Title, Category: d.Category, Year: d.Year.Int64, Href: href, External: ext})
	}
	renderPage(rw, r, DocumentsPage(w.meta("Documents", "Rules and documents of the Acro World Tour: FAI Sporting Code Section 7B and local regulations.", "/documents", ""), v))
}

// sportingCode resolves the document linked as "Learn the rules" on the
// home page, falling back to the FAI copy (spec 07 §9 #9).
func (w *www) sportingCode(r *http.Request, id string) (title, href string) {
	title, href = "FAI Sporting Code Section 7B (aerobatics)", sportingCodeURL
	if id == "" {
		return title, href
	}
	d, err := w.q.GetDocument(r.Context(), id)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("sporting code document", "id", id, "err", err)
		}
		return title, href
	}
	h, _ := docHref(d)
	if h == "" {
		return title, href
	}
	return d.Title, h
}
