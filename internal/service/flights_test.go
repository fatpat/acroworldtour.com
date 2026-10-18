package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/store"
	"awt/internal/store/gen"
)

func hasString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// flightEnv is an open solo competition of 2025 (production 2025 catalog)
// with run 1 open, two judges and the given pilots.
type flightEnv struct {
	*fixture
	comp   Competition
	judges []string
}

// newFlightEnv creates pilots (CIVL ids), makes awt ones AWT for 2025 and
// opens a published competition in the given seasons with run 1 open.
func newFlightEnv(t *testing.T, pilots, awt []int, seasons []string) *flightEnv {
	t.Helper()
	ctx := context.Background()
	f := newFixture(t)
	cat, err := os.ReadFile("../../internal/catalog/data/2025.json")
	ok(t, err)
	must(f.svc.ImportCatalog(ctx, 2025, bytes.NewReader(cat)))(t)
	for _, p := range pilots {
		must(f.svc.ImportPilotFromCIVL(ctx, p))(t)
	}
	for _, p := range awt {
		ok(t, f.svc.SetAWTYear(ctx, p, 2025, true))
	}
	for _, code := range []string{"awt-2025", "awq-2025"} {
		must(f.svc.CreateSeason(ctx, SeasonInput{Name: code, Code: code, Year: 2025, SortIndex: 1}))(t)
	}
	j1 := must(f.svc.CreateJudge(ctx, JudgeInput{Name: "Senior", Country: "fra", Level: domain.JudgeSenior}))(t)
	j2 := must(f.svc.CreateJudge(ctx, JudgeInput{Name: "Certified", Country: "che", Level: domain.JudgeCertified}))(t)
	c := must(f.svc.CreateCompetition(ctx, CompetitionInput{
		Name: "Flight Cup 2025", Type: domain.TypeSolo, StartDate: date("2025-06-01"), EndDate: date("2025-06-03"),
		Location: "Annecy", Published: true, Seasons: seasons,
	}))(t)
	ok(t, f.svc.SetParticipants(ctx, c.ID, pilots, nil))
	ok(t, f.svc.SetJudges(ctx, c.ID, []string{j1.ID, j2.ID}))
	ok(t, f.svc.OpenCompetition(ctx, c.ID))
	must(f.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone))(t)
	ok(t, f.svc.OpenRun(ctx, c.ID, 0))
	return &flightEnv{fixture: f, comp: must(f.svc.GetCompetition(ctx, c.ID))(t), judges: []string{j1.ID, j2.ID}}
}

func ptr(v float64) *float64 { return &v }

// input is a flight where both judges give technical t (classic and per
// trick), choreography 6 and landing 6 (spec 03 §8.2).
func (e *flightEnv) input(t float64, tricks ...string) FlightInput {
	in := FlightInput{Tricks: tricks}
	for _, j := range e.judges {
		tpt := make([]*float64, len(tricks))
		for i := range tpt {
			tpt[i] = ptr(t)
		}
		in.Marks = append(in.Marks, MarkInput{Judge: j, Technical: ptr(t), Choreography: ptr(6), Landing: ptr(6), TechnicalPerTrick: tpt})
	}
	return in
}

func (e *flightEnv) save(t *testing.T, run, pilot int, in FlightInput, publish bool) scoring.Result {
	t.Helper()
	return must(e.svc.SaveFlight(context.Background(), e.comp.ID, run, fmt.Sprint(pilot), in, publish, e.version(t, run, pilot)))(t)
}

// version is the current version of a pilot's stored flight ("" when none),
// as the judging form quotes it.
func (e *flightEnv) version(t *testing.T, run, pilot int) string {
	t.Helper()
	f, _, err := e.svc.GetFlight(context.Background(), e.comp.ID, run, fmt.Sprint(pilot))
	ok(t, err)
	return f.Version
}

// storedFlight reads a pilot's stored flight row and final mark.
func (e *flightEnv) storedFlight(t *testing.T, run, pilot int) (gen.Flight, scoring.FinalMark) {
	t.Helper()
	row, err := gen.New(e.db.Read).GetPilotFlight(context.Background(), gen.GetPilotFlightParams{
		CompetitionID: e.comp.ID, RunIdx: int64(run), Civlid: sql.NullInt64{Int64: int64(pilot), Valid: true},
	})
	ok(t, err)
	var fm scoring.FinalMark
	ok(t, json.Unmarshal([]byte(row.FinalMark.String), &fm))
	return row, fm
}

