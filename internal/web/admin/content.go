package admin

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/domain"
	"awt/internal/markdown"
	"awt/internal/service"
	"awt/internal/web"
)

// contentMax caps content form bodies (Markdown texts): 256 KB.
const contentMax = 256 << 10

// content holds the website content screens: news, events, documents, pages,
// redirects and site settings (spec 07 §10). Admin only (decision 0009).
type content struct {
	*handlers
	purger cache.Purger
	cfg    config.Config
}

type imageOption struct{ ID, Name string }

var eventStatusOptions = []option{
	{"planned", "Planned"}, {"confirmed", "Confirmed"}, {"cancelled", "Cancelled"}, {"finished", "Finished"},
}

func (h *handlers) registerContent(r *web.Router) {
	d := r.Deps()
	c := &content{handlers: h, purger: d.Purger, cfg: d.Cfg}
	get(r, "/manager/content", c.home)
	r.AdminMax("POST /manager/content/preview", contentMax, http.HandlerFunc(c.preview))

	get(r, "/manager/content/news", c.newsList)
	get(r, "/manager/content/news/new", c.newsNew)
	r.AdminMax("POST /manager/content/news", contentMax, http.HandlerFunc(c.newsCreate))
	get(r, "/manager/content/news/{id}/edit", c.newsEdit)
	r.AdminMax("POST /manager/content/news/{id}", contentMax, http.HandlerFunc(c.newsUpdate))
	get(r, "/manager/content/news/{id}/delete", c.newsDeleteConfirm)
	post(r, "/manager/content/news/{id}/delete", c.newsDelete)

	get(r, "/manager/content/events", c.eventsList)
	get(r, "/manager/content/events/new", c.eventNew)
	r.AdminMax("POST /manager/content/events", contentMax, http.HandlerFunc(c.eventCreate))
	get(r, "/manager/content/events/{id}/edit", c.eventEdit)
	r.AdminMax("POST /manager/content/events/{id}", contentMax, http.HandlerFunc(c.eventUpdate))
	get(r, "/manager/content/events/{id}/delete", c.eventDeleteConfirm)
	post(r, "/manager/content/events/{id}/delete", c.eventDelete)

	get(r, "/manager/content/documents", c.documentsList)
	get(r, "/manager/content/documents/new", c.documentNew)
	postUpload(r, "/manager/content/documents", c.documentCreate)
	get(r, "/manager/content/documents/{id}/edit", c.documentEdit)
	postUpload(r, "/manager/content/documents/{id}", c.documentUpdate)
	get(r, "/manager/content/documents/{id}/delete", c.documentDeleteConfirm)
	post(r, "/manager/content/documents/{id}/delete", c.documentDelete)

	get(r, "/manager/content/pages", c.pagesList)
	get(r, "/manager/content/pages/new", c.pageNew)
	r.AdminMax("POST /manager/content/pages", contentMax, http.HandlerFunc(c.pageCreate))
	get(r, "/manager/content/pages/{id}/edit", c.pageEdit)
	r.AdminMax("POST /manager/content/pages/{id}", contentMax, http.HandlerFunc(c.pageUpdate))
	get(r, "/manager/content/pages/{id}/delete", c.pageDeleteConfirm)
	post(r, "/manager/content/pages/{id}/delete", c.pageDelete)

	get(r, "/manager/content/redirects", c.redirectsList)
	post(r, "/manager/content/redirects", c.redirectSave)
	post(r, "/manager/content/redirects/delete", c.redirectDelete)

	get(r, "/manager/content/settings", c.settingsShow)
	post(r, "/manager/content/settings", c.settingsSave)
}

// purge asks the CDN to drop the pages a change affects. It runs after the
// commit and never fails the request.
func (c *content) purge(paths ...string) {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	if c.purger == nil || base == "" {
		return
	}
	urls := make([]string, len(paths))
	for i, p := range paths {
		urls[i] = base + p
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := c.purger.Purge(ctx, urls); err != nil {
			slog.Warn("cdn purge", "urls", urls, "err", err)
		}
	}()
}

