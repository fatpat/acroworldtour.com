package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"awt/internal/store/gen"
)

// Two-step login (spec 2026-10-19-manager-mfa): a correct password for a user
// with factors only opens a short pending login; a session is created when a
// factor succeeds.

// PendingCookieName carries the pending login token between the two steps.
const PendingCookieName = "awt_mfa"

const (
	pendingTTL       = 5 * time.Minute
	pendingPath      = "/manager/login"
	kindPendingLogin = "pending_login"
	mfaMaxFailures   = 5
	mfaLockDuration  = 15 * time.Minute
)

// Second-factor methods offered at login.
const (
	MethodTOTP     = "totp"
	MethodWebAuthn = "webauthn"
	MethodRecovery = "recovery"
)

var (
	// ErrMFARequired is returned by Login when the password was right but the
	// account has a second factor: use LoginStep1 to get the pending login.
	ErrMFARequired = errors.New("second factor required")
	// ErrMFAInvalid is a wrong, replayed or already used code.
	ErrMFAInvalid = errors.New("invalid second factor")
	// ErrPendingExpired means no live pending login backs the request.
	ErrPendingExpired = errors.New("pending login expired")
	// ErrTOTPUnavailable means the stored TOTP secret cannot be opened (an
	// AWT_MFA_KEY change) or the account has none; not counted as a failure.
	ErrTOTPUnavailable = errors.New("authenticator codes unavailable for this account")
)

// ErrMFALocked is returned while the account's factor step is locked.
type ErrMFALocked struct{ Until time.Time }

func (e *ErrMFALocked) Error() string {
	return fmt.Sprintf("second factor locked until %s", e.Until.Format(time.RFC3339))
}

// Pending is the result of a correct password for an account with factors.
// Token is only known here; the database keeps its SHA-256 hash.
type Pending struct {
	Token     string
	ExpiresAt time.Time
	Methods   []string
}

// PendingLogin describes the pending login behind a request.
type PendingLogin struct {
	UserID  string
	Email   string
	Methods []string
	// TOTPAvailable is false when the account has no TOTP or its secret
	// cannot be opened with the current AWT_MFA_KEY.
	TOTPAvailable bool
	// Next is where to go after the second step (already safeNext-checked).
	Next string
}

type pendingData struct {
	Next string `json:"next"`
}

// LoginStep1 checks the password. For an account without factors it opens a
// session (pending is nil); otherwise it opens a pending login and returns it
// with an empty session.
func (a *Auth) LoginStep1(ctx context.Context, email, password string) (Session, *Pending, error) {
	return a.loginStep1(ctx, email, password, "")
}

// loginStep1 is LoginStep1 keeping next (made safe) on the pending login.
func (a *Auth) loginStep1(ctx context.Context, email, password, next string) (Session, *Pending, error) {
	u, err := a.verifyPassword(ctx, email, password)
	if err != nil {
		return Session{}, nil, err
	}
	now := a.now().UTC()
	var (
		s Session
		p *Pending
	)
	err = a.db.Tx(ctx, func(q *gen.Queries) error {
		if err := q.DeleteExpiredLoginChallenges(ctx, now.Format(time.RFC3339)); err != nil {
			return fmt.Errorf("prune login challenges: %w", err)
		}
		methods, err := userMethods(ctx, q, u.ID)
		if err != nil {
			return err
		}
		if !hasFactor(methods) {
			s, err = a.insertSession(ctx, q, u, now)
			return err
		}
		token, err := randomToken()
		if err != nil {
			return err
		}
		data, err := json.Marshal(pendingData{Next: safeNext(next)})
		if err != nil {
			return err
		}
		p = &Pending{Token: token, ExpiresAt: now.Add(pendingTTL), Methods: methods}
		return q.InsertLoginChallenge(ctx, gen.InsertLoginChallengeParams{
			TokenHash: hashToken(token), UserID: u.ID, Kind: kindPendingLogin,
			Data: string(data), ExpiresAt: p.ExpiresAt.Format(time.RFC3339),
		})
	})
	if err != nil {
		return Session{}, nil, fmt.Errorf("login %s: %w", u.ID, err)
	}
	return s, p, nil
}

// userMethods lists the second-factor methods of an account, in display order.
func userMethods(ctx context.Context, q *gen.Queries, userID string) ([]string, error) {
	var methods []string
	if _, err := q.GetUserTOTP(ctx, userID); err == nil {
		methods = append(methods, MethodTOTP)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read totp: %w", err)
	}
	keys, err := q.ListUserWebAuthnCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read security keys: %w", err)
	}
	if len(keys) > 0 {
		methods = append(methods, MethodWebAuthn)
	}
	n, err := q.CountUnusedUserRecoveryCodes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count recovery codes: %w", err)
	}
	if n > 0 {
		methods = append(methods, MethodRecovery)
	}
	return methods, nil
}

