// Package layout holds the templ page shells: base, public and admin, plus
// the built-in pages (errors, login, admin home).
package layout

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"awt/internal/auth"

	"github.com/a-h/templ"
)

// Meta is the per-page <head> data. Every public page sets at least Title and
// Description (CLAUDE.md web rules: real title, meta and Open Graph tags).
type Meta struct {
	Title       string
	Description string
	// Canonical is an absolute URL; empty omits the tag.
	Canonical string
	// OGImage is an absolute URL; empty omits og:image.
	OGImage string
	// OGType defaults to "website".
	OGType string
	// NoIndex asks robots not to index the page (admin, errors).
	NoIndex bool
	// Heading, when set, renders the page header with an h1.
	Heading string
}

const siteName = "Acro World Tour"

func (m Meta) fullTitle() string {
	if m.Title == "" {
		return siteName
	}
	return m.Title + " | " + siteName
}

func (m Meta) ogType() string {
	if m.OGType == "" {
		return "website"
	}
	return m.OGType
}

// htmxAttrs gives <body> the hx-headers attribute that makes every htmx
// request carry the session CSRF token (auth.CSRF reads X-CSRF-Token). It is
// empty without a session.
func htmxAttrs(ctx context.Context) templ.Attributes {
	tok := auth.CSRFToken(ctx)
	if tok == "" {
		return nil
	}
	b, _ := json.Marshal(map[string]string{"X-CSRF-Token": tok})
	return templ.Attributes{"hx-headers": string(b)}
}

func csrfToken(ctx context.Context) string { return auth.CSRFToken(ctx) }

// copyright is the footer line; the year is rendered server side (the legacy
// site did it with JavaScript).
func copyright() string {
	return "© Copyright " + strconv.Itoa(time.Now().Year()) + " Acro World Tour. All Rights Reserved."
}

func itoa(n int) string { return strconv.Itoa(n) }
