// Package server owns the HTTP server lifecycle and the embedded SPA glue.
package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/bcrisp4/tap/web"
)

// SPAHandler serves the embedded SPA bundle. Falls back to index.html for
// unknown routes so client-side routing works for deep links.
//
// There is no dev-mode branch: in dev, the developer visits Vite directly on
// :5173, and Vite proxies /api + /healthz to the Go server on :8080. The Go
// server only ever serves the embedded bundle.
func SPAHandler() http.Handler {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		// Build-time failure to embed should never reach runtime.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "SPA bundle missing", http.StatusInternalServerError)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			clean = "index.html"
		}
		// Try the asset; fall back to index.html for SPA routes.
		if _, err := fs.Stat(dist, clean); err == nil {
			http.ServeFileFS(w, r, dist, clean)
			return
		}
		http.ServeFileFS(w, r, dist, "index.html")
	})
}
