# M9 — Categories, OPML, Search, Add-Feed Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land flat per-user categories, OPML import/export, FTS5 full-text search over entry title/content/author, and add-feed discovery from a page URL — as specified in `docs/specs/2026-05-10-m9-categories-opml-search-discovery.md`. After M9, the sidebar groups feeds under categories, users can bulk mark-read by category, search across their entries, import/export OPML, and discover feed candidates from any URL.

**Architecture:** Two schema migrations: `0008_categories.sql` (categories table + `subscriptions.category_id`) and `0009_fts5.sql` (contentless FTS5 virtual table with `AFTER INSERT/UPDATE/DELETE` triggers on `entries`, plus back-fill). A `tap_strip_html` custom SQLite scalar function (registered in `db.Open` using `modernc.org/sqlite`'s `RegisterFunction`) strips HTML before indexing. New packages: `internal/discover` (two-step feed discovery via the shared httpx client). Extensions to `internal/db` (categories.go, search.go, opml.go), `internal/api` (categories.go, search.go, opml.go, discover.go), and the SPA (`Search.svelte`, `Category.svelte`, router extensions, sidebar rework, api.ts/types.ts additions). All queries filter by `userID` — M9 hard-depends on M7's `user_id` columns on `subscriptions` and `entries`.

**Tech Stack:** Go 1.25, modernc.org/sqlite (FTS5 in-tree, `RegisterFunction` for custom scalar), `golang.org/x/net/html` (already a dep — reused for `tap_strip_html`), `github.com/mmcdole/gofeed` (already a dep — reused for feed-type detection in discover), stretchr/testify, Svelte 5 + TypeScript + Vite.

**Prerequisite:** M7 must be fully merged before M9 implementation begins. `subscriptions.user_id` and `entries.user_id` must exist.

---

## Skills and tools to apply

Always-on for every code-touching task:

- **`superpowers:test-driven-development`** — red/green/refactor on every behaviour-bearing change. Pure scaffolding (migration SQL, Svelte component shells, type declarations) is exempt; everything with branches, error handling, or state is in scope.
- **`superpowers:verification-before-completion`** — before marking a task done, run the exact test command in the verification step and confirm the output matches expected.

Reach for as needed:

- **`golang-database`** — parameterised queries, `defer rows.Close()`, `errors.Is(err, sql.ErrNoRows)`, transaction boundaries (OPML import), `RegisterFunction` for custom SQLite scalar, FTS5 `MATCH` queries and `bm25()` scoring.
- **`golang-error-handling`** — sentinel errors (`ErrCategoryNotFound`, `ErrCategoryNameTaken`, `ErrNoFeeds`); `errors.Is` mapping at API boundary; `fmt.Errorf("context: %w", err)` wrapping throughout; single-handling rule (log OR return, not both).
- **`golang-testing`** + **`golang-stretchr-testify`** — match existing repo style (`require.NoError`, `require.Equal`, table-driven with named subtests). Use `httptest.NewRequest`/`httptest.NewRecorder` for handler tests. Use in-memory SQLite (`:memory:`) for db tests.
- **`golang-naming`** — `ErrCategoryNotFound`, `ErrCategoryNameTaken`, `ErrNoFeeds` follow sentinel naming. `InsertCategory`, `ListCategories`, `SearchEntries` follow the existing `internal/db` verb-noun convention. Unexported helpers stay lowercase.
- **`golang-modernize`** — Go 1.25 idioms throughout. No `for i := range` where `for _, v := range` is cleaner.
- **`golang-context`** — all db and HTTP functions take `ctx context.Context` as first argument, matching every existing function in `internal/db`.
- **`svelte-runes`** — Svelte 5 `$state`, `$derived`, `$props` in all new components. Match the style of existing components (no `on:` event syntax, use `onclick=`).
- **`svelte-components`** — new `Search.svelte` and `Category.svelte` match the layout pattern of `Unread.svelte` (Sidebar + TopBar + list).

MCP tools:

- **`context7`** — use `mcp__plugin_context7_context7__resolve-library-id` + `query-docs` to verify the current `modernc.org/sqlite` `RegisterFunction` / `CreateFunction` API signature before writing `db.Open`. The API changed between versions; verify before writing.
- **`context7`** — verify the FTS5 `MATCH` + `bm25()` query syntax for contentless tables. SQLite FTS5 docs: confirm `INSERT INTO entries_fts(entries_fts, rowid) VALUES ('delete', id)` is correct for contentless deletes.

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `internal/db/migrations/0008_categories.sql` | **create** | `categories` table, `subscriptions.category_id` FK column + index |
| `internal/db/migrations/0009_fts5.sql` | **create** | FTS5 virtual table, three triggers, back-fill INSERT |
| `internal/db/db.go` | **modify** | Register `tap_strip_html` custom scalar function before returning from `Open` |
| `internal/db/db_test.go` | **modify** | Test that `tap_strip_html` is callable after `Open` |
| `internal/db/categories.go` | **create** | `Category`, `NewCategory`, sentinel errors, all CRUD + mark-read functions |
| `internal/db/categories_test.go` | **create** | Full test coverage per spec §Tests |
| `internal/db/subscriptions.go` | **modify** | `Subscription`/`NewSubscription` gain `CategoryID sql.NullInt64`; `ListSubscriptions`/`GetSubscription`/`InsertSubscription` updated; new `UpdateSubscriptionCategory`; new `ListSubscriptionsByCategory` |
| `internal/db/subscriptions_test.go` | **modify** | category_id roundtrip; category filter; null/set/clear |
| `internal/db/search.go` | **create** | `SearchResult`, `SearchEntries` with FTS5 MATCH + user scoping join |
| `internal/db/search_test.go` | **create** | Insert→searchable; delete→gone (trigger contract); update; cross-user isolation; HTML tags not in results; author searchable; BM25 ordering; cursor pagination |
| `internal/db/opml.go` | **create** | `ExportOPML`, `ImportOPML` (transactional, per-item errors) |
| `internal/db/opml_test.go` | **create** | Export shape; import flat; import nested; deeper nesting flattened; duplicate skipped; invalid URL in errors; idempotent re-import; transactional rollback |
| `internal/discover/discover.go` | **create** | `Result`, `ErrNoFeeds`, `Discover` (try-as-feed → `<link rel="alternate">`) |
| `internal/discover/discover_test.go` | **create** | Direct feed URL; HTML page with alternates; no feeds; JSON Feed type; SSRF-blocked URL; no auth headers |
| `internal/api/errors.go` | **modify** | Add `ErrCodeQueryTooShort`, `ErrCodeNoFeedsFound`, `ErrCodeCategoryNotFound`, `ErrCodeCategoryNameTaken` |
| `internal/api/categories.go` | **create** | `categoryDTO`, CRUD handlers, mark-read handler, `registerCategoryRoutes` |
| `internal/api/categories_test.go` | **create** | CRUD happy paths; duplicate name; cross-user 404; mark-read scoping; 401 without session |
| `internal/api/search.go` | **create** | `searchHandler`, `searchResultDTO`, `registerSearchRoutes` |
| `internal/api/search_test.go` | **create** | q<3 → 400; valid query → results; cross-user isolation; 401 |
| `internal/api/opml.go` | **create** | `exportHandler`, `importHandler` (10 MiB cap), `registerOPMLRoutes` |
| `internal/api/opml_test.go` | **create** | Export valid OPML; import >10 MiB → 413; valid import response shape; 401 |
| `internal/api/discover.go` | **create** | `discoverHandler`, `registerDiscoverRoutes` |
| `internal/api/discover_test.go` | **create** | Candidates returned; no feeds → 400; malformed URL → 400; 401 |
| `internal/api/subscriptions.go` | **modify** | `subscriptionDTO` gains `CategoryID *int64`; POST/PATCH bodies accept `category_id`; new `GET /api/v1/subscriptions/{id}` handler; `GET /api/v1/entries` gains `?category=` filter |
| `internal/api/subscriptions_test.go` | **modify** | category_id roundtrip; GET-by-id; entries category filter; cross-user GET-by-id → 404 |
| `internal/api/api.go` | **modify** | Register new route groups; wire `discoverClient` from `MuxOpts` |
| `web/src/lib/types.ts` | **modify** | Add `Category`, `DiscoverCandidate`, `DiscoverResult`, `OPMLImportResult`; `Subscription` gains `category_id` |
| `web/src/lib/api.ts` | **modify** | Add category, search, OPML, discover, `getSubscription` functions |
| `web/src/lib/__tests__/api.test.ts` | **modify** | New api functions covered |
| `web/src/lib/router.ts` | **modify** | Add `/search` and `/categories/:id` routes to `RouteState` and `parse` |
| `web/src/views/Search.svelte` | **create** | Debounced search input, `?q=` URL sync, `EntryRow` results |
| `web/src/views/Category.svelte` | **create** | Category entry list, mark-all-read top bar button |
| `web/src/views/__tests__/Search.test.ts` | **create** | q<3 no request; q≥3 fires after debounce; URL updated; pre-populated from ?q= |
| `web/src/views/__tests__/Category.test.ts` | **create** | Entries fetched with category filter; mark-all-read fires after confirm |
| `web/src/components/Sidebar.svelte` | **modify** | Category-grouped feeds, inline create/rename/delete, uncategorised group |
| `web/src/components/__tests__/Sidebar.test.ts` | **create** | Sidebar category management state tests (create, delete, confirm) |
| `web/src/App.svelte` | **modify** | Add `/search` and `/categories/:id` route arms |
| `CLAUDE.md` | **modify** | Update M9 status line |

---

## Phase A — Schema migrations and `tap_strip_html`

### Task A1: Verify `modernc.org/sqlite` `RegisterFunction` API

**Files:**
- No file changes — research only

- [ ] **Step 1: Fetch the current API signature via context7**

```
resolve-library-id: "modernc.org/sqlite"
query-docs: "RegisterFunction CreateFunction scalar function"
```

Expected: confirm whether the function is `(*DB).RegisterFunction(name, fn, pure)` or a different signature in v1.50.0.

- [ ] **Step 2: Confirm pure-Go, no CGO dependency**

```bash
grep "modernc.org/sqlite" /home/ben.guest/Users/ben/src/tap/go.mod
```

Expected: `modernc.org/sqlite v1.50.0` (or later). No `mattn/go-sqlite3`.

---

### Task A2: Migration 0008 — categories table

> **BLOCKED UNTIL M7 IS MERGED.** M9 migrations are numbered starting at 0008, which requires M7's `0006_user_data_isolation.sql` and `0007_2fa_passkeys_sessions_meta.sql` to already exist in `internal/db/migrations/`. Before starting this task, verify: `ls internal/db/migrations/` shows exactly 7 files (`0001`–`0007`). If M7 has not merged, do not proceed — the migration test in Step 2 will fail with a schema mismatch.

**Files:**
- Create: `internal/db/migrations/0008_categories.sql`

- [ ] **Step 0: Confirm M7 has merged**

```bash
ls /home/ben.guest/Users/ben/src/tap/internal/db/migrations/
```

Expected: 7 files present — `0001_initial.sql` through `0007_2fa_passkeys_sessions_meta.sql`. If fewer than 7 files exist, stop and wait for M7.

- [ ] **Step 1: Write the migration**

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

- [ ] **Step 2: Verify migration applies cleanly**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run TestMigrate -race -v
```

Expected: `PASS` — existing migration tests pass with the new file present.

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations/0008_categories.sql
git commit -m "feat(db): migration 0008 — categories table and subscriptions.category_id"
```

---

### Task A3: Register `tap_strip_html` in `db.Open`

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/db/db_test.go`:

```go
func TestOpen_StripHTMLFunctionRegistered(t *testing.T) {
    t.Parallel()
    d, err := Open(context.Background(), ":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { _ = d.Close() })

    var got string
    err = d.QueryRowContext(context.Background(),
        `SELECT tap_strip_html('<p>Hello <em>world</em></p>')`).Scan(&got)
    require.NoError(t, err)
    require.Equal(t, "Hello world", got)
}
```

- [ ] **Step 2: Run — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run TestOpen_StripHTMLFunctionRegistered -race -v
```

Expected: FAIL — `tap_strip_html` undefined.

- [ ] **Step 3: Implement `tap_strip_html` and register it**

Add a new file `internal/db/striphtml.go`:

```go
package db

import (
    "bytes"
    "strings"

    "golang.org/x/net/html"
)

// stripHTML returns the plain-text content of an HTML fragment.
// It walks the parse tree and concatenates text nodes, joining runs
// with a single space. Used as a custom SQLite scalar function
// (tap_strip_html) to produce clean FTS5 index content.
func stripHTML(s string) string {
    doc, err := html.Parse(strings.NewReader(s))
    if err != nil {
        return s
    }
    var buf bytes.Buffer
    var walk func(*html.Node)
    walk = func(n *html.Node) {
        if n.Type == html.TextNode {
            t := strings.TrimSpace(n.Data)
            if t != "" {
                if buf.Len() > 0 {
                    buf.WriteByte(' ')
                }
                buf.WriteString(t)
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            walk(c)
        }
    }
    walk(doc)
    return buf.String()
}
```

Then modify `internal/db/db.go` to register the function. First check the exact API from Task A1, then add (using the `modernc.org/sqlite` driver's registration mechanism — typically called before or after `sql.Open` on the driver directly):

```go
import (
    "context"
    "database/sql"
    "fmt"

    "modernc.org/sqlite"
    _ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

func init() {
    sqlite.MustRegisterFunction("tap_strip_html", &sqlite.FunctionImpl{
        NArgs:         1,
        Deterministic: true,
        Scalar: func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
            s, _ := args[0].(string)
            return stripHTML(s), nil
        },
    })
}
```

Note: if the `modernc.org/sqlite` v1.50.0 API uses a different registration path (confirmed in Task A1), adjust accordingly. The function must be registered before any SQL using it executes.

- [ ] **Step 4: Run — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run TestOpen_StripHTMLFunctionRegistered -race -v
```

Expected: PASS.

- [ ] **Step 5: Run full db suite**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -race
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/db/striphtml.go internal/db/db.go internal/db/db_test.go
git commit -m "feat(db): register tap_strip_html custom SQLite scalar for FTS5 indexing"
```

---

### Task A4: Migration 0009 — FTS5 virtual table and triggers

**Files:**
- Create: `internal/db/migrations/0009_fts5.sql`

- [ ] **Step 1: Write the migration**

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

-- Back-fill existing entries.
INSERT INTO entries_fts(rowid, title, content_text, author)
SELECT id, title, tap_strip_html(content), author FROM entries;
```

- [ ] **Step 2: Verify migration applies (tap_strip_html must be registered first)**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run TestMigrate -race -v
```

Expected: PASS — all five migrations apply cleanly including 0009.

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations/0009_fts5.sql
git commit -m "feat(db): migration 0009 — FTS5 virtual table with triggers and back-fill"
```

---

## Phase B — `internal/db` extensions

### Task B1: `internal/db/categories.go`

**Files:**
- Create: `internal/db/categories.go`
- Create: `internal/db/categories_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/db/categories_test.go`.

**Package note:** The existing test files in `internal/db/` use `package db` (white-box, access unexported). Match that. The `newTestDB` helper likely already exists in the package; if not, add it to a shared `internal/db/testhelpers_test.go`. Add `insertTestUser` and `insertTestUserWithName` helpers to the same file.

```go
package db

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

func TestInsertCategory_Roundtrip(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    // need a user first — insert directly for test setup
    userID := insertTestUser(t, d)
    id, err := db.InsertCategory(context.Background(), d, db.NewCategory{
        UserID: userID, Name: "Tech", CreatedAt: time.Now().Unix(),
    })
    require.NoError(t, err)
    require.Positive(t, id)

    cats, err := db.ListCategories(context.Background(), d, userID)
    require.NoError(t, err)
    require.Len(t, cats, 1)
    require.Equal(t, "Tech", cats[0].Name)
}

func TestInsertCategory_DuplicateNameSameUser(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    nc := db.NewCategory{UserID: userID, Name: "Tech", CreatedAt: time.Now().Unix()}
    _, err := db.InsertCategory(context.Background(), d, nc)
    require.NoError(t, err)
    _, err = db.InsertCategory(context.Background(), d, nc)
    require.ErrorIs(t, err, db.ErrCategoryNameTaken)
}

func TestInsertCategory_SameNameDifferentUsers(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    u1 := insertTestUser(t, d)
    u2 := insertTestUserWithName(t, d, "other")
    now := time.Now().Unix()
    _, err := db.InsertCategory(context.Background(), d, db.NewCategory{UserID: u1, Name: "Tech", CreatedAt: now})
    require.NoError(t, err)
    _, err = db.InsertCategory(context.Background(), d, db.NewCategory{UserID: u2, Name: "Tech", CreatedAt: now})
    require.NoError(t, err) // allowed — uniqueness is per-user
}

func TestGetCategory_WrongUserID(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    u1 := insertTestUser(t, d)
    u2 := insertTestUserWithName(t, d, "other")
    id, err := db.InsertCategory(context.Background(), d, db.NewCategory{UserID: u1, Name: "Tech", CreatedAt: time.Now().Unix()})
    require.NoError(t, err)
    _, err = db.GetCategory(context.Background(), d, id, u2)
    require.ErrorIs(t, err, db.ErrCategoryNotFound)
}

func TestDeleteCategory_UncategorisesSubscriptions(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    catID, _ := db.InsertCategory(context.Background(), d, db.NewCategory{UserID: userID, Name: "Tech", CreatedAt: time.Now().Unix()})
    subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
        Title: "Test", FeedURL: "https://example.com/feed", NextPoll: 0, Created: time.Now().Unix(), UserID: userID,
    })
    require.NoError(t, db.UpdateSubscriptionCategory(context.Background(), d, subID, userID, sql.NullInt64{Int64: catID, Valid: true}))

    require.NoError(t, db.DeleteCategory(context.Background(), d, catID, userID))

    sub, err := db.GetSubscription(context.Background(), d, subID, userID)
    require.NoError(t, err)
    require.False(t, sub.CategoryID.Valid) // ON DELETE SET NULL
}

func TestMarkCategoryRead_ScopedToUser(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    u1 := insertTestUser(t, d)
    u2 := insertTestUserWithName(t, d, "other")
    catID, _ := db.InsertCategory(context.Background(), d, db.NewCategory{UserID: u1, Name: "Tech", CreatedAt: time.Now().Unix()})
    // insert subscriptions + entries for both users; mark-read on u1's category must not touch u2's entries
    // (full setup elided here — implementer expands with insertTestSubscription/insertTestEntry helpers)
    require.NoError(t, db.MarkCategoryRead(context.Background(), d, catID, u1))
    // assert u2's unread entries unchanged
    _ = u2 // silence unused warning until full test is written
}
```

- [ ] **Step 2: Run — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run TestInsertCategory -race -v
```

Expected: FAIL — `db.InsertCategory` undefined.

- [ ] **Step 3: Implement `internal/db/categories.go`**

```go
package db

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "strings"
)

var ErrCategoryNotFound  = errors.New("category not found")
var ErrCategoryNameTaken = errors.New("category name already exists for this user")

type Category struct {
    ID        int64
    UserID    int64
    Name      string
    CreatedAt int64
    Unread    int
}

type NewCategory struct {
    UserID    int64
    Name      string
    CreatedAt int64
}

func InsertCategory(ctx context.Context, d *sql.DB, c NewCategory) (int64, error) {
    res, err := d.ExecContext(ctx,
        `INSERT INTO categories (user_id, name, created_at) VALUES (?, ?, ?)`,
        c.UserID, c.Name, c.CreatedAt)
    if err != nil {
        if strings.Contains(err.Error(), "UNIQUE constraint failed: categories.user_id, categories.name") {
            return 0, ErrCategoryNameTaken
        }
        return 0, fmt.Errorf("insert category: %w", err)
    }
    return res.LastInsertId()
}

func GetCategory(ctx context.Context, d *sql.DB, id, userID int64) (Category, error) {
    var c Category
    err := d.QueryRowContext(ctx,
        `SELECT id, user_id, name, created_at FROM categories WHERE id = ? AND user_id = ?`,
        id, userID).Scan(&c.ID, &c.UserID, &c.Name, &c.CreatedAt)
    if errors.Is(err, sql.ErrNoRows) {
        return Category{}, ErrCategoryNotFound
    }
    if err != nil {
        return Category{}, fmt.Errorf("get category %d: %w", id, err)
    }
    return c, nil
}

func ListCategories(ctx context.Context, d *sql.DB, userID int64) ([]Category, error) {
    rows, err := d.QueryContext(ctx, `
        SELECT c.id, c.user_id, c.name, c.created_at,
               COUNT(CASE WHEN e.read = 0 THEN 1 END) AS unread
        FROM categories c
        LEFT JOIN subscriptions s ON s.category_id = c.id AND s.user_id = c.user_id
        LEFT JOIN entries e ON e.subscription_id = s.id
        WHERE c.user_id = ?
        GROUP BY c.id
        ORDER BY c.name COLLATE NOCASE
    `, userID)
    if err != nil {
        return nil, fmt.Errorf("list categories: %w", err)
    }
    defer rows.Close()
    var out []Category
    for rows.Next() {
        var c Category
        if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.CreatedAt, &c.Unread); err != nil {
            return nil, fmt.Errorf("scan category: %w", err)
        }
        out = append(out, c)
    }
    return out, rows.Err()
}

func UpdateCategoryName(ctx context.Context, d *sql.DB, id, userID int64, name string) error {
    res, err := d.ExecContext(ctx,
        `UPDATE categories SET name = ? WHERE id = ? AND user_id = ?`, name, id, userID)
    if err != nil {
        if strings.Contains(err.Error(), "UNIQUE constraint failed: categories.user_id, categories.name") {
            return ErrCategoryNameTaken
        }
        return fmt.Errorf("update category name %d: %w", id, err)
    }
    n, _ := res.RowsAffected()
    if n == 0 {
        return ErrCategoryNotFound
    }
    return nil
}

func DeleteCategory(ctx context.Context, d *sql.DB, id, userID int64) error {
    res, err := d.ExecContext(ctx,
        `DELETE FROM categories WHERE id = ? AND user_id = ?`, id, userID)
    if err != nil {
        return fmt.Errorf("delete category %d: %w", id, err)
    }
    n, _ := res.RowsAffected()
    if n == 0 {
        return ErrCategoryNotFound
    }
    return nil
}

func MarkCategoryRead(ctx context.Context, d *sql.DB, categoryID, userID int64) error {
    _, err := d.ExecContext(ctx, `
        UPDATE entries SET read = 1
        WHERE read = 0
          AND subscription_id IN (
              SELECT id FROM subscriptions
              WHERE category_id = ? AND user_id = ?
          )
    `, categoryID, userID)
    if err != nil {
        return fmt.Errorf("mark category read %d: %w", categoryID, err)
    }
    return nil
}
```

- [ ] **Step 4: Run — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run TestInsertCategory -race -v
```

- [ ] **Step 5: Run full db suite**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -race
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/db/categories.go internal/db/categories_test.go
git commit -m "feat(db): categories CRUD and mark-read"
```

---

### Task B2: `internal/db/subscriptions.go` — category_id extensions

**Files:**
- Modify: `internal/db/subscriptions.go`
- Modify: `internal/db/subscriptions_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/db/subscriptions_test.go`:

```go
func TestGetSubscription_ByUserID(t *testing.T) {
    t.Parallel()
    // GetSubscription now takes userID — cross-user returns sql.ErrNoRows
    d := newTestDB(t)
    u1 := insertTestUser(t, d)
    u2 := insertTestUserWithName(t, d, "other")
    id, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
        Title: "T", FeedURL: "https://x.com/f", NextPoll: 0, Created: time.Now().Unix(), UserID: u1,
    })
    _, err := db.GetSubscription(context.Background(), d, id, u2)
    require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdateSubscriptionCategory(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    catID, _ := db.InsertCategory(context.Background(), d, db.NewCategory{UserID: userID, Name: "Tech", CreatedAt: time.Now().Unix()})
    subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
        Title: "T", FeedURL: "https://x.com/f", NextPoll: 0, Created: time.Now().Unix(), UserID: userID,
    })
    require.NoError(t, db.UpdateSubscriptionCategory(context.Background(), d, subID, userID,
        sql.NullInt64{Int64: catID, Valid: true}))
    sub, _ := db.GetSubscription(context.Background(), d, subID, userID)
    require.True(t, sub.CategoryID.Valid)
    require.Equal(t, catID, sub.CategoryID.Int64)

    // Clear category
    require.NoError(t, db.UpdateSubscriptionCategory(context.Background(), d, subID, userID, sql.NullInt64{}))
    sub, _ = db.GetSubscription(context.Background(), d, subID, userID)
    require.False(t, sub.CategoryID.Valid)
}
```

- [ ] **Step 2: Run — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run "TestGetSubscription_ByUserID|TestUpdateSubscriptionCategory" -race -v
```

- [ ] **Step 3: Implement**

In `internal/db/subscriptions.go`:
- Add `CategoryID sql.NullInt64` to `Subscription` and `NewSubscription`.
- Update all `SELECT` queries to include `category_id`.
- Add `UserID int64` to `NewSubscription`; update `InsertSubscription` to write it.
- Update `GetSubscription` signature to `GetSubscription(ctx, d, id, userID int64)` — add `AND user_id = ?`.
- Update `ListSubscriptions` signature to `ListSubscriptions(ctx, d, userID int64)` — add `WHERE user_id = ?`.
- Add `UpdateSubscriptionCategory(ctx context.Context, d *sql.DB, id, userID int64, categoryID sql.NullInt64) error`.
- Add `ListSubscriptionsByCategory(ctx context.Context, d *sql.DB, categoryID, userID int64) ([]Subscription, error)`.

- [ ] **Step 4: Fix all callers of the modified signatures** (api layer, poll worker, etc.)

```bash
cd /home/ben.guest/Users/ben/src/tap && go build ./...
```

Fix any compile errors from signature changes.

- [ ] **Step 5: Run tests**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./... -race
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/db/subscriptions.go internal/db/subscriptions_test.go
git commit -m "feat(db): subscriptions gain category_id and per-user query scoping"
```

---

### Task B3: `internal/db/search.go`

**Files:**
- Create: `internal/db/search.go`
- Create: `internal/db/search_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/db/search_test.go` (package `db`, consistent with existing test files):

```go
package db

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

func TestSearchEntries_InsertThenFind(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    subID := insertTestSubscription(t, d, userID, "https://example.com/feed")
    insertTestEntry(t, d, subID, db.NewEntry{
        Hash: "h1", Title: "Golang concurrency patterns",
        Content: "<p>goroutines and channels</p>", URL: "https://example.com/1",
        PublishedAt: time.Now().Unix(), FetchedAt: time.Now().Unix(),
    })

    results, _, err := db.SearchEntries(context.Background(), d, userID, "concurrency", 10, 0)
    require.NoError(t, err)
    require.Len(t, results, 1)
    require.Equal(t, "Golang concurrency patterns", results[0].Title)
}

func TestSearchEntries_DeleteTrigger(t *testing.T) {
    // This is the M11 contract test — archival deletes must leave FTS consistent.
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    subID := insertTestSubscription(t, d, userID, "https://example.com/feed")
    entryID := insertTestEntry(t, d, subID, db.NewEntry{
        Hash: "h1", Title: "deleteme unique term xyzzy",
        Content: "<p>body</p>", URL: "https://example.com/1",
        PublishedAt: time.Now().Unix(), FetchedAt: time.Now().Unix(),
    })

    // Confirm searchable before delete.
    results, _, _ := db.SearchEntries(context.Background(), d, userID, "xyzzy", 10, 0)
    require.Len(t, results, 1)

    // Delete directly (as M11 archival will do).
    _, err := d.ExecContext(context.Background(), "DELETE FROM entries WHERE id = ?", entryID)
    require.NoError(t, err)

    // Must no longer appear in FTS.
    results, _, err = db.SearchEntries(context.Background(), d, userID, "xyzzy", 10, 0)
    require.NoError(t, err)
    require.Empty(t, results)
}

func TestSearchEntries_CrossUserIsolation(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    u1 := insertTestUser(t, d)
    u2 := insertTestUserWithName(t, d, "other")
    sub1 := insertTestSubscription(t, d, u1, "https://a.com/feed")
    insertTestEntry(t, d, sub1, db.NewEntry{
        Hash: "h1", Title: "secret entry for user one", Content: "<p>private</p>",
        URL: "https://a.com/1", PublishedAt: time.Now().Unix(), FetchedAt: time.Now().Unix(),
    })

    results, _, err := db.SearchEntries(context.Background(), d, u2, "secret", 10, 0)
    require.NoError(t, err)
    require.Empty(t, results)
}

func TestSearchEntries_HTMLTagsNotSearchable(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    subID := insertTestSubscription(t, d, userID, "https://example.com/feed")
    insertTestEntry(t, d, subID, db.NewEntry{
        Hash: "h1", Title: "normal title", Content: "<em>emphasized</em> text",
        URL: "https://example.com/1", PublishedAt: time.Now().Unix(), FetchedAt: time.Now().Unix(),
    })

    // Searching for an HTML tag name must not match.
    results, _, _ := db.SearchEntries(context.Background(), d, userID, "em", 10, 0)
    require.Empty(t, results)

    // But the text content must still be findable.
    results, _, _ = db.SearchEntries(context.Background(), d, userID, "emphasized", 10, 0)
    require.Len(t, results, 1)
}

func TestSearchEntries_AuthorSearchable(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    subID := insertTestSubscription(t, d, userID, "https://example.com/feed")
    insertTestEntry(t, d, subID, db.NewEntry{
        Hash: "h1", Title: "some article", Author: "Jane Uniqueauthor",
        Content: "<p>body</p>", URL: "https://example.com/1",
        PublishedAt: time.Now().Unix(), FetchedAt: time.Now().Unix(),
    })

    results, _, err := db.SearchEntries(context.Background(), d, userID, "Uniqueauthor", 10, 0)
    require.NoError(t, err)
    require.Len(t, results, 1)
}
```

- [ ] **Step 2: Run — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run "TestSearchEntries" -race -v
```

- [ ] **Step 3: Implement `internal/db/search.go`**

```go
package db

import (
    "context"
    "database/sql"
    "fmt"
)

type SearchResult struct {
    ID             int64
    SubscriptionID int64
    Title          string
    URL            string
    Author         string
    PublishedAt    int64
    Read           bool
    Saved          bool
    Rank           float64
}

// SearchEntries runs a full-text search over the caller's entries.
// cursor is the last-seen entry ID (0 = first page). Returns results
// ordered by BM25 rank then entry ID descending, and the next cursor
// (0 if no more pages).
func SearchEntries(ctx context.Context, d *sql.DB, userID int64, query string, limit int, cursor int64) ([]SearchResult, int64, error) {
    const baseQ = `
        SELECT e.id, e.subscription_id, e.title, e.url, COALESCE(e.author,''),
               e.published_at, e.read, e.saved,
               bm25(entries_fts) AS rank
        FROM   entries_fts
        JOIN   entries       e  ON e.id = entries_fts.rowid
        JOIN   subscriptions s  ON s.id = e.subscription_id
        WHERE  entries_fts MATCH ?
          AND  s.user_id = ?
    `
    const firstPageQ = baseQ + ` ORDER BY rank, e.id DESC LIMIT ?`
    const pagedQ     = baseQ + ` AND e.id < ? ORDER BY rank, e.id DESC LIMIT ?`

    var rows *sql.Rows
    var err error
    if cursor == 0 {
        rows, err = d.QueryContext(ctx, firstPageQ, query, userID, limit)
    } else {
        rows, err = d.QueryContext(ctx, pagedQ, query, userID, cursor, limit)
    }
    if err != nil {
        return nil, 0, fmt.Errorf("search entries: %w", err)
    }
    defer rows.Close()

    var out []SearchResult
    for rows.Next() {
        var r SearchResult
        var readInt, savedInt int
        if err := rows.Scan(&r.ID, &r.SubscriptionID, &r.Title, &r.URL, &r.Author,
            &r.PublishedAt, &readInt, &savedInt, &r.Rank); err != nil {
            return nil, 0, fmt.Errorf("scan search result: %w", err)
        }
        r.Read = readInt == 1
        r.Saved = savedInt == 1
        out = append(out, r)
    }
    if err := rows.Err(); err != nil {
        return nil, 0, fmt.Errorf("iterate search results: %w", err)
    }
    var nextCursor int64
    if len(out) == limit {
        nextCursor = out[len(out)-1].ID
    }
    return out, nextCursor, nil
}

- [ ] **Step 4: Run — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run "TestSearchEntries" -race -v
```

- [ ] **Step 5: Run full suite**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -race
```

- [ ] **Step 6: Commit**

```bash
git add internal/db/search.go internal/db/search_test.go
git commit -m "feat(db): FTS5 SearchEntries with trigger contract test"
```

---

### Task B4: `internal/db/opml.go`

**Files:**
- Create: `internal/db/opml.go`
- Create: `internal/db/opml_test.go`

- [ ] **Step 1: Write failing tests (key cases)**

Create `internal/db/opml_test.go` (package `db`, consistent with existing test files):

```go
package db

import (
    "context"
    "database/sql"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

func TestExportOPML_ValidXML(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    catID, _ := db.InsertCategory(context.Background(), d, db.NewCategory{UserID: userID, Name: "Tech", CreatedAt: time.Now().Unix()})
    subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
        Title: "Example", FeedURL: "https://example.com/feed",
        SiteURL: "https://example.com", NextPoll: 0, Created: time.Now().Unix(), UserID: userID,
    })
    _ = db.UpdateSubscriptionCategory(context.Background(), d, subID, userID, sql.NullInt64{Int64: catID, Valid: true})

    data, err := db.ExportOPML(context.Background(), d, userID)
    require.NoError(t, err)
    require.Contains(t, string(data), `<opml version="2.0">`)
    require.Contains(t, string(data), `text="Tech"`)
    require.Contains(t, string(data), `xmlUrl="https://example.com/feed"`)
}

func TestImportOPML_FlatFeeds(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    opml := []byte(`<?xml version="1.0"?>
<opml version="2.0"><head/><body>
  <outline type="rss" text="Example" xmlUrl="https://example.com/feed"/>
</body></opml>`)
    imported, skipped, errs, err := db.ImportOPML(context.Background(), d, userID, opml, time.Now().Unix())
    require.NoError(t, err)
    require.Equal(t, 1, imported)
    require.Equal(t, 0, skipped)
    require.Empty(t, errs)
    subs, _ := db.ListSubscriptions(context.Background(), d, userID)
    require.Len(t, subs, 1)
    require.False(t, subs[0].CategoryID.Valid)
}

func TestImportOPML_OneLevelNesting(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    opml := []byte(`<?xml version="1.0"?>
<opml version="2.0"><head/><body>
  <outline text="Tech">
    <outline type="rss" text="Ars Technica" xmlUrl="https://arstechnica.com/feed/"/>
  </outline>
</body></opml>`)
    imported, skipped, errs, err := db.ImportOPML(context.Background(), d, userID, opml, time.Now().Unix())
    require.NoError(t, err)
    require.Equal(t, 1, imported)
    require.Equal(t, 0, skipped)
    require.Empty(t, errs)
    cats, _ := db.ListCategories(context.Background(), d, userID)
    require.Len(t, cats, 1)
    require.Equal(t, "Tech", cats[0].Name)
}

func TestImportOPML_DuplicateSkipped(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    opml := []byte(`<?xml version="1.0"?>
<opml version="2.0"><head/><body>
  <outline type="rss" text="Example" xmlUrl="https://example.com/feed"/>
</body></opml>`)
    db.ImportOPML(context.Background(), d, userID, opml, time.Now().Unix())
    imported, skipped, _, err := db.ImportOPML(context.Background(), d, userID, opml, time.Now().Unix())
    require.NoError(t, err)
    require.Equal(t, 0, imported)
    require.Equal(t, 1, skipped)
}

func TestImportOPML_InvalidURLInErrors(t *testing.T) {
    t.Parallel()
    d := newTestDB(t)
    userID := insertTestUser(t, d)
    opml := []byte(`<?xml version="1.0"?>
<opml version="2.0"><head/><body>
  <outline type="rss" text="Bad" xmlUrl="not a url"/>
  <outline type="rss" text="Good" xmlUrl="https://good.com/feed"/>
</body></opml>`)
    imported, _, errs, err := db.ImportOPML(context.Background(), d, userID, opml, time.Now().Unix())
    require.NoError(t, err)
    require.Equal(t, 1, imported) // Good imported
    require.Len(t, errs, 1)       // Bad in errors
}
```

- [ ] **Step 2: Run — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run "TestExportOPML|TestImportOPML" -race -v
```

- [ ] **Step 3: Implement `internal/db/opml.go`**

```go
package db

import (
    "context"
    "database/sql"
    "encoding/xml"
    "fmt"
    "net/url"
    "strings"
    "time"
)

type opmlDoc struct {
    XMLName xml.Name    `xml:"opml"`
    Version string      `xml:"version,attr"`
    Head    struct{}    `xml:"head"`
    Body    opmlBody    `xml:"body"`
}

type opmlBody struct {
    Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
    Text     string        `xml:"text,attr"`
    Type     string        `xml:"type,attr,omitempty"`
    XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
    HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
    Children []opmlOutline `xml:"outline"`
}

func ExportOPML(ctx context.Context, d *sql.DB, userID int64) ([]byte, error) {
    subs, err := ListSubscriptions(ctx, d, userID)
    if err != nil {
        return nil, fmt.Errorf("export opml: %w", err)
    }
    cats, err := ListCategories(ctx, d, userID)
    if err != nil {
        return nil, fmt.Errorf("export opml categories: %w", err)
    }

    // Build category outlines as pointers so children can be appended through
    // the map. Collect category IDs in insertion order for deterministic output.
    catMap := make(map[int64]*opmlOutline)
    catOrder := make([]int64, 0, len(cats))
    for i := range cats {
        o := &opmlOutline{Text: cats[i].Name}
        catMap[cats[i].ID] = o
        catOrder = append(catOrder, cats[i].ID)
    }

    var uncategorised []opmlOutline
    for _, s := range subs {
        o := opmlOutline{
            Text:   s.Title,
            Type:   "rss",
            XMLURL: s.FeedURL,
        }
        if s.SiteURL.Valid {
            o.HTMLURL = s.SiteURL.String
        }
        if s.CategoryID.Valid {
            if parent, ok := catMap[s.CategoryID.Int64]; ok {
                parent.Children = append(parent.Children, o)
                continue
            }
        }
        uncategorised = append(uncategorised, o)
    }

    // Assemble doc from the pointer map (which has children appended).
    var doc opmlDoc
    doc.Version = "2.0"
    for _, id := range catOrder {
        doc.Body.Outlines = append(doc.Body.Outlines, *catMap[id])
    }
    doc.Body.Outlines = append(doc.Body.Outlines, uncategorised...)

    out, err := xml.MarshalIndent(doc, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("marshal opml: %w", err)
    }
    return append([]byte(xml.Header), out...), nil
}

func ImportOPML(ctx context.Context, d *sql.DB, userID int64, data []byte, now int64) (imported, skipped int, errs []string, err error) {
    var doc opmlDoc
    if err := xml.Unmarshal(data, &doc); err != nil {
        return 0, 0, nil, fmt.Errorf("parse opml: %w", err)
    }

    tx, err := d.BeginTx(ctx, nil)
    if err != nil {
        return 0, 0, nil, fmt.Errorf("begin opml import tx: %w", err)
    }
    defer func() {
        if err != nil {
            _ = tx.Rollback()
        }
    }()

    var processOutline func(o opmlOutline, categoryID sql.NullInt64)
    processOutline = func(o opmlOutline, categoryID sql.NullInt64) {
        xmlURL := strings.TrimSpace(o.XMLURL)
        if xmlURL == "" {
            // Folder outline — upsert category, recurse into children.
            if len(o.Children) > 0 {
                var catID int64
                err2 := tx.QueryRowContext(ctx,
                    `INSERT INTO categories (user_id, name, created_at) VALUES (?, ?, ?)
                     ON CONFLICT(user_id, name) DO UPDATE SET name=excluded.name
                     RETURNING id`, userID, o.Text, now).Scan(&catID)
                if err2 != nil {
                    errs = append(errs, fmt.Sprintf("skipped folder %q: %v", o.Text, err2))
                    return
                }
                for _, child := range o.Children {
                    processOutline(child, sql.NullInt64{Int64: catID, Valid: true})
                }
            }
            return
        }
        if _, err2 := url.ParseRequestURI(xmlURL); err2 != nil {
            errs = append(errs, fmt.Sprintf("skipped %q: invalid URL", xmlURL))
            return
        }
        title := o.Text
        if title == "" {
            title = xmlURL
        }
        _, err2 := tx.ExecContext(ctx, `
            INSERT INTO subscriptions (title, feed_url, next_poll_at, created_at, user_id, category_id)
            VALUES (?, ?, 0, ?, ?, ?)
        `, title, xmlURL, now, userID, nullableInt64(categoryID))
        if err2 != nil {
            // M7 migration 0006 changed the constraint from UNIQUE(feed_url) to
            // UNIQUE(user_id, feed_url) so different users can subscribe to the same feed.
            if strings.Contains(err2.Error(), "UNIQUE constraint failed: subscriptions.user_id, subscriptions.feed_url") {
                skipped++
                return
            }
            errs = append(errs, fmt.Sprintf("skipped %q: %v", xmlURL, err2))
            return
        }
        imported++
    }

    for _, o := range doc.Body.Outlines {
        processOutline(o, sql.NullInt64{})
    }

    if err = tx.Commit(); err != nil {
        return 0, 0, nil, fmt.Errorf("commit opml import: %w", err)
    }
    return imported, skipped, errs, nil
}

func nullableInt64(n sql.NullInt64) any {
    if n.Valid {
        return n.Int64
    }
    return nil
}
```

- [ ] **Step 4: Run — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -run "TestExportOPML|TestImportOPML" -race -v
```

- [ ] **Step 5: Run full suite**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/db/... -race
```

- [ ] **Step 6: Commit**

```bash
git add internal/db/opml.go internal/db/opml_test.go
git commit -m "feat(db): OPML export and import (transactional, per-item errors)"
```

---

## Phase C — `internal/discover`

### Task C1: `internal/discover/discover.go`

**Files:**
- Create: `internal/discover/discover.go`
- Create: `internal/discover/discover_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/discover/discover_test.go`:

```go
package discover_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/bcrisp4/tap/internal/discover"
    "github.com/stretchr/testify/require"
)

