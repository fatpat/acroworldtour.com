package ranking

import (
	"reflect"
	"testing"
)

func pilot(id int, name string) Participant {
	return Participant{CivlID: id, Name: name, Gender: "man"}
}

func fs(p Participant, score float64) FlightScore {
	return FlightScore{Participant: p, Score: score, Published: true}
}

// ids returns the participant ids of a list (CIVL id, or -1 for teams).
func ids(list []Entry) []int {
	out := make([]int, len(list))
	for i, e := range list {
		out[i] = e.Participant.CivlID
	}
	return out
}

func ranks(list []Entry) []int {
	out := make([]int, len(list))
	for i, e := range list {
		out[i] = e.Rank
	}
	return out
}

func positions(list []Entry) []int {
	out := make([]int, len(list))
	for i, e := range list {
		out[i] = e.Position
	}
	return out
}

func scores(list []Entry) []float64 {
	out := make([]float64, len(list))
	for i, e := range list {
		out[i] = e.Score
	}
	return out
}

func eq[T any](t *testing.T, what string, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

// Decision 0007: (score desc, display name asc, id asc), shared rank for
// equal Round3 scores.
func TestTiesShareRank(t *testing.T) {
	a, b, c, d := pilot(1, "Dora"), pilot(2, "Bob"), pilot(3, "Alice"), pilot(4, "Carl")
	in := RunInput{Type: "solo", State: "closed", Flights: []FlightScore{
		fs(a, 10), fs(b, 9.5), fs(c, 9.5), fs(d, 8),
	}}
	got := Run(in, true).Lists["overall"]
	eq(t, "order", ids(got), []int{1, 3, 2, 4})
	eq(t, "ranks", ranks(got), []int{1, 2, 2, 4})
	eq(t, "positions", positions(got), []int{1, 2, 3, 4})
}

func TestTiesSameNameByID(t *testing.T) {
	in := RunInput{Type: "solo", State: "closed", Flights: []FlightScore{
		fs(pilot(9, "Same"), 5), fs(pilot(3, "Same"), 5),
	}}
	got := Run(in, true).Lists["overall"]
	eq(t, "order", ids(got), []int{3, 9})
	eq(t, "ranks", ranks(got), []int{1, 1})
}

func TestTeamTiesByTeamID(t *testing.T) {
	x := Participant{TeamID: "b", Name: "Same"}
	y := Participant{TeamID: "a", Name: "Same"}
	got := Run(RunInput{Type: "synchro", Flights: []FlightScore{fs(x, 5), fs(y, 5)}}, false).Lists["overall"]
	if got[0].Participant.TeamID != "a" || got[1].Participant.TeamID != "b" {
		t.Errorf("order = %v", got)
	}
}

// spec 03 §7: published_only skips unpublished flights; final needs a
// closed run with every flight published.
func TestRunPublishedOnlyAndFinal(t *testing.T) {
	a, b := pilot(1, "A"), pilot(2, "B")
	flights := []FlightScore{fs(a, 7), {Participant: b, Score: 8, Published: false}}
	in := RunInput{Type: "solo", State: "closed", Flights: flights}

	pub := Run(in, true)
	eq(t, "published only", ids(pub.Lists["overall"]), []int{1})
	eq(t, "final", pub.Final, false)

	all := Run(in, false)
	eq(t, "all flights", ids(all.Lists["overall"]), []int{2, 1})
	eq(t, "final", all.Final, false)

	flights[1].Published = true
	eq(t, "closed + all published", Run(in, true).Final, true)
	in.State = "open"
	eq(t, "open run", Run(in, true).Final, false)
}

// spec 03 §7: one entry per participant, the last saved flight wins.
func TestRunOneEntryPerParticipant(t *testing.T) {
	a := pilot(1, "A")
	got := Run(RunInput{Type: "solo", Flights: []FlightScore{fs(a, 3), fs(pilot(2, "B"), 4), fs(a, 5)}}, false)
	eq(t, "order", ids(got.Lists["overall"]), []int{1, 2})
	eq(t, "scores", scores(got.Lists["overall"]), []float64{5, 4})
}

// spec 03 §9: flight scores are read with Round3.
func TestRunScoresRounded(t *testing.T) {
	got := Run(RunInput{Type: "solo", Flights: []FlightScore{fs(pilot(1, "A"), 7.8933333)}}, false)
	eq(t, "score", got.Lists["overall"][0].Score, 7.893)
}

// spec 03 §7.1: awt/awq/country sub-rankings, women with at least 3 women.
func TestRunSubRankings(t *testing.T) {
	p := func(id int, name, country, gender string, awt bool) Participant {
		return Participant{CivlID: id, Name: name, Country: country, Gender: gender, IsAWT: awt}
	}
	a := p(1, "A", "fra", "woman", true)
	b := p(2, "B", "che", "woman", false)
	c := p(3, "C", "fra", "man", false)
	d := p(4, "D", "ita", "woman", true)
	in := RunInput{Type: "solo", Seasons: []string{"awt-2025", "awq-2025", "fra-2025", "notaseason"}, Flights: []FlightScore{
		fs(a, 6), fs(b, 9), fs(c, 8), fs(d, 7),
	}}
	l := Run(in, false).Lists
	eq(t, "overall", ids(l["overall"]), []int{2, 3, 4, 1})
	eq(t, "awt", ids(l["awt-2025"]), []int{4, 1})
	eq(t, "awt ranks", ranks(l["awt-2025"]), []int{1, 2})
	eq(t, "awq", ids(l["awq-2025"]), []int{2, 3})
	eq(t, "fra", ids(l["fra-2025"]), []int{3, 1})
	eq(t, "women", ids(l["women"]), []int{2, 4, 1})
	eq(t, "women positions", positions(l["women"]), []int{1, 2, 3})
	if _, ok := l["notaseason"]; ok {
		t.Error("a code that is not xxx-YYYY must not create a list")
	}

	in.Flights = in.Flights[:3] // two women only
	if _, ok := Run(in, false).Lists["women"]; ok {
		t.Error("women list with 2 women")
	}
}

// spec 03 §7.1: synchro sub-rankings are the overall list, no women list.
func TestRunSynchroSubLists(t *testing.T) {
	team := func(id string) Participant { return Participant{TeamID: id, Name: id, Gender: "woman"} }
	in := RunInput{Type: "synchro", Seasons: []string{"aws-2025", "awt-2025"}, Flights: []FlightScore{
		fs(team("x"), 5), fs(team("y"), 6), fs(team("z"), 7),
	}}
	l := Run(in, false).Lists
	eq(t, "aws", l["aws-2025"], l["overall"])
	eq(t, "awt", l["awt-2025"], l["overall"])
	if _, ok := l["women"]; ok {
		t.Error("synchro women list")
	}
}

// Worked example spec 03 §8.2 (mixed AWT/AWQ competition).
func workedExample() (runs []RunInput, p1, p2, p3, p4 Participant) {
	p1 = Participant{CivlID: 1, Name: "P1", IsAWT: true, Gender: "man"}
	p2 = Participant{CivlID: 2, Name: "P2", Gender: "man"}
	p3 = Participant{CivlID: 3, Name: "P3", IsAWT: true, Gender: "man"}
	p4 = Participant{CivlID: 4, Name: "P4", Gender: "man"}
	seasons := []string{"awt-2025", "awq-2025"}
	runs = []RunInput{
		{Type: "solo", Seasons: seasons, Year: 2025, State: "closed", Participants: []Participant{p1, p3, p4, p2},
			Flights: []FlightScore{fs(p1, 7.893), fs(p3, 7.28), fs(p4, 7.893), fs(p2, 8.507)}},
		{Type: "solo", Seasons: seasons, Year: 2025, State: "closed", Participants: []Participant{p4, p2, p3, p1},
			Flights: []FlightScore{fs(p4, 7.893), fs(p2, 7.28), fs(p3, 7.893), fs(p1, 8.507)}},
	}
	runs[1].Flights[0].Published = false
	return runs, p1, p2, p3, p4
}

// spec 03 §8, decision 0005: Round3 at each addition, unpublished flights
// counted, final false while a flight is unpublished.
func TestCompetitionWorkedExample(t *testing.T) {
	runs, _, _, _, _ := workedExample()
	got := Competition("closed", runs, -1)
	o := got.Lists["overall"]
	eq(t, "order", ids(o), []int{1, 2, 4, 3})
	eq(t, "scores", scores(o), []float64{16.4, 15.787, 15.786, 15.173})
	eq(t, "final", got.Final, false)
	eq(t, "awt", ids(got.Lists["awt-2025"]), []int{1, 3})
	eq(t, "awq", ids(got.Lists["awq-2025"]), []int{2, 4})
	eq(t, "runs", len(got.Runs), 2)
	eq(t, "run 1 overall", ids(got.Runs[0].Lists["overall"]), []int{2, 1, 4, 3})
	eq(t, "run 1 ranks", ranks(got.Runs[0].Lists["overall"]), []int{1, 2, 2, 4})
	eq(t, "run 1 final", got.Runs[0].Final, true)
	eq(t, "run 2 final", got.Runs[1].Final, false)

	eq(t, "run 2 published only", ids(Run(runs[1], true).Lists["overall"]), []int{1, 3, 2})

	runs[1].Flights[0].Published = true
	eq(t, "final when all published", Competition("closed", runs, -1).Final, true)
	eq(t, "final needs a closed competition", Competition("open", runs, -1).Final, false)
}

func TestCompetitionLimit(t *testing.T) {
	runs, _, _, _, _ := workedExample()
	got := Competition("closed", runs, 0)
	eq(t, "runs", len(got.Runs), 1)
	eq(t, "scores", scores(got.Lists["overall"]), []float64{8.507, 7.893, 7.893, 7.28})
	eq(t, "ranks", ranks(got.Lists["overall"]), []int{1, 2, 2, 4})
	eq(t, "final", got.Final, true)
}

func TestCompetitionRoundsEachAddition(t *testing.T) {
	a := pilot(1, "A")
	runs := []RunInput{
		{Type: "solo", State: "closed", Flights: []FlightScore{fs(a, 0.1)}},
		{Type: "solo", State: "closed", Flights: []FlightScore{fs(a, 0.2)}},
	}
	// 0.1 + 0.2 = 0.30000000000000004 in float64
	eq(t, "total", Competition("closed", runs, -1).Lists["overall"][0].Score, 0.3)
}

// Decision 0006 #3-4: per-run cells hold the participant's rank in the
// same list of that run, and a participant missing a run has no cell.
func TestCompetitionPerRunCellsPerList(t *testing.T) {
	a := Participant{CivlID: 1, Name: "A"}
	b := Participant{CivlID: 2, Name: "B", IsAWT: true}
	c := Participant{CivlID: 3, Name: "C", IsAWT: true}
	s := []string{"awt-2025"}
	runs := []RunInput{
		{Type: "solo", Seasons: s, State: "closed", Flights: []FlightScore{fs(a, 9), fs(b, 8), fs(c, 7)}},
		{Type: "solo", Seasons: s, State: "closed", Flights: []FlightScore{fs(b, 6), fs(a, 6)}},
	}
	got := Competition("closed", runs, -1)
	cells := func(list string, id int) []RunCell {
		for _, e := range got.Lists[list] {
			if e.Participant.CivlID == id {
				return e.PerRun
			}
		}
		t.Fatalf("%d not in %s", id, list)
		return nil
	}
	eq(t, "overall A", cells("overall", 1), []RunCell{{RunIndex: 0, Rank: 1, Score: 9}, {RunIndex: 1, Rank: 1, Score: 6}})
	eq(t, "overall B", cells("overall", 2), []RunCell{{RunIndex: 0, Rank: 2, Score: 8}, {RunIndex: 1, Rank: 1, Score: 6}})
	eq(t, "overall C", cells("overall", 3), []RunCell{{RunIndex: 0, Rank: 3, Score: 7}})
	eq(t, "awt B", cells("awt-2025", 2), []RunCell{{RunIndex: 0, Rank: 1, Score: 8}, {RunIndex: 1, Rank: 1, Score: 6}})
	eq(t, "awt C", cells("awt-2025", 3), []RunCell{{RunIndex: 0, Rank: 2, Score: 7}})
}

// spec 03 §7.1/§8: run sub-rankings inside a competition use the
// competition's participants (women list present in every run when the
// competition has at least 3 women).
func TestCompetitionRunWomenUseCompetitionPopulation(t *testing.T) {
	w := func(id int) Participant {
		return Participant{CivlID: id, Name: string(rune('A' + id)), Gender: "woman"}
	}
	runs := []RunInput{
		{Type: "solo", State: "closed", Flights: []FlightScore{fs(w(1), 5), fs(w(2), 6), fs(w(3), 7)}},
		{Type: "solo", State: "closed", Flights: []FlightScore{fs(w(1), 5)}},
	}
	got := Competition("closed", runs, -1)
	eq(t, "run 2 women", ids(got.Runs[1].Lists["women"]), []int{1})
	eq(t, "women", ids(got.Lists["women"]), []int{1, 3, 2})
}

func TestCompetitionNoRuns(t *testing.T) {
	got := Competition("closed", nil, -1)
	eq(t, "final", got.Final, true)
	eq(t, "overall", len(got.Lists["overall"]), 0)
}

func civlIDs(ps []Participant) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = p.CivlID
	}
	return out
}