// ---- hub and preview ----

type contentHomeView struct{ News, Events, Documents, Pages, Redirects int }

func (c *content) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var v contentHomeView
	news, err := c.svc.ListNews(ctx)
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	events, err := c.svc.ListCalendarEvents(ctx)
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	docs, err := c.svc.ListDocuments(ctx)
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	pages, err := c.svc.ListPages(ctx)
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	reds, err := c.svc.ListRedirects(ctx)
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	v = contentHomeView{News: len(news), Events: len(events), Documents: len(docs), Pages: len(pages), Redirects: len(reds)}
	render(w, r, http.StatusOK, contentHome(v))
}

// preview renders the Markdown of any content field. The output is goldmark's
// sanitised HTML (raw HTML in the source is dropped).
func (c *content) preview(w http.ResponseWriter, r *http.Request) {
	for _, f := range []string{"body_md", "description_md"} {
		if v := r.PostFormValue(f); v != "" {
			render(w, r, http.StatusOK, previewFragment(markdown.HTML(v)))
			return
		}
	}
	render(w, r, http.StatusOK, previewFragment(""))
}

func (c *content) images(w http.ResponseWriter, r *http.Request) ([]imageOption, bool) {
	files, ok := c.imageFiles(w, r)
	if !ok {
		return nil, false
	}
	out := make([]imageOption, len(files))
	for i, f := range files {
		out[i] = imageOption{ID: f.ID, Name: f.Filename}
	}
	return out, true
}

func formHeading(id, what string) string {
	if id == "" {
		return "New " + what
	}
	return "Edit " + what
}

func formAction(base, id string) string {
	if id == "" {
		return base
	}
	return base + "/" + id
}

func savedKey(id string) string {
	if id == "" {
		return "created"
	}
	return "saved"
}

// ---- news ----

type newsForm struct {
	ID, Title, Slug, Author, PublishedAt, Excerpt, Cover, BodyMD string
}

type newsListView struct {
	Items []service.News
	Flash string
}

type newsFormView struct {
	Form   newsForm
	Images []imageOption
	Errors formErrors
}

func publishedLabel(s string) string {
	if s == "" {
		return "Draft"
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		label := t.Format(time.DateOnly)
		if t.After(time.Now()) {
			label += " (scheduled)"
		}
		return label
	}
	return s
}

func (c *content) newsList(w http.ResponseWriter, r *http.Request) {
	items, err := c.svc.ListNews(r.Context())
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, newsListPage(newsListView{Items: items, Flash: flash(r)}))
}

func (c *content) newsFormPage(w http.ResponseWriter, r *http.Request, status int, f newsForm, fe formErrors) {
	images, ok := c.images(w, r)
	if !ok {
		return
	}
	render(w, r, status, newsFormPage(newsFormView{Form: f, Images: images, Errors: fe}))
}

func (c *content) newsNew(w http.ResponseWriter, r *http.Request) {
	c.newsFormPage(w, r, http.StatusOK, newsForm{}, formErrors{})
}

func (c *content) newsSave(w http.ResponseWriter, r *http.Request, id string) {
	f := newsForm{ID: id, Title: r.PostFormValue("title"), Slug: r.PostFormValue("slug"), Author: r.PostFormValue("author"),
		PublishedAt: r.PostFormValue("published_at"), Excerpt: r.PostFormValue("excerpt"), Cover: r.PostFormValue("cover"),
		BodyMD: r.PostFormValue("body_md")}
	redo := func(status int, fe formErrors) { c.newsFormPage(w, r, status, f, fe) }
	in := service.NewsInput{Slug: f.Slug, Title: f.Title, Excerpt: f.Excerpt, BodyMD: f.BodyMD, CoverFile: f.Cover, Author: f.Author, PublishedAt: f.PublishedAt}
	var n service.News
	var err error
	if id == "" {
		n, err = c.svc.CreateNews(r.Context(), in)
	} else {
		n, err = c.svc.UpdateNews(r.Context(), id, in)
	}
	if err != nil {
		c.fail(w, r, err, redo)
		return
	}
	c.purge("/", "/news", "/news/"+n.Slug, "/sitemap.xml")
	redirect(w, r, "/manager/content/news", savedKey(id))
}

