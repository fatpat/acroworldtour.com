package scoring

import (
	"math"
	"strconv"
	"strings"
)

// pyFloat formats x like Python's str(float) / repr(float), which the legacy
// notes f-strings use for float values (e.g. 13.0, 0.5, 1e-05).
func pyFloat(x float64) string {
	switch {
	case math.IsNaN(x):
		return "nan"
	case math.IsInf(x, 1):
		return "inf"
	case math.IsInf(x, -1):
		return "-inf"
	}
	e := strconv.FormatFloat(x, 'e', -1, 64)
	exp, _ := strconv.Atoi(e[strings.IndexByte(e, 'e')+1:])
	// Python switches to scientific notation when the decimal exponent is
	// below -4 or at least 16; both use at least two exponent digits.
	if exp < -4 || exp >= 16 {
		return e
	}
	s := strconv.FormatFloat(x, 'f', -1, 64)
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	return s
}

// pyInt formats a value that legacy held as a Python int (max bonus per run,
// tip-touch percentage) without a decimal part.
func pyInt(x float64) string {
	return strconv.FormatFloat(x, 'f', -1, 64)
}
