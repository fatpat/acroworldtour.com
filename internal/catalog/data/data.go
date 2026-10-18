// Package data embeds the per-year trick catalogs (<year>.json, public data)
// so that imports do not depend on the working directory.
package data

import "embed"

// FS holds <year>.json for 2022 to 2026.
//
//go:embed *.json
var FS embed.FS
