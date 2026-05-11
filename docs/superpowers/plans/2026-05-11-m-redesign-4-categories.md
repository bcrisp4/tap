# M-Redesign-4 — Categories management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the sidebar's inline category UI with a dedicated `/categories` management page (`views/Categories.svelte`) that ships every interaction the new design specifies — inline rename, reassign-feed popover, mark-all-read with confirm, delete with confirm, Uncategorised pseudo-category, empty state, mobile bottom sheet — plus a small additive backend change (a `position` column + reorder endpoint) so categories can be reordered up and down.

**Architecture:** New SPA route `/categories` rendering `Categories.svelte` inside `.ts-shell` (desktop) or the mobile shell. The page reads the existing `categories` and `subscriptions` stores (the latter is queried for category membership; per-category unread counts come from the API DTO; Uncategorised unread is computed client-side from the `entries` store). M1 primitives are consumed (`Button`, `Popover`, `Dialog`, `Field`, `EmptyState`). Two additive backend changes:

1. Migration `0011_category_position.sql` adding `categories.position INTEGER NOT NULL DEFAULT 0`; new endpoint `POST /api/v1/categories/reorder` accepts an ordered ID list and rewrites positions in a single transaction. List endpoint adds `position ASC, name COLLATE NOCASE` ordering. DTO gains a `position` field.
2. New DB function `db.MarkSubscriptionRead(ctx, d, subID, userID)` + endpoint `POST /api/v1/subscriptions/:id/mark-read` (mirrors `POST /api/v1/categories/:id/mark-read`). Used by the SPA to implement "Mark all read" on the Uncategorised pseudo-card without N entry-level PATCHes.

**Tech Stack:** Svelte 5 runes + TypeScript + Vite (frontend), Go 1.26+ + `modernc.org/sqlite` + `database/sql` (backend; `go.mod` declares `go 1.26.2`), `vitest` + `@testing-library/svelte` (frontend tests), `stretchr/testify` (backend tests).

---

## Dependencies

- **M-Redesign-1 (Foundations).** Must be merged first. This plan consumes:
  - `components/Button.svelte` (`is-primary`, `is-danger`, `is-quiet` variants + icon slot).
  - `components/Popover.svelte` (open/close on click-outside + Esc, anchored to a trigger).
  - `components/Dialog.svelte` (head + body + foot, `is-wide`, focus trap, Esc-to-close, optional `ts-dialog-warn` callout).
  - `components/EmptyState.svelte` (dot + serif title + sans sub + optional CTA).
  - `components/AppShell.svelte` + `components/TopTabs.svelte` + `components/MobileTabBar.svelte` (the `ts-shell` / mobile shell).
  - `views/Categories.svelte` (stubbed by M1 with `EmptyState`-only "Coming soon" content — this plan replaces the body of that stub).
  - `lib/router.ts` already routes `/categories` to the stub view (added by M1).
  - Existing `lib/store.ts` `categories` + `subscriptions` stores stay as-is; this plan extends them.

- **Coordination with M1.** The current sidebar inline-category UI in `web/src/components/Sidebar.svelte` is removed by M1 (per umbrella spec §3.3). After M1 ships, there is no UI surface to manage categories until M-Redesign-4 lands. That is intentional and accepted by the umbrella spec; this plan exists to close the gap.

---

## Reorder decision

**Ship reorder.** The brand spec (`Tap Brand and UI Spec.md` §6.4) lists "Reorder up · Reorder down" alongside Rename and Delete on every category card, and the JSX mockup (`ui_design/tap-categories-page.jsx`) renders cards in a stable order that the user can rearrange. Without reorder the UI cannot honour the design's "Ordering controls sidebar order" copy.

The backend cost is small: one migration, one column, one endpoint, one DB helper, four tests. The risk surface is bounded — the column is `NOT NULL DEFAULT 0`, the endpoint runs in a single transaction, and the list query stays correct if `position` is missing or duplicated (tie-broken by name). The umbrella spec §7 risk 3 already calls this out as the leaning decision.

## Mark-all-read on Uncategorised decision

**Ship `POST /api/v1/subscriptions/:id/mark-read` (Tasks 5a + 5b).** The pseudo-category has no row in `categories`, so `POST /api/v1/categories/null/mark-read` is not a valid path. The alternative (loop unread entries and PATCH each) means N round-trips per uncategorised feed and forces the SPA to pull entire entry lists into memory just to mark them read. A per-subscription endpoint mirrors the existing `MarkCategoryRead` shape: one round-trip per uncategorised feed, no client-side entry enumeration, and the same per-user scoping invariant. Two new DB tests + three new API tests. Lock it in.

---

## File map

### Backend — create

- `internal/db/migrations/0011_category_position.sql`
- `internal/db/categories_test.go` (new tests appended) — see Task 1, 2, 4
- `internal/db/subscriptions_test.go` (new tests appended) — see Task 5a
- `internal/api/categories_test.go` (new tests appended) — see Task 5
- `internal/api/subscriptions_test.go` (new tests appended) — see Task 5b

### Backend — modify

- `internal/db/categories.go`
  - Add `Position int64` field to the existing `Category` struct (alongside the existing `Unread int` — that field stays as-is).
  - Change `ListCategories` `ORDER BY` clause to `ORDER BY c.position ASC, c.name COLLATE NOCASE`.
  - Add `position` to the `Scan` call for `ListCategories` and `GetCategory`.
  - Add new function `ReorderCategories(ctx, d, userID, orderedIDs []int64) error`.
  - Update `InsertCategory` to assign `position = (SELECT COALESCE(MAX(position),-1)+1 FROM categories WHERE user_id=?)`.
- `internal/db/subscriptions.go`
  - Add new function `MarkSubscriptionRead(ctx, d, subscriptionID, userID int64) error` mirroring `MarkCategoryRead` — sets `entries.read = 1` for every unread entry under the subscription, scoped by `user_id`.
- `internal/api/categories.go`
  - Add `Position` field to `categoryDTO`.
  - Register new route: `POST /api/v1/categories/reorder` accepting `{"order": [int64, ...]}`.
- `internal/api/subscriptions.go`
  - Register new route: `POST /api/v1/subscriptions/{id}/mark-read` returning `204` on success, `404` if the subscription doesn't belong to the user.
- `internal/api/api.go`
  - **Outer mux route table extension (load-bearing).** The production mux at `internal/api/api.go:167-187` explicitly enumerates every per-mux route. Two entries must be added or the new endpoints 404 in production:
    - `{"POST", "/api/v1/categories/reorder", authedCSRF(catsMux)}`
    - `{"POST", "/api/v1/subscriptions/{id}/mark-read", authedCSRF(subsMux)}`
- `internal/api/testing.go`
  - **Test mux route table extension.** The test mux at `internal/api/testing.go:65-85` mirrors `api.go`. Add the same two entries with `inject(catsMux)` / `inject(subsMux)`. Without this the new tests will see 404s under the test mux.
- `internal/api/errors.go`
  - Add `ErrCodeReorderMismatch = "category_reorder_mismatch"` (rejected when the request omits or adds IDs vs. the user's current set).

### Frontend — create

- `web/src/views/Categories.svelte` — replaces the M1 stub.
- `web/src/views/__tests__/Categories.test.ts` — view-level behaviour tests.
- `web/src/components/CategoryCard.svelte` — single `.ts-cat` (desktop) or `.m-cat-card` (mobile) — owns rename input, action row, feeds list, reassign trigger.
- `web/src/components/__tests__/CategoryCard.test.ts`
- `web/src/components/CategoryReassignPopover.svelte` — **cross-milestone shared component.** Desktop popover (built on M1's `Popover.svelte`) listing categories + Uncategorised + leading check on current. M4 owns it; M-Redesign-5 (Feeds management) consumes it as-is for both per-row and bulk reassign actions. Public contract documented in Task 7.
- `web/src/components/__tests__/CategoryReassignPopover.test.ts`
- `web/src/components/__tests__/helpers/PopoverPassThrough.svelte` — two-line shim that renders `{#if open}<div>{@render children?.()}</div>{/if}` (or matches whatever child API M1's `Popover.svelte` actually exposes). Used by the `CategoryReassignPopover` test (and any other test that needs to mock out M1's positioning math) so content + callbacks can be exercised without the real Popover primitive. M1's Popover has its own tests; this shim is purely a test boundary.
- `web/src/components/CategoryReassignSheet.svelte` — mobile bottom sheet variant (`.m-cat-sheet`).
- `web/src/components/__tests__/CategoryReassignSheet.test.ts`
- `web/src/components/CategoryDeleteDialog.svelte` — uses `Dialog`. Lists up to 5 affected feeds + "+ N more".
- `web/src/components/__tests__/CategoryDeleteDialog.test.ts`
- `web/src/components/CategoryMarkReadDialog.svelte` — uses `Dialog`. Renders `.ts-dialog-stats` row.
- `web/src/components/__tests__/CategoryMarkReadDialog.test.ts`

### Frontend — modify

- `web/src/lib/types.ts` — `Category` gets `position: number`.
- `web/src/lib/api.ts` — add `reorderCategories(orderedIds: number[]): Promise<void>`. Existing `renameCategory`, `deleteCategory`, `markCategoryRead`, `createCategory`, `listCategories` are reused as-is.
- `web/src/lib/store.ts` — extend `categoriesStore()` with:
  - `reassignSubscription(subId, categoryId | null)` — optimistic update of `subscriptions` store + PATCH `/api/v1/subscriptions/:id` body `{category_id}`.
  - `reorder(orderedIds: number[])` — optimistic reorder + `api.reorderCategories`; rollback on rejection.
  - `markRead(categoryId | null)` — when `null`, marks all uncategorised entries read via a small per-feed loop over uncategorised subscriptions; otherwise calls `api.markCategoryRead(id)`. Calls `entries.load()` and `subscriptions.load()` on success.
  - `rename(id, name)` — wraps `api.renameCategory` + reloads.
  - `create(name)` — wraps `api.createCategory` + reloads (returns the new id for focus management).
  - `remove(id)` — wraps `api.deleteCategory` + reloads.

### Frontend — delete

Nothing in this milestone. M1 already removed `web/src/views/Category.svelte` and the category inline UI from `Sidebar.svelte`. The `Sidebar.svelte` file itself is deleted by M1.

---

## TDD posture

Per `CLAUDE.md`: TDD non-negotiable on branches, state, error handling. Pure layout/CSS in the desktop card chrome and mobile sheet chrome (no behaviour) is exempt.

**TDD-required (backend):**

- Migration applies cleanly on an existing populated DB (`TestMigrate_0011_CategoryPosition_OnPopulatedDB`).
- `InsertCategory` assigns the next position per user (`TestInsertCategory_AssignsNextPosition`).
- `ListCategories` orders by `position ASC, name COLLATE NOCASE` (`TestListCategories_OrdersByPosition`).
- `ReorderCategories` rewrites positions atomically, rejects missing/extra IDs, is scoped per user (`TestReorderCategories_*`).
- API: `POST /api/v1/categories/reorder` happy path, validation errors, cross-user rejection (`TestCategoriesAPI_Reorder_*`).
- DTO carries `position` field (`TestCategoriesAPI_DTOIncludesPosition`).

**TDD-required (frontend):**

- `Categories.svelte`: loads on mount; renders cards; renders Uncategorised pseudo-card only when uncategorised feeds exist; renders empty state when categories list is empty; handles store error.
- Inline rename: clicking title swaps to input; Enter commits; Esc cancels; empty trim cancels without API call.
- Inline create: "+ New" reveals input; Enter commits; Esc cancels.
- Reorder up/down: clicking the action calls `categories.reorder` with the new order; top card disables "Reorder up"; bottom card disables "Reorder down"; Uncategorised has neither.
- Mark-all-read: clicking the action opens the dialog; clicking Confirm calls `categories.markRead(id)` and refreshes entries; disabled when unread = 0.
- Delete: clicking opens the dialog; Confirm calls `categories.remove(id)`; dialog lists affected feeds.
- Reassign popover: clicking trigger opens; clicking item calls `categories.reassignSubscription`; clicking outside closes; Esc closes.
- Mobile bottom sheet: tapping a feed's Move opens the sheet; tapping an item closes it and reassigns; tapping backdrop closes without reassigning.
- Keyboard `R` on focused card opens rename. `?` shortcut listing in HotkeysModal is M1's job.

**Exempt:** the surrounding `.ts-cats-list` chrome inside `Categories.svelte` if it has no branching beyond `{#each}` and `{#if list.length === 0}`, and the static empty-state markup if it's a pure pass-through to `EmptyState.svelte`.

---

## Skills and tools for implementers

Implementers should invoke these skills via the `Skill` tool when starting the relevant tasks:

- `superpowers:test-driven-development` — every TDD-required task.
- `svelte-runes` — every Svelte component task; especially `$state`, `$derived`, `$effect`, and `$props` patterns.
- `svelte-styling` — for scoped `<style>` blocks, CSS custom properties, and `:global` overrides on third-party class names.
- `svelte-template-directives` — `{@render}`, `{@const}` (used inside the cards' `{#each}` blocks).
- `tdd` — the red-green-refactor cycle for any new helper logic (e.g. the day-band-style grouping is reused).
- `golang-database` — migration + reorder transaction; specifically transactions, parameterised queries, NULLable column handling.
- `golang-testing` — table-driven tests for `ReorderCategories` and the API handler.
- `golang-error-handling` — wrapping `%w` for new DB and API errors, sentinel `ErrCategoryReorderMismatch`.

MCP servers:

- `mcp__plugin_context7_context7__query-docs` — for Svelte 5 rune semantics if `svelte-runes` doesn't cover a corner case.
- `mcp__plugin_playwright_playwright__*` — only if the implementer wants a manual smoke; not part of the test suite.

Do **not** invoke `superpowers:writing-skills`, `superpowers:dispatching-parallel-agents`, or `claude-md-management:*` for this plan; they don't apply.

---

## Step-by-step tasks

### Task 1: Backend migration — add `position` column

**Files:**
- Create: `internal/db/migrations/0011_category_position.sql`
- Test: `internal/db/migrate_test.go` (append `TestMigrate_0011_CategoryPosition_OnPopulatedDB`)

- [ ] **Step 1: Write the failing test in `internal/db/migrate_test.go`**

```go
func TestMigrate_0011_CategoryPosition_OnPopulatedDB(t *testing.T) {
	t.Parallel()
	// Apply migrations through 0010 only, insert two categories with the
	// pre-0011 schema, then apply 0011 and verify the new column exists with
	// default 0 on both rows. Order is deterministic by id since position
	// defaults to 0 on both.
	d := openMemDB(t)
	require.NoError(t, applyMigrationsUpTo(t, d, "0010_tombstones.sql"))
	uid := insertTestUser(t, d, "alice")
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO categories (user_id, name, created_at) VALUES (?, 'A', 0), (?, 'B', 0)`,
		uid, uid)
	require.NoError(t, err)

	require.NoError(t, Migrate(context.Background(), d))

	var pa, pb int64
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT position FROM categories WHERE name = 'A'`).Scan(&pa))
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT position FROM categories WHERE name = 'B'`).Scan(&pb))
	require.Equal(t, int64(0), pa)
	require.Equal(t, int64(0), pb)
}
```

Note for the implementer: `applyMigrationsUpTo` and `openMemDB` may not exist with these exact names. If not, replicate the helper pattern already used in `migrate_test.go` (look at existing tests in that file) — apply migrations one by one through `0010`, insert seed rows, then run the rest. If no per-step helper exists, write a minimal one in the test file or use a temp `embed.FS` slice. Do **not** invent helpers in production code.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/db -run TestMigrate_0011_CategoryPosition_OnPopulatedDB -race -v`
Expected: FAIL — migration `0011_category_position.sql` does not exist, or `position` column missing.

