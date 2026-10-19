package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"awt/internal/auth/webauthntest"
	"awt/internal/store/gen"
)

const testPassword = "s3cret-passw0rd"

// accountTOTP enrols TOTP for u through the account API, keeping session
// keep, and returns the plain secret and the recovery codes shown.
func accountTOTP(t *testing.T, a *Auth, u User, keep string) (string, []string) {
	t.Helper()
	ctx := context.Background()
	secret, _, pending, err := a.StartTOTPEnrollment(ctx, u.ID)
	if err != nil {
		t.Fatalf("start enrolment: %v", err)
	}
	code, err := TOTPCode(secret, a.now())
	if err != nil {
		t.Fatal(err)
	}
	codes, err := a.ConfirmTOTP(ctx, u.ID, pending, code, testPassword, keep)
	if err != nil {
		t.Fatalf("confirm enrolment: %v", err)
	}
	return secret, codes
}

func sessionsOf(t *testing.T, a *Auth, u User) int {
	t.Helper()
	return countRows(t, a, `SELECT count(*) FROM sessions WHERE user_id = ?`, u.ID)
}

func hasSession(t *testing.T, a *Auth, s Session) bool {
	t.Helper()
	return countRows(t, a, `SELECT count(*) FROM sessions WHERE token_hash = ?`, hashToken(s.Token)) == 1
}

func TestEnrollTOTP(t *testing.T) {
	a, c := newMFAAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "app@example.org", RoleScorer)
	other := mustUser(t, a, "other@example.org", RoleScorer)
	current := mustLogin(t, a, "app@example.org")
	elsewhere := mustLogin(t, a, "app@example.org")

	secret, uri, pending, err := a.StartTOTPEnrollment(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if uri != TOTPURI(secret, "app@example.org") || strings.Contains(pending, secret) {
		t.Fatalf("uri %q, pending %q", uri, pending)
	}
	if n := countRows(t, a, `SELECT count(*) FROM user_totp`); n != 0 {
		t.Fatal("start stored a secret")
	}
	// The QR route reopens the pending value.
	if s2, uri2, err := a.TOTPEnrollment(ctx, u.ID, pending); err != nil || s2 != secret || uri2 != uri {
		t.Fatalf("reopen = %q %q %v", s2, uri2, err)
	}
	if _, _, err := a.TOTPEnrollment(ctx, other.ID, pending); !errors.Is(err, ErrEnrollmentExpired) {
		t.Fatalf("other user's pending = %v, want ErrEnrollmentExpired", err)
	}
	code, _ := TOTPCode(secret, c.now())
	unchanged := func() {
		t.Helper()
		if s, err := a.MFAStatus(ctx, u.ID); err != nil || s.TOTP || s.RecoveryLeft != 0 {
			t.Fatalf("status = %+v, %v; want nothing enrolled", s, err)
		}
		if sessionsOf(t, a, u) != 2 {
			t.Fatal("sessions revoked by a refused enrolment")
		}
	}
	if _, err := a.ConfirmTOTP(ctx, u.ID, pending, code, "wrong-password", current.Token); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password = %v", err)
	}
	unchanged()
	if _, err := a.ConfirmTOTP(ctx, u.ID, pending, wrongCode(t, secret, c.now()), testPassword, current.Token); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("wrong code = %v", err)
	}
	unchanged()
	if _, err := a.ConfirmTOTP(ctx, other.ID, pending, code, testPassword, current.Token); !errors.Is(err, ErrEnrollmentExpired) {
		t.Fatalf("other user = %v", err)
	}
	if _, err := a.ConfirmTOTP(ctx, u.ID, pending[:len(pending)-4]+"AAAA", code, testPassword, current.Token); !errors.Is(err, ErrEnrollmentExpired) {
		t.Fatalf("tampered pending = %v", err)
	}
	unchanged()

	codes, err := a.ConfirmTOTP(ctx, u.ID, pending, code, testPassword, current.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 10 {
		t.Fatalf("%d recovery codes, want 10", len(codes))
	}
	st, err := a.MFAStatus(ctx, u.ID)
	if err != nil || !st.TOTP || st.RecoveryLeft != 10 || len(st.Keys) != 0 {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if !hasSession(t, a, current) || hasSession(t, a, elsewhere) {
		t.Fatal("want the current session kept and the other revoked")
	}
	if n := countRows(t, a, `SELECT count(*) FROM user_totp WHERE user_id = ? AND secret_enc NOT LIKE ?`, u.ID, "%"+secret+"%"); n != 1 {
		t.Fatal("secret not stored sealed")
	}

	// The enrolment code is spent; the next step's code logs in.
	req := startPending(t, a, "app@example.org", "192.0.2.1")
	if _, err := a.CompleteWithTOTP(ctx, req, code); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("enrolment code reused at login = %v", err)
	}
	c.advance(30 * time.Second)
	next, _ := TOTPCode(secret, c.now())
	if s, err := a.CompleteWithTOTP(ctx, req, next); err != nil || s.User.ID != u.ID {
		t.Fatalf("login with the new factor: %v", err)
	}

	// A second app cannot replace the first without removing it.
	secret2, _, pending2, _ := a.StartTOTPEnrollment(ctx, u.ID)
	code2, _ := TOTPCode(secret2, c.now())
	if _, err := a.ConfirmTOTP(ctx, u.ID, pending2, code2, testPassword, current.Token); !errors.Is(err, ErrTOTPEnrolled) {
		t.Fatalf("second enrolment = %v, want ErrTOTPEnrolled", err)
	}
}

