package ranking

import "strconv"

// Participant is a pilot (solo, identified by CivlID) or a team (synchro,
// identified by TeamID). Name is the display name used as tie-break
// (decision 0007); Country, Gender and IsAWT (AWT status for the
// competition's start year, spec 03 §7.1) drive the sub-rankings.
type Participant struct {
	CivlID        int
	TeamID        string
	Name, Country string
	Gender        string
	IsAWT         bool
}

// key identifies a participant: CIVL id for pilots, team id for teams.
func (p Participant) key() string {
	if p.TeamID != "" {
		return "t:" + p.TeamID
	}
	return "p:" + strconv.Itoa(p.CivlID)
}

// less is the tie-break of decision 0007: display name, then id.
func (p Participant) less(o Participant) bool {
	if p.Name != o.Name {
		return p.Name < o.Name
	}
	if p.TeamID != o.TeamID {
		return p.TeamID < o.TeamID
	}
	return p.CivlID < o.CivlID
}

// FlightScore is one flight of a run: its participant, its stored score,
// whether it is published, and its save sequence (informational: flights
// are ranked in the order given). A run holds at most one flight per
// participant (guaranteed by the store); callers must not pass duplicates.
type FlightScore struct {
	Participant Participant
	Score       float64
	Published   bool
	SaveSeq     int
}

// RunInput is a run to rank. Type is "solo" or "synchro"; Seasons are the
// competition's season codes in order; Year is the competition's start
// year (the year Participant.IsAWT refers to); State is the run state;
// Participants is the run's starting list.
type RunInput struct {
	Type         string
	Seasons      []string
	Year         int
	State        string
	Flights      []FlightScore
	Participants []Participant
}

// Entry is one line of a ranking list. Position is the 1-based place in
// the list, Rank the displayed rank, shared by equal Round3 scores
// (decision 0007). PerRun is set on competition lists only.
type Entry struct {
	Participant    Participant
	Score          float64
	Position, Rank int
	PerRun         []RunCell
}

// RunCell is a participant's result in one run: its rank in the same list
// of that run (decision 0006 #3-4) and its flight score.
type RunCell struct {
	RunIndex, Rank int
	Score          float64
}

// Lists maps a list name to its ranking: "overall", season codes, "women".
type Lists map[string][]Entry

// RunResult is a run ranking (spec 03 §7).
type RunResult struct {
	Lists Lists
	Final bool
}

// CompetitionResult is a competition ranking (spec 03 §8) and the rankings
// of its runs.
type CompetitionResult struct {
	Lists Lists
	Runs  []RunResult
	Final bool
}

const (
	typeSynchro = "synchro"
	stateClosed = "closed"
	genderWoman = "woman"
	listOverall = "overall"
	listWomen   = "women"
	// minWomen is the number of women a women ranking needs (spec 03 §7.1, §10).
	minWomen = 3
)
