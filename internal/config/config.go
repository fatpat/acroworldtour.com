// Package config parses the environment into a typed Config once at startup.
package config

import (
	"errors"
	"fmt"
	"net/netip"
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
	SessionSecret    string
	// TrustedProxies are the peers allowed to set ClientIPHeader
	// (AWT_TRUSTED_PROXIES, comma-separated CIDRs; empty by default).
	TrustedProxies []netip.Prefix
	// ClientIPHeader is CF-Connecting-IP (default) or X-Forwarded-For.
	ClientIPHeader string
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
		SessionSecret:    getenv("AWT_SESSION_SECRET"),
		ClientIPHeader:   get("AWT_CLIENT_IP_HEADER", "CF-Connecting-IP"),
	}
	for _, c := range strings.Split(getenv("AWT_TRUSTED_PROXIES"), ",") {
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
	if cfg.Env != "dev" && cfg.SessionSecret == "" {
		return Config{}, errors.New("AWT_SESSION_SECRET is required when AWT_ENV is not dev")
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
	return cfg, nil
}
