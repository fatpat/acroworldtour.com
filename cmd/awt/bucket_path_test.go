package main

import (
	"bytes"
	"strings"
	"testing"

	"awt/internal/config"
)

func TestBucketPathLitestream(t *testing.T) {
	for _, c := range []struct{ prefix, want string }{
		{"", "litestream\n"},
		{"prod/", "prod/litestream\n"},
		{"training", "training/litestream\n"},
		{"/awt/prod/", "awt/prod/litestream\n"},
	} {
		var out bytes.Buffer
		getenv := func(k string) string {
			if k == "AWT_BUCKET_PREFIX" {
				return c.prefix
			}
			return ""
		}
		if err := bucketPathCmd([]string{"litestream"}, getenv, &out); err != nil {
			t.Fatalf("prefix %q: %v", c.prefix, err)
		}
		if out.String() != c.want {
			t.Errorf("prefix %q: printed %q, want %q", c.prefix, out.String(), c.want)
		}
	}
}

// The entrypoint relies on a non-zero exit to stop the container.
func TestBucketPathRefuses(t *testing.T) {
	bad := func(k string) string {
		if k == "AWT_BUCKET_PREFIX" {
			return "../prod"
		}
		return ""
	}
	var out bytes.Buffer
	if err := bucketPathCmd([]string{"litestream"}, bad, &out); err == nil || !strings.Contains(err.Error(), "AWT_BUCKET_PREFIX") {
		t.Errorf("invalid prefix: got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("invalid prefix printed %q", out.String())
	}
	none := func(string) string { return "" }
	for _, args := range [][]string{nil, {"files"}, {"litestream", "x"}} {
		if err := bucketPathCmd(args, none, &out); err == nil {
			t.Errorf("args %q: want a usage error", args)
		}
	}
}

// AWT_BUCKET_PREFIX=prod (no slash) reaches the S3 store as "prod/", which
// builds prod/files/<sha256> (internal/blob TestS3ObjectKey).
func TestS3ConfigPrefix(t *testing.T) {
	cfg, err := config.Load(func(k string) string {
		return map[string]string{"AWT_ENV": "dev", "AWT_BUCKET_PREFIX": "prod"}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := s3Config(cfg).Prefix; got != "prod/" {
		t.Errorf("S3Config.Prefix = %q, want %q", got, "prod/")
	}
}
