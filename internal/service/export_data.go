package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"awt/internal/domain"
	"awt/internal/export"
	"awt/internal/ranking"
	"awt/internal/scoring"
	"awt/internal/store/gen"
)

// exporter turns stored participants into export structs, with caches.
type exporter struct {
	q     *gen.Queries
	ppl   *people
	year  int
	teams map[string]*export.ExportTeam
}

func newExporter(q *gen.Queries, year int) *exporter {
	return &exporter{q: q, ppl: newPeople(q), year: year, teams: map[string]*export.ExportTeam{}}
}

func exportPilot(p ranking.Participant) export.ExportPilot {
	return export.ExportPilot{CivlID: p.CivlID, Name: p.Name, Country: p.Country, Gender: p.Gender, IsAWT: p.IsAWT}
}

// participant resolves a stored participant: a pilot or a team with its
// members in stored order.
func (x *exporter) participant(ctx context.Context, civlid sql.NullInt64, team sql.NullString, year int) (*export.ExportPilot, *export.ExportTeam, error) {
	p, err := x.ppl.get(ctx, civlid, team, year)
	if err != nil {
		return nil, nil, err
	}
	if p.TeamID == "" {
		ep := exportPilot(p)
		return &ep, nil, nil
	}
	if t, ok := x.teams[p.TeamID]; ok {
		return nil, t, nil
	}
	t := &export.ExportTeam{Name: p.Name}
	members, err := x.q.ListTeamPilots(ctx, p.TeamID)
	if err != nil {
		return nil, nil, fmt.Errorf("list pilots of team %s: %w", p.TeamID, err)
	}
	for _, m := range members {
		mp, err := x.ppl.get(ctx, sql.NullInt64{Int64: m.Civlid, Valid: true}, sql.NullString{}, year)
		if err != nil {
			return nil, nil, err
		}
		t.Pilots = append(t.Pilots, exportPilot(mp))
	}
	x.teams[p.TeamID] = t
	return nil, t, nil
}

// orderedKeys lists the keys of the lists with "overall" first.
func orderedKeys(keys []string) []string {
	out := slices.Clone(keys)
	slices.SortFunc(out, func(a, b string) int {
		switch {
		case a == b:
			return 0
		case a == export.KeyOverall:
			return -1
		case b == export.KeyOverall:
			return 1
		}
		return strings.Compare(a, b)
	})
	return out
}

func parseStamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}

// storedCompetitionLists reads the stored competition ranking with the per
// run cells of its details.
func storedCompetitionLists(ctx context.Context, q *gen.Queries, compID string) (ranking.Lists, error) {
	rows, err := q.ListCompetitionRankingEntries(ctx, compID)
	if err != nil {
		return nil, fmt.Errorf("read ranking of %s: %w", compID, err)
	}
	lists := ranking.Lists{}
	for _, r := range rows {
		e := ranking.Entry{
			Participant: ranking.Participant{CivlID: int(r.Civlid.Int64), TeamID: r.TeamID.String},
			Score:       r.Score, Position: int(r.Position), Rank: int(r.Rank),
		}
		var d struct {
			PerRun []struct {
				Run   int     `json:"run"`
				Rank  int     `json:"rank"`
				Score float64 `json:"score"`
			} `json:"per_run"`
		}
		if err := json.Unmarshal([]byte(r.Details), &d); err != nil {
			return nil, fmt.Errorf("decode ranking details of %s: %w", compID, err)
		}
		for _, c := range d.PerRun {
			e.PerRun = append(e.PerRun, ranking.RunCell{RunIndex: c.Run, Rank: c.Rank, Score: c.Score})
		}
		lists[r.ListKey] = append(lists[r.ListKey], e)
	}
	return lists, nil
}

// listTitle names a list by its season when the key is a season code.
func (x *exporter) listTitle(ctx context.Context, key string) string {
	if s, err := x.q.GetSeasonByCode(ctx, key); err == nil {
		return s.Name
	}
	return ""
}

// CompetitionExportData builds the export of a competition's results.
// limitRun is the 0-based last run included, -1 for every run: all runs read
// the stored rankings (frozen results are never recomputed), a limit
// recomputes the ranking over the stored flight scores (spec 03 §8).
func (s *Service) CompetitionExportData(ctx context.Context, compID string, limitRun int) (export.CompetitionExport, error) {
	q := s.read()
	c, _, err := loadCompetition(ctx, q, compID, false)
	if err != nil {
		return export.CompetitionExport{}, err
	}
	x := newExporter(q, c.Year())
	out := export.CompetitionExport{
		Name: c.Name, Code: c.Code, Type: string(c.Type), Location: c.Location, StartDate: c.StartDate, EndDate: c.EndDate,
		Seasons: c.Seasons, Final: c.ResultsFinal, LimitRun: limitRun, UpdatedAt: parseStamp(c.UpdatedAt),
	}
	var lists ranking.Lists
	if limitRun < 0 {
		lists, err = storedCompetitionLists(ctx, q, c.ID)
	} else {
		var res ranking.CompetitionResult
		if res, err = competitionRankingUpTo(ctx, q, x.ppl, c, limitRun); err == nil {
			lists, out.Final = res.Lists, res.Final
		}
	}
	if err != nil {
		return out, err
	}
	keys := make([]string, 0, len(lists))
	for k := range lists {
		keys = append(keys, k)
	}
	for _, k := range orderedKeys(keys) {
		l := export.ExportList{Key: k, Title: x.listTitle(ctx, k)}
		for _, e := range lists[k] {
			pilot, team, err := x.participant(ctx, sql.NullInt64{Int64: int64(e.Participant.CivlID), Valid: e.Participant.TeamID == ""},
				sql.NullString{String: e.Participant.TeamID, Valid: e.Participant.TeamID != ""}, c.Year())
			if err != nil {
				return out, err
			}
			ee := export.ExportEntry{Rank: e.Rank, Score: e.Score, Pilot: pilot, Team: team}
			for _, cell := range e.PerRun {
				ee.PerRun = append(ee.PerRun, export.ExportRunCell{Rank: cell.Rank, Score: cell.Score})
			}
			l.Entries = append(l.Entries, ee)
		}
		out.Lists = append(out.Lists, l)
	}
	return out, nil
}

