package archival

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return d
}

func insertTestUser(t *testing.T, d *sql.DB) int64 {
	t.Helper()
	uid, err := db.InsertUser(context.Background(), d, db.NewUser{
		Username: fmt.Sprintf("u%d", time.Now().UnixNano()), PasswordHash: "x", Role: "admin", CreatedAt: 0,
	})
	require.NoError(t, err)
	return uid
}

func insertSub(t *testing.T, d *sql.DB) int64 {
	t.Helper()
	uid := insertTestUser(t, d)
	id, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid, Title: "feed", FeedURL: fmt.Sprintf("https://%d.example/feed", time.Now().UnixNano()),
		NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	return id
}

func insertEntry(t *testing.T, d *sql.DB, subID int64, hash string, publishedAt int64, read, saved bool) {
	t.Helper()
	ri, si := 0, 0
	if read {
		ri = 1
	}
	if saved {
		si = 1
	}
	// Get user_id from the subscription
	var uid int64
	require.NoError(t, d.QueryRowContext(context.Background(),
		"SELECT user_id FROM subscriptions WHERE id = ?", subID).Scan(&uid))
	_, err := d.ExecContext(context.Background(), `
		INSERT INTO entries (user_id, subscription_id, hash, title, author, url, content,
		                     published_at, fetched_at, read, saved, extract_failed)
		VALUES (?, ?, ?, 'T', NULL, 'https://x', 'content', ?, 0, ?, ?, 0)
	`, uid, subID, hash, publishedAt, ri, si)
	require.NoError(t, err)
}

func writeCacheFile(t *testing.T, dir, hash string, fetchedAt int64) {
	t.Helper()
	bucket := filepath.Join(dir, hash[:2])
	require.NoError(t, os.MkdirAll(bucket, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bucket, hash+".bin"), []byte("imgdata"), 0o644))
	meta, err := json.Marshal(map[string]any{
		"content_type": "image/jpeg", "byte_count": 7, "fetched_at": fetchedAt,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bucket, hash+".meta"), meta, 0o644))
}

// DB pass tests

func TestDBPass_DeletesEligibleEntries(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)

	insertEntry(t, d, subID, "old-read-unsaved", 500, true, false)
	insertEntry(t, d, subID, "old-read-saved", 500, true, true)
	insertEntry(t, d, subID, "old-unread", 500, false, false)
	insertEntry(t, d, subID, "new-read-unsaved", 2000, true, false)

	deleted, tombstoned, err := dbPass(ctx, d, 1000, time.Now().Unix())
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, 1, tombstoned)

	var count int
	require.NoError(t, d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM entries WHERE subscription_id = ?", subID).Scan(&count))
	require.Equal(t, 3, count)

	var tc int
	require.NoError(t, d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM tombstones WHERE subscription_id = ?", subID).Scan(&tc))
	require.Equal(t, 1, tc)
}

func TestDBPass_Idempotent(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)
	insertEntry(t, d, subID, "e", 100, true, false)

	_, _, err := dbPass(ctx, d, 500, time.Now().Unix())
	require.NoError(t, err)

	deleted, tombstoned, err := dbPass(ctx, d, 500, time.Now().Unix())
	require.NoError(t, err)
	require.Equal(t, 0, deleted)
	require.Equal(t, 0, tombstoned)
}

func TestDBPass_ExistingTombstone_NoConflictError(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)
	insertEntry(t, d, subID, "entry", 100, true, false)

	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, db.InsertTombstones(ctx, tx, []db.NewTombstone{
		{SubscriptionID: subID, EntryHash: "entry", DeletedAt: 1},
	}))
	require.NoError(t, tx.Commit())

	deleted, _, err := dbPass(ctx, d, 500, time.Now().Unix())
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
}

