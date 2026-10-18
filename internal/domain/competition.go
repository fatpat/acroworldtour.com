package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type CompetitionType string

const (
	TypeSolo    CompetitionType = "solo"
	TypeSynchro CompetitionType = "synchro"
)

type CompetitionState string

const (
	CompetitionInit   CompetitionState = "init"
	CompetitionOpen   CompetitionState = "open"
	CompetitionClosed CompetitionState = "closed"
)

// ErrInvalidTransition is wrapped by every refused state transition.
var ErrInvalidTransition = errors.New("invalid state transition")

// Competition carries the fields the validators need (spec 02 §2.5).
type Competition struct {
	ID        string
	Name      string
	Code      string
	StartDate time.Time
	EndDate   time.Time
	Location  string
	Type      CompetitionType
	State     CompetitionState
	Seasons   []string
	Pilots    []int    // CIVL ids, solo only
	Teams     []string // team ids, synchro only
	Judges    []string // judge ids
}

// CanTransition checks spec 02 §5.1: init->open, open->closed, closed->open.
func CanTransition(from, to CompetitionState) error {
	switch {
	case from == CompetitionInit && to == CompetitionOpen,
		from == CompetitionOpen && to == CompetitionClosed,
		from == CompetitionClosed && to == CompetitionOpen:
		return nil
	}
	return fmt.Errorf("competition %s -> %s: %w", from, to, ErrInvalidTransition)
}

// CloseCompetitionCheck requires every run to be closed (spec 02 §5.1); the
// error lists the 1-based run numbers still not closed.
func CloseCompetitionCheck(runs []RunState) error {
	var pending []string
	for i, s := range runs {
		if s != RunClosed {
			pending = append(pending, fmt.Sprintf("%d", i+1))
		}
	}
	if len(pending) > 0 {
		return fmt.Errorf("runs not closed: %s", strings.Join(pending, ", "))
	}
	return nil
}

// CanCreateRun requires an open competition (spec 02 §5.1).
func CanCreateRun(state CompetitionState) error {
	if state != CompetitionOpen {
		return fmt.Errorf("competition must be open to create a run, is %s: %w", state, ErrInvalidTransition)
	}
	return nil
}

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]`)

// ValidateCompetition applies spec 02 §2.5.2 check() and field rules.
func ValidateCompetition(c Competition) []FieldError {
	var errs []FieldError
	add := func(f, m string) { errs = append(errs, FieldError{f, m}) }

	switch c.Type {
	case TypeSolo, TypeSynchro:
	default:
		add("type", "must be solo or synchro")
	}
	switch c.State {
	case CompetitionInit, CompetitionOpen, CompetitionClosed:
	default:
		add("state", "must be init, open or closed")
	}
	if c.Code != "" && !codePattern.MatchString(c.Code) {
		add("code", "invalid code")
	}
	if c.StartDate.After(c.EndDate) {
		add("start_date", "start date must be on or before end date")
	}
	if c.Type == TypeSolo {
		if dup(c.Pilots) {
			add("pilots", "duplicate pilots")
		}
	} else if c.Type == TypeSynchro && dup(c.Teams) {
		add("teams", "duplicate teams")
	}
	if c.State == CompetitionOpen || c.State == CompetitionClosed {
		if c.Type == TypeSolo && len(c.Pilots) < 2 {
			add("pilots", "at least 2 pilots are needed to open a competition")
		}
		if c.Type == TypeSynchro && len(c.Teams) < 2 {
			add("teams", "at least 2 teams are needed to open a competition")
		}
		if len(c.Judges) < 2 {
			add("judges", "at least 2 judges are needed to open a competition")
		}
	}
	return errs
}

func dup[T comparable](xs []T) bool {
	seen := make(map[T]struct{}, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			return true
		}
		seen[x] = struct{}{}
	}
	return false
}

// CanUpdateCompetition: updates need state != closed; name, code and type
// only change in init (spec 02 §5.1).
func CanUpdateCompetition(state CompetitionState, changesIdentity bool) error {
	if state == CompetitionClosed {
		return fmt.Errorf("competition is closed: %w", ErrInvalidTransition)
	}
	if changesIdentity && state != CompetitionInit {
		return fmt.Errorf("name, code and type can only change in init: %w", ErrInvalidTransition)
	}
	return nil
}
