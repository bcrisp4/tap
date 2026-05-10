package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"
	"time"

	"github.com/bcrisp4/tap/internal/db"
)

// ctxKey is unexported so external packages cannot read or override the
// per-request user/session injected by requireSession.
type ctxKey int

const (
	ctxKeyUser ctxKey = iota
	ctxKeySession
)

// userFromContext returns the user injected by requireSession, if any.
func userFromContext(ctx context.Context) (db.User, bool) {
	u, ok := ctx.Value(ctxKeyUser).(db.User)
	return u, ok
}

// sessionFromContext returns the session injected by requireSession, if any.
func sessionFromContext(ctx context.Context) (db.Session, bool) {
	s, ok := ctx.Value(ctxKeySession).(db.Session)
	return s, ok
}

// chain composes middleware. Outermost is first.
func chain(mws ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}

// requireSession reads the tap_session cookie, looks up the row by
// sha256(cookie), validates idle + absolute expiries, refreshes idle on
// success, and injects the user + session into the request context.
// 401 invalid_session on any failure.
func requireSession(d *sql.DB, idleTTL time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("tap_session")
			if err != nil || c.Value == "" {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
				return
			}
			tokenHash, ok := hashCookie(c.Value)
			if !ok {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "bad session cookie")
				return
			}
			s, err := db.GetSessionByTokenHash(r.Context(), d, tokenHash)
			if err != nil {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no such session")
				return
			}
			now := time.Now().Unix()
			if now > s.AbsoluteExpiresAt || now > s.IdleExpiresAt {
				_ = db.DeleteSession(r.Context(), d, s.ID)
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "session expired")
				return
			}
			u, err := db.GetUserByID(r.Context(), d, s.UserID)
			if err != nil {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "user not found")
				return
			}
			// Best-effort idle refresh. A failure here doesn't break the
			// request — worst case the session expires sooner than expected.
			_ = db.RefreshSessionIdle(r.Context(), d, s.ID, now, now+int64(idleTTL.Seconds()))

			ctx := context.WithValue(r.Context(), ctxKeyUser, u)
			ctx = context.WithValue(ctx, ctxKeySession, s)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// hashCookie decodes the base64url cookie value and returns its sha256-hex.
// Returns ok=false if the cookie is not valid base64url (which would never
// match a stored token_hash anyway).
func hashCookie(cookieValue string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(cookieValue)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), true
}

// requireCSRF passes through GET/HEAD/OPTIONS, otherwise:
//   - validates Origin or Referer host equals r.Host (when the header is
//     present; both absent = pass, since SameSite=Lax + an authenticated
//     session already cover that case);
//   - reads X-CSRF-Token and constant-time compares to session.CSRFToken;
//   - 403 csrf_invalid on any failure.
func requireCSRF() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			if !originOK(r) {
				writeError(w, http.StatusForbidden, ErrCodeCSRFInvalid, "origin mismatch")
				return
			}

			s, ok := sessionFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusForbidden, ErrCodeCSRFInvalid, "no session in context")
				return
			}
			got := r.Header.Get("X-CSRF-Token")
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.CSRFToken)) != 1 {
				writeError(w, http.StatusForbidden, ErrCodeCSRFInvalid, "csrf token mismatch")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// originOK is the Origin/Referer host check. Returns true if both headers
// are absent (concept §7.8: SameSite=Lax + auth already cover that case),
// or if at least one of them parses to a host equal to r.Host.
func originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")
	if origin == "" && referer == "" {
		return true
	}
	if origin != "" {
		u, err := url.Parse(origin)
		if err == nil && u.Host == r.Host {
			return true
		}
	}
	if referer != "" {
		u, err := url.Parse(referer)
		if err == nil && u.Host == r.Host {
			return true
		}
	}
	return false
}
