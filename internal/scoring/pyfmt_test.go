package scoring

import (
	"math"
	"testing"
)

// Expected values are Python 3 repr(float) outputs.
func TestPyFloat(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{13, "13.0"},
		{0.5, "0.5"},
		{1, "1.0"},
		{3.5227272727272725, "3.5227272727272725"},
		{0.30000000000000004, "0.30000000000000004"}, // 0.1+0.2 in floats
		{1e16, "1e+16"},
		{1e15, "1000000000000000.0"},
		{123456789012345678, "1.2345678901234568e+17"},
		{0.0001, "0.0001"},
		{0.00001, "1e-05"},
		{1.5e-7, "1.5e-07"},
		{math.Copysign(0, -1), "-0.0"},
		{-2.5, "-2.5"},
		{1.0 / 3, "0.3333333333333333"},
		{1e22, "1e+22"},
		{5e-324, "5e-324"},
		{math.Inf(1), "inf"},
		{math.Inf(-1), "-inf"},
		{math.NaN(), "nan"},
	}
	for _, tc := range tests {
		if got := pyFloat(tc.in); got != tc.want {
			t.Errorf("pyFloat(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// pyInt formats integral config values that legacy stored as Python ints.
func TestPyInt(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{5, "5"}, {10, "10"}, {6, "6"}, {0, "0"},
	}
	for _, tc := range tests {
		if got := pyInt(tc.in); got != tc.want {
			t.Errorf("pyInt(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
