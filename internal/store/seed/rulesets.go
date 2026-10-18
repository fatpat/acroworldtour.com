// Package seed holds the default data the database is initialised with.
// Default scoring values live here and in the rulesets table only (CLAUDE.md:
// rules are data).
package seed

import "encoding/json"

// Ruleset is one row of the rulesets table. Config and MaxTricksPerType are
// JSON documents (spec 02 §2.6 shape).
type Ruleset struct {
	Year                int
	Config              string
	MaxTricksPerType    string
	TipTouchPctPerTouch float64
	TipTouchPctMax      float64
	DefaultMaxBonus     float64
}

type judgeWeights struct {
	Senior    int `json:"senior"`
	Certified int `json:"certified"`
	Trainee   int `json:"trainee"`
}

type soloPercentages struct {
	Technical    int `json:"technical"`
	Choreography int `json:"choreography"`
	Landing      int `json:"landing"`
}

type synchroPercentages struct {
	Technical    int `json:"technical"`
	Choreography int `json:"choreography"`
	Landing      int `json:"landing"`
	Synchro      int `json:"synchro"`
}

type markPercentages struct {
	Solo    soloPercentages    `json:"solo"`
	Synchro synchroPercentages `json:"synchro"`
}

type maxBonusPerRun struct {
	Twist   int `json:"twist"`
	Reverse int `json:"reverse"`
	Flip    int `json:"flip"`
}

type config struct {
	Warning         float64         `json:"warning"`
	Warning2        float64         `json:"warning2"`
	MalusRepetition float64         `json:"malus_repetition"`
	WarningsToDSQ   int             `json:"warnings_to_dsq"`
	JudgeWeights    judgeWeights    `json:"judge_weights"`
	MarkPercentages markPercentages `json:"mark_percentages"`
	MaxBonusPerRun  maxBonusPerRun  `json:"max_bonus_per_run"`
}

// Rulesets returns the default ruleset of every season from 2022 to 2026,
// with the spec 03 §2 defaults.
func Rulesets() []Ruleset {
	cfg, err := json.Marshal(config{
		Warning:         0.5,
		Warning2:        1,
		MalusRepetition: 13,
		WarningsToDSQ:   3,
		JudgeWeights:    judgeWeights{Senior: 100, Certified: 100, Trainee: 20},
		MarkPercentages: markPercentages{
			Solo:    soloPercentages{Technical: 40, Choreography: 40, Landing: 20},
			Synchro: synchroPercentages{Technical: 20, Choreography: 20, Landing: 20, Synchro: 40},
		},
		MaxBonusPerRun: maxBonusPerRun{Twist: 5, Reverse: 3, Flip: 2},
	})
	if err != nil {
		panic(err) // impossible: static struct
	}
	var out []Ruleset
	for year := 2022; year <= 2026; year++ {
		out = append(out, Ruleset{
			Year:                year,
			Config:              string(cfg),
			MaxTricksPerType:    `{"tumbling":2,"to infinite":1}`,
			TipTouchPctPerTouch: 2,
			TipTouchPctMax:      6,
			DefaultMaxBonus:     10,
		})
	}
	return out
}
