package scoring

import "math/big"

// Weighted is a (value, weight) pair for WeightedAverage.
type Weighted struct {
	Value, Weight float64
}

// Average reproduces Python's statistics.mean on floats (spec 03 §4): every
// value is converted to an exact fraction, summed exactly, divided by n and
// rounded once to the nearest double. An empty slice yields 0.
func Average(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := new(big.Rat)
	var r big.Rat
	for _, x := range xs {
		sum.Add(sum, r.SetFloat64(x))
	}
	sum.Quo(sum, new(big.Rat).SetInt64(int64(len(xs))))
	f, _ := sum.Float64()
	return f
}

// WeightedAverage reproduces the legacy weight_average (spec 03 §4): a naive
// left-to-right float accumulation of value*weight and weight. Zero and
// negative weights are NOT skipped (the legacy skip is a no-op). A
// non-positive total weight, or no pairs, yields 0.
func WeightedAverage(pairs []Weighted) float64 {
	var top, bottom float64
	for _, p := range pairs {
		top += p.Value * p.Weight
		bottom += p.Weight
	}
	if bottom <= 0 {
		return 0
	}
	return top / bottom
}