- [ ] **Step 3: Create the migration**

`internal/db/migrations/0011_category_position.sql`:

```sql
ALTER TABLE categories ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_categories_user_position ON categories(user_id, position);
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/db -run TestMigrate_0011_CategoryPosition_OnPopulatedDB -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrations/0011_category_position.sql internal/db/migrate_test.go
git commit -m "db: add position column to categories (0011)"
```

---

### Task 2: `Category` struct + `ListCategories` ordering

**Files:**
- Modify: `internal/db/categories.go`
- Test: `internal/db/categories_test.go` (append `TestListCategories_OrdersByPosition`)

- [ ] **Step 1: Write the failing test**

Append to `internal/db/categories_test.go`:

```go
func TestListCategories_OrdersByPosition(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "alice")
	// Insert three categories; we set positions manually.
	for i, name := range []string{"Charlie", "Alpha", "Bravo"} {
		id, err := InsertCategory(context.Background(), d, NewCategory{UserID: userID, Name: name, CreatedAt: time.Now().Unix()})
		require.NoError(t, err)
		_, err = d.ExecContext(context.Background(),
			`UPDATE categories SET position = ? WHERE id = ?`, int64(2-i), id)
		require.NoError(t, err)
	}
	cats, err := ListCategories(context.Background(), d, userID)
	require.NoError(t, err)
	require.Len(t, cats, 3)
	require.Equal(t, "Bravo", cats[0].Name)   // position 0
	require.Equal(t, "Alpha", cats[1].Name)   // position 1
	require.Equal(t, "Charlie", cats[2].Name) // position 2
	require.Equal(t, int64(0), cats[0].Position)
	require.Equal(t, int64(1), cats[1].Position)
	require.Equal(t, int64(2), cats[2].Position)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/db -run TestListCategories_OrdersByPosition -race -v`
Expected: FAIL — `Position` field does not exist on `Category`, or order is by name.

- [ ] **Step 3: Implement**

Edit `internal/db/categories.go` to add the `Position int64` field. The existing fields (`ID`, `UserID`, `Name`, `CreatedAt`, `Unread`) stay as-is — only `Position` is new:

```go
type Category struct {
	ID        int64
	UserID    int64
	Name      string
	CreatedAt int64
	Position  int64 // NEW — every other field is unchanged
	Unread    int
}
```

Replace the `ListCategories` body's SELECT and ORDER BY:

```go
func ListCategories(ctx context.Context, d *sql.DB, userID int64) ([]Category, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.name, c.created_at, c.position,
		       COUNT(CASE WHEN e.read = 0 THEN 1 END) AS unread
		FROM categories c
		LEFT JOIN subscriptions s ON s.category_id = c.id AND s.user_id = c.user_id
		LEFT JOIN entries e ON e.subscription_id = s.id
		WHERE c.user_id = ?
		GROUP BY c.id
		ORDER BY c.position ASC, c.name COLLATE NOCASE
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.CreatedAt, &c.Position, &c.Unread); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
```

Also update `GetCategory` to scan `position`:

```go
func GetCategory(ctx context.Context, d *sql.DB, id, userID int64) (Category, error) {
	var c Category
	err := d.QueryRowContext(ctx,
		`SELECT id, user_id, name, created_at, position FROM categories WHERE id = ? AND user_id = ?`,
		id, userID).Scan(&c.ID, &c.UserID, &c.Name, &c.CreatedAt, &c.Position)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Category{}, ErrCategoryNotFound
		}
		return Category{}, fmt.Errorf("get category %d: %w", id, err)
	}
	return c, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/db -run TestListCategories_OrdersByPosition -race -v`
Expected: PASS.

Also run the full `internal/db` test package to catch regressions:

Run: `go test ./internal/db -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/categories.go internal/db/categories_test.go
git commit -m "db: order categories by position; expose Position on Category"
```

---

### Task 3: `InsertCategory` assigns next position

**Files:**
- Modify: `internal/db/categories.go`
- Test: `internal/db/categories_test.go` (append `TestInsertCategory_AssignsNextPosition`)

- [ ] **Step 1: Write the failing test**

```go
func TestInsertCategory_AssignsNextPosition(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "alice")
	u2 := insertTestUser(t, d, "bob")
	mk := func(uid int64, name string) Category {
		id, err := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: name, CreatedAt: time.Now().Unix()})
		require.NoError(t, err)
		c, err := GetCategory(context.Background(), d, id, uid)
		require.NoError(t, err)
		return c
	}
	a1 := mk(u1, "A")
	a2 := mk(u1, "B")
	a3 := mk(u1, "C")
	b1 := mk(u2, "X") // different user starts at 0 independently
	require.Equal(t, int64(0), a1.Position)
	require.Equal(t, int64(1), a2.Position)
	require.Equal(t, int64(2), a3.Position)
	require.Equal(t, int64(0), b1.Position)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/db -run TestInsertCategory_AssignsNextPosition -race -v`
Expected: FAIL — `InsertCategory` does not set position; all rows are `position = 0` (the column default).

- [ ] **Step 3: Implement**

Replace `InsertCategory` in `internal/db/categories.go`:

```go
func InsertCategory(ctx context.Context, d *sql.DB, c NewCategory) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO categories (user_id, name, created_at, position)
		VALUES (
			?, ?, ?,
			(SELECT COALESCE(MAX(position), -1) + 1 FROM categories WHERE user_id = ?)
		)
	`, c.UserID, c.Name, c.CreatedAt, c.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: categories.user_id, categories.name") {
			return 0, ErrCategoryNameTaken
		}
		return 0, fmt.Errorf("insert category: %w", err)
	}
	return res.LastInsertId()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/db -run TestInsertCategory_AssignsNextPosition -race -v`
Expected: PASS.

Also rerun `TestInsertCategory_Roundtrip` and `TestInsertCategory_DuplicateNameSameUser` to ensure no regression.

- [ ] **Step 5: Commit**

```bash
git add internal/db/categories.go internal/db/categories_test.go
git commit -m "db: InsertCategory assigns the next per-user position"
```

---

### Task 4: `ReorderCategories` helper

**Files:**
- Modify: `internal/db/categories.go`
- Test: `internal/db/categories_test.go` (append four tests)

- [ ] **Step 1: Write failing tests**

```go
func TestReorderCategories_RewritesPositions(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := insertTestUser(t, d, "alice")
	ids := make([]int64, 3)
	for i, n := range []string{"A", "B", "C"} {
		id, err := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: n, CreatedAt: time.Now().Unix()})
		require.NoError(t, err)
		ids[i] = id
	}
	// Reverse order: C, B, A.
	require.NoError(t, ReorderCategories(context.Background(), d, uid, []int64{ids[2], ids[1], ids[0]}))
	cats, err := ListCategories(context.Background(), d, uid)
	require.NoError(t, err)
	require.Equal(t, "C", cats[0].Name)
	require.Equal(t, "B", cats[1].Name)
	require.Equal(t, "A", cats[2].Name)
}

func TestReorderCategories_RejectsMissingIDs(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := insertTestUser(t, d, "alice")
	a, err := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: "A", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	_, err = InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: "B", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	err = ReorderCategories(context.Background(), d, uid, []int64{a}) // missing B
	require.ErrorIs(t, err, ErrCategoryReorderMismatch)
}

func TestReorderCategories_RejectsExtraOrCrossUserIDs(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "alice")
	u2 := insertTestUser(t, d, "bob")
	a, err := InsertCategory(context.Background(), d, NewCategory{UserID: u1, Name: "A", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	b, err := InsertCategory(context.Background(), d, NewCategory{UserID: u2, Name: "B", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	err = ReorderCategories(context.Background(), d, u1, []int64{a, b}) // b belongs to u2
	require.ErrorIs(t, err, ErrCategoryReorderMismatch)
}

func TestReorderCategories_Atomic(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := insertTestUser(t, d, "alice")
	a, _ := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: "A", CreatedAt: time.Now().Unix()})
	b, _ := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: "B", CreatedAt: time.Now().Unix()})
	// Duplicate ID in the order list — must be rejected, and positions must remain unchanged.
	err := ReorderCategories(context.Background(), d, uid, []int64{a, a})
	require.ErrorIs(t, err, ErrCategoryReorderMismatch)
	cats, err := ListCategories(context.Background(), d, uid)
	require.NoError(t, err)
	require.Equal(t, "A", cats[0].Name)
	require.Equal(t, int64(0), cats[0].Position)
	require.Equal(t, "B", cats[1].Name)
	require.Equal(t, int64(1), cats[1].Position)
	_ = b
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/db -run TestReorderCategories -race -v`
Expected: FAIL — `ReorderCategories` and `ErrCategoryReorderMismatch` do not exist.

- [ ] **Step 3: Implement in `internal/db/categories.go`**

```go
var ErrCategoryReorderMismatch = errors.New("category reorder ID set does not match user's categories")

// ReorderCategories rewrites the position column for every category owned by
// userID. orderedIDs must contain exactly the user's full set of category IDs
// (no duplicates, no extras, no missing). Position 0 is assigned to
// orderedIDs[0], position 1 to orderedIDs[1], and so on. Runs in a single
// transaction; on validation failure no rows change.
func ReorderCategories(ctx context.Context, d *sql.DB, userID int64, orderedIDs []int64) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // committed on success path

	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM categories WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("list user category ids: %w", err)
	}
	have := make(map[int64]struct{})
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan id: %w", err)
		}
		have[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate ids: %w", err)
	}
	rows.Close()

	if len(orderedIDs) != len(have) {
		return ErrCategoryReorderMismatch
	}
	seen := make(map[int64]struct{}, len(orderedIDs))
	for _, id := range orderedIDs {
		if _, ok := have[id]; !ok {
			return ErrCategoryReorderMismatch
		}
		if _, dup := seen[id]; dup {
			return ErrCategoryReorderMismatch
		}
		seen[id] = struct{}{}
	}

	for i, id := range orderedIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE categories SET position = ? WHERE id = ? AND user_id = ?`,
			int64(i), id, userID); err != nil {
			return fmt.Errorf("update position for %d: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/db -run TestReorderCategories -race -v`
Expected: PASS for all four.

Run `go test ./internal/db -race` to catch regressions.

- [ ] **Step 5: Commit**

```bash
git add internal/db/categories.go internal/db/categories_test.go
git commit -m "db: ReorderCategories — atomic per-user position rewrite"
```

---

### Task 5: API — reorder endpoint + DTO `position` + outer-mux wiring

**Files:**
- Modify: `internal/api/categories.go`, `internal/api/errors.go`, `internal/api/api.go`, `internal/api/testing.go`
- Test: `internal/api/categories_test.go` (append)

- [ ] **Step 1: Write failing tests**

Append to `internal/api/categories_test.go`:

