package export

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"awt/internal/domain"
	"awt/internal/export/templates"
	"awt/internal/scoring"

	"github.com/a-h/templ"
)

const timeLayout = "2006-01-02 15:04"

func f3(x float64) string { return fmt.Sprintf("%.3f", scoring.Round3(x)) }

func updated(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(timeLayout)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func listTitle(title, key string) string {
	if title != "" {
		return title
	}
	return capitalize(key)
}

func person(p ExportPilot, link bool) templates.Person {
	t := templates.Person{Name: p.Name, Country: strings.ToUpper(p.Country)}
	if link {
		t.Href = fmt.Sprintf("/pilots/%d/%s", p.CivlID, url.PathEscape(domain.Slug(p.Name)))
	}
	return t
}

func participant(pilot *ExportPilot, team *ExportTeam, link bool) templates.Participant {
	switch {
	case pilot != nil:
		return templates.Participant{Person: person(*pilot, link)}
	case team != nil:
		p := templates.Participant{Person: templates.Person{Name: team.Name}}
		for _, m := range team.Pilots {
			p.Members = append(p.Members, person(m, false))
		}
		return p
	}
	return templates.Participant{}
}

func render(ctx context.Context, w io.Writer, c templ.Component) error {
	return c.Render(ctx, w)
}

// ResultsHTML writes the printable competition results: one table per list.
func ResultsHTML(w io.Writer, data CompetitionExport) error {
	state := "intermediate"
	if data.Final {
		state = "final"
	}
	heading := state + " results"
	if data.LimitRun >= 0 {
		heading += " after run " + strconv.Itoa(data.LimitRun+1)
	}
	v := templates.ResultsView{Page: templates.Page{
		CompName: data.Name, Heading: heading, Updated: updated(data.UpdatedAt), Solo: data.Type == TypeSolo,
	}}
	for _, l := range data.Lists {
		t := templates.ResultsTable{Title: listTitle(l.Title, l.Key)}
		for i, e := range l.Entries {
			row := templates.ResultsRow{
				Rank: i + 1, P: participant(e.Pilot, e.Team, true), Total: f3(e.Score),
			}
			for j, c := range e.PerRun {
				row.Runs = append(row.Runs, templates.RunScore{Label: "Run " + strconv.Itoa(j+1), Score: f3(c.Score)})
			}
			t.Rows = append(t.Rows, row)
		}
		v.Tables = append(v.Tables, t)
	}
	return render(context.Background(), w, templates.ResultsPage(v))
}

// RunResultsHTML writes the printable results of one run, in the given order.
func RunResultsHTML(w io.Writer, data RunExport) error {
	state := "intermediate"
	if data.Final {
		state = "final"
	}
	v := templates.RunResultsView{Page: templates.Page{
		CompName: data.CompName,
		Heading:  fmt.Sprintf("run %d %s results", data.RunIndex+1, state),
		Updated:  updated(data.UpdatedAt),
		Solo:     data.Type == TypeSolo,
	}}
	for _, l := range data.Lists {
		t := templates.RunTable{Title: listTitle(l.Title, l.Key)}
		for i, e := range l.Entries {
			row := templates.RunRow{
				Rank: i + 1, P: participant(e.Pilot, e.Team, true),
				Technical: f3(e.Technical), Choreo: f3(e.Choreo), Landing: f3(e.Landing), Synchro: f3(e.Synchro),
				Technicity: f3(e.Technicity), Bonus: fmt.Sprintf("%.1f%%", e.BonusPct), Score: f3(e.Score),
			}
			for _, w := range e.Warnings2 {
				row.Warnings = append(row.Warnings, w+" (1pt)")
			}
			for _, w := range e.Warnings {
				row.Warnings = append(row.Warnings, w+" (0.5pt)")
			}
			if e.Malus > 0 {
				row.Malus = fmt.Sprintf("%d%%", int(e.Malus))
			}
			t.Rows = append(t.Rows, row)
		}
		v.Tables = append(v.Tables, t)
	}
	return render(context.Background(), w, templates.RunResultsPage(v))
}

// StartingOrderHTML writes the printable starting order of a run.
func StartingOrderHTML(w io.Writer, data StartingOrderExport) error {
	v := templates.StartingOrderView{Page: templates.Page{
		CompName: data.CompName,
		Heading:  fmt.Sprintf("Starting Order - run %d", data.RunIndex+1),
		Updated:  updated(data.UpdatedAt),
		Solo:     data.Type == TypeSolo,
	}}
	for _, l := range data.Lists {
		t := templates.StartingTable{Title: l.Title + " Starting Order"}
		for i, e := range l.Entries {
			t.Rows = append(t.Rows, templates.StartingRow{Position: i + 1, P: participant(e.Pilot, e.Team, false)})
		}
		v.Tables = append(v.Tables, t)
	}
	return render(context.Background(), w, templates.StartingOrderPage(v))
}
