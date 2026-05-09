// Package urlcleaner strips well-known tracking parameters from URLs.
//
// Parameter list adapted from github.com/miniflux/v2 internal/reader/urlcleaner.
// Original copyright miniflux contributors, Apache-2.0.
package urlcleaner

import (
	"net/url"
	"strings"
)

// trackingParams is the set of full-name query parameters to drop.
var trackingParams = map[string]struct{}{
	"utm_source": {},
}

// trackingPrefixes is the set of query-parameter name prefixes to drop.
var trackingPrefixes = []string{}

// Clean returns rawURL with well-known tracking parameters removed from
// the query string. Returns rawURL unchanged on parse error or non-URL
// input — URL hygiene must never silently break an entry.
func Clean(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.RawQuery == "" {
		return rawURL
	}
	q := u.Query()
	changed := false
	for k := range q {
		if isTracking(k) {
			q.Del(k)
			changed = true
		}
	}
	if !changed {
		return rawURL
	}
	u.RawQuery = q.Encode()
	// u.String() may re-encode the path/query (escape normalisation,
	// %20 <-> +). The URL stays semantically equivalent; we only reach
	// this line when tracking params were actually stripped.
	return u.String()
}

func isTracking(name string) bool {
	if _, ok := trackingParams[strings.ToLower(name)]; ok {
		return true
	}
	for _, p := range trackingPrefixes {
		if strings.HasPrefix(strings.ToLower(name), p) {
			return true
		}
	}
	return false
}
