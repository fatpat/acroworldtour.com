package importer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"awt/internal/blob"
	"awt/internal/markdown"
	"awt/internal/store"
	"awt/internal/store/gen"

	"golang.org/x/net/html"
)

// maxWWWFile is the largest legacy asset imported (the upload limit).
const maxWWWFile = 10 << 20

// WWW imports the legacy static site (legacy/legacy/www): news and event
// pages become news and events rows (body converted to Markdown), their
// images and the sporting code PDFs are stored as files, and a redirect is
// seeded for every legacy path (spec 07 §8, §10). It is idempotent: rows are
// upserted by slug (events by season and slug), so a second run changes
// nothing. It overwrites admin edits to the same slugs, so run it once at
// migration time, but it never restores news or events deleted in the admin
// (they are skipped with a warning). Undated articles get the date of the
// commit that added them to the legacy git repository (recorded in
// legacy_news_dates.tsv, else read from git when available), or stay drafts.
func WWW(ctx context.Context, db *store.DB, blobs blob.Store, dir string) (Report, error) {
	rep := Report{Counts: map[string]int{}}
	for _, sub := range []string{"news", "event"} {
		if st, err := os.Stat(filepath.Join(dir, sub)); err != nil || !st.IsDir() {
			return rep, fmt.Errorf("%s is not a legacy www directory (no %s/)", dir, sub)
		}
	}
	gitDates, err := parseLegacyNewsDates(legacyNewsDatesTSV)
	if err != nil {
		return rep, err
	}
	w := &wwwImport{dir: dir, blobs: blobs, rep: &rep, now: time.Now().UTC(), dates: teaserDates(dir), gitDates: gitDates}
	err = db.Tx(ctx, func(q *gen.Queries) error {
		w.q = q
		for _, step := range []func(context.Context) error{w.news, w.events, w.documents, w.redirects} {
			if err := step(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	return rep, err
}

type wwwImport struct {
	dir   string
	q     *gen.Queries
	blobs blob.Store
	rep   *Report
	now   time.Time
	dates map[string]string // news slug -> RFC 3339 date from the home teasers
	// news slug -> date of the commit that added it (legacy_news_dates.tsv)
	gitDates map[string]string
	files    map[string]string // legacy asset path -> stored file id ("" when skipped)
	err      error             // first database failure met while converting; checked by every step
}

func newHexID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ---- html helpers ----

func parseHTMLFile(p string) (*html.Node, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return html.Parse(bytes.NewReader(b))
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	return n.Type == html.ElementNode && slices.Contains(strings.Fields(attr(n, "class")), class)
}

// find returns the first descendant (depth first) matching pred.
func find(n *html.Node, pred func(*html.Node) bool) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if pred(c) {
			return c
		}
		if f := find(c, pred); f != nil {
			return f
		}
	}
	return nil
}

func findAll(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if pred(c) {
			out = append(out, c)
		}
		out = append(out, findAll(c, pred)...)
	}
	return out
}

func byClass(class string) func(*html.Node) bool {
	return func(n *html.Node) bool { return hasClass(n, class) }
}

func byTag(tag string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == tag }
}

// text returns the whitespace-collapsed text of n (comments are ignored by
// the parser's node types).
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// ---- dates ----

var teaserDateLayouts = []string{"2 January 2006", "02 January 2006", "2 Jan 2006"}

func parseTeaserDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range teaserDateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// teaserDates reads the publication dates the home page shows next to its
// news links: the articles carry no date themselves (spec 07 §2.3).
func teaserDates(dir string) map[string]string {
	out := map[string]string{}
	doc, err := parseHTMLFile(filepath.Join(dir, "index.html"))
	if err != nil {
		return out
	}
	for _, a := range findAll(doc, byTag("a")) {
		slug, ok := strings.CutPrefix(attr(a, "href"), "/news/")
		if !ok {
			continue
		}
		span := find(a, byClass("news__date"))
		if span == nil {
			continue
		}
		if t, ok := parseTeaserDate(text(span)); ok {
			out[strings.TrimSuffix(slug, ".html")] = t.Format(time.RFC3339)
		}
	}
	return out
}

