package api

import (
	"database/sql"
	"net/http"
)

// NewMux returns the API mux. db is required for everything except /healthz.
// poke (optional) is called after a successful POST /api/v1/subscriptions so the
// scheduler can run an immediate tick instead of waiting for the next interval.
// Pass nil if you don't have a scheduler (tests).
func NewMux(db *sql.DB, poke func()) *http.ServeMux {
	m := http.NewServeMux()

	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	if db != nil {
		registerSubscriptionRoutes(m, db, poke)
		registerEntryRoutes(m, db)
	}

	return m
}