// spec 03 §8.2: mixed event, AWQ pilots fly first then AWT, each worst to
// best; decision 0006 #2: the previous order is not mutated.
func TestNextStartOrderMixed(t *testing.T) {
	runs, p1, p2, p3, p4 := workedExample()
	comp := Competition("open", runs[:1], -1)
	prev := []Participant{p1, p3, p4, p2}
	got := NextStartOrder(comp, prev, runs[0].Seasons, 0)
	eq(t, "order", civlIDs(got), []int{4, 2, 3, 1})
	eq(t, "prev untouched", civlIDs(prev), []int{1, 3, 4, 2})
}

// spec 03 §8.1: leader last, participants without result after the leader.
func TestNextStartOrderLeaderLastNoResultLast(t *testing.T) {
	a, b, c, d, e := pilot(1, "A"), pilot(2, "B"), pilot(3, "C"), pilot(4, "D"), pilot(5, "E")
	comp := Competition("open", []RunInput{{Type: "solo", Seasons: []string{"awt-2025"}, State: "closed",
		Flights: []FlightScore{fs(a, 9), fs(b, 7), fs(c, 8)}}}, -1)
	prev := []Participant{d, a, e, b, c}
	got := NextStartOrder(comp, prev, []string{"awt-2025"}, 0)
	eq(t, "order", civlIDs(got), []int{2, 3, 1, 4, 5})
	eq(t, "prev untouched", civlIDs(prev), []int{4, 1, 5, 2, 3})
}

