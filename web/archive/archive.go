// Package archive embeds the frozen 2022-2023 result pages of the legacy
// site (spec 07 §2.4) with the few assets they need. They are public
// historical documents served as they were, except for the Google Analytics
// snippet, which is removed: analytics is an open owner decision (consent).
package archive

import (
	"awt/web/static"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"
)

//go:embed result assets
var files embed.FS

// gtag matches the legacy analytics loader and its inline configuration.
var gtag = regexp.MustCompile(`(?s)<!--\s*Google tag.*?-->\s*<script async src="https://www\.googletagmanager\.com[^>]*></script>\s*<script>.*?</script>`)

// fontTypes covers types a minimal system mime table may lack.
var fontTypes = map[string]string{".woff2": "font/woff2", ".woff": "font/woff"}

// pageFixes point the head icons the legacy host served from its root at the
// app's own copies, and drop the one without an equivalent.
var pageFixes = strings.NewReplacer(
	`href="/apple-touch-icon.png"`, `href="`+static.URL("img/apple-touch-icon.png")+`"`,
	`href="/favicon-32x32.png"`, `href="`+static.URL("img/favicon-32x32.png")+`"`,
	`href="/favicon-16x16.png"`, `href="`+static.URL("img/favicon-16x16.png")+`"`,
	`href="/site.webmanifest"`, `href="`+static.URL("img/site.webmanifest")+`"`,
)

var maskIcon = regexp.MustCompile(`[ \t]*<link rel="mask-icon"[^>]*>\r?\n?`)

func fixPage(b []byte) []byte {
	return maskIcon.ReplaceAll([]byte(pageFixes.Replace(string(b))), nil)
}

// cssFix drops the hero photo reference: header.jpg is not shipped (850 KB,
// only the home page used it).
var cssFix = strings.NewReplacer(`url('/assets/img/header.jpg')`, `none`)

var pages = loadPages()

func loadPages() map[string][]byte {
	m := map[string][]byte{}
	_ = fs.WalkDir(files, "result", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		b, err := files.ReadFile(p)
		if err != nil {
			return err
		}
		m[strings.TrimSuffix(strings.TrimPrefix(p, "result/"), ".html")] = fixPage(gtag.ReplaceAll(b, nil))
		return nil
	})
	return m
}

// Paths returns the archived pages as "year/slug", for the sitemap.
func Paths() []string {
	out := make([]string, 0, len(pages))
	for k := range pages {
		out = append(out, k)
	}
	return out
}

// Page returns the HTML of "year/slug".
func Page(key string) ([]byte, bool) {
	b, ok := pages[key]
	return b, ok
}

// Asset serves the CSS, fonts, flags and icons the archived pages use,
// mounted at /assets/. Unknown paths go to notFound (which may redirect).
func Asset(notFound http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		b, err := files.ReadFile(strings.TrimPrefix(p, "/"))
		if err == nil && path.Ext(p) == ".css" {
			b = []byte(cssFix.Replace(string(b)))
		}
		if err != nil || !strings.HasPrefix(p, "/assets/") {
			notFound(w, r)
			return
		}
		ct := mime.TypeByExtension(path.Ext(p))
		if ct == "" {
			ct = fontTypes[path.Ext(p)]
		}
		if ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write(b)
	})
}
