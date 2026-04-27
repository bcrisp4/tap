# Tap — Design Spec

**Name:** Tap
**Date:** 2026-04-26
**Status:** Draft
**Go module:** `github.com/bcrisp4/tap`

Tap into your feeds. Tap is a self-hosted feed reader built for speed,
simplicity, and offline reading. Inspired by Miniflux but opinionated
differently: SQLite instead of PostgreSQL, a Svelte SPA for rich offline
support, and a focus on being a single self-contained binary with zero
external infrastructure.

---

## 1. Goals & Constraints

### Goals

- Fast, content-focused reading experience — mobile-first, works on iPhone home screen as a PWA
- Genuine offline support — read cached articles with no connectivity, sync state on reconnect
- Self-contained deployment — single binary, single SQLite file, single container volume
- Privacy by default — no tracking, no external requests beyond feed fetching
- Easy data portability — OPML import/export, REST API, no lock-in
- Built with TDD methodology

### Hard constraints

- SQLite for storage (no PostgreSQL, no external database)
- Runs in a Linux container image (as small as possible)
- Light / dark / sepia themes + system theme following
- Serif/sans-serif font toggle for reading (defaults to serif)
- REST API

### Non-goals (for now)

- Multi-user / authentication (schema is multi-user ready, but no auth in v1)
- Third-party integrations (Pinboard, Telegram, etc.)
- Fever / Google Reader API compatibility
- i18n / multiple languages

---

## 2. Architecture

```
┌──────────────────────────────────────────────────┐
│                   Go Binary                       │
│                                                   │
│  ┌─────────────┐  ┌──────────────────────────┐   │
│  │  HTTP Server │  │  Background Scheduler     │   │
│  │  (net/http)  │  │  (internal poller)        │   │
│  │             │  │                           │   │
│  │  /api/v1/*  │  │  • Dispatcher (tick + chan)│  │
│  │  /*  (SPA)  │  │  • Worker pool             │  │
│  │             │  │  • Per-host limiter        │  │
│  └──────┬──────┘  └────────────┬──────────────┘   │
│         │                      │                   │
│         └──────────┬───────────┘                   │
│                    │                               │
│         ┌──────────▼──────────┐                   │
│         │    SQLite (WAL)     │                   │
│         │       + FTS5        │                   │
│         └─────────────────────┘                   │
│                                                   │
│  ┌─────────────────────────────────────────────┐ │
│  │  Embedded Svelte SPA (go:embed)             │ │
│  │  Service Worker → IndexedDB (offline cache) │ │
│  └─────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────┘
```

**Single process, three concerns:**

1. **HTTP server** (stdlib `net/http`) serves the REST API under `/api/v1/`
   and the embedded Svelte SPA for all other routes.

2. **Background scheduler** is an internal `poller` package — a dispatcher
   goroutine on a `time.Ticker`, a fixed worker pool reading from a channel,
   a per-host concurrency limiter, and a daily archival sweep. Each worker
   handles one feed end-to-end: fetch, parse, extract any new articles in
   parallel via `errgroup` (with media URLs rewritten to the proxy and
   content sanitized), commit everything atomically. The `feeds` table
   itself is the schedule — `next_poll_at` is when each row is due. See §5
   for HTTP-client policy, content pipeline, media proxy, and archival.

3. **SQLite** is the single data store. WAL mode for concurrent reads. FTS5
   for full-text search.

**The Svelte SPA** is compiled at build time (SvelteKit + Vite) and embedded
via `go:embed`. A service worker caches articles to IndexedDB for offline
reading. State changes made offline are queued and synced on reconnect.

---

## 3. Tech Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Backend language | Go | Single binary, fast, known, good SQLite ecosystem |
| Frontend framework | Svelte (SvelteKit) | Small bundles (no runtime), good PWA/service worker support, compiles to vanilla JS |
| Database | SQLite (WAL mode) | Zero external infra, single file, fast for read-heavy workloads |
| Full-text search | FTS5 (SQLite extension) | BM25 ranking, prefix queries, contentless mode |
| Content extraction | Go Readability port (from Miniflux) | Server-side article extraction for full-content feeds |
| Feed parsing | Go library (TBD — gofeed or similar) | RSS 1.0/2.0, Atom, JSON Feed support |
| Container base | Alpine (musl libc) | Small image, provides libc for CGo |

### CGo dependency

SQLite requires CGo (`CGO_ENABLED=1`). This means the binary isn't fully
static and needs libc at runtime. Alpine with musl is the pragmatic choice
for the container image.

---

## 4. Data Model

### Pragmas (set at connection time)

```sql
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
```

### Users

Single default user now, multi-user ready. Preferences stored as columns
(Miniflux pattern — avoids joins on every request).

```sql
CREATE TABLE users (
    id               INTEGER PRIMARY KEY,
    username         TEXT    NOT NULL UNIQUE,
    theme            TEXT    NOT NULL DEFAULT 'system',
    font             TEXT    NOT NULL DEFAULT 'serif',
    entries_per_page INTEGER NOT NULL DEFAULT 50,
    default_sort     TEXT    NOT NULL DEFAULT 'published_at',
    default_order    TEXT    NOT NULL DEFAULT 'desc',
    created_at       INTEGER NOT NULL DEFAULT (unixepoch())
);
```

