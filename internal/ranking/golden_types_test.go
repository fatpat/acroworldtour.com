package ranking

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoldenRankingFilesDecode checks that the generated ranking corpus
// matches the Go types exactly (no unknown fields).
func TestGoldenRankingFilesDecode(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "golden", "rankings", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no golden ranking files")
	}
	comps, seasons := 0, 0
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		base := filepath.Base(path)
		switch {
		case strings.HasPrefix(base, "competition-"):
			comps++
			var g GoldenCompetition
			if err := dec.Decode(&g); err != nil {
				t.Fatalf("%s: %v", base, err)
			}
			if g.Scope != "competition" || len(g.Expected.Results["overall"]) == 0 && len(g.Input.Runs) > 0 {
				t.Errorf("%s: unexpected content", base)
			}
		case strings.HasPrefix(base, "season-"):
			seasons++
			var g GoldenSeason
			if err := dec.Decode(&g); err != nil {
				t.Fatalf("%s: %v", base, err)
			}
			if g.Scope != "season" || (g.Expected == nil) == (g.Error == "") {
				t.Errorf("%s: unexpected content", base)
			}
		default:
			t.Errorf("unexpected file %s", base)
		}
	}
	// Corpus generated from the 2026-10-18 export: 48 competitions, 20
	// seasons; fewer means files went missing.
	if comps < 48 || seasons < 20 {
		t.Errorf("%d competition and %d season files, want at least 48 and 20", comps, seasons)
	}
}
