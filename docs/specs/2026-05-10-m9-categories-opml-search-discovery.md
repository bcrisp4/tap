# M9 — Categories, OPML, search, add-feed flow

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md). M9 is the ninth of twelve milestones — see [`../roadmap.md`](../roadmap.md). M1–M8 have shipped: the walking skeleton, sanitisation, media proxy, polling discipline, article extraction, auth foundations, 2FA + passkeys + per-user data isolation, and SPA polish.

M9 is the organisational and ingest UX layer. It bundles four features that all sit at the same conceptual boundary — getting feeds in, organising them, and finding content across them:

1. **Categories** — flat per-user list; a subscription belongs to at most one category; deleting a category uncategorises its feeds rather than deleting them.
2. **OPML import/export** — OPML 2.0; one-level folder nesting flattened to categories on import; grouped by category on export.
3. **Full-text search** — SQLite FTS5 over entry title, HTML-stripped content, and author; triggered sync; `unicode61` tokeniser.
4. **Add-feed discovery** — given any URL, try to parse it as a feed first, then fall back to `<link rel="alternate">` discovery; return candidates for the user to pick from.

**Hard dependency on M7.** M9 requires M7's `user_id` columns on `subscriptions` and `entries` to exist. Every M9 query that touches subscriptions, entries, or categories filters by `user_id`. M9 must not be implemented before M7 has shipped. The specs are written in parallel; implementation order is M7 → M8 → M9.

**M8 keybinding note.** The `/` focus-search keybinding ships in M9 alongside the search feature. M8's keyboard shortcut surface omits it entirely — it is not a stub, it is simply absent. See M8 spec "Out of scope" table.

## Goal

After M9, a logged-in user can:

- Create categories from the sidebar, assign feeds to them (during add-feed or by editing a feed), and see feeds grouped under their categories in the sidebar with per-category unread counts.
- Click a category to view its aggregated entry list, and bulk-mark all entries in it as read from the top bar.
- Import an OPML file (up to 10 MiB) to subscribe to many feeds at once, with top-level OPML folders becoming categories. Export their subscriptions as a valid OPML 2.0 file grouped by category.
- Search across all their entries by title, body, or author using full-text search, with results URL-synced via `?q=` and debounced at 300 ms.
- Paste any URL into the add-feed flow and get a list of feed candidates discovered from the page, or be told clearly when none are found.

## In scope

### Schema migrations

#### `internal/db/migrations/0008_categories.sql`

```sql
CREATE TABLE categories (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE (user_id, name)
);
CREATE INDEX idx_categories_user ON categories(user_id);

ALTER TABLE subscriptions ADD COLUMN category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL;
CREATE INDEX idx_subscriptions_category ON subscriptions(category_id);
```

`ON DELETE CASCADE` from `users` ensures all of a user's categories are removed when the user is deleted. `ON DELETE SET NULL` on `subscriptions.category_id` ensures that deleting a category uncategorises its feeds rather than cascading to delete them.

#### `internal/db/migrations/0009_fts5.sql`

```sql
CREATE VIRTUAL TABLE entries_fts USING fts5(
    title,
    content_text,
    author,
    content='',
    tokenize='unicode61'
);

CREATE TRIGGER entries_fts_insert AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(rowid, title, content_text, author)
    VALUES (new.id, new.title, tap_strip_html(new.content), new.author);
END;

CREATE TRIGGER entries_fts_delete AFTER DELETE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid) VALUES ('delete', old.id);
END;

CREATE TRIGGER entries_fts_update AFTER UPDATE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid) VALUES ('delete', old.id);
    INSERT INTO entries_fts(rowid, title, content_text, author)
    VALUES (new.id, new.title, tap_strip_html(new.content), new.author);
END;
```

`entries_fts` is a **contentless FTS5 table** — content is not duplicated inside FTS; `content=''` tells FTS5 not to store it. Lookups join back to `entries` by rowid. `tap_strip_html` is a custom SQLite scalar function registered in Go at DB open time (see `internal/db/db.go`) that strips HTML tags from the content string before indexing, preventing tag names from bleeding into tokenisation.

