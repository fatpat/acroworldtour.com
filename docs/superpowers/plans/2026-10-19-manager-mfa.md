# Manager Login MFA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Optional per-user second factor (TOTP and/or WebAuthn security keys) for `/manager` logins, with recovery codes and admin/CLI reset.

**Architecture:** All MFA logic lives in `internal/auth` beside the existing password/session code; a correct password for a user with factors creates a short-lived *pending login* (own cookie, own DB row) instead of a session, and the session is created only after a factor succeeds. Web pages (login second step, account page, admin reset) live in `internal/web` (login) and `internal/web/admin` (account, users). One new migration holds all MFA tables.

**Tech Stack:** Go stdlib (`crypto/hmac`, `crypto/aes`, `crypto/cipher`), `github.com/go-webauthn/webauthn`, `rsc.io/qr`, templ, htmx, plain JS module.

**Spec:** `docs/superpowers/specs/2026-10-19-manager-mfa-design.md` (read it fully; "Settled after builder check" overrides earlier lines). Project rules: `CLAUDE.md`.

## Global Constraints

- Every rule in `CLAUDE.md` applies (no inline scripts, CSRF + body limit on every POST, one Tx per mutation, errors wrapped, no secrets in logs).
- New dependencies allowed: `github.com/go-webauthn/webauthn` and `rsc.io/qr` only, each with a decision record (`docs/decisions/0014-webauthn.md`, `0015-qr.md`).
- TOTP: RFC 6238, SHA-1, 6 digits, 30 s step, ±1 step tolerance, issuer `Acro World Tour Manager`, account label = user email.
- Recovery codes: 10 codes, 10 chars from alphabet `23456789abcdefghjkmnpqrstuvwxyz`, displayed `xxxxx-xxxxx`, input accepted with or without dash/case, stored as SHA-256 hex.
- Pending login: 5 minutes, cookie `awt_mfa` (HttpOnly, Secure unless dev, SameSite=Strict, Path `/manager/login`).
- Factor lockout: 5 failed factor attempts per account → locked 15 minutes (429), not reset by a correct password nor by changing IP; reset by a successful factor.
- Sensitive account actions (add/remove factor, regenerate codes, change password) require the current password in the same form and revoke the user's other sessions.
- Env: `AWT_MFA_KEY` (base64, 32 bytes) required outside dev when any `user_totp` row exists; `AWT_WEBAUTHN_RP_ID` optional (default: host of `AWT_BASE_URL`; dev default `localhost`).
- Events logged with `slog.Info` (key `event`): `mfa.enroll.totp`, `mfa.enroll.webauthn`, `mfa.remove`, `mfa.reset_by_admin`, `mfa.recovery_used`, `mfa.recovery_regenerated`, `mfa.failed`, `mfa.locked`; fields `user_id`, `ip`, `actor`; never secrets or codes.

## Review Focus

1. **Clock skew on the scorer's phone** (codes 30–60 s off): a code from the previous or next step must work. Test in Task 2: `TestVerifyTOTPWindow`.
2. **Pending cookie used directly on `/manager/*`** (attacker who knows the password but not the factor): must be redirected to login, never served. Test in Task 3: `TestPendingLoginGrantsNothing`.
3. **User removes their last factor, then logs in**: login must work with password only, and their recovery codes must be gone. Test in Task 5: `TestRemoveLastFactorDisablesMFA`.
4. **Two tabs racing the same TOTP code** (double submit): only one login succeeds; the replayed code is rejected. Test in Task 3: `TestTOTPCodeReplayRejected`.
5. **Recovery code typed with spaces/uppercase/missing dash**: accepted. Test in Task 2: `TestRecoveryCodeNormalisation`.

---

### Task 1: Schema, config, decision records

**Files:** Create `internal/store/migrations/0004_mfa.sql`, `internal/store/queries/mfa.sql`, `docs/decisions/0014-webauthn.md`, `docs/decisions/0015-qr.md`; Modify `internal/config/config.go`, `internal/config/config_test.go`, `deploy/env*.example`, `docs/ops.md` (§2 secrets: MFA key generation `openssl rand -base64 32`, RP ID note), `internal/store/store_test.go` (migration count 4).

