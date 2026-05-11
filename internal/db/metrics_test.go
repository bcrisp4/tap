package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mustCreateUser creates a user and returns its ID.
func mustCreateUser(t *testing.T, d *sql.DB, username string) int64 {
	t.Helper()
	return insertTestUser(t, d, username)
}

// mustInsertSubscriptionFull inserts a subscription with explicit error_count,
// last_poll_at, and next_poll_at values.
func mustInsertSubscriptionFull(t *testing.T, d *sql.DB, userID int64, title, feedURL string, errorCount int, lastPollAt, nextPollAt int64) int64 {
	t.Helper()
	id, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID:  userID,
		Title:   title,
		FeedURL: feedURL,
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`UPDATE subscriptions SET error_count=?, last_poll_at=?, next_poll_at=? WHERE id=?`,
		errorCount, lastPollAt, nextPollAt, id,
	)
	require.NoError(t, err)
	return id
}

// mustInsertEntryFetched inserts an entry with explicit published_at and fetched_at values.
func mustInsertEntryFetched(t *testing.T, d *sql.DB, subID int64, hash, title string, publishedAt, fetchedAt int64) {
	t.Helper()
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO entries (user_id, subscription_id, hash, title, author, url, content, published_at, fetched_at, extract_failed)
		 SELECT s.user_id, ?, ?, ?, '', '', '', ?, ?, 0 FROM subscriptions s WHERE s.id = ?`,
		subID, hash, title, publishedAt, fetchedAt, subID,
	)
	require.NoError(t, err)
}

func TestGetAdminMetrics_EmptyDB(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	m, err := GetAdminMetrics(ctx, d, time.Unix(1_700_000_000, 0))
	require.NoError(t, err)
	require.Equal(t, 0, m.FeedsTotal)
	require.Equal(t, 0, m.FeedsOK)
	require.Equal(t, 0, m.EntriesTotal)
	require.Equal(t, 0, m.Entries24h)
	require.Equal(t, 0, m.FeedsWithErrors)
	require.Empty(t, m.OffendingFeeds)
}

func TestGetAdminMetrics_PopulatedDB(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, d, "alice")
	now := time.Unix(1_700_000_000, 0)

	// 3 feeds: 2 OK, 1 erroring
	feedOK1 := mustInsertSubscriptionFull(t, d, uid, "Feed OK 1", "https://a.example/feed", 0, now.Unix()-60, now.Unix()+60)
	feedOK2 := mustInsertSubscriptionFull(t, d, uid, "Feed OK 2", "https://b.example/feed", 0, now.Unix()-120, now.Unix()+120)
	mustInsertSubscriptionFull(t, d, uid, "Phoronix", "https://phoronix.com/feed", 4, now.Unix()-3600, now.Unix()+30)

	// 2 entries: 1 fetched within last 24h, 1 fetched 48h ago.
	mustInsertEntryFetched(t, d, feedOK1, "hash1", "Recent", now.Unix()-1800, now.Unix()-1800)
	mustInsertEntryFetched(t, d, feedOK2, "hash2", "Old", now.Unix()-(48*3600), now.Unix()-(48*3600))

	m, err := GetAdminMetrics(ctx, d, now)
	require.NoError(t, err)
	require.Equal(t, 3, m.FeedsTotal)
	require.Equal(t, 2, m.FeedsOK)
	require.Equal(t, 2, m.EntriesTotal)
	require.Equal(t, 1, m.Entries24h)
	require.Equal(t, 1, m.FeedsWithErrors)
	require.Equal(t, []string{"Phoronix"}, m.OffendingFeeds)
}

func TestGetAdminMetrics_OffendingFeedsLimitAndOrder(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, d, "alice")
	now := time.Unix(1_700_000_000, 0)

	// 5 erroring feeds, descending error_count
	mustInsertSubscriptionFull(t, d, uid, "e1", "https://e1/feed", 2, 0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e2", "https://e2/feed", 3, 0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e3", "https://e3/feed", 4, 0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e4", "https://e4/feed", 5, 0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e5", "https://e5/feed", 10, 0, now.Unix()+60)

	m, err := GetAdminMetrics(ctx, d, now)
	require.NoError(t, err)
	require.Equal(t, 5, m.FeedsWithErrors)
	require.Equal(t, []string{"e5", "e4", "e3"}, m.OffendingFeeds) // top 3 by error_count desc
}
