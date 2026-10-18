package importer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"awt/internal/ranking"
	"awt/internal/scoring"
	"awt/internal/store"
	"awt/internal/store/gen"
)

// refData is the reference data already imported: what competitions may link to.
type refData struct {
	pilots map[int64]bool
	teams  map[string]bool
	judges map[string]bool
	files  map[string]bool
}

type competitionRow struct {
	ID               ID              `json:"_id"`
	Name             string          `json:"name"`
	Code             string          `json:"code"`
	StartDate        string          `json:"start_date"`
	EndDate          string          `json:"end_date"`
	Location         string          `json:"location"`
	Published        bool            `json:"published"`
	Type             string          `json:"type"`
	Image            *string         `json:"image"`
	Logo             *string         `json:"logo"`
	Website          *string         `json:"website"`
	Seasons          []string        `json:"seasons"`
	Pilots           []Int           `json:"pilots"`
	Teams            []ID            `json:"teams"`
	Judges           []ID            `json:"judges"`
	RepeatableTricks []ID            `json:"repeatable_tricks"`
	State            string          `json:"state"`
	Config           json.RawMessage `json:"config"`
	Runs             []runRow        `json:"runs"`
	Deleted          Date            `json:"deleted"`
	LastUpdate       Date            `json:"last_update"`
}

type runRow struct {
	State                  string          `json:"state"`
	Pilots                 []Int           `json:"pilots"`
	Teams                  []ID            `json:"teams"`
	Judges                 []ID            `json:"judges"`
	Config                 json.RawMessage `json:"config"`
	Flights                []flightRow     `json:"flights"`
	RepetitionsResetPolicy Int             `json:"repetitions_reset_policy"`
	LastUpdate             Date            `json:"last_update"`
}

type flightRow struct {
	Pilot           Int             `json:"pilot"`
	Team            ID              `json:"team"`
	Tricks          []trickSnapshot `json:"tricks"`
	Marks           []markRow       `json:"marks"`
	DidNotStart     bool            `json:"did_not_start"`
	FinalMarks      *finalMarksRow  `json:"final_marks"`
	Published       bool            `json:"published"`
	Warnings        []string        `json:"warnings"`
	Warnings2       []string        `json:"warnings2"`
	TipTouchBonuses Int             `json:"tip_touch_bonuses"`
	LastUpdate      Date            `json:"last_update"`
}

// trickSnapshot is a unique trick as snapshotted in a flight. Older
// snapshots lack some keys: those stay NULL.
type trickSnapshot struct {
	Name                 string          `json:"name"`
	Acronym              string          `json:"acronym"`
	BaseTrick            string          `json:"base_trick"`
	TechnicalCoefficient float64         `json:"technical_coefficient"`
	Bonus                float64         `json:"bonus"`
	BonusTypes           json.RawMessage `json:"bonus_types"`
	Uniqueness           json.RawMessage `json:"uniqueness"`
	Bonuses              json.RawMessage `json:"bonuses"`
	Types                json.RawMessage `json:"types"`
	Solo                 *bool           `json:"solo"`
	SoloAWT              *bool           `json:"solo_awt"`
	Synchro              *bool           `json:"synchro"`
	TechnicalMark        *float64        `json:"technical_mark"`
	TechnicalMarks       json.RawMessage `json:"technical_marks"`
}

type markRow struct {
	Judge             ID              `json:"judge"`
	Technical         *float64        `json:"technical"`
	Choreography      *float64        `json:"choreography"`
	Landing           *float64        `json:"landing"`
	Synchro           *float64        `json:"synchro"`
	TechnicalPerTrick json.RawMessage `json:"technical_per_trick"`
}

// finalMarksRow is the stored final_marks; the legacy key "warnings" becomes
// warnings1 in scoring.FinalMark.
type finalMarksRow struct {
	JudgesMark      scoring.JudgeMark `json:"judges_mark"`
	Technicity      float64           `json:"technicity"`
	BonusPercentage float64           `json:"bonus_percentage"`
	Technical       float64           `json:"technical"`
	Choreography    float64           `json:"choreography"`
	Landing         float64           `json:"landing"`
	Synchro         float64           `json:"synchro"`
	Bonus           float64           `json:"bonus"`
	Score           float64           `json:"score"`
	Malus           float64           `json:"malus"`
	Warnings        []string          `json:"warnings"`
	Warnings2       []string          `json:"warnings2"`
	Notes           []string          `json:"notes"`
	MarkType        *string           `json:"mark_type"`
}

