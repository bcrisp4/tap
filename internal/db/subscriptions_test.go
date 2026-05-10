package db

import (
	"context"
	"database/sql"
	"fmt"
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

func TestListDuePolls_IncludesErrorCount(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "T", FeedURL: "http://x/", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	require.NoError(t, UpdateAfterError(ctx, d, subID, "transient", 0))
	require.NoError(t, UpdateAfterError(ctx, d, subID, "transient again", 0))

	due, err := ListDuePolls(ctx, d, time.Now().Unix(), 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, 2, due[0].ErrorCount)
}

func TestQueryVelocity_RollingWindow(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "Test", FeedURL: "http://x/feed", NextPoll: 0, Created: now.Unix(),
	})
	require.NoError(t, err)

	// 14 entries within last 7 days, 1 entry from 8 days ago (out of window).
	for i := 0; i < 14; i++ {
		_, err := d.ExecContext(ctx, `
			INSERT INTO entries (subscription_id, hash, title, url, content, published_at, fetched_at)
			VALUES (?, ?, '', '', '', ?, ?)
		`, subID, fmt.Sprintf("h%d", i), now.Add(-time.Duration(i)*time.Hour).Unix(), now.Unix())
		require.NoError(t, err)
	}
	_, err = d.ExecContext(ctx, `
		INSERT INTO entries (subscription_id, hash, title, url, content, published_at, fetched_at)
		VALUES (?, 'old', '', '', '', ?, ?)
	`, subID, now.Add(-8*24*time.Hour).Unix(), now.Unix())
	require.NoError(t, err)

	velocity, err := QueryVelocity(ctx, d, subID, now)
	require.NoError(t, err)
	require.Equal(t, 200, velocity, "14 entries / 7 days * 100")
}

func TestQueryVelocity_NoEntries(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "Empty", FeedURL: "http://e/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	v, err := QueryVelocity(ctx, d, subID, time.Now())
	require.NoError(t, err)
	require.Equal(t, 0, v, "velocity for empty feed")
}

func TestListDuePolls_ReturnsExtractionFields(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	_, err = d.ExecContext(ctx,
		`UPDATE subscriptions SET extract = 1, extract_selector = '.body' WHERE id = ?`, id)
	require.NoError(t, err)

	due, err := ListDuePolls(ctx, d, 0, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.True(t, due[0].Extract, "Extract should round-trip true")
	require.Equal(t, ".body", due[0].ExtractSelector, "ExtractSelector should round-trip")
}

func TestUpdateSubscriptionExtraction_HappyPath(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	require.NoError(t, UpdateSubscriptionExtraction(ctx, d, id, true, ".article"))

	due, err := ListDuePolls(ctx, d, 0, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.True(t, due[0].Extract)
	require.Equal(t, ".article", due[0].ExtractSelector)

	// Clearing the selector with empty string works.
	require.NoError(t, UpdateSubscriptionExtraction(ctx, d, id, true, ""))
	due, err = ListDuePolls(ctx, d, 0, 10)
	require.NoError(t, err)
	require.Equal(t, "", due[0].ExtractSelector)
}

func TestUpdateSubscriptionExtraction_MissingIDReturnsErrNoRows(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	err := UpdateSubscriptionExtraction(ctx, d, 999, true, "")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdateAfterNotModified_WritesVelocity(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "T", FeedURL: "http://x/", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	require.NoError(t, UpdateAfterNotModified(ctx, d, subID, 1000, 2000, 350))

	var velocity int
	var lastPoll, nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT velocity_24h_x100, last_poll_at, next_poll_at FROM subscriptions WHERE id = ?`,
		subID).Scan(&velocity, &lastPoll, &nextPoll))
	require.Equal(t, 350, velocity, "velocity_24h_x100")
	require.Equal(t, int64(1000), lastPoll, "last_poll_at")
	require.Equal(t, int64(2000), nextPoll, "next_poll_at")
}
