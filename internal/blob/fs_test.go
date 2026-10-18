package blob_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"awt/internal/blob"
)

func TestFSPutGetDelete(t *testing.T) {
	ctx := context.Background()
	s, err := blob.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, "abc123", "image/png", strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	rc, ct, err := s.Get(ctx, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "hello" || ct != "image/png" {
		t.Fatalf("got %q %q", b, ct)
	}

	if err := s.Put(ctx, "abc123", "text/plain", strings.NewReader("v2")); err != nil {
		t.Fatal(err)
	}
	rc, ct, _ = s.Get(ctx, "abc123")
	b, _ = io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "v2" || ct != "text/plain" {
		t.Fatalf("overwrite got %q %q", b, ct)
	}

	if err := s.Delete(ctx, "abc123"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Get(ctx, "abc123"); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
	if err := s.Delete(ctx, "abc123"); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestFSRejectsPathTraversal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, _ := blob.NewFS(dir)
	for _, key := range []string{"../evil", "a/b", "", ".."} {
		if err := s.Put(ctx, key, "x", strings.NewReader("x")); err == nil {
			t.Errorf("key %q accepted", key)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("files written: %v", entries)
	}
}

func TestS3Live(t *testing.T) {
	if os.Getenv("AWT_S3_TEST") != "1" {
		t.Skip("set AWT_S3_TEST=1 with AWT_S3_ENDPOINT/BUCKET/ACCESS_KEY/SECRET_KEY to run")
	}
	s, err := blob.NewS3(blob.S3Config{
		Endpoint:  os.Getenv("AWT_S3_ENDPOINT"),
		Bucket:    os.Getenv("AWT_S3_BUCKET"),
		AccessKey: os.Getenv("AWT_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("AWT_S3_SECRET_KEY"),
		UseSSL:    os.Getenv("AWT_S3_INSECURE") != "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.Put(ctx, "awt-test-key", "text/plain", strings.NewReader("hi")); err != nil {
		t.Fatal(err)
	}
	rc, ct, err := s.Get(ctx, "awt-test-key")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "hi" || ct != "text/plain" {
		t.Fatalf("got %q %q", b, ct)
	}
	if err := s.Delete(ctx, "awt-test-key"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Get(ctx, "awt-test-key"); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}