type rankLine struct {
	civlid int
	score  float64
}

func (e *flightEnv) ranking(t *testing.T, scope, key, list string) []rankLine {
	t.Helper()
	rows, err := gen.New(e.db.Read).ListRankingEntries(context.Background(), gen.ListRankingEntriesParams{Scope: scope, ScopeKey: key, ListKey: list})
	ok(t, err)
	out := []rankLine{}
	for _, r := range rows {
		out = append(out, rankLine{int(r.Civlid.Int64), r.Score})
	}
	return out
}

func sameLines(got []rankLine, want ...rankLine) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestSaveFlightScoresAndRanks reproduces spec 03 §8.2 run 1 for P1 (AWT,
// t=7 -> 7.893) and P2 (AWQ, t=8 -> 8.507).
func TestSaveFlightScoresAndRanks(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, []int{201}, []string{"awt-2025", "awq-2025"})
	ctx := context.Background()

	sim := must(e.svc.SimulateFlight(ctx, e.comp.ID, 0, "201", e.input(7, "LM", "RMC", "LS")))(t)
	if got := scoring.Round3(sim.Final.Score); got != 7.893 {
		t.Errorf("simulated score %v, want 7.893", got)
	}
	if n := len(must(gen.New(e.db.Read).ListFlightsOfRun(ctx, gen.ListFlightsOfRunParams{CompetitionID: e.comp.ID}))(t)); n != 0 {
		t.Fatalf("simulation wrote %d flights", n)
	}

	r1 := e.save(t, 0, 201, e.input(7, "LM", "RMC", "LS"), true)
	r2 := e.save(t, 0, 202, e.input(8, "left Misty Flip", "RMC", "LS"), true)
	if r1.Final.MarkType != scoring.MarkAWT || r2.Final.MarkType != scoring.MarkAWQ {
		t.Errorf("mark types %q %q, want awt awq", r1.Final.MarkType, r2.Final.MarkType)
	}
	row, fm := e.storedFlight(t, 0, 201)
	if !row.Score.Valid || row.Score.Float64 != 7.893 || fm.Score != r1.Final.Score || row.MarkType.String != "awt" || row.SaveSeq != 1 || !row.Published {
		t.Errorf("stored P1 flight: %+v final %+v", row, fm)
	}
	if row, _ := e.storedFlight(t, 0, 202); row.Score.Float64 != 8.507 || row.SaveSeq != 2 || row.MarkType.String != "awq" {
		t.Errorf("stored P2 flight: %+v", row)
	}
	tricks := must(gen.New(e.db.Read).ListFlightTricks(ctx, row.ID))(t)
	if len(tricks) != 3 || tricks[0].Acronym != "LM" || tricks[1].TechnicalCoefficient != 1.7 || !tricks[0].TechnicalMark.Valid || tricks[0].TechnicalMark.Float64 != 7 {
		t.Errorf("stored tricks: %+v", tricks)
	}

	run := store.RunScopeKey(e.comp.ID, 0)
	if got := e.ranking(t, store.ScopeRun, run, "overall"); !sameLines(got, rankLine{202, 8.507}, rankLine{201, 7.893}) {
		t.Errorf("run ranking %v", got)
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "awt-2025"); !sameLines(got, rankLine{201, 7.893}) {
		t.Errorf("competition awt list %v", got)
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "awq-2025"); !sameLines(got, rankLine{202, 8.507}) {
		t.Errorf("competition awq list %v", got)
	}

	// Closing the run and the competition makes results final and feeds
	// the seasons.
	ok(t, e.svc.CloseRun(ctx, e.comp.ID, 0))
	if !must(e.svc.GetRun(ctx, e.comp.ID, 0))(t).ResultsFinal {
		t.Error("closed run with all flights published is not final")
	}
	if len(e.ranking(t, store.ScopeSeason, "awt-2025", "overall")) != 0 {
		t.Error("season ranks an open competition")
	}
	ok(t, e.svc.CloseCompetition(ctx, e.comp.ID))
	if !must(e.svc.GetCompetition(ctx, e.comp.ID))(t).ResultsFinal {
		t.Error("closed competition is not final")
	}
	if got := e.ranking(t, store.ScopeSeason, "awt-2025", "overall"); !sameLines(got, rankLine{201, 7.893}) {
		t.Errorf("awt season %v", got)
	}
	if got := e.ranking(t, store.ScopeSeason, "awq-2025", "overall"); !sameLines(got, rankLine{202, 8.507}) {
		t.Errorf("awq season %v", got)
	}
	ok(t, e.svc.DeleteCompetition(ctx, e.comp.ID))
	if len(e.ranking(t, store.ScopeSeason, "awt-2025", "overall")) != 0 {
		t.Error("deleted competition still in the season")
	}
	ok(t, e.svc.RestoreCompetition(ctx, e.comp.ID))
	if got := e.ranking(t, store.ScopeSeason, "awt-2025", "overall"); !sameLines(got, rankLine{201, 7.893}) {
		t.Errorf("restored competition: awt season %v", got)
	}
	ok(t, e.svc.ReopenCompetition(ctx, e.comp.ID))
	if len(e.ranking(t, store.ScopeSeason, "awt-2025", "overall")) != 0 {
		t.Error("reopened competition still in the season")
	}
	ok(t, e.svc.ReopenRun(ctx, e.comp.ID, 0))
	if must(e.svc.GetRun(ctx, e.comp.ID, 0))(t).ResultsFinal {
		t.Error("reopened run still final")
	}
}

