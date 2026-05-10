# M11 — Archival + tombstones

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md). M11 is the eleventh of twelve milestones — see [`../roadmap.md`](../roadmap.md). M1–M6 have shipped; M7–M10 are in progress.

Through M10, every entry ever polled remains in the database indefinitely. A Tap instance running for a year against a few dozen active feeds will accumulate tens of thousands of rows, most of them read and forgotten. The media cache likewise grows without bound between M3's inline LRU eviction events (which only fire under cap pressure, not at rest). Concept §6.14 specifies the closure: a daily archival sweep that deletes old read-and-unsaved entries (recording tombstones), then prunes cache files older than a configurable age cap.

M11 also closes the tombstone dedup loop described in concept §6.9. Without tombstones, a feed that republishes an old entry — backdate, Atom re-export, feed migration — would resurface that entry as unread. The polling commit path is extended to consult tombstones before insert, silently skipping any entry whose hash has been tombstoned.

## Goal

After M11, a Tap instance running for a year with default settings keeps at most 90 days of read-and-unsaved entries per subscription, and media cache files older than 14 days are deleted daily. Re-published entries that have been archived never reappear in the unread list. The database and cache directory stay bounded without operator intervention.

## In scope

### New package: `internal/archival`

```
internal/archival/
  archiver.go       # Archiver struct — lifecycle, ticker, sweep orchestration
  archiver_test.go
  sweep.go          # dbPass, fsPass — the two sweep implementations
  sweep_test.go
```

`Archiver` is a self-contained concurrent concern with `Start()`/`Stop()` lifecycle mirroring `Scheduler`. It owns an internal ticker that fires at a configurable interval (default 24h) and calls `sweep()` on each tick.

```go
type ArchiverOpts struct {
    Horizon     time.Duration   // entries older than this (read+unsaved) are deleted; default 90d
    CacheAgeCap time.Duration   // cache files older than this are unlinked; default 14d
    Interval    time.Duration   // sweep cadence; default 24h
    CacheDir    string          // proxy cache root, e.g. ${TAP_DATA_DIR}/cache
    Now         func() time.Time // clock injection for tests; defaults to time.Now
}

func NewArchiver(db *sql.DB, opts ArchiverOpts) *Archiver
func (a *Archiver) Start()
func (a *Archiver) Stop()  // waits for any in-progress sweep before returning
```

`Start` launches a goroutine that ticks at `opts.Interval`. `Stop` cancels the context and waits on a `sync.WaitGroup` — same shutdown pattern as `Scheduler.Stop`. No sweep is attempted on shutdown; the goroutine simply exits.

`sweep()` runs two passes sequentially:

1. **DB pass** (`dbPass`) — deletes eligible entries and writes tombstones.
2. **FS pass** (`fsPass`) — unlinks old cache files.

Both passes are logged at `INFO` level on completion (entries deleted, tombstones written, cache files unlinked, duration). Errors in the FS pass are logged at `WARN` but do not abort the sweep or affect the DB pass result.

### Schema migration

`internal/db/migrations/0006_tombstones.sql`:

```sql
CREATE TABLE tombstones (
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    entry_hash      TEXT    NOT NULL,
    deleted_at      INTEGER NOT NULL,
    PRIMARY KEY (subscription_id, entry_hash)
);
CREATE INDEX idx_tombstones_subscription ON tombstones(subscription_id);
```

`ON DELETE CASCADE` means removing a subscription automatically removes its tombstones. No `user_id` column — once M7 adds `user_id` to `subscriptions`, the FK chain `tombstone → subscription → user` provides user-scoping transitively without any schema change to this table. The `(subscription_id, entry_hash)` primary key enforces uniqueness and serves as the lookup index for the poll commit path.

Tombstones survive forever. There is no TTL. The table is cheap (two integers + a timestamp per row), and the dedup guarantee is only meaningful if tombstones are durable. A re-published entry five years later is suppressed; this is the correct behaviour per concept §6.9.

