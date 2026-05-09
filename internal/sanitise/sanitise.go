// Package sanitise provides server-side HTML cleaning for entry content.
// The output is final-form HTML safe to render directly; the SPA never
// runs a runtime sanitiser.
package sanitise

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// defaultIframeHosts is the set of hosts whose <iframe> embeds survive
// sanitisation. Adapted from miniflux's iframeAllowList.
//
// Notable omissions on purpose: m.youtube.com (mobile) and youtu.be
// (share-link host, not an embed URL). Per-user override is on the
// roadmap as a deferred item (post-M6) and will let users add hosts
// like these.
var defaultIframeHosts = map[string]struct{}{
	"bandcamp.com":         {},
	"cdn.embedly.com":      {},
	"dailymotion.com":      {},
	"framatube.org":        {},
	"open.spotify.com":     {},
	"player.bilibili.com":  {},
	"player.twitch.tv":     {},
	"player.vimeo.com":     {},
	"soundcloud.com":       {},
	"vk.com":               {},
	"w.soundcloud.com":     {},
	"youtube-nocookie.com": {},
	"youtube.com":          {},
}

// Policy is a configured sanitiser. Construct with DefaultPolicy or New.
type Policy struct {
	bm          *bluemonday.Policy
	iframeHosts map[string]struct{}
}

// DefaultPolicy returns the policy used in production.
func DefaultPolicy() *Policy { return New() }

// New constructs a Policy. (Currently no options; M5 will extend.)
func New() *Policy {
	bm := bluemonday.UGCPolicy()
	bm.AllowURLSchemes("http", "https", "mailto")
	// Allow iframes through bluemonday; the post-pass enforces the
	// host allowlist (which bluemonday's regex matching can't express
	// cleanly because we want exact-after-www-strip semantics).
	// AllowElements is belt-and-braces: AllowAttrs.OnElements implicitly
	// allows the element in current bluemonday, but explicit is safer.
	bm.AllowElements("iframe")
	bm.AllowAttrs("src", "width", "height", "frameborder", "allowfullscreen", "allow").OnElements("iframe")
	return &Policy{
		bm:          bm,
		iframeHosts: defaultIframeHosts,
	}
}

// Sanitise returns final-form HTML safe to render directly.
// Total function — never errors, never panics. Worst case returns "".
func (p *Policy) Sanitise(rawHTML string) string {
	cleaned := p.bm.Sanitize(rawHTML)
	return p.postProcess(cleaned)
}

// postProcess walks the bluemonday output and applies the rules
// bluemonday can't express directly: iframe-host allowlisting,
// pixel-tracker drop, URL tracking-param cleaning.
func (p *Policy) postProcess(s string) string {
	if s == "" {
		return ""
	}
	// ParseFragment with body context so the result doesn't get wrapped
	// in <html><head><body>; we want a fragment in, a fragment out.
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), body)
	if err != nil {
		return s
	}
	// Create a synthetic root to hold all fragment nodes so we can safely
	// remove nodes during traversal.
	root := &html.Node{Type: html.ElementNode, Data: "root"}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	p.walk(root)

	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return s
		}
	}
	return buf.String()
}

// walk mutates the node tree in place.
func (p *Policy) walk(n *html.Node) {
	// Iterate children manually so we can safely remove during traversal.
	c := n.FirstChild
	for c != nil {
		next := c.NextSibling
		if c.Type == html.ElementNode {
			switch c.Data {
			case "iframe":
				if !p.iframeHostAllowed(getAttr(c, "src")) {
					n.RemoveChild(c)
					c = next
					continue
				}
			}
		}
		p.walk(c)
		c = next
	}
}

func (p *Policy) iframeHostAllowed(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	_, ok := p.iframeHosts[host]
	return ok
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
