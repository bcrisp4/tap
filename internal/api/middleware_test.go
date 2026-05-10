package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

// newTestDB opens an in-memory database, applies migrations, and registers
// cleanup. Used by middleware tests that need to seed users and sessions
// directly without going through the HTTP mux.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return d
}

func TestRequireSessionPassesValidCookie(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	// Insert user and session manually for the test seam.
	uid, err := db.InsertUser(ctx, d, db.NewUser{
		Username: "ben", PasswordHash: "x", Role: "admin", CreatedAt: 0,
	})
	require.NoError(t, err)

	// Generate cookie + hash deterministically: the cookie is the
	// base64url encoding of the raw 32 bytes; the stored token_hash is
	// hex(sha256(raw_bytes)).
	raw := []byte("0123456789abcdef0123456789abcdef")
	cookieVal := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	tokenHash := hex.EncodeToString(hash[:])

	sid, err := db.InsertSession(ctx, d, db.NewSession{
		UserID:            uid,
		TokenHash:         tokenHash,
		CSRFToken:         "csrf-1",
		CreatedAt:         time.Now().Unix(),
		LastSeenAt:        time.Now().Unix(),
		IdleExpiresAt:     time.Now().Add(time.Hour).Unix(),
		AbsoluteExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	})
	require.NoError(t, err)

	// Build a tiny handler that asserts user + session injection.
	var sawUser db.User
	var sawSession db.Session
	h := requireSession(d, time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := userFromContext(r.Context())
		s, _ := sessionFromContext(r.Context())
		sawUser = u
		sawSession = s
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: cookieVal})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, uid, sawUser.ID)
	require.Equal(t, sid, sawSession.ID)
	require.Equal(t, "csrf-1", sawSession.CSRFToken)
}

func TestRequireSessionRejectsMissingCookie(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	h := requireSession(d, time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusUnauthorized, rr.Code)
	require.Contains(t, rr.Body.String(), `"code":"invalid_session"`)
}

