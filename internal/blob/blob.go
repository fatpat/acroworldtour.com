// Package blob stores uploaded files (photos, logos, PDFs) outside the
// database, in object storage or on the local filesystem.
package blob

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound is returned by Get when the key does not exist.
var ErrNotFound = errors.New("blob not found")

// Store is a key/value byte store. Keys are file ids (sha256 hex).
type Store interface {
	Put(ctx context.Context, key, contentType string, r io.Reader) error
	// Get returns the content and its content type.
	Get(ctx context.Context, key string) (io.ReadCloser, string, error)
	// Delete is idempotent: deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error
}
