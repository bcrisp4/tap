package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func insertTestUser(t *testing.T, d *sql.DB, username string) int64 {
	t.Helper()
	id, err := InsertUser(context.Background(), d, NewUser{
		Username: username, PasswordHash: "x", Role: "admin", CreatedAt: 0,
	})
	require.NoError(t, err)
	return id
}

func TestInsertAndGetSessionByTokenHash(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")

	id, err := InsertSession(ctx, d, NewSession{
		UserID:            uid,
		TokenHash:         "abc123",
		CSRFToken:         "csrf-1",
		CreatedAt:         100,
		LastSeenAt:        100,
		IdleExpiresAt:     200,
		AbsoluteExpiresAt: 1_000_000,
	})
	require.NoError(t, err)
	require.NotZero(t, id)

	s, err := GetSessionByTokenHash(ctx, d, "abc123")
	require.NoError(t, err)
	require.Equal(t, id, s.ID)
	require.Equal(t, uid, s.UserID)
	require.Equal(t, "csrf-1", s.CSRFToken)
	require.Equal(t, int64(100), s.CreatedAt)
	require.Equal(t, int64(100), s.LastSeenAt)
	require.Equal(t, int64(200), s.IdleExpiresAt)
	require.Equal(t, int64(1_000_000), s.AbsoluteExpiresAt)

	_, err = GetSessionByTokenHash(ctx, d, "no-such-hash")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestRefreshSessionIdle(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")
	sid, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "h", CSRFToken: "c",
		CreatedAt: 100, LastSeenAt: 100, IdleExpiresAt: 200, AbsoluteExpiresAt: 1_000_000,
	})
	require.NoError(t, err)

	require.NoError(t, RefreshSessionIdle(ctx, d, sid, 500, 700))

	s, err := GetSessionByTokenHash(ctx, d, "h")
	require.NoError(t, err)
	require.Equal(t, int64(500), s.LastSeenAt)
	require.Equal(t, int64(700), s.IdleExpiresAt)
	require.Equal(t, int64(1_000_000), s.AbsoluteExpiresAt, "absolute should not change on idle refresh")
}

func TestUpdateSessionCSRFToken(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")
	sid, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "h", CSRFToken: "old",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	require.NoError(t, UpdateSessionCSRFToken(ctx, d, sid, "new"))
	s, err := GetSessionByTokenHash(ctx, d, "h")
	require.NoError(t, err)
	require.Equal(t, "new", s.CSRFToken)
}

func TestDeleteSession(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")
	sid, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "h", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	require.NoError(t, DeleteSession(ctx, d, sid))

	_, err = GetSessionByTokenHash(ctx, d, "h")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDeleteSessionsByUserID(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")
	other := insertTestUser(t, d, "alice")

	for _, h := range []string{"h1", "h2", "h3"} {
		_, err := InsertSession(ctx, d, NewSession{
			UserID: uid, TokenHash: h, CSRFToken: "c",
			CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
		})
		require.NoError(t, err)
	}
	_, err := InsertSession(ctx, d, NewSession{
		UserID: other, TokenHash: "untouched", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	require.NoError(t, DeleteSessionsByUserID(ctx, d, uid))

	for _, h := range []string{"h1", "h2", "h3"} {
		_, err := GetSessionByTokenHash(ctx, d, h)
		require.ErrorIs(t, err, sql.ErrNoRows, "hash %s should be gone", h)
	}
	_, err = GetSessionByTokenHash(ctx, d, "untouched")
	require.NoError(t, err, "the other user's session should remain")
}

func TestDeleteOtherSessionsForUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")

	keepID, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "keep", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)
	for _, h := range []string{"drop1", "drop2"} {
		_, err := InsertSession(ctx, d, NewSession{
			UserID: uid, TokenHash: h, CSRFToken: "c",
			CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
		})
		require.NoError(t, err)
	}

	require.NoError(t, DeleteOtherSessionsForUser(ctx, d, uid, keepID))

	_, err = GetSessionByTokenHash(ctx, d, "keep")
	require.NoError(t, err)
	for _, h := range []string{"drop1", "drop2"} {
		_, err := GetSessionByTokenHash(ctx, d, h)
		require.ErrorIs(t, err, sql.ErrNoRows, "hash %s should be gone", h)
	}
}

func TestListSessionsByUserID(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid1 := insertTestUser(t, d, "ken")
	uid2 := insertTestUser(t, d, "lou")

	_, err := InsertSession(ctx, d, NewSession{
		UserID: uid1, TokenHash: "h1", CSRFToken: "c",
		CreatedAt: 1, LastSeenAt: 1, IdleExpiresAt: 1, AbsoluteExpiresAt: 9999999999,
	})
	require.NoError(t, err)
	_, err = InsertSession(ctx, d, NewSession{
		UserID: uid1, TokenHash: "h2", CSRFToken: "c",
		CreatedAt: 2, LastSeenAt: 2, IdleExpiresAt: 2, AbsoluteExpiresAt: 9999999999,
	})
	require.NoError(t, err)
	_, err = InsertSession(ctx, d, NewSession{
		UserID: uid2, TokenHash: "h3", CSRFToken: "c",
		CreatedAt: 1, LastSeenAt: 1, IdleExpiresAt: 1, AbsoluteExpiresAt: 9999999999,
	})
	require.NoError(t, err)

	sessions, err := ListSessionsByUserID(ctx, d, uid1)
	require.NoError(t, err)
	require.Len(t, sessions, 2, "should return only uid1 sessions")

	sessions2, err := ListSessionsByUserID(ctx, d, uid2)
	require.NoError(t, err)
	require.Len(t, sessions2, 1)
}

func TestSession_UserAgentAndAddress(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "mia")

	_, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "h1ua", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 9999999999,
		UserAgent: "Mozilla/5.0", Address: "1.2.3.4",
	})
	require.NoError(t, err)

	s, err := GetSessionByTokenHash(ctx, d, "h1ua")
	require.NoError(t, err)
	require.Equal(t, "Mozilla/5.0", s.UserAgent)
	require.Equal(t, "1.2.3.4", s.Address)
}

func TestWebAuthnChallenge_SetAndClear(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ned")

	sid, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "hwac", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 9999999999,
	})
	require.NoError(t, err)

	challenge := []byte(`{"challenge":"abc123"}`)
	require.NoError(t, SetWebAuthnChallenge(ctx, d, sid, challenge))

	s, err := GetSessionByTokenHash(ctx, d, "hwac")
	require.NoError(t, err)
	require.Equal(t, challenge, s.WebAuthnChallenge)

	require.NoError(t, ClearWebAuthnChallenge(ctx, d, sid))
	s2, err := GetSessionByTokenHash(ctx, d, "hwac")
	require.NoError(t, err)
	require.Nil(t, s2.WebAuthnChallenge)
}
