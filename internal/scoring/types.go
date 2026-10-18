package scoring

import "awt/internal/catalog"

// CompetitionType is "solo" or "synchro".
type CompetitionType string

// Competition types.
const (
	Solo    CompetitionType = "solo"
	Synchro CompetitionType = "synchro"
)

// MarkType is how judges enter technical marks (spec 03 §1, §5.6): "" for
// classic marking (one technical mark per judge), "awt" or "awq" for one
// technical mark per trick per judge.
type MarkType string

// Mark types.
const (
	MarkClassic MarkType = ""
	MarkAWT     MarkType = "awt"
	MarkAWQ     MarkType = "awq"
)

// JudgeLevel selects a judge's weight in Config.JudgeWeights.
type JudgeLevel string

// Judge levels.
const (
	Trainee   JudgeLevel = "trainee"
	Certified JudgeLevel = "certified"
	Senior    JudgeLevel = "senior"
)

// ResetPolicy is a run's repetitions reset policy (spec 03 §5.3).
type ResetPolicy int

// Reset policies, numbered as in the legacy RunRepetitionsResetPolicy.
const (
	ResetNone ResetPolicy = 0
	ResetAWT  ResetPolicy = 1
	ResetAWQ  ResetPolicy = 2
	ResetAll  ResetPolicy = 3
)

// Config is the run's scoring configuration (spec 03 §2).
type Config struct {
	Warning         float64 `json:"warning"`
	Warning2        float64 `json:"warning2"`
	MalusRepetition float64 `json:"malus_repetition"`
	WarningsToDSQ   int     `json:"warnings_to_dsq"`
	// JudgeWeights is keyed by judge level.
	JudgeWeights map[JudgeLevel]float64 `json:"judge_weights"`
	// MarkPercentages is keyed by competition type, then by component
	// ("technical", "choreography", "landing", "synchro").
	MarkPercentages map[CompetitionType]map[string]float64 `json:"mark_percentages"`
	// MaxBonusPerRun is keyed by bonus type ("twist", "reverse", "flip").
	MaxBonusPerRun map[string]float64 `json:"max_bonus_per_run"`
}

// Rules are the season-level constants that the legacy engine hard-coded
// (spec 03 §2 "Hard-coded constants").
type Rules struct {
	MaxTricksPerType    map[string]int `json:"max_tricks_per_type"`
	TipTouchPctPerTouch float64        `json:"tip_touch_pct_per_touch"`
	TipTouchPctMax      float64        `json:"tip_touch_pct_max"`
	// DefaultMaxBonus is the per-run limit of a bonus type absent from
	// Config.MaxBonusPerRun (e.g. "other").
	DefaultMaxBonus float64 `json:"default_max_bonus"`
}

// PositionRule holds a catalog trick's position constraints (spec 03 §6.1).
type PositionRule struct {
	First   int `json:"first_maneuver"`
	NoFirst int `json:"no_first_maneuver"`
	Last    int `json:"last_maneuver"`
	NoLast  int `json:"no_last_maneuver"`
}

// FlownTrick is a unique trick as snapshotted in a flight, plus the per-trick
// technical marks computed by AWT/AWQ marking (spec 03 §5.6).
type FlownTrick struct {
	catalog.UniqueTrick
	// TechnicalMark is the judges' weighted mark for this trick. Score
	// recomputes it for "awt" (and clears it for "awq"); with classic
	// marking the input value is kept and still scales the bonus, as in
	// legacy (stored flights may carry one from an earlier scoring).
	TechnicalMark  *float64           `json:"technical_mark"`
	TechnicalMarks map[string]float64 `json:"technical_marks"`
}

// JudgeMark is one judge's marks for a flight. A nil field means "no mark".
type JudgeMark struct {
	Judge             string     `json:"judge"`
	Technical         *float64   `json:"technical,omitempty"`
	Choreography      *float64   `json:"choreography,omitempty"`
	Landing           *float64   `json:"landing,omitempty"`
	Synchro           *float64   `json:"synchro,omitempty"`
	TechnicalPerTrick []*float64 `json:"technical_per_trick,omitempty"`
}

