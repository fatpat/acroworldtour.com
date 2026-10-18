package scoring

import (
	"maps"
	"slices"
)

// Score computes a flight's final mark following spec 03 §5.0 step by step
// (legacy Competition.calculate_score). It never mutates in; the per-trick
// fill-ins and technical marks of steps 4-5 are returned in Result. Invalid
// input yields a ValidationError carrying the legacy HTTP 400 message.
func Score(in Input) (Result, error) {
	flight := cloneFlight(in.Flight)
	res := Result{Tricks: flight.Tricks, Marks: flight.Marks}
	mark := FinalMark{
		JudgesMark: JudgeMark{Technical: new(float64), Choreography: new(float64), Landing: new(float64), Synchro: new(float64)},
		Warnings1:  append([]string{}, flight.Warnings1...),
		Warnings2:  append([]string{}, flight.Warnings2...),
		Notes:      []string{},
		MarkType:   in.MarkType,
	}
	done := func() (Result, error) {
		res.Final = mark
		return res, nil
	}

	// step 1: DNS
	if flight.DidNotStart {
		mark.Notes = append(mark.Notes, "Got 0 because of DNS")
		return done()
	}

	// step 2: config. The caller passes the run's own config copy.
	cfg := in.Config

	// step 3: judges
	weights, err := judgeWeights(in, flight.Marks)
	if err != nil {
		return Result{}, err
	}

	// step 4: per-trick technical marks consistency and fill-in
	if err := fillTechnicalPerTrick(flight); err != nil {
		return Result{}, err
	}

	// step 5: AWT/AWQ per-trick structures
	judgeOrder := perTrickJudgeOrder(flight.Marks)
	if err := perTrickMarks(in.MarkType, flight, judgeOrder, weights); err != nil {
		return Result{}, err
	}

	// step 6: position warnings (cat 1, automatic)
	mark.Warnings1 = append(mark.Warnings1, positionWarnings(in, flight.Tricks)...)

	// step 7: previous warnings and DSQ
	prev1, prev2, dsqNote := checkDSQ(in, cfg, len(mark.Warnings1), len(mark.Warnings2))
	if dsqNote != "" {
		mark.Notes = append(mark.Notes, dsqNote)
		return done()
	}

	// step 8: weighted averages of judges' marks
	j, err := judgesAverages(in, flight.Marks, weights)
	if err != nil {
		return Result{}, err
	}

	// step 9: direction balance (solo only)
	if in.Type == Solo && len(flight.Tricks) > 1 && unbalancedDirections(flight.Tricks) && j.choreography >= 1 {
		mark.Notes = append(mark.Notes, "choreography mark has been lower by 1 point because direction is not even")
		j.choreography -= 1
	}

	// step 10: repetition malus
	mark.Malus, mark.Notes = repetitionMalus(in, cfg, flight.Tricks, mark.Notes)

	// step 11: ignored tricks
	var kept []FlownTrick
	kept, mark.Notes = keptTricks(in, cfg, flight.Tricks, mark.Notes)

	// step 12: technical mark per trick
	if err := perTrickTechnical(in.MarkType, kept, judgeOrder, weights, &j); err != nil {
		return Result{}, err
	}

	// step 13: technicity and bonus
	mark.Technicity, mark.BonusPercentage, mark.Notes = technicityAndBonus(kept, mark.Notes)

	// step 14: tip touches (synchro)
	if in.Type == Synchro && flight.TipTouches > 0 {
		tt := min(float64(flight.TipTouches)*in.Rules.TipTouchPctPerTouch, in.Rules.TipTouchPctMax)
		mark.Notes = append(mark.Notes, pyInt(tt)+"% tip-touch bonus")
		mark.BonusPercentage += tt
	}

	// step 15: components
	components(in, cfg, j, len(kept) > 0, &mark)

	// step 16: warning penalties
	applyPenalties(cfg, prev1, prev2, &mark)

	*mark.JudgesMark.Technical = j.technical
	*mark.JudgesMark.Choreography = j.choreography
	*mark.JudgesMark.Landing = j.landing
	*mark.JudgesMark.Synchro = j.synchro
	return done()
}

// judges holds the averaged judges' marks (J in spec 03 §5.0).
type judges struct {
	technical, choreography, landing, synchro float64
}

// cloneFlight deep-copies what Score replaces (tricks' technical marks,
// marks' per-trick values) so the input is never mutated.
func cloneFlight(f Flight) Flight {
	out := f
	out.Tricks = make([]FlownTrick, len(f.Tricks))
	for i, t := range f.Tricks {
		t.TechnicalMarks = maps.Clone(t.TechnicalMarks)
		out.Tricks[i] = t
	}
	out.Marks = make([]JudgeMark, len(f.Marks))
	for i, m := range f.Marks {
		m.TechnicalPerTrick = slices.Clone(m.TechnicalPerTrick)
		out.Marks[i] = m
	}
	return out
}