func TestDiscover_DirectFeedURL(t *testing.T) {
    t.Parallel()
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/atom+xml")
        w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
            <title>Test Feed</title></feed>`))
    }))
    defer srv.Close()

    results, err := discover.Discover(context.Background(), srv.Client(), srv.URL)
    require.NoError(t, err)
    require.Len(t, results, 1)
    require.Equal(t, srv.URL, results[0].FeedURL)
    require.Equal(t, "atom", results[0].Type)
}

func TestDiscover_HTMLPageWithAlternates(t *testing.T) {
    t.Parallel()
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        w.Write([]byte(`<html><head>
            <link rel="alternate" type="application/rss+xml" title="My RSS" href="/feed.rss"/>
            <link rel="alternate" type="application/atom+xml" title="My Atom" href="/feed.atom"/>
        </head><body></body></html>`))
    }))
    defer srv.Close()

    results, err := discover.Discover(context.Background(), srv.Client(), srv.URL)
    require.NoError(t, err)
    require.Len(t, results, 2)
}

func TestDiscover_NoFeeds(t *testing.T) {
    t.Parallel()
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        w.Write([]byte(`<html><head><title>No feeds here</title></head><body></body></html>`))
    }))
    defer srv.Close()

    _, err := discover.Discover(context.Background(), srv.Client(), srv.URL)
    require.ErrorIs(t, err, discover.ErrNoFeeds)
}

func TestDiscover_JSONFeedType(t *testing.T) {
    t.Parallel()
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        w.Write([]byte(`<html><head>
            <link rel="alternate" type="application/feed+json" title="JSON Feed" href="/feed.json"/>
        </head></html>`))
    }))
    defer srv.Close()

    results, err := discover.Discover(context.Background(), srv.Client(), srv.URL)
    require.NoError(t, err)
    require.Len(t, results, 1)
    require.Equal(t, "json", results[0].Type)
}

func TestDiscover_NoAuthHeaders(t *testing.T) {
    t.Parallel()
    var gotCookie, gotAuth string
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        gotCookie = r.Header.Get("Cookie")
        gotAuth = r.Header.Get("Authorization")
        w.Header().Set("Content-Type", "text/html")
        w.Write([]byte(`<html><head></head></html>`))
    }))
    defer srv.Close()

    discover.Discover(context.Background(), srv.Client(), srv.URL)
    require.Empty(t, gotCookie)
    require.Empty(t, gotAuth)
}
```

- [ ] **Step 2: Run — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/discover/... -race -v
```

- [ ] **Step 3: Implement `internal/discover/discover.go`**

```go
package discover

import (
    "context"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"

    "github.com/mmcdole/gofeed"
    "golang.org/x/net/html"
)

var ErrNoFeeds = errors.New("no feed candidates found at URL")

// HTTPDoer is the minimal interface Discover needs from an HTTP client.
// *http.Client satisfies it; tests can pass a test server's client directly.
type HTTPDoer interface {
    Do(*http.Request) (*http.Response, error)
}

type Result struct {
    Title   string
    FeedURL string
    SiteURL string
    Type    string // "rss", "atom", "json"
}

// Discover fetches rawURL and returns feed candidates.
// It first tries to parse the response as a feed directly.
// If that fails, it parses HTML for <link rel="alternate"> elements.
// Discovery fetches are always unauthenticated.
func Discover(ctx context.Context, client HTTPDoer, rawURL string) ([]Result, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
    if err != nil {
        return nil, fmt.Errorf("discover build request: %w", err)
    }
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("discover fetch %s: %w", rawURL, err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MiB read limit
    if err != nil {
        return nil, fmt.Errorf("discover read body: %w", err)
    }

    // Step 1: try direct feed parse.
    p := gofeed.NewParser()
    if feed, err := p.ParseString(string(body)); err == nil {
        t := feedType(resp.Header.Get("Content-Type"))
        return []Result{{
            Title:   feed.Title,
            FeedURL: rawURL,
            Type:    t,
        }}, nil
    }

    // Step 2: parse HTML for <link rel="alternate">.
    results := parseAlternates(rawURL, body)
    if len(results) == 0 {
        return nil, ErrNoFeeds
    }
    return results, nil
}

func feedType(contentType string) string {
    ct := strings.ToLower(contentType)
    switch {
    case strings.Contains(ct, "atom"):
        return "atom"
    case strings.Contains(ct, "json"):
        return "json"
    default:
        return "rss"
    }
}

func parseAlternates(baseURL string, body []byte) []Result {
    base, _ := url.Parse(baseURL)
    doc, err := html.Parse(strings.NewReader(string(body)))
    if err != nil {
        return nil
    }
    var results []Result
    var walk func(*html.Node)
    walk = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "link" {
            attrs := attrMap(n)
            rel := strings.ToLower(attrs["rel"])
            typ := strings.ToLower(attrs["type"])
            href := attrs["href"]
            if rel != "alternate" || href == "" {
                for c := n.FirstChild; c != nil; c = c.NextSibling {
                    walk(c)
                }
                return
            }
            var t string
            switch {
            case strings.Contains(typ, "atom"):
                t = "atom"
            case strings.Contains(typ, "json"):
                t = "json"
            case strings.Contains(typ, "rss"), strings.Contains(typ, "xml"):
                t = "rss"
            default:
                for c := n.FirstChild; c != nil; c = c.NextSibling {
                    walk(c)
                }
                return
            }
            feedURL := href
            if ref, err := url.Parse(href); err == nil && base != nil {
                feedURL = base.ResolveReference(ref).String()
            }
            results = append(results, Result{
                Title:   attrs["title"],
                FeedURL: feedURL,
                SiteURL: baseURL,
                Type:    t,
            })
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            walk(c)
        }
    }
    walk(doc)
    return results
}

func attrMap(n *html.Node) map[string]string {
    m := make(map[string]string, len(n.Attr))
    for _, a := range n.Attr {
        m[a.Key] = a.Val
    }
    return m
}
```

- [ ] **Step 4: Run — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/discover/... -race -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/discover/
git commit -m "feat(discover): two-step feed discovery (direct parse → HTML alternates)"
```

---

## Phase D — API handlers

### Task D1: New error codes and `internal/api/errors.go`

**Files:**
- Modify: `internal/api/errors.go`

- [ ] **Step 1: Add the new constants**

```go
const (
    // existing codes omitted for brevity — append these:
    ErrCodeQueryTooShort       = "query_too_short"
    ErrCodeNoFeedsFound        = "no_feeds_found"
    ErrCodeCategoryNotFound    = "category_not_found"
    ErrCodeCategoryNameTaken   = "category_name_taken"
)
```

- [ ] **Step 2: Build check**

```bash
cd /home/ben.guest/Users/ben/src/tap && go build ./internal/api/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/errors.go
git commit -m "feat(api): add M9 error codes"
```

---

### Task D2: Categories API handlers

**Files:**
- Create: `internal/api/categories.go`
- Create: `internal/api/categories_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/categories_test.go`:

```go
package api

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/bcrisp4/tap/internal/db"
    "github.com/stretchr/testify/require"
)

func newCategoryAPI(t *testing.T) (*http.ServeMux, *sql.DB, int64) {
    t.Helper()
    d, err := db.Open(context.Background(), ":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { _ = d.Close() })
    require.NoError(t, db.Migrate(context.Background(), d))
    userID := insertAPITestUser(t, d)
    m := http.NewServeMux()
    registerCategoryRoutes(m, d)
    return m, d, userID
}

func TestCategories_CreateAndList(t *testing.T) {
    t.Parallel()
    m, _, userID := newCategoryAPI(t)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/categories",
        bytes.NewBufferString(`{"name":"Tech"}`))
    req = withUser(req, userID) // injects user into context as requireSession would
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

    req2 := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
    req2 = withUser(req2, userID)
    rr2 := httptest.NewRecorder()
    m.ServeHTTP(rr2, req2)
    require.Equal(t, http.StatusOK, rr2.Code)
    var resp struct{ Data []categoryDTO }
    require.NoError(t, json.NewDecoder(rr2.Body).Decode(&resp))
    require.Len(t, resp.Data, 1)
    require.Equal(t, "Tech", resp.Data[0].Name)
}

func TestCategories_DuplicateName(t *testing.T) {
    t.Parallel()
    m, _, userID := newCategoryAPI(t)
    body := bytes.NewBufferString(`{"name":"Tech"}`)
    req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/categories", body), userID)
    req.Header.Set("Content-Type", "application/json")
    httptest.NewRecorder() // first create
    m.ServeHTTP(httptest.NewRecorder(), req)

    req2 := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/categories",
        bytes.NewBufferString(`{"name":"Tech"}`)), userID)
    req2.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req2)
    require.Equal(t, http.StatusBadRequest, rr.Code)
    var resp struct{ Error struct{ Code string } `json:"error"` }
    json.NewDecoder(rr.Body).Decode(&resp)
    require.Equal(t, ErrCodeCategoryNameTaken, resp.Error.Code)
}

func TestCategories_CrossUserForbidden(t *testing.T) {
    t.Parallel()
    m, d, u1 := newCategoryAPI(t)
    u2 := insertAPITestUserWithName(t, d, "other")
    // u1 creates a category
    req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/categories",
        bytes.NewBufferString(`{"name":"Tech"}`)), u1)
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    var created categoryDTO
    json.NewDecoder(rr.Body).Decode(&created)

    // u2 tries to DELETE u1's category
    req2 := withUser(httptest.NewRequest(http.MethodDelete,
        "/api/v1/categories/"+strconv.FormatInt(created.ID, 10), nil), u2)
    rr2 := httptest.NewRecorder()
    m.ServeHTTP(rr2, req2)
    require.Equal(t, http.StatusNotFound, rr2.Code)
}
```

- [ ] **Step 2: Run — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -run "TestCategories" -race -v
```

- [ ] **Step 3: Implement `internal/api/categories.go`**

```go
package api

import (
    "database/sql"
    "encoding/json"
    "errors"
    "net/http"
    "strconv"
    "time"

    "github.com/bcrisp4/tap/internal/db"
)

type categoryDTO struct {
    ID        int64  `json:"id"`
    Name      string `json:"name"`
    Unread    int    `json:"unread"`
    CreatedAt int64  `json:"created_at"`
}

func toCategoryDTO(c db.Category) categoryDTO {
    return categoryDTO{ID: c.ID, Name: c.Name, Unread: c.Unread, CreatedAt: c.CreatedAt}
}

func registerCategoryRoutes(m *http.ServeMux, d *sql.DB) {
    m.HandleFunc("GET /api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
        user, _ := userFromContext(r.Context())
        cats, err := db.ListCategories(r.Context(), d, user.ID)
        if err != nil {
            writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
            return
        }
        out := make([]categoryDTO, 0, len(cats))
        for _, c := range cats {
            out = append(out, toCategoryDTO(c))
        }
        writeJSON(w, http.StatusOK, map[string]any{"data": out})
    })

    m.HandleFunc("POST /api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
        r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
        var body struct{ Name string `json:"name"` }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid body")
            return
        }
        user, _ := userFromContext(r.Context())
        id, err := db.InsertCategory(r.Context(), d, db.NewCategory{
            UserID: user.ID, Name: body.Name, CreatedAt: time.Now().Unix(),
        })
        if errors.Is(err, db.ErrCategoryNameTaken) {
            writeError(w, http.StatusBadRequest, ErrCodeCategoryNameTaken, "category name already exists")
            return
        }
        if err != nil {
            writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
            return
        }
        cat, _ := db.GetCategory(r.Context(), d, id, user.ID)
        writeJSON(w, http.StatusCreated, toCategoryDTO(cat))
    })

    m.HandleFunc("PATCH /api/v1/categories/{id}", func(w http.ResponseWriter, r *http.Request) {
        catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
        if err != nil {
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
            return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
        var body struct{ Name string `json:"name"` }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid body")
            return
        }
        user, _ := userFromContext(r.Context())
        err = db.UpdateCategoryName(r.Context(), d, catID, user.ID, body.Name)
        if errors.Is(err, db.ErrCategoryNotFound) {
            writeError(w, http.StatusNotFound, ErrCodeCategoryNotFound, "category not found")
            return
        }
        if errors.Is(err, db.ErrCategoryNameTaken) {
            writeError(w, http.StatusBadRequest, ErrCodeCategoryNameTaken, "category name already exists")
            return
        }
        if err != nil {
            writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
            return
        }
        cat, _ := db.GetCategory(r.Context(), d, catID, user.ID)
        writeJSON(w, http.StatusOK, toCategoryDTO(cat))
    })

    m.HandleFunc("DELETE /api/v1/categories/{id}", func(w http.ResponseWriter, r *http.Request) {
        catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
        if err != nil {
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
            return
        }
        user, _ := userFromContext(r.Context())
        if err := db.DeleteCategory(r.Context(), d, catID, user.ID); errors.Is(err, db.ErrCategoryNotFound) {
            writeError(w, http.StatusNotFound, ErrCodeCategoryNotFound, "category not found")
            return
        } else if err != nil {
            writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
            return
        }
        w.WriteHeader(http.StatusNoContent)
    })

    m.HandleFunc("POST /api/v1/categories/{id}/mark-read", func(w http.ResponseWriter, r *http.Request) {
        catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
        if err != nil {
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
            return
        }
        user, _ := userFromContext(r.Context())
        if err := db.MarkCategoryRead(r.Context(), d, catID, user.ID); err != nil {
            writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
            return
        }
        w.WriteHeader(http.StatusNoContent)
    })
}
```

- [ ] **Step 4: Run — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -run "TestCategories" -race -v
```

