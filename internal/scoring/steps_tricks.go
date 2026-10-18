package scoring

import (
	"fmt"
	"slices"
)

// sameTrick is the repetition identity (spec 03 §5.3): same base trick and
// identical uniqueness list.
func sameTrick(a, b FlownTrick) bool {
	return a.BaseTrick == b.BaseTrick && slices.Equal(a.Uniqueness, b.Uniqueness)
}

// firstRun is the index of the last run in 0..RunIndex whose reset policy
// applies to this pilot (spec 03 §5.3, decision 0004). Synchro never resets.
func firstRun(in Input) int {
	first := 0
	if in.Type != Solo {
		return first
	}
	for i, p := range in.ResetPolicies {
		if i > in.RunIndex {
			break
		}
		if p == ResetAll ||
			(p == ResetAWT && in.IsAWTPilot && in.HasAWTSeason) ||
			(p == ResetAWQ && !in.IsAWTPilot && in.HasAWQSeason) {
			first = i
		}
	}
	return first
}

// repetitionMalus is step 10, run on all flown tricks before ignoring. A
// trick repeated both in a previous run and earlier in this run gets the
// malus twice.
func repetitionMalus(in Input, cfg Config, tricks []FlownTrick, notes []string) (float64, []string) {
	var malus float64
	repeatable := func(t FlownTrick) bool { return slices.Contains(in.RepeatableBaseTricks, t.BaseTrick) }
	pct := pyFloat(cfg.MalusRepetition)

	if in.RunIndex > 0 {
		first := firstRun(in)
		for k, trick := range tricks {
			if repeatable(trick) {
				continue
			}
		previous:
			for _, p := range in.Previous {
				if p.RunIndex < first || p.RunIndex >= in.RunIndex {
					continue
				}
				for _, t := range p.Tricks {
					if sameTrick(t, trick) {
						malus += cfg.MalusRepetition
						notes = append(notes, fmt.Sprintf("trick number #%d (%s) has already been performed in a previous run. Adding a %s%% malus.", k+1, trick.Name, pct))
						break previous
					}
				}
			}
		}
	}

	for k, trick := range tricks {
		if repeatable(trick) {
			continue
		}
		for _, t := range tricks[:k] {
			if sameTrick(t, trick) {
				malus += cfg.MalusRepetition
				notes = append(notes, fmt.Sprintf("trick number #%d (%s) has already been performed in this run. Adding a %s%% malus.", k+1, trick.Name, pct))
				break
			}
		}
	}
	return malus, notes
}

// keptTricks is step 11 (spec 03 §5.4). Notes number tricks from 0 here
// (legacy enumerate), unlike repetition notes. Counters are incremented even
// for a trick that ends up ignored by another counter.
func keptTricks(in Input, cfg Config, tricks []FlownTrick, notes []string) ([]FlownTrick, []string) {
	var kept []FlownTrick
	nBonus := map[string]int{}
	nType := map[string]int{}
	for i, trick := range tricks {
		ignore := false
		switch {
		case in.Type == Solo && !trick.Solo:
			notes = append(notes, fmt.Sprintf("Ignoring trick #%d (%s) because it's not allowed in a solo competition", i, trick.Name))
			ignore = true
		case in.Type == Solo && in.IsAWTPilot && in.HasAWTSeason && !trick.SoloAWT:
			notes = append(notes, fmt.Sprintf("Ignoring trick #%d (%s) because it's not allowed in AWT solo competition", i, trick.Name))
			ignore = true
		case in.Type == Synchro && !trick.Synchro:
			notes = append(notes, fmt.Sprintf("Ignoring trick #%d (%s) because it's not allowed in a synchro competition", i, trick.Name))
			ignore = true
		}
		if !ignore {
			for _, bt := range trick.BonusTypes {
				nBonus[bt]++
				maxPerRun, ok := cfg.MaxBonusPerRun[bt]
				if !ok {
					maxPerRun = in.Rules.DefaultMaxBonus
				}
				if float64(nBonus[bt]) > maxPerRun {
					notes = append(notes, fmt.Sprintf("trick number #%d (%s) has been ignored because more than %s %s tricks have been flown", i, trick.Name, pyInt(maxPerRun), bt))
					ignore = true
				}
			}
			for _, ty := range trick.Types {
				limit, ok := in.Rules.MaxTricksPerType[ty]
				if !ok {
					continue
				}
				nType[ty]++
				if nType[ty] > limit {
					notes = append(notes, fmt.Sprintf("trick number #%d (%s) has been ignored because more than %d '%s' tricks have been flown", i, trick.Name, limit, ty))
					ignore = true
				}
			}
		}
		if !ignore {
			kept = append(kept, trick)
		}
	}
	return kept, notes
}

// perTrickTechnical is step 12 (spec 03 §5.6).
func perTrickTechnical(mt MarkType, kept []FlownTrick, order []string, weights map[string]float64, j *judges) error {
	switch mt {
	case MarkAWT:
		marks := make([]float64, 0, len(kept))
		for _, t := range kept {
			if t.TechnicalMark == nil {
				return ValidationError{"not enough technicals marks for trick " + t.Acronym}
			}
			marks = append(marks, *t.TechnicalMark)
		}
		j.technical = Average(marks)
	case MarkAWQ:
		perJudge := map[string][]float64{}
		for _, t := range kept {
			if len(t.TechnicalMarks) == 0 {
				return ValidationError{"not enough technicals marks for trick " + t.Acronym}
			}
			for _, jd := range order {
				if v, ok := t.TechnicalMarks[jd]; ok {
					perJudge[jd] = append(perJudge[jd], v)
				}
			}
		}
		var pairs []Weighted
		for _, jd := range order {
			if vs, ok := perJudge[jd]; ok {
				pairs = append(pairs, Weighted{Average(vs), weights[jd]})
			}
		}
		j.technical = WeightedAverage(pairs)
	}
	return nil
}

// technicityAndBonus is step 13 (spec 03 §5.1, §5.6). Sums are left-to-right
// float additions like Python's sum().
func technicityAndBonus(kept []FlownTrick, notes []string) (technicity, bonus float64, _ []string) {
	coefs := make([]float64, 0, len(kept)+3)
	var details []float64
	for _, t := range kept {
		if t.Bonus > 0 {
			bonus += t.Bonus
			// spec 03 §6.6.1: each bonus is scaled by the trick's
			// technical mark when it has one.
			if t.TechnicalMark != nil {
				details = append(details, t.Bonus*(*t.TechnicalMark)/10)
			}
		}
		coefs = append(coefs, t.TechnicalCoefficient)
	}
	if details != nil {
		var scaled float64
		for _, d := range details {
			scaled += d
		}
		if bonus != scaled {
			notes = append(notes, fmt.Sprintf("Initial total bonus of %s%% has been lowered by technical mark to %s%%", pyFloat(bonus), pyFloat(Round3(scaled))))
			bonus = scaled
		}
	}
	slices.Sort(coefs)
	slices.Reverse(coefs)
	coefs = append(coefs, 1.0, 1.0, 1.0)
	return Average(coefs[:3]), bonus, notes
}