`theme` accepts `system`, `light`, `dark`, `sepia` (see §7 Brand & Visual
Identity). `font` accepts `serif`, `sans`.

### Categories

```sql
CREATE TABLE categories (
    id      INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name    TEXT    NOT NULL,
    UNIQUE(user_id, name)
);
```

No separate index needed — `UNIQUE(user_id, name)` covers `WHERE user_id = ?`.

### Icons

Deduplicated favicons. Multiple feeds from the same site share one icon row.

```sql
CREATE TABLE icons (
    id        INTEGER PRIMARY KEY,
    hash      TEXT    NOT NULL UNIQUE,
    mime_type TEXT    NOT NULL,
    content   BLOB   NOT NULL
);
```

### Feeds

```sql
CREATE TABLE feeds (
    id                   INTEGER PRIMARY KEY,
    user_id              INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id          INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    icon_id              INTEGER REFERENCES icons(id) ON DELETE SET NULL,
    title                TEXT    NOT NULL,
    feed_url             TEXT    NOT NULL,
    site_url             TEXT,
    description          TEXT,
    -- Polling state
    etag                 TEXT,
    last_modified        TEXT,
    last_polled_at       INTEGER,
    next_poll_at         INTEGER,
    poll_interval        INTEGER NOT NULL DEFAULT 3600,
    error_count          INTEGER NOT NULL DEFAULT 0,
    last_error           TEXT,
    -- Adaptive polling
    weekly_entry_count   INTEGER NOT NULL DEFAULT 0,
    -- Content extraction
    crawler              INTEGER NOT NULL DEFAULT 0,
    scraper_rules        TEXT,
    -- Behaviour flags
    disabled             INTEGER NOT NULL DEFAULT 0,
    ignore_entry_updates INTEGER NOT NULL DEFAULT 0,
    -- Per-feed HTTP overrides (NULL / 0 = use global default)
    user_agent              TEXT,
    cookie                  TEXT,
    username                TEXT,
    password                TEXT,
    proxy_url               TEXT,
    disable_http2           INTEGER NOT NULL DEFAULT 0,
    allow_self_signed_certs INTEGER NOT NULL DEFAULT 0,
    --
    created_at           INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at           INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(user_id, feed_url)
);
CREATE INDEX idx_feeds_user_category ON feeds(user_id, category_id);
CREATE INDEX idx_feeds_next_poll ON feeds(next_poll_at)
    WHERE disabled = 0 AND error_count < 10;
```

Design notes:
- `category_id ON DELETE SET NULL` — deleting a category doesn't delete feeds
- `crawler` defaults to 0 (off) — user enables per-feed for partial-content feeds
- `scraper_rules` — optional custom CSS selector, applied first; falls
  back to the (v2) built-in rules table, then the vendored Readability
  port (see §5 Content pipeline)
- Per-feed HTTP overrides (`user_agent`, `cookie`, `username` / `password`,
  `proxy_url`, `disable_http2`, `allow_self_signed_certs`) are all `NULL`
  or `0` by default; populated only for paywalled, intranet, or otherwise
  awkward feeds (see §5 HTTP client)
- Partial index on `next_poll_at` excludes disabled and broken feeds from the poller's scan

### Entries

```sql
CREATE TABLE entries (
    id            INTEGER PRIMARY KEY,
    feed_id       INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hash          TEXT    NOT NULL,
    title         TEXT    NOT NULL,
    url           TEXT,
    comments_url  TEXT,
    author        TEXT,
    summary       TEXT,
    content       TEXT,
    published_at  INTEGER,
    reading_time  INTEGER NOT NULL DEFAULT 0,
    read          INTEGER NOT NULL DEFAULT 0,
    read_at       INTEGER,
    saved             INTEGER NOT NULL DEFAULT 0,
    saved_at          INTEGER,
    extraction_failed INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL DEFAULT (unixepoch()),
    changed_at        INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE(feed_id, hash)
);
CREATE INDEX idx_entries_user_unread ON entries(user_id, read, published_at DESC);
CREATE INDEX idx_entries_user_pub    ON entries(user_id, published_at DESC);
CREATE INDEX idx_entries_user_saved  ON entries(user_id, saved, saved_at DESC)
    WHERE saved = 1;
CREATE INDEX idx_entries_feed_pub    ON entries(feed_id, published_at DESC);
```

Design notes:
- `hash` — computed dedup key (more robust than feed-provided GUIDs)
- `user_id` — denormalised from feeds for query efficiency on the river view
- `comments_url` — separate discussion URL for HN/Reddit-style feeds
- `reading_time` — computed at ingest time. The first 50 chars of content
  are scanned for CJK script (Han, Hiragana, Katakana, Hangul); CJK content
  is rated by character count (≈200 chars/min), non-CJK by word count
  (≈200 words/min). Avoids reporting Japanese/Chinese articles at ~5×
  their actual reading time.
- `changed_at` — tracks when content was last updated (distinct from created_at)
- `read` and `saved` are independent booleans (a saved entry can be read or unread)
- Partial index on saved entries — only indexes the small subset that are saved
- `extraction_failed` is `1` only when a `crawler=1` feed produced an entry whose
  article fetch failed — `content` then holds the feed-provided summary as a
  graceful fallback. `0` means either extraction wasn't needed or it succeeded.

