// Package reader implements Tap's article-content pipeline:
// extraction (CSS rules → go-readability fallback), media URL
// rewriting, and HTML sanitization.
package reader

import (
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
)

// Extract returns the article body for entryHTML. Order:
//  1. If scraperRules is non-empty, apply it via goquery and return the
//     concatenated outer HTML of every match.
//  2. Otherwise, run go-readability against the document.
//  3. If neither yields anything, return entryHTML unchanged.
func Extract(entryHTML, articleURL, scraperRules string) (string, error) {
	if rules := strings.TrimSpace(scraperRules); rules != "" {
		out, err := extractByRules(entryHTML, rules)
		if err != nil {
			return "", err
		}
		if out != "" {
			return out, nil
		}
	}

	out, err := extractByReadability(entryHTML, articleURL)
	if err != nil {
		// Graceful: unparsable HTML still gets stored verbatim. The
		// sanitizer downstream is the final safety net.
		return entryHTML, nil //nolint:nilerr
	}
	if strings.TrimSpace(out) == "" {
		return entryHTML, nil
	}
	return out, nil
}

func extractByRules(entryHTML, selector string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(entryHTML))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		// OuterHtml returns ("", err) only on programming bugs.
		if outer, err := goquery.OuterHtml(s); err == nil {
			b.WriteString(outer)
		}
	})
	return b.String(), nil
}

func extractByReadability(entryHTML, articleURL string) (string, error) {
	u, err := url.Parse(articleURL)
	if err != nil {
		// Readability needs a URL for relative-href resolution; fall
		// back to a placeholder so it still tries.
		u = &url.URL{Scheme: "https", Host: "example.invalid"}
	}
	article, err := readability.FromReader(strings.NewReader(entryHTML), u)
	if err != nil {
		return "", err
	}
	if article.Node == nil {
		return "", nil
	}
	var b strings.Builder
	if err := article.RenderHTML(&b); err != nil {
		return "", err
	}
	return b.String(), nil
}
