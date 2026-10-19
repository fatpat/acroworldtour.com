package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"awt/internal/store/gen"
)

// Account page actions on one's own factors and password (spec
// 2026-10-19-manager-mfa, "Settled after builder check"): each re-checks
// the current password and, in the same transaction, revokes the user's
// other sessions.

const totpEnrollTTL = 10 * time.Minute

var (
	// ErrWrongPassword: the current password given with a sensitive action
	// is wrong.
	ErrWrongPassword = errors.New("current password is wrong")
	// ErrEnrollmentExpired: the pending authenticator setup is damaged,
	// belongs to another user or is older than 10 minutes.
	ErrEnrollmentExpired = errors.New("authenticator setup expired")
	// ErrTOTPEnrolled: the account already has an authenticator app.
	ErrTOTPEnrolled = errors.New("an authenticator app is already set up")
	// ErrNoTOTP: the account has no authenticator app to remove.
	ErrNoTOTP = errors.New("no authenticator app is set up")
	// ErrNoMFAKey: the server has no AWT_MFA_KEY, so no TOTP secret can be
	// sealed: authenticator apps are unavailable.
	ErrNoMFAKey = errors.New("authenticator apps are not available: the server has no MFA key configured")
	// ErrKeyNotFound: the account has no security key with that id.
	ErrKeyNotFound = errors.New("security key not found")
	// ErrNoFactor: recovery codes only back up a factor; there is none.
	ErrNoFactor = errors.New("no second factor is set up")
	// ErrPasswordChanged: the password changed between its check and the
	// action (a concurrent change); nothing was done.
	ErrPasswordChanged = errors.New("password changed during the action")
)

// ErrReauthLimited is returned once a user gave 5 wrong current passwords
// within 15 minutes; RetryAfter is when one attempt is available again.
type ErrReauthLimited struct{ RetryAfter time.Duration }

func (e *ErrReauthLimited) Error() string {
	return fmt.Sprintf("too many wrong passwords, retry in %s", e.RetryAfter.Round(time.Second))
}

