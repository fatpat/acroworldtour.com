package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
)

func TestSynchroScoring(t *testing.T) {
	p := adminPage(t)
	_, judging := setupOpenRun(t, p, "synchro", "synchro", "Fictional Duo", "E2E Duo")
	_, err := p.Goto(judging)
	must(t, err, "open the judging screen")
	expectText(t, p.Locator("#flight-heading"), "Fictional Duo")

	// Saving moves on to the next team: come back to this one by its URL.
	href, err := p.Locator("#participants a").Filter(playwright.LocatorFilterOptions{HasText: "Fictional Duo"}).GetAttribute("href")
	must(t, err, "team link")
	flight := e.baseURL + href
	// Synchro adds a synchro mark column and a tip touches field.
	fillTrick(t, p, 0, "BS")
	fillMarks(t, p, "technical", "7", "7")
	fillMarks(t, p, "choreography", "7", "7")
	fillMarks(t, p, "landing", "8", "8")
	fillMarks(t, p, "synchro", "9", "9")
	must(t, p.GetByLabel("Tip touches").Fill("2"), "tip touches")
	score := scoreOf(t, p)
	expectText(t, p.Locator("#provisional"), "Synchro")

	// Save, reload: marks and tip touches persist.
	click(t, role(p, "button", "Save"), "Save")
	expectText(t, p.Locator("#flight .notice"), "Saved.")
	_, err = p.Goto(flight)
	must(t, err, "reopen the flight")
	expectText(t, p.Locator("#flight"), "Saved, not published")
	assertValue(t, p.GetByLabel("Tip touches"), "2")
	assertValue(t, p.GetByLabel("E2E Senior, synchro", playwright.PageGetByLabelOptions{Exact: new(true)}), "9")
	if got := scoreOf(t, p); got != score {
		t.Fatalf("score after reload %s, want %s", got, score)
	}

	// More tip touches lower the score, and persist.
	must(t, p.GetByLabel("Tip touches").Fill("3"), "tip touches 3")
	if got := scoreOf(t, p); got == score {
		t.Fatalf("score still %s with 3 tip touches instead of 2", got)
	}
	click(t, role(p, "button", "Save"), "Save tip touches 3")
	expectText(t, p.Locator("#flight .notice"), "Saved.")
	_, err = p.Goto(flight)
	must(t, err, "reopen the flight")
	assertValue(t, p.GetByLabel("Tip touches"), "3")

	// DNS persists and shows in the participant list.
	must(t, p.GetByLabel("Did not start (DNS)").Check(), "check DNS")
	click(t, role(p, "button", "Save"), "Save DNS")
	expectText(t, p.Locator("#flight .notice"), "Saved.")
	_, err = p.Goto(flight)
	must(t, err, "reopen the flight")
	checked, err := p.GetByLabel("Did not start (DNS)").IsChecked()
	must(t, err, "read DNS")
	if !checked {
		t.Fatal("DNS was not persisted")
	}
	expectText(t, p.Locator("#participants"), "DNS")
	failOnPageErrors(t, p)
}

func assertValue(t *testing.T, l playwright.Locator, want string) {
	t.Helper()
	got, err := l.InputValue()
	must(t, err, "read input value")
	if got != want {
		t.Fatalf("input value %q, want %q", got, want)
	}
}
