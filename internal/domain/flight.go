package domain

import "fmt"

// CanSaveFlight allows saving a flight only in an open run (decision 0006 #15).
func CanSaveFlight(state RunState) error {
	if state != RunOpen {
		return fmt.Errorf("flights can only be saved in an open run, run is %s: %w", state, ErrInvalidTransition)
	}
	return nil
}