**Interfaces — Produces:** tables `user_totp(user_id PK FK users ON DELETE CASCADE, secret_enc TEXT, created_at TEXT, last_step INTEGER NOT NULL DEFAULT 0)`, `user_webauthn_credentials(id TEXT PK /*base64url credential id*/, user_id FK CASCADE, name TEXT, credential TEXT CHECK json_valid, created_at TEXT, last_used_at TEXT NULL)`, `user_recovery_codes(user_id FK CASCADE, code_hash TEXT, used_at TEXT NULL, PRIMARY KEY(user_id, code_hash))`, `login_challenges(token_hash TEXT PK, user_id FK CASCADE, kind TEXT CHECK kind IN ('pending_login','webauthn_register','webauthn_login'), data TEXT CHECK json_valid, expires_at TEXT)`; columns `users.mfa_failed_count INTEGER NOT NULL DEFAULT 0`, `users.mfa_locked_until TEXT NULL`. sqlc queries for CRUD of each. `config.Config` gains `MFAKey []byte` (decoded; dev default = SHA-256 of "awt-dev-mfa-key" with a startup warning logged by `serve`) and `WebAuthnRPID string`.

- [ ] Step 1: Tests `TestLoadMFAKey` (valid base64 32 bytes ok; wrong length → error naming `AWT_MFA_KEY`; dev without var → dev key), `TestLoadWebAuthnRPID` (default from BaseURL host; override; dev default `localhost`), migration count test updated. Run → FAIL.
- [ ] Step 2: Implement migration, queries (`make generate`), config. The "required when any TOTP row exists" rule is enforced in Task 2 at `serve` start (config can't see the DB). Run tests → PASS. Commit `feat(store): MFA schema and config`.

### Task 2: Crypto primitives — TOTP, secret encryption, recovery codes

**Files:** Create `internal/auth/totp.go`, `internal/auth/mfacrypt.go`, `internal/auth/recovery.go`, `internal/auth/mfa_primitives_test.go`; Modify `cmd/awt/main.go` (startup check).

**Interfaces — Produces:**
- `NewTOTPSecret() (string, error)` (20 random bytes, base32 no padding); `TOTPCode(secret string, t time.Time) (string, error)`; `VerifyTOTP(secret, code string, t time.Time, lastStep int64) (step int64, ok bool)` — accepts steps cur-1..cur+1 strictly greater than `lastStep`, constant-time compare; `TOTPURI(secret, email string) string` (`otpauth://totp/Acro%20World%20Tour%20Manager:<email>?secret=…&issuer=Acro%20World%20Tour%20Manager&algorithm=SHA1&digits=6&period=30`).
- `sealSecret(key []byte, plain string) (string, error)` → `v1:<keyid8hex>:<base64(nonce|ciphertext)>`; `openSecret(key []byte, sealed string) (string, error)` → `ErrMFAKeyMismatch` when key id differs.
- `NewRecoveryCodes() (display []string, hashes []string, err error)`; `NormaliseRecoveryCode(in string) string` (lowercase, strip spaces and dashes); `HashRecoveryCode(normalised string) string`.
- `CheckMFAKeyRequired(ctx, db *store.DB, cfg config.Config) error` — error outside dev when `user_totp` has rows and `AWT_MFA_KEY` unset; called by `serve` before listening.

- [ ] Step 1: Tests: RFC 6238 Appendix B SHA-1 vectors truncated to 6 digits (secret ASCII `12345678901234567890`: t=59 → `287082`, t=1111111109 → `081804`, t=1234567890 → `005924`, t=2000000000 → `279037`); `TestVerifyTOTPWindow` (±1 step accepted, ±2 rejected, step ≤ lastStep rejected); seal/open round trip, tamper → error, other key → `ErrMFAKeyMismatch`; 10 distinct recovery codes matching `^[2-9a-hjkmnp-z]{5}-[2-9a-hjkmnp-z]{5}$`; `TestRecoveryCodeNormalisation` (`" ABCDE-fghjk "` and `"abcdefghjk"` normalise equal); `TestCheckMFAKeyRequired`. Run → FAIL.
- [ ] Step 2: Implement. Run → PASS. Commit `feat(auth): TOTP, secret sealing, recovery codes`.

### Task 3: Two-step login with TOTP and recovery codes

**Files:** Create `internal/auth/mfa_login.go`, `internal/auth/mfa_login_test.go`, `internal/web/layout/mfa.templ` (second-step page); Modify `internal/auth/sessions.go` (`Login`), `internal/web/errors.go` (`loginSubmit`), `internal/web/routes.go` (routes), `internal/web/routes_test.go`.

**Interfaces:**
- Consumes Task 1 queries, Task 2 primitives.
- Produces: `Login` returns `(Session, error)` where, for a user with factors, it returns `ErrMFARequired` and a `Pending{Token string; ExpiresAt time.Time; Methods []string /* "totp","webauthn","recovery" */}` via new `func (a *Auth) LoginStep1(ctx, email, password string) (Session, *Pending, error)` (keep `Login` for callers without MFA concerns = calls LoginStep1 and returns ErrMFARequired when pending). `func (a *Auth) PendingFromRequest(r *http.Request) (PendingLogin, error)` (`PendingLogin{UserID, Email, Methods, TOTPAvailable bool /* false on key mismatch */}`); `func (a *Auth) CompleteWithTOTP(ctx, r, code string) (Session, error)`; `CompleteWithRecovery(ctx, r, code string) (Session, int /*codes left*/, error)`; errors `ErrMFAInvalid`, `ErrMFALocked{Until time.Time}`; `SetPendingCookie`, `ClearPendingCookie`; `UserHasMFA(ctx, userID string) (bool, error)`.
- Routes: `GET /manager/login/mfa` (page: method choice + code field + "use a recovery code" + security key button placeholder for Task 4), `POST /manager/login/mfa/totp`, `POST /manager/login/mfa/recovery` — all Public (login paths), body cap `loginMax`, no CSRF token exists yet so protect with SameSite=Strict pending cookie + Origin header check (reject when `Origin` present and ≠ BaseURL origin).
- Lockout: failure increments `users.mfa_failed_count`; at 5 set `mfa_locked_until = now+15m`, reset count; while locked every factor POST → 429 with minutes left; success resets both and deletes the pending row; also delete all expired `login_challenges` on each login.

- [ ] Step 1: Tests (auth package, temp DB, injected clock): user without factors logs in exactly as before; user with TOTP → `ErrMFARequired` + pending; `TestPendingLoginGrantsNothing` (pending cookie on `GET /manager` → redirect to login; `UserFromRequest` fails); correct code → session, pending deleted; wrong code ×5 → 6th attempt `ErrMFALocked` even with a fresh pending created by re-entering the password (`TestFactorLockoutSurvivesPasswordReentry`) and from another IP; `TestTOTPCodeReplayRejected` (same code twice within the step → second rejected); recovery code works once, then rejected, codes-left returned; pending expires after 5 min; key mismatch → `TOTPAvailable=false` and TOTP POST refused with a clear error; web tests for the three routes (redirect to `/manager/login/mfa` after password, `next` preserved through the step, 429 page, Origin mismatch → 403). Run → FAIL.
- [ ] Step 2: Implement; log `mfa.failed`, `mfa.locked`, `mfa.recovery_used`. Run `go test -race ./internal/auth ./internal/web` → PASS. Commit `feat(auth): two-step login with TOTP and recovery codes`.

### Task 4: WebAuthn ceremonies

**Files:** Create `internal/auth/webauthn.go`, `internal/auth/webauthn_test.go`, `web/static/js/webauthn.js`; Modify `internal/web/layout/mfa.templ`, `internal/web/routes.go`, `go.mod`.

**Interfaces — Produces:** `func (a *Auth) BeginWebAuthnRegistration(ctx, userID string) (optionsJSON []byte, challengeToken string, err error)`; `FinishWebAuthnRegistration(ctx, userID, challengeToken, name string, body []byte) error`; `BeginWebAuthnLogin(ctx, r) (optionsJSON []byte, err error)` (challenge stored on the pending row's `data` or a `webauthn_login` challenge linked to it); `FinishWebAuthnLogin(ctx, r, body []byte) (Session, error)` (same lockout and pending rules as Task 3; updates `last_used_at` and sign count). Library: `webauthn.New(&webauthn.Config{RPID, RPDisplayName: "Acro World Tour Manager", RPOrigins: []string{origin of BaseURL (dev: http://localhost:<port> from BaseURL or request Host)}})`; user verification preferred, attestation none; challenges expire after 5 min; exclude already-registered credentials on registration.
- Routes: `POST /manager/login/mfa/webauthn/begin`, `POST /manager/login/mfa/webauthn/finish` (Public login paths, same Origin check and body cap). Registration routes are added in Task 5 (account page) and call these functions.
- `webauthn.js`: module reading `data-begin-url`/`data-finish-url` from a button, base64url⇄ArrayBuffer helpers, `navigator.credentials.create/get`, posts JSON with the CSRF header when present, then follows `{"redirect": "..."}` or shows the error text in an element `[data-webauthn-error]`; no inline script; shows "This browser does not support security keys" when `PublicKeyCredential` is undefined.

- [ ] Step 1: Tests with a software authenticator (use the library's test helpers or a minimal ES256 authenticator in the test: generate key, build attestation "none" and assertion with correct clientDataJSON/authenticatorData): register then log in → session; wrong origin → error; replayed assertion (same sign count) → rejected; failures count toward the Task 3 lockout. Run → FAIL.
- [ ] Step 2: Implement; decision record 0014 justifies the dependency. Run → PASS. Commit `feat(auth): WebAuthn security keys as second factor`.

### Task 5: Account page — enroll, remove, recovery codes, password change

**Files:** Create `internal/web/admin/account.go`, `account.templ`, `account_test.go`, `internal/auth/mfa_manage.go`, `mfa_manage_test.go`; Modify `internal/web/admin/module.go` (routes), layout admin nav (link "My account" in the user menu).

**Interfaces — Produces (auth):** `MFAStatus(ctx, userID) (Status{TOTP bool; Keys []KeyInfo{ID, Name, CreatedAt, LastUsedAt}; RecoveryLeft int}, error)`; `StartTOTPEnrollment(ctx, userID) (secret, uri, sealedPending string, err error)` — nothing stored yet; `sealedPending` = `sealSecret(MFAKey, userID+"|"+secret+"|"+expiryUnix)` (10 min), carried in a hidden form field and checked (same user, not expired) on confirm and by the QR route; `ConfirmTOTP(ctx, userID, sealedPending, code, currentPassword string, keepSession string) (recoveryCodes []string /* non-nil when first factor */, error)`; `RemoveTOTP`, `RemoveKey(ctx, userID, keyID, currentPassword, keepSession)`; `RegenerateRecoveryCodes(ctx, userID, currentPassword, keepSession) ([]string, error)`; `ChangePassword(ctx, userID, current, next, keepSession string) error`; all verify the current password (`ErrWrongPassword` → 422 field error), run in one Tx and delete the user's other sessions (`DeleteUserSessionsExcept(user_id, token_hash)`). First factor enrollment generates recovery codes; removing the last factor deletes recovery codes.
- Routes (any logged-in user, `r.User`/`UserMax`, CSRF): `GET /manager/account`, `POST /manager/account/totp/start`, `POST /manager/account/totp/confirm`, `POST /manager/account/totp/remove`, `POST /manager/account/keys/begin`, `POST /manager/account/keys/finish` (name field), `POST /manager/account/keys/{id}/remove`, `POST /manager/account/recovery/regenerate`, `POST /manager/account/password`.
- Page: QR as PNG served from `GET /manager/account/totp/qr.png?p=<sealedPending>` (rsc.io/qr, `Cache-Control: no-store`) plus the base32 secret grouped by 4; recovery codes shown once in a box with a copy button and a "I saved them" acknowledgement; in-page confirmation for removals.

- [ ] Step 1: Tests: enroll TOTP (start → confirm with computed code + password) → status TOTP true, 10 recovery codes returned once, other session revoked, current kept; wrong password → 422 and nothing changed; wrong code → 422; `TestRemoveLastFactorDisablesMFA` (remove only factor → login is password-only, recovery codes deleted); regenerate codes invalidates old ones; change password (wrong current → 422; ok → other sessions revoked, can log in with new password); QR route requires login and a valid unexpired sealed value; CSRF required on every POST; a scorer can manage only their own account (no user id in any route). Run → FAIL.
- [ ] Step 2: Implement; log `mfa.enroll.*`, `mfa.remove`, `mfa.recovery_regenerated`. Run `go test -race ./internal/auth ./internal/web/...` → PASS. Commit `feat(manager): account page with MFA management and password change`.

### Task 6: Admin reset, CLI, users badge, e2e

**Files:** Modify `internal/auth/mfa_manage.go` (`ResetMFA(ctx, userID, actor string) error`), `internal/web/admin/users.go`, `users.templ`, `cmd/awt/user.go` (`reset-mfa --email`), `e2e/` (new `mfa_test.go`), `docs/ops.md` (lost factor procedure).

**Interfaces — Consumes:** Task 5 `MFAStatus`; produces `ResetMFA` deleting TOTP, keys, recovery codes, MFA lockout fields, and ALL sessions in one Tx, logging `mfa.reset_by_admin` with `actor` (admin user id or `cli`).

- [ ] Step 1: Tests: users list shows badges ("App", "Key ×2", none); `POST /manager/users/{id}/reset-mfa` with in-page confirmation → factors gone, sessions gone; admin cannot reset self (409 "use your account page"); scorer → 403; CLI `awt user reset-mfa --email x` against a temp DB works and logs actor `cli`; e2e `TestTOTPLoginFlow`: admin enrolls TOTP on the account page (reads the displayed secret, computes the code in Go), logs out, logs in with password → second step → code → dashboard; then uses a recovery code once. Run → FAIL.
- [ ] Step 2: Implement. Run `make check` (with `make e2e-deps` if needed) → PASS. Commit `feat(manager): admin and CLI MFA reset`.