func (c *content) newsCreate(w http.ResponseWriter, r *http.Request) { c.newsSave(w, r, "") }
func (c *content) newsUpdate(w http.ResponseWriter, r *http.Request) { c.newsSave(w, r, pathID(r)) }

func (c *content) newsEdit(w http.ResponseWriter, r *http.Request) {
	n, err := c.svc.GetNews(r.Context(), pathID(r))
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	pub := ""
	if t, err := time.Parse(time.RFC3339, n.PublishedAt); err == nil {
		pub = t.UTC().Format(time.DateOnly)
	}
	c.newsFormPage(w, r, http.StatusOK, newsForm{ID: n.ID, Title: n.Title, Slug: n.Slug, Author: n.Author, PublishedAt: pub,
		Excerpt: n.Excerpt, Cover: n.CoverFile, BodyMD: n.BodyMD}, formErrors{})
}

func (c *content) newsDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	n, err := c.svc.GetNews(r.Context(), pathID(r))
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, confirmDelete("content", "article", n.Title, "/manager/content/news/"+n.ID+"/delete", "/manager/content/news"))
}

func (c *content) newsDelete(w http.ResponseWriter, r *http.Request) {
	n, err := c.svc.GetNews(r.Context(), pathID(r))
	if err == nil {
		err = c.svc.DeleteNews(r.Context(), n.ID)
	}
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	c.purge("/", "/news", "/news/"+n.Slug, "/sitemap.xml")
	redirect(w, r, "/manager/content/news", "deleted")
}

// ---- events ----

type eventForm struct {
	ID, Name, Slug, StartDate, EndDate, SeasonYear, Site, Country, Circuits string
	Solo, Synchro                                                           bool
	Status, Cover, Website, RegistrationURL, CivlURL, LiveStreamURL         string
	DescriptionMD                                                           string
	Competitions                                                            []string
}

func (f eventForm) hasCompetition(id string) bool {
	for _, c := range f.Competitions {
		if c == id {
			return true
		}
	}
	return false
}

type eventsListView struct {
	Items []service.CalendarEvent
	Flash string
}

type eventFormView struct {
	Form         eventForm
	Images       []imageOption
	Competitions []service.CompetitionChoice
	Errors       formErrors
}

func (c *content) eventsList(w http.ResponseWriter, r *http.Request) {
	items, err := c.svc.ListCalendarEvents(r.Context())
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, eventsListPage(eventsListView{Items: items, Flash: flash(r)}))
}

func (c *content) eventFormPage(w http.ResponseWriter, r *http.Request, status int, f eventForm, fe formErrors) {
	images, ok := c.images(w, r)
	if !ok {
		return
	}
	comps, err := c.svc.ListCompetitionChoices(r.Context())
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, status, eventFormPage(eventFormView{Form: f, Images: images, Competitions: comps, Errors: fe}))
}

func (c *content) eventNew(w http.ResponseWriter, r *http.Request) {
	c.eventFormPage(w, r, http.StatusOK, eventForm{Status: "planned", Solo: true}, formErrors{})
}

func splitCSV(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
}

