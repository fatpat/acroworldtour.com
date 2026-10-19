# 0014. go-webauthn for WebAuthn

Status: accepted
Date: 2026-10-19

## Context

Manager login gains a second factor. Passkeys and security keys (WebAuthn) are one of the factors; implementing the ceremonies, attestation parsing and signature checks by hand would be error-prone.

## Decision

Use `github.com/go-webauthn/webauthn`: maintained, widely used, BSD-3-Clause licensed, implements the WebAuthn Level 2 registration and assertion ceremonies.

## Consequences

- Credentials are stored as JSON in `user_webauthn_credentials`; ceremony session data lives in `login_challenges`.
- The relying party ID comes from `AWT_WEBAUTHN_RP_ID` (default: host of `AWT_BASE_URL`); changing it invalidates every registered credential.
- Pinned at v0.18.2 (released 2026-09-19). It brings `fxamacker/cbor`, `go-tpm`, `golang-jwt` and `go-webauthn/x` as indirect dependencies, all permissively licensed.
- Relying party: display name "Acro World Tour Manager", attestation "none", user verification "preferred", resident keys discouraged; origins are the origin of `AWT_BASE_URL` (dev: localhost and 127.0.0.1 on the `AWT_ADDR` port), never the request `Host` header. Challenges expire after 5 minutes, using the app clock rather than the library's.
- A login whose signature counter did not increase (a possibly cloned key) is refused and counts as a failed factor.
- Tested with a software ES256 authenticator (`internal/auth/webauthntest`), no browser.
- Added with a one-line justification in the commit that first imports it.
