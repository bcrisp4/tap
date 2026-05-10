# M11 — Archival + Tombstones Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a daily archival sweep that deletes read-and-unsaved entries older than a configurable horizon (recording tombstones in the same transaction), prunes old proxy cache files, and prevents re-published archived entries from reappearing as unread. Spec: `docs/specs/2026-05-10-m11-archival-tombstones.md`.

**Architecture:** A new `internal/archival` package owns an `Archiver` struct with `Start()`/`Stop()` lifecycle mirroring `Scheduler` (the third concurrent concern alongside HTTP and polling). The DB pass loops in 1000-row chunk transactions — `InsertTombstones` + `DELETE FROM entries` per chunk, idempotent via `ON CONFLICT DO NOTHING`. The FS pass walks the proxy cache directory and unlinks `.bin` + `.meta` pairs whose `fetched_at` sidecar field is older than the age cap; `ENOENT` is graceful on both sides. The poll worker gains a pre-commit tombstone consult (read-only, outside the commit transaction) so re-published entries are silently dropped. FTS stays in sync automatically via M9's per-row DELETE triggers — no explicit sync needed in M11.

**Tech Stack:** Go 1.25, `database/sql`, `modernc.org/sqlite`, `log/slog`, `sync.WaitGroup`, `context.WithCancel`, standard `os`/`filepath`/`encoding/json`, `github.com/stretchr/testify/require`.

**Cross-milestone dependencies:**
- **M9 migrations must be applied before the FTS regression test is meaningful.** M11 migration `0010_tombstones.sql` runs after M9's `0008_*` and `0009_*`. The FTS test is guarded: it skips FTS assertions if `entries_fts` does not exist.
- **M7 user_id:** No schema change needed in M11. Tombstones use `subscription_id` FK; the chain `tombstone → subscription → user` provides per-user scoping transitively once M7 lands.
- **M3 cache layout:** The FS pass reads `.meta` sidecars written by M3's `internal/proxy` package. The `cacheFileMeta` struct in `sweep.go` must match M3's JSON shape exactly (`content_type`, `etag`, `byte_count`, `fetched_at`).

---

## Skills and tools to apply

Always-on for every code-touching task:

- **`superpowers:test-driven-development`** — red/green/refactor on every behaviour-bearing change. Mandated by `docs/roadmap.md` §"Working cadence". Pure scaffolding (the migration SQL, README edit, flag declarations) is exempt; everything with branches, error handling, or state is in scope.
- **`superpowers:verification-before-completion`** — before marking a task done, actually run the test command listed in each task's verification step and confirm the output matches expected.

Reach for as needed:

