package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func mustEntry(t *testing.T, s *storage.Store) int64 {
	t.Helper()
	feedID := mustFeed(t, s)
	id, err := s.InsertEntry(context.Background(), &storage.Entry{
		FeedID: feedID, UserID: 1, Hash: "h", Title: "T",
	})
	require.NoError(t, err)
	return id
}

func TestEnclosures_InsertAndList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	entryID := mustEntry(t, s)

	id, err := s.InsertEnclosure(ctx, entryID, "https://example.com/a.mp3", "audio/mpeg", 12345)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	list, err := s.ListEnclosures(ctx, entryID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "audio/mpeg", list[0].MIMEType)
	require.Equal(t, int64(12345), list[0].Size)
}

func TestEnclosures_DuplicateURLPerEntryErrors(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	entryID := mustEntry(t, s)
	_, err := s.InsertEnclosure(ctx, entryID, "https://example.com/x", "", 0)
	require.NoError(t, err)
	_, err = s.InsertEnclosure(ctx, entryID, "https://example.com/x", "", 0)
	require.Error(t, err)
}
