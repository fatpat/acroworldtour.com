package scoring

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"awt/internal/catalog"
)

// Expected values below are the full-precision outputs of the legacy engine
// executed on spec 03 §12 examples (2025 catalog). Note strings use the
// production formatting: config floats loaded from MongoDB print as 13.0 /
// 1.0 (the spec's harness printed in-memory int defaults as 13 / 1).

//go:fix inline
func fp(v float64) *float64 { return new(v) }

var trickDB = map[string]catalog.UniqueTrick{
	"/LM":  ut("twisted left Misty Flip", "/LM", "Misty Flip", 1.65, 2.5, []string{"twist"}, []string{"left"}, nil),
	"/RM":  ut("twisted right Misty Flip", "/RM", "Misty Flip", 1.65, 2.5, []string{"twist"}, []string{"right"}, nil),
	"LM/":  ut("left Misty Flip twisted exit", "LM/", "Misty Flip", 1.65, 3.5, []string{"twist"}, []string{"left"}, nil),
	"RM/":  ut("right Misty Flip twisted exit", "RM/", "Misty Flip", 1.65, 3.5, []string{"twist"}, []string{"right"}, nil),
	"LM":   ut("left Misty Flip", "LM", "Misty Flip", 1.65, 0, nil, []string{"left"}, nil),
	"RM":   ut("right Misty Flip", "RM", "Misty Flip", 1.65, 0, nil, []string{"right"}, nil),
	"RMHR": ut("right Misty to Helicopter reverse", "RMHR", "Misty to Helicopter", 1.75, 3, []string{"reverse"}, []string{"reverse", "right"}, nil),
	"LT":   ut("left Tumbling", "LT", "Tumbling", 1.8, 0, nil, []string{"left"}, []string{"tumbling"}),
	"RT":   ut("right Tumbling", "RT", "Tumbling", 1.8, 0, nil, []string{"right"}, []string{"tumbling"}),
	"RTF":  ut("right Tumbling flip", "RTF", "Tumbling", 1.8, 5, []string{"flip"}, []string{"right"}, []string{"tumbling"}),
	"RS":   ut("right SAT", "RS", "SAT", 1.25, 0, nil, []string{"right"}, nil),
	"LS":   ut("left SAT", "LS", "SAT", 1.25, 0, nil, []string{"left"}, nil),
	"FS":   ut("Stall", "FS", "Stall", 1.6, 0, nil, []string{}, nil),
	"LMC":  ut("left Mac Twist", "LMC", "Mac Twist", 1.7, 0, nil, []string{"left"}, nil),
	"RMC":  ut("right Mac Twist", "RMC", "Mac Twist", 1.7, 0, nil, []string{"right"}, nil),
	"/LMC": ut("twisted left Mac Twist", "/LMC", "Mac Twist", 1.7, 3.5, []string{"twist"}, []string{"left"}, nil),
	"/RMC": ut("twisted right Mac Twist", "/RMC", "Mac Twist", 1.7, 3.5, []string{"twist"}, []string{"right"}, nil),
	"MCF":  ut("MacFly", "MCF", "MacFly", 2.1, 0, nil, []string{}, []string{"to infinite", "tumbling"}),
	"OMH":  withFlags(ut("opposite Misty to Helicopter", "OMH", "Misty to Helicopter", 1.75, 0, nil, []string{"opposite"}, nil), false, true, true),
	"FU":   withFlags(ut("Fusion", "FU", "Fusion", 2.2, 0, nil, []string{}, nil), false, false, true),
	"MM":   withFlags(ut("Misty to Misty", "MM", "Misty to Misty", 1.75, 0, nil, []string{}, nil), true, false, true),
}

func ut(name, acronym, base string, coef, bonus float64, bonusTypes, uniq, types []string) catalog.UniqueTrick {
	if bonusTypes == nil {
		bonusTypes = []string{}
	}
	if types == nil {
		types = []string{}
	}
	return catalog.UniqueTrick{Name: name, Acronym: acronym, TechnicalCoefficient: coef, Bonus: bonus,
		BonusTypes: bonusTypes, BaseTrick: base, Uniqueness: uniq, Solo: true, SoloAWT: true, Synchro: true, Types: types}
}

func withFlags(u catalog.UniqueTrick, solo, soloAWT, synchro bool) catalog.UniqueTrick {
	u.Solo, u.SoloAWT, u.Synchro = solo, soloAWT, synchro
	return u
}

func flown(acronyms ...string) []FlownTrick {
	out := make([]FlownTrick, 0, len(acronyms))
	for _, a := range acronyms {
		u, ok := trickDB[a]
		if !ok {
			panic("unknown test trick " + a)
		}
		out = append(out, FlownTrick{UniqueTrick: u})
	}
	return out
}

func defaultConfig() Config {
	return Config{
		Warning: 0.5, Warning2: 1, MalusRepetition: 13, WarningsToDSQ: 3,
		JudgeWeights: map[JudgeLevel]float64{Senior: 100, Certified: 100, Trainee: 20},
		MarkPercentages: map[CompetitionType]map[string]float64{
			Solo:    {"technical": 40, "choreography": 40, "landing": 20},
			Synchro: {"technical": 20, "choreography": 20, "landing": 20, "synchro": 40},
		},
		MaxBonusPerRun: map[string]float64{"twist": 5, "reverse": 3, "flip": 2},
	}
}