- **`golang-database`** — chunk-transaction pattern (`BeginTx` / `InsertTombstones` / `DELETE WHERE id IN (...)` / `Commit`); `ON CONFLICT DO NOTHING` idempotency; `rows.Close()` discipline; `errors.Is(err, sql.ErrNoRows)` in `IsTombstoned`. SQLite note: `journal_mode(WAL)` (set in `db.Open`) means readers are not blocked during the sweep's write transactions.
- **`golang-concurrency`** — `Archiver.Start()`/`Stop()` uses `context.WithCancel` + `sync.WaitGroup` matching `Scheduler`'s pattern. `stopOnce sync.Once` prevents double-stop races. The ticker goroutine selects on `ctx.Done()` so `Stop()` unblocks it cleanly. `sweepHook func()` test seam for blocking mid-sweep in tests.
- **`golang-context`** — pass `ctx` through `dbPass` and all `db.*` calls. The `Archiver` derives its own `context.WithCancel(context.Background())` (not the server's context) so sweep lifecycle is independent of HTTP lifecycle; `Stop()` cancels it explicitly.
- **`golang-error-handling`** — FS pass errors are logged at `WARN` but do not abort the sweep (best-effort). DB pass errors propagate. Error strings lowercase, no trailing punctuation, `fmt.Errorf("...: %w", err)` wrapping.
- **`golang-testing`** + **`golang-stretchr-testify`** — match existing repo style (`require.NoError`, `require.Equal`, `require.Len`). `openTestDB(t)` helper local to `internal/archival` opens `:memory:` + `db.Migrate`. FS tests use `t.TempDir()`. Worker tombstone tests reuse the existing `newDB(t)` in `internal/poll/worker_test.go`.
- **`golang-naming`** — `dbPass`, `fsPass`, `chunkSize`, `cacheFileMeta` are unexported. `Archiver`, `ArchiverOpts`, `NewArchiver`, `ExportedDBPass`, `ExportedFSPass` are exported. `sweepHook` is an unexported field on `ArchiverOpts` for tests only.
- **`golang-modernize`** — Go 1.25: use `for i := range N` (range-over-int) in test loops; `errors.Is(err, os.ErrNotExist)` preferred over `os.IsNotExist(err)`.

MCP tools:

- **`context7` (`mcp__plugin_context7_context7__query-docs`)** — reach for this if `filepath.WalkDir` callback semantics or `os.Remove`/`os.ErrNotExist` behaviour needs verification. Also useful for confirming `slog.Info("key", "attr", val)` attribute API.

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `internal/db/migrations/0010_tombstones.sql` | **create** | `tombstones` table with `(subscription_id, entry_hash)` PK + `ON DELETE CASCADE` |
| `internal/db/tombstones.go` | **create** | `NewTombstone`, `InsertTombstones(ctx, tx, rows)`, `IsTombstoned(ctx, d, subID, hash)` |
| `internal/db/tombstones_test.go` | **create** | Insert+query roundtrip; `ON CONFLICT DO NOTHING`; cascade delete with subscription |
| `internal/db/entries.go` | modify | Add `ArchivableEntry` type + `ListArchivable(ctx, d, horizonUnix, limit)` |
| `internal/db/entries_test.go` | modify | `TestListArchivable` — eligible/ineligible criteria; limit enforcement |
| `internal/poll/worker.go` | modify | Tombstone consult loop before building `newEntries`; `db.IsTombstoned` per candidate |
| `internal/poll/worker_test.go` | modify | Tombstone hit → not inserted; miss → inserted; mixed (3 entries, 1 tombstoned) |
| `internal/archival/sweep.go` | **create** | `dbPass`, `fsPass`, `cacheFileMeta`, `buildDeleteByIDs`, `ExportedDBPass`, `ExportedFSPass` |
| `internal/archival/sweep_test.go` | **create** | DB pass + FS pass correctness; FTS regression test |
| `internal/archival/archiver.go` | **create** | `ArchiverOpts`, `Archiver`, `NewArchiver`, `Start`, `Stop`, `loop`, `sweep` |
| `internal/archival/archiver_test.go` | **create** | Start+Stop clean exit; Stop waits for in-progress sweep; sweep fires |
| `cmd/tap/main.go` | modify | Three new flags; wire `archival.NewArchiver`; `archiver.Stop()` before `sched.Stop()` |
| `cmd/tap/main_test.go` | modify | End-to-end: poll → mark read → sweep → gone → re-poll → still gone; FS eviction |

---

## Task 1: Schema migration — `0010_tombstones.sql`

**Files:**
- Create: `internal/db/migrations/0010_tombstones.sql`

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

```bash
go test ./internal/db/... -run TestMigrate -race -v
```

Expected: PASS. The existing `TestMigrate` test calls `db.Open(":memory:")` + `db.Migrate()` — the new migration is picked up in lexical order automatically.

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations/0010_tombstones.sql
git commit -m "M11: add 0010_tombstones migration"
```

---

## Task 2: DB layer — `tombstones.go`

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

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "feed", FeedURL: "https://example.com/feed",
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
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "feed", FeedURL: "https://conflict.example/feed",
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
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "feed", FeedURL: "https://cascade.example/feed",
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
			return fmt.Errorf("insert tombstone (sub=%d hash=%s): %w",
				r.SubscriptionID, r.EntryHash, err)
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
		return false, fmt.Errorf("check tombstone (sub=%d hash=%s): %w",
			subscriptionID, entryHash, err)
	}
	return true, nil
}
```

- [ ] **Step 4: Run tests, confirm they pass**

```bash
go test ./internal/db/... -run TestTombstones -race -v
```

Expected: PASS.

- [ ] **Step 5: Run full `internal/db` suite**

```bash
go test ./internal/db/... -race
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/db/tombstones.go internal/db/tombstones_test.go
git commit -m "M11: add db.InsertTombstones and db.IsTombstoned"
```

---

## Task 3: DB layer — `ListArchivable`

**Files:**
- Modify: `internal/db/entries.go`
- Modify: `internal/db/entries_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/db/entries_test.go`:

```go
func TestListArchivable_EligibleCriteria(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "feed", FeedURL: "https://archivable.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	insertE := func(hash string, publishedAt int64, read, saved bool) {
		t.Helper()
		_, err := d.ExecContext(ctx, `
			INSERT INTO entries (subscription_id, hash, title, author, url, content,
			                     published_at, fetched_at, read, saved, extract_failed)
			VALUES (?, ?, 'T', NULL, 'https://x', 'c', ?, 0, ?, ?, 0)
		`, subID, hash, publishedAt, boolToInt(read), boolToInt(saved))
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

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "f", FeedURL: "https://limit.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	for i := range 5 {
		_, err := d.ExecContext(ctx, `
			INSERT INTO entries (subscription_id, hash, title, author, url, content,
			                     published_at, fetched_at, read, saved, extract_failed)
			VALUES (?, ?, 'T', NULL, 'https://x', 'c', 100, 0, 1, 0, 0)
		`, subID, fmt.Sprintf("h%d", i))
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

- [ ] **Step 3: Add to `internal/db/entries.go`**

Append after the existing functions:

```go
// ArchivableEntry is the minimal row shape returned by ListArchivable.
type ArchivableEntry struct {
	ID             int64
	SubscriptionID int64
	Hash           string
}

// ListArchivable returns up to limit entries eligible for archival:
// read=1, saved=0, published_at < horizonUnix. Ordered oldest-first.
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

- [ ] **Step 5: Run full `internal/db` suite**

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

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, sampleAtom) // one entry, id urn:sample:1
	}))
	t.Cleanup(origin.Close)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "s", FeedURL: origin.URL + "/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	sub := db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})

	// First poll — entry lands
	w.Run(ctx, sub)
	entries, _, _, err := db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Tombstone then delete the entry
	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, db.InsertTombstones(ctx, tx, []db.NewTombstone{
		{SubscriptionID: subID, EntryHash: entries[0].Hash, DeletedAt: 9999},
	}))
	require.NoError(t, tx.Commit())
	_, err = d.ExecContext(ctx, "DELETE FROM entries WHERE subscription_id = ?", subID)
	require.NoError(t, err)

	// Second poll — must not reinsert
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

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(ctx, db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"})

	entries, _, _, err := db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1, "non-tombstoned entry must be inserted normally")
}

