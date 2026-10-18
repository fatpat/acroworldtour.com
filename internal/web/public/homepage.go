package public

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"

	"awt/internal/service"
	"awt/internal/store"
	"awt/internal/store/gen"
)

// Home page sizes (spec 07 §2.1).
const (
	homeEvents   = 4
	homeNews     = 3
	homeStanding = 5
)

type standingRow struct {
	Rank  int64
	P     participant
	Score float64
}

type standingCard struct {
	Name, Code string
	Rows       []standingRow
}

type wwwHomeView struct {
	S            service.SiteSettings
	HeroImage    string
	Video        string
	Events       []eventCard
	News         []newsCard
	Standings    []standingCard
	CodeTitle    string
	CodeHref     string
	CodeExternal bool
}

// home is the institutional home: hero, upcoming events, latest news,
// featured season standings and the rules download (spec 07 §2.1). The
// standings are stored ranking_entries; nothing is computed here.
func (w *www) home(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st := w.settings(ctx)
	v := wwwHomeView{S: st, HeroImage: "", Video: youtubeEmbed(st.FeaturedVideo)}
	if st.HeroImage != "" {
		v.HeroImage = "/files/" + st.HeroImage
	}
	events, err := w.q.ListEventsFrom(ctx, gen.ListEventsFromParams{EndDate: w.today(), Limit: homeEvents})
	if err != nil {
		serverError(rw, r, err)
		return
	}
	for _, e := range events {
		v.Events = append(v.Events, w.eventCard(e))
	}
	news, err := w.q.ListPublishedNews(ctx, gen.ListPublishedNewsParams{Now: w.nowStored(), Max: homeNews})
	if err != nil {
		serverError(rw, r, err)
		return
	}
	for _, n := range news {
		v.News = append(v.News, cardFromNews(n))
	}
	for _, code := range st.FeaturedSeasons {
		card, ok, err := w.standings(r, code)
		if err != nil {
			serverError(rw, r, err)
			return
		}
		if ok {
			v.Standings = append(v.Standings, card)
		}
	}
	v.CodeTitle, v.CodeHref = w.sportingCode(r, st.SportingCodeDoc)
	v.CodeExternal = strings.HasPrefix(v.CodeHref, "http")
	desc := w.describe(st, st.HeroSubtitle)
	m := w.meta("", desc, "/", "")
	m.Heading = ""
	if st.DefaultOGImage != "" {
		m.OGImage = w.abs("/files/" + st.DefaultOGImage)
	}
	renderPage(rw, r, WWWHomePage(m, v))
}

// standings reads the top rows of a season's main list: the list named like
// the season, else "overall", else the first one.
func (w *www) standings(r *http.Request, code string) (standingCard, bool, error) {
	ctx := r.Context()
	s, err := w.q.GetSeasonByCode(ctx, code)
	if err != nil {
		return standingCard{}, false, ignoreNoRows(err)
	}
	entries, err := w.q.PublicRankingEntries(ctx, gen.PublicRankingEntriesParams{Scope: store.ScopeSeason, ScopeKey: s.Code})
	if err != nil {
		return standingCard{}, false, err
	}
	var keys []string
	for _, e := range entries {
		if len(keys) == 0 || keys[len(keys)-1] != e.ListKey {
			keys = append(keys, e.ListKey)
		}
	}
	if len(keys) == 0 {
		return standingCard{}, false, nil
	}
	sel := mainList(keys, s.Code)
	members := map[string][]member{}
	membersLoaded := false
	card := standingCard{Name: s.Name, Code: s.Code}
	for _, e := range entries {
		if e.ListKey != sel || len(card.Rows) >= homeStanding {
			continue
		}
		if e.TeamID.Valid && !membersLoaded {
			membersLoaded = true
			if members, err = (&results{q: w.q}).teamMembers(r); err != nil {
				return standingCard{}, false, err
			}
		}
		card.Rows = append(card.Rows, standingRow{Rank: e.Rank, Score: e.Score,
			P: newParticipant(e.Civlid, e.TeamID, e.PilotName, e.PilotCountry, e.TeamName, members)})
	}
	return card, len(card.Rows) > 0, nil
}

// mainList picks the list a season summary shows.
func mainList(keys []string, code string) string {
	for _, want := range []string{code, "overall"} {
		if slices.Contains(keys, want) {
			return want
		}
	}
	return keys[0]
}

func ignoreNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}
