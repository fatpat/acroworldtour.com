package markdown

import (
	"strings"
	"testing"
)

func TestHTMLDoesNotRenderRawHTML(t *testing.T) {
	for _, src := range []string{
		"hello <script>alert(1)</script>",
		"<img src=x onerror=alert(1)>",
		"[x](javascript:alert(1))",
		"<iframe src=//evil></iframe>",
	} {
		out := HTML(src)
		for _, bad := range []string{"<script", "<iframe", "onerror=alert", "href=\"javascript:"} {
			if strings.Contains(strings.ToLower(out), bad) {
				t.Errorf("HTML(%q) = %q contains %q", src, out, bad)
			}
		}
	}
}

func TestHTMLRendersMarkdown(t *testing.T) {
	out := HTML("# Title\n\nSome **bold** [link](https://example.org) and ![alt](/files/abc).\n\n- one\n- two\n")
	for _, want := range []string{"<h1>Title</h1>", "<strong>bold</strong>", `<a href="https://example.org">link</a>`, `<img src="/files/abc" alt="alt"`, "<li>one</li>"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

func TestPlain(t *testing.T) {
	got := Plain("# Head\n\n**Bold** text with [a link](https://x.y/z) and more words to cut off here", 30)
	if got != "Head Bold text with a link…" {
		t.Errorf("Plain = %q", got)
	}
}
