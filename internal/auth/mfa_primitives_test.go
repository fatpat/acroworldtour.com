package auth

import (
	"context"
	"encoding/base32"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"awt/internal/config"
	"awt/internal/store"
)

// RFC 6238 Appendix B secret "12345678901234567890".
var rfcSecret = base32.StdEncoding.EncodeToString([]byte("12345678901234567890"))

func TestTOTPRFC6238Vectors(t *testing.T) {
	for unix, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		got, err := TOTPCode(rfcSecret, time.Unix(unix, 0))
		if err != nil || got != want {
			t.Errorf("t=%d: got %q, %v; want %q", unix, got, err, want)
		}
	}
}

func TestTOTPSecretFormat(t *testing.T) {
	s, err := NewTOTPSecret()
	if err != nil || len(s) != 32 || strings.Contains(s, "=") {
		t.Fatalf("secret %q, %v", s, err)
	}
	// Lowercase and padded input are accepted.
	now := time.Unix(1234567890, 0)
	a, _ := TOTPCode(s, now)
	b, err := TOTPCode(strings.ToLower(s)+"====", now)
	if err != nil || a != b {
		t.Fatalf("lowercase/padded: %q vs %q, %v", a, b, err)
	}
	if _, err := TOTPCode("not base32!", now); err == nil {
		t.Fatal("invalid secret accepted")
	}
}

func TestVerifyTOTPWindow(t *testing.T) {
	now := time.Unix(1111111109, 0)
	cur := now.Unix() / 30
	code := func(step int64) string {
		c, err := TOTPCode(rfcSecret, time.Unix(step*30, 0))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	for _, d := range []int64{-1, 0, 1} {
		if step, ok := VerifyTOTP(rfcSecret, code(cur+d), now, 0); !ok || step != cur+d {
			t.Errorf("offset %d: step=%d ok=%v", d, step, ok)
		}
	}
	for _, d := range []int64{-2, 2} {
		if _, ok := VerifyTOTP(rfcSecret, code(cur+d), now, 0); ok {
			t.Errorf("offset %d accepted", d)
		}
	}
	if _, ok := VerifyTOTP(rfcSecret, code(cur), now, cur); ok {
		t.Error("step equal to lastStep accepted")
	}
	if _, ok := VerifyTOTP(rfcSecret, code(cur-1), now, cur); ok {
		t.Error("step below lastStep accepted")
	}
	if step, ok := VerifyTOTP(rfcSecret, code(cur+1), now, cur); !ok || step != cur+1 {
		t.Errorf("step above lastStep rejected: %d %v", step, ok)
	}
	if _, ok := VerifyTOTP(rfcSecret, "12345", now, 0); ok {
		t.Error("short code accepted")
	}
}

func TestTOTPURI(t *testing.T) {
	got := TOTPURI("ABC234", "a@b.com")
	want := "otpauth://totp/Acro%20World%20Tour%20Manager:a@b.com?secret=ABC234&issuer=Acro%20World%20Tour%20Manager&algorithm=SHA1&digits=6&period=30"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestSealOpenSecret(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 1
	sealed, err := sealSecret(key, "JBSWY3DP")
	if err != nil || !regexp.MustCompile(`^v1:[0-9a-f]{8}:[A-Za-z0-9+/=]+$`).MatchString(sealed) {
		t.Fatalf("sealed %q, %v", sealed, err)
	}
	if got, err := openSecret(key, sealed); err != nil || got != "JBSWY3DP" {
		t.Fatalf("open: %q, %v", got, err)
	}
	other, _ := sealSecret(key, "JBSWY3DP")
	if other == sealed {
		t.Fatal("nonce reused")
	}

	parts := strings.Split(sealed, ":")
	tampered := parts[0] + ":" + parts[1] + ":" + flip(parts[2])
	if _, err := openSecret(key, tampered); err == nil || errors.Is(err, ErrMFAKeyMismatch) {
		t.Fatalf("tamper: %v", err)
	}
	if _, err := openSecret(key, "garbage"); err == nil {
		t.Fatal("garbage accepted")
	}

	key2 := make([]byte, 32)
	key2[0] = 2
	if _, err := openSecret(key2, sealed); !errors.Is(err, ErrMFAKeyMismatch) {
		t.Fatalf("other key: %v", err)
	}
	if _, err := sealSecret([]byte("short"), "x"); err == nil {
		t.Fatal("short key accepted")
	}
}

// flip changes one base64 character in the middle of s.
func flip(s string) string {
	b := []byte(s)
	i := len(b) / 2
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}

func TestRecoveryCodes(t *testing.T) {
	display, hashes, err := NewRecoveryCodes()
	if err != nil || len(display) != 10 || len(hashes) != 10 {
		t.Fatalf("%d %d %v", len(display), len(hashes), err)
	}
	re := regexp.MustCompile(`^[2-9a-hjkmnp-z]{5}-[2-9a-hjkmnp-z]{5}$`)
	seen := map[string]bool{}
	for i, c := range display {
		if !re.MatchString(c) || seen[c] {
			t.Errorf("bad or duplicate code %q", c)
		}
		seen[c] = true
		if hashes[i] != HashRecoveryCode(NormaliseRecoveryCode(c)) || len(hashes[i]) != 64 {
			t.Errorf("hash mismatch for %q", c)
		}
	}
	// SHA-256("abc") known vector.
	if got := HashRecoveryCode("abc"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("hash = %s", got)
	}
}

func TestRecoveryCodeNormalisation(t *testing.T) {
	if a, b := NormaliseRecoveryCode(" ABCDE-fghjk "), NormaliseRecoveryCode("abcdefghjk"); a != b || a != "abcdefghjk" {
		t.Fatalf("%q vs %q", a, b)
	}
}

func TestCheckMFAKeyRequired(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "awt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	prod := config.Config{Env: "prod"}

	if err := CheckMFAKeyRequired(ctx, db, prod); err != nil {
		t.Fatalf("no rows: %v", err)
	}
	a := New(db, config.Config{Env: "prod"})
	a.cost = 4
	u, err := a.CreateUser(ctx, "m@example.com", "M", RoleAdmin, "s3cret-passw0rd")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`INSERT INTO user_totp (user_id, secret_enc, created_at, last_step) VALUES (?, 'x', 0, 0)`, u.ID); err != nil {
		t.Fatal(err)
	}
	if err := CheckMFAKeyRequired(ctx, db, prod); err == nil {
		t.Fatal("rows without key: want error")
	}
	if err := CheckMFAKeyRequired(ctx, db, config.Config{Env: "prod", MFAKey: key}); err != nil {
		t.Fatalf("rows with key: %v", err)
	}
	if err := CheckMFAKeyRequired(ctx, db, config.Config{Env: "dev"}); err != nil {
		t.Fatalf("dev: %v", err)
	}
}
