package public

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"

	"awt/internal/ranking"
	"awt/internal/scoring"
	"awt/internal/store"
	"awt/internal/store/gen"
	"awt/internal/web/layout"
)

// compCard is a competition in a list (spec 06 §4.4).
type compCard struct {
	Code, Name, Type, State, Location string
	Start, End                        string
	Image                             string
	Final                             bool
	Pilots, Teams                     int64
}

func (c compCard) Href() string { return "/competitions/" + c.Code }

func (c compCard) Count() string {
	if c.Type == "synchro" {
		return countLabel(c.Teams, "team")
	}
	return countLabel(c.Pilots, "pilot")
}

func cardFromList(c gen.PublicListCompetitionsRow) compCard {
	return compCard{Code: c.Code, Name: c.Name, Type: c.Type, State: c.State, Location: c.Location, Start: c.StartDate,
		End: c.EndDate, Image: fileURL(c.ImageFile), Final: c.ResultsFinal, Pilots: c.Pilots, Teams: c.Teams}
}

// compGroup is a season heading and its competitions.
type compGroup struct {
	Name, Href string
	Comps      []compCard
}

type competitionsView struct {
	Years  []tab
	Year   int
	Groups []compGroup
}

// competitions lists the competitions of a year grouped by season, then
// "Off season" (spec 06 §4.4).
func (p *results) competitions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	comps, err := p.q.PublicListCompetitions(ctx)
	if err != nil {
		serverError(w, r, err)
		return
	}
	links, err := p.q.PublicListCompetitionSeasonLinks(ctx)
	if err != nil {
		serverError(w, r, err)
		return
	}
	seasons, err := p.q.ListSeasons(ctx)
	if err != nil {
		serverError(w, r, err)
		return
	}
	now := p.now().Year()
	years := []int{now}
	for _, c := range comps {
		for _, y := range []int{year(c.StartDate), year(c.EndDate)} {
			if y > 0 { // a malformed date makes no year tab
				years = append(years, y)
			}
		}
	}
	sel := pickYear(r, years[1:], now)
	v := competitionsView{Year: sel, Years: yearTabs(append(years, sel), sel, "/competitions")}

	codes := map[string][]string{}
	for _, l := range links {
		codes[l.CompetitionID] = append(codes[l.CompetitionID], l.SeasonCode)
	}
	known := map[string]bool{}
	for _, s := range seasons {
		known[s.Code] = true
	}
	var inYear []gen.PublicListCompetitionsRow
	for _, c := range comps {
		if year(c.StartDate) == sel || year(c.EndDate) == sel {
			inYear = append(inYear, c)
		}
	}
	// seasons are ordered year desc, index, code; cards by start date asc
	for _, s := range seasons {
		g := compGroup{Name: s.Name, Href: "/seasons/" + s.Code}
		for i := len(inYear) - 1; i >= 0; i-- {
			if slices.Contains(codes[inYear[i].ID], s.Code) {
				g.Comps = append(g.Comps, cardFromList(inYear[i]))
			}
		}
		if len(g.Comps) > 0 {
			v.Groups = append(v.Groups, g)
		}
	}
	// A competition whose seasons all are unknown or deleted would vanish:
	// it goes to "Off season" with the season-less ones.
	off := compGroup{Name: "Off season"}
	for _, c := range inYear {
		if !slices.ContainsFunc(codes[c.ID], func(code string) bool { return known[code] }) {
			off.Comps = append(off.Comps, cardFromList(c))
		}
	}
	if len(off.Comps) > 0 {
		v.Groups = append(v.Groups, off)
	}
	renderPage(w, r, CompetitionsPage(p.meta("Competitions",
		"Acro World Tour competitions of "+strconv.Itoa(sel)+", grouped by season.", "/competitions", ""), v))
}

// compHeader is what every tab of a competition page shows.
type compHeader struct {
	C           gen.Competition
	Seasons     []gen.PublicCompetitionSeasonsRow
	SeasonNames map[string]string
	SeasonOrder []string
	Runs        []gen.Run
	Tabs        []tab
}

func (h compHeader) Synchro() bool { return h.C.Type == "synchro" }

func (h compHeader) Href() string { return "/competitions/" + h.C.Code }

// loadCompetition reads the competition (published, not deleted), its
// seasons and runs, and builds the tab bar. active is "overall", "details"
// or the 1-based run number.
func (p *results) loadCompetition(r *http.Request, active string) (compHeader, error) {
	ctx := r.Context()
	var h compHeader
	c, err := p.q.PublicGetCompetition(ctx, r.PathValue("code"))
	if err != nil {
		return h, err
	}
	h.C = c
	if h.Seasons, err = p.q.PublicCompetitionSeasons(ctx, c.ID); err != nil {
		return h, err
	}
	h.SeasonNames = map[string]string{}
	for _, s := range h.Seasons {
		if s.SeasonName != "" { // unknown or deleted seasons keep their code as label
			h.SeasonNames[s.SeasonCode] = s.SeasonName
		}
		h.SeasonOrder = append(h.SeasonOrder, s.SeasonCode)
	}
	if h.Runs, err = p.q.ListRuns(ctx, c.ID); err != nil {
		return h, err
	}
	base := h.Href()
	h.Tabs = append(h.Tabs, tab{Label: "Overall", Href: base, Current: active == "overall"})
	for i := range h.Runs {
		n := strconv.Itoa(i + 1)
		h.Tabs = append(h.Tabs, tab{Label: "Run " + n, Href: base + "/runs/" + n, Current: active == n})
	}
	h.Tabs = append(h.Tabs, tab{Label: "Details", Href: base + "/details", Current: active == "details"})
	return h, nil
}

