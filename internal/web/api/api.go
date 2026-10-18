// Package api serves the read-only public JSON API under /api/v1. It reads
// stored rows only (competitions must be published and not deleted, runs list
// published flights only) and never exposes gender or internal state.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/store"
	"awt/internal/store/gen"
	"awt/internal/web"
)

// Cache lifetimes: live data (competitions, runs, seasons, one pilot's
// results) is short, reference lists are cached for an hour.
const (
	liveMaxAge = 30 * time.Second
	liveSWR    = 60 * time.Second
	refMaxAge  = time.Hour
	refSWR     = 60 * time.Second

	prefix = "/api/v1"
)

// Module registers the API routes.
type Module struct{}

// Register implements web.Module.
func (Module) Register(r *web.Router) {
	s := &server{db: r.Deps().DB}
	live := func(pattern string, h http.HandlerFunc) {
		r.PublicCached("GET "+prefix+pattern, s.wrap(h), liveMaxAge, liveSWR)
	}
	ref := func(pattern string, h http.HandlerFunc) {
		r.PublicCached("GET "+prefix+pattern, s.wrap(h), refMaxAge, refSWR)
	}
	live("/competitions", s.competitions)
	live("/competitions/{code}", s.competition)
	live("/competitions/{code}/runs/{i}", s.run)
	live("/seasons", s.seasons)
	live("/seasons/{code}", s.season)
	live("/pilots/{civlid}", s.pilot)
	ref("/pilots", s.pilots)
	ref("/teams", s.teams)
	ref("/judges", s.judges)
	ref("/tricks", s.tricks)
	// Everything else under the prefix: CORS preflight, JSON 404 and 405.
	r.Public(prefix+"/", s.wrap(s.fallback))
}

type server struct{ db *store.DB }

var errNotFound = errors.New("not found")

// wrap adds CORS and the JSON content type.
func (s *server) wrap(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("Access-Control-Allow-Origin", "*")
		hd.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		hd.Set("Content-Type", "application/json; charset=utf-8")
		h(w, r)
	})
}

func (s *server) fallback(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.WriteHeader(http.StatusNoContent)
	case http.MethodGet, http.MethodHead:
		fail(w, errNotFound)
	default:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":"method not allowed"}` + "\n"))
	}
}

type badRequest string

func (b badRequest) Error() string { return string(b) }

// fail writes a JSON error. Internal errors are logged, never leaked.
func fail(w http.ResponseWriter, err error) {
	w.Header().Set("Cache-Control", "no-store")
	var br badRequest
	switch {
	case errors.Is(err, errNotFound), errors.Is(err, sql.ErrNoRows):
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}` + "\n"))
	case errors.As(err, &br):
		w.WriteHeader(http.StatusBadRequest)
		b, _ := json.Marshal(map[string]string{"error": string(br)})
		_, _ = w.Write(append(b, '\n'))
	default:
		slog.Error("api", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}` + "\n"))
	}
}

func send(w http.ResponseWriter, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fail(w, err)
		return
	}
	_, _ = w.Write(append(b, '\n'))
}

func sendList[T any](w http.ResponseWriter, items []T) {
	if items == nil {
		items = []T{}
	}
	send(w, map[string]any{"items": items})
}

// ---- JSON shapes ----

type pilotJSON struct {
	Civlid     int64           `json:"civlid"`
	Name       string          `json:"name"`
	Slug       string          `json:"slug"`
	Country    string          `json:"country"`
	CivlLink   string          `json:"civl_link"`
	About      string          `json:"about"`
	Photo      string          `json:"photo"`
	PhotoHigh  string          `json:"photo_highres"`
	Background string          `json:"background_picture"`
	Rank       int64           `json:"rank"`
	Links      json.RawMessage `json:"social_links"`
	Sponsors   json.RawMessage `json:"sponsors"`
	AwtYears   []int64         `json:"awt_years,omitempty"`
}

func newPilot(p gen.Pilot) *pilotJSON {
	return &pilotJSON{
		Civlid: p.Civlid, Name: p.Name, Slug: domain.Slug(p.Name), Country: p.Country, CivlLink: p.CivlLink,
		About: p.About, Photo: p.PhotoUrl, PhotoHigh: p.PhotoHighresUrl, Background: p.BackgroundUrl, Rank: p.Rank,
		Links: raw(p.SocialLinks), Sponsors: raw(p.Sponsors),
	}
}

