package ranking

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// goldenCorpus loads every ranking file and turns the competitions into
// ranking inputs. Solo participants get a CIVL id per corpus id (shared
// across files, so seasons see the same pilot) and a display name built
// from name_order, which stands in for the alphabetical name order.
type goldenCorpus struct {
	comps   map[string]GoldenCompetition
	seasons []GoldenSeason
	civl    map[string]int // corpus id -> CIVL id
	corpus  map[int]string // CIVL id -> corpus id
}

func loadGoldenCorpus(t *testing.T) *goldenCorpus {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "golden", "rankings", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no golden ranking files")
	}
	c := &goldenCorpus{comps: map[string]GoldenCompetition{}, civl: map[string]int{}, corpus: map[int]string{}}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		base := filepath.Base(path)
		switch {
		case strings.HasPrefix(base, "competition-"):
			var g GoldenCompetition
			if err := json.Unmarshal(raw, &g); err != nil {
				t.Fatalf("%s: %v", base, err)
			}
			c.comps[g.Code] = g
		case strings.HasPrefix(base, "season-"):
			var g GoldenSeason
			if err := json.Unmarshal(raw, &g); err != nil {
				t.Fatalf("%s: %v", base, err)
			}
			c.seasons = append(c.seasons, g)
		}
	}
	return c
}

func (c *goldenCorpus) participant(typ string, g GoldenParticipant) Participant {
	p := Participant{Name: fmt.Sprintf("%08d", g.NameOrder), Country: g.Country, Gender: g.Gender, IsAWT: g.AWT}
	if typ == "synchro" {
		p.TeamID = g.ID
		return p
	}
	id, ok := c.civl[g.ID]
	if !ok {
		id = len(c.civl) + 1
		c.civl[g.ID] = id
		c.corpus[id] = g.ID
	}
	p.CivlID = id
	return p
}

// id is the corpus id of a participant.
func (c *goldenCorpus) id(p Participant) string {
	if p.TeamID != "" {
		return p.TeamID
	}
	return c.corpus[p.CivlID]
}

func (c *goldenCorpus) competition(t *testing.T, g GoldenCompetition) CompetitionResult {
	t.Helper()
	in := g.Input
	byID := map[string]Participant{}
	for _, gp := range in.Participants {
		byID[gp.ID] = c.participant(in.Type, gp)
	}
	runs := make([]RunInput, len(in.Runs))
	for i, r := range in.Runs {
		runs[i] = RunInput{Type: in.Type, Seasons: in.Seasons, Year: in.Year, State: r.State}
		for _, f := range r.Flights {
			p, ok := byID[f.Participant]
			if !ok {
				t.Fatalf("%s: run %d: unknown participant %s", g.Code, i, f.Participant)
			}
			runs[i].Flights = append(runs[i].Flights, FlightScore{Participant: p, Score: f.Score, Published: f.Published})
		}
	}
	return Competition(in.State, runs, -1)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// compareList checks a ranking list against legacy: same participants with
// the same scores, and the same score at every position, which fixes the
// position of every group of equal scores (the order inside a group follows
// decision 0007 and may differ from legacy).
func compareList(t *testing.T, where string, c *goldenCorpus, got []Entry, want []GoldenScoreEntry, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: %d entries, want %d", where, len(got), len(want))
		return
	}
	gotScores := map[string]float64{}
	for i, e := range got {
		gotScores[c.id(e.Participant)] = e.Score
		if math.Abs(e.Score-want[i].Score) > tol {
			t.Errorf("%s: position %d score %v, want %v", where, i+1, e.Score, want[i].Score)
		}
	}
	for _, w := range want {
		s, ok := gotScores[w.Participant]
		if !ok {
			t.Errorf("%s: %s missing", where, w.Participant)
		} else if math.Abs(s-w.Score) > tol {
			t.Errorf("%s: %s score %v, want %v", where, w.Participant, s, w.Score)
		}
	}
}