func (p *results) compMeta(h compHeader, title, path string) layout.Meta {
	desc := h.C.Name + " (" + dateRange(h.C.StartDate, h.C.EndDate)
	if h.C.Location != "" {
		desc += ", " + h.C.Location
	}
	desc += "): paragliding aerobatics results."
	return p.meta(title, desc, path, fileURL(h.C.ImageFile))
}

// rankRow is a line of an overall or season ranking.
type rankRow struct {
	Rank   int64
	P      participant
	Score  float64
	PerRun map[int]runCell
	// Breakdown: the participant's places in the season's competitions.
	Breakdown []breakdownItem
}

type runCell struct {
	Rank  int64
	Score float64
}

type overallView struct {
	compHeader
	Lists []tab
	Sel   string // selected list key
	Rows  []rankRow
}

// competitionOverall is the overall tab: one list per visible list key,
// with each participant's rank and score in every run (spec 06 §4.5).
func (p *results) competitionOverall(w http.ResponseWriter, r *http.Request) {
	h, err := p.loadCompetition(r, "overall")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := p.overall(r, h)
	if err != nil {
		serverError(w, r, err)
		return
	}
	renderPage(w, r, CompetitionOverallPage(p.compMeta(h, h.C.Name, h.Href()), v))
}

// overall loads the overall ranking of the list ?list= selects.
func (p *results) overall(r *http.Request, h compHeader) (overallView, error) {
	v := overallView{compHeader: h}
	entries, err := p.q.PublicRankingEntries(r.Context(), gen.PublicRankingEntriesParams{Scope: store.ScopeCompetition, ScopeKey: h.C.ID})
	if err != nil {
		return v, err
	}
	members := map[string][]member{}
	if h.Synchro() {
		if members, err = p.teamMembers(r); err != nil {
			return v, err
		}
	}
	var keys []string
	for _, e := range entries {
		keys = append(keys, e.ListKey)
	}
	v.Lists, v.Sel = listTabs(visibleLists(keys, h.SeasonOrder), r.URL.Query().Get("list"), h.Href(), h.SeasonNames)
	cells, err := p.runCells(r, h.C.ID, v.Sel)
	if err != nil {
		return v, err
	}
	for _, e := range entries {
		if e.ListKey != v.Sel {
			continue
		}
		row := rankRow{Rank: e.Rank, Score: e.Score, P: newParticipant(e.Civlid, e.TeamID, e.PilotName, e.PilotCountry, e.TeamName, members),
			PerRun: cells[newParticipant(e.Civlid, e.TeamID, "", "", "", nil).key()]}
		v.Rows = append(v.Rows, row)
	}
	return v, nil
}

// runCells is every participant's rank and score in each run of one list,
// counting published flights only and re-ranked like the run tabs
// (controller ruling): a run whose flight is unpublished has no cell. The
// overall totals stay as stored (decision 0005).
func (p *results) runCells(r *http.Request, compID, list string) (map[string]map[int]runCell, error) {
	cells := map[string]map[int]runCell{}
	if list == "" {
		return cells, nil
	}
	places, err := p.q.PublicCompetitionRunPlaces(r.Context(), gen.PublicCompetitionRunPlacesParams{CompetitionID: compID, ListKey: list})
	if err != nil {
		return nil, err
	}
	rp := make([]ranking.RunPlace, len(places))
	for i, pl := range places {
		rp[i] = ranking.RunPlace{RunIndex: int(pl.RunIdx), Key: newParticipant(pl.Civlid, pl.TeamID, "", "", "", nil).key(), Score: pl.Score}
	}
	for k, cs := range ranking.PublishedRunCells(rp) {
		cells[k] = map[int]runCell{}
		// runs are keyed by their 0-based position among the competition's runs
		for _, c := range cs {
			cells[k][c.RunIndex] = runCell{Rank: int64(c.Rank), Score: c.Score}
		}
	}
	return cells, nil
}

// sharedRank is ranking.SharedRank on the page's int64 ranks.
func sharedRank(n int, prev runCell, score float64) int64 {
	return int64(ranking.SharedRank(n, int(prev.Rank), prev.Score, score))
}

// flightRow is a published flight in a run list, with its detail.
type flightRow struct {
	FlightID int64
	Rank     int64
	P        participant
	Score    float64
	Scored   bool
	DNS      bool
	Mark     *scoring.FinalMark
	Tricks   []gen.PublicRunFlightTricksRow
	Marks    []gen.PublicRunFlightMarksRow
}

