package public

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"

	"awt/internal/blob"
	"awt/internal/store/gen"
	"awt/internal/web"
)

// fileID is a stored file id: the sha256 of the content, lowercase hex.
var fileID = regexp.MustCompile(`^[0-9a-f]{64}$`)

// inlineTypes are shown in the browser; everything else downloads
// (CLAUDE.md: never serve user-uploaded HTML or SVG inline).
var inlineTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}

func registerFiles(r *web.Router) {
	f := &files{q: gen.New(r.Deps().DB.Read), blobs: r.Deps().Blobs}
	r.Public("GET /files/{id}", http.HandlerFunc(f.serve))
}

type files struct {
	q     *gen.Queries
	blobs blob.Store
}

// serve streams a stored file with the content type recorded in the files
// row. Ids are content addresses, so responses are immutable.
func (f *files) serve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !fileID.MatchString(id) || f.blobs == nil {
		notFound(w, r)
		return
	}
	row, err := f.q.PublicGetActiveFile(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	body, _, err := f.blobs.Get(r.Context(), id)
	if errors.Is(err, blob.ErrNotFound) {
		notFound(w, r)
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer body.Close()
	disposition := "attachment"
	if inlineTypes[row.ContentType] {
		disposition = "inline"
	}
	h := w.Header()
	h.Set("Content-Type", row.ContentType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": row.Filename}))
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	if _, err := io.Copy(w, body); err != nil {
		slog.Warn("serve file", "id", id, "err", err)
	}
}