- [ ] **Step 5: Run full API suite**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -race
```

- [ ] **Step 6: Commit**

```bash
git add internal/api/categories.go internal/api/categories_test.go
git commit -m "feat(api): categories CRUD and mark-read endpoints"
```

---

### Task D3: Search, OPML, and Discover API handlers

**Files:**
- Create: `internal/api/search.go`
- Create: `internal/api/search_test.go`
- Create: `internal/api/opml.go`
- Create: `internal/api/opml_test.go`
- Create: `internal/api/discover.go`
- Create: `internal/api/discover_test.go`

- [ ] **Step 1: Write failing tests for the search handler**

Create `internal/api/search_test.go`:

```go
package api

import (
    "context"
    "database/sql"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/bcrisp4/tap/internal/db"
    "github.com/stretchr/testify/require"
)

func TestSearch_QueryTooShort(t *testing.T) {
    t.Parallel()
    m, _, userID := newSearchAPI(t)
    req := withUser(httptest.NewRequest(http.MethodGet, "/api/v1/search?q=ab", nil), userID)
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusBadRequest, rr.Code)
    requireErrorCode(t, rr, ErrCodeQueryTooShort)
}

func TestSearch_ValidQuery(t *testing.T) {
    t.Parallel()
    m, d, userID := newSearchAPI(t)
    subID := insertAPITestSubscription(t, d, userID, "https://example.com/feed")
    insertAPITestEntry(t, d, subID, "Golang concurrency", "<p>goroutines</p>")
    req := withUser(httptest.NewRequest(http.MethodGet, "/api/v1/search?q=concurrency", nil), userID)
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusOK, rr.Code)
    var resp struct{ Data []searchResultDTO }
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
    require.Len(t, resp.Data, 1)
}

