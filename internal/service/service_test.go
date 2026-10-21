package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"awt/internal/blob"
	"awt/internal/civl"
	"awt/internal/domain"
	"awt/internal/scoring"
	"awt/internal/store"
	"awt/internal/store/gen"
)

type fakeCIVL struct {
	ranks map[int]int
}

// missingCIVLID is a pilot the fake CIVL site does not know (404).
const missingCIVLID = 404404

func (f *fakeCIVL) FetchPilot(_ context.Context, civlid int) (civl.PilotData, error) {
	if civlid == missingCIVLID {
		return civl.PilotData{}, fmt.Errorf("fetch: %w", civl.ErrNotFound)
	}
	return civl.PilotData{
		CivlID: civlid, Name: fmt.Sprintf("Pilot %d", civlid), Country: "fra",
		CivlLink:    fmt.Sprintf("https://civlcomps.org/pilot/%d", civlid),
		SocialLinks: []civl.Link{{Name: "web", Link: "https://example.org"}},
	}, nil
}

func (f *fakeCIVL) FetchRankings(context.Context) (map[int]int, error) { return f.ranks, nil }

const catalog2025 = `[
 {"name": "Misty Flip", "acronym": "M", "solo": true, "synchro": false, "directions": ["left", "right"],
  "technical_coefficient": 1.65, "bonuses": [{"name": "twisted", "bonus": 2.5}]},
 {"name": "Tumbling", "acronym": "T", "solo": true, "synchro": true, "directions": [],
  "technical_coefficient": 1.8, "bonuses": [], "repeatable": true, "types": ["tumbling"]}
]`

