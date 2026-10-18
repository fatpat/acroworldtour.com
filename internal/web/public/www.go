package public

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"awt/internal/config"
	"awt/internal/service"
	"awt/internal/store/gen"
	"awt/internal/web"
	"awt/internal/web/layout"
	"awt/web/archive"

	"github.com/a-h/templ"
)

// Cache lifetimes of the institutional pages: editors publish rarely and the
// admin purges the CDN, so a few minutes is safe.
const (
	wwwMaxAge = 2 * time.Minute
	wwwSWR    = 10 * time.Minute
	// archiveMaxAge: the 2022-2023 result pages are frozen.
	archiveMaxAge = 24 * time.Hour
)

// www serves the institutional site (spec 07): home, events, news,
// documents, generic pages, sitemap and the archived result pages.
type www struct {
	q    *gen.Queries
	cfg  config.Config
	deps web.Deps
	now  func() time.Time
}

func registerWWW(r *web.Router) {
	w := &www{q: gen.New(r.Deps().DB.Read), cfg: r.Deps().Cfg, deps: r.Deps(), now: time.Now}
	page := func(pattern string, h http.HandlerFunc) { r.PublicCached(pattern, h, wwwMaxAge, wwwSWR) }

	page("GET /{$}", w.home)
	page("GET /events", w.events)
	page("GET /event/{slug}", w.event)
	page("GET /news", w.newsList)
	page("GET /news/{slug}", w.newsArticle)
	page("GET /documents", w.documents)
	page("GET /sitemap.xml", w.sitemap)
	page("GET /robots.txt", w.robots)
	r.PublicCached("GET /result/{year}/{slug}", http.HandlerFunc(w.archived), archiveMaxAge, archiveMaxAge)
	r.Public("GET /assets/", archive.Asset(w.notFound))
	// Last and narrowest: a single path segment that is a stored page slug.
	// Literal routes (/events, /results, ...) always win over this wildcard.
	page("GET /{slug}", w.page)
}

// meta builds the page head with absolute canonical and og:image URLs.
func (w *www) meta(title, desc, path, image string) layout.Meta {
	return layout.Meta{Title: title, Description: desc, Canonical: w.abs(path), OGImage: w.abs(image), Heading: title}
}

func (w *www) abs(u string) string {
	if strings.HasPrefix(u, "/") {
		return strings.TrimRight(w.cfg.BaseURL, "/") + u
	}
	return u
}

// settings reads the site settings; a read failure falls back to defaults so
// the page still renders.
func (w *www) settings(ctx context.Context) service.SiteSettings {
	rows, err := w.q.ListSiteSettings(ctx)
	if err != nil {
		slog.Error("site settings", "err", err)
		return service.SiteSettings{}
	}
	return service.ParseSettings(rows)
}

const defaultDescription = "The world's best pilots fighting for the most prestigious title: Acro World Tour, paragliding aerobatics."

func (w *www) describe(st service.SiteSettings, desc string) string {
	if desc != "" {
		return desc
	}
	if st.DefaultDescription != "" {
		return st.DefaultDescription
	}
	return defaultDescription
}

func (w *www) today() string { return w.now().UTC().Format(time.DateOnly) }

// nowStored is "now" as the news query compares it: RFC 3339 UTC text.
func (w *www) nowStored() sql.NullString {
	return sql.NullString{String: w.now().UTC().Format(time.RFC3339), Valid: true}
}

// notFound answers 404, or the stored redirect of the path first.
func (w *www) notFound(rw http.ResponseWriter, r *http.Request) { w.deps.NotFound(rw, r) }

func (w *www) fail(rw http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		w.notFound(rw, r)
		return
	}
	serverError(rw, r, err)
}

// jsonLD renders a schema.org object in a <script type="application/ld+json">
// block. It marshals Go values with encoding/json, which escapes <, > and &,
// so no value can close the script element.
func jsonLD(v any) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, out io.Writer) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = out.Write(append(append([]byte(`<script type="application/ld+json">`), b...), `</script>`...))
		return err
	})
}

// ldOrganization is the publisher of every article.
type ldOrganization struct {
	Type string `json:"@type"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type ldNewsArticle struct {
	Context       string         `json:"@context"`
	Type          string         `json:"@type"`
	Headline      string         `json:"headline"`
	Description   string         `json:"description,omitempty"`
	DatePublished string         `json:"datePublished,omitempty"`
	Author        *ldPerson      `json:"author,omitempty"`
	Image         []string       `json:"image,omitempty"`
	MainEntity    string         `json:"mainEntityOfPage"`
	Publisher     ldOrganization `json:"publisher"`
}

type ldPerson struct {
	Type string `json:"@type"`
	Name string `json:"name"`
}

type ldPlace struct {
	Type    string     `json:"@type"`
	Name    string     `json:"name"`
	Address *ldAddress `json:"address,omitempty"`
}

type ldAddress struct {
	Type    string `json:"@type"`
	Country string `json:"addressCountry"`
}

type ldSportsEvent struct {
	Context     string          `json:"@context"`
	Type        string          `json:"@type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	StartDate   string          `json:"startDate"`
	EndDate     string          `json:"endDate"`
	Status      string          `json:"eventStatus"`
	Location    *ldPlace        `json:"location,omitempty"`
	Image       []string        `json:"image,omitempty"`
	URL         string          `json:"url"`
	Organizer   *ldOrganization `json:"organizer,omitempty"`
}

func (w *www) publisher() ldOrganization {
	return ldOrganization{Type: "Organization", Name: "Acro World Tour", URL: w.abs("/")}
}

// youtubeID extracts the video id of a YouTube watch, short or embed URL.
var ytID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func youtubeEmbed(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(u.Host, "www.")
	var id string
	switch host {
	case "youtube.com", "m.youtube.com", "youtube-nocookie.com":
		id = u.Query().Get("v")
		if id == "" {
			if rest, ok := strings.CutPrefix(u.Path, "/embed/"); ok {
				id = rest
			}
		}
	case "youtu.be":
		id = strings.TrimPrefix(u.Path, "/")
	}
	if !ytID.MatchString(id) {
		return ""
	}
	return "https://www.youtube-nocookie.com/embed/" + id
}

// renderPlain writes a non-HTML body.
func renderText(rw http.ResponseWriter, contentType string, body []byte) {
	rw.Header().Set("Content-Type", contentType)
	_, _ = rw.Write(body)
}

// archived serves a frozen 2022-2023 result page as it was.
func (w *www) archived(rw http.ResponseWriter, r *http.Request) {
	b, ok := archive.Page(r.PathValue("year") + "/" + r.PathValue("slug"))
	if !ok {
		w.notFound(rw, r)
		return
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = bytes.NewReader(b).WriteTo(rw)
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func joinWords(s []string) string { return strings.Join(s, " + ") }

func relFor(external bool) string {
	if external {
		return "noopener"
	}
	return ""
}

func heroTitle(t string) string {
	if t == "" {
		return "Acro World Tour"
	}
	return t
}

func itoa(n int) string { return strconv.Itoa(n) }