func defaultRules() Rules {
	return Rules{
		MaxTricksPerType:    map[string]int{"tumbling": 2, "to infinite": 1},
		TipTouchPctPerTouch: 2, TipTouchPctMax: 6, DefaultMaxBonus: 10,
	}
}

// baseInput is the spec 03 §12 common setup: solo comp in season awt-2025,
// pilot AWT, judges S (senior), C (certified), T (trainee).
func baseInput() Input {
	return Input{
		Type: Solo, Config: defaultConfig(), Rules: defaultRules(),
		IsAWTPilot: true, HasAWTSeason: true,
		RepeatableBaseTricks: []string{"Stall", "Tail Slide", "Wingovers"},
		ResetPolicies:        []ResetPolicy{ResetNone},
		JudgeLevels:          map[string]JudgeLevel{"S": Senior, "C": Certified, "T": Trainee},
		PositionRules:        map[string]PositionRule{"Tumbling": {NoLast: 2}, "MacFly": {NoLast: 2}},
	}
}

func marks3() []JudgeMark {
	return []JudgeMark{
		{Judge: "S", Technical: fp(7), Choreography: new(6.5), Landing: fp(8)},
		{Judge: "C", Technical: new(7.5), Choreography: fp(7), Landing: fp(9)},
		{Judge: "T", Technical: fp(6), Choreography: fp(5), Landing: fp(5)},
	}
}

type want struct {
	jt, jc, jl, js                      float64
	technicity, bonusPct                float64
	technical, choreo, landing, synchro float64
	bonus, score, malus                 float64
	w1, w2, notes                       []string
}

func deref(p *float64) float64 {
	if p == nil {
		return -1
	}
	return *p
}

// check compares every FinalMark field at full precision (which implies
// equality after Round3) and the exact strings.
func check(t *testing.T, got FinalMark, w want) {
	t.Helper()
	nums := []struct {
		name      string
		got, want float64
	}{
		{"judges_mark.technical", deref(got.JudgesMark.Technical), w.jt},
		{"judges_mark.choreography", deref(got.JudgesMark.Choreography), w.jc},
		{"judges_mark.landing", deref(got.JudgesMark.Landing), w.jl},
		{"judges_mark.synchro", deref(got.JudgesMark.Synchro), w.js},
		{"technicity", got.Technicity, w.technicity},
		{"bonus_percentage", got.BonusPercentage, w.bonusPct},
		{"technical", got.Technical, w.technical},
		{"choreography", got.Choreography, w.choreo},
		{"landing", got.Landing, w.landing},
		{"synchro", got.Synchro, w.synchro},
		{"bonus", got.Bonus, w.bonus},
		{"score", got.Score, w.score},
		{"malus", got.Malus, w.malus},
	}
	for _, n := range nums {
		if n.got != n.want {
			t.Errorf("%s = %v, want %v", n.name, n.got, n.want)
		}
		if Round3(n.got) != Round3(n.want) {
			t.Errorf("Round3(%s) = %v, want %v", n.name, Round3(n.got), Round3(n.want))
		}
	}
	if got.JudgesMark.Judge != "" {
		t.Errorf("judges_mark.judge = %q, want empty", got.JudgesMark.Judge)
	}
	strs := []struct {
		name      string
		got, want []string
	}{
		{"warnings1", got.Warnings1, w.w1},
		{"warnings2", got.Warnings2, w.w2},
		{"notes", got.Notes, w.notes},
	}
	for _, s := range strs {
		if !slices.Equal(s.got, s.want) {
			t.Errorf("%s =\n  %q\nwant\n  %q", s.name, s.got, s.want)
		}
	}
}

func mustScore(t *testing.T, in Input) Result {
	t.Helper()
	res, err := Score(in)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	return res
}

// exampleAFlight is spec 03 §12 Example A (run 1 = index 0).
func exampleAFlight() Flight {
	return Flight{Tricks: flown("/LM", "RMHR", "LT", "RS", "FS"), Marks: marks3(), Warnings1: []string{"late at briefing"}}
}

func TestExampleA(t *testing.T) {
	in := baseInput()
	in.Flight = exampleAFlight()
	res := mustScore(t, in)
	check(t, res.Final, want{
		jt: 7.136363636363637, jc: 6.590909090909091, jl: 8.181818181818182,
		technicity: 1.7333333333333334, bonusPct: 5.5,
		technical: 4.947878787878789, choreo: 2.6363636363636362, landing: 1.6363636363636362,
		bonus: 0.41713333333333336, score: 9.137739393939396,
		w1: []string{"late at briefing"}, w2: []string{},
		notes: []string{"final mark has been lowered by 0.5 because of category 1 warning(s)"},
	})
	if res.Final.MarkType != MarkClassic {
		t.Errorf("mark_type = %q", res.Final.MarkType)
	}
}

