package importer

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateLegacyDates = flag.Bool("update-legacy-dates", false, "rewrite legacy_news_dates.tsv from the legacy git history")

const legacyWWW = "../../legacy/legacy/www"

// legacy_news_dates.tsv must hold exactly what the legacy git history gives
// for every legacy news page, so imports without git (the app image) match
// imports run next to the legacy repository.
func TestLegacyNewsDatesMatchTheGitHistory(t *testing.T) {
	if _, err := os.Stat(filepath.Join(legacyWWW, "..", ".git")); err != nil {
		t.Skip("legacy repository not checked out")
	}
	files, err := htmlFiles(filepath.Join(legacyWWW, "news"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for _, name := range files {
		if d := firstCommitDate(context.Background(), filepath.Join(legacyWWW, "news", name)); d != "" {
			want[slugOfFile(name)] = d
		}
	}
	if *updateLegacyDates {
		if err := os.WriteFile(legacyNewsDatesFile, formatLegacyNewsDates(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := parseLegacyNewsDates(legacyNewsDatesTSV)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Errorf("%d recorded dates, git history gives %d: run make www-dates", len(got), len(want))
	}
	for slug, d := range want {
		if got[slug] != d {
			t.Errorf("%s: recorded %q, git history gives %q: run make www-dates", slug, got[slug], d)
		}
	}
}

func TestParseLegacyNewsDatesRejectsBadLines(t *testing.T) {
	for _, in := range []string{"slug-only\n", "a\tnot-a-date\n", "a\t2023-05-26T17:41:27Z\na\t2023-05-26T17:41:27Z\n"} {
		if _, err := parseLegacyNewsDates([]byte(in)); err == nil {
			t.Errorf("parseLegacyNewsDates(%q): want an error", in)
		}
	}
}
