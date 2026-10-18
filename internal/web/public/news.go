package public

import (
	"net/http"
	"strconv"
	"time"

	"awt/internal/markdown"
	"awt/internal/store/gen"
)

// newsPageSize is the news index page size (spec 07 §10.2: pagination).
const newsPageSize = 9

type newsCard struct {
	Slug, Title, Excerpt string
	Date, DateISO        string
	Image                string
}

func (c newsCard) Href() string { return "/news/" + c.Slug }

func cardFromNews(n gen.News) newsCard {
	c := newsCard{Slug: n.Slug, Title: n.Title, Excerpt: n.Excerpt, Image: fileURL(n.CoverFile)}
	if c.Excerpt == "" {
		c.Excerpt = markdown.Plain(n.BodyMd, 180)
	}
	if t, err := time.Parse(time.RFC3339, n.PublishedAt.String); err == nil {
		c.Date, c.DateISO = t.UTC().Format("2 January 2006"), t.UTC().Format(time.DateOnly)
	}
	return c
}

type newsListView struct {
	Cards      []newsCard
	Page       int
	Prev, Next string
}

// newsList is the paginated news index (published articles, newest first).
func (w *www) newsList(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(page, 1)
	total, err := w.q.CountPublishedNews(ctx, w.nowStored())
	if err != nil {
		serverError(rw, r, err)
		return
	}
	rows, err := w.q.ListPublishedNews(ctx, gen.ListPublishedNewsParams{Now: w.nowStored(), Max: newsPageSize, Skip: int64((page - 1) * newsPageSize)})
	if err != nil {
		serverError(rw, r, err)
		return
	}
	v := newsListView{Page: page}
	for _, n := range rows {
		v.Cards = append(v.Cards, cardFromNews(n))
	}
	if page > 1 {
		v.Prev = "/news"
		if page > 2 {
			v.Prev += "?page=" + strconv.Itoa(page-1)
		}
	}
	if int64(page*newsPageSize) < total {
		v.Next = "/news?page=" + strconv.Itoa(page+1)
	}
	canonical := "/news"
	if page > 1 {
		canonical += "?page=" + strconv.Itoa(page)
	}
	st := w.settings(ctx)
	renderPage(rw, r, NewsListPage(w.meta("News", w.describe(st, "News from the Acro World Tour: results, season updates and announcements."), canonical, ""), v))
}

type newsArticleView struct {
	Card   newsCard
	HTML   string
	Cover  string
	Author string
	LD     ldNewsArticle
}

// newsArticle is one article with its JSON-LD NewsArticle.
func (w *www) newsArticle(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, err := w.q.GetPublishedNewsBySlug(ctx, gen.GetPublishedNewsBySlugParams{Slug: r.PathValue("slug"), Now: w.nowStored()})
	if err != nil {
		w.fail(rw, r, err)
		return
	}
	card := cardFromNews(n)
	path := "/news/" + n.Slug
	m := w.meta(n.Title, card.Excerpt, path, card.Image)
	m.OGType = "article"
	ld := ldNewsArticle{Context: "https://schema.org", Type: "NewsArticle", Headline: n.Title, Description: card.Excerpt,
		DatePublished: n.PublishedAt.String, MainEntity: w.abs(path), Publisher: w.publisher()}
	if n.Author != "" {
		ld.Author = &ldPerson{Type: "Person", Name: n.Author}
	}
	if card.Image != "" {
		ld.Image = []string{w.abs(card.Image)}
	}
	renderPage(rw, r, NewsArticlePage(m, newsArticleView{Card: card, HTML: markdown.HTML(n.BodyMd), Cover: card.Image, Author: n.Author, LD: ld}))
}
