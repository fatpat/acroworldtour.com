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
- Added with a one-line justification in the commit that first imports it.