type fixture struct {
	svc  *Service
	db   *store.DB
	civl *fakeCIVL
	bus  *Bus
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.NewFS(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{db: db, civl: &fakeCIVL{}, bus: NewBus()}
	f.svc = New(db, blobs, f.bus, f.civl)
	return f
}

// must(f())(t) fails the test when f returns an error.
func must[T any](v T, err error) func(*testing.T) T {
	return func(t *testing.T) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func ok(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func date(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

// soloSetup creates pilots 101..104 (CIVL ranks 5, 50, 1, unranked), two
// judges, the 2025 catalog, the awt-2025 season and an init solo competition.
func (f *fixture) soloSetup(t *testing.T) (Competition, []string) {
	t.Helper()
	ctx := context.Background()
	for _, id := range []int{101, 102, 103, 104} {
		must(f.svc.ImportPilotFromCIVL(ctx, id))(t)
	}
	f.civl.ranks = map[int]int{101: 5, 102: 50, 103: 1}
	ok(t, f.svc.RefreshRankings(ctx))
	j1 := must(f.svc.CreateJudge(ctx, JudgeInput{Name: "Judge One", Country: "fra", Level: domain.JudgeSenior}))(t)
	j2 := must(f.svc.CreateJudge(ctx, JudgeInput{Name: "Judge Two", Country: "che", Level: domain.JudgeTrainee}))(t)
	must(f.svc.ImportCatalog(ctx, 2025, strings.NewReader(catalog2025)))(t)
	must(f.svc.CreateSeason(ctx, SeasonInput{Name: "AWT 2025", Code: "awt-2025", Year: 2025, SortIndex: 999}))(t)
	c := must(f.svc.CreateCompetition(ctx, CompetitionInput{
		Name: "Test Cup 2025", Type: domain.TypeSolo, StartDate: date("2025-06-01"), EndDate: date("2025-06-03"),
		Location: "Annecy", Seasons: []string{"awt-2025"},
	}))(t)
	return c, []string{j1.ID, j2.ID}
}

func (f *fixture) openSolo(t *testing.T) (Competition, []string) {
	t.Helper()
	ctx := context.Background()
	c, judges := f.soloSetup(t)
	ok(t, f.svc.SetParticipants(ctx, c.ID, []int{103, 101, 104, 102}, nil))
	ok(t, f.svc.SetJudges(ctx, c.ID, judges))
	ok(t, f.svc.OpenCompetition(ctx, c.ID))
	return must(f.svc.GetCompetition(ctx, c.ID))(t), judges
}

// insertFlight stores a flight directly (flight scoring is Task 13).
func (f *fixture) insertFlight(t *testing.T, compID string, run, civlid int, published bool) {
	t.Helper()
	ok(t, f.db.Tx(context.Background(), func(q *gen.Queries) error {
		_, err := q.InsertFlight(context.Background(), gen.InsertFlightParams{
			CompetitionID: compID, RunIdx: int64(run), Civlid: sql.NullInt64{Int64: int64(civlid), Valid: true},
			Published: published, Score: sql.NullFloat64{Float64: 5, Valid: true},
		})
		return err
	}))
}

func TestCreateCompetitionDefaults(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c, _ := f.soloSetup(t)
	if c.Code != "test-cup-2025" || c.State != domain.CompetitionInit || c.Frozen {
		t.Errorf("code/state: %+v", c)
	}
	rs := must(gen.New(f.db.Read).GetRuleset(ctx, 2025))(t)
	if string(c.Config) != rs.Config {
		t.Errorf("config not copied from ruleset 2025:\n%s\n%s", c.Config, rs.Config)
	}
	if !reflect.DeepEqual(c.RepeatableTricks, []string{"Tumbling"}) {
		t.Errorf("repeatable tricks: %v", c.RepeatableTricks)
	}
	if !reflect.DeepEqual(c.Seasons, []string{"awt-2025"}) {
		t.Errorf("seasons: %v", c.Seasons)
	}

	_, err := f.svc.CreateCompetition(ctx, CompetitionInput{
		Name: "Future Cup", Type: domain.TypeSolo, StartDate: date("2031-06-01"), EndDate: date("2031-06-02"),
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || !strings.Contains(err.Error(), "2031") {
		t.Errorf("no ruleset: %v", err)
	}
	_, err = f.svc.CreateCompetition(ctx, CompetitionInput{
		Name: "Test Cup 2025", Type: domain.TypeSolo, StartDate: date("2025-06-01"), EndDate: date("2025-06-01"),
	})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate name: %v", err)
	}
	_, err = f.svc.CreateCompetition(ctx, CompetitionInput{
		Name: "Other", Type: domain.TypeSolo, StartDate: date("2025-06-01"), EndDate: date("2025-06-01"),
		Seasons: []string{"xyz-2025"},
	})
	if !errors.As(err, &ve) {
		t.Errorf("unknown season: %v", err)
	}
}

func TestPilotsOrderedByRankDesc(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c, _ := f.soloSetup(t)
	ok(t, f.svc.SetParticipants(ctx, c.ID, []int{103, 101, 104, 102}, nil))
	got := must(f.svc.GetCompetition(ctx, c.ID))(t).Pilots
	if want := []int{104, 102, 101, 103}; !reflect.DeepEqual(got, want) {
		t.Errorf("pilots %v want %v", got, want)
	}
	var ve *ValidationError
	if err := f.svc.SetParticipants(ctx, c.ID, []int{101, 999}, nil); !errors.As(err, &ve) {
		t.Errorf("unknown pilot: %v", err)
	}
	if err := f.svc.SetParticipants(ctx, c.ID, nil, []string{"x"}); !errors.As(err, &ve) {
		t.Errorf("teams in solo: %v", err)
	}
}

func TestCompetitionLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c, judges := f.soloSetup(t)
	events, unsubscribe := f.bus.Subscribe(competitionTopic(c.Code))
	defer unsubscribe()

	var ve *ValidationError
	if err := f.svc.OpenCompetition(ctx, c.ID); !errors.As(err, &ve) {
		t.Fatalf("open without pilots: %v", err)
	}
	ok(t, f.svc.SetParticipants(ctx, c.ID, []int{103, 101, 104, 102}, nil))
	ok(t, f.svc.SetJudges(ctx, c.ID, judges))
	if _, err := f.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("run in init: %v", err)
	}
	ok(t, f.svc.OpenCompetition(ctx, c.ID))
	if err := f.svc.OpenCompetition(ctx, c.ID); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("open twice: %v", err)
	}
	// name is frozen once open
	in := CompetitionInput{Name: "Renamed", Type: domain.TypeSolo, StartDate: date("2025-06-01"), EndDate: date("2025-06-03")}
	if _, err := f.svc.UpdateCompetition(ctx, c.ID, in); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("rename when open: %v", err)
	}

	r := must(f.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone))(t)
	if r.Index != 0 || r.State != domain.RunInit {
		t.Errorf("run: %+v", r)
	}
	if want := []int{104, 102, 101, 103}; !reflect.DeepEqual(r.Pilots, want) {
		t.Errorf("run 1 order %v want %v (CIVL rank desc)", r.Pilots, want)
	}
	if !reflect.DeepEqual(r.Judges, judges) || string(r.Config) != string(c.Config) {
		t.Errorf("judges/config not copied: %+v", r)
	}
	if err := f.svc.CloseRun(ctx, c.ID, 0); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("close init run: %v", err)
	}
	ok(t, f.svc.OpenRun(ctx, c.ID, 0))

	err := f.svc.CloseCompetition(ctx, c.ID)
	if !errors.Is(err, domain.ErrInvalidTransition) || !strings.Contains(err.Error(), "runs not closed: 1") {
		t.Errorf("close comp with open run: %v", err)
	}

	for _, p := range []int{104, 102, 101} {
		f.insertFlight(t, c.ID, 0, p, true)
	}
	f.insertFlight(t, c.ID, 0, 103, false)
	err = f.svc.CloseRun(ctx, c.ID, 0)
	if !errors.Is(err, domain.ErrInvalidTransition) || !strings.Contains(err.Error(), "Pilot 103") ||
		strings.Contains(err.Error(), "Pilot 101") {
		t.Errorf("close run with unpublished flight: %v", err)
	}
	_, err = f.db.Write.ExecContext(ctx, `UPDATE flights SET published = 1`)
	ok(t, err)
	ok(t, f.svc.CloseRun(ctx, c.ID, 0))
	ok(t, f.svc.ReopenRun(ctx, c.ID, 0))
	ok(t, f.svc.CloseRun(ctx, c.ID, 0))
	ok(t, f.svc.CloseCompetition(ctx, c.ID))
	if err := f.svc.SetJudges(ctx, c.ID, judges); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("update closed: %v", err)
	}
	ok(t, f.svc.ReopenCompetition(ctx, c.ID))
	if got := must(f.svc.GetCompetition(ctx, c.ID))(t).State; got != domain.CompetitionOpen {
		t.Errorf("state %s", got)
	}

	kinds := map[string]int{}
	for len(events) > 0 {
		e := <-events
		kinds[e.Kind]++
	}
	if kinds[KindRun] < 4 || kinds[KindCompetition] < 3 {
		t.Errorf("events: %v", kinds)
	}

	ok(t, f.svc.DeleteCompetition(ctx, c.ID))
	if _, err := f.svc.GetCompetition(ctx, c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted: %v", err)
	}
	ok(t, f.svc.RestoreCompetition(ctx, c.ID))
	if got := must(f.svc.GetCompetition(ctx, c.ID))(t).State; got != domain.CompetitionOpen {
		t.Errorf("restore keeps state, got %s", got)
	}
}

