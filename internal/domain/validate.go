// Package domain holds entities, state machines and validation. It does no I/O.
package domain

import (
	"errors"
	"fmt"
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
// Only plain decimals are numbers: digits with at most one "." or ","
// followed by digits. A leading "-" is a negative mark, out of range.
func ValidateMark(raw string) (*float64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	if neg, ok := strings.CutPrefix(s, "-"); ok {
		if !plainDecimal(neg) {
			return nil, fmt.Errorf("mark must be a number: %q", raw)
		}
		return nil, ErrMarkRange
	}
	if !plainDecimal(s) {
		return nil, fmt.Errorf("mark must be a number: %q", raw)
	}
	v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil {
		return nil, fmt.Errorf("mark must be a number: %q", raw)
	}
	if v < 0 || v > 10 {
		return nil, ErrMarkRange
	}
	return &v, nil
}

// plainDecimal reports whether s is digits, optionally followed by one "."
// or "," and more digits.
func plainDecimal(s string) bool {
	intPart, frac, hasSep := strings.Cut(strings.Replace(s, ",", ".", 1), ".")
	return allDigits(intPart) && (!hasSep || allDigits(frac))
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
