package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTombstones_InsertAndHas(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID := mustFeed(t, s)

	has, err := s.HasTombstone(ctx, feedID, "h")
	require.NoError(t, err)
	require.False(t, has)

	require.NoError(t, s.InsertTombstone(ctx, feedID, "h"))

	has, err = s.HasTombstone(ctx, feedID, "h")
	require.NoError(t, err)
	require.True(t, has)
}

func TestTombstones_InsertIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID := mustFeed(t, s)
	require.NoError(t, s.InsertTombstone(ctx, feedID, "h"))
	require.NoError(t, s.InsertTombstone(ctx, feedID, "h"), "duplicate must not error (INSERT OR IGNORE)")
}
