package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/bcrisp4/tap/internal/ratelimit"
)

// CookieSecureMode controls the Secure attribute on the tap_session cookie.
type CookieSecureMode int

const (
	CookieSecureAuto CookieSecureMode = iota
	CookieSecureTrue
	CookieSecureFalse
)

// ResolveCookieSecure is the exported wrapper used by cmd/tap to resolve
// the --cookie-secure flag once at startup.
func ResolveCookieSecure(mode CookieSecureMode, addr string) bool {
	return resolveCookieSecure(mode, addr)
}

func resolveCookieSecure(mode CookieSecureMode, addr string) bool {
	switch mode {
	case CookieSecureTrue:
		return true
	case CookieSecureFalse:
		return false
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return true
	}
	host = strings.TrimSpace(host)
	if host == "localhost" {
		return false
	}
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}

// userDTO is the user shape exposed via the API. Never carries password_hash.
type userDTO struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	HasTOTP      bool   `json:"has_totp"`
	PasskeyCount int    `json:"passkey_count"`
}

func toUserDTO(u db.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Role: u.Role}
}

type loginRequest struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	PendingToken string `json:"pending_token"`
	TOTPCode     string `json:"totp_code"`
	RecoveryCode string `json:"recovery_code"`
}

type loginResponse struct {
	User      userDTO `json:"user"`
	CSRFToken string  `json:"csrf_token"`
}

type totpRequiredResponse struct {
	TOTPRequired bool   `json:"totp_required"`
	PendingToken string `json:"pending_token"`
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type passwordChangeResponse struct {
	CSRFToken string `json:"csrf_token"`
}

// authDeps bundles the dependencies the auth handlers need.
type authDeps struct {
	d                  *sql.DB
	sessionIdleTTL     time.Duration
	sessionAbsoluteTTL time.Duration
	cookieSecure       bool
	limiter            *ratelimit.Limiter // nil = no rate limiting
	trustedProxy       bool
	hashParams         auth.Params
}

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

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "tap_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// hashPendingToken converts a pending-token cookie value to its storage hash.
func hashPendingToken(value string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), true
}

// buildUserDTO populates TOTP and passkey counts on a userDTO.
// d may be nil (for tests that pass nil DB); in that case extra fields stay zero.
func buildUserDTO(ctx context.Context, d *sql.DB, u db.User) userDTO {
	dto := toUserDTO(u)
	if d != nil {
		hasTOTP, confirmed, _ := db.GetUserTOTPStatus(ctx, d, u.ID)
		dto.HasTOTP = hasTOTP && confirmed
		dto.PasskeyCount, _ = db.GetUserPasskeyCount(ctx, d, u.ID)
	}
	return dto
}

// mintAndInsertSession creates a new session row and sets the cookie.
func mintAndInsertSession(w http.ResponseWriter, r *http.Request, dep authDeps, userID int64) (csrfToken string, err error) {
	cookieValue, tokenHash, err := auth.MintSessionToken()
	if err != nil {
		return "", err
	}
	csrfToken, err = auth.MintCSRFToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	_, err = db.InsertSession(r.Context(), dep.d, db.NewSession{
		UserID:            userID,
		TokenHash:         tokenHash,
		CSRFToken:         csrfToken,
		CreatedAt:         now.Unix(),
		LastSeenAt:        now.Unix(),
		IdleExpiresAt:     now.Add(dep.sessionIdleTTL).Unix(),
		AbsoluteExpiresAt: now.Add(dep.sessionAbsoluteTTL).Unix(),
		UserAgent:         r.Header.Get("User-Agent"),
		Address:           clientAddress(r),
	})
	if err != nil {
		return "", err
	}
	setSessionCookie(w, cookieValue, dep.sessionAbsoluteTTL, dep.cookieSecure)
	return csrfToken, nil
}

// validateRecoveryCode checks a plaintext recovery code against stored argon2id hashes.
func validateRecoveryCode(ctx context.Context, d *sql.DB, userID int64, code string) bool {
	codes, err := db.GetUnconsumedRecoveryCodes(ctx, d, userID)
	if err != nil {
		return false
	}
	for _, rc := range codes {
		ok, err := auth.Verify(rc.CodeHash, code)
		if err == nil && ok {
			_ = db.ConsumeRecoveryCode(ctx, d, rc.ID)
			return true
		}
	}
	return false
}

