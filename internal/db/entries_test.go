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

// seedSub creates a user and subscription, returning (subID, userID).
func seedSub(t *testing.T, d *sql.DB) (subID, userID int64) {
	t.Helper()
	uid := insertTestUser(t, d, fmt.Sprintf("u%d", time.Now().UnixNano()))
	id, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://example.com/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	return id, uid
}

func TestEntry_ListWithFilters(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID, uid := seedSub(t, d)

	_, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		UserID: uid, NowUnix: 100, Floor: 15 * time.Minute, Ceiling: 24 * time.Hour, NewEntries: []NewEntry{
			{Hash: "h1", Title: "A", URL: "https://e.com/a", Content: "<p>a</p>", PublishedAt: 50},
			{Hash: "h2", Title: "B", URL: "https://e.com/b", Content: "<p>b</p>", PublishedAt: 60},
		},
	})
	require.NoError(t, err)

	got, _, _, err := ListEntries(ctx, d, ListEntriesParams{UserID: uid, Limit: 100})
	require.NoError(t, err)
	require.Len(t, got, 2)

	// PATCH first entry as read.
	require.NoError(t, UpdateEntry(ctx, d, got[0].ID, uid, EntryUpdate{Read: ptrBool(true)}))

	unread, _, _, err := ListEntries(ctx, d, ListEntriesParams{UserID: uid, UnreadOnly: true, Limit: 100})
	require.NoError(t, err)
	require.Len(t, unread, 1)
}

func TestEntry_ListIsolation(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID, uid1 := seedSub(t, d)
	_, uid2 := seedSub(t, d)

	_, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		UserID: uid1, NowUnix: 100, Floor: 15 * time.Minute, Ceiling: 24 * time.Hour, NewEntries: []NewEntry{
			{Hash: "h1", Title: "A", URL: "https://e.com/a", Content: "<p>a</p>", PublishedAt: 50},
		},
	})
	require.NoError(t, err)

	got, _, _, err := ListEntries(ctx, d, ListEntriesParams{UserID: uid1, Limit: 100})
	require.NoError(t, err)
	require.Len(t, got, 1, "user1 should see their entry")

	got2, _, _, err := ListEntries(ctx, d, ListEntriesParams{UserID: uid2, Limit: 100})
	require.NoError(t, err)
	require.Empty(t, got2, "user2 should see no entries")
}

func TestEntry_GetIsolation(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID, uid1 := seedSub(t, d)
	_, uid2 := seedSub(t, d)

	_, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		UserID: uid1, NowUnix: 100, Floor: 15 * time.Minute, Ceiling: 24 * time.Hour, NewEntries: []NewEntry{
			{Hash: "h1", Title: "A", URL: "https://e.com/a", Content: "<p>a</p>", PublishedAt: 50},
		},
	})
	require.NoError(t, err)

	entries, _, _, err := ListEntries(ctx, d, ListEntriesParams{UserID: uid1, Limit: 1})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	entryID := entries[0].ID

	_, err = GetEntry(ctx, d, entryID, uid2)
	require.Error(t, err, "user2 should not be able to get user1's entry")
}

func TestEntry_CompositeCursorPaginationDoesNotDropEntries(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID, uid := seedSub(t, d)

	// Two entries where ID order disagrees with published_at order.
	_, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		UserID: uid, NowUnix: 1000, Floor: 15 * time.Minute, Ceiling: 24 * time.Hour, NewEntries: []NewEntry{
			{Hash: "h1", Title: "newer-low-id", URL: "https://e.com/1", Content: "x", PublishedAt: 200},
		},
	})
	require.NoError(t, err)
	_, err = UpdateAfterPoll(ctx, d, subID, PollResult{
		UserID: uid, NowUnix: 1000, Floor: 15 * time.Minute, Ceiling: 24 * time.Hour, NewEntries: []NewEntry{
			{Hash: "h2", Title: "older-high-id", URL: "https://e.com/2", Content: "x", PublishedAt: 100},
		},
	})
	require.NoError(t, err)

	page1, nextPub, nextID, err := ListEntries(ctx, d, ListEntriesParams{UserID: uid, Limit: 1})
	require.NoError(t, err)
	require.Len(t, page1, 1)
	require.Equal(t, "newer-low-id", page1[0].Title)
	require.Greater(t, nextPub, int64(0), "should have a next cursor")

	page2, _, _, err := ListEntries(ctx, d, ListEntriesParams{
		UserID:            uid,
		Limit:             1,
		CursorPublishedAt: nextPub,
		CursorID:          nextID,
	})
	require.NoError(t, err)
	require.Len(t, page2, 1, "older entry must NOT be silently dropped")
	require.Equal(t, "older-high-id", page2[0].Title)
}