// articleDate returns the date of a page when it states one: <time datetime>
// or article:published_time.
func articleDate(doc *html.Node) string {
	if t := find(doc, byTag("time")); t != nil {
		if d, err := time.Parse(time.RFC3339, attr(t, "datetime")); err == nil {
			return d.UTC().Format(time.RFC3339)
		}
		if d, err := time.Parse(time.DateOnly, attr(t, "datetime")); err == nil {
			return d.Format(time.RFC3339)
		}
	}
	if m := find(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "meta" && attr(n, "property") == "article:published_time"
	}); m != nil {
		if d, err := time.Parse(time.RFC3339, attr(m, "content")); err == nil {
			return d.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

// ---- files ----

// upload stores a legacy asset (path like "/assets/img/news/x.jpg" or
// "../assets/...") and returns its file id. Only files under assets/ are read
// and only images and PDFs are accepted; anything else is skipped with a
// warning.
func (w *wwwImport) upload(ctx context.Context, src string) (string, bool) {
	rel := path.Clean("/" + strings.TrimLeft(strings.ReplaceAll(src, "../", ""), "/"))
	rel = strings.TrimPrefix(rel, "/")
	if !strings.HasPrefix(rel, "assets/") {
		w.rep.warnf("file %q: not under assets/, skipped", src)
		return "", false
	}
	if id, ok := w.files[rel]; ok {
		return id, id != ""
	}
	if w.files == nil {
		w.files = map[string]string{}
	}
	w.files[rel] = ""
	data, err := os.ReadFile(filepath.Join(w.dir, filepath.FromSlash(rel)))
	if err != nil {
		w.rep.warnf("file %q: %v, skipped", src, err)
		return "", false
	}
	ct := http.DetectContentType(data)
	if len(data) > maxWWWFile || !slices.Contains([]string{"image/jpeg", "image/png", "image/webp", "application/pdf"}, ct) {
		w.rep.warnf("file %q: unsupported type %s or too large (%d bytes), skipped", src, ct, len(data))
		return "", false
	}
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])
	if err := w.blobs.Put(ctx, id, ct, bytes.NewReader(data)); err != nil {
		w.rep.warnf("file %q: store: %v, skipped", src, err)
		return "", false
	}
	err = w.q.UpsertFile(ctx, gen.UpsertFileParams{ID: id, Filename: path.Base(rel), ContentType: ct, Size: int64(len(data)),
		CreatedAt: w.now.Format(time.RFC3339)})
	if err != nil {
		w.err = fmt.Errorf("import file %q: %w", src, err)
		return "", false
	}
	w.files[rel] = id
	w.rep.Counts["files"]++
	return id, true
}

// ---- html to markdown ----

// mdConv converts the content of a legacy page to Markdown. Images are
// uploaded as it meets them; the first one is returned as the cover instead of
// being repeated in the body when coverFirst is set.
type mdConv struct {
	ctx        context.Context
	w          *wwwImport
	coverFirst bool
	cover      string
	blocks     []string
}

var mdEscaper = strings.NewReplacer(`\`, `\\`, "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`, "<", `\<`, "`", "\\`")

func escapeMD(s string) string { return mdEscaper.Replace(s) }

// escapeLead stops a paragraph starting like a heading, list or quote.
var orderedLead = regexp.MustCompile(`^\d+[.)]`)

