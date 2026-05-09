// Package sanitise provides server-side HTML cleaning for entry content.
// The output is final-form HTML safe to render directly; the SPA never
// runs a runtime sanitiser.
package sanitise

import (
	"bytes"
	"log/slog"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/bcrisp4/tap/internal/urlcleaner"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// MaxInputBytes caps the raw-HTML input to Sanitise. Defence-in-depth on
// top of feed.Fetch's 10 MiB body cap; per-entry HTML beyond this is
// pathological. Truncation snaps back to a UTF-8 codepoint boundary so
// bluemonday never sees a partial multi-byte sequence.
const MaxInputBytes = 1 << 20 // 1 MiB

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

type walkStats struct {
	droppedIframes       int
	droppedPixelTrackers int
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
	bm.AllowAttrs("width", "height").OnElements("img")
	return &Policy{
		bm:          bm,
		iframeHosts: defaultIframeHosts,
	}
}

// Sanitise returns final-form HTML safe to render directly.
// Total function — never errors, never panics. Worst case returns "".
func (p *Policy) Sanitise(rawHTML string) string {
	truncated := false
	if len(rawHTML) > MaxInputBytes {
		rawHTML = rawHTML[:MaxInputBytes]
		// Snap back to a UTF-8 boundary. UTF-8 codepoints are at most
		// 4 bytes; this loop runs at most 3 times.
		for len(rawHTML) > 0 && !utf8.ValidString(rawHTML) {
			rawHTML = rawHTML[:len(rawHTML)-1]
		}
		truncated = true
	}
	cleaned := p.bm.Sanitize(rawHTML)
	out, stats := p.postProcess(cleaned)
	if truncated || stats.droppedIframes > 0 || stats.droppedPixelTrackers > 0 {
		slog.Debug("sanitise",
			"input_bytes", len(rawHTML),
			"output_bytes", len(out),
			"truncated", truncated,
			"dropped_iframes", stats.droppedIframes,
			"dropped_pixel_trackers", stats.droppedPixelTrackers,
		)
	}
	return out
}

// postProcess walks the bluemonday output and applies the rules
// bluemonday can't express directly: iframe-host allowlisting,
// pixel-tracker drop, URL tracking-param cleaning.
func (p *Policy) postProcess(s string) (string, walkStats) {
	var stats walkStats
	if s == "" {
		return "", stats
	}
	// ParseFragment with body context so the result doesn't get wrapped
	// in <html><head><body>; we want a fragment in, a fragment out.
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), body)
	if err != nil {
		// Worst-case path per Sanitise's contract: better to lose
		// content than to render unwalked HTML.
		slog.Error("sanitise.postProcess: html.ParseFragment failed", "err", err)
		return "", stats
	}
	// Create a synthetic root to hold all fragment nodes so we can safely
	// remove nodes during traversal.
	root := &html.Node{Type: html.ElementNode, Data: "root"}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	p.walk(root, &stats)

	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			slog.Error("sanitise.postProcess: html.Render failed", "err", err)
			return "", stats
		}
	}
	return buf.String(), stats
}

// walk mutates the node tree in place.
func (p *Policy) walk(n *html.Node, stats *walkStats) {
	// Iterate children manually so we can safely remove during traversal.
	c := n.FirstChild
	for c != nil {
		next := c.NextSibling
		if c.Type == html.ElementNode {
			switch c.Data {
			case "iframe":
				if !p.iframeHostAllowed(getAttr(c, "src")) {
					n.RemoveChild(c)
					stats.droppedIframes++
					c = next
					continue
				}
			case "img":
				if isPixelTracker(c) {
					n.RemoveChild(c)
					stats.droppedPixelTrackers++
					c = next
					continue
				}
				cleanAttrURL(c, "src")
			case "a":
				cleanAttrURL(c, "href")
			}
		}
		p.walk(c, stats)
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

// isPixelTracker matches <img> whose width AND height attributes are
// both "0" or "1". Mirrors miniflux's heuristic
// (internal/reader/sanitizer/sanitizer.go isPixelTracker).
//
// Known gaps (intentional — concept doc scopes us to "obvious" trackers):
// CSS-styled trackers (<img style="width:1px">), 2x2 pixels, and naked
// <img> with no dims that the browser sizes from a 1x1 source bitmap
// all survive. Expanding the heuristic risks false positives on
// legitimate content (icons, spacers).
func isPixelTracker(n *html.Node) bool {
	w := getAttr(n, "width")
	h := getAttr(n, "height")
	if w == "" || h == "" {
		return false
	}
	return (w == "0" || w == "1") && (h == "0" || h == "1")
}

// cleanAttrURL strips tracking parameters from the named attribute's
// URL value, in place. No-op if the attribute is missing.
//
// Scope: M2 covers <a href> and <img src> only. UGCPolicy may also
// permit <source src/srcset>, <video src/poster>, <audio src>, etc.;
// tracking parameters in those URLs are not yet cleaned. Add cases
// here if a real feed surfaces survivors.
func cleanAttrURL(n *html.Node, key string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = urlcleaner.Clean(a.Val)
			return
		}
	}
}
