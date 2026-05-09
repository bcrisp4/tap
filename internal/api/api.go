package api

import (
	"database/sql"
	"net/http"
)

// NewMux returns the API mux. db is required for everything except /healthz.
// poke (optional) is called after a successful POST /api/v1/subscriptions so the
// scheduler can run an immediate tick instead of waiting for the next interval.
// proxyHandler (optional) is mounted at /api/v1/proxy/{token} when non-nil.
func NewMux(db *sql.DB, poke func(), proxyHandler http.Handler) *http.ServeMux {
	m := http.NewServeMux()

	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	if db != nil {
		registerSubscriptionRoutes(m, db, poke)
		registerEntryRoutes(m, db)
	}

	if proxyHandler != nil {
		// {token} is a Go 1.22+ ServeMux path placeholder; the handler
		// reads it via r.PathValue("token").
		m.Handle("GET /api/v1/proxy/{token}", proxyHandler)
	}

	return m
}
