-- Training refresh (docs/ops.md, decision 0018): the cloned prod database
-- loses every session, pending login and second factor.

-- name: DeleteAllSessions :execrows
DELETE FROM sessions;

-- name: DeleteAllLoginChallenges :execrows
DELETE FROM login_challenges;

-- name: DeleteAllUserTOTP :execrows
DELETE FROM user_totp;

-- name: DeleteAllUserWebAuthnCredentials :execrows
DELETE FROM user_webauthn_credentials;

-- name: DeleteAllUserRecoveryCodes :execrows
DELETE FROM user_recovery_codes;

-- name: ResetAllUserMFAFailures :execrows
UPDATE users SET mfa_failed_count = 0, mfa_locked_until = NULL
WHERE mfa_failed_count <> 0 OR mfa_locked_until IS NOT NULL;

-- name: CloneSummary :one
SELECT
    (SELECT count(*) FROM users) AS users,
    (SELECT count(*) FROM competitions) AS competitions,
    (SELECT count(*) FROM news) AS news,
    (SELECT count(*) FROM files) AS files;
