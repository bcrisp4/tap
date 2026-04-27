package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func newFeed(title, url string) *storage.Feed {
	return &storage.Feed{
		UserID:       1,
		Title:        title,
		FeedURL:      url,
		PollInterval: 3600,
	}
}

func TestFeeds_CreateGetListUpdateDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id, err := s.CreateFeed(ctx, newFeed("Julia Evans", "https://jvns.ca/atom.xml"))
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	got, err := s.GetFeed(ctx, 1, id)
	require.NoError(t, err)
	require.Equal(t, "Julia Evans", got.Title)
	require.Equal(t, "https://jvns.ca/atom.xml", got.FeedURL)
	require.Equal(t, int64(3600), got.PollInterval)

	list, err := s.ListFeeds(ctx, 1)
	require.NoError(t, err)
	require.Len(t, list, 1)

	got.Title = "jvns.ca"
	got.Crawler = true
	require.NoError(t, s.UpdateFeed(ctx, got))
	got2, err := s.GetFeed(ctx, 1, id)
	require.NoError(t, err)
	require.Equal(t, "jvns.ca", got2.Title)
	require.True(t, got2.Crawler)

	require.NoError(t, s.DeleteFeed(ctx, 1, id))
	_, err = s.GetFeed(ctx, 1, id)
	require.True(t, errors.Is(err, storage.ErrNotFound))
}

func TestFeeds_DuplicateURLPerUserErrors(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, err := s.CreateFeed(ctx, newFeed("A", "https://example.com/feed"))
	require.NoError(t, err)
	_, err = s.CreateFeed(ctx, newFeed("B", "https://example.com/feed"))
	require.Error(t, err)
}

func TestFeeds_ListDueRespectsExclusions(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Now().Unix()

	// Three feeds, all due.
	ids := make([]int64, 3)
	for i := 0; i < 3; i++ {
		f := newFeed("F", "https://example.com/feed"+string(rune('0'+i)))
		next := now - 60
		f.NextPollAt = &next
		id, err := s.CreateFeed(ctx, f)
		require.NoError(t, err)
		ids[i] = id
	}

	// Without exclusions: all three returned.
	due, err := s.ListDueFeeds(ctx, now, 10, nil)
	require.NoError(t, err)
	require.Len(t, due, 3)

	// Excluding one: two returned.
	due, err = s.ListDueFeeds(ctx, now, 10, []int64{ids[0]})
	require.NoError(t, err)
	require.Len(t, due, 2)
	for _, id := range due {
		require.NotEqual(t, ids[0], id)
	}
}

func TestFeeds_ListDueIgnoresDisabledOrBroken(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Now().Unix()

	due := func(disabled int, errCount int) int64 {
		f := newFeed("F", "https://example.com/"+string(rune('a'+disabled+errCount)))
		next := now - 60
		f.NextPollAt = &next
		f.Disabled = disabled == 1
		f.ErrorCount = errCount
		id, err := s.CreateFeed(ctx, f)
		require.NoError(t, err)
		return id
	}
	good := due(0, 0)
	_ = due(1, 0)  // disabled
	_ = due(0, 10) // too many errors

	got, err := s.ListDueFeeds(ctx, now, 10, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{good}, got)
}

func TestFeeds_CommitPollSuccess_InsertsAndUpdates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID, err := s.CreateFeed(ctx, &storage.Feed{
		UserID: 1, Title: "F", FeedURL: "https://f/", PollInterval: 3600,
	})
	require.NoError(t, err)

	entries := []*storage.Entry{
		{FeedID: feedID, UserID: 1, Hash: "a", Title: "A"},
		{FeedID: feedID, UserID: 1, Hash: "b", Title: "B"},
	}
	weekly, err := s.CommitPollSuccess(ctx, feedID, entries, `"v1"`, "Sat, 26 Apr 2026 08:00:00 GMT", 0, 999_999_000)
	require.NoError(t, err)
	require.Equal(t, 2, weekly)

	got, _ := s.GetFeed(ctx, 1, feedID)
	require.NotNil(t, got.ETag)
	require.Equal(t, `"v1"`, *got.ETag)
	require.NotNil(t, got.LastModified)
	require.Equal(t, 0, got.ErrorCount)
	require.Nil(t, got.LastError)
	require.Equal(t, int64(999_999_000), *got.NextPollAt)
	require.Equal(t, 2, got.WeeklyEntryCount)

	all, _ := s.ListEntries(ctx, 1, storage.EntriesFilter{Limit: 10})
	require.Len(t, all, 2)
}

func TestFeeds_CommitPollSuccess_DuplicateHashSilentlySkipped(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID, _ := s.CreateFeed(ctx, &storage.Feed{
		UserID: 1, Title: "F", FeedURL: "https://f/", PollInterval: 3600,
	})
	// First insert.
	_, err := s.CommitPollSuccess(ctx, feedID,
		[]*storage.Entry{{FeedID: feedID, UserID: 1, Hash: "h", Title: "T1"}},
		"", "", 0, 1)
	require.NoError(t, err)
	// Second commit with the same hash + an additional new one;
	// the duplicate must be silently ignored, not fail the whole tx.
	_, err = s.CommitPollSuccess(ctx, feedID,
		[]*storage.Entry{
			{FeedID: feedID, UserID: 1, Hash: "h", Title: "T1-dup"},
			{FeedID: feedID, UserID: 1, Hash: "i", Title: "T2"},
		}, "", "", 0, 2)
	require.NoError(t, err)

	all, _ := s.ListEntries(ctx, 1, storage.EntriesFilter{Limit: 10})
	require.Len(t, all, 2, "duplicate hash must not double-insert")
}

func TestFeeds_CommitPollNotModified(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID, _ := s.CreateFeed(ctx, &storage.Feed{
		UserID: 1, Title: "F", FeedURL: "https://f/", PollInterval: 3600,
		ErrorCount: 3, LastError: strPtr("earlier failure"),
	})
	require.NoError(t, s.CommitPollNotModified(ctx, feedID, `"v1"`, "", 1234567))
	got, _ := s.GetFeed(ctx, 1, feedID)
	require.Equal(t, 0, got.ErrorCount)
	require.Nil(t, got.LastError)
	require.Equal(t, int64(1234567), *got.NextPollAt)
	require.NotNil(t, got.ETag)
}

func TestFeeds_CommitPollFailure(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	feedID, _ := s.CreateFeed(ctx, &storage.Feed{
		UserID: 1, Title: "F", FeedURL: "https://f/", PollInterval: 3600,
	})
	require.NoError(t, s.CommitPollFailure(ctx, feedID, 1, "dial tcp: timeout", 5000))
	got, _ := s.GetFeed(ctx, 1, feedID)
	require.Equal(t, 1, got.ErrorCount)
	require.NotNil(t, got.LastError)
	require.Equal(t, "dial tcp: timeout", *got.LastError)
	require.Equal(t, int64(5000), *got.NextPollAt)
}