Index coverage:

| Query | Index |
|-------|-------|
| River (unread, chronological) | `idx_entries_user_unread` |
| All entries (chronological) | `idx_entries_user_pub` |
| Saved entries | `idx_entries_user_saved` (partial) |
| Per-feed entries | `idx_entries_feed_pub` |
| Duplicate detection (polling) | `UNIQUE(feed_id, hash)` |

### Entry Tombstones

Prevents re-importing entries after deletion (Miniflux pattern).

```sql
CREATE TABLE entry_tombstones (
    feed_id    INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
    hash       TEXT    NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (feed_id, hash)
);
```

### Enclosures

Media attachments (podcast audio, video, images).

```sql
CREATE TABLE enclosures (
    id        INTEGER PRIMARY KEY,
    entry_id  INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    url       TEXT    NOT NULL,
    mime_type TEXT    NOT NULL DEFAULT '',
    size      INTEGER NOT NULL DEFAULT 0,
    UNIQUE(entry_id, url)
);
```

### Full-Text Search

Contentless FTS5 — the index references the entries table rather than
duplicating content. Requires sync triggers.

```sql
CREATE VIRTUAL TABLE entries_fts USING fts5(
    title, content,
    content=entries,
    content_rowid=id,
    tokenize='unicode61'
);

CREATE TRIGGER entries_fts_insert AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(rowid, title, content)
    VALUES (new.id, new.title, new.content);
END;

CREATE TRIGGER entries_fts_delete AFTER DELETE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, title, content)
    VALUES ('delete', old.id, old.title, old.content);
END;

CREATE TRIGGER entries_fts_update AFTER UPDATE OF title, content ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, title, content)
    VALUES ('delete', old.id, old.title, old.content);
    INSERT INTO entries_fts(rowid, title, content)
    VALUES (new.id, new.title, new.content);
END;
```

### Config

Single-row-per-key store for runtime-generated secrets and small bits of
state that don't justify their own table.

```sql
CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

Initial keys:

- `proxy_hmac_secret` — generated on first run if absent (hex-encoded
  256-bit random value); used by the media proxy to sign URLs (see §5
  Media proxy).

---

## 5. Background Scheduler & Feed Polling

Tap runs an internal scheduler — no external job framework, no separate queue
tables. It is built from `time.Ticker`, `chan`, `sync.Map`, and
`golang.org/x/sync/semaphore`. The `feeds` table itself is the schedule:
`next_poll_at` is when the row is due, and the dispatcher just queries it.

```
Dispatcher tick (default 60s)
    │
    ▼
Dispatcher
    │ SELECT id FROM feeds
    │   WHERE next_poll_at <= now()
    │     AND disabled = 0 AND error_count < 10
    │     AND id NOT IN (in_flight)
    │   ORDER BY next_poll_at ASC LIMIT N
    │
    ▼  chan int64 (feed IDs, unbuffered)
    │
    ▼
Worker pool (default 4 workers)
    │
    │ For each feed ID:
    │
    ├─▶ in_flight.Store(id)  // sync.Map
    ├─▶ acquire(feed_host)
    ├─▶ HTTP GET feed (ETag / If-Modified-Since)
    │     304? → update next_poll_at only, release, done
    ├─▶ Parse feed, dedupe against entries + tombstones
    │
    ├─▶ If feed.crawler = 1, errgroup over new entries:
    │     acquire(article_host)
    │     fetch + extract article
    │     success → content = extracted body
    │     failure → content = summary, extraction_failed = 1
    │   errgroup.Wait()
    │
    ├─▶ BEGIN IMMEDIATE
    │     INSERT new entries (every row has content)
    │     UPDATE feeds (next_poll_at, etag, last_modified,
    │                   error_count, weekly_entry_count, …)
    │   COMMIT
    │
    └─▶ in_flight.Delete(id)
