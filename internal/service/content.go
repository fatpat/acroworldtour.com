package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"awt/internal/domain"
	"awt/internal/store/gen"
)

// Content use cases of the institutional site (spec 07 §10): news, events,
// documents, pages, redirects and site settings. Every mutation is one
// transaction; validation failures are *ValidationError.

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reservedPageSlugs are the top-level routes a page must not shadow.
var reservedPageSlugs = []string{
	"admin", "api", "assets", "competitions", "documents", "event", "events", "favicon.ico", "files", "healthz",
	"judges", "live", "news", "pilots", "result", "results", "robots.txt", "seasons", "simulator", "sitemap.xml",
	"static", "teams", "tricks",
}

const dateLayout = "2006-01-02"

func slugErr(slug string) []domain.FieldError {
	if !slugRe.MatchString(slug) {
		return []domain.FieldError{{Field: "slug", Msg: "use lowercase letters, digits and hyphens"}}
	}
	return nil
}

func httpURLErr(field, raw string) []domain.FieldError {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return []domain.FieldError{{Field: field, Msg: "must be an http(s) URL"}}
	}
	return nil
}

// ---- news ----

// News is a news article.
type News struct {
	ID          string
	Slug        string
	Title       string
	Excerpt     string
	BodyMD      string
	CoverFile   string
	Author      string
	PublishedAt string // RFC 3339 UTC; empty for a draft
}

// NewsInput is the editable part of an article. PublishedAt accepts RFC 3339
// or a plain date (taken as midnight UTC); empty keeps it a draft.
type NewsInput struct {
	Slug        string
	Title       string
	Excerpt     string
	BodyMD      string
	CoverFile   string
	Author      string
	PublishedAt string
}

func newsFromRow(r gen.News) News {
	return News{ID: r.ID, Slug: r.Slug, Title: r.Title, Excerpt: r.Excerpt, BodyMD: r.BodyMd, CoverFile: r.CoverFile.String,
		Author: r.Author, PublishedAt: r.PublishedAt.String}
}

// ParsePublishedAt normalises a publication date to RFC 3339 UTC ("" stays "").
func ParsePublishedAt(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", dateLayout} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC().Format(time.RFC3339), nil
		}
	}
	return "", errors.New("must be a date (YYYY-MM-DD)")
}

// ListNews returns every active article, drafts first then newest first.
func (s *Service) ListNews(ctx context.Context) ([]News, error) {
	rows, err := s.read().ListNewsAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("list news: %w", err)
	}
	out := make([]News, 0, len(rows))
	for _, r := range rows {
		out = append(out, newsFromRow(r))
	}
	return out, nil
}

// GetNews returns an active article by id.
func (s *Service) GetNews(ctx context.Context, id string) (News, error) {
	r, err := s.read().GetNewsByID(ctx, id)
	if err != nil {
		return News{}, notFound(err, "news "+id)
	}
	if r.DeletedAt.Valid {
		return News{}, fmt.Errorf("news %s: %w", id, ErrNotFound)
	}
	return newsFromRow(r), nil
}

func (s *Service) saveNews(ctx context.Context, id string, in NewsInput, mustExist bool) (News, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Slug = strings.TrimSpace(in.Slug)
	if in.Slug == "" {
		in.Slug = domain.Slug(in.Title)
	}
	var out gen.News
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		if mustExist {
			r, err := q.GetNewsByID(ctx, id)
			if err != nil || r.DeletedAt.Valid {
				return fmt.Errorf("news %s: %w", id, ErrNotFound)
			}
		}
		var errs []domain.FieldError
		if in.Title == "" {
			errs = append(errs, domain.FieldError{Field: "title", Msg: "required"})
		}
		errs = append(errs, slugErr(in.Slug)...)
		published, perr := ParsePublishedAt(in.PublishedAt)
		if perr != nil {
			errs = append(errs, domain.FieldError{Field: "published_at", Msg: perr.Error()})
		}
		if in.CoverFile != "" {
			if err := checkFile(ctx, q, in.CoverFile); err != nil {
				errs = append(errs, domain.FieldError{Field: "cover", Msg: err.Error()})
			}
		}
		if err := invalid(errs...); err != nil {
			return err
		}
		out = gen.News{ID: id, Slug: in.Slug, Title: in.Title, Excerpt: strings.TrimSpace(in.Excerpt), BodyMd: in.BodyMD,
			CoverFile: nullString(in.CoverFile), Author: strings.TrimSpace(in.Author), PublishedAt: nullString(published)}
		if err := q.UpsertNews(ctx, gen.UpsertNewsParams(out)); err != nil {
			return conflict(err, "news "+in.Slug)
		}
		return nil
	})
	return newsFromRow(out), err
}