func TestMarkTypeFor(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, []int{201}, []string{"awt-2025"})
	ctx := context.Background()
	cases := []struct {
		seasons []string
		pilot   string
		want    scoring.MarkType
	}{
		{[]string{"awt-2025", "awq-2025"}, "201", scoring.MarkAWT},
		{[]string{"awt-2025", "awq-2025"}, "202", scoring.MarkAWQ},
		{[]string{"awt-2025"}, "202", scoring.MarkClassic},
		{[]string{"awq-2025"}, "201", scoring.MarkAWQ},
		{nil, "201", scoring.MarkClassic},
	}
	for _, c := range cases {
		comp := e.comp
		comp.Seasons = c.seasons
		if got := must(e.svc.MarkTypeFor(ctx, comp, c.pilot))(t); got != c.want {
			t.Errorf("seasons %v pilot %s: %q, want %q", c.seasons, c.pilot, got, c.want)
		}
	}
	comp := e.comp
	comp.Type = domain.TypeSynchro
	comp.Seasons = []string{"awt-2025", "awq-2025"}
	if got := must(e.svc.MarkTypeFor(ctx, comp, "any-team"))(t); got != scoring.MarkClassic {
		t.Errorf("synchro: %q", got)
	}
}

func TestSaveRefusedWhenRunNotOpen(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	must(e.svc.NewRun(ctx, e.comp.ID, 0, scoring.ResetNone))(t) // run 2 stays init
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 1, "201", e.input(7, "LM"), true, ""); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("init run: %v", err)
	}
	e.save(t, 0, 201, e.input(7, "LM"), true)
	e.save(t, 0, 202, e.input(7, "LM"), true)
	ok(t, e.svc.CloseRun(ctx, e.comp.ID, 0))
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", e.input(8, "LM"), true, ""); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("closed run: %v", err)
	}
	if _, fm := e.storedFlight(t, 0, 201); scoring.Round3(fm.Score) == 0 {
		t.Error("flight lost")
	}
	_, err := e.db.Write.ExecContext(ctx, `UPDATE runs SET state = 'open' WHERE competition_id = ?`, e.comp.ID)
	ok(t, err)
	_, err = e.db.Write.ExecContext(ctx, `UPDATE competitions SET frozen = 1 WHERE id = ?`, e.comp.ID)
	ok(t, err)
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", e.input(8, "LM"), true, ""); !errors.Is(err, ErrFrozen) {
		t.Errorf("frozen competition: %v", err)
	}
}

func TestUnknownTrickError(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	var ve scoring.ValidationError
	_, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", e.input(7, "LM", "XYZ"), true, "")
	if !errors.As(err, &ve) || ve.Msg != "Unknown trick(s): XYZ" {
		t.Errorf("unknown trick: %v", err)
	}
	_, err = e.svc.SimulateFlight(ctx, e.comp.ID, 0, "201", e.input(7, "XYZ", "LM", "ABC", "XYZ"))
	if !errors.As(err, &ve) || ve.Msg != "Unknown trick(s): XYZ, ABC" {
		t.Errorf("unknown tricks: %v", err)
	}
	// The typed error lists the tricks, first seen first, without duplicates.
	var ue *UnknownTricksError
	if !errors.As(err, &ue) || strings.Join(ue.Tricks, "|") != "XYZ|ABC" {
		t.Errorf("typed unknown tricks: %#v", err)
	}
}

