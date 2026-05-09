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

func TestQueryVelocity_RollingWindow(t *testing.T) {
	ctx := context.Background()
	d, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := Migrate(ctx, d); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "Test", FeedURL: "http://x/feed", NextPoll: 0, Created: now.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// 14 entries within last 7 days, 1 entry from 8 days ago (out of window).
	for i := 0; i < 14; i++ {
		if _, err := d.ExecContext(ctx, `
            INSERT INTO entries (subscription_id, hash, title, url, content, published_at, fetched_at)
            VALUES (?, ?, '', '', '', ?, ?)
        `, subID, fmt.Sprintf("h%d", i), now.Add(-time.Duration(i)*time.Hour).Unix(), now.Unix()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.ExecContext(ctx, `
        INSERT INTO entries (subscription_id, hash, title, url, content, published_at, fetched_at)
        VALUES (?, 'old', '', '', '', ?, ?)
    `, subID, now.Add(-8*24*time.Hour).Unix(), now.Unix()); err != nil {
		t.Fatal(err)
	}

	velocity, err := QueryVelocity(ctx, d, subID, now)
	if err != nil {
		t.Fatal(err)
	}
	if velocity != 200 {
		t.Errorf("velocity = %d; want 200", velocity)
	}
}

func TestQueryVelocity_NoEntries(t *testing.T) {
	ctx := context.Background()
	d, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	_ = Migrate(ctx, d)

	subID, _ := InsertSubscription(ctx, d, NewSubscription{
		Title: "Empty", FeedURL: "http://e/feed", NextPoll: 0, Created: 0,
	})
	v, err := QueryVelocity(ctx, d, subID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Errorf("velocity for empty feed = %d; want 0", v)
	}
}

func TestUpdateAfterNotModified_WritesVelocity(t *testing.T) {
	ctx := context.Background()
	d, _ := Open(ctx, ":memory:")
	defer d.Close()
	_ = Migrate(ctx, d)

	subID, _ := InsertSubscription(ctx, d, NewSubscription{
		Title: "T", FeedURL: "http://x/", NextPoll: 0, Created: 0,
	})

	if err := UpdateAfterNotModified(ctx, d, subID, 1000, 2000, 350); err != nil {
		t.Fatal(err)
	}

	var velocity int
	var lastPoll, nextPoll int64
	err := d.QueryRowContext(ctx,
		`SELECT velocity_24h_x100, last_poll_at, next_poll_at FROM subscriptions WHERE id = ?`,
		subID).Scan(&velocity, &lastPoll, &nextPoll)
	if err != nil {
		t.Fatal(err)
	}
	if velocity != 350 || lastPoll != 1000 || nextPoll != 2000 {
		t.Errorf("got velocity=%d last=%d next=%d; want 350,1000,2000",
			velocity, lastPoll, nextPoll)
	}
}
