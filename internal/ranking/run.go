package ranking

import (
	"regexp"
	"sort"

	"awt/internal/scoring"
)

// Season code patterns of spec 03 §7.1 (legacy create_sub_results).
var (
	awtSeason     = regexp.MustCompile(`^awt-\d{4}$`)
	awqSeason     = regexp.MustCompile(`^awq-\d{4}$`)
	countrySeason = regexp.MustCompile(`^\w\w\w-\d{4}$`)
)

// Run ranks one run (spec 03 §7). With publishedOnly, unpublished flights
// are left out; Final is computed over every flight either way.
func Run(in RunInput, publishedOnly bool) RunResult {
	overall, allPublished := runEntries(in.Flights, publishedOnly)
	return RunResult{
		Lists: buildLists(in.Type, in.Seasons, overall, overall),
		Final: in.State == stateClosed && allPublished,
	}
}

// runEntries returns one entry per participant (the last saved flight
// wins, spec 03 §7 step 1) with the flight score as read from storage
// (Round3, spec 03 §9), and whether every flight is published.
func runEntries(flights []FlightScore, publishedOnly bool) ([]Entry, bool) {
	allPublished := true
	var entries []Entry
	index := map[string]int{}
	for _, f := range flights {
		if !f.Published {
			allPublished = false
			if publishedOnly {
				continue
			}
		}
		e := Entry{Participant: f.Participant, Score: scoring.Round3(f.Score)}
		if i, ok := index[f.Participant.key()]; ok {
			entries[i] = e
			continue
		}
		index[f.Participant.key()] = len(entries)
		entries = append(entries, e)
	}
	return entries, allPublished
}

// buildLists sorts overall and builds the sub-rankings of spec 03 §7.1.
// population is the set of participants whose women count decides the
// women list (the competition's participants for runs of a competition).
func buildLists(typ string, seasons []string, overall, population []Entry) Lists {
	sorted := rankEntries(overall)
	lists := Lists{listOverall: sorted}
	for _, s := range seasons {
		var keep func(Participant) bool
		switch {
		case typ == typeSynchro:
			keep = func(Participant) bool { return true }
		case awtSeason.MatchString(s):
			keep = func(p Participant) bool { return p.IsAWT }
		case awqSeason.MatchString(s):
			keep = func(p Participant) bool { return !p.IsAWT }
		case countrySeason.MatchString(s):
			country := s[:3]
			keep = func(p Participant) bool { return p.Country == country }
		default:
			continue
		}
		lists[s] = filterEntries(sorted, keep)
	}
	if typ != typeSynchro && countWomen(population) >= minWomen {
		lists[listWomen] = filterEntries(sorted, func(p Participant) bool { return p.Gender == genderWoman })
	}
	return lists
}

func countWomen(entries []Entry) int {
	n := 0
	for _, e := range entries {
		if e.Participant.Gender == genderWoman {
			n++
		}
	}
	return n
}

// filterEntries copies the kept entries of a sorted list and re-ranks them.
func filterEntries(sorted []Entry, keep func(Participant) bool) []Entry {
	out := []Entry{}
	for _, e := range sorted {
		if keep(e.Participant) {
			out = append(out, e)
		}
	}
	return rankEntries(out)
}

// rankEntries returns a copy of entries ordered by (Round3 score desc,
// display name asc, id asc), with positions and shared ranks for equal
// Round3 scores (decision 0007).
func rankEntries(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	copy(out, entries)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := scoring.Round3(out[i].Score), scoring.Round3(out[j].Score)
		if si != sj {
			return si > sj
		}
		return out[i].Participant.less(out[j].Participant)
	})
	for i := range out {
		out[i].Position = i + 1
		out[i].Rank = i + 1
		if i > 0 && scoring.Round3(out[i].Score) == scoring.Round3(out[i-1].Score) {
			out[i].Rank = out[i-1].Rank
		}
	}
	return out
}
