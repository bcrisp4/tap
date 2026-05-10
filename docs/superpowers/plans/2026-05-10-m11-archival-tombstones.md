# M11 — Archival + Tombstones Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a daily archival sweep that deletes read-and-unsaved entries older than a configurable horizon (recording tombstones), prunes old media cache files, and prevents re-published archived entries from reappearing as unread.

**Architecture:** A new `internal/archival` package owns an `Archiver` struct with `Start()`/`Stop()` lifecycle matching `Scheduler`. The DB pass runs in 1000-row chunk transactions; the FS pass walks the proxy cache dir and unlinks files whose `.meta` sidecar `fetched_at` is too old. The poll worker gains a tombstone consult (read-only, before the commit transaction) so re-published entries are silently dropped. FTS stays in sync automatically via M9's per-row DELETE triggers — no explicit sync needed.

**Tech Stack:** Go 1.25, `database/sql`, `modernc.org/sqlite`, `log/slog`, `sync.WaitGroup`, `context.WithCancel`, standard `os`/`filepath` for FS operations, `github.com/stretchr/testify/require` for tests.

**Cross-milestone dependencies:**
- **Requires M9 migrations to be applied** before M11's FTS regression test is meaningful (FTS triggers must exist). M11 migration is `0010_tombstones.sql` — runs after M9's `0008_*` and `0009_*`.
- **M7 user_id:** Tombstone table uses `subscription_id` FK; the FK chain `tombstone → subscription → user` provides per-user scoping transitively once M7 lands. No schema change needed in M11.
- **M3 cache layout:** FS pass reads `.meta` sidecars written by M3's `internal/proxy` package (field `fetched_at`). The `sidecar` struct must match M3's JSON shape exactly.

---

## File Map

| Action | Path | Responsibility |
|---|---|---|
| Create | `internal/db/migrations/0010_tombstones.sql` | Schema for `tombstones` table |
| Create | `internal/db/tombstones.go` | `InsertTombstones`, `IsTombstoned` |
| Create | `internal/db/tombstones_test.go` | DB-layer tombstone tests |
| Modify | `internal/db/entries.go` | Add `ListArchivable` + `ArchivableEntry` |
| Modify | `internal/db/entries_test.go` | Tests for `ListArchivable` |
| Modify | `internal/poll/worker.go` | Tombstone consult before insert |
| Modify | `internal/poll/worker_test.go` | Tombstone consult tests |
| Create | `internal/archival/archiver.go` | `Archiver` struct, lifecycle, ticker |
| Create | `internal/archival/archiver_test.go` | Lifecycle + clock-injection tests |
| Create | `internal/archival/sweep.go` | `dbPass`, `fsPass` implementations |
| Create | `internal/archival/sweep_test.go` | Sweep correctness + FTS regression |
| Modify | `cmd/tap/main.go` | Wire `Archiver` into `runServer` |

---

## Task 1: Schema migration — tombstones table

**Skills to invoke:** `superpowers:test-driven-development`

**Files:**
- Create: `internal/db/migrations/0010_tombstones.sql`
- Modify: `internal/db/migrate_test.go` (extend migration smoke test)

- [ ] **Step 1: Write the migration SQL**

Create `internal/db/migrations/0010_tombstones.sql`:

```sql
CREATE TABLE tombstones (
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    entry_hash      TEXT    NOT NULL,
    deleted_at      INTEGER NOT NULL,
    PRIMARY KEY (subscription_id, entry_hash)
);
CREATE INDEX idx_tombstones_subscription ON tombstones(subscription_id);
```

- [ ] **Step 2: Verify migration applies cleanly**

Run:
```bash
go test ./internal/db/... -run TestMigrate -race -v
```
Expected: PASS. The existing `TestMigrate` test in `internal/db/migrate_test.go` calls `db.Open(":memory:")` + `db.Migrate()` and verifies all migrations apply without error. The new migration is picked up automatically (lexical ordering). If `TestMigrate` doesn't exist, check `migrate_test.go` — it uses `newTestDB(t)` which calls `Migrate`.

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations/0010_tombstones.sql
git commit -m "M11: add 0010_tombstones migration"
```

---

## Task 2: DB layer — `tombstones.go`

**Skills to invoke:** `superpowers:test-driven-development`, `golang-database`

**Files:**
- Create: `internal/db/tombstones.go`
- Create: `internal/db/tombstones_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/db/tombstones_test.go`:

```go
package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTombstones_InsertAndQuery(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	// Insert a subscription to satisfy the FK
	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "feed", FeedURL: "https://example.com/feed",
		NextPoll: 0, Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)

	err = InsertTombstones(ctx, tx, []NewTombstone{
		{SubscriptionID: subID, EntryHash: "abc123", DeletedAt: 1000},
		{SubscriptionID: subID, EntryHash: "def456", DeletedAt: 1001},
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// IsTombstoned: hit
	found, err := IsTombstoned(ctx, d, subID, "abc123")
	require.NoError(t, err)
	require.True(t, found)

	// IsTombstoned: miss
	found, err = IsTombstoned(ctx, d, subID, "nothere")
	require.NoError(t, err)
	require.False(t, found)
}

func TestTombstones_OnConflictDoNothing(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "feed", FeedURL: "https://example.com/feed2",
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
	insert(200) // second insert — must not error

	// deleted_at from first insert is preserved
	var got int64
	require.NoError(t, d.QueryRowContext(ctx,
		"SELECT deleted_at FROM tombstones WHERE subscription_id=? AND entry_hash=?",
		subID, "same").Scan(&got))
	require.Equal(t, int64(100), got)
}

func TestTombstones_CascadeDeleteWithSubscription(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "feed", FeedURL: "https://example.com/feed3",
		NextPoll: 0, Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, InsertTombstones(ctx, tx, []NewTombstone{
		{SubscriptionID: subID, EntryHash: "xyz", DeletedAt: 500},
	}))
	require.NoError(t, tx.Commit())

	// Delete the subscription — tombstones must cascade
	_, err = d.ExecContext(ctx, "DELETE FROM subscriptions WHERE id = ?", subID)
	require.NoError(t, err)

	found, err := IsTombstoned(ctx, d, subID, "xyz")
	require.NoError(t, err)
	require.False(t, found)
}
```

- [ ] **Step 2: Run tests, confirm they fail**

```bash
go test ./internal/db/... -run TestTombstones -race -v
```
Expected: FAIL — `InsertTombstones`, `IsTombstoned`, `NewTombstone` undefined.

- [ ] **Step 3: Implement `internal/db/tombstones.go`**

```go
package db

import (
	"context"
	"database/sql"
	"fmt"
)

