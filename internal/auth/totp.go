package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 authenticator apps interoperate on HMAC-SHA1; not used for collision resistance.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	totpIssuer = "Acro World Tour Manager"
	totpPeriod = 30
	totpDigits = 6
)

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a fresh 160-bit secret, base32 without padding.
func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return totpEncoding.EncodeToString(b), nil
}

func decodeTOTPSecret(secret string) ([]byte, error) {
	s := strings.ToUpper(strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""), "="))
	key, err := totpEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode totp secret: %w", err)
	}
	if len(key) < 16 { // RFC 4226 requirement R6: at least 128 bits
		return nil, errors.New("decode totp secret: shorter than 16 bytes")
	}
	return key, nil
}

func totpStep(t time.Time) int64 { return t.Unix() / totpPeriod }

func totpAt(key []byte, step int64) string {
	mac := hmac.New(sha1.New, key) //nolint:gosec // see import comment
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) //nolint:gosec // step is non-negative for any realistic clock
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, bin%1_000_000)
}

// TOTPCode returns the 6-digit code valid at t.
func TOTPCode(secret string, t time.Time) (string, error) {
	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return "", err
	}
	return totpAt(key, totpStep(t)), nil
}

// VerifyTOTP accepts the steps around t (one before to one after) that are
// strictly greater than lastStep, which blocks replay of a used code. It
// returns the matching step.
func VerifyTOTP(secret, code string, t time.Time, lastStep int64) (int64, bool) {
	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return 0, false
	}
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	cur := totpStep(t)
	var matched int64
	ok := false
	for step := cur - 1; step <= cur+1; step++ {
		// Evaluate every step so timing does not reveal which one matched.
		eq := subtle.ConstantTimeCompare([]byte(code), []byte(totpAt(key, step))) == 1
		if eq && step > lastStep && !ok {
			matched, ok = step, true
		}
	}
	return matched, ok
}

// TOTPURI is the otpauth URI the authenticator app imports.
func TOTPURI(secret, email string) string {
	label := url.PathEscape(totpIssuer) + ":" + url.PathEscape(email)
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		label, url.QueryEscape(secret), url.PathEscape(totpIssuer), totpDigits, totpPeriod)
}