// CreateNews creates an article.
func (s *Service) CreateNews(ctx context.Context, in NewsInput) (News, error) {
	id, err := newID()
	if err != nil {
		return News{}, err
	}
	return s.saveNews(ctx, id, in, false)
}

// UpdateNews replaces an active article's fields.
func (s *Service) UpdateNews(ctx context.Context, id string, in NewsInput) (News, error) {
	return s.saveNews(ctx, id, in, true)
}

// DeleteNews soft-deletes an article.
func (s *Service) DeleteNews(ctx context.Context, id string) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		r, err := q.GetNewsByID(ctx, id)
		if err != nil || r.DeletedAt.Valid {
			return fmt.Errorf("news %s: %w", id, ErrNotFound)
		}
		return q.SoftDeleteNews(ctx, gen.SoftDeleteNewsParams{DeletedAt: nullString(now()), ID: id})
	})
}

// ---- events ----

// Event statuses (schema CHECK).
var eventStatuses = []string{"planned", "confirmed", "cancelled", "finished"}

// Event disciplines.
var eventDisciplines = []string{"solo", "synchro"}

// CalendarEvent is a calendar event (named apart from the live-update Event), optionally linked to competitions.
type CalendarEvent struct {
	ID              string
	Slug            string
	SeasonYear      int
	Name            string
	StartDate       string
	EndDate         string
	Site            string
	Country         string
	Circuits        []string
	Disciplines     []string
	CoverFile       string
	DescriptionMD   string
	Website         string
	RegistrationURL string
	CivlURL         string
	LiveStreamURL   string
	Status          string
	CompetitionIDs  []string
}

// CalendarEventInput is the editable part of an event. SeasonYear defaults to the
// year of StartDate.
type CalendarEventInput struct {
	Slug            string
	SeasonYear      int
	Name            string
	StartDate       string
	EndDate         string
	Site            string
	Country         string
	Circuits        []string
	Disciplines     []string
	CoverFile       string
	DescriptionMD   string
	Website         string
	RegistrationURL string
	CivlURL         string
	LiveStreamURL   string
	Status          string
	CompetitionIDs  []string
}

// DecodeList decodes a JSON string array column (circuits, disciplines).
func DecodeList(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func eventFromRow(r gen.Event, comps []string) CalendarEvent {
	return CalendarEvent{ID: r.ID, Slug: r.Slug, SeasonYear: int(r.SeasonYear), Name: r.Name, StartDate: r.StartDate, EndDate: r.EndDate,
		Site: r.Site, Country: r.Country, Circuits: DecodeList(r.Circuits), Disciplines: DecodeList(r.Disciplines),
		CoverFile: r.CoverFile.String, DescriptionMD: r.DescriptionMd, Website: r.Website.String,
		RegistrationURL: r.RegistrationUrl.String, CivlURL: r.CivlUrl.String, LiveStreamURL: r.LiveStreamUrl.String,
		Status: r.Status, CompetitionIDs: comps}
}

// ListEvents returns the active events, latest season first.
func (s *Service) ListCalendarEvents(ctx context.Context) ([]CalendarEvent, error) {
	rows, err := s.read().ListEventsAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	out := make([]CalendarEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, eventFromRow(r, nil))
	}
	return out, nil
}

// GetEvent returns an active event with its linked competition ids.
func (s *Service) GetCalendarEvent(ctx context.Context, id string) (CalendarEvent, error) {
	q := s.read()
	r, err := q.GetEvent(ctx, id)
	if err != nil {
		return CalendarEvent{}, notFound(err, "event "+id)
	}
	if r.DeletedAt.Valid {
		return CalendarEvent{}, fmt.Errorf("event %s: %w", id, ErrNotFound)
	}
	comps, err := q.ListEventCompetitions(ctx, id)
	if err != nil {
		return CalendarEvent{}, fmt.Errorf("list event competitions: %w", err)
	}
	return eventFromRow(r, comps), nil
}