func TestDBPass_Chunking(t *testing.T) {
	// Spec requires: 2500 entries → 3 transactions (1000 + 1000 + 500).
	// Verified via chunkHook, which fires once per chunk transaction commit.
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)
	for i := range 2500 {
		insertEntry(t, d, subID, fmt.Sprintf("h%d", i), 100, true, false)
	}

	var txCount int
	deleted, tombstoned, err := dbPassWithHook(ctx, d, 500, time.Now().Unix(), func() { txCount++ })
	require.NoError(t, err)
	require.Equal(t, 2500, deleted)
	require.Equal(t, 2500, tombstoned)
	require.Equal(t, 3, txCount, "expected 3 chunk transactions (1000+1000+500)")

	var count int
	require.NoError(t, d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM entries WHERE subscription_id = ?", subID).Scan(&count))
	require.Equal(t, 0, count)
}

// FS pass tests

func TestFSPass_EvictsOldFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeCacheFile(t, dir, "aabbccddeeff00112233445566778899aabbccddeeff001122", 500)
	writeCacheFile(t, dir, "bb00112233445566778899001122334455667788990011223344", 2000)

	evicted, err := fsPass(dir, 1000, nil)
	require.NoError(t, err)
	require.Equal(t, 1, evicted)
	require.NoFileExists(t, filepath.Join(dir, "aa", "aabbccddeeff00112233445566778899aabbccddeeff001122.bin"))
	require.FileExists(t, filepath.Join(dir, "bb", "bb00112233445566778899001122334455667788990011223344.bin"))
}

func TestFSPass_MissingBin_GracefulENOENT(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hash := "cc001122334455667788990011223344556677889900aabbccdd"
	bucket := filepath.Join(dir, hash[:2])
	require.NoError(t, os.MkdirAll(bucket, 0o755))
	meta, _ := json.Marshal(map[string]any{"content_type": "image/jpeg", "byte_count": 0, "fetched_at": int64(100)})
	require.NoError(t, os.WriteFile(filepath.Join(bucket, hash+".meta"), meta, 0o644))

	evicted, err := fsPass(dir, 500, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, evicted, 1)
	require.NoFileExists(t, filepath.Join(bucket, hash+".meta"))
}

func TestFSPass_OrphanBin_Removed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hash := "dd00112233445566778899001122334455667788990011223344"
	bucket := filepath.Join(dir, hash[:2])
	require.NoError(t, os.MkdirAll(bucket, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bucket, hash+".bin"), []byte("orphan"), 0o644))

	evicted, err := fsPass(dir, 500, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, evicted, 1)
	require.NoFileExists(t, filepath.Join(bucket, hash+".bin"))
}

func TestFSPass_EmptyDir(t *testing.T) {
	t.Parallel()
	evicted, err := fsPass(t.TempDir(), 1000, nil)
	require.NoError(t, err)
	require.Equal(t, 0, evicted)
}

func TestFSPass_NonExistentDir(t *testing.T) {
	t.Parallel()
	evicted, err := fsPass("/tmp/tap-m11-nonexistent-99999", 1000, nil)
	require.NoError(t, err)
	require.Equal(t, 0, evicted)
}

func TestFSPass_OnEvictCallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeCacheFile(t, dir, "ee001122334455667788990011223344556677889900aabbccdd", 100)
	var n int
	_, err := fsPass(dir, 500, func(count int) { n = count })
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

// FTS regression test

func TestDBPass_FTSTriggers_KeepFTSInSync(t *testing.T) {
	// Contractual proof that M9's trigger-based FTS sync survives archival deletes.
	// Skips FTS assertions if entries_fts does not exist (M9 not yet applied).
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)
	insertEntry(t, d, subID, "to-delete", 100, true, false)
	insertEntry(t, d, subID, "to-keep", 100, false, false)

	var ftsExists int
	_ = d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='entries_fts'",
	).Scan(&ftsExists)

	if ftsExists == 1 {
		var before int
		require.NoError(t, d.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM entries_fts WHERE entries_fts MATCH 'content'",
		).Scan(&before))
		require.Equal(t, 2, before)
	}

	_, _, err := dbPass(ctx, d, 500, time.Now().Unix())
	require.NoError(t, err)

	if ftsExists == 1 {
		var after int
		require.NoError(t, d.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM entries_fts WHERE entries_fts MATCH 'content'",
		).Scan(&after))
		require.Equal(t, 1, after)
	}

	var entryCount int
	require.NoError(t, d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM entries WHERE subscription_id = ?", subID,
	).Scan(&entryCount))
	require.Equal(t, 1, entryCount)
}
