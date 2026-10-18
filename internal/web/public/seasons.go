package public

import (
	"net/http"
	"strconv"

	"awt/internal/store"
	"awt/internal/store/gen"
)

// seasonCard is a season in the list (spec 06 §4.2).
type seasonCard struct {
	gen.PublicListSeasonsRow
}

func (s seasonCard) Href() string { return "/seasons/" + s.Code }

func (s seasonCard) Synchro() bool { return s.Teams > 0 && s.Pilots == 0 }

func (s seasonCard) Count() string {
	if s.Synchro() {
		return countLabel(s.Teams, "team")
	}
	return countLabel(s.Pilots, "pilot")
}

type seasonsView struct {
	Years   []tab
	Year    int
	Seasons []seasonCard
}

// seasons lists the seasons of a year, ordered by index then name.
func (p *results) seasons(w http.ResponseWriter, r *http.Request) {
	rows, err := p.q.PublicListSeasons(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}
	now := p.now().Year()
	var years []int
	for _, s := range rows {
		years = append(years, int(s.Year))
	}
	sel := pickYear(r, years, now)
	v := seasonsView{Year: sel, Years: yearTabs(append(years, now, sel), sel, "/seasons")}
	for _, s := range rows {
		if int(s.Year) == sel {
			v.Seasons = append(v.Seasons, seasonCard{s})
		}
	}
	renderPage(w, r, SeasonsPage(p.meta("Seasons", "Acro World Tour seasons of "+strconv.Itoa(sel)+".", "/seasons", ""), v))
}

// breakdownItem is a participant's place in one competition of a season.
type breakdownItem struct {
	Rank  int64
	Score float64
	Comp  compCard
}

type seasonView struct {
	S       gen.Season
	Comps   []compCard
	Synchro bool
	Pilots  []gen.PublicSeasonPilotsRow
	Lists   []tab
	Rows    []rankRow
}

// season is a season's standings per list, with each participant's place
// in the season's competitions (spec 06 §4.3).
func (p *results) season(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, err := p.q.GetSeasonByCode(ctx, r.PathValue("code"))
	if err != nil {
		fail(w, r, err)
		return
	}
	comps, err := p.q.PublicSeasonCompetitions(ctx, s.Code)
	if err != nil {
		serverError(w, r, err)
		return
	}
	v := seasonView{S: s}
	byID := map[string]compCard{}
	solo := false
	for _, c := range comps {
		card := compCard{Code: c.Code, Name: c.Name, Type: c.Type, State: c.State, Location: c.Location, Start: c.StartDate,
			End: c.EndDate, Image: fileURL(c.ImageFile), Final: c.ResultsFinal, Pilots: c.Pilots, Teams: c.Teams}
		v.Comps = append(v.Comps, card)
		byID[c.ID] = card
		if c.Type == "synchro" {
			v.Synchro = true
		} else {
			solo = true
		}
	}
	v.Synchro = v.Synchro && !solo
	entries, err := p.q.PublicRankingEntries(ctx, gen.PublicRankingEntriesParams{Scope: store.ScopeSeason, ScopeKey: s.Code})
	if err != nil {
		serverError(w, r, err)
		return
	}
	places, err := p.q.PublicSeasonCompetitionEntries(ctx, s.Code)
	if err != nil {
		serverError(w, r, err)
		return
	}
	members := map[string][]member{}
	if v.Synchro {
		if members, err = p.teamMembers(r); err != nil {
			serverError(w, r, err)
			return
		}
	} else if v.Pilots, err = p.q.PublicSeasonPilots(ctx, s.Code); err != nil {
		serverError(w, r, err)
		return
	}
	breakdown := map[string][]breakdownItem{}
	for _, e := range places {
		k := newParticipant(e.Civlid, e.TeamID, "", "", "", nil).key()
		breakdown[k] = append(breakdown[k], breakdownItem{Rank: e.Rank, Score: e.Score, Comp: byID[e.CompetitionID]})
	}
	var keys []string
	for _, e := range entries {
		keys = append(keys, e.ListKey)
	}
	var sel string
	v.Lists, sel = listTabs(visibleLists(keys, nil), r.URL.Query().Get("list"), "/seasons/"+s.Code, nil)
	for _, e := range entries {
		if e.ListKey != sel {
			continue
		}
		pt := newParticipant(e.Civlid, e.TeamID, e.PilotName, e.PilotCountry, e.TeamName, members)
		v.Rows = append(v.Rows, rankRow{Rank: e.Rank, Score: e.Score, P: pt, Breakdown: breakdown[pt.key()]})
	}
	desc := s.Name + ": season standings of the Acro World Tour, paragliding aerobatics."
	renderPage(w, r, SeasonPage(p.meta(s.Name, desc, "/seasons/"+s.Code, fileURL(s.ImageFile)), v))
}