// finalMarkJSON normalises the stored final marks to the scoring.FinalMark
// JSON shape.
func finalMarkJSON(f finalMarksRow) (string, error) {
	fm := scoring.FinalMark{
		JudgesMark: f.JudgesMark, Technicity: f.Technicity, BonusPercentage: f.BonusPercentage,
		Technical: f.Technical, Choreography: f.Choreography, Landing: f.Landing, Synchro: f.Synchro,
		Bonus: f.Bonus, Score: f.Score, Malus: f.Malus,
		Warnings1: nonNil(f.Warnings), Warnings2: nonNil(f.Warnings2), Notes: nonNil(f.Notes),
	}
	if f.MarkType != nil {
		fm.MarkType = scoring.MarkType(*f.MarkType)
	}
	b, err := json.Marshal(fm)
	return string(b), err
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

type teamPilotsRow struct {
	ID     ID    `json:"_id"`
	Pilots []Int `json:"pilots"`
}

// createPlaceholderPilots inserts a placeholder pilot for every CIVL id
// referenced by the export but missing from pilots.json (hard-deleted in
// production), so that teams, competitions and flights keep their links.
func createPlaceholderPilots(ctx context.Context, q *gen.Queries, s *source, comps []competitionRow, pilots map[int64]bool, rep *Report) error {
	refs := map[int64]bool{}
	for _, c := range comps {
		for _, p := range c.Pilots {
			refs[p.V] = true
		}
		for _, r := range c.Runs {
			for _, p := range r.Pilots {
				refs[p.V] = true
			}
			for _, f := range r.Flights {
				if f.Pilot.V != 0 {
					refs[f.Pilot.V] = true
				}
			}
		}
	}
	teams, err := decodeLines[teamPilotsRow](s, "teams")
	if err != nil {
		return err
	}
	for _, t := range teams {
		if len(t.Pilots) == 2 { // other teams are skipped by importTeams
			for _, p := range t.Pilots {
				refs[p.V] = true
			}
		}
	}
	judges, err := decodeLines[judgeRow](s, "judges")
	if err != nil {
		return err
	}
	for _, j := range judges {
		if j.Civlid.Valid {
			refs[j.Civlid.V] = true
		}
	}
	var missing []int64
	for id := range refs {
		if !pilots[id] {
			missing = append(missing, id)
		}
	}
	slices.Sort(missing)
	for _, id := range missing {
		err := q.UpsertPilot(ctx, gen.UpsertPilotParams{
			Civlid: id, Name: fmt.Sprintf("Pilot #%d", id), Country: "", Rank: 9999, Gender: "none",
			SocialLinks: "[]", Sponsors: "[]",
		})
		if err != nil {
			return fmt.Errorf("create placeholder pilot %d: %w", id, err)
		}
		pilots[id] = true
		rep.Counts["placeholder_pilots"]++
	}
	if len(missing) > 0 {
		ids := make([]string, len(missing))
		for i, id := range missing {
			ids[i] = strconv.FormatInt(id, 10)
		}
		rep.warnf("%d referenced pilots are missing from pilots.json, placeholders created: %s", len(missing), strings.Join(ids, ", "))
	}
	return nil
}

// rankedComp is what the ranking step needs from an imported competition.
type rankedComp struct {
	id, code, name, typ, state string
	published                  bool
	deleted                    bool
	seasons                    []string
	runs                       []ranking.RunInput
	result                     ranking.CompetitionResult
}

// people resolves ranking participants from the database, with a cache.
type people struct {
	q      *gen.Queries
	pilots map[int64]gen.Pilot
	awt    map[int64][]int64
	teams  map[string]string
}

func (p *people) pilot(ctx context.Context, civlid int64, year int) (ranking.Participant, error) {
	row, ok := p.pilots[civlid]
	if !ok {
		var err error
		if row, err = p.q.GetPilot(ctx, civlid); err != nil {
			return ranking.Participant{}, fmt.Errorf("pilot %d: %w", civlid, err)
		}
		p.pilots[civlid] = row
		if p.awt[civlid], err = p.q.ListPilotAwtYears(ctx, civlid); err != nil {
			return ranking.Participant{}, fmt.Errorf("awt years of pilot %d: %w", civlid, err)
		}
	}
	return ranking.Participant{
		CivlID: int(civlid), Name: row.Name, Country: row.Country, Gender: row.Gender,
		IsAWT: slices.Contains(p.awt[civlid], int64(year)),
	}, nil
}

func (p *people) team(ctx context.Context, id string) (ranking.Participant, error) {
	name, ok := p.teams[id]
	if !ok {
		t, err := p.q.GetTeam(ctx, id)
		if err != nil {
			return ranking.Participant{}, fmt.Errorf("team %s: %w", id, err)
		}
		name = t.Name
		p.teams[id] = name
	}
	return ranking.Participant{TeamID: id, Name: name}, nil
}

func importCompetitions(ctx context.Context, q *gen.Queries, s *source, comps []competitionRow, ref refData, rep *Report) error {
	trickNames, err := trickNamesByID(s)
	if err != nil {
		return err
	}
	ppl := &people{q: q, pilots: map[int64]gen.Pilot{}, awt: map[int64][]int64{}, teams: map[string]string{}}
	var ranked []*rankedComp
	for i := range comps {
		rc, err := importCompetition(ctx, q, &comps[i], ref, trickNames, ppl, rep)
		if err != nil {
			return fmt.Errorf("import competition %s (%s): %w", comps[i].ID, comps[i].Code, err)
		}
		ranked = append(ranked, rc)
	}
	for _, rc := range ranked {
		if err := rankCompetition(ctx, q, rc); err != nil {
			return fmt.Errorf("rank competition %s (%s): %w", rc.id, rc.code, err)
		}
	}
	return rankSeasons(ctx, q, ranked, rep)
}

type trickNameRow struct {
	ID   ID     `json:"_id"`
	Name string `json:"name"`
}

// trickNamesByID maps every exported trick id (deleted ones included:
// competitions may still list them as repeatable) to its name.
func trickNamesByID(s *source) (map[string]string, error) {
	rows, err := decodeLines[trickNameRow](s, "tricks")
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(rows))
	for _, r := range rows {
		names[string(r.ID)] = r.Name
	}
	return names, nil
}

