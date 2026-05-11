# M-Redesign-5 (Feeds management) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `/feeds` management page from the new design (Brand spec §6.5): a single centred page that lists every subscription with filter chips, sort, search, bulk actions, per-row health, an add-feed dialog with discovery, an edit dialog (absorbing the deleted `FeedSettingsModal`'s job), and OPML import/export — all on top of existing endpoints plus one small `PATCH next_poll_at` extension so per-feed Refresh has real teeth.

**Architecture:** Frontend-heavy milestone. One new view (`views/Feeds.svelte`) drives a tree of new components under `components/feeds/`. Every dialog rides on the M1 `Dialog` primitive; every chip on M1 `Chip`; every button on M1 `Button`; the form on M1 `Field` and `EmptyState`. State stays in the existing Svelte 5 stores. Bulk actions sequence as `Promise.allSettled` over per-feed endpoints (concept: N×1) with partial-failure surfacing. One narrow backend change: `PATCH /api/v1/subscriptions/:id` accepts `{"refresh_now": true}` to set `next_poll_at = 0` and call `Scheduler.Poke()` so the per-row Refresh icon and "Refresh all" actually re-poll.

**Tech Stack:**
- Frontend: Svelte 5 + TypeScript + Vite (Svelte runes); vitest + `@testing-library/svelte`.
- Existing primitives from M1 Foundations: `Button.svelte`, `Field.svelte`, `Chip.svelte`, `Popover.svelte`, `Dialog.svelte`, `EmptyState.svelte`, `FeedAvatar.svelte` (existing, restyled).
- Existing API: `internal/api/subscriptions.go`, `internal/api/discover.go`, `internal/api/opml.go`, `internal/api/categories.go`; `internal/poll.Scheduler.Poke()`.
- One backend touch: `internal/api/subscriptions.go` PATCH handler + `internal/db.UpdateSubscriptionPatch` to support `refresh_now`.

**Spec:** [`docs/specs/2026-05-11-ui-redesign.md`](../../specs/2026-05-11-ui-redesign.md) — umbrella; row M5 in §5 is the scope, §6.5 of `ui_design/Tap Brand and UI Spec.md` is the design source, `ui_design/styles.css` lines 3554–4393 are the visual source, `ui_design/tap-feeds-page.jsx` is the structural reference.

---

## Dependencies on other milestones

This milestone **blocks on M-Redesign-1 (Foundations)**. Specifically it consumes:

- `web/src/components/AppShell.svelte`, `web/src/components/TopTabs.svelte`, `web/src/components/StatusFoot.svelte`.
- `web/src/components/Button.svelte` (default / primary / quiet / danger; size; icon slot).
- `web/src/components/Field.svelte` (label + input; mono variant via prop or class).
- `web/src/components/Chip.svelte` (pill chip with count; `is-active`; `is-warn` variants).
- `web/src/components/Popover.svelte` (scrim + positioned content; click-outside dismiss; Esc closes).
- `web/src/components/Dialog.svelte` (head + body + foot; `is-wide` variant; focus trap; Esc closes; warn callout).
- `web/src/components/EmptyState.svelte` (dot + serif title + sans sub + optional CTA slot).
- `web/src/components/FeedAvatar.svelte` (existing; uses `colorForFeed(feed_url)`; 18 px size supported via the existing `size` prop).
- Stub `views/Feeds.svelte` from M1 (renders "Coming soon — M5" EmptyState); this milestone **replaces** that stub file wholesale.
- Router has `/feeds` registered as `feeds` and the desktop top tab + mobile bottom tab are present.

If any primitive interface diverges from what this plan assumes, fix the call sites in this plan's `Feeds.svelte` and child components — do **not** monkey-patch the primitives.

**Coordinates with M-Redesign-4 (Categories management):** M4 may add a `position INTEGER` column to `categories`. This plan does **not** care about category ordering — the only thing the Feeds page needs from categories is `{ id, name, unread }`. Render categories in the order returned by `api.listCategories()`. M4 controls that order.

---

## Skills and tools for implementers

Invoke before each kind of work. Don't skip.

| Phase / task type | Skill(s) |
|---|---|
| All Svelte tasks | `svelte-runes` (use `$state`/`$derived`/`$effect`/`$props` — no Svelte 4 stores in new code), `svelte-styling` (scoped `<style>` blocks; CSS custom props for cross-component theming), `svelte-template-directives` (`{@attach}` over `use:` for dialog focus trap, drag-and-drop on OPML drop zone, click-outside on popovers) |
| All tests | `tdd` (red/green/refactor on every interactive bit), `superpowers:test-driven-development`, `superpowers:verification-before-completion` |
| Before claiming task done | `superpowers:verification-before-completion` (run `pnpm --dir web test`, run `pnpm --dir web run check`, run the affected single test in `-v` mode) |
| Backend touch (Phase 1) | `golang-error-handling`, `golang-testing`, `golang-stretchr-testify`, `golang-database` |
| Frontend smoke (Phase 12) | Playwright MCP (`mcp__plugin_playwright_playwright__browser_navigate`, `browser_snapshot`, `browser_click`, `browser_fill_form`, `browser_file_upload`) to exercise each dialog and bulk path |
| Library docs | `mcp__plugin_context7_context7__query-docs` for `FileReader` / `DataTransfer` / drag-and-drop / `URL.createObjectURL` / `<input type="file">` when wiring OPML import |
| Working on `data-testid` selectors / role queries | `@testing-library/svelte` Context7 docs |

---

## TDD posture

Per `CLAUDE.md` and the umbrella spec §6, every behaviour-bearing change is red/green/refactor. Specifically TDD-required in this milestone:

- **Filter logic**: `All` / `Errors (n)` / `Unread (n)` / `Stale (n)` chip counts; the predicate each chip applies.
- **Sort logic**: each of `Recently active`, `Name`, `Added`, `Most unread`; tie-breaks are by `id` ascending so the order is deterministic.
- **Search**: case-insensitive substring match against `title` and `feed_url`; debounced 150 ms in the bound state (tested via fake timers).
- **Bulk selection**: select-one / select-many / select-all (header checkbox) / clear; bulk bar appears when ≥ 1 row checked.
- **Bulk actions with partial failure**: a 3-feed delete where the second feed fails should delete the first and third and surface a single mono status string at the foot ("Removed 2 of 3 · 1 failed: <message>"). Same shape for bulk refresh and bulk set-category.
- **Per-row actions**: refresh-now (busy state), expand health panel, change-category popover, open more-menu, delete.
- **Add-feed dialog**: 0/1/N candidate paths from discovery; submitting the chosen candidate writes via `subscriptions.add`; "no feeds found" branch; error branch.
- **Edit-feed dialog**: load PATCH payload omits unchanged fields; cookie/basic-auth fields only sent when populated (mirrors the deleted `FeedSettingsModal` semantics); category change triggers the M9 `category_id` PATCH.
- **OPML import dialog**: file picker → drop zone → POST → result summary. Round-trip: imported / skipped / errors counts surface in the dialog foot, dialog closes only on explicit dismissal, store refresh on close.
- **OPML export**: clicking download triggers `api.exportOPML()`, the returned `Blob` becomes an object URL on a transient `<a download>`, URL is revoked after click; export quiet button reflects busy state.
- **Empty states**: (a) zero feeds in account → "No feeds yet" + Add CTA, (b) filter/search returns zero matches → "No feeds match" + reset CTA.
- **Mobile sheet variants** (smoke; not full TDD): add-feed sheet and edit sheet render through the same component tree on `is-mobile`.

**Exempt** (per the umbrella spec §6 "Exempt"):

- Pure CSS port tasks where the only behaviour is hover/transition (Phase 5.1 row chrome, Phase 5.2 toolbar chrome, Phase 5.3 disc list CSS). Style-port tasks still get a Playwright snapshot in the final verification step.
- `FeedAvatar` is reused as-is; no new tests needed.

---

## File structure

### Files created

```
web/src/views/Feeds.svelte                          REPLACES the M1 stub. Owns layout + state for the management page.
web/src/components/feeds/FeedsToolbar.svelte        Row 1 (search + Add feed primary) + Row 2 (filter chips + sort + utility buttons).
web/src/components/feeds/FeedsBulkBar.svelte        Solid --ink strip when selected.size > 0. Refresh / Reassign / Delete.
web/src/components/feeds/FeedsListRow.svelte        Per-feed row (.ts-feed-row). Composes FeedAvatar + chips + per-row actions; emits row events to parent.
web/src/components/feeds/FeedsHealthPanel.svelte    Expanded panel inside a row when row error + isExpanded. Owns the 3-cell grid.
web/src/components/feeds/AddFeedDialog.svelte       Wide dialog. Absorbs current AddFeedForm behaviour. Uses Dialog + discovery list + category picker.
web/src/components/feeds/EditFeedDialog.svelte      Wide dialog. Replaces FeedSettingsModal. Category buttons + extract toggle + selector + cookie + basic auth.
web/src/components/feeds/DeleteFeedsDialog.svelte   Single + bulk confirm. Avatars + URL list (first 5, "…and N more").
web/src/components/feeds/ImportOpmlDialog.svelte    Drop zone or file picker → POST → result summary. is-wide.
web/src/components/feeds/ExportOpmlDialog.svelte    Optional. The M5 spec calls for a button; using the dialog (stats + preview + Download) matches §6.5 better. is-wide.
web/src/components/feeds/ReassignCategoryPopover.svelte
                                                    Popover used by both per-row "change category" and bulk reassign. Reused selector pattern from the design.
web/src/lib/feedsFilter.ts                          Pure functions: filterFeeds(), sortFeeds(), bulk* helpers. Easy to unit-test without DOM.

web/src/views/__tests__/Feeds.test.ts               View-level integration tests (filter+sort+search; bulk selection wiring; mobile branch).
web/src/components/feeds/__tests__/FeedsToolbar.test.ts
web/src/components/feeds/__tests__/FeedsBulkBar.test.ts
web/src/components/feeds/__tests__/FeedsListRow.test.ts
web/src/components/feeds/__tests__/AddFeedDialog.test.ts
web/src/components/feeds/__tests__/EditFeedDialog.test.ts
web/src/components/feeds/__tests__/DeleteFeedsDialog.test.ts
web/src/components/feeds/__tests__/ImportOpmlDialog.test.ts
web/src/components/feeds/__tests__/ExportOpmlDialog.test.ts
web/src/components/feeds/__tests__/ReassignCategoryPopover.test.ts
web/src/lib/__tests__/feedsFilter.test.ts
```

### Files modified

```
web/src/lib/api.ts                                  + refreshSubscription(id) helper that PATCHes {refresh_now:true}.
                                                    + bulkUpdateSubscription helper (thin wrapper for ergonomics; not strictly required).
web/src/lib/store.ts                                + subscriptions.refresh(id), subscriptions.remove(id), subscriptions.setCategory(id, catId).
                                                      Keeps the optimistic-write pattern already in entriesStore.toggleSaved().
web/src/lib/router.ts                               No change expected (M1 already registered /feeds). If /feeds is not registered, add it here.
web/src/lib/types.ts                                Unchanged. The new FeedRow type lives in feedsFilter.ts so the helpers stay self-contained.
internal/api/subscriptions.go                       PATCH handler accepts {refresh_now: bool}; when true after the patch transaction, call poke().
internal/db/subscriptions.go                        + UpdateSubscriptionRefreshNow(ctx, d, id, userID) to set next_poll_at = 0 in one statement. Or extend UpdateSubscriptionPatch — implementer's call, see Task 1.2.
internal/api/subscriptions_test.go                  Tests for the new refresh_now flag.
internal/db/subscriptions_test.go                   Tests for the new DB function (if added).
internal/api/testing.go                             No change.
```

### Files deleted

```
web/src/components/AddFeedForm.svelte               Replaced by AddFeedDialog. Spec calls for the add-feed flow to live on /feeds.
web/src/components/AddFeedForm.svelte tests         Tests for the old component move into AddFeedDialog.test.ts in spirit.
web/src/components/FeedSettingsModal.svelte         M1 spec calls for it gone; M5 absorbs its job into EditFeedDialog. (M1 may have deleted it; if so, this step is a no-op.)
web/src/components/FeedSettingsModal.test.ts        Likewise.
```

> **Note on the deletes:** umbrella §3.3 says `FeedSettingsModal` is removed by M1 ("its job moves into the Feeds management page row actions in M5"). The current `web/src/components/FeedSettingsModal.svelte` is still on disk on this branch. If M1 lands before this milestone, the deletion is already done — skip the delete step. If M1 has not landed yet, this plan deletes it as part of Task 9.4.

---

## Layout of the page

Mirror the desktop canvas from §6.5 and `tap-feeds-page.jsx` lines 868–1064. Top to bottom within the `.ts-shell`:

