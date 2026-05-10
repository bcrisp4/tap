package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPasskey_Roundtrip(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	userID := insertTestUser(t, d, "dave")

	p := Passkey{
		UserID:       userID,
		CredentialID: []byte("credid1"),
		PublicKey:    []byte("pubkey1"),
		SignCounter:  0,
		AAGUID:       []byte("aaguid1"),
		Label:        "MacBook Touch ID",
		CreatedAt:    time.Now().Unix(),
	}
	id, err := InsertPasskey(ctx, d, p)
	require.NoError(t, err)
	require.Positive(t, id)

	got, err := GetPasskeyByCredentialID(ctx, d, []byte("credid1"))
	require.NoError(t, err)
	require.Equal(t, userID, got.UserID)
	require.Equal(t, "MacBook Touch ID", got.Label)
}

func TestPasskey_ListAndDelete(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	u1 := insertTestUser(t, d, "eve")
	u2 := insertTestUser(t, d, "frank")

	// Insert 2 passkeys for u1, 1 for u2
	_, _ = InsertPasskey(ctx, d, Passkey{UserID: u1, CredentialID: []byte("c1"), PublicKey: []byte("k1"), Label: "A"})
	id2, _ := InsertPasskey(ctx, d, Passkey{UserID: u1, CredentialID: []byte("c2"), PublicKey: []byte("k2"), Label: "B"})
	_, _ = InsertPasskey(ctx, d, Passkey{UserID: u2, CredentialID: []byte("c3"), PublicKey: []byte("k3"), Label: "C"})

	keys1, err := GetPasskeysByUserID(ctx, d, u1)
	require.NoError(t, err)
	require.Len(t, keys1, 2)

	keys2, err := GetPasskeysByUserID(ctx, d, u2)
	require.NoError(t, err)
	require.Len(t, keys2, 1)

	// Delete u1's second passkey
	require.NoError(t, DeletePasskey(ctx, d, id2, u1))
	keys1After, _ := GetPasskeysByUserID(ctx, d, u1)
	require.Len(t, keys1After, 1)

	// u2 cannot delete u1's passkey
	keys1First := keys1After[0]
	err = DeletePasskey(ctx, d, keys1First.ID, u2)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestPasskey_UpdateSignCounter(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	userID := insertTestUser(t, d, "grace")

	id, err := InsertPasskey(ctx, d, Passkey{UserID: userID, CredentialID: []byte("c"), PublicKey: []byte("k"), SignCounter: 0, Label: "x"})
	require.NoError(t, err)

	require.NoError(t, UpdatePasskeySignCounter(ctx, d, id, 42))

	got, err := GetPasskeyByCredentialID(ctx, d, []byte("c"))
	require.NoError(t, err)
	require.Equal(t, int64(42), got.SignCounter)
}
