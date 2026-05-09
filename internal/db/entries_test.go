package db

import (
	"context"
	"database/sql"
	"testing"

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
		NowUnix: 100, NextPollAt: 200, NewEntries: []NewEntry{
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
		NowUnix: 1000, NextPollAt: 2000, NewEntries: []NewEntry{
			{Hash: "h1", Title: "newer-low-id", URL: "https://e.com/1", Content: "x", PublishedAt: 200},
		},
	})
	require.NoError(t, err)
	_, err = UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix: 1000, NextPollAt: 2000, NewEntries: []NewEntry{
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