```go
func TestCategoriesAPI_DTOIncludesPosition(t *testing.T) {
	t.Parallel()
	mux, _ := newCatTestSetup(t)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories",
		strings.NewReader(`{"name":"Tech"}`)))
	require.Equal(t, http.StatusCreated, w.Code)
	var cat categoryDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cat))
	require.Equal(t, int64(0), cat.Position)
	// Ensure the JSON wire key is present.
	require.Contains(t, w.Body.String(), `"position"`)
}

func TestCategoriesAPI_Reorder_Happy(t *testing.T) {
	t.Parallel()
	mux, _ := newCatTestSetup(t)

	mkCat := func(name string) int64 {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories",
			strings.NewReader(`{"name":"`+name+`"}`)))
		require.Equal(t, http.StatusCreated, w.Code)
		var c categoryDTO
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &c))
		return c.ID
	}
	a, b, c := mkCat("A"), mkCat("B"), mkCat("C")

	body := fmt.Sprintf(`{"order":[%d,%d,%d]}`, c, a, b)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories/reorder",
		strings.NewReader(body)))
	require.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/categories", nil))
	var listResp struct{ Data []categoryDTO `json:"data"` }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 3)
	require.Equal(t, c, listResp.Data[0].ID)
	require.Equal(t, a, listResp.Data[1].ID)
	require.Equal(t, b, listResp.Data[2].ID)
}

func TestCategoriesAPI_Reorder_Mismatch(t *testing.T) {
	t.Parallel()
	mux, _ := newCatTestSetup(t)

	// Create one category.
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories",
		strings.NewReader(`{"name":"A"}`)))
	require.Equal(t, http.StatusCreated, w.Code)
	var c categoryDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &c))

	// Submit reorder with an extra (non-existent) ID.
	body := fmt.Sprintf(`{"order":[%d,9999]}`, c.ID)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories/reorder",
		strings.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, w.Code)
	var resp ErrorEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, ErrCodeReorderMismatch, resp.Error.Code)
}

func TestCategoriesAPI_Reorder_BadJSON(t *testing.T) {
	t.Parallel()
	mux, _ := newCatTestSetup(t)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories/reorder",
		strings.NewReader(`{not json`)))
	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

Add `fmt` to the test file imports if not already present.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api -run TestCategoriesAPI_Reorder -race -v`
Expected: FAIL — endpoint and `ErrCodeReorderMismatch` do not exist.

- [ ] **Step 3: Implement — add error code**

Edit `internal/api/errors.go`, inside the `// M9 error codes.` block add:

```go
	ErrCodeReorderMismatch  = "category_reorder_mismatch"
```

- [ ] **Step 4: Implement — DTO + endpoint**

In `internal/api/categories.go`:

```go
type categoryDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Position  int64  `json:"position"`
	Unread    int    `json:"unread"`
	CreatedAt int64  `json:"created_at"`
}

func toCategoryDTO(c db.Category) categoryDTO {
	return categoryDTO{
		ID:        c.ID,
		Name:      c.Name,
		Position:  c.Position,
		Unread:    c.Unread,
		CreatedAt: c.CreatedAt,
	}
}
```

Append a new route registration inside `registerCategoryRoutes`. **Important:** registering on the inner `catsMux` is necessary but not sufficient. The outer mux at `internal/api/api.go` and `internal/api/testing.go` explicitly enumerates every category route — without adding the new path there, the request never reaches `catsMux` and the endpoint 404s.

In `internal/api/api.go`, inside the production route table (currently lines 167-187), add:

```go
	{"POST", "/api/v1/categories/reorder", authedCSRF(catsMux)},
```

In `internal/api/testing.go`, inside the test route table (currently lines 65-85), add:

```go
	{"POST", "/api/v1/categories/reorder", inject(catsMux)},
```

Both must be added. The order within the table doesn't matter; grouping it next to the other `/api/v1/categories/*` entries is the natural place.

Now add the handler:

```go
	m.HandleFunc("POST /api/v1/categories/reorder", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			Order []int64 `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		if err := db.ReorderCategories(r.Context(), d, u.ID, body.Order); err != nil {
			if errors.Is(err, db.ErrCategoryReorderMismatch) {
				writeError(w, http.StatusBadRequest, ErrCodeReorderMismatch, "order list does not match this user's categories")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api -run TestCategoriesAPI -race -v`
Expected: PASS for all category API tests, including the new ones.

Run: `make test`
Expected: PASS. (This also covers the SPA build dependency.)

- [ ] **Step 6: Commit**

```bash
git add internal/api/categories.go internal/api/categories_test.go internal/api/errors.go internal/api/api.go internal/api/testing.go
git commit -m "api: POST /categories/reorder; include position in DTO"
```

---

### Task 5a: `db.MarkSubscriptionRead`

**Files:**
- Modify: `internal/db/subscriptions.go`
- Test: `internal/db/subscriptions_test.go` (append)

- [ ] **Step 1: Write failing tests**

Append to `internal/db/subscriptions_test.go`:

```go
func TestMarkSubscriptionRead_ScopedToUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "alice")
	u2 := insertTestUser(t, d, "bob")
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
	u1 := insertTestUser(t, d, "alice")
	u2 := insertTestUser(t, d, "bob")
	sub, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: u1, Title: "F", FeedURL: "https://u1/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h', 'E', '', 'https://u1/1', '<p>x</p>', ?, ?, 0, 0, ?)`,
		sub, time.Now().Unix(), time.Now().Unix(), u1)
	require.NoError(t, err)

	// u2 cannot mark u1's subscription read — must be a silent no-op (no error,
	// no rows affected). The handler layer translates "0 affected for wrong
	// user" into a 404 by checking ownership before calling this helper.
	require.NoError(t, MarkSubscriptionRead(context.Background(), d, sub, u2))

	var n int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT read FROM entries WHERE subscription_id = ?`, sub).Scan(&n))
	require.Equal(t, 0, n, "u1's entry must remain unread when u2 tried to mark it read")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/db -run TestMarkSubscriptionRead -race -v`
Expected: FAIL — `MarkSubscriptionRead` does not exist.

- [ ] **Step 3: Implement**

Append to `internal/db/subscriptions.go`:

```go
// MarkSubscriptionRead sets read = 1 on every unread entry under the given
// subscription, scoped to userID. Silent no-op when the subscription does not
// belong to userID. The handler layer is responsible for translating
// "no matching subscription" into a 404 by calling GetSubscription first.
func MarkSubscriptionRead(ctx context.Context, d *sql.DB, subscriptionID, userID int64) error {
	_, err := d.ExecContext(ctx, `
		UPDATE entries SET read = 1
		WHERE read = 0
		  AND subscription_id = ?
		  AND subscription_id IN (
		      SELECT id FROM subscriptions WHERE id = ? AND user_id = ?
		  )
	`, subscriptionID, subscriptionID, userID)
	if err != nil {
		return fmt.Errorf("mark subscription read %d: %w", subscriptionID, err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/db -run TestMarkSubscriptionRead -race -v`
Expected: PASS for both cases.

Run: `go test ./internal/db -race`
Expected: PASS for the full db package.

- [ ] **Step 5: Commit**

```bash
git add internal/db/subscriptions.go internal/db/subscriptions_test.go
git commit -m "db: MarkSubscriptionRead — per-user, idempotent"
```

---

### Task 5b: API — `POST /subscriptions/:id/mark-read` + outer-mux wiring

**Files:**
- Modify: `internal/api/subscriptions.go`, `internal/api/api.go`, `internal/api/testing.go`
- Test: `internal/api/subscriptions_test.go` (append)

- [ ] **Step 1: Write failing tests**

```go
func TestSubscriptionsAPI_MarkRead_Happy(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertAPITestUser(t, d, "subuser")
	user := db.User{ID: userID, Username: "subuser", Role: "admin"}
	mux := NewTestMux(d, TestMuxOpts{TestUser: user})

	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: userID, Title: "F", FeedURL: "https://x/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h', 'E', '', 'https://x/1', '<p>x</p>', ?, ?, 0, 0, ?)`,
		subID, time.Now().Unix(), time.Now().Unix(), userID)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST",
		fmt.Sprintf("/api/v1/subscriptions/%d/mark-read", subID), nil))
	require.Equal(t, http.StatusNoContent, w.Code)

	var n int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT read FROM entries WHERE subscription_id = ?`, subID).Scan(&n))
	require.Equal(t, 1, n)
}

func TestSubscriptionsAPI_MarkRead_OtherUserGets404(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertAPITestUser(t, d, "u1")
	u2 := insertAPITestUser(t, d, "u2")
	muxAsU2 := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: u2, Username: "u2"}})

	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: u1, Title: "F", FeedURL: "https://x/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	muxAsU2.ServeHTTP(w, httptest.NewRequest("POST",
		fmt.Sprintf("/api/v1/subscriptions/%d/mark-read", subID), nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSubscriptionsAPI_MarkRead_BadID(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertAPITestUser(t, d, "subuser")
	mux := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: userID, Username: "subuser"}})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/subscriptions/abc/mark-read", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api -run TestSubscriptionsAPI_MarkRead -race -v`
Expected: FAIL — endpoint does not exist; test mux 404s.

- [ ] **Step 3: Implement — outer mux wiring**

In `internal/api/api.go` (inside the route table at lines 167-187), add:

```go
	{"POST", "/api/v1/subscriptions/{id}/mark-read", authedCSRF(subsMux)},
```

In `internal/api/testing.go` (inside the route table at lines 65-85), add:

```go
	{"POST", "/api/v1/subscriptions/{id}/mark-read", inject(subsMux)},
```

- [ ] **Step 4: Implement — handler**

In `internal/api/subscriptions.go`, register a new handler inside `registerSubscriptionRoutes`:

```go
	m.HandleFunc("POST /api/v1/subscriptions/{id}/mark-read", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		// Ownership check — return 404 (not 403) on a wrong-user attempt to
		// avoid leaking the existence of another user's subscription IDs.
		// db.GetSubscription wraps sql.ErrNoRows via fmt.Errorf("%w") so
		// errors.Is(err, sql.ErrNoRows) is the canonical "not found" check.
		if _, err := db.GetSubscription(r.Context(), d, id, u.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.MarkSubscriptionRead(r.Context(), d, id, u.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
```

Add `"database/sql"` to the imports of `internal/api/subscriptions.go` if it isn't already present.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api -run TestSubscriptionsAPI_MarkRead -race -v`
Expected: PASS for all three.

Run: `make test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/subscriptions.go internal/api/subscriptions_test.go internal/api/api.go internal/api/testing.go
git commit -m "api: POST /subscriptions/:id/mark-read"
```

---

### Task 6: Frontend types + API client + store extensions

**Files:**
- Modify: `web/src/lib/types.ts`, `web/src/lib/api.ts`, `web/src/lib/store.ts`
- Test: `web/src/lib/__tests__/api.test.ts`, `web/src/lib/__tests__/store.test.ts` (append)

- [ ] **Step 1: Write failing tests**

Append to `web/src/lib/__tests__/api.test.ts`:

```ts
describe('api.reorderCategories', () => {
  it('POSTs the ordered ID list to /api/v1/categories/reorder', async () => {
    const fetchSpy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchSpy);
    await api.reorderCategories([3, 1, 2]);
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [url, init] = fetchSpy.mock.calls[0];
    expect(url).toBe('/api/v1/categories/reorder');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body as string)).toEqual({ order: [3, 1, 2] });
  });
});
```

Append to `web/src/lib/__tests__/store.test.ts` (mock `api`):

```ts
describe('categories store', () => {
  it('reassignSubscription PATCHes the subscription and reloads', async () => {
    const patchSpy = vi.spyOn(api, 'patchSubscription').mockResolvedValueOnce(undefined as any);
    const loadSubs = vi.spyOn(api, 'listSubscriptions').mockResolvedValueOnce([] as any);
    await categories.reassignSubscription(5, 7);
    expect(patchSpy).toHaveBeenCalledWith(5, { category_id: 7 });
    expect(loadSubs).toHaveBeenCalled();
  });

  it('reorder calls api.reorderCategories with the new order and reloads', async () => {
    const reorderSpy = vi.spyOn(api, 'reorderCategories').mockResolvedValueOnce(undefined as any);
    const listSpy = vi.spyOn(api, 'listCategories').mockResolvedValueOnce([] as any);
    await categories.reorder([3, 1, 2]);
    expect(reorderSpy).toHaveBeenCalledWith([3, 1, 2]);
    expect(listSpy).toHaveBeenCalled();
  });

  it('markRead(id) calls api.markCategoryRead and reloads entries', async () => {
    const markSpy = vi.spyOn(api, 'markCategoryRead').mockResolvedValueOnce(undefined as any);
    const entriesSpy = vi.spyOn(api, 'listEntries').mockResolvedValueOnce({ data: [] } as any);
    await categories.markRead(7);
    expect(markSpy).toHaveBeenCalledWith(7);
    expect(entriesSpy).toHaveBeenCalled();
  });

  it('markRead(null) calls api.markSubscriptionRead for each uncategorised subscription', async () => {
    // Tasks 5a + 5b shipped POST /api/v1/subscriptions/:id/mark-read and
    // db.MarkSubscriptionRead. This test asserts the SPA uses the dedicated
    // endpoint rather than N entry-level PATCHes.
    const markFeed = vi.spyOn(api, 'markSubscriptionRead').mockResolvedValue(undefined as any);
    vi.spyOn(api, 'listSubscriptions').mockResolvedValueOnce([
      { id: 1, category_id: null }, { id: 2, category_id: null }, { id: 3, category_id: 4 },
    ] as any);
    await subscriptions.load();
    await categories.markRead(null);
    expect(markFeed).toHaveBeenCalledTimes(2);
    expect(markFeed).toHaveBeenCalledWith(1);
    expect(markFeed).toHaveBeenCalledWith(2);
  });

  it('reorder rolls back the in-memory order when the API rejects', async () => {
    // Seed the store with [A=1, B=2, C=3] in their natural position order.
    vi.spyOn(api, 'listCategories').mockResolvedValueOnce([
      { id: 1, name: 'A', unread: 0, created_at: 0, position: 0 },
      { id: 2, name: 'B', unread: 0, created_at: 0, position: 1 },
      { id: 3, name: 'C', unread: 0, created_at: 0, position: 2 },
    ] as any);
    await categories.load();

    // First call (the user's reorder attempt) rejects; the rollback path must
    // restore the original order without a second list call.
    vi.spyOn(api, 'reorderCategories').mockRejectedValueOnce(new Error('boom'));

    await expect(categories.reorder([3, 1, 2])).rejects.toThrow('boom');

    const after = get(categories);
    expect(after.map(c => c.id)).toEqual([1, 2, 3]);
  });
});
```

Note: the rollback test uses `get` from `svelte/store`. Add it to the test file's imports if it isn't already there. The `categories` store snapshot returned by `get()` must equal the pre-reorder list — that's the contract `reorderInMemory` + the catch-block rollback guarantee in Task 6 Step 5.

**Backend dependency note.** Tasks 5a (`db.MarkSubscriptionRead`) and 5b (`POST /api/v1/subscriptions/:id/mark-read`) must merge first; this task assumes they exist. `api.markSubscriptionRead` in step 4 below is a thin wrapper around that endpoint — no fallback path is needed.

- [ ] **Step 2: Run tests to verify they fail**

Run: `pnpm --dir web test -- src/lib/__tests__/api.test.ts src/lib/__tests__/store.test.ts`
Expected: FAIL — `api.reorderCategories` and `categories.reorder` do not exist.

- [ ] **Step 3: Implement — types**

Edit `web/src/lib/types.ts`:

```ts
export type Category = {
  id: number;
  name: string;
  unread: number;
  created_at: number;
  position: number;
};
```

- [ ] **Step 4: Implement — api client**

Edit `web/src/lib/api.ts`. Two new methods alongside the existing categories and subscriptions sections:

```ts
  // Categories — M-Redesign-4.
  reorderCategories: (orderedIds: number[]) =>
    request<void>('/categories/reorder', {
      method: 'POST',
      body: JSON.stringify({ order: orderedIds }),
    }),

  // Subscriptions — M-Redesign-4 (Uncategorised mark-all-read).
  markSubscriptionRead: (id: number) =>
    request<void>(`/subscriptions/${id}/mark-read`, { method: 'POST', body: '{}' }),
```

If `api.patchSubscription` is not yet defined in this file (M9 added category PATCH on subscriptions; verify before assuming), the implementer adds the standard wrapper:

```ts
  patchSubscription: (id: number, body: Partial<{ category_id: number | null; extract: boolean; extract_selector: string; cookie: string; basic_auth_user: string; basic_auth_pass: string }>) =>
    request<Subscription>(`/subscriptions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
```

— mirroring the existing `PATCH /api/v1/subscriptions/{id}` route in `internal/api/subscriptions.go`. Only add this wrapper if a grep of `web/src/lib/api.ts` for `patchSubscription` returns nothing.

- [ ] **Step 5: Implement — store extensions**

Edit `web/src/lib/store.ts`'s `categoriesStore` factory to return additional methods:

```ts
function categoriesStore() {
  const { subscribe, set, update } = writable<Category[]>([]);
  return {
    subscribe,
    async load() {
      try {
        set(await api.listCategories());
      } catch (e) {
        console.error('categories.load failed:', e);
      }
    },
    async create(name: string) {
      const c = await api.createCategory(name);
      await this.load();
      return c;
    },
    async rename(id: number, name: string) {
      await api.renameCategory(id, name);
      await this.load();
    },
    async remove(id: number) {
      await api.deleteCategory(id);
      await Promise.all([this.load(), subscriptions.load()]);
    },
    async reorder(orderedIds: number[]) {
      // Optimistic: reorder in-memory immediately.
      let snapshot: Category[] = [];
      update(cs => { snapshot = cs.slice(); return reorderInMemory(cs, orderedIds); });
      try {
        await api.reorderCategories(orderedIds);
      } catch (e) {
        set(snapshot); // rollback
        throw e;
      }
      await this.load();
    },
    async reassignSubscription(subId: number, categoryId: number | null) {
      await api.patchSubscription(subId, { category_id: categoryId });
      await Promise.all([subscriptions.load(), this.load()]);
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/categories'] });
    },
    async markRead(categoryId: number | null) {
      if (categoryId !== null) {
        await api.markCategoryRead(categoryId);
      } else {
        // Uncategorised: hit the dedicated per-subscription endpoint shipped by
        // Task 5b for each uncategorised feed. No fallback path — the backend
        // dependency is hard.
        const uncatIds = get(subscriptions).filter(s => s.category_id == null).map(s => s.id);
        await Promise.all(uncatIds.map(id => api.markSubscriptionRead(id)));
      }
      await Promise.all([entries.load(), this.load()]);
      notifySW({ type: 'invalidate', paths: ['/api/v1/entries', '/api/v1/categories'] });
    },
  };
}

