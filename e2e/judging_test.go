package e2e

import (
	"regexp"
	"strings"
	"testing"
)

var numberRE = regexp.MustCompile(`^\d+(\.\d+)?$`)

// scoreOf reads the provisional score once it is a number.
func scoreOf(t *testing.T, p *page) string {
	t.Helper()
	settled(t, p)
	l := p.Locator("#provisional dt:has-text('Score') + dd")
	must(t, l.WaitFor(), "provisional score")
	got, err := l.InnerText()
	must(t, err, "read the provisional score")
	got = strings.TrimSpace(got)
	if !numberRE.MatchString(got) {
		t.Fatalf("provisional score %q is not a number", got)
	}
	return got
}

// flyPilot enters a flight on the judging screen and returns its provisional
// score, which must show before anything is saved.
func flyPilot(t *testing.T, p *page, trick string, technical, choreography, landing [2]string) string {
	t.Helper()
	fillTrick(t, p, 0, trick)
	fillMarks(t, p, "technical", technical[0], technical[1])
	fillMarks(t, p, "choreography", choreography[0], choreography[1])
	fillMarks(t, p, "landing", landing[0], landing[1])
	// Live simulate: the score appears without any save.
	return scoreOf(t, p)
}

func TestJudgingWorkflow(t *testing.T) {
	p := adminPage(t)
	c, judging := setupOpenRun(t, p, "solo", "workflow", "Alice Aerial", "Bob Barrel")

	// A second, anonymous browser follows the live competition page.
	spectator := newPage(t, newContext(t))
	_, err := spectator.ExpectResponse("**/live/competitions/"+c.Code, func() error {
		_, err := spectator.Goto(c.public())
		return err
	})
	if err != nil {
		t.Fatalf("spectator SSE connection: %v", err)
	}
	must(t, spectator.Locator("#ranking").WaitFor(), "spectator ranking")
	if _, err := spectator.Evaluate(`window.__noReload = true`); err != nil {
		t.Fatalf("mark the spectator page: %v", err)
	}
	expectText(t, spectator.Locator("#ranking"), "No results at the moment")

	_, err = p.Goto(judging)
	must(t, err, "open the judging screen")
	// Bob flies first (starting order), Alice, an AWT pilot, second.
	expectText(t, p.Locator("#flight-heading"), "Bob Barrel")
	bob := flyPilot(t, p, "RK", [2]string{"6", "6.5"}, [2]string{"6", "6"}, [2]string{"7", "7"})
	click(t, role(p, "button", "Save & publish"), "Save & publish Bob")
	// Saving moves on to the next participant and lists Bob's score.
	expectText(t, p.Locator("#flight .notice"), "Published.")
	expectText(t, p.Locator("#participants"), bob)

	// The spectator gets the ranking by SSE, without reload.
	expectText(t, spectator.Locator("#ranking"), "Bob Barrel")
	expectText(t, spectator.Locator("#ranking"), bob)
	if v, err := spectator.Evaluate(`window.__noReload === true`); err != nil || v != true {
		t.Fatalf("spectator page was reloaded (marker %v, err %v)", v, err)
	}

	expectText(t, p.Locator("#flight-heading"), "Alice Aerial")
	alice := flyPilot(t, p, "LJ", [2]string{"8", "8.5"}, [2]string{"8", "8"}, [2]string{"9", "9"})
	click(t, role(p, "button", "Save & publish"), "Save & publish Alice")
	expectText(t, p.Locator("#flight"), "Saved and published")
	expectText(t, p.Locator("#participants"), alice)
	// Second publish within 5 s of the first: it reaches the spectator too
	// (the fragment must not be served from the browser cache).
	expectText(t, spectator.Locator("#ranking"), "Alice Aerial")
	expectText(t, spectator.Locator("#ranking"), alice)
	if v, err := spectator.Evaluate(`window.__noReload === true`); err != nil || v != true {
		t.Fatalf("spectator page was reloaded after the second update (marker %v, err %v)", v, err)
	}
	if alice == bob {
		t.Fatalf("both pilots scored %s", alice)
	}

	// Close the run, then the competition.
	changeState(t, p, c, 1, "close")
	changeState(t, p, c, 0, "close")

	// The public page shows the winner first and a final badge.
	// (A fresh browser: public pages are browser-cacheable for 30 s.)
	visitor := newPage(t, newContext(t))
	_, err = visitor.Goto(c.public())
	must(t, err, "open the public page")
	first := visitor.Locator("table.results tbody tr").First()
	expectText(t, first, "Alice Aerial")
	expectText(t, first, alice)
	expectText(t, visitor.Locator("#overall h2"), "Final")
	failOnPageErrors(t, p, spectator, visitor)
}