func TestNextStartOrderSynchro(t *testing.T) {
	x, y, z := Participant{TeamID: "x", Name: "x"}, Participant{TeamID: "y", Name: "y"}, Participant{TeamID: "z", Name: "z"}
	s := []string{"awt-2025", "awq-2025"}
	comp := Competition("open", []RunInput{{Type: "synchro", Seasons: s, State: "closed",
		Flights: []FlightScore{fs(x, 9), fs(y, 7), fs(z, 8)}}}, -1)
	got := NextStartOrder(comp, []Participant{x, y, z}, s, 0)
	if got[0].TeamID != "y" || got[1].TeamID != "z" || got[2].TeamID != "x" {
		t.Errorf("order = %v", got)
	}
}

// Decision 0006 #1: qualify keeps the best N, then orders them worst to best.
func TestNextStartOrderQualifyKeepsBest(t *testing.T) {
	a, b, c, d := pilot(1, "A"), pilot(2, "B"), pilot(3, "C"), pilot(4, "D")
	comp := Competition("open", []RunInput{{Type: "solo", State: "closed",
		Flights: []FlightScore{fs(a, 9), fs(b, 7), fs(c, 8), fs(d, 6)}}}, -1)
	prev := []Participant{a, b, c, d}
	eq(t, "best 2", civlIDs(NextStartOrder(comp, prev, nil, 2)), []int{3, 1})
	eq(t, "best 3", civlIDs(NextStartOrder(comp, prev, nil, 3)), []int{2, 3, 1})
	eq(t, "N above count", civlIDs(NextStartOrder(comp, prev, nil, 10)), []int{4, 2, 3, 1})
	eq(t, "prev untouched", civlIDs(prev), []int{1, 2, 3, 4})

	// participants without result only fill the remaining places
	e := pilot(5, "E")
	eq(t, "no result fills", civlIDs(NextStartOrder(comp, []Participant{e, a, b, c, d}, nil, 5)), []int{4, 2, 3, 1, 5})
	eq(t, "no result cut", civlIDs(NextStartOrder(comp, []Participant{e, a, b, c, d}, nil, 4)), []int{4, 2, 3, 1})
}

