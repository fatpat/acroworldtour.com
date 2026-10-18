package config_test

import (
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
