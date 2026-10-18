package export

import (
	"bytes"
	_ "embed"
	"encoding/xml"
	"strconv"
	"strings"
)

// overlayTemplate is the legacy 1920x1080 lower-third (legacy
// api/templates/live-overlay-run-pilot.svg) with %{RUN}, %{PILOT}, %{RANK},
// %{SCORE} and %{COUNTRY} placeholders.
//
//go:embed overlay.svg
var overlayTemplate string

// Overlay is the data of a broadcast lower-third.
type Overlay struct {
	Run     int
	Name    string // pilot or team name
	Country string
	Rank    int64 // 0 when unknown
	Score   string
}

// svgText escapes s for an XML text node (the legacy overlay interpolated
// names unescaped, which allowed SVG injection).
func svgText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// RenderOverlay fills the lower-third template. Names are upper-cased like
// the legacy overlay; every text is XML-escaped.
func RenderOverlay(o Overlay) []byte {
	rank := ""
	if o.Rank > 0 {
		rank = "#" + strconv.FormatInt(o.Rank, 10)
	}
	// strings.NewReplacer is single pass: inserted text is never rescanned.
	return []byte(strings.NewReplacer(
		"%{RUN}", svgText("RUN "+strconv.Itoa(o.Run)),
		"%{PILOT}", svgText(strings.ToUpper(o.Name)),
		"%{RANK}", svgText(rank),
		"%{SCORE}", svgText(o.Score),
		"%{COUNTRY}", svgText(strings.ToUpper(o.Country)),
	).Replace(overlayTemplate))
}

// EmptyOverlay is a transparent 1920x1080 canvas, served when there is
// nothing to show yet so a broadcast browser source keeps working.
func EmptyOverlay() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<svg xmlns="http://www.w3.org/2000/svg" width="1920" height="1080" viewBox="0 0 1920 1080"/>` + "\n")
}