// CompetitionChoice is a competition an event can link to.
type CompetitionChoice struct{ ID, Code, Name string }

// ListCompetitionChoices returns the competitions an event can be linked to.
func (s *Service) ListCompetitionChoices(ctx context.Context) ([]CompetitionChoice, error) {
	rows, err := s.read().ListCompetitionChoices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list competitions: %w", err)
	}
	out := make([]CompetitionChoice, 0, len(rows))
	for _, r := range rows {
		out = append(out, CompetitionChoice{ID: r.ID, Code: r.Code, Name: r.Name})
	}
	return out, nil
}

func cleanList(in []string, upper bool) []string {
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if upper {
			v = strings.ToUpper(v)
		} else {
			v = strings.ToLower(v)
		}
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func validateEvent(ctx context.Context, q *gen.Queries, in *CalendarEventInput) []domain.FieldError {
	var errs []domain.FieldError
	in.Name = strings.TrimSpace(in.Name)
	in.Slug = strings.TrimSpace(in.Slug)
	if in.Name == "" {
		errs = append(errs, domain.FieldError{Field: "name", Msg: "required"})
	}
	if in.Slug == "" {
		in.Slug = domain.Slug(in.Name)
	}
	errs = append(errs, slugErr(in.Slug)...)
	start, serr := time.Parse(dateLayout, in.StartDate)
	end, eerr := time.Parse(dateLayout, in.EndDate)
	if serr != nil {
		errs = append(errs, domain.FieldError{Field: "start_date", Msg: "must be a date (YYYY-MM-DD)"})
	}
	if eerr != nil {
		errs = append(errs, domain.FieldError{Field: "end_date", Msg: "must be a date (YYYY-MM-DD)"})
	}
	if serr == nil && eerr == nil && end.Before(start) {
		errs = append(errs, domain.FieldError{Field: "end_date", Msg: "must not be before the start date"})
	}
	if in.SeasonYear == 0 && serr == nil {
		in.SeasonYear = start.Year()
	}
	if in.SeasonYear < 1901 || in.SeasonYear > 2999 {
		errs = append(errs, domain.FieldError{Field: "season_year", Msg: "must be a year between 1901 and 2999"})
	}
	if in.Status == "" {
		in.Status = "planned"
	}
	if !slices.Contains(eventStatuses, in.Status) {
		errs = append(errs, domain.FieldError{Field: "status", Msg: "must be one of " + strings.Join(eventStatuses, ", ")})
	}
	in.Circuits = cleanList(in.Circuits, true)
	in.Disciplines = cleanList(in.Disciplines, false)
	for _, d := range in.Disciplines {
		if !slices.Contains(eventDisciplines, d) {
			errs = append(errs, domain.FieldError{Field: "disciplines", Msg: "must be solo or synchro"})
		}
	}
	for _, f := range []struct{ field, v string }{
		{"website", in.Website}, {"registration_url", in.RegistrationURL}, {"civl_url", in.CivlURL}, {"live_stream_url", in.LiveStreamURL},
	} {
		errs = append(errs, httpURLErr(f.field, f.v)...)
	}
	if in.CoverFile != "" {
		if err := checkFile(ctx, q, in.CoverFile); err != nil {
			errs = append(errs, domain.FieldError{Field: "cover", Msg: err.Error()})
		}
	}
	for _, id := range in.CompetitionIDs {
		if _, err := q.GetCompetition(ctx, id); err != nil {
			errs = append(errs, domain.FieldError{Field: "competitions", Msg: "unknown competition " + id})
		}
	}
	return errs
}

func (s *Service) saveEvent(ctx context.Context, id string, in CalendarEventInput, mustExist bool) (CalendarEvent, error) {
	var out CalendarEvent
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		if mustExist {
			r, err := q.GetEvent(ctx, id)
			if err != nil || r.DeletedAt.Valid {
				return fmt.Errorf("event %s: %w", id, ErrNotFound)
			}
		}
		if err := invalid(validateEvent(ctx, q, &in)...); err != nil {
			return err
		}
		circuits, _ := json.Marshal(in.Circuits)
		disciplines, _ := json.Marshal(in.Disciplines)
		err := q.UpsertEvent(ctx, gen.UpsertEventParams{
			ID: id, Slug: in.Slug, SeasonYear: int64(in.SeasonYear), Name: in.Name, StartDate: in.StartDate, EndDate: in.EndDate,
			Site: strings.TrimSpace(in.Site), Country: strings.TrimSpace(in.Country), Circuits: string(circuits),
			Disciplines: string(disciplines), CoverFile: nullString(in.CoverFile), DescriptionMd: in.DescriptionMD,
			Website: nullString(in.Website), RegistrationUrl: nullString(in.RegistrationURL), CivlUrl: nullString(in.CivlURL),
			LiveStreamUrl: nullString(in.LiveStreamURL), Status: in.Status,
		})
		if err != nil {
			return conflict(err, fmt.Sprintf("event %s in %d", in.Slug, in.SeasonYear))
		}
		if err := q.DeleteEventCompetitions(ctx, id); err != nil {
			return fmt.Errorf("clear event competitions: %w", err)
		}
		for _, c := range in.CompetitionIDs {
			if err := q.AddEventCompetition(ctx, gen.AddEventCompetitionParams{EventID: id, CompetitionID: c}); err != nil {
				return fmt.Errorf("link event competition: %w", err)
			}
		}
		r, err := q.GetEvent(ctx, id)
		if err != nil {
			return fmt.Errorf("reload event: %w", err)
		}
		out = eventFromRow(r, in.CompetitionIDs)
		return nil
	})
	return out, err
}