func importCompetition(ctx context.Context, q *gen.Queries, c *competitionRow, ref refData, trickNames map[string]string, ppl *people, rep *Report) (*rankedComp, error) {
	if c.Type != "solo" && c.Type != "synchro" {
		return nil, fmt.Errorf("unknown type %q", c.Type)
	}
	if !slices.Contains([]string{"init", "open", "closed"}, c.State) {
		return nil, fmt.Errorf("unknown state %q", c.State)
	}
	if len(c.StartDate) < 10 || len(c.EndDate) < 10 {
		return nil, fmt.Errorf("invalid dates %q - %q", c.StartDate, c.EndDate)
	}
	startDate, endDate := c.StartDate[:10], c.EndDate[:10]
	year, err := strconv.Atoi(startDate[:4])
	if err != nil {
		return nil, fmt.Errorf("invalid start date %q", c.StartDate)
	}
	compConfig, err := configWithDefaults(ctx, q, c.Config, year)
	if err != nil {
		return nil, err
	}
	id := string(c.ID)
	err = q.UpsertCompetition(ctx, gen.UpsertCompetitionParams{
		ID: id, Code: c.Code, Name: c.Name, Type: c.Type, State: c.State, StartDate: startDate, EndDate: endDate,
		Location: c.Location, Published: c.Published, Website: nullString(c.Website),
		ImageFile: c.file(ref, "image", c.Image, rep), LogoFile: c.file(ref, "logo", c.Logo, rep),
		Config: compConfig, Frozen: true, ResultsFinal: false, UpdatedAt: c.LastUpdate.null(), DeletedAt: c.Deleted.null(),
	})
	if err != nil {
		return nil, err
	}
	rep.Counts["competitions"]++
	if err := linkCompetition(ctx, q, c, ref, trickNames, rep); err != nil {
		return nil, err
	}

	rc := &rankedComp{
		id: id, code: c.Code, name: c.Name, typ: c.Type, state: c.State, published: c.Published,
		deleted: c.Deleted != "", seasons: c.Seasons,
	}
	for idx := range c.Runs {
		in, err := importRun(ctx, q, c, idx, year, compConfig, ref, ppl, rep)
		if err != nil {
			return nil, fmt.Errorf("run %d: %w", idx, err)
		}
		rc.runs = append(rc.runs, in)
	}
	return rc, nil
}