// exampleBInput is run 2 (index 1) of the same pilot after Example A.
func exampleBInput(tricks []FlownTrick, warnings ...string) Input {
	in := baseInput()
	in.RunIndex = 1
	in.ResetPolicies = []ResetPolicy{ResetNone, ResetNone}
	a := exampleAFlight()
	in.Previous = []PreviousFlight{{RunIndex: 0, Tricks: a.Tricks, Warnings1: 1}}
	in.Flight = Flight{Tricks: tricks, Marks: marks3(), Warnings1: warnings}
	return in
}

func TestExampleB(t *testing.T) {
	in := exampleBInput(flown("LM", "RMHR", "RT", "LS", "FS", "/LMC"), "box")
	res := mustScore(t, in)
	check(t, res.Final, want{
		jt: 7.136363636363637, jc: 6.590909090909091, jl: 8.181818181818182,
		technicity: 1.75, bonusPct: 6.5,
		technical: 4.995454545454545, choreo: 2.6363636363636362, landing: 1.6363636363636362,
		bonus: -1.4882045454545454, score: 6.7799772727272725, malus: 26,
		w1: []string{"box"}, w2: []string{},
		notes: []string{
			"trick number #1 (left Misty Flip) has already been performed in a previous run. Adding a 13.0% malus.",
			"trick number #2 (right Misty to Helicopter reverse) has already been performed in a previous run. Adding a 13.0% malus.",
			"final mark has been lowered by 1.0 because of category 1 warning(s)",
		},
	})
}

func TestExampleB2DSQ(t *testing.T) {
	in := exampleBInput(flown("LM"), "box", "x")
	res := mustScore(t, in)
	check(t, res.Final, want{
		w1: []string{"box", "x"}, w2: []string{},
		notes: []string{"Pilot has been DSQ he/she's got 3 warnings"},
	})
}

func TestExampleB3UnbalancedDirections(t *testing.T) {
	in := baseInput()
	in.IsAWTPilot = false
	in.Flight = Flight{Tricks: flown("LM", "LMC", "LS"), Marks: marks3()}
	res := mustScore(t, in)
	check(t, res.Final, want{
		jt: 7.136363636363637, jc: 5.590909090909091, jl: 8.181818181818182,
		technicity: 1.5333333333333332,
		technical:  4.376969696969696, choreo: 2.2363636363636363, landing: 1.6363636363636362,
		score: 8.249696969696968,
		w1:    []string{}, w2: []string{},
		notes: []string{"choreography mark has been lower by 1 point because direction is not even"},
	})
}

func exampleCInput(mt MarkType) Input {
	in := baseInput()
	in.MarkType = mt
	in.Flight = Flight{
		Tricks: flown("/LM", "RMHR", "LT"),
		Marks: []JudgeMark{
			{Judge: "S", TechnicalPerTrick: []*float64{fp(8), fp(6), fp(7)}, Choreography: fp(7), Landing: fp(8)},
			{Judge: "C", TechnicalPerTrick: []*float64{fp(7), nil, fp(8)}, Choreography: fp(6), Landing: fp(7)},
			{Judge: "T", TechnicalPerTrick: []*float64{fp(5), fp(5), fp(5)}, Choreography: fp(5), Landing: fp(5)},
		},
	}
	return in
}

func TestExampleCAWT(t *testing.T) {
	res := mustScore(t, exampleCInput(MarkAWT))
	check(t, res.Final, want{
		jt: 6.742424242424242, jc: 6.363636363636363, jl: 7.2727272727272725,
		technicity: 1.7333333333333334, bonusPct: 3.5227272727272725,
		technical: 4.674747474747474, choreo: 2.5454545454545454, landing: 1.4545454545454544,
		bonus: 0.25434802571166204, score: 8.429095500459137,
		w1: []string{"left Tumbling can't be one of the last 2 maneuvers"}, w2: []string{},
		notes: []string{
			"Initial total bonus of 5.5% has been lowered by technical mark to 3.523%",
			"final mark has been lowered by 0.5 because of category 1 warning(s)",
		},
	})
	if res.Final.MarkType != MarkAWT {
		t.Errorf("mark_type = %q", res.Final.MarkType)
	}
	// step 4: C's missing RMHR mark is filled with the plain mean of 6 and 5.
	if got := deref(res.Marks[1].TechnicalPerTrick[1]); got != 5.5 {
		t.Errorf("filled mark = %v, want 5.5", got)
	}
	// step 5: per-trick weighted technical marks.
	wantTM := []float64{7.273, 5.682, 7.273}
	for i, tr := range res.Tricks {
		if tr.TechnicalMark == nil || Round3(*tr.TechnicalMark) != wantTM[i] {
			t.Errorf("trick %d technical_mark = %v, want %v", i, tr.TechnicalMark, wantTM[i])
		}
	}
	if !reflect.DeepEqual(res.Tricks[1].TechnicalMarks, map[string]float64{"S": 6, "C": 5.5, "T": 5}) {
		t.Errorf("technical_marks = %v", res.Tricks[1].TechnicalMarks)
	}
}