// TestRawMarksStored: flight_marks keep the judges' marks as entered (a
// blank per-trick mark stays null); the engine fills it in at every
// scoring, so a rescore gives the same score.
func TestRawMarksStored(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, []int{201}, []string{"awt-2025"})
	ctx := context.Background()
	in := e.input(7, "LM", "RMC")
	in.Marks[1].TechnicalPerTrick[1] = nil
	res := e.save(t, 0, 201, in, true)
	row, _ := e.storedFlight(t, 0, 201)
	marks, err := gen.New(e.db.Read).ListFlightMarks(ctx, row.ID)
	ok(t, err)
	var nulls int
	for _, m := range marks {
		nulls += strings.Count(m.TechnicalPerTrick.String, "null")
	}
	if nulls != 1 {
		t.Fatalf("stored per-trick marks %+v, want exactly one null", marks)
	}
	got, _, err := e.svc.GetFlight(ctx, e.comp.ID, 0, "201")
	ok(t, err)
	for _, m := range got.Input.Marks {
		if m.Judge == in.Marks[1].Judge && m.TechnicalPerTrick[1] != nil {
			t.Fatalf("blank mark read back as %v", *m.TechnicalPerTrick[1])
		}
	}
	ok(t, e.db.Tx(ctx, func(q *gen.Queries) error { return rescoreCompetition(ctx, q, e.comp) }))
	after, _ := e.storedFlight(t, 0, 201)
	if after.Score.Float64 != scoring.Round3(res.Final.Score) {
		t.Fatalf("rescored score %v, saved %v", after.Score.Float64, scoring.Round3(res.Final.Score))
	}
}

func TestFlightVersionAndDelete(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	// New flight: no version; the same stale "new" form cannot save twice.
	must(e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", e.input(7, "LM"), true, ""))(t)
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", e.input(8, "LM"), true, ""); !errors.Is(err, ErrConflict) || !errors.Is(err, ErrStaleFlight) {
		t.Fatalf("second new save: %v", err)
	}
	must(e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", e.input(8, "LM"), true, "1"))(t)
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", e.input(9, "LM"), true, "1"); !errors.Is(err, ErrStaleFlight) {
		t.Fatalf("stale save: %v", err)
	}
	if v := e.version(t, 0, 201); v != "2" {
		t.Fatalf("version = %s, want 2", v)
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "overall"); len(got) != 1 {
		t.Fatalf("competition ranking before delete: %v", got)
	}

	// Delete: stale version refused, then deleted and the ranking rebuilt.
	if err := e.svc.DeleteFlight(ctx, e.comp.ID, 0, "201", "1"); !errors.Is(err, ErrStaleFlight) {
		t.Fatalf("stale delete: %v", err)
	}
	ok(t, e.svc.DeleteFlight(ctx, e.comp.ID, 0, "201", "2"))
	if _, found, _ := e.svc.GetFlight(ctx, e.comp.ID, 0, "201"); found {
		t.Fatal("flight not deleted")
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "overall"); len(got) != 0 {
		t.Fatalf("competition ranking after delete: %v", got)
	}
	if err := e.svc.DeleteFlight(ctx, e.comp.ID, 0, "201", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete of no flight: %v", err)
	}
}

func TestSaveFlightValidation(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202, 203}, nil, nil)
	ctx := context.Background()
	stranger := must(e.svc.CreateJudge(ctx, JudgeInput{Name: "Stranger", Country: "fra", Level: domain.JudgeSenior}))(t)
	var ve *ValidationError
	bad := e.input(7, "LM")
	bad.Marks[0].Landing = ptr(10.5)
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", bad, true, ""); !errors.As(err, &ve) {
		t.Errorf("mark 10.5: %v", err)
	}
	bad = e.input(7, "LM")
	bad.Marks[1].TechnicalPerTrick[0] = ptr(-1)
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", bad, true, ""); !errors.As(err, &ve) {
		t.Errorf("per-trick mark -1: %v", err)
	}
	bad = e.input(7, "LM")
	bad.Marks[1].Judge = stranger.ID
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", bad, true, ""); !errors.As(err, &ve) ||
		ve.Fields[0].Msg != fmt.Sprintf("judge %s is not a judge of this run", stranger.ID) {
		t.Errorf("judge outside the run: %v", err)
	}
	bad = e.input(7, "LM")
	bad.Marks[1].Judge = bad.Marks[0].Judge
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", bad, true, ""); !errors.As(err, &ve) {
		t.Errorf("duplicate judge: %v", err)
	}
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "999", e.input(7, "LM"), true, ""); !errors.As(err, &ve) {
		t.Errorf("pilot outside the run: %v", err)
	}
	// nil marks are "no mark": no choreography at all is a legacy 400
	none := e.input(7, "LM")
	for i := range none.Marks {
		none.Marks[i].Choreography = nil
	}
	var se scoring.ValidationError
	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", none, true, ""); !errors.As(err, &se) || se.Msg != "not enough choreography marks" {
		t.Errorf("no choreography: %v", err)
	}
	if n := len(must(gen.New(e.db.Read).ListFlightsOfRun(ctx, gen.ListFlightsOfRunParams{CompetitionID: e.comp.ID}))(t)); n != 0 {
		t.Errorf("invalid flights stored: %d", n)
	}
	// DNS needs no marks
	dns := FlightInput{DidNotStart: true, Warnings1: []string{"late"}}
	if r := e.save(t, 0, 203, dns, false); r.Final.Score != 0 || len(r.Final.Notes) != 1 {
		t.Errorf("DNS: %+v", r.Final)
	}
}