function reorderInMemory(cs: Category[], orderedIds: number[]): Category[] {
  const byId = new Map(cs.map(c => [c.id, c]));
  const out: Category[] = [];
  for (const id of orderedIds) {
    const c = byId.get(id);
    if (c) out.push({ ...c, position: out.length });
  }
  return out;
}
```

Add `import { get } from 'svelte/store';` to the top of `web/src/lib/store.ts` if not already imported. The factory's `writable` destructure already needs `update` for the optimistic-reorder rollback path — make sure the line reads `const { subscribe, set, update } = writable<Category[]>([]);`, not `const { subscribe, set }` as the current file does.

- [ ] **Step 6: Run tests to verify they pass**

Run: `pnpm --dir web test -- src/lib/__tests__/api.test.ts src/lib/__tests__/store.test.ts`
Expected: PASS.

Run: `pnpm --dir web run check`
Expected: PASS — no svelte-check / TS errors.

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/types.ts web/src/lib/api.ts web/src/lib/store.ts web/src/lib/__tests__/api.test.ts web/src/lib/__tests__/store.test.ts
git commit -m "spa: extend categories store with create/rename/remove/reorder/reassignSubscription/markRead"
```

---

### Task 7: `CategoryReassignPopover.svelte` (cross-milestone shared component)

**Files:**
- Create: `web/src/components/CategoryReassignPopover.svelte`, `web/src/components/__tests__/CategoryReassignPopover.test.ts`

**Ownership and scope (load-bearing).** M4 owns this component per the team-lead's cross-plan decision: M4 ships first in milestone order and owns category semantics (Uncategorised pseudo-category, ordering, etc.). **M-Redesign-5 (Feeds management) consumes this component as-is** — once for the per-row category change action on a feed row, and once for the bulk "Set category…" action in the multi-select toolbar. M5 will **not** create its own popover.

Path: `web/src/components/CategoryReassignPopover.svelte`, directly under `components/` (not `components/categories/`) — keep the path stable across milestones so M5 imports cleanly.

**Public contract (frozen).** The prop shape below is the cross-milestone contract. Do not break it without coordinating with M5's planner; if a new prop is needed, add it as optional with a sensible default. Any breaking change must be flagged in the umbrella spec.

```ts
type Props = {
  /** Whether the popover is visible. M4's parent toggles this on click. M5 mirrors. */
  open: boolean;
  /** The anchor element the popover positions itself against. Passed through
   *  to M1's Popover primitive. May be null while opening; the primitive should
   *  no-op until anchor is non-null. */
  anchor: HTMLElement | null;
  /** Subject feed's display name — rendered in the eyebrow ("Move <b>jvns</b> to"). */
  feedName: string;
  /** Current category id, or null for an uncategorised feed. The matching item
   *  renders bold accent + leading check. */
  currentCategoryId: number | null;
  /** Full category list. Caller is responsible for ordering (typically by
   *  position ASC, name COLLATE NOCASE — the same order ListCategories returns). */
  categories: Category[];
  /** Eyebrow verb. Defaults to "Move"; pass "Assign" when the feed is currently uncategorised. */
  label?: 'Move' | 'Assign';
  /** Called with the new category id (or null for Uncategorised). The consumer
   *  is responsible for closing the popover after dispatch — typically by
   *  setting open=false in the onPick handler. */
  onPick: (id: number | null) => void;
  /** Called when the user dismisses without picking (Esc, outside-click,
   *  clicking the trigger again). Primitive owns the keystroke + click-outside
   *  detection; this callback just notifies the consumer. */
  onClose: () => void;
};
```

The component composes M1's `Popover` primitive — it does **not** re-implement positioning, click-outside detection, or Esc-to-close. It only owns the menu content (the category list, current-item check, Uncategorised pseudo-row) and the visual treatment (`.ts-cat-pop` chrome).

- [ ] **Step 1: Write failing tests**

```ts
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// Mock M1's Popover primitive to a transparent pass-through so the test
// exercises this component's content + callbacks, not the primitive's
// positioning math. The primitive has its own tests in M1.
vi.mock('../Popover.svelte', () => ({
  default: (await import('./helpers/PopoverPassThrough.svelte')).default,
}));

import CategoryReassignPopover from '../CategoryReassignPopover.svelte';

const cats = [
  { id: 1, name: 'People', unread: 0, created_at: 0, position: 0 },
  { id: 2, name: 'Systems', unread: 0, created_at: 0, position: 1 },
];

function baseProps(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    open: true,
    anchor: document.createElement('div'),
    feedName: 'jvns',
    currentCategoryId: 1 as number | null,
    categories: cats,
    onPick: () => {},
    onClose: () => {},
    ...overrides,
  };
}

describe('CategoryReassignPopover', () => {
  it('renders every category plus Uncategorised when open', () => {
    render(CategoryReassignPopover, baseProps());
    expect(screen.getByText('People')).toBeTruthy();
    expect(screen.getByText('Systems')).toBeTruthy();
    expect(screen.getByText('Uncategorised')).toBeTruthy();
  });

  it('does not render content when open=false', () => {
    const { container } = render(CategoryReassignPopover, baseProps({ open: false }));
    expect(container.querySelector('.ts-cat-pop')).toBeNull();
  });

  it('marks the current category with .is-current', () => {
    const { container } = render(CategoryReassignPopover, baseProps({ currentCategoryId: 1 }));
    const current = container.querySelector('.ts-cat-pop-item.is-current');
    expect(current?.textContent).toContain('People');
  });

  it('marks Uncategorised current when currentCategoryId is null', () => {
    const { container } = render(CategoryReassignPopover, baseProps({ currentCategoryId: null }));
    const current = container.querySelector('.ts-cat-pop-item.is-current');
    expect(current?.textContent).toContain('Uncategorised');
  });

  it('calls onPick(id) when an item is clicked', async () => {
    const onPick = vi.fn();
    render(CategoryReassignPopover, baseProps({ onPick }));
    await fireEvent.click(screen.getByText('Systems'));
    expect(onPick).toHaveBeenCalledWith(2);
  });

  it('calls onPick(null) when Uncategorised is clicked', async () => {
    const onPick = vi.fn();
    render(CategoryReassignPopover, baseProps({ onPick }));
    await fireEvent.click(screen.getByText('Uncategorised'));
    expect(onPick).toHaveBeenCalledWith(null);
  });

  it('renders the eyebrow with the "Move" label by default', () => {
    render(CategoryReassignPopover, baseProps());
    expect(screen.getByText(/Move/)).toBeTruthy();
    expect(screen.getByText('jvns')).toBeTruthy();
  });

  it('renders the eyebrow with the "Assign" label when passed', () => {
    render(CategoryReassignPopover, baseProps({ label: 'Assign' }));
    expect(screen.getByText(/Assign/)).toBeTruthy();
  });
});
```

The `PopoverPassThrough.svelte` helper is a one-line shim — a Svelte component that renders its child slot/snippet unconditionally so the test bypasses M1's positioning. If M1's `Popover` API is snippet-based (`{#snippet content()}`), the helper renders `{@render content?.()}`; if it's a default slot, the helper renders `{@render children?.()}`. Implementer creates `web/src/components/__tests__/helpers/PopoverPassThrough.svelte` matching M1's actual API.

- [ ] **Step 2: Run tests to verify they fail**

Run: `pnpm --dir web test -- src/components/__tests__/CategoryReassignPopover.test.ts`
Expected: FAIL — component does not exist.

- [ ] **Step 3: Implement**

`web/src/components/CategoryReassignPopover.svelte`:

```svelte
<script lang="ts">
  import Popover from './Popover.svelte';
  import type { Category } from '../lib/types';

  type Props = {
    open: boolean;
    anchor: HTMLElement | null;
    feedName: string;
    currentCategoryId: number | null;
    categories: Category[];
    label?: 'Move' | 'Assign';
    onPick: (id: number | null) => void;
    onClose: () => void;
  };
  const { open, anchor, feedName, currentCategoryId, categories, label = 'Move', onPick, onClose }: Props = $props();
</script>

<Popover {open} {anchor} {onClose}>
  <div class="ts-cat-pop" role="menu" aria-label="{label} {feedName}">
    <div class="ts-cat-pop-eyebrow">{label} <b>{feedName}</b> to</div>
    <div class="ts-cat-pop-rule"></div>
    {#each categories as cat (cat.id)}
      <button
        class="ts-cat-pop-item"
        class:is-current={cat.id === currentCategoryId}
        onclick={() => onPick(cat.id)}
        role="menuitem"
      >
        <span class="check" aria-hidden="true">✓</span>
        <span>{cat.name}</span>
        <span class="ts-cat-pop-item-ct">{cat.unread}</span>
      </button>
    {/each}
    <div class="ts-cat-pop-rule"></div>
    <button
      class="ts-cat-pop-item is-uncat"
      class:is-current={currentCategoryId === null}
      onclick={() => onPick(null)}
      role="menuitem"
    >
      <span class="check" aria-hidden="true">✓</span>
      <span>Uncategorised</span>
      <span class="ts-cat-pop-item-ct"></span>
    </button>
  </div>
</Popover>

<style>
  /* Ports .ts-cat-pop, .ts-cat-pop-eyebrow, .ts-cat-pop-rule, .ts-cat-pop-item,
     .ts-cat-pop-item .check, .ts-cat-pop-item.is-current, .ts-cat-pop-item.is-uncat,
     .ts-cat-pop-item-ct from ui_design/styles.css:3002-3064.
     Tokens (--bg, --rule, --ink, --ink-3, --ink-4, --accent, --bg-soft) come from
     web/src/styles/tokens.css. Keep selector names identical to the upstream so
     theme overrides (.theme-dark .ts-cat-pop ...) work without re-prefixing.
     Positioning rules (position: absolute, top, right, z-index) are NOT ported
     here — M1's Popover primitive owns positioning. The .ts-cat-pop block below
     only owns visual chrome. */
  .ts-cat-pop {
    min-width: 220px;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 5px;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.18);
    padding: 4px;
  }
  :global(html.theme-dark) .ts-cat-pop { box-shadow: 0 20px 50px rgba(0, 0, 0, 0.55); }
  .ts-cat-pop-eyebrow {
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.12em;
    text-transform: uppercase; color: var(--ink-3);
    padding: 8px 10px 6px; display: flex; gap: 6px;
  }
  .ts-cat-pop-eyebrow b {
    color: var(--ink); font-weight: 500; text-transform: none; letter-spacing: 0.02em;
  }
  .ts-cat-pop-rule { height: 1px; background: var(--rule); margin: 2px 4px; }
  .ts-cat-pop-item {
    display: flex; align-items: center; gap: 8px; width: 100%;
    padding: 7px 10px; border: 0; background: transparent;
    text-align: left;
    font-family: var(--sans); font-size: 13px; font-weight: 500;
    color: var(--ink); cursor: pointer; border-radius: 3px;
  }
  .ts-cat-pop-item:hover { background: var(--bg-soft); }
  .ts-cat-pop-item .check {
    width: 10px; display: inline-flex; color: var(--accent);
    font-size: 11px; visibility: hidden;
  }
  .ts-cat-pop-item.is-current { color: var(--accent); }
  .ts-cat-pop-item.is-current .check { visibility: visible; }
  .ts-cat-pop-item.is-uncat { color: var(--ink-3); font-style: italic; font-weight: 400; }
  .ts-cat-pop-item-ct {
    margin-left: auto;
    font-family: var(--mono); font-size: 10.5px;
    color: var(--ink-4); font-style: normal; font-weight: 400;
  }
</style>
```

Note on M1's `Popover` API: this implementation assumes a default-slot/children-render API (`<Popover {open} {anchor} {onClose}>...content...</Popover>`). If M1 lands a snippet-based API (`{#snippet content()}...{/snippet}`), the implementer wraps the inner `<div class="ts-cat-pop">` in the appropriate snippet and updates the `PopoverPassThrough.svelte` helper to match. The contract `(open, anchor, onClose) -> content` stays the same.

- [ ] **Step 4: Run tests to verify they pass**

Run: `pnpm --dir web test -- src/components/__tests__/CategoryReassignPopover.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/CategoryReassignPopover.svelte web/src/components/__tests__/CategoryReassignPopover.test.ts
git commit -m "spa: CategoryReassignPopover component"
```

---

### Task 8: `CategoryReassignSheet.svelte` (mobile)

**Files:**
- Create: `web/src/components/CategoryReassignSheet.svelte`, `web/src/components/__tests__/CategoryReassignSheet.test.ts`

- [ ] **Step 1: Write failing tests**

```ts
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import CategoryReassignSheet from '../CategoryReassignSheet.svelte';

const cats = [{ id: 1, name: 'People', unread: 0, created_at: 0, position: 0 }];

describe('CategoryReassignSheet', () => {
  it('renders the grab handle, eyebrow, feed name, items, and Cancel', () => {
    render(CategoryReassignSheet, { open: true, feedName: 'jvns', currentCategoryId: null, categories: cats, onSelect: () => {}, onClose: () => {} });
    expect(screen.getByText('Move feed')).toBeTruthy();
    expect(screen.getByText('jvns')).toBeTruthy();
    expect(screen.getByText('People')).toBeTruthy();
    expect(screen.getByText('Uncategorised')).toBeTruthy();
    expect(screen.getByText('Cancel')).toBeTruthy();
  });

  it('does not render when open=false', () => {
    const { container } = render(CategoryReassignSheet, { open: false, feedName: 'x', currentCategoryId: null, categories: cats, onSelect: () => {}, onClose: () => {} });
    expect(container.querySelector('.m-cat-sheet')).toBeNull();
  });

  it('calls onClose when backdrop is clicked', async () => {
    const onClose = vi.fn();
    const { container } = render(CategoryReassignSheet, { open: true, feedName: 'x', currentCategoryId: null, categories: cats, onSelect: () => {}, onClose });
    await fireEvent.click(container.querySelector('.m-cat-sheet') as Element);
    expect(onClose).toHaveBeenCalled();
  });

  it('does NOT call onClose when card body is clicked', async () => {
    const onClose = vi.fn();
    const { container } = render(CategoryReassignSheet, { open: true, feedName: 'x', currentCategoryId: null, categories: cats, onSelect: () => {}, onClose });
    await fireEvent.click(container.querySelector('.m-cat-sheet-card') as Element);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('calls onSelect(id) then onClose when an item is tapped', async () => {
    const onSelect = vi.fn();
    const onClose = vi.fn();
    render(CategoryReassignSheet, { open: true, feedName: 'x', currentCategoryId: null, categories: cats, onSelect, onClose });
    await fireEvent.click(screen.getByText('People'));
    expect(onSelect).toHaveBeenCalledWith(1);
    expect(onClose).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Expected: FAIL — component does not exist.

- [ ] **Step 3: Implement**

`web/src/components/CategoryReassignSheet.svelte`:

```svelte
<script lang="ts">
  import type { Category } from '../lib/types';

  type Props = {
    open: boolean;
    feedName: string;
    currentCategoryId: number | null;
    categories: Category[];
    onSelect: (id: number | null) => void;
    onClose: () => void;
  };
  const { open, feedName, currentCategoryId, categories, onSelect, onClose }: Props = $props();

  function pick(id: number | null) {
    onSelect(id);
    onClose();
  }
