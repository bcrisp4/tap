package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func TestCategories_CRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Create
	id, err := s.CreateCategory(ctx, 1, "Tech")
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// List
	cats, err := s.ListCategories(ctx, 1)
	require.NoError(t, err)
	require.Len(t, cats, 1)
	require.Equal(t, "Tech", cats[0].Name)

	// Rename
	require.NoError(t, s.RenameCategory(ctx, 1, id, "Engineering"))
	cats, err = s.ListCategories(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "Engineering", cats[0].Name)

	// Delete
	require.NoError(t, s.DeleteCategory(ctx, 1, id))
	cats, err = s.ListCategories(ctx, 1)
	require.NoError(t, err)
	require.Len(t, cats, 0)
}

func TestCategories_DuplicateNamePerUserErrors(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, err := s.CreateCategory(ctx, 1, "X")
	require.NoError(t, err)
	_, err = s.CreateCategory(ctx, 1, "X")
	require.Error(t, err, "UNIQUE(user_id, name) must reject")
}

func TestCategories_RenameMissingErrors(t *testing.T) {
	s := newTestStore(t)
	err := s.RenameCategory(context.Background(), 1, 999, "Y")
	require.True(t, errors.Is(err, storage.ErrNotFound))
}