func TestResaveEarlierRunRescoresLaterRuns(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	e.save(t, 0, 201, e.input(7, "LM", "RMC", "LS"), true)
	must(e.svc.NewRun(ctx, e.comp.ID, 0, scoring.ResetNone))(t)
	ok(t, e.svc.OpenRun(ctx, e.comp.ID, 1))
	r2 := e.save(t, 1, 201, e.input(7, "LM", "RM", "RS"), true)
	if r2.Final.Malus != 13 {
		t.Fatalf("run 2 malus %v, want 13 (LM repeated)", r2.Final.Malus)
	}
	before, _ := e.storedFlight(t, 1, 201)

	r1 := e.save(t, 0, 201, e.input(7, "RMC", "LS"), true)
	after, fm := e.storedFlight(t, 1, 201)
	if fm.Malus != 0 {
		t.Errorf("run 2 malus after re-saving run 1: %v, want 0", fm.Malus)
	}
	if after.Score.Float64 <= before.Score.Float64 || after.SaveSeq != before.SaveSeq || !after.Published {
		t.Errorf("run 2 flight before %+v after %+v", before, after)
	}
	want := scoring.Round3(scoring.Round3(r1.Final.Score) + after.Score.Float64)
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "overall"); !sameLines(got, rankLine{201, want}) {
		t.Errorf("competition total %v, want %v", got, want)
	}
}

func TestConcurrentFlightSaves(t *testing.T) {
	pilots := make([]int, 20)
	for i := range pilots {
		pilots[i] = 301 + i
	}
	e := newFlightEnv(t, pilots, nil, nil)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, len(pilots))
	for i, p := range pilots {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, fmt.Sprint(p), e.input(5+float64(i)/10, "LM", "RMC", "LS"), true, "")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	flights := must(gen.New(e.db.Read).ListFlightsOfRun(ctx, gen.ListFlightsOfRunParams{CompetitionID: e.comp.ID}))(t)
	seqs := map[int64]bool{}
	for _, f := range flights {
		seqs[f.SaveSeq] = true
	}
	if len(flights) != 20 || len(seqs) != 20 {
		t.Errorf("%d flights, %d distinct save_seq", len(flights), len(seqs))
	}
	if n := len(e.ranking(t, store.ScopeRun, store.RunScopeKey(e.comp.ID, 0), "overall")); n != 20 {
		t.Errorf("run ranking has %d entries", n)
	}
	if n := len(e.ranking(t, store.ScopeCompetition, e.comp.ID, "overall")); n != 20 {
		t.Errorf("competition ranking has %d entries", n)
	}
}

func TestSavePublishesEvent(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, []string{"awt-2025"})
	ctx := context.Background()
	comp, unsubscribe := e.bus.Subscribe("competition:" + e.comp.Code)
	defer unsubscribe()
	season, unsubscribeSeason := e.bus.Subscribe("season:awt-2025")
	defer unsubscribeSeason()

	if _, err := e.svc.SaveFlight(ctx, e.comp.ID, 0, "201", e.input(7, "XYZ"), true, ""); err == nil {
		t.Fatal("want error")
	}
	if _, err := e.svc.SimulateFlight(ctx, e.comp.ID, 0, "201", e.input(7, "LM")); err != nil {
		t.Fatal(err)
	}
	if len(comp) != 0 || len(season) != 0 {
		t.Fatal("event published by a failed save or a simulation")
	}
	e.save(t, 0, 201, e.input(7, "LM"), false)
	select {
	case ev := <-comp:
		if ev.Kind != KindFlight {
			t.Errorf("competition event kind %q", ev.Kind)
		}
	case <-time.After(time.Second):
		t.Error("no competition event")
	}
	// an open competition is not in its seasons' rankings: no season event
	if len(season) != 0 {
		t.Error("season event for a flight of an open competition")
	}
}