// NewTombstone is the write shape for a tombstone row.
type NewTombstone struct {
	SubscriptionID int64
	EntryHash      string
	DeletedAt      int64 // unix seconds
}

// InsertTombstones bulk-inserts tombstone rows within the caller's transaction.
// ON CONFLICT DO NOTHING — existing tombstones are kept as-is (idempotent).
func InsertTombstones(ctx context.Context, tx *sql.Tx, rows []NewTombstone) error {
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tombstones (subscription_id, entry_hash, deleted_at)
			VALUES (?, ?, ?)
			ON CONFLICT (subscription_id, entry_hash) DO NOTHING
		`, r.SubscriptionID, r.EntryHash, r.DeletedAt); err != nil {
			return fmt.Errorf("insert tombstone (sub=%d hash=%s): %w", r.SubscriptionID, r.EntryHash, err)
		}
	}
	return nil
}

// IsTombstoned returns true if (subscriptionID, entryHash) exists in tombstones.
func IsTombstoned(ctx context.Context, d *sql.DB, subscriptionID int64, entryHash string) (bool, error) {
	var exists int
	err := d.QueryRowContext(ctx, `
		SELECT 1 FROM tombstones
		WHERE subscription_id = ? AND entry_hash = ?
		LIMIT 1
	`, subscriptionID, entryHash).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check tombstone (sub=%d hash=%s): %w", subscriptionID, entryHash, err)
	}
	return true, nil
}
```

- [ ] **Step 4: Run tests, confirm they pass**

```bash
go test ./internal/db/... -run TestTombstones -race -v
```
Expected: PASS — all three tests green.

- [ ] **Step 5: Commit**

```bash
git add internal/db/tombstones.go internal/db/tombstones_test.go
git commit -m "M11: add db.InsertTombstones and db.IsTombstoned"
```

---

## Task 3: DB layer — `ListArchivable`

**Skills to invoke:** `superpowers:test-driven-development`, `golang-database`

**Files:**
- Modify: `internal/db/entries.go` (add `ArchivableEntry` type + `ListArchivable` func)
- Modify: `internal/db/entries_test.go` (add tests)

- [ ] **Step 1: Write failing tests**

Add to `internal/db/entries_test.go`:

```go
func TestListArchivable(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	// Insert a subscription
	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "feed", FeedURL: "https://a.example/feed",
		NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	now := int64(10000)
	horizon := int64(1000) // entries older than unix ts 1000 are eligible

	// Helper: insert an entry with given published_at, read, saved
	insert := func(hash string, publishedAt int64, read, saved bool) {
		t.Helper()
		_, err := d.ExecContext(ctx, `
			INSERT INTO entries (subscription_id, hash, title, author, url, content,
			                     published_at, fetched_at, read, saved, extract_failed)
			VALUES (?, ?, 'T', NULL, 'https://x', 'c', ?, ?, ?, ?, 0)
		`, subID, hash, publishedAt, now, boolToInt(read), boolToInt(saved))
		require.NoError(t, err)
	}

	// Eligible: read=1, saved=0, old
	insert("old-read-unsaved", 500, true, false)
	// Not eligible: saved
	insert("old-read-saved", 500, true, true)
	// Not eligible: unread
	insert("old-unread", 500, false, false)
	// Not eligible: too recent
	insert("new-read-unsaved", 2000, true, false)

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

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "f", FeedURL: "https://b.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	for i := range 5 {
		_, err := d.ExecContext(ctx, `
			INSERT INTO entries (subscription_id, hash, title, author, url, content,
			                     published_at, fetched_at, read, saved, extract_failed)
			VALUES (?, ?, 'T', NULL, 'https://x', 'c', 100, 200, 1, 0, 0)
		`, subID, fmt.Sprintf("hash%d", i))
		require.NoError(t, err)
	}

	rows, err := ListArchivable(ctx, d, 500, 3)
	require.NoError(t, err)
	require.Len(t, rows, 3)
}
```

- [ ] **Step 2: Run tests, confirm they fail**

```bash
go test ./internal/db/... -run TestListArchivable -race -v
```
Expected: FAIL — `ListArchivable`, `ArchivableEntry` undefined.

- [ ] **Step 3: Implement in `internal/db/entries.go`**

Add after the existing functions:

```go
// ArchivableEntry is the minimal row shape returned by ListArchivable.
type ArchivableEntry struct {
	ID             int64
	SubscriptionID int64
	Hash           string
}

// ListArchivable returns up to limit entries eligible for archival:
// read=1, saved=0, published_at < horizonUnix. Results are ordered by
// published_at ASC so the oldest entries are evicted first.
func ListArchivable(ctx context.Context, d *sql.DB, horizonUnix int64, limit int) ([]ArchivableEntry, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, subscription_id, hash
		FROM entries
		WHERE read = 1 AND saved = 0 AND published_at < ?
		ORDER BY published_at ASC
		LIMIT ?
	`, horizonUnix, limit)
	if err != nil {
		return nil, fmt.Errorf("list archivable: %w", err)
	}
	defer rows.Close()

	var out []ArchivableEntry
	for rows.Next() {
		var e ArchivableEntry
		if err := rows.Scan(&e.ID, &e.SubscriptionID, &e.Hash); err != nil {
			return nil, fmt.Errorf("scan archivable entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run tests, confirm they pass**

```bash
go test ./internal/db/... -run TestListArchivable -race -v
```
Expected: PASS.

- [ ] **Step 5: Run full db package tests to check for regressions**

```bash
go test ./internal/db/... -race
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/db/entries.go internal/db/entries_test.go
git commit -m "M11: add db.ListArchivable"
```

---

## Task 4: Poll worker — tombstone consult before insert

**Skills to invoke:** `superpowers:test-driven-development`, `golang-database`

**Files:**
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/poll/worker_test.go`:

```go
func TestWorker_TombstonedEntryNotInserted(t *testing.T) {
	t.Parallel()
	d := newDB(t)
	ctx := context.Background()

	// Set up a fixture feed origin
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, sampleAtom) // sampleAtom has one entry with id urn:sample:1
	}))
	t.Cleanup(origin.Close)

	// Insert the subscription
	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "s", FeedURL: origin.URL + "/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	sub := db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"}

	// Compute the hash that will be assigned to the one entry in sampleAtom.
	// feed.EntryHash uses the subscription ID and item — we need to do a real
	// poll first to discover it, then tombstone it and verify re-poll skips it.

	// First poll — entry should land
	w := poll.NewWorker(d, http.DefaultClient, poll.WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(ctx, sub)

	entries, _, _, err := db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1, "first poll must insert the entry")
	entryHash := entries[0].Hash

	// Manually tombstone the entry
	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, db.InsertTombstones(ctx, tx, []db.NewTombstone{
		{SubscriptionID: subID, EntryHash: entryHash, DeletedAt: 9999},
	}))
	require.NoError(t, tx.Commit())

	// Delete it from entries so the next poll would re-insert it (without tombstone check)
	_, err = d.ExecContext(ctx, "DELETE FROM entries WHERE hash = ? AND subscription_id = ?", entryHash, subID)
	require.NoError(t, err)

	// Second poll — entry must NOT reappear
	w.Run(ctx, sub)
	entries, _, _, err = db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, entries, "tombstoned entry must not reappear")
}