// CreateEvent creates an event.
func (s *Service) CreateCalendarEvent(ctx context.Context, in CalendarEventInput) (CalendarEvent, error) {
	id, err := newID()
	if err != nil {
		return CalendarEvent{}, err
	}
	return s.saveEvent(ctx, id, in, false)
}

// UpdateEvent replaces an active event's fields and competition links.
func (s *Service) UpdateCalendarEvent(ctx context.Context, id string, in CalendarEventInput) (CalendarEvent, error) {
	return s.saveEvent(ctx, id, in, true)
}

// DeleteEvent soft-deletes an event.
func (s *Service) DeleteCalendarEvent(ctx context.Context, id string) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		r, err := q.GetEvent(ctx, id)
		if err != nil || r.DeletedAt.Valid {
			return fmt.Errorf("event %s: %w", id, ErrNotFound)
		}
		return q.SoftDeleteEvent(ctx, gen.SoftDeleteEventParams{DeletedAt: nullString(now()), ID: id})
	})
}

// ---- documents ----

// Document is a downloadable document: an uploaded PDF or an external URL.
type Document struct {
	ID       string
	Title    string
	Year     int // 0: none
	Category string
	FileID   string
	URL      string
}

// DocumentInput is the editable part of a document.
type DocumentInput struct {
	Title    string
	Year     int
	Category string
	FileID   string
	URL      string
}

func documentFromRow(r gen.Document) Document {
	return Document{ID: r.ID, Title: r.Title, Year: int(r.Year.Int64), Category: r.Category, FileID: r.FileID.String, URL: r.Url.String}
}

// ListDocuments returns the documents, latest year first.
func (s *Service) ListDocuments(ctx context.Context) ([]Document, error) {
	rows, err := s.read().ListDocuments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	out := make([]Document, 0, len(rows))
	for _, r := range rows {
		out = append(out, documentFromRow(r))
	}
	return out, nil
}

// GetDocument returns a document by id.
func (s *Service) GetDocument(ctx context.Context, id string) (Document, error) {
	r, err := s.read().GetDocument(ctx, id)
	if err != nil {
		return Document{}, notFound(err, "document "+id)
	}
	return documentFromRow(r), nil
}

