package domain

import (
	"fmt"
	"strings"
)

type RunState string

const (
	RunInit   RunState = "init"
	RunOpen   RunState = "open"
	RunClosed RunState = "closed"
)

// RunTransition checks spec 02 §5.2: init->open, open->closed, closed->open.
func RunTransition(from, to RunState) error {
	switch {
	case from == RunInit && to == RunOpen,
		from == RunOpen && to == RunClosed,
		from == RunClosed && to == RunOpen:
		return nil
	}
	return fmt.Errorf("run %s -> %s: %w", from, to, ErrInvalidTransition)
}

// Participant is a run participant (pilot or team) for CloseRunCheck.
type Participant struct {
	ID   string
	Name string // display name; the id is used when empty
}

// FlightRef says whether a participant has a saved flight and its publication.
type FlightRef struct {
	ParticipantID string
	Published     bool
}

// CloseRunCheck requires every participant to have a published flight
// (spec 02 §5.2); the error lists the names of those who do not.
func CloseRunCheck(participants []Participant, flights []FlightRef) error {
	published := make(map[string]bool, len(flights))
	for _, f := range flights {
		if f.Published {
			published[f.ParticipantID] = true
		}
	}
	var missing []string
	for _, p := range participants {
		if published[p.ID] {
			continue
		}
		if p.Name != "" {
			missing = append(missing, p.Name)
		} else {
			missing = append(missing, p.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("participants without a published flight: %s", strings.Join(missing, ", "))
	}
	return nil
}
