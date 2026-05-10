package db

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/cadence"
	"github.com/stretchr/testify/require"
)

func seedSub(t *testing.T, d *sql.DB) int64 {
	t.Helper()
	id, err := InsertSubscription(context.Background(), d, NewSubscription{
		Title: "x", FeedURL: "https://example.com/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	return id
}

func TestEntry_ListWithFilters(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID := seedSub(t, d)

	_, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix: 100, Floor: 15 * time.Minute, Ceiling: 24 * time.Hour, NewEntries: []NewEntry{
			{Hash: "h1", Title: "A", URL: "https://e.com/a", Content: "<p>a</p>", PublishedAt: 50},
			{Hash: "h2", Title: "B", URL: "https://e.com/b", Content: "<p>b</p>", PublishedAt: 60},
		},
	})
	require.NoError(t, err)

	got, _, _, err := ListEntries(ctx, d, ListEntriesParams{Limit: 100})
	require.NoError(t, err)
	require.Len(t, got, 2)

	// PATCH first entry as read.
	require.NoError(t, UpdateEntry(ctx, d, got[0].ID, EntryUpdate{Read: ptrBool(true)}))

	unread, _, _, err := ListEntries(ctx, d, ListEntriesParams{UnreadOnly: true, Limit: 100})
	require.NoError(t, err)
	require.Len(t, unread, 1)
}

func TestEntry_CompositeCursorPaginationDoesNotDropEntries(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID := seedSub(t, d)

	// Two entries where ID order disagrees with published_at order.
	// (pub=200, id=lowest) and (pub=100, id=highest). A bare `id < cursor` would
	// drop the second one.
	_, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix: 1000, Floor: 15 * time.Minute, Ceiling: 24 * time.Hour, NewEntries: []NewEntry{
			{Hash: "h1", Title: "newer-low-id", URL: "https://e.com/1", Content: "x", PublishedAt: 200},
		},
	})
	require.NoError(t, err)
	_, err = UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix: 1000, Floor: 15 * time.Minute, Ceiling: 24 * time.Hour, NewEntries: []NewEntry{
			{Hash: "h2", Title: "older-high-id", URL: "https://e.com/2", Content: "x", PublishedAt: 100},
		},
	})
	require.NoError(t, err)

	page1, nextPub, nextID, err := ListEntries(ctx, d, ListEntriesParams{Limit: 1})
	require.NoError(t, err)
	require.Len(t, page1, 1)
	require.Equal(t, "newer-low-id", page1[0].Title)
	require.Greater(t, nextPub, int64(0), "should have a next cursor")

	page2, _, _, err := ListEntries(ctx, d, ListEntriesParams{
		Limit:             1,
		CursorPublishedAt: nextPub,
		CursorID:          nextID,
	})
	require.NoError(t, err)
	require.Len(t, page2, 1, "older entry must NOT be silently dropped")
	require.Equal(t, "older-high-id", page2[0].Title)
}

func ptrBool(b bool) *bool { return &b }

func TestUpdateAfterPoll_ComputesVelocityAndNextPoll(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "Daily", FeedURL: "http://d/", NextPoll: 0, Created: now.Unix(),
	})
	require.NoError(t, err)

	var newEntries []NewEntry
	for i := 0; i < 14; i++ {
		newEntries = append(newEntries, NewEntry{
			Hash:        fmt.Sprintf("h%d", i),
			Title:       fmt.Sprintf("e%d", i),
			URL:         "http://x",
			Content:     "",
			PublishedAt: now.Add(-time.Duration(i) * 12 * time.Hour).Unix(),
		})
	}

	inserted, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix:     now.Unix(),
		NewEntries:  newEntries,
		Floor:       15 * time.Minute,
		Ceiling:     24 * time.Hour,
		RetryAfter:  time.Time{},
		CacheMaxAge: 0,
	})
	require.NoError(t, err)
	require.Equal(t, 14, inserted, "all entries inserted")

	var velocity int
	var nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx, `
		SELECT velocity_24h_x100, next_poll_at FROM subscriptions WHERE id = ?
	`, subID).Scan(&velocity, &nextPoll))
	require.Equal(t, 200, velocity, "14 entries / 7 days * 100")
	expected := now.Add(cadence.IntervalFromVelocity(200, 15*time.Minute, 24*time.Hour)).Unix()
	require.Equal(t, expected, nextPoll, "next_poll = now + 12h")
}

func TestUpdateAfterPoll_RetryAfterPushesNextPoll(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	retryAfter := now.Add(6 * time.Hour)
	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "Active", FeedURL: "http://a/", NextPoll: 0, Created: now.Unix(),
	})
	require.NoError(t, err)

	var newEntries []NewEntry
	for i := 0; i < 1400; i++ {
		newEntries = append(newEntries, NewEntry{
			Hash:        fmt.Sprintf("h%d", i),
			Title:       "e",
			URL:         "http://x",
			PublishedAt: now.Add(-time.Duration(i) * time.Minute).Unix(),
		})
	}
	_, err = UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix:    now.Unix(),
		NewEntries: newEntries,
		Floor:      15 * time.Minute,
		Ceiling:    24 * time.Hour,
		RetryAfter: retryAfter,
	})
	require.NoError(t, err)

	var nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).Scan(&nextPoll))
	require.Equal(t, retryAfter.Unix(), nextPoll, "retry-after wins over 15m floor")
}

func TestUpdateAfterPoll_WritesExtractFailed(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	_, err = UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix: 1_700_000_000,
		Floor:   15 * time.Minute,
		Ceiling: 24 * time.Hour,
		NewEntries: []NewEntry{
			{Hash: "ok", Title: "ok", URL: "https://x/1", Content: "<p>ok</p>", PublishedAt: 1_700_000_000, ExtractFailed: false},
			{Hash: "bad", Title: "bad", URL: "https://x/2", Content: "<p>summary</p>", PublishedAt: 1_700_000_000, ExtractFailed: true},
		},
	})
	require.NoError(t, err)

	rows, err := d.QueryContext(ctx, `SELECT hash, extract_failed FROM entries WHERE subscription_id = ? ORDER BY hash`, subID)
	require.NoError(t, err)
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var h string
		var ef int
		require.NoError(t, rows.Scan(&h, &ef))
		got[h] = ef
	}
	require.Equal(t, map[string]int{"bad": 1, "ok": 0}, got)
}
