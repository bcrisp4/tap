package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/storage"
)

// newTestStore opens a fresh on-disk SQLite database in t.TempDir,
// applies migrations, and returns a *Store. Closes the DB at test
// teardown. Used by every test file in this package.
func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return storage.New(d)
}

func TestNew_HasUnderlyingDB(t *testing.T) {
	s := newTestStore(t)
	require.NotNil(t, s.DB())
	require.NoError(t, s.DB().Ping())
}

func TestStorage_CrossRepoSmoke(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Category + feed pinned to that category.
	catID, err := s.CreateCategory(ctx, 1, "Tech")
	require.NoError(t, err)

	feed := &storage.Feed{
		UserID: 1, CategoryID: &catID, Title: "Julia Evans",
		FeedURL: "https://jvns.ca/atom.xml", PollInterval: 3600,
	}
	feedID, err := s.CreateFeed(ctx, feed)
	require.NoError(t, err)

	// Two entries.
	for i, h := range []string{"h1", "h2"} {
		_, err := s.InsertEntry(ctx, &storage.Entry{
			FeedID: feedID, UserID: 1, Hash: h, Title: "Title " + h,
			PublishedAt: int64Ptr(int64(1700000000 + i*60)),
		})
		require.NoError(t, err)
	}

	// Mark one entry read; attach an enclosure to the other.
	all, err := s.ListEntries(ctx, 1, storage.EntriesFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, all, 2)
	read := true
	require.NoError(t, s.UpdateEntryState(ctx, 1, all[0].ID, &read, nil))
	_, err = s.InsertEnclosure(ctx, all[1].ID, "https://example.com/x.mp3", "audio/mpeg", 100)
	require.NoError(t, err)

	// Tombstone & config kv.
	require.NoError(t, s.InsertTombstone(ctx, feedID, "old-hash"))
	require.NoError(t, s.SetConfig(ctx, "proxy_hmac_secret", "deadbeef"))

	// Read everything back.
	unread, err := s.ListEntries(ctx, 1, storage.EntriesFilter{Status: "unread", Limit: 10})
	require.NoError(t, err)
	require.Len(t, unread, 1)

	encs, err := s.ListEnclosures(ctx, all[1].ID)
	require.NoError(t, err)
	require.Len(t, encs, 1)

	has, err := s.HasTombstone(ctx, feedID, "old-hash")
	require.NoError(t, err)
	require.True(t, has)

	secret, err := s.GetConfig(ctx, "proxy_hmac_secret")
	require.NoError(t, err)
	require.Equal(t, "deadbeef", secret)
}