func setCompetitionRanking(t *testing.T, f *fixture, compID string, civlids ...int) {
	t.Helper()
	ok(t, f.db.Tx(context.Background(), func(q *gen.Queries) error {
		if err := q.DeleteRankingEntries(context.Background(), gen.DeleteRankingEntriesParams{Scope: "competition", ScopeKey: compID, ListKey: "overall"}); err != nil {
			return err
		}
		for i, id := range civlids {
			err := q.InsertRankingEntry(context.Background(), gen.InsertRankingEntryParams{
				Scope: "competition", ScopeKey: compID, ListKey: "overall", Position: int64(i + 1), Rank: int64(i + 1),
				Civlid: sql.NullInt64{Int64: int64(id), Valid: true}, Score: float64(10 - i), Details: "{}",
			})
			if err != nil {
				return err
			}
		}
		return nil
	}))
}

func TestNewRunStartOrder(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c, _ := f.openSolo(t)
	must(f.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone))(t)

	// competition ranking: 101 leads, then 104, 102, 103
	setCompetitionRanking(t, f, c.ID, 101, 104, 102, 103)
	r2 := must(f.svc.NewRun(ctx, c.ID, 0, scoring.ResetAll))(t)
	if want := []int{103, 102, 104, 101}; !reflect.DeepEqual(r2.Pilots, want) {
		t.Errorf("run 2 order %v want %v (leader last)", r2.Pilots, want)
	}
	if r2.Index != 1 || r2.ResetPolicy != scoring.ResetAll {
		t.Errorf("run 2: %+v", r2)
	}
	r1 := must(f.svc.GetRun(ctx, c.ID, 0))(t)
	if want := []int{104, 102, 101, 103}; !reflect.DeepEqual(r1.Pilots, want) {
		t.Errorf("run 1 order modified: %v (decision 0006 #2)", r1.Pilots)
	}

	r3 := must(f.svc.NewRun(ctx, c.ID, 2, scoring.ResetNone))(t)
	if want := []int{104, 101}; !reflect.DeepEqual(r3.Pilots, want) {
		t.Errorf("run 3 qualify 2: %v want %v", r3.Pilots, want)
	}

	var ve *ValidationError
	if _, err := f.svc.NewRun(ctx, c.ID, 0, scoring.ResetPolicy(7)); !errors.As(err, &ve) {
		t.Errorf("bad policy: %v", err)
	}

	ok(t, f.svc.SetRunOrder(ctx, c.ID, 2, []int{101, 103}, nil))
	if got := must(f.svc.GetRun(ctx, c.ID, 2))(t).Pilots; !reflect.DeepEqual(got, []int{101, 103}) {
		t.Errorf("set order: %v", got)
	}
	if err := f.svc.SetRunOrder(ctx, c.ID, 2, []int{101, 101}, nil); !errors.As(err, &ve) {
		t.Errorf("duplicate: %v", err)
	}
	if err := f.svc.SetRunOrder(ctx, c.ID, 2, []int{101, 999}, nil); !errors.As(err, &ve) {
		t.Errorf("not a participant: %v", err)
	}
	// the next run starts from the previous run's list, not the competition's
	r4 := must(f.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone))(t)
	if want := []int{103, 101}; !reflect.DeepEqual(r4.Pilots, want) {
		t.Errorf("run 4 from run 3 list: %v want %v", r4.Pilots, want)
	}
}