```

### Components

**Dispatcher** — single goroutine on a `time.Ticker`. Each tick: query feeds
that are due *and not currently in flight*, push their IDs onto an unbuffered
channel. Backpressure is natural — the channel blocks when workers are busy,
so the dispatcher never over-claims work.

**Worker pool** — fixed-size pool. Each worker handles one feed end-to-end:
fetch, parse, extract any new articles in parallel via `errgroup`, commit
everything in a single write transaction. No worker ever yields a half-formed
entry to SQLite — every committed row has its `content` populated.

**Per-host limiter** — `sync.Map[string]*semaphore.Weighted`, lazy-initialised,
default weight 1 (strict serialisation per host). Keyed by the hostname of
whichever URL is being fetched (feed *or* article), so a feed and its
articles on the same host share the cap.

**In-flight set** — `sync.Map[int64]struct{}` tracking feed IDs currently
being polled. Lost on restart, which is the desired semantic: an in-progress
poll from before a crash gets re-dispatched on the next tick. The poll itself
is idempotent (`UNIQUE(feed_id, hash)` blocks duplicates).

**RunState** — small mutex-protected struct exposing live counters via
`/api/v1/system/status`: `active_polls`, `last_poll_at`, `recent_errors`. No
persistence; visibility only.

### HTTP client

A single `*http.Client` is shared by feed fetching, article-content
fetching, and the media proxy. Defaults are tighter than stdlib's:

| Setting | Default | Rationale |
|---|---|---|
| Overall request timeout | 20s (`TAP_HTTP_TIMEOUT`) | One slow feed can't tie up a worker |
| Dial timeout | 10s | Fail fast on dead hosts |
| KeepAlive | 15s | Bursty workload; long keepalive wastes connections |
| MaxIdleConns | 50 | Cap pool memory in the long-running process |
| IdleConnTimeout | 10s | Match keepalive |
| `Accept-Encoding` | `br, gzip` | Brotli + gzip; meaningful saving on verbose feeds |
| Max response body | 10 MiB (`TAP_HTTP_MAX_BODY_BYTES`) | `http.MaxBytesReader`; rejects feed bombs |
| Redirects | Followed; final URL written back to `feeds.feed_url` | Auto-heal moved feeds |
| User-Agent | `TAP_USER_AGENT` (per-feed override via `feeds.user_agent`) | Identify Tap politely |

**SSRF protection.** A `Dialer.Control` callback runs after DNS resolution
but before TCP connect, rejecting any RFC1918, loopback, link-local, or
ULA address. This defeats DNS-rebinding attacks (a hostile DNS record
can't sneak past). Two escape hatches:

- `TAP_ALLOW_PRIVATE_NETWORKS=true` disables the check entirely.
- `TAP_ALLOWED_HOSTS` accepts a comma-separated list of hostname suffixes
  and CIDR blocks that bypass the check — e.g.
  `marlin-tet.ts.net,10.0.0.0/24` for Tailscale-resident feeds.

**Per-feed overrides.** All optional; `NULL`/`0` falls back to the global
default. Used for paywalled, intranet, and otherwise awkward feeds:

| Column | Effect |
|---|---|
| `feeds.user_agent` | Override `TAP_USER_AGENT` (some sites gate on UA) |
| `feeds.cookie` | Sent verbatim as `Cookie:` header |
| `feeds.username` / `feeds.password` | HTTP Basic auth |
| `feeds.proxy_url` | Per-feed HTTP/HTTPS/SOCKS5 proxy URL |
| `feeds.disable_http2` | Force HTTP/1.1 for servers with broken HTTP/2 |
| `feeds.allow_self_signed_certs` | Relax TLS verification (intranet only) |

### Content pipeline

When a worker polls a `crawler=1` feed and finds new entries, each new
entry runs through a four-stage pipeline before the poll commits:

1. **Fetch** the article URL (same HTTP client, same per-host limiter).
2. **Extract** with three-tier precedence:
   1. If `feeds.scraper_rules` is set, apply the CSS selector via
      `goquery.Find(rules).Each(...)`.
   2. *(v2)* Built-in rules table for known domains — extension point,
      not shipped in v1.
   3. Fallback: vendored Miniflux Readability port (~390 lines, depends
      only on `goquery` and `golang.org/x/net/html`).
3. **Rewrite media URLs** to the proxy (see Media proxy below). Updates
   `<img src>`, `<img srcset>`, `<picture><source srcset>`. Iframes,
   audio, and video go direct.
4. **Sanitize** with an allowlist policy.

**Sanitization policy** (modelled after Miniflux's sanitizer):

- ~50-tag allowlist for content (`p`, `div`, `article`, `figure`, `pre`,
  `code`, headings, lists, tables) plus media (`img`, `video`, `audio`,
  `picture`, `source`, `iframe`).
- Per-tag attribute allowlists. `<a>` keeps `href`, `title`, `id`. `<img>`
  keeps `alt`, `src`, `srcset`, `sizes`, `width`, `height`,
  `fetchpriority`, `decoding`.
- `<script>` and `<style>` removed at parse time.
- Inline `on*` handlers stripped (not in any allowlist).
- URL-scheme allowlist for resource attributes: `http`, `https`,
  `mailto`, `tel`. `data:` allowed *only* for image MIME types.
- `<iframe>` `src` host allowlist (override via `TAP_IFRAME_ALLOWLIST`):
  `youtube.com`, `youtube-nocookie.com`, `vimeo.com`, `bandcamp.com`,
  `soundcloud.com`, `spotify.com`, `twitch.tv`, `dailymotion.com`.
- Relative URLs resolved against the entry URL.
- Maximum DOM depth 512 (anti-billion-laughs).
- 1×1 pixel images stripped (tracking-pixel heuristic).

Final sanitized HTML is stored in `entries.content`. The fallback summary
on `extraction_failed = 1` runs through the sanitizer too — Tap never
stores unsanitized HTML.

### Media proxy

Article images are routed through Tap rather than fetched directly from
origin. Three reasons: (1) **privacy** — origin sites only see Tap's IP,
not the user's browser; (2) **mixed-content fix** — HTTP-only image URLs
work even when Tap is served over HTTPS; (3) **offline reading** —
pre-loaded entries can have their proxy URLs cached by the service worker.

**URL format:**

```
/api/v1/proxy/<token>
token = base64url(source_url) + "." + hex(hmac_sha256(secret, source_url))[:16]
```

The HMAC is static (no expiry) using the 256-bit secret in
`config.proxy_hmac_secret` (auto-generated on first run if absent).
Rotating the secret invalidates browser caches but not the on-disk cache.

**Rewriting.** Done at extraction time, as stage 3 of the content
pipeline, before commit. The rewriter parses HTML once with `goquery`,
finds `<img>` and `<picture><source>` elements, and replaces:

- `src` attributes
- `srcset` attributes — split on comma, each candidate URL rewritten,
  width/density descriptors preserved verbatim

Stored content is final-form. Future scheme changes are handled by a
one-shot resanitization sweep over `entries.content` if ever needed.

**Origin fetch.**

- Singleflight (`golang.org/x/sync/singleflight`) coalesces concurrent
  requests for the same URL into one origin fetch.
- Same per-host limiter as feed fetching — a feed and its images on the
  same CDN serialise together.
- Same SSRF protection as feed fetching.
- Timeout `TAP_PROXY_TIMEOUT` (default 10s), max body
  `TAP_PROXY_MAX_BODY_BYTES` (default 10 MiB).
- Response MIME-type allowlist: `image/jpeg`, `image/png`, `image/gif`,
  `image/webp`, `image/avif`, `image/svg+xml`. Anything else → 415.
- Origin error → 404. Browser shows broken-image icon natively; no
  placeholder image to maintain.

**Cache.** Filesystem-backed at `TAP_PROXY_CACHE_DIR` (default
`./tap-cache/`). Layout:

```
<cache_dir>/<2-char-hex-shard>/<sha256-of-source-url>
<cache_dir>/<2-char-hex-shard>/<sha256-of-source-url>.meta
```

The `.meta` sidecar holds the origin Content-Type and ETag. Outbound
headers: `Cache-Control: public, max-age=2592000` (30 days), `ETag` of
the body SHA-256, honors `If-None-Match` with 304.

**Eviction is two-layered:**

- **Size cap** — on insert, if total cache size exceeds
  `TAP_PROXY_CACHE_MAX_BYTES` (default 1 GiB), evict by oldest mtime
  until under. Inline at write time.
- **Age cap** — files older than `TAP_PROXY_CACHE_MAX_AGE` (default 30
  days) deleted by the daily archival sweep.

**Pre-fetching for offline.** SPA-side responsibility (see §8). When the
service worker pre-loads an entry it also fetches every `/api/v1/proxy/...`
URL referenced in the article HTML, so offline reads have images.

### Adaptive polling

`next_poll_at` is computed in exactly two places: when a feed is created
(`next_poll_at = now`, so the next dispatcher tick picks it up) and at the
end of every poll job — success or failure.

The adaptive formula consumes `weekly_entry_count`: the count of entries
inserted for this feed in the last 7 days, recomputed in the same transaction
as the poll commit:

```sql
UPDATE feeds SET weekly_entry_count = (
    SELECT COUNT(*) FROM entries
    WHERE feed_id = feeds.id
      AND created_at > unixepoch() - 7*86400
)
WHERE id = ?;
```

After a poll, the worker walks these conditions in order. The first match
wins:

1. **Server sent `Retry-After`.** `next_poll_at = now + retry_after`. Reset
   `error_count = 0` — server-mandated backoff is not our error.
2. **Poll failed (network, 5xx, parse error).** Increment `error_count`,
   then `next_poll_at = now + min(2^error_count hours, 24h)`. After 10
   consecutive failures the partial index `idx_feeds_next_poll` excludes
   the row entirely — the feed is dormant until manually revived.
3. **Server sent `Cache-Control: max-age` (or `Expires`).** Compute the
   adaptive interval (step 4), then `interval = max(adaptive, max_age)`.
   `max-age` is the response's freshness lifetime — the server is
   asserting the response won't change before then, so polling sooner
   wastes bandwidth. The hint is a *floor* on the interval, not a ceiling.
4. **Normal case (success or 304).** Reset `error_count = 0`, then:

   ```
   if weekly_entry_count == 0:
       interval = 24h                                 # quiet or brand-new feed
   else:
       interval = (7d / weekly_entry_count) / factor
   interval = clamp(interval, 15m, 24h)
   next_poll_at = now + interval
   ```

   `factor` is exposed as `TAP_POLL_FACTOR` (default `1.0`). Lower values
   poll more often; higher values poll less often.

ETag / If-None-Match and Last-Modified / If-Modified-Since are sent on every
poll request to minimise bandwidth — a 304 response skips parsing entirely
but still updates `next_poll_at` via the normal-case branch.

Manual refresh (`POST /feeds/:id/refresh`) sets `next_poll_at = now`. The
dispatcher claims the feed on its next tick; no special code path.

### Error handling

- **Feed fetch errors** (timeout, DNS, 5xx, parse error): handled by step 2
  above. After 10 consecutive failures the partial index excludes the feed
  entirely; manual retry via API or UI revives it.
- **Article extraction failures**: one attempt, no retry. Entry is written
  with `content = summary` and `extraction_failed = 1`. The poll itself
  succeeds; the entry is visible immediately. A future API endpoint can
  expose manual re-extraction.
- **Worker panic**: caught by `recover()`, logged, surfaced in `RunState`.
  The worker continues; the in-flight entry is cleared in a `defer` so the
  feed gets re-dispatched on the next tick.

### Politeness

- Per-host concurrency cap (default 1) via the per-host semaphore limiter
- Conditional HTTP (ETag, Last-Modified) to minimise bandwidth
- Server-mandated backoff via `Retry-After` always respected (step 1)
- Server-suggested freshness via `Cache-Control: max-age` (or `Expires`)
  respected as a floor on the adaptive interval (step 3)

### Archival

A second goroutine, sibling to the dispatcher, runs on a daily ticker.
Each sweep, in order:

1. **Archive read entries.** Delete entries where `read = 1 AND saved = 0
   AND created_at < unixepoch() - TAP_ARCHIVE_DAYS * 86400` (default 60
   days). For each deleted entry, insert `(feed_id, hash)` into
   `entry_tombstones` in the same transaction. The poll-time dedup check
   already consults tombstones, so re-published entries don't reappear.
2. **Sweep the proxy cache.** Walk `TAP_PROXY_CACHE_DIR`, delete any file
   whose mtime is older than `TAP_PROXY_CACHE_MAX_AGE` (default 30 days).
   Cheap — one `stat` per file. Resume-safe: if killed mid-sweep, the
   next day picks up wherever it left off.

The sweep is single-threaded. No per-feed parallelism, no contention with
poll workers beyond brief write transactions for the archival deletes.

---

## 6. REST API

**Base path:** `/api/v1`

### Feeds

| Method | Path | Description |
|--------|------|-------------|
| GET | `/feeds` | List all feeds (with unread counts) |
| POST | `/feeds` | Subscribe to a new feed |
| GET | `/feeds/:id` | Get feed details |
| PUT | `/feeds/:id` | Update feed (title, category, crawler, etc.) |
| DELETE | `/feeds/:id` | Unsubscribe (deletes entries + tombstones) |
| POST | `/feeds/:id/refresh` | Trigger immediate poll |

### Categories

| Method | Path | Description |
|--------|------|-------------|
| GET | `/categories` | List categories (with unread counts) |
| POST | `/categories` | Create category |
| PUT | `/categories/:id` | Rename category |
| DELETE | `/categories/:id` | Delete (feeds become uncategorised) |

### Entries

| Method | Path | Description |
|--------|------|-------------|
| GET | `/entries` | List entries (filterable, paginated) |
| GET | `/entries/:id` | Get single entry with full content |
| PUT | `/entries/:id` | Update entry state (read, saved) |
| PUT | `/entries/read` | Bulk mark as read (by feed, category, or all) |

Entry listing query params:

| Param | Values | Default |
|-------|--------|---------|
| `status` | `unread`, `read`, `all` | `unread` |
| `saved` | `true`, `false` | (not filtered) |
| `feed_id` | integer | (not filtered) |
| `category_id` | integer | (not filtered) |
| `sort` | `published_at`, `created_at` | `published_at` |
| `order` | `asc`, `desc` | `desc` |
| `limit` | integer | 50 |
| `offset` | integer | 0 |

Entry list responses exclude `content` (full article body) to keep payloads
small. Full content is fetched via `GET /entries/:id`.

### Search

| Method | Path | Description |
|--------|------|-------------|
| GET | `/search?q=...` | Full-text search across entries |

### OPML

| Method | Path | Description |
|--------|------|-------------|
| POST | `/opml/import` | Import feeds from OPML file |
| GET | `/opml/export` | Export all feeds as OPML |

### Discovery

| Method | Path | Description |
|--------|------|-------------|
| POST | `/feeds/discover` | Find feed URLs from a website URL |

### Media proxy

| Method | Path | Description |
|--------|------|-------------|
| GET | `/proxy/:token` | Serve a cached image referenced in extracted entry content. Token = `base64url(url).hex(hmac)`. See §5 Media proxy for cache, eviction, and security details. |

---

## 7. Brand & Visual Identity

### Name & metaphor

**Tap** — to tap into your feeds. The product aggregates multiple RSS/Atom
sources into a single chronological stream, the way a signal tap draws a clean
copy off a main line without interrupting it. The name reads on two levels:
the electrical engineering term, and the everyday gesture of tapping a screen.

In schematic diagrams a tap is shown as a small filled dot at the
intersection of two wires, marking an electrical connection rather than a
crossover. The junction dot is the load-bearing detail of Tap's identity.

### Product philosophy

Minimal, fast, clean. Open source. Self-hosted, with no central service, no
telemetry, and no account required. No algorithm — chronological feeds, the
way RSS was meant to be consumed. The interface is built for reading, not for
engagement metrics.

### Logo system

Three concept variants form a system, all built around the schematic junction
dot:

- **Primary mark (favicon, app icon).** A T-junction. Short horizontal line
  with a vertical line branching down from its midpoint, with a filled dot at
  the intersection. The T-shape doubles as the letter T in "tap." Stroke
  weight medium-bold, rounded line caps. Junction dot in Klein Blue against
  a monochrome line.

- **Wordmark.** Lowercase "tap" in a geometric sans-serif, tight
  letter-spacing. The period after "tap" is rendered slightly larger and
  heavier than typographic norm, so it reads as both a period and a schematic
  junction dot. The dot is Klein Blue.

- **Hero graphic (landing page, marketing).** A multi-tap. One horizontal
  main line with three short vertical branches rising from it, junction dots
  at each branch point, and one branch descending to indicate output.
  Communicates "many sources, one stream." Junction dots in Klein Blue.

### Color

- **Klein Blue / International Klein Blue (IKB)** — approximately `#002FA7`
  — is the brand accent. Used for the junction dot, interactive elements
  (links, active states, focus rings), and small accents. It evokes
  electrical signal, voltage, ink. Used sparingly, never as a flood.