func TestExampleCAWQ(t *testing.T) {
	res := mustScore(t, exampleCInput(MarkAWQ))
	check(t, res.Final, want{
		jt: 6.742424242424242, jc: 6.363636363636363, jl: 7.2727272727272725,
		technicity: 1.7333333333333334, bonusPct: 5.5,
		technical: 4.674747474747474, choreo: 2.5454545454545454, landing: 1.4545454545454544,
		bonus: 0.3971111111111111, score: 8.571858585858585,
		w1: []string{"left Tumbling can't be one of the last 2 maneuvers"}, w2: []string{},
		notes: []string{"final mark has been lowered by 0.5 because of category 1 warning(s)"},
	})
	for i, tr := range res.Tricks {
		if tr.TechnicalMark != nil {
			t.Errorf("trick %d technical_mark = %v, want nil for awq", i, *tr.TechnicalMark)
		}
	}
	if !reflect.DeepEqual(res.Tricks[0].TechnicalMarks, map[string]float64{"S": 8, "C": 7, "T": 5}) {
		t.Errorf("technical_marks = %v", res.Tricks[0].TechnicalMarks)
	}
}

func TestExampleCClassicNeedsTechnical(t *testing.T) {
	_, err := Score(exampleCInput(MarkClassic))
	assertValidation(t, err, "not enough technical marks")
}

func assertValidation(t *testing.T, err error, msg string) {
	t.Helper()
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError %q", err, msg)
	}
	if ve.Msg != msg {
		t.Errorf("err = %q, want %q", ve.Msg, msg)
	}
}

func TestExampleDLimitsAndInRunRepetition(t *testing.T) {
	in := baseInput()
	in.JudgeLevels = map[string]JudgeLevel{"S": Senior}
	in.Flight = Flight{
		Tricks: flown("MCF", "/LM", "/RM", "LM/", "RM/", "/LMC", "/RMC", "RTF"),
		Marks:  []JudgeMark{{Judge: "S", Technical: fp(7), Choreography: fp(7), Landing: fp(7)}},
	}
	res := mustScore(t, in)
	check(t, res.Final, want{
		jt: 7, jc: 7, jl: 7,
		technicity: 1.8666666666666667, bonusPct: 20.5,
		technical: 5.226666666666667, choreo: 2.8, landing: 1.4,
		bonus: -0.4414666666666667, score: 7.985200000000001, malus: 26,
		w1: []string{
			"right Tumbling flip can't be one of the last 2 maneuvers",
			"flips can't be performed as one of the last 2 maneuvers (right Tumbling flip)",
		},
		w2: []string{},
		notes: []string{
			"trick number #4 (left Misty Flip twisted exit) has already been performed in this run. Adding a 13.0% malus.",
			"trick number #5 (right Misty Flip twisted exit) has already been performed in this run. Adding a 13.0% malus.",
			"trick number #6 (twisted right Mac Twist) has been ignored because more than 5 twist tricks have been flown",
			"final mark has been lowered by 1.0 because of category 1 warning(s)",
		},
	})
}

func TestExampleESynchro(t *testing.T) {
	in := baseInput()
	in.Type = Synchro
	in.IsAWTPilot, in.HasAWTSeason = false, false
	in.Flight = Flight{
		Tricks: flown("OMH", "/LM", "LM", "FU", "LMC"),
		Marks: []JudgeMark{
			{Judge: "S", Technical: fp(7), Choreography: fp(6), Landing: fp(8), Synchro: fp(7)},
			{Judge: "C", Technical: fp(8), Choreography: fp(7), Landing: fp(6), Synchro: fp(8)},
		},
		TipTouches: 4,
	}
	res := mustScore(t, in)
	check(t, res.Final, want{
		jt: 7.5, jc: 6.5, jl: 7, js: 7.5,
		technicity: 1.8833333333333333, bonusPct: 8.5,
		technical: 2.825, choreo: 1.3, landing: 1.4, synchro: 3,
		bonus: -0.185625, score: 8.339375, malus: 13,
		w1: []string{}, w2: []string{},
		notes: []string{
			"trick number #3 (left Misty Flip) has already been performed in this run. Adding a 13.0% malus.",
			"6% tip-touch bonus",
		},
	})
}

func exampleFInput(awt bool, run int) Input {
	in := baseInput()
	in.JudgeLevels = map[string]JudgeLevel{"S": Senior}
	in.IsAWTPilot, in.HasAWTSeason, in.HasAWQSeason = awt, true, true
	in.RunIndex = run
	in.ResetPolicies = []ResetPolicy{ResetNone, ResetAWT}[:run+1]
	in.Flight = Flight{
		Tricks: flown("LM", "RM", "MM"),
		Marks:  []JudgeMark{{Judge: "S", Technical: fp(7), Choreography: fp(7), Landing: fp(7)}},
	}
	return in
}