type teamJSON struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Pilots []*pilotJSON `json:"pilots"`
}

type judgeJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Level   string `json:"level"`
	Civlid  *int64 `json:"civlid,omitempty"`
}

func newJudge(j gen.Judge) *judgeJSON {
	out := &judgeJSON{ID: j.ID, Name: j.Name, Country: j.Country, Level: j.Level}
	if j.Civlid.Valid {
		out.Civlid = &j.Civlid.Int64
	}
	return out
}

type competitionJSON struct {
	ID             string   `json:"id"`
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	State          string   `json:"state"`
	StartDate      string   `json:"start_date"`
	EndDate        string   `json:"end_date"`
	Location       string   `json:"location"`
	Website        string   `json:"website,omitempty"`
	Image          string   `json:"image,omitempty"`
	Logo           string   `json:"logo,omitempty"`
	Seasons        []string `json:"seasons"`
	Final          bool     `json:"final"`
	NumberOfPilots int64    `json:"number_of_pilots"`
	NumberOfTeams  int64    `json:"number_of_teams"`
	NumberOfJudges int64    `json:"number_of_judges"`
	NumberOfRuns   int64    `json:"number_of_runs"`
	LastUpdate     string   `json:"last_update,omitempty"`
}

type competitionDetailJSON struct {
	competitionJSON
	Pilots  []*pilotJSON           `json:"pilots"`
	Teams   []*teamJSON            `json:"teams"`
	Judges  []*judgeJSON           `json:"judges"`
	Runs    []runSummaryJSON       `json:"runs"`
	Results map[string][]entryJSON `json:"results"`
}

type runSummaryJSON struct {
	Index           int64  `json:"index"`
	State           string `json:"state"`
	Final           bool   `json:"final"`
	NumberOfFlights int64  `json:"number_of_flights"`
	LastUpdate      string `json:"last_update,omitempty"`
}

type runDetailJSON struct {
	runSummaryJSON
	Results map[string][]entryJSON `json:"results"`
	Flights []flightJSON           `json:"flights"`
}

type cellJSON struct {
	Run   int     `json:"run"`
	Rank  int     `json:"rank"`
	Score float64 `json:"score"`
}

type entryJSON struct {
	Rank         int64      `json:"rank"`
	Position     int64      `json:"position"`
	Pilot        *pilotJSON `json:"pilot,omitempty"`
	Team         *teamJSON  `json:"team,omitempty"`
	Score        float64    `json:"score"`
	ResultPerRun []cellJSON `json:"result_per_run,omitempty"`
}

type trickJSON struct {
	Name                 string             `json:"name"`
	Acronym              string             `json:"acronym"`
	BaseTrick            string             `json:"base_trick"`
	TechnicalCoefficient float64            `json:"technical_coefficient"`
	Bonus                float64            `json:"bonus"`
	BonusTypes           json.RawMessage    `json:"bonus_types"`
	Uniqueness           json.RawMessage    `json:"uniqueness"`
	Bonuses              json.RawMessage    `json:"bonuses"`
	Solo                 *bool              `json:"solo,omitempty"`
	SoloAwt              *bool              `json:"solo_awt,omitempty"`
	Synchro              *bool              `json:"synchro,omitempty"`
	Types                json.RawMessage    `json:"types"`
	TechnicalMark        *float64           `json:"technical_mark,omitempty"`
	TechnicalMarks       map[string]float64 `json:"technical_marks,omitempty"`
}

type markJSON struct {
	Judge             *judgeJSON `json:"judge"`
	Technical         *float64   `json:"technical,omitempty"`
	TechnicalPerTrick []*float64 `json:"technical_per_trick,omitempty"`
	Choreography      *float64   `json:"choreography,omitempty"`
	Landing           *float64   `json:"landing,omitempty"`
	Synchro           *float64   `json:"synchro,omitempty"`
}

