package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"awt/internal/domain"
	"awt/internal/store"
)

// Reference-data edits that change an input of stored scores or ranking
// lists rescore and re-rank every affected non-frozen competition in the
// same transaction, then publish after commit (controller ruling, final
// review item 3: compute on write).

// gotEvent reports whether an event arrived on ch within a short delay.
func gotEvent(ch <-chan Event) bool {
	select {
	case <-ch:
		return true
	case <-time.After(200 * time.Millisecond):
		return false
	}
}

func TestSetAWTYearRescoresAffectedCompetitions(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, []int{201}, []string{"awt-2025", "awq-2025"})
	ctx := context.Background()
	e.save(t, 0, 201, e.input(7, "LM", "RMC", "LS"), true)
	e.save(t, 0, 202, e.input(8, "LM", "RMC", "LS"), true)
	comp, unsub := e.bus.Subscribe(competitionTopic(e.comp.Code))
	defer unsub()

	// another year: the competition (2025) is not affected
	ok(t, e.svc.SetAWTYear(ctx, 202, 2024, true))
	if gotEvent(comp) {
		t.Error("event for an AWT year the competition does not use")
	}

	ok(t, e.svc.SetAWTYear(ctx, 202, 2025, true))
	row, fm := e.storedFlight(t, 0, 202)
	if row.MarkType.String != "awt" || fm.MarkType != "awt" {
		t.Errorf("mark type after SetAWTYear: %q/%q, want awt", row.MarkType.String, fm.MarkType)
	}
	awt := e.ranking(t, store.ScopeCompetition, e.comp.ID, "awt-2025")
	if len(awt) != 2 {
		t.Errorf("awt list %v, want both pilots", awt)
	}
	if awq := e.ranking(t, store.ScopeCompetition, e.comp.ID, "awq-2025"); len(awq) != 0 {
		t.Errorf("awq list %v, want empty", awq)
	}
	if !gotEvent(comp) {
		t.Error("no competition event after SetAWTYear")
	}

	// back to AWQ
	ok(t, e.svc.SetAWTYear(ctx, 202, 2025, false))
	if row, _ := e.storedFlight(t, 0, 202); row.MarkType.String != "awq" || row.Score.Float64 != 8.507 {
		t.Errorf("after removing the AWT year: %q %v", row.MarkType.String, row.Score.Float64)
	}
}

func TestSetAWTYearSkipsFrozenCompetitions(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, []int{201}, []string{"awt-2025", "awq-2025"})
	ctx := context.Background()
	e.save(t, 0, 202, e.input(8, "LM", "RMC", "LS"), true)
	_, err := e.db.Write.Exec(`UPDATE competitions SET frozen = 1 WHERE id = ?`, e.comp.ID)
	ok(t, err)
	ok(t, e.svc.SetAWTYear(ctx, 202, 2025, true))
	if row, _ := e.storedFlight(t, 0, 202); row.MarkType.String != "awq" {
		t.Errorf("frozen competition rescored: mark type %q", row.MarkType.String)
	}
}

// An edit the engine cannot apply to a stored flight is an ErrRescore and
// changes nothing.
func TestSetAWTYearRescoreRefused(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, []string{"awt-2025"})
	ctx := context.Background()
	in := e.input(7, "LM", "RMC", "LS")
	for i := range in.Marks {
		in.Marks[i].TechnicalPerTrick = nil // classic marking: refused for AWT pilots
	}
	e.save(t, 0, 202, in, true)
	err := e.svc.SetAWTYear(ctx, 202, 2025, true)
	if !errors.Is(err, ErrRescore) {
		t.Fatalf("err = %v, want ErrRescore", err)
	}
	if p := must(e.svc.GetPilot(ctx, 202))(t); len(p.AWTYears) != 0 {
		t.Errorf("AWT year stored despite the refusal: %v", p.AWTYears)
	}
}

func TestToggleGenderRebuildsWomenList(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202, 203}, nil, nil)
	ctx := context.Background()
	for i, p := range []int{201, 202, 203} {
		e.save(t, 0, p, e.input(float64(6+i), "LM", "RMC", "LS"), true)
	}
	comp, unsub := e.bus.Subscribe(competitionTopic(e.comp.Code))
	defer unsub()
	for _, p := range []int{201, 202, 203} {
		if g := must(e.svc.ToggleGender(ctx, p))(t); g != domain.GenderWoman {
			t.Fatalf("gender %q", g)
		}
	}
	if women := e.ranking(t, store.ScopeCompetition, e.comp.ID, "women"); len(women) != 3 {
		t.Errorf("women list %v, want 3 pilots", women)
	}
	if !gotEvent(comp) {
		t.Error("no competition event after ToggleGender")
	}
	must(e.svc.ToggleGender(ctx, 203))(t)
	if women := e.ranking(t, store.ScopeCompetition, e.comp.ID, "women"); len(women) != 0 {
		t.Errorf("women list %v, want none below 3 women", women)
	}
}

func TestJudgeLevelChangeRescores(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	in := e.input(7, "LM", "RMC", "LS")
	in.Marks[1].Technical, in.Marks[1].Choreography, in.Marks[1].Landing = ptr(3), ptr(3), ptr(3)
	e.save(t, 0, 201, in, true)
	before, _ := e.storedFlight(t, 0, 201)
	comp, unsub := e.bus.Subscribe(competitionTopic(e.comp.Code))
	defer unsub()

	// a rename does not change any score: no rescoring, no event
	j := must(e.svc.GetJudge(ctx, e.judges[1]))(t)
	must(e.svc.UpdateJudge(ctx, j.ID, JudgeInput{Name: "Renamed", Country: j.Country, Level: j.Level}))(t)
	if gotEvent(comp) {
		t.Error("event for a judge rename")
	}

	must(e.svc.UpdateJudge(ctx, j.ID, JudgeInput{Name: "Renamed", Country: j.Country, Level: domain.JudgeTrainee}))(t)
	after, _ := e.storedFlight(t, 0, 201)
	if after.Score.Float64 == before.Score.Float64 {
		t.Errorf("score unchanged after the judge level change: %v", after.Score.Float64)
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "overall"); !sameLines(got, rankLine{201, after.Score.Float64}) {
		t.Errorf("overall %v, want the rescored flight", got)
	}
	if !gotEvent(comp) {
		t.Error("no competition event after the judge level change")
	}
}

func TestSetRepeatableTricksRescores(t *testing.T) {
	e := newFlightEnv(t, []int{201, 202}, nil, nil)
	ctx := context.Background()
	if r := e.save(t, 0, 201, e.input(7, "FS", "LM", "FS"), true); r.Final.Malus != 0 {
		t.Fatalf("repeated Stall: malus %v", r.Final.Malus)
	}
	comp, unsub := e.bus.Subscribe(competitionTopic(e.comp.Code))
	defer unsub()
	ok(t, e.svc.SetRepeatableTricks(ctx, e.comp.ID, nil))
	row, fm := e.storedFlight(t, 0, 201)
	if fm.Malus == 0 {
		t.Errorf("Stall no longer repeatable but no repetition malus: %+v", fm)
	}
	if got := e.ranking(t, store.ScopeCompetition, e.comp.ID, "overall"); !sameLines(got, rankLine{201, row.Score.Float64}) {
		t.Errorf("overall %v, want the rescored flight %v", got, row.Score.Float64)
	}
	if !gotEvent(comp) {
		t.Error("no competition event")
	}
}
