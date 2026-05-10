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
