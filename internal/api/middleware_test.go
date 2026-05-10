package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
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
	_ = strings.TrimSpace
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
