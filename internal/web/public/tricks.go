package public

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"awt/internal/catalog"
	"awt/internal/store/gen"
)

// trickRow is a catalog trick as the tricks table shows it (spec 06 §4.10).
type trickRow struct {
	gen.Trick
	Left, Right, Mirror bool
	Bonuses             []catalog.Bonus
}

func (t trickRow) Video() string {
	if t.SampleVideo.Valid {
		return t.SampleVideo.String
	}
	return ""
}

func (t trickRow) Coefficient() string {
	return strconv.FormatFloat(t.TechnicalCoefficient, 'f', 2, 64)
}

type tricksView struct {
	Years       []tab
	Year        int
	Solo        []trickRow
	SynchroOnly []trickRow
}

// tricks is the trick catalog of a year (?year=, default the current one or
// the latest earlier).
func (p *results) tricks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ys, err := p.q.PublicTrickYears(ctx)
	if err != nil {
		serverError(w, r, err)
		return
	}
	years := make([]int, len(ys))
	for i, y := range ys {
		years[i] = int(y)
	}
	sel := pickYear(r, years, p.now().Year())
	v := tricksView{Year: sel, Years: yearTabs(append(years, sel), sel, "/tricks")}
	rows, err := p.q.ListTricksByYear(ctx, int64(sel))
	if err != nil {
		serverError(w, r, err)
		return
	}
	slices.SortStableFunc(rows, func(a, b gen.Trick) int { return cmp.Compare(a.TechnicalCoefficient, b.TechnicalCoefficient) })
	for _, t := range rows {
		row := trickRow{Trick: t}
		var dirs []string
		if err := json.Unmarshal([]byte(t.Directions), &dirs); err != nil {
			serverError(w, r, fmt.Errorf("trick %s directions: %w", t.ID, err))
			return
		}
		row.Left, row.Right = slices.Contains(dirs, "left"), slices.Contains(dirs, "right")
		row.Mirror = slices.Contains(dirs, "opposite") || slices.Contains(dirs, "mirror")
		if err := json.Unmarshal([]byte(t.Bonuses), &row.Bonuses); err != nil {
			serverError(w, r, fmt.Errorf("trick %s bonuses: %w", t.ID, err))
			return
		}
		slices.SortStableFunc(row.Bonuses, func(a, b catalog.Bonus) int { return cmp.Compare(a.Bonus, b.Bonus) })
		switch {
		case t.Solo:
			v.Solo = append(v.Solo, row)
		case t.Synchro:
			v.SynchroOnly = append(v.SynchroOnly, row)
		}
	}
	renderPage(w, r, TricksPage(p.meta("Tricks "+strconv.Itoa(sel),
		"Paragliding aerobatics tricks of "+strconv.Itoa(sel)+": acronyms, technical coefficients, bonuses.", "/tricks", ""), v))
}