</script>

{#if open}
  <div
    class="m-cat-sheet"
    role="dialog"
    aria-modal="true"
    aria-label="Move feed"
    onclick={onClose}
  >
    <div class="m-cat-sheet-card" onclick={(e) => e.stopPropagation()}>
      <div class="m-cat-sheet-handle" aria-hidden="true"></div>
      <div class="m-cat-sheet-head">
        <div class="m-cat-sheet-eyebrow">Move feed</div>
        <div class="m-cat-sheet-title"><span>{feedName}</span></div>
      </div>
      <div class="m-cat-sheet-list">
        {#each categories as cat (cat.id)}
          <button
            class="m-cat-sheet-item"
            class:is-current={cat.id === currentCategoryId}
            onclick={() => pick(cat.id)}
          >
            <span class="m-cat-sheet-item-check">✓</span>
            <span>{cat.name}</span>
          </button>
        {/each}
        <button
          class="m-cat-sheet-item is-uncat"
          class:is-current={currentCategoryId === null}
          onclick={() => pick(null)}
        >
          <span class="m-cat-sheet-item-check">✓</span>
          <span>Uncategorised</span>
        </button>
      </div>
      <div class="m-cat-sheet-foot">
        <span></span>
        <button class="m-cat-sheet-cancel" onclick={onClose}>Cancel</button>
      </div>
    </div>
  </div>
{/if}

<style>
  /* Ports .m-cat-sheet* selectors from ui_design/styles.css:3439-3547. */
  .m-cat-sheet {
    position: fixed; inset: 0;
    background: rgba(0, 0, 0, 0.42);
    display: flex; align-items: flex-end; justify-content: stretch;
    z-index: 60;
  }
  :global(html.theme-dark) .m-cat-sheet { background: rgba(0, 0, 0, 0.62); }
  .m-cat-sheet-card {
    flex: 1; background: var(--bg);
    border-radius: 16px 16px 0 0;
    padding: 10px 0 calc(env(safe-area-inset-bottom, 0px) + 18px);
    max-height: 78%;
    display: flex; flex-direction: column;
  }
  .m-cat-sheet-handle {
    width: 38px; height: 4px; background: var(--ink-4);
    border-radius: 2px; margin: 0 auto 10px; opacity: 0.5;
  }
  .m-cat-sheet-head { padding: 8px 22px 14px; border-bottom: 1px solid var(--rule); }
  .m-cat-sheet-eyebrow {
    font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em;
    text-transform: uppercase; color: var(--ink-3);
  }
  .m-cat-sheet-title {
    font-family: var(--serif); font-size: 20px; font-weight: 600;
    letter-spacing: -0.01em; color: var(--ink);
    margin-top: 4px; display: flex; align-items: center; gap: 8px;
  }
  .m-cat-sheet-list { flex: 1; overflow-y: auto; padding: 6px 0; }
  .m-cat-sheet-item {
    display: flex; align-items: center; width: 100%;
    padding: 16px 22px; background: transparent; border: 0;
    border-bottom: 1px solid var(--rule);
    font-family: var(--serif); font-size: 18px; font-weight: 500;
    color: var(--ink); letter-spacing: -0.005em;
    text-align: left; cursor: pointer; gap: 12px;
  }
  .m-cat-sheet-item:last-child { border-bottom: 0; }
  .m-cat-sheet-item.is-current { color: var(--accent); }
  .m-cat-sheet-item.is-uncat { color: var(--ink-3); font-style: italic; font-weight: 400; }
  .m-cat-sheet-item-check {
    width: 14px; color: var(--accent);
    visibility: hidden; text-align: center;
  }
  .m-cat-sheet-item.is-current .m-cat-sheet-item-check { visibility: visible; }
  .m-cat-sheet-foot {
    padding: 14px 22px 4px; border-top: 1px solid var(--rule);
    display: flex; justify-content: space-between; align-items: center;
  }
  .m-cat-sheet-cancel {
    font-family: var(--sans); font-size: 14px; font-weight: 500;
    color: var(--ink-2); background: transparent; border: 0;
    padding: 8px 4px; cursor: pointer;
  }
</style>
```

- [ ] **Step 4: Run tests to verify they pass**

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/CategoryReassignSheet.svelte web/src/components/__tests__/CategoryReassignSheet.test.ts
git commit -m "spa: CategoryReassignSheet mobile bottom sheet"
```

---

### Task 9: `CategoryDeleteDialog.svelte`

**Files:**
- Create: `web/src/components/CategoryDeleteDialog.svelte`, `web/src/components/__tests__/CategoryDeleteDialog.test.ts`

- [ ] **Step 1: Write failing tests**

```ts
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import CategoryDeleteDialog from '../CategoryDeleteDialog.svelte';

vi.mock('../Dialog.svelte', () => ({ default: vi.fn() }));

const feeds = (n: number) => Array.from({ length: n }, (_, i) => ({
  id: i + 1, title: `Feed ${i+1}`, feed_url: `https://feed${i+1}.com`,
  next_poll_at: 0, error_count: 0, created_at: 0, extract: false, extract_selector: '',
  has_cookie: false, has_basic_auth: false, category_id: 1,
}));