func ptrBool(b bool) *bool { return &b }

func TestListEntries_SavedOnly(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID, uid := seedSub(t, d)

	// Insert two entries via direct SQL so we can control saved state.
	var e1ID int64
	res, err := d.ExecContext(ctx, `
		INSERT INTO entries (user_id, subscription_id, hash, title, url, content, published_at, fetched_at)
		VALUES (?, ?, 'h-saved', 'saved one', 'https://x/1', 'c', 100, 0)
	`, uid, subID)
	require.NoError(t, err)
	e1ID, err = res.LastInsertId()
	require.NoError(t, err)

	_, err = d.ExecContext(ctx, `
		INSERT INTO entries (user_id, subscription_id, hash, title, url, content, published_at, fetched_at)
		VALUES (?, ?, 'h-unsaved', 'unsaved', 'https://x/2', 'c', 90, 0)
	`, uid, subID)
	require.NoError(t, err)

	// Mark first entry saved.
	require.NoError(t, UpdateEntry(ctx, d, e1ID, uid, EntryUpdate{Saved: ptrBool(true)}))

	out, _, _, err := ListEntries(ctx, d, ListEntriesParams{
		UserID: uid, SavedOnly: true, Limit: 50,
	})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "saved one", out[0].Title)
}

func TestUpdateAfterPoll_ComputesVelocityAndNextPoll(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "Daily", FeedURL: "http://d/", NextPoll: 0, Created: now.Unix(),
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
		UserID:      uid,
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
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	retryAfter := now.Add(6 * time.Hour)
	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "Active", FeedURL: "http://a/", NextPoll: 0, Created: now.Unix(),
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
		UserID:     uid,
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
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	_, err = UpdateAfterPoll(ctx, d, subID, PollResult{
		UserID:  uid,
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

func TestListArchivable_EligibleCriteria(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID, uid := seedSub(t, d)

	insertE := func(hash string, publishedAt int64, read, saved bool) {
		t.Helper()
		_, err := d.ExecContext(ctx, `
			INSERT INTO entries (user_id, subscription_id, hash, title, author, url, content,
			                     published_at, fetched_at, read, saved, extract_failed)
			VALUES (?, ?, ?, 'T', NULL, 'https://x', 'c', ?, 0, ?, ?, 0)
		`, uid, subID, hash, publishedAt, boolToInt(read), boolToInt(saved))
		require.NoError(t, err)
	}

	horizon := int64(1000)
	insertE("old-read-unsaved", 500, true, false)  // eligible
	insertE("old-read-saved", 500, true, true)     // saved — retain
	insertE("old-unread", 500, false, false)       // unread — retain
	insertE("new-read-unsaved", 2000, true, false) // too new — retain

	rows, err := ListArchivable(ctx, d, horizon, 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "old-read-unsaved", rows[0].Hash)
	require.Equal(t, subID, rows[0].SubscriptionID)
}

func TestListArchivable_RespectsLimit(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID, uid := seedSub(t, d)

	for i := range 5 {
		_, err := d.ExecContext(ctx, `
			INSERT INTO entries (user_id, subscription_id, hash, title, author, url, content,
			                     published_at, fetched_at, read, saved, extract_failed)
			VALUES (?, ?, ?, 'T', NULL, 'https://x', 'c', 100, 0, 1, 0, 0)
		`, uid, subID, fmt.Sprintf("h%d", i))
		require.NoError(t, err)
	}

	rows, err := ListArchivable(ctx, d, 500, 3)
	require.NoError(t, err)
	require.Len(t, rows, 3)
}
