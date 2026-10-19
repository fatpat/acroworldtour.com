package config_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"awt/internal/config"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// Sessions are random tokens stored in the database: no secret is needed.
// Outside dev, AWT_BASE_URL is required (CDN purge URLs, absolute links).
func TestLoadRequiresBaseURLOutsideDev(t *testing.T) {
	for _, e := range []string{"prod", "training"} {
		_, err := config.Load(env(map[string]string{"AWT_ENV": e}))
		if err == nil || !strings.Contains(err.Error(), "AWT_BASE_URL") {
			t.Fatalf("%s without AWT_BASE_URL: err = %v, want one mentioning AWT_BASE_URL", e, err)
		}
	}
	cfg, err := config.Load(env(map[string]string{"AWT_ENV": "prod", "AWT_BASE_URL": "https://www.example.org"}))
	if err != nil {
		t.Fatalf("prod with AWT_BASE_URL and no session secret: %v", err)
	}
	if cfg.BaseURL != "https://www.example.org" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if _, err := config.Load(env(map[string]string{"AWT_ENV": "dev"})); err != nil {
		t.Fatalf("dev needs no AWT_BASE_URL: %v", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"AWT_ENV": "dev"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.DBPath != "data/awt.db" {
		t.Errorf("DBPath = %q", cfg.DBPath)
	}
	if cfg.BlobBackend != "fs" {
		t.Errorf("BlobBackend = %q", cfg.BlobBackend)
	}
	if cfg.BlobDir != "data/blobs" {
		t.Errorf("BlobDir = %q", cfg.BlobDir)
	}
}

func TestLoadRejectsUnknownEnv(t *testing.T) {
	if _, err := config.Load(env(map[string]string{"AWT_ENV": "staging"})); err == nil {
		t.Fatal("expected an error for unknown AWT_ENV")
	}
}

func TestLoadS3RequiresCredentials(t *testing.T) {
	_, err := config.Load(env(map[string]string{
		"AWT_ENV": "dev", "AWT_BLOB_BACKEND": "s3",
	}))
	if err == nil || !strings.Contains(err.Error(), "AWT_S3_ENDPOINT") {
		t.Fatalf("expected error mentioning AWT_S3_ENDPOINT, got %v", err)
	}
}

func TestLoadClientIPSettings(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"AWT_ENV": "dev", "AWT_TRUSTED_PROXIES": "10.0.0.0/8, 2001:db8::/32"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.ClientIPHeader != "CF-Connecting-IP" {
		t.Fatalf("%v %q", cfg.TrustedProxies, cfg.ClientIPHeader)
	}
	if _, err := config.Load(env(map[string]string{"AWT_ENV": "dev", "AWT_TRUSTED_PROXIES": "nope"})); err == nil {
		t.Fatal("bad CIDR accepted")
	}
	if _, err := config.Load(env(map[string]string{"AWT_ENV": "dev", "AWT_CLIENT_IP_HEADER": "X-Foo"})); err == nil {
		t.Fatal("bad header accepted")
	}
}

func TestLoadRejectsRelativeBaseURL(t *testing.T) {
	for _, u := range []string{"www.example.org", "/x", "ftp://example.org"} {
		if _, err := config.Load(env(map[string]string{"AWT_ENV": "prod", "AWT_BASE_URL": u})); err == nil {
			t.Errorf("AWT_BASE_URL %q accepted", u)
		}
	}
}

func TestLoadMFAKey(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	cfg, err := config.Load(env(map[string]string{"AWT_ENV": "prod", "AWT_BASE_URL": "https://www.example.org", "AWT_MFA_KEY": good}))
	if err != nil {
		t.Fatalf("valid key: %v", err)
	}
	if !bytes.Equal(cfg.MFAKey, bytes.Repeat([]byte{7}, 32)) {
		t.Errorf("MFAKey = %x", cfg.MFAKey)
	}
	for name, bad := range map[string]string{
		"short":      base64.StdEncoding.EncodeToString(make([]byte, 16)),
		"long":       base64.StdEncoding.EncodeToString(make([]byte, 33)),
		"not base64": "***not base64***",
	} {
		_, err := config.Load(env(map[string]string{"AWT_ENV": "dev", "AWT_MFA_KEY": bad}))
		if err == nil || !strings.Contains(err.Error(), "AWT_MFA_KEY") {
			t.Errorf("%s: err = %v, want one naming AWT_MFA_KEY", name, err)
		}
	}
	dev, err := config.Load(env(map[string]string{"AWT_ENV": "dev"}))
	if err != nil {
		t.Fatalf("dev without key: %v", err)
	}
	want := sha256.Sum256([]byte("awt-dev-mfa-key"))
	if !bytes.Equal(dev.MFAKey, want[:]) {
		t.Errorf("dev MFAKey = %x, want sha256 of awt-dev-mfa-key", dev.MFAKey)
	}
	// Outside dev the key stays empty when unset: serve enforces it when TOTP rows exist.
	prod, err := config.Load(env(map[string]string{"AWT_ENV": "prod", "AWT_BASE_URL": "https://www.example.org"}))
	if err != nil || len(prod.MFAKey) != 0 {
		t.Errorf("prod without key: key=%x err=%v, want empty and no error", prod.MFAKey, err)
	}
}

func TestLoadWebAuthnRPID(t *testing.T) {
	prod := map[string]string{"AWT_ENV": "prod", "AWT_BASE_URL": "https://www.example.org:8443"}
	cfg, err := config.Load(env(prod))
	if err != nil || cfg.WebAuthnRPID != "www.example.org" {
		t.Fatalf("default: rpid=%q err=%v, want www.example.org", cfg.WebAuthnRPID, err)
	}
	prod["AWT_WEBAUTHN_RP_ID"] = "example.org"
	cfg, err = config.Load(env(prod))
	if err != nil || cfg.WebAuthnRPID != "example.org" {
		t.Fatalf("override: rpid=%q err=%v", cfg.WebAuthnRPID, err)
	}
	cfg, err = config.Load(env(map[string]string{"AWT_ENV": "dev"}))
	if err != nil || cfg.WebAuthnRPID != "localhost" {
		t.Fatalf("dev default: rpid=%q err=%v, want localhost", cfg.WebAuthnRPID, err)
	}
}