func TestTOTPEnrollmentExpires(t *testing.T) {
	a, c := newMFAAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "app@example.org", RoleScorer)
	secret, _, pending, err := a.StartTOTPEnrollment(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	c.advance(10*time.Minute + time.Second)
	code, _ := TOTPCode(secret, c.now())
	if _, _, err := a.TOTPEnrollment(ctx, u.ID, pending); !errors.Is(err, ErrEnrollmentExpired) {
		t.Fatalf("reopen after 10 minutes = %v", err)
	}
	if _, err := a.ConfirmTOTP(ctx, u.ID, pending, code, testPassword, ""); !errors.Is(err, ErrEnrollmentExpired) {
		t.Fatalf("confirm after 10 minutes = %v", err)
	}
}

func TestRemoveLastFactorDisablesMFA(t *testing.T) {
	a, _ := newMFAAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "app@example.org", RoleScorer)
	current := mustLogin(t, a, "app@example.org")
	accountTOTP(t, a, u, current.Token)
	elsewhere := extraSession(t, a, u)

	if err := a.RemoveTOTP(ctx, u.ID, "wrong-password", current.Token); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password = %v", err)
	}
	if st, _ := a.MFAStatus(ctx, u.ID); !st.TOTP || st.RecoveryLeft != 10 {
		t.Fatalf("refused removal changed %+v", st)
	}
	if err := a.RemoveTOTP(ctx, u.ID, testPassword, current.Token); err != nil {
		t.Fatal(err)
	}
	st, err := a.MFAStatus(ctx, u.ID)
	if err != nil || st.TOTP || st.RecoveryLeft != 0 {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if n := countRows(t, a, `SELECT count(*) FROM user_recovery_codes WHERE user_id = ?`, u.ID); n != 0 {
		t.Fatal("recovery codes left after the last factor")
	}
	if !hasSession(t, a, current) || hasSession(t, a, elsewhere) {
		t.Fatal("want the current session kept and the other revoked")
	}
	s, p, err := a.LoginStep1(ctx, "app@example.org", testPassword)
	if err != nil || p != nil || s.Token == "" {
		t.Fatalf("login after removal: %v %v", p, err)
	}
	if err := a.RemoveTOTP(ctx, u.ID, testPassword, current.Token); !errors.Is(err, ErrNoTOTP) {
		t.Fatalf("remove twice = %v", err)
	}
}