**M11 note.** M11's archival sweep deletes rows from `entries` directly. The `entries_fts_delete` trigger fires automatically on those deletes, keeping the FTS index consistent without any explicit sync call in M11. M11 must include a regression test that performs an archival delete and asserts the deleted entry is no longer findable via FTS.

Because `entries_fts` is contentless, `entries_fts_rebuild` / `entries_fts_integrity-check` commands work as expected. If the index ever drifts (e.g. trigger failure during bulk import), it can be rebuilt with `INSERT INTO entries_fts(entries_fts) VALUES('rebuild')`.

**Populating the FTS index on migration.** `0009_fts5.sql` inserts all existing entries into `entries_fts` after creating the table and triggers:

```sql
INSERT INTO entries_fts(rowid, title, content_text, author)
SELECT id, title, tap_strip_html(content), author FROM entries;
```

This back-fill runs inside the migration transaction. `tap_strip_html` must be registered before migrations run — `db.Open` registers it, so the order is already correct.

### New package: `internal/discover`

```
internal/discover/
  discover.go
  discover_test.go
```

```go
type Result struct {
    Title   string
    FeedURL string
    SiteURL string
    Type    string // "rss", "atom", "json"
}

var ErrNoFeeds = errors.New("no feed candidates found at URL")

// Discover fetches url via client. It first attempts to parse the response
// as a feed directly (RSS, Atom, JSON Feed). If that fails it parses the
// HTML for <link rel="alternate" type="application/...+xml|json"> elements
// and returns all candidates found. Returns ErrNoFeeds if neither strategy
// finds anything. Discovery fetches are always unauthenticated — ApplyFeedCreds
// is never called here.
func Discover(ctx context.Context, client *http.Client, rawURL string) ([]Result, error)
```

- Uses the shared `httpx.NewClient` (SSRF guard, per-host cap, timeout all apply).
- No `ApplyFeedCreds` — discovery is always anonymous.
- No well-known path probing (`/feed`, `/rss`, etc.) — YAGNI.
- `<link rel="alternate">` parsing handles `type="application/rss+xml"`, `type="application/atom+xml"`, `type="application/feed+json"`, `type="application/json"` (the last per JSON Feed spec).
- The two-step (try-parse → HTML fallback) means a user pasting a direct feed URL gets an immediate single-candidate response without an extra page fetch.

### `internal/db` extensions

#### `internal/db/categories.go`

```go
type Category struct {
    ID        int64
    UserID    int64
    Name      string
    CreatedAt int64
    Unread    int // populated on ListCategories via COUNT join
}

type NewCategory struct {
    UserID    int64
    Name      string
    CreatedAt int64
}

var ErrCategoryNotFound  = errors.New("category not found")
var ErrCategoryNameTaken = errors.New("category name already exists for this user")

func InsertCategory(ctx context.Context, d *sql.DB, c NewCategory) (int64, error)
func GetCategory(ctx context.Context, d *sql.DB, id, userID int64) (Category, error)
func ListCategories(ctx context.Context, d *sql.DB, userID int64) ([]Category, error)
func UpdateCategoryName(ctx context.Context, d *sql.DB, id, userID int64, name string) error
func DeleteCategory(ctx context.Context, d *sql.DB, id, userID int64) error
func MarkCategoryRead(ctx context.Context, d *sql.DB, categoryID, userID int64, now int64) error
```

`ListCategories` computes the `Unread` count in a single query with a LEFT JOIN + COUNT rather than N+1 queries.

`GetCategory`, `UpdateCategoryName`, `DeleteCategory`, and `MarkCategoryRead` all take `userID` and include `AND user_id = ?` in their WHERE clause — cross-user access returns `ErrCategoryNotFound`, not a permission error, to avoid enumeration.

#### `internal/db/subscriptions.go` extensions

- `Subscription`, `NewSubscription`, and `UpdateSubscription` gain `CategoryID sql.NullInt64`.
- `ListSubscriptions` and `GetSubscription` SELECT include `category_id`.
- `InsertSubscription` and `UpdateSubscriptionCategory` (new) accept `category_id`.
- New function: `ListSubscriptionsByCategory(ctx, d *sql.DB, categoryID, userID int64) ([]Subscription, error)` — used by the entries filter.