func TestWorker_TombstoneMiss_EntryInserted(t *testing.T) {
	t.Parallel()
	d := newDB(t)
	ctx := context.Background()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, sampleAtom)
	}))
	t.Cleanup(origin.Close)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "s", FeedURL: origin.URL + "/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	sub := db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"}
	w := poll.NewWorker(d, http.DefaultClient, poll.WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(ctx, sub)

	entries, _, _, err := db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1, "non-tombstoned entry must be inserted normally")
}

func TestWorker_MixedTombstones(t *testing.T) {
	// Feed has 3 entries: 1 tombstoned, 2 not. After poll: 2 inserted, 1 skipped.
	t.Parallel()
	d := newDB(t)
	ctx := context.Background()

	const threeEntryAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Three</title><id>urn:three</id><updated>2026-05-01T00:00:00Z</updated>
  <entry><title>A</title><id>urn:three:1</id><link href="https://x/1"/>
    <updated>2026-05-01T00:00:00Z</updated><content type="html">a</content></entry>
  <entry><title>B</title><id>urn:three:2</id><link href="https://x/2"/>
    <updated>2026-05-01T00:00:00Z</updated><content type="html">b</content></entry>
  <entry><title>C</title><id>urn:three:3</id><link href="https://x/3"/>
    <updated>2026-05-01T00:00:00Z</updated><content type="html">c</content></entry>
</feed>`

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, threeEntryAtom)
	}))
	t.Cleanup(origin.Close)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "s", FeedURL: origin.URL + "/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	sub := db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"}

	// First poll to discover hashes
	w := poll.NewWorker(d, http.DefaultClient, poll.WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(ctx, sub)

	entries, _, _, err := db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 3)

	// Tombstone entry B, delete all entries to force re-insert attempt
	hashB := entries[1].Hash // ordered newest-first; middle entry
	// find the one with URL https://x/2
	var tombstoneHash string
	for _, e := range entries {
		if e.URL == "https://x/2" {
			tombstoneHash = e.Hash
		}
	}
	require.NotEmpty(t, tombstoneHash)

	_ = hashB // suppress lint
	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, db.InsertTombstones(ctx, tx, []db.NewTombstone{
		{SubscriptionID: subID, EntryHash: tombstoneHash, DeletedAt: 1000},
	}))
	require.NoError(t, tx.Commit())
	_, err = d.ExecContext(ctx, "DELETE FROM entries WHERE subscription_id = ?", subID)
	require.NoError(t, err)

	// Second poll — 2 entries inserted, 1 skipped
	w.Run(ctx, sub)
	entries, _, _, err = db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	for _, e := range entries {
		require.NotEqual(t, tombstoneHash, e.Hash, "tombstoned entry must not appear")
	}
}
```

- [ ] **Step 2: Run tests, confirm they fail**

```bash
go test ./internal/poll/... -run "TestWorker_Tombstone" -race -v
```
Expected: FAIL — worker doesn't consult tombstones yet, so the tombstoned entry re-appears.

- [ ] **Step 3: Add tombstone consult to `internal/poll/worker.go`**

In `Worker.Run`, after building `newEntries` and before calling `db.UpdateAfterPoll`, add the tombstone filter. Find the block starting `newEntries := make([]db.NewEntry...)` and replace it:

```go
	// Build candidate entries from parse results.
	candidates := make([]db.NewEntry, 0, len(pendings))
	for _, p := range pendings {
		pubAt := now.Unix()
		if p.item.PublishedParsed != nil {
			pubAt = p.item.PublishedParsed.Unix()
		}
		candidates = append(candidates, db.NewEntry{
			Hash:          feed.EntryHash(sub.ID, p.item),
			Title:         p.item.Title,
			Author:        authorName(p.item),
			URL:           p.item.Link,
			Content:       w.opts.Processor.Process(p.content),
			PublishedAt:   pubAt,
			ExtractFailed: p.extractFailed,
		})
	}

	// Consult tombstones — drop any entry that has been previously archived.
	// This read happens outside the commit transaction (read-only point lookup).
	newEntries := make([]db.NewEntry, 0, len(candidates))
	for _, e := range candidates {
		tombstoned, terr := db.IsTombstoned(ctx, w.db, sub.ID, e.Hash)
		if terr != nil {
			slog.WarnContext(ctx, "tombstone check failed, including entry",
				"feed_id", sub.ID, "hash", e.Hash, "err", terr)
			newEntries = append(newEntries, e)
			continue
		}
		if tombstoned {
			continue // silently drop
		}
		newEntries = append(newEntries, e)
	}
```

Also add `"github.com/bcrisp4/tap/internal/db"` to the import if not already present (it is).

- [ ] **Step 4: Run tests, confirm they pass**

```bash
go test ./internal/poll/... -run "TestWorker_Tombstone" -race -v
```
Expected: PASS.

- [ ] **Step 5: Run full poll package tests**

```bash
go test ./internal/poll/... -race
```
Expected: PASS — no regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/poll/worker.go internal/poll/worker_test.go
git commit -m "M11: poll worker consults tombstones before insert"
```

---

## Task 5: `internal/archival/sweep.go` — DB pass

**Skills to invoke:** `superpowers:test-driven-development`, `golang-database`, `golang-context`

**Files:**
- Create: `internal/archival/sweep.go`
- Create: `internal/archival/sweep_test.go` (partial — DB pass tests)

- [ ] **Step 1: Write failing tests for `dbPass`**

Create `internal/archival/sweep_test.go`:

```go
package archival

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

// openTestDB creates an in-memory SQLite DB with all migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return d
}

// insertSub inserts a test subscription and returns its ID.
func insertSub(t *testing.T, d *sql.DB) int64 {
	t.Helper()
	id, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "feed", FeedURL: fmt.Sprintf("https://%d.example/feed", time.Now().UnixNano()),
		NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	return id
}

