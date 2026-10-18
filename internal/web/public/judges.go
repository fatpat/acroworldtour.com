package public

import (
	"net/http"

	"awt/internal/store/gen"
)

type judgeGroup struct {
	Title  string
	Judges []gen.PublicListJudgesRow
}

// judges lists the judges by level (spec 06 §4.9). The photo comes from the
// linked pilot in the same query (legacy fetched one pilot per judge).
func (p *results) judges(w http.ResponseWriter, r *http.Request) {
	rows, err := p.q.PublicListJudges(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}
	groups := []judgeGroup{{Title: "Senior judges"}, {Title: "Certified judges"}, {Title: "Trainee judges"}}
	for _, j := range rows {
		switch j.Level {
		case "senior":
			groups[0].Judges = append(groups[0].Judges, j)
		case "certified":
			groups[1].Judges = append(groups[1].Judges, j)
		case "trainee":
			groups[2].Judges = append(groups[2].Judges, j)
		}
	}
	renderPage(w, r, JudgesPage(p.meta("Judges", "Acro World Tour judges by level: senior, certified and trainee.", "/judges", ""), groups))
}
