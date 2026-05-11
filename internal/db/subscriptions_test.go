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

// newTestUserAndDB creates a DB with a single admin user and returns both.
func newTestUserAndDB(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	d := newTestDB(t)
	uid := insertTestUser(t, d, "testuser")
	return d, uid
}

func TestSubscription_InsertAndGet(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	now := time.Now().Unix()
	id, err := InsertSubscription(ctx, d, NewSubscription{
		UserID:   uid,
		Title:    "Example",
		FeedURL:  "https://example.com/feed",
		SiteURL:  "https://example.com",
		NextPoll: 0,
		Created:  now,
	})
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	got, err := GetSubscription(ctx, d, id, uid)
	require.NoError(t, err)
	require.Equal(t, "Example", got.Title)
	require.Equal(t, "https://example.com/feed", got.FeedURL)
	require.Equal(t, int64(0), got.NextPollAt)
}

func TestSubscription_Insert_RejectsDuplicateURL(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	_, err := InsertSubscription(ctx, d, NewSubscription{UserID: uid, Title: "a", FeedURL: "https://example.com/x", NextPoll: 0, Created: 0})
	require.NoError(t, err)
	_, err = InsertSubscription(ctx, d, NewSubscription{UserID: uid, Title: "b", FeedURL: "https://example.com/x", NextPoll: 0, Created: 0})
	require.ErrorIs(t, err, ErrSubscriptionExists)
}

func TestSubscription_TwoUsersSameURL(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid1 := insertTestUser(t, d, "alice")
	uid2 := insertTestUser(t, d, "bob")

	_, err := InsertSubscription(ctx, d, NewSubscription{UserID: uid1, Title: "x", FeedURL: "https://shared.example/feed", NextPoll: 0, Created: 0})
	require.NoError(t, err, "first user should be able to subscribe")
	_, err = InsertSubscription(ctx, d, NewSubscription{UserID: uid2, Title: "x", FeedURL: "https://shared.example/feed", NextPoll: 0, Created: 0})
	require.NoError(t, err, "second user must also be able to subscribe to the same URL")
}

func TestSubscription_GetIsolation(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid1 := insertTestUser(t, d, "alice")
	uid2 := insertTestUser(t, d, "bob")

	id, err := InsertSubscription(ctx, d, NewSubscription{UserID: uid1, Title: "a", FeedURL: "https://a.example/feed", NextPoll: 0, Created: 0})
	require.NoError(t, err)

	// userID 2 cannot see userID 1's subscription
	_, err = GetSubscription(ctx, d, id, uid2)
	require.Error(t, err)
}

func TestSubscription_ListIsolation(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid1 := insertTestUser(t, d, "alice")
	uid2 := insertTestUser(t, d, "bob")

	_, err := InsertSubscription(ctx, d, NewSubscription{UserID: uid1, Title: "a", FeedURL: "https://a.example/feed", NextPoll: 0, Created: 0})
	require.NoError(t, err)

	subs, err := ListSubscriptions(ctx, d, uid1)
	require.NoError(t, err)
	require.Len(t, subs, 1)

	subs2, err := ListSubscriptions(ctx, d, uid2)
	require.NoError(t, err)
	require.Empty(t, subs2, "user 2 should see no subscriptions")
}

func TestSubscription_DeleteIsolation(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid1 := insertTestUser(t, d, "alice")
	uid2 := insertTestUser(t, d, "bob")

	id, err := InsertSubscription(ctx, d, NewSubscription{UserID: uid1, Title: "a", FeedURL: "https://a.example/feed", NextPoll: 0, Created: 0})
	require.NoError(t, err)

	err = DeleteSubscription(ctx, d, id, uid2)
	require.ErrorIs(t, err, sql.ErrNoRows, "user2 cannot delete user1's subscription")

	err = DeleteSubscription(ctx, d, id, uid1)
	require.NoError(t, err)
}