func TestSynchroKeepsTeamOrder(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, id := range []int{1, 2, 3, 4, 5, 6} {
		must(f.svc.ImportPilotFromCIVL(ctx, id))(t)
	}
	var teams []string
	for i, ps := range [][]int{{1, 2}, {3, 4}, {5, 6}} {
		teams = append(teams, must(f.svc.CreateTeam(ctx, TeamInput{Name: fmt.Sprintf("Team %d", i), Pilots: ps}))(t).ID)
	}
	j1 := must(f.svc.CreateJudge(ctx, JudgeInput{Name: "Judge One", Country: "fra", Level: domain.JudgeSenior}))(t)
	j2 := must(f.svc.CreateJudge(ctx, JudgeInput{Name: "Judge Two", Country: "fra", Level: domain.JudgeSenior}))(t)
	c := must(f.svc.CreateCompetition(ctx, CompetitionInput{
		Name: "Synchro Cup", Type: domain.TypeSynchro, StartDate: date("2025-06-01"), EndDate: date("2025-06-01"),
	}))(t)
	order := []string{teams[2], teams[0], teams[1]}
	var ve *ValidationError
	if err := f.svc.SetParticipants(ctx, c.ID, []int{1}, nil); !errors.As(err, &ve) {
		t.Errorf("pilots in synchro: %v", err)
	}
	ok(t, f.svc.SetParticipants(ctx, c.ID, nil, order))
	ok(t, f.svc.SetJudges(ctx, c.ID, []string{j1.ID, j2.ID}))
	ok(t, f.svc.OpenCompetition(ctx, c.ID))
	r := must(f.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone))(t)
	if !reflect.DeepEqual(r.Teams, order) || len(r.Pilots) != 0 {
		t.Errorf("run 1 teams %v want %v", r.Teams, order)
	}
}

