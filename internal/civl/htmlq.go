package civl

import (
	"strings"

	"golang.org/x/net/html"
)

// step is one descendant-combinator step of a minimal css selector:
// tag, .class and #id (all optional).
type step struct{ tag, class, id string }

func (s step) match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if s.tag != "" && n.Data != s.tag {
		return false
	}
	if s.id != "" && attr(n, "id") != s.id {
		return false
	}
	if s.class != "" {
		ok := false
		for _, c := range strings.Fields(attr(n, "class")) {
			if c == s.class {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// walk visits n and its descendants in document order; fn returns false to stop.
func walk(n *html.Node, fn func(*html.Node) bool) bool {
	if !fn(n) {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !walk(c, fn) {
			return false
		}
	}
	return true
}

// all returns the elements under root matching the descendant chain, in
// document order and without duplicates.
func all(root *html.Node, steps ...step) []*html.Node {
	cur := []*html.Node{root}
	for _, s := range steps {
		seen := map[*html.Node]bool{}
		var next []*html.Node
		for _, r := range cur {
			for c := r.FirstChild; c != nil; c = c.NextSibling {
				walk(c, func(n *html.Node) bool {
					if s.match(n) && !seen[n] {
						seen[n] = true
						next = append(next, n)
					}
					return true
				})
			}
		}
		cur = next
	}
	return cur
}

func first(root *html.Node, steps ...step) *html.Node {
	if r := all(root, steps...); len(r) > 0 {
		return r[0]
	}
	return nil
}

// text is the concatenated text content of n (lxml text_content / itertext).
func text(n *html.Node) string {
	var sb strings.Builder
	walk(n, func(c *html.Node) bool {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
		return true
	})
	return sb.String()
}
