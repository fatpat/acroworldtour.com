// Package config parses the environment into a typed Config once at startup.
package config

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// Config holds every runtime setting. Environment variables are AWT_ plus the
// upper snake case of the field name (Addr -> AWT_ADDR).
type Config struct {
	Addr             string
	DBPath           string
	BlobBackend      string // "fs" or "s3"
	BlobDir          string
	S3Endpoint       string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	BaseURL          string
	Env              string // "dev", "training" or "prod"
	CloudflareZoneID string
	CloudflareToken  string
	// TrustedProxies are the peers allowed to set ClientIPHeader
	// (AWT_TRUSTED_PROXIES, comma-separated CIDRs; empty by default).
	TrustedProxies []netip.Prefix
	// ClientIPHeader is CF-Connecting-IP (default) or X-Forwarded-For.
	ClientIPHeader string
	// MFAKey is the 32-byte key sealing TOTP secrets (AWT_MFA_KEY, base64).
	// Dev without the variable gets a fixed key derived from a constant;
	// outside dev it stays empty and serve requires it once any TOTP row
	// exists.
	MFAKey []byte
	// WebAuthnRPID is the WebAuthn relying party ID (AWT_WEBAUTHN_RP_ID):
	// default is the host of AWT_BASE_URL, localhost in dev without one.
	WebAuthnRPID string
}

// Load reads the configuration through getenv and refuses to start on a
// missing or invalid required value.
func Load(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}
	cfg := Config{
		Addr:             get("AWT_ADDR", ":8080"),
		DBPath:           get("AWT_DB_PATH", "data/awt.db"),
		BlobBackend:      get("AWT_BLOB_BACKEND", "fs"),
		BlobDir:          get("AWT_BLOB_DIR", "data/blobs"),
		S3Endpoint:       getenv("AWT_S3_ENDPOINT"),
		S3Bucket:         getenv("AWT_S3_BUCKET"),
		S3AccessKey:      getenv("AWT_S3_ACCESS_KEY"),
		S3SecretKey:      getenv("AWT_S3_SECRET_KEY"),
		BaseURL:          getenv("AWT_BASE_URL"),
		Env:              get("AWT_ENV", "dev"),
		CloudflareZoneID: getenv("AWT_CLOUDFLARE_ZONE_ID"),
		CloudflareToken:  getenv("AWT_CLOUDFLARE_TOKEN"),
		ClientIPHeader:   get("AWT_CLIENT_IP_HEADER", "CF-Connecting-IP"),
	}
	for c := range strings.SplitSeq(getenv("AWT_TRUSTED_PROXIES"), ",") {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return Config{}, fmt.Errorf("AWT_TRUSTED_PROXIES %q: %w", c, err)
		}
		cfg.TrustedProxies = append(cfg.TrustedProxies, p.Masked())
	}
	switch cfg.ClientIPHeader {
	case "CF-Connecting-IP", "X-Forwarded-For":
	default:
		return Config{}, fmt.Errorf("AWT_CLIENT_IP_HEADER %q: must be CF-Connecting-IP or X-Forwarded-For", cfg.ClientIPHeader)
	}

	switch cfg.Env {
	case "dev", "training", "prod":
	default:
		return Config{}, fmt.Errorf("AWT_ENV %q: must be dev, training or prod", cfg.Env)
	}
	// Sessions are random tokens stored in the database, so no signing
	// secret exists. The public base URL builds the CDN purge URLs.
	if cfg.Env != "dev" && cfg.BaseURL == "" {
		return Config{}, errors.New("AWT_BASE_URL is required when AWT_ENV is not dev")
	}
	if cfg.BaseURL != "" {
		u, err := url.Parse(cfg.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Config{}, fmt.Errorf("AWT_BASE_URL %q: must be an absolute http(s) URL", cfg.BaseURL)
		}
	}

	switch cfg.BlobBackend {
	case "fs":
	case "s3":
		required := []struct{ name, value string }{
			{"AWT_S3_ENDPOINT", cfg.S3Endpoint},
			{"AWT_S3_BUCKET", cfg.S3Bucket},
			{"AWT_S3_ACCESS_KEY", cfg.S3AccessKey},
			{"AWT_S3_SECRET_KEY", cfg.S3SecretKey},
		}
		for _, r := range required {
			if r.value == "" {
				return Config{}, fmt.Errorf("%s is required when AWT_BLOB_BACKEND is s3", r.name)
			}
		}
	default:
		return Config{}, fmt.Errorf("AWT_BLOB_BACKEND %q: must be fs or s3", cfg.BlobBackend)
	}
	if v := getenv("AWT_MFA_KEY"); v != "" {
		key, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return Config{}, errors.New("AWT_MFA_KEY: must be base64 (generate with: openssl rand -base64 32)")
		}
		if len(key) != 32 {
			return Config{}, fmt.Errorf("AWT_MFA_KEY: decoded length is %d bytes, must be 32", len(key))
		}
		cfg.MFAKey = key
	} else if cfg.Env == "dev" {
		sum := sha256.Sum256([]byte("awt-dev-mfa-key"))
		cfg.MFAKey = sum[:]
	}
	cfg.WebAuthnRPID = getenv("AWT_WEBAUTHN_RP_ID")
	if cfg.WebAuthnRPID == "" {
		cfg.WebAuthnRPID = "localhost"
		if cfg.BaseURL != "" {
			u, _ := url.Parse(cfg.BaseURL) // validated above
			cfg.WebAuthnRPID = u.Hostname()
		}
	}
	return cfg, nil
}

// Origins are the browser origins the manager is served from: the origin of
// AWT_BASE_URL; in dev without one, the local addresses of AWT_ADDR. Never
// derived from a request's Host header, which the client controls. The
// login steps' Origin check and the WebAuthn relying party use them.
func (c Config) Origins() []string {
	if c.BaseURL != "" {
		u, err := url.Parse(c.BaseURL)
		if err != nil {
			return nil
		}
		return []string{strings.ToLower(u.Scheme + "://" + u.Host)}
	}
	if c.Env != "dev" {
		return nil
	}
	_, port, err := net.SplitHostPort(c.Addr)
	if err != nil {
		return nil
	}
	return []string{"http://localhost:" + port, "http://127.0.0.1:" + port}
}
