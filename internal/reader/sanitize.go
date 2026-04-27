package reader

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

// SanitizeOptions controls the per-call sanitizer state.
type SanitizeOptions struct {
	// ArticleURL is the base for resolving relative URLs.
	ArticleURL string
	// IframeAllowlist is the host (suffix) allowlist for <iframe src>.
	// Use DefaultIframeHosts() for the spec's defaults.
	IframeAllowlist []string
}

// DefaultIframeHosts mirrors design.md §5: the spec-default iframe
// host allowlist.
func DefaultIframeHosts() []string {
	return []string{
		"youtube.com", "youtube-nocookie.com",
		"vimeo.com", "bandcamp.com",
		"soundcloud.com", "spotify.com",
		"twitch.tv", "dailymotion.com",
	}
}

// Sanitize runs the Tap pre-pass and bluemonday over entryHTML and
// returns the cleaned body.
func Sanitize(entryHTML string, opts SanitizeOptions) (string, error) {
	pre, err := tapPreSanitize(entryHTML, opts)
	if err != nil {
		return "", err
	}
	policy := tapPolicy()
	return policy.Sanitize(pre), nil
}

// tapPreSanitize runs Tap-specific transforms bluemonday can't
// natively express:
//   - drop <iframe> whose src host doesn't match IframeAllowlist
//   - drop 1×1 image tracking pixels
//   - resolve relative URLs in href / src against ArticleURL
func tapPreSanitize(entryHTML string, opts SanitizeOptions) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(entryHTML))
	if err != nil {
		return "", err
	}
	base, _ := url.Parse(opts.ArticleURL)

	// 1×1 images.
	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		w, _ := s.Attr("width")
		h, _ := s.Attr("height")
		if w == "1" && h == "1" {
			s.Remove()
		}
	})

	// iframes whose src host isn't allowlisted.
	allowed := lowerSet(opts.IframeAllowlist)
	doc.Find("iframe").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		u, err := url.Parse(src)
		if err != nil || !u.IsAbs() {
			s.Remove()
			return
		}
		if !hostInAllowSet(u.Host, allowed) {
			s.Remove()
		}
	})

	// Resolve relative href / src.
	for _, attr := range []string{"href", "src"} {
		doc.Find("[" + attr + "]").Each(func(_ int, s *goquery.Selection) {
			v, _ := s.Attr(attr)
			s.SetAttr(attr, resolveURL(v, base))
		})
	}

	return innerBodyHTML(doc), nil
}

// tapPolicy assembles the bluemonday policy that mirrors design.md §5.
//
// UGCPolicy already covers the spec's ~50-tag allowlist, the standard
// URL schemes (http, https, mailto), and `on*` handler stripping. Tap
// layers on:
//   - tel: scheme
//   - data: scheme limited to image/* MIME types
//   - the media subset (img attrs, picture/source, video/audio, iframe)
//   - explicit removal of <script> and <style> contents
func tapPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	p.AllowURLSchemes("tel")
	p.AllowURLSchemeWithCustomPolicy("data", isImageDataURI)

	// Image attributes beyond what UGCPolicy already sets.
	p.AllowAttrs("alt", "src", "srcset", "sizes", "width", "height",
		"fetchpriority", "decoding").OnElements("img")

	// Media + reading-shell tags.
	p.AllowElements("picture", "figure", "figcaption")
	p.AllowAttrs("srcset", "src", "type", "media", "sizes",
		"width", "height").OnElements("source")
	p.AllowElements("video", "audio")
	p.AllowAttrs("controls", "src", "poster", "preload",
		"width", "height").OnElements("video", "audio")

	// iframes: bluemonday's built-in policy only allows known-good
	// embed sources. Tap pre-pass already enforced the host allowlist;
	// here we just whitelist the tag + safe attrs.
	p.AllowAttrs("src", "width", "height", "title", "loading",
		"allowfullscreen", "frameborder").OnElements("iframe")

	// Belt-and-braces: explicitly drop <script> and <style> (and their
	// contents). UGCPolicy already strips them, but stating it here
	// keeps the spec mapping obvious.
	p.SkipElementsContent("script", "style")

	return p
}

// isImageDataURI returns true iff a data: URL declares an image MIME
// type. Both forms net/url surfaces are covered: when there is no path,
// the MIME and payload land in u.Opaque (e.g. "image/png;base64,…");
// when a path is present, u.Path holds it.
func isImageDataURI(u *url.URL) bool {
	body := u.Opaque
	if body == "" {
		body = u.Path
	}
	return strings.HasPrefix(strings.ToLower(body), "image/")
}

func resolveURL(raw string, base *url.URL) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || base == nil {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() {
		return raw
	}
	return base.ResolveReference(u).String()
}

func lowerSet(list []string) map[string]struct{} {
	out := make(map[string]struct{}, len(list))
	for _, s := range list {
		out[strings.ToLower(s)] = struct{}{}
	}
	return out
}

// hostInAllowSet returns true if host is in allowed (exact match) or is
// a subdomain of any host in allowed.
func hostInAllowSet(host string, allowed map[string]struct{}) bool {
	host = strings.ToLower(host)
	if _, ok := allowed[host]; ok {
		return true
	}
	for s := range allowed {
		if strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}