func TestFrozenCompetitionRefusesMutations(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c, judges := f.openSolo(t)
	must(f.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone))(t)
	_, err := f.db.Write.ExecContext(ctx, `UPDATE competitions SET frozen = 1 WHERE id = ?`, c.ID)
	ok(t, err)

	in := CompetitionInput{Name: c.Name, Type: c.Type, StartDate: c.StartDate, EndDate: c.EndDate}
	ops := map[string]func() error{
		"update":     func() error { _, err := f.svc.UpdateCompetition(ctx, c.ID, in); return err },
		"pilots":     func() error { return f.svc.SetParticipants(ctx, c.ID, []int{101, 102}, nil) },
		"judges":     func() error { return f.svc.SetJudges(ctx, c.ID, judges) },
		"repeatable": func() error { return f.svc.SetRepeatableTricks(ctx, c.ID, nil) },
		"config":     func() error { return f.svc.SetConfig(ctx, c.ID, c.Config) },
		"open":       func() error { return f.svc.OpenCompetition(ctx, c.ID) },
		"close":      func() error { return f.svc.CloseCompetition(ctx, c.ID) },
		"reopen":     func() error { return f.svc.ReopenCompetition(ctx, c.ID) },
		"delete":     func() error { return f.svc.DeleteCompetition(ctx, c.ID) },
		"new run":    func() error { _, err := f.svc.NewRun(ctx, c.ID, 0, scoring.ResetNone); return err },
		"run order":  func() error { return f.svc.SetRunOrder(ctx, c.ID, 0, []int{101}, nil) },
		"open run":   func() error { return f.svc.OpenRun(ctx, c.ID, 0) },
		"close run":  func() error { return f.svc.CloseRun(ctx, c.ID, 0) },
		"reopen run": func() error { return f.svc.ReopenRun(ctx, c.ID, 0) },
	}
	for name, op := range ops {
		if err := op(); !errors.Is(err, ErrFrozen) {
			t.Errorf("%s: %v, want ErrFrozen", name, err)
		}
	}
}

func TestSetConfigAndRepeatableTricks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c, _ := f.soloSetup(t)
	var ve *ValidationError
	if err := f.svc.SetConfig(ctx, c.ID, json.RawMessage(`{"warning": "x"}`)); !errors.As(err, &ve) {
		t.Errorf("bad config: %v", err)
	}
	cfg := strings.Replace(string(c.Config), `"warning":0.5`, `"warning":0.7`, 1)
	if cfg == string(c.Config) {
		t.Fatalf("fixture: config shape changed: %s", c.Config)
	}
	for name, bad := range map[string]string{
		"missing key":     strings.Replace(cfg, `"warning2":1,`, ``, 1),
		"missing level":   strings.Replace(cfg, `"trainee":20`, `"novice":20`, 1),
		"percent sum":     strings.Replace(cfg, `"landing":20`, `"landing":25`, 1),
		"negative weight": strings.Replace(cfg, `"trainee":20`, `"trainee":-1`, 1),
		"dsq 0":           strings.Replace(cfg, `"warnings_to_dsq":3`, `"warnings_to_dsq":0`, 1),
		"no flip limit":   strings.Replace(cfg, `,"flip":2`, ``, 1),
	} {
		if bad == cfg {
			t.Fatalf("%s: fixture config shape changed: %s", name, cfg)
		}
		if err := f.svc.SetConfig(ctx, c.ID, json.RawMessage(bad)); !errors.As(err, &ve) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
	events, unsubscribe := f.bus.Subscribe(competitionTopic(c.Code))
	defer unsubscribe()
	ok(t, f.svc.SetConfig(ctx, c.ID, json.RawMessage(cfg)))
	if len(events) != 1 {
		t.Errorf("SetConfig published %d events", len(events))
	}
	if got := must(f.svc.GetCompetition(ctx, c.ID))(t).Config; !strings.Contains(string(got), `"warning":0.7`) {
		t.Errorf("config: %s", got)
	}
	ok(t, f.svc.SetRepeatableTricks(ctx, c.ID, []string{"Misty Flip", "Tumbling"}))
	if got := must(f.svc.GetCompetition(ctx, c.ID))(t).RepeatableTricks; !reflect.DeepEqual(got, []string{"Misty Flip", "Tumbling"}) {
		t.Errorf("repeatable: %v", got)
	}
	if err := f.svc.SetRepeatableTricks(ctx, c.ID, []string{"Nope"}); !errors.As(err, &ve) {
		t.Errorf("unknown trick: %v", err)
	}
}

