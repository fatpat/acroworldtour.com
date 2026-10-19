package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"awt/internal/auth/webauthntest"
)

const (
	waOrigin = "https://manager.example.org"
	waRPID   = "manager.example.org"
)

// newWebAuthnAuth is newMFAAuth served from https://manager.example.org.
func newWebAuthnAuth(t *testing.T) (*Auth, *mfaClock) {
	t.Helper()
	a, c := newMFAAuth(t)
	a.cfg.BaseURL = waOrigin
	a.cfg.WebAuthnRPID = waRPID
	return a, c
}

// registerKey runs the registration ceremony for u with a new software key.
func registerKey(t *testing.T, a *Auth, u User, name string) *webauthntest.Key {
	t.Helper()
	ctx := context.Background()
	key, err := webauthntest.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	opts, token, err := a.BeginWebAuthnRegistration(ctx, u.ID)
	if err != nil {
		t.Fatalf("begin registration: %v", err)
	}
	body, err := key.Create(opts, waOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, name, testPassword, "", body); err != nil {
		t.Fatalf("finish registration: %v", err)
	}
	return key
}

// assertion begins a login for the pending request and answers it with key
// from origin, at the key's current sign count.
func assertion(t *testing.T, a *Auth, key *webauthntest.Key, req *http.Request, origin string) []byte {
	t.Helper()
	opts, err := a.BeginWebAuthnLogin(context.Background(), req)
	if err != nil {
		t.Fatalf("begin login: %v", err)
	}
	body, err := key.Get(opts, origin)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestWebAuthnRegisterThenLogin(t *testing.T) {
	a, _ := newWebAuthnAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "key@example.org", RoleAdmin)

	opts, token, err := a.BeginWebAuthnRegistration(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	var creation struct {
		PublicKey struct {
			RP struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"rp"`
			User struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"user"`
			Timeout                int    `json:"timeout"`
			Attestation            string `json:"attestation"`
			AuthenticatorSelection struct {
				UserVerification string `json:"userVerification"`
				ResidentKey      string `json:"residentKey"`
			} `json:"authenticatorSelection"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(opts, &creation); err != nil {
		t.Fatal(err)
	}
	pk := creation.PublicKey
	if pk.RP.ID != waRPID || pk.RP.Name != "Acro World Tour Manager" || pk.User.Name != "key@example.org" ||
		pk.Attestation != "none" || pk.AuthenticatorSelection.UserVerification != "preferred" || pk.Timeout != 300000 {
		t.Fatalf("creation options = %s", opts)
	}
	if uid, _ := base64.RawURLEncoding.DecodeString(pk.User.ID); string(uid) != u.ID {
		t.Fatalf("user handle %q, want the user id (no email)", uid)
	}
	if token == "" || countRows(t, a, `SELECT count(*) FROM login_challenges WHERE user_id = ? AND kind = 'webauthn_register' AND token_hash = ?`, u.ID, hashToken(token)) != 1 {
		t.Fatal("registration challenge not stored by token hash")
	}
	key, err := webauthntest.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	body, err := key.Create(opts, waOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, "  YubiKey 5  ", testPassword, "", body); err != nil {
		t.Fatalf("finish registration: %v", err)
	}
	var name, credential string
	if err := a.db.Read.QueryRow(`SELECT name, credential FROM user_webauthn_credentials WHERE user_id = ? AND id = ?`, u.ID, base64.RawURLEncoding.EncodeToString(key.ID)).Scan(&name, &credential); err != nil {
		t.Fatalf("stored credential: %v", err)
	}
	if name != "YubiKey 5" || !strings.Contains(credential, `"publicKey"`) {
		t.Fatalf("stored name %q credential %s", name, credential)
	}
	if n := countRows(t, a, `SELECT count(*) FROM login_challenges WHERE user_id = ?`, u.ID); n != 0 {
		t.Fatalf("%d challenges left after registration", n)
	}
	if has, err := a.UserHasMFA(ctx, u.ID); err != nil || !has {
		t.Fatalf("UserHasMFA = %v, %v", has, err)
	}

	// Login: password, then the key.
	req := startPending(t, a, "key@example.org", "192.0.2.1")
	p, err := a.PendingFromRequest(req)
	// The key is the first factor: recovery codes came with it.
	if err != nil || strings.Join(p.Methods, ",") != MethodWebAuthn+","+MethodRecovery {
		t.Fatalf("pending = %+v, %v", p, err)
	}
	opts, err = a.BeginWebAuthnLogin(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		PublicKey struct {
			RPID             string `json:"rpId"`
			UserVerification string `json:"userVerification"`
			Timeout          int    `json:"timeout"`
			AllowCredentials []struct {
				ID string `json:"id"`
			} `json:"allowCredentials"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(opts, &request); err != nil {
		t.Fatal(err)
	}
	if r := request.PublicKey; r.RPID != waRPID || r.UserVerification != "preferred" || r.Timeout != 300000 ||
		len(r.AllowCredentials) != 1 || r.AllowCredentials[0].ID != base64.RawURLEncoding.EncodeToString(key.ID) {
		t.Fatalf("request options = %s", opts)
	}
	key.SignCount = 1
	body, err = key.Get(opts, waOrigin)
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.FinishWebAuthnLogin(ctx, req, body)
	if err != nil || s.Token == "" || s.User.ID != u.ID {
		t.Fatalf("finish login: %+v, %v", s, err)
	}
	if n := countRows(t, a, `SELECT count(*) FROM login_challenges WHERE user_id = ?`, u.ID); n != 0 {
		t.Fatal("pending login not spent")
	}
	if n := countRows(t, a, `SELECT count(*) FROM user_webauthn_credentials WHERE user_id = ? AND last_used_at IS NOT NULL AND json_extract(credential, '$.authenticator.signCount') = 1`, u.ID); n != 1 {
		t.Fatal("last_used_at or sign count not updated")
	}
}

func TestWebAuthnRegistrationRules(t *testing.T) {
	a, c := newWebAuthnAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "key@example.org", RoleAdmin)
	other := mustUser(t, a, "other@example.org", RoleScorer)
	key, err := webauthntest.NewKey()
	if err != nil {
		t.Fatal(err)
	}
	stored := func() int {
		return countRows(t, a, `SELECT count(*) FROM user_webauthn_credentials`)
	}

	// Collected at another origin.
	opts, token, err := a.BeginWebAuthnRegistration(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := key.Create(opts, "https://evil.example")
	if _, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, "Key", testPassword, "", body); !errors.Is(err, ErrWebAuthnInvalid) {
		t.Fatalf("wrong origin = %v, want ErrWebAuthnInvalid", err)
	}
	// The challenge is single use, even after a failure.
	body, _ = key.Create(opts, waOrigin)
	if _, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, "Key", testPassword, "", body); !errors.Is(err, ErrWebAuthnChallenge) {
		t.Fatalf("reused challenge = %v, want ErrWebAuthnChallenge", err)
	}
	// Another user's challenge.
	opts, token, _ = a.BeginWebAuthnRegistration(ctx, u.ID)
	body, _ = key.Create(opts, waOrigin)
	if _, err := a.FinishWebAuthnRegistration(ctx, other.ID, token, "Key", testPassword, "", body); !errors.Is(err, ErrWebAuthnChallenge) {
		t.Fatalf("other user's challenge = %v", err)
	}
	// Expired after 5 minutes.
	opts, token, _ = a.BeginWebAuthnRegistration(ctx, u.ID)
	body, _ = key.Create(opts, waOrigin)
	c.advance(5*time.Minute + time.Second)
	if _, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, "Key", testPassword, "", body); !errors.Is(err, ErrWebAuthnChallenge) {
		t.Fatalf("expired challenge = %v", err)
	}
	// A name is required (checked before the challenge is spent).
	opts, token, _ = a.BeginWebAuthnRegistration(ctx, u.ID)
	body, _ = key.Create(opts, waOrigin)
	var in *InputError
	for _, name := range []string{"  ", strings.Repeat("x", 65)} {
		if _, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, name, testPassword, "", body); !errors.As(err, &in) || in.Field != "name" {
			t.Fatalf("name %q = %v, want an InputError on name", name, err)
		}
	}
	if stored() != 0 {
		t.Fatal("a refused registration stored a credential")
	}
	if _, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, "Key", testPassword, "", body); err != nil {
		t.Fatalf("valid registration: %v", err)
	}

	// Registered credentials are excluded from the next registration, and
	// registering the same credential again is refused.
	opts, token, _ = a.BeginWebAuthnRegistration(ctx, u.ID)
	var creation struct {
		PublicKey struct {
			Exclude []struct {
				ID string `json:"id"`
			} `json:"excludeCredentials"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(opts, &creation); err != nil {
		t.Fatal(err)
	}
	if len(creation.PublicKey.Exclude) != 1 || creation.PublicKey.Exclude[0].ID != base64.RawURLEncoding.EncodeToString(key.ID) {
		t.Fatalf("excludeCredentials = %+v", creation.PublicKey.Exclude)
	}
	body, _ = key.Create(opts, waOrigin)
	if _, err := a.FinishWebAuthnRegistration(ctx, u.ID, token, "Again", testPassword, "", body); !errors.Is(err, ErrWebAuthnInvalid) {
		t.Fatalf("duplicate credential = %v", err)
	}
	if stored() != 1 {
		t.Fatalf("%d credentials stored, want 1", stored())
	}
}

func TestWebAuthnLoginWrongOrigin(t *testing.T) {
	a, _ := newWebAuthnAuth(t)
	u := mustUser(t, a, "key@example.org", RoleAdmin)
	key := registerKey(t, a, u, "Key")
	req := startPending(t, a, "key@example.org", "192.0.2.1")
	key.SignCount = 1
	body := assertion(t, a, key, req, "https://manager.example.org.evil.example")
	if _, err := a.FinishWebAuthnLogin(context.Background(), req, body); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("wrong origin = %v, want ErrMFAInvalid", err)
	}
	if n := countRows(t, a, `SELECT mfa_failed_count FROM users WHERE id = ?`, u.ID); n != 1 {
		t.Fatalf("failure count = %d, want 1", n)
	}
	if n := countRows(t, a, `SELECT count(*) FROM sessions WHERE user_id = ?`, u.ID); n != 0 {
		t.Fatal("session opened")
	}
}

func TestWebAuthnReplayRejected(t *testing.T) {
	a, _ := newWebAuthnAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "key@example.org", RoleAdmin)
	key := registerKey(t, a, u, "Key")

	key.SignCount = 5
	req := startPending(t, a, "key@example.org", "192.0.2.1")
	body := assertion(t, a, key, req, waOrigin)
	if _, err := a.FinishWebAuthnLogin(ctx, req, body); err != nil {
		t.Fatalf("first login: %v", err)
	}

	// The same assertion again, on a fresh pending login with a new
	// challenge: it answers the old challenge and is refused.
	req = startPending(t, a, "key@example.org", "192.0.2.1")
	if _, err := a.BeginWebAuthnLogin(ctx, req); err != nil {
		t.Fatal(err)
	}
	if _, err := a.FinishWebAuthnLogin(ctx, req, body); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("replayed assertion = %v, want ErrMFAInvalid", err)
	}
	// Any attempt spends the challenge: after a rejected answer, a valid
	// answer to the same challenge is refused too.
	key.SignCount = 6
	opts, err := a.BeginWebAuthnLogin(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	bad, _ := key.Get(opts, "https://evil.example")
	good, _ := key.Get(opts, waOrigin)
	if _, err := a.FinishWebAuthnLogin(ctx, req, bad); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("wrong origin = %v", err)
	}
	if _, err := a.FinishWebAuthnLogin(ctx, req, good); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("second answer to a spent challenge = %v, want ErrMFAInvalid", err)
	}
	body = assertion(t, a, key, req, waOrigin)
	if _, err := a.FinishWebAuthnLogin(ctx, req, body); err != nil {
		t.Fatalf("second login: %v", err)
	}
	// A signature counter that did not move (a cloned key) is refused.
	req = startPending(t, a, "key@example.org", "192.0.2.1")
	body = assertion(t, a, key, req, waOrigin)
	if _, err := a.FinishWebAuthnLogin(ctx, req, body); !errors.Is(err, ErrMFAInvalid) {
		t.Fatalf("same sign count = %v, want ErrMFAInvalid", err)
	}
	if n := countRows(t, a, `SELECT count(*) FROM user_webauthn_credentials WHERE json_extract(credential, '$.authenticator.signCount') = 6`); n != 1 {
		t.Fatal("stored sign count moved on a refused assertion")
	}
}

func TestWebAuthnFailuresCountTowardLockout(t *testing.T) {
	a, c := newWebAuthnAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "key@example.org", RoleAdmin)
	secret := enrolTOTP(t, a, u)
	key := registerKey(t, a, u, "Key")

	req := startPending(t, a, "key@example.org", "192.0.2.1")
	for i := 1; i <= 4; i++ {
		if _, err := a.CompleteWithTOTP(ctx, req, wrongCode(t, secret, c.now())); !errors.Is(err, ErrMFAInvalid) {
			t.Fatalf("totp attempt %d = %v", i, err)
		}
	}
	key.SignCount = 1
	body := assertion(t, a, key, req, "https://evil.example")
	var locked *ErrMFALocked
	if _, err := a.FinishWebAuthnLogin(ctx, req, body); !errors.As(err, &locked) {
		t.Fatalf("5th failure (security key) = %v, want ErrMFALocked", err)
	}
	// Locked: no ceremony starts, and a valid assertion prepared before is refused.
	req = startPending(t, a, "key@example.org", "198.51.100.7")
	if _, err := a.BeginWebAuthnLogin(ctx, req); !errors.As(err, &locked) {
		t.Fatalf("begin while locked = %v, want ErrMFALocked", err)
	}
	if _, err := a.FinishWebAuthnLogin(ctx, req, body); !errors.As(err, &locked) {
		t.Fatalf("finish while locked = %v, want ErrMFALocked", err)
	}
	// After the lock, a valid key logs in and resets the counter.
	c.advance(15*time.Minute + time.Second)
	req = startPending(t, a, "key@example.org", "198.51.100.7")
	key.SignCount = 2
	body = assertion(t, a, key, req, waOrigin)
	if s, err := a.FinishWebAuthnLogin(ctx, req, body); err != nil || s.User.ID != u.ID {
		t.Fatalf("after lock: %v", err)
	}
	if n := countRows(t, a, `SELECT mfa_failed_count FROM users WHERE id = ?`, u.ID); n != 0 {
		t.Fatalf("failure count = %d after success", n)
	}
}

func TestWebAuthnLoginNeedsLivePending(t *testing.T) {
	a, c := newWebAuthnAuth(t)
	ctx := context.Background()
	u := mustUser(t, a, "key@example.org", RoleAdmin)
	key := registerKey(t, a, u, "Key")
	req := startPending(t, a, "key@example.org", "192.0.2.1")
	key.SignCount = 1
	body := assertion(t, a, key, req, waOrigin)
	c.advance(5*time.Minute + time.Second)
	if _, err := a.FinishWebAuthnLogin(ctx, req, body); !errors.Is(err, ErrPendingExpired) {
		t.Fatalf("expired pending = %v, want ErrPendingExpired", err)
	}
	if _, err := a.BeginWebAuthnLogin(ctx, req); !errors.Is(err, ErrPendingExpired) {
		t.Fatalf("begin on expired pending = %v", err)
	}

	// An account whose only factor is TOTP has no key to ask for.
	v := mustUser(t, a, "app@example.org", RoleAdmin)
	enrolTOTP(t, a, v)
	req = startPending(t, a, "app@example.org", "192.0.2.1")
	if _, err := a.BeginWebAuthnLogin(ctx, req); !errors.Is(err, ErrNoSecurityKey) {
		t.Fatalf("begin without keys = %v, want ErrNoSecurityKey", err)
	}
}