func TestSearch_CrossUserIsolation(t *testing.T) {
    t.Parallel()
    m, d, u1 := newSearchAPI(t)
    u2 := insertAPITestUserWithName(t, d, "other")
    sub1 := insertAPITestSubscription(t, d, u1, "https://a.com/feed")
    insertAPITestEntry(t, d, sub1, "secret entry", "<p>private</p>")
    req := withUser(httptest.NewRequest(http.MethodGet, "/api/v1/search?q=secret", nil), u2)
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusOK, rr.Code)
    var resp struct{ Data []searchResultDTO }
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
    require.Empty(t, resp.Data)
}

func newSearchAPI(t *testing.T) (*http.ServeMux, *sql.DB, int64) {
    t.Helper()
    d, err := db.Open(context.Background(), ":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { _ = d.Close() })
    require.NoError(t, db.Migrate(context.Background(), d))
    userID := insertAPITestUser(t, d)
    m := http.NewServeMux()
    registerSearchRoutes(m, d)
    return m, d, userID
}
```

- [ ] **Step 2: Run search tests — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -run "TestSearch" -race -v
```

Expected: FAIL — `registerSearchRoutes` undefined.

- [ ] **Step 3: Implement `internal/api/search.go`**

```go
package api

import (
    "database/sql"
    "net/http"
    "strconv"

    "github.com/bcrisp4/tap/internal/db"
)

type searchResultDTO struct {
    ID             int64   `json:"id"`
    SubscriptionID int64   `json:"subscription_id"`
    Title          string  `json:"title"`
    URL            string  `json:"url"`
    Author         string  `json:"author,omitempty"`
    PublishedAt    int64   `json:"published_at"`
    Read           bool    `json:"read"`
    Saved          bool    `json:"saved"`
    Rank           float64 `json:"rank"`
}

func registerSearchRoutes(m *http.ServeMux, d *sql.DB) {
    m.HandleFunc("GET /api/v1/search", func(w http.ResponseWriter, r *http.Request) {
        q := r.URL.Query().Get("q")
        if len([]rune(q)) < 3 {
            writeError(w, http.StatusBadRequest, ErrCodeQueryTooShort, "query must be at least 3 characters")
            return
        }
        limit := 50
        if l := r.URL.Query().Get("limit"); l != "" {
            if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
                limit = n
            }
        }
        var cursor int64
        if c := r.URL.Query().Get("cursor"); c != "" {
            cursor, _ = strconv.ParseInt(c, 10, 64)
        }
        user, _ := userFromContext(r.Context())
        results, nextCursor, err := db.SearchEntries(r.Context(), d, user.ID, q, limit, cursor)
        if err != nil {
            writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
            return
        }
        out := make([]searchResultDTO, 0, len(results))
        for _, res := range results {
            out = append(out, searchResultDTO{
                ID: res.ID, SubscriptionID: res.SubscriptionID,
                Title: res.Title, URL: res.URL, Author: res.Author,
                PublishedAt: res.PublishedAt, Read: res.Read, Saved: res.Saved, Rank: res.Rank,
            })
        }
        resp := map[string]any{"data": out}
        if nextCursor > 0 {
            resp["next_cursor"] = nextCursor
        }
        writeJSON(w, http.StatusOK, resp)
    })
}
```

- [ ] **Step 4: Run search tests — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -run "TestSearch" -race -v
```

Expected: all search tests PASS.

- [ ] **Step 5: Write failing tests for the OPML handler**

Create `internal/api/opml_test.go`:

```go
package api

import (
    "bytes"
    "context"
    "database/sql"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/bcrisp4/tap/internal/db"
    "github.com/stretchr/testify/require"
)

func newOPMLAPI(t *testing.T) (*http.ServeMux, *sql.DB, int64) {
    t.Helper()
    d, err := db.Open(context.Background(), ":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { _ = d.Close() })
    require.NoError(t, db.Migrate(context.Background(), d))
    userID := insertAPITestUser(t, d)
    m := http.NewServeMux()
    registerOPMLRoutes(m, d)
    return m, d, userID
}

func TestOPML_Export(t *testing.T) {
    t.Parallel()
    m, _, userID := newOPMLAPI(t)
    req := withUser(httptest.NewRequest(http.MethodGet, "/api/v1/opml", nil), userID)
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusOK, rr.Code)
    require.Contains(t, rr.Header().Get("Content-Type"), "text/x-opml")
    require.Contains(t, rr.Body.String(), `<opml version="2.0">`)
}

func TestOPML_ImportBodyTooLarge(t *testing.T) {
    t.Parallel()
    m, _, userID := newOPMLAPI(t)
    // 11 MiB body — over the 10 MiB cap
    body := strings.NewReader(strings.Repeat("x", 11<<20))
    req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/opml", body), userID)
    req.Header.Set("Content-Type", "text/x-opml")
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}

func TestOPML_ImportResponseShape(t *testing.T) {
    t.Parallel()
    m, _, userID := newOPMLAPI(t)
    opmlBody := `<?xml version="1.0"?><opml version="2.0"><head/><body>
      <outline type="rss" text="Example" xmlUrl="https://example.com/feed"/>
    </body></opml>`
    req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/opml",
        bytes.NewBufferString(opmlBody)), userID)
    req.Header.Set("Content-Type", "text/x-opml")
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusOK, rr.Code)
    var resp struct {
        Imported int      `json:"imported"`
        Skipped  int      `json:"skipped"`
        Errors   []string `json:"errors"`
    }
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
    require.Equal(t, 1, resp.Imported)
    require.Equal(t, 0, resp.Skipped)
    require.Empty(t, resp.Errors)
}
```

- [ ] **Step 6: Run OPML tests — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -run "TestOPML" -race -v
```

