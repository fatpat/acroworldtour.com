package public

import (
	"net/http"
	"slices"
	"strings"
)

// latestClosed is how many recently closed competitions the results home
// lists.
const latestClosed = 6

// sportingCodeURL is the FAI Sporting Code Section 7B linked from the
// results home (spec 06 §2.3, SPORTING_CODE_MANUAL_URL).
const sportingCodeURL = "https://www.fai.org/sites/default/files/civl/documents/sporting_code_s7_b_-_aerobatics_2025.pdf"

type homeView struct {
	Live   []compCard
	Closed []compCard
}

// home is the results home (spec 06 §4.1): about, current competitions
// (state open), the latest closed ones and downloads.
func (p *results) home(w http.ResponseWriter, r *http.Request) {
	comps, err := p.q.PublicListCompetitions(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}
	var v homeView
	var closed []compCard
	for _, c := range comps {
		switch c.State {
		case "open":
			v.Live = append(v.Live, cardFromList(c))
		case "closed":
			closed = append(closed, cardFromList(c))
		}
	}
	slices.SortStableFunc(closed, func(a, b compCard) int { return strings.Compare(b.End, a.End) })
	v.Closed = closed[:min(len(closed), latestClosed)]
	renderPage(w, r, HomePage(p.meta("Results",
		"Official results of the Acro World Tour: live competitions, standings, pilots and judges.", "/results", ""), v))
}
