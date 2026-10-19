package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"awt/internal/store/gen"
)

// mfaClock pins the Auth clock; tests move it with advance.
type mfaClock struct{ t time.Time }

func (c *mfaClock) now() time.Time          { return c.t }
func (c *mfaClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func testMFAKey(b byte) []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = b
	}
	return k
}

// newMFAAuth is newAuth with an MFA key and a pinned clock.
func newMFAAuth(t *testing.T) (*Auth, *mfaClock) {
	t.Helper()
	a := newAuth(t, "dev")
	a.cfg.MFAKey = testMFAKey(1)
	c := &mfaClock{t: time.Date(2026, 10, 19, 12, 0, 10, 0, time.UTC)}
	a.now = c.now
	return a, c
}

// enrolTOTP seeds a TOTP factor for u (Task 5 builds the enrolment UI) and
// returns the plain secret.
func enrolTOTP(t *testing.T, a *Auth, u User) string {
	t.Helper()
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := sealSecret(a.cfg.MFAKey, PurposeTOTPSecret+u.ID, secret)
	if err != nil {
		t.Fatal(err)
	}
	err = a.db.Tx(context.Background(), func(q *gen.Queries) error {
		return q.UpsertUserTOTP(context.Background(), gen.UpsertUserTOTPParams{UserID: u.ID, SecretEnc: sealed, CreatedAt: a.now().UTC().Format(time.RFC3339)})
	})
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

// seedRecovery stores fresh recovery codes for u and returns them for display.
func seedRecovery(t *testing.T, a *Auth, u User) []string {
	t.Helper()
	display, hashes, err := NewRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	err = a.db.Tx(context.Background(), func(q *gen.Queries) error {
		for _, h := range hashes {
			if err := q.InsertUserRecoveryCode(context.Background(), gen.InsertUserRecoveryCodeParams{UserID: u.ID, CodeHash: h}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return display
}

func code(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	c, err := TOTPCode(secret, at)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func wrongCode(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	for _, c := range []string{"000000", "111111"} {
		if _, ok := VerifyTOTP(secret, c, at, 0); !ok {
			return c
		}
	}
	t.Fatal("no wrong code found")
	return ""
}

// startPending runs the password step and returns a request carrying the
// pending cookie, from ip.
func startPending(t *testing.T, a *Auth, email, ip string) *http.Request {
	t.Helper()
	s, p, err := a.LoginStep1(context.Background(), email, "s3cret-passw0rd")
	if err != nil || p == nil || s.Token != "" {
		t.Fatalf("step 1: session %q, pending %v, %v", s.Token, p, err)
	}
	req := httptest.NewRequest(http.MethodPost, "/manager/login/mfa/totp", nil)
	req.RemoteAddr = ip + ":1234"
	req.AddCookie(&http.Cookie{Name: PendingCookieName, Value: p.Token})
	return req
}

func countRows(t *testing.T, a *Auth, query string, args ...any) int {
	t.Helper()
	var n int
	if err := a.db.Read.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLoginWithoutFactorsUnchanged(t *testing.T) {
	a, _ := newMFAAuth(t)
	u := mustUser(t, a, "plain@example.org", RoleAdmin)
	s, p, err := a.LoginStep1(context.Background(), "plain@example.org", "s3cret-passw0rd")
	if err != nil || p != nil || s.Token == "" || s.User.ID != u.ID {
		t.Fatalf("step 1: %+v %v %v", s, p, err)
	}
	s = mustLogin(t, a, "plain@example.org")
	if s.Token == "" {
		t.Fatal("Login without factors gave no session")
	}
	if has, err := a.UserHasMFA(context.Background(), u.ID); err != nil || has {
		t.Fatalf("UserHasMFA = %v, %v", has, err)
	}
}

func TestLoginWithTOTPRequiresSecondStep(t *testing.T) {
	a, c := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	enrolTOTP(t, a, u)
	seedRecovery(t, a, u)
	if has, err := a.UserHasMFA(context.Background(), u.ID); err != nil || !has {
		t.Fatalf("UserHasMFA = %v, %v", has, err)
	}
	if _, err := a.Login(context.Background(), "mfa@example.org", "s3cret-passw0rd"); !errors.Is(err, ErrMFARequired) {
		t.Fatalf("Login = %v, want ErrMFARequired", err)
	}
	if _, _, err := a.LoginStep1(context.Background(), "mfa@example.org", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password = %v", err)
	}
	s, p, err := a.LoginStep1(context.Background(), "mfa@example.org", "s3cret-passw0rd")
	if err != nil || p == nil || s.Token != "" {
		t.Fatalf("step 1: %+v %v %v", s, p, err)
	}
	if !p.ExpiresAt.Equal(c.now().Add(5*time.Minute)) || len(p.Methods) != 2 || p.Methods[0] != MethodTOTP || p.Methods[1] != MethodRecovery {
		t.Fatalf("pending = %+v", p)
	}
	if n := countRows(t, a, `SELECT count(*) FROM sessions WHERE user_id = ?`, u.ID); n != 0 {
		t.Fatalf("%d sessions after password step", n)
	}
	if n := countRows(t, a, `SELECT count(*) FROM login_challenges WHERE token_hash = ?`, p.Token); n != 0 {
		t.Fatal("pending token stored in clear")
	}
}

func TestPendingLoginGrantsNothing(t *testing.T) {
	a, _ := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	enrolTOTP(t, a, u)
	req := startPending(t, a, "mfa@example.org", "192.0.2.1")
	pc, _ := req.Cookie(PendingCookieName)

	h := a.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("in")) }))
	rec := do(h, http.MethodGet, "/manager", pc, nil, "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/manager/login?next=%2Fmanager" {
		t.Fatalf("pending cookie on /manager: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	// The pending token is no session token either.
	r := httptest.NewRequest(http.MethodGet, "/manager", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: pc.Value})
	if _, err := a.UserFromRequest(r); err == nil {
		t.Fatal("pending token accepted as a session")
	}
}

func TestPendingCookieAttributes(t *testing.T) {
	a, c := newMFAAuth(t)
	a.cfg.Env = "prod"
	rec := httptest.NewRecorder()
	a.SetPendingCookie(rec, Pending{Token: "tok", ExpiresAt: c.now().Add(pendingTTL)})
	ck := rec.Result().Cookies()[0]
	if ck.Name != "awt_mfa" || ck.Path != "/manager/login" || !ck.HttpOnly || !ck.Secure || ck.SameSite != http.SameSiteStrictMode || ck.MaxAge != 300 {
		t.Fatalf("cookie = %+v", ck)
	}
	rec = httptest.NewRecorder()
	a.ClearPendingCookie(rec)
	if ck := rec.Result().Cookies()[0]; ck.Name != "awt_mfa" || ck.MaxAge >= 0 || ck.Path != "/manager/login" {
		t.Fatalf("cleared cookie = %+v", ck)
	}
}

func TestTOTPCompletesLogin(t *testing.T) {
	a, c := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	secret := enrolTOTP(t, a, u)
	req := startPending(t, a, "mfa@example.org", "192.0.2.1")
	p, err := a.PendingFromRequest(req)
	if err != nil || p.UserID != u.ID || p.Email != u.Email || !p.TOTPAvailable || p.Next != "/manager" {
		t.Fatalf("pending = %+v, %v", p, err)
	}
	// One failure first, to check success resets the counter.
	if _, err := a.CompleteWithTOTP(context.Background(), req, wrongCode(t, secret, c.now())); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("wrong code = %v", err)
	}
	s, err := a.CompleteWithTOTP(context.Background(), req, code(t, secret, c.now()))
	if err != nil || s.Token == "" || s.User.ID != u.ID {
		t.Fatalf("complete: %+v %v", s, err)
	}
	r := httptest.NewRequest(http.MethodGet, "/manager", nil)
	r.AddCookie(cookieFor(s))
	if got, err := a.UserFromRequest(r); err != nil || got.User.ID != u.ID {
		t.Fatalf("session not usable: %v", err)
	}
	if n := countRows(t, a, `SELECT count(*) FROM login_challenges`); n != 0 {
		t.Fatalf("%d challenges left after success", n)
	}
	if n := countRows(t, a, `SELECT mfa_failed_count FROM users WHERE id = ?`, u.ID); n != 0 {
		t.Fatalf("failed count %d after success", n)
	}
	if _, err := a.PendingFromRequest(req); !errors.Is(err, ErrPendingExpired) {
		t.Fatalf("pending reused after success: %v", err)
	}
}

func TestTOTPCodeReplayRejected(t *testing.T) {
	a, c := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	secret := enrolTOTP(t, a, u)
	cd := code(t, secret, c.now())
	if _, err := a.CompleteWithTOTP(context.Background(), startPending(t, a, u.Email, "192.0.2.1"), cd); err != nil {
		t.Fatal(err)
	}
	c.advance(5 * time.Second) // same 30 s step
	if _, err := a.CompleteWithTOTP(context.Background(), startPending(t, a, u.Email, "192.0.2.1"), cd); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("replayed code = %v, want ErrMFAInvalid", err)
	}
	// The next step's code still works.
	c.advance(30 * time.Second)
	if _, err := a.CompleteWithTOTP(context.Background(), startPending(t, a, u.Email, "192.0.2.1"), code(t, secret, c.now())); err != nil {
		t.Fatalf("next step: %v", err)
	}
}

// The compare-and-set on last_step refuses a step that a concurrent login
// already consumed, even when the verification read an older last_step.
func TestTOTPStepCompareAndSet(t *testing.T) {
	a, _ := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	enrolTOTP(t, a, u)
	ctx := context.Background()
	for i, want := range []int64{1, 0} {
		var n int64
		err := a.db.Tx(ctx, func(q *gen.Queries) error {
			var err error
			n, err = q.AdvanceUserTOTPStep(ctx, gen.AdvanceUserTOTPStepParams{LastStep: 42, UserID: u.ID, LastStep_2: 42})
			return err
		})
		if err != nil || n != want {
			t.Fatalf("advance #%d: %d rows, %v", i, n, err)
		}
	}
}

func TestFactorLockoutSurvivesPasswordReentry(t *testing.T) {
	a, c := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	secret := enrolTOTP(t, a, u)
	seedRecovery(t, a, u)
	ctx := context.Background()
	req := startPending(t, a, u.Email, "192.0.2.1")
	bad := wrongCode(t, secret, c.now())
	for i := 1; i <= 4; i++ {
		if _, err := a.CompleteWithTOTP(ctx, req, bad); !errors.Is(err, ErrMFAInvalid) {
			t.Fatalf("attempt %d = %v", i, err)
		}
	}
	var locked *ErrMFALocked
	if _, err := a.CompleteWithTOTP(ctx, req, bad); !errors.As(err, &locked) || !locked.Until.Equal(c.now().Add(15*time.Minute)) {
		t.Fatalf("5th attempt = %v", err)
	}
	// A fresh pending login from another IP, with the correct code: still locked.
	c.advance(time.Minute)
	req2 := startPending(t, a, u.Email, "198.51.100.7")
	if _, err := a.CompleteWithTOTP(ctx, req2, code(t, secret, c.now())); !errors.As(err, &locked) {
		t.Fatalf("6th attempt (correct code, new pending, other IP) = %v", err)
	}
	if _, _, err := a.CompleteWithRecovery(ctx, req2, "whatever"); !errors.As(err, &locked) {
		t.Fatalf("recovery while locked = %v", err)
	}
	if n := countRows(t, a, `SELECT count(*) FROM sessions WHERE user_id = ?`, u.ID); n != 0 {
		t.Fatalf("%d sessions while locked", n)
	}

	// After the lock, a single failure does not re-lock (the count restarts).
	c.advance(15 * time.Minute)
	req3 := startPending(t, a, u.Email, "192.0.2.1")
	if _, err := a.CompleteWithTOTP(ctx, req3, wrongCode(t, secret, c.now())); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("first failure after lock = %v", err)
	}
	if _, err := a.CompleteWithTOTP(ctx, req3, code(t, secret, c.now())); err != nil {
		t.Fatalf("correct code after lock = %v", err)
	}
	var until sql.NullString
	if err := a.db.Read.QueryRow(`SELECT mfa_locked_until FROM users WHERE id = ?`, u.ID).Scan(&until); err != nil || until.Valid {
		t.Fatalf("lock not cleared on success: %v %v", until, err)
	}
}

func TestRecoveryCodeSingleUse(t *testing.T) {
	a, c := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	enrolTOTP(t, a, u)
	codes := seedRecovery(t, a, u)
	ctx := context.Background()
	// Accepted upper-case and without the dash.
	in := codes[3][:5] + codes[3][6:]
	s, left, err := a.CompleteWithRecovery(ctx, startPending(t, a, u.Email, "192.0.2.1"), " "+toUpper(in)+" ")
	if err != nil || s.Token == "" || left != 9 {
		t.Fatalf("recovery: %v left=%d %v", s.Token != "", left, err)
	}
	c.advance(time.Second)
	if _, _, err := a.CompleteWithRecovery(ctx, startPending(t, a, u.Email, "192.0.2.1"), codes[3]); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("reused recovery code = %v", err)
	}
	if _, left, err := a.CompleteWithRecovery(ctx, startPending(t, a, u.Email, "192.0.2.1"), "ABCDE-"+codes[4][6:]); !errors.Is(err, ErrMFAInvalid) || left != 0 {
		t.Fatalf("unknown code = %v", err)
	}
	if _, left, err := a.CompleteWithRecovery(ctx, startPending(t, a, u.Email, "192.0.2.1"), codes[4]); err != nil || left != 8 {
		t.Fatalf("displayed form: left=%d %v", left, err)
	}
}

func toUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

func TestPendingLoginExpires(t *testing.T) {
	a, c := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	secret := enrolTOTP(t, a, u)
	req := startPending(t, a, u.Email, "192.0.2.1")
	c.advance(5*time.Minute + time.Second)
	if _, err := a.PendingFromRequest(req); !errors.Is(err, ErrPendingExpired) {
		t.Fatalf("expired pending = %v", err)
	}
	if _, err := a.CompleteWithTOTP(context.Background(), req, code(t, secret, c.now())); !errors.Is(err, ErrPendingExpired) {
		t.Fatalf("complete after expiry = %v", err)
	}
	// The next password step prunes expired challenges.
	startPending(t, a, u.Email, "192.0.2.1")
	if n := countRows(t, a, `SELECT count(*) FROM login_challenges`); n != 1 {
		t.Fatalf("%d challenges, want only the fresh one", n)
	}
	// No cookie at all.
	if _, err := a.PendingFromRequest(httptest.NewRequest(http.MethodGet, "/manager/login/mfa", nil)); !errors.Is(err, ErrPendingExpired) {
		t.Fatalf("no cookie = %v", err)
	}
}

func TestTOTPUnavailableOnKeyMismatch(t *testing.T) {
	a, c := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	secret := enrolTOTP(t, a, u)
	codes := seedRecovery(t, a, u)
	a.cfg.MFAKey = testMFAKey(2) // AWT_MFA_KEY changed since enrolment
	req := startPending(t, a, u.Email, "192.0.2.1")
	p, err := a.PendingFromRequest(req)
	if err != nil || p.TOTPAvailable {
		t.Fatalf("pending = %+v, %v", p, err)
	}
	if _, err := a.CompleteWithTOTP(context.Background(), req, code(t, secret, c.now())); !errors.Is(err, ErrTOTPUnavailable) {
		t.Fatalf("TOTP with wrong key = %v", err)
	}
	if n := countRows(t, a, `SELECT mfa_failed_count FROM users WHERE id = ?`, u.ID); n != 0 {
		t.Fatalf("key mismatch counted as a failure: %d", n)
	}
	if _, _, err := a.CompleteWithRecovery(context.Background(), req, codes[0]); err != nil {
		t.Fatalf("recovery fallback: %v", err)
	}
}

func TestHandleLoginRedirectsToSecondStep(t *testing.T) {
	a, _ := newMFAAuth(t)
	u := mustUser(t, a, "mfa@example.org", RoleAdmin)
	enrolTOTP(t, a, u)
	rec := do(http.HandlerFunc(a.HandleLogin), http.MethodPost, "/manager/login", nil, nil,
		"email=mfa%40example.org&password=s3cret-passw0rd&next=%2Fmanager%2Fpilots")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/manager/login/mfa" {
		t.Fatalf("step 1: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	cks := rec.Result().Cookies()
	if len(cks) != 1 || cks[0].Name != PendingCookieName {
		t.Fatalf("cookies = %v", cks)
	}
	req := httptest.NewRequest(http.MethodGet, "/manager/login/mfa", nil)
	req.AddCookie(cks[0])
	if p, err := a.PendingFromRequest(req); err != nil || p.Next != "/manager/pilots" {
		t.Fatalf("next not kept: %+v %v", p, err)
	}
	// An unsafe next is replaced by the default.
	rec = do(http.HandlerFunc(a.HandleLogin), http.MethodPost, "/manager/login", nil, nil,
		"email=mfa%40example.org&password=s3cret-passw0rd&next=%2F%2Fevil.example")
	req = httptest.NewRequest(http.MethodGet, "/manager/login/mfa", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	if p, err := a.PendingFromRequest(req); err != nil || p.Next != "/manager" {
		t.Fatalf("unsafe next: %+v %v", p, err)
	}
}
