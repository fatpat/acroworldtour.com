package scoring

import "strconv"

// Round3 reproduces Python's built-in round(x, 3): correct rounding of the
// exact binary value, ties-to-even on exact halves (spec 03 §9). It is the
// only rounding helper the scoring engine may use.
func Round3(x float64) float64 {
	r, err := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 3, 64), 64)
	if err != nil {
		// Only reachable for NaN/Inf, which FormatFloat renders as text
		// ParseFloat accepts; keep the input unchanged otherwise.
		return x
	}
	return r
}
