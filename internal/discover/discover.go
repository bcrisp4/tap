// Package discover implements two-step feed discovery: try to parse a URL as
// a feed directly, then fall back to HTML <link rel="alternate"> parsing.
package discover

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
)

// Result is a discovered feed candidate.
type Result struct {
	Title   string
	FeedURL string
	SiteURL string
	Type    string // "rss", "atom", or "json"
}

// ErrNoFeeds is returned when no feed candidates are found at the given URL.
var ErrNoFeeds = errors.New("no feed candidates found at URL")

// Discover fetches url via client and returns feed candidates.
// It first tries to parse the response as a feed. If that fails it falls back
// to parsing the HTML for <link rel="alternate"> elements. Returns ErrNoFeeds
// if neither strategy finds anything.
//
// Discovery is always unauthenticated — no credentials are applied to the request.
func Discover(ctx context.Context, client *http.Client, rawURL string) ([]Result, error) {
	u, err := url.Parse(rawURL)
	if err != nil || !u.IsAbs() {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Tap/1 (+https://github.com/bcrisp4/tap)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MiB
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Step 1: try to parse as a feed.
	if results := tryParseFeed(rawURL, body); len(results) > 0 {
		return results, nil
	}

	// Step 2: parse HTML for <link rel="alternate">.
	ct := resp.Header.Get("Content-Type")
	if isHTML(ct) || isHTML(sniffContentType(body)) {
		results := parseAlternateLinks(rawURL, body)
		if len(results) > 0 {
			return results, nil
		}
	}

	return nil, ErrNoFeeds
}

func tryParseFeed(sourceURL string, body []byte) []Result {
	fp := gofeed.NewParser()
	feed, err := fp.ParseString(string(body))
	if err != nil {
		return nil
	}
	t := feedType(feed.FeedType)
	title := feed.Title
	if title == "" {
		title = sourceURL
	}
	siteURL := feed.Link
	if siteURL == "" {
		siteURL = sourceURL
	}
	return []Result{{
		Title:   title,
		FeedURL: sourceURL,
		SiteURL: siteURL,
		Type:    t,
	}}
}

func feedType(ft string) string {
	switch strings.ToLower(ft) {
	case "atom":
		return "atom"
	case "json":
		return "json"
	default:
		return "rss"
	}
}

func parseAlternateLinks(baseURL string, body []byte) []Result {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	base, _ := url.Parse(baseURL)
	var results []Result
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.ToLower(n.Data) == "link" {
			if r, ok := parseLinkElement(base, n); ok {
				results = append(results, r)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return results
}

func parseLinkElement(base *url.URL, n *html.Node) (Result, bool) {
	attrs := attrMap(n)
	rel := strings.ToLower(attrs["rel"])
	if rel != "alternate" {
		return Result{}, false
	}
	ct := strings.ToLower(attrs["type"])
	t := feedTypeFromMIME(ct)
	if t == "" {
		return Result{}, false
	}
	href := strings.TrimSpace(attrs["href"])
	if href == "" {
		return Result{}, false
	}
	u, err := url.Parse(href)
	if err != nil {
		return Result{}, false
	}
	if base != nil && !u.IsAbs() {
		u = base.ResolveReference(u)
	}
	if !u.IsAbs() {
		return Result{}, false
	}
	title := strings.TrimSpace(attrs["title"])
	if title == "" {
		title = u.String()
	}
	return Result{
		Title:   title,
		FeedURL: u.String(),
		SiteURL: base.String(),
		Type:    t,
	}, true
}

func feedTypeFromMIME(ct string) string {
	// Strip charset and params.
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "application/rss+xml":
		return "rss"
	case "application/atom+xml":
		return "atom"
	case "application/feed+json", "application/json":
		return "json"
	default:
		return ""
	}
}

func attrMap(n *html.Node) map[string]string {
	m := make(map[string]string, len(n.Attr))
	for _, a := range n.Attr {
		m[strings.ToLower(a.Key)] = a.Val
	}
	return m
}

func isHTML(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

func sniffContentType(body []byte) string {
	if len(body) > 512 {
		body = body[:512]
	}
	s := strings.TrimSpace(string(body))
	if strings.HasPrefix(s, "<html") || strings.HasPrefix(s, "<!DOCTYPE") || strings.HasPrefix(s, "<!doctype") {
		return "text/html"
	}
	return ""
}