// extraSession opens another session for u, as a login elsewhere would.
func extraSession(t *testing.T, a *Auth, u User) Session {
	t.Helper()
	var s Session
	err := a.db.Tx(context.Background(), func(q *gen.Queries) error {
		var err error
		s, err = a.insertSession(context.Background(), q, u, a.now().UTC())
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRemoveOneOfTwoFactorsKeepsRecoveryCodes(t *testing.T) {
	a, _ := newWebAuthnAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "key@example.org", RoleAdmin)
	other := mustUser(t, a, "other@example.org", RoleAdmin)
	current := mustLogin(t, a, "key@example.org")
	_, codes := accountTOTP(t, a, u, current.Token)
	key := registerKey(t, a, u, "Key")
	otherKey := registerKey(t, a, other, "Theirs")
	if st, _ := a.MFAStatus(ctx, u.ID); len(st.Keys) != 1 || st.Keys[0].Name != "Key" || st.Keys[0].ID != credentialID(key.ID) || st.Keys[0].CreatedAt.IsZero() || !st.Keys[0].LastUsedAt.IsZero() || st.RecoveryLeft != 10 {
		t.Fatalf("status = %+v", st)
	}

	if err := a.RemoveKey(ctx, u.ID, credentialID(otherKey.ID), testPassword, current.Token); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("another user's key = %v, want ErrKeyNotFound", err)
	}
	if err := a.RemoveKey(ctx, u.ID, credentialID(key.ID), "wrong-password", current.Token); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password = %v", err)
	}
	if err := a.RemoveKey(ctx, u.ID, credentialID(key.ID), testPassword, current.Token); err != nil {
		t.Fatal(err)
	}
	st, _ := a.MFAStatus(ctx, u.ID)
	if len(st.Keys) != 0 || !st.TOTP || st.RecoveryLeft != 10 {
		t.Fatalf("status = %+v", st)
	}
	req := startPending(t, a, "key@example.org", "192.0.2.1")
	if _, _, err := a.CompleteWithRecovery(ctx, req, codes[0]); err != nil {
		t.Fatalf("recovery code after removing one factor: %v", err)
	}
	if n := countRows(t, a, `SELECT count(*) FROM user_webauthn_credentials WHERE user_id = ?`, other.ID); n != 1 {
		t.Fatal("other user's key touched")
	}
}

func TestRegenerateRecoveryCodes(t *testing.T) {
	a, _ := newMFAAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "app@example.org", RoleScorer)
	current := mustLogin(t, a, "app@example.org")

	if _, err := a.RegenerateRecoveryCodes(ctx, u.ID, testPassword, current.Token); !errors.Is(err, ErrNoFactor) {
		t.Fatalf("without a factor = %v, want ErrNoFactor", err)
	}
	_, old := accountTOTP(t, a, u, current.Token)
	elsewhere := extraSession(t, a, u)
	if _, err := a.RegenerateRecoveryCodes(ctx, u.ID, "wrong-password", current.Token); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password = %v", err)
	}
	fresh, err := a.RegenerateRecoveryCodes(ctx, u.ID, testPassword, current.Token)
	if err != nil || len(fresh) != 10 {
		t.Fatalf("regenerate = %d codes, %v", len(fresh), err)
	}
	if !hasSession(t, a, current) || hasSession(t, a, elsewhere) {
		t.Fatal("want the current session kept and the other revoked")
	}
	req := startPending(t, a, "app@example.org", "192.0.2.1")
	if _, _, err := a.CompleteWithRecovery(ctx, req, old[0]); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("old code = %v, want ErrMFAInvalid", err)
	}
	if _, left, err := a.CompleteWithRecovery(ctx, req, fresh[0]); err != nil || left != 9 {
		t.Fatalf("new code = %d left, %v", left, err)
	}
}

