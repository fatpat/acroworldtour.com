-- name: GetUserTOTP :one
SELECT * FROM user_totp WHERE user_id = ?;

-- name: CountUserTOTP :one
SELECT count(*) FROM user_totp;

-- name: UpsertUserTOTP :exec
INSERT INTO user_totp (user_id, secret_enc, created_at, last_step) VALUES (?, ?, ?, 0)
ON CONFLICT (user_id) DO UPDATE SET secret_enc = excluded.secret_enc, created_at = excluded.created_at, last_step = 0;

-- name: AdvanceUserTOTPStep :execrows
UPDATE user_totp SET last_step = ? WHERE user_id = ? AND last_step < ?;

-- name: DeleteUserTOTP :exec
DELETE FROM user_totp WHERE user_id = ?;

-- name: ListUserWebAuthnCredentials :many
SELECT * FROM user_webauthn_credentials WHERE user_id = ? ORDER BY created_at, id;

-- name: GetUserWebAuthnCredential :one
SELECT * FROM user_webauthn_credentials WHERE id = ? AND user_id = ?;

-- name: InsertUserWebAuthnCredential :exec
INSERT INTO user_webauthn_credentials (id, user_id, name, credential, created_at) VALUES (?, ?, ?, ?, ?);

-- name: UpdateUserWebAuthnCredential :exec
UPDATE user_webauthn_credentials SET credential = ?, last_used_at = ? WHERE id = ? AND user_id = ?;

-- name: DeleteUserWebAuthnCredential :exec
DELETE FROM user_webauthn_credentials WHERE id = ? AND user_id = ?;

-- name: DeleteUserWebAuthnCredentials :exec
DELETE FROM user_webauthn_credentials WHERE user_id = ?;

-- name: ListUserRecoveryCodes :many
SELECT * FROM user_recovery_codes WHERE user_id = ? ORDER BY code_hash;

-- name: InsertUserRecoveryCode :exec
INSERT INTO user_recovery_codes (user_id, code_hash) VALUES (?, ?);

-- name: UseUserRecoveryCode :execrows
UPDATE user_recovery_codes SET used_at = ? WHERE user_id = ? AND code_hash = ? AND used_at IS NULL;

-- name: CountUnusedUserRecoveryCodes :one
SELECT count(*) FROM user_recovery_codes WHERE user_id = ? AND used_at IS NULL;

-- name: DeleteUserRecoveryCodes :exec
DELETE FROM user_recovery_codes WHERE user_id = ?;

-- name: InsertLoginChallenge :exec
INSERT INTO login_challenges (token_hash, user_id, kind, data, expires_at) VALUES (?, ?, ?, ?, ?);

-- name: GetLoginChallenge :one
SELECT * FROM login_challenges WHERE token_hash = ? AND kind = ? AND expires_at > ?;

-- name: UpdateLoginChallengeData :exec
UPDATE login_challenges SET data = ? WHERE token_hash = ?;

-- name: DeleteLoginChallenge :exec
DELETE FROM login_challenges WHERE token_hash = ?;

-- name: DeleteUserLoginChallenges :exec
DELETE FROM login_challenges WHERE user_id = ?;

-- name: DeleteExpiredLoginChallenges :exec
DELETE FROM login_challenges WHERE expires_at <= ?;

-- name: RecordMFAFailure :one
UPDATE users SET mfa_failed_count = mfa_failed_count + 1 WHERE id = ? RETURNING mfa_failed_count;

-- name: LockUserMFA :exec
UPDATE users SET mfa_locked_until = ? WHERE id = ?;

-- name: ResetUserMFAFailures :exec
UPDATE users SET mfa_failed_count = 0, mfa_locked_until = NULL WHERE id = ?;
