package scoring

import "slices"

import "fmt"

// positionWarnings is step 6 (spec 03 §6.1): automatic cat-1 warnings from
// the catalog position rules, over all flown tricks (i 0-based, n flown).
func positionWarnings(in Input, tricks []FlownTrick) []string {
	var out []string
	n := len(tricks)
	for i, t := range tricks {
		r := in.PositionRules[t.BaseTrick]
		if r.First > 0 && i >= r.First {
			out = append(out, positionMessage(t.Name, "must be", "first", r.First))
		}
		if r.NoFirst > 0 && i < r.NoFirst {
			out = append(out, positionMessage(t.Name, "can't be", "first", r.NoFirst))
		}
		if r.Last > 0 && lastManeuverViolated(in.LegacyLastManeuver, i, n, r.Last) {
			out = append(out, positionMessage(t.Name, "must be", "last", r.Last))
		}
		if r.NoLast > 0 && i >= n-r.NoLast {
			out = append(out, positionMessage(t.Name, "can't be", "last", r.NoLast))
		}
		if slices.Contains(t.BonusTypes, "flip") {
			if i >= n-2 {
				out = append(out, fmt.Sprintf("flips can't be performed as one of the last 2 maneuvers (%s)", t.Name))
			}
		}
	}
	return out
}

// lastManeuverViolated: legacy warns when the trick IS in the last N
// (inverted test, spec 03 §14 #6); decision 0006 warns when it is not.
func lastManeuverViolated(legacy bool, i, n, last int) bool {
	if legacy {
		return i >= n-last
	}
	return i < n-last
}

func positionMessage(name, verb, where string, n int) string {
	if n == 1 {
		return fmt.Sprintf("%s %s the %s maneuver", name, verb, where)
	}
	return fmt.Sprintf("%s %s one of the %s %d maneuvers", name, verb, where, n)
}

// checkDSQ is step 7 (spec 03 §6.2). Previous counts are the stored operator
// warnings of all previous runs; it returns the DSQ note, if any.
func checkDSQ(in Input, cfg Config, n1, n2 int) (prev1, prev2 int, note string) {
	if in.RunIndex > 0 {
		for _, p := range in.Previous {
			if p.RunIndex < in.RunIndex {
				prev1 += p.Warnings1
				prev2 += p.Warnings2
			}
		}
		if prev1+prev2 >= cfg.WarningsToDSQ {
			return prev1, prev2, fmt.Sprintf("Pilot has been DSQ because he/she already had %d warnings", cfg.WarningsToDSQ)
		}
	}
	if n1+n2+prev1+prev2 >= cfg.WarningsToDSQ {
		return prev1, prev2, fmt.Sprintf("Pilot has been DSQ he/she's got %d warnings", cfg.WarningsToDSQ)
	}
	return prev1, prev2, ""
}