// insertEntry inserts an entry and returns its ID.
func insertEntry(t *testing.T, d *sql.DB, subID int64, hash string, publishedAt int64, read, saved bool) {
	t.Helper()
	_, err := d.ExecContext(context.Background(), `
		INSERT INTO entries (subscription_id, hash, title, author, url, content,
		                     published_at, fetched_at, read, saved, extract_failed)
		VALUES (?, ?, 'T', NULL, 'https://x', 'content of `+"`"+hash+"`"+`', ?, 0, ?, ?, 0)
	`, subID, hash, publishedAt, boolInt(read), boolInt(saved))
	require.NoError(t, err)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestDBPass_DeletesEligibleEntries(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)

	horizon := int64(1000)
	insertEntry(t, d, subID, "old-read-unsaved", 500, true, false)   // eligible
	insertEntry(t, d, subID, "old-read-saved", 500, true, true)      // saved — retain
	insertEntry(t, d, subID, "old-unread", 500, false, false)        // unread — retain
	insertEntry(t, d, subID, "new-read-unsaved", 2000, true, false)  // too new — retain

	deleted, tombstoned, err := dbPass(ctx, d, horizon, time.Now().Unix())
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, 1, tombstoned)

	// Verify only the eligible entry was deleted
	var count int
	require.NoError(t, d.QueryRowContext(ctx, "SELECT COUNT(*) FROM entries WHERE subscription_id = ?", subID).Scan(&count))
	require.Equal(t, 3, count)

	// Verify tombstone was written
	var tc int
	require.NoError(t, d.QueryRowContext(ctx, "SELECT COUNT(*) FROM tombstones WHERE subscription_id = ?", subID).Scan(&tc))
	require.Equal(t, 1, tc)
}

func TestDBPass_Idempotent(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)

	insertEntry(t, d, subID, "eligible", 100, true, false)

	_, _, err := dbPass(ctx, d, 500, time.Now().Unix())
	require.NoError(t, err)

	// Second run — nothing left to delete, no error
	deleted, tombstoned, err := dbPass(ctx, d, 500, time.Now().Unix())
	require.NoError(t, err)
	require.Equal(t, 0, deleted)
	require.Equal(t, 0, tombstoned)
}

func TestDBPass_OnConflictDoNothing_ExistingTombstone(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)

	insertEntry(t, d, subID, "entry", 100, true, false)

	// Pre-insert tombstone
	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, db.InsertTombstones(ctx, tx, []db.NewTombstone{
		{SubscriptionID: subID, EntryHash: "entry", DeletedAt: 1},
	}))
	require.NoError(t, tx.Commit())

	// dbPass must succeed without error despite tombstone conflict
	deleted, _, err := dbPass(ctx, d, 500, time.Now().Unix())
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
}

func TestDBPass_Chunking(t *testing.T) {
	// 2500 eligible entries → 3 chunk transactions (1000 + 1000 + 500)
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)

	for i := range 2500 {
		insertEntry(t, d, subID, fmt.Sprintf("h%d", i), 100, true, false)
	}

	deleted, tombstoned, err := dbPass(ctx, d, 500, time.Now().Unix())
	require.NoError(t, err)
	require.Equal(t, 2500, deleted)
	require.Equal(t, 2500, tombstoned)

	var count int
	require.NoError(t, d.QueryRowContext(ctx, "SELECT COUNT(*) FROM entries WHERE subscription_id = ?", subID).Scan(&count))
	require.Equal(t, 0, count)
}
```

- [ ] **Step 2: Run tests, confirm they fail**

```bash
go test ./internal/archival/... -run "TestDBPass" -race -v
```
Expected: FAIL — package doesn't exist yet, or `dbPass` undefined.

- [ ] **Step 3: Implement `internal/archival/sweep.go` (DB pass only)**

Create `internal/archival/sweep.go`:

```go
package archival

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/bcrisp4/tap/internal/db"
)

const chunkSize = 1000

// dbPass deletes eligible entries and records tombstones in chunk transactions.
// Returns (deleted, tombstoned, error). tombstoned may be less than deleted if
// some tombstones already existed (ON CONFLICT DO NOTHING).
func dbPass(ctx context.Context, d *sql.DB, horizonUnix, nowUnix int64) (deleted, tombstoned int, err error) {
	for {
		rows, err := db.ListArchivable(ctx, d, horizonUnix, chunkSize)
		if err != nil {
			return deleted, tombstoned, fmt.Errorf("list archivable: %w", err)
		}
		if len(rows) == 0 {
			return deleted, tombstoned, nil
		}

		ids := make([]int64, len(rows))
		tombstones := make([]db.NewTombstone, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
			tombstones[i] = db.NewTombstone{
				SubscriptionID: r.SubscriptionID,
				EntryHash:      r.Hash,
				DeletedAt:      nowUnix,
			}
		}

		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return deleted, tombstoned, fmt.Errorf("begin tx: %w", err)
		}

		if err := db.InsertTombstones(ctx, tx, tombstones); err != nil {
			_ = tx.Rollback()
			return deleted, tombstoned, err
		}

		// Build DELETE ... WHERE id IN (...)
		// Inline ids — safe because these are int64 from our own DB, not user input.
		query := buildDeleteByIDs(ids)
		res, err := tx.ExecContext(ctx, query)
		if err != nil {
			_ = tx.Rollback()
			return deleted, tombstoned, fmt.Errorf("delete entries chunk: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return deleted, tombstoned, fmt.Errorf("commit chunk: %w", err)
		}

		n, _ := res.RowsAffected()
		deleted += int(n)
		tombstoned += len(tombstones)
	}
}

// buildDeleteByIDs constructs a DELETE statement for a slice of int64 IDs.
// IDs are internal DB values, not user input, so direct interpolation is safe.
func buildDeleteByIDs(ids []int64) string {
	if len(ids) == 0 {
		return "DELETE FROM entries WHERE 1=0"
	}
	q := "DELETE FROM entries WHERE id IN ("
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += fmt.Sprintf("%d", id)
	}
	return q + ")"
}

// cacheFileMeta mirrors the JSON sidecar written by internal/proxy.Cache.
// Only FetchedAt is read by the FS pass; other fields are decoded but ignored.
type cacheFileMeta struct {
	ContentType string `json:"content_type"`
	ETag        string `json:"etag,omitempty"`
	ByteCount   int64  `json:"byte_count"`
	FetchedAt   int64  `json:"fetched_at"`
}

