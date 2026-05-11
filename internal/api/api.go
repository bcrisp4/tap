package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/bcrisp4/tap/internal/ratelimit"
	"github.com/bcrisp4/tap/internal/ring"
)

// MuxOpts carries optional dependencies for NewMux.
type MuxOpts struct {
	Poke               func()
	ProxyHandler       http.Handler
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	CookieSecure       bool
	HashParams         auth.Params
	WebAuthnInstance   *webauthn.WebAuthn
	DiscoverClient     *http.Client

	Limiter        *ratelimit.Limiter // nil = no rate limiting
	TrustedProxy   bool
	MetricsEnabled bool
	RingBuffer     *ring.Buffer
	StartTime      time.Time
	Version        string
	PollsActive    func() int64 // current in-flight poll count
}

// NewMux returns the API mux.
func NewMux(db *sql.DB, opts MuxOpts) *http.ServeMux {
	m := http.NewServeMux()

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
		limiter:            opts.Limiter,
		trustedProxy:       opts.TrustedProxy,
		hashParams:         opts.HashParams,
	}

	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if db != nil {
			if err := db.PingContext(r.Context()); err != nil {
				dbStatus = "degraded"
			}
		}
		var active int64
		if opts.PollsActive != nil {
			active = opts.PollsActive()
		}
		status := "ok"
		if dbStatus == "degraded" {
			status = "degraded"
		}
		var uptime int64
		if !opts.StartTime.IsZero() {
			uptime = int64(time.Since(opts.StartTime).Seconds())
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":         status,
			"version":        opts.Version,
			"uptime_seconds": uptime,
			"db":             dbStatus,
			"polls_active":   active,
		})
	})

	if opts.MetricsEnabled {
		m.Handle("GET /metrics", metrics.Handler())
	}

	if db == nil {
		return m
	}

	m.Handle("POST /api/v1/sessions", loginHandler(deps))

	authed := requireSession(db, opts.SessionIdleTTL)
	authedCSRF := chain(authed, requireCSRF())
	authedAdmin := chain(authed, requireAdmin())
	authedAdminCSRF := chain(authed, requireAdmin(), requireCSRF())

	// Existing auth routes.
	m.Handle("GET /api/v1/sessions/current", authed(getSessionCurrentHandler(db)))
	m.Handle("DELETE /api/v1/sessions/current", authedCSRF(logoutHandler(deps)))
	m.Handle("PATCH /api/v1/me/password", authedCSRF(passwordChangeHandler(deps, opts.HashParams)))
	m.Handle("DELETE /api/v1/me", authedCSRF(deleteAccountHandler(deps)))

	// Session listing and revocation.
	m.Handle("GET /api/v1/sessions", authed(listSessionsHandler(db)))
	m.Handle("DELETE /api/v1/sessions/{id}", authedCSRF(revokeSessionHandler(db)))
	m.Handle("DELETE /api/v1/sessions", authedCSRF(revokeAllOtherSessionsHandler(db)))

	// TOTP endpoints.
	m.Handle("POST /api/v1/me/totp", authedCSRF(beginTOTPEnrolmentHandler(db, opts.HashParams)))
	m.Handle("POST /api/v1/me/totp/confirm", authedCSRF(confirmTOTPEnrolmentHandler(db, opts.HashParams)))
	m.Handle("DELETE /api/v1/me/totp", authedCSRF(deleteTOTPHandler(db)))
	m.Handle("POST /api/v1/me/totp/recovery-codes", authedCSRF(regenerateRecoveryCodesHandler(db, opts.HashParams)))

	// Passkey registration endpoints (require active session).
	if opts.WebAuthnInstance != nil {
		m.Handle("POST /api/v1/me/passkeys/registration/begin", authedCSRF(beginPasskeyRegistrationHandler(db, opts.WebAuthnInstance)))
		m.Handle("POST /api/v1/me/passkeys/registration/finish", authedCSRF(finishPasskeyRegistrationHandler(db, opts.WebAuthnInstance)))
		// Passkey login (public — no session required).
		m.Handle("POST /api/v1/passkey-sessions/begin", beginPasskeyLoginHandler(db, opts.WebAuthnInstance, deps))
		m.Handle("POST /api/v1/passkey-sessions/finish", finishPasskeyLoginHandler(db, opts.WebAuthnInstance, deps))
	}

	m.Handle("GET /api/v1/me/passkeys", authed(listPasskeysHandler(db)))
	m.Handle("DELETE /api/v1/me/passkeys/{id}", authedCSRF(deletePasskeyHandler(db)))

	// System status endpoint (admin-only, M12).
	m.Handle("GET /api/v1/status", authed(statusHandler(statusDeps{
		db:          db,
		buf:         opts.RingBuffer,
		startTime:   opts.StartTime,
		version:     opts.Version,
		pollsActive: opts.PollsActive,
	})))

	// Admin endpoints.
	m.Handle("GET /api/v1/admin/users", authedAdmin(listUsersHandler(db)))
	m.Handle("POST /api/v1/admin/users", authedAdminCSRF(createUserHandler(db, opts.HashParams)))
	m.Handle("PATCH /api/v1/admin/users/{id}", authedAdminCSRF(patchUserHandler(db)))
	m.Handle("POST /api/v1/admin/users/{id}/password-reset", authedAdminCSRF(resetUserPasswordHandler(db, opts.HashParams)))
	m.Handle("POST /api/v1/admin/users/{id}/disable-totp", authedAdminCSRF(disableUserTOTPHandler(db)))
	m.Handle("DELETE /api/v1/admin/users/{id}", authedAdminCSRF(deleteUserHandler(db)))

	subsMux := http.NewServeMux()
	registerSubscriptionRoutes(subsMux, db, opts.Poke)
	entriesMux := http.NewServeMux()
	registerEntryRoutes(entriesMux, db)
	catsMux := http.NewServeMux()
	registerCategoryRoutes(catsMux, db)
	searchMux := http.NewServeMux()
	registerSearchRoutes(searchMux, db)
	opmlMux := http.NewServeMux()
	registerOPMLRoutes(opmlMux, db)
	if opts.DiscoverClient != nil {
		discoverMux := http.NewServeMux()
		registerDiscoverRoutes(discoverMux, opts.DiscoverClient)
		m.Handle("POST /api/v1/discover", authedCSRF(discoverMux))
	}

	for _, p := range []struct {
		method, path string
		handler      http.Handler
	}{
		{"GET", "/api/v1/subscriptions", authed(subsMux)},
		{"GET", "/api/v1/subscriptions/{id}", authed(subsMux)},
		{"POST", "/api/v1/subscriptions", authedCSRF(subsMux)},
		{"PATCH", "/api/v1/subscriptions/{id}", authedCSRF(subsMux)},
		{"DELETE", "/api/v1/subscriptions/{id}", authedCSRF(subsMux)},
		{"POST", "/api/v1/subscriptions/{id}/mark-read", authedCSRF(subsMux)},
		{"GET", "/api/v1/entries", authed(entriesMux)},
		{"GET", "/api/v1/entries/{id}", authed(entriesMux)},
		{"PATCH", "/api/v1/entries/{id}", authedCSRF(entriesMux)},
		{"GET", "/api/v1/categories", authed(catsMux)},
		{"POST", "/api/v1/categories", authedCSRF(catsMux)},
		{"PATCH", "/api/v1/categories/{id}", authedCSRF(catsMux)},
		{"DELETE", "/api/v1/categories/{id}", authedCSRF(catsMux)},
		{"POST", "/api/v1/categories/{id}/mark-read", authedCSRF(catsMux)},
		{"POST", "/api/v1/categories/reorder", authedCSRF(catsMux)},
		{"GET", "/api/v1/search", authed(searchMux)},
		{"GET", "/api/v1/opml", authed(opmlMux)},
		{"POST", "/api/v1/opml", authedCSRF(opmlMux)},
	} {
		m.Handle(p.method+" "+p.path, p.handler)
	}

	if opts.ProxyHandler != nil {
		m.Handle("GET /api/v1/proxy/{token}", authed(opts.ProxyHandler))
	}

	return m
}