#### `internal/db/search.go`

```go
type SearchResult struct {
    ID             int64
    SubscriptionID int64
    Title          string
    URL            string
    Author         string
    PublishedAt    int64
    Read           bool
    Saved          bool
    Rank           float64 // bm25() score, lower = more relevant
}

func SearchEntries(ctx context.Context, d *sql.DB, userID int64, query string, limit int, cursor int64) ([]SearchResult, int64, error)
```

The SQL joins `entries_fts` → `entries` → `subscriptions` to enforce user scoping:

```sql
SELECT e.id, e.subscription_id, e.title, e.url, e.author,
       e.published_at, e.read, e.saved,
       bm25(entries_fts) AS rank
FROM   entries_fts
JOIN   entries       ON entries.id = entries_fts.rowid
JOIN   subscriptions ON subscriptions.id = entries.subscription_id
WHERE  entries_fts MATCH ?
  AND  subscriptions.user_id = ?
  AND  e.id < ?  -- cursor
ORDER  BY rank, e.id DESC
LIMIT  ?
```

The FTS index itself has no `user_id` column — user scoping comes entirely from the `subscriptions.user_id` join. This is correct: an entry is only reachable if its subscription belongs to the querying user.

#### `internal/db/opml.go`

```go
func ExportOPML(ctx context.Context, d *sql.DB, userID int64) ([]byte, error)
func ImportOPML(ctx context.Context, d *sql.DB, userID int64, data []byte, now int64) (imported, skipped int, errs []string, err error)
```

`ExportOPML` generates OPML 2.0 XML. Subscriptions with a `category_id` are grouped under a `<outline type="folder">` element named after the category. Uncategorised subscriptions appear as top-level `<outline type="rss|atom|json">` elements.

`ImportOPML` is transactional:
1. Parse the OPML document.
2. For each top-level folder outline: upsert a category by name (INSERT OR IGNORE; fetch the existing row if name already exists). Child feed outlines go under that category.
3. Deeper nesting (folder inside folder): collapse to immediate parent category, no warning.
4. For each feed outline: `INSERT OR IGNORE INTO subscriptions` with `next_poll_at = 0` so the poller picks it up immediately. Duplicate feed URLs (already subscribed) increment `skipped`. Per-item errors (malformed URL, SSRF-blocked) append to `errs` and do not abort.
5. All category inserts and non-erroring subscription inserts commit in one transaction.

`errs` are strings suitable for display to the user (e.g. `"skipped 'http://bad': invalid URL"`).

### API endpoints

All require a valid session. State-changing routes require CSRF. DTOs are explicit per-project convention. New stable error codes: `query_too_short`, `no_feeds_found`, `category_not_found`, `category_name_taken`.

#### Categories — new file `internal/api/categories.go`

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/categories` | session | List caller's categories with unread counts |
| `POST` | `/api/v1/categories` | session + CSRF | Create. Body: `{"name": string}`. >1 MiB → 413. |
| `PATCH` | `/api/v1/categories/:id` | session + CSRF | Rename. Body: `{"name": string}`. |
| `DELETE` | `/api/v1/categories/:id` | session + CSRF | Delete (feeds become uncategorised via FK). |
| `POST` | `/api/v1/categories/:id/mark-read` | session + CSRF | Bulk mark all unread entries in category as read. |

`categoryDTO`:
```go
type categoryDTO struct {
    ID        int64  `json:"id"`
    Name      string `json:"name"`
    Unread    int    `json:"unread"`
    CreatedAt int64  `json:"created_at"`
}
```

#### Subscriptions — extensions to `internal/api/subscriptions.go`

- `subscriptionDTO` gains `CategoryID *int64 \`json:"category_id"\`` (nullable).
- `POST /api/v1/subscriptions` body accepts optional `"category_id": int|null`.
- `PATCH /api/v1/subscriptions/:id` body accepts optional `"category_id": int|null` (omit = no change; `null` = uncategorise).
- `GET /api/v1/entries` gains `?category=<id>` filter (returns entries from all subscriptions in that category belonging to the caller).