// Flight is the flight being scored.
type Flight struct {
	DidNotStart bool         `json:"did_not_start"`
	Tricks      []FlownTrick `json:"tricks"`
	Marks       []JudgeMark  `json:"marks"`
	// Warnings1 and Warnings2 are the operator-entered cat-1 and cat-2
	// warnings.
	Warnings1  []string `json:"warnings1"`
	Warnings2  []string `json:"warnings2"`
	TipTouches int      `json:"tip_touches"`
}

// PreviousFlight is the same pilot's (solo) or team's (synchro) flight in an
// earlier run of the competition.
type PreviousFlight struct {
	RunIndex int          `json:"run_index"`
	Tricks   []FlownTrick `json:"tricks"`
	// Warnings1 and Warnings2 count the stored operator warnings only.
	Warnings1 int `json:"warnings1"`
	Warnings2 int `json:"warnings2"`
}

// Input is everything Score needs; see doc.go for the JSON shape.
type Input struct {
	Type         CompetitionType `json:"type"`
	Config       Config          `json:"config"`
	Rules        Rules           `json:"rules"`
	RunIndex     int             `json:"run_index"`
	MarkType     MarkType        `json:"mark_type"`
	IsAWTPilot   bool            `json:"is_awt_pilot"`
	HasAWTSeason bool            `json:"has_awt_season"`
	HasAWQSeason bool            `json:"has_awq_season"`
	// RepeatableBaseTricks are the names of the competition's repeatable
	// catalog tricks.
	RepeatableBaseTricks []string `json:"repeatable_base_tricks"`
	// ResetPolicies holds the reset policy of runs 0..RunIndex
	// (decision 0004).
	ResetPolicies []ResetPolicy `json:"reset_policies"`
	// Previous holds the same pilot's/team's flights in runs < RunIndex,
	// ascending, only those that exist.
	Previous    []PreviousFlight      `json:"previous"`
	JudgeLevels map[string]JudgeLevel `json:"judge_levels"`
	// PositionRules is keyed by base trick name.
	PositionRules map[string]PositionRule `json:"position_rules"`
	// LegacyLastManeuver reproduces the inverted legacy last_maneuver test
	// (decision 0006); true only in golden tests.
	LegacyLastManeuver bool   `json:"legacy_last_maneuver"`
	Flight             Flight `json:"flight"`
}

// FinalMark is a flight's score. Numbers are full precision; Round3 them
// when persisting or displaying (spec 03 §9).
type FinalMark struct {
	JudgesMark      JudgeMark `json:"judges_mark"`
	Technicity      float64   `json:"technicity"`
	BonusPercentage float64   `json:"bonus_percentage"`
	Technical       float64   `json:"technical"`
	Choreography    float64   `json:"choreography"`
	Landing         float64   `json:"landing"`
	Synchro         float64   `json:"synchro"`
	Bonus           float64   `json:"bonus"`
	Score           float64   `json:"score"`
	Malus           float64   `json:"malus"`
	// Warnings1 holds the operator cat-1 warnings plus the automatic
	// position warnings; Warnings2 the operator cat-2 warnings.
	Warnings1 []string `json:"warnings1"`
	Warnings2 []string `json:"warnings2"`
	Notes     []string `json:"notes"`
	MarkType  MarkType `json:"mark_type"`
}

// Result is the outcome of Score: the final mark, and the flight's tricks
// and marks after the per-trick fill-in and AWT/AWQ computations (spec 03
// §5.0 steps 4-5), which the caller persists with the flight.
type Result struct {
	Final  FinalMark    `json:"final"`
	Tricks []FlownTrick `json:"tricks"`
	Marks  []JudgeMark  `json:"marks"`
}

// ValidationError is an invalid flight input (legacy HTTP 400).
type ValidationError struct {
	Msg string
}

func (e ValidationError) Error() string { return e.Msg }
