package e2e

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"

	"awt/internal/auth"
)

const (
	mfaEmail    = "mfa-admin@example.org"
	mfaPassword = "e2e mfa password"
)

// loginPassword runs the password step as email and returns once the
// browser left the login form.
func loginPassword(t *testing.T, p *page, email, password string) {
	t.Helper()
	_, err := p.Goto(e.baseURL + "/manager/login")
	must(t, err, "open the login form")
	must(t, p.GetByLabel("Email").Fill(email), "email")
	must(t, p.GetByLabel("Password").Fill(password), "password")
	click(t, role(p, "button", "Log in"), "Log in")
}

func logout(t *testing.T, p *page) {
	t.Helper()
	click(t, role(p, "button", "Log out"), "Log out")
	must(t, p.WaitForURL("**/manager/login"), "back on the login form")
}

// TestTOTPLoginFlow enrols an authenticator app on a dedicated admin (the
// shared admin must keep logging in with a password only), logs in with a
// code and with a recovery code, checks the code is spent, then lets the
// default admin remove the second factor.
func TestTOTPLoginFlow(t *testing.T) {
	root := adminPage(t)
	_, err := root.Goto(e.baseURL + "/manager/users")
	must(t, err, "open users")
	must(t, root.Locator("#email").Fill(mfaEmail), "new user email")
	must(t, root.Locator("#name").Fill("E2E MFA"), "new user name")
	_, err = root.Locator("#role").SelectOption(playwright.SelectOptionValues{Values: &[]string{"admin"}})
	must(t, err, "role")
	must(t, root.Locator("#password").Fill(mfaPassword), "new user password")
	click(t, role(root, "button", "Create user"), "Create user")
	expectText(t, root.Locator("table"), mfaEmail)

	p := newPage(t, newContext(t))
	loginPassword(t, p, mfaEmail, mfaPassword)
	must(t, p.WaitForURL("**/manager"), "password-only login before enrolment")

	// Enrol: read the setup key shown on the page, compute the code here.
	_, err = p.Goto(e.baseURL + "/manager/account")
	must(t, err, "open the account page")
	click(t, role(p, "button", "Set up an authenticator app"), "Set up")
	must(t, p.Locator("#totp-secret").WaitFor(), "setup key shown")
	shown, err := p.Locator("#totp-secret").InnerText()
	must(t, err, "read the setup key")
	secret := strings.ReplaceAll(shown, " ", "")
	code, err := auth.TOTPCode(secret, time.Now())
	must(t, err, "compute the code")
	must(t, p.Locator("#code").Fill(code), "code")
	must(t, p.Locator("#totp-password").Fill(mfaPassword), "current password")
	click(t, role(p, "button", "Turn on"), "Turn on")

	// Recovery codes are shown once; keep them.
	must(t, p.Locator("#recovery-codes li").First().WaitFor(), "recovery codes shown")
	codes, err := p.Locator("#recovery-codes li").AllInnerTexts()
	must(t, err, "read recovery codes")
	if len(codes) != 10 || !regexp.MustCompile(`^[2-9a-z]{5}-[2-9a-z]{5}$`).MatchString(codes[0]) {
		t.Fatalf("recovery codes = %q", codes)
	}
	click(t, role(p, "link", "I saved them"), "I saved them")
	expectText(t, p.Locator("main"), "10 recovery codes left")

	// The security key button is there (no authenticator in headless
	// chromium: only its presence, or the "does not support" message).
	if n, _ := p.Locator("[data-begin-url='/manager/account/keys/begin']").Count(); n != 1 {
		t.Fatalf("security key button count = %d", n)
	}
	logout(t, p)

	// Password, then the second step with a code of the next 30 s step (the
	// enrolment code's step is spent).
	loginPassword(t, p, mfaEmail, mfaPassword)
	must(t, p.WaitForURL("**/manager/login/mfa"), "second step")
	next, err := auth.TOTPCode(secret, time.Now().Add(30*time.Second))
	must(t, err, "next code")
	must(t, p.GetByLabel("Code from your authenticator app").Fill(next), "second step code")
	click(t, role(p, "button", "Verify"), "Verify")
	must(t, p.WaitForURL("**/manager"), "dashboard after the code")
	logout(t, p)

	// A recovery code logs in once.
	loginPassword(t, p, mfaEmail, mfaPassword)
	must(t, p.WaitForURL("**/manager/login/mfa"), "second step again")
	click(t, p.Locator("summary").Filter(playwright.LocatorFilterOptions{HasText: "Use a recovery code"}), "recovery summary")
	must(t, p.GetByLabel("Recovery code").Fill(codes[0]), "recovery code")
	click(t, role(p, "button", "Use recovery code"), "Use recovery code")
	expectText(t, p.Locator("main"), "9 recovery codes left")
	click(t, role(p, "link", "Continue"), "Continue")
	must(t, p.WaitForURL("**/manager"), "dashboard after the recovery code")
	failOnPageErrors(t, p, root)
	logout(t, p)

	// The same code is now refused.
	loginPassword(t, p, mfaEmail, mfaPassword)
	must(t, p.WaitForURL("**/manager/login/mfa"), "second step a third time")
	click(t, p.Locator("summary").Filter(playwright.LocatorFilterOptions{HasText: "Use a recovery code"}), "recovery summary")
	must(t, p.GetByLabel("Recovery code").Fill(codes[0]), "spent recovery code")
	click(t, role(p, "button", "Use recovery code"), "Use spent recovery code")
	must(t, p.Locator("[role=alert]").First().WaitFor(), "spent code refused")
	if strings.HasSuffix(p.URL(), "/manager") {
		t.Fatal("a spent recovery code logged in")
	}

	// The default admin removes the second factor: password only again.
	_, err = root.Goto(e.baseURL + "/manager/users")
	must(t, err, "open users")
	click(t, root.Locator("tr").Filter(playwright.LocatorFilterOptions{HasText: mfaEmail}).GetByRole("link", playwright.LocatorGetByRoleOptions{Name: "Manage"}), "Manage")
	click(t, role(root, "button", "Remove second factor"), "Remove second factor")
	click(t, role(root, "button", "Confirm reset"), "Confirm reset")
	expectText(t, root.Locator("main"), "Second factor removed")
	loginPassword(t, p, mfaEmail, mfaPassword)
	must(t, p.WaitForURL("**/manager"), "password-only login after the reset")
}
