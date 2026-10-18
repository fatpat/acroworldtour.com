package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

var validKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// FS stores blobs as files in a directory; the content type sits in a
// sidecar ".type" file. Used in dev and tests.
type FS struct{ dir string }

// NewFS returns a filesystem store rooted at dir, creating it if needed.
func NewFS(dir string) (*FS, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create blob dir: %w", err)
	}
	return &FS{dir: dir}, nil
}

func (f *FS) path(key string) (string, error) {
	if !validKey.MatchString(key) || key == "." || key == ".." {
		return "", fmt.Errorf("invalid blob key %q", key)
	}
	return filepath.Join(f.dir, key), nil
}

// Put writes the blob atomically (temp file then rename).
func (f *FS) Put(_ context.Context, key, contentType string, r io.Reader) error {
	p, err := f.path(key)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(f.dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("put %s: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	if err := os.WriteFile(p+".type", []byte(contentType), 0o644); err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

// Get opens the blob.
func (f *FS) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	p, err := f.path(key)
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("get %s: %w", key, err)
	}
	ct, err := os.ReadFile(p + ".type")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		_ = file.Close()
		return nil, "", fmt.Errorf("get %s: %w", key, err)
	}
	return file, string(ct), nil
}

// Delete removes the blob; a missing key is not an error.
func (f *FS) Delete(_ context.Context, key string) error {
	p, err := f.path(key)
	if err != nil {
		return err
	}
	for _, name := range []string{p, p + ".type"} {
		if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("delete %s: %w", key, err)
		}
	}
	return nil
}
