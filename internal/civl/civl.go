// Package civl is a client for civlcomps.org: pilot identity pages and the
// paragliding aerobatics solo world ranking export. It ports the legacy
// logic of legacy/api/controllers/pilots.py (update_pilot, update_rankings).
package civl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/net/html"
)

// DefaultBaseURL is the production CIVL site.
const DefaultBaseURL = "https://civlcomps.org"

const userAgent = "awt (+https://www.acroworldtour.com)"

// UnrankedRank is the rank legacy assigned to a ranking row without a rank.
const UnrankedRank = 9999

// Link is a named external link.
type Link struct {
	Name string
	Link string
}

// Sponsor is a pilot sponsor.
type Sponsor struct {
	Name string
	Link string
	Img  string
}

// PilotData is the identity data scraped from a CIVL pilot page.
type PilotData struct {
	CivlID            int
	Name              string
	Country           string // lower-case ISO alpha-3 from the flag-xxx css class
	CivlLink          string
	About             string
	Photo             string
	PhotoHighres      string
	BackgroundPicture string
	SocialLinks       []Link
	Sponsors          []Sponsor
}

// Client talks to civlcomps.org. Timeouts come from the supplied http.Client.
type Client struct {
	http    *http.Client
	baseURL string
}

// New returns a Client. A nil httpClient means http.DefaultClient; an empty
// baseURL means DefaultBaseURL.
func New(httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{http: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

// ErrNotFound: CIVL has no pilot page for the CIVL id (HTTP 404).
var ErrNotFound = errors.New("civl: pilot not found")

// StatusError is returned when CIVL answers with a non-200 status.
type StatusError struct {
	Method string
	URL    string
	Status int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("civl: %s %s: unexpected status %d", e.Method, e.URL, e.Status)
}

func (c *Client) do(ctx context.Context, method, u string, body io.Reader, hdr map[string]string, cookies []*http.Cookie) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, nil, fmt.Errorf("civl: %s %s: %w", method, u, err)
	}
	req.Header.Set("User-Agent", userAgent)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("civl: %s %s: %w", method, u, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("civl: %s %s: reading body: %w", method, u, err)
	}
	if resp.StatusCode != http.StatusOK {
		return resp, b, &StatusError{Method: method, URL: u, Status: resp.StatusCode}
	}
	return resp, b, nil
}

