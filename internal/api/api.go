package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/auth"
)

// MuxOpts carries optional dependencies for NewMux.
//
//   - Poke is called after a successful POST /api/v1/subscriptions so the
//     scheduler can run an immediate tick.
//   - ProxyHandler is mounted at GET /api/v1/proxy/{token} when non-nil.
//   - SessionIdleTTL / SessionAbsoluteTTL set the cookie + sessions-row
//     expiries (defaults: 7d / 90d).
//   - CookieSecure controls whether the session cookie carries the Secure
//     attribute. Resolved against the listen address by main().
//   - HashParams: argon2 cost. Production passes auth.DefaultParams; tests
//     pass a low-cost variant for speed.
//
// The zero value is valid: the defaults above kick in for the TTLs and
// HashParams, and the proxy / scheduler routes are skipped.
type MuxOpts struct {
	Poke               func()
	ProxyHandler       http.Handler
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	CookieSecure       bool
	HashParams         auth.Params
}

// NewMux returns the API mux. db is required for everything except /healthz
// (and /api/v1/sessions login, which writes into db too — but the mux
// degenerates to login-only when db is nil so callers that just want
// /healthz still work).
//
// The mux mounts the auth middleware on every authenticated route; the only
// public surfaces are GET /healthz and POST /api/v1/sessions. State-changing
// authenticated routes additionally pass through the CSRF middleware.
func NewMux(db *sql.DB, opts MuxOpts) *http.ServeMux {
	m := http.NewServeMux()

	// Defaults for callers (notably tests) that leave fields zero.
	if opts.SessionIdleTTL <= 0 {
		opts.SessionIdleTTL = 7 * 24 * time.Hour
	}
	if opts.SessionAbsoluteTTL <= 0 {
		opts.SessionAbsoluteTTL = 90 * 24 * time.Hour
	}
	if opts.HashParams == (auth.Params{}) {
		opts.HashParams = auth.DefaultParams
	}

	deps := authDeps{
		d:                  db,
		sessionIdleTTL:     opts.SessionIdleTTL,
		sessionAbsoluteTTL: opts.SessionAbsoluteTTL,
		cookieSecure:       opts.CookieSecure,
	}

	// Public routes.
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	m.Handle("POST /api/v1/sessions", loginHandler(deps))

	if db == nil {
		return m
	}

	// Authenticated routes. Sub-muxes carry the existing handler bodies;
	// each route is then mounted on the parent mux through the appropriate
	// middleware. Going via sub-muxes (instead of wrapping the parent)
	// keeps /healthz and POST /sessions outside the auth chain — concept
	// §7.5: only login + healthz are public.
	authed := requireSession(db, opts.SessionIdleTTL)
	authedCSRF := chain(authed, requireCSRF())

	m.Handle("GET /api/v1/sessions/current", authed(getSessionCurrentHandler()))
	m.Handle("DELETE /api/v1/sessions/current", authedCSRF(logoutHandler(deps)))
	m.Handle("PATCH /api/v1/me/password", authedCSRF(passwordChangeHandler(deps, opts.HashParams)))

	subsMux := http.NewServeMux()
	registerSubscriptionRoutes(subsMux, db, opts.Poke)
	entriesMux := http.NewServeMux()
	registerEntryRoutes(entriesMux, db)

	// Explicit per-pattern mount: the stdlib mux doesn't expose registered
	// patterns, and we want each pattern wrapped with the right middleware
	// (writes go through CSRF; reads only need the session).
	for _, p := range []struct {
		method, path string
		handler      http.Handler
	}{
		{"GET", "/api/v1/subscriptions", authed(subsMux)},
		{"POST", "/api/v1/subscriptions", authedCSRF(subsMux)},
		{"PATCH", "/api/v1/subscriptions/{id}", authedCSRF(subsMux)},
		{"DELETE", "/api/v1/subscriptions/{id}", authedCSRF(subsMux)},
		{"GET", "/api/v1/entries", authed(entriesMux)},
		{"GET", "/api/v1/entries/{id}", authed(entriesMux)},
		{"PATCH", "/api/v1/entries/{id}", authedCSRF(entriesMux)},
	} {
		m.Handle(p.method+" "+p.path, p.handler)
	}

	if opts.ProxyHandler != nil {
		// {token} is a Go 1.22+ ServeMux path placeholder; the handler
		// reads it via r.PathValue("token"). The proxy is a GET-only
		// authenticated route — no CSRF check needed (CSRF only protects
		// state-changing requests), and importantly the proxy's outbound
		// fetch never forwards the SPA's Cookie/Authorization headers.
		m.Handle("GET /api/v1/proxy/{token}", authed(opts.ProxyHandler))
	}

	return m
}