func (e *flightEnv) update(t *testing.T, seasons ...string) error {
	t.Helper()
	c := e.comp
	_, err := e.svc.UpdateCompetition(context.Background(), c.ID, CompetitionInput{
		Name: c.Name, Code: c.Code, Type: c.Type, StartDate: c.StartDate, EndDate: c.EndDate, Location: c.Location,
		Published: c.Published, Seasons: seasons,
	})
	return err
}

// A seasons change rescores every flight: mark types follow (controller
// ruling on decision 0006).
func TestSeasonChangeRescoresFlights(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, []int{201}, nil)
	e.save(t, 0, 201, e.input(7, "LM", "RMC", "LS"), true)
	e.save(t, 0, 202, e.input(8, "LM", "RMC", "LS"), true)
	if row, _ := e.storedFlight(t, 0, 201); row.MarkType.Valid {
		t.Fatalf("classic flight stored with mark type %q", row.MarkType.String)
	}
	ok(t, e.update(t, "awt-2025", "awq-2025"))
	row1, fm1 := e.storedFlight(t, 0, 201)
	row2, fm2 := e.storedFlight(t, 0, 202)
	if row1.MarkType.String != "awt" || fm1.MarkType != scoring.MarkAWT || row2.MarkType.String != "awq" || fm2.MarkType != scoring.MarkAWQ {
		t.Errorf("mark types after the change: %q/%q %q/%q", row1.MarkType.String, fm1.MarkType, row2.MarkType.String, fm2.MarkType)
	}
	if row1.Score.Float64 != 7.893 || row2.Score.Float64 != 8.507 {
		t.Errorf("scores %v %v", row1.Score.Float64, row2.Score.Float64)
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "awt-2025"); !sameLines(got, rankLine{201, 7.893}) {
		t.Errorf("awt list %v", got)
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "awq-2025"); !sameLines(got, rankLine{202, 8.507}) {
		t.Errorf("awq list %v", got)
	}
}

// A flight that the change makes unscorable is an ErrRescore naming it,
// not a validation error of the caller; nothing is changed.
func TestSeasonChangeRescoreFailure(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, []int{201}, nil)
	in := e.input(7, "LM", "RMC", "LS")
	for i := range in.Marks {
		in.Marks[i].TechnicalPerTrick = nil // classic marking only
	}
	e.save(t, 0, 201, in, true)
	err := e.update(t, "awt-2025")
	var ve scoring.ValidationError
	if !errors.Is(err, ErrRescore) || errors.As(err, &ve) || !strings.Contains(err.Error(), "pilot 201 in run 1") {
		t.Fatalf("err = %v, want ErrRescore naming pilot 201 in run 1", err)
	}
	if c := must(e.svc.GetCompetition(context.Background(), e.comp.ID))(t); len(c.Seasons) != 0 {
		t.Errorf("seasons changed despite the failure: %v", c.Seasons)
	}
}

func TestAmbiguousTrickName(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	_, err := e.db.Write.Exec(`INSERT INTO unique_tricks (trick_id, year, acronym, name, technical_coefficient, bonus, solo, solo_awt, synchro)
		SELECT trick_id, year, 'ZZLM', name, technical_coefficient, bonus, solo, solo_awt, synchro FROM unique_tricks WHERE year = 2025 AND acronym = 'LM'`)
	ok(t, err)
	var ve scoring.ValidationError
	_, err = e.svc.SimulateFlight(context.Background(), e.comp.ID, 0, "201", e.input(7, "left Misty Flip"))
	if !errors.As(err, &ve) || ve.Msg != "Ambiguous trick name left Misty Flip, use the acronym" {
		t.Errorf("ambiguous name: %v", err)
	}
	if _, err := e.svc.SimulateFlight(context.Background(), e.comp.ID, 0, "201", e.input(7, "LM")); err != nil {
		t.Errorf("acronym: %v", err)
	}
}