// RunExportData builds the export of one run's results from its published
// flights only (staff exports never leak unpublished marks into a file that
// is handed to the federation).
func (s *Service) RunExportData(ctx context.Context, compID string, idx int) (export.RunExport, error) {
	q := s.read()
	c, _, err := loadCompetition(ctx, q, compID, false)
	if err != nil {
		return export.RunExport{}, err
	}
	run, _, err := loadRun(ctx, q, compID, idx)
	if err != nil {
		return export.RunExport{}, err
	}
	rows, err := q.PublicRunEntries(ctx, gen.PublicRunEntriesParams{CompetitionID: compID, RunIdx: int64(idx)})
	if err != nil {
		return export.RunExport{}, fmt.Errorf("list entries of run %d: %w", idx+1, err)
	}
	x := newExporter(q, c.Year())
	out := export.RunExport{
		CompName: c.Name, CompCode: c.Code, Type: string(c.Type), Seasons: c.Seasons, StartDate: c.StartDate,
		RunIndex: idx, Final: run.ResultsFinal, UpdatedAt: parseStamp(run.UpdatedAt),
	}
	byKey := map[string][]export.RunEntry{}
	for _, r := range rows { // ordered by list, then position
		pilot, team, err := x.participant(ctx, r.Civlid, r.TeamID, c.Year())
		if err != nil {
			return out, err
		}
		e := export.RunEntry{Score: r.Score, Pilot: pilot, Team: team}
		if r.FinalMark.Valid {
			var fm scoring.FinalMark
			if err := json.Unmarshal([]byte(r.FinalMark.String), &fm); err != nil {
				return out, fmt.Errorf("decode final mark of run %d: %w", idx+1, err)
			}
			e.Warnings, e.Warnings2, e.Malus = fm.Warnings1, fm.Warnings2, fm.Malus
			e.Technical, e.Choreo, e.Landing, e.Synchro = fm.Technical, fm.Choreography, fm.Landing, fm.Synchro
			e.Technicity, e.BonusPct = fm.Technicity, fm.BonusPercentage
		}
		byKey[r.ListKey] = append(byKey[r.ListKey], e)
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	for _, k := range orderedKeys(keys) {
		out.Lists = append(out.Lists, export.RunList{Key: k, Title: x.listTitle(ctx, k), Entries: byKey[k]})
	}
	return out, nil
}

// StartingOrderExportData builds the printable starting order of a run: one
// table per AWT or AWQ season of a solo tour event (AWT pilots of the
// season's year, or the others), one overall table otherwise (legacy
// export_starting_order).
func (s *Service) StartingOrderExportData(ctx context.Context, compID string, idx int) (export.StartingOrderExport, error) {
	q := s.read()
	c, _, err := loadCompetition(ctx, q, compID, false)
	if err != nil {
		return export.StartingOrderExport{}, err
	}
	run, _, err := loadRun(ctx, q, compID, idx)
	if err != nil {
		return export.StartingOrderExport{}, err
	}
	x := newExporter(q, c.Year())
	out := export.StartingOrderExport{CompName: c.Name, Type: string(c.Type), RunIndex: idx, UpdatedAt: parseStamp(run.UpdatedAt)}
	all := func(year int) ([]export.StartingEntry, error) {
		var entries []export.StartingEntry
		for _, p := range run.Pilots {
			pilot, _, err := x.participant(ctx, sql.NullInt64{Int64: int64(p), Valid: true}, sql.NullString{}, year)
			if err != nil {
				return nil, err
			}
			entries = append(entries, export.StartingEntry{Pilot: pilot})
		}
		for _, t := range run.Teams {
			_, team, err := x.participant(ctx, sql.NullInt64{}, sql.NullString{String: t, Valid: true}, year)
			if err != nil {
				return nil, err
			}
			entries = append(entries, export.StartingEntry{Team: team})
		}
		return entries, nil
	}
	var tour []string
	for _, code := range c.Seasons {
		if strings.HasPrefix(code, "awt") || strings.HasPrefix(code, "awq") {
			tour = append(tour, code)
		}
	}
	if c.Type != domain.TypeSolo || len(tour) == 0 {
		entries, err := all(c.Year())
		if err != nil {
			return out, err
		}
		out.Lists = []export.StartingList{{Title: "Overall", Entries: entries}}
		return out, nil
	}
	for _, code := range tour {
		season, err := q.GetSeasonByCode(ctx, code)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		} else if err != nil {
			return out, fmt.Errorf("get season %s: %w", code, err)
		}
		entries, err := all(int(season.Year))
		if err != nil {
			return out, err
		}
		wantAWT := strings.HasPrefix(code, "awt")
		l := export.StartingList{Title: season.Name}
		for _, e := range entries {
			if e.Pilot.IsAWT == wantAWT {
				l.Entries = append(l.Entries, e)
			}
		}
		out.Lists = append(out.Lists, l)
	}
	return out, nil
}
