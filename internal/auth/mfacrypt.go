package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"awt/internal/config"
	"awt/internal/store"
	"awt/internal/store/gen"
)

// ErrMFAKeyMismatch means the secret was sealed with another AWT_MFA_KEY.
var ErrMFAKeyMismatch = errors.New("mfa key mismatch: secret sealed with a different AWT_MFA_KEY")

func mfaKeyID(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:4])
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("mfa key: %w", err)
	}
	return cipher.NewGCM(block)
}

// Purposes bind a sealed value to its use and owner (GCM additional data);
// callers append the user id.
const (
	PurposeTOTPSecret = "awt/totp-secret/v1|user:"
	PurposeTOTPEnroll = "awt/totp-enroll/v1|user:"
)

// sealSecret encrypts plain with AES-256-GCM as
// v1:<keyid8hex>:<base64(nonce|ciphertext)>, authenticating purpose.
func sealSecret(key []byte, purpose, plain string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("seal secret: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), []byte(purpose))
	return "v1:" + mfaKeyID(key) + ":" + base64.StdEncoding.EncodeToString(ct), nil
}

// openSecret reverses sealSecret. A key id that differs yields
// ErrMFAKeyMismatch; a damaged value yields a plain error.
func openSecret(key []byte, purpose, sealed string) (string, error) {
	parts := strings.Split(sealed, ":")
	if len(parts) != 3 || parts[0] != "v1" {
		return "", errors.New("open secret: unrecognised format")
	}
	if parts[1] != mfaKeyID(key) {
		return "", ErrMFAKeyMismatch
	}
	raw, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("open secret: %w", err)
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("open secret: truncated value")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte(purpose))
	if err != nil {
		return "", fmt.Errorf("open secret: %w", err)
	}
	return string(plain), nil
}

// CheckMFAKeyRequired fails outside dev when TOTP secrets exist but
// AWT_MFA_KEY is unset: they could not be opened and logins would break.
func CheckMFAKeyRequired(ctx context.Context, db *store.DB, cfg config.Config) error {
	if cfg.Env == "dev" || len(cfg.MFAKey) > 0 {
		return nil
	}
	n, err := gen.New(db.Read).CountUserTOTP(ctx)
	if err != nil {
		return fmt.Errorf("count totp secrets: %w", err)
	}
	if n > 0 {
		return errors.New("AWT_MFA_KEY is required: TOTP secrets exist (generate with: openssl rand -base64 32)")
	}
	return nil
}
