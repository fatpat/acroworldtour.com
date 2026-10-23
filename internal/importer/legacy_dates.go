package importer

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"time"
)

// legacyNewsDatesFile records, for every legacy news page, the date of the
// commit that added it to the legacy repository (spec 07 §10). The app image
// has no git, so imports read this file instead of the history. Regenerate
// with `make www-dates`; a test keeps it in line with the history.
const legacyNewsDatesFile = "legacy_news_dates.tsv"

//go:embed legacy_news_dates.tsv
var legacyNewsDatesTSV []byte

// parseLegacyNewsDates reads "slug<TAB>RFC 3339 UTC date" lines.
func parseLegacyNewsDates(b []byte) (map[string]string, error) {
	out := map[string]string{}
	for i, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		slug, date, ok := strings.Cut(line, "\t")
		if !ok || slug == "" {
			return nil, fmt.Errorf("%s line %d: want slug<TAB>date", legacyNewsDatesFile, i+1)
		}
		if _, err := time.Parse(time.RFC3339, date); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", legacyNewsDatesFile, i+1, err)
		}
		if _, dup := out[slug]; dup {
			return nil, fmt.Errorf("%s line %d: duplicate slug %s", legacyNewsDatesFile, i+1, slug)
		}
		out[slug] = date
	}
	return out, nil
}

func formatLegacyNewsDates(dates map[string]string) []byte {
	slugs := make([]string, 0, len(dates))
	for s := range dates {
		slugs = append(slugs, s)
	}
	slices.Sort(slugs)
	var b strings.Builder
	for _, s := range slugs {
		fmt.Fprintf(&b, "%s\t%s\n", s, dates[s])
	}
	return []byte(b.String())
}