func TestSubscription_ListDuePolls(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	due, _ := InsertSubscription(ctx, d, NewSubscription{UserID: uid, Title: "due", FeedURL: "https://example.com/a", NextPoll: 0, Created: 0})
	_, _ = InsertSubscription(ctx, d, NewSubscription{UserID: uid, Title: "future", FeedURL: "https://example.com/b", NextPoll: time.Now().Unix() + 86400, Created: 0})

	rows, err := ListDuePolls(ctx, d, time.Now().Unix(), 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, due, rows[0].ID)
}

func TestListDuePolls_IncludesErrorCount(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "T", FeedURL: "http://x/", NextPoll: 0, Created: 0,
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
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "Test", FeedURL: "http://x/feed", NextPoll: 0, Created: now.Unix(),
	})
	require.NoError(t, err)

	// 14 entries within last 7 days, 1 entry from 8 days ago (out of window).
	for i := 0; i < 14; i++ {
		_, err := d.ExecContext(ctx, `
			INSERT INTO entries (user_id, subscription_id, hash, title, url, content, published_at, fetched_at)
			VALUES (?, ?, ?, '', '', '', ?, ?)
		`, uid, subID, fmt.Sprintf("h%d", i), now.Add(-time.Duration(i)*time.Hour).Unix(), now.Unix())
		require.NoError(t, err)
	}
	_, err = d.ExecContext(ctx, `
		INSERT INTO entries (user_id, subscription_id, hash, title, url, content, published_at, fetched_at)
		VALUES (?, ?, 'old', '', '', '', ?, ?)
	`, uid, subID, now.Add(-8*24*time.Hour).Unix(), now.Unix())
	require.NoError(t, err)

	velocity, err := QueryVelocity(ctx, d, subID, now)
	require.NoError(t, err)
	require.Equal(t, 200, velocity, "14 entries / 7 days * 100")
}

func TestQueryVelocity_NoEntries(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "Empty", FeedURL: "http://e/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	v, err := QueryVelocity(ctx, d, subID, time.Now())
	require.NoError(t, err)
	require.Equal(t, 0, v, "velocity for empty feed")
}

func TestListDuePolls_ReturnsExtractionFields(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	id, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
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

func TestUpdateSubscriptionPatch_AllFieldsAtomic(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	id, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	// Happy path: writes all five columns in one statement.
	require.NoError(t, UpdateSubscriptionPatch(ctx, d, id, uid, true, ".body", "c", "u", "p"))
	s, err := GetSubscription(ctx, d, id, uid)
	require.NoError(t, err)
	require.True(t, s.Extract)
	require.Equal(t, ".body", s.ExtractSelector)
	require.Equal(t, "c", s.Cookie)
	require.Equal(t, "u", s.BasicAuthUser)
	require.Equal(t, "p", s.BasicAuthPass)

	// Empty strings clear the selector and credentials atomically.
	require.NoError(t, UpdateSubscriptionPatch(ctx, d, id, uid, false, "", "", "", ""))
	s, err = GetSubscription(ctx, d, id, uid)
	require.NoError(t, err)
	require.False(t, s.Extract)
	require.Empty(t, s.ExtractSelector)
	require.Empty(t, s.Cookie)
	require.Empty(t, s.BasicAuthUser)
	require.Empty(t, s.BasicAuthPass)

	// Missing id → sql.ErrNoRows.
	err = UpdateSubscriptionPatch(ctx, d, id+999, uid, false, "", "", "", "")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdateAfterNotModified_WritesVelocity(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "T", FeedURL: "http://x/", NextPoll: 0, Created: 0,
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

func TestInsertAndGetSubscriptionWithCreds(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	id, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
		Cookie: "session=abc", BasicAuthUser: "ben", BasicAuthPass: "secret",
	})
	require.NoError(t, err)

	s, err := GetSubscription(ctx, d, id, uid)
	require.NoError(t, err)
	require.Equal(t, "session=abc", s.Cookie)
	require.Equal(t, "ben", s.BasicAuthUser)
	require.Equal(t, "secret", s.BasicAuthPass)
}

func TestUpdateAfterPoll_OverwritesURLDefaultTitle(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := insertTestUser(t, d, "u-overwrite")
	ctx := context.Background()
	id, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "https://example.test/feed.xml",
		FeedURL: "https://example.test/feed.xml",
		NextPoll: 0, Created: 1,
	})
	require.NoError(t, err)

	_, err = UpdateAfterPoll(ctx, d, id, PollResult{
		UserID: uid, NowUnix: 100, FeedTitle: "Example News",
		Floor: 15 * time.Minute, Ceiling: 24 * time.Hour,
	})
	require.NoError(t, err)

	s, err := GetSubscription(ctx, d, id, uid)
	require.NoError(t, err)
	require.Equal(t, "Example News", s.Title)
}

