package ranking

// SeasonInput is a season to rank: its code, its country ("" when not a
// national season), its type ("solo" or "synchro") and its competitions.
type SeasonInput struct {
	Code, Country string
	Type          string
	Competitions  []SeasonComp
}

// SeasonComp is a competition of the season with its ranking.
type SeasonComp struct {
	Code              string
	Published, Closed bool
	Result            CompetitionResult
}

// Season ranks a season (spec 03 §10): the plain, unrounded sum of the
// competition totals of the competition list named after the season code,
// over published, closed and final competitions. It returns "overall" when
// any participant scored and "women" (solo) with at least 3 women.
func Season(in SeasonInput) Lists {
	var totals []Entry
	index := map[string]int{}
	for _, c := range in.Competitions {
		if !c.Published || !c.Closed || !c.Result.Final {
			continue
		}
		for _, e := range c.Result.Lists[in.Code] {
			if in.Type != typeSynchro && in.Country != "" && e.Participant.Country != in.Country {
				continue
			}
			k := e.Participant.key()
			j, ok := index[k]
			if !ok {
				j = len(totals)
				index[k] = j
				totals = append(totals, Entry{Participant: e.Participant})
			}
			totals[j].Score += e.Score
		}
	}
	lists := Lists{}
	if len(totals) == 0 {
		return lists
	}
	overall := rankEntries(totals)
	lists[listOverall] = overall
	if in.Type != typeSynchro && countWomen(overall) >= minWomen {
		lists[listWomen] = filterEntries(overall, func(p Participant) bool { return p.Gender == genderWoman })
	}
	return lists
}