func (c *competitionRow) file(ref refData, what string, id *string, rep *Report) sql.NullString {
	if id == nil || *id == "" {
		return sql.NullString{}
	}
	if !ref.files[*id] {
		rep.warnf("competition %s (%s): %s file %s was not imported, dropped", c.ID, c.Code, what, *id)
		return sql.NullString{}
	}
	return sql.NullString{String: *id, Valid: true}
}

func nullString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// linkCompetition stores the ordered link tables of a competition. Links to
// rows that were not imported are dropped with a warning.
func linkCompetition(ctx context.Context, q *gen.Queries, c *competitionRow, ref refData, trickNames map[string]string, rep *Report) error {
	id := string(c.ID)
	pos := 0
	seenCodes := map[string]bool{}
	for _, code := range c.Seasons {
		if seenCodes[code] {
			continue
		}
		seenCodes[code] = true
		if err := q.AddCompetitionSeason(ctx, gen.AddCompetitionSeasonParams{CompetitionID: id, Position: int64(pos), SeasonCode: code}); err != nil {
			return fmt.Errorf("season link %s: %w", code, err)
		}
		pos++
	}
	seen := map[int64]bool{}
	pos = 0
	for _, p := range c.Pilots {
		if seen[p.V] {
			continue
		}
		seen[p.V] = true
		if err := q.AddCompetitionPilot(ctx, gen.AddCompetitionPilotParams{CompetitionID: id, Position: int64(pos), Civlid: p.V}); err != nil {
			return fmt.Errorf("pilot link %d: %w", p.V, err)
		}
		pos++
	}
	seenIDs := map[string]bool{}
	pos = 0
	for _, t := range c.Teams {
		if seenIDs[string(t)] {
			continue
		}
		seenIDs[string(t)] = true
		if !ref.teams[string(t)] {
			rep.warnf("competition %s (%s): team %s was not imported, link dropped", c.ID, c.Code, t)
			continue
		}
		if err := q.AddCompetitionTeam(ctx, gen.AddCompetitionTeamParams{CompetitionID: id, Position: int64(pos), TeamID: string(t)}); err != nil {
			return fmt.Errorf("team link %s: %w", t, err)
		}
		pos++
	}
	seenIDs = map[string]bool{}
	pos = 0
	for _, j := range c.Judges {
		if seenIDs[string(j)] {
			continue
		}
		seenIDs[string(j)] = true
		if !ref.judges[string(j)] {
			rep.warnf("competition %s (%s): judge %s was not imported, link dropped", c.ID, c.Code, j)
			continue
		}
		if err := q.AddCompetitionJudge(ctx, gen.AddCompetitionJudgeParams{CompetitionID: id, Position: int64(pos), JudgeID: string(j)}); err != nil {
			return fmt.Errorf("judge link %s: %w", j, err)
		}
		pos++
	}
	pos = 0
	for _, t := range c.RepeatableTricks {
		name, ok := trickNames[string(t)]
		if !ok {
			name = string(t) // the field may hold names (spec 02 §2.5.2)
			if len(name) == 24 {
				rep.warnf("competition %s (%s): repeatable trick %s is unknown, dropped", c.ID, c.Code, t)
				continue
			}
		}
		err := q.AddCompetitionRepeatableTrick(ctx, gen.AddCompetitionRepeatableTrickParams{CompetitionID: id, Position: int64(pos), TrickName: name})
		if err != nil {
			return fmt.Errorf("repeatable trick link %s: %w", t, err)
		}
		pos++
	}
	return nil
}

