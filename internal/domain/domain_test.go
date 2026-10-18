package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Coupe Icare 2025 – Acro Élite!": "coupe-icare-2025--acro-elite",
		"Şahin Cup":                      "sahin-cup",
		"  Hello  ":                      "hello",
	} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCompetitionTransitions(t *testing.T) {
	st := []CompetitionState{CompetitionInit, CompetitionOpen, CompetitionClosed}
	ok := map[[2]CompetitionState]bool{
		{CompetitionInit, CompetitionOpen}:   true,
		{CompetitionOpen, CompetitionClosed}: true,
		{CompetitionClosed, CompetitionOpen}: true,
	}
	for _, f := range st {
		for _, to := range st {
			err := CanTransition(f, to)
			if ok[[2]CompetitionState{f, to}] != (err == nil) {
				t.Errorf("%s->%s err=%v", f, to, err)
			}
			if err != nil && !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("not sentinel: %v", err)
			}
		}
	}
}

func TestRunTransitions(t *testing.T) {
	st := []RunState{RunInit, RunOpen, RunClosed}
	ok := map[[2]RunState]bool{
		{RunInit, RunOpen}:   true,
		{RunOpen, RunClosed}: true,
		{RunClosed, RunOpen}: true,
	}
	for _, f := range st {
		for _, to := range st {
			if ok[[2]RunState{f, to}] != (RunTransition(f, to) == nil) {
				t.Errorf("%s->%s", f, to)
			}
		}
	}
}

func TestCloseCompetitionAndGuards(t *testing.T) {
	if err := CloseCompetitionCheck([]RunState{RunClosed, RunClosed}); err != nil {
		t.Fatal(err)
	}
	err := CloseCompetitionCheck([]RunState{RunClosed, RunOpen, RunInit})
	if err == nil || !strings.Contains(err.Error(), "2, 3") {
		t.Fatalf("got %v", err)
	}
	if CanCreateRun(CompetitionInit) == nil || CanCreateRun(CompetitionOpen) != nil {
		t.Error("CanCreateRun")
	}
	if CanSaveFlight(RunClosed) == nil || CanSaveFlight(RunInit) == nil || CanSaveFlight(RunOpen) != nil {
		t.Error("CanSaveFlight")
	}
	if CanUpdateCompetition(CompetitionClosed, false) == nil ||
		CanUpdateCompetition(CompetitionOpen, true) == nil ||
		CanUpdateCompetition(CompetitionOpen, false) != nil ||
		CanUpdateCompetition(CompetitionInit, true) != nil {
		t.Error("CanUpdateCompetition")
	}
}

func fields(errs []FieldError) string {
	var f []string
	for _, e := range errs {
		f = append(f, e.Field)
	}
	return strings.Join(f, ",")
}

func TestValidateCompetition(t *testing.T) {
	d1 := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
	d2 := d1.AddDate(0, 0, 3)
	base := Competition{Name: "x", Code: "coupe-icare", StartDate: d1, EndDate: d2, Type: TypeSolo, State: CompetitionInit}
	if errs := ValidateCompetition(base); len(errs) != 0 {
		t.Fatalf("init valid: %v", errs)
	}
	c := base
	c.State = CompetitionOpen
	c.Pilots = []int{1}
	c.Judges = []string{"a"}
	if got := fields(ValidateCompetition(c)); got != "pilots,judges" {
		t.Errorf("open few: %s", got)
	}
	c.Pilots = []int{1, 2}
	c.Judges = []string{"a", "b"}
	if errs := ValidateCompetition(c); len(errs) != 0 {
		t.Errorf("open ok: %v", errs)
	}
	c.Pilots = []int{1, 1}
	if got := fields(ValidateCompetition(c)); got != "pilots" {
		t.Errorf("dup: %s", got)
	}
	s := base
	s.Type, s.State, s.Teams, s.Judges = TypeSynchro, CompetitionOpen, []string{"t1"}, []string{"a", "b"}
	if got := fields(ValidateCompetition(s)); got != "teams" {
		t.Errorf("synchro: %s", got)
	}
	b := base
	b.StartDate, b.EndDate = d2, d1
	if got := fields(ValidateCompetition(b)); got != "start_date" {
		t.Errorf("dates: %s", got)
	}
	b = base
	b.StartDate, b.EndDate = d1, d1
	if len(ValidateCompetition(b)) != 0 {
		t.Error("same-day must be valid")
	}
	b.Code = "1bad"
	b.Type = "other"
	if got := fields(ValidateCompetition(b)); got != "type,code" {
		t.Errorf("code/type: %s", got)
	}
}

