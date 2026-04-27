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