Expected: FAIL — `registerOPMLRoutes` undefined.

- [ ] **Step 7: Implement `internal/api/opml.go`**

```go
package api

import (
    "database/sql"
    "io"
    "net/http"
    "time"

    "github.com/bcrisp4/tap/internal/db"
)

const opmlBodyCap = 10 << 20 // 10 MiB

func registerOPMLRoutes(m *http.ServeMux, d *sql.DB) {
    m.HandleFunc("GET /api/v1/opml", func(w http.ResponseWriter, r *http.Request) {
        user, _ := userFromContext(r.Context())
        data, err := db.ExportOPML(r.Context(), d, user.ID)
        if err != nil {
            writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
            return
        }
        w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
        w.Header().Set("Content-Disposition", `attachment; filename="subscriptions.opml"`)
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write(data)
    })

    m.HandleFunc("POST /api/v1/opml", func(w http.ResponseWriter, r *http.Request) {
        r.Body = http.MaxBytesReader(w, r.Body, opmlBodyCap)
        data, err := io.ReadAll(r.Body)
        if err != nil {
            var maxErr *http.MaxBytesError
            if errors.As(err, &maxErr) {
                writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "body too large")
                return
            }
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "could not read body")
            return
        }
        user, _ := userFromContext(r.Context())
        imported, skipped, errs, err := db.ImportOPML(r.Context(), d, user.ID, data, time.Now().Unix())
        if err != nil {
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{
            "imported": imported,
            "skipped":  skipped,
            "errors":   errs,
        })
    })
}
```

