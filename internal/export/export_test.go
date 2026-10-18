package export

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func pilot(id int, name, country, gender string, awt bool) *ExportPilot {
	return &ExportPilot{CivlID: id, Name: name, Country: country, Gender: gender, IsAWT: awt}
}

func solo(score float64, p *ExportPilot) ExportEntry {
	return ExportEntry{Score: score, Pilot: p, PerRun: []ExportRunCell{{Rank: 1, Score: score}}}
}

func mixedFixture() CompetitionExport {
	return CompetitionExport{
		Name: "Test Open", Type: TypeSolo, Seasons: []string{"awt-2026"}, Final: true, LimitRun: -1,
		Lists: []ExportList{{Key: KeyOverall, Entries: []ExportEntry{
			solo(90, pilot(4, "Dan Dee", "de", "man", false)),
			solo(70, pilot(3, "Cara Cee", "it", "woman", true)),
			solo(80, pilot(2, "Bob Bee", "fr", "man", true)),
			solo(95, pilot(1, "Al Ay", "es", "man", false)),
			solo(60, pilot(5, "Eve Eff", "ch", "woman", false)),
		}}},
	}
}

func readRows(t *testing.T, b []byte) ([][]string, string) {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatal(err)
	}
	return rows, sheet
}

func TestCompetitionXLSXMixedAWTAWQ(t *testing.T) {
	var buf bytes.Buffer
	if err := CompetitionXLSX(&buf, mixedFixture()); err != nil {
		t.Fatal(err)
	}
	rows, sheet := readRows(t, buf.Bytes())
	if sheet != "overall solo" {
		t.Fatalf("sheet %q", sheet)
	}
	if got := strings.Join(rows[0], ","); got != "Rank,Number,First Name,Last Name,Nat,Gender,Glider,Sponsor,CIVL ID,Score" {
		t.Fatalf("header %s", got)
	}
	// AWT first (Bob 80, Cara 70) then AWQ (Al 95, Dan 90, Eve 60)
	want := []struct{ rank, civl, first, nat, g, score string }{
		{"1", "2", "Bob", "FR", "M", "100"},
		{"2", "3", "Cara", "IT", "F", "99"},
		{"3", "1", "Al", "ES", "M", "80"},
		{"4", "4", "Dan", "DE", "M", "79"},
		{"5", "5", "Eve", "CH", "F", "78"},
	}
	if len(rows) != 6 {
		t.Fatalf("rows %d", len(rows))
	}
	for i, w := range want {
		r := rows[i+1]
		if r[0] != w.rank || r[1] != w.civl || r[2] != w.first || r[4] != w.nat || r[5] != w.g || r[8] != w.civl || r[9] != w.score {
			t.Errorf("row %d = %v, want %+v", i+1, r, w)
		}
	}
}

func TestCompetitionXLSXNonTourSortedOnePointSequence(t *testing.T) {
	d := mixedFixture()
	d.Seasons = nil
	var buf bytes.Buffer
	if err := CompetitionXLSX(&buf, d); err != nil {
		t.Fatal(err)
	}
	rows, _ := readRows(t, buf.Bytes())
	var first, scores []string
	for _, r := range rows[1:] {
		first = append(first, r[2])
		scores = append(scores, r[9])
	}
	if strings.Join(first, ",") != "Al,Dan,Bob,Cara,Eve" || strings.Join(scores, ",") != "100,99,98,97,96" {
		t.Fatalf("%v %v", first, scores)
	}
}

func TestCompetitionXLSXSynchro(t *testing.T) {
	team := func(name string, s float64, ids ...int) ExportEntry {
		tm := &ExportTeam{Name: name}
		for _, id := range ids {
			tm.Pilots = append(tm.Pilots, ExportPilot{CivlID: id, Name: "Pilot Num", Country: "fr", Gender: "woman"})
		}
		return ExportEntry{Score: s, Team: tm}
	}
	d := CompetitionExport{Type: TypeSynchro, Seasons: []string{"awt-2026"}, LimitRun: -1,
		Lists: []ExportList{{Key: KeyOverall, Entries: []ExportEntry{team("B", 5, 11, 12), team("A", 9, 21, 22)}}}}
	var buf bytes.Buffer
	if err := CompetitionXLSX(&buf, d); err != nil {
		t.Fatal(err)
	}
	rows, sheet := readRows(t, buf.Bytes())
	if sheet != "overall synchro" || len(rows) != 5 {
		t.Fatalf("sheet %q rows %d", sheet, len(rows))
	}
	if rows[0][1] != "Team" || rows[0][10] != "Score" {
		t.Fatalf("header %v", rows[0])
	}
	want := [][]string{
		{"1", "A", "21", "Pilot", "Num", "FR", "M", "", "", "21", "100"},
		{"1", "A", "22", "Pilot", "Num", "FR", "M", "", "", "22", "100"},
		{"2", "B", "11", "Pilot", "Num", "FR", "M", "", "", "11", "99"},
		{"2", "B", "12", "Pilot", "Num", "FR", "M", "", "", "12", "99"},
	}
	for i, w := range want {
		if strings.Join(rows[i+1], "|") != strings.Join(w, "|") {
			t.Errorf("row %d = %v, want %v", i+1, rows[i+1], w)
		}
	}
}

