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
//
// A nil encode is treated as the identity (no rewriting), matching
// NewPipeline's default and avoiding a nil-deref for callers that
// don't need media rewriting.
func RewriteMedia(entryHTML, articleURL string, encode ProxyEncoder) (string, error) {
	if encode == nil {
		encode = func(u string) string { return u }
	}
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

// rewriteSrcset splits a srcset into candidates, rewrites each URL,
// and preserves any width/density descriptor unchanged.
//
// data: URIs may contain commas in their payload; splitting naively on
// "," would corrupt them. To handle that without pulling in a full
// WHATWG parser, candidates beginning with "data:" are treated as a
// single unit up to the first whitespace (their descriptor terminator)
// and only commas outside the data: payload act as separators.
func rewriteSrcset(raw string, base *url.URL, encode ProxyEncoder) string {
	candidates := splitSrcsetCandidates(raw)
	for i, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		urlPart, descriptor := splitURLAndDescriptor(c)
		if urlPart == "" {
			continue
		}
		candidates[i] = rewriteOne(urlPart, base, encode) + descriptor
	}
	return strings.Join(candidates, ", ")
}

const srcsetWhitespace = " \t\n\r"

// splitSrcsetCandidates splits a srcset on commas, except commas that
// appear inside the payload of a "data:" URI (i.e. before the first
// whitespace following the "data:" prefix).
func splitSrcsetCandidates(raw string) []string {
	var out []string
	for i := 0; i < len(raw); {
		// Skip leading whitespace and commas between candidates.
		j := i
		for j < len(raw) && (raw[j] == ',' || strings.IndexByte(srcsetWhitespace, raw[j]) >= 0) {
			j++
		}
		if j >= len(raw) {
			break
		}
		start := j
		// data: URIs may contain commas; skip past the URI to the
		// first whitespace (descriptor terminator) before scanning
		// for the next comma separator.
		if len(raw)-j >= 5 && strings.EqualFold(raw[j:j+5], "data:") {
			for j < len(raw) && strings.IndexByte(srcsetWhitespace, raw[j]) < 0 {
				j++
			}
		}
		for j < len(raw) && raw[j] != ',' {
			j++
		}
		out = append(out, raw[start:j])
		i = j
	}
	return out
}

// splitURLAndDescriptor returns (url, descriptor) where descriptor
// includes its leading whitespace so it round-trips byte-for-byte.
func splitURLAndDescriptor(c string) (string, string) {
	if i := strings.IndexAny(c, srcsetWhitespace); i >= 0 {
		return c[:i], c[i:]
	}
	return c, ""
}