func TestWorker_MixedTombstones(t *testing.T) {
	t.Parallel()
	d := newDB(t)
	ctx := context.Background()

	const threeAtom = `<?xml version="1.0" encoding="UTF-8"?>
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
		fmt.Fprint(w, threeAtom)
	}))
	t.Cleanup(origin.Close)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "s", FeedURL: origin.URL + "/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	sub := db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"}
	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})

	w.Run(ctx, sub)
	entries, _, _, err := db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 3)

	var tombstoneHash string
	for _, e := range entries {
		if e.URL == "https://x/2" {
			tombstoneHash = e.Hash
		}
	}
	require.NotEmpty(t, tombstoneHash)

	tx, err := d.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, db.InsertTombstones(ctx, tx, []db.NewTombstone{
		{SubscriptionID: subID, EntryHash: tombstoneHash, DeletedAt: 1000},
	}))
	require.NoError(t, tx.Commit())
	_, err = d.ExecContext(ctx, "DELETE FROM entries WHERE subscription_id = ?", subID)
	require.NoError(t, err)

	w.Run(ctx, sub)
	entries, _, _, err = db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	for _, e := range entries {
		require.NotEqual(t, tombstoneHash, e.Hash)
	}
}
```

- [ ] **Step 2: Run tests, confirm they fail**

```bash
go test ./internal/poll/... -run "TestWorker_Tombstone|TestWorker_Mixed" -race -v
```

Expected: FAIL — tombstoned entries still reappear.

- [ ] **Step 3: Add tombstone consult to `internal/poll/worker.go`**

In `Worker.Run`, find `newEntries := make([]db.NewEntry, 0, len(pendings))` and replace the entire loop that builds it with:

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
	// Read-only point lookups; runs outside the commit transaction.
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
			continue
		}
		newEntries = append(newEntries, e)
	}