type judgesMarkJSON struct {
	Technical    *float64 `json:"technical,omitempty"`
	Choreography *float64 `json:"choreography,omitempty"`
	Landing      *float64 `json:"landing,omitempty"`
	Synchro      *float64 `json:"synchro,omitempty"`
}

type finalMarkJSON struct {
	JudgesMark      judgesMarkJSON `json:"judges_mark"`
	Technicity      float64        `json:"technicity"`
	BonusPercentage float64        `json:"bonus_percentage"`
	Technical       float64        `json:"technical"`
	Choreography    float64        `json:"choreography"`
	Landing         float64        `json:"landing"`
	Synchro         float64        `json:"synchro"`
	Bonus           float64        `json:"bonus"`
	Score           float64        `json:"score"`
	Malus           float64        `json:"malus"`
	Warnings        []string       `json:"warnings"`
	Warnings2       []string       `json:"warnings2"`
	Notes           []string       `json:"notes"`
	MarkType        string         `json:"mark_type,omitempty"`
}

type flightJSON struct {
	Pilot       *pilotJSON     `json:"pilot,omitempty"`
	Team        *teamJSON      `json:"team,omitempty"`
	DidNotStart bool           `json:"did_not_start"`
	TipTouches  int64          `json:"tip_touches"`
	MarkType    string         `json:"mark_type,omitempty"`
	Score       *float64       `json:"score,omitempty"`
	Tricks      []trickJSON    `json:"tricks"`
	Marks       []markJSON     `json:"marks"`
	FinalMarks  *finalMarkJSON `json:"final_marks,omitempty"`
	Warnings    []string       `json:"warnings"`
	Warnings2   []string       `json:"warnings2"`
	LastUpdate  string         `json:"last_update,omitempty"`
}

type seasonJSON struct {
	ID                   string `json:"id"`
	Code                 string `json:"code"`
	Name                 string `json:"name"`
	Year                 int64  `json:"year"`
	Country              string `json:"country,omitempty"`
	Image                string `json:"image,omitempty"`
	Index                int64  `json:"index"`
	NumberOfCompetitions int64  `json:"number_of_competitions"`
}

type seasonDetailJSON struct {
	seasonJSON
	Competitions []competitionJSON      `json:"competitions"`
	Results      map[string][]entryJSON `json:"results"`
}

type catalogTrickJSON struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Acronym              string          `json:"acronym"`
	Solo                 bool            `json:"solo"`
	SoloAwt              bool            `json:"solo_awt"`
	Synchro              bool            `json:"synchro"`
	Directions           json.RawMessage `json:"directions"`
	TechnicalCoefficient float64         `json:"technical_coefficient"`
	Bonuses              json.RawMessage `json:"bonuses"`
	BonusConstraints     json.RawMessage `json:"bonus_constraints"`
	FirstManeuver        int64           `json:"first_maneuver"`
	NoFirstManeuver      int64           `json:"no_first_maneuver"`
	LastManeuver         int64           `json:"last_maneuver"`
	NoLastManeuver       int64           `json:"no_last_maneuver"`
	Repeatable           bool            `json:"repeatable"`
	Types                json.RawMessage `json:"types"`
	SampleVideo          string          `json:"sample_video,omitempty"`
}

// ---- helpers ----

