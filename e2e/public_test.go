package e2e

import (
	"sort"
	"testing"

	"github.com/playwright-community/playwright-go"
)

// links returns the distinct same-site hrefs of the links matching selector.
func links(t *testing.T, p *page, selector string) []string {
	t.Helper()
	raw, err := p.Locator(selector).EvaluateAll(`els => els.map(a => a.getAttribute("href"))`)
	must(t, err, "collect links "+selector)
	seen := map[string]bool{}
	var out []string
	for _, h := range raw.([]any) {
		href, _ := h.(string)
		if href == "" || href[0] != '/' || seen[href] {
			continue
		}
		seen[href] = true
		out = append(out, href)
	}
	sort.Strings(out)
	return out
}

// visit loads path and checks it answers 200 with a clean console and no
// CSP violation.
func visit(t *testing.T, p *page, path string) {
	t.Helper()
	before := len(p.errors())
	resp, err := p.Goto(e.baseURL+path, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
	if err != nil {
		t.Errorf("GET %s: %v", path, err)
		return
	}
	if resp.Status() != 200 {
		t.Errorf("GET %s: status %d, want 200", path, resp.Status())
	}
	if errs := p.errors(); len(errs) > before {
		t.Errorf("GET %s: browser errors:\n  %v", path, errs[before:])
	}
}

func TestPublicNavigation(t *testing.T) {
	p := newPage(t, newContext(t))
	visit(t, p, "/")

	nav := links(t, p, `nav[aria-label="Main"] a`)
	if len(nav) < 8 {
		t.Fatalf("only %d links in the main navigation: %v", len(nav), nav)
	}
	for _, href := range nav {
		visit(t, p, href)
	}

	// A competition of the mini fixture and each of its tabs.
	const comp = "/competitions/mini-solo-2024"
	visit(t, p, comp)
	for _, href := range links(t, p, `nav[aria-label="Competition sections"] a`) {
		visit(t, p, href)
	}
	// The simulator page works without errors too.
	visit(t, p, "/simulator")
}