func TestExampleF(t *testing.T) {
	t.Run("run1 AWT pilot: Misty to Misty ignored", func(t *testing.T) {
		res := mustScore(t, exampleFInput(true, 0))
		check(t, res.Final, want{
			jt: 7, jc: 7, jl: 7, technicity: 1.4333333333333333,
			technical: 4.013333333333333, choreo: 2.8, landing: 1.4, score: 8.213333333333333,
			w1: []string{}, w2: []string{},
			notes: []string{"Ignoring trick #2 (Misty to Misty) because it's not allowed in AWT solo competition"},
		})
	})
	t.Run("run1 AWQ pilot: cat-2 warning", func(t *testing.T) {
		in := exampleFInput(false, 0)
		in.Flight.Warnings2 = []string{"cat2"}
		res := mustScore(t, in)
		check(t, res.Final, want{
			jt: 7, jc: 7, jl: 7, technicity: 1.6833333333333333,
			technical: 4.713333333333333, choreo: 2.8, landing: 1.4, score: 7.913333333333332,
			w1: []string{}, w2: []string{"cat2"},
			notes: []string{"final mark has been lowered by 1.0 because of category 2 warning(s)"},
		})
	})
	prev := []PreviousFlight{{RunIndex: 0, Tricks: flown("LM", "RM", "MM")}}
	t.Run("run2 AWT pilot: repetitions reset", func(t *testing.T) {
		in := exampleFInput(true, 1)
		in.Previous = prev
		in.Flight.Tricks = flown("LM", "RM")
		in.Flight.Warnings1 = []string{"w1"}
		res := mustScore(t, in)
		check(t, res.Final, want{
			jt: 7, jc: 7, jl: 7, technicity: 1.4333333333333333,
			technical: 4.013333333333333, choreo: 2.8, landing: 1.4, score: 7.713333333333333,
			w1: []string{"w1"}, w2: []string{},
			notes: []string{"final mark has been lowered by 0.5 because of category 1 warning(s)"},
		})
	})
	t.Run("run2 AWQ pilot: DSQ", func(t *testing.T) {
		in := exampleFInput(false, 1)
		in.Previous = []PreviousFlight{{RunIndex: 0, Tricks: prev[0].Tricks, Warnings2: 1}}
		in.Flight.Tricks = flown("LM", "RM")
		in.Flight.Warnings1 = []string{"w1"}
		in.Flight.Warnings2 = []string{"c2"}
		res := mustScore(t, in)
		check(t, res.Final, want{
			w1: []string{"w1"}, w2: []string{"c2"},
			notes: []string{"Pilot has been DSQ he/she's got 3 warnings"},
		})
	})
	t.Run("run2 AWQ pilot: no reset, repetition malus", func(t *testing.T) {
		in := exampleFInput(false, 1)
		in.Previous = []PreviousFlight{{RunIndex: 0, Tricks: prev[0].Tricks, Warnings2: 1}}
		in.Flight.Tricks = flown("LM", "RM")
		res := mustScore(t, in)
		check(t, res.Final, want{
			jt: 7, jc: 7, jl: 7, technicity: 1.4333333333333333,
			technical: 4.013333333333333, choreo: 2.8, landing: 1.4,
			bonus: -1.7714666666666665, score: 6.441866666666666, malus: 26,
			w1: []string{}, w2: []string{},
			notes: []string{
				"trick number #1 (left Misty Flip) has already been performed in a previous run. Adding a 13.0% malus.",
				"trick number #2 (right Misty Flip) has already been performed in a previous run. Adding a 13.0% malus.",
			},
		})
	})
}

func TestDNS(t *testing.T) {
	in := baseInput()
	in.Flight = Flight{DidNotStart: true, Tricks: flown("LM"), Warnings1: []string{"w"},
		Marks: []JudgeMark{{Judge: "nobody"}}}
	res := mustScore(t, in)
	check(t, res.Final, want{w1: []string{"w"}, w2: []string{}, notes: []string{"Got 0 because of DNS"}})
}

func TestPreviousDSQ(t *testing.T) {
	in := exampleBInput(flown("LM"))
	in.Previous[0].Warnings1, in.Previous[0].Warnings2 = 2, 1
	res := mustScore(t, in)
	check(t, res.Final, want{w1: []string{}, w2: []string{},
		notes: []string{"Pilot has been DSQ because he/she already had 3 warnings"}})
}

func TestPenaltyBothCategoriesCumulative(t *testing.T) {
	in := exampleBInput(flown("RM"), "a")
	in.Previous[0].Warnings1 = 0
	in.Previous[0].Warnings2 = 1
	in.Config.WarningsToDSQ = 10
	in.Flight.Warnings2 = []string{"b"}
	res := mustScore(t, in)
	// cat 1: (1+0)*0.5; cat 2: (1+1)*1 -> note prints the running total.
	wantNotes := []string{
		"final mark has been lowered by 0.5 because of category 1 warning(s)",
		"final mark has been lowered by 2.5 because of category 2 warning(s)",
	}
	if !slices.Equal(res.Final.Notes, wantNotes) {
		t.Errorf("notes = %q", res.Final.Notes)
	}
}

func TestRepetitionPreviousAndInRun(t *testing.T) {
	in := exampleBInput(flown("LM", "/LM"))
	in.Previous[0].Warnings1 = 0
	res := mustScore(t, in)
	if res.Final.Malus != 39 {
		t.Errorf("malus = %v, want 39 (LM prev, /LM prev + in-run)", res.Final.Malus)
	}
}

