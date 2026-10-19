# Manager login MFA — design

Date: 2026-10-19. Status: builder-checked; awaiting owner approval.

## Intent

Manager accounts (admins and scorers) can change official competition results. A stolen password alone should not be enough to log in. Users protect their account with a second factor of their choice; nobody is locked out on competition day when a phone or key goes missing.

## Goals

- Each user can enroll **an authenticator app (TOTP)**, **one or more security keys / passkeys (WebAuthn)**, or both.
- MFA is **optional per user** (owner decision: "optional for now"). Once a user has at least one factor, it is **required at every login**.
- At login, after a correct password, the user picks among the methods they enrolled.
- **Recovery codes**: 10 single-use codes generated at first enrollment, shown once, regenerable by the user. A recovery code replaces the second factor once.
- **Admin reset**: an admin removes all of a user's factors and recovery codes and revokes their sessions; the user then logs in with password only until they enroll again.
- Security events are logged.

## Non-goals

- Forcing MFA for a role or for everyone (later; design must not prevent it).
- "Remember this device" / trusted devices.
- SMS or email codes.
- Passwordless login (passkey replacing the password). Keys are a second factor only.

## Anti-goals (failure even if it works)

- A half-authenticated user (password OK, factor not yet given) can reach any manager page or API, or holds a usable session cookie.
- MFA codes can be brute-forced faster than passwords.
- A user locked out with no path back except editing the database.
- Secrets (TOTP seeds, recovery codes) readable from the database or logs in plain form beyond what verification strictly needs.

## Constraints

- Follows CLAUDE.md: Go standard library first; new dependency needs a decision record; templ + htmx, no SPA, no inline scripts (CSP `script-src 'self'`); every POST behind CSRF and a body limit; one transaction per mutation.
- Existing auth: `internal/auth` (bcrypt passwords, random session tokens stored hashed in `sessions`, `awt_session` cookie, `LoginLimiter` keyed by email + client IP, `RequireUser`/`RequireAdmin`/`RequireScorer`). Login page `/manager/login`. Users screen under `/manager/users` (admin only).
- WebAuthn needs a stable Relying Party ID = host of `AWT_BASE_URL` and HTTPS (localhost allowed in dev). Origin = `AWT_BASE_URL` origin.

## Decided how (owner)

- Methods: TOTP and WebAuthn, user's choice, both allowed together.
- Optional per user.
- Recovery: both recovery codes and admin reset.

## Decided how (my calls, owner may override)

- **TOTP**: RFC 6238, SHA-1, 6 digits, 30 s step, ±1 step tolerance; implemented with the standard library (`crypto/hmac`). Enrollment shows a QR code (otpauth URI, issuer "Acro World Tour Manager", account = email) plus the base32 secret for manual entry; enrollment completes only after the user types a valid code. A code (step) accepted once cannot be reused (store last used step). QR rendering via a small library (`rsc.io/qr`) rendered as a PNG data URL or server-generated image (decision record).
- **WebAuthn**: `github.com/go-webauthn/webauthn` (decision record). User verification "preferred", attestation "none", resident keys not required. Each key has a user-given name, created date and last-used date; user can remove a key. Sign counter checked when the authenticator provides one.
- **Login flow**: password step unchanged in validation and rate limiting. If the user has factors, instead of a session the server creates a short-lived **pending login** (random token, hashed in DB, 5 minutes, bound to user id, client IP not required) and redirects to `/manager/login/mfa`. Only after a successful factor is the real session created (same as today) and the pending login deleted. The pending token is in its own cookie (`awt_mfa`, HttpOnly, Secure outside dev, SameSite=Strict, path `/manager/login`); it grants nothing else.
- **Attempt limits**: see "Settled after builder check" (persisted per-account factor counter).
- **Recovery codes**: 10 codes, 10 characters from an unambiguous alphabet, shown grouped (`xxxxx-xxxxx`); stored as SHA-256 hashes (high-entropy, bcrypt unnecessary); single use; using one logs a warning event and shows "N codes left" after login. Regenerating replaces all.
- **TOTP secret at rest**: stored encrypted with AES-GCM using a key from a new env var `AWT_MFA_KEY` (32 bytes, base64), required outside dev when any TOTP exists — dev uses a fixed dev key with a startup warning. (Reason: a DB backup leak must not yield usable seeds.)
- **Account page** `/manager/account` (any logged-in user): shows enrolled methods, add TOTP, add security key, remove each, view count / regenerate recovery codes, and change own password. Removing the last factor turns MFA off for that user (confirmation in-page). Sensitive actions require the current password (see below).
- **Admin**: Users list shows an "MFA" badge (methods). "Reset MFA" button with in-page confirmation; refuses resetting yourself (use your account page). Also available as CLI `awt user reset-mfa --email` for the last-admin-locked-out case.
- **Events**: slog at INFO with user id, event (`mfa.enroll.totp`, `mfa.enroll.webauthn`, `mfa.remove`, `mfa.reset_by_admin`, `mfa.recovery_used`, `mfa.recovery_regenerated`, `mfa.failed`), client IP; never secrets or codes.
- **Storage**: new migration with tables `user_totp(user_id PK, secret_enc, created_at, last_step)`, `user_webauthn_credentials(id PK, user_id, name, credential JSON, created_at, last_used_at)`, `user_recovery_codes(user_id, code_hash, used_at NULL)`, `login_challenges(token_hash PK, user_id, kind ('pending_login'|'webauthn_register'|'webauthn_login'), data JSON, expires_at, failed_attempts)`. All `ON DELETE CASCADE` from users. Expired rows pruned on login.

