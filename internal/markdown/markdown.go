// Package markdown renders editor-written Markdown (news, events, pages) to
// HTML. Raw HTML in the source is never passed through (goldmark's default;
// html.WithUnsafe is deliberately not enabled, decision 0010) and links with
// dangerous schemes (javascript:, vbscript:, data:) are neutralised, so
// content cannot inject scripts.
package markdown

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.Strikethrough, extension.Table),
	goldmark.WithRendererOptions(html.WithHardWraps()),
)

// HTML converts Markdown to safe HTML.
func HTML(src string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		// goldmark only fails on a failing writer; a bytes.Buffer never does.
		return ""
	}
	return buf.String()
}

// Plain returns the text of src for meta descriptions: Markdown markers are
// dropped, whitespace collapsed and the result cut to max runes at a word.
func Plain(src string, max int) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#>*-+ ")
		if line == "" || strings.HasPrefix(line, "![") {
			continue
		}
		b.WriteString(line)
		b.WriteByte(' ')
	}
	text := strings.NewReplacer("**", "", "__", "", "`", "", "*", "").Replace(b.String())
	text = stripLinks(text)
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	if len(r) <= max {
		return text
	}
	cut := string(r[:max])
	if i := strings.LastIndexByte(cut, ' '); i > max/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:.") + "…"
}

// stripLinks turns [label](url) into label.
func stripLinks(s string) string {
	for {
		i := strings.Index(s, "](")
		if i < 0 {
			return s
		}
		open := strings.LastIndexByte(s[:i], '[')
		end := strings.IndexByte(s[i:], ')')
		if open < 0 || end < 0 {
			return s
		}
		s = s[:open] + s[open+1:i] + s[i+end+1:]
	}
}
