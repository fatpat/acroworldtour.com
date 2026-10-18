package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awt/internal/blob"
	"awt/internal/importer"
	"awt/internal/ranking"
	"awt/internal/store"
	"awt/internal/store/gen"
)

func se(scores ...float64) []ranking.GoldenScoreEntry {
	var out []ranking.GoldenScoreEntry
	for i, s := range scores {
		out = append(out, ranking.GoldenScoreEntry{Participant: string(rune('a' + i)), Score: s})
	}
	return out
}

func goldenEntries(scores ...float64) []ranking.GoldenEntry {
	var out []ranking.GoldenEntry
	for _, e := range se(scores...) {
		out = append(out, ranking.GoldenEntry{Participant: e.Participant, Score: e.Score})
	}
	return out
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGoldenVerify(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mini := filepath.Join("..", "..", "internal", "importer", "testdata", "mini")
	if _, err := importer.Run(ctx, db, blobs, mini, importer.WithCatalogDir(filepath.Join("..", "..", "testdata", "catalog"))); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	// ids are pseudonymised in the real corpus: only scores, counts and flags are compared.
	comp := ranking.GoldenCompetition{Scope: "competition", Code: "mini-solo-2024", Expected: ranking.GoldenCompetitionExpected{
		Final: true,
		Results: map[string][]ranking.GoldenEntry{
			"overall": goldenEntries(16.25, 14.5, 0), "awt-2024": goldenEntries(14.5), "women": {},
		},
		RunsResults: []ranking.GoldenRunResult{
			{Final: true, Results: map[string][]ranking.GoldenScoreEntry{"overall": se(8.5, 7.25, 0), "awt-2024": se(8.5)}},
			{Final: true, Results: map[string][]ranking.GoldenScoreEntry{"overall": se(9, 6), "awt-2024": se(6)}},
		},
	}}
	writeJSON(t, filepath.Join(dir, "competition-mini-solo-2024.json"), comp)
	season := ranking.GoldenSeason{Scope: "season", Code: "awt-2024", Expected: &ranking.GoldenSeasonExpected{
		Type: "solo", Results: map[string][]ranking.GoldenScoreEntry{"overall": se(14.5)},
	}}
	writeJSON(t, filepath.Join(dir, "season-awt-2024.json"), season)

	var out bytes.Buffer
	if err := verifyGolden(ctx, gen.New(db.Read), dir, &out); err != nil {
		t.Fatalf("verify: %v\n%s", err, out.String())
	}
	if strings.TrimSpace(out.String()) != "1 competitions, 1 seasons OK" {
		t.Errorf("output = %q", out.String())
	}

	// a wrong score, a wrong final flag and a missing competition are all reported
	comp.Expected.Results["overall"] = goldenEntries(16.25, 14.4, 0)
	comp.Expected.Final = false
	writeJSON(t, filepath.Join(dir, "competition-mini-solo-2024.json"), comp)
	writeJSON(t, filepath.Join(dir, "competition-unknown.json"), ranking.GoldenCompetition{Scope: "competition", Code: "unknown"})
	out.Reset()
	err = verifyGolden(ctx, gen.New(db.Read), dir, &out)
	if err == nil {
		t.Fatal("mismatch not detected")
	}
	for _, want := range []string{"overall: scores differ", "final true, want false", "competition unknown: not in the database"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}
