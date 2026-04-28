// Package iconfetch discovers and fetches a feed's favicon.
//
// Strategy (Plan 17 option A):
//  1. If the feed parses an article with a usable site URL, look for
//     <link rel="icon"> / <link rel="shortcut icon"> / <link rel="apple-touch-icon">.
//  2. Otherwise (or if no link tag is found), fall back to /favicon.ico
//     at the site's origin.
//
// The package is intentionally small and stateless. Fetcher uses the
// shared SSRF-safe httpclient so any future host-allowlist work
// applies to favicon scrapes too. Bytes are capped at maxBodyBytes
// to keep favicon storage bounded.
package iconfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"

	"github.com/bcrisp4/tap/internal/httpclient"
)

// maxBodyBytes caps how much of an icon we'll read into memory + the
// database. Real favicons sit comfortably under 32 KiB; the 256 KiB
// ceiling is a safety net, not a target.
const maxBodyBytes = 256 * 1024

// allowedMIMETypes is the closed set we'll persist. Anything else is
// dropped on the floor — the SPA only renders <img>, so SVG / PNG /
// ICO / JPEG / WebP cover every realistic feed.
var allowedMIMETypes = map[string]bool{
	"image/x-icon":  true,
	"image/vnd.microsoft.icon": true, // common alias for .ico
	"image/png":     true,
	"image/svg+xml": true,
	"image/jpeg":    true,
	"image/webp":    true,
}

// Result is the successful return shape — the bytes the caller will
// hash + insert into the icons table, plus the canonical MIME type
// the API will replay on Content-Type.
type Result struct {
	Bytes []byte
	MIME  string
}

// ErrNoIcon is returned when discovery + the /favicon.ico fallback
// both yielded nothing usable. Callers treat this as "skip, retry on
// next poll" rather than as a hard failure.
var ErrNoIcon = errors.New("iconfetch: no usable icon found")

// Fetcher is the package's single dependency container. Construct via
// New; safe for concurrent use because the underlying httpclient is
// concurrency-safe.
type Fetcher struct {
	client *httpclient.Client
}

// New wires a Fetcher around the shared httpclient.
func New(client *httpclient.Client) *Fetcher {
	return &Fetcher{client: client}
}

// FromHTML extracts candidate icon URLs from a rendered HTML
// document. baseURL is used to resolve relative hrefs. Returned URLs
// are absolute and de-duplicated; order matches document order so the
// first <link rel="icon"> wins.
//
// We accept rel values that contain "icon" as a token; that matches
// real-world markup like `rel="shortcut icon"`, `rel="icon shortcut"`,
// or `rel="apple-touch-icon"`.
func FromHTML(htmlBody []byte, baseURL string) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		base = nil
	}
	z := html.NewTokenizer(bytes.NewReader(htmlBody))
	seen := map[string]bool{}
	var out []string
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return out
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		name, hasAttr := z.TagName()
		if string(name) != "link" || !hasAttr {
			continue
		}
		var rel, href string
		for {
			k, v, more := z.TagAttr()
			switch strings.ToLower(string(k)) {
			case "rel":
				rel = strings.ToLower(string(v))
			case "href":
				href = string(v)
			}
			if !more {
				break
			}
		}
		if href == "" || !relMentionsIcon(rel) {
			continue
		}
		abs := resolve(href, base)
		if abs == "" || seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
}

// relMentionsIcon reports whether a `rel` attribute names a favicon
// candidate. Real-world markup uses `rel="icon"`, `rel="shortcut icon"`,
// `rel="apple-touch-icon"`, `rel="mask-icon"`, etc. — i.e. the token
// is either exactly `icon` or ends with `-icon`. Bare `shortcut`
// alone (without an accompanying `icon` token) is intentionally NOT
// matched; legacy markup always pairs it with `icon`, and matching
// `shortcut` standalone risks pulling in unrelated `<link>` tags.
func relMentionsIcon(rel string) bool {
	for _, tok := range strings.Fields(rel) {
		if tok == "icon" || strings.HasSuffix(tok, "-icon") {
			return true
		}
	}
	return false
}