func TestPositionWarning(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	r := e.save(t, 0, 201, e.input(7, "LM", "LT"), true)
	if !hasString(r.Final.Warnings1, "left Tumbling can't be one of the last 2 maneuvers") || len(r.Final.Warnings1) != 1 {
		t.Errorf("warnings %v", r.Final.Warnings1)
	}
	// automatic warnings are not operator warnings: not stored as such
	row, _ := e.storedFlight(t, 0, 201)
	if w := must(gen.New(e.db.Read).ListFlightWarnings(context.Background(), row.ID))(t); len(w) != 0 {
		t.Errorf("stored warnings %v", w)
	}
}

func TestRepeatableBaseTrick(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	if !hasString(e.comp.RepeatableTricks, "Stall") {
		t.Fatalf("repeatable tricks %v", e.comp.RepeatableTricks)
	}
	if r := e.save(t, 0, 201, e.input(7, "FS", "LM", "FS"), true); r.Final.Malus != 0 {
		t.Errorf("repeated Stall: malus %v", r.Final.Malus)
	}
	if r := e.save(t, 0, 202, e.input(7, "LM", "FS", "LM"), true); r.Final.Malus != 13 {
		t.Errorf("repeated LM: malus %v", r.Final.Malus)
	}
}

func TestPreviousWarningsDSQ(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		must(e.svc.NewRun(ctx, e.comp.ID, 0, scoring.ResetNone))(t)
		ok(t, e.svc.OpenRun(ctx, e.comp.ID, i))
	}
	in := e.input(7, "LM", "RMC")
	in.Warnings1 = []string{"w", "w"}
	if r := e.save(t, 0, 201, in, true); r.Final.Score == 0 {
		t.Errorf("run 1 with 2 warnings: %+v", r.Final)
	}
	in.Warnings1 = []string{"w"}
	r := e.save(t, 1, 201, in, true)
	if r.Final.Score != 0 || !hasString(r.Final.Notes, "Pilot has been DSQ he/she's got 3 warnings") {
		t.Errorf("run 2: %+v", r.Final)
	}
	r = e.save(t, 2, 201, e.input(7, "LM", "RMC"), true)
	if r.Final.Score != 0 || !hasString(r.Final.Notes, "Pilot has been DSQ because he/she already had 3 warnings") {
		t.Errorf("run 3: %+v", r.Final)
	}
	// removing a run-1 warning lifts both DSQs (later runs rescored)
	in.Warnings1 = []string{"w"}
	e.save(t, 0, 201, in, true)
	for run := 1; run <= 2; run++ {
		if row, fm := e.storedFlight(t, run, 201); row.Score.Float64 == 0 {
			t.Errorf("run %d still DSQ: %v", run+1, fm.Notes)
		}
	}
}

func TestResetPolicyAll(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	e.save(t, 0, 201, e.input(7, "LM", "RMC"), true)
	must(e.svc.NewRun(ctx, e.comp.ID, 0, scoring.ResetAll))(t)
	ok(t, e.svc.OpenRun(ctx, e.comp.ID, 1))
	if r := e.save(t, 1, 201, e.input(7, "LM", "RS"), true); r.Final.Malus != 0 {
		t.Errorf("LM after a reset-all run: malus %v", r.Final.Malus)
	}
}

func TestThreeRunRescoreChain(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		must(e.svc.NewRun(ctx, e.comp.ID, 0, scoring.ResetNone))(t)
		ok(t, e.svc.OpenRun(ctx, e.comp.ID, i))
	}
	e.save(t, 0, 201, e.input(7, "LM", "RMC", "LS"), true)
	e.save(t, 1, 201, e.input(7, "RM", "LMC"), true)
	if r := e.save(t, 2, 201, e.input(7, "LM", "RM"), true); r.Final.Malus != 26 {
		t.Fatalf("run 3 malus %v, want 26 (LM run 1, RM run 2)", r.Final.Malus)
	}
	e.save(t, 0, 201, e.input(7, "RMC", "LS"), true)
	if _, fm := e.storedFlight(t, 2, 201); fm.Malus != 13 {
		t.Errorf("run 3 malus after run 1 re-save %v, want 13", fm.Malus)
	}
	e.save(t, 1, 201, e.input(7, "LMC", "RS"), true)
	if _, fm := e.storedFlight(t, 2, 201); fm.Malus != 0 {
		t.Errorf("run 3 malus after run 2 re-save %v, want 0", fm.Malus)
	}
	total := 0.0
	for run := range 3 {
		row, _ := e.storedFlight(t, run, 201)
		total = scoring.Round3(total + row.Score.Float64)
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "overall"); !sameLines(got, rankLine{201, total}) {
		t.Errorf("competition total %v, want %v", got, total)
	}
}

