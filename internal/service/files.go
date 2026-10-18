package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"awt/internal/domain"
	"awt/internal/store/gen"
)

// maxUploadSize is the upload size limit (10 MB).
const maxUploadSize = 10 << 20

// allowedContentTypes are the uploadable types, detected from the bytes:
// never HTML or SVG (CLAUDE.md web rules).
var allowedContentTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/webp": true, "application/pdf": true,
}

// File is an uploaded file; its bytes live in the blob store under ID.
type File struct {
	ID          string // sha256 hex of the content
	Filename    string
	ContentType string
	Size        int64
	CreatedAt   string
}

// UploadFile checks the size and the sniffed content type, stores the bytes
// in the blob store and records the file. The id is the sha256 of the bytes,
// so uploading the same content twice returns the existing file.
func (s *Service) UploadFile(ctx context.Context, filename string, r io.Reader) (File, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxUploadSize+1))
	if err != nil {
		return File{}, fmt.Errorf("read upload: %w", err)
	}
	filename = filepath.Base(strings.ReplaceAll(filename, `\`, "/"))
	var errs []domain.FieldError
	if filename == "" || filename == "." || filename == "/" {
		errs = append(errs, domain.FieldError{Field: "filename", Msg: "required"})
	}
	if len(data) > maxUploadSize {
		errs = append(errs, domain.FieldError{Field: "file", Msg: "must be at most 10 MB"})
	}
	contentType := http.DetectContentType(data)
	if !allowedContentTypes[contentType] {
		errs = append(errs, domain.FieldError{Field: "file", Msg: "must be a PNG, JPEG or WebP image or a PDF, got " + contentType})
	}
	if err := invalid(errs...); err != nil {
		return File{}, err
	}
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])

	existing, err := s.read().GetFile(ctx, id)
	switch {
	case err == nil && !existing.DeletedAt.Valid:
		return fileFromRow(existing), nil
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return File{}, fmt.Errorf("get file %s: %w", id, err)
	}
	// Blob first: a row never points to missing bytes; an orphan blob after
	// a failed insert is harmless (same key on retry).
	if err := s.blobs.Put(ctx, id, contentType, bytes.NewReader(data)); err != nil {
		return File{}, fmt.Errorf("store file %s: %w", id, err)
	}
	out := gen.File{ID: id, Filename: filename, ContentType: contentType, Size: int64(len(data)), CreatedAt: now()}
	err = s.db.Tx(ctx, func(q *gen.Queries) error {
		return q.UpsertFile(ctx, gen.UpsertFileParams{
			ID: out.ID, Filename: out.Filename, ContentType: out.ContentType, Size: out.Size, CreatedAt: out.CreatedAt,
		})
	})
	if err != nil {
		return File{}, fmt.Errorf("save file %s: %w", id, err)
	}
	return fileFromRow(out), nil
}

// GetFile returns an active file's metadata.
func (s *Service) GetFile(ctx context.Context, id string) (File, error) {
	f, err := s.read().GetFile(ctx, id)
	if err != nil {
		return File{}, notFound(err, "file "+id)
	}
	if f.DeletedAt.Valid {
		return File{}, fmt.Errorf("file %s: %w", id, ErrNotFound)
	}
	return fileFromRow(f), nil
}

func fileFromRow(f gen.File) File {
	return File{ID: f.ID, Filename: f.Filename, ContentType: f.ContentType, Size: f.Size, CreatedAt: f.CreatedAt}
}
