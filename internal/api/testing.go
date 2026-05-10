package api

import (
	"database/sql"
	"net/http"
)

// NewTestMux returns an UNAUTHENTICATED mux carrying the same /api/v1/*
// routes as NewMux, but skipping requireSession + requireCSRF. Intended for
// unit and integration tests that exercise the route plumbing without
// driving the auth flow.
//
// Production code path is NewMux, which wraps every authenticated route
// (everything except /healthz and POST /api/v1/sessions) with the
// session-and-CSRF middleware chain. End-to-end auth coverage lives in
// cmd/tap/main_test.go; this helper exists so that the M5-era end-to-end
// tests (subscribe -> poll -> serve entries, proxy roundtrip, extract)
// don't have to take a dependency on the auth flow.
//
// Mounted routes mirror NewMux:
//   - GET /healthz
//   - GET/POST/PATCH/DELETE /api/v1/subscriptions[/{id}]
//   - GET /api/v1/entries[/{id}]
//   - PATCH /api/v1/entries/{id}
//   - GET /api/v1/proxy/{token} (when opts.ProxyHandler != nil)
//
// Login / logout / password-change routes are NOT mounted here: tests that
// need them mount the production NewMux.
func NewTestMux(d *sql.DB, opts MuxOpts) *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	if d != nil {
		registerSubscriptionRoutes(m, d, opts.Poke)
		registerEntryRoutes(m, d)
	}
	if opts.ProxyHandler != nil {
		m.Handle("GET /api/v1/proxy/{token}", opts.ProxyHandler)
	}
	return m
}
