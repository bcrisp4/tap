package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTombstones_InsertAndQuery(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "feed", FeedURL: "https://example.com/feed",
		NextPoll: 0, Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, InsertTombstones(ctx, tx, []NewTombstone{
		{SubscriptionID: subID, EntryHash: "abc123", DeletedAt: 1000},
		{SubscriptionID: subID, EntryHash: "def456", DeletedAt: 1001},
	}))
	require.NoError(t, tx.Commit())

	found, err := IsTombstoned(ctx, d, subID, "abc123")
	require.NoError(t, err)
	require.True(t, found)

	found, err = IsTombstoned(ctx, d, subID, "nothere")
	require.NoError(t, err)
	require.False(t, found)
}

func TestTombstones_OnConflictDoNothing(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "feed", FeedURL: "https://conflict.example/feed",
		NextPoll: 0, Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	insert := func(deletedAt int64) {
		tx, err := d.BeginTx(ctx, nil)
		require.NoError(t, err)
		require.NoError(t, InsertTombstones(ctx, tx, []NewTombstone{
			{SubscriptionID: subID, EntryHash: "same", DeletedAt: deletedAt},
		}))
		require.NoError(t, tx.Commit())
	}
	insert(100)
	insert(200) // second insert on same key — must not error

	var got int64
	require.NoError(t, d.QueryRowContext(ctx,
		"SELECT deleted_at FROM tombstones WHERE subscription_id=? AND entry_hash=?",
		subID, "same").Scan(&got))
	require.Equal(t, int64(100), got, "first deleted_at must be preserved")
}

func TestTombstones_CascadeDeleteWithSubscription(t *testing.T) {
	t.Parallel()
	d, uid := newTestUserAndDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		UserID: uid, Title: "feed", FeedURL: "https://cascade.example/feed",
		NextPoll: 0, Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, InsertTombstones(ctx, tx, []NewTombstone{
		{SubscriptionID: subID, EntryHash: "xyz", DeletedAt: 500},
	}))
	require.NoError(t, tx.Commit())

	_, err = d.ExecContext(ctx, "DELETE FROM subscriptions WHERE id = ?", subID)
	require.NoError(t, err)

	found, err := IsTombstoned(ctx, d, subID, "xyz")
	require.NoError(t, err)
	require.False(t, found, "tombstones must cascade-delete with subscription")
}