// fsPass walks cacheDir and unlinks .bin + .meta pairs whose fetched_at is
// older than ageCapUnix. Returns count of evicted file pairs.
func fsPass(cacheDir string, ageCapUnix int64, onEvict func(int)) (evicted int, err error) {
	// Two-pass: collect candidates from .meta files, then delete.
	type candidate struct {
		binPath  string
		metaPath string
	}
	var candidates []candidate
	var orphanBins []string // .bin files with no corresponding .meta

	seenBins := map[string]bool{}

	err = filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			if errors.Is(werr, os.ErrNotExist) {
				return nil
			}
			return werr
		}
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		base := path[:len(path)-len(ext)]

		if ext == ".bin" {
			seenBins[base] = true
			return nil
		}
		if ext != ".meta" {
			return nil
		}

		// Read and decode the sidecar
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			slog.Warn("archival: read meta sidecar", "path", path, "err", err)
			return nil
		}
		var meta cacheFileMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			slog.Warn("archival: decode meta sidecar", "path", path, "err", err)
			return nil
		}

		if meta.FetchedAt < ageCapUnix {
			candidates = append(candidates, candidate{
				binPath:  base + ".bin",
				metaPath: path,
			})
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("walk cache dir: %w", err)
	}

	// Detect orphan .bin files (no .meta sibling found in walk)
	err = filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			return werr
		}
		if filepath.Ext(path) != ".bin" {
			return nil
		}
		base := path[:len(path)-len(".bin")]
		if !seenBins[base] {
			// seenBins is populated from first walk; check if .meta exists
		}
		// Simpler: check if .meta exists alongside
		if _, err := os.Stat(base + ".meta"); errors.Is(err, os.ErrNotExist) {
			orphanBins = append(orphanBins, path)
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("archival: walk for orphan bins", "err", err)
	}

	// Evict candidates
	for _, c := range candidates {
		if err := os.Remove(c.binPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("archival: remove bin", "path", c.binPath, "err", err)
		}
		if err := os.Remove(c.metaPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("archival: remove meta", "path", c.metaPath, "err", err)
		}
		evicted++
	}

	// Remove orphan bins
	for _, path := range orphanBins {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("archival: remove orphan bin", "path", path, "err", err)
		}
		evicted++
	}

	if onEvict != nil && evicted > 0 {
		onEvict(evicted)
	}
	return evicted, nil
}

// Note: boolInt is defined in sweep_test.go (same package, test-only helper).
// Do NOT add boolInt here — it would be a duplicate compile error.
```

Note: `boolInt` is unexported and lives in sweep.go for use by tests in the same package. If there's already a `boolToInt` in the `db` package, do not import it here — keep packages clean.

- [ ] **Step 4: Run tests, confirm they pass**

```bash
go test ./internal/archival/... -run "TestDBPass" -race -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/archival/sweep.go internal/archival/sweep_test.go
git commit -m "M11: implement archival dbPass with chunk transactions"
```

---

## Task 6: `sweep_test.go` — FS pass tests + FTS regression

**Skills to invoke:** `superpowers:test-driven-development`, `golang-testing`

**Files:**
- Modify: `internal/archival/sweep_test.go` (add FS pass tests and FTS regression)

- [ ] **Step 1: Write FS pass tests**

Add to `internal/archival/sweep_test.go`:

```go
import (
	"encoding/json"
	"os"
	"path/filepath"
	// ... existing imports
)

// writeCacheFile writes a .bin + .meta pair to the test cache dir.
func writeCacheFile(t *testing.T, dir, hash string, fetchedAt int64) {
	t.Helper()
	bucket := hash[:2]
	bucketDir := filepath.Join(dir, bucket)
	require.NoError(t, os.MkdirAll(bucketDir, 0o755))

	binPath := filepath.Join(bucketDir, hash+".bin")
	metaPath := filepath.Join(bucketDir, hash+".meta")

	require.NoError(t, os.WriteFile(binPath, []byte("imgdata"), 0o644))
	meta, err := json.Marshal(map[string]any{
		"content_type": "image/jpeg",
		"byte_count":   7,
		"fetched_at":   fetchedAt,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(metaPath, meta, 0o644))
}

func TestFSPass_EvictsOldFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ageCap := int64(1000) // files with fetched_at < 1000 are evicted
	writeCacheFile(t, dir, "aabbcc001122334455667788990011223344556677889900aabb", 500) // old → evict
	writeCacheFile(t, dir, "bb00112233445566778899001122334455667788990011223344", 2000) // new → keep

	evicted, err := fsPass(dir, ageCap, nil)
	require.NoError(t, err)
	require.Equal(t, 1, evicted)

	// Old files must be gone
	require.NoFileExists(t, filepath.Join(dir, "aa", "aabbcc001122334455667788990011223344556677889900aabb.bin"))
	require.NoFileExists(t, filepath.Join(dir, "aa", "aabbcc001122334455667788990011223344556677889900aabb.meta"))
	// New files must remain
	require.FileExists(t, filepath.Join(dir, "bb", "bb00112233445566778899001122334455667788990011223344.bin"))
}

func TestFSPass_MissingBin_GracefulENOENT(t *testing.T) {
	// .meta exists (old), .bin already gone (evicted by M3 LRU) — should succeed, remove .meta
	t.Parallel()
	dir := t.TempDir()

	hash := "cc001122334455667788990011223344556677889900aabbccdd"
	bucket := hash[:2]
	bucketDir := filepath.Join(dir, bucket)
	require.NoError(t, os.MkdirAll(bucketDir, 0o755))

	meta, _ := json.Marshal(map[string]any{"content_type": "image/jpeg", "byte_count": 0, "fetched_at": int64(100)})
	require.NoError(t, os.WriteFile(filepath.Join(bucketDir, hash+".meta"), meta, 0o644))
	// No .bin file

	evicted, err := fsPass(dir, 500, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, evicted, 1)
	require.NoFileExists(t, filepath.Join(bucketDir, hash+".meta"))
}

func TestFSPass_MissingMeta_OrphanBinRemoved(t *testing.T) {
	// .bin exists, .meta missing — orphan cleanup
	t.Parallel()
	dir := t.TempDir()

	hash := "dd00112233445566778899001122334455667788990011223344"
	bucket := hash[:2]
	bucketDir := filepath.Join(dir, bucket)
	require.NoError(t, os.MkdirAll(bucketDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bucketDir, hash+".bin"), []byte("orphan"), 0o644))

	evicted, err := fsPass(dir, 500, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, evicted, 1)
	require.NoFileExists(t, filepath.Join(bucketDir, hash+".bin"))
}

func TestFSPass_EmptyDir_NoError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evicted, err := fsPass(dir, 1000, nil)
	require.NoError(t, err)
	require.Equal(t, 0, evicted)
}

