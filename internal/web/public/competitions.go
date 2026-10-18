package public

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"

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
		years = append(years, year(c.StartDate), year(c.EndDate))
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
	entries, err := p.q.PublicRankingEntries(r.Context(), gen.PublicRankingEntriesParams{Scope: store.ScopeCompetition, ScopeKey: h.C.ID})
	if err != nil {
		serverError(w, r, err)
		return
	}
	members := map[string][]member{}
	if h.Synchro() {
		if members, err = p.teamMembers(r); err != nil {
			serverError(w, r, err)
			return
		}
	}
	var keys []string
	for _, e := range entries {
		keys = append(keys, e.ListKey)
	}
	v := overallView{compHeader: h}
	var sel string
	v.Lists, sel = listTabs(visibleLists(keys, h.SeasonOrder), r.URL.Query().Get("list"), h.Href(), h.SeasonNames)
	for _, e := range entries {
		if e.ListKey != sel {
			continue
		}
		row := rankRow{Rank: e.Rank, Score: e.Score, P: newParticipant(e.Civlid, e.TeamID, e.PilotName, e.PilotCountry, e.TeamName, members),
			PerRun: map[int]runCell{}}
		var d struct {
			PerRun []struct {
				Run   int     `json:"run"`
				Rank  int64   `json:"rank"`
				Score float64 `json:"score"`
			} `json:"per_run"`
		}
		if err := json.Unmarshal([]byte(e.Details), &d); err != nil {
			serverError(w, r, err)
			return
		}
		for _, c := range d.PerRun {
			row.PerRun[c.Run] = runCell{Rank: c.Rank, Score: c.Score}
		}
		v.Rows = append(v.Rows, row)
	}
	renderPage(w, r, CompetitionOverallPage(p.compMeta(h, h.C.Name, h.Href()), v))
}

// flightRow is a published flight in a run list, with its detail.
type flightRow struct {
	Rank   int64
	P      participant
	Score  float64
	Scored bool
	DNS    bool
	Mark   *scoring.FinalMark
	Tricks []gen.PublicRunFlightTricksRow
	Marks  []gen.PublicRunFlightMarksRow
}

func (f flightRow) Anchor() string { return "flight-" + slugOr(f.P.key()) }

type runView struct {
	compHeader
	N     int
	Run   gen.Run
	Lists []tab
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
	ctx := r.Context()
	run := h.Runs[n-1]
	arg := gen.PublicRunEntriesParams{CompetitionID: h.C.ID, RunIdx: run.Idx}
	entries, err := p.q.PublicRunEntries(ctx, arg)
	if err != nil {
		serverError(w, r, err)
		return
	}
	tricks, err := p.q.PublicRunFlightTricks(ctx, gen.PublicRunFlightTricksParams(arg))
	if err != nil {
		serverError(w, r, err)
		return
	}
	marks, err := p.q.PublicRunFlightMarks(ctx, gen.PublicRunFlightMarksParams(arg))
	if err != nil {
		serverError(w, r, err)
		return
	}
	members := map[string][]member{}
	if h.Synchro() {
		if members, err = p.teamMembers(r); err != nil {
			serverError(w, r, err)
			return
		}
	}
	var keys []string
	for _, e := range entries {
		keys = append(keys, e.ListKey)
	}
	v := runView{compHeader: h, N: n, Run: run}
	base := h.Href() + "/runs/" + strconv.Itoa(n)
	var sel string
	v.Lists, sel = listTabs(visibleLists(keys, h.SeasonOrder), r.URL.Query().Get("list"), base, h.SeasonNames)
	for _, e := range entries {
		if e.ListKey != sel {
			continue
		}
		row := flightRow{Score: e.Score, Scored: e.FlightScore.Valid, DNS: e.DidNotStart,
			P: newParticipant(e.Civlid, e.TeamID, e.PilotName, e.PilotCountry, e.TeamName, members)}
		// shared rank for equal Round3 scores (decision 0007)
		row.Rank = int64(len(v.Rows) + 1)
		if last := len(v.Rows) - 1; last >= 0 && scoring.Round3(v.Rows[last].Score) == scoring.Round3(e.Score) {
			row.Rank = v.Rows[last].Rank
		}
		if e.FinalMark.Valid {
			var fm scoring.FinalMark
			if err := json.Unmarshal([]byte(e.FinalMark.String), &fm); err != nil {
				serverError(w, r, err)
				return
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
		v.Rows = append(v.Rows, row)
	}
	title := h.C.Name + " – Run " + strconv.Itoa(n)
	renderPage(w, r, CompetitionRunPage(p.compMeta(h, title, base), v))
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