// loginHandler returns POST /api/v1/sessions. Public, CSRF not required.
func loginHandler(dep authDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = db.DeleteExpiredPendingLogins(r.Context(), dep.d, time.Now().Unix())

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body loginRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		// TOTP second step: pending_token present.
		if body.PendingToken != "" {
			tokenHash, ok := hashPendingToken(body.PendingToken)
			if !ok {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "invalid pending token")
				return
			}
			pl, err := db.GetPendingLoginByTokenHash(r.Context(), dep.d, tokenHash)
			if err != nil {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "invalid or expired pending token")
				return
			}
			if time.Now().Unix() > pl.ExpiresAt {
				_ = db.DeletePendingLogin(r.Context(), dep.d, pl.ID)
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "pending token expired")
				return
			}
			_ = db.DeletePendingLogin(r.Context(), dep.d, pl.ID)

			u, err := db.GetUserByID(r.Context(), dep.d, pl.UserID)
			if err != nil {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "user not found")
				return
			}

			if body.RecoveryCode != "" {
				if !validateRecoveryCode(r.Context(), dep.d, pl.UserID, body.RecoveryCode) {
					writeError(w, http.StatusUnauthorized, ErrCodeRecoveryCodeInvalid, "invalid recovery code")
					return
				}
			} else if body.TOTPCode != "" {
				totpSecret, err := db.GetTOTPSecret(r.Context(), dep.d, pl.UserID)
				if err != nil {
					writeError(w, http.StatusUnauthorized, ErrCodeTOTPInvalid, "no TOTP secret found")
					return
				}
				encKey, err := getTOTPEncryptionKey(r.Context(), dep.d)
				if err != nil {
					writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
					return
				}
				plainSecret, err := auth.DecryptTOTPSecret(encKey, totpSecret.SecretEncrypted)
				if err != nil {
					writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
					return
				}
				if !auth.VerifyTOTP(plainSecret, body.TOTPCode) {
					writeError(w, http.StatusUnauthorized, ErrCodeTOTPInvalid, "invalid TOTP code")
					return
				}
			} else {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "totp_code or recovery_code required")
				return
			}

			csrfToken, err := mintAndInsertSession(w, r, dep, u.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, loginResponse{User: buildUserDTO(r.Context(), dep.d, u), CSRFToken: csrfToken})
			return
		}

		// First step: username + password.
		username := strings.TrimSpace(body.Username)
		if username == "" || body.Password == "" {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}

		source := sourceIP(r, dep.trustedProxy)

		// Rate limit check.
		if dep.limiter != nil {
			allowed, retryAfter := dep.limiter.Allow(source, username)
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				metrics.LoginAttempts.Add(r.Context(), 1, metric.WithAttributes(attribute.String("result", "rate_limited")))
				slog.WarnContext(r.Context(), "login rate limited",
					"event", "auth.login.failure",
					"username", username,
					"source", source,
					"reason", "rate_limited")
				writeError(w, http.StatusTooManyRequests, ErrCodeRateLimited, "too many requests")
				return
			}
		}

		u, err := db.GetUserByUsername(r.Context(), dep.d, username)
		if err != nil {
			if dep.limiter != nil {
				dep.limiter.RecordFailure(source, username)
			}
			metrics.LoginAttempts.Add(r.Context(), 1, metric.WithAttributes(attribute.String("result", "failure")))
			slog.WarnContext(r.Context(), "login failed",
				"event", "auth.login.failure",
				"username", username,
				"source", source,
				"reason", "bad_credentials")
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}
		if u.DisabledAt.Valid {
			if dep.limiter != nil {
				dep.limiter.RecordFailure(source, username)
			}
			metrics.LoginAttempts.Add(r.Context(), 1, metric.WithAttributes(attribute.String("result", "failure")))
			slog.WarnContext(r.Context(), "login failed",
				"event", "auth.login.failure",
				"username", username,
				"source", source,
				"reason", "disabled")
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}
		ok, err := auth.Verify(u.PasswordHash, body.Password)
		if err != nil || !ok {
			if dep.limiter != nil {
				dep.limiter.RecordFailure(source, username)
			}
			metrics.LoginAttempts.Add(r.Context(), 1, metric.WithAttributes(attribute.String("result", "failure")))
			slog.WarnContext(r.Context(), "login failed",
				"event", "auth.login.failure",
				"username", username,
				"source", source,
				"reason", "bad_credentials")
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}

		// Re-hash on verify if params are weaker than current.
		if dep.hashParams != (auth.Params{}) {
			if needs, err := auth.NeedsRehash(u.PasswordHash, dep.hashParams); err == nil && needs {
				if newHash, err := auth.Hash(body.Password, dep.hashParams); err == nil {
					_ = db.UpdatePasswordHash(r.Context(), dep.d, u.ID, newHash)
					slog.InfoContext(r.Context(), "argon2 params upgraded on verify",
						"event", "auth.rehash",
						"user_id", u.ID)
				}
			}
		}

		if dep.limiter != nil {
			dep.limiter.RecordSuccess(username)
		}
		metrics.LoginAttempts.Add(r.Context(), 1, metric.WithAttributes(attribute.String("result", "success")))
		slog.InfoContext(r.Context(), "login successful",
			"event", "auth.login.success",
			"user_id", u.ID,
			"username", u.Username,
			"source", source)

		hasTOTP, confirmed, err := db.GetUserTOTPStatus(r.Context(), dep.d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if hasTOTP && confirmed {
			tokenValue, tokenHash, err := auth.MintPendingToken()
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			expiresAt := time.Now().Add(5 * time.Minute).Unix()
			if err := db.InsertPendingLogin(r.Context(), dep.d, u.ID, tokenHash, expiresAt); err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, totpRequiredResponse{TOTPRequired: true, PendingToken: tokenValue})
			return
		}

		csrfToken, err := mintAndInsertSession(w, r, dep, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, loginResponse{User: buildUserDTO(r.Context(), dep.d, u), CSRFToken: csrfToken})
	})
}