// hasFactor: recovery codes only back up a real factor, they never turn MFA on.
func hasFactor(methods []string) bool {
	for _, m := range methods {
		if m == MethodTOTP || m == MethodWebAuthn {
			return true
		}
	}
	return false
}

// UserHasMFA reports whether the account has a TOTP or a security key.
func (a *Auth) UserHasMFA(ctx context.Context, userID string) (bool, error) {
	methods, err := userMethods(ctx, gen.New(a.db.Read), userID)
	if err != nil {
		return false, fmt.Errorf("user %s: %w", userID, err)
	}
	return hasFactor(methods), nil
}

// SetPendingCookie writes the pending login cookie, scoped to the login paths.
func (a *Auth) SetPendingCookie(w http.ResponseWriter, p Pending) {
	http.SetCookie(w, &http.Cookie{
		Name: PendingCookieName, Value: p.Token, Path: pendingPath,
		MaxAge: int(pendingTTL / time.Second), Expires: p.ExpiresAt,
		HttpOnly: true, Secure: a.cfg.Env != "dev", SameSite: http.SameSiteStrictMode,
	})
}

// ClearPendingCookie removes the pending login cookie.
func (a *Auth) ClearPendingCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: PendingCookieName, Value: "", Path: pendingPath, MaxAge: -1,
		HttpOnly: true, Secure: a.cfg.Env != "dev", SameSite: http.SameSiteStrictMode,
	})
}

// pendingChallenge returns the live pending login row behind r.
func (a *Auth) pendingChallenge(ctx context.Context, q *gen.Queries, r *http.Request, now time.Time) (gen.LoginChallenge, User, error) {
	c, err := r.Cookie(PendingCookieName)
	if err != nil || c.Value == "" {
		return gen.LoginChallenge{}, User{}, ErrPendingExpired
	}
	ch, err := q.GetLoginChallenge(ctx, gen.GetLoginChallengeParams{TokenHash: hashToken(c.Value), Kind: kindPendingLogin, ExpiresAt: now.Format(time.RFC3339)})
	if errors.Is(err, sql.ErrNoRows) {
		return gen.LoginChallenge{}, User{}, ErrPendingExpired
	}
	if err != nil {
		return gen.LoginChallenge{}, User{}, fmt.Errorf("read pending login: %w", err)
	}
	u, err := q.GetUser(ctx, ch.UserID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && u.Disabled) {
		return gen.LoginChallenge{}, User{}, ErrPendingExpired
	}
	if err != nil {
		return gen.LoginChallenge{}, User{}, fmt.Errorf("read user %s: %w", ch.UserID, err)
	}
	return ch, u, nil
}

