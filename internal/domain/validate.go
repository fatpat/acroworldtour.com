// Package domain holds entities, state machines and validation. It does no I/O.
package domain

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FieldError describes one invalid field.
type FieldError struct {
	Field string
	Msg   string
}

func (e FieldError) Error() string { return e.Field + ": " + e.Msg }

// ErrMarkRange is returned for marks outside 0-10 (decision 0006).
var ErrMarkRange = errors.New("mark must be between 0 and 10")

// ValidateMark parses a judge mark. Blank means "no mark" (nil). Both "7.5"
// and "7,5" are accepted; values outside 0-10 are rejected (decision 0006).
func ValidateMark(raw string) (*float64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, fmt.Errorf("mark must be a number: %q", raw)
	}
	if v < 0 || v > 10 {
		return nil, ErrMarkRange
	}
	return &v, nil
}