func TestSynchroTipTouches(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	cat, err := os.ReadFile("../../internal/catalog/data/2025.json")
	ok(t, err)
	must(f.svc.ImportCatalog(ctx, 2025, bytes.NewReader(cat)))(t)
	for _, p := range []int{201, 202, 203, 204} {
		must(f.svc.ImportPilotFromCIVL(ctx, p))(t)
	}
	t1 := must(f.svc.CreateTeam(ctx, TeamInput{Name: "Team A", Pilots: []int{201, 202}}))(t)
	t2 := must(f.svc.CreateTeam(ctx, TeamInput{Name: "Team B", Pilots: []int{203, 204}}))(t)
	j1 := must(f.svc.CreateJudge(ctx, JudgeInput{Name: "J1", Country: "fra", Level: domain.JudgeSenior}))(t)
	j2 := must(f.svc.CreateJudge(ctx, JudgeInput{Name: "J2", Country: "fra", Level: domain.JudgeSenior}))(t)
	c := must(f.svc.CreateCompetition(ctx, CompetitionInput{
		Name: "Synchro Cup 2025", Type: domain.TypeSynchro, StartDate: date("2025-07-01"), EndDate: date("2025-07-02"),
	}))(t)
	ok(t, f.svc.SetParticipants(ctx, c.ID, nil, []string{t1.ID, t2.ID}))
	ok(t, f.svc.SetJudges(ctx, c.ID, []string{j1.ID, j2.ID}))
	ok(t, f.svc.OpenCompetition(ctx, c.ID))
	must(f.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone))(t)
	ok(t, f.svc.OpenRun(ctx, c.ID, 0))
	in := FlightInput{Tricks: []string{"LM", "RM"}, TipTouches: 2}
	for _, j := range []string{j1.ID, j2.ID} {
		in.Marks = append(in.Marks, MarkInput{Judge: j, Technical: ptr(7), Choreography: ptr(6), Landing: ptr(6), Synchro: ptr(8)})
	}
	r := must(f.svc.SaveFlight(ctx, c.ID, 0, t1.ID, in, true, ""))(t)
	if r.Final.MarkType != scoring.MarkClassic || r.Final.BonusPercentage != 4 || !hasString(r.Final.Notes, "4% tip-touch bonus") || r.Final.Synchro == 0 {
		t.Errorf("synchro flight: %+v", r.Final)
	}
	rows, err := gen.New(f.db.Read).ListRankingEntries(ctx, gen.ListRankingEntriesParams{Scope: store.ScopeCompetition, ScopeKey: c.ID, ListKey: "overall"})
	ok(t, err)
	if len(rows) != 1 || rows[0].TeamID.String != t1.ID || rows[0].Score != scoring.Round3(r.Final.Score) {
		t.Errorf("synchro ranking %+v", rows)
	}
	if _, err := f.svc.SaveFlight(ctx, c.ID, 0, "201", in, true, ""); err == nil {
		t.Error("pilot id accepted in a synchro run")
	}
}

// Deleting an earlier flight rescores the participant's later runs (the
// repetition malus depends on them) and rebuilds the rankings.
func TestDeleteEarlierRunRescoresLaterRuns(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	e.save(t, 0, 201, e.input(7, "LM", "RMC", "LS"), true)
	must(e.svc.NewRun(ctx, e.comp.ID, 0, scoring.ResetNone))(t)
	ok(t, e.svc.OpenRun(ctx, e.comp.ID, 1))
	if r2 := e.save(t, 1, 201, e.input(7, "LM", "RM", "RS"), true); r2.Final.Malus != 13 {
		t.Fatalf("run 2 malus %v, want 13 (LM repeated)", r2.Final.Malus)
	}
	before, _ := e.storedFlight(t, 1, 201)
	ok(t, e.svc.DeleteFlight(ctx, e.comp.ID, 0, "201", e.version(t, 0, 201)))
	after, fm := e.storedFlight(t, 1, 201)
	if fm.Malus != 0 || after.Score.Float64 <= before.Score.Float64 {
		t.Errorf("run 2 after deleting run 1: malus %v score %v (before %v)", fm.Malus, after.Score.Float64, before.Score.Float64)
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "overall"); !sameLines(got, rankLine{201, after.Score.Float64}) {
		t.Errorf("competition total %v, want %v", got, after.Score.Float64)
	}
}