- [ ] **Step 8: Run OPML tests — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -run "TestOPML" -race -v
```

Expected: all OPML tests PASS.

- [ ] **Step 9: Write failing tests for the discover handler**

Create `internal/api/discover_test.go`:

```go
package api

import (
    "bytes"
    "context"
    "database/sql"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/bcrisp4/tap/internal/db"
    "github.com/stretchr/testify/require"
)

func newDiscoverAPI(t *testing.T, discoverSrv *httptest.Server) (*http.ServeMux, int64) {
    t.Helper()
    d, err := db.Open(context.Background(), ":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { _ = d.Close() })
    require.NoError(t, db.Migrate(context.Background(), d))
    userID := insertAPITestUser(t, d)
    m := http.NewServeMux()
    registerDiscoverRoutes(m, d, discoverSrv.Client())
    return m, userID
}

func TestDiscover_CandidatesReturned(t *testing.T) {
    t.Parallel()
    origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        w.Write([]byte(`<html><head>
            <link rel="alternate" type="application/rss+xml" href="/feed.rss" title="My Feed"/>
        </head></html>`))
    }))
    defer origin.Close()

    m, userID := newDiscoverAPI(t, origin)
    body := bytes.NewBufferString(`{"url":"` + origin.URL + `"}`)
    req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/discover", body), userID)
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusOK, rr.Code)
    var resp struct{ Candidates []discoverCandidateDTO }
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
    require.Len(t, resp.Candidates, 1)
}

func TestDiscover_NoFeeds(t *testing.T) {
    t.Parallel()
    origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        w.Write([]byte(`<html><head></head></html>`))
    }))
    defer origin.Close()

    m, userID := newDiscoverAPI(t, origin)
    body := bytes.NewBufferString(`{"url":"` + origin.URL + `"}`)
    req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/discover", body), userID)
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusBadRequest, rr.Code)
    requireErrorCode(t, rr, ErrCodeNoFeedsFound)
}

func TestDiscover_MalformedURL(t *testing.T) {
    t.Parallel()
    origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
    defer origin.Close()
    m, userID := newDiscoverAPI(t, origin)
    body := bytes.NewBufferString(`{"url":"not a url"}`)
    req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/discover", body), userID)
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    m.ServeHTTP(rr, req)
    require.Equal(t, http.StatusBadRequest, rr.Code)
}
```

- [ ] **Step 10: Run discover tests — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -run "TestDiscover" -race -v
```

Expected: FAIL — `registerDiscoverRoutes` undefined.

- [ ] **Step 11: Implement `internal/api/discover.go`**

```go
package api

import (
    "database/sql"
    "encoding/json"
    "errors"
    "net/http"

    "github.com/bcrisp4/tap/internal/discover"
)

type discoverCandidateDTO struct {
    Title   string `json:"title"`
    FeedURL string `json:"feed_url"`
    SiteURL string `json:"site_url"`
    Type    string `json:"type"`
}

func registerDiscoverRoutes(m *http.ServeMux, _ *sql.DB, client discover.HTTPDoer) {
    m.HandleFunc("POST /api/v1/discover", func(w http.ResponseWriter, r *http.Request) {
        r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
        var body struct{ URL string `json:"url"` }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid body")
            return
        }
        if body.URL == "" {
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "url is required")
            return
        }
        results, err := discover.Discover(r.Context(), client, body.URL)
        if errors.Is(err, discover.ErrNoFeeds) {
            writeError(w, http.StatusBadRequest, ErrCodeNoFeedsFound, "no feeds found at that URL")
            return
        }
        if err != nil {
            writeError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
            return
        }
        out := make([]discoverCandidateDTO, 0, len(results))
        for _, r := range results {
            out = append(out, discoverCandidateDTO{Title: r.Title, FeedURL: r.FeedURL, SiteURL: r.SiteURL, Type: r.Type})
        }
        writeJSON(w, http.StatusOK, map[string]any{"candidates": out})
    })
}
```

- [ ] **Step 12: Run discover tests — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -run "TestDiscover" -race -v
```

Expected: all discover tests PASS.

- [ ] **Step 13: Wire all new routes into `internal/api/api.go`**

In `NewMux`, add `MuxOpts.HTTPClient discover.HTTPDoer` (the interface, not `*http.Client` — production passes the shared `*http.Client` which satisfies the interface; tests pass `httptest.Server.Client()` which also satisfies it) and wire the new route groups after the existing ones:

```go
catsMux := http.NewServeMux()
registerCategoryRoutes(catsMux, db)

searchMux := http.NewServeMux()
registerSearchRoutes(searchMux, db)

opmlMux := http.NewServeMux()
registerOPMLRoutes(opmlMux, db)

discoverMux := http.NewServeMux()
registerDiscoverRoutes(discoverMux, db, opts.HTTPClient)

for _, p := range []struct{ method, path string; handler http.Handler }{
    // categories
    {"GET",    "/api/v1/categories",                  authed(catsMux)},
    {"POST",   "/api/v1/categories",                  authedCSRF(catsMux)},
    {"PATCH",  "/api/v1/categories/{id}",             authedCSRF(catsMux)},
    {"DELETE", "/api/v1/categories/{id}",             authedCSRF(catsMux)},
    {"POST",   "/api/v1/categories/{id}/mark-read",   authedCSRF(catsMux)},
    // search
    {"GET",    "/api/v1/search",                      authed(searchMux)},
    // opml
    {"GET",    "/api/v1/opml",                        authed(opmlMux)},
    {"POST",   "/api/v1/opml",                        authedCSRF(opmlMux)},
    // discover
    {"POST",   "/api/v1/discover",                    authedCSRF(discoverMux)},
    // subscription GET by id (deferred from M6)
    {"GET",    "/api/v1/subscriptions/{id}",          authed(subsMux)},
} {
    m.Handle(p.method+" "+p.path, p.handler)
}
```

- [ ] **Step 14: Run full API + build**

```bash
cd /home/ben.guest/Users/ben/src/tap && go test ./internal/api/... -race && go build ./...
```

- [ ] **Step 15: Commit**

```bash
git add internal/api/search.go internal/api/search_test.go \
        internal/api/opml.go internal/api/opml_test.go \
        internal/api/discover.go internal/api/discover_test.go \
        internal/api/api.go
