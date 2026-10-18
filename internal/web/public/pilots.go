package public

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"awt/internal/store/gen"
)

type pilotsView struct {
	AWTYear int
	AWT     []gen.Pilot
	Others  []gen.Pilot
}

// pilots lists the pilots in two disjoint sections (spec 06 §4.6, B9 fixed):
// the AWT pilots of the latest AWT year up to now, then everyone else.
func (p *results) pilots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pilots, err := p.q.ListPilots(ctx)
	if err != nil {
		serverError(w, r, err)
		return
	}
	years, err := p.q.PublicAllAwtYears(ctx)
	if err != nil {
		serverError(w, r, err)
		return
	}
	now := int64(p.now().Year())
	var v pilotsView
	for _, y := range years {
		if y.Year <= now && int(y.Year) > v.AWTYear {
			v.AWTYear = int(y.Year)
		}
	}
	awt := map[int64]bool{}
	for _, y := range years {
		if int(y.Year) == v.AWTYear {
			awt[y.Civlid] = true
		}
	}
	for _, pl := range pilots {
		if awt[pl.Civlid] {
			v.AWT = append(v.AWT, pl)
		} else {
			v.Others = append(v.Others, pl)
		}
	}
	renderPage(w, r, PilotsPage(p.meta("Pilots", "Acro World Tour and qualifier pilots: photos, countries and FAI ranks.", "/pilots", ""), v))
}

type link struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

type sponsor struct {
	Name string `json:"name"`
	Link string `json:"link"`
	Img  string `json:"img"`
}

// compResult is a competition in a pilot's or team's history.
type compResult struct {
	Code, Name, Start string
	Final             bool
	Rank              int64
	Score             float64
	HasOverall        bool
	Others            []listPlace
}

type listPlace struct {
	Label string
	Rank  int64
	Score float64
}

// seasonResult is a season standing in a pilot's or team's history.
type seasonResult struct {
	Code, Name, List string
	Year             int64
	Rank             int64
	Score            float64
}

type pilotView struct {
	P        gen.Pilot
	Years    []string
	Social   []link
	Sponsors []sponsor
	Comps    []compResult
	Seasons  []seasonResult
}

func (v pilotView) Photo() string {
	if v.P.PhotoHighresUrl != "" {
		return v.P.PhotoHighresUrl
	}
	return v.P.PhotoUrl
}

// pilot is a pilot's profile and results history (spec 06 §4.7). The slug
// is cosmetic: a missing or stale one redirects to the canonical URL.
func (p *results) pilot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("civlid"), 10, 64)
	if err != nil {
		notFound(w, r)
		return
	}
	ctx := r.Context()
	pl, err := p.q.GetPilot(ctx, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	href := pilotHref(pl.Civlid, pl.Name)
	if r.URL.Path != href {
		http.Redirect(w, r, href, http.StatusMovedPermanently)
		return
	}
	v := pilotView{P: pl}
	years, err := p.q.ListPilotAwtYears(ctx, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	for _, y := range years {
		v.Years = append(v.Years, strconv.FormatInt(y, 10))
	}
	// Malformed JSON in these display-only fields is ignored, not a 500.
	_ = json.Unmarshal([]byte(pl.SocialLinks), &v.Social)
	_ = json.Unmarshal([]byte(pl.Sponsors), &v.Sponsors)
	who := sql.NullInt64{Int64: id, Valid: true}
	if v.Comps, v.Seasons, err = p.history(r, who, sql.NullString{}); err != nil {
		serverError(w, r, err)
		return
	}
	desc := pl.Name
	if pl.Country != "" {
		desc += " (" + country(pl.Country) + ")"
	}
	desc += ": paragliding aerobatics pilot profile and Acro World Tour results."
	renderPage(w, r, PilotPage(p.meta(pl.Name, desc, href, v.Photo()), v))
}

// history reads a pilot's or a team's competition and season results:
// published, non-deleted competitions and non-deleted seasons only.
func (p *results) history(r *http.Request, civlid sql.NullInt64, teamID sql.NullString) ([]compResult, []seasonResult, error) {
	ctx := r.Context()
	rows, err := p.q.PublicPilotCompetitionResults(ctx, gen.PublicPilotCompetitionResultsParams{Civlid: civlid, TeamID: teamID})
	if err != nil {
		return nil, nil, err
	}
	var comps []compResult
	for _, row := range rows {
		if len(comps) == 0 || comps[len(comps)-1].Code != row.Code {
			comps = append(comps, compResult{Code: row.Code, Name: row.Name, Start: row.StartDate, Final: row.ResultsFinal})
		}
		c := &comps[len(comps)-1]
		if row.ListKey == "overall" {
			c.HasOverall, c.Rank, c.Score = true, row.Rank, row.Score
			continue
		}
		label := row.SeasonName
		if label == "" {
			label = row.ListKey
		}
		c.Others = append(c.Others, listPlace{Label: label, Rank: row.Rank, Score: row.Score})
	}
	srows, err := p.q.PublicPilotSeasonResults(ctx, gen.PublicPilotSeasonResultsParams{Civlid: civlid, TeamID: teamID})
	if err != nil {
		return nil, nil, err
	}
	seasons := make([]seasonResult, 0, len(srows))
	for _, s := range srows {
		seasons = append(seasons, seasonResult{Code: s.Code, Name: s.Name, List: listLabel(s.ListKey, nil), Year: s.Year, Rank: s.Rank, Score: s.Score})
	}
	return comps, seasons, nil
}

func joinYears(years []string) string { return strings.Join(years, ", ") }
