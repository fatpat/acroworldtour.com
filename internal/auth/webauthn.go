package auth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"awt/internal/store/gen"
)

// Security keys and passkeys as a second factor (spec 2026-10-19-manager-mfa,
// decision 0014). Registration challenges live in login_challenges
// (webauthn_register); a login challenge rides on the pending login row, so
// it never outlives it.

const (
	rpDisplayName        = "Acro World Tour Manager"
	kindWebAuthnRegister = "webauthn_register"
	webauthnTTL          = 5 * time.Minute
	maxKeyNameLen        = 64
)

var (
	// ErrWebAuthnChallenge means the registration challenge is unknown,
	// expired, already used or belongs to another user.
	ErrWebAuthnChallenge = errors.New("security key challenge expired")
	// ErrWebAuthnInvalid is a registration response that does not verify, or
	// a credential the user already registered.
	ErrWebAuthnInvalid = errors.New("security key response rejected")
	// ErrNoSecurityKey: the pending login's account has no security key.
	ErrNoSecurityKey = errors.New("no security key registered")
)

// webauthnUser adapts an account to webauthn.User. The user handle is the
// opaque user id, never the email (WebAuthn §14.6.1).
type webauthnUser struct {
	u     User
	creds []webauthn.Credential
}

func (w webauthnUser) WebAuthnID() []byte                         { return []byte(w.u.ID) }
func (w webauthnUser) WebAuthnName() string                       { return w.u.Email }
func (w webauthnUser) WebAuthnDisplayName() string                { return w.u.Name }
func (w webauthnUser) WebAuthnCredentials() []webauthn.Credential { return w.creds }

// pendingWebAuthnData is the pending login data with the login ceremony's
// session data while one is open.
type pendingWebAuthnData struct {
	pendingData
	WebAuthn *webauthn.SessionData `json:"webauthn,omitempty"`
}

// relyingParty is the WebAuthn relying party: RP ID from AWT_WEBAUTHN_RP_ID
// (default host of AWT_BASE_URL), origins from config (never the request
// Host). Expiry is enforced by login_challenges with the Auth clock, so the
// library's wall-clock enforcement stays off.
func (a *Auth) relyingParty() (*webauthn.WebAuthn, error) {
	timeout := webauthn.TimeoutConfig{Timeout: webauthnTTL, TimeoutUVD: webauthnTTL}
	rp, err := webauthn.New(&webauthn.Config{
		RPID:                  a.cfg.WebAuthnRPID,
		RPDisplayName:         rpDisplayName,
		RPOrigins:             a.cfg.Origins(),
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementDiscouraged,
			UserVerification: protocol.VerificationPreferred,
		},
		Timeouts: webauthn.TimeoutsConfig{Login: timeout, Registration: timeout},
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn relying party: %w", err)
	}
	return rp, nil
}

func credentialID(id []byte) string { return base64.RawURLEncoding.EncodeToString(id) }

// userCredentials loads the user's security keys as library credentials.
func userCredentials(ctx context.Context, q *gen.Queries, userID string) ([]webauthn.Credential, error) {
	rows, err := q.ListUserWebAuthnCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read security keys: %w", err)
	}
	creds := make([]webauthn.Credential, 0, len(rows))
	for _, row := range rows {
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(row.Credential), &c); err != nil {
			return nil, fmt.Errorf("security key %s: %w", row.ID, err)
		}
		creds = append(creds, c)
	}
	return creds, nil
}