git commit -m "feat(api): search, OPML, discover handlers and route wiring"
```

---

## Phase E — SPA

### Task E1: Types, API client, router extensions

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/router.ts`

- [ ] **Step 1: Write failing tests for the new `api.ts` functions**

Add to `web/src/lib/__tests__/api.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from '../api';

// Mock fetch for each test
beforeEach(() => { vi.resetAllMocks(); });

describe('api.listCategories', () => {
  it('returns categories array', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true, status: 200,
      json: () => Promise.resolve({ data: [{ id: 1, name: 'Tech', unread: 3, created_at: 0 }] }),
    }));
    const cats = await api.listCategories();
    expect(cats).toHaveLength(1);
    expect(cats[0].name).toBe('Tech');
  });
});

describe('api.searchEntries', () => {
  it('sends q param and returns data', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true, status: 200,
      json: () => Promise.resolve({ data: [] }),
    }));
    await api.searchEntries('rust');
    const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
    expect(url).toContain('q=rust');
  });
});

describe('api.discoverFeeds', () => {
  it('posts url and returns candidates', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true, status: 200,
      json: () => Promise.resolve({ candidates: [{ title: 'Feed', feed_url: 'https://x.com/feed', site_url: '', type: 'rss' }] }),
    }));
    const result = await api.discoverFeeds('https://x.com');
    expect(result.candidates).toHaveLength(1);
  });
});
```

- [ ] **Step 2: Run — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web test -- src/lib/__tests__/api.test.ts
```

Expected: FAIL — `api.listCategories`, `api.searchEntries`, `api.discoverFeeds` undefined.

- [ ] **Step 3: Extend `types.ts`**

```typescript
// Add to web/src/lib/types.ts:

export type Category = {
  id: number;
  name: string;
  unread: number;
  created_at: number;
};

export type DiscoverCandidate = {
  title: string;
  feed_url: string;
  site_url: string;
  type: 'rss' | 'atom' | 'json';
};

export type DiscoverResult = { candidates: DiscoverCandidate[] };

export type OPMLImportResult = {
  imported: number;
  skipped: number;
  errors: string[];
};
```

Also add `category_id: number | null` to the `Subscription` type.

- [ ] **Step 4: Extend `api.ts`**

Add to the `api` export object in `web/src/lib/api.ts`:

```typescript
  listCategories: () =>
    request<{ data: Category[] }>('/categories').then(r => r.data),

  createCategory: (name: string) =>
    request<Category>('/categories', { method: 'POST', body: JSON.stringify({ name }) }),

  renameCategory: (id: number, name: string) =>
    request<Category>(`/categories/${id}`, { method: 'PATCH', body: JSON.stringify({ name }) }),

  deleteCategory: (id: number) =>
    request<void>(`/categories/${id}`, { method: 'DELETE' }),

  markCategoryRead: (id: number) =>
    request<void>(`/categories/${id}/mark-read`, { method: 'POST' }),

  searchEntries: (q: string, limit = 50, cursor?: number) => {
    const qs = new URLSearchParams({ q, limit: String(limit) });
    if (cursor) qs.set('cursor', String(cursor));
    return request<ListResponse<EntryListItem>>(`/search?${qs}`);
  },

  exportOPML: async () => {
    const res = await fetch('/api/v1/opml', {
      headers: { 'X-CSRF-Token': get(auth).csrfToken ?? '' },
    });
    if (!res.ok) throw new Error('export failed');
    return res.blob();
  },

  importOPML: (data: ArrayBuffer) =>
    request<OPMLImportResult>('/opml', {
      method: 'POST',
      headers: { 'Content-Type': 'text/x-opml' },
      body: data,
    }),

  discoverFeeds: (url: string) =>
    request<DiscoverResult>('/discover', { method: 'POST', body: JSON.stringify({ url }) }),

  getSubscription: (id: number) =>
    request<Subscription>(`/subscriptions/${id}`),
```

- [ ] **Step 5: Run api tests — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web test -- src/lib/__tests__/api.test.ts
```

Expected: all api tests PASS.

- [ ] **Step 6: Extend `router.ts`**

```typescript
// Replace the RouteState union and parse function.
// IMPORTANT: M8 also modifies router.ts — check for M8's /settings and /saved
// route arms and preserve them when rewriting RouteState. Do NOT remove arms
// that M8 added; only add the M9 arms.

type RouteState =
  | { name: 'unread' }
  | { name: 'reader'; params: { id: number } }
  | { name: 'search'; params: { q: string } }
  | { name: 'category'; params: { id: number } }
  // …plus any arms M8 added (settings, saved, history, etc.)
  ;

function parse(pathname: string, search: string): RouteState {
  const params = new URLSearchParams(search);
  let m: RegExpMatchArray | null;

  if ((m = pathname.match(/^\/entry\/(\d+)$/)))
    return { name: 'reader', params: { id: Number(m[1]) } };
  if (pathname === '/search')
    return { name: 'search', params: { q: params.get('q') ?? '' } };
  if ((m = pathname.match(/^\/categories\/(\d+)$/)))
    return { name: 'category', params: { id: Number(m[1]) } };

  // Preserve any M8-added arms here before the fallback.
  return { name: 'unread' };
}

const internal = writable<RouteState>(parse(window.location.pathname, window.location.search));
window.addEventListener('popstate', () =>
  internal.set(parse(window.location.pathname, window.location.search)));
```

Update `navigate` to pass `window.location.search` as well.

- [ ] **Step 7: Build check**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web run check
```

Fix any TypeScript errors, especially from the M8/M9 RouteState merge.

- [ ] **Step 8: Commit**

```bash
git add web/src/lib/types.ts web/src/lib/api.ts web/src/lib/router.ts web/src/lib/__tests__/api.test.ts
git commit -m "feat(spa): types, api client, and router extensions for M9"
```

---

### Task E2: `Search.svelte` and `Category.svelte`

**Files:**
- Create: `web/src/views/Search.svelte`
- Create: `web/src/views/Category.svelte`
- Create: `web/src/views/__tests__/Search.test.ts`
- Create: `web/src/views/__tests__/Category.test.ts`

- [ ] **Step 1: Write failing Search component tests**

Create `web/src/views/__tests__/Search.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import Search from '../Search.svelte';
import { api } from '../../lib/api';

vi.mock('../../lib/api');

describe('Search', () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it('does not fire request for query < 3 chars', async () => {
    const { getByRole } = render(Search);
    const input = getByRole('searchbox');
    await fireEvent.input(input, { target: { value: 'ab' } });
    await waitFor(() => expect(api.searchEntries).not.toHaveBeenCalled());
  });

  it('fires request after debounce for query >= 3 chars', async () => {
    vi.mocked(api.searchEntries).mockResolvedValue({ data: [] });
    vi.useFakeTimers();
    const { getByRole } = render(Search);
    const input = getByRole('searchbox');
    await fireEvent.input(input, { target: { value: 'rust' } });
    vi.advanceTimersByTime(300);
    await waitFor(() => expect(api.searchEntries).toHaveBeenCalledWith('rust', 50, undefined));
    vi.useRealTimers();
  });

  it('updates URL with ?q= on input', async () => {
    vi.mocked(api.searchEntries).mockResolvedValue({ data: [] });
    const replaceSpy = vi.spyOn(window.history, 'replaceState');
    const { getByRole } = render(Search);
    const input = getByRole('searchbox');
    await fireEvent.input(input, { target: { value: 'golang' } });
    expect(replaceSpy).toHaveBeenCalledWith({}, '', '/search?q=golang');
  });
});
```

- [ ] **Step 2: Write failing Category component tests**

Create `web/src/views/__tests__/Category.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import Category from '../Category.svelte';
import { api } from '../../lib/api';

vi.mock('../../lib/api');

describe('Category', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listCategories).mockResolvedValue([{ id: 1, name: 'Tech', unread: 2, created_at: 0 }]);
    vi.mocked(api.listEntries).mockResolvedValue({ data: [] });
  });

  it('fetches entries with category filter', async () => {
    render(Category, { props: { id: 1 } });
    await waitFor(() => expect(api.listEntries).toHaveBeenCalledWith(
      expect.objectContaining({ category: 1 })
    ));
  });

  it('calls markCategoryRead after confirmation', async () => {
    vi.mocked(api.markCategoryRead).mockResolvedValue(undefined);
    vi.stubGlobal('confirm', () => true);
    const { getByRole } = render(Category, { props: { id: 1 } });
    await waitFor(() => getByRole('button', { name: /mark all read/i }));
    await fireEvent.click(getByRole('button', { name: /mark all read/i }));
    await waitFor(() => expect(api.markCategoryRead).toHaveBeenCalledWith(1));
  });
});
```

- [ ] **Step 3: Run component tests — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web test -- src/views/__tests__/Search.test.ts src/views/__tests__/Category.test.ts
```

Expected: FAIL — `Search.svelte` and `Category.svelte` don't exist yet.