func TestValidateSeason(t *testing.T) {
	if errs := ValidateSeason(Season{Name: "AWT 2025", Code: "awt-2025", Year: 2025}); len(errs) != 0 {
		t.Errorf("valid: %v", errs)
	}
	s := Season{Name: "France", Code: "xxx-2025", Year: 2025, Country: "fra"}
	if s.EffectiveCode() != "fra-2025" || len(ValidateSeason(s)) != 0 {
		t.Errorf("forced code: %q %v", s.EffectiveCode(), ValidateSeason(s))
	}
	if got := fields(ValidateSeason(Season{Name: "AWT", Code: "awt2025", Year: 2025})); got != "code" {
		t.Errorf("pattern: %s", got)
	}
	if got := fields(ValidateSeason(Season{Name: "AWT", Code: "awt-2024", Year: 2025})); got != "code" {
		t.Errorf("year suffix: %s", got)
	}
	if got := fields(ValidateSeason(Season{Name: "A", Code: "awt-1900", Year: 1900})); got != "name,year" {
		t.Errorf("name/year: %s", got)
	}
}

func TestValidateTeamJudge(t *testing.T) {
	if len(ValidateTeam(Team{Name: "x", Pilots: []int{1, 2}})) != 0 || len(ValidateTeam(Team{Name: "x", Pilots: []int{1}})) != 1 {
		t.Error("team")
	}
	if len(ValidateJudge(Judge{Name: "Jo", Country: "fra", Level: JudgeSenior})) != 0 {
		t.Error("judge ok")
	}
	if got := fields(ValidateJudge(Judge{Name: "J", Country: "fr", Level: "x"})); got != "name,country,level" {
		t.Errorf("judge: %s", got)
	}
}

func TestValidateMark(t *testing.T) {
	for in, want := range map[string]float64{"7.5": 7.5, "7,5": 7.5, "0": 0, "10": 10, " 8 ": 8} {
		v, err := ValidateMark(in)
		if err != nil || v == nil || *v != want {
			t.Errorf("%q: %v %v", in, v, err)
		}
	}
	for _, in := range []string{"", "  "} {
		if v, err := ValidateMark(in); v != nil || err != nil {
			t.Errorf("blank %q: %v %v", in, v, err)
		}
	}
	for _, in := range []string{"75", "-1", "-0", "-0.0", "10.01"} {
		if _, err := ValidateMark(in); err == nil || err.Error() != "mark must be between 0 and 10" {
			t.Errorf("%q: %v", in, err)
		}
	}
	// Only digits with at most one "." or "," separator: no sign, exponent,
	// hex, digit separators or second separator.
	for _, in := range []string{"abc", "NaN", "Inf", "+5", "1e1", "0x1p3", "1_0", "7.5.1", "7,5,1", "7.", ".5", "7 5", "- 1", "-abc"} {
		if _, err := ValidateMark(in); err == nil || errors.Is(err, ErrMarkRange) {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestCloseRunCheck(t *testing.T) {
	ps := []Participant{{"1", "Ann"}, {"2", "Bob"}, {"3", ""}}
	if err := CloseRunCheck(ps, []FlightRef{{"1", true}, {"2", true}, {"3", true}}); err != nil {
		t.Fatal(err)
	}
	err := CloseRunCheck(ps, []FlightRef{{"1", true}, {"2", false}})
	if err == nil || !strings.Contains(err.Error(), "Bob, 3") || strings.Contains(err.Error(), "Ann") {
		t.Fatalf("got %v", err)
	}
	if CloseRunCheck(nil, nil) != nil {
		t.Error("empty run")
	}
}
