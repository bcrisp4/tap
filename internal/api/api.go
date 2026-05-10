package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/bcrisp4/tap/internal/auth"
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
	}

	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

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
		m.Handle("GET /api/v1/proxy/{token}", authed(opts.ProxyHandler))
	}

	return m
}
