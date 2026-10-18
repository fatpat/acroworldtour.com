package ranking

import "sort"

// NextStartOrder computes the starting order of a new run (spec 03 §8.1)
// from the current competition ranking and the previous run's starting
// list prev, which is not modified (decision 0006 #2).
//
// Participants fly from worst to best (the leader last), participants
// without result after them in prev order. In a mixed solo event (exactly
// one awt-YYYY and one awq-YYYY season) positions are taken in the AWT
// list followed by the AWQ list, so AWQ pilots fly first. With qualify > 0
// only the best qualify participants by that position are kept
// (decision 0006 #1).
func NextStartOrder(comp CompetitionResult, prev []Participant, seasons []string, qualify int) []Participant {
	order := comp.Lists[listOverall]
	if awt, awq, ok := mixedSeasons(seasons); ok && isSolo(order) {
		order = append(append([]Entry{}, comp.Lists[awt]...), comp.Lists[awq]...)
	}
	pos := map[string]int{}
	for i, e := range order {
		if _, ok := pos[e.Participant.key()]; !ok {
			pos[e.Participant.key()] = i
		}
	}

	out := make([]Participant, len(prev))
	copy(out, prev)
	if qualify > 0 && qualify < len(out) {
		// best first, participants without result last in prev order
		sort.SliceStable(out, func(i, j int) bool {
			pi, oki := pos[out[i].key()]
			pj, okj := pos[out[j].key()]
			if oki != okj {
				return oki
			}
			return oki && pi < pj
		})
		out = out[:qualify]
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, oki := pos[out[i].key()]
		pj, okj := pos[out[j].key()]
		if oki != okj {
			return oki
		}
		return oki && pi > pj
	})
	return out
}

// mixedSeasons returns the awt and awq season codes when there is exactly
// one of each.
func mixedSeasons(seasons []string) (awt, awq string, ok bool) {
	var nAWT, nAWQ int
	for _, s := range seasons {
		switch {
		case awtSeason.MatchString(s):
			awt = s
			nAWT++
		case awqSeason.MatchString(s):
			awq = s
			nAWQ++
		}
	}
	return awt, awq, nAWT == 1 && nAWQ == 1
}

func isSolo(entries []Entry) bool {
	return len(entries) > 0 && entries[0].Participant.TeamID == ""
}
