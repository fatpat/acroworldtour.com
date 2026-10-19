package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/playwright-community/playwright-go"
)

const defaultTimeout = 10_000 // ms

// cspHook reports every securitypolicyviolation as a console error, so that
// the console listener sees it even when the browser did not log it.
//
// It also tracks the last input and the htmx requests in flight, so that
// settled can wait for the live simulate to be done (no sleeps).
const cspHook = `
document.addEventListener("securitypolicyviolation", e => console.error("CSP violation: " + e.violatedDirective + " " + e.blockedURI));
window.__awt = {lastInput: 0, inflight: 0};
for (const ev of ["input", "change"]) document.addEventListener(ev, () => { window.__awt.lastInput = performance.now(); }, true);
document.addEventListener("htmx:beforeRequest", () => { window.__awt.inflight++; });
document.addEventListener("htmx:afterRequest", () => { window.__awt.inflight--; });
`

// page is a browser page that records console errors, uncaught exceptions
// and CSP violations, with the URL of the page that logged them.
//
// On the judging screen a live simulate of a half-filled form answers 422 on
// purpose (the browser logs it as a failed resource), a conflicting save
// answers 409, and a save aborts a pending simulate (htmx logs sendAbort and
// the failed afterRequest): those exact messages are expected there and
// nowhere else. CSP violations and uncaught exceptions are never ignored.
type page struct {
	playwright.Page
	mu   sync.Mutex
	errs []consoleError
}

type consoleError struct{ url, msg string }

var (
	judgingURL   = regexp.MustCompile(`^https?://[^/]+/manager/competitions/[^/]+/runs/\d+/judging(/[^/?#]+)?([?#].*)?$`)
	judgingNoise = regexp.MustCompile(`^console error: (Failed to load resource: the server responded with a status of (422 \(Unprocessable Entity\)|409 \(Conflict\))|htmx:(sendAbort|afterRequest))$`)
)

func (p *page) errors() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for _, e := range p.errs {
		if judgingURL.MatchString(e.url) && judgingNoise.MatchString(e.msg) {
			continue
		}
		out = append(out, e.msg+" (on "+e.url+")")
	}
	return out
}

func (p *page) record(url, s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.errs = append(p.errs, consoleError{url: url, msg: s})
}

func watch(raw playwright.Page) *page {
	p := &page{Page: raw}
	raw.SetDefaultTimeout(defaultTimeout)
	raw.Context().SetDefaultTimeout(defaultTimeout)
	raw.OnConsole(func(m playwright.ConsoleMessage) {
		if m.Type() == "error" {
			p.record(raw.URL(), "console error: "+m.Text())
		}
	})
	raw.OnPageError(func(err error) { p.record(raw.URL(), "page error: "+err.Error()) })
	return p
}

// newContext opens an isolated browser context (own cookies, so a second one
// is a second browser). On failure it saves a trace and a screenshot of every
// page under e2e/.artifacts.
func newContext(t *testing.T) playwright.BrowserContext {
	t.Helper()
	ctx, err := e.browser.NewContext()
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	if err := ctx.AddInitScript(playwright.Script{Content: playwright.String(cspHook)}); err != nil {
		t.Fatalf("init script: %v", err)
	}
	if err := ctx.Tracing().Start(playwright.TracingStartOptions{Screenshots: playwright.Bool(true), Snapshots: playwright.Bool(true)}); err != nil {
		t.Fatalf("tracing: %v", err)
	}
	t.Cleanup(func() {
		var tracePath []string
		if t.Failed() {
			dir := artifactDir(t)
			for i, p := range ctx.Pages() {
				_, _ = p.Screenshot(playwright.PageScreenshotOptions{Path: playwright.String(filepath.Join(dir, fmt.Sprintf("page-%d.png", i))), FullPage: playwright.Bool(true)})
				if html, err := p.Content(); err == nil {
					_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("page-%d.html", i)), []byte(html), 0o644)
				}
			}
			tracePath = []string{filepath.Join(dir, "trace.zip")}
		}
		_ = ctx.Tracing().Stop(tracePath...)
		_ = ctx.Close()
	})
	return ctx
}

func artifactDir(t *testing.T) string {
	dir := filepath.Join(".artifacts", slug(t.Name()))
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func newPage(t *testing.T, ctx playwright.BrowserContext) *page {
	t.Helper()
	raw, err := ctx.NewPage()
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	return watch(raw)
}

// newAdminPage is the setup variant (no *testing.T): a logged-in page.
func newAdminPage() (*page, func(), error) {
	ctx, err := e.browser.NewContext()
	if err != nil {
		return nil, nil, err
	}
	raw, err := ctx.NewPage()
	if err != nil {
		return nil, nil, err
	}
	p := watch(raw)
	if err := doLogin(p); err != nil {
		return nil, nil, err
	}
	return p, func() { _ = ctx.Close() }, nil
}

func doLogin(p *page) error {
	if _, err := p.Goto(e.baseURL + "/manager/login"); err != nil {
		return err
	}
	if err := p.GetByLabel("Email").Fill(adminEmail); err != nil {
		return err
	}
	if err := p.GetByLabel("Password").Fill(adminPassword); err != nil {
		return err
	}
	if err := p.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Log in"}).Click(); err != nil {
		return err
	}
	return p.WaitForURL("**/manager")
}

// adminPage is a page of a fresh context, logged in as the admin.
func adminPage(t *testing.T) *page {
	t.Helper()
	p := newPage(t, newContext(t))
	if err := doLogin(p); err != nil {
		t.Fatalf("login: %v", err)
	}
	return p
}

// adminPageIn is a second logged-in page of its own context (a second
// scorer).
func adminPageIn(t *testing.T) *page { return adminPage(t) }

func must(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func role(p playwright.Page, r string, name string) playwright.Locator {
	return p.GetByRole(playwright.AriaRole(r), playwright.PageGetByRoleOptions{Name: name, Exact: playwright.Bool(true)})
}

func click(t *testing.T, l playwright.Locator, what string) {
	t.Helper()
	must(t, l.Click(), "click "+what)
}

// expectText waits until l contains want (web-first, no sleeps).
func expectText(t *testing.T, l playwright.Locator, want string) {
	t.Helper()
	if err := l.Filter(playwright.LocatorFilterOptions{HasText: want}).First().WaitFor(); err != nil {
		got, _ := l.First().InnerText()
		t.Fatalf("waiting for %q: %v\nlast text: %q", want, err, got)
	}
}

func failOnPageErrors(t *testing.T, pages ...*page) {
	t.Helper()
	for _, p := range pages {
		if errs := p.errors(); len(errs) > 0 {
			t.Errorf("browser errors on %s:\n  %s", p.URL(), strings.Join(errs, "\n  "))
		}
	}
}

// settled waits until the live simulate has caught up with the last edit:
// the 300 ms input debounce is over and no htmx request is in flight.
func settled(t *testing.T, p *page) {
	t.Helper()
	_, err := p.WaitForFunction(`() => window.__awt && performance.now() - window.__awt.lastInput > 400 && window.__awt.inflight === 0`, nil)
	must(t, err, "wait for the live simulate to settle")
}