// KeyInfo describes a registered security key. LastUsedAt is zero when the
// key never logged in.
type KeyInfo struct {
	ID, Name   string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

// Status is what the account page shows of a user's second factors.
type Status struct {
	TOTP         bool
	Keys         []KeyInfo
	RecoveryLeft int
}

// MFAStatus returns the user's factors and unused recovery code count.
func (a *Auth) MFAStatus(ctx context.Context, userID string) (Status, error) {
	q := gen.New(a.db.Read)
	var st Status
	if _, err := q.GetUserTOTP(ctx, userID); err == nil {
		st.TOTP = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Status{}, fmt.Errorf("user %s totp: %w", userID, err)
	}
	rows, err := q.ListUserWebAuthnCredentials(ctx, userID)
	if err != nil {
		return Status{}, fmt.Errorf("user %s security keys: %w", userID, err)
	}
	for _, row := range rows {
		k := KeyInfo{ID: row.ID, Name: row.Name}
		if k.CreatedAt, err = time.Parse(time.RFC3339, row.CreatedAt); err != nil {
			return Status{}, fmt.Errorf("security key %s created_at: %w", row.ID, err)
		}
		if row.LastUsedAt.Valid {
			if k.LastUsedAt, err = time.Parse(time.RFC3339, row.LastUsedAt.String); err != nil {
				return Status{}, fmt.Errorf("security key %s last_used_at: %w", row.ID, err)
			}
		}
		st.Keys = append(st.Keys, k)
	}
	n, err := q.CountUnusedUserRecoveryCodes(ctx, userID)
	if err != nil {
		return Status{}, fmt.Errorf("user %s recovery codes: %w", userID, err)
	}
	st.RecoveryLeft = int(n)
	return st, nil
}

// StartTOTPEnrollment draws a secret for userID and returns it with its
// otpauth URI and sealedPending, the secret sealed for this user with a
// 10-minute expiry, to carry in the confirmation form. Nothing is stored.
func (a *Auth) StartTOTPEnrollment(ctx context.Context, userID string) (secret, uri, sealedPending string, err error) {
	if len(a.cfg.MFAKey) == 0 {
		return "", "", "", ErrNoMFAKey
	}
	u, err := gen.New(a.db.Read).GetUser(ctx, userID)
	if err != nil {
		return "", "", "", fmt.Errorf("totp enrolment for %s: %w", userID, err)
	}
	if secret, err = NewTOTPSecret(); err != nil {
		return "", "", "", err
	}
	expiry := a.now().Add(totpEnrollTTL).Unix()
	sealedPending, err = sealSecret(a.cfg.MFAKey, PurposeTOTPEnroll+userID, userID+"|"+secret+"|"+strconv.FormatInt(expiry, 10))
	if err != nil {
		return "", "", "", fmt.Errorf("totp enrolment for %s: %w", userID, err)
	}
	return secret, TOTPURI(secret, u.Email), sealedPending, nil
}

// openEnrollment returns the secret inside sealedPending when it was sealed
// for userID and has not expired.
func (a *Auth) openEnrollment(userID, sealedPending string) (string, error) {
	plain, err := openSecret(a.cfg.MFAKey, PurposeTOTPEnroll+userID, sealedPending)
	if err != nil {
		return "", ErrEnrollmentExpired
	}
	parts := strings.Split(plain, "|")
	if len(parts) != 3 || parts[0] != userID {
		return "", ErrEnrollmentExpired
	}
	expiry, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || !a.now().Before(time.Unix(expiry, 0)) {
		return "", ErrEnrollmentExpired
	}
	return parts[1], nil
}

// TOTPEnrollment reopens a pending setup (for the QR code and to show the
// form again): the secret and its otpauth URI.
func (a *Auth) TOTPEnrollment(ctx context.Context, userID, sealedPending string) (secret, uri string, err error) {
	if secret, err = a.openEnrollment(userID, sealedPending); err != nil {
		return "", "", err
	}
	u, err := gen.New(a.db.Read).GetUser(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("totp enrolment for %s: %w", userID, err)
	}
	return secret, TOTPURI(secret, u.Email), nil
}

// reauthenticate checks the current password of userID, outside any
// transaction (bcrypt is slow), within the per-user limit of wrong
// passwords: the limit is consulted first, a wrong password counts, a
// right one resets it.
func (a *Auth) reauthenticate(ctx context.Context, userID, currentPassword string) (User, error) {
	key := "reauth|" + userID
	if !a.reauth.Attempt(key) {
		return User{}, &ErrReauthLimited{RetryAfter: a.reauth.RetryAfter(key)}
	}
	u, err := gen.New(a.db.Read).GetUser(ctx, userID)
	if err != nil {
		return User{}, fmt.Errorf("user %s: %w", userID, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)) != nil {
		return User{}, ErrWrongPassword
	}
	a.reauth.Reset(key)
	return u, nil
}

// sensitive runs a sensitive account action: it checks currentPassword
// (reauthenticate), then in one transaction requires the password to be
// unchanged since, runs fn and revokes the user's sessions other than
// keepSession (a session token).
func (a *Auth) sensitive(ctx context.Context, userID, currentPassword, keepSession string, fn func(q *gen.Queries, u User, now time.Time) (bool, error)) error {
	u, err := a.reauthenticate(ctx, userID, currentPassword)
	if err != nil {
		return err
	}
	return a.sensitiveTx(ctx, u, keepSession, fn)
}

// sensitiveTx is the transaction of a sensitive action for u, whose
// password was just verified. fn reports whether it changed anything; when
// it did not (a spent challenge, say) the sessions are left alone and the
// transaction still commits. An error from fn rolls everything back.
func (a *Auth) sensitiveTx(ctx context.Context, verified User, keepSession string, fn func(q *gen.Queries, u User, now time.Time) (bool, error)) error {
	if a.beforeSensitiveTx != nil {
		a.beforeSensitiveTx()
	}
	now := a.now().UTC()
	return a.db.Tx(ctx, func(q *gen.Queries) error {
		u, err := q.GetUser(ctx, verified.ID)
		if err != nil {
			return fmt.Errorf("user %s: %w", verified.ID, err)
		}
		if u.PasswordHash != verified.PasswordHash {
			return ErrPasswordChanged
		}
		changed, err := fn(q, u, now)
		if err != nil || !changed {
			return err
		}
		if err := q.DeleteUserSessionsExcept(ctx, gen.DeleteUserSessionsExceptParams{UserID: u.ID, TokenHash: hashToken(keepSession)}); err != nil {
			return fmt.Errorf("revoke other sessions: %w", err)
		}
		return nil
	})
}

// replaceRecoveryCodes stores 10 fresh recovery codes for userID in place of
// the old ones and returns them for display (shown once).
func replaceRecoveryCodes(ctx context.Context, q *gen.Queries, userID string) ([]string, error) {
	display, hashes, err := NewRecoveryCodes()
	if err != nil {
		return nil, err
	}
	if err := q.DeleteUserRecoveryCodes(ctx, userID); err != nil {
		return nil, fmt.Errorf("delete recovery codes: %w", err)
	}
	for _, h := range hashes {
		if err := q.InsertUserRecoveryCode(ctx, gen.InsertUserRecoveryCodeParams{UserID: userID, CodeHash: h}); err != nil {
			return nil, fmt.Errorf("store recovery code: %w", err)
		}
	}
	return display, nil
}

// firstFactorCodes gives the account recovery codes when the factor just
// added is its first one (hadFactor false); otherwise nil.
func firstFactorCodes(ctx context.Context, q *gen.Queries, userID string, hadFactor bool) ([]string, error) {
	if hadFactor {
		return nil, nil
	}
	return replaceRecoveryCodes(ctx, q, userID)
}

// dropCodesIfNoFactor deletes the recovery codes once the last factor is
// gone: MFA is off for the account.
func dropCodesIfNoFactor(ctx context.Context, q *gen.Queries, userID string) error {
	methods, err := userMethods(ctx, q, userID)
	if err != nil {
		return err
	}
	if hasFactor(methods) {
		return nil
	}
	if err := q.DeleteUserRecoveryCodes(ctx, userID); err != nil {
		return fmt.Errorf("delete recovery codes: %w", err)
	}
	return nil
}

// ConfirmTOTP stores the authenticator app behind sealedPending once code
// matches it. The matched step is recorded, so the same code cannot log in
// afterwards. It returns the recovery codes when this is the account's
// first factor (nil otherwise).
func (a *Auth) ConfirmTOTP(ctx context.Context, userID, sealedPending, code, currentPassword, keepSession string) ([]string, error) {
	secret, err := a.openEnrollment(userID, sealedPending)
	if err != nil {
		return nil, err
	}
	var codes []string
	err = a.sensitive(ctx, userID, currentPassword, keepSession, func(q *gen.Queries, u User, now time.Time) (bool, error) {
		methods, err := userMethods(ctx, q, u.ID)
		if err != nil {
			return false, err
		}
		if slices.Contains(methods, MethodTOTP) {
			return false, ErrTOTPEnrolled
		}
		step, ok := VerifyTOTP(secret, code, now, 0)
		if !ok {
			return false, ErrMFAInvalid
		}
		sealed, err := sealSecret(a.cfg.MFAKey, PurposeTOTPSecret+u.ID, secret)
		if err != nil {
			return false, err
		}
		if err := q.UpsertUserTOTP(ctx, gen.UpsertUserTOTPParams{UserID: u.ID, SecretEnc: sealed, CreatedAt: now.Format(time.RFC3339)}); err != nil {
			return false, fmt.Errorf("store totp: %w", err)
		}
		if _, err := q.AdvanceUserTOTPStep(ctx, gen.AdvanceUserTOTPStepParams{LastStep: step, UserID: u.ID, LastStep_2: step}); err != nil {
			return false, fmt.Errorf("store totp step: %w", err)
		}
		codes, err = firstFactorCodes(ctx, q, u.ID, hasFactor(methods))
		return true, err
	})
	if err != nil {
		return nil, fmt.Errorf("confirm totp for %s: %w", userID, err)
	}
	return codes, nil
}

// RemoveTOTP removes the user's authenticator app; removing the last factor
// turns MFA off (recovery codes deleted).
func (a *Auth) RemoveTOTP(ctx context.Context, userID, currentPassword, keepSession string) error {
	err := a.sensitive(ctx, userID, currentPassword, keepSession, func(q *gen.Queries, u User, _ time.Time) (bool, error) {
		if _, err := q.GetUserTOTP(ctx, u.ID); errors.Is(err, sql.ErrNoRows) {
			return false, ErrNoTOTP
		} else if err != nil {
			return false, fmt.Errorf("read totp: %w", err)
		}
		if err := q.DeleteUserTOTP(ctx, u.ID); err != nil {
			return false, fmt.Errorf("delete totp: %w", err)
		}
		return true, dropCodesIfNoFactor(ctx, q, u.ID)
	})
	if err != nil {
		return fmt.Errorf("remove totp for %s: %w", userID, err)
	}
	return nil
}

// RemoveKey removes one of the user's security keys; removing the last
// factor turns MFA off (recovery codes deleted).
func (a *Auth) RemoveKey(ctx context.Context, userID, keyID, currentPassword, keepSession string) error {
	err := a.sensitive(ctx, userID, currentPassword, keepSession, func(q *gen.Queries, u User, _ time.Time) (bool, error) {
		if _, err := q.GetUserWebAuthnCredential(ctx, gen.GetUserWebAuthnCredentialParams{ID: keyID, UserID: u.ID}); errors.Is(err, sql.ErrNoRows) {
			return false, ErrKeyNotFound
		} else if err != nil {
			return false, fmt.Errorf("read security key: %w", err)
		}
		if err := q.DeleteUserWebAuthnCredential(ctx, gen.DeleteUserWebAuthnCredentialParams{ID: keyID, UserID: u.ID}); err != nil {
			return false, fmt.Errorf("delete security key: %w", err)
		}
		return true, dropCodesIfNoFactor(ctx, q, u.ID)
	})
	if err != nil {
		return fmt.Errorf("remove security key for %s: %w", userID, err)
	}
	return nil
}

// RegenerateRecoveryCodes replaces the user's recovery codes and returns
// the new ones (shown once).
func (a *Auth) RegenerateRecoveryCodes(ctx context.Context, userID, currentPassword, keepSession string) ([]string, error) {
	var codes []string
	err := a.sensitive(ctx, userID, currentPassword, keepSession, func(q *gen.Queries, u User, _ time.Time) (bool, error) {
		methods, err := userMethods(ctx, q, u.ID)
		if err != nil {
			return false, err
		}
		if !hasFactor(methods) {
			return false, ErrNoFactor
		}
		codes, err = replaceRecoveryCodes(ctx, q, u.ID)
		return true, err
	})
	if err != nil {
		return nil, fmt.Errorf("regenerate recovery codes for %s: %w", userID, err)
	}
	return codes, nil
}

// ChangePassword sets the user's own password (same policy as account
// creation). Pending logins opened with the old password are dropped.
func (a *Auth) ChangePassword(ctx context.Context, userID, current, next, keepSession string) error {
	u, err := a.reauthenticate(ctx, userID, current)
	if err != nil {
		return fmt.Errorf("change password for %s: %w", userID, err)
	}
	if err := checkPassword(next); err != nil {
		var in *InputError
		if errors.As(err, &in) {
			in.Field = "new_password"
		}
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), a.cost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	err = a.sensitiveTx(ctx, u, keepSession, func(q *gen.Queries, u User, _ time.Time) (bool, error) {
		if err := q.UpdateUserPassword(ctx, gen.UpdateUserPasswordParams{PasswordHash: string(hash), ID: u.ID}); err != nil {
			return false, fmt.Errorf("update password: %w", err)
		}
		if err := q.DeleteUserLoginChallenges(ctx, u.ID); err != nil {
			return false, fmt.Errorf("drop pending logins: %w", err)
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("change password for %s: %w", userID, err)
	}
	return nil
}

// ResetMFA removes every second factor of userID (authenticator app, security
// keys, recovery codes), its pending logins and its MFA lockout, and closes
// all its sessions, in one transaction: the account logs in with its password
// only. actor is the admin's user id, or "cli"; ip is the client address
// ("" from the CLI). sql.ErrNoRows (wrapped) when
// the user does not exist.
func (a *Auth) ResetMFA(ctx context.Context, userID, actor, ip string) error {
	err := a.db.Tx(ctx, func(q *gen.Queries) error {
		if _, err := q.GetUser(ctx, userID); err != nil {
			return fmt.Errorf("user %s: %w", userID, err)
		}
		steps := []struct {
			what string
			do   func() error
		}{
			{"totp", func() error { return q.DeleteUserTOTP(ctx, userID) }},
			{"security keys", func() error { return q.DeleteUserWebAuthnCredentials(ctx, userID) }},
			{"recovery codes", func() error { return q.DeleteUserRecoveryCodes(ctx, userID) }},
			{"pending logins", func() error { return q.DeleteUserLoginChallenges(ctx, userID) }},
			{"lockout", func() error { return q.ResetUserMFAFailures(ctx, userID) }},
			{"sessions", func() error { return q.DeleteUserSessions(ctx, userID) }},
		}
		for _, s := range steps {
			if err := s.do(); err != nil {
				return fmt.Errorf("delete %s: %w", s.what, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("reset mfa for %s: %w", userID, err)
	}
	slog.Info("second factors reset", "event", "mfa.reset_by_admin", "user_id", userID, "actor", actor, "ip", ip)
	return nil
}

// MFABadge is what the users list shows of one account's second factors.
type MFABadge struct {
	TOTP bool
	Keys int
}

// Labels are the badges to show: "App", "Key" or "Key ×N".
func (b MFABadge) Labels() []string {
	var out []string
	if b.TOTP {
		out = append(out, "App")
	}
	switch {
	case b.Keys == 1:
		out = append(out, "Key")
	case b.Keys > 1:
		out = append(out, "Key ×"+strconv.Itoa(b.Keys))
	}
	return out
}

// MFABadges returns the factors of every user, by user id, in one query.
func (a *Auth) MFABadges(ctx context.Context) (map[string]MFABadge, error) {
	rows, err := gen.New(a.db.Read).ListUsersMFABadges(ctx)
	if err != nil {
		return nil, fmt.Errorf("list mfa badges: %w", err)
	}
	out := make(map[string]MFABadge, len(rows))
	for _, r := range rows {
		out[r.UserID] = MFABadge{TOTP: r.HasTotp, Keys: int(r.KeyCount)}
	}
	return out, nil
}