func TestFSPass_OnEvictCallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeCacheFile(t, dir, "ee001122334455667788990011223344556677889900aabbccdd", 100)

	var callbackN int
	_, err := fsPass(dir, 500, func(n int) { callbackN = n })
	require.NoError(t, err)
	require.Equal(t, 1, callbackN)
}

func TestFSPass_NonExistentDir_NoError(t *testing.T) {
	t.Parallel()
	evicted, err := fsPass("/tmp/tap-archival-test-nonexistent-dir-12345", 1000, nil)
	require.NoError(t, err)
	require.Equal(t, 0, evicted)
}
```

- [ ] **Step 2: Write FTS regression test**

Add to `internal/archival/sweep_test.go`:

```go
func TestDBPass_FTSTriggers_KeepFTSInSync(t *testing.T) {
	// This test is the contractual proof that M9's trigger-based FTS sync
	// survives archival deletes. If M9's migrations are not applied, this
	// test is still valid — dbPass deletes from entries correctly — but the
	// FTS assertion will trivially pass (no FTS index to corrupt).
	t.Parallel()
	d := openTestDB(t)
	ctx := context.Background()
	subID := insertSub(t, d)

	// Insert two entries: one to be archived, one to keep
	insertEntry(t, d, subID, "to-delete", 100, true, false)
	insertEntry(t, d, subID, "to-keep", 100, false, false) // unread — retained

	// Check if the FTS table exists (M9 may not have run yet in this environment)
	var ftsExists int
	_ = d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='entries_fts'",
	).Scan(&ftsExists)

	if ftsExists == 1 {
		// Verify both entries appear in FTS before sweep
		var beforeCount int
		require.NoError(t, d.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM entries_fts WHERE entries_fts MATCH 'content'",
		).Scan(&beforeCount))
		require.Equal(t, 2, beforeCount, "both entries must appear in FTS before sweep")
	}

	_, _, err := dbPass(ctx, d, 500, time.Now().Unix())
	require.NoError(t, err)

	if ftsExists == 1 {
		// After sweep: deleted entry must be gone from FTS, kept entry must remain
		var afterCount int
		require.NoError(t, d.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM entries_fts WHERE entries_fts MATCH 'content'",
		).Scan(&afterCount))
		require.Equal(t, 1, afterCount, "only retained entry must appear in FTS after sweep")
	}

	// Regardless of FTS: entries table must have exactly 1 row
	var entryCount int
	require.NoError(t, d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM entries WHERE subscription_id = ?", subID,
	).Scan(&entryCount))
	require.Equal(t, 1, entryCount)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/archival/... -run "TestFSPass|TestDBPass_FTS" -race -v
```
Expected: PASS — FS pass tests and FTS regression test all green.

- [ ] **Step 4: Commit**

```bash
git add internal/archival/sweep_test.go
git commit -m "M11: add FS pass tests and FTS regression test"
```

---

## Task 7: `internal/archival/archiver.go` — lifecycle

**Skills to invoke:** `superpowers:test-driven-development`, `golang-concurrency`, `golang-context`

**Files:**
- Create: `internal/archival/archiver.go`
- Create: `internal/archival/archiver_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/archival/archiver_test.go`:

```go
package archival

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestArchiver_StartStop_NoSweep(t *testing.T) {
	// Start and stop with a long interval — no sweep fires, clean exit.
	t.Parallel()
	d := openTestDB(t)

	a := NewArchiver(d, ArchiverOpts{
		Horizon:     90 * 24 * time.Hour,
		CacheAgeCap: 14 * 24 * time.Hour,
		Interval:    24 * time.Hour, // won't fire during test
		CacheDir:    t.TempDir(),
	})
	a.Start()
	a.Stop() // must not block indefinitely
}

func TestArchiver_Stop_WaitsForInProgressSweep(t *testing.T) {
	// Block the DB pass via a gate channel; Stop() must wait until it unblocks.
	t.Parallel()
	d := openTestDB(t)

	gate := make(chan struct{})
	var sweepStarted atomic.Bool

	a := NewArchiver(d, ArchiverOpts{
		Horizon:     1 * time.Second,
		CacheAgeCap: 1 * time.Second,
		Interval:    1 * time.Millisecond, // fires immediately
		CacheDir:    t.TempDir(),
		// inject a hook to block the sweep
		sweepHook: func() {
			sweepStarted.Store(true)
			<-gate
		},
	})
	a.Start()

	// Wait until the sweep actually starts
	require.Eventually(t, func() bool { return sweepStarted.Load() }, 2*time.Second, 10*time.Millisecond)

	stopDone := make(chan struct{})
	go func() {
		a.Stop()
		close(stopDone)
	}()

	// Stop should be blocked while sweep is in progress
	select {
	case <-stopDone:
		t.Fatal("Stop() returned before sweep finished")
	case <-time.After(100 * time.Millisecond):
	}

	// Unblock the sweep — Stop() should now return
	close(gate)
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return after sweep unblocked")
	}
}

func TestArchiver_ClockInjection_SweepFires(t *testing.T) {
	// Use a controlled clock — advance past interval, verify sweep was called.
	t.Parallel()
	d := openTestDB(t)

	var sweepCount atomic.Int32
	now := time.Now()
	mu := sync.Mutex{}

	a := NewArchiver(d, ArchiverOpts{
		Horizon:     90 * 24 * time.Hour,
		CacheAgeCap: 14 * 24 * time.Hour,
		Interval:    1 * time.Millisecond, // fires almost immediately with real ticker
		CacheDir:    t.TempDir(),
		Now:         func() time.Time { mu.Lock(); defer mu.Unlock(); return now },
		sweepHook:   func() { sweepCount.Add(1) },
	})
	a.Start()

	require.Eventually(t, func() bool {
		return sweepCount.Load() >= 1
	}, 2*time.Second, 10*time.Millisecond)

	a.Stop()
	require.GreaterOrEqual(t, sweepCount.Load(), int32(1))
}
```

Note: `sweepHook` is an unexported field on `ArchiverOpts` used for testing — it's called at the start of each sweep. This is a test seam; nil in production.

- [ ] **Step 2: Run tests, confirm they fail**

```bash
go test ./internal/archival/... -run "TestArchiver" -race -v
```
Expected: FAIL — `NewArchiver`, `ArchiverOpts`, `Archiver` undefined.

- [ ] **Step 3: Implement `internal/archival/archiver.go`**

```go
package archival

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"
)