func TestRunXLSXRequiresFinal(t *testing.T) {
	var buf bytes.Buffer
	err := RunXLSX(&buf, RunExport{Type: TypeSolo, Final: false})
	if !errors.Is(err, ErrRunNotFinal) || err.Error() != "run results are not final" {
		t.Fatalf("err = %v", err)
	}
	if buf.Len() != 0 {
		t.Fatal("wrote output for a non-final run")
	}
	d := RunExport{Type: TypeSolo, Final: true, Seasons: []string{"awt-2026"}, Lists: []RunList{{Key: KeyOverall, Entries: []RunEntry{
		{Score: 8, Pilot: pilot(1, "Q Pilot", "fr", "man", false)},
		{Score: 7, Pilot: pilot(2, "T Pilot", "fr", "man", true)},
	}}}}
	if err := RunXLSX(&buf, d); err != nil {
		t.Fatal(err)
	}
	rows, sheet := readRows(t, buf.Bytes())
	if sheet != "run solo" || rows[1][1] != "2" || rows[1][9] != "100" || rows[2][1] != "1" || rows[2][9] != "80" {
		t.Fatalf("%q %v", sheet, rows)
	}
}

func TestExportsUnicodeAndEscaping(t *testing.T) {
	names := []string{"Élise Ünal-Çakır", "Sean O'Brien", "Evil <script>alert(1)</script>"}
	var entries []ExportEntry
	var runEntries []RunEntry
	var start []StartingEntry
	for i, n := range names {
		p := pilot(100+i, n, "fr", "woman", true)
		entries = append(entries, solo(float64(10-i)+0.12345, p))
		runEntries = append(runEntries, RunEntry{Score: 9.5, Pilot: p, Warnings: []string{"<b>late</b>"}})
		start = append(start, StartingEntry{Pilot: p})
	}
	comp := CompetitionExport{Name: "Cup <1>", Type: TypeSolo, Seasons: []string{"awt-2026"}, Final: true, LimitRun: 1,
		UpdatedAt: time.Date(2026, 10, 18, 9, 5, 0, 0, time.UTC),
		Lists:     []ExportList{{Key: KeyOverall, Title: "Overall", Entries: entries}}}

	var x bytes.Buffer
	if err := CompetitionXLSX(&x, comp); err != nil {
		t.Fatal(err)
	}
	rows, _ := readRows(t, x.Bytes())
	if rows[1][2] != "Élise" || rows[1][3] != "Ünal-Çakır" || rows[2][2] != "Sean" || rows[2][3] != "O'Brien" ||
		rows[3][3] != "<script>alert(1)</script>" {
		t.Fatalf("xlsx names mangled: %v", rows)
	}

	var h bytes.Buffer
	if err := ResultsHTML(&h, comp); err != nil {
		t.Fatal(err)
	}
	s := h.String()
	for _, want := range []string{"Élise Ünal-Çakır", "O&#39;Brien", "&lt;script&gt;alert(1)&lt;/script&gt;", "Cup &lt;1&gt;",
		"final results after run 2", "Last update: 2026-10-18 09:05 UTC", "10.123", "/pilots/100/elise-unalcakr"} {
		if !strings.Contains(s, want) {
			t.Errorf("results html missing %q", want)
		}
	}
	if strings.Contains(s, "<script>") || strings.Contains(s, "https://") {
		t.Error("unescaped script or external URL in results html")
	}

	h.Reset()
	run := RunExport{CompName: "Cup", Type: TypeSolo, RunIndex: 0, Final: false,
		Lists: []RunList{{Key: KeyOverall, Entries: runEntries}}}
	if err := RunResultsHTML(&h, run); err != nil {
		t.Fatal(err)
	}
	s = h.String()
	for _, want := range []string{"Overall", "run 1 intermediate results", "&lt;b&gt;late&lt;/b&gt; (0.5pt)", "&lt;script&gt;", "O&#39;Brien"} {
		if !strings.Contains(s, want) {
			t.Errorf("run html missing %q", want)
		}
	}
	if strings.Contains(s, "<script>") || strings.Contains(s, "<b>late") {
		t.Error("unescaped markup in run html")
	}

	h.Reset()
	so := StartingOrderExport{CompName: "Cup", Type: TypeSolo, RunIndex: 1,
		Lists: []StartingList{{Title: "AWT", Entries: start}}}
	if err := StartingOrderHTML(&h, so); err != nil {
		t.Fatal(err)
	}
	s = h.String()
	if !strings.Contains(s, "Starting Order - run 2") || !strings.Contains(s, "AWT Starting Order") ||
		strings.Contains(s, "<script>") || !strings.Contains(s, "Élise Ünal-Çakır") {
		t.Errorf("starting order html wrong")
	}
}

func TestSynchroHTMLListsMembers(t *testing.T) {
	team := &ExportTeam{Name: "Duo", Pilots: []ExportPilot{{Name: "A One", Country: "fr"}, {Name: "B Two", Country: "de"}}}
	var h bytes.Buffer
	err := ResultsHTML(&h, CompetitionExport{Name: "C", Type: TypeSynchro, LimitRun: -1,
		Lists: []ExportList{{Key: "overall", Entries: []ExportEntry{{Score: 1, Team: team}}}}})
	if err != nil {
		t.Fatal(err)
	}
	s := h.String()
	for _, want := range []string{"Team", "Duo", "<li>A One</li>", "<li>B Two</li>", "<li>FR</li>", "<li>DE</li>", "intermediate results<"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
}
