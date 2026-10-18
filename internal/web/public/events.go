package public

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"awt/internal/markdown"
	"awt/internal/service"
	"awt/internal/store/gen"
)

type eventCard struct {
	Slug, Name  string
	Dates       string
	Place       string
	Circuits    []string
	Disciplines []string
	Image       string
	Status      string
	Upcoming    bool
	StartISO    string
}

func (c eventCard) Href() string { return "/event/" + c.Slug }

func (w *www) eventCard(e gen.Event) eventCard {
	place := e.Site
	if e.Country != "" {
		if place != "" {
			place += ", "
		}
		place += strings.ToUpper(e.Country)
	}
	return eventCard{Slug: e.Slug, Name: e.Name, Dates: dateRange(e.StartDate, e.EndDate), Place: place,
		Circuits: service.DecodeList(e.Circuits), Disciplines: service.DecodeList(e.Disciplines), Image: fileURL(e.CoverFile),
		Status: e.Status, Upcoming: e.EndDate >= w.today(), StartISO: e.StartDate}
}

type eventYear struct {
	Year   int
	Events []eventCard
}

type eventsView struct {
	Upcoming []eventCard
	Past     []eventYear
	Circuits []tab
	Circuit  string
}

// events is the calendar: upcoming events first, then past ones grouped by
// season year (latest first), filtered with ?circuit=AWT (spec 07 §10.2).
func (w *www) events(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := w.q.ListEventsAdmin(ctx)
	if err != nil {
		serverError(rw, r, err)
		return
	}
	want := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("circuit")))
	var circuits []string
	var all []eventCard
	for _, e := range rows {
		c := w.eventCard(e)
		for _, code := range c.Circuits {
			if !slices.Contains(circuits, code) {
				circuits = append(circuits, code)
			}
		}
		if want != "" && !slices.Contains(c.Circuits, want) {
			continue
		}
		all = append(all, c)
	}
	slices.Sort(circuits)
	v := eventsView{Circuit: want}
	byYear := map[int]*eventYear{}
	for _, c := range all {
		if c.Upcoming {
			v.Upcoming = append(v.Upcoming, c)
			continue
		}
		y := year(c.StartISO)
		if byYear[y] == nil {
			byYear[y] = &eventYear{Year: y}
			v.Past = append(v.Past, eventYear{Year: y})
		}
		byYear[y].Events = append(byYear[y].Events, c)
	}
	slices.SortFunc(v.Upcoming, func(a, b eventCard) int { return strings.Compare(a.StartISO, b.StartISO) })
	slices.SortFunc(v.Past, func(a, b eventYear) int { return b.Year - a.Year })
	for i := range v.Past {
		v.Past[i].Events = byYear[v.Past[i].Year].Events // rows are already newest first
	}
	v.Circuits = append(v.Circuits, tab{Label: "All", Href: "/events", Current: want == ""})
	for _, c := range circuits {
		v.Circuits = append(v.Circuits, tab{Label: c, Href: "/events?circuit=" + c, Current: want == c})
	}
	canonical := "/events"
	if want != "" {
		canonical += "?circuit=" + want
	}
	st := w.settings(ctx)
	renderPage(rw, r, EventsPage(w.meta("Events", w.describe(st, "The Acro World Tour calendar: upcoming and past paragliding aerobatics events."), canonical, ""), v))
}

type eventCompetition struct{ Code, Name string }

type eventView struct {
	Card         eventCard
	HTML         string
	Website      string
	Registration string
	CIVL         string
	Live         string
	Competitions []eventCompetition
	LD           ldSportsEvent
}

var ldStatus = map[string]string{
	"cancelled": "https://schema.org/EventCancelled",
}

// event is one event with its JSON-LD SportsEvent and links to the results
// of the competitions linked to it.
func (w *www) event(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	e, err := w.q.GetEventBySlugLatest(ctx, r.PathValue("slug"))
	if err != nil {
		w.fail(rw, r, err)
		return
	}
	comps, err := w.q.ListEventCompetitionsPublished(ctx, e.ID)
	if err != nil {
		serverError(rw, r, err)
		return
	}
	card := w.eventCard(e)
	v := eventView{Card: card, HTML: markdown.HTML(e.DescriptionMd), Website: e.Website.String, Registration: e.RegistrationUrl.String,
		CIVL: e.CivlUrl.String, Live: e.LiveStreamUrl.String}
	for _, c := range comps {
		v.Competitions = append(v.Competitions, eventCompetition{Code: c.Code, Name: c.Name})
	}
	desc := markdown.Plain(e.DescriptionMd, 180)
	if desc == "" {
		desc = e.Name + ", " + card.Dates + " (" + card.Place + "): an Acro World Tour paragliding aerobatics event."
	}
	path := "/event/" + e.Slug
	status := ldStatus[e.Status]
	if status == "" {
		status = "https://schema.org/EventScheduled"
	}
	v.LD = ldSportsEvent{Context: "https://schema.org", Type: "SportsEvent", Name: e.Name, Description: desc,
		StartDate: e.StartDate, EndDate: e.EndDate, Status: status, URL: w.abs(path)}
	if card.Place != "" {
		v.LD.Location = &ldPlace{Type: "Place", Name: card.Place}
		if e.Country != "" {
			v.LD.Location.Address = &ldAddress{Type: "PostalAddress", Country: strings.ToUpper(e.Country)}
		}
	}
	if card.Image != "" {
		v.LD.Image = []string{w.abs(card.Image)}
	}
	org := w.publisher()
	v.LD.Organizer = &org
	m := w.meta(e.Name+" "+strconv.FormatInt(e.SeasonYear, 10), desc, path, card.Image)
	m.Heading = e.Name
	renderPage(rw, r, EventPage(m, v))
}