1. Page head — eyebrow line ("Subscriptions · N feeds · M categories"; if errorCount > 0, " · K with errors" in `#c43a3a`) + serif title "Feeds".
2. `<FeedsToolbar>` — search + Add (row 1), filter chips + sort + Refresh all + Import OPML + Export (row 2).
3. `<FeedsBulkBar>` — conditional, only when `selected.size > 0`.
4. List — flat `<FeedsListRow>` per filtered feed. No grouping in the M5 deliverable (the JSX's group-by-category toggle is out of scope; see Risks).
5. Foot strip (`.ts-feeds-foot`) — pulse dot + "Auto-polling · last full sweep Xm ago · N feeds across M categories"; if errorCount > 0, append "· K need attention" in `#c43a3a`. The "last full sweep" string reads from the existing `pollStatus` store; if the store has never resolved, render "—".

Mobile branch (`isMobile === true` in M1's shell state): page reuses `<FeedsToolbar>` in a horizontally scrolling row, lists render via `<FeedsListRow>` with `density="mobile"`, the Add button becomes a FAB anchored to the shell. Bulk bar is rendered as a bottom drawer rather than a strip — implement that as a CSS-only branch on `.ts-feeds-bulk.is-mobile` so the same component tree handles both.

---

## Step-by-step tasks

### Task 1: Backend — `PATCH /api/v1/subscriptions/:id` accepts `refresh_now`

The Feeds page's per-row Refresh icon (and the toolbar's "Refresh all") need a way to schedule an immediate poll. The existing scheduler reads `subscriptions.next_poll_at`; setting it to `0` and calling `Scheduler.Poke()` is exactly the same path the `POST /api/v1/subscriptions` handler already uses for new subscriptions (see `cmd/tap/main.go` wiring `poke` into the API mux).

#### Task 1.1: Add `refresh_now` to the PATCH payload (RED)

**Files:**
- Modify: `internal/api/subscriptions_test.go`

- [ ] **Step 1: Add failing test** — append to `TestPatchSubscription` (or add a new top-level test `TestPatchSubscriptionRefreshNow`):

```go
func TestPatchSubscriptionRefreshNow(t *testing.T) {
    t.Parallel()
    h := newSubsHarness(t)
    sub := h.insertSubscription(t, "https://example.com/feed.xml")

    // Push next_poll_at into the future so the test can prove it gets reset.
    _, err := h.DB.ExecContext(t.Context(),
        `UPDATE subscriptions SET next_poll_at = ? WHERE id = ?`, time.Now().Add(1*time.Hour).Unix(), sub.ID)
    require.NoError(t, err)

    poked := 0
    h.poke = func() { poked++ }

    req := h.authenticated(t, "PATCH", "/api/v1/subscriptions/"+strconv.FormatInt(sub.ID, 10),
        strings.NewReader(`{"refresh_now":true}`))
    rec := httptest.NewRecorder()
    h.mux.ServeHTTP(rec, req)

    require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

    var dto subscriptionDTO
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dto))
    require.Equal(t, int64(0), dto.NextPollAt, "next_poll_at must reset to 0")
    require.Equal(t, 1, poked, "scheduler must be poked exactly once")
}
```

Reuse whatever helpers the existing `subscriptions_test.go` uses; if a `poke` field doesn't exist on the test harness, wire it through the same mechanism the existing `POST /api/v1/subscriptions` test already uses (see the `NewMux` call in `internal/api/testing.go`).

- [ ] **Step 2: Run the test to confirm RED** — `go test ./internal/api -run TestPatchSubscriptionRefreshNow -race -count=1`. Expect failure: PATCH handler ignores unknown fields, `next_poll_at` stays at the future timestamp, `poked == 0`.

#### Task 1.2: DB function `UpdateSubscriptionRefreshNow` (GREEN)

**Files:**
- Modify: `internal/db/subscriptions.go`
- Modify: `internal/db/subscriptions_test.go`

- [ ] **Step 1: Failing test** in `subscriptions_test.go`:

```go
func TestUpdateSubscriptionRefreshNow(t *testing.T) {
    t.Parallel()
    d := testdb.New(t)
    u := testdb.InsertUser(t, d, "owner")
    s := testdb.InsertSubscription(t, d, u.ID, "https://example.com/feed.xml")
    _, err := d.ExecContext(t.Context(),
        `UPDATE subscriptions SET next_poll_at = ? WHERE id = ?`, time.Now().Add(time.Hour).Unix(), s.ID)
    require.NoError(t, err)

    require.NoError(t, db.UpdateSubscriptionRefreshNow(t.Context(), d, s.ID, u.ID))

    var n int64
    require.NoError(t, d.QueryRowContext(t.Context(),
        `SELECT next_poll_at FROM subscriptions WHERE id = ?`, s.ID).Scan(&n))
    require.Equal(t, int64(0), n)
}

func TestUpdateSubscriptionRefreshNow_WrongUser(t *testing.T) {
    t.Parallel()
    d := testdb.New(t)
    owner := testdb.InsertUser(t, d, "owner")
    other := testdb.InsertUser(t, d, "other")
    s := testdb.InsertSubscription(t, d, owner.ID, "https://example.com/feed.xml")

    err := db.UpdateSubscriptionRefreshNow(t.Context(), d, s.ID, other.ID)
    require.ErrorIs(t, err, sql.ErrNoRows)
}
```

- [ ] **Step 2: Run** — `go test ./internal/db -run TestUpdateSubscriptionRefreshNow -race -count=1`. Expect compile error: function not defined.

- [ ] **Step 3: Implement** in `internal/db/subscriptions.go`:

```go
// UpdateSubscriptionRefreshNow sets next_poll_at = 0 for the given subscription,
// scoped to the owning user. Returns sql.ErrNoRows if no row matched (wrong user
// or unknown id) so callers can return a 404.
func UpdateSubscriptionRefreshNow(ctx context.Context, d *sql.DB, id, userID int64) error {
    res, err := d.ExecContext(ctx,
        `UPDATE subscriptions SET next_poll_at = 0 WHERE id = ? AND user_id = ?`, id, userID)
    if err != nil {
        return fmt.Errorf("refresh now: %w", err)
    }
    n, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("refresh now rows: %w", err)
    }
    if n == 0 {
        return sql.ErrNoRows
    }
    return nil
}
```

- [ ] **Step 4: Run the DB test green** — `go test ./internal/db -run TestUpdateSubscriptionRefreshNow -race -count=1`. Expect both pass.

#### Task 1.3: Wire `refresh_now` into the PATCH handler (GREEN)

**Files:**
- Modify: `internal/api/subscriptions.go` (around line 180 — the existing `PATCH /api/v1/subscriptions/{id}` HandleFunc)

- [ ] **Step 1: Implement** — extend the request struct used by the PATCH decoder with `RefreshNow *bool \`json:"refresh_now,omitempty"\``. After the existing patch path completes (or in lieu of it when the body only contains `refresh_now`), call `db.UpdateSubscriptionRefreshNow(ctx, d, id, u.ID)` and then `h.poke()`. Order:
  1. Decode body.
  2. If any standard PATCH field is present, run the existing patch transaction.
  3. If `refresh_now` is true, run `db.UpdateSubscriptionRefreshNow`. On `sql.ErrNoRows`, return 404 via `writeError(w, 404, "subscription_not_found", ...)`.
  4. After the row update succeeds and `refresh_now` was true, call `h.poke()` exactly once (do not call it for non-refresh PATCHes — those don't change `next_poll_at`).
  5. Re-read the subscription row and write the existing DTO response.

Do not introduce a separate handler; do not change the route. The reason is mechanical: this is the smallest possible surface change and the umbrella spec explicitly permits "narrow additions a milestone explicitly justifies."

- [ ] **Step 2: Run** — `go test ./internal/api -run TestPatchSubscription -race -count=1`. Existing PATCH tests still pass, new `TestPatchSubscriptionRefreshNow` goes green.

- [ ] **Step 3: Commit**

```bash
git add internal/api/subscriptions.go internal/api/subscriptions_test.go \
        internal/db/subscriptions.go internal/db/subscriptions_test.go
git commit -m "PATCH /subscriptions/:id accepts refresh_now to force immediate poll

Wired into Scheduler.Poke() so the Feeds management page (M-Redesign-5)
can offer real per-row and bulk refresh. New DB function
UpdateSubscriptionRefreshNow sets next_poll_at = 0, scoped to user."
```

#### Task 1.4: Frontend helper for the new flag

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/__tests__/` — new `api.refresh.test.ts` if no existing api.ts test, otherwise extend the existing one.

- [ ] **Step 1: Failing test** — `web/src/lib/__tests__/api.refresh.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from '../api';

const fetchMock = vi.fn();
vi.stubGlobal('fetch', fetchMock);

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue(new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } }));
});