func importRun(ctx context.Context, q *gen.Queries, c *competitionRow, idx, year int, compConfig string, ref refData, ppl *people, rep *Report) (ranking.RunInput, error) {
	r := c.Runs[idx]
	id := string(c.ID)
	in := ranking.RunInput{Type: c.Type, Seasons: c.Seasons, Year: year, State: r.State}
	if !slices.Contains([]string{"init", "open", "closed"}, r.State) {
		return in, fmt.Errorf("unknown state %q", r.State)
	}
	cfg := compConfig
	if !isNull(r.Config) {
		var err error
		if cfg, err = configWithDefaults(ctx, q, r.Config, year); err != nil {
			return in, err
		}
	}
	policy := r.RepetitionsResetPolicy.V // absent = 0
	if policy < 0 || policy > 3 {
		return in, fmt.Errorf("invalid repetitions_reset_policy %d", policy)
	}
	err := q.UpsertRun(ctx, gen.UpsertRunParams{
		CompetitionID: id, Idx: int64(idx), State: r.State, Config: cfg, ResetPolicy: policy, UpdatedAt: r.LastUpdate.null(),
	})
	if err != nil {
		return in, err
	}
	rep.Counts["runs"]++

	pos := 0
	for _, p := range r.Pilots {
		err := q.AddRunParticipant(ctx, gen.AddRunParticipantParams{
			CompetitionID: id, RunIdx: int64(idx), Position: int64(pos), Civlid: sql.NullInt64{Int64: p.V, Valid: true},
		})
		if err != nil {
			return in, fmt.Errorf("participant %d: %w", p.V, err)
		}
		part, err := ppl.pilot(ctx, p.V, year)
		if err != nil {
			return in, err
		}
		in.Participants = append(in.Participants, part)
		pos++
	}
	for _, t := range r.Teams {
		if !ref.teams[string(t)] {
			rep.warnf("competition %s (%s) run %d: team %s was not imported, participant dropped", c.ID, c.Code, idx, t)
			continue
		}
		err := q.AddRunParticipant(ctx, gen.AddRunParticipantParams{
			CompetitionID: id, RunIdx: int64(idx), Position: int64(pos), TeamID: sql.NullString{String: string(t), Valid: true},
		})
		if err != nil {
			return in, fmt.Errorf("participant %s: %w", t, err)
		}
		part, err := ppl.team(ctx, string(t))
		if err != nil {
			return in, err
		}
		in.Participants = append(in.Participants, part)
		pos++
	}
	pos = 0
	seen := map[string]bool{}
	for _, j := range r.Judges {
		if seen[string(j)] {
			continue
		}
		seen[string(j)] = true
		if !ref.judges[string(j)] {
			rep.warnf("competition %s (%s) run %d: judge %s was not imported, link dropped", c.ID, c.Code, idx, j)
			continue
		}
		if err := q.AddRunJudge(ctx, gen.AddRunJudgeParams{CompetitionID: id, RunIdx: int64(idx), Position: int64(pos), JudgeID: string(j)}); err != nil {
			return in, fmt.Errorf("judge %s: %w", j, err)
		}
		pos++
	}

	seq := 0
	for fi := range r.Flights {
		f := &r.Flights[fi]
		fs, ok, err := importFlight(ctx, q, c, idx, year, seq+1, f, ref, ppl, rep)
		if err != nil {
			return in, fmt.Errorf("flight %d: %w", fi, err)
		}
		if ok {
			seq++
			in.Flights = append(in.Flights, fs)
		}
	}
	return in, nil
}

func isNull(r json.RawMessage) bool {
	t := bytes.TrimSpace(r)
	return len(t) == 0 || string(t) == "null"
}