```

- [ ] **Step 4: Run tests, confirm they pass**

```bash
go test ./internal/poll/... -run "TestWorker_Tombstone|TestWorker_Mixed" -race -v
```

Expected: PASS.

- [ ] **Step 5: Run full `internal/poll` suite**

```bash
go test ./internal/poll/... -race
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/poll/worker.go internal/poll/worker_test.go
git commit -m "M11: poll worker consults tombstones before insert"
```

---

## Task 5: `internal/archival/sweep.go` — DB pass + FS pass

**Files:**
- Create: `internal/archival/sweep.go`
- Create: `internal/archival/sweep_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/archival/sweep_test.go`:

```go
package archival

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

func insertSub(t *testing.T, d *sql.DB) int64 {
	t.Helper()
	id, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "feed", FeedURL: fmt.Sprintf("https://%d.example/feed", time.Now().UnixNano()),
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
	_, err := d.ExecContext(context.Background(), `
		INSERT INTO entries (subscription_id, hash, title, author, url, content,
		                     published_at, fetched_at, read, saved, extract_failed)
		VALUES (?, ?, 'T', NULL, 'https://x', 'content', ?, 0, ?, ?, 0)
	`, subID, hash, publishedAt, ri, si)
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

// suppress unused import error when errors package is only used in sweep.go
var _ = errors.New
```

- [ ] **Step 2: Run tests, confirm they fail**

```bash
go test ./internal/archival/... -race -v
```

Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement `internal/archival/sweep.go`**

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

	"github.com/bcrisp4/tap/internal/db"
)

const chunkSize = 1000

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
		tbs := make([]db.NewTombstone, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
			tbs[i] = db.NewTombstone{SubscriptionID: r.SubscriptionID, EntryHash: r.Hash, DeletedAt: nowUnix}
		}

		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return deleted, tombstoned, fmt.Errorf("begin tx: %w", err)
		}

		if err := db.InsertTombstones(ctx, tx, tbs); err != nil {
			_ = tx.Rollback()
			return deleted, tombstoned, err
		}

		res, err := tx.ExecContext(ctx, buildDeleteByIDs(ids))
		if err != nil {
			_ = tx.Rollback()
			return deleted, tombstoned, fmt.Errorf("delete entries chunk: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return deleted, tombstoned, fmt.Errorf("commit chunk: %w", err)
		}

		n, _ := res.RowsAffected()
		deleted += int(n)
		tombstoned += len(tbs)
	}
}

// buildDeleteByIDs constructs a DELETE for a slice of internal int64 IDs.
// IDs come from our own DB rows, not user input, so direct interpolation is safe.
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
// Must match M3's sidecar shape exactly.
type cacheFileMeta struct {
	ContentType string `json:"content_type"`
	ETag        string `json:"etag,omitempty"`
	ByteCount   int64  `json:"byte_count"`
	FetchedAt   int64  `json:"fetched_at"`
}

func fsPass(cacheDir string, ageCapUnix int64, onEvict func(int)) (evicted int, err error) {
	type candidate struct{ binPath, metaPath string }
	var candidates []candidate
	var orphanBins []string
	metaBases := map[string]bool{}

	walkErr := filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, werr error) error {
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
		if ext != ".meta" {
			return nil
		}
		metaBases[base] = true
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			if !errors.Is(rerr, os.ErrNotExist) {
				slog.Warn("archival: read meta sidecar", "path", path, "err", rerr)
			}
			return nil
		}
		var meta cacheFileMeta
		if jerr := json.Unmarshal(data, &meta); jerr != nil {
			slog.Warn("archival: decode meta sidecar", "path", path, "err", jerr)
			return nil
		}
		if meta.FetchedAt < ageCapUnix {
			candidates = append(candidates, candidate{binPath: base + ".bin", metaPath: path})
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, os.ErrNotExist) {
		return 0, fmt.Errorf("walk cache dir: %w", walkErr)
	}

	_ = filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".bin" {
			return nil
		}
		base := path[:len(path)-len(".bin")]
		if !metaBases[base] {
			orphanBins = append(orphanBins, path)
		}
		return nil
	})

	for _, c := range candidates {
		if rerr := os.Remove(c.binPath); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			slog.Warn("archival: remove bin", "path", c.binPath, "err", rerr)
		}
		if rerr := os.Remove(c.metaPath); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			slog.Warn("archival: remove meta", "path", c.metaPath, "err", rerr)
		}
		evicted++
	}
	for _, path := range orphanBins {
		if rerr := os.Remove(path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			slog.Warn("archival: remove orphan bin", "path", path, "err", rerr)
		}
		evicted++
	}

	if onEvict != nil && evicted > 0 {
		onEvict(evicted)
	}
	return evicted, nil
}

// ExportedDBPass is a test-only shim for cmd/tap/main_test.go.
func ExportedDBPass(ctx context.Context, d *sql.DB, horizonUnix, nowUnix int64) (int, int, error) {
	return dbPass(ctx, d, horizonUnix, nowUnix)
}

// ExportedFSPass is a test-only shim for cmd/tap/main_test.go.
func ExportedFSPass(cacheDir string, ageCapUnix int64, onEvict func(int)) (int, error) {
	return fsPass(cacheDir, ageCapUnix, onEvict)
}
```

- [ ] **Step 4: Run tests, confirm they pass**

```bash
go test ./internal/archival/... -race -v
```

Expected: PASS — all DB pass, FS pass, and FTS regression tests green.

- [ ] **Step 5: Commit**

```bash
git add internal/archival/sweep.go internal/archival/sweep_test.go
git commit -m "M11: implement archival dbPass and fsPass"
```

---

## Task 6: `internal/archival/archiver.go` — lifecycle

**Files:**
- Create: `internal/archival/archiver.go`
- Create: `internal/archival/archiver_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/archival/archiver_test.go`:

```go
package archival

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestArchiver_StartStop_NoSweep(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	a := NewArchiver(d, ArchiverOpts{
		Interval: 24 * time.Hour,
		CacheDir: t.TempDir(),
	})
	a.Start()
	a.Stop()
}

func TestArchiver_Stop_WaitsForInProgressSweep(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	gate := make(chan struct{})
	var started atomic.Bool

	a := NewArchiver(d, ArchiverOpts{
		Interval:  1 * time.Millisecond,
		CacheDir:  t.TempDir(),
		sweepHook: func() { started.Store(true); <-gate },
	})
	a.Start()

	require.Eventually(t, func() bool { return started.Load() }, 2*time.Second, 5*time.Millisecond)

	stopDone := make(chan struct{})
	go func() { a.Stop(); close(stopDone) }()

	select {
	case <-stopDone:
		t.Fatal("Stop() returned before sweep finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(gate)
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return after sweep unblocked")
	}
}

func TestArchiver_SweepFires(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	var count atomic.Int32
	a := NewArchiver(d, ArchiverOpts{
		Interval:  1 * time.Millisecond,
		CacheDir:  t.TempDir(),
		sweepHook: func() { count.Add(1) },
	})
	a.Start()
	require.Eventually(t, func() bool { return count.Load() >= 1 }, 2*time.Second, 5*time.Millisecond)
	a.Stop()
	require.GreaterOrEqual(t, count.Load(), int32(1))
}
```

- [ ] **Step 2: Run tests, confirm they fail**

```bash
go test ./internal/archival/... -run TestArchiver -race -v
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
	Horizon     time.Duration    // entries older than Now()-Horizon are deleted; default 90d
	CacheAgeCap time.Duration    // cache files older than Now()-CacheAgeCap are unlinked; default 14d
	Interval    time.Duration    // sweep cadence; default 24h
	CacheDir    string           // proxy cache root directory
	Now         func() time.Time // clock injection; defaults to time.Now
	OnEvict     func(n int)      // optional; M12 wires tap_proxy_cache_evictions_total{reason="age_sweep"}
	sweepHook   func()           // test seam: called at sweep entry before passes run
}

// Archiver is the third concurrent concern in Tap: daily sweep of old entries
// and old cache files.
type Archiver struct {
	db   *sql.DB
	opts ArchiverOpts

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	startOnce sync.Once
	stopOnce  sync.Once
}

// NewArchiver creates an Archiver. Call Start() to begin the sweep ticker.
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
	return &Archiver{db: d, opts: opts, ctx: ctx, cancel: cancel}
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

- [ ] **Step 4: Run all archival tests**

```bash
go test ./internal/archival/... -race -v
```

Expected: PASS — all tests green.

- [ ] **Step 5: Run full test suite**

```bash
make test
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/archival/archiver.go internal/archival/archiver_test.go
git commit -m "M11: implement Archiver lifecycle with Start/Stop"
```

---

## Task 7: Wire `Archiver` into `cmd/tap/main.go`

**Files:**
- Modify: `cmd/tap/main.go`

- [ ] **Step 1: Add three new flag declarations**

In `runServer()`, after the `proxyCacheCap` / `proxyBodyCap` flags, add:

```go
archiveInterval = flag.Duration("archive-interval",
    envOrDuration("TAP_ARCHIVE_INTERVAL", 24*time.Hour),
    "how often the archival sweep runs")
archiveHorizon = flag.Duration("archive-horizon",
    envOrDuration("TAP_ARCHIVE_HORIZON", 2160*time.Hour), // 90d
    "delete read+unsaved entries older than this horizon")
cacheAgeCap = flag.Duration("cache-age-cap",
    envOrDuration("TAP_CACHE_AGE_CAP", 336*time.Hour), // 14d
    "unlink proxy cache files with fetched_at older than this age")
```

- [ ] **Step 2: Wire the Archiver after `sched.Start()`**

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

Change the shutdown block from:

```go
_ = srv.Shutdown(shutdownCtx)
sched.Stop()
```

to:

```go
_ = srv.Shutdown(shutdownCtx)
archiver.Stop()
sched.Stop()
```

Ordering is load-bearing: HTTP drains first → archiver finishes → scheduler stops → `d.Close()`.

- [ ] **Step 4: Add import**

```go
"github.com/bcrisp4/tap/internal/archival"
```

- [ ] **Step 5: Build and smoke-test**

```bash
make build
TAP_ADMIN_USERNAME=admin TAP_ADMIN_PASSWORD=password12345 ./bin/tap --data /tmp/tap-m11-smoke &
PID=$!
sleep 1
curl -sf http://127.0.0.1:8080/healthz && echo "OK"
kill $PID
rm -rf /tmp/tap-m11-smoke
```

Expected: `OK`, clean exit.

- [ ] **Step 6: Run full test suite**

```bash
make test
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/tap/main.go
git commit -m "M11: wire Archiver into runServer with archive flags"
```

---

## Task 8: End-to-end tests in `cmd/tap/main_test.go`

**Files:**
- Modify: `cmd/tap/main_test.go`

- [ ] **Step 1: Add end-to-end DB sweep test**

Add to `cmd/tap/main_test.go` (check existing imports; add any missing ones from `encoding/json`, `os`, `path/filepath`, `github.com/bcrisp4/tap/internal/archival`, `github.com/bcrisp4/tap/internal/poll`, `github.com/bcrisp4/tap/internal/processor`, `github.com/bcrisp4/tap/internal/sanitise`):

```go
func TestArchival_DBSweep_EndToEnd(t *testing.T) {
	t.Parallel()
	d := newTestDB(t) // opens :memory: + Migrate

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
	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "e2e", FeedURL: origin.URL + "/feed", NextPoll: 0, Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	w := poll.NewWorker(d, http.DefaultClient, poll.WorkerOpts{Processor: proc})
	w.Run(ctx, db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"})

	entries, _, _, err := db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, db.UpdateEntry(ctx, d, entries[0].ID, db.EntryUpdate{Read: boolPtr(true)}))

	// published_at is 2024-01-01; horizon = now is well past it
	deleted, tombstoned, err := archival.ExportedDBPass(ctx, d, time.Now().Unix(), time.Now().Unix())
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, 1, tombstoned)

	entries, _, _, err = db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, entries)

	// Re-poll — tombstoned entry must not reappear
	w.Run(ctx, db.DueSubscription{ID: subID, FeedURL: origin.URL + "/feed"})
	entries, _, _, err = db.ListEntries(ctx, d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, entries, "tombstoned entry must not reappear")
}

