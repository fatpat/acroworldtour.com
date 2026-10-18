package public

import (
	"encoding/xml"
	"net/http"
	"slices"
	"strings"

	"awt/internal/store/gen"
	"awt/web/archive"
)

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapSet struct {
	XMLName xml.Name     `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []sitemapURL `xml:"url"`
}

// sitemap lists the home and index pages, news, events, pages, published
// competitions, seasons, pilots and the archived result pages (spec 07 §7).
func (w *www) sitemap(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	set := sitemapSet{}
	add := func(path, lastMod string) {
		set.URLs = append(set.URLs, sitemapURL{Loc: w.abs(path), LastMod: firstN(lastMod, 10)})
	}
	for _, p := range []string{"/", "/events", "/news", "/documents", "/results", "/seasons", "/competitions", "/pilots", "/teams", "/judges", "/tricks"} {
		add(p, "")
	}
	fail := func(err error) bool {
		if err != nil {
			serverError(rw, r, err)
		}
		return err != nil
	}
	news, err := w.q.ListPublishedNews(ctx, gen.ListPublishedNewsParams{Now: w.nowStored(), Max: 100000})
	if fail(err) {
		return
	}
	for _, n := range news {
		add("/news/"+n.Slug, n.PublishedAt.String)
	}
	events, err := w.q.ListEventsAdmin(ctx)
	if fail(err) {
		return
	}
	seen := map[string]bool{}
	for _, e := range events { // latest season first: the one /event/{slug} shows
		if !seen[e.Slug] {
			seen[e.Slug] = true
			add("/event/"+e.Slug, e.EndDate)
		}
	}
	pages, err := w.q.ListPages(ctx)
	if fail(err) {
		return
	}
	for _, p := range pages {
		add("/"+p.Slug, "")
	}
	comps, err := w.q.ListSitemapCompetitions(ctx)
	if fail(err) {
		return
	}
	for _, c := range comps {
		add("/competitions/"+c.Code, c.EndDate)
	}
	seasons, err := w.q.ListSitemapSeasons(ctx)
	if fail(err) {
		return
	}
	for _, s := range seasons {
		add("/seasons/"+s, "")
	}
	pilots, err := w.q.ListSitemapPilots(ctx)
	if fail(err) {
		return
	}
	for _, p := range pilots {
		add(pilotHref(p.Civlid, p.Name), "")
	}
	arch := archive.Paths()
	slices.Sort(arch)
	for _, p := range arch {
		add("/result/"+p, "")
	}
	b, err := xml.MarshalIndent(set, "", "  ")
	if fail(err) {
		return
	}
	renderText(rw, "application/xml; charset=utf-8", append([]byte(xml.Header), b...))
}

// robots keeps crawlers out of the back-office and points to the sitemap.
func (w *www) robots(rw http.ResponseWriter, _ *http.Request) {
	body := "User-agent: *\nAllow: /\nDisallow: /admin/\n\nSitemap: " + strings.TrimRight(w.cfg.BaseURL, "/") + "/sitemap.xml\n"
	renderText(rw, "text/plain; charset=utf-8", []byte(body))
}
