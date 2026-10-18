package ranking

import "awt/internal/scoring"

// SharedRank is the rank of the n-th (0-based) entry of a list ordered by
// score, given the previous entry's rank and score: equal Round3 scores
// share the previous rank (decision 0007).
func SharedRank(n, prevRank int, prevScore, score float64) int {
	if n > 0 && scoring.Round3(prevScore) == scoring.Round3(score) {
		return prevRank
	}
	return n + 1
}

// RunPlace is a participant's place in one run's stored list, restricted to
// published flights. Key identifies the participant (any stable caller key).
type RunPlace struct {
	RunIndex int
	Key      string
	Score    float64
}

// PublishedRunCells re-ranks every run's published places with shared ranks
// and returns each participant's cells in run order. places must be ordered
// by run, then by stored position. Public reads (overall tab, JSON API) show
// these instead of the stored per-run cells, which count unpublished flights
// (decision 0005 keeps those in the totals only).
func PublishedRunCells(places []RunPlace) map[string][]RunCell {
	cells := map[string][]RunCell{}
	n, prev := 0, RunCell{}
	for i, pl := range places {
		if i == 0 || places[i-1].RunIndex != pl.RunIndex {
			n = 0
		}
		c := RunCell{RunIndex: pl.RunIndex, Rank: SharedRank(n, prev.Rank, prev.Score, pl.Score), Score: pl.Score}
		cells[pl.Key] = append(cells[pl.Key], c)
		n, prev = n+1, c
	}
	return cells
}
