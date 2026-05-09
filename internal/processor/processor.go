// Package processor composes M2's HTML sanitiser with M3's image-URL
// rewriter so the polling worker has a single Process(rawHTML) string
// entry point. The sanitiser is unchanged from M2; the rewriting walk
// is a small post-pass over the cleaned HTML.
package processor

import (
	"bytes"
	"log/slog"
	"strings"

	"github.com/bcrisp4/tap/internal/sanitise"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Processor struct {
	sanitiser   *sanitise.Policy
	imgRewriter func(string) string
}

// New returns a Processor. If rewriter is nil, Process is equivalent to
// sanitiser.Sanitise (used by tests that don't care about the proxy).
func New(s *sanitise.Policy, rewriter func(string) string) *Processor {
	return &Processor{sanitiser: s, imgRewriter: rewriter}
}

// Process sanitises rawHTML, then (if a rewriter is configured) replaces
// every <img src> with the rewriter's output. Total function: never panics,
// never errors. Falls back to the sanitised-but-not-rewritten output if the
// post-pass parse fails.
func (p *Processor) Process(rawHTML string) string {
	cleaned := p.sanitiser.Sanitise(rawHTML)
	if p.imgRewriter == nil || cleaned == "" {
		return cleaned
	}
	out, err := rewriteImageURLs(cleaned, p.imgRewriter)
	if err != nil {
		slog.Error("processor.Process: rewrite failed; returning sanitised-only output", "err", err)
		return cleaned
	}
	return out
}

func rewriteImageURLs(s string, rewriter func(string) string) (string, error) {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), body)
	if err != nil {
		return "", err
	}
	root := &html.Node{Type: html.ElementNode, Data: "root"}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	walkAndRewrite(root, rewriter)

	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func walkAndRewrite(n *html.Node, rewriter func(string) string) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "img" {
			for i, a := range c.Attr {
				if a.Key == "src" {
					c.Attr[i].Val = rewriter(a.Val)
					break
				}
			}
		}
		walkAndRewrite(c, rewriter)
	}
}