func boolPtr(b bool) *bool { return &b }

func TestArchival_FSSweep_EndToEnd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeE2ECacheFile(t, dir, "aabbccddeeff00112233445566778899aabbccddeeff001122", 100)
	writeE2ECacheFile(t, dir, "ffeeddccbbaa99887766554433221100ffeeddccbbaa998877", 99999)

	evicted, err := archival.ExportedFSPass(dir, 1000, nil)
	require.NoError(t, err)
	require.Equal(t, 1, evicted)
	require.NoFileExists(t, filepath.Join(dir, "aa", "aabbccddeeff00112233445566778899aabbccddeeff001122.bin"))
	require.FileExists(t, filepath.Join(dir, "ff", "ffeeddccbbaa99887766554433221100ffeeddccbbaa998877.bin"))
}

func writeE2ECacheFile(t *testing.T, dir, hash string, fetchedAt int64) {
	t.Helper()
	bucket := filepath.Join(dir, hash[:2])
	require.NoError(t, os.MkdirAll(bucket, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bucket, hash+".bin"), []byte("data"), 0o644))
	meta, _ := json.Marshal(map[string]any{
		"content_type": "image/jpeg", "byte_count": 4, "fetched_at": fetchedAt,
	})
	require.NoError(t, os.WriteFile(filepath.Join(bucket, hash+".meta"), meta, 0o644))
}
```

- [ ] **Step 2: Run the new tests**

```bash
go test ./cmd/tap/... -run "TestArchival" -race -v
```

Expected: PASS.

- [ ] **Step 3: Run full test suite**

```bash
make test
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/tap/main_test.go
git commit -m "M11: end-to-end archival tests"
```

---

## Task 9: README / CLAUDE.md update

**Files:**
- Modify: `CLAUDE.md` (trust-posture section)

- [ ] **Step 1: Find the M6 trust-posture paragraph**

```bash
grep -n "Authentication (M6)\|M6 in progress\|M6.*auth" CLAUDE.md | head -5
```

- [ ] **Step 2: Add M11 paragraph after the M6 block**

> **Archival + tombstones (M11).** A daily sweep deletes read-and-unsaved entries older than `--archive-horizon` (default 90d), recording tombstones so re-published entries do not resurface as unread. A second daily pass unlinks proxy cache files older than `--cache-age-cap` (default 14d). Both bounds are configurable via flags or environment variables. The tombstone table (`tombstones`) is small and grows slowly; tombstones are permanent by design — the dedup guarantee requires durability. The archival sweep is the third concurrent concern alongside the HTTP server and polling pipeline; it starts after migrations and stops cleanly on shutdown.

Also update the "M6 in progress" status line to "M11 in progress" (the spec at the top of CLAUDE.md tracks the current milestone).

And add the upgrade note wherever prior upgrade notes appear:

> Migration 0010 adds the `tombstones` table. Existing M10 databases migrate cleanly. Entries already in the database are subject to archival on the next sweep if they meet the horizon criterion.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "M11: update CLAUDE.md trust posture and milestone status"
```

