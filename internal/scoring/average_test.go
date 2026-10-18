package scoring

import "testing"

func TestAverage(t *testing.T) {
	tests := []struct {
		name string
		in   []float64
		want float64
	}{
		{"exact sum avoids naive error", []float64{0.1, 0.2, 0.3}, 0.2},
		{"empty", nil, 0},
		{"single", []float64{7.5}, 7.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Average(tc.in); got != tc.want {
				t.Errorf("Average(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestWeightedAverage(t *testing.T) {
	tests := []struct {
		name string
		in   []Weighted
		want float64
	}{
		{"basic", []Weighted{{7, 100}, {8, 20}}, 7.166666666666667},
		{"zero weight only", []Weighted{{5, 0}}, 0},
		{"empty", nil, 0},
		{"negative total weight", []Weighted{{5, -1}}, 0},
		{"zero weight not skipped among others", []Weighted{{4, 0}, {6, 2}}, 6},
		{"negative weight not skipped", []Weighted{{10, 3}, {4, -1}}, 13},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := WeightedAverage(tc.in); got != tc.want {
				t.Errorf("WeightedAverage(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
