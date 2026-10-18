package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"awt/internal/ranking"
	"awt/internal/scoring"
	"awt/internal/store"
	"awt/internal/store/gen"
)

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

	pilotsLoaded bool
	counts       map[string]gen.ApiListPublishedCompetitionCountsRow
	seasonCodes  map[string][]string
	seasonCounts map[string]int64
}

// decode unmarshals stored JSON; a failure is logged (the value is left zero).
func decode(what, data string, v any) bool {
	if err := json.Unmarshal([]byte(data), v); err != nil {
		slog.Error("api: stored JSON does not decode", "what", what, "err", err)
		return false
	}
	return true
}

func (s *server) reader(r *http.Request) *reader {
	return &reader{ctx: r.Context(), q: gen.New(s.db.Read), pilots: map[int64]*pilotJSON{}, judges: map[string]*judgeJSON{}}
}

func (rd *reader) pilot(id int64) (*pilotJSON, error) {
	if !rd.pilotsLoaded { // one query for every pilot beats one per ranking row
		rows, err := rd.q.ListPilots(rd.ctx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			rd.pilots[row.Civlid] = newPilot(row)
		}
		rd.pilotsLoaded = true
	}
	if p, ok := rd.pilots[id]; ok {
		return p, nil
	}
	return &pilotJSON{Civlid: id}, nil
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
	perRun := map[string]map[string][]ranking.RunCell{}
	for _, e := range rows {
		p, t, err := rd.participant(e.Civlid, e.TeamID)
		if err != nil {
			return nil, err
		}
		entry := entryJSON{Rank: e.Rank, Position: e.Position, Pilot: p, Team: t, Score: scoring.Round3(e.Score)}
		if scope == store.ScopeCompetition {
			cells, ok := perRun[e.ListKey]
			if !ok {
				if cells, err = rd.publishedRunCells(key, e.ListKey); err != nil {
					return nil, err
				}
				perRun[e.ListKey] = cells
			}
			for _, c := range cells[participantKey(e.Civlid, e.TeamID)] {
				entry.ResultPerRun = append(entry.ResultPerRun, cellJSON{Run: c.RunIndex, Rank: c.Rank, Score: scoring.Round3(c.Score)})
			}
		}
		out[e.ListKey] = append(out[e.ListKey], entry)
	}
	return out, nil
}

// publishedRunCells is every participant's rank and score in each run of
// one competition list, counting published flights only and re-ranked like
// the public overall tab (the stored per-run cells include unpublished
// flights, decision 0005).
func (rd *reader) publishedRunCells(compID, list string) (map[string][]ranking.RunCell, error) {
	places, err := rd.q.PublicCompetitionRunPlaces(rd.ctx, gen.PublicCompetitionRunPlacesParams{CompetitionID: compID, ListKey: list})
	if err != nil {
		return nil, err
	}
	rp := make([]ranking.RunPlace, len(places))
	for i, pl := range places {
		rp[i] = ranking.RunPlace{RunIndex: int(pl.RunIdx), Key: participantKey(pl.Civlid, pl.TeamID), Score: pl.Score}
	}
	return ranking.PublishedRunCells(rp), nil
}

func participantKey(civlid sql.NullInt64, teamID sql.NullString) string {
	if teamID.Valid {
		return "t:" + teamID.String
	}
	return "p:" + strconv.FormatInt(civlid.Int64, 10)
}

// loadMeta loads the counts and season codes of every published competition
// in two queries.
func (rd *reader) loadMeta() error {
	if rd.counts != nil {
		return nil
	}
	counts, err := rd.q.ApiListPublishedCompetitionCounts(rd.ctx)
	if err != nil {
		return err
	}
	codes, err := rd.q.ApiListPublishedCompetitionSeasonCodes(rd.ctx)
	if err != nil {
		return err
	}
	rd.counts = map[string]gen.ApiListPublishedCompetitionCountsRow{}
	for _, c := range counts {
		rd.counts[c.CompetitionID] = c
	}
	rd.seasonCodes = map[string][]string{}
	rd.seasonCounts = map[string]int64{}
	for _, c := range codes {
		rd.seasonCodes[c.CompetitionID] = append(rd.seasonCodes[c.CompetitionID], c.SeasonCode)
		rd.seasonCounts[c.SeasonCode]++
	}
	return nil
}

func (rd *reader) competitionSummary(c gen.Competition) (competitionJSON, error) {
	if err := rd.loadMeta(); err != nil {
		return competitionJSON{}, err
	}
	counts := rd.counts[c.ID]
	return competitionJSON{
		ID: c.ID, Code: c.Code, Name: c.Name, Type: c.Type, State: c.State, StartDate: date(c.StartDate), EndDate: date(c.EndDate),
		Location: c.Location, Website: nullStr(c.Website), Image: fileURL(c.ImageFile), Logo: fileURL(c.LogoFile),
		Seasons: nonNil(rd.seasonCodes[c.ID]), Final: c.ResultsFinal, NumberOfPilots: counts.Pilots, NumberOfTeams: counts.Teams,
		NumberOfJudges: counts.Judges, NumberOfRuns: counts.Runs, LastUpdate: nullStr(c.UpdatedAt),
	}, nil
}