func (f flightRow) Anchor() string { return "flight-" + slugOr(f.P.key()) }

type runView struct {
	compHeader
	N     int
	Run   gen.Run
	Lists []tab
	Sel   string // selected list key
	Rows  []flightRow
}

// competitionRun is a run tab: published flights only, re-ranked after the
// unpublished ones are left out (controller ruling; legacy public run
// results were published-only), each with its flight detail.
func (p *results) competitionRun(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		notFound(w, r)
		return
	}
	h, err := p.loadCompetition(r, r.PathValue("n"))
	if err != nil {
		fail(w, r, err)
		return
	}
	if n < 1 || n > len(h.Runs) {
		notFound(w, r)
		return
	}
	v, err := p.run(r, h, n)
	if err != nil {
		serverError(w, r, err)
		return
	}
	title := h.C.Name + " – Run " + strconv.Itoa(n)
	renderPage(w, r, CompetitionRunPage(p.compMeta(h, title, h.Href()+"/runs/"+strconv.Itoa(n)), v))
}

// run loads the published flights of run n (1-based, in range) of the list
// ?list= selects.
func (p *results) run(r *http.Request, h compHeader, n int) (runView, error) {
	ctx := r.Context()
	v := runView{compHeader: h, N: n, Run: h.Runs[n-1]}
	arg := gen.PublicRunEntriesParams{CompetitionID: h.C.ID, RunIdx: v.Run.Idx}
	entries, err := p.q.PublicRunEntries(ctx, arg)
	if err != nil {
		return v, err
	}
	tricks, err := p.q.PublicRunFlightTricks(ctx, gen.PublicRunFlightTricksParams(arg))
	if err != nil {
		return v, err
	}
	marks, err := p.q.PublicRunFlightMarks(ctx, gen.PublicRunFlightMarksParams(arg))
	if err != nil {
		return v, err
	}
	members := map[string][]member{}
	if h.Synchro() {
		if members, err = p.teamMembers(r); err != nil {
			return v, err
		}
	}
	var keys []string
	for _, e := range entries {
		keys = append(keys, e.ListKey)
	}
	base := h.Href() + "/runs/" + strconv.Itoa(n)
	v.Lists, v.Sel = listTabs(visibleLists(keys, h.SeasonOrder), r.URL.Query().Get("list"), base, h.SeasonNames)
	for _, e := range entries {
		if e.ListKey != v.Sel {
			continue
		}
		row := flightRow{Score: e.Score, Scored: e.FlightScore.Valid, DNS: e.DidNotStart,
			P: newParticipant(e.Civlid, e.TeamID, e.PilotName, e.PilotCountry, e.TeamName, members)}
		var prev runCell
		if last := len(v.Rows) - 1; last >= 0 {
			prev = runCell{Rank: v.Rows[last].Rank, Score: v.Rows[last].Score}
		}
		row.Rank = sharedRank(len(v.Rows), prev, e.Score)
		if e.FinalMark.Valid {
			var fm scoring.FinalMark
			if err := json.Unmarshal([]byte(e.FinalMark.String), &fm); err != nil {
				return v, err
			}
			row.Mark = &fm
		}
		for _, t := range tricks {
			if t.FlightID == e.FlightID {
				row.Tricks = append(row.Tricks, t)
			}
		}
		for _, m := range marks {
			if m.FlightID == e.FlightID {
				row.Marks = append(row.Marks, m)
			}
		}
		row.FlightID = e.FlightID
		v.Rows = append(v.Rows, row)
	}
	return v, nil
}

type detailsView struct {
	compHeader
	Pilots []gen.Pilot
	Teams  []participant
	Judges []gen.PublicCompetitionJudgesRow
}

// competitionDetails shows dates, location, website, seasons, pilots or
// teams and judges (spec 06 §4.5 summary).
func (p *results) competitionDetails(w http.ResponseWriter, r *http.Request) {
	h, err := p.loadCompetition(r, "details")
	if err != nil {
		fail(w, r, err)
		return
	}
	ctx := r.Context()
	v := detailsView{compHeader: h}
	if v.Pilots, err = p.q.PublicCompetitionPilots(ctx, h.C.ID); err != nil {
		serverError(w, r, err)
		return
	}
	teams, err := p.q.PublicCompetitionTeams(ctx, h.C.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(teams) > 0 {
		members, err := p.teamMembers(r)
		if err != nil {
			serverError(w, r, err)
			return
		}
		for _, t := range teams {
			v.Teams = append(v.Teams, participant{TeamID: t.ID, Name: t.Name, Members: members[t.ID]})
		}
	}
	if v.Judges, err = p.q.PublicCompetitionJudges(ctx, h.C.ID); err != nil {
		serverError(w, r, err)
		return
	}
	renderPage(w, r, CompetitionDetailsPage(p.compMeta(h, h.C.Name+" – Details", h.Href()+"/details"), v))
}
