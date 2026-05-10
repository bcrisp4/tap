package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPendingLogin_Roundtrip(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	userID := insertTestUser(t, d, "henry")

	err := InsertPendingLogin(ctx, d, userID, "testhash", 9999999999)
	require.NoError(t, err)

	p, err := GetPendingLoginByTokenHash(ctx, d, "testhash")
	require.NoError(t, err)
	require.Equal(t, userID, p.UserID)
	require.Equal(t, "testhash", p.TokenHash)
	require.Equal(t, int64(9999999999), p.ExpiresAt)
}

func TestPendingLogin_Delete(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	userID := insertTestUser(t, d, "ida")

	require.NoError(t, InsertPendingLogin(ctx, d, userID, "h1", 9999999999))

	p, err := GetPendingLoginByTokenHash(ctx, d, "h1")
	require.NoError(t, err)
	require.NoError(t, DeletePendingLogin(ctx, d, p.ID))

	_, err = GetPendingLoginByTokenHash(ctx, d, "h1")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestPendingLogin_DeleteExpired(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	userID := insertTestUser(t, d, "jack")

	// Insert one expired and one valid.
	require.NoError(t, InsertPendingLogin(ctx, d, userID, "expired", 100))
	require.NoError(t, InsertPendingLogin(ctx, d, userID, "valid", 9999999999))

	require.NoError(t, DeleteExpiredPendingLogins(ctx, d, 200))

	_, err := GetPendingLoginByTokenHash(ctx, d, "expired")
	require.ErrorIs(t, err, sql.ErrNoRows, "expired token should be deleted")

	_, err = GetPendingLoginByTokenHash(ctx, d, "valid")
	require.NoError(t, err, "valid token should still exist")
}