---

## Task 10: Final verification

- [ ] **Step 1: Full test suite with race detector**

```bash
make test
```

Expected: `ok` for every package, zero failures, zero races.

- [ ] **Step 2: Static binary build**

```bash
make build
file bin/tap
```

Expected: `ELF 64-bit LSB executable ... statically linked`.

- [ ] **Step 3: Verify migration sequence**

```bash
ls internal/db/migrations/
```

Expected: `0001_initial.sql` through `0010_tombstones.sql`, no gaps.

- [ ] **Step 4: Smoke-test against fresh data dir**

```bash
TAP_ADMIN_USERNAME=admin TAP_ADMIN_PASSWORD=password12345 ./bin/tap --data /tmp/tap-m11-final &
PID=$!
sleep 1
curl -sf http://127.0.0.1:8080/healthz && echo "OK"
kill $PID
rm -rf /tmp/tap-m11-final
```

Expected: `OK`, clean exit.

- [ ] **Step 5: Run `/simplify`**

Per global CLAUDE.md — once all tasks are complete, invoke `/simplify` to review changed code for reuse, quality, and efficiency and fix any issues found.

---

## Spec coverage

| Spec requirement | Task |
|---|---|
| `0010_tombstones.sql` with `ON DELETE CASCADE` | Task 1 |
| `db.InsertTombstones` (bulk, in tx, idempotent) | Task 2 |
| `db.IsTombstoned` (point lookup) | Task 2 |
| `ON CONFLICT DO NOTHING` — first `deleted_at` preserved | Task 2 |
| Cascade delete when subscription removed | Task 2 |
| `db.ListArchivable` with criteria + limit | Task 3 |
| Worker tombstone consult before insert | Task 4 |
| Tombstone hit → not inserted | Task 4 |
| Tombstone miss → inserted | Task 4 |
| Mixed (1 of 3 tombstoned) | Task 4 |
| `dbPass` eligible deleted + tombstoned in same tx | Task 5 |
| `dbPass` saved/unread/too-new retained | Task 5 |
| `dbPass` idempotent | Task 5 |
| `dbPass` chunking (2500 → 3 transactions) | Task 5 |
| `dbPass` pre-existing tombstone → no error | Task 5 |
| FTS regression test | Task 5 |
| `fsPass` old files evicted | Task 5 |
| `fsPass` new files retained | Task 5 |
| `fsPass` missing `.bin` → graceful | Task 5 |
| `fsPass` orphan `.bin` → removed | Task 5 |
| `fsPass` empty/non-existent dir | Task 5 |
| `fsPass` `OnEvict` callback | Task 5 |
| `Archiver.Start()`/`Stop()` lifecycle | Task 6 |
| `Stop()` waits for in-progress sweep | Task 6 |
| `sweepHook` test seam | Task 6 |
| `archival.sweep.start` log event | Task 6 |
| `archival.sweep.complete` with all 4 attributes | Task 6 |
| `OnEvict` nil in M11 wire-up | Task 7 |
| `--archive-interval`, `--archive-horizon`, `--cache-age-cap` | Task 7 |
| Shutdown ordering: HTTP → archiver → scheduler → DB | Task 7 |
| End-to-end DB: poll → mark read → sweep → gone → re-poll → still gone | Task 8 |
| End-to-end FS: old evicted, new retained | Task 8 |
| Migration 0010 applies cleanly | Task 10 |
| `make build` static binary | Task 10 |