func TestNextStartOrderQualifyMixed(t *testing.T) {
	runs, p1, p2, p3, p4 := workedExample()
	comp := Competition("open", runs[:1], -1)
	// best 3 overall after run 1: 2 (8.507), 1 and 4 (7.893); then AWQ
	// worst to best (4, 2), then AWT (1)
	eq(t, "best 3", civlIDs(NextStartOrder(comp, []Participant{p1, p3, p4, p2}, runs[0].Seasons, 3)), []int{4, 2, 1})
}

// First run: no results yet, the given order is kept.
func TestNextStartOrderNoResults(t *testing.T) {
	a, b, c := pilot(1, "A"), pilot(2, "B"), pilot(3, "C")
	comp := Competition("open", nil, -1)
	eq(t, "order", civlIDs(NextStartOrder(comp, []Participant{c, a, b}, nil, 0)), []int{3, 1, 2})
	eq(t, "qualify", civlIDs(NextStartOrder(comp, []Participant{c, a, b}, nil, 2)), []int{3, 1})
}

func compOf(t *testing.T, typ, season string, flights ...FlightScore) CompetitionResult {
	t.Helper()
	return Competition("closed", []RunInput{{Type: typ, Seasons: []string{season}, State: "closed", Flights: flights}}, -1)
}