// PendingFromRequest resolves the pending login cookie (ErrPendingExpired
// when there is none, it expired or was used).
func (a *Auth) PendingFromRequest(r *http.Request) (PendingLogin, error) {
	ctx := r.Context()
	q := gen.New(a.db.Read)
	ch, u, err := a.pendingChallenge(ctx, q, r, a.now().UTC())
	if err != nil {
		return PendingLogin{}, err
	}
	var data pendingData
	if err := json.Unmarshal([]byte(ch.Data), &data); err != nil {
		return PendingLogin{}, fmt.Errorf("pending login data: %w", err)
	}
	methods, err := userMethods(ctx, q, u.ID)
	if err != nil {
		return PendingLogin{}, err
	}
	p := PendingLogin{UserID: u.ID, Email: u.Email, Methods: methods, Next: safeNext(data.Next)}
	if row, err := q.GetUserTOTP(ctx, u.ID); err == nil {
		_, err := openSecret(a.cfg.MFAKey, PurposeTOTPSecret+u.ID, row.SecretEnc)
		p.TOTPAvailable = err == nil
		if err != nil {
			slog.Error("totp secret unreadable", "user_id", u.ID, "err", err)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return PendingLogin{}, fmt.Errorf("read totp: %w", err)
	}
	return p, nil
}

// CompleteWithTOTP finishes the pending login behind r with an authenticator
// code. The matched step is stored with a compare-and-set in the same
// transaction as the session, so a code is accepted once.
func (a *Auth) CompleteWithTOTP(ctx context.Context, r *http.Request, code string) (Session, error) {
	return a.completeFactor(ctx, r, MethodTOTP, func(q *gen.Queries, u User, now time.Time) (bool, error) {
		row, err := q.GetUserTOTP(ctx, u.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrTOTPUnavailable
		}
		if err != nil {
			return false, fmt.Errorf("read totp: %w", err)
		}
		secret, err := openSecret(a.cfg.MFAKey, PurposeTOTPSecret+u.ID, row.SecretEnc)
		if err != nil {
			slog.Error("totp secret unreadable", "user_id", u.ID, "err", err)
			return false, ErrTOTPUnavailable
		}
		step, ok := VerifyTOTP(secret, code, now, row.LastStep)
		if !ok {
			return false, nil
		}
		n, err := q.AdvanceUserTOTPStep(ctx, gen.AdvanceUserTOTPStepParams{LastStep: step, UserID: u.ID, LastStep_2: step})
		if err != nil {
			return false, fmt.Errorf("store totp step: %w", err)
		}
		return n == 1, nil // 0 rows: the step was consumed meanwhile (replay)
	})
}

// CompleteWithRecovery finishes the pending login behind r with a recovery
// code (consumed in the same transaction) and returns the codes left.
func (a *Auth) CompleteWithRecovery(ctx context.Context, r *http.Request, code string) (Session, int, error) {
	var left int64
	s, err := a.completeFactor(ctx, r, MethodRecovery, func(q *gen.Queries, u User, now time.Time) (bool, error) {
		n, err := q.UseUserRecoveryCode(ctx, gen.UseUserRecoveryCodeParams{
			UsedAt: sql.NullString{String: now.Format(time.RFC3339), Valid: true}, UserID: u.ID,
			CodeHash: HashRecoveryCode(NormaliseRecoveryCode(code)),
		})
		if err != nil {
			return false, fmt.Errorf("use recovery code: %w", err)
		}
		if n != 1 {
			return false, nil
		}
		left, err = q.CountUnusedUserRecoveryCodes(ctx, u.ID)
		return true, err
	})
	if err != nil {
		return Session{}, 0, err
	}
	slog.Info("recovery code used", "event", "mfa.recovery_used", "user_id", s.User.ID, "actor", s.User.ID, "ip", a.ClientIP(r), "codes_left", left)
	return s, int(left), nil
}

// completeFactor runs one factor attempt for the pending login behind r in a
// single transaction. check reports whether the factor is right; its error
// aborts without counting a failure. A failure is counted per account (not
// per pending login or IP, spec "Settled after builder check"): the fifth
// locks the factor step for 15 minutes. Success resets the counter, spends
// the pending login and opens the session.
func (a *Auth) completeFactor(ctx context.Context, r *http.Request, method string, check func(q *gen.Queries, u User, now time.Time) (bool, error)) (Session, error) {
	now := a.now().UTC()
	var (
		s       Session
		user    User
		outcome error
		locked  bool
	)
	err := a.db.Tx(ctx, func(q *gen.Queries) error {
		ch, u, err := a.pendingChallenge(ctx, q, r, now)
		if err != nil {
			return err
		}
		user = u
		if u.MfaLockedUntil.Valid {
			until, err := time.Parse(time.RFC3339, u.MfaLockedUntil.String)
			if err != nil {
				return fmt.Errorf("user %s lock time: %w", u.ID, err)
			}
			if now.Before(until) {
				return &ErrMFALocked{Until: until}
			}
			// The lock is over: count afresh, or one failure would re-lock.
			if err := q.ResetUserMFAFailures(ctx, u.ID); err != nil {
				return fmt.Errorf("reset factor failures: %w", err)
			}
		}
		ok, err := check(q, u, now)
		if err != nil {
			return err
		}
		if !ok {
			n, err := q.RecordMFAFailure(ctx, u.ID)
			if err != nil {
				return fmt.Errorf("record factor failure: %w", err)
			}
			outcome = ErrMFAInvalid
			if n >= mfaMaxFailures {
				until := now.Add(mfaLockDuration)
				if err := q.ResetUserMFAFailures(ctx, u.ID); err != nil {
					return fmt.Errorf("reset factor failures: %w", err)
				}
				if err := q.LockUserMFA(ctx, gen.LockUserMFAParams{MfaLockedUntil: sql.NullString{String: until.Format(time.RFC3339), Valid: true}, ID: u.ID}); err != nil {
					return fmt.Errorf("lock factor step: %w", err)
				}
				outcome, locked = &ErrMFALocked{Until: until}, true
			}
			return nil // commit the failure count
		}
		if err := q.ResetUserMFAFailures(ctx, u.ID); err != nil {
			return fmt.Errorf("reset factor failures: %w", err)
		}
		if err := q.DeleteLoginChallenge(ctx, ch.TokenHash); err != nil {
			return fmt.Errorf("spend pending login: %w", err)
		}
		s, err = a.insertSession(ctx, q, u, now)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	if outcome != nil {
		ip := a.ClientIP(r)
		slog.Info("second factor failed", "event", "mfa.failed", "user_id", user.ID, "actor", user.ID, "ip", ip, "method", method)
		if locked {
			slog.Info("second factor locked", "event", "mfa.locked", "user_id", user.ID, "actor", user.ID, "ip", ip)
		}
		return Session{}, outcome
	}
	return s, nil
}