func (s *Service) saveDocument(ctx context.Context, id string, in DocumentInput, mustExist bool) (Document, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.URL = strings.TrimSpace(in.URL)
	var out gen.Document
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		if mustExist {
			if _, err := q.GetDocument(ctx, id); err != nil {
				return notFound(err, "document "+id)
			}
		}
		var errs []domain.FieldError
		if in.Title == "" {
			errs = append(errs, domain.FieldError{Field: "title", Msg: "required"})
		}
		if in.Year != 0 && (in.Year < 1901 || in.Year > 2999) {
			errs = append(errs, domain.FieldError{Field: "year", Msg: "must be a year between 1901 and 2999"})
		}
		if in.FileID == "" && in.URL == "" {
			errs = append(errs, domain.FieldError{Field: "file", Msg: "upload a PDF or give a URL"})
		}
		errs = append(errs, httpURLErr("url", in.URL)...)
		if in.FileID != "" {
			f, err := q.GetFile(ctx, in.FileID)
			switch {
			case errors.Is(err, sql.ErrNoRows) || (err == nil && f.DeletedAt.Valid):
				errs = append(errs, domain.FieldError{Field: "file", Msg: "file " + in.FileID + " is unknown"})
			case err != nil:
				return fmt.Errorf("get file: %w", err)
			case f.ContentType != "application/pdf":
				errs = append(errs, domain.FieldError{Field: "file", Msg: "must be a PDF"})
			}
		}
		if err := invalid(errs...); err != nil {
			return err
		}
		out = gen.Document{ID: id, Title: in.Title, Year: sql.NullInt64{Int64: int64(in.Year), Valid: in.Year != 0},
			Category: strings.TrimSpace(in.Category), FileID: nullString(in.FileID), Url: nullString(in.URL)}
		return q.UpsertDocument(ctx, gen.UpsertDocumentParams(out))
	})
	return documentFromRow(out), err
}

// CreateDocument creates a document.
func (s *Service) CreateDocument(ctx context.Context, in DocumentInput) (Document, error) {
	id, err := newID()
	if err != nil {
		return Document{}, err
	}
	return s.saveDocument(ctx, id, in, false)
}

// UpdateDocument replaces a document's fields.
func (s *Service) UpdateDocument(ctx context.Context, id string, in DocumentInput) (Document, error) {
	return s.saveDocument(ctx, id, in, true)
}

// DeleteDocument removes a document (the uploaded file stays in the store).
func (s *Service) DeleteDocument(ctx context.Context, id string) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		n, err := q.DeleteDocument(ctx, id)
		if err != nil {
			return fmt.Errorf("delete document %s: %w", id, err)
		}
		if n == 0 {
			return fmt.Errorf("document %s: %w", id, ErrNotFound)
		}
		return nil
	})
}

// ---- pages ----

// Page is a generic static page served at /{slug}.
type Page struct {
	Slug   string
	Title  string
	BodyMD string
}

// ListPages returns the pages by slug.
func (s *Service) ListPages(ctx context.Context) ([]Page, error) {
	rows, err := s.read().ListPages(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	out := make([]Page, 0, len(rows))
	for _, r := range rows {
		out = append(out, Page{Slug: r.Slug, Title: r.Title, BodyMD: r.BodyMd})
	}
	return out, nil
}

// GetPage returns a page by slug.
func (s *Service) GetPage(ctx context.Context, slug string) (Page, error) {
	r, err := s.read().GetPage(ctx, slug)
	if err != nil {
		return Page{}, notFound(err, "page "+slug)
	}
	return Page{Slug: r.Slug, Title: r.Title, BodyMD: r.BodyMd}, nil
}

// SavePage creates the page (create true: ErrConflict when the slug is taken)
// or replaces its title and body.
func (s *Service) SavePage(ctx context.Context, create bool, p Page) (Page, error) {
	p.Slug = strings.TrimSpace(p.Slug)
	p.Title = strings.TrimSpace(p.Title)
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		var errs []domain.FieldError
		if p.Title == "" {
			errs = append(errs, domain.FieldError{Field: "title", Msg: "required"})
		}
		errs = append(errs, slugErr(p.Slug)...)
		if slices.Contains(reservedPageSlugs, p.Slug) {
			errs = append(errs, domain.FieldError{Field: "slug", Msg: "is reserved by the site"})
		}
		if err := invalid(errs...); err != nil {
			return err
		}
		_, err := q.GetPage(ctx, p.Slug)
		switch {
		case err == nil && create:
			return fmt.Errorf("page %s: %w", p.Slug, ErrConflict)
		case errors.Is(err, sql.ErrNoRows) && !create:
			return fmt.Errorf("page %s: %w", p.Slug, ErrNotFound)
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("get page: %w", err)
		}
		return q.UpsertPage(ctx, gen.UpsertPageParams{Slug: p.Slug, Title: p.Title, BodyMd: p.BodyMD})
	})
	return p, err
}