func raw(s string) json.RawMessage {
	if s == "" {
		return json.RawMessage("null")
	}
	return json.RawMessage(s)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func round(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := scoring.Round3(*p)
	return &v
}

func fileURL(id sql.NullString) string {
	if !id.Valid || id.String == "" {
		return ""
	}
	return "/files/" + id.String
}

func date(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

func nullStr(s sql.NullString) string { return s.String }

// reader resolves referenced pilots, teams and judges once per request.
type reader struct {
	ctx    context.Context
	q      *gen.Queries
	pilots map[int64]*pilotJSON
	teams  map[string]*teamJSON
	judges map[string]*judgeJSON
}

func (s *server) reader(r *http.Request) *reader {
	return &reader{ctx: r.Context(), q: gen.New(s.db.Read), pilots: map[int64]*pilotJSON{}, judges: map[string]*judgeJSON{}}
}

func (rd *reader) pilot(id int64) (*pilotJSON, error) {
	if p, ok := rd.pilots[id]; ok {
		return p, nil
	}
	row, err := rd.q.GetPilot(rd.ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return &pilotJSON{Civlid: id}, nil
	}
	if err != nil {
		return nil, err
	}
	p := newPilot(row)
	rd.pilots[id] = p
	return p, nil
}

// loadTeams loads every team with its pilots, once.
func (rd *reader) loadTeams() error {
	if rd.teams != nil {
		return nil
	}
	teams, err := rd.q.ListTeams(rd.ctx)
	if err != nil {
		return err
	}
	members, err := rd.q.ApiListTeamPilots(rd.ctx)
	if err != nil {
		return err
	}
	rd.teams = map[string]*teamJSON{}
	for _, t := range teams {
		rd.teams[t.ID] = &teamJSON{ID: t.ID, Name: t.Name, Pilots: []*pilotJSON{}}
	}
	for _, m := range members {
		t, ok := rd.teams[m.TeamID]
		if !ok {
			continue
		}
		t.Pilots = append(t.Pilots, newPilot(gen.Pilot{
			Civlid: m.Civlid, Name: m.Name, Country: m.Country, About: m.About, CivlLink: m.CivlLink, PhotoUrl: m.PhotoUrl,
			PhotoHighresUrl: m.PhotoHighresUrl, BackgroundUrl: m.BackgroundUrl, Rank: m.Rank, SocialLinks: m.SocialLinks,
			Sponsors: m.Sponsors,
		}))
	}
	return nil
}

// team resolves a team by id, including soft-deleted ones (history stays visible).
func (rd *reader) team(id string) (*teamJSON, error) {
	if err := rd.loadTeams(); err != nil {
		return nil, err
	}
	if t, ok := rd.teams[id]; ok {
		return t, nil
	}
	row, err := rd.q.GetTeam(rd.ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return &teamJSON{ID: id, Pilots: []*pilotJSON{}}, nil
	}
	if err != nil {
		return nil, err
	}
	t := &teamJSON{ID: row.ID, Name: row.Name, Pilots: []*pilotJSON{}}
	members, err := rd.q.ListTeamPilots(rd.ctx, id)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		p, err := rd.pilot(m.Civlid)
		if err != nil {
			return nil, err
		}
		t.Pilots = append(t.Pilots, p)
	}
	rd.teams[id] = t
	return t, nil
}

func (rd *reader) judge(id string) (*judgeJSON, error) {
	if j, ok := rd.judges[id]; ok {
		return j, nil
	}
	row, err := rd.q.GetJudge(rd.ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return &judgeJSON{ID: id}, nil
	}
	if err != nil {
		return nil, err
	}
	j := newJudge(row)
	rd.judges[id] = j
	return j, nil
}

func (rd *reader) participant(civlid sql.NullInt64, teamID sql.NullString) (*pilotJSON, *teamJSON, error) {
	if teamID.Valid {
		t, err := rd.team(teamID.String)
		return nil, t, err
	}
	if civlid.Valid {
		p, err := rd.pilot(civlid.Int64)
		return p, nil, err
	}
	return nil, nil, nil
}

// lists reads the stored ranking lists of one scope. "overall" is always present.
func (rd *reader) lists(scope, key string) (map[string][]entryJSON, error) {
	rows, err := rd.q.ApiListRankingEntriesOfScope(rd.ctx, gen.ApiListRankingEntriesOfScopeParams{Scope: scope, ScopeKey: key})
	if err != nil {
		return nil, err
	}
	out := map[string][]entryJSON{"overall": {}}
	for _, e := range rows {
		p, t, err := rd.participant(e.Civlid, e.TeamID)
		if err != nil {
			return nil, err
		}
		var d struct {
			PerRun []cellJSON `json:"per_run"`
		}
		_ = json.Unmarshal([]byte(e.Details), &d)
		for i := range d.PerRun {
			d.PerRun[i].Score = scoring.Round3(d.PerRun[i].Score)
		}
		out[e.ListKey] = append(out[e.ListKey], entryJSON{
			Rank: e.Rank, Position: e.Position, Pilot: p, Team: t, Score: scoring.Round3(e.Score), ResultPerRun: d.PerRun,
		})
	}
	return out, nil
}

func (rd *reader) competitionSummary(c gen.Competition) (competitionJSON, error) {
	counts, err := rd.q.ApiCompetitionCounts(rd.ctx, c.ID)
	if err != nil {
		return competitionJSON{}, err
	}
	seasons, err := rd.q.ApiListCompetitionSeasonCodes(rd.ctx, c.ID)
	if err != nil {
		return competitionJSON{}, err
	}
	return competitionJSON{
		ID: c.ID, Code: c.Code, Name: c.Name, Type: c.Type, State: c.State, StartDate: date(c.StartDate), EndDate: date(c.EndDate),
		Location: c.Location, Website: nullStr(c.Website), Image: fileURL(c.ImageFile), Logo: fileURL(c.LogoFile),
		Seasons: nonNil(seasons), Final: c.ResultsFinal, NumberOfPilots: counts.Pilots, NumberOfTeams: counts.Teams,
		NumberOfJudges: counts.Judges, NumberOfRuns: counts.Runs, LastUpdate: nullStr(c.UpdatedAt),
	}, nil
}

// ---- handlers ----

func (s *server) competitions(w http.ResponseWriter, r *http.Request) {
	rd := s.reader(r)
	rows, err := rd.q.ApiListPublishedCompetitions(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]competitionJSON, 0, len(rows))
	for _, c := range rows {
		cj, err := rd.competitionSummary(c)
		if err != nil {
			fail(w, err)
			return
		}
		out = append(out, cj)
	}
	sendList(w, out)
}

