package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
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

func userFromContext(ctx context.Context) (db.User, bool) {
	u, ok := ctx.Value(ctxKeyUser).(db.User)
	return u, ok
}

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

// idleRefreshThreshold skips the last_seen_at write when the session was
// touched recently. An active SPA polling every few seconds would otherwise
// produce hundreds of UPDATEs/hour with effectively no state change.
const idleRefreshThreshold = 60 * time.Second

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
			// Skipped when last_seen_at is fresher than idleRefreshThreshold
			// to avoid flooding SQLite with no-op writes on a busy
			// authenticated SPA.
			if now-s.LastSeenAt >= int64(idleRefreshThreshold.Seconds()) {
				_ = db.RefreshSessionIdle(r.Context(), d, s.ID, now, now+int64(idleTTL.Seconds()))
			}

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

// requireAdmin reads the user from context and returns 403 admin_required
// if the user's role is not "admin".
func requireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := userFromContext(r.Context())
			if !ok || u.Role != "admin" {
				writeError(w, http.StatusForbidden, ErrCodeAdminRequired, "admin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientAddress extracts a best-effort client IP from X-Forwarded-For or RemoteAddr.
// Picks the first non-private IP from X-Forwarded-For so deployments behind a single
// trusted reverse proxy get a meaningful address without over-trusting arbitrary chains.
func clientAddress(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for _, part := range strings.Split(xff, ",") {
			candidate := strings.TrimSpace(part)
			ip := net.ParseIP(candidate)
			if ip != nil && !ip.IsPrivate() && !ip.IsLoopback() {
				return candidate
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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

// originOK enforces same-origin discipline as a defence-in-depth check
// alongside the X-CSRF-Token. Origin is authoritative when present;
// Referer is only consulted as a fallback when Origin is absent. If both
// are absent, requireCSRF accepts (concept §7.8: SameSite=Lax + an
// authenticated session already cover that case).
func originOK(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		u, err := url.Parse(referer)
		return err == nil && u.Host == r.Host
	}
	return true
}