// getSessionCurrentHandler returns GET /api/v1/sessions/current.
func getSessionCurrentHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok1 := userFromContext(r.Context())
		s, ok2 := sessionFromContext(r.Context())
		if !ok1 || !ok2 {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		dto := buildUserDTO(r.Context(), d, u)
		writeJSON(w, http.StatusOK, loginResponse{User: dto, CSRFToken: s.CSRFToken})
	})
}

// logoutHandler returns DELETE /api/v1/sessions/current.
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

// passwordChangeHandler returns PATCH /api/v1/me/password.
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
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "request body too large")
				return
			}
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

type sessionListItemDTO struct {
	ID            int64  `json:"id"`
	CreatedAt     int64  `json:"created_at"`
	LastSeenAt    int64  `json:"last_seen_at"`
	IdleExpiresAt int64  `json:"idle_expires_at"`
	UserAgent     string `json:"user_agent"`
	Address       string `json:"address"`
	Current       bool   `json:"current"`
}

func listSessionsHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		s, ok2 := sessionFromContext(r.Context())
		if !ok || !ok2 {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		sessions, err := db.ListSessionsByUserID(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		out := make([]sessionListItemDTO, 0, len(sessions))
		for _, sess := range sessions {
			out = append(out, sessionListItemDTO{
				ID:            sess.ID,
				CreatedAt:     sess.CreatedAt,
				LastSeenAt:    sess.LastSeenAt,
				IdleExpiresAt: sess.IdleExpiresAt,
				UserAgent:     sess.UserAgent,
				Address:       sess.Address,
				Current:       sess.ID == s.ID,
			})
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func revokeSessionHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		current, ok2 := sessionFromContext(r.Context())
		if !ok || !ok2 {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid session id")
			return
		}
		if id == current.ID {
			writeError(w, http.StatusBadRequest, ErrCodeCannotRevokeCurrentSession, "cannot revoke current session")
			return
		}
		sessions, err := db.ListSessionsByUserID(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		var found bool
		for _, s := range sessions {
			if s.ID == id {
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "session not found")
			return
		}
		if err := db.DeleteSession(r.Context(), d, id); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// sourceIP extracts the client IP from the request.
// If trustedProxy is true, uses the leftmost X-Forwarded-For value.
func sourceIP(r *http.Request, trustedProxy bool) string {
	if trustedProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if idx := strings.Index(fwd, ","); idx >= 0 {
				return strings.TrimSpace(fwd[:idx])
			}
			return strings.TrimSpace(fwd)
		}
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func revokeAllOtherSessionsHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		s, ok2 := sessionFromContext(r.Context())
		if !ok || !ok2 {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		if err := db.DeleteOtherSessionsForUser(r.Context(), d, u.ID, s.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
