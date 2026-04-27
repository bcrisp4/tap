// Package web exposes an http.Handler that serves the embedded
// SvelteKit SPA.
//
// Build modes:
//   - default (no build tag): buildFS is empty; Handler returns a
//     small placeholder page. `go build ./...` and `go test ./...`
//     work against a fresh checkout that hasn't run npm build.
//   - -tags embed_spa: the production build. The real //go:embed
//     directive in embed_with_spa.go populates buildFS from
//     web/build/.
//
// Makefile's `build` target always passes `-tags embed_spa`.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// buildFS is reassigned by embed_with_spa.go's init() when the
// embed_spa build tag is set. The blank var keeps the type stable
// across both modes.
var buildFS embed.FS

// Handler returns an http.Handler serving the SPA. /api/v1/* is NOT
// served by this handler — register it on the mux as a fallback (e.g.
// mux.Handle("/", web.Handler())).
//
// When no SPA is embedded, Handler returns a friendly placeholder
// page that points at the API. This shows up in dev only; production
// images always include the SPA.
func Handler() http.Handler {
	if !hasSPA() {
		return http.HandlerFunc(servePlaceholder)
	}
	sub, err := fs.Sub(buildFS, "build")
	if err != nil {
		panic(err)
	}
	return spaHandler{root: sub, fileServer: http.FileServer(http.FS(sub))}
}

func hasSPA() bool {
	_, err := fs.Stat(buildFS, "build/index.html")
	return err == nil
}

func servePlaceholder(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(placeholderHTML))
}

const placeholderHTML = `<!doctype html><meta charset="utf-8"><title>tap (api only)</title>
<style>body{font:14px/1.6 system-ui;max-width:640px;margin:60px auto;padding:0 24px;color:#333}
code{background:#f3f3f3;padding:1px 6px;border-radius:3px}</style>
<h1>tap</h1>
<p>The Svelte SPA isn't embedded into this binary. Run
<code>make build</code> (which runs <code>npm --prefix web run build</code>
and <code>go build -tags embed_spa</code>) to ship the full app.</p>
<p>The API is live at <code>/api/v1/...</code>.</p>`

type spaHandler struct {
	root       fs.FS
	fileServer http.Handler
}

// ServeHTTP delegates to the embedded FS for real files; falls back to
// index.html for any 404 so SvelteKit's client-side router can take
// over.
func (s spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(s.root, path); err != nil {
		// Unknown path → serve index.html so the SPA router can
		// resolve it client-side.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		s.fileServer.ServeHTTP(w, r2)
		return
	}
	s.fileServer.ServeHTTP(w, r)
}