// BeginWebAuthnRegistration opens a registration ceremony for userID: it
// returns the creation options for navigator.credentials.create and a token
// naming the stored challenge (5 minutes, single use). The user's keys are
// excluded so the same authenticator is not registered twice.
func (a *Auth) BeginWebAuthnRegistration(ctx context.Context, userID string) ([]byte, string, error) {
	rp, err := a.relyingParty()
	if err != nil {
		return nil, "", err
	}
	now := a.now().UTC()
	var (
		opts  []byte
		token string
	)
	err = a.db.Tx(ctx, func(q *gen.Queries) error {
		u, err := q.GetUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("read user: %w", err)
		}
		creds, err := userCredentials(ctx, q, u.ID)
		if err != nil {
			return err
		}
		exclude := webauthn.Credentials(creds).CredentialDescriptors()
		creation, session, err := rp.BeginRegistration(webauthnUser{u: u, creds: creds}, webauthn.WithExclusions(exclude))
		if err != nil {
			return fmt.Errorf("begin registration: %w", err)
		}
		if opts, err = json.Marshal(creation); err != nil {
			return err
		}
		data, err := json.Marshal(session)
		if err != nil {
			return err
		}
		if token, err = randomToken(); err != nil {
			return err
		}
		if err := q.DeleteExpiredLoginChallenges(ctx, now.Format(time.RFC3339)); err != nil {
			return fmt.Errorf("prune login challenges: %w", err)
		}
		return q.InsertLoginChallenge(ctx, gen.InsertLoginChallengeParams{
			TokenHash: hashToken(token), UserID: u.ID, Kind: kindWebAuthnRegister,
			Data: string(data), ExpiresAt: now.Add(webauthnTTL).Format(time.RFC3339),
		})
	})
	if err != nil {
		return nil, "", fmt.Errorf("security key registration for %s: %w", userID, err)
	}
	return opts, token, nil
}

// FinishWebAuthnRegistration verifies the authenticator's response (body,
// the PublicKeyCredential JSON) to the challenge behind challengeToken and
// stores the key under name. The challenge is spent whatever the outcome.
func (a *Auth) FinishWebAuthnRegistration(ctx context.Context, userID, challengeToken, name string, body []byte) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxKeyNameLen {
		return &InputError{Field: "name", Msg: fmt.Sprintf("give the security key a name of at most %d characters", maxKeyNameLen)}
	}
	rp, err := a.relyingParty()
	if err != nil {
		return err
	}
	now := a.now().UTC()
	var outcome error
	err = a.db.Tx(ctx, func(q *gen.Queries) error {
		ch, err := q.GetLoginChallenge(ctx, gen.GetLoginChallengeParams{TokenHash: hashToken(challengeToken), Kind: kindWebAuthnRegister, ExpiresAt: now.Format(time.RFC3339)})
		if errors.Is(err, sql.ErrNoRows) || (err == nil && ch.UserID != userID) {
			outcome = ErrWebAuthnChallenge
			return nil
		}
		if err != nil {
			return fmt.Errorf("read challenge: %w", err)
		}
		if err := q.DeleteLoginChallenge(ctx, ch.TokenHash); err != nil {
			return fmt.Errorf("spend challenge: %w", err)
		}
		var session webauthn.SessionData
		if err := json.Unmarshal([]byte(ch.Data), &session); err != nil {
			return fmt.Errorf("challenge data: %w", err)
		}
		u, err := q.GetUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("read user: %w", err)
		}
		creds, err := userCredentials(ctx, q, u.ID)
		if err != nil {
			return err
		}
		parsed, err := protocol.ParseCredentialCreationResponseBytes(body)
		if err != nil {
			outcome = fmt.Errorf("%w: %w", ErrWebAuthnInvalid, err)
			return nil
		}
		cred, err := rp.CreateCredential(webauthnUser{u: u, creds: creds}, session, parsed)
		if err != nil {
			outcome = fmt.Errorf("%w: %w", ErrWebAuthnInvalid, err)
			return nil
		}
		for _, c := range creds {
			if credentialID(c.ID) == credentialID(cred.ID) {
				outcome = fmt.Errorf("%w: already registered", ErrWebAuthnInvalid)
				return nil
			}
		}
		stored, err := json.Marshal(cred)
		if err != nil {
			return err
		}
		return q.InsertUserWebAuthnCredential(ctx, gen.InsertUserWebAuthnCredentialParams{
			ID: credentialID(cred.ID), UserID: u.ID, Name: name, Credential: string(stored), CreatedAt: now.Format(time.RFC3339),
		})
	})
	if err != nil {
		return fmt.Errorf("security key registration for %s: %w", userID, err)
	}
	return outcome
}