// DeletePage removes a page.
func (s *Service) DeletePage(ctx context.Context, slug string) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		n, err := q.DeletePage(ctx, slug)
		if err != nil {
			return fmt.Errorf("delete page %s: %w", slug, err)
		}
		if n == 0 {
			return fmt.Errorf("page %s: %w", slug, ErrNotFound)
		}
		return nil
	})
}

// ---- redirects ----

// Redirect sends a legacy or removed path to another URL.
type Redirect struct {
	From      string
	To        string
	Permanent bool
}

// ValidRedirectTarget accepts a local absolute path or an http(s) URL, and
// refuses protocol-relative targets and control characters (open redirect).
func ValidRedirectTarget(to string) bool {
	for _, c := range to {
		if c < 0x20 || c == 0x7f || c == '\\' {
			return false
		}
	}
	switch {
	case strings.HasPrefix(to, "//"):
		return false
	case strings.HasPrefix(to, "/"):
		return true
	}
	return strings.HasPrefix(to, "https://") || strings.HasPrefix(to, "http://")
}

// ListRedirects returns the redirects by source path.
func (s *Service) ListRedirects(ctx context.Context) ([]Redirect, error) {
	rows, err := s.read().ListRedirects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list redirects: %w", err)
	}
	out := make([]Redirect, 0, len(rows))
	for _, r := range rows {
		out = append(out, Redirect{From: r.FromPath, To: r.ToUrl, Permanent: r.Permanent})
	}
	return out, nil
}

// SaveRedirect creates or replaces the redirect of a source path.
func (s *Service) SaveRedirect(ctx context.Context, r Redirect) error {
	r.From, r.To = strings.TrimSpace(r.From), strings.TrimSpace(r.To)
	var errs []domain.FieldError
	switch {
	case !strings.HasPrefix(r.From, "/") || strings.HasPrefix(r.From, "//") || strings.ContainsAny(r.From, "?# \\"):
		errs = append(errs, domain.FieldError{Field: "from", Msg: "must be a path starting with / (no query or fragment)"})
	case r.From == "/admin" || strings.HasPrefix(r.From, "/admin/"):
		errs = append(errs, domain.FieldError{Field: "from", Msg: "admin paths cannot be redirected"})
	}
	if !ValidRedirectTarget(r.To) {
		errs = append(errs, domain.FieldError{Field: "to", Msg: "must be a path starting with / or an http(s) URL"})
	}
	if r.From == r.To {
		errs = append(errs, domain.FieldError{Field: "to", Msg: "must differ from the source"})
	}
	if err := invalid(errs...); err != nil {
		return err
	}
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		return q.UpsertRedirect(ctx, gen.UpsertRedirectParams{FromPath: r.From, ToUrl: r.To, Permanent: r.Permanent})
	})
}

// DeleteRedirect removes the redirect of a source path.
func (s *Service) DeleteRedirect(ctx context.Context, from string) error {
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		n, err := q.DeleteRedirect(ctx, from)
		if err != nil {
			return fmt.Errorf("delete redirect %s: %w", from, err)
		}
		if n == 0 {
			return fmt.Errorf("redirect %s: %w", from, ErrNotFound)
		}
		return nil
	})
}

// ---- site settings ----

// SiteSettings are the editable texts of the home page and the defaults of
// the SEO tags (spec 07 §10.1). Each field is one site_settings row holding
// a JSON value.
type SiteSettings struct {
	HeroPreTitle       string
	HeroTitle          string
	HeroSubtitle       string
	HeroImage          string   // file id
	FeaturedVideo      string   // YouTube URL
	FeaturedSeasons    []string // season codes shown on the home page
	DefaultDescription string
	DefaultOGImage     string // file id
	ContactEmail       string
	SportingCodeDoc    string // document id linked as "Learn the rules"
}