describe('api.refreshSubscription', () => {
  it('PATCHes refresh_now:true to /api/v1/subscriptions/:id', async () => {
    await api.refreshSubscription(42);
    expect(fetchMock).toHaveBeenCalledOnce();
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/v1/subscriptions/42');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body as string)).toEqual({ refresh_now: true });
  });
});
```

- [ ] **Step 2: Run** — `pnpm --dir web test -- src/lib/__tests__/api.refresh.test.ts`. Expect failure (method does not exist).

- [ ] **Step 3: Implement** — add to `api` object in `web/src/lib/api.ts`:

```ts
refreshSubscription: (id: number) =>
  request<Subscription>(`/subscriptions/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ refresh_now: true }),
  }),
```

- [ ] **Step 4: Run green**

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/api.ts web/src/lib/__tests__/api.refresh.test.ts
git commit -m "api.refreshSubscription: PATCH refresh_now:true to force immediate poll"
```

---

### Task 2: Store helpers for the Feeds page

**Files:**
- Modify: `web/src/lib/store.ts`
- Modify: `web/src/lib/__tests__/store.test.ts` (or add `store.feeds.test.ts` if cleaner)

The current `subscriptions` store only exposes `subscribe`, `load`, `add`. The Feeds page needs `refresh(id)`, `remove(id)`, `setCategory(id, catId)`. Each is a thin wrapper around `api.*` that calls `subscriptions.load()` on success and emits via the existing `notifySW` invalidation pattern.

#### Task 2.1: `subscriptions.refresh(id)`

- [ ] **Step 1: Failing test** in `store.test.ts`:

```ts
it('subscriptions.refresh calls api.refreshSubscription and reloads', async () => {
  vi.mocked(api.refreshSubscription).mockResolvedValue({ /* … */ } as any);
  vi.mocked(api.listSubscriptions).mockResolvedValue([]);
  await subscriptions.refresh(11);
  expect(api.refreshSubscription).toHaveBeenCalledWith(11);
  expect(api.listSubscriptions).toHaveBeenCalled();
});
```

- [ ] **Step 2: Run RED** — `pnpm --dir web test -- src/lib/__tests__/store.test.ts`

- [ ] **Step 3: Implement** — extend `subscriptionsStore()`:

```ts
async refresh(id: number) {
  await api.refreshSubscription(id);
  notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions'] });
  await this.load();
},
```

- [ ] **Step 4: Run GREEN**

#### Task 2.2: `subscriptions.remove(id)`

- [ ] **Step 1: Failing test** (same pattern as 2.1 but with `api.deleteSubscription`)

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement**:

```ts
async remove(id: number) {
  await api.deleteSubscription(id);
  notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/entries'] });
  await this.load();
},
```

(entries are invalidated because deleting a subscription cascades to its entries server-side.)

- [ ] **Step 4: GREEN**

#### Task 2.3: `subscriptions.setCategory(id, catId)`

- [ ] **Step 1: Failing test** — call with `setCategory(7, 3)` then `setCategory(7, null)`; assert both call `api.updateSubscription` with `{category_id}` and reload.

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement**:

```ts
async setCategory(id: number, categoryId: number | null) {
  await api.updateSubscription(id, { category_id: categoryId });
  notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/categories'] });
  await this.load();
},
```

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/store.ts web/src/lib/__tests__/store.test.ts
git commit -m "subscriptions store: refresh/remove/setCategory helpers for Feeds page"
```

---

### Task 3: Pure-function filter/sort/search helpers

**Files:**
- Create: `web/src/lib/feedsFilter.ts`
- Create: `web/src/lib/__tests__/feedsFilter.test.ts`

Keep filter/sort/search logic out of the Svelte view so it's trivially testable. Each function is pure; the view passes in `Subscription[]` plus computed unread counts and gets back a filtered/sorted list.

#### Task 3.1: Types and shared shape

- [ ] **Step 1: Write** the shape — `web/src/lib/feedsFilter.ts`:

```ts
import type { Subscription } from './types';

// Decorates a Subscription with view-only counts so filter/sort can see them.
export type FeedRow = Subscription & {
  unread: number;
  lastPollAgo: number; // seconds; Number.POSITIVE_INFINITY when last_poll_at is unset
};

export type FilterKey = 'all' | 'errors' | 'unread' | 'stale';
export type SortKey = 'recent' | 'name' | 'added' | 'unread';

// A feed is "stale" if its most recent successful poll was > 7 days ago AND it
// is not currently in error backoff (errors handle their own chip).
export const STALE_THRESHOLD_SECONDS = 7 * 24 * 60 * 60;
```

(Why a `FeedRow` decoration? The view computes `unread` once via a derived rune over `categories` + `entries.byFeed`. Passing the decorated array to filter/sort keeps both the JSX-mirroring loop and the pure functions simple.)

#### Task 3.2: `filterFeeds(rows, key, search) → FeedRow[]`

- [ ] **Step 1: Failing test** — `feedsFilter.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { filterFeeds, sortFeeds, STALE_THRESHOLD_SECONDS, type FeedRow } from '../feedsFilter';

function row(p: Partial<FeedRow> & { id: number; title: string; feed_url: string }): FeedRow {
  return {
    id: p.id,
    title: p.title,
    feed_url: p.feed_url,
    site_url: p.site_url ?? '',
    next_poll_at: 0,
    last_poll_at: p.last_poll_at ?? 0,
    error_count: p.error_count ?? 0,
    last_error: p.last_error,
    created_at: p.created_at ?? 0,
    extract: false,
    extract_selector: '',
    has_cookie: false,
    has_basic_auth: false,
    category_id: p.category_id ?? null,
    unread: p.unread ?? 0,
    lastPollAgo: p.lastPollAgo ?? 0,
  };
}

describe('filterFeeds', () => {
  const rows = [
    row({ id: 1, title: 'Alpha',  feed_url: 'a.example/feed', unread: 4 }),
    row({ id: 2, title: 'Beta',   feed_url: 'b.example/feed', error_count: 2, last_error: 'http 500' }),
    row({ id: 3, title: 'Gamma',  feed_url: 'g.example/feed', unread: 0, lastPollAgo: STALE_THRESHOLD_SECONDS + 1 }),
    row({ id: 4, title: 'Delta',  feed_url: 'd.example/feed', unread: 1 }),
  ];

  it('all: returns every row', () => {
    expect(filterFeeds(rows, 'all', '').map(r => r.id)).toEqual([1, 2, 3, 4]);
  });
  it('errors: returns rows with error_count > 0', () => {
    expect(filterFeeds(rows, 'errors', '').map(r => r.id)).toEqual([2]);
  });
  it('unread: returns rows where unread > 0', () => {
    expect(filterFeeds(rows, 'unread', '').map(r => r.id)).toEqual([1, 4]);
  });
  it('stale: returns rows whose lastPollAgo exceeds STALE_THRESHOLD_SECONDS AND error_count is 0', () => {
    expect(filterFeeds(rows, 'stale', '').map(r => r.id)).toEqual([3]);
  });
  it('search: case-insensitive substring across title and feed_url; ANDs with filter', () => {
    expect(filterFeeds(rows, 'all', 'ALPHA').map(r => r.id)).toEqual([1]);
    expect(filterFeeds(rows, 'all', 'example').map(r => r.id)).toEqual([1, 2, 3, 4]);
    expect(filterFeeds(rows, 'errors', 'beta').map(r => r.id)).toEqual([2]);
    expect(filterFeeds(rows, 'errors', 'alpha').map(r => r.id)).toEqual([]);
  });
});
```

- [ ] **Step 2: RED** — `pnpm --dir web test -- src/lib/__tests__/feedsFilter.test.ts`

- [ ] **Step 3: Implement** in `feedsFilter.ts`:

```ts
export function filterFeeds(rows: FeedRow[], key: FilterKey, search: string): FeedRow[] {
  const q = search.trim().toLowerCase();
  return rows.filter((r) => {
    if (key === 'errors' && r.error_count <= 0) return false;
    if (key === 'unread' && r.unread <= 0) return false;
    if (key === 'stale') {
      if (r.error_count > 0) return false;
      if (r.lastPollAgo <= STALE_THRESHOLD_SECONDS) return false;
    }
    if (q) {
      const hay = (r.title + ' ' + r.feed_url).toLowerCase();
      if (!hay.includes(q)) return false;
    }
    return true;
  });
}
```

- [ ] **Step 4: GREEN**

#### Task 3.3: `sortFeeds(rows, key) → FeedRow[]`

- [ ] **Step 1: Failing test** — sort orderings: `name` is title alphabetic with case-insensitive compare; `recent` is by `last_poll_at` descending (zero → bottom); `added` is by `created_at` descending; `unread` is by `unread` descending. Tie-break is `id` ascending.

```ts
describe('sortFeeds', () => {
  const rows = [
    row({ id: 1, title: 'beta',  feed_url: '', last_poll_at: 100, unread: 0, created_at: 1 }),
    row({ id: 2, title: 'Alpha', feed_url: '', last_poll_at: 300, unread: 5, created_at: 2 }),
    row({ id: 3, title: 'alpha', feed_url: '', last_poll_at: 200, unread: 5, created_at: 3 }),
    row({ id: 4, title: 'Gamma', feed_url: '', last_poll_at: 0,   unread: 1, created_at: 4 }),
  ];
  it('name is case-insensitive alphabetical, id-asc on tie', () => {
    expect(sortFeeds(rows, 'name').map(r => r.id)).toEqual([2, 3, 1, 4]);
  });
  it('recent is by last_poll_at desc with zero last (never polled) sinking to bottom', () => {
    expect(sortFeeds(rows, 'recent').map(r => r.id)).toEqual([2, 3, 1, 4]);
  });
  it('added is by created_at desc', () => {
    expect(sortFeeds(rows, 'added').map(r => r.id)).toEqual([4, 3, 2, 1]);
  });
  it('unread is by unread desc, id-asc on tie', () => {
    expect(sortFeeds(rows, 'unread').map(r => r.id)).toEqual([2, 3, 4, 1]);
  });
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement**:

```ts
const compareByName    = (a: FeedRow, b: FeedRow) => a.title.localeCompare(b.title, undefined, { sensitivity: 'base' });
const compareByRecent  = (a: FeedRow, b: FeedRow) => (b.last_poll_at || 0) - (a.last_poll_at || 0);
const compareByAdded   = (a: FeedRow, b: FeedRow) => (b.created_at || 0) - (a.created_at || 0);
const compareByUnread  = (a: FeedRow, b: FeedRow) => b.unread - a.unread;

export function sortFeeds(rows: FeedRow[], key: SortKey): FeedRow[] {
  const cmp = { name: compareByName, recent: compareByRecent, added: compareByAdded, unread: compareByUnread }[key];
  return [...rows].sort((a, b) => {
    const c = cmp(a, b);
    return c !== 0 ? c : a.id - b.id;
  });
}
```

- [ ] **Step 4: GREEN**

#### Task 3.4: Counts for the chips

- [ ] **Step 1: Failing test** for `countByKey(rows)`:

```ts
it('countByKey returns chip counts for all / errors / unread / stale', () => {
  expect(countByKey(rows)).toEqual({ all: 4, errors: 1, unread: 2, stale: 1 });
});
```

- [ ] **Step 2: Implement**:

```ts
export function countByKey(rows: FeedRow[]): Record<FilterKey, number> {
  return {
    all:    rows.length,
    errors: filterFeeds(rows, 'errors', '').length,
    unread: filterFeeds(rows, 'unread', '').length,
    stale:  filterFeeds(rows, 'stale',  '').length,
  };
}
```

(Calling `filterFeeds` four times on a feeds list of ≤ a few hundred is fine. Don't micro-optimise.)

- [ ] **Step 3: GREEN**

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/feedsFilter.ts web/src/lib/__tests__/feedsFilter.test.ts
git commit -m "feedsFilter: pure filter/sort/count helpers for /feeds page"
```

---

### Task 4: `FeedsToolbar.svelte` (row 1 + row 2)

**Files:**
- Create: `web/src/components/feeds/FeedsToolbar.svelte`
- Create: `web/src/components/feeds/__tests__/FeedsToolbar.test.ts`

Props:

```ts
type Props = {
  search: string;
  filter: FilterKey;
  sort: SortKey;
  counts: Record<FilterKey, number>;
  refreshingAll: boolean;
  onSearch: (q: string) => void;       // emits on every keystroke; parent debounces
  onFilter: (k: FilterKey) => void;
  onSort: (s: SortKey) => void;
  onAdd: () => void;
  onRefreshAll: () => void;
  onImport: () => void;
  onExport: () => void;
};
```

#### Task 4.1: Row 1 — search input + Add feed button

- [ ] **Step 1: Failing test** — `FeedsToolbar.test.ts`:

```ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import FeedsToolbar from '../FeedsToolbar.svelte';

const baseProps = {
  search: '', filter: 'all' as const, sort: 'name' as const,
  counts: { all: 0, errors: 0, unread: 0, stale: 0 },
  refreshingAll: false,
  onSearch: () => {}, onFilter: () => {}, onSort: () => {},
  onAdd: () => {}, onRefreshAll: () => {}, onImport: () => {}, onExport: () => {},
};

it('renders search input and Add feed button', () => {
  render(FeedsToolbar, { props: baseProps });
  expect(screen.getByPlaceholderText(/search feeds by name or url/i)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /add feed/i })).toBeInTheDocument();
});

it('emits onSearch on every keystroke', async () => {
  const onSearch = vi.fn();
  render(FeedsToolbar, { props: { ...baseProps, onSearch } });
  await fireEvent.input(screen.getByPlaceholderText(/search feeds/i), { target: { value: 'jvns' } });
  expect(onSearch).toHaveBeenLastCalledWith('jvns');
});

it('emits onAdd when the Add feed button is clicked', async () => {
  const onAdd = vi.fn();
  render(FeedsToolbar, { props: { ...baseProps, onAdd } });
  await fireEvent.click(screen.getByRole('button', { name: /add feed/i }));
  expect(onAdd).toHaveBeenCalledOnce();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — selectors `.ts-feeds-top`, `.ts-feeds-search`, `.ts-feeds-search-input`, `.ts-feeds-search-ico`, `.ts-feeds-add`. Use `<input type="text" oninput={(e) => onSearch(e.currentTarget.value)}>` with `value={search}` (not bind — keeps the source of truth in the parent so debouncing is the parent's choice). The search icon is the existing M1 design system icon set's `search` glyph; render inline as an SVG matching `TF_ICO.search` in the JSX (lines 67 of `tap-feeds-page.jsx`).

The Add button uses the M1 `Button` with `variant="primary"` and a `+` icon slot. Falling back to a local `.ts-feeds-add` class is fine if `Button` doesn't yet support icon slots — the milestone owns this style port.

- [ ] **Step 4: GREEN**

#### Task 4.2: Row 2 — filter chips

- [ ] **Step 1: Failing test**:

```ts
it('renders 4 filter chips with counts', () => {
  render(FeedsToolbar, { props: { ...baseProps, counts: { all: 12, errors: 1, unread: 7, stale: 2 } } });
  ['All', 'Errors', 'Unread', 'Stale'].forEach((label) => {
    expect(screen.getByRole('button', { name: new RegExp(label, 'i') })).toBeInTheDocument();
  });
  // Count tags
  expect(screen.getByText('12')).toBeInTheDocument();
  expect(screen.getByText('1')).toBeInTheDocument();
  expect(screen.getByText('7')).toBeInTheDocument();
  expect(screen.getByText('2')).toBeInTheDocument();
});

it('marks the active filter chip with is-active', () => {
  const { container } = render(FeedsToolbar, { props: { ...baseProps, filter: 'errors' } });
  const errorsChip = container.querySelector('button[data-filter-key="errors"]')!;
  expect(errorsChip.classList.contains('is-active')).toBe(true);
});

it('Errors chip carries is-warn when its count is > 0', () => {
  const { container } = render(FeedsToolbar, { props: { ...baseProps, counts: { ...baseProps.counts, errors: 3 } } });
  const errorsChip = container.querySelector('button[data-filter-key="errors"]')!;
  expect(errorsChip.classList.contains('is-warn')).toBe(true);
});

it('emits onFilter when a chip is clicked', async () => {
  const onFilter = vi.fn();
  render(FeedsToolbar, { props: { ...baseProps, onFilter } });
  await fireEvent.click(screen.getByRole('button', { name: /unread/i }));
  expect(onFilter).toHaveBeenLastCalledWith('unread');
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — render four `<Chip>` from M1 (or local `<button class="ts-feeds-chip">` with `is-active` / `is-warn` classes). `data-filter-key` is for the test; not strictly part of the design. Chip count uses `<span class="ct">{count}</span>` per `.ts-feeds-chip .ct` styling rule (CSS line 3630).

- [ ] **Step 4: GREEN**

#### Task 4.3: Row 2 — sort select

- [ ] **Step 1: Failing test**:

```ts
it('renders a sort select with four options', () => {
  render(FeedsToolbar, { props: baseProps });
  const select = screen.getByRole('combobox', { name: /sort/i });
  expect(select).toBeInTheDocument();
  ['Recently active', 'Name', 'Added', 'Most unread'].forEach((label) => {
    expect(screen.getByRole('option', { name: new RegExp(label, 'i') })).toBeInTheDocument();
  });
});

it('emits onSort when the select changes', async () => {
  const onSort = vi.fn();
  render(FeedsToolbar, { props: { ...baseProps, onSort } });
  const select = screen.getByRole('combobox', { name: /sort/i });
  await fireEvent.change(select, { target: { value: 'unread' } });
  expect(onSort).toHaveBeenLastCalledWith('unread');
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — native `<select>` with `.ts-feeds-sort-select` styling. Wrap in `<span class="ts-feeds-sort">Sort <select … /></span>`. Mono caps styling comes from the CSS rule (line 3655). Option values: `recent`, `name`, `added`, `unread`.

- [ ] **Step 4: GREEN**

#### Task 4.4: Row 2 — Refresh all / Import OPML / Export quiet buttons

- [ ] **Step 1: Failing test**:

```ts
it('renders Refresh all, Import OPML, and Export buttons', () => {
  render(FeedsToolbar, { props: baseProps });
  expect(screen.getByRole('button', { name: /refresh all/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /import opml/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /export/i })).toBeInTheDocument();
});

it('Refresh all button spins when refreshingAll is true', () => {
  const { container } = render(FeedsToolbar, { props: { ...baseProps, refreshingAll: true } });
  const btn = container.querySelector('button[data-action="refresh-all"]')!;
  expect(btn.classList.contains('is-spinning')).toBe(true);
});

it('emits onRefreshAll / onImport / onExport when clicked', async () => {
  const onRefreshAll = vi.fn(), onImport = vi.fn(), onExport = vi.fn();
  render(FeedsToolbar, { props: { ...baseProps, onRefreshAll, onImport, onExport } });
  await fireEvent.click(screen.getByRole('button', { name: /refresh all/i }));
  await fireEvent.click(screen.getByRole('button', { name: /import opml/i }));
  await fireEvent.click(screen.getByRole('button', { name: /export/i }));
  expect(onRefreshAll).toHaveBeenCalledOnce();
  expect(onImport).toHaveBeenCalledOnce();
  expect(onExport).toHaveBeenCalledOnce();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — three `<button class="ts-feeds-util-btn">` with the refresh / upload / download SVG icons inline. Add `is-spinning` class when `refreshingAll` is true (existing keyframe `tf-spin` already defined in styles.css line 3703 — copy that into the global stylesheet from M1).

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Commit**

```bash
git add web/src/components/feeds/FeedsToolbar.svelte web/src/components/feeds/__tests__/FeedsToolbar.test.ts
git commit -m "FeedsToolbar: search+add row, filter chips, sort, utility buttons"
```

---

### Task 5: `FeedsListRow.svelte` (per-feed row)

**Files:**
- Create: `web/src/components/feeds/FeedsListRow.svelte`
- Create: `web/src/components/feeds/FeedsHealthPanel.svelte`
- Create: `web/src/components/feeds/__tests__/FeedsListRow.test.ts`

Props (`FeedsListRow`):

```ts
type Props = {
  feed: FeedRow;
  categoryName: string | null;
  isSelected: boolean;
  anySelected: boolean;
  isExpanded: boolean;
  isRefreshing: boolean;
  onToggleSelect: () => void;
  onToggleExpand: () => void;
  onRefresh: () => void;
  onChangeCategory: () => void;   // opens a Popover positioned to the row's category chip
  onEdit: () => void;             // opens EditFeedDialog
  onDelete: () => void;           // opens DeleteFeedsDialog with [feed.id]
};
```

#### Task 5.1: Row chrome — checkbox + avatar + body line 1 + actions

- [ ] **Step 1: Failing test** — `FeedsListRow.test.ts`:

```ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import FeedsListRow from '../FeedsListRow.svelte';
import type { FeedRow } from '../../../lib/feedsFilter';

const feed: FeedRow = {
  id: 1, title: 'Julia Evans', feed_url: 'jvns.ca/atom.xml', site_url: 'https://jvns.ca',
  next_poll_at: 0, last_poll_at: Math.floor(Date.now()/1000) - 60, error_count: 0,
  created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false,
  category_id: 3, unread: 12, lastPollAgo: 60,
};

const noop = () => {};
const props = {
  feed, categoryName: 'People', isSelected: false, anySelected: false,
  isExpanded: false, isRefreshing: false,
  onToggleSelect: noop, onToggleExpand: noop, onRefresh: noop,
  onChangeCategory: noop, onEdit: noop, onDelete: noop,
};

it('renders avatar, name, category chip, and url', () => {
  render(FeedsListRow, { props });
  expect(screen.getByText('Julia Evans')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /people/i })).toBeInTheDocument();
  expect(screen.getByText('jvns.ca/atom.xml')).toBeInTheDocument();
});

it('renders an uncategorised chip with the is-uncat class when category_id is null', () => {
  const { container } = render(FeedsListRow, { props: { ...props, feed: { ...feed, category_id: null }, categoryName: null } });
  const chip = container.querySelector('.ts-feed-cat.is-uncat');
  expect(chip).not.toBeNull();
  expect(chip!.textContent).toMatch(/uncategorised/i);
});

it('checkbox emits onToggleSelect; selected adds is-selected class', async () => {
  const onToggleSelect = vi.fn();
  const { container, rerender } = render(FeedsListRow, { props: { ...props, onToggleSelect } });
  const cb = container.querySelector('.ts-feed-check')!;
  await fireEvent.click(cb);
  expect(onToggleSelect).toHaveBeenCalledOnce();
  await rerender({ ...props, isSelected: true });
  expect(container.querySelector('.ts-feed-row.is-selected')).not.toBeNull();
});

it('refresh button emits onRefresh; isRefreshing adds is-spinning', async () => {
  const onRefresh = vi.fn();
  const { container, rerender } = render(FeedsListRow, { props: { ...props, onRefresh } });
  await fireEvent.click(container.querySelector('button[data-action="refresh"]')!);
  expect(onRefresh).toHaveBeenCalledOnce();
  await rerender({ ...props, isRefreshing: true });
  expect(container.querySelector('button[data-action="refresh"]')?.classList.contains('is-spinning')).toBe(true);
});

it('more menu emits onEdit when its first item is clicked', async () => {
  const onEdit = vi.fn();
  render(FeedsListRow, { props: { ...props, onEdit } });
  await fireEvent.click(screen.getByRole('button', { name: /edit feed/i }));
  expect(onEdit).toHaveBeenCalledOnce();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — match `tap-feeds-page.jsx` `TFFeedRow` (lines 123–248) structure:

```svelte
<div class="ts-feed-row" class:is-selected={isSelected} class:has-error={feed.error_count > 0} class:is-busy={isRefreshing}>
  <button class="ts-feed-check" class:is-checked={isSelected} onclick={onToggleSelect} aria-label={isSelected ? 'Deselect' : 'Select'}>
    {#if isSelected}<svg …check icon… />{/if}
  </button>
  <span class="ts-feed-avatar-wrap"><FeedAvatar feedURL={feed.feed_url} size={18} /></span>
  <div class="ts-feed-body">
    <div class="ts-feed-line1">
      <h3 class="ts-feed-name">{feed.title}</h3>
      <button class="ts-feed-cat" class:is-uncat={!feed.category_id} onclick={onChangeCategory}>
        {categoryName ?? 'uncategorised'}
      </button>
      {#if feed.error_count > 0}
        <span class="ts-feed-err-chip" title={feed.last_error ?? ''}>
          <svg …warn… /> {feed.error_count} {feed.error_count === 1 ? 'error' : 'errors'}
        </span>
      {/if}
    </div>
    <div class="ts-feed-line2">
      <a class="ts-feed-url" href={feed.site_url || `https://${feed.feed_url}`} target="_blank" rel="noopener noreferrer">
        {feed.feed_url}<span class="ico"><svg …external… /></span>
      </a>
      <span class="dot" aria-hidden="true"></span>
      {#if feed.error_count > 0}
        <span class="err">last poll <b>{formatAgo(feed.lastPollAgo)}</b> ago</span>
        <span class="dot" aria-hidden="true"></span>
        <button class="ts-feeds-util-btn" onclick={onToggleExpand}>{isExpanded ? 'Hide details' : 'Why?'}</button>
      {:else}
        <span>polled <b>{formatAgo(feed.lastPollAgo)}</b> ago</span>
        <span class="dot" aria-hidden="true"></span>
        <span class="unread">unread <b>{feed.unread}</b></span>
      {/if}
    </div>
  </div>
  <div class="ts-feed-actions">
    <span class="ts-feed-act-unread" class:is-zero={feed.unread === 0}><b>{feed.unread}</b><span>unread</span></span>
    <button class="ts-feed-act" class:is-spinning={isRefreshing} data-action="refresh" onclick={onRefresh} aria-label="Refresh now"><svg …refresh… /></button>
    <button class="ts-feed-act" data-action="edit" onclick={onEdit} aria-label="Edit feed"><svg …more… /></button>
  </div>
  {#if isExpanded && feed.error_count > 0}
    <FeedsHealthPanel {feed} onRetry={onRefresh} onEdit={onEdit} />
  {/if}
</div>
```

`formatAgo(seconds)` is a small helper local to this file (or in `web/src/lib/time.ts`): seconds < 60 → "Xs", < 3600 → "Xm", < 86400 → "Xh", otherwise "Xd". Test it via the row's rendered output, not separately.

- [ ] **Step 4: GREEN**

#### Task 5.2: Health panel

- [ ] **Step 1: Failing test** — `FeedsHealthPanel.test.ts`:

```ts
it('renders the last error string and the 3-cell grid', () => {
  const { container } = render(FeedsHealthPanel, {
    props: {
      feed: { …feedWithError, error_count: 5, last_error: 'HTTP 502 Bad Gateway', last_poll_at: Math.floor(Date.now()/1000) - 7200, lastPollAgo: 7200 },
      onRetry: vi.fn(), onEdit: vi.fn(),
    },
  });
  expect(container.querySelector('.ts-feed-health-msg')!.textContent).toContain('HTTP 502 Bad Gateway');
  expect(container.querySelectorAll('.ts-feed-health-cell')).toHaveLength(3);
});

it('Retry now button calls onRetry; Edit credentials calls onEdit', async () => {
  const onRetry = vi.fn(), onEdit = vi.fn();
  render(FeedsHealthPanel, { props: { feed: feedWithError, onRetry, onEdit } });
  await fireEvent.click(screen.getByRole('button', { name: /retry now/i }));
  await fireEvent.click(screen.getByRole('button', { name: /edit credentials/i }));
  expect(onRetry).toHaveBeenCalledOnce();
  expect(onEdit).toHaveBeenCalledOnce();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — three cells: `Consecutive errors` / `Last try` / `Status`. Match `tap-feeds-page.jsx` lines 218–244 markup. Selectors `.ts-feed-health`, `.ts-feed-health-eyebrow`, `.ts-feed-health-msg`, `.ts-feed-health-grid`, `.ts-feed-health-cell`, `.ts-feed-health-l`, `.ts-feed-health-v`, `.ts-feed-health-actions`. The footer brings in two `.ts-feeds-util-btn` buttons (Retry now, Edit credentials). "Mark all read" from the JSX is dropped — there's no per-feed mark-read endpoint; see Risk #1.

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Commit**

```bash
git add web/src/components/feeds/FeedsListRow.svelte \
        web/src/components/feeds/FeedsHealthPanel.svelte \
        web/src/components/feeds/__tests__/FeedsListRow.test.ts \
        web/src/components/feeds/__tests__/FeedsHealthPanel.test.ts
git commit -m "FeedsListRow + FeedsHealthPanel: per-feed row chrome and expanded panel"
```

---

### Task 6: `FeedsBulkBar.svelte`

**Files:**
- Create: `web/src/components/feeds/FeedsBulkBar.svelte`
- Create: `web/src/components/feeds/__tests__/FeedsBulkBar.test.ts`

Props:

```ts
type Props = {
  count: number;
  onClear: () => void;
  onRefresh: () => void;
  onReassign: () => void;
  onDelete: () => void;
};
```

- [ ] **Step 1: Failing test**:

```ts
it('renders "N selected" and Clear / Refresh / Set category / Delete', () => {
  render(FeedsBulkBar, { props: { count: 3, onClear: noop, onRefresh: noop, onReassign: noop, onDelete: noop } });
  expect(screen.getByText('3 selected')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /clear/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /set category/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
});

it('emits each handler when its button is clicked', async () => {
  const onClear = vi.fn(), onRefresh = vi.fn(), onReassign = vi.fn(), onDelete = vi.fn();
  render(FeedsBulkBar, { props: { count: 2, onClear, onRefresh, onReassign, onDelete } });
  await fireEvent.click(screen.getByRole('button', { name: /clear/i }));
  await fireEvent.click(screen.getByRole('button', { name: /refresh/i }));
  await fireEvent.click(screen.getByRole('button', { name: /set category/i }));
  await fireEvent.click(screen.getByRole('button', { name: /delete/i }));
  expect(onClear).toHaveBeenCalledOnce();
  expect(onRefresh).toHaveBeenCalledOnce();
  expect(onReassign).toHaveBeenCalledOnce();
  expect(onDelete).toHaveBeenCalledOnce();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — match `tap-feeds-page.jsx` `TFBulkBar` (lines 253–270). Selectors `.ts-feeds-bulk`, `.ts-feeds-bulk-count`, `.ts-feeds-bulk-clear`, `.ts-feeds-bulk-spacer`, `.ts-feeds-bulk-btn`, `.ts-feeds-bulk-btn.is-danger`. Delete button takes `.is-danger`.

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Commit**

```bash
git add web/src/components/feeds/FeedsBulkBar.svelte \
        web/src/components/feeds/__tests__/FeedsBulkBar.test.ts
git commit -m "FeedsBulkBar: N selected + clear/refresh/reassign/delete actions"
```

---

### Task 7: `ReassignCategoryPopover.svelte`

**Files:**
- Create: `web/src/components/feeds/ReassignCategoryPopover.svelte`
- Create: `web/src/components/feeds/__tests__/ReassignCategoryPopover.test.ts`

Used by:
- Per-row "change category" (clicking the category chip on a row).
- Bulk "Set category…" (clicking the bulk bar's Reassign button).

Mounted inside the M1 `Popover` primitive. Renders the category list with a leading check on the current selection. "Uncategorised" is always the first row.

Props:

```ts
type Props = {
  categories: Category[];
  currentId: number | null;  // ignored in bulk mode where there's no single current
  onPick: (id: number | null) => void;
  onDismiss: () => void;
};
```

- [ ] **Step 1: Failing test**:

```ts
it('renders Uncategorised first followed by each category', () => {
  render(ReassignCategoryPopover, { props: { categories, currentId: null, onPick: noop, onDismiss: noop } });
  const rows = screen.getAllByRole('button');
  expect(rows[0].textContent).toMatch(/uncategorised/i);
  expect(rows[1].textContent).toContain('People');
});

it('marks the current selection with a check column / is-current class', () => {
  const { container } = render(ReassignCategoryPopover, { props: { categories, currentId: categories[0].id, onPick: noop, onDismiss: noop } });
  const row = container.querySelector(`button[data-cat-id="${categories[0].id}"]`)!;
  expect(row.classList.contains('is-current')).toBe(true);
});

it('calls onPick with null for Uncategorised, and onDismiss after', async () => {
  const onPick = vi.fn(), onDismiss = vi.fn();
  render(ReassignCategoryPopover, { props: { categories, currentId: 1, onPick, onDismiss } });
  await fireEvent.click(screen.getAllByRole('button')[0]);
  expect(onPick).toHaveBeenCalledWith(null);
  expect(onDismiss).toHaveBeenCalledOnce();
});

it('calls onPick with the category id when a row is clicked', async () => {
  const onPick = vi.fn();
  render(ReassignCategoryPopover, { props: { categories, currentId: null, onPick, onDismiss: noop } });
  await fireEvent.click(screen.getAllByRole('button')[1]);
  expect(onPick).toHaveBeenCalledWith(categories[0].id);
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — markup mirroring §4.9 / `.ts-cat-pop`. Each row: leading 10 px check column + sans 13/500 name. `data-cat-id` is for tests. On click: call `onPick(id)` then `onDismiss()`.

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Commit**

```bash
git add web/src/components/feeds/ReassignCategoryPopover.svelte \
        web/src/components/feeds/__tests__/ReassignCategoryPopover.test.ts
git commit -m "ReassignCategoryPopover: shared popover for per-row + bulk reassign"
```

---

### Task 8: `AddFeedDialog.svelte`

**Files:**
- Create: `web/src/components/feeds/AddFeedDialog.svelte`
- Create: `web/src/components/feeds/__tests__/AddFeedDialog.test.ts`

This **replaces** the current sidebar `AddFeedForm.svelte`. The brand spec is explicit: add-feed lives on `/feeds`, opened from the toolbar's primary Add button (or from the empty-state CTA when there are zero feeds).

Props:

```ts
type Props = {
  categories: Category[];
  onClose: () => void;
  onAdded: () => void;            // parent calls subscriptions.load() and closes
};
```

#### Task 8.1: URL + Look up button (discovery launch)

- [ ] **Step 1: Failing test**:

```ts
it('renders a mono URL input and a disabled Look up button', () => {
  render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
  expect(screen.getByPlaceholderText(/example.com/i)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /look up/i })).toBeDisabled();
});

it('Look up enables once a non-blank URL is typed', async () => {
  render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
  await fireEvent.input(screen.getByPlaceholderText(/example.com/i), { target: { value: 'https://jvns.ca' } });
  expect(screen.getByRole('button', { name: /look up/i })).not.toBeDisabled();
});

it('calls api.discoverFeeds with the trimmed URL on Look up', async () => {
  vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [] });
  render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
  await fireEvent.input(screen.getByPlaceholderText(/example.com/i), { target: { value: '  https://jvns.ca ' } });
  await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
  expect(api.discoverFeeds).toHaveBeenCalledWith('https://jvns.ca');
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — open inside an M1 `Dialog` with `wide={true}`. Body uses `.ts-feeds-form` and `.ts-feeds-form-field-row`. URL input is `.ts-feeds-form-input` (mono per CSS line 4061). Submit button is `.ts-feeds-form-go` (or M1 `Button variant="primary"`).

- [ ] **Step 4: GREEN**

#### Task 8.2: Discovery results — 0, 1, N branches

- [ ] **Step 1: Failing test**:

```ts
it('zero candidates: shows "No feeds found" inline', async () => {
  vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [] });
  render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
  await fireEvent.input(screen.getByPlaceholderText(/example.com/i), { target: { value: 'https://nothing/' } });
  await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
  expect(await screen.findByText(/no feeds found/i)).toBeInTheDocument();
});

it('one candidate: pre-picks it', async () => {
  vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [{ title: 'Solo', feed_url: 'https://solo/feed.xml', site_url: 'https://solo', type: 'atom' }] });
  const { container } = render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
  await fireEvent.input(screen.getByPlaceholderText(/example.com/i), { target: { value: 'https://solo' } });
  await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
  await waitFor(() => {
    expect(container.querySelector('.ts-feeds-disc-row.is-picked')).not.toBeNull();
  });
});

it('multiple candidates: lets the user pick one with a radio row', async () => {
  vi.mocked(api.discoverFeeds).mockResolvedValue({
    candidates: [
      { title: 'main', feed_url: 'https://rbtb/atom.xml',   site_url: '', type: 'atom' },
      { title: 'comments', feed_url: 'https://rbtb/comments.xml', site_url: '', type: 'rss' },
    ],
  });
  const { container } = render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
  await fireEvent.input(screen.getByPlaceholderText(/example.com/i), { target: { value: 'https://rbtb' } });
  await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
  const rows = await screen.findAllByText(/main|comments/);
  await fireEvent.click(rows[1].closest('.ts-feeds-disc-row')!);
  const picked = container.querySelector('.ts-feeds-disc-row.is-picked')!;
  expect(picked.textContent).toMatch(/comments/);
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — render `.ts-feeds-disc` block with one `.ts-feeds-disc-head` line and one `.ts-feeds-disc-row` per candidate. Pre-select index 0 when `candidates.length === 1`. Each row's `.ts-feeds-disc-tag` shows the type (rss/atom/json — uppercase, mono).

- [ ] **Step 4: GREEN**

#### Task 8.3: Category assignment

- [ ] **Step 1: Failing test**:

```ts
it('renders Uncategorised + a button for each category', async () => {
  vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [{ …c… }] });
  render(AddFeedDialog, { props: { categories: [{ id: 3, name: 'People', unread: 0, created_at: 0 }], onClose: noop, onAdded: noop } });
  await fireEvent.input(screen.getByPlaceholderText(/example.com/i), { target: { value: 'https://x' } });
  await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
  await waitFor(() => {
    expect(screen.getByRole('button', { name: /uncategorised/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /people/i })).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Implement** — use `.ts-feeds-edit-cat-list` with `.ts-feeds-edit-cat-btn` per category. Default selected: `null` (Uncategorised). Clicking flips the `is-active` class.

#### Task 8.4: Subscribe

- [ ] **Step 1: Failing test**:

```ts
it('Subscribe calls api.addSubscription with feed_url, then api.updateSubscription if a category is picked, then onAdded', async () => {
  vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [{ title: 'X', feed_url: 'https://x/feed.xml', site_url: '', type: 'rss' }] });
  vi.mocked(api.addSubscription).mockResolvedValue({ id: 99, …subFields });
  vi.mocked(api.updateSubscription).mockResolvedValue(undefined);
  const onAdded = vi.fn();
  render(AddFeedDialog, { props: { categories: [{ id: 3, name: 'People', unread: 0, created_at: 0 }], onClose: noop, onAdded } });
  await fireEvent.input(screen.getByPlaceholderText(/example.com/i), { target: { value: 'https://x' } });
  await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
  await fireEvent.click(screen.getByRole('button', { name: /people/i }));
  await fireEvent.click(screen.getByRole('button', { name: /^subscribe$/i }));
  expect(api.addSubscription).toHaveBeenCalledWith({ feed_url: 'https://x/feed.xml' });
  expect(api.updateSubscription).toHaveBeenCalledWith(99, { category_id: 3 });
  expect(onAdded).toHaveBeenCalledOnce();
});

it('Subscribe surfaces api.addSubscription errors inline', async () => {
  vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [{ title: 'X', feed_url: 'https://x/feed.xml', site_url: '', type: 'rss' }] });
  vi.mocked(api.addSubscription).mockRejectedValue(new Error('feed already subscribed'));
  render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
  await fireEvent.input(screen.getByPlaceholderText(/example.com/i), { target: { value: 'https://x' } });
  await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
  await fireEvent.click(screen.getByRole('button', { name: /^subscribe$/i }));
  expect(await screen.findByText(/feed already subscribed/i)).toBeInTheDocument();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — Submit handler:

```ts
async function subscribe() {
  if (!picked) return;
  busy = true;
  error = null;
  try {
    const sub = await api.addSubscription({ feed_url: picked.feed_url });
    if (selectedCategory !== null) {
      await api.updateSubscription(sub.id, { category_id: selectedCategory });
    }
    onAdded();
  } catch (e) {
    error = (e as Error).message;
  } finally {
    busy = false;
  }
}
```

(Why two calls instead of one? `POST /api/v1/subscriptions` does accept `category_id` per M9 — see `internal/api/subscriptions.go`. So this can collapse to one call. **Implementer's choice**: if extending the POST body in this dialog is easier than chaining, do so. Update the test to match.)

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Commit**

```bash
git add web/src/components/feeds/AddFeedDialog.svelte \
        web/src/components/feeds/__tests__/AddFeedDialog.test.ts
git commit -m "AddFeedDialog: discovery + category pick + subscribe with error surfacing"
```

---

### Task 9: `EditFeedDialog.svelte` (absorbs FeedSettingsModal)

**Files:**
- Create: `web/src/components/feeds/EditFeedDialog.svelte`
- Create: `web/src/components/feeds/__tests__/EditFeedDialog.test.ts`
- Delete (if not already deleted by M1): `web/src/components/FeedSettingsModal.svelte`, `web/src/components/__tests__/FeedSettingsModal.test.ts`
- Delete (if not already deleted by M1): `web/src/components/AddFeedForm.svelte`, `web/src/components/__tests__/AddFeedForm.test.ts`
- Update call sites that still import either of the above (likely zero by this point; M1's deletion would have caught them).

Props:

```ts
type Props = {
  feed: Subscription;
  categories: Category[];
  onClose: () => void;
  onSaved: () => void;          // parent reloads + closes
  onDelete: () => void;         // parent opens DeleteFeedsDialog with [feed.id]
};
```

#### Task 9.1: Form chrome + Basics section

- [ ] **Step 1: Failing test**:

```ts
it('renders Title + Feed URL (readonly) + Category list', () => {
  render(EditFeedDialog, { props: { feed, categories, onClose: noop, onSaved: noop, onDelete: noop } });
  expect(screen.getByDisplayValue(feed.title)).toBeInTheDocument();
  expect((screen.getByDisplayValue(`https://${feed.feed_url}`) as HTMLInputElement).readOnly).toBe(true);
  expect(screen.getByRole('button', { name: /people/i })).toBeInTheDocument();
});
```

Note: the JSX renders `https://${feed.url}` with a leading scheme. In Tap's actual data shape, `feed_url` is already a full URL. The implementer should render `feed.feed_url` verbatim (no leading prefix). Adjust the test accordingly.

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — uses `.ts-feeds-edit-grid` two-column layout (CSS line 4154). Section headers via `.ts-feeds-edit-grid > .head`. Three sections: Basics / Article extraction / Credentials. Form bound to local `$state` (snapshot props on mount so cancel reverts).

- [ ] **Step 4: GREEN**

#### Task 9.2: Extraction toggle + selector

- [ ] **Step 1: Failing test**:

```ts
it('extract toggle starts in feed.extract state and flips on click', async () => {
  const { container } = render(EditFeedDialog, { props: { feed: { ...feed, extract: false }, categories, onClose: noop, onSaved: noop, onDelete: noop } });
  const toggle = container.querySelector('.ts-feeds-edit-toggle')!;
  expect(toggle.classList.contains('is-on')).toBe(false);
  await fireEvent.click(toggle);
  expect(toggle.classList.contains('is-on')).toBe(true);
});

it('selector input is disabled when extract is off', async () => {
  const { container } = render(EditFeedDialog, { props: { feed: { ...feed, extract: false }, categories, onClose: noop, onSaved: noop, onDelete: noop } });
  const selectorInput = container.querySelector('input[placeholder*="article"]') as HTMLInputElement;
  expect(selectorInput.disabled).toBe(true);
  await fireEvent.click(container.querySelector('.ts-feeds-edit-toggle')!);
  expect(selectorInput.disabled).toBe(false);
});
```

- [ ] **Step 2: Implement** — match `.ts-feeds-edit-toggle` markup (CSS line 4190; SW span + label).

- [ ] **Step 3: GREEN**

#### Task 9.3: Credentials (cookie + basic auth) — write-only

- [ ] **Step 1: Failing test**:

```ts
it('cookie field is empty on open even if has_cookie is true (server never returns secrets)', () => {
  render(EditFeedDialog, { props: { feed: { ...feed, has_cookie: true }, categories, onClose: noop, onSaved: noop, onDelete: noop } });
  // Textarea is present, value empty
  const ta = screen.getByPlaceholderText(/session=|cookie/i) as HTMLTextAreaElement;
  expect(ta.value).toBe('');
});

it('on Save, omits unchanged cookie/basic_auth but sends ones that were typed', async () => {
  vi.mocked(api.updateSubscription).mockResolvedValue(undefined);
  render(EditFeedDialog, { props: { feed: { ...feed }, categories, onClose: noop, onSaved: noop, onDelete: noop } });
  await fireEvent.input(screen.getByPlaceholderText(/session=|cookie/i), { target: { value: 'session=abc' } });
  await fireEvent.click(screen.getByRole('button', { name: /save changes/i }));
  expect(api.updateSubscription).toHaveBeenCalledWith(feed.id, expect.objectContaining({ cookie: 'session=abc' }));
  expect(api.updateSubscription).not.toHaveBeenCalledWith(feed.id, expect.objectContaining({ basic_auth_user: expect.anything() }));
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — mirror the existing `FeedSettingsModal.save` logic: only include `cookie`, `basic_auth_user`, `basic_auth_pass` in the PATCH when their state is non-empty. Always include `extract`, `extract_selector`. Include `category_id` only when changed (track via `originalCategoryId`).

- [ ] **Step 4: GREEN**

#### Task 9.4: Save + Delete + Cancel handlers

- [ ] **Step 1: Failing test**:

```ts
it('Save calls api.updateSubscription and onSaved', async () => {
  vi.mocked(api.updateSubscription).mockResolvedValue(undefined);
  const onSaved = vi.fn();
  render(EditFeedDialog, { props: { feed, categories, onClose: noop, onSaved, onDelete: noop } });
  await fireEvent.click(screen.getByRole('button', { name: /save changes/i }));
  expect(api.updateSubscription).toHaveBeenCalled();
  expect(onSaved).toHaveBeenCalledOnce();
});

it('Save surfaces errors inline; onSaved is not called', async () => {
  vi.mocked(api.updateSubscription).mockRejectedValue(new Error('csrf invalid'));
  const onSaved = vi.fn();
  render(EditFeedDialog, { props: { feed, categories, onClose: noop, onSaved, onDelete: noop } });
  await fireEvent.click(screen.getByRole('button', { name: /save changes/i }));
  expect(await screen.findByText(/csrf invalid/i)).toBeInTheDocument();
  expect(onSaved).not.toHaveBeenCalled();
});

it('Delete this feed button calls onDelete', async () => {
  const onDelete = vi.fn();
  render(EditFeedDialog, { props: { feed, categories, onClose: noop, onSaved: noop, onDelete } });
  await fireEvent.click(screen.getByRole('button', { name: /delete this feed/i }));
  expect(onDelete).toHaveBeenCalledOnce();
});

it('Cancel calls onClose', async () => {
  const onClose = vi.fn();
  render(EditFeedDialog, { props: { feed, categories, onClose, onSaved: noop, onDelete: noop } });
  await fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }));
  expect(onClose).toHaveBeenCalledOnce();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** Footer: `.ts-dialog-foot` with `Delete this feed` on the left (danger; bottom-bordered like `tap-feeds-page.jsx` line 540), `Cancel` and `Save changes` on the right.

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Delete obsolete files**:

```bash
git rm web/src/components/AddFeedForm.svelte \
       web/src/components/__tests__/AddFeedForm.test.ts \
       web/src/components/FeedSettingsModal.svelte \
       web/src/components/__tests__/FeedSettingsModal.test.ts
```

If any of these are already gone (M1 deleted them), skip those lines without erroring.

- [ ] **Step 6: Commit**

```bash
git add web/src/components/feeds/EditFeedDialog.svelte \
        web/src/components/feeds/__tests__/EditFeedDialog.test.ts
git commit -m "EditFeedDialog: form for title/category/extract/credentials; absorbs FeedSettingsModal"
```

---

### Task 10: `DeleteFeedsDialog.svelte`

**Files:**
- Create: `web/src/components/feeds/DeleteFeedsDialog.svelte`
- Create: `web/src/components/feeds/__tests__/DeleteFeedsDialog.test.ts`

Props:

```ts
type Props = {
  feeds: Subscription[];     // length >= 1
  onClose: () => void;
  onConfirm: () => Promise<void>;
};
```

- [ ] **Step 1: Failing test**:

```ts
it('single feed: title reads `Delete "<feed.title>"?`', () => {
  render(DeleteFeedsDialog, { props: { feeds: [feed], onClose: noop, onConfirm: async () => {} } });
  expect(screen.getByText(`Delete "${feed.title}"?`)).toBeInTheDocument();
});

it('bulk: title reads `Delete N feeds?`', () => {
  render(DeleteFeedsDialog, { props: { feeds: [feed, feed2, feed3], onClose: noop, onConfirm: async () => {} } });
  expect(screen.getByText(/delete 3 feeds\?/i)).toBeInTheDocument();
});

it('shows up to 5 feed rows; "…and N more" when more', () => {
  render(DeleteFeedsDialog, { props: { feeds: bigArray.slice(0, 7), onClose: noop, onConfirm: async () => {} } });
  expect(screen.getByText(/and 2 more/i)).toBeInTheDocument();
});

it('Confirm calls onConfirm; Cancel calls onClose', async () => {
  const onConfirm = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  render(DeleteFeedsDialog, { props: { feeds: [feed], onClose, onConfirm } });
  await fireEvent.click(screen.getByRole('button', { name: /^delete feed$/i }));
  await fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }));
  expect(onConfirm).toHaveBeenCalledOnce();
  expect(onClose).toHaveBeenCalledOnce();
});

it('Confirm button shows is-busy during in-flight onConfirm and re-enables after rejection', async () => {
  let rej!: (e: Error) => void;
  const onConfirm = vi.fn(() => new Promise<void>((_, r) => { rej = r; }));
  render(DeleteFeedsDialog, { props: { feeds: [feed], onClose: noop, onConfirm } });
  const confirm = screen.getByRole('button', { name: /^delete feed$/i });
  await fireEvent.click(confirm);
  expect(confirm).toBeDisabled();
  rej(new Error('boom'));
  await waitFor(() => expect(confirm).not.toBeDisabled());
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — markup mirrors `tap-feeds-page.jsx` `TFDeleteDialog` (lines 557–601). Body uses `<ul class="ts-dialog-list">` with up to 5 `<li class="ts-dialog-list-item">` each carrying `<FeedAvatar>` + title + URL. Foot uses M1 `Dialog` footer pattern. Confirm is danger variant.

Note: this dialog **does not** delete the feeds itself — the caller does, sequencing the N×1 calls with partial failure handling. This dialog only emits `onConfirm`.

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Commit**

```bash
git add web/src/components/feeds/DeleteFeedsDialog.svelte \
        web/src/components/feeds/__tests__/DeleteFeedsDialog.test.ts
git commit -m "DeleteFeedsDialog: single + bulk confirm with avatar list"
```

---

### Task 11: `ImportOpmlDialog.svelte` and `ExportOpmlDialog.svelte`

#### Task 11.1: ImportOpmlDialog

**Files:**
- Create: `web/src/components/feeds/ImportOpmlDialog.svelte`
- Create: `web/src/components/feeds/__tests__/ImportOpmlDialog.test.ts`

Stages: `drop` (initial), `uploading`, `done` (with result summary), `error`.

Props:

```ts
type Props = {
  onClose: () => void;
  onImported: () => void;     // parent reloads subscriptions and categories
};
```

- [ ] **Step 1: Failing test**:

```ts
it('drop stage: renders a drop zone + browse link', () => {
  render(ImportOpmlDialog, { props: { onClose: noop, onImported: noop } });
  expect(screen.getByText(/drop an \.opml file/i)).toBeInTheDocument();
  expect(screen.getByText(/browse/i)).toBeInTheDocument();
});

it('selecting a file calls api.importOPML with the file bytes', async () => {
  const buf = new TextEncoder().encode('<opml/>').buffer;
  const result: OPMLImportResult = { imported: 5, skipped: 1, errors: [] };
  vi.mocked(api.importOPML).mockResolvedValue(result);
  const { container } = render(ImportOpmlDialog, { props: { onClose: noop, onImported: noop } });
  const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
  const file = new File([buf], 'subs.opml', { type: 'text/x-opml' });
  await fireEvent.change(fileInput, { target: { files: [file] } });
  await waitFor(() => expect(api.importOPML).toHaveBeenCalled());
  expect(await screen.findByText(/5 imported.*1 skipped/i)).toBeInTheDocument();
});

it('drop stage: drag-and-drop a file triggers the upload path', async () => {
  vi.mocked(api.importOPML).mockResolvedValue({ imported: 3, skipped: 0, errors: [] });
  const { container } = render(ImportOpmlDialog, { props: { onClose: noop, onImported: noop } });
  const dropzone = container.querySelector('.ts-feeds-opml-drop')!;
  const file = new File([new TextEncoder().encode('<opml/>').buffer], 'subs.opml', { type: 'text/x-opml' });
  const data = new DataTransfer();
  data.items.add(file);
  await fireEvent.drop(dropzone, { dataTransfer: data });
  await waitFor(() => expect(api.importOPML).toHaveBeenCalled());
});

it('errors are surfaced in the result summary', async () => {
  vi.mocked(api.importOPML).mockResolvedValue({ imported: 1, skipped: 2, errors: ['bad url x', 'bad url y'] });
  const { container } = render(ImportOpmlDialog, { props: { onClose: noop, onImported: noop } });
  const file = new File([new TextEncoder().encode('<opml/>').buffer], 'subs.opml');
  await fireEvent.change(container.querySelector('input[type="file"]') as HTMLInputElement, { target: { files: [file] } });
  expect(await screen.findByText(/bad url x/i)).toBeInTheDocument();
});

it('Done dismiss calls onImported then onClose', async () => {
  vi.mocked(api.importOPML).mockResolvedValue({ imported: 2, skipped: 0, errors: [] });
  const onImported = vi.fn(); const onClose = vi.fn();
  const { container } = render(ImportOpmlDialog, { props: { onClose, onImported } });
  const file = new File([new TextEncoder().encode('<opml/>').buffer], 'subs.opml');
  await fireEvent.change(container.querySelector('input[type="file"]') as HTMLInputElement, { target: { files: [file] } });
  await screen.findByText(/2 imported/i);
  await fireEvent.click(screen.getByRole('button', { name: /done/i }));
  expect(onImported).toHaveBeenCalledOnce();
  expect(onClose).toHaveBeenCalledOnce();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — drop zone uses `.ts-feeds-opml-drop`. Hidden `<input type="file" accept=".opml,application/xml,text/x-opml">` triggered by `.link` click. `onchange` reads the first file with `FileReader.readAsArrayBuffer`, then calls `api.importOPML(buf)`. Drag-and-drop wires via `@attach` (svelte-template-directives) — prevent default on `dragover` + `drop`, then re-route through the same upload handler. On success transitions to `done` stage with the result summary (mono caps eyebrow + counts + error list when non-empty); footer button reads "Done" and calls `onImported(); onClose()`.

- [ ] **Step 4: GREEN**

#### Task 11.2: ExportOpmlDialog

**Files:**
- Create: `web/src/components/feeds/ExportOpmlDialog.svelte`
- Create: `web/src/components/feeds/__tests__/ExportOpmlDialog.test.ts`

Props:

```ts
type Props = {
  feedCount: number;
  categoryCount: number;
  onClose: () => void;
};
```

- [ ] **Step 1: Failing test**:

```ts
it('renders stats and a Download button', () => {
  render(ExportOpmlDialog, { props: { feedCount: 24, categoryCount: 4, onClose: noop } });
  expect(screen.getByText(/24/)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /download/i })).toBeInTheDocument();
});

it('Download calls api.exportOPML, builds a blob URL, triggers click, revokes URL', async () => {
  const blob = new Blob(['<opml/>'], { type: 'text/x-opml' });
  vi.mocked(api.exportOPML).mockResolvedValue(blob);
  const createObjectURL = vi.fn().mockReturnValue('blob:fake');
  const revokeObjectURL = vi.fn();
  vi.stubGlobal('URL', { ...URL, createObjectURL, revokeObjectURL });
  const onClose = vi.fn();
  render(ExportOpmlDialog, { props: { feedCount: 1, categoryCount: 1, onClose } });
  await fireEvent.click(screen.getByRole('button', { name: /download/i }));
  expect(api.exportOPML).toHaveBeenCalled();
  expect(createObjectURL).toHaveBeenCalledWith(blob);
  await waitFor(() => expect(revokeObjectURL).toHaveBeenCalledWith('blob:fake'));
  expect(onClose).toHaveBeenCalledOnce();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — render `.ts-feeds-opml-stats` block. Download handler:

```ts
async function download() {
  busy = true;
  try {
    const blob = await api.exportOPML();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'tap-subscriptions.opml';
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
    onClose();
  } catch (e) {
    error = (e as Error).message;
  } finally {
    busy = false;
  }
}
```

(Why a dialog and not a quiet download? Brand spec §6.5 lists Import OPML as a dialog. The export-as-direct-download is an option too; using a dialog matches the JSX export design (`TFExportDialog` lines 680–737) and makes "what file you're about to download" visible. The OPML preview from the JSX is optional and may be skipped — server-side OPML doesn't currently expose a preview endpoint. Render stats only.)

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Commit**

```bash
git add web/src/components/feeds/ImportOpmlDialog.svelte \
        web/src/components/feeds/ExportOpmlDialog.svelte \
        web/src/components/feeds/__tests__/ImportOpmlDialog.test.ts \
        web/src/components/feeds/__tests__/ExportOpmlDialog.test.ts
git commit -m "ImportOpmlDialog + ExportOpmlDialog"
```

---

### Task 12: `Feeds.svelte` view — wire everything together

**Files:**
- Replace (overwrite the M1 stub): `web/src/views/Feeds.svelte`
- Create: `web/src/views/__tests__/Feeds.test.ts`

Pulls in: the existing `subscriptions` + `categories` + `entries` stores, the new `feedsFilter` helpers, and each new dialog/popover/component.

Skeleton:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { subscriptions, categories, entries } from '../lib/store';
  import { filterFeeds, sortFeeds, countByKey, type FilterKey, type SortKey, type FeedRow } from '../lib/feedsFilter';
  import FeedsToolbar from '../components/feeds/FeedsToolbar.svelte';
  import FeedsBulkBar from '../components/feeds/FeedsBulkBar.svelte';
  import FeedsListRow from '../components/feeds/FeedsListRow.svelte';
  import AddFeedDialog from '../components/feeds/AddFeedDialog.svelte';
  import EditFeedDialog from '../components/feeds/EditFeedDialog.svelte';
  import DeleteFeedsDialog from '../components/feeds/DeleteFeedsDialog.svelte';
  import ImportOpmlDialog from '../components/feeds/ImportOpmlDialog.svelte';
  import ExportOpmlDialog from '../components/feeds/ExportOpmlDialog.svelte';
  import ReassignCategoryPopover from '../components/feeds/ReassignCategoryPopover.svelte';
  import EmptyState from '../components/EmptyState.svelte';

  let search = $state('');
  let filter = $state<FilterKey>('all');
  let sort = $state<SortKey>('recent');
  let selected = $state(new Set<number>());
  let expanded = $state(new Set<number>());
  let refreshing = $state(new Set<number>());
  let refreshingAll = $state(false);
  let dialog = $state<
    | null
    | { type: 'add' }
    | { type: 'edit'; feedId: number }
    | { type: 'delete'; feedIds: number[] }
    | { type: 'import' }
    | { type: 'export' }
    | { type: 'reassign'; feedIds: number[] }
  >(null);
  let foot = $state<string | null>(null); // ephemeral status string from bulk ops

  onMount(() => {
    subscriptions.load();
    categories.load();
  });

  // Decorate subscriptions with computed unread counts. Reads from a
  // store of unread counts per feed; the existing entriesStore doesn't
  // expose this directly. Compute from $entries.items by feed_id, or load
  // /api/v1/subscriptions which already returns per-feed counts (see types.ts).
  // For M5 v1 use a simple counter:
  const decorated: FeedRow[] = $derived.by(() => {
    const now = Math.floor(Date.now() / 1000);
    return $subscriptions.map((s) => ({
      ...s,
      unread: countUnreadFor(s.id),
      lastPollAgo: s.last_poll_at ? now - s.last_poll_at : Number.POSITIVE_INFINITY,
    }));
  });
  const counts = $derived(countByKey(decorated));
  const visible = $derived(sortFeeds(filterFeeds(decorated, filter, search), sort));
  const errorCount = $derived(counts.errors);

  function countUnreadFor(feedId: number): number {
    // Naive O(n) over entries; entries are already in memory. Replace with a
    // store-side memo if needed.
    let n = 0;
    for (const e of $entries.items) if (e.subscription_id === feedId && !e.read) n++;
    return n;
  }

  async function refreshOne(id: number) {
    refreshing.add(id);
    refreshing = new Set(refreshing);
    try {
      await subscriptions.refresh(id);
    } catch (e) {
      foot = `Refresh failed: ${(e as Error).message}`;
    } finally {
      refreshing.delete(id);
      refreshing = new Set(refreshing);
    }
  }

  async function refreshAll() {
    refreshingAll = true;
    const ids = visible.map((f) => f.id);
    const results = await Promise.allSettled(ids.map((id) => subscriptions.refresh(id)));
    const failed = results.filter((r) => r.status === 'rejected').length;
    refreshingAll = false;
    if (failed > 0) foot = `Refreshed ${ids.length - failed} of ${ids.length} · ${failed} failed`;
    else if (ids.length > 0) foot = `Refreshed ${ids.length} feed${ids.length === 1 ? '' : 's'}`;
  }

  async function bulkDelete(ids: number[]) {
    const results = await Promise.allSettled(ids.map((id) => subscriptions.remove(id)));
    const failed = results.filter((r) => r.status === 'rejected').length;
    selected = new Set();
    if (failed > 0) foot = `Removed ${ids.length - failed} of ${ids.length} · ${failed} failed`;
    dialog = null;
  }

  async function bulkRefresh(ids: number[]) {
    const results = await Promise.allSettled(ids.map((id) => subscriptions.refresh(id)));
    const failed = results.filter((r) => r.status === 'rejected').length;
    if (failed > 0) foot = `Refreshed ${ids.length - failed} of ${ids.length} · ${failed} failed`;
  }

  async function bulkReassign(ids: number[], categoryId: number | null) {
    const results = await Promise.allSettled(ids.map((id) => subscriptions.setCategory(id, categoryId)));
    const failed = results.filter((r) => r.status === 'rejected').length;
    selected = new Set();
    if (failed > 0) foot = `Moved ${ids.length - failed} of ${ids.length} · ${failed} failed`;
    dialog = null;
  }
</script>

<header class="ts-set-head">
  <div class="ts-set-eyebrow">
    Subscriptions · {decorated.length} feeds · {$categories.length} categories
    {#if errorCount > 0}<span class="err"> · {errorCount} with errors</span>{/if}
  </div>
  <h1 class="ts-set-title">Feeds</h1>
</header>

<div class="ts-feeds">
  <FeedsToolbar
    {search} {filter} {sort} {counts} {refreshingAll}
    onSearch={(q) => { search = q; }}
    onFilter={(k) => { filter = k; }}
    onSort={(s) => { sort = s; }}
    onAdd={() => { dialog = { type: 'add' }; }}
    onRefreshAll={refreshAll}
    onImport={() => { dialog = { type: 'import' }; }}
    onExport={() => { dialog = { type: 'export' }; }}
  />

  {#if selected.size > 0}
    <FeedsBulkBar
      count={selected.size}
      onClear={() => { selected = new Set(); }}
      onRefresh={() => bulkRefresh([...selected])}
      onReassign={() => { dialog = { type: 'reassign', feedIds: [...selected] }; }}
      onDelete={() => { dialog = { type: 'delete', feedIds: [...selected] }; }}
    />
  {/if}

  <div class="ts-feeds-list">
    {#if visible.length === 0 && decorated.length === 0}
      <EmptyState title="No feeds yet" subtitle="Add a feed by URL, or import an OPML export from another reader." cta="Add a feed" onCta={() => { dialog = { type: 'add' }; }} />
    {:else if visible.length === 0}
      <EmptyState title="No feeds match" subtitle="Nothing matches the current filter." cta="Reset filters" onCta={() => { search = ''; filter = 'all'; }} />
    {:else}
      {#each visible as feed (feed.id)}
        <FeedsListRow
          {feed}
          categoryName={feed.category_id ? $categories.find((c) => c.id === feed.category_id)?.name ?? null : null}
          isSelected={selected.has(feed.id)}
          anySelected={selected.size > 0}
          isExpanded={expanded.has(feed.id)}
          isRefreshing={refreshing.has(feed.id) || refreshingAll}
          onToggleSelect={() => { selected.has(feed.id) ? selected.delete(feed.id) : selected.add(feed.id); selected = new Set(selected); }}
          onToggleExpand={() => { expanded.has(feed.id) ? expanded.delete(feed.id) : expanded.add(feed.id); expanded = new Set(expanded); }}
          onRefresh={() => refreshOne(feed.id)}
          onChangeCategory={() => { dialog = { type: 'reassign', feedIds: [feed.id] }; }}
          onEdit={() => { dialog = { type: 'edit', feedId: feed.id }; }}
          onDelete={() => { dialog = { type: 'delete', feedIds: [feed.id] }; }}
        />
      {/each}
    {/if}
  </div>

  <div class="ts-feeds-foot" role="status" aria-live="polite">
    <span class="pulse" aria-hidden="true"></span>
    {#if foot}{foot}{:else}Auto-polling{/if}
    <span class="sep">·</span>
    {decorated.length} feeds across {$categories.length} categories
    {#if errorCount > 0}
      <span class="sep">·</span><span class="err">{errorCount} need attention</span>
    {/if}
  </div>
</div>

{#if dialog?.type === 'add'}
  <AddFeedDialog categories={$categories} onClose={() => { dialog = null; }} onAdded={() => { dialog = null; void subscriptions.load(); }} />
{/if}
{#if dialog?.type === 'edit'}
  {@const f = $subscriptions.find((x) => x.id === dialog.feedId)}
  {#if f}
    <EditFeedDialog feed={f} categories={$categories} onClose={() => { dialog = null; }} onSaved={() => { dialog = null; void subscriptions.load(); }} onDelete={() => { dialog = { type: 'delete', feedIds: [dialog.feedId] }; }} />
  {/if}
{/if}
{#if dialog?.type === 'delete'}
  {@const fs = $subscriptions.filter((x) => dialog.feedIds.includes(x.id))}
  <DeleteFeedsDialog feeds={fs} onClose={() => { dialog = null; }} onConfirm={() => bulkDelete(dialog.feedIds)} />
{/if}
{#if dialog?.type === 'import'}
  <ImportOpmlDialog onClose={() => { dialog = null; }} onImported={() => { void subscriptions.load(); void categories.load(); }} />
{/if}
{#if dialog?.type === 'export'}
  <ExportOpmlDialog feedCount={decorated.length} categoryCount={$categories.length} onClose={() => { dialog = null; }} />
{/if}
{#if dialog?.type === 'reassign'}
  <ReassignCategoryPopover
    categories={$categories}
    currentId={dialog.feedIds.length === 1 ? $subscriptions.find((x) => x.id === dialog.feedIds[0])?.category_id ?? null : null}
    onPick={(catId) => bulkReassign(dialog.feedIds, catId)}
    onDismiss={() => { dialog = null; }}
  />
{/if}
```

#### Task 12.1: View test — renders + dispatches dialog opens

- [ ] **Step 1: Failing test** — `Feeds.test.ts`:

```ts
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import Feeds from '../Feeds.svelte';
import { subscriptions, categories } from '../../lib/store';

vi.mock('../../lib/store', () => {
  const _subs = writable([
    { id: 1, title: 'Julia', feed_url: 'jvns.ca/feed', error_count: 0, /* … */ },
    { id: 2, title: 'Dan',   feed_url: 'danluu.com/feed', error_count: 2, last_error: 'http 500', /* … */ },
  ]);
  return {
    subscriptions: { subscribe: _subs.subscribe, load: vi.fn(), refresh: vi.fn(), remove: vi.fn(), setCategory: vi.fn() },
    categories:    { subscribe: writable([{ id: 3, name: 'People', unread: 0, created_at: 0 }]).subscribe, load: vi.fn() },
    entries:       { subscribe: writable({ items: [] }).subscribe },
  };
});

it('renders the page head, toolbar, and one row per subscription', () => {
  render(Feeds);
  expect(screen.getByText(/feeds/i)).toBeInTheDocument();
  expect(screen.getByPlaceholderText(/search feeds/i)).toBeInTheDocument();
  expect(screen.getByText('Julia')).toBeInTheDocument();
  expect(screen.getByText('Dan')).toBeInTheDocument();
});

it('clicking Add feed opens the AddFeedDialog', async () => {
  render(Feeds);
  await fireEvent.click(screen.getByRole('button', { name: /add feed/i }));
  // Dialog title
  expect(await screen.findByText(/add a feed/i)).toBeInTheDocument();
});

it('typing in search filters the list', async () => {
  render(Feeds);
  await fireEvent.input(screen.getByPlaceholderText(/search feeds/i), { target: { value: 'julia' } });
  expect(screen.getByText('Julia')).toBeInTheDocument();
  expect(screen.queryByText('Dan')).not.toBeInTheDocument();
});

it('clicking the Errors chip filters to feeds with errors', async () => {
  render(Feeds);
  await fireEvent.click(screen.getByRole('button', { name: /errors/i }));
  expect(screen.queryByText('Julia')).not.toBeInTheDocument();
  expect(screen.getByText('Dan')).toBeInTheDocument();
});

it('selecting a row shows the bulk bar', async () => {
  const { container } = render(Feeds);
  await fireEvent.click(container.querySelector('.ts-feed-check')!);
  expect(screen.getByText(/1 selected/i)).toBeInTheDocument();
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** the view per the skeleton above.

- [ ] **Step 4: GREEN**

#### Task 12.2: View test — bulk delete with partial failure

- [ ] **Step 1: Failing test**:

```ts
it('bulk delete: succeeds for 2, fails for 1; foot reads "Removed 2 of 3 · 1 failed"', async () => {
  // Setup: select 3 rows, mock subscriptions.remove to reject on the second id.
  vi.mocked(subscriptions.remove).mockImplementation(async (id: number) => {
    if (id === 2) throw new Error('csrf invalid');
  });
  const { container } = render(Feeds);
  // Select all three (rows 1, 2, 3)
  for (const cb of container.querySelectorAll<HTMLButtonElement>('.ts-feed-check')) await fireEvent.click(cb);
  await fireEvent.click(screen.getByRole('button', { name: /^delete$/i })); // bulk bar Delete
  await fireEvent.click(screen.getByRole('button', { name: /^delete 3 feeds$/i }));
  await waitFor(() => expect(screen.getByText(/removed 2 of 3.*1 failed/i)).toBeInTheDocument());
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** is already covered by `bulkDelete` in the skeleton. Verify the partial-failure message format matches what the test expects.

- [ ] **Step 4: GREEN**

#### Task 12.3: View test — refresh-all flow

- [ ] **Step 1: Failing test**:

```ts
it('Refresh all calls subscriptions.refresh for every visible feed', async () => {
  render(Feeds);
  await fireEvent.click(screen.getByRole('button', { name: /refresh all/i }));
  expect(vi.mocked(subscriptions.refresh)).toHaveBeenCalledTimes(2); // 2 mock feeds
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — covered by `refreshAll`.

- [ ] **Step 4: GREEN**

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Feeds.svelte web/src/views/__tests__/Feeds.test.ts
git commit -m "Feeds.svelte: wire toolbar/bulkbar/list/dialogs into one view"
```

---

### Task 13: Mobile branch

**Files:**
- Modify: `web/src/views/Feeds.svelte`
- Modify: `web/src/components/feeds/FeedsListRow.svelte` (add `density="mobile"` class branch)
- Modify: `web/src/components/feeds/FeedsBulkBar.svelte` (add `.is-mobile` CSS)

Per the umbrella spec each milestone ships desktop **and** mobile. The mobile branch is reached when the app's `isMobile` shell state is true (foundations milestone wires this).

#### Task 13.1: Mobile FAB

- [ ] **Step 1: Failing test**:

```ts
it('mobile: renders an Add FAB in place of the toolbar Add button', () => {
  // Mock the shell's isMobile state to true
  …
  render(Feeds, { props: {} });
  expect(screen.getByRole('button', { name: /add feed/i })).toHaveClass('ts-mobile-feeds-fab');
});
```

- [ ] **Step 2: RED**

- [ ] **Step 3: Implement** — within `Feeds.svelte`, when the shell exposes `isMobile === true` (read from M1's app store; if not available, expose a `isMobile` prop that the AppShell sets), render an additional FAB at `.ts-mobile-feeds-fab` that calls the same `dialog = { type: 'add' }`. The toolbar still renders but row 1's Add button can hide on mobile (CSS: `display: none` inside `.is-mobile`).

- [ ] **Step 4: GREEN**

#### Task 13.2: Mobile bulk bar — drawer variant

- [ ] **Step 1: Failing test** — assert that on mobile the bulk bar gets `.is-mobile` and its layout puts each action on its own row.

- [ ] **Step 2: Implement** — add a CSS-only branch in `FeedsBulkBar.svelte`:

```css
.ts-feeds-bulk.is-mobile {
  position: fixed;
  left: 12px;
  right: 12px;
  bottom: calc(env(safe-area-inset-bottom, 0) + 12px);
  flex-direction: column;
  align-items: stretch;
  gap: 6px;
  padding: 12px;
}
.ts-feeds-bulk.is-mobile .ts-feeds-bulk-spacer { display: none; }
```

The caller passes `mobile={isMobile}` and the component toggles the class.

- [ ] **Step 3: GREEN**

- [ ] **Step 4: Commit**

```bash
git add web/src/views/Feeds.svelte web/src/components/feeds/FeedsListRow.svelte web/src/components/feeds/FeedsBulkBar.svelte web/src/views/__tests__/Feeds.test.ts
git commit -m "Feeds: mobile FAB + drawer-style bulk bar"
```

---

### Task 14: CSS port — global stylesheet additions

**Files:**
- Modify: `web/src/styles/global.css` (foundations-owned; add only the keyframe declaration if M1 hasn't already)
- Each new component carries its own scoped `<style>` block translating the corresponding `ui_design/styles.css` selectors.

Per umbrella §4, no Svelte file imports `ui_design/styles.css`. Each component owns the slice of CSS it needs. The mapping:

| Component | Owns (selectors from `ui_design/styles.css`) | Lines |
|---|---|---|
| `Feeds.svelte` | `.ts-feeds`, `.ts-feeds-list`, `.ts-feeds-foot`, `.ts-feeds-empty*`, `.ts-set-head`, `.ts-set-eyebrow`, `.ts-set-title` (cross-cutting head) | 3554–3556, 3771–3772, 4000–4036 |
| `FeedsToolbar.svelte` | `.ts-feeds-top`, `.ts-feeds-search*`, `.ts-feeds-add`, `.ts-feeds-row2`, `.ts-feeds-chip*`, `.ts-feeds-sep`, `.ts-feeds-sort*`, `.ts-feeds-spacer`, `.ts-feeds-util-btn*` | 3558–3702 |
| `FeedsBulkBar.svelte` | `.ts-feeds-bulk*` | 3706–3751 |
| `FeedsListRow.svelte` | `.ts-feed-row`, `.ts-feed-check*`, `.ts-feed-avatar-wrap`, `.ts-feed-body`, `.ts-feed-line1`, `.ts-feed-line2`, `.ts-feed-name`, `.ts-feed-cat*`, `.ts-feed-err-chip*`, `.ts-feed-backoff-chip`, `.ts-feed-url*`, `.ts-feed-actions`, `.ts-feed-act*`, `.ts-feeds-group*` (when grouped); mobile variants `.ts-mobile-feed-row*`, `.ts-mobile-feeds-fab` | 3754–3943, 4336–4362 |
| `FeedsHealthPanel.svelte` | `.ts-feed-health*` | 3946–3997 |
| `AddFeedDialog.svelte`, `EditFeedDialog.svelte`, `DeleteFeedsDialog.svelte` | `.ts-dialog.is-wide`, `.ts-dialog.is-edit`, `.ts-feeds-form*`, `.ts-feeds-disc*`, `.ts-feeds-edit-grid`, `.ts-feeds-edit-toggle*`, `.ts-feeds-edit-cat-list`, `.ts-feeds-edit-cat-btn*`, `.ts-feeds-kv-pair` | 4038–4244 |
| `ImportOpmlDialog.svelte` | `.ts-feeds-opml-drop*`, `.ts-feeds-opml-stats*`, `.ts-feeds-opml-preview*` | 4247–4308 |
| `ExportOpmlDialog.svelte` | same set as Import (re-uses `.ts-feeds-opml-stats`) | 4247–4267 |

The keyframe `@keyframes tf-spin` lives in `web/src/styles/global.css` (M1 owns it). If M1 forgot it, add it as part of Task 14:

```css
@keyframes tf-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) {
  .is-spinning svg { animation: none !important; }
}
```

- [ ] **Step 1: Port CSS for each component**. No tests — pure markup/styling. Browser-verify each in Task 16.

- [ ] **Step 2: Run svelte-check** — `pnpm --dir web run check`. Expect 0 errors.

- [ ] **Step 3: Run all unit tests** — `pnpm --dir web test`. Expect 0 failures.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/Feeds.svelte web/src/components/feeds/*.svelte web/src/styles/global.css
git commit -m "CSS port: scoped <style> per component for the Feeds page"
```

---

### Task 15: Service worker invalidation + warm-cache

**Files:**
- Modify: `web/src/lib/warmCache.ts` (if it pre-warms `/api/v1/subscriptions` it's already fine; otherwise no change)
- Verify: every mutation in `subscriptions.refresh/remove/setCategory` calls `notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions'] })`. Bulk operations issue one `notifySW` per mutation, which is fine: the SW dedupes by path.

- [ ] **Step 1: Test** — Playwright smoke test (Task 16) catches stale cache. No unit test needed here.

---

### Task 16: Playwright smoke checklist

**Tooling:** Playwright MCP.

Run the dev server before each session: `make dev` (Go on :8080, Vite on :5173). Sign in as the seeded admin (or any test user). Then for each of the following, drive Playwright to assert UI is correct:

- [ ] **Step 1: Navigate** — `mcp__plugin_playwright_playwright__browser_navigate` to `http://localhost:5173/feeds`. Wait for the page head. Snapshot the toolbar.

- [ ] **Step 2: Add a feed** — click Add feed, fill `https://jvns.ca`, Look up, assert at least one candidate appears, click the first row, Subscribe. Wait. Assert the new feed appears in the list.

- [ ] **Step 3: Per-row refresh** — click the refresh icon on one row, assert `.is-spinning` for ≥ 1 second, then assert the icon stops spinning when the next subscriptions load resolves.

- [ ] **Step 4: Per-row category change** — click the category chip, popover opens with category list, pick a different one, popover closes, the chip's text updates.

- [ ] **Step 5: Filter chip Errors** — assert the chip count matches the number of `.has-error` rows. Click. Assert visible rows is filtered.

- [ ] **Step 6: Sort** — pick Most unread. Assert the topmost row has the highest unread count.

- [ ] **Step 7: Search** — type into search. Assert one row remains.

- [ ] **Step 8: Bulk select + Reassign** — select 2 rows. Bulk bar appears. Click Set category, pick a category. Both rows update.

- [ ] **Step 9: Bulk delete** — select 2 rows. Click Delete. Dialog shows 2 feeds. Click Delete 2 feeds. Both disappear; foot reads "Removed 2 of 2".

- [ ] **Step 10: Import OPML** — open Import OPML. Upload a small fixture file (provide `web/tests/fixtures/sample.opml`). Watch the dialog flip to a result summary. Click Done. New feeds appear.

- [ ] **Step 11: Export OPML** — open Export. Click Download. Browser triggers a download with filename `tap-subscriptions.opml`. (Playwright's downloads handler captures this.)

- [ ] **Step 12: Edit feed** — open the more-menu on a row, click Edit. EditFeedDialog renders. Toggle extract off then on. Save. The PATCH lands.

- [ ] **Step 13: Health panel expansion** — find a row with errors, click "Why?", panel expands with the 3-cell grid. Click Hide details.

- [ ] **Step 14: Mobile** — resize viewport to 400×800. Assert the toolbar wraps, the FAB appears, the bulk bar (when active) is a drawer.

- [ ] **Step 15: Three themes** — for light / dark / sepia, snapshot the page head + one row + one expanded health panel. Visually compare against `ui_design/Tap Desktop.html` / `ui_design/Tap Mobile.html` mental model.

- [ ] **Step 16: Empty states** — temporarily delete every feed. Assert "No feeds yet" empty state with Add CTA. Type a non-matching search, assert "No feeds match" with Reset CTA.

If any step fails: investigate, fix root cause, re-run that step. Do not paper over with timeouts.

---

### Task 17: Final verification + PR

- [ ] **Step 1: Run the full unit suite** — `pnpm --dir web test`. Zero failures.
- [ ] **Step 2: Run svelte-check** — `pnpm --dir web run check`. Zero errors.
- [ ] **Step 3: Run Go tests** — `make test`. Zero failures (catches the M5 backend change).
- [ ] **Step 4: Build** — `make build`. Binary at `bin/tap`.
- [ ] **Step 5: Run** — `./bin/tap --addr 127.0.0.1:8080`. Open `http://127.0.0.1:8080/feeds`. Repeat one round of Task 16 against the static binary (catches embed mistakes — M1 set this trap).
- [ ] **Step 6: Push + open PR** — branch `redesign/m5-feeds`. PR body: scope + screenshots of each theme + check-marks for the Task 16 list.

---

## Acceptance criteria

The page is "done" when **every** bullet below is true.

### Routing and shell
- `/feeds` navigates to the new `Feeds.svelte` (the M1 stub is gone).
- The desktop top tab "Feeds" lights up when on `/feeds`.
- The mobile bottom tab "Feeds" lights up when on `/feeds`.

### Page head
- Eyebrow line `.ts-set-eyebrow` reads `Subscriptions · N feeds · M categories` in mono caps.
- When `errorCount > 0`, the eyebrow appends ` · K with errors` in `#c43a3a` (light) / `#ec7a7a` (dark).
- Page title `.ts-set-title` reads "Feeds" in serif 38/600.

### Toolbar row 1
- Search input fills the row, mono 13/sans body per the brand spec, placeholder "Search feeds by name or URL…", icon at left, focus ring `var(--accent)` with `0 0 0 3px var(--accent-soft)`.
- Add feed button on the right is `.ts-feeds-add` (primary, dark background, white text). Clicking opens `AddFeedDialog`.

### Toolbar row 2
- Filter chips, in order: `All`, `Errors`, `Unread`, `Stale`. Each carries `<span class="ct">N</span>`.
- The active chip has `.is-active` (`--bg-soft` + hairline).
- The `Errors` chip carries `.is-warn` when its count > 0; if both `.is-warn` and `.is-active`, the warn palette overrides per CSS lines 3637–3647.
- Right side: sort `<select>` (mono caps), Refresh all (with `is-spinning` while polling), Import OPML, Export — all `.ts-feeds-util-btn`.

### Bulk bar
- Renders only when `selected.size > 0`.
- `.ts-feeds-bulk` strip (solid `--ink` / `--bg-soft` in dark) with `N selected`, `clear`, Refresh, Set category, Delete.
- Set category opens `ReassignCategoryPopover`.
- Delete opens `DeleteFeedsDialog` with the selected ids.
- After a bulk op, the selection clears and a status string appears in `.ts-feeds-foot`.

### Per-row anatomy
- Grid `.ts-feed-row`: 18 px checkbox · 18 px avatar · 1fr body · auto actions.
- Selected: row gets `.is-selected` + `--accent-soft` background + 2 px accent left strip via `::before`.
- Body line 1: serif 19/600 name, category chip (`.ts-feed-cat`; or `.ts-feed-cat.is-uncat` in italic when no category), optional `.ts-feed-err-chip` + `.ts-feed-backoff-chip`.
- Body line 2: mono URL with external-link icon, dot separators, mono `polled <ago> ago · next <ago> · cadence <X>` (no errors) or `last poll <ago> ago · Why?` (errors).
- Actions: unread count chip (`.ts-feed-act-unread`; `.is-zero` when 0), refresh icon (`.is-spinning` during refresh), more menu icon (opens Edit dialog).
- Hover: `--bg-soft` background.

### Expanded health panel
- Shown only when `isExpanded === true` and `feed.error_count > 0`.
- `.ts-feed-health` panel inside the row's grid (`grid-column: 3 / span 2`).
- 2 px danger left border (`#c43a3a` light, `#ec7a7a` dark).
- Eyebrow: "Last error" + first-seen relative time.
- Mono error string in `.ts-feed-health-msg`.
- 3-cell grid: Consecutive errors / Last try / Status (cadence).
- Two utility buttons: Retry now, Edit credentials.

### Add-feed dialog
- `.ts-dialog.is-wide` (640 px). Mounted via M1 `Dialog`. Focus trap + Esc close from the primitive.
- Single mono URL input (`.ts-feeds-form-input`), placeholder hints both raw feed URLs and site URLs.
- "Look up" button disables while empty, calls `api.discoverFeeds(url)`.
- 0 candidates → inline error "No feeds found at that URL."
- 1 candidate → it auto-picks; user can still see and confirm before Subscribe.
- N candidates → radio rows with name, mono URL, type tag.
- Category section: `Uncategorised` + each category as a chip-button.
- Subscribe button disabled until a candidate is selected.
- Errors from `api.addSubscription` surface inline; the dialog stays open.

### Edit-feed dialog
- `.ts-dialog.is-edit` (640 px).
- Three sections: Basics (Title / Feed URL readonly / Category) · Article extraction (toggle + selector input — selector disabled when extract is off) · Credentials (Cookie textarea + Basic auth username + password).
- Cookie and basic_auth fields are always empty on open (the server never returns secrets per CLAUDE.md).
- On save, only changed/non-empty fields are PATCHed.
- Delete this feed (danger button, left of foot) opens DeleteFeedsDialog with `[feed.id]`.

### Delete dialog
- Single: title `Delete "<name>"?`. Body explains saved-articles-stay-in-archive.
- Bulk: title `Delete N feeds?`. List shows up to 5 rows + "…and N more".
- Confirm is danger variant; the dialog stays open and shows the button as disabled while the bulk Promise.allSettled resolves.

### Import OPML dialog
- Drop zone `.ts-feeds-opml-drop` with browse link.
- File picker accepts `.opml` / `application/xml` / `text/x-opml`.
- Drag-and-drop also triggers upload.
- Stage transitions: drop → uploading → done.
- Done stage shows imported / skipped counts + per-error list when non-empty.
- "Done" button calls `onImported()` then `onClose()`.

### Export OPML dialog
- Stats: feed count + category count + (optionally) approximate file size.
- Download button calls `api.exportOPML()`, creates a transient `<a>` with the blob URL, clicks it, revokes the URL, closes the dialog.

### Empty states
- Zero feeds → `EmptyState` with serif title "No feeds yet", sans subtitle, primary "Add a feed" CTA and quiet "Import OPML" CTA.
- Filter/search returns zero → `EmptyState` "No feeds match" + "Reset filters" CTA.

### Foot strip
- Pulse dot + `Auto-polling` + ` · N feeds across M categories` + (when errorCount > 0) ` · K need attention` in danger colour.
- After a bulk op, the leading text becomes the result string ("Removed 2 of 3 · 1 failed") for a single load cycle, then reverts.

### Mobile branch
- Toolbar row 2 chips scroll horizontally (`overflow-x: auto`).
- Add button is replaced by a FAB at `.ts-mobile-feeds-fab` (right: 16, bottom: 96).
- Bulk bar renders with `.is-mobile` (drawer style, fixed at bottom).
- Per-row layout uses `.ts-mobile-feed-row` overrides (22 px avatar; smaller title).

### Themes
- Light, dark, and sepia themes all render correctly. Dark theme uses the desaturated accent and the dark-variants of `.ts-feed-err-chip`, `.ts-feed-health`, `.ts-feeds-bulk*`.

### Tests
- `pnpm --dir web test` is green.
- `pnpm --dir web run check` is green.
- `make test` is green (M5 backend changes covered by go tests).

### Accessibility
- Every interactive element is a `<button>` or `<a>`.
- The foot strip is `role="status" aria-live="polite"` so bulk-op result strings are announced.
- Filter chips, sort `<select>`, search input, and action buttons all have either visible labels or `aria-label`.
- Esc closes any open dialog or popover.
- Focus is trapped inside dialogs (M1 `Dialog` primitive owns this; if it doesn't, this milestone adds it).

---

## Verification commands

Run each from `/home/ben.guest/Users/ben/src/tap-plan-m5` (or your worktree).

```bash
# Frontend
pnpm --dir web test                                                     # Vitest, zero failures
pnpm --dir web test -- src/lib/__tests__/feedsFilter.test.ts            # filter/sort unit
pnpm --dir web test -- src/views/__tests__/Feeds.test.ts                # view integration
pnpm --dir web test -- src/components/feeds/__tests__                   # all feeds component tests
pnpm --dir web run check                                                # svelte-check
pnpm --dir web build                                                    # web/dist gets produced

# Backend
go test ./internal/api -run TestPatchSubscriptionRefreshNow -race -count=1
go test ./internal/db -run TestUpdateSubscriptionRefreshNow -race -count=1
go test ./... -race                                                     # everything

# End-to-end
make build
./bin/tap --addr 127.0.0.1:8080 &
# then drive Playwright per Task 16
```

---

## Risks

### 1. There is no per-feed mark-all-read endpoint

The JSX's health panel offers a "Mark all read" action; the brand spec §6.5 also lists it on the bulk bar. Currently only `/api/v1/categories/:id/mark-read` exists (M9). Per-feed mark-all-read would either:
  - (a) Add `POST /api/v1/subscriptions/:id/mark-read` (one handler + DB function — moderate scope).
  - (b) Do it client-side via `GET /api/v1/entries?feed=<id>&unread=1` + N×1 PATCHes per entry (the Unread view already does this for the whole list — see `Unread.svelte:62` `markAllRead`). Quotas could be hit on a feed with thousands of unread entries.

**Decision for M5:** drop "Mark all read" from the bulk bar and the health panel in this milestone. The bulk bar's three actions are Refresh / Set category / Delete. The brand spec is illustrative, not contractual on this point. If a follow-up milestone wants per-feed mark-read, add the endpoint then.

### 2. There is no per-feed refresh endpoint — added in Task 1

This is the only backend change. Risk: the M9/M11 scheduler reads `next_poll_at` every 60 s. Setting it to 0 plus calling `Poke()` is the documented way to force an immediate poll (it's what `POST /api/v1/subscriptions` does — see `cmd/tap/main.go`). Safe.

### 3. Bulk operations — partial failure semantics

`Promise.allSettled` is the right primitive. The view surfaces `Removed 2 of 3 · 1 failed: <message>` to the foot strip. **Risk:** after a partial failure, the user has no way to know *which* feed failed. The current trade-off is "small project, small lists; the next subscriptions.load() shows them which feeds came back." If users complain in practice, escalate to a toast-like inline error per row — that's deferrable.

### 4. Categories ordering coupling with M-Redesign-4

If M-Redesign-4 lands `position` on `categories` and `api.listCategories()` starts returning them ordered, the Feeds page benefits automatically because the only place it shows categories is in dropdown chip lists where order is deferred to the API. If M4 ships before this milestone, no change here. If M4 ships after, no change either. **Risk:** if M4 also reuses `ReassignCategoryPopover` (likely), the two milestones must agree on the props shape. This plan owns the popover; M4 should consume it as-is.

### 5. Group-by-category toggle in the JSX is out of scope

The JSX has a "Group by category" chip on row 2 (`tap-feeds-page.jsx` lines 924–928). The brand spec §6.5 does **not** mention it. Cutting it keeps the row 2 layout cleaner and saves a CSS port. If the user wants it later, it's an additive feature — one chip + one group-header component + a derived `groups()` rune.

### 6. Discovery returning zero feeds

Handled in `AddFeedDialog` Task 8.2: a `.ts-feeds-form` inline error "No feeds found at that URL." Don't auto-dismiss; the user might want to fix the URL and try again. The dialog footer's Subscribe button stays disabled.

### 7. OPML round-trip fidelity

`api.importOPML` returns `{ imported, skipped, errors[] }`. M9 already specs the import behaviour (categories matched by name, deeper nesting flattened, idempotent on duplicates). This milestone trusts that contract. **Risk:** if the OPML preview in the JSX (`TFImportDialog` lines 643–660) is required, the backend doesn't expose a preview endpoint and this milestone won't add one. Drop the preview; the result-summary stage is what the user actually needs.

### 8. Service worker — stale cache after delete

Existing pattern (`store.ts:97`) calls `notifySW({ type: 'invalidate', paths: [...] })` after every mutation. The new bulk operations call this N times, once per per-feed mutation. The SW dedupes by path. **Risk:** if SW's `notifySW` is missing in this branch (foundations milestone may have changed it), each store helper must still send. Verify in Task 16 step 1 (after delete, the next page load doesn't show the deleted feed from cache).

### 9. The "Stale" filter chip's predicate is debatable

This plan defines `stale` as "last successful poll > 7 days ago AND not currently in error backoff." This is one reasonable interpretation; the brand spec doesn't pin it down. Alternatives: "stale = no new entries in 30 days regardless of error state" (data-driven) or "stale = scheduler-flagged" (would need server support). The 7-day heuristic is the cheapest, most-correct-for-most-users choice. If users complain, retune `STALE_THRESHOLD_SECONDS`.

### 10. Mobile bulk bar covers the FAB

When the bulk bar is open on mobile, it sits at `bottom: env(safe-area-inset-bottom) + 12px`, the FAB at `bottom: 96px`. Currently no overlap (the FAB is above the bulk bar). **Risk:** on very short viewports the bulk bar can creep up. Mitigation: when the bulk bar is open on mobile, hide the FAB (`display: none`). Tested implicitly via Task 13.

### 11. Browser-native `<select>` for sort doesn't match design exactly

The CSS rule for `.ts-feeds-sort-select` uses native `<select>` with a CSS-drawn caret. This is fine for keyboard + accessibility. Alternative: implement a custom dropdown via M1 `Popover`. Cost: extra component + tests. Defer; the native version satisfies the design intent at minimum effort.

---

## Self-review notes

- Every JSX-mirrored component has a corresponding task with tests for the branches the JSX exercises (selection, expansion, refresh, more menu, dialog open/close).
- Every brand-spec §6.5 selector is mapped to a Svelte component in Task 14's table.
- Every filter chip count (`All`, `Errors`, `Unread`, `Stale`) is defined and tested in Task 3.
- Every dialog inherits focus-trap + Esc-close from the M1 `Dialog` primitive — no manual focus trap reimplementation.
- The one backend change is narrow (one new flag + one new DB function) and explicitly justified per the umbrella spec's §1 wording.
- TDD: every behaviour-bearing component has at least one RED step.

---

*End of plan.*
