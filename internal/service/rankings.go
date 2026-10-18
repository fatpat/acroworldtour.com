package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"awt/internal/domain"
	"awt/internal/ranking"
	"awt/internal/store"
	"awt/internal/store/gen"
)

// people resolves ranking participants (display name, country, gender, AWT
// status of a year) within one transaction, with a cache.
type people struct {
	q      *gen.Queries
	pilots map[int]gen.Pilot
	awt    map[int][]int64
	teams  map[string]string
}

func newPeople(q *gen.Queries) *people {
	return &people{q: q, pilots: map[int]gen.Pilot{}, awt: map[int][]int64{}, teams: map[string]string{}}
}

func (p *people) get(ctx context.Context, civlid sql.NullInt64, team sql.NullString, year int) (ranking.Participant, error) {
	if !civlid.Valid {
		name, ok := p.teams[team.String]
		if !ok {
			t, err := p.q.GetTeam(ctx, team.String)
			if err != nil {
				return ranking.Participant{}, fmt.Errorf("team %s: %w", team.String, err)
			}
			name = t.Name
			p.teams[team.String] = name
		}
		return ranking.Participant{TeamID: team.String, Name: name}, nil
	}
	id := int(civlid.Int64)
	row, ok := p.pilots[id]
	if !ok {
		var err error
		if row, err = p.q.GetPilot(ctx, civlid.Int64); err != nil {
			return ranking.Participant{}, fmt.Errorf("pilot %d: %w", id, err)
		}
		p.pilots[id] = row
		if p.awt[id], err = p.q.ListPilotAwtYears(ctx, civlid.Int64); err != nil {
			return ranking.Participant{}, fmt.Errorf("awt years of pilot %d: %w", id, err)
		}
	}
	return ranking.Participant{
		CivlID: id, Name: row.Name, Country: row.Country, Gender: row.Gender, IsAWT: slices.Contains(p.awt[id], int64(year)),
	}, nil
}

// competitionRanking computes a competition's ranking from its stored
// flight scores (spec 03 §7-8).
func competitionRanking(ctx context.Context, q *gen.Queries, ppl *people, c Competition) (ranking.CompetitionResult, error) {
	runs, err := q.ListRuns(ctx, c.ID)
	if err != nil {
		return ranking.CompetitionResult{}, fmt.Errorf("list runs of %s: %w", c.ID, err)
	}
	inputs := make([]ranking.RunInput, 0, len(runs))
	for _, r := range runs {
		in := ranking.RunInput{Type: string(c.Type), Seasons: c.Seasons, Year: c.Year(), State: r.State}
		parts, err := q.ListRunParticipants(ctx, gen.ListRunParticipantsParams{CompetitionID: c.ID, RunIdx: r.Idx})
		if err != nil {
			return ranking.CompetitionResult{}, fmt.Errorf("list participants of run %d: %w", r.Idx+1, err)
		}
		for _, rp := range parts {
			p, err := ppl.get(ctx, rp.Civlid, rp.TeamID, c.Year())
			if err != nil {
				return ranking.CompetitionResult{}, err
			}
			in.Participants = append(in.Participants, p)
		}
		// stored (first save) order, spec 03 §7 step 1
		flights, err := q.ListFlightsOfRun(ctx, gen.ListFlightsOfRunParams{CompetitionID: c.ID, RunIdx: r.Idx})
		if err != nil {
			return ranking.CompetitionResult{}, fmt.Errorf("list flights of run %d: %w", r.Idx+1, err)
		}
		for _, f := range flights {
			p, err := ppl.get(ctx, f.Civlid, f.TeamID, c.Year())
			if err != nil {
				return ranking.CompetitionResult{}, err
			}
			in.Flights = append(in.Flights, ranking.FlightScore{
				Participant: p, Score: f.Score.Float64, Published: f.Published, SaveSeq: int(f.SaveSeq),
			})
		}
		inputs = append(inputs, in)
	}
	return ranking.Competition(string(c.State), inputs, -1), nil
}

// RebuildRankings recomputes a competition's run and competition rankings
// from its stored flight scores, stores them with the results_final flags,
// then rebuilds the rankings of each of its seasons. It runs inside the
// caller's write transaction (CLAUDE.md "compute on write").
func RebuildRankings(ctx context.Context, q *gen.Queries, compID string) error {
	c, _, err := loadCompetition(ctx, q, compID, true)
	if err != nil {
		return err
	}
	ppl := newPeople(q)
	res, err := competitionRanking(ctx, q, ppl, c)
	if err != nil {
		return fmt.Errorf("rank competition %s: %w", c.Code, err)
	}
	for i, rr := range res.Runs {
		if err := store.SaveRankings(ctx, q, store.ScopeRun, store.RunScopeKey(c.ID, i), rr.Lists); err != nil {
			return err
		}
		err := q.SetRunResultsFinal(ctx, gen.SetRunResultsFinalParams{ResultsFinal: rr.Final, CompetitionID: c.ID, Idx: int64(i)})
		if err != nil {
			return fmt.Errorf("set run %d final flag: %w", i+1, err)
		}
	}
	if err := store.SaveRankings(ctx, q, store.ScopeCompetition, c.ID, res.Lists); err != nil {
		return err
	}
	if err := q.SetCompetitionResultsFinal(ctx, gen.SetCompetitionResultsFinalParams{ResultsFinal: res.Final, ID: c.ID}); err != nil {
		return fmt.Errorf("set competition %s final flag: %w", c.Code, err)
	}
	return rebuildSeasons(ctx, q, ppl, c.Seasons)
}

// rebuildSeasons recomputes and stores the rankings of the given seasons
// over their active competitions in legacy order (by name); only
// published, closed and final competitions count (spec 03 §10).
func rebuildSeasons(ctx context.Context, q *gen.Queries, ppl *people, codes []string) error {
	for _, code := range codes {
		in := ranking.SeasonInput{Code: code, Type: string(domain.TypeSolo)}
		switch s, err := q.GetSeasonByCode(ctx, code); {
		case err == nil:
			in.Country = s.Country.String
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("get season %s: %w", code, err)
		}
		ids, err := q.ListSeasonCompetitionIDs(ctx, code)
		if err != nil {
			return fmt.Errorf("list competitions of season %s: %w", code, err)
		}
		for i, id := range ids {
			c, _, err := loadCompetition(ctx, q, id, false)
			if err != nil {
				return err
			}
			if i == 0 {
				in.Type = string(c.Type)
			}
			sc := ranking.SeasonComp{Code: c.Code, Published: c.Published, Closed: c.State == domain.CompetitionClosed}
			if sc.Published && sc.Closed {
				if sc.Result, err = competitionRanking(ctx, q, ppl, c); err != nil {
					return fmt.Errorf("rank competition %s: %w", c.Code, err)
				}
			}
			in.Competitions = append(in.Competitions, sc)
		}
		if err := store.SaveRankings(ctx, q, store.ScopeSeason, code, ranking.Season(in)); err != nil {
			return fmt.Errorf("season %s: %w", code, err)
		}
	}
	return nil
}
