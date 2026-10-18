package export

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ErrRunNotFinal is returned by RunXLSX when the run is not final.
var ErrRunNotFinal = errors.New("run results are not final")

var tourSeason = regexp.MustCompile(`^aw[tqs]`)

const (
	pointsStart    = 100 // CIVL ranking points of the first row
	pointsFirstAWQ = 80  // first AWQ pilot of an aw[tqs] solo comp restarts here
)

type xlsxRow struct {
	score float64 // sort key
	pilot *ExportPilot
	team  *ExportTeam
}

// CompetitionXLSX writes the CIVL results-upload workbook of the "overall"
// list (spec 03 §11, spec 04 §7.2).
func CompetitionXLSX(w io.Writer, data CompetitionExport) error {
	var rows []xlsxRow
	for _, l := range data.Lists {
		if l.Key != KeyOverall {
			continue
		}
		for _, e := range l.Entries {
			rows = append(rows, xlsxRow{score: e.Score, pilot: e.Pilot, team: e.Team})
		}
	}
	return writeXLSX(w, "overall "+data.Type, data.Type, data.Seasons, rows)
}

// RunXLSX writes the CIVL workbook of a final run; it returns ErrRunNotFinal
// otherwise.
func RunXLSX(w io.Writer, data RunExport) error {
	if !data.Final {
		return ErrRunNotFinal
	}
	var rows []xlsxRow
	for _, l := range data.Lists {
		if l.Key != KeyOverall {
			continue
		}
		for _, e := range l.Entries {
			rows = append(rows, xlsxRow{score: e.Score, pilot: e.Pilot, team: e.Team})
		}
	}
	return writeXLSX(w, "run "+data.Type, data.Type, data.Seasons, rows)
}

func (r xlsxRow) name() string {
	switch {
	case r.pilot != nil:
		return r.pilot.Name
	case r.team != nil:
		return r.team.Name
	}
	return ""
}

func sortRows(rows []xlsxRow) {
	slices.SortStableFunc(rows, func(a, b xlsxRow) int {
		switch {
		case a.score > b.score:
			return -1
		case a.score < b.score:
			return 1
		}
		return strings.Compare(a.name(), b.name())
	})
}

func hasTourSeason(seasons []string) bool {
	return slices.ContainsFunc(seasons, tourSeason.MatchString)
}

func splitName(name string) (first, last string) {
	first, last, _ = strings.Cut(name, " ")
	return first, last
}

func gender(g string) string {
	if g == "man" {
		return "M"
	}
	return "F"
}

func writeXLSX(w io.Writer, sheet, typ string, seasons []string, rows []xlsxRow) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return err
	}

	tour := typ == TypeSolo && hasTourSeason(seasons)
	rows = slices.Clone(rows)
	if tour {
		var awt, awq []xlsxRow
		for _, r := range rows {
			if r.pilot != nil && r.pilot.IsAWT {
				awt = append(awt, r)
			} else {
				awq = append(awq, r)
			}
		}
		sortRows(awt)
		sortRows(awq)
		rows = append(awt, awq...)
	} else {
		sortRows(rows)
	}

	header := []any{"Rank", "Number", "First Name", "Last Name", "Nat", "Gender", "Glider", "Sponsor", "CIVL ID", "Score"}
	if typ == TypeSynchro {
		header = []any{"Rank", "Team", "Number", "First Name", "Last Name", "Nat", "Gender", "Glider", "Sponsor", "CIVL ID", "Score"}
	}
	if err := f.SetSheetRow(sheet, "A1", &header); err != nil {
		return err
	}

	line := 1
	score := pointsStart
	firstAWQ := false
	put := func(vals []any) error {
		line++
		cell, err := excelize.CoordinatesToCellName(1, line)
		if err != nil {
			return err
		}
		return f.SetSheetRow(sheet, cell, &vals)
	}
	for i, r := range rows {
		rank := i + 1
		if tour && !firstAWQ && (r.pilot == nil || !r.pilot.IsAWT) {
			score = pointsFirstAWQ
			firstAWQ = true
		}
		switch typ {
		case TypeSolo:
			if r.pilot == nil {
				return fmt.Errorf("export: solo entry %d has no pilot", rank)
			}
			p := r.pilot
			first, last := splitName(p.Name)
			if err := put([]any{rank, p.CivlID, first, last, strings.ToUpper(p.Country), gender(p.Gender), "", "", p.CivlID, score}); err != nil {
				return err
			}
		case TypeSynchro:
			if r.team == nil {
				return fmt.Errorf("export: synchro entry %d has no team", rank)
			}
			for _, p := range r.team.Pilots {
				first, last := splitName(p.Name)
				if err := put([]any{rank, r.team.Name, p.CivlID, first, last, strings.ToUpper(p.Country), "M", "", "", p.CivlID, score}); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("export: unknown competition type %q", typ)
		}
		score--
	}
	return f.Write(w)
}
