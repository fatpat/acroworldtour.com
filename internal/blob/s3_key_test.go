package blob

import "testing"

// The prefix comes normalised from config (AWT_BUCKET_PREFIX, decision 0017):
// "" or "<segments>/". "prod" without a slash is normalised there to "prod/".
func TestS3ObjectKey(t *testing.T) {
	const id = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	for _, c := range []struct{ prefix, want string }{
		{"", "files/" + id},
		{"prod/", "prod/files/" + id},
		{"awt/training/", "awt/training/files/" + id},
	} {
		s, err := NewS3(S3Config{Endpoint: "localhost:9000", Bucket: "b", Prefix: c.prefix})
		if err != nil {
			t.Fatal(err)
		}
		if got := s.objectKey(id); got != c.want {
			t.Errorf("prefix %q: objectKey = %q, want %q", c.prefix, got, c.want)
		}
	}
}

// AWT_S3_ENDPOINT carries a scheme on the VPS (https://s3.gra.io.cloud.ovh.net,
// docs/ops.md section 2: Litestream needs it) and http://minio:9000 in the
// local drills; the scheme then decides TLS. A bare host keeps UseSSL.
func TestS3Endpoint(t *testing.T) {
	for _, c := range []struct {
		endpoint string
		useSSL   bool
		want     string
	}{
		{"https://s3.gra.io.cloud.ovh.net", false, "https://s3.gra.io.cloud.ovh.net"},
		{"https://s3.gra.io.cloud.ovh.net/", false, "https://s3.gra.io.cloud.ovh.net"},
		{"http://minio:9000", true, "http://minio:9000"},
		{"localhost:9000", false, "http://localhost:9000"},
		{"s3.example.org", true, "https://s3.example.org"},
	} {
		s, err := NewS3(S3Config{Endpoint: c.endpoint, Bucket: "b", UseSSL: c.useSSL})
		if err != nil {
			t.Fatalf("%q: %v", c.endpoint, err)
		}
		u := s.client.EndpointURL()
		if got := u.Scheme + "://" + u.Host; got != c.want {
			t.Errorf("%q (UseSSL %v): endpoint %q, want %q", c.endpoint, c.useSSL, got, c.want)
		}
	}
	for _, bad := range []string{"ftp://s3.example.org", "https://s3.example.org/bucket", "https://"} {
		if _, err := NewS3(S3Config{Endpoint: bad, Bucket: "b"}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
