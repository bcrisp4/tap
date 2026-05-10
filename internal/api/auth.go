package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

// _ keeps imports tidy.
var _ = json.Marshal
var _ = errors.New
var _ = context.Background
var _ = auth.Hash