func (c *content) eventSave(w http.ResponseWriter, r *http.Request, id string) {
	p := r.PostFormValue
	f := eventForm{ID: id, Name: p("name"), Slug: p("slug"), StartDate: p("start_date"), EndDate: p("end_date"), SeasonYear: p("season_year"),
		Site: p("site"), Country: p("country"), Circuits: p("circuits"), Solo: p("solo") != "", Synchro: p("synchro") != "",
		Status: p("status"), Cover: p("cover"), Website: strings.TrimSpace(p("website")), RegistrationURL: strings.TrimSpace(p("registration_url")),
		CivlURL: strings.TrimSpace(p("civl_url")), LiveStreamURL: strings.TrimSpace(p("live_stream_url")), DescriptionMD: p("description_md"),
		Competitions: r.PostForm["competitions"]}
	redo := func(status int, fe formErrors) { c.eventFormPage(w, r, status, f, fe) }
	in := service.CalendarEventInput{Slug: f.Slug, Name: f.Name, StartDate: strings.TrimSpace(f.StartDate), EndDate: strings.TrimSpace(f.EndDate),
		Site: f.Site, Country: strings.ToLower(strings.TrimSpace(f.Country)), Circuits: splitCSV(f.Circuits), CoverFile: f.Cover,
		DescriptionMD: f.DescriptionMD, Website: f.Website, RegistrationURL: f.RegistrationURL, CivlURL: f.CivlURL,
		LiveStreamURL: f.LiveStreamURL, Status: f.Status, CompetitionIDs: f.Competitions}
	if f.Solo {
		in.Disciplines = append(in.Disciplines, "solo")
	}
	if f.Synchro {
		in.Disciplines = append(in.Disciplines, "synchro")
	}
	if y := strings.TrimSpace(f.SeasonYear); y != "" {
		n, bad := atoi("season_year", y)
		if bad != nil {
			redo(http.StatusUnprocessableEntity, validationErrors([]domain.FieldError{*bad}))
			return
		}
		in.SeasonYear = n
	}
	var e service.CalendarEvent
	var err error
	if id == "" {
		e, err = c.svc.CreateCalendarEvent(r.Context(), in)
	} else {
		e, err = c.svc.UpdateCalendarEvent(r.Context(), id, in)
	}
	if err != nil {
		c.fail(w, r, err, redo)
		return
	}
	c.purge("/", "/events", "/event/"+e.Slug, "/sitemap.xml")
	redirect(w, r, "/manager/content/events", savedKey(id))
}

func (c *content) eventCreate(w http.ResponseWriter, r *http.Request) { c.eventSave(w, r, "") }
func (c *content) eventUpdate(w http.ResponseWriter, r *http.Request) { c.eventSave(w, r, pathID(r)) }

func (c *content) eventEdit(w http.ResponseWriter, r *http.Request) {
	e, err := c.svc.GetCalendarEvent(r.Context(), pathID(r))
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	f := eventForm{ID: e.ID, Name: e.Name, Slug: e.Slug, StartDate: e.StartDate, EndDate: e.EndDate, SeasonYear: strconv.Itoa(e.SeasonYear),
		Site: e.Site, Country: e.Country, Circuits: strings.Join(e.Circuits, ", "), Status: e.Status, Cover: e.CoverFile,
		Website: e.Website, RegistrationURL: e.RegistrationURL, CivlURL: e.CivlURL, LiveStreamURL: e.LiveStreamURL,
		DescriptionMD: e.DescriptionMD, Competitions: e.CompetitionIDs}
	for _, d := range e.Disciplines {
		f.Solo = f.Solo || d == "solo"
		f.Synchro = f.Synchro || d == "synchro"
	}
	c.eventFormPage(w, r, http.StatusOK, f, formErrors{})
}

func (c *content) eventDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	e, err := c.svc.GetCalendarEvent(r.Context(), pathID(r))
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, confirmDelete("content", "event", e.Name, "/manager/content/events/"+e.ID+"/delete", "/manager/content/events"))
}

func (c *content) eventDelete(w http.ResponseWriter, r *http.Request) {
	e, err := c.svc.GetCalendarEvent(r.Context(), pathID(r))
	if err == nil {
		err = c.svc.DeleteCalendarEvent(r.Context(), e.ID)
	}
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	c.purge("/", "/events", "/event/"+e.Slug, "/sitemap.xml")
	redirect(w, r, "/manager/content/events", "deleted")
}

