// Package export renders the CIVL results-upload xlsx files and the printable
// HTML exports (spec 03 §11, spec 04 §7.1-7.2). The renderers are pure: they
// take the input structs below and write to an io.Writer. Loading the data
// and the HTTP/admin routes live elsewhere.
package export

import "time"

// Competition types as stored in the domain model.
const (
	TypeSolo    = "solo"
	TypeSynchro = "synchro"
)

// KeyOverall is the key of the list the xlsx exports are built from.
const KeyOverall = "overall"

// ExportPilot is a pilot as printed in an export. Gender is "man" or "woman"
// (domain values); anything but "man" is exported as F.
type ExportPilot struct {
	CivlID  int
	Name    string
	Country string
	Gender  string
	IsAWT   bool
}

// ExportTeam is a synchro team.
type ExportTeam struct {
	Name   string
	Pilots []ExportPilot
}

// ExportRunCell is one run's rank and score inside a competition result.
type ExportRunCell struct {
	Rank  int
	Score float64
}

// ExportEntry is one row of a competition result list. Solo entries set
// Pilot, synchro entries set Team. Lists are expected in ranking order (the
// row number is the displayed rank); the xlsx export re-sorts "overall".
type ExportEntry struct {
	Rank   int
	Score  float64
	Pilot  *ExportPilot
	Team   *ExportTeam
	PerRun []ExportRunCell
}

// ExportList is one result type (overall, women, a season, ...). Title is the
// heading printed above the table; when empty the capitalised Key is used.
type ExportList struct {
	Key     string
	Title   string
	Entries []ExportEntry
}

// CompetitionExport is the input of CompetitionXLSX and ResultsHTML.
// LimitRun is the zero-based last run included, or -1 when all runs are
// included (the heading then omits "after run N").
type CompetitionExport struct {
	Name      string
	Code      string
	Type      string
	Location  string
	StartDate time.Time
	EndDate   time.Time
	Seasons   []string
	Final     bool
	LimitRun  int
	Lists     []ExportList
	UpdatedAt time.Time
}

// RunEntry is one row of a run result list, with the final marks printed in
// run results.
type RunEntry struct {
	Score      float64
	Pilot      *ExportPilot
	Team       *ExportTeam
	Warnings   []string // 0.5 point each
	Warnings2  []string // 1 point each
	Malus      float64  // percent
	Technical  float64
	Choreo     float64
	Landing    float64
	Synchro    float64
	Technicity float64
	BonusPct   float64
}

// RunList is one result type of a run.
type RunList struct {
	Key     string
	Title   string
	Entries []RunEntry
}

// RunExport is the input of RunXLSX and RunResultsHTML. RunIndex is
// zero-based.
type RunExport struct {
	CompName  string
	CompCode  string
	Type      string
	Seasons   []string
	StartDate time.Time
	RunIndex  int
	Final     bool
	Lists     []RunList
	UpdatedAt time.Time
}

// StartingEntry is one starting position: Pilot for solo, Team for synchro.
type StartingEntry struct {
	Pilot *ExportPilot
	Team  *ExportTeam
}

// StartingList is one table of the starting order (a season or "overall").
type StartingList struct {
	Title   string
	Entries []StartingEntry
}

// StartingOrderExport is the input of StartingOrderHTML.
type StartingOrderExport struct {
	CompName  string
	Type      string
	RunIndex  int
	Lists     []StartingList
	UpdatedAt time.Time
}
