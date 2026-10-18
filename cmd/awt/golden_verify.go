package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"awt/internal/config"
	"awt/internal/ranking"
	"awt/internal/store"
	"awt/internal/store/gen"
)

const goldenVerifyUsage = "usage: awt golden-verify [--db path] [--golden dir]"

// tolerance of the score comparison: scores are Round3 values or plain float
// sums, so anything above float noise is a real difference.
const goldenTolerance = 1e-9

// goldenVerifyCmd checks the rankings stored by the importer against the
// golden corpus.
func goldenVerifyCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("golden-verify", flag.ContinueOnError)
	dbPath := fs.String("db", "", "database to verify (default: AWT_DB_PATH)")
	golden := fs.String("golden", filepath.Join("testdata", "golden", "rankings"), "directory of golden ranking files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New(goldenVerifyUsage)
	}
	path := *dbPath
	if path == "" {
		cfg, err := config.Load(os.Getenv)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		path = cfg.DBPath
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	db, err := store.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	return verifyGolden(context.Background(), gen.New(db.Read), *golden, out)
}

// verifyGolden compares, per golden file, the stored ranking lists with the
// legacy output. The corpus ids are pseudonymised, so participants cannot be
// matched: each list is compared by its number of entries and its sorted
// scores, plus the final flags. Season files are compared the same way.
func verifyGolden(ctx context.Context, q *gen.Queries, dir string, out io.Writer) error {
	comps, seasons, err := loadGolden(dir)
	if err != nil {
		return err
	}
	var diffs []string
	for _, g := range comps {
		diffs = append(diffs, verifyCompetition(ctx, q, g)...)
	}
	for _, g := range seasons {
		diffs = append(diffs, verifySeason(ctx, q, g)...)
	}
	if len(diffs) > 0 {
		for _, d := range diffs {
			fmt.Fprintln(out, d)
		}
		return fmt.Errorf("%d differences with the golden corpus", len(diffs))
	}
	fmt.Fprintf(out, "%d competitions, %d seasons OK\n", len(comps), len(seasons))
	return nil
}

func loadGolden(dir string) ([]ranking.GoldenCompetition, []ranking.GoldenSeason, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no golden ranking files in %s", dir)
	}
	slices.Sort(files)
	var comps []ranking.GoldenCompetition
	var seasons []ranking.GoldenSeason
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		base := filepath.Base(f)
		switch {
		case strings.HasPrefix(base, "competition-"):
			var g ranking.GoldenCompetition
			if err := json.Unmarshal(raw, &g); err != nil {
				return nil, nil, fmt.Errorf("%s: %w", base, err)
			}
			comps = append(comps, g)
		case strings.HasPrefix(base, "season-"):
			var g ranking.GoldenSeason
			if err := json.Unmarshal(raw, &g); err != nil {
				return nil, nil, fmt.Errorf("%s: %w", base, err)
			}
			seasons = append(seasons, g)
		}
	}
	return comps, seasons, nil
}

func verifyCompetition(ctx context.Context, q *gen.Queries, g ranking.GoldenCompetition) []string {
	where := "competition " + g.Code
	comp, err := q.GetCompetitionByCode(ctx, g.Code)
	if err != nil {
		return []string{fmt.Sprintf("%s: not in the database (%v)", where, err)}
	}
	var diffs []string
	if comp.ResultsFinal != g.Expected.Final {
		diffs = append(diffs, fmt.Sprintf("%s: final %v, want %v", where, comp.ResultsFinal, g.Expected.Final))
	}
	want := map[string][]float64{}
	for name, list := range g.Expected.Results {
		for _, e := range list {
			want[name] = append(want[name], e.Score)
		}
	}
	diffs = append(diffs, compareLists(ctx, q, where, store.ScopeCompetition, comp.ID, want)...)

	runs, err := q.ListRuns(ctx, comp.ID)
	if err != nil {
		return append(diffs, fmt.Sprintf("%s: %v", where, err))
	}
	if len(runs) != len(g.Expected.RunsResults) {
		return append(diffs, fmt.Sprintf("%s: %d runs, want %d", where, len(runs), len(g.Expected.RunsResults)))
	}
	for i, wr := range g.Expected.RunsResults {
		rw := fmt.Sprintf("%s run %d", where, i)
		if runs[i].ResultsFinal != wr.Final {
			diffs = append(diffs, fmt.Sprintf("%s: final %v, want %v", rw, runs[i].ResultsFinal, wr.Final))
		}
		wantRun := map[string][]float64{}
		for name, list := range wr.Results {
			for _, e := range list {
				wantRun[name] = append(wantRun[name], e.Score)
			}
		}
		diffs = append(diffs, compareLists(ctx, q, rw, store.ScopeRun, store.RunScopeKey(comp.ID, i), wantRun)...)
	}
	return diffs
}

func verifySeason(ctx context.Context, q *gen.Queries, g ranking.GoldenSeason) []string {
	where := "season " + g.Code
	if g.Expected == nil {
		return []string{fmt.Sprintf("%s: legacy error %q, not handled", where, g.Error)}
	}
	want := map[string][]float64{}
	for name, list := range g.Expected.Results {
		for _, e := range list {
			want[name] = append(want[name], e.Score)
		}
	}
	return compareLists(ctx, q, where, store.ScopeSeason, g.Code, want)
}

// compareLists compares the stored lists of (scope, key) with the wanted
// scores per list name. A missing stored list equals an empty one.
func compareLists(ctx context.Context, q *gen.Queries, where, scope, key string, want map[string][]float64) []string {
	keys, err := q.ListRankingListKeys(ctx, gen.ListRankingListKeysParams{Scope: scope, ScopeKey: key})
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", where, err)}
	}
	got := map[string][]float64{}
	for _, name := range keys {
		rows, err := q.ListRankingEntries(ctx, gen.ListRankingEntriesParams{Scope: scope, ScopeKey: key, ListKey: name})
		if err != nil {
			return []string{fmt.Sprintf("%s %s: %v", where, name, err)}
		}
		for _, r := range rows {
			got[name] = append(got[name], r.Score)
		}
	}
	names := map[string]bool{}
	for n, s := range want {
		if len(s) > 0 {
			names[n] = true
		}
	}
	for n := range got {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	slices.Sort(sorted)
	var diffs []string
	for _, n := range sorted {
		g, w := got[n], want[n]
		if len(g) != len(w) {
			diffs = append(diffs, fmt.Sprintf("%s %s: %d entries, want %d", where, n, len(g), len(w)))
			continue
		}
		slices.Sort(g)
		slices.Sort(w)
		for i := range g {
			if math.Abs(g[i]-w[i]) > goldenTolerance {
				diffs = append(diffs, fmt.Sprintf("%s %s: scores differ (got %v, want %v)", where, n, g, w))
				break
			}
		}
	}
	return diffs
}