// ---- documents ----

type documentForm struct {
	ID, Title, Year, Category, FileID, URL string
}

type documentsListView struct {
	Items []service.Document
	Flash string
}

type documentFormView struct {
	Form   documentForm
	Errors formErrors
}

func docTarget(d service.Document) string {
	if d.FileID != "" {
		return "PDF " + shortID(d.FileID)
	}
	return d.URL
}

func (c *content) documentsList(w http.ResponseWriter, r *http.Request) {
	items, err := c.svc.ListDocuments(r.Context())
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, documentsListPage(documentsListView{Items: items, Flash: flash(r)}))
}

func (c *content) documentFormPage(w http.ResponseWriter, r *http.Request, status int, f documentForm, fe formErrors) {
	render(w, r, status, documentFormPage(documentFormView{Form: f, Errors: fe}))
}

func (c *content) documentNew(w http.ResponseWriter, r *http.Request) {
	c.documentFormPage(w, r, http.StatusOK, documentForm{}, formErrors{})
}

func (c *content) documentSave(w http.ResponseWriter, r *http.Request, id, currentFile string) {
	f := documentForm{ID: id, Title: r.PostFormValue("title"), Year: strings.TrimSpace(r.PostFormValue("year")),
		Category: r.PostFormValue("category"), FileID: currentFile, URL: r.PostFormValue("url")}
	redo := func(status int, fe formErrors) { c.documentFormPage(w, r, status, f, fe) }
	in := service.DocumentInput{Title: f.Title, Category: f.Category, FileID: currentFile, URL: f.URL}
	if f.Year != "" {
		n, bad := atoi("year", f.Year)
		if bad != nil {
			redo(http.StatusUnprocessableEntity, validationErrors([]domain.FieldError{*bad}))
			return
		}
		in.Year = n
	}
	if file, hdr, err := r.FormFile("file"); err == nil {
		defer file.Close()
		up, err := c.svc.UploadFile(r.Context(), hdr.Filename, file)
		if err != nil {
			c.fail(w, r, err, redo)
			return
		}
		in.FileID = up.ID
	}
	var err error
	if id == "" {
		_, err = c.svc.CreateDocument(r.Context(), in)
	} else {
		_, err = c.svc.UpdateDocument(r.Context(), id, in)
	}
	if err != nil {
		c.fail(w, r, err, redo)
		return
	}
	c.purge("/", "/documents")
	redirect(w, r, "/manager/content/documents", savedKey(id))
}

func (c *content) documentCreate(w http.ResponseWriter, r *http.Request) {
	c.documentSave(w, r, "", "")
}

func (c *content) documentUpdate(w http.ResponseWriter, r *http.Request) {
	d, err := c.svc.GetDocument(r.Context(), pathID(r))
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	c.documentSave(w, r, d.ID, d.FileID)
}

func (c *content) documentEdit(w http.ResponseWriter, r *http.Request) {
	d, err := c.svc.GetDocument(r.Context(), pathID(r))
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	f := documentForm{ID: d.ID, Title: d.Title, Category: d.Category, FileID: d.FileID, URL: d.URL}
	if d.Year != 0 {
		f.Year = strconv.Itoa(d.Year)
	}
	c.documentFormPage(w, r, http.StatusOK, f, formErrors{})
}

func (c *content) documentDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	d, err := c.svc.GetDocument(r.Context(), pathID(r))
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, confirmDelete("content", "document", d.Title, "/manager/content/documents/"+d.ID+"/delete", "/manager/content/documents"))
}

func (c *content) documentDelete(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.DeleteDocument(r.Context(), pathID(r)); err != nil {
		c.fail(w, r, err, nil)
		return
	}
	c.purge("/", "/documents")
	redirect(w, r, "/manager/content/documents", "deleted")
}

// ---- pages ----

type pageForm struct{ Slug, Title, BodyMD string }

type pagesListView struct {
	Items []service.Page
	Flash string
}