describe('CategoryDeleteDialog', () => {
  it('renders the title with the category name', () => {
    render(CategoryDeleteDialog, { categoryName: 'People', affectedFeeds: feeds(2), onCancel: () => {}, onConfirm: () => {} });
    expect(screen.getByText(/Delete "People"\?/)).toBeTruthy();
  });

  it('shows "no feeds affected" copy when feed list is empty', () => {
    render(CategoryDeleteDialog, { categoryName: 'People', affectedFeeds: [], onCancel: () => {}, onConfirm: () => {} });
    expect(screen.getByText(/No feeds are assigned/i)).toBeTruthy();
  });

  it('shows up to 5 feeds + a "+ N more" line when over 5', () => {
    render(CategoryDeleteDialog, { categoryName: 'P', affectedFeeds: feeds(7), onCancel: () => {}, onConfirm: () => {} });
    expect(screen.getByText('Feed 1')).toBeTruthy();
    expect(screen.getByText('Feed 5')).toBeTruthy();
    expect(screen.queryByText('Feed 6')).toBeNull();
    expect(screen.getByText('+ 2 more')).toBeTruthy();
  });

  it('calls onConfirm when "Delete category" is clicked', async () => {
    const onConfirm = vi.fn();
    render(CategoryDeleteDialog, { categoryName: 'P', affectedFeeds: feeds(1), onCancel: () => {}, onConfirm });
    await fireEvent.click(screen.getByText('Delete category'));
    expect(onConfirm).toHaveBeenCalled();
  });

  it('calls onCancel when Cancel is clicked', async () => {
    const onCancel = vi.fn();
    render(CategoryDeleteDialog, { categoryName: 'P', affectedFeeds: feeds(1), onCancel, onConfirm: () => {} });
    await fireEvent.click(screen.getByText('Cancel'));
    expect(onCancel).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Expected: FAIL — component does not exist.

- [ ] **Step 3: Implement**

`web/src/components/CategoryDeleteDialog.svelte`:

```svelte
<script lang="ts">
  import Dialog from './Dialog.svelte';
  import type { Subscription } from '../lib/types';

  type Props = {
    categoryName: string;
    affectedFeeds: Subscription[];
    onCancel: () => void;
    onConfirm: () => void;
  };
  const { categoryName, affectedFeeds, onCancel, onConfirm }: Props = $props();

  const shown = $derived(affectedFeeds.slice(0, 5));
  const extra = $derived(Math.max(0, affectedFeeds.length - 5));
</script>

<Dialog title={`Delete "${categoryName}"?`} onClose={onCancel}>
  {#snippet body()}
    <p class="ts-dialog-p">
      The category is removed.
      {#if affectedFeeds.length > 0}
        The {affectedFeeds.length} {affectedFeeds.length === 1 ? 'feed' : 'feeds'} inside stay subscribed —
        they drop back to <b>Uncategorised</b> and keep appearing in Unread.
      {:else}
        No feeds are assigned to it, so nothing else changes.
      {/if}
    </p>
    {#if affectedFeeds.length > 0}
      <ul class="ts-dialog-list">
        {#each shown as f (f.id)}
          <li class="ts-dialog-list-item">
            <span class="ts-dialog-list-item-name">{f.title}</span>
            <span class="ts-dialog-list-item-arrow">&rarr; Uncategorised</span>
          </li>
        {/each}
        {#if extra > 0}
          <li class="ts-dialog-list-more">+ {extra} more</li>
        {/if}
      </ul>
    {/if}
  {/snippet}
  {#snippet foot()}
    <div class="ts-dialog-foot-l">esc to cancel</div>
    <button class="ts-btn" onclick={onCancel}>Cancel</button>
    <button class="ts-btn is-danger" onclick={onConfirm}>Delete category</button>
  {/snippet}
</Dialog>

<style>
  /* Ports .ts-dialog-list, .ts-dialog-list-item, .ts-dialog-list-item-name,
     .ts-dialog-list-item-arrow, .ts-dialog-list-more, .ts-dialog-p,
     .ts-dialog-foot-l from ui_design/styles.css (~3213-3270). */
  .ts-dialog-p {
    font-family: var(--serif); font-size: 15px; line-height: 1.55;
    color: var(--ink-2); margin: 0;
  }
  .ts-dialog-list {
    list-style: none; margin: 14px 0 0; padding: 12px 0 0;
    border-top: 1px solid var(--rule);
  }
  .ts-dialog-list-item {
    display: flex; align-items: center; gap: 10px; padding: 6px 0;
    font-family: var(--sans); font-size: 13px; color: var(--ink-2);
  }
  .ts-dialog-list-item-name { color: var(--ink); font-weight: 500; }
  .ts-dialog-list-item-arrow {
    margin-left: auto; font-family: var(--mono); font-size: 10.5px; color: var(--ink-3);
  }
  .ts-dialog-list-more {
    font-family: var(--mono); font-size: 10.5px; color: var(--ink-4);
    padding: 6px 0 0; letter-spacing: 0.04em;
  }
  .ts-dialog-foot-l {
    font-family: var(--mono); font-size: 10.5px; color: var(--ink-3);
    letter-spacing: 0.06em; margin-right: auto;
  }
</style>
```

Note: the exact `Dialog.svelte` slot/snippet API comes from M1. If M1 exposes `body` and `foot` as named snippets, the markup above is correct. If M1's API is different (e.g., `<svelte:fragment slot="body">` or default slot + actions prop), the implementer adapts the markup to match — but must keep the same DOM structure and class names.

- [ ] **Step 4: Run tests to verify they pass**

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/CategoryDeleteDialog.svelte web/src/components/__tests__/CategoryDeleteDialog.test.ts
git commit -m "spa: CategoryDeleteDialog confirmation dialog"
```

---

### Task 10: `CategoryMarkReadDialog.svelte`

**Files:**
- Create: `web/src/components/CategoryMarkReadDialog.svelte`, `web/src/components/__tests__/CategoryMarkReadDialog.test.ts`

- [ ] **Step 1: Write failing tests**

```ts
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import CategoryMarkReadDialog from '../CategoryMarkReadDialog.svelte';

vi.mock('../Dialog.svelte', () => ({ default: vi.fn() }));

describe('CategoryMarkReadDialog', () => {
  it('renders the title with unread count', () => {
    render(CategoryMarkReadDialog, { categoryName: 'People', unread: 12, feedCount: 4, onCancel: () => {}, onConfirm: () => {} });
    expect(screen.getByText(/Mark 12 entries as read\?/)).toBeTruthy();
  });

  it('uses singular when unread = 1', () => {
    render(CategoryMarkReadDialog, { categoryName: 'P', unread: 1, feedCount: 2, onCancel: () => {}, onConfirm: () => {} });
    expect(screen.getByText(/Mark 1 entry as read\?/)).toBeTruthy();
  });

  it('renders the stats row with unread, feeds, and category', () => {
    render(CategoryMarkReadDialog, { categoryName: 'People', unread: 12, feedCount: 4, onCancel: () => {}, onConfirm: () => {} });
    expect(screen.getByText('12')).toBeTruthy();
    expect(screen.getByText('unread')).toBeTruthy();
    expect(screen.getByText('4')).toBeTruthy();
    expect(screen.getByText(/across People/i)).toBeTruthy();
  });

  it('calls onConfirm when "Mark all read" is clicked', async () => {
    const onConfirm = vi.fn();
    render(CategoryMarkReadDialog, { categoryName: 'P', unread: 3, feedCount: 1, onCancel: () => {}, onConfirm });
    await fireEvent.click(screen.getByText('Mark all read'));
    expect(onConfirm).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Expected: FAIL — component does not exist.

- [ ] **Step 3: Implement**

`web/src/components/CategoryMarkReadDialog.svelte`:

```svelte
<script lang="ts">
  import Dialog from './Dialog.svelte';

  type Props = {
    categoryName: string;
    unread: number;
    feedCount: number;
    onCancel: () => void;
    onConfirm: () => void;
  };
  const { categoryName, unread, feedCount, onCancel, onConfirm }: Props = $props();
</script>

<Dialog
  title={`Mark ${unread} ${unread === 1 ? 'entry' : 'entries'} as read?`}
  onClose={onCancel}
>
  {#snippet body()}
    <p class="ts-dialog-p">
      Everything currently unread in <b>{categoryName}</b> will be marked as read.
      Anything you've saved stays saved &mdash; only the unread dot disappears.
    </p>
    <div class="ts-dialog-stats">
      <span><b>{unread}</b> unread</span>
      <span class="dot" aria-hidden="true"></span>
      <span><b>{feedCount}</b> {feedCount === 1 ? 'feed' : 'feeds'}</span>
      <span class="dot" aria-hidden="true"></span>
      <span>across {categoryName}</span>
    </div>
  {/snippet}
  {#snippet foot()}
    <div class="ts-dialog-foot-l">esc to cancel</div>
    <button class="ts-btn" onclick={onCancel}>Cancel</button>
    <button class="ts-btn is-primary" onclick={onConfirm}>Mark all read</button>
  {/snippet}
</Dialog>

<style>
  .ts-dialog-p {
    font-family: var(--serif); font-size: 15px; line-height: 1.55;
    color: var(--ink-2); margin: 0;
  }
  .ts-dialog-stats {
    display: flex; align-items: center; gap: 6px;
    margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--rule);
    font-family: var(--mono); font-size: 11px; color: var(--ink-3);
    letter-spacing: 0.02em;
  }
  .ts-dialog-stats b {
    color: var(--ink); font-weight: 500;
    font-feature-settings: "tnum";
  }
  .ts-dialog-stats .dot {
    display: inline-block; width: 3px; height: 3px; border-radius: 50%;
    background: var(--ink-4); margin: 0 4px;
  }
  .ts-dialog-foot-l {
    font-family: var(--mono); font-size: 10.5px; color: var(--ink-3);
    letter-spacing: 0.06em; margin-right: auto;
  }
</style>
```

- [ ] **Step 4: Run tests to verify they pass**

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/CategoryMarkReadDialog.svelte web/src/components/__tests__/CategoryMarkReadDialog.test.ts
git commit -m "spa: CategoryMarkReadDialog confirmation dialog"
```

---

### Task 11: `CategoryCard.svelte` (desktop + mobile)

**Files:**
- Create: `web/src/components/CategoryCard.svelte`, `web/src/components/__tests__/CategoryCard.test.ts`

The card owns the visual chrome and the rename input. It does not own the dialogs (parent handles those) or the popover (passed in as a child via prop or rendered conditionally inside the trailing `.ts-cat-feed-right`).

- [ ] **Step 1: Write failing tests**

```ts
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import CategoryCard from '../CategoryCard.svelte';

const cat = { id: 1, name: 'People', unread: 5, created_at: 0, position: 0 };
const feeds = [
  { id: 10, title: 'jvns', feed_url: 'https://jvns.ca', next_poll_at: 0, error_count: 0,
    created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false,
    category_id: 1 },
];

describe('CategoryCard', () => {
  it('renders the category title and stats', () => {
    render(CategoryCard, { category: cat, feeds, unread: 5, isFirst: true, isLast: false,
      onRename: () => {}, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    expect(screen.getByText('People')).toBeTruthy();
    expect(screen.getByText('5')).toBeTruthy();
  });

  it('clicking the title swaps to a .ts-cat-title-input and focuses it', async () => {
    const { container } = render(CategoryCard, { category: cat, feeds, unread: 5, isFirst: true, isLast: true,
      onRename: () => {}, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    await fireEvent.click(screen.getByText('People'));
    const input = container.querySelector('.ts-cat-title-input') as HTMLInputElement;
    expect(input).toBeTruthy();
    expect(input.value).toBe('People');
  });

  it('Enter on rename input calls onRename with trimmed value and exits rename mode', async () => {
    const onRename = vi.fn();
    const { container } = render(CategoryCard, { category: cat, feeds, unread: 5, isFirst: true, isLast: true,
      onRename, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    await fireEvent.click(screen.getByText('People'));
    const input = container.querySelector('.ts-cat-title-input') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: '  Folks  ' } });
    await fireEvent.keyDown(input, { key: 'Enter' });
    expect(onRename).toHaveBeenCalledWith('Folks');
  });

  it('Esc on rename input cancels without calling onRename', async () => {
    const onRename = vi.fn();
    const { container } = render(CategoryCard, { category: cat, feeds, unread: 5, isFirst: true, isLast: true,
      onRename, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    await fireEvent.click(screen.getByText('People'));
    const input = container.querySelector('.ts-cat-title-input') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'other' } });
    await fireEvent.keyDown(input, { key: 'Escape' });
    expect(onRename).not.toHaveBeenCalled();
    expect(container.querySelector('.ts-cat-title-input')).toBeNull();
  });

  it('blank rename value cancels instead of calling onRename', async () => {
    const onRename = vi.fn();
    const { container } = render(CategoryCard, { category: cat, feeds, unread: 5, isFirst: true, isLast: true,
      onRename, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    await fireEvent.click(screen.getByText('People'));
    const input = container.querySelector('.ts-cat-title-input') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: '   ' } });
    await fireEvent.keyDown(input, { key: 'Enter' });
    expect(onRename).not.toHaveBeenCalled();
  });

  it('disables Reorder up when isFirst', () => {
    render(CategoryCard, { category: cat, feeds, unread: 5, isFirst: true, isLast: false,
      onRename: () => {}, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    const up = screen.getByRole('button', { name: /reorder up/i }) as HTMLButtonElement;
    expect(up.disabled).toBe(true);
  });

  it('disables Reorder down when isLast', () => {
    render(CategoryCard, { category: cat, feeds, unread: 5, isFirst: false, isLast: true,
      onRename: () => {}, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    const down = screen.getByRole('button', { name: /reorder down/i }) as HTMLButtonElement;
    expect(down.disabled).toBe(true);
  });

  it('disables Mark read when unread = 0', () => {
    render(CategoryCard, { category: cat, feeds, unread: 0, isFirst: false, isLast: false,
      onRename: () => {}, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    const m = screen.getByRole('button', { name: /nothing unread/i }) as HTMLButtonElement;
    expect(m.disabled).toBe(true);
  });

  it('renders nothing-but-Mark-read for the Uncategorised pseudo-card', () => {
    const uncat = { id: -1, name: 'Uncategorised', unread: 2, created_at: 0, position: 9999 };
    render(CategoryCard, { category: uncat, feeds, unread: 2, isUncategorised: true,
      isFirst: false, isLast: true,
      onRename: () => {}, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    expect(screen.queryByRole('button', { name: /^rename$/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /^delete$/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /reorder/i })).toBeNull();
    expect(screen.getByRole('button', { name: /mark/i })).toBeTruthy();
  });

  it('keyboard "R" on a focused card opens rename', async () => {
    const { container } = render(CategoryCard, { category: cat, feeds, unread: 5, isFirst: true, isLast: true,
      onRename: () => {}, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} });
    const card = container.querySelector('.ts-cat') as HTMLElement;
    card.focus();
    await fireEvent.keyDown(card, { key: 'r' });
    expect(container.querySelector('.ts-cat-title-input')).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Expected: FAIL — component does not exist.

- [ ] **Step 3: Implement**

`web/src/components/CategoryCard.svelte`:

```svelte
<script lang="ts">
  import type { Category, Subscription } from '../lib/types';
  import CategoryReassignPopover from './CategoryReassignPopover.svelte';
  import CategoryReassignSheet from './CategoryReassignSheet.svelte';

  type Props = {
    category: Category;
    feeds: Subscription[];
    unread: number;
    /** All categories — needed by the reassign popover/sheet. */
    allCategories?: Category[];
    isFirst: boolean;
    isLast: boolean;
    isUncategorised?: boolean;
    /** When true, render the mobile variant (.m-cat-card + bottom sheet for reassign). */
    isMobile?: boolean;
    onRename: (newName: string) => void;
    onDelete: () => void;
    onMarkRead: () => void;
    onReorderUp: () => void;
    onReorderDown: () => void;
    onReassignFeed: (subscriptionId: number, newCategoryId: number | null) => void;
  };
  const {
    category, feeds, unread, allCategories = [],
    isFirst, isLast, isUncategorised = false, isMobile = false,
    onRename, onDelete, onMarkRead, onReorderUp, onReorderDown, onReassignFeed,
  }: Props = $props();

  let renaming = $state(false);
  let renameValue = $state('');
  let renameInput = $state<HTMLInputElement | null>(null);
  let openPickerFeedId = $state<number | null>(null);
  // Trigger DOM refs keyed by feed id — passed to CategoryReassignPopover as
  // its `anchor` prop so M1's Popover primitive can position itself.
  const triggerByFeedId: Record<number, HTMLButtonElement | null> = $state({});

  function beginRename() {
    if (isUncategorised) return;
    renameValue = category.name;
    renaming = true;
  }
  function cancelRename() { renaming = false; }
  function commitRename() {
    const v = renameValue.trim();
    if (!v) { renaming = false; return; }
    onRename(v);
    renaming = false;
  }

  $effect(() => { if (renaming && renameInput) renameInput.focus(); });

  function onCardKey(e: KeyboardEvent) {
    if (renaming) return;
    if ((e.key === 'r' || e.key === 'R') && !isUncategorised) {
      e.preventDefault();
      beginRename();
    }
  }
</script>

<section
  class="ts-cat"
  class:is-renaming={renaming}
  class:is-uncat={isUncategorised}
  class:m-cat-card={isMobile}
  tabindex="0"
  onkeydown={onCardKey}
>
  <header class="ts-cat-head">
    {#if renaming}
      <div class="ts-cat-rename-row">
        <input
          bind:this={renameInput}
          bind:value={renameValue}
          class="ts-cat-title-input"
          aria-label="Rename category"
          onkeydown={(e) => {
            if (e.key === 'Enter') commitRename();
            if (e.key === 'Escape') cancelRename();
          }}
          onblur={() => { if (renameValue.trim()) commitRename(); else cancelRename(); }}
        />
        <span class="ts-cat-rename-hint">↵ save · esc cancel</span>
      </div>
    {:else}
      <h2 class="ts-cat-title" onclick={beginRename}>
        <span>{category.name}</span>
        {#if !isUncategorised}<span class="ts-cat-title-edit">click to rename</span>{/if}
      </h2>
    {/if}
    <div class="ts-cat-stats" class:is-zero={unread === 0}>
      <span class="ll">{unread}</span><span>unread</span>
      <span class="dot" aria-hidden="true"></span>
      <b>{feeds.length}</b><span>{feeds.length === 1 ? 'feed' : 'feeds'}</span>
    </div>
  </header>

  {#if isUncategorised}
    <p class="ts-cat-uncat-note">
      Feeds without a category &mdash; still subscribed, still appear in Unread.
    </p>
  {/if}

  {#if feeds.length === 0}
    <div class="ts-cat-empty">
      No feeds here yet. Move one in from another category, or from <i>Uncategorised</i> below.
    </div>
  {:else}
    <ul class="ts-cat-feeds">
      {#each feeds as f (f.id)}
        <li class="ts-cat-feed">
          <span class="ts-cat-feed-name">{f.title}</span>
          <span class="ts-cat-feed-url">{f.feed_url}</span>
          <span class="ts-cat-feed-right">
            <button
              class="ts-cat-feed-pick"
              class:is-open={openPickerFeedId === f.id}
              aria-expanded={openPickerFeedId === f.id}
              aria-haspopup="menu"
              bind:this={triggerByFeedId[f.id]}
              onclick={() => openPickerFeedId = openPickerFeedId === f.id ? null : f.id}
            >
              <span>{isUncategorised ? 'Assign' : 'Move'}</span>
            </button>
            {#if !isMobile}
              <CategoryReassignPopover
                open={openPickerFeedId === f.id}
                anchor={triggerByFeedId[f.id] ?? null}
                feedName={f.title}
                currentCategoryId={f.category_id}
                categories={allCategories}
                label={isUncategorised ? 'Assign' : 'Move'}
                onPick={(catId) => {
                  openPickerFeedId = null;
                  onReassignFeed(f.id, catId);
                }}
                onClose={() => { if (openPickerFeedId === f.id) openPickerFeedId = null; }}
              />
            {/if}
          </span>
        </li>
      {/each}
    </ul>
  {/if}

  <footer class="ts-cat-actions">
    <button
      class="ts-cat-action"
      onclick={onMarkRead}
      disabled={unread === 0}
      aria-label={unread === 0 ? 'Nothing unread' : `Mark ${unread} read`}
    >
      {unread === 0 ? 'Nothing unread' : `Mark ${unread} read`}
    </button>
    {#if !isUncategorised}
      <button class="ts-cat-action" onclick={beginRename}>Rename</button>
      <button
        class="ts-cat-action"
        onclick={onReorderUp}
        disabled={isFirst}
        aria-label="Reorder up"
      >Reorder up</button>
      <button
        class="ts-cat-action"
        onclick={onReorderDown}
        disabled={isLast}
        aria-label="Reorder down"
      >Reorder down</button>
      <button class="ts-cat-action is-danger" onclick={onDelete}>Delete</button>
    {/if}
  </footer>

  {#if isMobile && openPickerFeedId !== null}
    {@const sheetFeed = feeds.find(x => x.id === openPickerFeedId)}
    {#if sheetFeed}
      <CategoryReassignSheet
        open
        feedName={sheetFeed.title}
        currentCategoryId={sheetFeed.category_id}
        categories={allCategories}
        onSelect={(catId) => onReassignFeed(sheetFeed.id, catId)}
        onClose={() => openPickerFeedId = null}
      />
    {/if}
  {/if}
</section>

<style>
  /* Ports the .ts-cat, .ts-cat-head, .ts-cat-title, .ts-cat-title-edit,
     .ts-cat-title-input, .ts-cat-rename-row, .ts-cat-rename-hint,
     .ts-cat-stats, .ts-cat-empty, .ts-cat-feeds, .ts-cat-feed,
     .ts-cat-feed-name, .ts-cat-feed-url, .ts-cat-feed-right,
     .ts-cat-feed-pick, .ts-cat-actions, .ts-cat-action,
     .ts-cat-uncat-note, .ts-cat.is-uncat .ts-cat-title selectors from
     ui_design/styles.css:2800-3128. Mobile variant overlays from
     ui_design/styles.css:3342-3436 are applied via the .m-cat-card class
     (added to the host when isMobile). Keep selectors named identically so
     :global(html.theme-dark) hooks already established in tokens.css work. */
  .ts-cat {
    padding: 26px 2px 18px;
    border-top: 1px solid var(--rule);
    position: relative;
  }
  .ts-cat:focus { outline: none; }
  .ts-cat:focus-visible { box-shadow: inset 0 0 0 2px var(--accent); }
  .ts-cat.is-renaming { background: var(--accent-soft); }
  /* …remaining CSS ports continue here verbatim from styles.css —
     full block is omitted from the plan for brevity but MUST be ported
     by the implementer. Use the styles.css line ranges in the comment
     above as the source. Add :global(html.theme-dark) variants where the
     upstream rule uses .theme-dark. */
</style>
```

The implementer must port the full CSS block (selector by selector) from `ui_design/styles.css:2800-3128` (desktop) and `3342-3436` (mobile overlay when `isMobile`). The plan does not inline 300+ lines of CSS verbatim because the upstream file is the source of truth and the diff is mechanical translation, but the port is required. The test "renders the category title and stats" is the smoke test that the markup wires up; visual fidelity must be confirmed by the manual smoke checklist (Acceptance criteria).

- [ ] **Step 4: Run tests to verify they pass**

Run: `pnpm --dir web test -- src/components/__tests__/CategoryCard.test.ts`
Expected: PASS for every test case (rename behaviour, reorder disabled states, Uncategorised variant, R-key shortcut, etc.).

Run: `pnpm --dir web run check`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/CategoryCard.svelte web/src/components/__tests__/CategoryCard.test.ts
git commit -m "spa: CategoryCard component (desktop + mobile)"
```

---

### Task 12: `Categories.svelte` view (replaces M1 stub)

**Files:**
- Modify: `web/src/views/Categories.svelte` (stub created by M1; this task replaces its body)
- Test: `web/src/views/__tests__/Categories.test.ts`

- [ ] **Step 1: Write failing tests**

The mock pattern below mirrors `web/src/views/__tests__/Saved.test.ts` and `UnreadMarkAll.test.ts`: hoist `vi.mock` for the whole `lib/store` module, supply per-store `subscribe`/`load`/action stubs as `vi.fn()`, then `vi.mocked(...)` inside each `it` to override default behaviour. The view's `onMount` calls both `categories.load()` and `subscriptions.load()`; without the module-level mock they would hit `api.listCategories()` and crash the test.

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const seedCats = [
  { id: 1, name: 'People', unread: 5, created_at: 0, position: 0 },
  { id: 2, name: 'Systems', unread: 3, created_at: 0, position: 1 },
];
const seedSubs = [
  { id: 10, title: 'jvns', feed_url: 'https://jvns.ca', next_poll_at: 0, error_count: 0,
    created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false,
    category_id: 1 },
  { id: 11, title: 'orphan', feed_url: 'https://x', next_poll_at: 0, error_count: 0,
    created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false,
    category_id: null },
];
const seedEntries = {
  items: [
    { id: 100, subscription_id: 10, title: 'a', url: '', published_at: 0, fetched_at: 0, read: false, saved: false, extract_failed: false },
    { id: 101, subscription_id: 11, title: 'b', url: '', published_at: 0, fetched_at: 0, read: false, saved: false, extract_failed: false },
    { id: 102, subscription_id: 11, title: 'c', url: '', published_at: 0, fetched_at: 0, read: false, saved: false, extract_failed: false },
  ],
  loading: false,
  error: null,
};

let catsValue = seedCats;
let subsValue = seedSubs;
let entriesValue = seedEntries;

vi.mock('../../lib/store', () => ({
  categories: {
    subscribe: (fn: (v: typeof catsValue) => void) => { fn(catsValue); return () => {}; },
    load: vi.fn(),
    create: vi.fn(),
    rename: vi.fn(),
    remove: vi.fn(),
    reorder: vi.fn(),
    reassignSubscription: vi.fn(),
    markRead: vi.fn(),
  },
  subscriptions: {
    subscribe: (fn: (v: typeof subsValue) => void) => { fn(subsValue); return () => {}; },
    load: vi.fn(),
  },
  entries: {
    subscribe: (fn: (v: typeof entriesValue) => void) => { fn(entriesValue); return () => {}; },
    load: vi.fn(),
  },
}));

vi.mock('../../components/AppShell.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/TopTabs.svelte', () => ({ default: vi.fn() }));

const { default: Categories } = await import('../Categories.svelte');
const { categories, subscriptions } = await import('../../lib/store');

beforeEach(() => {
  vi.clearAllMocks();
  catsValue = seedCats;
  subsValue = seedSubs;
  entriesValue = seedEntries;
});

describe('Categories view', () => {
  it('renders a card for each category', () => {
    const { container } = render(Categories);
    expect(container.querySelectorAll('.ts-cat:not(.is-uncat)')).toHaveLength(2);
  });

  it('renders the Uncategorised pseudo-card when uncategorised feeds exist', () => {
    const { container } = render(Categories);
    expect(container.querySelector('.ts-cat.is-uncat')).toBeTruthy();
    expect(screen.getByText('Uncategorised')).toBeTruthy();
  });

  it('does not render the Uncategorised card when all feeds are categorised', () => {
    subsValue = [seedSubs[0]];
    const { container } = render(Categories);
    expect(container.querySelector('.ts-cat.is-uncat')).toBeNull();
  });

  it('renders the empty state when there are zero categories', () => {
    catsValue = [];
    const { container } = render(Categories);
    expect(container.querySelector('.ts-cats-empty')).toBeTruthy();
    expect(screen.getByText(/No categories yet/i)).toBeTruthy();
  });

  it('clicking "+ New category" reveals the inline create input', async () => {
    render(Categories);
    await fireEvent.click(screen.getByRole('button', { name: /new category/i }));
    expect(screen.getByPlaceholderText(/name the category/i)).toBeTruthy();
  });

  it('Enter on the create input calls categories.create with the trimmed value', async () => {
    const createSpy = vi.mocked(categories.create).mockResolvedValue({ id: 99, name: 'New', unread: 0, created_at: 0, position: 2 });
    render(Categories);
    await fireEvent.click(screen.getByRole('button', { name: /new category/i }));
    const input = screen.getByPlaceholderText(/name the category/i) as HTMLInputElement;
    await fireEvent.input(input, { target: { value: '  Letters  ' } });
    await fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(createSpy).toHaveBeenCalledWith('Letters'));
  });

  it('Esc on the create input cancels without calling categories.create', async () => {
    const createSpy = vi.mocked(categories.create);
    render(Categories);
    await fireEvent.click(screen.getByRole('button', { name: /new category/i }));
    const input = screen.getByPlaceholderText(/name the category/i) as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'something' } });
    await fireEvent.keyDown(input, { key: 'Escape' });
    expect(createSpy).not.toHaveBeenCalled();
  });

  it('Reorder up on the second card calls categories.reorder with the swapped order', async () => {
    const reorderSpy = vi.mocked(categories.reorder).mockResolvedValue();
    const { container } = render(Categories);
    const cards = container.querySelectorAll('.ts-cat:not(.is-uncat)');
    // Second card's "Reorder up" button.
    const upBtns = cards[1].querySelectorAll('button[aria-label="Reorder up"]');
    await fireEvent.click(upBtns[0]);
    expect(reorderSpy).toHaveBeenCalledWith([2, 1]);
  });

  it('Reorder down on the first card calls categories.reorder with the swapped order', async () => {
    const reorderSpy = vi.mocked(categories.reorder).mockResolvedValue();
    const { container } = render(Categories);
    const cards = container.querySelectorAll('.ts-cat:not(.is-uncat)');
    const downBtns = cards[0].querySelectorAll('button[aria-label="Reorder down"]');
    await fireEvent.click(downBtns[0]);
    expect(reorderSpy).toHaveBeenCalledWith([2, 1]);
  });

  it('clicking Delete opens the delete confirm dialog', async () => {
    const { container } = render(Categories);
    const deleteBtn = container.querySelector('.ts-cat:not(.is-uncat) .ts-cat-action.is-danger') as HTMLElement;
    await fireEvent.click(deleteBtn);
    expect(screen.getByText(/Delete "People"\?/)).toBeTruthy();
  });

  it('confirming delete calls categories.remove', async () => {
    const removeSpy = vi.mocked(categories.remove).mockResolvedValue();
    const { container } = render(Categories);
    await fireEvent.click(container.querySelector('.ts-cat:not(.is-uncat) .ts-cat-action.is-danger') as HTMLElement);
    await fireEvent.click(screen.getByText('Delete category'));
    expect(removeSpy).toHaveBeenCalledWith(1);
  });

  it('confirming mark-read calls categories.markRead', async () => {
    const markSpy = vi.mocked(categories.markRead).mockResolvedValue();
    const { container } = render(Categories);
    const markBtn = container.querySelector('.ts-cat:not(.is-uncat) .ts-cat-action[aria-label*="Mark"]') as HTMLElement;
    await fireEvent.click(markBtn);
    await fireEvent.click(screen.getByText('Mark all read'));
    expect(markSpy).toHaveBeenCalledWith(1);
  });

  it('Uncategorised mark-read calls categories.markRead(null)', async () => {
    const markSpy = vi.mocked(categories.markRead).mockResolvedValue();
    const { container } = render(Categories);
    const uncatCard = container.querySelector('.ts-cat.is-uncat') as HTMLElement;
    const markBtn = uncatCard.querySelector('.ts-cat-action[aria-label*="Mark"]') as HTMLElement;
    await fireEvent.click(markBtn);
    await fireEvent.click(screen.getByText('Mark all read'));
    expect(markSpy).toHaveBeenCalledWith(null);
  });

  it('renaming a category calls categories.rename', async () => {
    const renameSpy = vi.mocked(categories.rename).mockResolvedValue();
    const { container } = render(Categories);
    await fireEvent.click(screen.getAllByText('People')[0]);
    const input = container.querySelector('.ts-cat-title-input') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'Folks' } });
    await fireEvent.keyDown(input, { key: 'Enter' });
    expect(renameSpy).toHaveBeenCalledWith(1, 'Folks');
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `pnpm --dir web test -- src/views/__tests__/Categories.test.ts`
Expected: FAIL — the M1 stub view does not implement any of this behaviour.

- [ ] **Step 3: Implement the view**

Replace the body of `web/src/views/Categories.svelte`:

**M1 precondition (`isMobile`).** Per umbrella spec §2.1, `AppShell.svelte` already exposes the media-query-driven `isMobile` state. The umbrella does not name the exact wire format yet — at the time this plan was written, M1's plan was still in flight. This view must consume M1's `isMobile` exactly as M1 ships it. Three likely shapes:

1. A `Readable<boolean>` exported from `web/src/lib/preferences.svelte.ts` (likely; matches other prefs).
2. A Svelte context key set by `AppShell.svelte` and read via `getContext('isMobile')`.
3. A `$state` rune lifted to a module-level export.

Whichever M1 lands, the implementer wires this view to it — no parallel `matchMedia` call. If, for any reason, M1 has not yet shipped a wire and this view must merge first, document the deviation in the PR description and keep the local `matchMedia` block below as a temporary fallback only.

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { categories, subscriptions, entries } from '../lib/store';
  import type { Category, Subscription } from '../lib/types';
  import CategoryCard from '../components/CategoryCard.svelte';
  import CategoryDeleteDialog from '../components/CategoryDeleteDialog.svelte';
  import CategoryMarkReadDialog from '../components/CategoryMarkReadDialog.svelte';
  // Replace the next import once M1 lands the canonical isMobile wire.
  // Example for shape #1 (a Readable<boolean>):
  //   import { isMobile } from '../lib/preferences.svelte.ts';
  // Example for shape #2 (a context):
  //   import { getContext } from 'svelte';
  //   const isMobile = getContext<Readable<boolean>>('isMobile');
  // Until M1 lands, use the local fallback below.

  let isMobile = $state(false);
  $effect(() => {
    // FALLBACK — remove once M1's isMobile wire is consumed.
    isMobile = typeof window !== 'undefined' && window.matchMedia?.('(max-width: 720px)').matches;
  });

  let creating = $state(false);
  let newName = $state('');
  let newInput = $state<HTMLInputElement | null>(null);
  $effect(() => { if (creating && newInput) newInput.focus(); });

  let confirmDelete = $state<{ category: Category } | null>(null);
  let confirmMarkRead = $state<{ category: Category | { id: null; name: 'Uncategorised' } } | null>(null);

  onMount(() => {
    void categories.load();
    void subscriptions.load();
    void entries.load(false); // load all entries (not just unread) — needed for the Uncategorised unread count
  });

  const catList = $derived($categories.slice().sort((a, b) => a.position - b.position));
  const uncatFeeds = $derived($subscriptions.filter(s => s.category_id == null));
  const uncatUnread = $derived(() => {
    const uncatIds = new Set(uncatFeeds.map(s => s.id));
    return $entries.items.filter(e => !e.read && uncatIds.has(e.subscription_id)).length;
  });
  const feedsByCategory = $derived.by(() => {
    const m = new Map<number, Subscription[]>();
    for (const s of $subscriptions) {
      if (s.category_id != null) {
        const arr = m.get(s.category_id) ?? [];
        arr.push(s);
        m.set(s.category_id, arr);
      }
    }
    return m;
  });

  function feedsFor(catId: number): Subscription[] { return feedsByCategory.get(catId) ?? []; }

  async function commitCreate() {
    const v = newName.trim();
    if (!v) { creating = false; newName = ''; return; }
    await categories.create(v);
    newName = '';
    creating = false;
  }
  function cancelCreate() { creating = false; newName = ''; }

  async function reorderUp(catId: number) {
    const ids = catList.map(c => c.id);
    const idx = ids.indexOf(catId);
    if (idx <= 0) return;
    [ids[idx - 1], ids[idx]] = [ids[idx], ids[idx - 1]];
    await categories.reorder(ids);
  }
  async function reorderDown(catId: number) {
    const ids = catList.map(c => c.id);
    const idx = ids.indexOf(catId);
    if (idx === -1 || idx >= ids.length - 1) return;
    [ids[idx], ids[idx + 1]] = [ids[idx + 1], ids[idx]];
    await categories.reorder(ids);
  }

  async function performMarkRead() {
    if (!confirmMarkRead) return;
    const target = confirmMarkRead.category;
    confirmMarkRead = null;
    await categories.markRead(target.id);
  }

  async function performDelete() {
    if (!confirmDelete) return;
    const id = confirmDelete.category.id;
    confirmDelete = null;
    await categories.remove(id);
  }
</script>

<main class="ts-main">
  <div class="ts-cats">
    <header class="ts-set-head">
      <div class="ts-set-eyebrow">
        Organisation · {catList.length} {catList.length === 1 ? 'category' : 'categories'}
      </div>
      <h1 class="ts-set-title">Categories</h1>
    </header>

    <div class="ts-cats-toolbar">
      <span class="ts-cats-toolbar-l">
        <b>{catList.length}</b><span>{catList.length === 1 ? 'category' : 'categories'}</span>
        <span class="dot" aria-hidden="true"></span>
        <b>{$subscriptions.length}</b><span>{$subscriptions.length === 1 ? 'feed' : 'feeds'}</span>
        {#if uncatFeeds.length > 0}
          <span class="dot" aria-hidden="true"></span>
          <b>{uncatFeeds.length}</b><span>uncategorised</span>
        {/if}
      </span>
      {#if !creating && catList.length > 0}
        <button class="ts-cats-new" onclick={() => creating = true}>New category</button>
      {/if}
    </div>

    {#if creating}
      <div class="ts-cats-newrow">
        <input
          bind:this={newInput}
          bind:value={newName}
          class="ts-cats-newinput"
          placeholder="Name the category…"
          onkeydown={(e) => {
            if (e.key === 'Enter') void commitCreate();
            if (e.key === 'Escape') cancelCreate();
          }}
          onblur={() => { if (!newName.trim()) cancelCreate(); }}
        />
        <div class="ts-cats-newrow-hint">
          <span class="k">↵</span> create · <span class="k">esc</span> cancel
        </div>
      </div>
    {/if}

    {#if catList.length === 0 && !creating}
      <div class="ts-cats-empty">
        <div class="ts-cats-empty-mark" aria-hidden="true">
          <span class="ts-cats-empty-dot"></span>
        </div>
        <div class="ts-cats-empty-title">No categories yet.</div>
        <p class="ts-cats-empty-sub">
          Categories group your {$subscriptions.length} {$subscriptions.length === 1 ? 'feed' : 'feeds'} into named sections.
          Add one to start sorting; feeds you don't assign keep working as normal.
        </p>
        <div class="ts-cats-empty-examples">
          <span class="ts-cats-empty-example">People</span>
          <span class="ts-cats-empty-example">Systems &amp; PL</span>
          <span class="ts-cats-empty-example">Letters</span>
        </div>
        <button class="ts-cats-empty-cta" onclick={() => creating = true}>
          Create your first category
        </button>
      </div>
    {:else}
      <div class="ts-cats-list">
        {#each catList as cat, i (cat.id)}
          <CategoryCard
            category={cat}
            feeds={feedsFor(cat.id)}
            unread={cat.unread}
            allCategories={catList}
            isMobile={isMobile}
            isFirst={i === 0}
            isLast={i === catList.length - 1}
            onRename={(name) => categories.rename(cat.id, name)}
            onDelete={() => confirmDelete = { category: cat }}
            onMarkRead={() => confirmMarkRead = { category: cat }}
            onReorderUp={() => reorderUp(cat.id)}
            onReorderDown={() => reorderDown(cat.id)}
            onReassignFeed={(subId, newCatId) => categories.reassignSubscription(subId, newCatId)}
          />
        {/each}

        {#if uncatFeeds.length > 0}
          <CategoryCard
            category={{ id: -1, name: 'Uncategorised', unread: 0, created_at: 0, position: Number.MAX_SAFE_INTEGER }}
            feeds={uncatFeeds}
            unread={uncatUnread()}
            allCategories={catList}
            isUncategorised
            isMobile={isMobile}
            isFirst={false}
            isLast
            onRename={() => {}}
            onDelete={() => {}}
            onMarkRead={() => confirmMarkRead = { category: { id: null, name: 'Uncategorised' } }}
            onReorderUp={() => {}}
            onReorderDown={() => {}}
            onReassignFeed={(subId, newCatId) => categories.reassignSubscription(subId, newCatId)}
          />
        {/if}
      </div>
    {/if}
  </div>

  {#if confirmDelete}
    <CategoryDeleteDialog
      categoryName={confirmDelete.category.name}
      affectedFeeds={feedsFor(confirmDelete.category.id)}
      onCancel={() => confirmDelete = null}
      onConfirm={performDelete}
    />
  {/if}

  {#if confirmMarkRead}
    <CategoryMarkReadDialog
      categoryName={confirmMarkRead.category.name}
      unread={confirmMarkRead.category.id == null ? uncatUnread() : (confirmMarkRead.category as Category).unread}
      feedCount={confirmMarkRead.category.id == null ? uncatFeeds.length : feedsFor((confirmMarkRead.category as Category).id).length}
      onCancel={() => confirmMarkRead = null}
      onConfirm={performMarkRead}
    />
  {/if}
</main>

<style>
  /* Ports .ts-main, .ts-cats, .ts-set-head, .ts-set-eyebrow, .ts-set-title,
     .ts-cats-toolbar, .ts-cats-toolbar-l, .ts-cats-new, .ts-cats-newrow,
     .ts-cats-newinput, .ts-cats-newrow-hint, .ts-cats-list, .ts-cats-empty
     and children from ui_design/styles.css:2680-3210.

     The implementer must port the full CSS verbatim — only token names from
     web/src/styles/tokens.css and the existing :global(html.theme-{light|dark|sepia})
     overrides should appear. Mobile-only rules (.m-cats-*) port from
     ui_design/styles.css:3274-3340 and apply when isMobile is true via class
     toggling. */
</style>
```

The implementer ports the CSS verbatim from the line ranges in the style block's comment. The plan does not inline it because it is mechanical and the upstream file is authoritative.

For unread counts on the Uncategorised pseudo-card, the simplest correct path is to thread per-feed unread counts through the `subscriptions` store. If that's not already wired (M1 may have done it), the implementer either:
1. Adds a `unread_count` field to the subscription DTO via existing `internal/api/subscriptions.go` and `internal/db/subscriptions.go` (small additive change), OR
2. Computes uncategorised unread on the client from `entries` store + the subset of subscriptions that are uncategorised.

Option 2 is preferred because it requires no backend change. The plan currently leaves that count at "compute on demand from the entries store" — the implementer wires it in. Open and resolve this decision in the implementation PR; do not block on it.

- [ ] **Step 4: Run tests to verify they pass**

Run: `pnpm --dir web test -- src/views/__tests__/Categories.test.ts`
Expected: PASS for every test case.

Run: `pnpm --dir web test`
Expected: PASS for the full suite.

Run: `pnpm --dir web run check`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Categories.svelte web/src/views/__tests__/Categories.test.ts
git commit -m "spa: Categories management view (replaces M1 stub)"
```

---

### Task 13: Manual smoke + acceptance review

**No new files. No commits unless a regression is found.**

- [ ] Run `make dev` and open `http://localhost:5173/categories` in Chrome.
- [ ] Confirm desktop empty state renders correctly when the user has no categories: T-junction mark, "No categories yet" title, three example chips, dark CTA. Click the CTA, type a name, hit Enter, watch the card appear.
- [ ] Create three more categories. Click "Reorder up" on the second; the order swaps. Click "Reorder down" on the first; back to original. Disabled states appear on the boundary cards.
- [ ] Click a category title; the inline rename input appears with focus and accent bottom border. Type, hit Enter; the new name persists after a hard refresh.
- [ ] Add a feed to a category via the reassign popover: click "Move" on an Uncategorised feed, click a target category, confirm the popover closes and the feed re-renders under the target.
- [ ] Mark a category read: click "Mark N read", confirm the dialog, watch the unread count drop to 0. The action button now reads "Nothing unread" and is disabled.
- [ ] Delete a category with feeds inside: click Delete, confirm the dialog (which lists up to 5 affected feeds + "+ N more"), confirm; the category disappears and the feeds drop to Uncategorised.
- [ ] Uncategorised pseudo-card: confirm it sits last, has italic ink-3 title, and only offers Mark all read. Rename/Delete/Reorder are absent. Reassign popovers show "Assign" instead of "Move".
- [ ] Resize the viewport below 720px (or open DevTools mobile emulator). Confirm the page now uses `.m-cat-card` layout. Tap "Move" on a feed; the bottom sheet (`.m-cat-sheet`) appears with grab handle + eyebrow + items. Tap an item; the sheet closes and the feed moves.
- [ ] Cycle through themes (light/dark/sepia via AccountMenu): every selector should follow tokens; no white-on-white or black-on-black artifacts.
- [ ] Keyboard: tab to a category card, press `R`; rename input opens. Press `Esc`; rename cancels.
- [ ] Network failure path: open DevTools, throttle to offline, click Reorder. The optimistic update should appear then roll back; the categories list returns to its pre-click order. Bring back online; the next action succeeds.

If any of the above fails, stop and file a follow-up — don't paper over with a rule change.

---

## Acceptance criteria

A reviewer is satisfied when **all** of the following hold:

1. **Backend**
   - `internal/db/migrations/0011_category_position.sql` exists; `make test` applies it on an empty DB and on a DB that has rows under the 0008 schema.
   - `db.Category` has a `Position int64` field (added alongside existing fields); `db.ListCategories` orders by `position ASC, name COLLATE NOCASE`; `db.InsertCategory` assigns the next per-user position; `db.ReorderCategories` exists, is atomic, scoped per user, and returns `db.ErrCategoryReorderMismatch` on bad input.
   - `db.MarkSubscriptionRead` exists, is scoped per user, and is a silent no-op for cross-user IDs.
   - `categoryDTO` carries `position`; `POST /api/v1/categories/reorder` returns `204` on success, `400 category_reorder_mismatch` on bad input, `401` without a session.
   - `POST /api/v1/subscriptions/:id/mark-read` returns `204` on success, `404 not_found` when the subscription belongs to another user, `400 bad_request` on a non-numeric id, `401` without a session.
   - Outer mux at `internal/api/api.go` and test mux at `internal/api/testing.go` both list the two new routes (`POST /categories/reorder`, `POST /subscriptions/{id}/mark-read`).
   - Backend test count grows by at least: migration test (1), DB tests (4 ReorderCategories + 1 InsertCategory + 1 ListCategories + 2 MarkSubscriptionRead = 8), API tests (4 reorder + 3 mark-sub-read = 7) — **16 total new tests**.

2. **Frontend chrome**
   - `/categories` renders inside the `.ts-shell` (or `.m-cats-root` on mobile) provided by M1.
   - Page chrome ports `.ts-cats`, `.ts-cats-toolbar`, `.ts-cats-newrow`, `.ts-cats-empty`, `.ts-cats-list` from `ui_design/styles.css` (lines 2680-3210). Selector names preserved or scoped equivalently.
   - The Uncategorised pseudo-card matches `ui_design/Tap Brand and UI Spec.md` §6.4: italic `--ink-3` title, no rename/reorder/delete, action row offers only Mark all read.
   - Empty state renders the T-junction mark, three example chips, and the dark CTA exactly per `ui_design/tap-categories-page.jsx:239-258`.

3. **Frontend behaviour**
   - Click-title-to-rename works; Enter commits, Esc cancels, blank input cancels.
   - "+ New category" reveals `.ts-cats-newinput`; Enter creates, Esc cancels, blur with blank cancels.
   - Reorder up/down swaps adjacent positions and POSTs `/categories/reorder`. Top card disables Reorder up; bottom card disables Reorder down.
   - Delete opens `CategoryDeleteDialog` listing up to 5 affected feeds + "+ N more"; Confirm calls `categories.remove`.
   - Mark-all-read opens `CategoryMarkReadDialog`; Confirm calls `categories.markRead(id)` (or `null` for Uncategorised).
   - Reassign-feed popover opens on Move/Assign click, closes on outside-click or item selection, calls `categories.reassignSubscription`. Mobile uses the bottom sheet variant.
   - All three themes (light/dark/sepia) render without contrast regressions.

4. **Tests**
   - `make test` exits 0.
   - `pnpm --dir web test` exits 0; new files contribute at least 36 new test cases (35 from review pass 1 + 1 reorder-rollback test).
   - `pnpm --dir web run check` exits 0 (no TS/svelte-check errors).

5. **No regressions**
   - Other views still load.
   - The categories store does not break the M1 chrome (account menu category-link counts, etc., if any).

---

## Verification commands

```bash
# Backend
go test ./internal/db -race
go test ./internal/api -race
make test

# Frontend
pnpm --dir web test
pnpm --dir web test -- src/views/__tests__/Categories.test.ts
pnpm --dir web test -- 'src/components/__tests__/Category*.test.ts'
pnpm --dir web run check

# Manual
make dev   # then open http://localhost:5173/categories
```

Expected: every command exits 0. Manual smoke per Task 13 passes every checkbox.

---

## Risks

1. **Backend migration on populated DBs.** Migration 0011 adds a `NOT NULL DEFAULT 0` column, which SQLite handles cleanly via `ALTER TABLE ADD COLUMN`. Existing rows get `position = 0`; the list query then ties them by name, so order is stable across the upgrade. Mitigation: the migration test in Task 1 explicitly seeds two pre-0011 categories before applying 0011.

2. **Uncategorised pseudo-card unread count.** Decision (was open in review pass 1): compute on the client from the `$entries` store via `uncatUnread()` (Task 12 step 3). No DTO extension. The view eagerly loads all entries (`entries.load(false)`) in `onMount` so the count is correct on first paint. The action button is disabled when `uncatUnread() === 0`, which is the same disabled state the test "disables Mark read when unread = 0" exercises.

3. **M1 primitive API drift.** `Dialog.svelte` and `Popover.svelte` are M1's responsibility. This plan assumes a snippet-based API (`{#snippet body()}` / `{#snippet foot()}`). If M1 lands a different shape (default slot, action prop, etc.), the implementer adapts `CategoryDeleteDialog.svelte` and `CategoryMarkReadDialog.svelte` accordingly — but only those wrappers; the test cases stay valid.

4. **The current sidebar's inline category UI is removed by M1 before this milestone lands.** Between M1 merge and M4 merge there is no SPA-visible way to manage categories. This is accepted by the umbrella spec §3.3. If M4 is delayed, users can still use the API directly or fall back to OPML re-import; the operator has the `tap admin` CLI escape hatches.

5. **Mark-all-read on Uncategorised dispatches per-subscription, not per-entry.** Decision (was open in review pass 1): ship the dedicated `POST /api/v1/subscriptions/:id/mark-read` endpoint via Tasks 5a + 5b. The SPA fans out one POST per uncategorised feed. For typical user fleets (single-digit uncategorised feeds), this is faster than entry-level PATCH and easier to reason about. If a user has hundreds of uncategorised feeds and feels the latency, a future milestone can add a true bulk endpoint.

6. **Optimistic reorder may visibly flicker on slow connections.** Mitigation: the store rolls back the in-memory order if the API call rejects (Task 6 step 5's catch block + the rollback test added in review pass 1). The page is small enough that no spinner is needed.

7. **M1 `isMobile` wire format not yet finalised.** M1's plan is still in flight at the time this plan was written. Task 12 step 3 documents three likely shapes M1 might land (`Readable<boolean>` from preferences, Svelte context, or module-level `$state`) and provides a local `matchMedia` fallback to keep this view shippable in isolation. The implementer replaces the fallback with the canonical wire when M1 lands; the change is purely the `import` line and the `$effect` removal.

---

## Self-review

Re-read against the umbrella spec §5 row M4 and `ui_design/Tap Brand and UI Spec.md` §6.4, §6.8:

- **Inline rename** — Task 11. ✓
- **Reassign-feed popover** — Tasks 7 (desktop), 8 (mobile sheet). ✓
- **Mark-all-read (per category)** — Task 10 (dialog), Task 12 (wiring). ✓
- **Mark-all-read (Uncategorised)** — Tasks 5a (`db.MarkSubscriptionRead`), 5b (`POST /subscriptions/:id/mark-read`), 6 (store fans out). ✓
- **Uncategorised pseudo-cat (italic, no rename/reorder/delete, only Mark all read)** — Task 11 (`isUncategorised` prop) and Task 12 (rendering condition + null target). ✓
- **Empty state with example chips + dark CTA** — Task 12 markup. ✓
- **Reorder up/down (backend addition)** — Tasks 1-5. ✓
- **Outer mux wiring for new endpoints** — Task 5 (categories/reorder), Task 5b (subscriptions/mark-read). ✓
- **Optimistic-reorder rollback** — Task 6 (store impl + test added review pass 1). ✓
- **Mobile sheet variants** — Task 8 + Task 11's `isMobile` branch + Task 12's `isMobile` consumption. ✓
- **Confirm dialogs for destructive actions** — Tasks 9, 10. ✓

No placeholder text; every step has runnable commands or actual code. Types are consistent across tasks: `Category.position: number`, `ReorderCategories(ctx, d, userID, []int64)`, `categories.reorder(orderedIds: number[])`, `ErrCategoryReorderMismatch`, `ErrCodeReorderMismatch = "category_reorder_mismatch"`.

Mobile-only behaviour is exercised by Task 8 (sheet) and is reachable from Task 11's `isMobile` branch and Task 12's media-query detection.

Spec coverage: every bullet from the team-lead's "Scope" list is mapped to at least one task.