func uniqueAcronyms(t *testing.T, f *fixture, year int) []string {
	t.Helper()
	var out []string
	for _, u := range must(f.svc.ListUniqueTricks(context.Background(), year))(t) {
		out = append(out, u.Acronym)
	}
	return out
}

func TestTrickDeleteRestoreRegeneratesUniqueTricks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if n := must(f.svc.ImportCatalog(ctx, 2025, strings.NewReader(catalog2025)))(t); n != 2 {
		t.Fatalf("imported %d", n)
	}
	tricks := must(f.svc.ListTricks(ctx, 2025))(t)
	misty := tricks[0]
	if misty.Name != "Misty Flip" || len(misty.Tricks) != 4 {
		t.Fatalf("misty: %+v", misty)
	}
	if got := uniqueAcronyms(t, f, 2025); !reflect.DeepEqual(got, []string{"/LM", "/RM", "LM", "RM", "T"}) {
		t.Errorf("unique tricks: %v", got)
	}

	ok(t, f.svc.DeleteTrick(ctx, misty.ID))
	if got := uniqueAcronyms(t, f, 2025); !reflect.DeepEqual(got, []string{"T"}) {
		t.Errorf("after delete: %v", got)
	}
	// the acronym is free again
	other := must(f.svc.CreateTrick(ctx, 2025, misty.Trick))(t)
	if _, err := f.svc.CreateTrick(ctx, 2025, misty.Trick); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate trick: %v", err)
	}
	if err := f.svc.RestoreTrick(ctx, misty.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("restore over active duplicate: %v", err)
	}
	ok(t, f.svc.DeleteTrick(ctx, other.ID))
	ok(t, f.svc.RestoreTrick(ctx, misty.ID))
	if got := uniqueAcronyms(t, f, 2025); !reflect.DeepEqual(got, []string{"/LM", "/RM", "LM", "RM", "T"}) {
		t.Errorf("after restore: %v", got)
	}

	upd := misty.Trick
	upd.Acronym = "MF"
	upd.Bonuses = nil
	must(f.svc.UpdateTrick(ctx, misty.ID, upd))(t)
	if got := uniqueAcronyms(t, f, 2025); !reflect.DeepEqual(got, []string{"LMF", "RMF", "T"}) {
		t.Errorf("after update: %v", got)
	}

	// re-import upserts by (year, name): same id, variants regenerated
	must(f.svc.ImportCatalog(ctx, 2025, strings.NewReader(catalog2025)))(t)
	again := must(f.svc.ListTricks(ctx, 2025))(t)
	if len(again) != 2 || again[0].ID != misty.ID || again[0].Acronym != "M" {
		t.Errorf("re-import: %+v", again)
	}

	var ve *ValidationError
	bad := misty.Trick
	bad.Name, bad.Acronym, bad.Directions = "Bad", "B", []string{"left"}
	if _, err := f.svc.CreateTrick(ctx, 2025, bad); !errors.As(err, &ve) {
		t.Errorf("one direction: %v", err)
	}
}

