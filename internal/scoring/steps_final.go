package scoring

import "fmt"

// components is step 15 (spec 03 §5.7), in the legacy operation order.
func components(in Input, cfg Config, j judges, anyKept bool, mark *FinalMark) {
	p := cfg.MarkPercentages[in.Type]
	mark.Technical = mark.Technicity * j.technical * p["technical"] / 100
	if anyKept {
		mark.Choreography = j.choreography * p["choreography"] / 100
	}
	mark.Landing = j.landing * p["landing"] / 100
	if in.Type == Synchro {
		mark.Synchro = j.synchro * p["synchro"] / 100
	}
	mark.Bonus = (mark.Technical + mark.Choreography) * (mark.BonusPercentage - mark.Malus) / 100
	mark.Score = mark.Technical + mark.Choreography + mark.Landing + mark.Synchro + mark.Bonus
}

// applyPenalties is step 16 (spec 03 §6.4): cumulative deduction per
// category; the cat-2 note prints the running cat 1 + cat 2 total.
func applyPenalties(cfg Config, prev1, prev2 int, mark *FinalMark) {
	var penalty float64
	if n := len(mark.Warnings1); n > 0 {
		// The conversion prevents a fused multiply-add (Go spec).
		penalty += float64(float64(n+prev1) * cfg.Warning)
		mark.Notes = append(mark.Notes, fmt.Sprintf("final mark has been lowered by %s because of category 1 warning(s)", pyFloat(penalty)))
	}
	if n := len(mark.Warnings2); n > 0 {
		penalty += float64(float64(n+prev2) * cfg.Warning2)
		mark.Notes = append(mark.Notes, fmt.Sprintf("final mark has been lowered by %s because of category 2 warning(s)", pyFloat(penalty)))
	}
	mark.Score -= penalty
	if mark.Score < 0 {
		mark.Score = 0
	}
}