type pageFormView struct {
	Form   pageForm
	Create bool
	Errors formErrors
}

// pageKey is "" for a new page, else its slug (the form action and heading).
func pageKey(v pageFormView) string {
	if v.Create {
		return ""
	}
	return v.Form.Slug
}

func (c *content) pagesList(w http.ResponseWriter, r *http.Request) {
	items, err := c.svc.ListPages(r.Context())
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, pagesListPage(pagesListView{Items: items, Flash: flash(r)}))
}

func (c *content) pageFormPage(w http.ResponseWriter, r *http.Request, status int, f pageForm, create bool, fe formErrors) {
	render(w, r, status, pageFormPage(pageFormView{Form: f, Create: create, Errors: fe}))
}

func (c *content) pageNew(w http.ResponseWriter, r *http.Request) {
	c.pageFormPage(w, r, http.StatusOK, pageForm{}, true, formErrors{})
}

func (c *content) pageSave(w http.ResponseWriter, r *http.Request, create bool, slug string) {
	f := pageForm{Slug: slug, Title: r.PostFormValue("title"), BodyMD: r.PostFormValue("body_md")}
	if create {
		f.Slug = strings.TrimSpace(r.PostFormValue("slug"))
	}
	redo := func(status int, fe formErrors) { c.pageFormPage(w, r, status, f, create, fe) }
	if _, err := c.svc.SavePage(r.Context(), create, service.Page{Slug: f.Slug, Title: f.Title, BodyMD: f.BodyMD}); err != nil {
		c.fail(w, r, err, redo)
		return
	}
	c.purge("/"+f.Slug, "/sitemap.xml")
	redirect(w, r, "/manager/content/pages", map[bool]string{true: "created", false: "saved"}[create])
}

func (c *content) pageCreate(w http.ResponseWriter, r *http.Request) { c.pageSave(w, r, true, "") }
func (c *content) pageUpdate(w http.ResponseWriter, r *http.Request) {
	c.pageSave(w, r, false, pathID(r))
}

func (c *content) pageEdit(w http.ResponseWriter, r *http.Request) {
	p, err := c.svc.GetPage(r.Context(), pathID(r))
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	c.pageFormPage(w, r, http.StatusOK, pageForm{Slug: p.Slug, Title: p.Title, BodyMD: p.BodyMD}, false, formErrors{})
}

func (c *content) pageDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	p, err := c.svc.GetPage(r.Context(), pathID(r))
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, http.StatusOK, confirmDelete("content", "page", p.Title+" (/"+p.Slug+")", "/manager/content/pages/"+p.Slug+"/delete", "/manager/content/pages"))
}

func (c *content) pageDelete(w http.ResponseWriter, r *http.Request) {
	slug := pathID(r)
	if err := c.svc.DeletePage(r.Context(), slug); err != nil {
		c.fail(w, r, err, nil)
		return
	}
	c.purge("/"+slug, "/sitemap.xml")
	redirect(w, r, "/manager/content/pages", "deleted")
}

// ---- redirects ----

type redirectForm struct {
	From, To  string
	Temporary bool
}

type redirectsView struct {
	Items  []service.Redirect
	Form   redirectForm
	Flash  string
	Errors formErrors
}

func redirectKind(permanent bool) string {
	if permanent {
		return "301"
	}
	return "302"
}

func (c *content) redirectsPage(w http.ResponseWriter, r *http.Request, status int, f redirectForm, fe formErrors) {
	items, err := c.svc.ListRedirects(r.Context())
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, status, redirectsPage(redirectsView{Items: items, Form: f, Flash: flash(r), Errors: fe}))
}

func (c *content) redirectsList(w http.ResponseWriter, r *http.Request) {
	c.redirectsPage(w, r, http.StatusOK, redirectForm{}, formErrors{})
}