func (s *server) publishedCompetition(rd *reader, code string) (gen.Competition, error) {
	c, err := rd.q.ApiGetPublishedCompetitionByCode(rd.ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return c, errNotFound
	}
	return c, err
}

func (s *server) competition(w http.ResponseWriter, r *http.Request) {
	rd := s.reader(r)
	c, err := s.publishedCompetition(rd, r.PathValue("code"))
	if err != nil {
		fail(w, err)
		return
	}
	out, err := s.competitionDetail(rd, c)
	if err != nil {
		fail(w, err)
		return
	}
	send(w, out)
}

func (s *server) competitionDetail(rd *reader, c gen.Competition) (*competitionDetailJSON, error) {
	sum, err := rd.competitionSummary(c)
	if err != nil {
		return nil, err
	}
	out := &competitionDetailJSON{competitionJSON: sum, Pilots: []*pilotJSON{}, Teams: []*teamJSON{}, Judges: []*judgeJSON{}, Runs: []runSummaryJSON{}}
	pilots, err := rd.q.ApiListCompetitionPilotRows(rd.ctx, c.ID)
	if err != nil {
		return nil, err
	}
	for _, p := range pilots {
		out.Pilots = append(out.Pilots, newPilot(p))
	}
	teams, err := rd.q.ApiListCompetitionTeamRows(rd.ctx, c.ID)
	if err != nil {
		return nil, err
	}
	for _, t := range teams {
		tj, err := rd.team(t.ID)
		if err != nil {
			return nil, err
		}
		out.Teams = append(out.Teams, tj)
	}
	judges, err := rd.q.ApiListCompetitionJudgeRows(rd.ctx, c.ID)
	if err != nil {
		return nil, err
	}
	for _, j := range judges {
		out.Judges = append(out.Judges, newJudge(j))
	}
	runs, err := rd.q.ListRuns(rd.ctx, c.ID)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		rs, err := rd.runSummary(c.ID, run)
		if err != nil {
			return nil, err
		}
		out.Runs = append(out.Runs, rs)
	}
	if out.Results, err = rd.lists(store.ScopeCompetition, c.ID); err != nil {
		return nil, err
	}
	return out, nil
}

func (rd *reader) runSummary(compID string, run gen.Run) (runSummaryJSON, error) {
	n, err := rd.q.ApiCountFlightsOfRun(rd.ctx, gen.ApiCountFlightsOfRunParams{CompetitionID: compID, RunIdx: run.Idx})
	if err != nil {
		return runSummaryJSON{}, err
	}
	return runSummaryJSON{Index: run.Idx, State: run.State, Final: run.ResultsFinal, NumberOfFlights: n, LastUpdate: nullStr(run.UpdatedAt)}, nil
}

