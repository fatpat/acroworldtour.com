package domain

import (
	"fmt"
	"regexp"
	"strings"
)

type JudgeLevel string

const (
	JudgeTrainee   JudgeLevel = "trainee"
	JudgeCertified JudgeLevel = "certified"
	JudgeSenior    JudgeLevel = "senior"
)

type Gender string

const (
	GenderMan   Gender = "man"
	GenderWoman Gender = "woman"
	GenderNone  Gender = "none"
)

// Season (spec 02 §2.11).
type Season struct {
	Name    string
	Code    string
	Year    int
	Country string // optional lower-case alpha-3
}

// Team (spec 02 §2.2).
type Team struct {
	Name   string
	Pilots []int
}

// Judge (spec 02 §2.3).
type Judge struct {
	Name    string
	Country string
	Level   JudgeLevel
}

var (
	seasonCodePattern = regexp.MustCompile(`^[a-z]{3}-[0-9]{4}$`)
	countryPattern    = regexp.MustCompile(`^[a-z]{3}$`)
)

// EffectiveCode forces the code to {country}-{year} when a country is set.
func (s Season) EffectiveCode() string {
	if s.Country != "" {
		return fmt.Sprintf("%s-%d", s.Country, s.Year)
	}
	return s.Code
}

func ValidateSeason(s Season) []FieldError {
	var errs []FieldError
	add := func(f, m string) { errs = append(errs, FieldError{f, m}) }
	if len([]rune(strings.TrimSpace(s.Name))) < 2 {
		add("name", "must be at least 2 characters")
	}
	if s.Year <= 1900 {
		add("year", "must be after 1900")
	}
	if s.Country != "" && !countryPattern.MatchString(s.Country) {
		add("country", "must be a lower-case alpha-3 code")
	}
	code := s.EffectiveCode()
	switch {
	case !seasonCodePattern.MatchString(code):
		add("code", "must match ^[a-z]{3}-[0-9]{4}$")
	case !strings.HasSuffix(code, fmt.Sprintf("-%d", s.Year)):
		add("code", "must end with -{year}")
	}
	return errs
}

func ValidateTeam(t Team) []FieldError {
	var errs []FieldError
	if strings.TrimSpace(t.Name) == "" {
		errs = append(errs, FieldError{"name", "required"})
	}
	if len(t.Pilots) != 2 {
		errs = append(errs, FieldError{"pilots", "a team has exactly 2 pilots"})
	}
	return errs
}

func ValidateJudge(j Judge) []FieldError {
	var errs []FieldError
	if len([]rune(strings.TrimSpace(j.Name))) < 2 {
		errs = append(errs, FieldError{"name", "must be at least 2 characters"})
	}
	if len(j.Country) != 3 {
		errs = append(errs, FieldError{"country", "must be an alpha-3 code"})
	}
	switch j.Level {
	case JudgeTrainee, JudgeCertified, JudgeSenior:
	default:
		errs = append(errs, FieldError{"level", "must be trainee, certified or senior"})
	}
	return errs
}
