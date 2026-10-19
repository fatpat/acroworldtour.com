-- Manager-login MFA: TOTP, WebAuthn, recovery codes, pending challenges and
-- per-account factor lockout.

CREATE TABLE user_totp (
    user_id TEXT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    secret_enc TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_step INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE user_webauthn_credentials (
    id TEXT PRIMARY KEY, -- base64url credential id
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    credential TEXT NOT NULL CHECK (json_valid(credential)),
    created_at TEXT NOT NULL,
    last_used_at TEXT
);
CREATE INDEX user_webauthn_credentials_user ON user_webauthn_credentials (user_id);

CREATE TABLE user_recovery_codes (
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL, -- SHA-256 hex
    used_at TEXT,
    PRIMARY KEY (user_id, code_hash)
);

CREATE TABLE login_challenges (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('pending_login', 'webauthn_register', 'webauthn_login')),
    data TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(data)),
    expires_at TEXT NOT NULL
);
CREATE INDEX login_challenges_user ON login_challenges (user_id);

ALTER TABLE users ADD COLUMN mfa_failed_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN mfa_locked_until TEXT;
