package blob

import "testing"

func TestS3ObjectKey(t *testing.T) {
	const id = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	if got, want := objectKey(id), "files/"+id; got != want {
		t.Fatalf("objectKey(%q) = %q, want %q", id, got, want)
	}
}
