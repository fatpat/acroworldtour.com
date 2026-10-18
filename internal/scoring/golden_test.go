package scoring

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The golden corpus (testdata/golden/flights) holds every production flight
// scored by the legacy engine (tools/golden/generate.py). It is generated,
// never hand-edited: a mismatch means the engine is wrong.

type goldenFile struct {
	Competition string       `json:"competition"`
	Code        string       `json:"code"`
	Cases       []goldenCase `json:"cases"`
}

type goldenCase struct {
	Run            int          `json:"run"`
	Participant    string       `json:"participant"`
	Input          Input        `json:"input"`
	Expected       *FinalMark   `json:"expected"`
	ExpectedTricks []FlownTrick `json:"expected_tricks"`
	ExpectedMarks  []JudgeMark  `json:"expected_marks"`
	Error          string       `json:"error"`
}

// goldenTolerance bounds the raw difference between Go and Python floats;
// values must also be equal after Round3.
const goldenTolerance = 1e-9

func TestGoldenFlights(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "golden", "flights", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no golden flight files")
	}
	total, failed := 0, 0
	for _, path := range files {
		gf := readGoldenFile(t, path)
		for _, c := range gf.Cases {
			total++
			if diff := checkGoldenCase(c); diff != "" {
				failed++
				t.Errorf("%s run %d participant %s: %s", gf.Code, c.Run, c.Participant, diff)
			}
		}
	}
	t.Logf("%d golden cases, %d mismatches", total, failed)
}

func readGoldenFile(t *testing.T, path string) goldenFile {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var gf goldenFile
	if err := dec.Decode(&gf); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return gf
}

// checkGoldenCase returns the first differing field, or "".
func checkGoldenCase(c goldenCase) string {
	res, err := Score(c.Input)
	if c.Error != "" {
		var ve ValidationError
		if !errors.As(err, &ve) {
			return fmt.Sprintf("error: want ValidationError %q, got %v", c.Error, err)
		}
		if ve.Msg != c.Error {
			return fmt.Sprintf("error: want %q, got %q", c.Error, ve.Msg)
		}
		return ""
	}
	if err != nil {
		return fmt.Sprintf("unexpected error %v", err)
	}
	if c.Expected == nil {
		return "corpus case without expected result"
	}
	if d := diffFinal(*c.Expected, res.Final); d != "" {
		return d
	}
	if d := diffTricks(c.ExpectedTricks, res.Tricks); d != "" {
		return d
	}
	return diffMarks(c.ExpectedMarks, res.Marks)
}

func diffFloat(field string, want, got float64) string {
	if Round3(want) != Round3(got) || math.Abs(want-got) > goldenTolerance {
		return fmt.Sprintf("%s: want %v, got %v", field, want, got)
	}
	return ""
}

func diffFloatPtr(field string, want, got *float64) string {
	switch {
	case want == nil && got == nil:
		return ""
	case want == nil || got == nil:
		return fmt.Sprintf("%s: want %v, got %v", field, fmtPtr(want), fmtPtr(got))
	}
	return diffFloat(field, *want, *got)
}

func fmtPtr(p *float64) string {
	if p == nil {
		return "null"
	}
	return fmt.Sprint(*p)
}

func diffStrings(field string, want, got []string) string {
	if !slices.Equal(want, got) {
		return fmt.Sprintf("%s: want %q, got %q", field, want, got)
	}
	return ""
}

func diffFinal(want, got FinalMark) string {
	for _, d := range []string{
		diffFloat("technicity", want.Technicity, got.Technicity),
		diffFloat("bonus_percentage", want.BonusPercentage, got.BonusPercentage),
		diffFloat("technical", want.Technical, got.Technical),
		diffFloat("choreography", want.Choreography, got.Choreography),
		diffFloat("landing", want.Landing, got.Landing),
		diffFloat("synchro", want.Synchro, got.Synchro),
		diffFloat("bonus", want.Bonus, got.Bonus),
		diffFloat("score", want.Score, got.Score),
		diffFloat("malus", want.Malus, got.Malus),
		diffJudgeMark("judges_mark", want.JudgesMark, got.JudgesMark),
		diffStrings("warnings1", want.Warnings1, got.Warnings1),
		diffStrings("warnings2", want.Warnings2, got.Warnings2),
		diffStrings("notes", want.Notes, got.Notes),
	} {
		if d != "" {
			return d
		}
	}
	if want.MarkType != got.MarkType {
		return fmt.Sprintf("mark_type: want %q, got %q", want.MarkType, got.MarkType)
	}
	return ""
}

func diffJudgeMark(field string, want, got JudgeMark) string {
	if want.Judge != got.Judge {
		return fmt.Sprintf("%s.judge: want %q, got %q", field, want.Judge, got.Judge)
	}
	for _, d := range []string{
		diffFloatPtr(field+".technical", want.Technical, got.Technical),
		diffFloatPtr(field+".choreography", want.Choreography, got.Choreography),
		diffFloatPtr(field+".landing", want.Landing, got.Landing),
		diffFloatPtr(field+".synchro", want.Synchro, got.Synchro),
	} {
		if d != "" {
			return d
		}
	}
	if (want.TechnicalPerTrick == nil) != (got.TechnicalPerTrick == nil) || len(want.TechnicalPerTrick) != len(got.TechnicalPerTrick) {
		return fmt.Sprintf("%s.technical_per_trick: want %d values, got %d", field, len(want.TechnicalPerTrick), len(got.TechnicalPerTrick))
	}
	for i := range want.TechnicalPerTrick {
		if d := diffFloatPtr(fmt.Sprintf("%s.technical_per_trick[%d]", field, i), want.TechnicalPerTrick[i], got.TechnicalPerTrick[i]); d != "" {
			return d
		}
	}
	return ""
}

func diffTricks(want, got []FlownTrick) string {
	if len(want) != len(got) {
		return fmt.Sprintf("tricks: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		field := fmt.Sprintf("tricks[%d]", i)
		if want[i].Acronym != got[i].Acronym {
			return fmt.Sprintf("%s.acronym: want %q, got %q", field, want[i].Acronym, got[i].Acronym)
		}
		if d := diffFloatPtr(field+".technical_mark", want[i].TechnicalMark, got[i].TechnicalMark); d != "" {
			return d
		}
		wm, gm := want[i].TechnicalMarks, got[i].TechnicalMarks
		if (wm == nil) != (gm == nil) || len(wm) != len(gm) {
			return fmt.Sprintf("%s.technical_marks: want %v, got %v", field, wm, gm)
		}
		for j, w := range wm {
			g, ok := gm[j]
			if !ok {
				return fmt.Sprintf("%s.technical_marks[%s]: missing", field, j)
			}
			if d := diffFloat(fmt.Sprintf("%s.technical_marks[%s]", field, j), w, g); d != "" {
				return d
			}
		}
	}
	return ""
}

func diffMarks(want, got []JudgeMark) string {
	if len(want) != len(got) {
		return fmt.Sprintf("marks: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		if d := diffJudgeMark(fmt.Sprintf("marks[%d]", i), want[i], got[i]); d != "" {
			return d
		}
	}
	return ""
}