// importFlight stores a flight with its snapshot, marks, warnings and stored
// final marks (frozen history, decision 0003). It reports false when the
// flight was skipped.
func importFlight(ctx context.Context, q *gen.Queries, c *competitionRow, idx, year, seq int, f *flightRow, ref refData, ppl *people, rep *Report) (ranking.FlightScore, bool, error) {
	p := gen.InsertFlightParams{
		CompetitionID: string(c.ID), RunIdx: int64(idx), DidNotStart: f.DidNotStart, TipTouches: f.TipTouchBonuses.V,
		Published: f.Published, SaveSeq: int64(seq), UpdatedAt: f.LastUpdate.null(),
	}
	var part ranking.Participant
	var err error
	if f.Pilot.V != 0 {
		p.Civlid = sql.NullInt64{Int64: f.Pilot.V, Valid: true}
		part, err = ppl.pilot(ctx, f.Pilot.V, year)
	} else {
		if !ref.teams[string(f.Team)] {
			rep.warnf("competition %s (%s) run %d: flight of team %q skipped, team was not imported", c.ID, c.Code, idx, f.Team)
			return ranking.FlightScore{}, false, nil
		}
		p.TeamID = sql.NullString{String: string(f.Team), Valid: true}
		part, err = ppl.team(ctx, string(f.Team))
	}
	if err != nil {
		return ranking.FlightScore{}, false, err
	}
	score := 0.0
	if f.FinalMarks != nil {
		fm, err := finalMarkJSON(*f.FinalMarks)
		if err != nil {
			return ranking.FlightScore{}, false, fmt.Errorf("encode final marks: %w", err)
		}
		score = scoring.Round3(f.FinalMarks.Score)
		p.FinalMark = sql.NullString{String: fm, Valid: true}
		p.Score = sql.NullFloat64{Float64: score, Valid: true}
		switch mt := f.FinalMarks.MarkType; {
		case mt == nil || *mt == "":
		case *mt == "awt" || *mt == "awq":
			p.MarkType = sql.NullString{String: *mt, Valid: true}
		default:
			rep.warnf("competition %s (%s) run %d: flight mark_type %q unknown, dropped", c.ID, c.Code, idx, *mt)
		}
	}
	fid, err := q.InsertFlight(ctx, p)
	if err != nil {
		return ranking.FlightScore{}, false, err
	}
	rep.Counts["flights"]++

	for i, t := range f.Tricks {
		err := q.InsertFlightTrick(ctx, gen.InsertFlightTrickParams{
			FlightID: fid, Position: int64(i), Name: t.Name, Acronym: t.Acronym, BaseTrick: t.BaseTrick,
			TechnicalCoefficient: t.TechnicalCoefficient, Bonus: t.Bonus, BonusTypes: rawJSONArray(t.BonusTypes),
			Uniqueness: rawJSONArray(t.Uniqueness), Bonuses: rawJSONArray(t.Bonuses), Types: rawJSONArray(t.Types),
			Solo: nullBool(t.Solo), SoloAwt: nullBool(t.SoloAWT), Synchro: nullBool(t.Synchro),
			TechnicalMark: nullFloat(t.TechnicalMark), TechnicalMarks: rawJSONOrNull(t.TechnicalMarks),
		})
		if err != nil {
			return ranking.FlightScore{}, false, fmt.Errorf("trick %d (%s): %w", i, t.Acronym, err)
		}
		rep.Counts["flight_tricks"]++
	}
	for _, m := range f.Marks {
		err := q.InsertFlightMark(ctx, gen.InsertFlightMarkParams{
			FlightID: fid, JudgeID: string(m.Judge), Technical: nullFloat(m.Technical), Choreography: nullFloat(m.Choreography),
			Landing: nullFloat(m.Landing), Synchro: nullFloat(m.Synchro), TechnicalPerTrick: rawJSONOrNull(m.TechnicalPerTrick),
		})
		if err != nil {
			return ranking.FlightScore{}, false, fmt.Errorf("mark of judge %s: %w", m.Judge, err)
		}
		rep.Counts["flight_marks"]++
	}
	for cat, texts := range [][]string{f.Warnings, f.Warnings2} {
		for i, text := range texts {
			err := q.InsertFlightWarning(ctx, gen.InsertFlightWarningParams{FlightID: fid, Category: int64(cat + 1), Position: int64(i), Text: text})
			if err != nil {
				return ranking.FlightScore{}, false, fmt.Errorf("warning %d.%d: %w", cat+1, i, err)
			}
			rep.Counts["flight_warnings"]++
		}
	}
	return ranking.FlightScore{Participant: part, Score: score, Published: f.Published, SaveSeq: seq}, true, nil
}

