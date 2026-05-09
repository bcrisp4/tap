package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, Migrate(context.Background(), d))
	return d
}

func TestSubscription_InsertAndGet(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	now := time.Now().Unix()
	id, err := InsertSubscription(ctx, d, NewSubscription{
		Title:    "Example",
		FeedURL:  "https://example.com/feed",
		SiteURL:  "https://example.com",
		NextPoll: 0,
		Created:  now,
	})
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	got, err := GetSubscription(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, "Example", got.Title)
	require.Equal(t, "https://example.com/feed", got.FeedURL)
	require.Equal(t, int64(0), got.NextPollAt)
}

func TestSubscription_Insert_RejectsDuplicateURL(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	_, err := InsertSubscription(ctx, d, NewSubscription{Title: "a", FeedURL: "https://example.com/x", NextPoll: 0, Created: 0})
	require.NoError(t, err)
	_, err = InsertSubscription(ctx, d, NewSubscription{Title: "b", FeedURL: "https://example.com/x", NextPoll: 0, Created: 0})
	require.Error(t, err)
}

func TestSubscription_ListDuePolls(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	due, _ := InsertSubscription(ctx, d, NewSubscription{Title: "due", FeedURL: "https://example.com/a", NextPoll: 0, Created: 0})
	_, _ = InsertSubscription(ctx, d, NewSubscription{Title: "future", FeedURL: "https://example.com/b", NextPoll: time.Now().Unix() + 86400, Created: 0})

	rows, err := ListDuePolls(ctx, d, time.Now().Unix(), 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, due, rows[0].ID)
}
