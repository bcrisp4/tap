package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTOTPSecret_Roundtrip(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	userID := insertTestUser(t, d, "alice")

	err := InsertTOTPSecret(ctx, d, userID, []byte("encryptedbytes"))
	require.NoError(t, err)

	s, err := GetTOTPSecret(ctx, d, userID)
	require.NoError(t, err)
	require.Equal(t, userID, s.UserID)
	require.Equal(t, []byte("encryptedbytes"), s.SecretEncrypted)
	require.False(t, s.Confirmed)
}

func TestTOTPSecret_ConfirmAndDelete(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	userID := insertTestUser(t, d, "bob")

	require.NoError(t, InsertTOTPSecret(ctx, d, userID, []byte("enc")))
	require.NoError(t, ConfirmTOTPSecret(ctx, d, userID))

	s, err := GetTOTPSecret(ctx, d, userID)
	require.NoError(t, err)
	require.True(t, s.Confirmed)

	require.NoError(t, DeleteTOTPSecret(ctx, d, userID))
	_, err = GetTOTPSecret(ctx, d, userID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestTOTPSecret_UpsertReplacesUnconfirmed(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	userID := insertTestUser(t, d, "carol")

	require.NoError(t, InsertTOTPSecret(ctx, d, userID, []byte("first")))
	require.NoError(t, InsertTOTPSecret(ctx, d, userID, []byte("second"))) // upsert

	s, err := GetTOTPSecret(ctx, d, userID)
	require.NoError(t, err)
	require.Equal(t, []byte("second"), s.SecretEncrypted)
	require.False(t, s.Confirmed, "upsert should reset confirmed to false")
}

func TestRecoveryCodes_InsertConsumeDelete(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	userID := insertTestUser(t, d, "dave")

	hashes := []string{"hash1", "hash2", "hash3"}
	require.NoError(t, InsertRecoveryCodes(ctx, d, userID, hashes))

	codes, err := GetUnconsumedRecoveryCodes(ctx, d, userID)
	require.NoError(t, err)
	require.Len(t, codes, 3)

	require.NoError(t, ConsumeRecoveryCode(ctx, d, codes[0].ID))
	remaining, err := GetUnconsumedRecoveryCodes(ctx, d, userID)
	require.NoError(t, err)
	require.Len(t, remaining, 2)

	require.NoError(t, DeleteRecoveryCodes(ctx, d, userID))
	empty, err := GetUnconsumedRecoveryCodes(ctx, d, userID)
	require.NoError(t, err)
	require.Empty(t, empty)
}
