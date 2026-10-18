package api

import (
	"encoding/json"

	"awt/internal/domain"
	"awt/internal/store/gen"
)

// ---- JSON shapes ----

type pilotJSON struct {
	Civlid     int64           `json:"civlid"`
	Name       string          `json:"name"`
	Slug       string          `json:"slug"`
	Country    string          `json:"country"`
	CivlLink   string          `json:"civl_link"`
	About      string          `json:"about"`
	Photo      string          `json:"photo"`
	PhotoHigh  string          `json:"photo_highres"`
	Background string          `json:"background_picture"`
	Rank       int64           `json:"rank"`
	Links      json.RawMessage `json:"social_links"`
	Sponsors   json.RawMessage `json:"sponsors"`
	AwtYears   []int64         `json:"awt_years,omitempty"`
}

func newPilot(p gen.Pilot) *pilotJSON {
	return &pilotJSON{
		Civlid: p.Civlid, Name: p.Name, Slug: domain.Slug(p.Name), Country: p.Country, CivlLink: p.CivlLink,
		About: p.About, Photo: p.PhotoUrl, PhotoHigh: p.PhotoHighresUrl, Background: p.BackgroundUrl, Rank: p.Rank,
		Links: raw(p.SocialLinks), Sponsors: raw(p.Sponsors),
	}
}

type teamJSON struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Pilots []*pilotJSON `json:"pilots"`
}

type judgeJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Level   string `json:"level"`
	Civlid  *int64 `json:"civlid,omitempty"`
}

func newJudge(j gen.Judge) *judgeJSON {
	out := &judgeJSON{ID: j.ID, Name: j.Name, Country: j.Country, Level: j.Level}
	if j.Civlid.Valid {
		out.Civlid = &j.Civlid.Int64
	}
	return out
}

type competitionJSON struct {
	ID             string   `json:"id"`
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	State          string   `json:"state"`
	StartDate      string   `json:"start_date"`
	EndDate        string   `json:"end_date"`
	Location       string   `json:"location"`
	Website        string   `json:"website,omitempty"`
	Image          string   `json:"image,omitempty"`
	Logo           string   `json:"logo,omitempty"`
	Seasons        []string `json:"seasons"`
	Final          bool     `json:"final"`
	NumberOfPilots int64    `json:"number_of_pilots"`
	NumberOfTeams  int64    `json:"number_of_teams"`
	NumberOfJudges int64    `json:"number_of_judges"`
	NumberOfRuns   int64    `json:"number_of_runs"`
	LastUpdate     string   `json:"last_update,omitempty"`
}

type competitionDetailJSON struct {
	competitionJSON
	Pilots  []*pilotJSON           `json:"pilots"`
	Teams   []*teamJSON            `json:"teams"`
	Judges  []*judgeJSON           `json:"judges"`
	Runs    []runSummaryJSON       `json:"runs"`
	Results map[string][]entryJSON `json:"results"`
}

type runSummaryJSON struct {
	Index           int64  `json:"index"`
	State           string `json:"state"`
	Final           bool   `json:"final"`
	NumberOfFlights int64  `json:"number_of_flights"`
	LastUpdate      string `json:"last_update,omitempty"`
}

type runDetailJSON struct {
	runSummaryJSON
	Results map[string][]entryJSON `json:"results"`
	Flights []flightJSON           `json:"flights"`
}

type cellJSON struct {
	Run   int     `json:"run"`
	Rank  int     `json:"rank"`
	Score float64 `json:"score"`
}

type entryJSON struct {
	Rank         int64      `json:"rank"`
	Position     int64      `json:"position"`
	Pilot        *pilotJSON `json:"pilot,omitempty"`
	Team         *teamJSON  `json:"team,omitempty"`
	Score        float64    `json:"score"`
	ResultPerRun []cellJSON `json:"result_per_run,omitempty"`
}