// spec 03 §10: only published, closed and final competitions count;
// plain (unrounded) sum.
func TestSeasonCountsPublishedClosedFinal(t *testing.T) {
	a, b := pilot(1, "A"), pilot(2, "B")
	a.IsAWT, b.IsAWT = true, true
	c1 := compOf(t, "solo", "awt-2025", fs(a, 0.1), fs(b, 0.3))
	c2 := compOf(t, "solo", "awt-2025", fs(a, 0.2), fs(b, 0.1))
	notFinal := c2
	notFinal.Final = false
	in := SeasonInput{Code: "awt-2025", Type: "solo", Competitions: []SeasonComp{
		{Code: "c1", Published: true, Closed: true, Result: c1},
		{Code: "c2", Published: true, Closed: true, Result: c2},
		{Code: "unpublished", Published: false, Closed: true, Result: c1},
		{Code: "open", Published: true, Closed: false, Result: c1},
		{Code: "notfinal", Published: true, Closed: true, Result: notFinal},
	}}
	got := Season(in)["overall"]
	unrounded := 0.1
	unrounded += 0.2 // 0.30000000000000004
	eq(t, "order", ids(got), []int{2, 1})
	eq(t, "scores", scores(got), []float64{0.4, unrounded})
	eq(t, "ranks", ranks(got), []int{1, 2})
}

