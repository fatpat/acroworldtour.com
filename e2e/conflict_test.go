package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
)

// TestConcurrentEditConflict: two scorers open the same saved flight; the
// second to save is told the flight changed under it and nothing is lost.
func TestConcurrentEditConflict(t *testing.T) {
	a := adminPage(t)
	_, judging := setupOpenRun(t, a, "solo", "conflict", "Alice Aerial", "Bob Barrel")
	_, err := a.Goto(judging)
	must(t, err, "open the judging screen")
	expectText(t, a.Locator("#flight-heading"), "Bob Barrel")
	href, err := a.Locator("#participants a").Filter(playwright.LocatorFilterOptions{HasText: "Bob Barrel"}).GetAttribute("href")
	must(t, err, "Bob's link")
	bobURL := e.baseURL + href

	// A first save, so that the flight has a version both scorers load.
	flyPilot(t, a, "RK", [2]string{"6", "6"}, [2]string{"6", "6"}, [2]string{"7", "7"})
	click(t, role(a, "button", "Save"), "first save")
	expectText(t, a.Locator("#flight .notice"), "Saved.")

	_, err = a.Goto(bobURL)
	must(t, err, "A reopens Bob")
	b := adminPageIn(t)
	_, err = b.Goto(bobURL)
	must(t, err, "B opens Bob")
	expectText(t, b.Locator("#flight"), "Saved, not published")

	// A saves a change first.
	fillMarks(t, a, "technical", "8", "8")
	scoreA := scoreOf(t, a)
	click(t, role(a, "button", "Save"), "A saves")
	expectText(t, a.Locator("#flight .notice"), "Saved.")

	// B, still on the old version, saves: refused with the conflict message.
	fillMarks(t, b, "technical", "3", "3")
	scoreOf(t, b)
	click(t, role(b, "button", "Save"), "B saves")
	expectText(t, b.Locator("#provisional"), "changed by someone else")

	// Nothing was overwritten: reloading B shows A's flight.
	_, err = b.Goto(bobURL)
	must(t, err, "B reloads")
	if got := scoreOf(t, b); got != scoreA {
		t.Fatalf("stored score after the conflict %s, want A's %s", got, scoreA)
	}
	failOnPageErrors(t, a, b)
}
