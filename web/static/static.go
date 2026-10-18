// Package static embeds the CSS, fonts, images and vendored scripts, and
// serves them with content-hash cache busting.
package static

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed css fonts img js vendor
var files embed.FS

// Prefix is the URL prefix the handler is mounted on.
const Prefix = "/static/"

// hashes maps an embedded path to a short content hash. It is computed once
// at package initialisation: the embedded files never change at runtime.
var hashes = computeHashes()

func computeHashes() map[string]string {
	m := map[string]string{}
	_ = fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := files.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		m[p] = hex.EncodeToString(sum[:])[:12]
		return nil
	})
	return m
}

// URL returns the cache-busted URL of an embedded file, for example
// URL("css/app.css") -> "/static/css/app.css?v=0123456789ab". An unknown path
// gets no version so the 404 is visible instead of hidden.
func URL(p string) string {
	p = strings.TrimPrefix(p, "/")
	if h, ok := hashes[p]; ok {
		return Prefix + p + "?v=" + h
	}
	return Prefix + p
}

// Handler serves the embedded files under Prefix. A request carrying `?v=`
// is immutable for a year; others revalidate through the ETag.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := path.Clean(strings.TrimPrefix(r.URL.Path, Prefix))
		h, ok := hashes[p]
		if !ok {
			http.NotFound(w, r)
			return
		}
		b, err := files.ReadFile(p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		hd := w.Header()
		if ct := contentType(p); ct != "" {
			hd.Set("Content-Type", ct)
		}
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("ETag", `"`+h+`"`)
		if r.URL.Query().Get("v") != "" {
			hd.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			hd.Set("Cache-Control", "public, max-age=300, must-revalidate")
		}
		http.ServeContent(w, r, p, time.Time{}, bytes.NewReader(b))
	})
}

func contentType(p string) string {
	switch path.Ext(p) {
	case ".woff2":
		return "font/woff2"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".webmanifest":
		return "application/manifest+json"
	}
	return mime.TypeByExtension(path.Ext(p))
}

// Open returns an embedded file, for handlers that serve it at another URL
// (favicon.ico).
func Open(p string) ([]byte, error) { return files.ReadFile(p) }