func TestGoldenRankings(t *testing.T) {
	c := loadGoldenCorpus(t)
	results := map[string]CompetitionResult{}

	for _, code := range sortedKeys(c.comps) {
		g := c.comps[code]
		got := c.competition(t, g)
		results[code] = got
		exp := g.Expected
		where := "competition " + code

		if got.Final != exp.Final {
			t.Errorf("%s: final %v, want %v", where, got.Final, exp.Final)
		}
		if len(g.Input.Runs) == 0 {
			// Without runs Competition knows neither the type nor the
			// seasons: only "overall" exists. Legacy also lists empty
			// season lists and an empty "women" list (Pilot.getall([])
			// returns every pilot, so its women count is the database's).
			for key, want := range exp.Results {
				if len(want) != 0 || len(got.Lists[key]) != 0 {
					t.Errorf("%s: no runs but list %s not empty", where, key)
				}
			}
			continue
		}
		if gk, wk := sortedKeys(got.Lists), sortedKeys(exp.Results); strings.Join(gk, ",") != strings.Join(wk, ",") {
			t.Errorf("%s: lists %v, want %v", where, gk, wk)
		}
		for key, want := range exp.Results {
			list := got.Lists[key]
			plain := make([]GoldenScoreEntry, len(want))
			for i, w := range want {
				plain[i] = GoldenScoreEntry{Participant: w.Participant, Score: w.Score}
			}
			compareList(t, where+" "+key, c, list, plain, 0)

			// per-run cell scores, in run order (legacy per-run ranks are
			// buggy, spec 03 §14 #3-4: not compared)
			cells := map[string][]float64{}
			for _, e := range list {
				for _, cell := range e.PerRun {
					cells[c.id(e.Participant)] = append(cells[c.id(e.Participant)], cell.Score)
				}
			}
			for _, w := range want {
				var ws []float64
				for _, r := range w.ResultPerRun {
					ws = append(ws, r.Score)
				}
				if fmt.Sprint(cells[w.Participant]) != fmt.Sprint(ws) {
					t.Errorf("%s %s: %s per-run scores %v, want %v", where, key, w.Participant, cells[w.Participant], ws)
				}
			}
		}

		if len(got.Runs) != len(exp.RunsResults) {
			t.Errorf("%s: %d runs, want %d", where, len(got.Runs), len(exp.RunsResults))
			continue
		}
		for i, wr := range exp.RunsResults {
			gr := got.Runs[i]
			rw := fmt.Sprintf("%s run %d", where, i)
			if gr.Final != wr.Final {
				t.Errorf("%s: final %v, want %v", rw, gr.Final, wr.Final)
			}
			if gk, wk := sortedKeys(gr.Lists), sortedKeys(wr.Results); strings.Join(gk, ",") != strings.Join(wk, ",") {
				t.Errorf("%s: lists %v, want %v", rw, gk, wk)
			}
			for key, want := range wr.Results {
				compareList(t, rw+" "+key, c, gr.Lists[key], want, 0)
			}
		}
	}

	for _, g := range c.seasons {
		where := "season " + g.Code
		if g.Expected == nil {
			t.Errorf("%s: legacy error %q, not handled", where, g.Error)
			continue
		}
		in := SeasonInput{Code: g.Input.Code, Type: g.Expected.Type}
		if g.Input.Country != nil {
			in.Country = *g.Input.Country
		}
		for _, code := range g.Input.Competitions {
			gc, ok := c.comps[code]
			if !ok {
				t.Fatalf("%s: unknown competition %s", where, code)
			}
			in.Competitions = append(in.Competitions, SeasonComp{
				Code:      code,
				Published: gc.Input.Published,
				Closed:    gc.Input.State == "closed",
				Result:    results[code],
			})
		}
		got := Season(in)
		if gk, wk := sortedKeys(got), sortedKeys(g.Expected.Results); strings.Join(gk, ",") != strings.Join(wk, ",") {
			t.Errorf("%s: lists %v, want %v", where, gk, wk)
		}
		for key, want := range g.Expected.Results {
			compareList(t, where+" "+key, c, got[key], want, 1e-9)
		}
	}
}