#### Search — new file `internal/api/search.go`

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/search?q=<term>&limit=N&cursor=<id>` | session | FTS5 search across caller's entries |

- `q` shorter than 3 characters → `400 query_too_short`.
- `limit` default 50, max 200.
- Response shape mirrors `GET /api/v1/entries`: `{"data": [...], "next_cursor": int|null}`.
- Entry body is stripped from list results (same convention as entries list).

#### OPML — new file `internal/api/opml.go`

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/opml` | session | Export OPML 2.0. `Content-Type: text/x-opml; charset=utf-8`. |
| `POST` | `/api/v1/opml` | session + CSRF | Import OPML. Body cap: **10 MiB** (per-route override of the default 1 MiB cap). |

`POST /api/v1/opml` response body:
```json
{"imported": 12, "skipped": 2, "errors": ["skipped 'http://bad': invalid URL"]}
```

`imported` counts newly created subscriptions. `skipped` counts duplicate URLs. `errors` lists per-item failures that were non-fatal.

#### Discovery — new file `internal/api/discover.go`

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/discover` | session + CSRF | Discover feed candidates from a URL |

Body: `{"url": string}`. >1 MiB → 413.

Response on success:
```json
{
  "candidates": [
    {"title": "Example Blog", "feed_url": "https://example.com/feed.xml", "site_url": "https://example.com", "type": "atom"}
  ]
}
```

Response when no feeds found: `400 no_feeds_found` with an empty `candidates` array in the body for the SPA to switch on.

#### GET by subscription ID

`GET /api/v1/subscriptions/:id` — noted as deferred in the M6 spec ("no SPA caller"). M9's add-feed/edit-feed flow needs to load a single subscription (e.g. to show the category assignment in the edit form). This endpoint lands in M9.

### SPA changes

#### New routes (added to `web/src/lib/router.ts`)

- `/search` — search view; reads `?q=` on mount.
- `/categories/:id` — category entry list view.

#### Sidebar (`web/src/views/Unread.svelte` + sidebar component)

The FEEDS group becomes a category-grouped layout:

- Each category row shows: category name + unread count. Feeds nested under it show their individual unread counts.
- Uncategorised feeds appear under an implicit "Uncategorised" group at the bottom of the list.
- Clicking a category name navigates to `/categories/:id`.
- Inline category management:
  - A "+" button next to the section header opens an inline text input; blur or Enter saves (calls `POST /api/v1/categories`).
  - A "..." button on hover over a category row shows a menu with Rename (inline edit) and Delete (confirmation dialog — delete uncategorises feeds, shown in the dialog text).

#### `web/src/views/Search.svelte` (new)

- Single search input, auto-focused on mount.
- Reads `?q=` from URL on mount and pre-populates the input.
- Debounced 300 ms; fires `searchEntries(q)` only when `q.length >= 3`. Below 3 chars: renders empty state with hint "Type at least 3 characters to search."
- Results render using the existing `EntryRow` component.
- URL updated via `history.replaceState` on each debounce tick so the query is bookmarkable.
- No cursor pagination in M9 — returns up to 50 results.

#### `web/src/views/Category.svelte` (new)

- Fetches `GET /api/v1/entries?category=<id>` with unread filter and cursor pagination.
- Top bar: category name, unread count, "Mark all read" button (fires `POST /api/v1/categories/:id/mark-read` after a confirmation, then refreshes the list).
- Entry list, read/save toggles, infinite scroll — identical behaviour to the Unread view.

#### Add-feed flow (update to existing inline form)

- Input accepts any URL (feed URL or page URL).
- On submit: `POST /api/v1/discover`.
  - Single candidate whose `feed_url` matches the input exactly → subscribe immediately, skip the picker.
  - Multiple candidates → list the candidates for the user to choose one.
  - `no_feeds_found` → display "No feeds found at that URL. Try pasting the feed URL directly."
- After choosing a feed: show a small form with optional Title override and a Category dropdown (populated from `GET /api/v1/categories`). Subscribe on confirm.

#### `/` keybinding

Pressing `/` outside a form control focuses the search input. If the current route is not `/search`, navigate there first (preserving any existing `?q=` if the input is non-empty). This keybinding is registered entirely in M9 — M8's keyboard shortcut surface does not include it, not even as a stub or no-op. There is no `focusSearch()` interface contract in M8 that M9 implements; M9 adds the binding from scratch alongside the search feature it targets.

#### `web/src/lib/api.ts` extensions

```ts
listCategories(): Promise<Category[]>
createCategory(name: string): Promise<Category>
renameCategory(id: number, name: string): Promise<Category>
deleteCategory(id: number): Promise<void>
markCategoryRead(id: number): Promise<void>

