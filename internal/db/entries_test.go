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
	ctx := context.Background()
	d, _ := Open(ctx, ":memory:")
	defer d.Close()
	_ = Migrate(ctx, d)

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	subID, _ := InsertSubscription(ctx, d, NewSubscription{
		Title: "Daily", FeedURL: "http://d/", NextPoll: 0, Created: now.Unix(),
	})

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
	if err != nil {
		t.Fatal(err)
	}
	if inserted != 14 {
		t.Errorf("inserted = %d; want 14", inserted)
	}

	var velocity int
	var nextPoll int64
	_ = d.QueryRowContext(ctx, `
        SELECT velocity_24h_x100, next_poll_at FROM subscriptions WHERE id = ?
    `, subID).Scan(&velocity, &nextPoll)
	if velocity != 200 {
		t.Errorf("velocity = %d; want 200", velocity)
	}
	expected := now.Add(cadence.IntervalFromVelocity(200, 15*time.Minute, 24*time.Hour)).Unix()
	if nextPoll != expected {
		t.Errorf("next_poll = %d; want %d (12h after now)", nextPoll, expected)
	}
}

func TestUpdateAfterPoll_RetryAfterPushesNextPoll(t *testing.T) {
	ctx := context.Background()
	d, _ := Open(ctx, ":memory:")
	defer d.Close()
	_ = Migrate(ctx, d)

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	retryAfter := now.Add(6 * time.Hour)
	subID, _ := InsertSubscription(ctx, d, NewSubscription{
		Title: "Active", FeedURL: "http://a/", NextPoll: 0, Created: now.Unix(),
	})

	var newEntries []NewEntry
	for i := 0; i < 1400; i++ {
		newEntries = append(newEntries, NewEntry{
			Hash:        fmt.Sprintf("h%d", i),
			Title:       "e",
			URL:         "http://x",
			PublishedAt: now.Add(-time.Duration(i) * time.Minute).Unix(),
		})
	}
	_, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix:    now.Unix(),
		NewEntries: newEntries,
		Floor:      15 * time.Minute,
		Ceiling:    24 * time.Hour,
		RetryAfter: retryAfter,
	})
	if err != nil {
		t.Fatal(err)
	}
	var nextPoll int64
	_ = d.QueryRowContext(ctx, `SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).Scan(&nextPoll)
	if nextPoll != retryAfter.Unix() {
		t.Errorf("next_poll = %d; want %d (retry-after wins over 15m floor)", nextPoll, retryAfter.Unix())
	}
}
