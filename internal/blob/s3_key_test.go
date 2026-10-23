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
