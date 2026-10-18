// Package ranking builds run, competition and season rankings (spec 03
// §7, §8, §10).
package ranking

// The types below decode the ranking golden corpus
// (testdata/golden/rankings, format in tools/golden/README.md): inputs and
// outputs of the legacy Competition.results()/export() and
// Season.export_public, reduced to ids and numbers.

// GoldenCompetition is a competition-<code>.json file.
type GoldenCompetition struct {
	Scope    string                    `json:"scope"` // "competition"
	Code     string                    `json:"code"`
	Input    GoldenCompetitionInput    `json:"input"`
	Expected GoldenCompetitionExpected `json:"expected"`
}

// GoldenCompetitionInput is what the legacy ranking read from the
// competition (flight scores as reloaded from MongoDB, i.e. Round3).
type GoldenCompetitionInput struct {
	Type      string   `json:"type"`  // "solo" or "synchro"
	State     string   `json:"state"` // competition state
	Published bool     `json:"published"`
	Year      int      `json:"year"` // start date year (AWT status year)
	Seasons   []string `json:"seasons"`
	// Participants are the pilots (solo) or teams (synchro) of the
	// competition and of its flights.
	Participants []GoldenParticipant `json:"participants"`
	Runs         []GoldenRun         `json:"runs"`
}

// GoldenParticipant is a pilot (solo: CIVL id, country, gender, AWT for the
// competition year) or a team (synchro: team id and its pilots' CIVL ids).
// NameOrder is the position in the participants sorted by display name,
// standing in for the name tie-break of CompetitionResults.export.
type GoldenParticipant struct {
	ID        string   `json:"id"`
	Country   string   `json:"country,omitempty"`
	Gender    string   `json:"gender,omitempty"`
	AWT       bool     `json:"awt,omitempty"`
	Pilots    []string `json:"pilots,omitempty"`
	NameOrder int      `json:"name_order"`
}

// GoldenRun is a run's state and its flights in stored (first save) order.
type GoldenRun struct {
	State   string         `json:"state"`
	Flights []GoldenFlight `json:"flights"`
}

// GoldenFlight is a flight's participant, stored score and published flag.
type GoldenFlight struct {
	Participant string  `json:"participant"`
	Score       float64 `json:"score"`
	Published   bool    `json:"published"`
}

// GoldenCompetitionExpected is the legacy output.
type GoldenCompetitionExpected struct {
	Final bool `json:"final"`
	// Results is CompetitionResults.export(): every list ("overall",
	// season codes, "women") re-sorted by (-score, name).
	Results map[string][]GoldenEntry `json:"results"`
	// InternalResults is the participant order of CompetitionResults
	// before export (descending score, ties in reverse insertion order).
	InternalResults map[string][]string `json:"internal_results"`
	// RunsResults are the per-run rankings (unpublished flights included).
	RunsResults []GoldenRunResult `json:"runs_results"`
}

// GoldenEntry is one competition ranking line.
type GoldenEntry struct {
	Participant  string             `json:"participant"`
	Score        float64            `json:"score"`
	ResultPerRun []GoldenRunSummary `json:"result_per_run"`
}

// GoldenRunSummary is a RunResultSummary (rank and score in one run).
type GoldenRunSummary struct {
	Rank  int     `json:"rank"`
	Score float64 `json:"score"`
}

// GoldenRunResult is one run's rankings.
type GoldenRunResult struct {
	Final   bool                          `json:"final"`
	Results map[string][]GoldenScoreEntry `json:"results"`
}

// GoldenScoreEntry is a participant and a score.
type GoldenScoreEntry struct {
	Participant string  `json:"participant"`
	Score       float64 `json:"score"`
}

// GoldenSeason is a season-<code>.json file.
type GoldenSeason struct {
	Scope string            `json:"scope"` // "season"
	Code  string            `json:"code"`
	Input GoldenSeasonInput `json:"input"`
	// Expected is nil when the legacy export failed (Error set).
	Expected *GoldenSeasonExpected `json:"expected"`
	Error    string                `json:"error"`
}

// GoldenSeasonInput lists the season's competitions in legacy order (by
// competition name); their data is in the competition files.
type GoldenSeasonInput struct {
	Code         string   `json:"code"`
	Country      *string  `json:"country"`
	Competitions []string `json:"competitions"`
}

// GoldenSeasonExpected is the legacy Season.export_public, reduced.
type GoldenSeasonExpected struct {
	Type           string `json:"type"`
	NumberOfPilots int    `json:"number_of_pilots"`
	NumberOfTeams  int    `json:"number_of_teams"`
	// Results is keyed by "overall" and, with at least 3 women, "women".
	// Scores are plain float sums (not rounded).
	Results map[string][]GoldenScoreEntry `json:"results"`
	// CompetitionsResults is each counted competition's contribution.
	CompetitionsResults map[string][]GoldenScoreEntry `json:"competitions_results"`
}