// BeginWebAuthnLogin opens the login ceremony for the pending login behind
// r and returns the request options for navigator.credentials.get. A new
// call replaces the previous challenge.
func (a *Auth) BeginWebAuthnLogin(ctx context.Context, r *http.Request) ([]byte, error) {
	rp, err := a.relyingParty()
	if err != nil {
		return nil, err
	}
	now := a.now().UTC()
	var opts []byte
	err = a.db.Tx(ctx, func(q *gen.Queries) error {
		ch, u, err := a.pendingChallenge(ctx, q, r, now)
		if err != nil {
			return err
		}
		if u.MfaLockedUntil.Valid {
			until, err := time.Parse(time.RFC3339, u.MfaLockedUntil.String)
			if err != nil {
				return fmt.Errorf("user %s lock time: %w", u.ID, err)
			}
			if now.Before(until) {
				return &ErrMFALocked{Until: until}
			}
		}
		creds, err := userCredentials(ctx, q, u.ID)
		if err != nil {
			return err
		}
		if len(creds) == 0 {
			return ErrNoSecurityKey
		}
		assertion, session, err := rp.BeginLogin(webauthnUser{u: u, creds: creds})
		if err != nil {
			return fmt.Errorf("begin login: %w", err)
		}
		var data pendingWebAuthnData
		if err := json.Unmarshal([]byte(ch.Data), &data); err != nil {
			return fmt.Errorf("pending login data: %w", err)
		}
		data.WebAuthn = session
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if err := q.UpdateLoginChallengeData(ctx, gen.UpdateLoginChallengeDataParams{Data: string(raw), TokenHash: ch.TokenHash}); err != nil {
			return fmt.Errorf("store login challenge: %w", err)
		}
		opts, err = json.Marshal(assertion)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("security key login: %w", err)
	}
	return opts, nil
}

// FinishWebAuthnLogin finishes the pending login behind r with the
// authenticator's assertion (body, the PublicKeyCredential JSON). Same rules
// as the other factors (completeFactor): a rejected assertion counts toward
// the account lockout. The challenge is spent by any attempt; a signature
// counter that did not move (a cloned authenticator) is refused. Success
// stores the new counter and last use.
func (a *Auth) FinishWebAuthnLogin(ctx context.Context, r *http.Request, body []byte) (Session, error) {
	rp, err := a.relyingParty()
	if err != nil {
		return Session{}, err
	}
	return a.completeFactor(ctx, r, MethodWebAuthn, func(q *gen.Queries, u User, now time.Time) (bool, error) {
		ch, _, err := a.pendingChallenge(ctx, q, r, now)
		if err != nil {
			return false, err
		}
		var data pendingWebAuthnData
		if err := json.Unmarshal([]byte(ch.Data), &data); err != nil {
			return false, fmt.Errorf("pending login data: %w", err)
		}
		session := data.WebAuthn
		if session == nil {
			return false, nil
		}
		data.WebAuthn = nil
		raw, err := json.Marshal(data)
		if err != nil {
			return false, err
		}
		if err := q.UpdateLoginChallengeData(ctx, gen.UpdateLoginChallengeDataParams{Data: string(raw), TokenHash: ch.TokenHash}); err != nil {
			return false, fmt.Errorf("spend login challenge: %w", err)
		}
		creds, err := userCredentials(ctx, q, u.ID)
		if err != nil {
			return false, err
		}
		parsed, err := protocol.ParseCredentialRequestResponseBytes(body)
		if err != nil {
			slog.Info("security key assertion rejected", "user_id", u.ID, "err", err)
			return false, nil
		}
		cred, err := rp.ValidateLogin(webauthnUser{u: u, creds: creds}, *session, parsed)
		if err != nil {
			slog.Info("security key assertion rejected", "user_id", u.ID, "err", err)
			return false, nil
		}
		if cred.Authenticator.CloneWarning {
			slog.Warn("security key signature counter did not increase", "user_id", u.ID, "key_id", credentialID(cred.ID))
			return false, nil
		}
		stored, err := json.Marshal(cred)
		if err != nil {
			return false, err
		}
		err = q.UpdateUserWebAuthnCredential(ctx, gen.UpdateUserWebAuthnCredentialParams{
			Credential: string(stored), LastUsedAt: sql.NullString{String: now.Format(time.RFC3339), Valid: true},
			ID: credentialID(cred.ID), UserID: u.ID,
		})
		if err != nil {
			return false, fmt.Errorf("update security key: %w", err)
		}
		return true, nil
	})
}