searchEntries(q: string, limit?: number, cursor?: number): Promise<EntryListResponse>

exportOPML(): Promise<Blob>                      // GET /api/v1/opml
importOPML(data: ArrayBuffer): Promise<OPMLImportResult>  // POST /api/v1/opml

discoverFeeds(url: string): Promise<DiscoverResult>

getSubscription(id: number): Promise<Subscription>  // GET /api/v1/subscriptions/:id (new)
```

#### `web/src/lib/types.ts` additions

```ts
type Category = { id: number; name: string; unread: number; created_at: number }
type DiscoverCandidate = { title: string; feed_url: string; site_url: string; type: string }
type DiscoverResult = { candidates: DiscoverCandidate[] }
type OPMLImportResult = { imported: number; skipped: number; errors: string[] }
```

`Subscription` gains `category_id: number | null`.

### Configuration

No new flags. The OPML 10 MiB cap is a hard-coded per-route constant in `internal/api/opml.go`, not a configurable knob. YAGNI.

### `tap_strip_html` registration

`internal/db/db.go`'s `Open` function registers `tap_strip_html` as a SQLite scalar function before returning the `*sql.DB`. It accepts one string argument and returns the plain text of the HTML (tags stripped, entities decoded). A pure-Go implementation using `golang.org/x/net/html` (already a dependency from M2) is sufficient — walk the parse tree, concatenate text nodes, join with spaces. This is not a sanitiser; it only needs to produce clean plain text for indexing.

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| Nested categories / category hierarchy | Won't ship — Tap categories are intentionally flat |
| Per-category entry retention horizon | Future — M11 does not include per-category horizon preferences (M11 ships only a global `--archive-horizon` flag). Requires a new column on `categories` and sweep query changes; not yet sequenced. |
| OPML import progress stream / async job | Won't ship — synchronous import with per-item error list is sufficient |
| Well-known path probing on discovery (`/feed`, `/rss`) | Deferred — add when a real user case surfaces |
| Search snippets / highlighted matches | Deferred — FTS5 snippet() function can be added later |
| Search pagination beyond 50 results | Deferred |
| Feed icon fetching | Separate concern, not sequenced |
| Per-user iframe-host allowlist | Deferred items table in roadmap.md |
| `GET /api/v1/subscriptions/:id` beyond M9 SPA needs | Lands in M9; any further extension is future work |

## Cross-spec touch-points

- **M7** — hard dependency. M9 requires M7's `user_id INTEGER NOT NULL` on `subscriptions` and `entries`. All M9 db-layer functions that touch subscriptions take a `userID int64` parameter following the M7 pattern.
- **M8** — the `/` focus-search keybinding is absent from M8's keyboard shortcut surface. It lands in M9. M8's "Out of scope" table documents this explicitly.
- **M10** — OPML import triggers subscription inserts with `next_poll_at = 0`. The service worker must not cache `POST /api/v1/opml` responses. Search (`GET /api/v1/search?q=`) should not be pre-warmed by the warm-cache driver.
- **M11** — the `entries_fts_delete` trigger keeps the FTS index consistent when M11's archival sweep deletes from `entries`. M11 must not add any explicit FTS sync call. M11 must include a regression test: archival delete → entry no longer returned by FTS search.
- **M12** — no new observability surface added in M9 beyond what the existing request logging covers. FTS query latency will be visible in M12's request histogram.

## Tests and methodology

M9 follows TDD per `docs/roadmap.md`. Exemptions: migration SQL, OPML XML template, flag/constant declarations, pure Svelte component templates without logic.

### `internal/discover/discover_test.go`

- URL that is a valid Atom feed → single candidate, no page fetch performed.
- URL that is an HTML page with `<link rel="alternate">` elements → all candidates returned.
- URL with no feed links → `ErrNoFeeds`.
- `type="application/feed+json"` and `type="application/json"` both produce `type: "json"` candidates.
- Discovery fetch goes through SSRF-aware client: private-IP URL → error propagated, not panic.
- Discovery never sends `Cookie` or `Authorization` headers, even when called with a client that has a transport.

### `internal/db/categories_test.go`

- `InsertCategory` + `ListCategories` roundtrip; `Unread` count correct.
- Duplicate name for same user → `ErrCategoryNameTaken`.
- Same name for different users → allowed (uniqueness is per-user).
- `GetCategory` with wrong `userID` → `ErrCategoryNotFound`.
- `DeleteCategory` sets `subscriptions.category_id = NULL` (FK SET NULL verified).
- `MarkCategoryRead` marks only entries belonging to subscriptions in that category for that user; other users' entries untouched.

### `internal/db/search_test.go`

- Insert entry → searchable via FTS.
- **Delete entry → no longer searchable** (pins `entries_fts_delete` trigger; this test is the contract M11 relies on).
- Update entry title → new title searchable, old title not.
- Search is scoped to `userID`: user A's entries are not visible to user B.
- HTML tags do not appear in search results (pins `tap_strip_html`).
- Author field is searchable.
- BM25 ranking: entry with query term in title ranks above entry with term only in body.
- Cursor pagination: second page does not overlap with first.

### `internal/db/opml_test.go`

- `ExportOPML`: valid OPML 2.0 XML; feeds grouped under category outlines; uncategorised feeds as top-level outlines.
- `ImportOPML`: flat feeds imported uncategorised.
- One-level nested folder → category created + feeds assigned to it.
- Two-level nesting → flattened to immediate parent category.
- Duplicate feed URL → counted in `skipped`, not in `imported`.
- Invalid URL in outline → counted in `errors`, does not abort import.
- Idempotent on re-import (duplicate feed URLs all skipped second time).
- `ImportOPML` is transactional: a fatal parse error rolls back partial inserts.

### `internal/api/categories_test.go`

- Full CRUD happy paths.
- Missing session → 401.
- Duplicate name → 400 `category_name_taken`.
- Cross-user: user A cannot GET / PATCH / DELETE user B's category (returns 404 `category_not_found`).
- `POST /categories/:id/mark-read` marks only the correct user's entries.

### `internal/api/search_test.go`

- `?q=` below 3 chars → 400 `query_too_short`.
- Valid query → matching entries in response.
- Cross-user: results scoped to caller's subscriptions only.
- Missing session → 401.

### `internal/api/opml_test.go`

- `GET /api/v1/opml` → valid OPML, correct `Content-Type`.
- `POST /api/v1/opml` > 10 MiB body → 413.
- Valid import → `{imported, skipped, errors}` shape.
- Unauthenticated → 401.

### `internal/api/discover_test.go`

- Valid page URL with `<link rel="alternate">` → candidates array.
- No feeds → 400 `no_feeds_found`.
- Malformed URL → 400.
- Unauthenticated → 401.

### `internal/api/subscriptions_test.go` (extended)

- `POST` with `category_id` → persisted; `GET` returns `category_id`.
- `PATCH` with `category_id: null` → uncategorised.
- `PATCH` with `category_id` omitted → unchanged.
- `GET /api/v1/entries?category=<id>` → only entries from subscriptions in that category for that user.
- `GET /api/v1/subscriptions/:id` → returns single subscription including `category_id`.

### `web/src/lib/__tests__/api.test.ts` (extended)

- `listCategories`, `createCategory`, `renameCategory`, `deleteCategory`, `markCategoryRead`.
- `searchEntries`.
- `discoverFeeds`: candidates array on success; `no_feeds_found` error on empty.
- `exportOPML`: returns a `Blob`.
- `importOPML`: posts body, returns `OPMLImportResult`.
- `getSubscription`.

### `web/src/views/__tests__/Search.test.ts` (new)

- Query below 3 chars: no API request fired.
- Query ≥ 3 chars: request fired after debounce interval.
- URL updated with `?q=` on each debounce tick.
- Pre-populated from `?q=` on mount.
- Results rendered using `EntryRow`.

### `web/src/views/__tests__/Category.test.ts` (new)

- Entry list fetched with `?category=<id>`.
- "Mark all read" button fires `markCategoryRead` after confirmation.
- Empty state rendered when no entries.

## Risks and open questions

- **`tap_strip_html` in triggers.** Custom SQLite functions registered in Go must be registered before any SQL that uses them executes — including trigger bodies. `db.Open` registers `tap_strip_html` before returning, so the function is available for the migration back-fill and for all subsequent trigger executions. This is correct as long as the same `*sql.DB` connection pool is used throughout. The in-memory DB tests use the same `db.Open` path, so they exercise this registration.

- **FTS5 contentless table and snippet().** A contentless FTS5 table (`content=''`) does not support the `snippet()` or `highlight()` auxiliary functions — those require the original content to be stored. If search snippets are added in a future milestone, the options are: (a) switch to `content='entries'` (stores a copy in FTS — doubles storage for content), or (b) fetch the snippet from `entries.content` after the FTS query and extract it in Go. Option (b) is the right call when the time comes; do not add content storage now.

- **FTS index size.** With `unicode61` tokenisation and three columns (title, stripped content, author), the FTS index will be a meaningful fraction of the entries table size. For a user with 50,000 entries averaging 5 KB of stripped content, the FTS index could be 50–150 MB. This is acceptable for a self-hosted single-user app; document it in the README so operators are not surprised.

- **OPML import and the poller.** Importing a large OPML file with 200 feeds sets `next_poll_at = 0` on all 200 subscriptions simultaneously. The next scheduler tick will dispatch up to the worker pool limit at once; the remainder wait for the next tick. The per-host concurrency cap prevents bursts against any single origin. No further throttling is needed for M9.

- **Discovery and SSRF.** `Discover` calls the shared `httpx.NewClient` which enforces the SSRF policy. A user who pastes `http://192.168.1.1/feed` will get an SSRF error, not a feed. The API returns `400` with a clear message; the SPA displays it as a fetch error rather than `no_feeds_found`.