func TestResetPolicyAll(t *testing.T) {
	in := exampleBInput(flown("LM"))
	in.ResetPolicies = []ResetPolicy{ResetNone, ResetAll}
	if res := mustScore(t, in); res.Final.Malus != 0 {
		t.Errorf("malus = %v, want 0 after reset", res.Final.Malus)
	}
	// An AWQ reset does not apply to an AWT pilot.
	in.ResetPolicies = []ResetPolicy{ResetNone, ResetAWQ}
	in.HasAWQSeason = true
	if res := mustScore(t, in); res.Final.Malus != 13 {
		t.Errorf("malus = %v, want 13", res.Final.Malus)
	}
	// Synchro never resets.
	in.Type = Synchro
	in.ResetPolicies = []ResetPolicy{ResetNone, ResetAll}
	for i := range in.Flight.Marks {
		in.Flight.Marks[i].Synchro = fp(7)
	}
	if res := mustScore(t, in); res.Final.Malus != 13 {
		t.Errorf("synchro malus = %v, want 13", res.Final.Malus)
	}
}

func TestDirectionBalance(t *testing.T) {
	tests := []struct {
		name    string
		tricks  []string
		choreo  float64
		lowered bool
	}{
		{"L,L,L,R diff 2", []string{"LM", "LMC", "LS", "RS"}, 6.5, true},
		{"L,L,R diff 1", []string{"LM", "LMC", "RS"}, 6.5, false},
		{"L,L one direction", []string{"LM", "LS"}, 6.5, true},
		{"one directional trick", []string{"LM", "FS"}, 6.5, false},
		{"single trick", []string{"LM"}, 6.5, false},
		{"mark below 1 not lowered", []string{"LM", "LS"}, 0.5, false},
		{"opposite counts as a direction", []string{"LM", "RS", "OMH"}, 6.5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInput()
			in.JudgeLevels = map[string]JudgeLevel{"S": Senior}
			in.Flight = Flight{Tricks: flown(tc.tricks...),
				Marks: []JudgeMark{{Judge: "S", Technical: fp(7), Choreography: new(tc.choreo), Landing: fp(7)}}}
			res := mustScore(t, in)
			want := tc.choreo
			if tc.lowered {
				want--
			}
			if got := deref(res.Final.JudgesMark.Choreography); got != want {
				t.Errorf("J.choreography = %v, want %v", got, want)
			}
		})
	}
}