### `db` layer extensions

New file `internal/db/tombstones.go`:

```go
// InsertTombstones bulk-inserts tombstone rows within the caller's transaction.
// ON CONFLICT DO NOTHING — if a tombstone already exists for (sub, hash), it is kept as-is.
func InsertTombstones(ctx context.Context, tx *sql.Tx, rows []NewTombstone) error

// IsTombstoned returns true if (subscriptionID, entryHash) exists in tombstones.
// Used by the poll commit path for per-entry consult.
func IsTombstoned(ctx context.Context, d *sql.DB, subscriptionID int64, entryHash string) (bool, error)

type NewTombstone struct {
    SubscriptionID int64
    EntryHash      string
    DeletedAt      int64 // unix seconds
}
```

`InsertTombstones` accepts a `*sql.Tx` because it always runs inside the DB pass's chunk transaction. `IsTombstoned` accepts a `*sql.DB` (the poll worker has no transaction open when it consults tombstones — the consult happens before the commit transaction begins).

New function in `internal/db/entries.go`:

```go
// ListArchivable returns up to limit entry rows eligible for archival:
// read = 1, saved = 0, published_at < horizonUnix.
// Returns (id, subscription_id, hash) — the minimum needed to write tombstones and delete.
func ListArchivable(ctx context.Context, d *sql.DB, horizonUnix int64, limit int) ([]ArchivableEntry, error)

type ArchivableEntry struct {
    ID             int64
    SubscriptionID int64
    Hash           string
}
```

### DB pass — chunked sweep

`dbPass` loops until no eligible rows remain:

```
for {
    rows = ListArchivable(ctx, db, horizonUnix, 1000)
    if len(rows) == 0 { break }

    BEGIN tx
    InsertTombstones(ctx, tx, toTombstones(rows))
    DELETE FROM entries WHERE id IN (ids...)
    COMMIT

    totalDeleted += len(rows)
    totalTombstoned += len(rows)
}
```

Each chunk is one transaction — concept §6.15's single-transaction commit invariant extends to the sweep. Chunking to 1000 rows avoids a long-held write lock on large tables. SQLite FTS5 triggers on `entries` (added by M9's migration) fire per-row on the DELETE, keeping the FTS index in sync automatically. M11 adds no explicit FTS sync call.

`ON CONFLICT DO NOTHING` in `InsertTombstones` is idempotent — if the sweep crashes mid-chunk after writing tombstones but before committing the DELETE, the next run re-selects the same rows (not yet deleted), re-inserts tombstones (no-ops on conflict), and deletes them. No double-tombstone, no lost entries.

### FS pass — cache eviction

`fsPass` walks `${CacheDir}/` reading `.meta` sidecars. For each `<hash>.meta` whose `fetched_at` field is older than `CacheAgeCap`:

1. Unlink `<hash>.bin`. `ENOENT` is not an error (already evicted by M3's inline LRU).
2. Unlink `<hash>.meta`. `ENOENT` is not an error.

If `.meta` is missing but `.bin` exists (orphan from a crashed write): unlink `.bin`. This is detected by walking for `.bin` files whose corresponding `.meta` is absent after the main walk completes.

POSIX `unlink` is safe while a proxy handler holds the `.bin` open — the open file descriptor survives until closed; the directory entry is removed immediately. The FS pass does not hold any lock during the walk; it is not transactional.

The `.meta` sidecar format is defined by M3:

```json
{
  "content_type": "image/jpeg",
  "etag": "\"abc123\"",
  "byte_count": 12345,
  "fetched_at": 1746754800
}
```

M11 reads `fetched_at` (unix seconds) for the age check. No other fields are touched.

### Tombstone consult on insert

In `internal/poll/worker.go`, the per-poll commit path gains a tombstone check before inserting each entry. The check runs outside the commit transaction (it is a read — no write lock needed):

```go
for _, item := range newEntries {
    tombstoned, err := db.IsTombstoned(ctx, d, sub.ID, item.Hash)
    if err != nil { /* log warn, skip entry */ }
    if tombstoned {
        continue // silently drop — concept §6.9
    }
    toInsert = append(toInsert, item)
}
// existing single-transaction commit with toInsert
```

The tombstone consult adds at most N point-lookups per poll (where N = new entries on this fetch, typically < 10). Each lookup is a primary-key seek on `(subscription_id, entry_hash)` — sub-millisecond at any realistic table size.

The single-transaction commit per poll (concept §6.15) is preserved: the consult is outside the transaction, the insert of surviving entries is inside it, exactly as before.

### Lifecycle wiring in `cmd/tap/main.go`

`Archiver` starts after `Scheduler` and stops before it:

```go
archiver := archival.NewArchiver(d, archival.ArchiverOpts{
    Horizon:     *archiveHorizon,
    CacheAgeCap: *cacheAgeCap,
    Interval:    *archiveInterval,
    CacheDir:    cacheDir,
})
archiver.Start()

// graceful shutdown (reverse order):
srv.Shutdown(ctx)
archiver.Stop()
sched.Stop()
db.Close()
```

Stopping the archiver before the scheduler ensures a sweep in progress drains cleanly before the DB closes. HTTP is drained first so no new proxy reads race the FS pass.

### Configuration knobs

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--archive-interval` | `TAP_ARCHIVE_INTERVAL` | `24h` | How often the sweep runs. `flag.Duration`. |
| `--archive-horizon` | `TAP_ARCHIVE_HORIZON` | `2160h` (90d) | Entries read+unsaved older than this are deleted. `flag.Duration`. |
| `--cache-age-cap` | `TAP_CACHE_AGE_CAP` | `336h` (14d) | Cache files with `fetched_at` older than this are unlinked. `flag.Duration`. |

`--proxy-cache-dir` is already defined by M3; `Archiver` receives it as `CacheDir` from the same resolved value used by the proxy handler. No new flag needed for the cache directory.

`--archive-horizon` and `--cache-age-cap` are expressed in hours as `flag.Duration` values, consistent with M6's session TTL flags. Operators can express them naturally: `--archive-horizon=720h` (30d), `--cache-age-cap=168h` (7d).

### README update

After the M6 trust-posture paragraph:

> **Archival + tombstones (M11).** A daily sweep deletes read-and-unsaved entries older than `--archive-horizon` (default 90d), recording tombstones so re-published entries do not resurface as unread. A second daily pass unlinks proxy cache files older than `--cache-age-cap` (default 14d). Both bounds are configurable via flags or environment variables. The tombstone table (`tombstones`) is small and grows slowly; tombstones are permanent by design — the dedup guarantee requires durability. The archival sweep is the third concurrent concern alongside the HTTP server and polling pipeline; it starts after migrations and stops cleanly on shutdown.

Plus an upgrade note: "Migration 0006 adds the `tombstones` table. Existing M10 databases migrate cleanly. Entries already in the database are subject to archival on the next sweep if they meet the horizon criterion."

### Tests and methodology

M11 follows the test-first discipline of M1–M10 (`docs/roadmap.md` §"Working cadence"). Pure scaffolding (the migration SQL, README edit, flag declarations) is exempt; everything with branches, error handling, or state is in scope.

**`internal/archival/sweep_test.go`**

DB pass correctness (real in-memory SQLite DB with migrations applied):

- Read + unsaved + older than horizon → deleted + tombstoned.
- Saved (regardless of read state, age) → retained.
- Unread (regardless of age) → retained.
- Read + unsaved + newer than horizon → retained.
- Chunking: insert 2500 eligible entries; assert three transactions fired (1000 + 1000 + 500); assert all deleted + tombstoned.
- Idempotency: run sweep twice on the same DB; second run deletes 0 entries (no double-tombstone, no error).
- Tombstone `ON CONFLICT DO NOTHING`: manually insert a tombstone row before the sweep; assert the sweep proceeds without error and the pre-existing tombstone is unchanged.

FS pass correctness (real temp directory):

- `.meta` older than age cap → `.bin` + `.meta` unlinked.
- `.meta` newer than age cap → both files retained.
- `.bin` present, `.meta` absent (orphan) → `.bin` unlinked.
- `.meta` present, `.bin` absent (already LRU-evicted by M3) → `.meta` unlinked, no error.
- Empty cache directory → no error.

**`internal/archival/archiver_test.go`**

- Clock injection: advance `Now` past `Interval`; assert `sweep` called.
- `Stop()` waits for an in-progress sweep: block `dbPass` on a channel; call `Stop()` concurrently; assert `Stop()` returns only after the channel is unblocked.
- Start + stop with no sweeps triggered: clean shutdown, no panic.

**`internal/db/tombstones_test.go`**

- `InsertTombstones` round-trip: insert rows, verify via `IsTombstoned`.
- `IsTombstoned` returns false for unknown `(sub, hash)`.
- `ON CONFLICT DO NOTHING`: insert same `(sub, hash)` twice in separate calls; no error; `deleted_at` from first insert is preserved.
- Cascade delete: delete the parent subscription row; assert tombstone rows are gone.

**`internal/poll/worker_test.go`** (extended)

- Tombstone consult hit: pre-insert a tombstone for `(sub.ID, entry.Hash)`; poll the feed; assert that entry is not in `entries` table after the commit.
- Tombstone consult miss: no tombstone; entry is inserted normally.
- Mixed: 3 entries, 1 tombstoned; assert 2 inserted, 1 skipped, poll succeeds, `error_count` unchanged.

**FTS regression test** (in `internal/archival/sweep_test.go`, which already runs against a real in-memory SQLite DB with all migrations applied):

- Insert entries into a DB with M9's FTS triggers active.
- Mark some entries read+unsaved; leave others unread or saved.
- Run `dbPass`.
- Query the FTS index for content from a deleted entry → 0 results.
- Query the FTS index for content from a retained entry → 1 result.
- This test is the contractual proof that M9's trigger-based FTS sync survives archival deletes.

**`cmd/tap/main_test.go`** (extended)

- End-to-end: poll a fixture feed, mark an entry read, advance clock past `--archive-horizon`, trigger a sweep manually (call `archiver.sweep()` directly, bypassing the ticker), assert the entry is gone from `entries`, a tombstone exists.
- Re-poll the same feed after tombstoning: the entry does not reappear in `entries`.
- End-to-end FS: write a fake `.bin` + `.meta` to the cache dir with an old `fetched_at`; run sweep; assert both files gone. A file with a recent `fetched_at` is retained.

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| Per-user archival horizon preferences | M7 — M7 adds `user_id` to subscriptions/entries. Per-user horizon can be added as a `users.archive_horizon` column in M7; the sweep query extends to `MIN(global, user_horizon)` at that point. |
| Tombstone garbage collection / TTL | Won't ship — tombstones are cheap and permanent by design. If operators observe unexpected tombstone growth, a `--tombstone-ttl` flag can be added cheaply. |
| Archival of entries for deleted subscriptions | Handled by `subscriptions` CASCADE DELETE on `entries` (existing M1 schema invariant); M11 adds the same cascade to `tombstones`. |
| "Re-archive" or manual sweep trigger via API | Won't ship — the sweep is background-only. Operators can restart the server to trigger a sweep at next tick. |
| OTel metrics / traces for sweep events | M12 — structured log on sweep completion is sufficient for M11. |
| Sweep status in the system-status panel | M12 — the panel is not yet shipped; sweep status will ride along with M12's observability work. |
| Per-subscription archival horizon override | Won't ship — global + per-user (M7) is the right granularity. Per-feed is over-engineered. |
| Negative caching or suppression of entries deleted via the API | Out of scope — tombstones are only written by the archival sweep, not by user-initiated unsubscribe or entry delete actions. |

## Risks and open questions

- **M7 `user_id` migration and tombstone scoping.** The `tombstones.subscription_id` FK chain provides per-user scoping transitively once M7 lands. M11 does not need a `user_id` column on `tombstones`. Verified: if user A and user B both subscribe to the same feed URL (via separate `subscriptions` rows), each has independent tombstones — correct per the strict per-user isolation posture.

- **FTS trigger compatibility.** M11's `DELETE FROM entries` fires M9's per-row FTS triggers. This is the correct path; no TRUNCATE or table-replace is used. The FTS regression test in M11's suite is the contractual proof.

- **Clock skew on `published_at`.** Feeds sometimes publish entries with future-dated `published_at` values (buggy feed generators). Those entries will never be archived until `published_at` passes the horizon. Acceptable — a future-dated entry is not "old"; archiving it early would be surprising. The `published_at` field is set at insert time from the feed item's date.

- **POSIX unlink safety.** A proxy handler holding a `.bin` open while `fsPass` unlinks it: the open fd survives on Linux; the directory entry is removed. The proxy response completes normally from the open fd. Safe on all supported platforms (Linux-first; concept §2 notes the static binary targets Linux hosts).

- **M3 inline LRU + M11 age sweep coexistence.** LRU evicts under bytes-cap pressure (M3); M11 evicts under age (once per day). Both agree on the on-disk file layout (`<aa>/<hash>.{bin,meta}`). M3's eviction path deletes `.bin` then `.meta`; M11 does the same. If M3 evicts a file that M11 would have evicted anyway, M11's `ENOENT` handling covers it gracefully.

- **Long sweeps on large DBs.** A DB with 500,000 eligible entries will run 500 chunk transactions. At ~5ms per chunk (1000 row delete + tombstone insert on a local SQLite), that's ~2.5s — acceptable for a daily background job. WAL mode keeps readers unblocked for the duration.

- **Archival sweep and concurrent poll workers.** A worker mid-poll inserts entries; the sweeper deletes entries. These are disjoint row sets: the sweeper only targets rows older than the horizon (typically 90 days), while the worker is inserting brand-new rows. No coordination needed beyond WAL-mode serialisation.

## Definition of done

1. `make test` passes (`go test ./... -race`) including the new package, migration, DB extensions, worker extension, and FTS regression test.
2. Fresh DB: start server, poll a fixture feed with multiple entries, mark some read, advance clock past `--archive-horizon`, trigger sweep; read+unsaved older than horizon are gone, saved and unread entries remain.
3. Tombstoned entry: re-poll the same feed after step 2; tombstoned entry does not reappear in `entries`.
4. FS pass: write fake `.bin`+`.meta` files to the cache directory with `fetched_at` older than `--cache-age-cap`; after sweep, both files are gone; a file with a recent `fetched_at` is retained.
5. `Stop()` drains a sweep in progress before returning (race-tested with `-race`).
6. Migration 0006 applies cleanly against an M10 database with no data loss.
7. `make build` produces a static binary that boots cleanly against a fresh `data/` directory and against an existing M10 database.
8. README gains the M11 trust-posture paragraph and upgrade note.

## What this milestone deliberately does *not* prove

- That per-user archival horizon preferences work — M7.
- That tombstones are ever garbage-collected — they aren't, by design.
- That sweep events appear in the system-status panel or OTel metrics — M12.
- That entries deleted via the unsubscribe API path are tombstoned — they aren't; tombstones are sweep-only.
- That the FTS index is consistent after a sweep if M9's triggers are not present — M11 assumes M9's migrations have run.

If you find yourself adding per-user horizon columns, tombstone TTL, sweep metrics, or API-triggered archival, push back. M11's job is the smallest daily sweep that bounds DB and cache growth, with tombstones that prevent re-published entries from resurfacing. Anything beyond that belongs in M12 (observability) or M7 (per-user prefs).
