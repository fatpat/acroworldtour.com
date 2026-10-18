package scoring

import (
	"math"
	"testing"
)

func TestRound3(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"exact half ties to even", 0.0625, 0.062},
		{"binary above half rounds up", 2.0005, 2.001},
		{"binary below half rounds down", 1.0005, 1.0},
		{"plain value", 7.8933333, 7.893},
		{"tiny negative", -0.0004, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Round3(tc.in); got != tc.want {
				t.Errorf("Round3(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestRound3NegativeZero(t *testing.T) {
	got := Round3(-0.0004)
	if got != 0 || !math.Signbit(got) {
		t.Errorf("Round3(-0.0004) = %v (signbit %v), want -0.0", got, math.Signbit(got))
	}
}
