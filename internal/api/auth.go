package api

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
)

// CookieSecureMode controls the Secure attribute on the tap_session cookie.
//
//	CookieSecureAuto:  Secure when the listen address resolves to non-loopback.
//	CookieSecureTrue:  always set Secure.
//	CookieSecureFalse: never set Secure.
type CookieSecureMode int

const (
	CookieSecureAuto CookieSecureMode = iota
	CookieSecureTrue
	CookieSecureFalse
)

// ResolveCookieSecure is the exported wrapper used by cmd/tap to resolve
// the --cookie-secure flag once at startup. The actual logic lives in
// resolveCookieSecure (kept lowercase so the package's middleware tests
// can call it directly without changing).
func ResolveCookieSecure(mode CookieSecureMode, addr string) bool {
	return resolveCookieSecure(mode, addr)
}

// resolveCookieSecure decides whether to set the Secure attribute on the
// session cookie. In auto mode, it inspects the listen address: bound to
// 127.0.0.0/8, ::1, or "localhost" → Secure off; anything else → Secure on.
func resolveCookieSecure(mode CookieSecureMode, addr string) bool {
	switch mode {
	case CookieSecureTrue:
		return true
	case CookieSecureFalse:
		return false
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// If --addr is malformed, default to Secure ON (fail-safe).
		return true
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// A bare hostname that isn't "localhost" — be conservative.
		return true
	}
	return !ip.IsLoopback()
}

// userDTO is the user shape exposed via the API. Never carries password_hash.
type userDTO struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func toUserDTO(u db.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Role: u.Role}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	User      userDTO `json:"user"`
	CSRFToken string  `json:"csrf_token"`
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type passwordChangeResponse struct {
	CSRFToken string `json:"csrf_token"`
}

// authDeps bundles the dependencies the auth handlers need so api.go's
// NewMux can construct them once and pass them to handler factories.
type authDeps struct {
	d                  *sql.DB
	sessionIdleTTL     time.Duration
	sessionAbsoluteTTL time.Duration
	cookieSecure       bool
}

// setSessionCookie writes the session cookie on the response.
func setSessionCookie(w http.ResponseWriter, value string, absoluteTTL time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "tap_session",
		Value:    value,
		Path:     "/",
		MaxAge:   int(absoluteTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie writes a Max-Age=0 cookie that overrides the existing one.
func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "tap_session",
		Value:    "",
		Path:     "/",
		MaxAge:   0,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// loginHandler returns POST /api/v1/sessions. Public, CSRF not required.
func loginHandler(dep authDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body loginRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		username := strings.TrimSpace(body.Username)
		if username == "" || body.Password == "" {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}

		u, err := db.GetUserByUsername(r.Context(), dep.d, username)
		if err != nil {
			// Includes sql.ErrNoRows (unknown user). Same response either way
			// to avoid disclosing username existence.
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}
		if u.DisabledAt.Valid {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}
		ok, err := auth.Verify(u.PasswordHash, body.Password)
		if err != nil || !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}

		cookieValue, tokenHash, err := auth.MintSessionToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		csrfToken, err := auth.MintCSRFToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		now := time.Now()
		_, err = db.InsertSession(r.Context(), dep.d, db.NewSession{
			UserID:            u.ID,
			TokenHash:         tokenHash,
			CSRFToken:         csrfToken,
			CreatedAt:         now.Unix(),
			LastSeenAt:        now.Unix(),
			IdleExpiresAt:     now.Add(dep.sessionIdleTTL).Unix(),
			AbsoluteExpiresAt: now.Add(dep.sessionAbsoluteTTL).Unix(),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		setSessionCookie(w, cookieValue, dep.sessionAbsoluteTTL, dep.cookieSecure)
		writeJSON(w, http.StatusOK, loginResponse{User: toUserDTO(u), CSRFToken: csrfToken})
	})
}

// getSessionCurrentHandler returns GET /api/v1/sessions/current.
// Authenticated; CSRF not required (GET). The SPA calls this on boot to
// recover its in-memory CSRF token after a reload.
func getSessionCurrentHandler(_ authDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok1 := userFromContext(r.Context())
		s, ok2 := sessionFromContext(r.Context())
		if !ok1 || !ok2 {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		writeJSON(w, http.StatusOK, loginResponse{User: toUserDTO(u), CSRFToken: s.CSRFToken})
	})
}

// logoutHandler returns DELETE /api/v1/sessions/current. Authenticated;
// CSRF required (the middleware chain enforces that, not this handler).
func logoutHandler(dep authDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := sessionFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		if err := db.DeleteSession(r.Context(), dep.d, s.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		clearSessionCookie(w, dep.cookieSecure)
		w.WriteHeader(http.StatusNoContent)
	})
}

// passwordChangeHandler returns PATCH /api/v1/me/password. Authenticated;
// CSRF required (middleware enforces). Verifies current_password, validates
// new_password, hashes + updates, deletes other sessions for the user
// (keeps current), rotates the current session's CSRF token, returns
// the new csrf_token.
//
// hashParams is exposed so tests can inject testHashParams; production
// passes auth.DefaultParams.
func passwordChangeHandler(dep authDeps, hashParams auth.Params) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		s, ok := sessionFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body passwordChangeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		ok, err := auth.Verify(u.PasswordHash, body.CurrentPassword)
		if err != nil || !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "current password incorrect")
			return
		}
		if err := auth.ValidatePassword(body.NewPassword); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodePasswordTooShort, "new password too short")
			return
		}
		newHash, err := auth.Hash(body.NewPassword, hashParams)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.UpdatePasswordHash(r.Context(), dep.d, u.ID, newHash); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.DeleteOtherSessionsForUser(r.Context(), dep.d, u.ID, s.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		newCSRF, err := auth.MintCSRFToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.UpdateSessionCSRFToken(r.Context(), dep.d, s.ID, newCSRF); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, passwordChangeResponse{CSRFToken: newCSRF})
	})
}
