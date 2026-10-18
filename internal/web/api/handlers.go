package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"awt/internal/scoring"
	"awt/internal/store"
	"awt/internal/store/gen"
)

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
	flights, err := rd.flightCounts(c.ID)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		out.Runs = append(out.Runs, runSummary(run, flights[run.Idx]))
	}
	if out.Results, err = rd.lists(store.ScopeCompetition, c.ID); err != nil {
		return nil, err
	}
	return out, nil
}

func (rd *reader) flightCounts(compID string) (map[int64]int64, error) {
	rows, err := rd.q.ApiCountPublishedFlightsPerRun(rd.ctx, compID)
	if err != nil {
		return nil, err
	}
	out := map[int64]int64{}
	for _, r := range rows {
		out[r.RunIdx] = r.Flights
	}
	return out, nil
}

func runSummary(run gen.Run, flights int64) runSummaryJSON {
	return runSummaryJSON{Index: run.Idx, State: run.State, Final: run.ResultsFinal, NumberOfFlights: flights, LastUpdate: nullStr(run.UpdatedAt)}
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
	counts, err := rd.flightCounts(c.ID)
	if err != nil {
		fail(w, err)
		return
	}
	out := runDetailJSON{runSummaryJSON: runSummary(run, counts[idx]), Flights: []flightJSON{}}
	flights, err := rd.q.ApiListPublishedFlightsOfRun(rd.ctx, gen.ApiListPublishedFlightsOfRunParams{CompetitionID: c.ID, RunIdx: idx})
	if err != nil {
		fail(w, err)
		return
	}
	if out.Results, err = rd.lists(store.ScopeRun, store.RunScopeKey(c.ID, int(idx))); err != nil {
		fail(w, err)
		return
	}
	out.Results = publishedOnly(out.Results, flights)
	ch, err := rd.loadChildren(c.ID, idx)
	if err != nil {
		fail(w, err)
		return
	}
	for _, f := range flights {
		fj, err := rd.flight(f, ch)
		if err != nil {
			fail(w, err)
			return
		}
		out.Flights = append(out.Flights, fj)
	}
	send(w, out)
}

// publishedOnly keeps the ranking entries of participants with a published
// flight in the run and renumbers position and rank (equal Round3 scores
// share a rank, as in internal/ranking). Competition and season totals are
// not filtered (decision 0005).
func publishedOnly(lists map[string][]entryJSON, flights []gen.Flight) map[string][]entryJSON {
	pilots := map[int64]bool{}
	teams := map[string]bool{}
	for _, f := range flights {
		if f.TeamID.Valid {
			teams[f.TeamID.String] = true
		} else if f.Civlid.Valid {
			pilots[f.Civlid.Int64] = true
		}
	}
	out := make(map[string][]entryJSON, len(lists))
	for name, entries := range lists {
		kept := []entryJSON{}
		for _, e := range entries {
			if (e.Pilot != nil && pilots[e.Pilot.Civlid]) || (e.Team != nil && teams[e.Team.ID]) {
				kept = append(kept, e)
			}
		}
		for i := range kept {
			kept[i].Position = int64(i + 1)
			kept[i].Rank = int64(i + 1)
			if i > 0 && kept[i].Score == kept[i-1].Score {
				kept[i].Rank = kept[i-1].Rank
			}
		}
		out[name] = kept
	}
	return out
}

// children holds the tricks, marks and warnings of a run's published flights,
// grouped by flight id.
type children struct {
	tricks map[int64][]gen.FlightTrick
	marks  map[int64][]gen.FlightMark
	warns  map[int64][]gen.FlightWarning
}

func (rd *reader) loadChildren(compID string, idx int64) (*children, error) {
	ch := &children{tricks: map[int64][]gen.FlightTrick{}, marks: map[int64][]gen.FlightMark{}, warns: map[int64][]gen.FlightWarning{}}
	tricks, err := rd.q.ApiListTricksOfRun(rd.ctx, gen.ApiListTricksOfRunParams{CompetitionID: compID, RunIdx: idx})
	if err != nil {
		return nil, err
	}
	for _, t := range tricks {
		ch.tricks[t.FlightID] = append(ch.tricks[t.FlightID], t)
	}
	marks, err := rd.q.ApiListMarksOfRun(rd.ctx, gen.ApiListMarksOfRunParams{CompetitionID: compID, RunIdx: idx})
	if err != nil {
		return nil, err
	}
	for _, m := range marks {
		ch.marks[m.FlightID] = append(ch.marks[m.FlightID], m)
	}
	warns, err := rd.q.ApiListWarningsOfRun(rd.ctx, gen.ApiListWarningsOfRunParams{CompetitionID: compID, RunIdx: idx})
	if err != nil {
		return nil, err
	}
	for _, wn := range warns {
		ch.warns[wn.FlightID] = append(ch.warns[wn.FlightID], wn)
	}
	return ch, nil
}

func (rd *reader) flight(f gen.Flight, ch *children) (flightJSON, error) {
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
	for _, t := range ch.tricks[f.ID] {
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
			if decode("technical_marks", t.TechnicalMarks.String, &m) {
				for k, v := range m {
					m[k] = scoring.Round3(v)
				}
				tj.TechnicalMarks = m
			}
		}
		out.Tricks = append(out.Tricks, tj)
	}
	for _, m := range ch.marks[f.ID] {
		j, err := rd.judge(m.JudgeID)
		if err != nil {
			return out, err
		}
		mj := markJSON{Judge: j, Technical: nullFloat(m.Technical), Choreography: nullFloat(m.Choreography),
			Landing: nullFloat(m.Landing), Synchro: nullFloat(m.Synchro)}
		if m.TechnicalPerTrick.Valid {
			var per []*float64
			if decode("technical_per_trick", m.TechnicalPerTrick.String, &per) {
				for i := range per {
					per[i] = round(per[i])
				}
				mj.TechnicalPerTrick = per
			}
		}
		out.Marks = append(out.Marks, mj)
	}
	for _, wn := range ch.warns[f.ID] {
		if wn.Category == 2 {
			out.Warnings2 = append(out.Warnings2, wn.Text)
		} else {
			out.Warnings = append(out.Warnings, wn.Text)
		}
	}
	if f.FinalMark.Valid {
		var fm scoring.FinalMark
		if decode("final_mark", f.FinalMark.String, &fm) {
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
	if err := rd.loadMeta(); err != nil {
		return seasonJSON{}, err
	}
	return seasonJSON{ID: s.ID, Code: s.Code, Name: s.Name, Year: s.Year, Country: nullStr(s.Country), Image: fileURL(s.ImageFile),
		Index: s.SortIndex, NumberOfCompetitions: rd.seasonCounts[s.Code]}, nil
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
	published, err := rd.q.ApiListPublishedCompetitions(rd.ctx)
	if err != nil {
		fail(w, err)
		return
	}
	byCode := map[string]gen.Competition{}
	for _, c := range published {
		byCode[c.Code] = c
	}
	for _, rk := range ranks {
		c, ok := byCode[rk.CompetitionCode]
		if !ok {
			continue
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
	seasons, err := rd.q.ListSeasons(rd.ctx)
	if err != nil {
		fail(w, err)
		return
	}
	bySeason := map[string]gen.Season{}
	for _, se := range seasons {
		bySeason[se.Code] = se
	}
	for _, rk := range sranks {
		se, ok := bySeason[rk.SeasonCode]
		if !ok {
			continue
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