func (c *content) redirectSave(w http.ResponseWriter, r *http.Request) {
	f := redirectForm{From: strings.TrimSpace(r.PostFormValue("from")), To: strings.TrimSpace(r.PostFormValue("to")),
		Temporary: r.PostFormValue("temporary") != ""}
	redo := func(status int, fe formErrors) { c.redirectsPage(w, r, status, f, fe) }
	if err := c.svc.SaveRedirect(r.Context(), service.Redirect{From: f.From, To: f.To, Permanent: !f.Temporary}); err != nil {
		c.fail(w, r, err, redo)
		return
	}
	c.purge(f.From)
	redirect(w, r, "/manager/content/redirects", "saved")
}

func (c *content) redirectDelete(w http.ResponseWriter, r *http.Request) {
	from := r.PostFormValue("from")
	if err := c.svc.DeleteRedirect(r.Context(), from); err != nil {
		c.fail(w, r, err, nil)
		return
	}
	c.purge(from)
	redirect(w, r, "/manager/content/redirects", "deleted")
}

// ---- settings ----

type settingsForm struct {
	HeroPreTitle, HeroTitle, HeroSubtitle, HeroImage, FeaturedVideo, FeaturedSeasons string
	DefaultDescription, DefaultOGImage, ContactEmail, SportingCodeDoc                string
}

type settingsView struct {
	Form      settingsForm
	Images    []imageOption
	Documents []service.Document
	Flash     string
	Errors    formErrors
}

func (c *content) settingsPage(w http.ResponseWriter, r *http.Request, status int, f settingsForm, fe formErrors) {
	images, ok := c.images(w, r)
	if !ok {
		return
	}
	docs, err := c.svc.ListDocuments(r.Context())
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	render(w, r, status, settingsPage(settingsView{Form: f, Images: images, Documents: docs, Flash: flash(r), Errors: fe}))
}

func (c *content) settingsShow(w http.ResponseWriter, r *http.Request) {
	st, err := c.svc.GetSettings(r.Context())
	if err != nil {
		c.fail(w, r, err, nil)
		return
	}
	c.settingsPage(w, r, http.StatusOK, settingsForm{HeroPreTitle: st.HeroPreTitle, HeroTitle: st.HeroTitle, HeroSubtitle: st.HeroSubtitle,
		HeroImage: st.HeroImage, FeaturedVideo: st.FeaturedVideo, FeaturedSeasons: strings.Join(st.FeaturedSeasons, ", "),
		DefaultDescription: st.DefaultDescription, DefaultOGImage: st.DefaultOGImage, ContactEmail: st.ContactEmail,
		SportingCodeDoc: st.SportingCodeDoc}, formErrors{})
}

func (c *content) settingsSave(w http.ResponseWriter, r *http.Request) {
	p := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	f := settingsForm{HeroPreTitle: p("hero_pre_title"), HeroTitle: p("hero_title"), HeroSubtitle: p("hero_subtitle"), HeroImage: p("hero_image"),
		FeaturedVideo: p("featured_video"), FeaturedSeasons: p("featured_seasons"), DefaultDescription: p("default_description"),
		DefaultOGImage: p("default_og_image"), ContactEmail: p("contact_email"), SportingCodeDoc: p("sporting_code_document")}
	redo := func(status int, fe formErrors) { c.settingsPage(w, r, status, f, fe) }
	err := c.svc.SaveSettings(r.Context(), service.SiteSettings{HeroPreTitle: f.HeroPreTitle, HeroTitle: f.HeroTitle, HeroSubtitle: f.HeroSubtitle,
		HeroImage: f.HeroImage, FeaturedVideo: f.FeaturedVideo, FeaturedSeasons: splitCSV(f.FeaturedSeasons),
		DefaultDescription: f.DefaultDescription, DefaultOGImage: f.DefaultOGImage, ContactEmail: f.ContactEmail, SportingCodeDoc: f.SportingCodeDoc})
	if err != nil {
		c.fail(w, r, err, redo)
		return
	}
	c.purge("/", "/documents")
	redirect(w, r, "/manager/content/settings", "saved")
}

// shortID abbreviates a file id for display.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