- **Category name collisions on OPML import.** If the user already has a category named "Tech" and the OPML being imported also has a "Tech" folder, `ImportOPML` upserts by name (INSERT OR IGNORE, then fetches the existing row). The imported feeds are added to the existing "Tech" category. This is the correct behaviour — categories are identified by name within a user's namespace.

- **`GET /api/v1/subscriptions/:id` authorization.** The new endpoint must include `user_id = ?` in its WHERE clause, consistent with all other M7+ subscription queries. Attempting to fetch another user's subscription returns 404 `subscription_not_found` (not 403 — avoid enumeration).

## Definition of done

1. `make test` passes (`go test ./... -race`) including the new packages, both migrations, all API extensions, the discover package, and the SPA tests.
2. Fresh M8 database: OPML import of a real multi-category OPML file creates categories, subscribes feeds, and the poller picks up new feeds within one tick.
3. Search: typing a term from a known entry title returns that entry; typing fewer than 3 characters shows no results and fires no request.
4. Add-feed discovery: pasting a page URL with `<link rel="alternate">` returns candidates; pasting a direct feed URL subscribes immediately.
5. Category sidebar: feeds grouped under categories; uncategorised feeds at bottom; creating a category inline works; deleting a category leaves its feeds uncategorised.
6. Bulk mark-read by category: "Mark all read" in the category view marks the correct entries and updates the unread count in the sidebar.
7. OPML export: `GET /api/v1/opml` returns valid OPML 2.0 importable by Miniflux and NetNewsWire.
8. `/` keybinding focuses search from any non-form context.
9. Migration 0008 and 0009 apply cleanly against an M8 database; existing subscriptions get `category_id = NULL`; existing entries are indexed in FTS.
10. `make build` produces a static binary that boots cleanly against a fresh database and against an existing M8 database.

## What this milestone deliberately does not prove

- That categories have a hierarchy or nesting — they are intentionally flat.
- That search returns snippets or highlighted matches — contentless FTS5 does not support this without additional work.
- That OPML import is async or streamed — synchronous with per-item errors is sufficient.
- That per-feed icons are fetched — separate concern, not sequenced.
- That the `/` keybinding exists in M8 — it does not; it lands here.
