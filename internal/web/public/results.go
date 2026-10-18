package public

import (
	"bytes"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"awt/internal/config"
	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/store/gen"
	"awt/internal/web"
	"awt/internal/web/layout"

	"github.com/a-h/templ"
)

// Cache lifetimes of the results pages: competition and season pages change
// while a competition is live, reference pages rarely.
const (
	liveMaxAge = 30 * time.Second
	liveSWR    = 5 * time.Minute
	refMaxAge  = 10 * time.Minute
	refSWR     = time.Hour
)

// results serves the public results pages (spec 06 §4). Every page reads
// stored rows and ranking_entries only; nothing is computed on read.
type results struct {
	q   *gen.Queries
	cfg config.Config
	now func() time.Time
}

func registerResults(r *web.Router) {
	p := &results{q: gen.New(r.Deps().DB.Read), cfg: r.Deps().Cfg, now: time.Now}
	live := func(pattern string, h http.HandlerFunc) { r.PublicCached(pattern, h, liveMaxAge, liveSWR) }
	ref := func(pattern string, h http.HandlerFunc) { r.PublicCached(pattern, h, refMaxAge, refSWR) }

	live("GET /results", p.home)
	live("GET /seasons", p.seasons)
	live("GET /seasons/{code}", p.season)
	live("GET /competitions", p.competitions)
	live("GET /competitions/{code}", p.competitionOverall)
	live("GET /competitions/{code}/runs/{n}", p.competitionRun)
	live("GET /competitions/{code}/details", p.competitionDetails)
	ref("GET /pilots", p.pilots)
	ref("GET /pilots/{civlid}", p.pilot)
	ref("GET /pilots/{civlid}/{slug}", p.pilot)
	ref("GET /teams", p.teams)
	ref("GET /teams/{id}", p.team)
	ref("GET /teams/{id}/{slug}", p.team)
	ref("GET /judges", p.judges)
	ref("GET /tricks", p.tricks)
}

// meta builds the page head: title, description, canonical/og:url from the
// configured base URL, og:image made absolute.
func (p *results) meta(title, desc, path, image string) layout.Meta {
	return layout.Meta{Title: title, Description: desc, Canonical: p.abs(path), OGImage: p.abs(image), Heading: title}
}

func (p *results) abs(u string) string {
	if strings.HasPrefix(u, "/") {
		return strings.TrimRight(p.cfg.BaseURL, "/") + u
	}
	return u
}

func renderPage(w http.ResponseWriter, r *http.Request, c templ.Component) {
	var buf bytes.Buffer
	if err := c.Render(r.Context(), &buf); err != nil {
		serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var buf bytes.Buffer
	_ = layout.ErrorPage(http.StatusNotFound, "Page not found",
		"The page you are looking for does not exist or has moved.").Render(r.Context(), &buf)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(buf.Bytes())
}

func serverError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("public page", "path", r.URL.Path, "err", err)
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// fail maps a load error: a missing row is a 404, anything else a 500.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		notFound(w, r)
		return
	}
	serverError(w, r, err)
}

// participant is a ranked pilot (solo) or team (synchro).
type participant struct {
	CivlID  int64
	TeamID  string
	Name    string
	Country string
	Members []member
}

type member struct {
	CivlID  int64
	Name    string
	Country string
	Rank    int64
	Photo   string
}

func (p participant) Href() string {
	if p.TeamID != "" {
		return teamHref(p.TeamID, p.Name)
	}
	return pilotHref(p.CivlID, p.Name)
}

// key identifies a participant across ranking lists.
func (p participant) key() string {
	if p.TeamID != "" {
		return "t:" + p.TeamID
	}
	return "p:" + strconv.FormatInt(p.CivlID, 10)
}

func newParticipant(civlid sql.NullInt64, teamID sql.NullString, pilotName, country, teamName string, members map[string][]member) participant {
	if teamID.Valid {
		return participant{TeamID: teamID.String, Name: teamName, Members: members[teamID.String]}
	}
	return participant{CivlID: civlid.Int64, Name: pilotName, Country: country}
}

func pilotHref(civlid int64, name string) string {
	return "/pilots/" + strconv.FormatInt(civlid, 10) + "/" + slugOr(name)
}

func teamHref(id, name string) string { return "/teams/" + id + "/" + slugOr(name) }

func slugOr(name string) string {
	if s := domain.Slug(name); s != "" {
		return s
	}
	return "-"
}

// teamMembers loads every team's pilots in one query.
func (p *results) teamMembers(r *http.Request) (map[string][]member, error) {
	rows, err := p.q.PublicTeamMembers(r.Context())
	if err != nil {
		return nil, err
	}
	out := map[string][]member{}
	for _, m := range rows {
		out[m.TeamID] = append(out[m.TeamID], member{CivlID: m.Civlid, Name: m.Name, Country: m.Country, Rank: m.Rank, Photo: m.PhotoUrl})
	}
	return out, nil
}

