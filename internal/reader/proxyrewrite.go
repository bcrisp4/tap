package reader

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ProxyEncoder turns an absolute source URL into the Tap proxy URL
// served at /api/v1/proxy/<token>. Plan 06 supplies the real
// implementation.
type ProxyEncoder func(sourceURL string) string

// RewriteMedia rewrites every <img src>, <img srcset>, and
// <picture><source srcset> in entryHTML to point at the Tap media
// proxy. Relative URLs are resolved against articleURL. data: URIs
// and absolute non-http(s) schemes are passed through untouched.
func RewriteMedia(entryHTML, articleURL string, encode ProxyEncoder) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(entryHTML))
	if err != nil {
		return "", err
	}
	base, _ := url.Parse(articleURL)

	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		if v, ok := s.Attr("src"); ok {
			s.SetAttr("src", rewriteOne(v, base, encode))
		}
		if v, ok := s.Attr("srcset"); ok {
			s.SetAttr("srcset", rewriteSrcset(v, base, encode))
		}
	})
	doc.Find("picture source").Each(func(_ int, s *goquery.Selection) {
		if v, ok := s.Attr("srcset"); ok {
			s.SetAttr("srcset", rewriteSrcset(v, base, encode))
		}
	})

	return innerBodyHTML(doc), nil
}

// innerBodyHTML returns the concatenated outer HTML of every direct
// child of <body>. goquery wraps the input in <html><head><body>; we
// want the same fragment-shaped output the caller passed in.
func innerBodyHTML(doc *goquery.Document) string {
	var b strings.Builder
	doc.Find("body").Each(func(_ int, s *goquery.Selection) {
		s.Contents().Each(func(_ int, c *goquery.Selection) {
			if h, err := goquery.OuterHtml(c); err == nil {
				b.WriteString(h)
			}
		})
	})
	return b.String()
}

func rewriteOne(raw string, base *url.URL, encode ProxyEncoder) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if strings.HasPrefix(raw, "data:") {
		return raw // sanitizer enforces image/* MIME-only on data: URIs
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if !u.IsAbs() && base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return raw
	}
	return encode(u.String())
}

// rewriteSrcset splits a srcset on commas, rewrites each URL, and
// preserves the descriptor verbatim. Whitespace inside descriptors is
// kept; surrounding whitespace is trimmed.
func rewriteSrcset(raw string, base *url.URL, encode ProxyEncoder) string {
	parts := strings.Split(raw, ",")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// First whitespace splits the URL from the descriptor.
		fields := strings.Fields(p)
		if len(fields) == 0 {
			continue
		}
		fields[0] = rewriteOne(fields[0], base, encode)
		parts[i] = strings.Join(fields, " ")
	}
	return strings.Join(parts, ", ")
}
