package scoring

import "fmt"

// judgeWeights is step 3: every marking judge must be known; its weight is
// config.judge_weights[level] (spec 03 §4).
func judgeWeights(in Input, marks []JudgeMark) (map[string]float64, error) {
	weights := make(map[string]float64, len(marks))
	for _, m := range marks {
		if _, ok := weights[m.Judge]; ok {
			continue
		}
		level, ok := in.JudgeLevels[m.Judge]
		if !ok {
			return nil, ValidationError{"unknown judge " + m.Judge}
		}
		w, ok := in.Config.JudgeWeights[level]
		if !ok {
			return nil, ValidationError{fmt.Sprintf("no weight for judge level %q", level)}
		}
		weights[m.Judge] = w
	}
	return weights, nil
}

// fillTechnicalPerTrick is step 4. Legacy names the judge by its name in the
// first message; the engine only knows judge ids.
func fillTechnicalPerTrick(f Flight) error {
	withTPT := 0
	for _, m := range f.Marks {
		if m.TechnicalPerTrick == nil {
			continue
		}
		withTPT++
		if len(m.TechnicalPerTrick) != len(f.Tricks) {
			return ValidationError{fmt.Sprintf("judge '%s' did not mark enough tricks (%d instead of %d)",
				m.Judge, len(m.TechnicalPerTrick), len(f.Tricks))}
		}
	}
	if withTPT > 0 && withTPT < len(f.Marks) {
		return ValidationError{"somes judges sent technical mark per trick while other(s) did not"}
	}
	if withTPT == 0 {
		return nil
	}
	for i, trick := range f.Tricks {
		var set []float64
		missing := false
		for _, m := range f.Marks {
			if v := m.TechnicalPerTrick[i]; v != nil {
				set = append(set, *v)
			} else {
				missing = true
			}
		}
		if !missing {
			continue
		}
		if len(set) == 0 {
			return ValidationError{"not enough marks for " + trick.Name}
		}
		// Plain mean, not weighted (spec 03 §5.6).
		guessed := Average(set)
		for _, m := range f.Marks {
			if m.TechnicalPerTrick[i] == nil {
				v := guessed
				m.TechnicalPerTrick[i] = &v
			}
		}
	}
	return nil
}

// perTrickJudgeOrder lists the judges having per-trick marks in order of
// first appearance: the iteration order of the legacy technical_marks dicts.
func perTrickJudgeOrder(marks []JudgeMark) []string {
	var order []string
	seen := map[string]bool{}
	for _, m := range marks {
		if m.TechnicalPerTrick == nil || seen[m.Judge] {
			continue
		}
		seen[m.Judge] = true
		order = append(order, m.Judge)
	}
	return order
}

// perTrickMarks is step 5: per-trick technical marks for awt/awq. A judge
// appearing twice keeps its last value (legacy dict assignment).
func perTrickMarks(mt MarkType, f Flight, order []string, weights map[string]float64) error {
	if mt != MarkAWT && mt != MarkAWQ {
		return nil
	}
	for i := range f.Tricks {
		trick := &f.Tricks[i]
		trick.TechnicalMark = nil
		trick.TechnicalMarks = map[string]float64{}
		for _, m := range f.Marks {
			if m.TechnicalPerTrick == nil || i >= len(m.TechnicalPerTrick) {
				continue
			}
			trick.TechnicalMarks[m.Judge] = *m.TechnicalPerTrick[i]
		}
		if len(trick.TechnicalMarks) == 0 {
			return ValidationError{"no technical mark for trick " + trick.Acronym}
		}
		if mt == MarkAWT {
			pairs := make([]Weighted, 0, len(order))
			for _, j := range order {
				pairs = append(pairs, Weighted{trick.TechnicalMarks[j], weights[j]})
			}
			tm := WeightedAverage(pairs)
			trick.TechnicalMark = &tm
		}
	}
	return nil
}

// judgesAverages is step 8. A judge appearing twice counts twice here.
func judgesAverages(in Input, marks []JudgeMark, weights map[string]float64) (judges, error) {
	var t, c, l, s []Weighted
	for _, m := range marks {
		w := weights[m.Judge]
		if m.Technical != nil {
			t = append(t, Weighted{*m.Technical, w})
		}
		if m.Choreography != nil {
			c = append(c, Weighted{*m.Choreography, w})
		}
		if m.Landing != nil {
			l = append(l, Weighted{*m.Landing, w})
		}
		if in.Type == Synchro && m.Synchro != nil {
			s = append(s, Weighted{*m.Synchro, w})
		}
	}
	switch {
	case len(t) == 0 && in.MarkType != MarkAWT && in.MarkType != MarkAWQ:
		return judges{}, ValidationError{"not enough technical marks"}
	case len(c) == 0:
		return judges{}, ValidationError{"not enough choreography marks"}
	case len(l) == 0:
		return judges{}, ValidationError{"not enough landing marks"}
	case in.Type == Synchro && len(s) == 0:
		return judges{}, ValidationError{"not enough synchro marks"}
	}
	j := judges{
		technical:    WeightedAverage(t),
		choreography: WeightedAverage(c),
		landing:      WeightedAverage(l),
	}
	if in.Type == Synchro {
		j.synchro = WeightedAverage(s)
	}
	return j, nil
}

// directions are the legacy available_directions.
var directions = map[string]bool{"right": true, "left": true, "opposite": true}

// unbalancedDirections is the step 9 test (spec 03 §5.5) over all flown
// tricks, ignored ones included.
func unbalancedDirections(tricks []FlownTrick) bool {
	counts := map[string]int{}
	n := 0
	for _, t := range tricks {
		for _, u := range t.Uniqueness {
			if directions[u] {
				counts[u]++
				n++
				break
			}
		}
	}
	if n <= 1 {
		return false
	}
	lo, hi := n, 0
	for _, c := range counts {
		lo, hi = min(lo, c), max(hi, c)
	}
	return len(counts) == 1 || hi-lo > 1
}