func TestPilotCIVLImportKeepsManualFields(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := must(f.svc.ImportPilotFromCIVL(ctx, 7))(t)
	if p.Name != "Pilot 7" || p.Rank != 9999 || p.Gender != domain.GenderMan {
		t.Errorf("imported: %+v", p)
	}
	if g := must(f.svc.ToggleGender(ctx, 7))(t); g != domain.GenderWoman {
		t.Errorf("toggle: %s", g)
	}
	ok(t, f.svc.SetAWTYear(ctx, 7, 2025, true))
	ok(t, f.svc.SetAWTYear(ctx, 7, 2024, true))
	ok(t, f.svc.SetAWTYear(ctx, 7, 2024, false))
	f.civl.ranks = map[int]int{7: 12}
	ok(t, f.svc.RefreshRankings(ctx))
	p = must(f.svc.ImportPilotFromCIVL(ctx, 7))(t)
	if p.Gender != domain.GenderWoman || !reflect.DeepEqual(p.AWTYears, []int{2025}) || p.Rank != 12 {
		t.Errorf("re-import lost manual fields: %+v", p)
	}
	f.civl.ranks = map[int]int{}
	if err := f.svc.RefreshRankings(ctx); err == nil || !strings.Contains(err.Error(), "CIVL returned no rankings") {
		t.Errorf("empty ranking accepted: %v", err)
	}
	if p := must(f.svc.GetPilot(ctx, 7))(t); p.Rank != 12 {
		t.Errorf("empty ranking changed rank: %d", p.Rank)
	}
	f.civl.ranks = map[int]int{8: 1}
	ok(t, f.svc.RefreshRankings(ctx))
	if p := must(f.svc.GetPilot(ctx, 7))(t); p.Rank != 9999 {
		t.Errorf("unranked: %d", p.Rank)
	}
	var ve *ValidationError
	for _, y := range []int{1989, 2101} {
		if err := f.svc.SetAWTYear(ctx, 7, y, true); !errors.As(err, &ve) {
			t.Errorf("AWT year %d: %v", y, err)
		}
	}
	if _, err := f.svc.ImportPilotFromCIVL(ctx, missingCIVLID); !errors.Is(err, ErrNotFound) {
		t.Errorf("pilot unknown to CIVL: %v", err)
	}
	_, err := f.db.Write.ExecContext(ctx, `UPDATE pilots SET gender = 'none'`)
	ok(t, err)
	if g := must(f.svc.ToggleGender(ctx, 7))(t); g != domain.GenderNone {
		t.Errorf("none stays none: %s", g)
	}
	if _, err := f.svc.ToggleGender(ctx, 8); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown pilot: %v", err)
	}
}

func TestTeamsJudgesSeasons(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	must(f.svc.ImportPilotFromCIVL(ctx, 1))(t)
	must(f.svc.ImportPilotFromCIVL(ctx, 2))(t)
	tm := must(f.svc.CreateTeam(ctx, TeamInput{Name: "Duo", Pilots: []int{2, 1}}))(t)
	if got := must(f.svc.GetTeam(ctx, tm.ID))(t); !reflect.DeepEqual(got.Pilots, []int{2, 1}) || len(tm.ID) != 24 {
		t.Errorf("team: %+v", got)
	}
	if _, err := f.svc.CreateTeam(ctx, TeamInput{Name: "Duo", Pilots: []int{1, 2}}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate team: %v", err)
	}
	var ve *ValidationError
	for _, ps := range [][]int{{1}, {1, 1}, {1, 3}} {
		if _, err := f.svc.CreateTeam(ctx, TeamInput{Name: "Bad", Pilots: ps}); !errors.As(err, &ve) {
			t.Errorf("team %v: %v", ps, err)
		}
	}
	ok(t, f.svc.DeleteTeam(ctx, tm.ID))
	if len(must(f.svc.ListTeams(ctx))(t)) != 0 {
		t.Error("deleted team listed")
	}
	if err := f.svc.DeleteTeam(ctx, tm.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete twice: %v", err)
	}
	ok(t, f.svc.RestoreTeam(ctx, tm.ID))

	j := must(f.svc.CreateJudge(ctx, JudgeInput{Name: "Ann", Country: "FRA", Level: domain.JudgeCertified, CivlID: 1}))(t)
	if j.Country != "fra" || j.CivlID != 1 {
		t.Errorf("judge: %+v", j)
	}
	if _, err := f.svc.CreateJudge(ctx, JudgeInput{Name: "Bob", Country: "fra", Level: domain.JudgeSenior, CivlID: 99}); !errors.As(err, &ve) {
		t.Errorf("unknown civlid: %v", err)
	}
	j = must(f.svc.UpdateJudge(ctx, j.ID, JudgeInput{Name: "Ann B", Country: "fra", Level: domain.JudgeSenior}))(t)
	if j.Level != domain.JudgeSenior || j.CivlID != 0 {
		t.Errorf("updated judge: %+v", j)
	}

	s := must(f.svc.CreateSeason(ctx, SeasonInput{Name: "France 2025", Code: "xxx-2025", Year: 2025, Country: "fra", SortIndex: 3}))(t)
	if s.Code != "fra-2025" {
		t.Errorf("forced code: %s", s.Code)
	}
	if _, err := f.svc.CreateSeason(ctx, SeasonInput{Name: "Other", Code: "fra-2025", Year: 2025}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate code: %v", err)
	}
	if _, err := f.svc.CreateSeason(ctx, SeasonInput{Name: "Bad", Code: "bad", Year: 2025}); !errors.As(err, &ve) {
		t.Errorf("bad code: %v", err)
	}
	ok(t, f.svc.DeleteSeason(ctx, s.ID))
	if _, err := f.svc.GetSeason(ctx, "fra-2025"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted season: %v", err)
	}
}