func TestUpdateAfterPoll_PreservesUserSetTitle(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := insertTestUser(t, d, "u-preserve")
	ctx := context.Background()
	id, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "My Custom Name",
		FeedURL: "https://example.test/feed.xml",
		NextPoll: 0, Created: 1,
	})
	require.NoError(t, err)

	_, err = UpdateAfterPoll(ctx, d, id, PollResult{
		UserID: uid, NowUnix: 100, FeedTitle: "Example News",
		Floor: 15 * time.Minute, Ceiling: 24 * time.Hour,
	})
	require.NoError(t, err)

	s, err := GetSubscription(ctx, d, id, uid)
	require.NoError(t, err)
	require.Equal(t, "My Custom Name", s.Title)
}

func TestMarkSubscriptionRead_ScopedToUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "msr-alice")
	u2 := insertTestUser(t, d, "msr-bob")
	// u1: subscription + unread entry.
	sub1, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: u1, Title: "F1", FeedURL: "https://u1/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h1', 'E1', '', 'https://u1/1', '<p>x</p>', ?, ?, 0, 0, ?)`,
		sub1, time.Now().Unix(), time.Now().Unix(), u1)
	require.NoError(t, err)
	// u2: subscription + unread entry.
	sub2, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: u2, Title: "F2", FeedURL: "https://u2/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h2', 'E2', '', 'https://u2/1', '<p>x</p>', ?, ?, 0, 0, ?)`,
		sub2, time.Now().Unix(), time.Now().Unix(), u2)
	require.NoError(t, err)

	require.NoError(t, MarkSubscriptionRead(context.Background(), d, sub1, u1))

	var u1Read, u2Read int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT read FROM entries WHERE subscription_id = ?`, sub1).Scan(&u1Read))
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT read FROM entries WHERE subscription_id = ?`, sub2).Scan(&u2Read))
	require.Equal(t, 1, u1Read)
	require.Equal(t, 0, u2Read)
}

func TestMarkSubscriptionRead_WrongUser_NoOp(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "msr-u1")
	u2 := insertTestUser(t, d, "msr-u2")
	sub, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: u1, Title: "F", FeedURL: "https://u1/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h', 'E', '', 'https://u1/1', '<p>x</p>', ?, ?, 0, 0, ?)`,
		sub, time.Now().Unix(), time.Now().Unix(), u1)
	require.NoError(t, err)

	// u2 cannot mark u1's subscription read — must be a silent no-op.
	require.NoError(t, MarkSubscriptionRead(context.Background(), d, sub, u2))

	var n int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT read FROM entries WHERE subscription_id = ?`, sub).Scan(&n))
	require.Equal(t, 0, n, "u1's entry must remain unread when u2 tried to mark it read")
}

func TestListDuePollsCarriesCreds(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	_, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
		Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})
	require.NoError(t, err)

	due, err := ListDuePolls(ctx, d, 0, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, "c", due[0].Cookie)
	require.Equal(t, "u", due[0].BasicAuthUser)
	require.Equal(t, "p", due[0].BasicAuthPass)
}
