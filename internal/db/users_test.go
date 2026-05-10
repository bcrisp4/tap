package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInsertUserAndGetByUsername(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertUser(ctx, d, NewUser{
		Username:     "ben",
		PasswordHash: "$argon2id$...",
		Role:         "admin",
		CreatedAt:    1_700_000_000,
	})
	require.NoError(t, err)
	require.NotZero(t, id)

	u, err := GetUserByUsername(ctx, d, "ben")
	require.NoError(t, err)
	require.Equal(t, id, u.ID)
	require.Equal(t, "ben", u.Username)
	require.Equal(t, "$argon2id$...", u.PasswordHash)
	require.Equal(t, "admin", u.Role)
	require.Equal(t, int64(1_700_000_000), u.CreatedAt)
	require.False(t, u.DisabledAt.Valid)
}

func TestInsertUserDuplicateUsername(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	_, err := InsertUser(ctx, d, NewUser{Username: "alice", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)
	_, err = InsertUser(ctx, d, NewUser{Username: "alice", PasswordHash: "y", Role: "user", CreatedAt: 0})
	require.ErrorIs(t, err, ErrUserExists)
}

func TestGetUserByUsernameMissing(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	_, err := GetUserByUsername(context.Background(), d, "nobody")
	require.True(t, errors.Is(err, sql.ErrNoRows), "got: %v", err)
}

func TestGetUserByID(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	id, err := InsertUser(ctx, d, NewUser{Username: "ben", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)

	u, err := GetUserByID(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, "ben", u.Username)

	_, err = GetUserByID(ctx, d, id+999)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdatePasswordHash(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	id, err := InsertUser(ctx, d, NewUser{Username: "ben", PasswordHash: "old", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)

	require.NoError(t, UpdatePasswordHash(ctx, d, id, "new"))

	u, err := GetUserByID(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, "new", u.PasswordHash)
}

func TestDisableUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	id, err := InsertUser(ctx, d, NewUser{Username: "ben", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)

	require.NoError(t, DisableUser(ctx, d, id, 1_700_000_999))

	u, err := GetUserByID(ctx, d, id)
	require.NoError(t, err)
	require.True(t, u.DisabledAt.Valid)
	require.Equal(t, int64(1_700_000_999), u.DisabledAt.Int64)
}

func TestCountUsers(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	n, err := CountUsers(ctx, d)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	_, err = InsertUser(ctx, d, NewUser{Username: "a", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)
	_, err = InsertUser(ctx, d, NewUser{Username: "b", PasswordHash: "y", Role: "user", CreatedAt: 0})
	require.NoError(t, err)

	n, err = CountUsers(ctx, d)
	require.NoError(t, err)
	require.Equal(t, 2, n)
}

func TestListUsers(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	_, _ = InsertUser(ctx, d, NewUser{Username: "alice", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	_, _ = InsertUser(ctx, d, NewUser{Username: "bob", PasswordHash: "y", Role: "user", CreatedAt: 0})

	users, err := ListUsers(ctx, d)
	require.NoError(t, err)
	require.Len(t, users, 2)
	require.Equal(t, "alice", users[0].Username)
	require.Equal(t, "bob", users[1].Username)
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertUser(ctx, d, NewUser{Username: "charlie", PasswordHash: "x", Role: "user", CreatedAt: 0})
	require.NoError(t, err)

	require.NoError(t, DeleteUser(ctx, d, id))

	_, err = GetUserByID(ctx, d, id)
	require.ErrorIs(t, err, sql.ErrNoRows)

	err = DeleteUser(ctx, d, id)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestEnableUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertUser(ctx, d, NewUser{Username: "diana", PasswordHash: "x", Role: "user", CreatedAt: 0})
	require.NoError(t, err)
	require.NoError(t, DisableUser(ctx, d, id, 1000))

	u, err := GetUserByID(ctx, d, id)
	require.NoError(t, err)
	require.True(t, u.DisabledAt.Valid)

	require.NoError(t, EnableUser(ctx, d, id))

	u2, err := GetUserByID(ctx, d, id)
	require.NoError(t, err)
	require.False(t, u2.DisabledAt.Valid)
}

func TestGetUserTOTPStatus(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	id := insertTestUser(t, d, "ethan")

	hasTOTP, confirmed, err := GetUserTOTPStatus(ctx, d, id)
	require.NoError(t, err)
	require.False(t, hasTOTP)
	require.False(t, confirmed)

	require.NoError(t, InsertTOTPSecret(ctx, d, id, []byte("enc")))
	hasTOTP, confirmed, err = GetUserTOTPStatus(ctx, d, id)
	require.NoError(t, err)
	require.True(t, hasTOTP)
	require.False(t, confirmed)

	require.NoError(t, ConfirmTOTPSecret(ctx, d, id))
	hasTOTP, confirmed, err = GetUserTOTPStatus(ctx, d, id)
	require.NoError(t, err)
	require.True(t, hasTOTP)
	require.True(t, confirmed)
}

func TestGetUserPasskeyCount(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	id := insertTestUser(t, d, "fiona")

	n, err := GetUserPasskeyCount(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	_, _ = InsertPasskey(ctx, d, Passkey{UserID: id, CredentialID: []byte("c1"), PublicKey: []byte("k1")})
	_, _ = InsertPasskey(ctx, d, Passkey{UserID: id, CredentialID: []byte("c2"), PublicKey: []byte("k2")})

	n, err = GetUserPasskeyCount(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, 2, n)
}