- The rest of the interface is monochrome: paper-like backgrounds, ink-like
  text.
- **Three themes**, switchable and following the system default:
  - **Light** — off-white background, near-black text, Klein Blue accent.
  - **Dark** — near-black background, off-white text, Klein Blue accent
    (slightly desaturated for contrast in dark mode).
  - **Sepia** — warm cream background (approximately `#F4ECD8`), dark brown
    text, Klein Blue accent retained for continuity. This is the dedicated
    reading mode.

### Typography

- **Body and reading content** — a serif typeface optimised for long-form
  reading. Generous line height (1.6–1.7), comfortable measure (65–72
  characters per line), no full-width text. Suggested options: Source Serif,
  Iowan Old Style, Charter, or similar.
- **UI chrome, navigation, metadata** — geometric sans-serif. Smaller, lower
  contrast, recedes from the content.
- **Wordmark and brand** — geometric sans-serif, lowercase, tight tracking.
- Sentence case throughout. Never all caps.

### Layout & UI principles

- **Content is the interface.** Article text dominates the viewport when
  reading. Chrome, sidebars, and controls collapse or fade when not in
  active use.
- **No engagement metrics.** No view counts, no reaction icons, no
  share-count badges.
- **Single-column reading view** by default. Multi-column or list views are
  available but not primary.