func nullBool(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

func nullFloat(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

// rawJSONOrNull returns the raw JSON text, NULL for null/absent.
func rawJSONOrNull(r json.RawMessage) sql.NullString {
	if isNull(r) {
		return sql.NullString{}
	}
	return sql.NullString{String: string(bytes.TrimSpace(r)), Valid: true}
}

// configWithDefaults completes a stored config with the keys it lacks (e.g.
// warning2, absent from older competitions) from the ruleset of the
// competition's year, as the legacy model defaults did on load.
func configWithDefaults(ctx context.Context, q *gen.Queries, raw json.RawMessage, year int) (string, error) {
	cfg := map[string]any{}
	if !isNull(raw) {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&cfg); err != nil {
			return "", fmt.Errorf("decode config: %w", err)
		}
	}
	rs, err := q.GetRuleset(ctx, int64(year))
	switch {
	case err == nil:
		def := map[string]any{}
		dec := json.NewDecoder(strings.NewReader(rs.Config))
		dec.UseNumber()
		if err := dec.Decode(&def); err != nil {
			return "", fmt.Errorf("decode ruleset %d: %w", year, err)
		}
		mergeMissing(cfg, def)
	case !errors.Is(err, sql.ErrNoRows):
		return "", fmt.Errorf("ruleset %d: %w", year, err)
	}
	b, err := json.Marshal(cfg)
	return string(b), err
}

// mergeMissing copies into dst the keys of def that dst lacks, recursively.
func mergeMissing(dst, def map[string]any) {
	for k, dv := range def {
		sv, ok := dst[k]
		if !ok {
			dst[k] = dv
			continue
		}
		sm, ok1 := sv.(map[string]any)
		dm, ok2 := dv.(map[string]any)
		if ok1 && ok2 {
			mergeMissing(sm, dm)
		}
	}
}

// rankCompetition rebuilds the rankings of a competition from its stored
// flight scores and persists them (runs and competition), then sets the
// results_final flags.
func rankCompetition(ctx context.Context, q *gen.Queries, rc *rankedComp) error {
	rc.result = ranking.Competition(rc.state, rc.runs, -1)
	for i, rr := range rc.result.Runs {
		if err := store.SaveRankings(ctx, q, store.ScopeRun, store.RunScopeKey(rc.id, i), rr.Lists); err != nil {
			return err
		}
		err := q.SetRunResultsFinal(ctx, gen.SetRunResultsFinalParams{ResultsFinal: rr.Final, CompetitionID: rc.id, Idx: int64(i)})
		if err != nil {
			return err
		}
	}
	if err := store.SaveRankings(ctx, q, store.ScopeCompetition, rc.id, rc.result.Lists); err != nil {
		return err
	}
	return q.SetCompetitionResultsFinal(ctx, gen.SetCompetitionResultsFinalParams{ResultsFinal: rc.result.Final, ID: rc.id})
}

// rankSeasons ranks every season code (active seasons and codes referenced by
// competitions) from the competitions ranked above.
func rankSeasons(ctx context.Context, q *gen.Queries, ranked []*rankedComp, rep *Report) error {
	seasons, err := q.ListSeasons(ctx)
	if err != nil {
		return err
	}
	country := map[string]string{}
	codes := map[string]bool{}
	for _, se := range seasons {
		codes[se.Code] = true
		country[se.Code] = se.Country.String
	}
	for _, rc := range ranked {
		for _, code := range rc.seasons {
			codes[code] = true
		}
	}
	byName := make([]*rankedComp, 0, len(ranked))
	for _, rc := range ranked {
		if !rc.deleted {
			byName = append(byName, rc)
		}
	}
	sort.SliceStable(byName, func(i, j int) bool { return byName[i].name < byName[j].name })

	sorted := make([]string, 0, len(codes))
	for code := range codes {
		sorted = append(sorted, code)
	}
	slices.Sort(sorted)
	for _, code := range sorted {
		in := ranking.SeasonInput{Code: code, Country: country[code], Type: "solo"}
		for _, rc := range byName {
			if !slices.Contains(rc.seasons, code) {
				continue
			}
			if len(in.Competitions) == 0 {
				in.Type = rc.typ
			} else if in.Type != rc.typ {
				rep.warnf("season %s: competitions of different types (%s, %s)", code, in.Type, rc.typ)
			}
			in.Competitions = append(in.Competitions, ranking.SeasonComp{
				Code: rc.code, Published: rc.published, Closed: rc.state == "closed", Result: rc.result,
			})
		}
		lists := ranking.Season(in)
		if err := store.SaveRankings(ctx, q, store.ScopeSeason, code, lists); err != nil {
			return fmt.Errorf("season %s: %w", code, err)
		}
		if len(lists) > 0 {
			rep.Counts["season_rankings"]++
		}
	}
	return nil
}
