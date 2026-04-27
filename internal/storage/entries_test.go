package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func mustFeed(t *testing.T, s *storage.Store) int64 {
	t.Helper()
	id, err := s.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "F", FeedURL: "https://example.com/f", PollInterval: 3600,
	})
	require.NoError(t, err)
	return id
}

func TestEntries_InsertGet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID := mustFeed(t, s)

	pub := int64(1700000000)
	id, err := s.InsertEntry(ctx, &storage.Entry{
		FeedID:      feedID,
		UserID:      1,
		Hash:        "h1",
		Title:       "Hello",
		URL:         strPtr("https://example.com/hello"),
		PublishedAt: &pub,
		Content:     strPtr("<p>Hi</p>"),
		ReadingTime: 3,
	})
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	got, err := s.GetEntry(ctx, 1, id)
	require.NoError(t, err)
	require.Equal(t, "Hello", got.Title)
	require.Equal(t, "<p>Hi</p>", *got.Content)
	require.Equal(t, 3, got.ReadingTime)
	require.False(t, got.Read)
	require.False(t, got.Saved)
}

func TestEntries_InsertDuplicateHashErrors(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID := mustFeed(t, s)

	_, err := s.InsertEntry(ctx, &storage.Entry{FeedID: feedID, UserID: 1, Hash: "h", Title: "A"})
	require.NoError(t, err)
	_, err = s.InsertEntry(ctx, &storage.Entry{FeedID: feedID, UserID: 1, Hash: "h", Title: "B"})
	require.Error(t, err, "UNIQUE(feed_id, hash) must reject")
}

func TestEntries_ListByStatus(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID := mustFeed(t, s)
	for i, hash := range []string{"a", "b", "c"} {
		_, err := s.InsertEntry(ctx, &storage.Entry{
			FeedID: feedID, UserID: 1, Hash: hash, Title: hash,
			PublishedAt: int64Ptr(int64(1700000000 + i*60)),
		})
		require.NoError(t, err)
	}

	all, err := s.ListEntries(ctx, 1, storage.EntriesFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, all, 3)
	// Default sort: published_at DESC.
	require.Equal(t, "c", all[0].Title)

	unread, err := s.ListEntries(ctx, 1, storage.EntriesFilter{Status: "unread", Limit: 10})
	require.NoError(t, err)
	require.Len(t, unread, 3)
}

func TestEntries_UpdateStateMarksRead(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID := mustFeed(t, s)
	id, err := s.InsertEntry(ctx, &storage.Entry{FeedID: feedID, UserID: 1, Hash: "h", Title: "T"})
	require.NoError(t, err)

	read := true
	require.NoError(t, s.UpdateEntryState(ctx, 1, id, &read, nil))
	got, err := s.GetEntry(ctx, 1, id)
	require.NoError(t, err)
	require.True(t, got.Read)
	require.NotNil(t, got.ReadAt)

	saved := true
	require.NoError(t, s.UpdateEntryState(ctx, 1, id, nil, &saved))
	got, err = s.GetEntry(ctx, 1, id)
	require.NoError(t, err)
	require.True(t, got.Read, "earlier read state preserved")
	require.True(t, got.Saved)
	require.NotNil(t, got.SavedAt)
}

func TestEntries_UpdateStateIdempotentPreservesTimestamp(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID := mustFeed(t, s)
	id, err := s.InsertEntry(ctx, &storage.Entry{FeedID: feedID, UserID: 1, Hash: "h", Title: "T"})
	require.NoError(t, err)

	read := true
	require.NoError(t, s.UpdateEntryState(ctx, 1, id, &read, nil))
	first, err := s.GetEntry(ctx, 1, id)
	require.NoError(t, err)
	require.NotNil(t, first.ReadAt)
	originalReadAt := *first.ReadAt

	// Sleep past unixepoch() granularity so a mistaken overwrite would be visible.
	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, s.UpdateEntryState(ctx, 1, id, &read, nil))
	again, err := s.GetEntry(ctx, 1, id)
	require.NoError(t, err)
	require.NotNil(t, again.ReadAt)
	require.Equal(t, originalReadAt, *again.ReadAt, "re-marking read must not bump read_at")

	// Toggle to false, then back to true; read_at must update on the new 0->1.
	unread := false
	require.NoError(t, s.UpdateEntryState(ctx, 1, id, &unread, nil))
	cleared, err := s.GetEntry(ctx, 1, id)
	require.NoError(t, err)
	require.Nil(t, cleared.ReadAt)

	require.NoError(t, s.UpdateEntryState(ctx, 1, id, &read, nil))
	relit, err := s.GetEntry(ctx, 1, id)
	require.NoError(t, err)
	require.NotNil(t, relit.ReadAt)
	require.GreaterOrEqual(t, *relit.ReadAt, originalReadAt)
}

func TestEntries_BulkMarkReadFeed(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID := mustFeed(t, s)
	for _, h := range []string{"a", "b", "c"} {
		_, err := s.InsertEntry(ctx, &storage.Entry{FeedID: feedID, UserID: 1, Hash: h, Title: h})
		require.NoError(t, err)
	}

	require.NoError(t, s.BulkMarkRead(ctx, 1, storage.BulkScope{FeedID: int64Ptr(feedID)}))

	unread, err := s.ListEntries(ctx, 1, storage.EntriesFilter{Status: "unread", Limit: 10})
	require.NoError(t, err)
	require.Len(t, unread, 0)
}

func TestEntries_EntryExists(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID := mustFeed(t, s)

	exists, err := s.EntryExists(ctx, feedID, "missing")
	require.NoError(t, err)
	require.False(t, exists)

	_, err = s.InsertEntry(ctx, &storage.Entry{
		FeedID: feedID, UserID: 1, Hash: "h", Title: "T",
	})
	require.NoError(t, err)

	exists, err = s.EntryExists(ctx, feedID, "h")
	require.NoError(t, err)
	require.True(t, exists)
}

func strPtr(s string) *string { return &s }
func int64Ptr(i int64) *int64 { return &i }