// ArchiverOpts configures the archival sweep.
type ArchiverOpts struct {
	Horizon     time.Duration    // entries older than Now()-Horizon are deleted (read+unsaved)
	CacheAgeCap time.Duration    // cache files with fetched_at older than Now()-CacheAgeCap are unlinked
	Interval    time.Duration    // sweep cadence; default 24h
	CacheDir    string           // proxy cache root directory
	Now         func() time.Time // clock injection for tests; defaults to time.Now
	OnEvict     func(n int)      // optional; M12 wires tap_proxy_cache_evictions_total{reason="age_sweep"}
	sweepHook   func()           // test seam: called at start of each sweep, before passes run
}

// Archiver is the third concurrent concern in Tap alongside the HTTP server
// and the polling scheduler. It runs a daily two-pass archival sweep:
// 1. DB pass: delete read+unsaved entries older than Horizon, record tombstones.
// 2. FS pass: unlink proxy cache files older than CacheAgeCap.
type Archiver struct {
	db   *sql.DB
	opts ArchiverOpts

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	startOnce sync.Once
	stopOnce  sync.Once
}

// NewArchiver creates an archiver. Call Start() to begin the sweep ticker.
func NewArchiver(d *sql.DB, opts ArchiverOpts) *Archiver {
	if opts.Interval <= 0 {
		opts.Interval = 24 * time.Hour
	}
	if opts.Horizon <= 0 {
		opts.Horizon = 90 * 24 * time.Hour
	}
	if opts.CacheAgeCap <= 0 {
		opts.CacheAgeCap = 14 * 24 * time.Hour
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Archiver{
		db:     d,
		opts:   opts,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start launches the archival ticker goroutine. Idempotent.
func (a *Archiver) Start() {
	a.startOnce.Do(func() {
		a.wg.Add(1)
		go a.loop()
	})
}

// Stop cancels the ticker and waits for any in-progress sweep to complete.
func (a *Archiver) Stop() {
	a.stopOnce.Do(func() {
		a.cancel()
		a.wg.Wait()
	})
}

func (a *Archiver) loop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.opts.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.sweep()
		}
	}
}