- **Keyboard navigation is first-class.** The full app is operable without a
  mouse.
- **Density is restrained.** Whitespace is the dominant design material.
- **Animation is minimal and functional** — fade and slide for state
  transitions, never decorative motion.
- **Iconography**, where used, is line-based and matches the schematic logo
  language: thin strokes, rounded caps, monochrome with Klein Blue reserved
  for the active state.

### Visual rules

- The junction dot is always present in some form across brand surfaces. It
  is the load-bearing detail.
- Avoid imagery that suggests faucets, water, drips, or plumbing. The tap is
  electrical, not domestic.
- Avoid full circuit diagrams, PCB traces, or busy schematic clutter. The
  brand uses one symbol, used well.
- Klein Blue is precious. If it appears more than three times on a single
  screen, something is wrong.

### Tone

Quiet confidence. Engineering-literate but not exclusionary. The product is
for people who want to read, not for people who want to be impressed by
software.

---

## 8. Frontend (Svelte SPA)

### Views

| View | Route | Description |
|------|-------|-------------|
| River | `/` | Chronological unread entries (default home) |
| All entries | `/all` | Includes read entries |
| Saved | `/saved` | Saved/starred entries |
| Category | `/category/:id` | Entries for a category |
| Feed | `/feed/:id` | Entries for a single feed |
| Article | `/entry/:id` | Reader view + "view original" button |
| Search | `/search?q=...` | Search results |
| Settings | `/settings` | Theme, font, feed management, OPML |
| Add feed | `/feeds/add` | URL → autodiscovery → subscribe |

