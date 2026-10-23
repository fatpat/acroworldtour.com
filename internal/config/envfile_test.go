package config_test

import (
	"maps"
	"strings"
	"testing"

	"awt/internal/config"
)

// The VPS env files (.env.prod, .env.training) as init-vps.sh writes them,
// plus the hand-edited forms docker compose also reads.
func TestParseEnvFile(t *testing.T) {
	in := `# comment
AWT_ENV=prod

AWT_S3_ENDPOINT=https://s3.gra.io.cloud.ovh.net
AWT_MFA_KEY=q83vEjRWeJASNFZ4kBI0VniQEjRWeJASNFZ4kBI0Vng=
AWT_BUCKET_PREFIX=
export AWT_S3_BUCKET=awt-prod
AWT_S3_ACCESS_KEY="quoted value"
AWT_S3_SECRET_KEY='single # not a comment'
AWT_CLOUDFLARE_ZONE_ID=zone # trailing comment
  AWT_BASE_URL = https://www.example.org  
AWT_ENV=prod
`
	got, err := config.ParseEnvFile(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"AWT_ENV":                "prod",
		"AWT_S3_ENDPOINT":        "https://s3.gra.io.cloud.ovh.net",
		"AWT_MFA_KEY":            "q83vEjRWeJASNFZ4kBI0VniQEjRWeJASNFZ4kBI0Vng=",
		"AWT_BUCKET_PREFIX":      "",
		"AWT_S3_BUCKET":          "awt-prod",
		"AWT_S3_ACCESS_KEY":      "quoted value",
		"AWT_S3_SECRET_KEY":      "single # not a comment",
		"AWT_CLOUDFLARE_ZONE_ID": "zone",
		"AWT_BASE_URL":           "https://www.example.org",
	}
	if !maps.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

// A bad line is reported by number only: env files hold secrets.
func TestParseEnvFileRejects(t *testing.T) {
	for _, in := range []string{
		"AWT_ENV=prod\nsecret-without-equals\n",
		"AWT_ENV=prod\n1BAD=x\n",
		"AWT_ENV=prod\nKEY=\"s3cr3t-value\n",
	} {
		_, err := config.ParseEnvFile(strings.NewReader(in))
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Errorf("%q: err = %v, want one naming line 2", in, err)
		}
		if err != nil && (strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "s3cr3t")) {
			t.Errorf("%q: error echoes the line: %v", in, err)
		}
	}
}