// Season code patterns of spec 06 §4.5: wac-* lists are hidden, and the
// generic overall is hidden when an AWT/AWQ/AWS season list exists.
var (
	wacList = regexp.MustCompile(`^wac-`)
	awList  = regexp.MustCompile(`^aw[tqs]-`)
)

// visibleLists filters and orders ranking list keys (spec 06 §4.5): the
// competition's season lists in their order, then overall, women, others.
func visibleLists(keys []string, seasonOrder []string) []string {
	hideOverall := slices.ContainsFunc(keys, awList.MatchString)
	rank := func(k string) int {
		if i := slices.Index(seasonOrder, k); i >= 0 {
			return i
		}
		switch k {
		case "overall":
			return 1000
		case "women":
			return 1001
		}
		return 1002
	}
	var out []string
	for _, k := range keys {
		if wacList.MatchString(k) || (k == "overall" && hideOverall) || slices.Contains(out, k) {
			continue
		}
		out = append(out, k)
	}
	slices.SortStableFunc(out, func(a, b string) int {
		if d := rank(a) - rank(b); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	return out
}

func listLabel(key string, seasonNames map[string]string) string {
	switch key {
	case "overall":
		return "Overall"
	case "women":
		return "Women"
	}
	if n := seasonNames[key]; n != "" {
		return n
	}
	return key
}

// tab is a link of a tab bar; Current marks the selected one.
type tab struct {
	Label, Href string
	Current     bool
}

// listTabs builds the list selector and returns the selected key (the
// requested one when visible, else the first).
func listTabs(keys []string, requested, base string, names map[string]string) ([]tab, string) {
	if len(keys) == 0 {
		return nil, ""
	}
	sel := keys[0]
	if slices.Contains(keys, requested) {
		sel = requested
	}
	tabs := make([]tab, len(keys))
	for i, k := range keys {
		href := base
		if i > 0 {
			href += "?list=" + url.QueryEscape(k)
		}
		tabs[i] = tab{Label: listLabel(k, names), Href: href, Current: k == sel}
	}
	return tabs, sel
}

// score formats a stored (already Round3'd) score with 3 decimals.
func score(f float64) string { return strconv.FormatFloat(f, 'f', 3, 64) }

// pct formats a percentage without trailing zeros.
func pct(f float64) string { return strconv.FormatFloat(scoring.Round3(f), 'f', -1, 64) + "%" }

func medal(rank int64) string {
	switch rank {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	}
	return ""
}

func ordinal(n int64) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.FormatInt(n, 10) + suffix
}

func countLabel(n int64, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.FormatInt(n, 10) + " " + word + "s"
}

func country(c string) string { return strings.ToUpper(c) }

// dateRange formats stored YYYY-MM-DD dates in one locale (spec 06 B11):
// "15–18 August 2024", "30 August – 2 September 2024".
func dateRange(start, end string) string {
	s, err1 := time.Parse(time.DateOnly, firstN(start, 10))
	e, err2 := time.Parse(time.DateOnly, firstN(end, 10))
	switch {
	case err1 != nil || err2 != nil:
		return start + " – " + end
	case s.Equal(e):
		return s.Format("2 January 2006")
	case s.Year() == e.Year() && s.Month() == e.Month():
		return s.Format("2") + "–" + e.Format("2 January 2006")
	case s.Year() == e.Year():
		return s.Format("2 January") + " – " + e.Format("2 January 2006")
	}
	return s.Format("2 January 2006") + " – " + e.Format("2 January 2006")
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func year(date string) int {
	y, _ := strconv.Atoi(firstN(date, 4))
	return y
}

func fileURL(id sql.NullString) string {
	if !id.Valid || id.String == "" {
		return ""
	}
	return "/files/" + id.String
}

// yearTabs builds a year selector: the given years plus the current one,
// descending.
func yearTabs(years []int, selected int, base string) []tab {
	years = slices.DeleteFunc(years, func(y int) bool { return y <= 0 })
	slices.Sort(years)
	years = slices.Compact(years)
	slices.Reverse(years)
	out := make([]tab, len(years))
	for i, y := range years {
		out[i] = tab{Label: strconv.Itoa(y), Href: base + "?year=" + strconv.Itoa(y), Current: y == selected}
	}
	return out
}

// pickYear returns the requested ?year= when valid, else the current year
// when it has data, else the latest year with data (spec 06 §10.3: an empty
// page in January).
func pickYear(r *http.Request, years []int, now int) int {
	if y, err := strconv.Atoi(r.URL.Query().Get("year")); err == nil && y > 0 {
		return y
	}
	if len(years) == 0 || slices.Contains(years, now) {
		return now
	}
	best := 0
	for _, y := range years {
		if y <= now && y > best {
			best = y
		}
	}
	if best == 0 {
		best = slices.Max(years)
	}
	return best
}

// markPtr formats an optional judge mark with 3 decimals, "–" when absent.
func markPtr(f *float64) string {
	if f == nil {
		return "–"
	}
	return score(*f)
}

// markNull formats an optional stored judge mark.
func markNull(f sql.NullFloat64) string {
	if !f.Valid {
		return "–"
	}
	return score(f.Float64)
}