func TestChangePassword(t *testing.T) {
	a, _ := newMFAAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "pw@example.org", RoleScorer)
	current := mustLogin(t, a, "pw@example.org")
	elsewhere := mustLogin(t, a, "pw@example.org")

	if err := a.ChangePassword(ctx, u.ID, "wrong-password", "a-brand-new-one", current.Token); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong current = %v", err)
	}
	var in *InputError
	if err := a.ChangePassword(ctx, u.ID, testPassword, "short", current.Token); !errors.As(err, &in) || in.Field != "new_password" {
		t.Fatalf("short password = %v, want InputError on new_password", err)
	}
	if sessionsOf(t, a, u) != 2 {
		t.Fatal("refused change revoked sessions")
	}
	if err := a.ChangePassword(ctx, u.ID, testPassword, "a-brand-new-one", current.Token); err != nil {
		t.Fatal(err)
	}
	if !hasSession(t, a, current) || hasSession(t, a, elsewhere) {
		t.Fatal("want the current session kept and the other revoked")
	}
	if _, err := a.Login(ctx, "pw@example.org", testPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password = %v", err)
	}
	if _, err := a.Login(ctx, "pw@example.org", "a-brand-new-one"); err != nil {
		t.Fatalf("new password: %v", err)
	}
}

func TestAddSecurityKeyFromAccount(t *testing.T) {
	a, _ := newWebAuthnAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "key@example.org", RoleScorer)
	current := mustLogin(t, a, "key@example.org")
	elsewhere := mustLogin(t, a, "key@example.org")
	key, err := webauthntest.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	opts, token, err := a.BeginWebAuthnRegistration(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := key.Create(opts, waOrigin)
	if _, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, "Key", "wrong-password", current.Token, body); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password = %v", err)
	}
	if n := countRows(t, a, `SELECT count(*) FROM user_webauthn_credentials`); n != 0 || sessionsOf(t, a, u) != 2 {
		t.Fatal("refused registration changed something")
	}
	codes, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, "Key", testPassword, current.Token, body)
	if err != nil || len(codes) != 10 {
		t.Fatalf("first factor = %d codes, %v", len(codes), err)
	}
	if !hasSession(t, a, current) || hasSession(t, a, elsewhere) {
		t.Fatal("want the current session kept and the other revoked")
	}
	// A second factor keeps the codes.
	if _, more := accountTOTP(t, a, u, current.Token); more != nil {
		t.Fatalf("second factor returned %d new codes", len(more))
	}
	if st, _ := a.MFAStatus(ctx, u.ID); st.RecoveryLeft != 10 || !st.TOTP || len(st.Keys) != 1 {
		t.Fatalf("status = %+v", st)
	}
}

func TestWebAuthnCredentialOfAnotherUserRejected(t *testing.T) {
	a, _ := newWebAuthnAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "key@example.org", RoleAdmin)
	other := mustUser(t, a, "other@example.org", RoleAdmin)
	key := registerKey(t, a, u, "Key")
	opts, token, err := a.BeginWebAuthnRegistration(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := key.Create(opts, waOrigin)
	if _, err := a.FinishWebAuthnRegistration(ctx, other.ID, token, "Same", testPassword, "", body); !errors.Is(err, ErrWebAuthnRegistered) || !errors.Is(err, ErrWebAuthnInvalid) {
		t.Fatalf("credential owned by another user = %v, want ErrWebAuthnRegistered", err)
	}
}

// Synced passkeys always report a zero counter: 0 then 0 is accepted.
func TestWebAuthnZeroCounterAccepted(t *testing.T) {
	a, _ := newWebAuthnAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "key@example.org", RoleAdmin)
	key := registerKey(t, a, u, "Passkey")
	for i := range 2 {
		req := startPending(t, a, "key@example.org", "192.0.2.1")
		body := assertion(t, a, key, req, waOrigin)
		if _, err := a.FinishWebAuthnLogin(ctx, req, body); err != nil {
			t.Fatalf("login %d with counter 0: %v", i+1, err)
		}
	}
}