func (s *server) run(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.ParseInt(r.PathValue("i"), 10, 32)
	if err != nil || idx < 0 {
		fail(w, errNotFound)
		return
	}
	rd := s.reader(r)
	c, err := s.publishedCompetition(rd, r.PathValue("code"))
	if err != nil {
		fail(w, err)
		return
	}
	run, err := rd.q.GetRun(rd.ctx, gen.GetRunParams{CompetitionID: c.ID, Idx: idx})
	if err != nil {
		fail(w, err)
		return
	}
	sum, err := rd.runSummary(c.ID, run)
	if err != nil {
		fail(w, err)
		return
	}
	out := runDetailJSON{runSummaryJSON: sum, Flights: []flightJSON{}}
	if out.Results, err = rd.lists(store.ScopeRun, store.RunScopeKey(c.ID, int(idx))); err != nil {
		fail(w, err)
		return
	}
	flights, err := rd.q.ApiListPublishedFlightsOfRun(rd.ctx, gen.ApiListPublishedFlightsOfRunParams{CompetitionID: c.ID, RunIdx: idx})
	if err != nil {
		fail(w, err)
		return
	}
	for _, f := range flights {
		fj, err := rd.flight(f)
		if err != nil {
			fail(w, err)
			return
		}
		out.Flights = append(out.Flights, fj)
	}
	send(w, out)
}

func (rd *reader) flight(f gen.Flight) (flightJSON, error) {
	out := flightJSON{DidNotStart: f.DidNotStart, TipTouches: f.TipTouches, MarkType: nullStr(f.MarkType),
		Tricks: []trickJSON{}, Marks: []markJSON{}, Warnings: []string{}, Warnings2: []string{}, LastUpdate: nullStr(f.UpdatedAt)}
	if f.Score.Valid {
		v := scoring.Round3(f.Score.Float64)
		out.Score = &v
	}
	var err error
	if out.Pilot, out.Team, err = rd.participant(f.Civlid, f.TeamID); err != nil {
		return out, err
	}
	tricks, err := rd.q.ListFlightTricks(rd.ctx, f.ID)
	if err != nil {
		return out, err
	}
	for _, t := range tricks {
		tj := trickJSON{Name: t.Name, Acronym: t.Acronym, BaseTrick: t.BaseTrick, TechnicalCoefficient: t.TechnicalCoefficient,
			Bonus: t.Bonus, BonusTypes: raw(t.BonusTypes), Uniqueness: raw(t.Uniqueness), Bonuses: raw(t.Bonuses), Types: raw(t.Types)}
		if t.Solo.Valid {
			tj.Solo = &t.Solo.Bool
		}
		if t.SoloAwt.Valid {
			tj.SoloAwt = &t.SoloAwt.Bool
		}
		if t.Synchro.Valid {
			tj.Synchro = &t.Synchro.Bool
		}
		if t.TechnicalMark.Valid {
			tj.TechnicalMark = round(&t.TechnicalMark.Float64)
		}
		if t.TechnicalMarks.Valid {
			var m map[string]float64
			if json.Unmarshal([]byte(t.TechnicalMarks.String), &m) == nil {
				for k, v := range m {
					m[k] = scoring.Round3(v)
				}
				tj.TechnicalMarks = m
			}
		}
		out.Tricks = append(out.Tricks, tj)
	}
	marks, err := rd.q.ListFlightMarks(rd.ctx, f.ID)
	if err != nil {
		return out, err
	}
	for _, m := range marks {
		j, err := rd.judge(m.JudgeID)
		if err != nil {
			return out, err
		}
		mj := markJSON{Judge: j, Technical: nullFloat(m.Technical), Choreography: nullFloat(m.Choreography),
			Landing: nullFloat(m.Landing), Synchro: nullFloat(m.Synchro)}
		if m.TechnicalPerTrick.Valid {
			var per []*float64
			if json.Unmarshal([]byte(m.TechnicalPerTrick.String), &per) == nil {
				for i := range per {
					per[i] = round(per[i])
				}
				mj.TechnicalPerTrick = per
			}
		}
		out.Marks = append(out.Marks, mj)
	}
	warns, err := rd.q.ListFlightWarnings(rd.ctx, f.ID)
	if err != nil {
		return out, err
	}
	for _, wn := range warns {
		if wn.Category == 2 {
			out.Warnings2 = append(out.Warnings2, wn.Text)
		} else {
			out.Warnings = append(out.Warnings, wn.Text)
		}
	}
	if f.FinalMark.Valid {
		var fm scoring.FinalMark
		if err := json.Unmarshal([]byte(f.FinalMark.String), &fm); err == nil {
			out.FinalMarks = &finalMarkJSON{
				JudgesMark: judgesMarkJSON{Technical: round(fm.JudgesMark.Technical), Choreography: round(fm.JudgesMark.Choreography),
					Landing: round(fm.JudgesMark.Landing), Synchro: round(fm.JudgesMark.Synchro)},
				Technicity: scoring.Round3(fm.Technicity), BonusPercentage: scoring.Round3(fm.BonusPercentage),
				Technical: scoring.Round3(fm.Technical), Choreography: scoring.Round3(fm.Choreography),
				Landing: scoring.Round3(fm.Landing), Synchro: scoring.Round3(fm.Synchro), Bonus: scoring.Round3(fm.Bonus),
				Score: scoring.Round3(fm.Score), Malus: scoring.Round3(fm.Malus),
				Warnings: nonNil(fm.Warnings1), Warnings2: nonNil(fm.Warnings2), Notes: nonNil(fm.Notes), MarkType: string(fm.MarkType),
			}
		}
	}
	return out, nil
}