// FetchPilot scrapes the CIVL pilot page of civlid.
func (c *Client) FetchPilot(ctx context.Context, civlid int) (PilotData, error) {
	pageURL := fmt.Sprintf("%s/pilot/%d", c.baseURL, civlid)
	_, body, err := c.do(ctx, http.MethodGet, pageURL, nil, nil, nil)
	var se *StatusError
	if errors.As(err, &se) && se.Status == http.StatusNotFound {
		return PilotData{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	if err != nil {
		return PilotData{}, err
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return PilotData{}, fmt.Errorf("civl: parsing %s: %w", pageURL, err)
	}

	p := PilotData{CivlID: civlid, CivlLink: pageURL}

	h1 := first(doc, step{tag: "h1", class: "title-pilot"})
	if h1 == nil {
		return PilotData{}, fmt.Errorf("civl: %s: pilot name (h1.title-pilot) not found", pageURL)
	}
	p.Name = strings.TrimSpace(text(h1))

	flag := first(doc, step{tag: "div", class: "country-place"}, step{tag: "i"})
	if flag == nil {
		return PilotData{}, fmt.Errorf("civl: %s: country flag (div.country-place i) not found", pageURL)
	}
	for _, cl := range strings.Fields(attr(flag, "class")) {
		if cl != "flag" && strings.HasPrefix(cl, "flag-") {
			p.Country = strings.TrimPrefix(cl, "flag-")
		}
	}
	if p.Country == "" {
		return PilotData{}, fmt.Errorf("civl: %s: no flag-xxx class on country icon", pageURL)
	}

	for _, e := range all(doc, step{tag: "article", id: "info"}, step{tag: "p"}) {
		p.About += strings.ReplaceAll(text(e), "\n", " ")
	}

	for _, e := range all(doc, step{tag: "a", class: "social-link"}) {
		var rest []string
		for _, cl := range strings.Fields(attr(e, "class")) {
			if cl != "social-link" {
				rest = append(rest, cl)
			}
		}
		p.SocialLinks = append(p.SocialLinks, Link{Name: strings.Join(rest, " "), Link: attr(e, "href")})
	}
	for _, e := range all(doc, step{tag: "article", class: "links-block"}, step{tag: "a"}) {
		p.SocialLinks = append(p.SocialLinks, Link{Name: strings.ReplaceAll(text(e), "\n", " "), Link: attr(e, "href")})
	}

	for _, e := range all(doc, step{tag: "aside", class: "sponsors-wrapper"}, step{tag: "a"}) {
		img := first(e, step{tag: "img"})
		if img == nil {
			continue
		}
		name := attr(e, "alt")
		if name == "" {
			name = attr(img, "alt")
		}
		p.Sponsors = append(p.Sponsors, Sponsor{Name: name, Link: attr(e, "href"), Img: attr(img, "src")})
	}

	// Legacy raised when photo / background were missing; we keep the pilot
	// importable and leave the fields empty instead.
	if img := first(doc, step{class: "photo-pilot"}, step{tag: "img"}); img != nil {
		p.Photo = attr(img, "src")
	}
	p.PhotoHighres = p.Photo
	if p.Photo != "" {
		hi := strings.Replace(p.Photo, "/uploads/resize/profile/header/", "/uploads/images/profile/", 1)
		if hi != p.Photo && c.headOK(ctx, hi) {
			p.PhotoHighres = hi
		}
	}
	if img := first(doc, step{class: "image-fon"}, step{tag: "img"}); img != nil {
		p.BackgroundPicture = attr(img, "src")
	}
	return p, nil
}

// headOK reports whether a HEAD on u answers 200 (legacy: any failure is ignored).
func (c *Client) headOK(ctx context.Context, u string) bool {
	if ctx.Err() != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

var exportURLRe = regexp.MustCompile(`'(https?://[^']*/ranking/export-new[^']+)&async=1'`)

type rankingSession struct {
	exportURL string
	csrf      string
	cookies   []*http.Cookie
}

// rankingSession loads the ranking page and extracts the export URL, the CSRF
// token and the session cookies (legacy get_ranking_url_solo).
func (c *Client) rankingSession(ctx context.Context) (rankingSession, error) {
	pageURL := c.baseURL + "/ranking/paragliding-aerobatics/pilots"
	resp, body, err := c.do(ctx, http.MethodGet, pageURL, nil, nil, nil)
	if err != nil {
		return rankingSession{}, err
	}
	m := exportURLRe.FindSubmatch(body)
	if m == nil {
		return rankingSession{}, fmt.Errorf("civl: %s: unable to extract ranking export URL", pageURL)
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return rankingSession{}, fmt.Errorf("civl: parsing %s: %w", pageURL, err)
	}
	var token string
	walk(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "meta" && attr(n, "name") == "csrf-token" {
			token = attr(n, "content")
			return false
		}
		return true
	})
	if token == "" {
		return rankingSession{}, fmt.Errorf("civl: %s: unable to retrieve CSRF token", pageURL)
	}
	return rankingSession{exportURL: string(m[1]), csrf: token, cookies: resp.Cookies()}, nil
}

// FetchRankings returns civlid -> rank for the current month of the
// paragliding aerobatics solo ranking.
func (c *Client) FetchRankings(ctx context.Context) (map[int]int, error) {
	return c.fetchRankings(ctx, time.Now())
}

func (c *Client) fetchRankings(ctx context.Context, now time.Time) (map[int]int, error) {
	s, err := c.rankingSession(ctx)
	if err != nil {
		return nil, err
	}
	// Legacy formatted the month with strftime('%M') (minutes!), a bug; the
	// real zero-padded month is used here.
	month := fmt.Sprintf("%02d", int(now.Month()))
	year := strconv.Itoa(now.Year())
	form := url.Values{}
	form.Set("search[nation_id]", "")
	form.Set("search[profile_id]", "")
	form.Add("search[continent_id][]", "")
	form.Add("search[continent_id][]", "0")
	form.Add("search[scoringCategory][]", "")
	form.Add("search[scoringCategory][]", "0")
	form.Set("search[rankingDate]", year+"-"+month+"-01")
	form.Set("date_[day]", "01")
	form.Set("date_[month]", month)
	form.Set("date_[year]", year)

	hdr := map[string]string{
		"X-CSRF-Token": s.csrf,
		"Content-Type": "application/x-www-form-urlencoded",
	}
	_, body, err := c.do(ctx, http.MethodPost, s.exportURL, strings.NewReader(form.Encode()), hdr, s.cookies)
	if err != nil {
		return nil, err
	}
	return parseRankingXLSX(body)
}

// parseRankingXLSX reads the first sheet: column B holds the CIVL id, column A
// the rank (blank rank => 9999). Non-numeric and zero ids are skipped.
func parseRankingXLSX(data []byte) (map[int]int, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("civl: opening ranking xlsx: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("civl: ranking xlsx has no sheet")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("civl: reading ranking xlsx: %w", err)
	}
	out := map[int]int{}
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		id, ok := toInt(row[1])
		if !ok || id == 0 {
			continue
		}
		rank, ok := toInt(row[0])
		if !ok || rank == 0 {
			rank = UnrankedRank
		}
		out[id] = rank
	}
	return out, nil
}

func toInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f == float64(int(f)) {
		return int(f), true
	}
	return 0, false
}
