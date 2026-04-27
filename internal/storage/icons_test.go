package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func TestIcons_InsertAndGetByHash(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id, err := s.InsertIcon(ctx, "abc123", "image/png", []byte{0x89, 'P', 'N', 'G'})
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	got, err := s.GetIconByHash(ctx, "abc123")
	require.NoError(t, err)
	require.Equal(t, id, got.ID)
	require.Equal(t, "image/png", got.MIMEType)
	require.Equal(t, []byte{0x89, 'P', 'N', 'G'}, got.Content)
}

func TestIcons_GetByHashMissingReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetIconByHash(context.Background(), "nope")
	require.True(t, errors.Is(err, storage.ErrNotFound))
}

func TestIcons_DuplicateHashErrors(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, err := s.InsertIcon(ctx, "h", "image/png", []byte{1})
	require.NoError(t, err)
	_, err = s.InsertIcon(ctx, "h", "image/png", []byte{1})
	require.Error(t, err)
}