## Settled after builder check (my calls)

- **Factor-step brute force**: a separate, persisted per-account counter (`users.mfa_failed_count`, `users.mfa_locked_until`), independent of IP and NOT reset by a correct password. 5 failed factor attempts → factor step locked for that account for 15 minutes (429 with "Too many attempts, try again in N minutes"). A successful factor resets it. The password limiter is unchanged. Lockout abuse requires knowing the password, so it is acceptable. The per-pending-login cap is dropped (redundant).
- **Sessions on changes**: adding a factor, removing a factor, regenerating recovery codes and changing the password all revoke the user's **other** sessions (current one kept). All four require the current password re-entered in the same form (not only after 10 minutes), so a stolen live session cannot enroll its own factor. Admin reset revokes all sessions.
- **Password change** is new on the account page: current password required, same policy as today (min 10 characters), revokes other sessions. `sessions.created_at` already exists.
- **WebAuthn RP ID**: host of `AWT_BASE_URL`, optional override `AWT_WEBAUTHN_RP_ID` for an apex/parent domain. The manager must be served from that host; documented in docs/ops.md. Changing it invalidates enrolled keys (users fall back to TOTP/recovery codes or admin reset).
- **`AWT_MFA_KEY`**: stored ciphertext is `v1:` + key id (first 8 hex of SHA-256 of the key) + nonce + ciphertext, so a later rotation can add keys. Required at startup outside dev only if any `user_totp` row exists (app refuses to start otherwise with a clear message). If decryption fails for a user (key changed), the TOTP option is shown as unavailable at login with a message to use a security key or a recovery code; an error is logged. Dev uses a fixed key with a startup warning; e2e runs in dev mode.
- **CLI `awt user reset-mfa --email`**: same effect as the admin button (removes factors and codes, revokes all sessions, logs `mfa.reset_by_admin` with actor `cli`).

## Left to the builder

- Exact package split inside `internal/auth` (e.g. `totp.go`, `webauthn.go`, `recovery.go`, `mfa_login.go`).
- Page layout and copy, within the existing manager layout and CSS tokens.
- The small JS module for WebAuthn ceremonies (`navigator.credentials.create/get`, base64url conversion) in `web/static/js/webauthn.js`, no inline script.
- Test structure; WebAuthn tested with the library's virtual authenticator or a hand-built test authenticator; e2e coverage of TOTP login (the e2e can compute codes from the secret it reads at enrollment).

## Acceptance

- User without factors: login unchanged.
- User with TOTP: correct password → MFA page; wrong code rejected; correct code → logged in; same code reused → rejected; recovery code → logged in once, then rejected.
- User with a key: login with the key works (virtual authenticator test).
- Pending login cookie alone cannot access any `/manager` page (403/redirect), and expires after 5 minutes.
- Admin reset removes all factors and sessions; user logs in with password only.
- 6th failed factor attempt on an account within 15 min → 429, even after re-entering the correct password or from another IP.
- Enrolling a factor, removing one, regenerating codes or changing the password requires the current password and logs out the user's other sessions.
- No TOTP secret or recovery code appears in logs; TOTP secrets are not stored in plain text.
