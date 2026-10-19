package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

const (
	recoveryAlphabet = "23456789abcdefghjkmnpqrstuvwxyz"
	recoveryCount    = 10
	recoveryLen      = 10
)

// NewRecoveryCodes returns 10 codes formatted xxxxx-xxxxx for display and
// the SHA-256 hex of each normalised code for storage. The codes carry 50
// bits of entropy, so an unsalted hash is enough.
func NewRecoveryCodes() (display, hashes []string, err error) {
	max := big.NewInt(int64(len(recoveryAlphabet)))
	for range recoveryCount {
		var b strings.Builder
		for range recoveryLen {
			n, err := rand.Int(rand.Reader, max)
			if err != nil {
				return nil, nil, fmt.Errorf("generate recovery code: %w", err)
			}
			b.WriteByte(recoveryAlphabet[n.Int64()])
		}
		code := b.String()
		display = append(display, code[:5]+"-"+code[5:])
		hashes = append(hashes, HashRecoveryCode(code))
	}
	return display, hashes, nil
}

// NormaliseRecoveryCode lowercases the input and strips spaces and dashes.
func NormaliseRecoveryCode(in string) string {
	return strings.NewReplacer(" ", "", "-", "").Replace(strings.ToLower(in))
}

// HashRecoveryCode is the SHA-256 hex of an already normalised code.
func HashRecoveryCode(normalised string) string {
	sum := sha256.Sum256([]byte(normalised))
	return hex.EncodeToString(sum[:])
}
