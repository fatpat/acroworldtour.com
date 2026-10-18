package e2e

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/playwright-community/playwright-go"
)

var compSeq atomic.Int64

// comp is a competition created through the admin UI.
type comp struct {
	Code, Name string
	Synchro    bool
}

func (c comp) admin(path string) string { return e.baseURL + "/admin/competitions/" + c.Code + path }
func (c comp) public() string           { return e.baseURL + "/competitions/" + c.Code }

// createComp creates a published competition of the catalog year (the mini
// fixture has no catalog of the current year, the importer loads 2022-2026).
func createComp(t *testing.T, p *page, typ, label string) comp {
	t.Helper()
	n := compSeq.Add(1)
	c := comp{Name: fmt.Sprintf("E2E %s %d", label, n), Code: fmt.Sprintf("e2e-%s-%d", slug(label), n), Synchro: typ == "synchro"}
	_, err := p.Goto(e.baseURL + "/admin/competitions/new")
	must(t, err, "open the new competition form")
	must(t, p.Locator("#name").Fill(c.Name), "name")
	must(t, p.Locator("#code").Fill(c.Code), "code")
	_, err = p.Locator("#type").SelectOption(playwright.SelectOptionValues{Values: &[]string{typ}})
	must(t, err, "type")
	must(t, p.Locator("#start_date").Fill(fmt.Sprintf("%d-06-01", catalogYear)), "start date")
	must(t, p.Locator("#end_date").Fill(fmt.Sprintf("%d-06-03", catalogYear)), "end date")
	must(t, p.Locator("#location").Fill("E2E valley"), "location")
	must(t, p.GetByLabel("Published").Check(), "published")
	click(t, role(p, "button", "Save"), "Save competition")
	expectText(t, p.GetByRole("heading", playwright.PageGetByRoleOptions{Name: c.Name}), c.Name)
	return c
}

// addParticipant adds a pilot (solo) or team (synchro) found by name.
func addParticipant(t *testing.T, p *page, c comp, name string) {
	t.Helper()
	_, err := p.Goto(c.admin("/participants") + "?q=" + strings.ReplaceAll(name, " ", "+"))
	must(t, err, "open participants")
	row := p.Locator("tr").Filter(playwright.LocatorFilterOptions{HasText: name})
	click(t, row.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "Add", Exact: playwright.Bool(true)}), "Add "+name)
	must(t, p.Locator("tr").Filter(playwright.LocatorFilterOptions{HasText: name}).GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "Remove"}).WaitFor(), "registered "+name)
}

// pickJudges registers the two judges created by the suite setup.
func pickJudges(t *testing.T, p *page, c comp) {
	t.Helper()
	_, err := p.Goto(c.admin("/judges"))
	must(t, err, "open judges")
	must(t, p.GetByLabel("E2E Senior").Check(), "judge 1")
	must(t, p.GetByLabel("E2E Certified").Check(), "judge 2")
	click(t, role(p, "button", "Save judges"), "Save judges")
	must(t, p.GetByRole("status").WaitFor(), "judges saved")
}

// changeState walks a state confirmation page ("open" or "close") of the
// competition, or of one run when run > 0.
func changeState(t *testing.T, p *page, c comp, run int, action string) {
	t.Helper()
	path := "/state/" + action
	if run > 0 {
		path = fmt.Sprintf("/runs/%d/state/%s", run, action)
	}
	_, err := p.Goto(c.admin(path))
	must(t, err, "open "+path)
	click(t, role(p, "button", "Confirm "+action), "confirm "+action)
	must(t, p.GetByRole("status").WaitFor(), action+" done")
}

// setupOpenRun builds the competition up to an open run 1 with the given
// participants, and returns the judging URL.
func setupOpenRun(t *testing.T, p *page, typ, label string, participants ...string) (comp, string) {
	t.Helper()
	c := createComp(t, p, typ, label)
	for _, name := range participants {
		addParticipant(t, p, c, name)
	}
	pickJudges(t, p, c)
	changeState(t, p, c, 0, "open")
	_, err := p.Goto(c.admin("/runs/new"))
	must(t, err, "open the new run form")
	click(t, role(p, "button", "Create run"), "Create run")
	must(t, p.GetByRole("status").WaitFor(), "run created")
	changeState(t, p, c, 1, "open")
	return c, c.admin("/runs/1/judging")
}

// fillTrick types a trick acronym in the n-th (0-based) trick input.
func fillTrick(t *testing.T, p *page, n int, acronym string) {
	t.Helper()
	must(t, p.Locator(fmt.Sprintf("#trick-%d", n)).Fill(acronym), "trick "+acronym)
}

// fillMarks fills the mark of each judge for one column (technical,
// choreography, landing, synchro).
func fillMarks(t *testing.T, p *page, col string, marks ...string) {
	t.Helper()
	judges := []string{"E2E Senior", "E2E Certified"}
	for i, m := range marks {
		must(t, p.GetByLabel(judges[i]+", "+col, playwright.PageGetByLabelOptions{Exact: playwright.Bool(true)}).Fill(m), "mark "+col)
	}
}