func TestUploadFile(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)
	a := must(f.svc.UploadFile(ctx, "logo.png", bytes.NewReader(png)))(t)
	if a.ContentType != "image/png" || len(a.ID) != 64 || a.Size != int64(len(png)) {
		t.Errorf("file: %+v", a)
	}
	b := must(f.svc.UploadFile(ctx, "other.png", bytes.NewReader(png)))(t)
	if b.ID != a.ID {
		t.Errorf("dedupe: %s %s", a.ID, b.ID)
	}
	var ve *ValidationError
	if _, err := f.svc.UploadFile(ctx, "x.html", strings.NewReader("<html><body>x</body></html>")); !errors.As(err, &ve) {
		t.Errorf("html accepted: %v", err)
	}
	big := append([]byte("%PDF-1.4\n"), make([]byte, maxUploadSize)...)
	if _, err := f.svc.UploadFile(ctx, "big.pdf", bytes.NewReader(big)); !errors.As(err, &ve) {
		t.Errorf("11 MB accepted: %v", err)
	}
}

func TestBus(t *testing.T) {
	b := NewBus()
	ch, unsubscribe := b.Subscribe("competition:x")
	other, unsubscribeOther := b.Subscribe("competition:y")
	defer unsubscribeOther()
	for range subscriberBuffer + 3 {
		b.Publish(Event{Topic: "competition:x", Kind: KindRun})
	}
	if len(ch) != subscriberBuffer || b.Dropped() != 3 {
		t.Errorf("buffered %d dropped %d", len(ch), b.Dropped())
	}
	if len(other) != 0 {
		t.Error("event leaked to another topic")
	}
	unsubscribe()
	unsubscribe()
	n := 0
	for range ch {
		n++
	}
	if n != subscriberBuffer {
		t.Errorf("drained %d", n)
	}
	b.Publish(Event{Topic: "competition:x"}) // no subscriber: must not panic
}

func TestUpdateCompetitionPublishesToSeasons(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c, _ := f.soloSetup(t)
	must(f.svc.CreateSeason(ctx, SeasonInput{Name: "AWQ 2025", Code: "awq-2025", Year: 2025, SortIndex: 1}))(t)
	var chans []<-chan Event
	for _, topic := range []string{competitionTopic(c.Code), seasonTopic("awt-2025"), seasonTopic("awq-2025")} {
		ch, unsubscribe := f.bus.Subscribe(topic)
		defer unsubscribe()
		chans = append(chans, ch)
	}
	in := CompetitionInput{Name: c.Name, Type: c.Type, StartDate: c.StartDate, EndDate: c.EndDate, Seasons: []string{"awq-2025"}}
	must(f.svc.UpdateCompetition(ctx, c.ID, in))(t)
	for i, ch := range chans {
		if len(ch) != 1 {
			t.Errorf("topic %d got %d events", i, len(ch))
		}
	}
}