### Reading experience

- Clean reader view with serif typography by default
- Single-column layout, comfortable measure (65–72 ch), generous line height
- "View original" button opens source URL in new tab
- Reading time displayed per article
- Feed icon + source name in article metadata, set in geometric sans-serif

### Interactions

- **Mark as read**: explicit button/tap only (no auto-mark on scroll)
- **Swipe gestures (mobile)**: swipe right → mark read/unread, swipe left → save/unsave
- **Keyboard shortcuts (desktop)**: `j`/`k` navigate, `m` toggle read, `s` save, `v` view original, `o` open article — keyboard navigation is first-class, the full app is operable without a mouse
- **Bulk actions**: mark all read for feed/category/everything
- **Pull-to-refresh**: triggers feed refresh on mobile
- **Infinite scroll**: paginated via API offset/limit

### Theming

- Light / dark / sepia / follow system (`prefers-color-scheme`)
- Serif / sans-serif toggle for reading (defaults to serif)
- Klein Blue (`#002FA7`) accent across all themes; slightly desaturated in dark
- CSS custom properties — theme switching is instant, no reload
- See §7 Brand & Visual Identity for the full palette and visual rules

### Offline

- Service worker caches the app shell on first load (PWA opens instantly)
- API responses cached to IndexedDB (articles with full content)
- Configurable: proactively cache the N most recent entries (default 200)
- For each pre-loaded entry, the service worker also fetches every
  `/api/v1/proxy/...` URL referenced in the article HTML, populating the
  Cache Storage API so images are available offline alongside body text
- Offline reads from IndexedDB; state changes queued and synced on reconnect

### PWA

- Web app manifest with icon for iOS home screen (uses the T-junction primary mark)
- `display: standalone` — looks like a native app
- Status bar styling matches the active theme

---

## 9. Deployment

### Container build

```
Multi-stage Dockerfile:

Stage 1: Node        → npm ci, npm run build (Svelte → static assets)
Stage 2: Go + CGo    → copy assets into embed dir, go build
Stage 3: Alpine      → copy binary, minimal runtime

Target: < 30MB final image
```

