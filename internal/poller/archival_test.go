package poller_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/poller"
	"github.com/bcrisp4/tap/internal/storage"
)

func newStoreForArchival(t *testing.T) *storage.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return storage.New(d)
}

func TestArchive_DeletesReadUnsavedOlderThanThreshold(t *testing.T) {
	ctx := context.Background()
	s := newStoreForArchival(t)

	feedID, err := s.CreateFeed(ctx, &storage.Feed{
		UserID: 1, Title: "F", FeedURL: "https://t/", PollInterval: 3600,
	})
	require.NoError(t, err)

	// Old, read, unsaved → archived.
	id1, _ := s.InsertEntry(ctx, &storage.Entry{FeedID: feedID, UserID: 1, Hash: "old", Title: "old"})
	_, err = s.DB().Exec(`UPDATE entries SET read=1, created_at=unixepoch()-90*86400 WHERE id=?`, id1)
	require.NoError(t, err)

	// Old, saved → kept.
	id2, _ := s.InsertEntry(ctx, &storage.Entry{FeedID: feedID, UserID: 1, Hash: "saved", Title: "saved"})
	_, err = s.DB().Exec(`UPDATE entries SET saved=1, read=1, created_at=unixepoch()-90*86400 WHERE id=?`, id2)
	require.NoError(t, err)

	// Recent, read, unsaved → kept.
	id3, _ := s.InsertEntry(ctx, &storage.Entry{FeedID: feedID, UserID: 1, Hash: "recent", Title: "recent"})
	_, err = s.DB().Exec(`UPDATE entries SET read=1 WHERE id=?`, id3)
	require.NoError(t, err)

	require.NoError(t, poller.ArchiveOnce(ctx, s, 60*24*time.Hour, nil))

	// id1 deleted, tombstone written.
	all, _ := s.ListEntries(ctx, 1, storage.EntriesFilter{Limit: 10})
	require.Len(t, all, 2)
	has, _ := s.HasTombstone(ctx, feedID, "old")
	require.True(t, has)
}