func escapeLead(s string) string {
	if s == "" {
		return s
	}
	if strings.ContainsRune("#>+-=|", rune(s[0])) {
		return `\` + s
	}
	if loc := orderedLead.FindString(s); loc != "" {
		return s[:len(loc)-1] + `\` + s[len(loc)-1:]
	}
	return s
}

func safeHref(h string) string {
	h = strings.TrimSpace(h)
	switch {
	case strings.HasPrefix(h, "https://"), strings.HasPrefix(h, "http://"), strings.HasPrefix(h, "mailto:"):
		return h
	case strings.HasPrefix(h, "/") && !strings.HasPrefix(h, "//"):
		return strings.TrimSuffix(h, ".html")
	}
	return ""
}

func (c *mdConv) inline(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			switch {
			case ch.Type == html.TextNode:
				b.WriteString(escapeMD(collapseSpaces(ch.Data)))
			case ch.Type != html.ElementNode:
			case ch.Data == "br":
				b.WriteString("  \n")
			case ch.Data == "strong" || ch.Data == "b":
				b.WriteString(emphasis(c.inline(ch), "**"))
			case ch.Data == "em" || ch.Data == "i":
				b.WriteString(emphasis(c.inline(ch), "*"))
			case ch.Data == "a":
				label := c.inline(ch)
				if href := safeHref(attr(ch, "href")); href != "" && strings.TrimSpace(label) != "" {
					b.WriteString("[" + strings.TrimSpace(label) + "](" + href + ")")
				} else {
					b.WriteString(label)
				}
			case ch.Data == "img":
				b.WriteString(c.image(ch))
			default:
				walk(ch)
			}
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

func collapseSpaces(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			space = true
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}

// emphasis wraps s keeping its surrounding blanks outside the markers, which
// CommonMark requires.
func emphasis(s, mark string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return s
	}
	lead := s[:len(s)-len(strings.TrimLeft(s, " "))]
	trail := s[len(strings.TrimRight(s, " ")):]
	return lead + mark + t + mark + trail
}

// image uploads an <img> and returns its Markdown, or "" when skipped (the
// first image becomes the cover when coverFirst is set).
func (c *mdConv) image(n *html.Node) string {
	id, ok := c.w.upload(c.ctx, attr(n, "src"))
	if !ok {
		return ""
	}
	if c.coverFirst && c.cover == "" {
		c.cover = id
		return ""
	}
	alt := strings.ReplaceAll(strings.TrimSpace(attr(n, "alt")), "\n", " ")
	return "![" + escapeMD(alt) + "](/files/" + id + ")"
}

func (c *mdConv) push(s string) {
	if s = strings.TrimSpace(s); s != "" {
		c.blocks = append(c.blocks, s)
	}
}

func (c *mdConv) walk(n *html.Node) {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		switch {
		case ch.Type == html.TextNode:
			if t := strings.TrimSpace(collapseSpaces(ch.Data)); t != "" {
				c.push(escapeLead(escapeMD(t)))
			}
		case ch.Type != html.ElementNode:
		case hasClass(ch, "results__card"):
			c.ranking(ch)
		case ch.Data == "p":
			c.push(escapeLead(c.inline(ch)))
		case len(ch.Data) == 2 && ch.Data[0] == 'h' && ch.Data[1] >= '1' && ch.Data[1] <= '6':
			c.push(strings.Repeat("#", int(ch.Data[1]-'0')) + " " + c.inline(ch))
		case ch.Data == "ul" || ch.Data == "ol":
			c.list(ch)
		case ch.Data == "img":
			c.push(c.image(ch))
		case ch.Data == "script" || ch.Data == "style":
		default:
			c.walk(ch)
		}
	}
}

func (c *mdConv) list(n *html.Node) {
	var items []string
	i := 0
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		i++
		marker := "- "
		if n.Data == "ol" {
			marker = strconv.Itoa(i) + ". "
		}
		if t := c.inline(li); t != "" {
			items = append(items, marker+t)
		}
	}
	c.push(strings.Join(items, "\n"))
}

// ranking turns a legacy hand-typed result card into a Markdown table.
func (c *mdConv) ranking(card *html.Node) {
	if h := find(card, byClass("results__title")); h != nil {
		c.push("### " + c.inline(h))
	}
	if s := find(card, byClass("results__subtitle")); s != nil {
		c.push(emphasis(c.inline(s), "*"))
	}
	var rows []string
	for _, li := range findAll(card, byClass("ranking-list__result")) {
		cell := func(class string) string {
			if n := find(li, byClass(class)); n != nil {
				return strings.ReplaceAll(c.inline(n), "|", `\|`)
			}
			return ""
		}
		rows = append(rows, "| "+cell("ranking-list__pos")+" | "+cell("ranking-list__pilot")+" | "+cell("ranking-list__pts")+" |")
	}
	if len(rows) > 0 {
		c.push("| Pos. | Pilot | Pts. |\n|---|---|---|\n" + strings.Join(rows, "\n"))
	}
}

func (c *mdConv) markdown() string { return strings.Join(c.blocks, "\n\n") }

// ---- news ----

func titleOf(doc *html.Node) string {
	if h := find(doc, byClass("header__title")); h != nil {
		return text(h)
	}
	if t := find(doc, byTag("title")); t != nil {
		return strings.TrimSpace(strings.TrimSuffix(text(t), "| Acro World Tour"))
	}
	return ""
}

func slugOfFile(name string) string { return strings.TrimSuffix(name, ".html") }

func htmlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".html") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func (w *wwwImport) news(ctx context.Context) error {
	files, err := htmlFiles(filepath.Join(w.dir, "news"))
	if err != nil {
		return fmt.Errorf("list news: %w", err)
	}
	for _, name := range files {
		slug := slugOfFile(name)
		doc, err := parseHTMLFile(filepath.Join(w.dir, "news", name))
		if err != nil {
			w.rep.warnf("news %s: %v, skipped", name, err)
			continue
		}
		title := titleOf(doc)
		body := find(doc, byClass("default-page__content"))
		if title == "" || body == nil {
			w.rep.warnf("news %s: no title or content, skipped", name)
			continue
		}
		conv := &mdConv{ctx: ctx, w: w, coverFirst: true}
		conv.walk(body)
		if w.err != nil {
			return w.err
		}
		md := conv.markdown()
		published := articleDate(doc)
		if published == "" {
			published = w.dates[slug]
		}
		if published == "" {
			published = w.gitDates[slug]
		}
		if published == "" {
			published = firstCommitDate(ctx, filepath.Join(w.dir, "news", name))
		}
		if published == "" {
			w.rep.warnf("news %s: no publication date found (page, teasers, git history), imported as a draft (set the date in the admin to publish it)", slug)
		}
		id := ""
		old, err := w.q.GetNewsBySlugAny(ctx, slug)
		switch {
		case err == nil && old.DeletedAt.Valid:
			w.rep.warnf("news %s: deleted in the admin, not restored", slug)
			continue
		case err == nil:
			id = old.ID
		case errors.Is(err, sql.ErrNoRows):
			if id, err = newHexID(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("get news %s: %w", slug, err)
		}
		err = w.q.UpsertNews(ctx, gen.UpsertNewsParams{ID: id, Slug: slug, Title: title, Excerpt: markdown.Plain(md, 200), BodyMd: md,
			CoverFile: sql.NullString{String: conv.cover, Valid: conv.cover != ""}, Author: "",
			PublishedAt: sql.NullString{String: published, Valid: published != ""}})
		if err != nil {
			return fmt.Errorf("import news %s: %w", slug, err)
		}
		w.rep.Counts["news"]++
	}
	return nil
}

// ---- events ----

var eventDateRe = regexp.MustCompile(`^([A-Za-z]+)\s+(\d{1,2})(?:st|nd|rd|th)?(?:\s*[-–]\s*(?:([A-Za-z]+)\s+)?(\d{1,2})(?:st|nd|rd|th)?)?\s+(\d{4})$`)

// parseEventDates reads "August 9th - 15th 2024", "Aug 30th - Sep 2nd 2024"
// or "June 8th 2024" into YYYY-MM-DD dates.
func parseEventDates(s string) (start, end string, ok bool) {
	m := eventDateRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", "", false
	}
	month := func(name, day string) (time.Time, bool) {
		for _, layout := range []string{"January 2 2006", "Jan 2 2006"} {
			if t, err := time.Parse(layout, name+" "+day+" "+m[5]); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}
	s0, ok1 := month(m[1], m[2])
	if !ok1 {
		return "", "", false
	}
	e0 := s0
	if m[4] != "" {
		endMonth := m[1]
		if m[3] != "" {
			endMonth = m[3]
		}
		var ok2 bool
		if e0, ok2 = month(endMonth, m[4]); !ok2 {
			return "", "", false
		}
		if e0.Before(s0) { // "December 30th - January 2nd 2025"
			e0 = e0.AddDate(1, 0, 0)
		}
	}
	return s0.Format(time.DateOnly), e0.Format(time.DateOnly), true
}

var flagRe = regexp.MustCompile(`flags/([a-z]{3})\.png`)

func (w *wwwImport) events(ctx context.Context) error {
	files, err := htmlFiles(filepath.Join(w.dir, "event"))
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}
	today := w.now.Format(time.DateOnly)
	for _, name := range files {
		slug := slugOfFile(name)
		doc, err := parseHTMLFile(filepath.Join(w.dir, "event", name))
		if err != nil {
			w.rep.warnf("event %s: %v, skipped", name, err)
			continue
		}
		title := titleOf(doc)
		dateEl := find(doc, byClass("event__date"))
		if title == "" || dateEl == nil {
			w.rep.warnf("event %s: no title or date, skipped", name)
			continue
		}
		start, end, ok := parseEventDates(text(dateEl))
		if !ok {
			w.rep.warnf("event %s: cannot read the date %q, skipped", name, text(dateEl))
			continue
		}
		conv := &mdConv{ctx: ctx, w: w}
		if t := find(doc, byClass("event__text")); t != nil {
			conv.walk(t)
		}
		cover := ""
		if img := find(doc, byClass("event__image")); img != nil {
			cover, _ = w.upload(ctx, attr(img, "src"))
		}
		if w.err != nil {
			return w.err
		}
		site, country := "", ""
		if loc := find(doc, byClass("event__location")); loc != nil {
			site = text(loc)
			if i := find(loc, byTag("i")); i != nil {
				if m := flagRe.FindStringSubmatch(attr(i, "style")); m != nil {
					country = m[1]
				}
			}
			if i := strings.LastIndex(site, ", "); i >= 0 {
				site = site[:i]
			}
		}
		var circuits, disciplines []string
		if cat := find(doc, byClass("event__cat")); cat != nil {
			for _, s := range findAll(cat, byTag("span")) {
				circuits = append(circuits, strings.ToUpper(text(s)))
			}
		}
		if d := find(doc, byClass("event__discipline")); d != nil {
			for part := range strings.SplitSeq(text(d), "+") {
				if p := strings.ToLower(strings.TrimSpace(part)); p == "solo" || p == "synchro" {
					disciplines = append(disciplines, p)
				}
			}
		}
		website := ""
		if b := find(doc, byClass("event-details__button")); b != nil {
			website = safeHref(attr(b, "href"))
		}
		status := "confirmed"
		if end < today {
			status = "finished"
		}
		year, _ := strconv.Atoi(start[:4])
		id := ""
		old, err := w.q.GetEventBySeasonSlugAny(ctx, gen.GetEventBySeasonSlugAnyParams{SeasonYear: int64(year), Slug: slug})
		switch {
		case err == nil:
			id = old.ID
		case errors.Is(err, sql.ErrNoRows):
			gone, derr := w.q.CountDeletedEvents(ctx, gen.CountDeletedEventsParams{SeasonYear: int64(year), Slug: slug})
			if derr != nil {
				return fmt.Errorf("get event %s: %w", slug, derr)
			}
			if gone > 0 {
				w.rep.warnf("event %s (%d): deleted in the admin, not restored", slug, year)
				continue
			}
			if id, err = newHexID(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("get event %s: %w", slug, err)
		}
		cj, _ := json.Marshal(nonNil(circuits))
		dj, _ := json.Marshal(nonNil(disciplines))
		err = w.q.UpsertEvent(ctx, gen.UpsertEventParams{ID: id, Slug: slug, SeasonYear: int64(year), Name: title, StartDate: start,
			EndDate: end, Site: site, Country: country, Circuits: string(cj), Disciplines: string(dj),
			CoverFile: sql.NullString{String: cover, Valid: cover != ""}, DescriptionMd: conv.markdown(),
			Website: sql.NullString{String: website, Valid: website != ""}, Status: status})
		if err != nil {
			return fmt.Errorf("import event %s: %w", slug, err)
		}
		w.rep.Counts["events"]++
	}
	return nil
}

// ---- documents ----

var sportingCodeRe = regexp.MustCompile(`^sporting_code_s7_b_-_aerobatics_(\d{4})\.pdf$`)

// documents stores the sporting code PDFs and lists them as documents. The
// latest year becomes the home page's "Learn the rules" link when no document
// is configured yet.
func (w *wwwImport) documents(ctx context.Context) error {
	entries, err := os.ReadDir(filepath.Join(w.dir, "assets", "files"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("list documents: %w", err)
	}
	existing, err := w.q.ListDocuments(ctx)
	if err != nil {
		return fmt.Errorf("list documents: %w", err)
	}
	latestYear, latestID := 0, ""
	for _, e := range entries {
		m := sportingCodeRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		year, _ := strconv.Atoi(m[1])
		fileID, ok := w.upload(ctx, "assets/files/"+e.Name())
		if w.err != nil {
			return w.err
		}
		if !ok {
			continue
		}
		id := ""
		for _, d := range existing {
			if d.FileID.String == fileID {
				id = d.ID
			}
		}
		if id == "" {
			if id, err = newHexID(); err != nil {
				return err
			}
		}
		err = w.q.UpsertDocument(ctx, gen.UpsertDocumentParams{ID: id, Title: "FAI Sporting Code Section 7B Aerobatics " + m[1],
			Year: sql.NullInt64{Int64: int64(year), Valid: true}, Category: "sporting-code", FileID: sql.NullString{String: fileID, Valid: true}})
		if err != nil {
			return fmt.Errorf("import document %s: %w", e.Name(), err)
		}
		w.rep.Counts["documents"]++
		if year > latestYear {
			latestYear, latestID = year, id
		}
	}
	if latestID == "" {
		return nil
	}
	if _, err := w.q.GetSiteSetting(ctx, "sporting_code_document"); errors.Is(err, sql.ErrNoRows) {
		v, _ := json.Marshal(latestID)
		return w.q.UpsertSiteSetting(ctx, gen.UpsertSiteSettingParams{Key: "sporting_code_document", Value: string(v)})
	} else if err != nil {
		return fmt.Errorf("get setting: %w", err)
	}
	return nil
}

// ---- redirects ----

// redirects seeds a permanent redirect for every legacy path: the .html
// variants of news, events and result pages (the legacy host served both
// spellings), the top-level pages and the self-hosted PDFs.
func (w *wwwImport) redirects(ctx context.Context) error {
	pairs := [][2]string{{"/index.html", "/"}, {"/events.html", "/events"}, {"/pilots.html", "/pilots"}, {"/results.html", "/results"}}
	for _, d := range []struct{ sub, prefix string }{{"news", "/news/"}, {"event", "/event/"}} {
		files, err := htmlFiles(filepath.Join(w.dir, d.sub))
		if err != nil {
			return fmt.Errorf("list %s: %w", d.sub, err)
		}
		for _, f := range files {
			pairs = append(pairs, [2]string{d.prefix + f, d.prefix + slugOfFile(f)})
		}
	}
	years, _ := os.ReadDir(filepath.Join(w.dir, "result"))
	for _, y := range years {
		if !y.IsDir() {
			continue
		}
		files, _ := htmlFiles(filepath.Join(w.dir, "result", y.Name()))
		for _, f := range files {
			pairs = append(pairs, [2]string{"/result/" + y.Name() + "/" + f, "/result/" + y.Name() + "/" + slugOfFile(f)})
		}
	}
	for rel, id := range w.files {
		if id != "" && strings.HasPrefix(rel, "assets/files/") {
			pairs = append(pairs, [2]string{"/" + rel, "/files/" + id})
		}
	}
	slices.SortFunc(pairs, func(a, b [2]string) int { return strings.Compare(a[0], b[0]) })
	for _, p := range pairs {
		if err := w.q.UpsertRedirect(ctx, gen.UpsertRedirectParams{FromPath: p[0], ToUrl: p[1], Permanent: true}); err != nil {
			return fmt.Errorf("seed redirect %s: %w", p[0], err)
		}
		w.rep.Counts["redirects"]++
	}
	return nil
}

// firstCommitDate is the author date of the commit that added the file in the
// git repository containing it: the best available publication date of an
// undated legacy article. It returns "" when git or the history is missing.
func firstCommitDate(ctx context.Context, file string) string {
	// Absolute path: git resolves a pathspec against the repository root, not
	// against the current directory, so a relative --from would not match.
	abs, err := filepath.Abs(file)
	if err != nil {
		return ""
	}
	run := func(args ...string) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return exec.CommandContext(c, "git", args...).Output()
	}
	top, err := run("-C", filepath.Dir(abs), "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	out, err := run("-C", strings.TrimSpace(string(top)), "log", "--diff-filter=A", "--follow", "--format=%aI", "--", abs)
	if err != nil {
		return ""
	}
	lines := strings.Fields(string(out))
	if len(lines) == 0 {
		return ""
	}
	t, err := time.Parse(time.RFC3339, lines[len(lines)-1]) // oldest addition
	if err != nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