CGo is required (SQLite). Alpine provides musl libc.

### 12-Factor compliance

| Factor | Implementation |
|--------|---------------|
| Codebase | One repo, one deployable |
| Dependencies | Go modules + npm lockfile |
| Config | Environment variables |
| Backing services | SQLite co-located (not networked) |
| Build/release/run | Dockerfile builds; container image is the release |
| Processes | Single process |
| Port binding | Configurable HTTP port (default 8080) |
| Concurrency | Single process, goroutines internally |
| Disposability | Fast startup, graceful shutdown |
| Dev/prod parity | Same binary, same SQLite |
| Logs | Structured JSON to stdout |
| Admin processes | CLI subcommands |

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `TAP_DB_PATH` | `./tap.db` | SQLite database file path |
| `TAP_LISTEN` | `:8080` | HTTP listen address |
| `TAP_LOG_LEVEL` | `info` | debug, info, warn, error |
| `TAP_LOG_FORMAT` | `json` | json, text |
| `TAP_POLL_INTERVAL` | `60s` | Dispatcher tick interval |
| `TAP_POLL_WORKERS` | `4` | Worker pool size |
| `TAP_POLL_FACTOR` | `1.0` | Adaptive polling multiplier (lower = poll more often) |
| `TAP_USER_AGENT` | `Tap/1.0 (+https://github.com/bcrisp4/tap)` | Default User-Agent; per-feed override via `feeds.user_agent` |
| `TAP_HTTP_TIMEOUT` | `20s` | Overall outbound request timeout |
| `TAP_HTTP_MAX_BODY_BYTES` | `10485760` | Max response body (10 MiB) |
| `TAP_ALLOW_PRIVATE_NETWORKS` | `false` | If true, disables the SSRF private-network block entirely |
| `TAP_ALLOWED_HOSTS` | (empty) | Comma-separated hostname suffixes / CIDR blocks bypassing the SSRF check (e.g. `marlin-tet.ts.net,10.0.0.0/24`) |
| `TAP_IFRAME_ALLOWLIST` | (built-in) | Override the iframe `src` host allowlist (comma-separated) |
| `TAP_ARCHIVE_DAYS` | `60` | Read & unsaved entries older than this are archived (with tombstones) |
| `TAP_PROXY_CACHE_DIR` | `./tap-cache/` | Filesystem cache directory for the media proxy |
| `TAP_PROXY_CACHE_MAX_BYTES` | `1073741824` | Max proxy cache size (1 GiB); excess evicted by oldest mtime |
| `TAP_PROXY_CACHE_MAX_AGE` | `30d` | Cache files older than this are deleted by the daily archival sweep |
| `TAP_PROXY_TIMEOUT` | `10s` | Origin-fetch timeout for proxy requests |
| `TAP_PROXY_MAX_BODY_BYTES` | `10485760` | Max body for proxy origin fetches (10 MiB) |

### Data persistence

Single volume mount for the SQLite database file. Backup = copy the file
(or `VACUUM INTO` for a consistent snapshot).

### Graceful shutdown

On SIGTERM: stop accepting connections → drain in-flight requests → let current
poll job finish (30s timeout) → close database.

---

## 10. Pre-Implementation Research

The Miniflux research is complete and folded into §4 (schema, `config`
table, per-feed HTTP overrides), §5 (HTTP client, content pipeline, media
proxy, archival), and §8 (offline pre-fetch). Key takeaways:

- The Readability port at
  `~/vendor/miniflux/internal/reader/readability/readability.go` (~390
  lines) can be vendored largely as-is. It depends only on `goquery` and
  `golang.org/x/net/html`, plus one trivial dependency on
  `urllib.IsAbsoluteURL` (a 5-line replacement against Tap's URL helpers).
- The sanitizer model in §5 mirrors
  `~/vendor/miniflux/internal/reader/sanitizer/sanitizer.go` — the
  allowlist (tags and per-tag attributes, lines 26–119) is the
  reference; confirm completeness during implementation.
- Miniflux's HTTP fetcher at
  `~/vendor/miniflux/internal/reader/fetcher/` is the reference for the
  client tuning in §5 HTTP client (timeouts, body cap, brotli, SSRF guard,
  per-feed overrides).
- Miniflux does *not* run a media proxy; Tap does. The rewriter walks the
  DOM with `goquery`, replacing `<img src>`, `<img srcset>`, and
  `<picture><source srcset>`. Iframes/audio/video go direct.

Implementation-time validation tasks:

- Confirm the chosen feed parser (`gofeed` or similar) handles RSS 1.0,
  RSS 2.0, Atom, and JSON Feed against fixtures pulled from
  `~/vendor/miniflux/internal/reader/(rss|atom|rdf|json)/`.
- Decide whether to ship a starter built-in scraper-rules table in v2;
  for v1, only per-feed `scraper_rules` and the Readability fallback are
  wired up.

---

## 11. Inspiration

- **Miniflux** — architecture, adaptive polling, Readability extraction,
  tombstone pattern, privacy approach
- **Wiki research** — SQLite concepts, FTS5, adaptive polling,
  PRAGMA data_version polling, partial index queue design, web content
  extraction (Readability vs Defuddle)