// spec 03 §10: the season reads the competition's list for its code.
func TestSeasonUsesSeasonSubRanking(t *testing.T) {
	a := Participant{CivlID: 1, Name: "A", IsAWT: true}
	b := Participant{CivlID: 2, Name: "B"}
	c := compOf(t, "solo", "awq-2025", fs(a, 9), fs(b, 8))
	got := Season(SeasonInput{Code: "awq-2025", Type: "solo", Competitions: []SeasonComp{{Published: true, Closed: true, Result: c}}})
	eq(t, "awq", ids(got["overall"]), []int{2})

	missing := Season(SeasonInput{Code: "awt-2025", Type: "solo", Competitions: []SeasonComp{{Published: true, Closed: true, Result: c}}})
	if _, ok := missing["overall"]; ok {
		t.Error("overall list without results")
	}
}

func TestSeasonCountryFilter(t *testing.T) {
	a := Participant{CivlID: 1, Name: "A", Country: "che"}
	b := Participant{CivlID: 2, Name: "B", Country: "fra"}
	c := compOf(t, "solo", "awt-2025", fs(a, 9), fs(b, 8))
	c.Lists["awt-2025"] = c.Lists["overall"]
	got := Season(SeasonInput{Code: "awt-2025", Country: "che", Type: "solo", Competitions: []SeasonComp{{Published: true, Closed: true, Result: c}}})
	eq(t, "che", ids(got["overall"]), []int{1})
}

func TestSeasonWomenAndTies(t *testing.T) {
	w := func(id int, name string) Participant {
		return Participant{CivlID: id, Name: name, Country: "fra", Gender: "woman"}
	}
	m := Participant{CivlID: 9, Name: "M", Country: "fra", Gender: "man"}
	c := compOf(t, "solo", "fra-2025", fs(w(1, "Zoe"), 5), fs(w(2, "Ann"), 5), fs(m, 7))
	in := SeasonInput{Code: "fra-2025", Type: "solo", Competitions: []SeasonComp{{Published: true, Closed: true, Result: c}}}
	got := Season(in)
	eq(t, "overall", ids(got["overall"]), []int{9, 2, 1})
	eq(t, "ranks", ranks(got["overall"]), []int{1, 2, 2})
	if _, ok := got["women"]; ok {
		t.Error("women list with 2 women")
	}

	c = compOf(t, "solo", "fra-2025", fs(w(1, "Zoe"), 5), fs(w(2, "Ann"), 5), fs(m, 7), fs(w(3, "Eve"), 6))
	in.Competitions[0].Result = c
	got = Season(in)
	eq(t, "women", ids(got["women"]), []int{3, 2, 1})
	eq(t, "women ranks", ranks(got["women"]), []int{1, 2, 2})
}

func TestSeasonSynchroNoWomen(t *testing.T) {
	team := func(id string) Participant { return Participant{TeamID: id, Name: id, Gender: "woman"} }
	c := compOf(t, "synchro", "aws-2025", fs(team("x"), 5), fs(team("y"), 6), fs(team("z"), 7))
	got := Season(SeasonInput{Code: "aws-2025", Type: "synchro", Competitions: []SeasonComp{{Published: true, Closed: true, Result: c}}})
	eq(t, "overall", len(got["overall"]), 3)
	if _, ok := got["women"]; ok {
		t.Error("synchro women list")
	}
}