func resolve(href string, base *url.URL) string {
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if base == nil {
		return ref.String()
	}
	return base.ResolveReference(ref).String()
}

// FaviconFallback returns the conventional /favicon.ico URL for a
// site URL. Returns "" if siteURL is unparseable or has no host.
func FaviconFallback(siteURL string) string {
	u, err := url.Parse(siteURL)
	if err != nil || u.Host == "" {
		return ""
	}
	u.Path = "/favicon.ico"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// Fetch retrieves a single candidate URL. Returns the decoded MIME
// type + bytes, capped at maxBodyBytes. Non-2xx, disallowed MIME, or
// empty bodies all error.
func (f *Fetcher) Fetch(ctx context.Context, candidateURL string) (*Result, error) {
	resp, err := f.client.Get(ctx, candidateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("origin returned %d", resp.StatusCode)
	}
	mime := normaliseMIME(resp.Header.Get("Content-Type"), candidateURL)
	if !allowedMIMETypes[mime] {
		return nil, fmt.Errorf("disallowed mime %q", mime)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if len(body) == 0 {
		return nil, errors.New("empty body")
	}
	// vnd.microsoft.icon is just an alias; the API + clients
	// understand image/x-icon — store the canonical form.
	if mime == "image/vnd.microsoft.icon" {
		mime = "image/x-icon"
	}
	return &Result{Bytes: body, MIME: mime}, nil
}

// extByMIME maps a known file extension to the canonical MIME we
// store. Used by normaliseMIME to recover from CDNs that serve
// favicons as application/octet-stream — the URL extension is the
// most reliable hint left in that case.
var extByMIME = map[string]string{
	".ico":  "image/x-icon",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
}

// normaliseMIME extracts the bare type/subtype from a Content-Type
// header (dropping `; charset=...` and similar parameters). Falls
// back to a path-extension guess — /favicon.ico responses are
// occasionally served as application/octet-stream, which is otherwise
// rejected by the closed allow-list.
func normaliseMIME(contentType, fetchedURL string) string {
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = contentType[:i]
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if allowedMIMETypes[contentType] {
		return contentType
	}
	u, err := url.Parse(fetchedURL)
	if err != nil {
		return contentType
	}
	if mime, ok := extByMIME[strings.ToLower(path.Ext(u.Path))]; ok {
		return mime
	}
	return contentType
}

// Discover walks a list of candidate URLs (article HTML hints first,
// then the /favicon.ico fallback) and returns the first that yields
// a usable Result. Returns ErrNoIcon if none of the candidates work.
//
// The order of candidates is the caller's responsibility — Discover
// just iterates. Failures along the way are silently swallowed; only
// the last error is preserved if no candidate succeeds.
func (f *Fetcher) Discover(ctx context.Context, candidates []string) (*Result, error) {
	if len(candidates) == 0 {
		return nil, ErrNoIcon
	}
	var lastErr error
	for _, c := range candidates {
		r, err := f.Fetch(ctx, c)
		if err == nil {
			return r, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoIcon, lastErr)
	}
	return nil, ErrNoIcon
}

// Verify candidateURL is HTTP(S). The httpclient already enforces
// SSRF-safety, but we drop schemes like data:/javascript: early.
func httpScheme(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// CandidatesFor builds the ordered list of icon URLs to try for a
// feed: HTML hints first (de-duplicated, http(s) only), then the
// /favicon.ico fallback. Exposed as a package-level helper so the
// poller can build the list once and call Discover.
func CandidatesFor(htmlBody []byte, siteURL string) []string {
	var out []string
	for _, c := range FromHTML(htmlBody, siteURL) {
		if httpScheme(c) {
			out = append(out, c)
		}
	}
	if fb := FaviconFallback(siteURL); fb != "" {
		// Avoid duplicating the fallback if it's already in the list.
		dup := false
		for _, c := range out {
			if c == fb {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, fb)
		}
	}
	return out
}

