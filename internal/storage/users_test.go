package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func TestUsers_DefaultUserSeeded(t *testing.T) {
	s := newTestStore(t)
	u, err := s.GetUser(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), u.ID)
	require.Equal(t, "default", u.Username)
	require.Equal(t, "system", u.Theme)
	require.Equal(t, "serif", u.Font)
	require.Equal(t, 50, u.EntriesPerPage)
	require.Equal(t, "published_at", u.DefaultSort)
	require.Equal(t, "desc", u.DefaultOrder)
}

func TestUsers_GetMissingReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetUser(context.Background(), 999)
	require.True(t, errors.Is(err, storage.ErrNotFound))
}

func TestUsers_Create(t *testing.T) {
	s := newTestStore(t)
	id, err := s.CreateUser(context.Background(), "alice")
	require.NoError(t, err)
	require.Greater(t, id, int64(1))

	u, err := s.GetUser(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "alice", u.Username)
}

func TestUsers_CreateDuplicateUsernameErrors(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CreateUser(context.Background(), "default") // already seeded
	require.Error(t, err)
}
