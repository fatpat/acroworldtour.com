package domain

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Slug derives a competition code from a name (spec 02 §2.5.2): NFD, drop
// non-ASCII, lower-case, keep [a-z0-9 ], trim, each space becomes "-".
func Slug(name string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(name) {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == ' ':
			b.WriteRune(r)
		}
	}
	return strings.ReplaceAll(strings.TrimSpace(b.String()), " ", "-")
}
