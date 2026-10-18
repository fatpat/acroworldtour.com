package ranking

import "awt/internal/scoring"

// Competition ranks a competition from its runs (spec 03 §8). limit is the
// index of the last run to include (-1: all runs). Every flight counts,
// published or not (decision 0005); Final requires a closed competition
// and every included run final.
func Competition(state string, runs []RunInput, limit int) CompetitionResult {
	if limit >= 0 && limit+1 < len(runs) {
		runs = runs[:limit+1]
	}
	final := state == stateClosed
	var typ string
	var seasons []string
	if len(runs) > 0 {
		typ, seasons = runs[0].Type, runs[0].Seasons
	}

	runEntriesList := make([][]Entry, len(runs))
	runFinal := make([]bool, len(runs))
	var overall []Entry
	index := map[string]int{}
	for i, r := range runs {
		entries, allPublished := runEntries(r.Flights, false)
		runEntriesList[i] = entries
		runFinal[i] = r.State == stateClosed && allPublished
		if !runFinal[i] {
			final = false
		}
		for _, e := range entries {
			k := e.Participant.key()
			j, ok := index[k]
			if !ok {
				j = len(overall)
				index[k] = j
				overall = append(overall, Entry{Participant: e.Participant})
			}
			// spec 03 §8: Round3 at every addition
			overall[j].Score = scoring.Round3(overall[j].Score + e.Score)
		}
	}

	lists := buildLists(typ, seasons, overall, overall)
	result := CompetitionResult{Lists: lists, Runs: make([]RunResult, len(runs)), Final: final}
	for i := range runs {
		// run sub-rankings use the competition's participants (spec 03 §8)
		result.Runs[i] = RunResult{Lists: buildLists(typ, seasons, runEntriesList[i], overall), Final: runFinal[i]}
	}
	for name, list := range lists {
		for j := range list {
			list[j].PerRun = perRun(list[j].Participant, name, result.Runs)
		}
	}
	return result
}

// perRun is the participant's rank and score in the given list of every
// run it flew (decision 0006 #3-4).
func perRun(p Participant, list string, runs []RunResult) []RunCell {
	var cells []RunCell
	k := p.key()
	for i, r := range runs {
		for _, e := range r.Lists[list] {
			if e.Participant.key() == k {
				cells = append(cells, RunCell{RunIndex: i, Rank: e.Rank, Score: e.Score})
				break
			}
		}
	}
	return cells
}