func TestTechnicityPadding(t *testing.T) {
	tests := []struct {
		name  string
		coefs []float64
		want  float64
	}{
		{"top 3 of 5", []float64{1.8, 1.75, 1.65, 1.6, 1.25}, 1.7333333333333334},
		{"padded with one 1.0", []float64{1.65, 1.65}, 1.4333333333333333},
		{"no kept trick", nil, 1.0},
		{"unsorted input", []float64{1.25, 2.1, 1.6, 1.8}, Average([]float64{2.1, 1.8, 1.6})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInput()
			in.Flight.Marks = marks3()
			for i, c := range tc.coefs {
				u := ut("t", "T", "base"+strings.Repeat("x", i), c, 0, nil, []string{}, nil)
				in.Flight.Tricks = append(in.Flight.Tricks, FlownTrick{UniqueTrick: u})
			}
			if got := mustScore(t, in).Final.Technicity; got != tc.want {
				t.Errorf("technicity = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLastManeuverFixed(t *testing.T) {
	tests := []struct {
		name   string
		legacy bool
		first  bool
		last   int
		want   []string
	}{
		{"fixed: placed first warns", false, true, 1, []string{"left SAT must be the last maneuver"}},
		{"fixed: placed last ok", false, false, 1, []string{}},
		{"fixed: N=2 among last 2 ok", false, false, 2, []string{}},
		{"legacy: placed first ok", true, true, 1, []string{}},
		{"legacy: placed last warns", true, false, 1, []string{"left SAT must be the last maneuver"}},
		{"legacy: N=2 message", true, false, 2, []string{"left SAT must be one of the last 2 maneuvers"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInput()
			in.LegacyLastManeuver = tc.legacy
			in.PositionRules["SAT"] = PositionRule{Last: tc.last}
			in.Flight.Marks = marks3()
			in.Flight.Tricks = flown("LS", "RM", "LM")
			if !tc.first {
				in.Flight.Tricks = flown("RM", "LM", "LS")
			}
			res := mustScore(t, in)
			if !slices.Equal(res.Final.Warnings1, tc.want) {
				t.Errorf("warnings1 = %q, want %q", res.Final.Warnings1, tc.want)
			}
		})
	}
}

func TestPositionRuleMessages(t *testing.T) {
	in := baseInput()
	in.Config.WarningsToDSQ = 100
	in.PositionRules["SAT"] = PositionRule{First: 1, NoFirst: 1}
	in.PositionRules["Mac Twist"] = PositionRule{First: 2, NoFirst: 2, NoLast: 2}
	in.Flight.Marks = marks3()
	in.Flight.Tricks = flown("RS", "LMC", "LM", "RMC", "LS")
	res := mustScore(t, in)
	want := []string{
		"right SAT can't be the first maneuver",
		"left Mac Twist can't be one of the first 2 maneuvers",
		"right Mac Twist must be one of the first 2 maneuvers",
		"right Mac Twist can't be one of the last 2 maneuvers",
		"left SAT must be the first maneuver",
	}
	if !slices.Equal(res.Final.Warnings1, want) {
		t.Errorf("warnings1 =\n %q\nwant\n %q", res.Final.Warnings1, want)
	}
}

func TestIgnoredTrickNotes(t *testing.T) {
	in := baseInput()
	in.Flight.Marks = marks3()
	in.Flight.Tricks = flown("OMH", "LM")
	res := mustScore(t, in)
	if res.Final.Notes[0] != "Ignoring trick #0 (opposite Misty to Helicopter) because it's not allowed in a solo competition" {
		t.Errorf("notes = %q", res.Final.Notes)
	}
	in.Type = Synchro
	for i := range in.Flight.Marks {
		in.Flight.Marks[i].Synchro = fp(7)
	}
	in.Flight.Tricks = flown("LM", "LM")
	in.Flight.Tricks[1].Synchro = false
	in.Flight.Tricks[1].Types = []string{"to infinite"}
	in.Flight.Tricks[0].Types = []string{"to infinite"}
	res = mustScore(t, in)
	want := []string{
		"trick number #2 (left Misty Flip) has already been performed in this run. Adding a 13.0% malus.",
		"Ignoring trick #1 (left Misty Flip) because it's not allowed in a synchro competition",
	}
	if !slices.Equal(res.Final.Notes, want) {
		t.Errorf("notes = %q", res.Final.Notes)
	}
	// A type limit note, and a default max (other) note.
	in.Flight.Tricks[1].Synchro = true
	in.Flight.Tricks[0].BonusTypes = []string{"other"}
	in.Flight.Tricks[1].BonusTypes = []string{"other"}
	in.Rules.DefaultMaxBonus = 1
	res = mustScore(t, in)
	want = []string{
		"trick number #2 (left Misty Flip) has already been performed in this run. Adding a 13.0% malus.",
		"trick number #1 (left Misty Flip) has been ignored because more than 1 other tricks have been flown",
		"trick number #1 (left Misty Flip) has been ignored because more than 1 'to infinite' tricks have been flown",
	}
	if !slices.Equal(res.Final.Notes, want) {
		t.Errorf("notes = %q", res.Final.Notes)
	}
}

// Classic marking keeps the stored per-trick technical mark, which still
// scales the bonus (legacy reads trick.technical_mark whatever the mode).
func TestClassicStoredTechnicalMarkScalesBonus(t *testing.T) {
	in := baseInput()
	in.Flight = exampleAFlight()
	in.Flight.Tricks[0].TechnicalMark = fp(8)
	res := mustScore(t, in)
	// Only tricks having a technical mark contribute once one has: RMHR's
	// 3% is dropped (legacy behaviour).
	if res.Final.BonusPercentage != 2 {
		t.Errorf("bonus_percentage = %v, want 2", res.Final.BonusPercentage)
	}
	if res.Final.Notes[0] != "Initial total bonus of 5.5% has been lowered by technical mark to 2.0%" {
		t.Errorf("notes = %q", res.Final.Notes)
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		mod  func(in *Input)
		msg  string
	}{
		{"unknown judge", func(in *Input) { in.Flight.Marks[0].Judge = "X" }, "unknown judge X"},
		{"no choreography", func(in *Input) {
			for i := range in.Flight.Marks {
				in.Flight.Marks[i].Choreography = nil
			}
		}, "not enough choreography marks"},
		{"no landing", func(in *Input) {
			for i := range in.Flight.Marks {
				in.Flight.Marks[i].Landing = nil
			}
		}, "not enough landing marks"},
		{"no technical", func(in *Input) {
			for i := range in.Flight.Marks {
				in.Flight.Marks[i].Technical = nil
			}
		}, "not enough technical marks"},
		{"no synchro", func(in *Input) { in.Type = Synchro }, "not enough synchro marks"},
		{"tpt length", func(in *Input) {
			in.Flight.Marks[1].TechnicalPerTrick = []*float64{fp(1)}
		}, "judge 'C' did not mark enough tricks (1 instead of 5)"},
		{"tpt mixed", func(in *Input) {
			in.Flight.Marks[1].TechnicalPerTrick = []*float64{fp(1), fp(1), fp(1), fp(1), fp(1)}
		}, "somes judges sent technical mark per trick while other(s) did not"},
		{"tpt all null", func(in *Input) {
			for i := range in.Flight.Marks {
				in.Flight.Marks[i].TechnicalPerTrick = []*float64{fp(1), nil, fp(1), fp(1), fp(1)}
			}
		}, "not enough marks for right Misty to Helicopter reverse"},
		{"awt without tpt", func(in *Input) { in.MarkType = MarkAWT }, "no technical mark for trick /LM"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInput()
			in.Flight = exampleAFlight()
			tc.mod(&in)
			_, err := Score(in)
			assertValidation(t, err, tc.msg)
		})
	}
}

func TestScoreDoesNotMutateInput(t *testing.T) {
	in := exampleCInput(MarkAWT)
	in.Flight.Tricks[0].TechnicalMarks = map[string]float64{"old": 1}
	before := cloneFlight(in.Flight)
	mustScore(t, in)
	if !reflect.DeepEqual(in.Flight, before) {
		t.Errorf("input flight mutated")
	}
	if in.Flight.Marks[1].TechnicalPerTrick[1] != nil {
		t.Errorf("input mark filled in place")
	}
}

func TestFlownTrickJSON(t *testing.T) {
	tr := flown("/LM")[0]
	tr.TechnicalMark = new(7.5)
	tr.TechnicalMarks = map[string]float64{"S": 7.5}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"name", "acronym", "base_trick", "solo_awt", "technical_mark", "technical_marks"} {
		if _, ok := m[k]; !ok {
			t.Errorf("key %q missing in %s", k, b)
		}
	}
	var back FlownTrick
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, tr) {
		t.Errorf("round trip = %+v, want %+v", back, tr)
	}
}

// randomInput builds a valid input: known judges, marks within 0-10, every
// required mark present.
func randomInput(r *rand.Rand) Input {
	in := baseInput()
	in.Type = []CompetitionType{Solo, Synchro}[r.IntN(2)]
	in.MarkType = []MarkType{MarkClassic, MarkAWT, MarkAWQ}[r.IntN(3)]
	in.IsAWTPilot, in.HasAWTSeason, in.HasAWQSeason = r.IntN(2) == 0, r.IntN(2) == 0, r.IntN(2) == 0
	in.LegacyLastManeuver = r.IntN(2) == 0
	in.Config.WarningsToDSQ = 1 + r.IntN(5)
	in.PositionRules["SAT"] = PositionRule{First: r.IntN(3), NoFirst: r.IntN(3), Last: r.IntN(3), NoLast: r.IntN(3)}
	acronyms := make([]string, 0, len(trickDB))
	for a := range trickDB {
		acronyms = append(acronyms, a)
	}
	slices.Sort(acronyms)
	pick := func(n int) []FlownTrick {
		var out []string
		for range n {
			out = append(out, acronyms[r.IntN(len(acronyms))])
		}
		return flown(out...)
	}
	in.RunIndex = r.IntN(4)
	in.ResetPolicies = nil
	for i := 0; i <= in.RunIndex; i++ {
		in.ResetPolicies = append(in.ResetPolicies, ResetPolicy(r.IntN(4)))
	}
	for i := 0; i < in.RunIndex; i++ {
		if r.IntN(4) > 0 {
			in.Previous = append(in.Previous, PreviousFlight{RunIndex: i, Tricks: pick(r.IntN(8)), Warnings1: r.IntN(2), Warnings2: r.IntN(2)})
		}
	}
	mark := func() *float64 { return new(Round3(r.Float64() * 10)) }
	f := Flight{DidNotStart: r.IntN(20) == 0, Tricks: pick(r.IntN(10)), TipTouches: r.IntN(5)}
	for range r.IntN(2) {
		f.Warnings1 = append(f.Warnings1, "w")
	}
	for range r.IntN(2) {
		f.Warnings2 = append(f.Warnings2, "w")
	}
	judgeIDs := []string{"S", "C", "T"}
	for range 1 + r.IntN(4) {
		m := JudgeMark{Judge: judgeIDs[r.IntN(3)], Choreography: mark(), Landing: mark(), Synchro: mark()}
		if in.MarkType == MarkClassic {
			m.Technical = mark()
		} else {
			m.TechnicalPerTrick = make([]*float64, len(f.Tricks))
			for i := range m.TechnicalPerTrick {
				if r.IntN(4) > 0 {
					m.TechnicalPerTrick[i] = mark()
				}
			}
		}
		f.Marks = append(f.Marks, m)
	}
	if in.MarkType != MarkClassic {
		// At least one judge marks every trick.
		for i := range f.Tricks {
			f.Marks[0].TechnicalPerTrick[i] = mark()
		}
	}
	in.Flight = f
	return in
}

func FuzzScore(f *testing.F) {
	for i := range uint64(20) {
		f.Add(i, i*7919)
	}
	f.Fuzz(func(t *testing.T, s1, s2 uint64) {
		in := randomInput(rand.New(rand.NewPCG(s1, s2)))
		res, err := Score(in)
		if err != nil {
			t.Fatalf("valid input rejected: %v", err)
		}
		m := res.Final
		if m.Score < 0 || math.IsNaN(m.Score) {
			t.Errorf("score = %v, want >= 0", m.Score)
		}
		zeroed := in.Flight.DidNotStart || (len(m.Notes) > 0 && strings.HasPrefix(m.Notes[0], "Pilot has been DSQ"))
		if zeroed {
			if m.Score != 0 || m.Technicity != 0 {
				t.Errorf("DNS/DSQ mark not zero: %+v", m)
			}
		} else if m.Technicity < 1 {
			t.Errorf("technicity = %v, want >= 1", m.Technicity)
		}
	})
}