func nullFloat(f sql.NullFloat64) *float64 {
	if !f.Valid {
		return nil
	}
	return round(&f.Float64)
}

func newSeason(rd *reader, s gen.Season) (seasonJSON, error) {
	n, err := rd.q.ApiCountPublishedCompetitionsOfSeason(rd.ctx, s.Code)
	if err != nil {
		return seasonJSON{}, err
	}
	return seasonJSON{ID: s.ID, Code: s.Code, Name: s.Name, Year: s.Year, Country: nullStr(s.Country), Image: fileURL(s.ImageFile),
		Index: s.SortIndex, NumberOfCompetitions: n}, nil
}

func (s *server) seasons(w http.ResponseWriter, r *http.Request) {
	rd := s.reader(r)
	rows, err := rd.q.ListSeasons(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]seasonJSON, 0, len(rows))
	for _, row := range rows {
		sj, err := newSeason(rd, row)
		if err != nil {
			fail(w, err)
			return
		}
		out = append(out, sj)
	}
	sendList(w, out)
}

func (s *server) season(w http.ResponseWriter, r *http.Request) {
	rd := s.reader(r)
	row, err := rd.q.GetSeasonByCode(r.Context(), r.PathValue("code"))
	if err != nil {
		fail(w, err)
		return
	}
	sj, err := newSeason(rd, row)
	if err != nil {
		fail(w, err)
		return
	}
	out := seasonDetailJSON{seasonJSON: sj, Competitions: []competitionJSON{}}
	comps, err := rd.q.ApiListPublishedCompetitionsOfSeason(rd.ctx, row.Code)
	if err != nil {
		fail(w, err)
		return
	}
	for _, c := range comps {
		cj, err := rd.competitionSummary(c)
		if err != nil {
			fail(w, err)
			return
		}
		out.Competitions = append(out.Competitions, cj)
	}
	if out.Results, err = rd.lists(store.ScopeSeason, row.Code); err != nil {
		fail(w, err)
		return
	}
	send(w, out)
}

func (s *server) pilots(w http.ResponseWriter, r *http.Request) {
	rows, err := gen.New(s.db.Read).ListPilots(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]*pilotJSON, 0, len(rows))
	for _, p := range rows {
		out = append(out, newPilot(p))
	}
	sendList(w, out)
}

type pilotDetailJSON struct {
	*pilotJSON
	CompetitionsResults []pilotCompetitionResult `json:"competitions_results"`
	SeasonsResults      []pilotSeasonResult      `json:"seasons_results"`
}