// ParseSettings builds the settings from stored rows; malformed values are
// ignored (the page falls back to its defaults).
func ParseSettings(rows []gen.SiteSetting) SiteSettings {
	var st SiteSettings
	for _, r := range rows {
		str := func() string {
			var v string
			_ = json.Unmarshal([]byte(r.Value), &v)
			return v
		}
		switch r.Key {
		case "hero_pre_title":
			st.HeroPreTitle = str()
		case "hero_title":
			st.HeroTitle = str()
		case "hero_subtitle":
			st.HeroSubtitle = str()
		case "hero_image":
			st.HeroImage = str()
		case "featured_video":
			st.FeaturedVideo = str()
		case "featured_seasons":
			_ = json.Unmarshal([]byte(r.Value), &st.FeaturedSeasons)
		case "default_description":
			st.DefaultDescription = str()
		case "default_og_image":
			st.DefaultOGImage = str()
		case "contact_email":
			st.ContactEmail = str()
		case "sporting_code_document":
			st.SportingCodeDoc = str()
		}
	}
	return st
}

// GetSettings returns the site settings.
func (s *Service) GetSettings(ctx context.Context) (SiteSettings, error) {
	rows, err := s.read().ListSiteSettings(ctx)
	if err != nil {
		return SiteSettings{}, fmt.Errorf("list settings: %w", err)
	}
	return ParseSettings(rows), nil
}

// SaveSettings replaces every setting.
func (s *Service) SaveSettings(ctx context.Context, st SiteSettings) error {
	st.FeaturedSeasons = cleanList(st.FeaturedSeasons, false)
	var errs []domain.FieldError
	for _, c := range st.FeaturedSeasons {
		if !slugRe.MatchString(c) {
			errs = append(errs, domain.FieldError{Field: "featured_seasons", Msg: "season codes like awt-2026, separated by commas"})
			break
		}
	}
	errs = append(errs, httpURLErr("featured_video", st.FeaturedVideo)...)
	if st.ContactEmail != "" && !strings.Contains(st.ContactEmail, "@") {
		errs = append(errs, domain.FieldError{Field: "contact_email", Msg: "must be an email address"})
	}
	if err := invalid(errs...); err != nil {
		return err
	}
	values := map[string]any{
		"hero_pre_title": st.HeroPreTitle, "hero_title": st.HeroTitle, "hero_subtitle": st.HeroSubtitle, "hero_image": st.HeroImage,
		"featured_video": st.FeaturedVideo, "featured_seasons": st.FeaturedSeasons, "default_description": st.DefaultDescription,
		"default_og_image": st.DefaultOGImage, "contact_email": st.ContactEmail, "sporting_code_document": st.SportingCodeDoc,
	}
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		var refs []domain.FieldError
		for _, f := range []struct{ field, id string }{{"hero_image", st.HeroImage}, {"default_og_image", st.DefaultOGImage}} {
			if f.id == "" {
				continue
			}
			if err := checkFile(ctx, q, f.id); err != nil {
				refs = append(refs, domain.FieldError{Field: f.field, Msg: err.Error()})
			}
		}
		if st.SportingCodeDoc != "" {
			if _, err := q.GetDocument(ctx, st.SportingCodeDoc); errors.Is(err, sql.ErrNoRows) {
				refs = append(refs, domain.FieldError{Field: "sporting_code_document", Msg: "unknown document"})
			} else if err != nil {
				return fmt.Errorf("get document: %w", err)
			}
		}
		if err := invalid(refs...); err != nil {
			return err
		}
		for _, key := range slices.Sorted(mapsKeys(values)) {
			b, err := json.Marshal(values[key])
			if err != nil {
				return fmt.Errorf("encode setting %s: %w", key, err)
			}
			if err := q.UpsertSiteSetting(ctx, gen.UpsertSiteSettingParams{Key: key, Value: string(b)}); err != nil {
				return fmt.Errorf("save setting %s: %w", key, err)
			}
		}
		return nil
	})
}

func mapsKeys[V any](m map[string]V) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}