type trickJSON struct {
	Name                 string             `json:"name"`
	Acronym              string             `json:"acronym"`
	BaseTrick            string             `json:"base_trick"`
	TechnicalCoefficient float64            `json:"technical_coefficient"`
	Bonus                float64            `json:"bonus"`
	BonusTypes           json.RawMessage    `json:"bonus_types"`
	Uniqueness           json.RawMessage    `json:"uniqueness"`
	Bonuses              json.RawMessage    `json:"bonuses"`
	Solo                 *bool              `json:"solo,omitempty"`
	SoloAwt              *bool              `json:"solo_awt,omitempty"`
	Synchro              *bool              `json:"synchro,omitempty"`
	Types                json.RawMessage    `json:"types"`
	TechnicalMark        *float64           `json:"technical_mark,omitempty"`
	TechnicalMarks       map[string]float64 `json:"technical_marks,omitempty"`
}

type markJSON struct {
	Judge             *judgeJSON `json:"judge"`
	Technical         *float64   `json:"technical,omitempty"`
	TechnicalPerTrick []*float64 `json:"technical_per_trick,omitempty"`
	Choreography      *float64   `json:"choreography,omitempty"`
	Landing           *float64   `json:"landing,omitempty"`
	Synchro           *float64   `json:"synchro,omitempty"`
}

type judgesMarkJSON struct {
	Technical    *float64 `json:"technical,omitempty"`
	Choreography *float64 `json:"choreography,omitempty"`
	Landing      *float64 `json:"landing,omitempty"`
	Synchro      *float64 `json:"synchro,omitempty"`
}

type finalMarkJSON struct {
	JudgesMark      judgesMarkJSON `json:"judges_mark"`
	Technicity      float64        `json:"technicity"`
	BonusPercentage float64        `json:"bonus_percentage"`
	Technical       float64        `json:"technical"`
	Choreography    float64        `json:"choreography"`
	Landing         float64        `json:"landing"`
	Synchro         float64        `json:"synchro"`
	Bonus           float64        `json:"bonus"`
	Score           float64        `json:"score"`
	Malus           float64        `json:"malus"`
	Warnings        []string       `json:"warnings"`
	Warnings2       []string       `json:"warnings2"`
	Notes           []string       `json:"notes"`
	MarkType        string         `json:"mark_type,omitempty"`
}

type flightJSON struct {
	Pilot       *pilotJSON     `json:"pilot,omitempty"`
	Team        *teamJSON      `json:"team,omitempty"`
	DidNotStart bool           `json:"did_not_start"`
	TipTouches  int64          `json:"tip_touches"`
	MarkType    string         `json:"mark_type,omitempty"`
	Score       *float64       `json:"score,omitempty"`
	Tricks      []trickJSON    `json:"tricks"`
	Marks       []markJSON     `json:"marks"`
	FinalMarks  *finalMarkJSON `json:"final_marks,omitempty"`
	Warnings    []string       `json:"warnings"`
	Warnings2   []string       `json:"warnings2"`
	LastUpdate  string         `json:"last_update,omitempty"`
}

type seasonJSON struct {
	ID                   string `json:"id"`
	Code                 string `json:"code"`
	Name                 string `json:"name"`
	Year                 int64  `json:"year"`
	Country              string `json:"country,omitempty"`
	Image                string `json:"image,omitempty"`
	Index                int64  `json:"index"`
	NumberOfCompetitions int64  `json:"number_of_competitions"`
}

type seasonDetailJSON struct {
	seasonJSON
	Competitions []competitionJSON      `json:"competitions"`
	Results      map[string][]entryJSON `json:"results"`
}

type catalogTrickJSON struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Acronym              string          `json:"acronym"`
	Solo                 bool            `json:"solo"`
	SoloAwt              bool            `json:"solo_awt"`
	Synchro              bool            `json:"synchro"`
	Directions           json.RawMessage `json:"directions"`
	TechnicalCoefficient float64         `json:"technical_coefficient"`
	Bonuses              json.RawMessage `json:"bonuses"`
	BonusConstraints     json.RawMessage `json:"bonus_constraints"`
	FirstManeuver        int64           `json:"first_maneuver"`
	NoFirstManeuver      int64           `json:"no_first_maneuver"`
	LastManeuver         int64           `json:"last_maneuver"`
	NoLastManeuver       int64           `json:"no_last_maneuver"`
	Repeatable           bool            `json:"repeatable"`
	Types                json.RawMessage `json:"types"`
	SampleVideo          string          `json:"sample_video,omitempty"`
}