func (a *Archiver) sweep() {
	if a.opts.sweepHook != nil {
		a.opts.sweepHook()
	}

	now := a.opts.Now()
	horizonUnix := now.Add(-a.opts.Horizon).Unix()
	ageCapUnix := now.Add(-a.opts.CacheAgeCap).Unix()
	start := time.Now()

	slog.Info("archival.sweep.start")

	deleted, tombstoned, err := dbPass(a.ctx, a.db, horizonUnix, now.Unix())
	if err != nil {
		slog.Warn("archival: db pass error", "err", err)
	}

	evicted, err := fsPass(a.opts.CacheDir, ageCapUnix, a.opts.OnEvict)
	if err != nil {
		slog.Warn("archival: fs pass error", "err", err)
	}

	slog.Info("archival.sweep.complete",
		"entries_deleted", deleted,
		"tombstones_written", tombstoned,
		"cache_files_evicted", evicted,
		"duration_ms", time.Since(start).Milliseconds(),
	)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/archival/... -race -v
```
Expected: PASS — all archiver and sweep tests green.

- [ ] **Step 5: Run full test suite**

```bash
make test
```
Expected: PASS — no regressions across the full project.

- [ ] **Step 6: Commit**

```bash
git add internal/archival/archiver.go internal/archival/archiver_test.go
git commit -m "M11: implement Archiver lifecycle with Start/Stop"
```

---

## Task 8: Wire `Archiver` into `cmd/tap/main.go`

**Skills to invoke:** `golang-context`

**Files:**
- Modify: `cmd/tap/main.go`

- [ ] **Step 1: Add flag declarations**

In `runServer()`, add three new flags alongside the existing `proxyCacheCap` and related flags:

```go
archiveInterval = flag.Duration("archive-interval",
    envOrDuration("TAP_ARCHIVE_INTERVAL", 24*time.Hour),
    "how often the archival sweep runs")
archiveHorizon = flag.Duration("archive-horizon",
    envOrDuration("TAP_ARCHIVE_HORIZON", 2160*time.Hour), // 90d
    "delete read+unsaved entries older than this")
cacheAgeCap = flag.Duration("cache-age-cap",
    envOrDuration("TAP_CACHE_AGE_CAP", 336*time.Hour), // 14d
    "unlink proxy cache files with fetched_at older than this")
```

- [ ] **Step 2: Wire the Archiver**

After `sched.Start()` and before the `<-ctx.Done()` block, add:

```go
archiver := archival.NewArchiver(d, archival.ArchiverOpts{
    Horizon:     *archiveHorizon,
    CacheAgeCap: *cacheAgeCap,
    Interval:    *archiveInterval,
    CacheDir:    cacheDir,
    OnEvict:     nil, // M12 wires tap_proxy_cache_evictions_total{reason="age_sweep"}
})
archiver.Start()
```

- [ ] **Step 3: Update shutdown ordering**

Replace:
```go
_ = srv.Shutdown(shutdownCtx)
sched.Stop()
```

With:
```go
_ = srv.Shutdown(shutdownCtx)
archiver.Stop()
sched.Stop()
```

This ensures: HTTP drains first → archiver sweep finishes → scheduler stops → DB closes.

- [ ] **Step 4: Add import**

Add to the import block:
```go
"github.com/bcrisp4/tap/internal/archival"
```

- [ ] **Step 5: Build and smoke-test**

```bash
make build
./bin/tap --archive-horizon=168h --cache-age-cap=72h &
sleep 1
kill %1
```
Expected: binary starts, logs `"no users in database"` or `"bootstrapped admin"`, exits cleanly on SIGTERM.

- [ ] **Step 6: Run full test suite**

```bash
make test
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/tap/main.go
git commit -m "M11: wire Archiver into runServer with archive-interval/horizon/cache-age-cap flags"
```

---

## Task 9: End-to-end test in `cmd/tap/main_test.go`

**Skills to invoke:** `superpowers:test-driven-development`, `golang-testing`

**Files:**
- Modify: `cmd/tap/main_test.go`

- [ ] **Step 1: Write end-to-end tests**

Add to `cmd/tap/main_test.go` (following the pattern of existing integration tests in that file):

```go
func TestArchival_EndToEnd(t *testing.T) {
	// Poll a fixture feed, mark an entry read, run a sweep, assert it's gone,
	// re-poll and assert the tombstoned entry does not reappear.
	t.Parallel()

	d := newTestDB(t) // uses db.Open(":memory:") + db.Migrate

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>E2E</title><id>urn:e2e</id><updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>Old Entry</title><id>urn:e2e:1</id>
    <link href="https://example.com/1"/>
    <updated>2024-01-01T00:00:00Z</updated>
    <content type="html">body text</content>
  </entry>
</feed>`)
	}))
	t.Cleanup(origin.Close)

	ctx := context.Background()
	proc := processor.New(sanitise.DefaultPolicy(), nil)

	// Subscribe
	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "e2e", FeedURL: origin.URL + "/feed", NextPoll: 0, Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	// Poll
	w := poll.NewWorker(d, http.DefaultClient, poll.WorkerOpts{Processor: proc})
	w.Run(ctx, db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"})

	entries, _, _, err := db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1, "entry must land after poll")

	// Mark read
	require.NoError(t, db.UpdateEntry(ctx, d, entries[0].ID, db.EntryUpdate{Read: boolPtr(true)}))

	// Run sweep with a horizon far in the future so the entry (published 2024) is archived
	horizonUnix := time.Now().Unix() // now is well past 2024-01-01
	deleted, tombstoned, err := archival.ExportedDBPass(ctx, d, horizonUnix, time.Now().Unix())
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, 1, tombstoned)

	// Entry must be gone
	entries, _, _, err = db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, entries, "archived entry must be gone")

	// Re-poll — tombstoned entry must NOT reappear
	w.Run(ctx, db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"})
	entries, _, _, err = db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, entries, "tombstoned entry must not reappear after re-poll")
}

func boolPtr(b bool) *bool { return &b }
```

Note: `archival.ExportedDBPass` is a thin exported wrapper around the unexported `dbPass` for test access. Add this to `sweep.go`:

```go
// ExportedDBPass is a test-only export of dbPass.
func ExportedDBPass(ctx context.Context, d *sql.DB, horizonUnix, nowUnix int64) (int, int, error) {
	return dbPass(ctx, d, horizonUnix, nowUnix)
}
```

- [ ] **Step 2: Add FS sweep end-to-end test**

Also add to `cmd/tap/main_test.go`:

```go
func TestArchival_FSPass_EndToEnd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write old and new cache files
	writeTestCacheFile(t, dir, "aabbccddeeff00112233445566778899aabbccddeeff001122", 100)   // old
	writeTestCacheFile(t, dir, "ffeeddccbbaa99887766554433221100ffeeddccbbaa998877", 99999) // new

	evicted, err := archival.ExportedFSPass(dir, 1000, nil)
	require.NoError(t, err)
	require.Equal(t, 1, evicted)

	bucket1 := filepath.Join(dir, "aa")
	require.NoFileExists(t, filepath.Join(bucket1, "aabbccddeeff00112233445566778899aabbccddeeff001122.bin"))
	require.FileExists(t, filepath.Join(dir, "ff", "ffeeddccbbaa99887766554433221100ffeeddccbbaa998877.bin"))
}

func writeTestCacheFile(t *testing.T, dir, hash string, fetchedAt int64) {
	t.Helper()
	bucket := filepath.Join(dir, hash[:2])
	require.NoError(t, os.MkdirAll(bucket, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bucket, hash+".bin"), []byte("data"), 0o644))
	meta, _ := json.Marshal(map[string]any{"content_type": "image/jpeg", "byte_count": 4, "fetched_at": fetchedAt})
	require.NoError(t, os.WriteFile(filepath.Join(bucket, hash+".meta"), meta, 0o644))
}
```

Also add `ExportedFSPass` to `sweep.go`:

```go
// ExportedFSPass is a test-only export of fsPass.
func ExportedFSPass(cacheDir string, ageCapUnix int64, onEvict func(int)) (int, error) {
	return fsPass(cacheDir, ageCapUnix, onEvict)
}
```

- [ ] **Step 3: Run the new tests**

```bash
go test ./cmd/tap/... -run "TestArchival" -race -v
```
Expected: PASS.

- [ ] **Step 4: Run full test suite**

```bash
make test
```
Expected: PASS — full green.

- [ ] **Step 5: Commit**

```bash
git add cmd/tap/main_test.go internal/archival/sweep.go
git commit -m "M11: end-to-end archival tests"
```

---

## Task 10: Final verification

**Skills to invoke:** `superpowers:verification-before-completion`

- [ ] **Step 1: Full test suite with race detector**

```bash
make test
```
Expected: `ok` for all packages, no race conditions, no failures.

- [ ] **Step 2: Static binary build**

```bash
make build
ls -lh bin/tap
```
Expected: binary exists, `file bin/tap` shows a statically linked ELF.

- [ ] **Step 3: Smoke-test against fresh data dir**

```bash
TAP_ADMIN_USERNAME=admin TAP_ADMIN_PASSWORD=password12345 ./bin/tap --data /tmp/tap-m11-test &
PID=$!
sleep 2
curl -sf http://127.0.0.1:8080/healthz && echo "OK"
kill $PID
rm -rf /tmp/tap-m11-test
```
Expected: `OK`, clean shutdown.

- [ ] **Step 4: Verify migration sequence**

```bash
ls internal/db/migrations/
```
Expected: `0001_initial.sql` through `0010_tombstones.sql` with no gaps.

- [ ] **Step 5: Commit final state if any cleanup happened**

```bash
git status
# If clean, nothing to do. If any minor fixes landed:
git add -p
git commit -m "M11: final cleanup"
```

---

## Spec coverage check

| Spec requirement | Task |
|---|---|
| `0010_tombstones.sql` migration | Task 1 |
| `db.InsertTombstones` | Task 2 |
| `db.IsTombstoned` | Task 2 |
| `ON CONFLICT DO NOTHING` idempotency | Task 2 |
| Cascade delete with subscription | Task 2 |
| `db.ListArchivable` | Task 3 |
| Tombstone consult in poll worker | Task 4 |
| `dbPass` chunking (1000-row transactions) | Task 5 |
| `dbPass` idempotency | Task 5 |
| FTS regression test | Task 6 |
| `fsPass` age-cap eviction | Task 6 |
| `fsPass` ENOENT graceful handling | Task 6 |
| `fsPass` orphan bin cleanup | Task 6 |
| `OnEvict` callback | Task 6 |
| `Archiver.Start()`/`Stop()` lifecycle | Task 7 |
| Clock injection via `Now func()` | Task 7 |
| `Stop()` waits for in-progress sweep | Task 7 |
| `archival.sweep.start` log event | Task 7 (archiver.go) |
| `archival.sweep.complete` log event with required attributes | Task 7 (archiver.go) |
| `--archive-interval`, `--archive-horizon`, `--cache-age-cap` flags | Task 8 |
| Shutdown ordering (HTTP → archiver → scheduler → DB) | Task 8 |
| End-to-end: poll → mark read → sweep → gone → re-poll → not reappear | Task 9 |
| End-to-end FS pass | Task 9 |
| Migration 0010 applies cleanly | Task 10 |
| `make build` static binary | Task 10 |
