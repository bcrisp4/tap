package api

import (
	"database/sql"
	"net/http"
)

// MuxOpts carries optional dependencies for NewMux.
//   - Poke is called after a successful POST /api/v1/subscriptions so the
//     scheduler can run an immediate tick.
//   - ProxyHandler is mounted at GET /api/v1/proxy/{token} when non-nil.
//
// The zero value is valid (no scheduler poke, no proxy route).
type MuxOpts struct {
	Poke         func()
	ProxyHandler http.Handler
}

// NewMux returns the API mux. db is required for everything except /healthz.
func NewMux(db *sql.DB, opts MuxOpts) *http.ServeMux {
	m := http.NewServeMux()

	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	if db != nil {
		registerSubscriptionRoutes(m, db, opts.Poke)
		registerEntryRoutes(m, db)
	}

	if opts.ProxyHandler != nil {
		// {token} is a Go 1.22+ ServeMux path placeholder; the handler
		// reads it via r.PathValue("token").
		m.Handle("GET /api/v1/proxy/{token}", opts.ProxyHandler)
	}

	return m
}
