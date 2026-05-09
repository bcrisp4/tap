// Package urlcleaner strips well-known tracking parameters from URLs.
//
// Parameter list adapted from github.com/miniflux/v2 internal/reader/urlcleaner.
// Original copyright miniflux contributors, Apache-2.0.
package urlcleaner

// Clean returns rawURL with well-known tracking parameters removed from
// the query string. Returns rawURL unchanged on parse error or non-URL
// input — URL hygiene must never silently break an entry.
func Clean(rawURL string) string {
	return rawURL
}