type pilotCompetitionResult struct {
	Competition competitionJSON `json:"competition"`
	Rank        int64           `json:"rank"`
	Score       float64         `json:"score"`
}

type pilotSeasonResult struct {
	Season seasonJSON `json:"season"`
	Rank   int64      `json:"rank"`
	Score  float64    `json:"score"`
}

func (s *server) pilot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("civlid"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, errNotFound)
		return
	}
	rd := s.reader(r)
	row, err := rd.q.GetPilot(rd.ctx, id)
	if err != nil {
		fail(w, err)
		return
	}
	pj := newPilot(row)
	if pj.AwtYears, err = rd.q.ListPilotAwtYears(rd.ctx, id); err != nil {
		fail(w, err)
		return
	}
	out := pilotDetailJSON{pilotJSON: pj, CompetitionsResults: []pilotCompetitionResult{}, SeasonsResults: []pilotSeasonResult{}}
	ranks, err := rd.q.ApiListPilotCompetitionRanks(rd.ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		fail(w, err)
		return
	}
	for _, rk := range ranks {
		c, err := rd.q.ApiGetPublishedCompetitionByCode(rd.ctx, rk.CompetitionCode)
		if err != nil {
			fail(w, err)
			return
		}
		cj, err := rd.competitionSummary(c)
		if err != nil {
			fail(w, err)
			return
		}
		out.CompetitionsResults = append(out.CompetitionsResults, pilotCompetitionResult{Competition: cj, Rank: rk.FinalRank, Score: scoring.Round3(rk.Score)})
	}
	sranks, err := rd.q.ApiListPilotSeasonRanks(rd.ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		fail(w, err)
		return
	}
	for _, rk := range sranks {
		se, err := rd.q.GetSeasonByCode(rd.ctx, rk.SeasonCode)
		if err != nil {
			fail(w, err)
			return
		}
		sj, err := newSeason(rd, se)
		if err != nil {
			fail(w, err)
			return
		}
		out.SeasonsResults = append(out.SeasonsResults, pilotSeasonResult{Season: sj, Rank: rk.FinalRank, Score: scoring.Round3(rk.Score)})
	}
	send(w, out)
}

func (s *server) teams(w http.ResponseWriter, r *http.Request) {
	rd := s.reader(r)
	rows, err := rd.q.ListTeams(rd.ctx)
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]*teamJSON, 0, len(rows))
	for _, t := range rows {
		tj, err := rd.team(t.ID)
		if err != nil {
			fail(w, err)
			return
		}
		out = append(out, tj)
	}
	sendList(w, out)
}

func (s *server) judges(w http.ResponseWriter, r *http.Request) {
	rows, err := gen.New(s.db.Read).ListJudges(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]*judgeJSON, 0, len(rows))
	for _, j := range rows {
		out = append(out, newJudge(j))
	}
	sendList(w, out)
}

func (s *server) tricks(w http.ResponseWriter, r *http.Request) {
	year := time.Now().Year()
	if v := strings.TrimSpace(r.URL.Query().Get("year")); v != "" {
		y, err := strconv.Atoi(v)
		if err != nil {
			fail(w, badRequest("invalid year"))
			return
		}
		year = y
	}
	rows, err := gen.New(s.db.Read).ListTricksByYear(r.Context(), int64(year))
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]catalogTrickJSON, 0, len(rows))
	for _, t := range rows {
		out = append(out, catalogTrickJSON{ID: t.ID, Name: t.Name, Acronym: t.Acronym, Solo: t.Solo, SoloAwt: t.SoloAwt, Synchro: t.Synchro,
			Directions: raw(t.Directions), TechnicalCoefficient: t.TechnicalCoefficient, Bonuses: raw(t.Bonuses),
			BonusConstraints: raw(t.BonusConstraints), FirstManeuver: t.FirstManeuver, NoFirstManeuver: t.NoFirstManeuver,
			LastManeuver: t.LastManeuver, NoLastManeuver: t.NoLastManeuver, Repeatable: t.Repeatable, Types: raw(t.Types),
			SampleVideo: nullStr(t.SampleVideo)})
	}
	sendList(w, out)
}