func TestRequireSessionRejectsBadCookie(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	h := requireSession(d, time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: "not-base64-not-a-known-hash!"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestRequireSessionThrottlesIdleRefresh exercises the within-threshold
// guard on RefreshSessionIdle. A session whose last_seen_at is fresher than
// idleRefreshThreshold should NOT be touched on every authenticated request
// (active SPA polls would otherwise flood SQLite with no-op writes); a
// session whose last_seen_at is stale should be refreshed.
func TestRequireSessionThrottlesIdleRefresh(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid, err := db.InsertUser(ctx, d, db.NewUser{
		Username: "ben", PasswordHash: "x", Role: "admin", CreatedAt: 0,
	})
	require.NoError(t, err)

	// Session A: last_seen_at within threshold → middleware must skip refresh.
	rawA := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	cookieA := base64.RawURLEncoding.EncodeToString(rawA)
	hashA := sha256.Sum256(rawA)
	tokenHashA := hex.EncodeToString(hashA[:])
	freshLastSeen := time.Now().Unix() - 30 // well within the 60s threshold
	idA, err := db.InsertSession(ctx, d, db.NewSession{
		UserID: uid, TokenHash: tokenHashA, CSRFToken: "csrf-a",
		CreatedAt: freshLastSeen, LastSeenAt: freshLastSeen,
		IdleExpiresAt: time.Now().Add(time.Hour).Unix(), AbsoluteExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	})
	require.NoError(t, err)

	// Session B: last_seen_at past threshold → middleware must refresh.
	rawB := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	cookieB := base64.RawURLEncoding.EncodeToString(rawB)
	hashB := sha256.Sum256(rawB)
	tokenHashB := hex.EncodeToString(hashB[:])
	staleLastSeen := time.Now().Unix() - 3600 // 1h ago, well past 60s threshold
	idB, err := db.InsertSession(ctx, d, db.NewSession{
		UserID: uid, TokenHash: tokenHashB, CSRFToken: "csrf-b",
		CreatedAt: staleLastSeen, LastSeenAt: staleLastSeen,
		IdleExpiresAt: time.Now().Add(time.Hour).Unix(), AbsoluteExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	})
	require.NoError(t, err)

	h := requireSession(d, time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	// Hit session A: within threshold → last_seen_at preserved.
	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.AddCookie(&http.Cookie{Name: "tap_session", Value: cookieA})
	rrA := httptest.NewRecorder()
	h.ServeHTTP(rrA, reqA)
	require.Equal(t, http.StatusNoContent, rrA.Code)

	gotA, err := db.GetSessionByTokenHash(ctx, d, tokenHashA)
	require.NoError(t, err)
	require.Equal(t, idA, gotA.ID)
	require.Equal(t, freshLastSeen, gotA.LastSeenAt,
		"within-threshold refresh must NOT update last_seen_at")

	// Hit session B: past threshold → last_seen_at advances to now.
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.AddCookie(&http.Cookie{Name: "tap_session", Value: cookieB})
	rrB := httptest.NewRecorder()
	h.ServeHTTP(rrB, reqB)
	require.Equal(t, http.StatusNoContent, rrB.Code)

	gotB, err := db.GetSessionByTokenHash(ctx, d, tokenHashB)
	require.NoError(t, err)
	require.Equal(t, idB, gotB.ID)
	require.Greater(t, gotB.LastSeenAt, staleLastSeen,
		"past-threshold refresh must update last_seen_at")
}

func TestRequireSessionRejectsExpiredAndDeletes(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid, err := db.InsertUser(ctx, d, db.NewUser{Username: "ben", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)

	raw := []byte("0123456789abcdef0123456789abcdef")
	cookieVal := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)

	sid, err := db.InsertSession(ctx, d, db.NewSession{
		UserID:    uid,
		TokenHash: hex.EncodeToString(hash[:]),
		CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0,
		IdleExpiresAt:     1, // far in the past
		AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	h := requireSession(d, time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: cookieVal})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)

	// Session row should be deleted by the middleware.
	_, err = db.GetSessionByTokenHash(ctx, d, hex.EncodeToString(hash[:]))
	require.Error(t, err) // sql.ErrNoRows
	_ = sid
}

// withSession builds a handler chain that fakes session injection without
// going through requireSession (so requireCSRF can be tested in isolation).
func withSession(t *testing.T, csrfToken string, next http.Handler) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := db.Session{ID: 1, UserID: 1, CSRFToken: csrfToken}
		ctx := context.WithValue(r.Context(), ctxKeySession, s)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestRequireCSRFPassesGET(t *testing.T) {
	t.Parallel()
	called := false
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.True(t, called)
}

func TestRequireCSRFRejectsMissingToken(t *testing.T) {
	t.Parallel()
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Origin", "https://tap.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code)
	require.Contains(t, rr.Body.String(), `"code":"csrf_invalid"`)
}

func TestRequireCSRFRejectsWrongToken(t *testing.T) {
	t.Parallel()
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Origin", "https://tap.example")
	req.Header.Set("X-CSRF-Token", "wrong")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireCSRFAcceptsRightToken(t *testing.T) {
	t.Parallel()
	called := false
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Origin", "https://tap.example")
	req.Header.Set("X-CSRF-Token", "tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.True(t, called)
}

func TestRequireCSRFRejectsMismatchedOrigin(t *testing.T) {
	t.Parallel()
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("X-CSRF-Token", "tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireCSRFAcceptsMatchingReferer(t *testing.T) {
	t.Parallel()
	called := false
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Referer", "https://tap.example/some/path")
	req.Header.Set("X-CSRF-Token", "tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.True(t, called)
}

func TestRequireCSRFAcceptsBothMissing(t *testing.T) {
	// Both Origin and Referer missing → SameSite=Lax + an authenticated
	// session already cover the cross-site case. requireCSRF still needs
	// the token; absent Origin/Referer is not on its own grounds for 403.
	t.Parallel()
	called := false
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("X-CSRF-Token", "tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.True(t, called)
}