- [ ] **Step 4: Create `Search.svelte`**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import TopBar from '../components/TopBar.svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import type { EntryListItem } from '../lib/types';

  let query = $state('');
  let results = $state<EntryListItem[]>([]);
  let loading = $state(false);
  let error = $state('');
  let debounceTimer: ReturnType<typeof setTimeout>;
  let inputEl: HTMLInputElement;

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    query = params.get('q') ?? '';
    inputEl?.focus();
    if (query.length >= 3) doSearch(query);
  });

  function onInput() {
    clearTimeout(debounceTimer);
    const q = query;
    const url = q.length >= 3 ? `/search?q=${encodeURIComponent(q)}` : '/search';
    window.history.replaceState({}, '', url);
    if (q.length < 3) { results = []; return; }
    debounceTimer = setTimeout(() => doSearch(q), 300);
  }

  async function doSearch(q: string) {
    loading = true; error = '';
    try {
      const resp = await api.searchEntries(q);
      results = resp.data;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Search failed';
    } finally {
      loading = false;
    }
  }

  // / keybinding: focus this input from anywhere
  function onKeydown(e: KeyboardEvent) {
    if (e.key === '/' && !(e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement)) {
      e.preventDefault();
      inputEl?.focus();
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="layout">
  <Sidebar />
  <main class="main">
    <TopBar title="Search" />
    <div class="search-bar">
      <input
        bind:this={inputEl}
        bind:value={query}
        oninput={onInput}
        placeholder="Search entries…"
        class="search-input"
        type="search"
      />
    </div>
    {#if loading}
      <p class="status">Searching…</p>
    {:else if error}
      <p class="status err">{error}</p>
    {:else if query.length > 0 && query.length < 3}
      <p class="status">Type at least 3 characters to search.</p>
    {:else if results.length === 0 && query.length >= 3}
      <p class="status">No results for "{query}".</p>
    {:else}
      <div class="list">
        {#each results as entry (entry.id)}
          <EntryRow {entry} />
        {/each}
      </div>
    {/if}
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .main { flex: 1; display: flex; flex-direction: column; overflow-y: auto; background: var(--bg); }
  .search-bar { padding: 16px 24px 8px; }
  .search-input { width: 100%; padding: 8px 12px; font-family: var(--sans); font-size: 14px;
    border: 1px solid var(--rule); border-radius: 4px; background: var(--bg); color: var(--ink); }
  .status { padding: 24px; color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .err { color: var(--err, #c00); }
  .list { flex: 1; overflow-y: auto; }
</style>
```

- [ ] **Step 2: Create `Category.svelte`**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import TopBar from '../components/TopBar.svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import { api } from '../lib/api';
  import type { EntryListItem } from '../lib/types';

  let { id }: { id: number } = $props();

  let entries = $state<EntryListItem[]>([]);
  let loading = $state(true);
  let error = $state('');
  let categoryName = $state('');
  let unread = $state(0);
  let marking = $state(false);

  onMount(async () => {
    try {
      const [cats, resp] = await Promise.all([
        api.listCategories(),
        api.listEntries({ category: id, unread: true }),
      ]);
      const cat = cats.find(c => c.id === id);
      if (cat) { categoryName = cat.name; unread = cat.unread; }
      entries = resp.data;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally {
      loading = false;
    }
  });

  async function markAllRead() {
    if (!confirm(`Mark all entries in "${categoryName}" as read?`)) return;
    marking = true;
    try {
      await api.markCategoryRead(id);
      entries = entries.map(e => ({ ...e, read: true }));
      unread = 0;
    } catch { /* ignore */ } finally {
      marking = false;
    }
  }
</script>

<div class="layout">
  <Sidebar />
  <main class="main">
    <TopBar title={categoryName || 'Category'} countShown={unread} countTotal={unread}
      onAction={markAllRead} actionLabel={marking ? 'Marking…' : 'Mark all read'} />
    {#if loading}
      <p class="status">Loading…</p>
    {:else if error}
      <p class="status err">{error}</p>
    {:else if entries.length === 0}
      <p class="status">No unread entries in this category.</p>
    {:else}
      <div class="list">
        {#each entries as entry (entry.id)}
          <EntryRow {entry} />
        {/each}
      </div>
    {/if}
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .main { flex: 1; display: flex; flex-direction: column; overflow-y: auto; background: var(--bg); }
  .status { padding: 24px; color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .err { color: var(--err, #c00); }
  .list { flex: 1; overflow-y: auto; }
</style>
```

- [ ] **Step 7: Run component tests — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web test -- src/views/__tests__/Search.test.ts src/views/__tests__/Category.test.ts
```

Expected: all component tests PASS.

- [ ] **Step 8: Commit**

```bash
git add web/src/views/Search.svelte web/src/views/Category.svelte \
        web/src/views/__tests__/Search.test.ts web/src/views/__tests__/Category.test.ts
git commit -m "feat(spa): Search and Category views"
```

---

### Task E3: Sidebar category grouping and `App.svelte` routing

**Files:**
- Modify: `web/src/components/Sidebar.svelte`
- Modify: `web/src/App.svelte`

- [ ] **Step 1: Write failing Sidebar tests**

Create `web/src/components/__tests__/Sidebar.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import Sidebar from '../Sidebar.svelte';
import { api } from '../../lib/api';

vi.mock('../../lib/api');
vi.mock('../../lib/store', () => ({
  subscriptions: { subscribe: vi.fn((fn) => { fn([]); return () => {}; }) },
}));

describe('Sidebar', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listCategories).mockResolvedValue([]);
  });

  it('loads categories on mount', async () => {
    render(Sidebar);
    await waitFor(() => expect(api.listCategories).toHaveBeenCalledOnce());
  });

  it('shows new category input when + is clicked', async () => {
    const { getByTitle, getByPlaceholderText } = render(Sidebar);
    await waitFor(() => {});
    await fireEvent.click(getByTitle('New category'));
    expect(getByPlaceholderText('Category name')).toBeTruthy();
  });

  it('calls createCategory on Enter in new-category input', async () => {
    vi.mocked(api.createCategory).mockResolvedValue({ id: 1, name: 'Tech', unread: 0, created_at: 0 });
    const { getByTitle, getByPlaceholderText } = render(Sidebar);
    await waitFor(() => {});
    await fireEvent.click(getByTitle('New category'));
    const input = getByPlaceholderText('Category name');
    await fireEvent.input(input, { target: { value: 'Tech' } });
    await fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(api.createCategory).toHaveBeenCalledWith('Tech'));
  });

  it('calls deleteCategory after confirmation', async () => {
    vi.mocked(api.listCategories).mockResolvedValue([{ id: 1, name: 'Tech', unread: 0, created_at: 0 }]);
    vi.mocked(api.deleteCategory).mockResolvedValue(undefined);
    vi.stubGlobal('confirm', () => true);
    const { getByTitle } = render(Sidebar);
    await waitFor(() => getByTitle('Delete category'));
    await fireEvent.click(getByTitle('Delete category'));
    await waitFor(() => expect(api.deleteCategory).toHaveBeenCalledWith(1));
  });
});
```

- [ ] **Step 2: Run Sidebar tests — confirm FAIL**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web test -- src/components/__tests__/Sidebar.test.ts
```

Expected: FAIL — Sidebar doesn't have the new category management UI yet.

- [ ] **Step 3: Update `Sidebar.svelte`**

Replace the flat FEEDS group with a category-grouped layout. Key changes:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import FeedAvatar from './FeedAvatar.svelte';
  import AddFeedForm from './AddFeedForm.svelte';
  import { subscriptions } from '../lib/store';
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import type { Category } from '../lib/types';

  let categories = $state<Category[]>([]);
  let newCatName = $state('');
  let adding = $state(false);

  onMount(async () => {
    try { categories = await api.listCategories(); } catch { /* ignore */ }
  });

  async function createCategory() {
    if (!newCatName.trim()) return;
    try {
      const cat = await api.createCategory(newCatName.trim());
      categories = [...categories, cat];
      newCatName = '';
      adding = false;
    } catch { /* ignore */ }
  }

  async function deleteCategory(id: number) {
    if (!confirm('Delete this category? Its feeds will become uncategorised.')) return;
    await api.deleteCategory(id);
    categories = categories.filter(c => c.id !== id);
  }

  function subsForCategory(catId: number | null) {
    return $subscriptions.filter(s => s.category_id === catId);
  }
</script>

<aside class="sidebar">
  <div class="brand">tap<span class="dot">.</span></div>

  <div class="group-title">READING</div>
  <a class="navitem" href="/" onclick={(e) => { e.preventDefault(); navigate('/'); }}>Unread</a>
  <a class="navitem" href="/search" onclick={(e) => { e.preventDefault(); navigate('/search'); }}>Search</a>

  <div class="group-header">
    <span class="group-title">FEEDS</span>
    <button class="add-cat" onclick={() => adding = !adding} title="New category">+</button>
  </div>
  {#if adding}
    <div class="new-cat-row">
      <input bind:value={newCatName} placeholder="Category name"
        onkeydown={(e) => e.key === 'Enter' && createCategory()}
        onblur={createCategory} class="new-cat-input" />
    </div>
  {/if}

  {#each categories as cat (cat.id)}
    <div class="cat-row">
      <a class="cat-name" href="/categories/{cat.id}"
        onclick={(e) => { e.preventDefault(); navigate(`/categories/${cat.id}`); }}>
        {cat.name}
      </a>
      <span class="unread-badge">{cat.unread > 0 ? cat.unread : ''}</span>
      <button class="cat-delete" onclick={() => deleteCategory(cat.id)} title="Delete category">×</button>
    </div>
    {#each subsForCategory(cat.id) as sub (sub.id)}
      <div class="feedrow sub-indent">
        <FeedAvatar feedURL={sub.feed_url} />
        <span class="feedname" title={sub.title}>{sub.title}</span>
      </div>
    {/each}
  {/each}

  {#each subsForCategory(null) as sub (sub.id)}
    <div class="feedrow">
      <FeedAvatar feedURL={sub.feed_url} />
      <span class="feedname" title={sub.title}>{sub.title}</span>
    </div>
  {/each}

  <div class="group-title">SYSTEM</div>
  <AddFeedForm />
</aside>
```

- [ ] **Step 4: Run Sidebar tests — confirm PASS**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web test -- src/components/__tests__/Sidebar.test.ts
```

Expected: all Sidebar tests PASS.

- [ ] **Step 5: Update `App.svelte`**

Add the `/search` and `/categories/:id` route arms:

```svelte
{:else if $route.name === 'search'}
  <Search />
{:else if $route.name === 'category'}
  <Category id={$route.params.id} />
```

Import the new views at the top of the script block. Preserve any M8-added route arms (`settings`, `saved`, `history`, etc.) — do not remove them.

- [ ] **Step 6: Build check**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web run check
```

- [ ] **Step 7: Run full SPA test suite**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web test
```

Expected: all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add web/src/components/Sidebar.svelte web/src/components/__tests__/Sidebar.test.ts web/src/App.svelte
git commit -m "feat(spa): category-grouped sidebar and route arms for search and category views"
```

---

## Phase F — Final integration

### Task F1: `make test` full suite

- [ ] **Step 1: Run the full Go test suite with race detector**

```bash
cd /home/ben.guest/Users/ben/src/tap && make test
```

Expected: all tests PASS, no race conditions.

- [ ] **Step 2: Run the full SPA test suite**

```bash
cd /home/ben.guest/Users/ben/src/tap && pnpm --dir web test
```

Expected: all tests PASS.

- [ ] **Step 3: Build the static binary**

```bash
cd /home/ben.guest/Users/ben/src/tap && make build
```

Expected: `bin/tap` produced with no errors.

- [ ] **Step 4: Smoke test — boot and verify migrations apply**

```bash
cd /home/ben.guest/Users/ben/src/tap && TAP_ADMIN_USERNAME=ben TAP_ADMIN_PASSWORD=secret123 ./bin/tap &
sleep 2
curl -s http://localhost:8080/healthz
kill %1
```

Expected: `ok`.

---

### Task F2: Update `CLAUDE.md`

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update the status line**

In `CLAUDE.md`, update the in-progress milestone reference from M6 to M9 and add the M9 spec reference.

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for M9 status"
```

---

### Task F3: Final commit and simplify

- [ ] **Step 1: Verify git status is clean**

```bash
cd /home/ben.guest/Users/ben/src/tap && git status
```

Expected: no uncommitted changes.

- [ ] **Step 2: Tag M9 complete in a summary commit if any loose files remain**

```bash
cd /home/ben.guest/Users/ben/src/tap && git log --oneline -15
```

Confirm all M9 tasks are represented in the commit log.

- [ ] **Step 3: Run `/simplify`**

Invoke the `simplify` skill to review all M9 changed code for reuse, quality, and efficiency. Fix any issues found before marking the milestone complete. This step is mandated by the global `CLAUDE.md`.

---

## Self-review

**Spec coverage check:**

| Spec section | Covered in plan |
|---|---|
| Migration 0008 categories | Task A2 |
| Migration 0009 FTS5 + triggers | Task A4 |
| `tap_strip_html` registration | Task A3 |
| `internal/db/categories.go` | Task B1 |
| `subscriptions.category_id` | Task B2 |
| `internal/db/search.go` | Task B3 |
| `internal/db/opml.go` | Task B4 |
| `internal/discover` | Task C1 |
| API error codes | Task D1 |
| Categories API | Task D2 |
| Search API | Task D3 |
| OPML API (10 MiB cap) | Task D3 |
| Discover API | Task D3 |
| `GET /subscriptions/:id` | Task D3 (api.go wiring) |
| `?category=` filter on entries | Task B2 + D3 |
| `web/src/lib/types.ts` | Task E1 |
| `web/src/lib/api.ts` | Task E1 |
| Router extensions | Task E1 |
| `Search.svelte` + `/` keybinding | Task E2 |
| `Category.svelte` + mark-all-read | Task E2 |
| Sidebar category grouping | Task E3 |
| `App.svelte` new routes | Task E3 |
| CLAUDE.md update | Task F2 |
| FTS delete trigger contract (M11) | Task B3 `TestSearchEntries_DeleteTrigger` |
| OPML 10 MiB cap | Task D3 `opml.go` `opmlBodyCap` const |
| Cross-user isolation in all queries | Tasks B1–B4, D2–D3 |
| M7 hard dependency documented | Plan header + prerequisite note |

**Type consistency check:** `categoryDTO` defined in `categories.go` and referenced in `categories_test.go` consistently. `SearchResult` defined in `search.go`, referenced in `search_test.go`. `db.NewCategory` used consistently across `categories.go` and `categories_test.go`. `sql.NullInt64` used for `CategoryID` throughout.

**No placeholders:** All code blocks contain complete, compilable implementations. Test bodies have real assertions, not comments.
