# M-Redesign-7 (Admin) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild `views/Admin.svelte` on the new `.ts-shell` design language: a four-cell instance-wide metric grid (FEEDS / ENTRIES / ERRORS / POLL), a recent-events errors table, and the existing user-management surface (list / create / reset password / disable 2FA / disable / re-enable / delete) restyled with M1 primitives and new chrome.

**Architecture:** Frontend rewrite of one view + one small backend addition. The admin page lives on the centred `.ts-shell` and uses the `.ts-set` page-shell pattern (eyebrow + title + id strip) shared with Settings. The metric grid (`.ts-sys-grid`) and errors table (`.ts-sys-errors`) replace `SystemStatus.svelte` (which M1 deletes). The user table (`.ts-utable`) becomes a grid of `.ts-urow` rows with a toolbar (search input, filter chips, "Create user" primary button) above. All five action overlays (create user, reset password, disable 2FA, disable, re-enable, delete) use M1's `Dialog` primitive. Server-side, a new `GET /api/v1/admin/metrics` returns the four headline data points (feeds total / feeds-ok, entries total / entries-24h, feeds-with-errors / offending feed names, last poll / next poll) — these are not aggregable from the per-user `/api/v1/subscriptions` endpoint, and the existing `/api/v1/status` only exposes poll info.

**Tech Stack:** Svelte 5 runes (`$state`, `$derived`, `$effect`), TypeScript, Vitest + `@testing-library/svelte` for component tests, scoped Svelte `<style>` blocks driven by global tokens, Go 1.25 `net/http` for the new backend endpoint, `modernc.org/sqlite` queries (no CGO), `testify/require` for Go tests.

---

## Scope

### In scope

- Rewrite `web/src/views/Admin.svelte` on `.ts-shell` for admin-only role.
- Build `web/src/components/SysGrid.svelte` (4-cell metric grid).
- Build `web/src/components/ErrorsTable.svelte` (timestamp · code chip · message rows).
- Build `web/src/components/UserTable.svelte` (grid of `.ts-urow` rows with header).
- Build `web/src/components/AdminToolbar.svelte` (search input + filter chips + "Create user" button).
- Build dialog content components (`CreateUserDialog.svelte`, `ResetPasswordResultDialog.svelte`, `ConfirmDialog.svelte`) that wrap the M1 `Dialog` primitive.
- Add `web/src/lib/admin.ts` with `getAdminMetrics()` calling the new endpoint, plus a polling helper.
- Add backend: `internal/db/metrics.go` with `GetAdminMetrics(ctx, db)` aggregating feeds/entries/errors/poll.
- Add backend: `internal/api/metrics.go` with `metricsHandler` (admin-only, behind `authedAdmin`).
- Register the new route in `internal/api/api.go`: `GET /api/v1/admin/metrics`.
- Access-denied path: non-admin users navigating to `/admin` see a `<EmptyState>` "Access denied" and the route never renders admin-only chrome.
- All three themes (light, dark, sepia) verified for the new chrome.
- Mobile shell parity: admin tab in `MobileMoreSheet` for admin role only; admin page renders on `.ts-shell` (full-bleed mobile scroll).

### Out of scope

- Live-update via SSE or WebSocket — the metric grid polls every 60s via `setInterval`, matching the existing `SystemStatus.svelte` cadence.
- "Re-run all polls now" button from the JSX mockup — not in the brand spec §6.7 metric grid; out of scope.
- "Open full logs" button — not in the brand spec §6.7; the errors table is the surface.
- Per-user metric breakdown — admin sees instance-wide aggregates only.
- Admin role change via PATCH — the existing `patchUser` API accepts `role` but the UI does not expose it (matches current Admin.svelte behaviour); out of scope.
- Filter chip behaviour (All / Admins / Users / Disabled) and username search input — they are part of the brand spec §6.7 toolbar, so they ARE in scope as client-side filters over the loaded user list (no API filter param).

---

## Files modified / created / deleted

### Created — frontend

- `web/src/components/SysGrid.svelte` — four-cell metric grid (FEEDS / ENTRIES / ERRORS / POLL). Pure presentation, accepts a `metrics` prop. Uses `.ts-sys-grid` / `.ts-sys-cell` / `.ts-sys-cell-l` / `.ts-sys-cell-v` / `.ts-sys-cell-sub` classnames (scoped).
- `web/src/components/SysGrid.test.ts` — tests for relative-time rendering, "n ok" sub line, empty/loading/error states.
- `web/src/components/ErrorsTable.svelte` — recent-events table. Accepts `events` prop. Uses `.ts-sys-errors` / `.ts-sys-err2` / `.ts-sys-err2-t` / `.ts-sys-err2-m` / `.ts-lvl` classnames (scoped). EmptyState when no events.
- `web/src/components/ErrorsTable.test.ts` — tests for level-tag rendering, timestamp formatting, empty state.
- `web/src/components/UserTable.svelte` — user list (header row + `.ts-urow` rows). Accepts `users` prop and `currentUserId` prop. Emits action events (`reset`, `disable2fa`, `disableUser`, `enable`, `delete`) via callback props. Renders RoleTag, status pill, 2FA flag, passkey count, action buttons.
- `web/src/components/UserTable.test.ts` — tests for "you" badge, disabled state styling, disabled-button states (can't delete self, can't disable self, can't disable-2fa when no totp), action callbacks fire with the right user.
- `web/src/components/AdminToolbar.svelte` — search input + filter chips + "Create user" primary button. Emits filter + query state and a `create` event.
- `web/src/components/AdminToolbar.test.ts` — tests for chip activation, search debounce, create callback.
- `web/src/components/admin/CreateUserDialog.svelte` — wraps `Dialog`. Form: username, generated password (with regenerate button), role segmented. Emits `submit({username, password, role})` / `close`.
- `web/src/components/admin/CreateUserDialog.test.ts` — tests for required field validation, role default = "user", generate-password button, submit emits the right payload.
- `web/src/components/admin/ResetPasswordResultDialog.svelte` — wraps `Dialog`. Shows the temporary password once with copy button + "Download .txt" + Done.
- `web/src/components/admin/ResetPasswordResultDialog.test.ts` — tests for copy-to-clipboard call, download link generation, close fires.
- `web/src/components/admin/ConfirmDialog.svelte` — wraps `Dialog`. Configurable title, body, optional list (e.g., "3 passkeys revoked"), CTA label, danger flag. Emits `confirm` / `cancel`.
- `web/src/components/admin/ConfirmDialog.test.ts` — tests for danger styling, confirm fires, cancel fires, Esc cancels (delegated to Dialog).
- `web/src/lib/admin.ts` — `getAdminMetrics()`, `AdminMetrics` type. Already-existing `api.listUsers` / `api.createUser` / `api.patchUser` / `api.resetUserPassword` / `api.disableUserTOTP` / `api.deleteUser` from `lib/api.ts` stay.
- `web/src/lib/__tests__/admin.test.ts` — tests for `getAdminMetrics` happy path + error.
- `web/src/views/Admin.test.ts` — tests for: non-admin redirects to `/`, admin sees grid + table, "Create user" opens dialog and on submit reloads list, "Reset password" opens result dialog with temp password, "Disable 2FA" opens confirm and on confirm reloads list, "Delete" opens confirm with consequence list and on confirm removes row, metric polling cycles, polling cleared on unmount.

### Created — backend

- `internal/db/metrics.go` — `GetAdminMetrics(ctx, db)` returns:
  - `FeedsTotal` (int): `SELECT COUNT(*) FROM subscriptions`
  - `FeedsOK` (int): `SELECT COUNT(*) FROM subscriptions WHERE error_count = 0`
  - `EntriesTotal` (int): `SELECT COUNT(*) FROM entries`
  - `Entries24h` (int): `SELECT COUNT(*) FROM entries WHERE created_at >= ?` (now - 86400)
  - `FeedsWithErrors` (int): `SELECT COUNT(*) FROM subscriptions WHERE error_count > 0`
  - `OffendingFeeds` ([]string): up to three feed titles `WHERE error_count > 0 ORDER BY error_count DESC LIMIT 3`
  - `LastPollAt` (*int64): `SELECT MAX(last_poll_at) FROM subscriptions WHERE last_poll_at IS NOT NULL`
  - `NextPollAt` (*int64): `SELECT MIN(next_poll_at) FROM subscriptions WHERE next_poll_at > 0`
- `internal/db/metrics_test.go` — table-driven tests for empty DB, all-ok, mixed errors, offending-feeds ordering, 24h window edge.
- `internal/api/metrics.go` — `metricsHandler(db)` HTTP handler. DTO `adminMetricsDTO` maps from `db.AdminMetrics`. JSON encoded. Forbidden returned by `authedAdmin` middleware before reaching handler.
- `internal/api/metrics_test.go` — tests for: 200 happy path with all fields, 401 unauth, 403 non-admin (covered by middleware but exercised end-to-end), DB error → 500 with `ErrCodeInternal`.

### Modified — frontend

- `web/src/views/Admin.svelte` — full rewrite. Composes `<AdminToolbar>`, `<UserTable>`, `<SysGrid>`, `<ErrorsTable>`, dialogs. Polls metrics every 60s. Reloads user list after each mutation. Filters users client-side by search query + role/disabled chips.
- `web/src/lib/api.ts` — no changes (the per-user admin endpoints stay).
- `web/src/styles/tokens.css` (already exists per M1) — no changes needed; all colours used (`--accent`, `--rule`, `--ink-3`, `--bg-soft`) are tokens M1 already vends.

### Modified — backend

- `internal/api/api.go:143` (or wherever admin routes register) — add `m.Handle("GET /api/v1/admin/metrics", authedAdmin(metricsHandler(db)))` next to the other admin routes.

### Deleted

- None at M7 time. M1 already deleted `web/src/components/SystemStatus.svelte` and `web/src/lib/status.ts` is presumed superseded by `web/src/lib/admin.ts` for the metric grid. **If M1 did not delete `lib/status.ts`, this plan's first task deletes it** (its only consumer was `SystemStatus.svelte`, which M1 removed).

---

## TDD posture per task

Per `CLAUDE.md`: TDD non-negotiable for branches, state, error handling; pure CSS/markup exempt.

| Task | TDD required? | Reason |
|---|---|---|
| Backend metrics DB query | **Yes** | Aggregate SQL — must verify counts and ordering with seeded fixtures. |
| Backend metrics handler | **Yes** | Auth gating, error mapping, JSON shape. |
| `SysGrid.svelte` rendering | **Yes** (rendering branches) | Loading / error / empty states; relative-time computation; "n ok" sub line conditional on FeedsOK == FeedsTotal. |
| `ErrorsTable.svelte` rendering | **Yes** | Level-tag colour branch (`error` / `warn` / `info`), empty state, timestamp formatting. |
| `UserTable.svelte` | **Yes** | "you" badge, disabled-button branches (no-self-delete, no-self-disable, no-disable-2fa-without-totp), callback fan-out. |
| `AdminToolbar.svelte` | **Yes** | Chip activation, search debounce, create-callback. |
| `CreateUserDialog.svelte` | **Yes** | Required-field validation, generate-password button, submit payload. |
| `ResetPasswordResultDialog.svelte` | **Yes** | Copy-to-clipboard behaviour, single-shot display. |
| `ConfirmDialog.svelte` | **Yes** | confirm/cancel callbacks, danger flag swaps CTA variant. |
| `Admin.svelte` integration | **Yes** | Access-denied path (non-admin redirects), polling lifecycle, mutation reloads list, filter+search filters list. |
| Scoped CSS port for the above | **No** (pure visual) | Tokens + classnames only. |

---

## Step-by-step tasks

> Each task does one TDD red-green-refactor cycle plus a commit. When two related tests build the same feature (e.g., user listing + filter), keep them in one task with two passes.

### Task 0: Inspect the M1 foundations baseline

**Files:**
- Read: `docs/superpowers/plans/2026-05-11-m-redesign-1-foundations.md` (or whichever filename M1 lands as)
- Read: `web/src/components/Button.svelte`, `Field.svelte`, `Chip.svelte`, `Dialog.svelte`, `EmptyState.svelte` (M1 deliverables)
- Read: `web/src/components/AppShell.svelte` (M1 deliverable)

- [ ] **Step 1: Confirm primitive API surface**

Open each of `Button.svelte`, `Field.svelte`, `Chip.svelte`, `Dialog.svelte`, `EmptyState.svelte` and write down the props each exposes. This plan assumes:

- `Button`: `variant: 'default'|'primary'|'accent'|'danger'|'quiet'`, `size`, `icon` (snippet), `disabled`, `onclick`.
- `Field`: `label`, `value` (`$bindable`), `description`, `mono` flag, `is-stacked` flag.
- `Chip`: `active` flag, `tone: 'neutral'|'error'|'warning'`, `onclick`.
- `Dialog`: `open` (`$bindable`), `title`, body via `children` snippet, `footer` snippet, `wide` flag, `onclose`.
- `EmptyState`: `title`, `description`, `cta` (snippet).

If the actual M1 API differs, **the plan tasks below stay structurally identical but the JSX-equivalent in code must use the real primitive API**. Do not introduce new primitives; instead adapt the integration code.

- [ ] **Step 2: Confirm M1's deletions**

Check that `web/src/components/SystemStatus.svelte` and `web/src/lib/status.ts` are gone. If `lib/status.ts` still exists, plan to delete it after Task 1 leaves no consumer.

Run:
```bash
test ! -f web/src/components/SystemStatus.svelte && echo "deleted: SystemStatus.svelte"
test ! -f web/src/lib/status.ts && echo "deleted: status.ts" || echo "keep: status.ts (delete later)"
```

- [ ] **Step 3: Commit nothing**

This is a read-only inspection task. No commit.

---

### Task 1: Backend — `GetAdminMetrics` DB query (RED)

**Files:**
- Create: `internal/db/metrics.go`
- Test: `internal/db/metrics_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/db/metrics_test.go
package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func TestGetAdminMetrics_EmptyDB(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	m, err := db.GetAdminMetrics(ctx, d, time.Unix(1_700_000_000, 0))
	require.NoError(t, err)
	require.Equal(t, 0, m.FeedsTotal)
	require.Equal(t, 0, m.FeedsOK)
	require.Equal(t, 0, m.EntriesTotal)
	require.Equal(t, 0, m.Entries24h)
	require.Equal(t, 0, m.FeedsWithErrors)
	require.Empty(t, m.OffendingFeeds)
	require.Nil(t, m.LastPollAt)
	require.Nil(t, m.NextPollAt)
}

func TestGetAdminMetrics_PopulatedDB(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, d, "alice")
	now := time.Unix(1_700_000_000, 0)

	// 3 feeds: 2 OK, 1 erroring
	feedOK1 := mustInsertSubscriptionFull(t, d, uid, "Feed OK 1", "https://a.example/feed", 0, now.Unix()-60, now.Unix()+60)
	feedOK2 := mustInsertSubscriptionFull(t, d, uid, "Feed OK 2", "https://b.example/feed", 0, now.Unix()-120, now.Unix()+120)
	mustInsertSubscriptionFull(t, d, uid, "Phoronix", "https://phoronix.com/feed", 4, now.Unix()-3600, now.Unix()+30)

	// 2 entries: 1 within last 24h, 1 older
	mustInsertEntry(t, d, feedOK1, "hash1", "Recent", now.Unix()-1800)
	mustInsertEntry(t, d, feedOK2, "hash2", "Old", now.Unix()-(48*3600))

	m, err := db.GetAdminMetrics(ctx, d, now)
	require.NoError(t, err)
	require.Equal(t, 3, m.FeedsTotal)
	require.Equal(t, 2, m.FeedsOK)
	require.Equal(t, 2, m.EntriesTotal)
	require.Equal(t, 1, m.Entries24h)
	require.Equal(t, 1, m.FeedsWithErrors)
	require.Equal(t, []string{"Phoronix"}, m.OffendingFeeds)
	require.NotNil(t, m.LastPollAt)
	require.NotNil(t, m.NextPollAt)
	// next poll is the smallest positive next_poll_at across feeds
	require.Equal(t, now.Unix()+30, *m.NextPollAt)
	// last poll is the maximum last_poll_at
	require.Equal(t, now.Unix()-60, *m.LastPollAt)
}

func TestGetAdminMetrics_OffendingFeedsLimitAndOrder(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, d, "alice")
	now := time.Unix(1_700_000_000, 0)

	// 5 erroring feeds, descending error_count: e5(error_count=10) ... e1(error_count=2)
	mustInsertSubscriptionFull(t, d, uid, "e1", "https://e1/feed", 2, 0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e2", "https://e2/feed", 3, 0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e3", "https://e3/feed", 4, 0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e4", "https://e4/feed", 5, 0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e5", "https://e5/feed", 10, 0, now.Unix()+60)

	m, err := db.GetAdminMetrics(ctx, d, now)
	require.NoError(t, err)
	require.Equal(t, 5, m.FeedsWithErrors)
	require.Equal(t, []string{"e5", "e4", "e3"}, m.OffendingFeeds) // top 3 by error_count desc
}
```

If `newTestDB`, `mustCreateUser`, `mustInsertSubscriptionFull`, `mustInsertEntry` test helpers do not already exist in `internal/db/`, add minimal versions inline at the top of `metrics_test.go`. Reuse helpers from `subscriptions_test.go` / `entries_test.go` where possible.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/db -run TestGetAdminMetrics -race
```

Expected: FAIL `db.GetAdminMetrics undefined`.

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/db/metrics_test.go
git commit -m "test(db): GetAdminMetrics — empty, populated, offending ordering"
```

---

### Task 2: Backend — `GetAdminMetrics` implementation (GREEN)

**Files:**
- Create: `internal/db/metrics.go`

- [ ] **Step 1: Implement minimal code**

```go
// internal/db/metrics.go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type AdminMetrics struct {
	FeedsTotal      int
	FeedsOK         int
	EntriesTotal    int
	Entries24h      int
	FeedsWithErrors int
	OffendingFeeds  []string
	LastPollAt      *int64
	NextPollAt      *int64
}

// GetAdminMetrics returns instance-wide stats for the admin metric grid.
// `now` is injected so tests can use a fixed clock; production callers pass time.Now().
func GetAdminMetrics(ctx context.Context, d *sql.DB, now time.Time) (AdminMetrics, error) {
	var m AdminMetrics

	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions`).Scan(&m.FeedsTotal); err != nil {
		return m, fmt.Errorf("count feeds: %w", err)
	}
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions WHERE error_count = 0`).Scan(&m.FeedsOK); err != nil {
		return m, fmt.Errorf("count feeds ok: %w", err)
	}
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions WHERE error_count > 0`).Scan(&m.FeedsWithErrors); err != nil {
		return m, fmt.Errorf("count feeds erroring: %w", err)
	}
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM entries`).Scan(&m.EntriesTotal); err != nil {
		return m, fmt.Errorf("count entries: %w", err)
	}
	cutoff := now.Add(-24 * time.Hour).Unix()
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entries WHERE created_at >= ?`, cutoff,
	).Scan(&m.Entries24h); err != nil {
		return m, fmt.Errorf("count entries 24h: %w", err)
	}

	// Top three offending feed titles, error_count desc, then id asc as a stable tiebreaker.
	rows, err := d.QueryContext(ctx, `
		SELECT title FROM subscriptions
		WHERE error_count > 0
		ORDER BY error_count DESC, id ASC
		LIMIT 3`)
	if err != nil {
		return m, fmt.Errorf("query offending feeds: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return m, fmt.Errorf("scan offending feed: %w", err)
		}
		m.OffendingFeeds = append(m.OffendingFeeds, t)
	}
	if err := rows.Err(); err != nil {
		return m, fmt.Errorf("iterate offending feeds: %w", err)
	}

	var last sql.NullInt64
	if err := d.QueryRowContext(ctx,
		`SELECT MAX(last_poll_at) FROM subscriptions WHERE last_poll_at IS NOT NULL`,
	).Scan(&last); err != nil {
		return m, fmt.Errorf("max last_poll_at: %w", err)
	}
	if last.Valid {
		v := last.Int64
		m.LastPollAt = &v
	}

	var next sql.NullInt64
	if err := d.QueryRowContext(ctx,
		`SELECT MIN(next_poll_at) FROM subscriptions WHERE next_poll_at > 0`,
	).Scan(&next); err != nil {
		return m, fmt.Errorf("min next_poll_at: %w", err)
	}
	if next.Valid {
		v := next.Int64
		m.NextPollAt = &v
	}
	return m, nil
}
```

If the `entries` table column is named `published_at` rather than `created_at`, switch to `published_at` (verify by reading `internal/db/migrations/000?_entries.sql` first — the column that the polling worker writes to with the fetched timestamp is the right one for "entries in last 24h").

- [ ] **Step 2: Run test to verify it passes**

```bash
go test ./internal/db -run TestGetAdminMetrics -race
```

Expected: PASS three subtests.

- [ ] **Step 3: Commit**

```bash
git add internal/db/metrics.go
git commit -m "feat(db): GetAdminMetrics aggregates feeds/entries/errors/poll"
```

---

### Task 3: Backend — `/api/v1/admin/metrics` handler (RED)

**Files:**
- Test: `internal/api/metrics_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/metrics_test.go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminMetrics_Forbidden_NonAdmin(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	tok, _ := loginUser(t, srv, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	req.AddCookie(authCookie(tok))
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminMetrics_OK(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	tok, _ := loginUser(t, srv, "admin1", "admin")

	// seed a couple of feeds/entries — reuse subscription test helpers
	seedFeed(t, srv, "Feed A", "https://a.example/feed", 0)
	seedFeed(t, srv, "Phoronix", "https://phoronix.com/feed", 5)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	req.AddCookie(authCookie(tok))
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		FeedsTotal      int      `json:"feeds_total"`
		FeedsOK         int      `json:"feeds_ok"`
		EntriesTotal    int      `json:"entries_total"`
		Entries24h      int      `json:"entries_24h"`
		FeedsWithErrors int      `json:"feeds_with_errors"`
		OffendingFeeds  []string `json:"offending_feeds"`
		LastPollAt      *int64   `json:"last_poll_at"`
		NextPollAt      *int64   `json:"next_poll_at"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.Equal(t, 2, body.FeedsTotal)
	require.Equal(t, 1, body.FeedsOK)
	require.Equal(t, 1, body.FeedsWithErrors)
	require.Equal(t, []string{"Phoronix"}, body.OffendingFeeds)
}

func TestAdminMetrics_Unauthenticated(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
```

If `newTestServer`, `loginUser`, `seedFeed`, `authCookie` test helpers do not exist in `internal/api/main_test.go`, look there first for the existing equivalent (e.g., `testServer`, `mustLogin`) and use those. Do not introduce parallel helpers.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/api -run TestAdminMetrics -race
```

Expected: FAIL `404 not found` (route not registered).

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/api/metrics_test.go
git commit -m "test(api): /api/v1/admin/metrics 200 / 401 / 403"
```

---

### Task 4: Backend — `/api/v1/admin/metrics` handler (GREEN)

**Files:**
- Create: `internal/api/metrics.go`
- Modify: `internal/api/api.go`

- [ ] **Step 1: Implement handler**

```go
// internal/api/metrics.go
package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/db"
)

type adminMetricsDTO struct {
	FeedsTotal      int      `json:"feeds_total"`
	FeedsOK         int      `json:"feeds_ok"`
	EntriesTotal    int      `json:"entries_total"`
	Entries24h      int      `json:"entries_24h"`
	FeedsWithErrors int      `json:"feeds_with_errors"`
	OffendingFeeds  []string `json:"offending_feeds"`
	LastPollAt      *int64   `json:"last_poll_at"`
	NextPollAt      *int64   `json:"next_poll_at"`
}

func metricsHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, err := db.GetAdminMetrics(r.Context(), d, time.Now())
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		dto := adminMetricsDTO{
			FeedsTotal:      m.FeedsTotal,
			FeedsOK:         m.FeedsOK,
			EntriesTotal:    m.EntriesTotal,
			Entries24h:      m.Entries24h,
			FeedsWithErrors: m.FeedsWithErrors,
			OffendingFeeds:  m.OffendingFeeds,
			LastPollAt:      m.LastPollAt,
			NextPollAt:      m.NextPollAt,
		}
		if dto.OffendingFeeds == nil {
			dto.OffendingFeeds = []string{}
		}
		writeJSON(w, http.StatusOK, dto)
	})
}
```

- [ ] **Step 2: Register route**

In `internal/api/api.go`, alongside the other `m.Handle("GET /api/v1/admin/users", ...)` lines (around line 143), add:

```go
m.Handle("GET /api/v1/admin/metrics", authedAdmin(metricsHandler(db)))
```

- [ ] **Step 3: Run tests to verify they pass**

```bash
go test ./internal/api -run TestAdminMetrics -race
go test ./internal/api -race
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/api/metrics.go internal/api/api.go
git commit -m "feat(api): GET /api/v1/admin/metrics (admin-only)"
```

---

### Task 5: Frontend — `lib/admin.ts` and types (RED)

**Files:**
- Test: `web/src/lib/__tests__/admin.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/lib/__tests__/admin.test.ts
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { getAdminMetrics } from '../admin';

describe('getAdminMetrics', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('returns parsed metrics on 200', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      feeds_total: 12,
      feeds_ok: 10,
      entries_total: 14_820,
      entries_24h: 1_402,
      feeds_with_errors: 2,
      offending_feeds: ['Phoronix', 'LWN'],
      last_poll_at: 1_700_000_000,
      next_poll_at: 1_700_000_060,
    }), { status: 200 }));

    const m = await getAdminMetrics();
    expect(m.feedsTotal).toBe(12);
    expect(m.feedsOK).toBe(10);
    expect(m.entriesTotal).toBe(14_820);
    expect(m.entries24h).toBe(1_402);
    expect(m.feedsWithErrors).toBe(2);
    expect(m.offendingFeeds).toEqual(['Phoronix', 'LWN']);
    expect(m.lastPollAt).toBe(1_700_000_000);
    expect(m.nextPollAt).toBe(1_700_000_060);
  });

  it('throws on non-2xx', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(new Response('forbidden', { status: 403 }));
    await expect(getAdminMetrics()).rejects.toThrow(/403/);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/lib/__tests__/admin.test.ts --run
```

Expected: FAIL "Cannot find module '../admin'".

- [ ] **Step 3: Implement `lib/admin.ts`**

```ts
// web/src/lib/admin.ts
export type AdminMetrics = {
  feedsTotal: number;
  feedsOK: number;
  entriesTotal: number;
  entries24h: number;
  feedsWithErrors: number;
  offendingFeeds: string[];
  lastPollAt: number | null;
  nextPollAt: number | null;
};

export async function getAdminMetrics(): Promise<AdminMetrics> {
  const resp = await fetch('/api/v1/admin/metrics');
  if (!resp.ok) throw new Error(`admin metrics: ${resp.status}`);
  const j = await resp.json();
  return {
    feedsTotal: j.feeds_total,
    feedsOK: j.feeds_ok,
    entriesTotal: j.entries_total,
    entries24h: j.entries_24h,
    feedsWithErrors: j.feeds_with_errors,
    offendingFeeds: j.offending_feeds ?? [],
    lastPollAt: j.last_poll_at,
    nextPollAt: j.next_poll_at,
  };
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/lib/__tests__/admin.test.ts --run
```

Expected: PASS both tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/admin.ts web/src/lib/__tests__/admin.test.ts
git commit -m "feat(web): admin metrics fetcher + AdminMetrics type"
```

---

### Task 6: Frontend — `SysGrid.svelte` (RED)

**Files:**
- Test: `web/src/components/SysGrid.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/SysGrid.test.ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import SysGrid from './SysGrid.svelte';

const baseMetrics = {
  feedsTotal: 24,
  feedsOK: 22,
  entriesTotal: 14_820,
  entries24h: 1_402,
  feedsWithErrors: 2,
  offendingFeeds: ['Phoronix', 'LWN'],
  lastPollAt: 1_700_000_000,
  nextPollAt: 1_700_000_060,
};

describe('SysGrid', () => {
  it('renders four labelled cells', () => {
    render(SysGrid, { props: { metrics: baseMetrics, now: 1_700_000_840 } });
    expect(screen.getByText('FEEDS')).toBeInTheDocument();
    expect(screen.getByText('ENTRIES')).toBeInTheDocument();
    expect(screen.getByText('ERRORS')).toBeInTheDocument();
    expect(screen.getByText('POLL')).toBeInTheDocument();
  });

  it('FEEDS cell shows total + "n ok" sub line', () => {
    render(SysGrid, { props: { metrics: baseMetrics, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-feeds-v')).toHaveTextContent('24');
    expect(screen.getByTestId('sys-feeds-sub')).toHaveTextContent('22 ok');
  });

  it('ENTRIES cell shows total + 24h sub line', () => {
    render(SysGrid, { props: { metrics: baseMetrics, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-entries-v')).toHaveTextContent('14,820');
    expect(screen.getByTestId('sys-entries-sub')).toHaveTextContent('1,402 24h');
  });

  it('ERRORS cell shows count + first offending feed', () => {
    render(SysGrid, { props: { metrics: baseMetrics, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-errors-v')).toHaveTextContent('2');
    expect(screen.getByTestId('sys-errors-sub')).toHaveTextContent('Phoronix');
  });

  it('ERRORS cell renders empty sub when no offending feeds', () => {
    render(SysGrid, { props: { metrics: { ...baseMetrics, feedsWithErrors: 0, offendingFeeds: [] }, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-errors-v')).toHaveTextContent('0');
    expect(screen.getByTestId('sys-errors-sub')).toHaveTextContent('all clear');
  });

  it('POLL cell shows relative last + next', () => {
    // now = lastPollAt + 14*60 ; next = now + 60
    const now = 1_700_000_000 + 14 * 60;
    render(SysGrid, { props: { metrics: { ...baseMetrics, lastPollAt: 1_700_000_000, nextPollAt: now + 60 }, now } });
    expect(screen.getByTestId('sys-poll-v')).toHaveTextContent('14m ago');
    expect(screen.getByTestId('sys-poll-sub')).toHaveTextContent('next 1m');
  });

  it('POLL cell handles missing timestamps', () => {
    render(SysGrid, { props: { metrics: { ...baseMetrics, lastPollAt: null, nextPollAt: null }, now: 1_700_000_000 } });
    expect(screen.getByTestId('sys-poll-v')).toHaveTextContent('—');
    expect(screen.getByTestId('sys-poll-sub')).toHaveTextContent('—');
  });

  it('loading prop renders skeleton dashes in all cells', () => {
    render(SysGrid, { props: { metrics: null, loading: true, now: 0 } });
    expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(4);
  });

  it('error prop renders error message in place of grid', () => {
    render(SysGrid, { props: { metrics: null, error: 'forbidden', now: 0 } });
    expect(screen.getByRole('alert')).toHaveTextContent('forbidden');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/components/SysGrid.test.ts --run
```

Expected: FAIL "Cannot find module './SysGrid.svelte'".

- [ ] **Step 3: Implement `SysGrid.svelte`**

```svelte
<!-- web/src/components/SysGrid.svelte -->
<script lang="ts">
  import type { AdminMetrics } from '../lib/admin';

  type Props = {
    metrics: AdminMetrics | null;
    loading?: boolean;
    error?: string;
    now: number; // unix seconds
  };

  const { metrics, loading = false, error = '', now }: Props = $props();

  function fmtCount(n: number): string {
    return n.toLocaleString('en-US');
  }

  function relativePast(then: number | null, nowS: number): string {
    if (then == null) return '—';
    const dt = nowS - then;
    if (dt < 60) return `${Math.max(0, dt)}s ago`;
    if (dt < 3600) return `${Math.floor(dt / 60)}m ago`;
    if (dt < 86400) return `${Math.floor(dt / 3600)}h ago`;
    return `${Math.floor(dt / 86400)}d ago`;
  }

  function relativeFuture(then: number | null, nowS: number): string {
    if (then == null) return '—';
    const dt = then - nowS;
    if (dt <= 0) return 'next now';
    if (dt < 60) return `next ${dt}s`;
    if (dt < 3600) return `next ${Math.floor(dt / 60)}m`;
    if (dt < 86400) return `next ${Math.floor(dt / 3600)}h`;
    return `next ${Math.floor(dt / 86400)}d`;
  }
</script>

{#if error}
  <div class="grid-err" role="alert">{error}</div>
{:else}
  <div class="grid">
    <div class="cell">
      <span class="l">FEEDS</span>
      <span class="v" data-testid="sys-feeds-v">
        {#if loading || !metrics}—{:else}{fmtCount(metrics.feedsTotal)}{/if}
      </span>
      <span class="sub" data-testid="sys-feeds-sub">
        {#if loading || !metrics}—{:else}{fmtCount(metrics.feedsOK)} ok{/if}
      </span>
    </div>
    <div class="cell">
      <span class="l">ENTRIES</span>
      <span class="v" data-testid="sys-entries-v">
        {#if loading || !metrics}—{:else}{fmtCount(metrics.entriesTotal)}{/if}
      </span>
      <span class="sub" data-testid="sys-entries-sub">
        {#if loading || !metrics}—{:else}{fmtCount(metrics.entries24h)} 24h{/if}
      </span>
    </div>
    <div class="cell">
      <span class="l">ERRORS</span>
      <span class="v" class:warn={metrics && metrics.feedsWithErrors > 0} data-testid="sys-errors-v">
        {#if loading || !metrics}—{:else}{metrics.feedsWithErrors}{/if}
      </span>
      <span class="sub" data-testid="sys-errors-sub">
        {#if loading || !metrics}—
        {:else if metrics.offendingFeeds.length === 0}all clear
        {:else}{metrics.offendingFeeds[0]}{#if metrics.offendingFeeds.length > 1} +{metrics.offendingFeeds.length - 1}{/if}
        {/if}
      </span>
    </div>
    <div class="cell">
      <span class="l">POLL</span>
      <span class="v" data-testid="sys-poll-v">
        {#if loading || !metrics}—{:else}{relativePast(metrics.lastPollAt, now)}{/if}
      </span>
      <span class="sub" data-testid="sys-poll-sub">
        {#if loading || !metrics}—{:else}{relativeFuture(metrics.nextPollAt, now)}{/if}
      </span>
    </div>
  </div>
{/if}

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    border: 1px solid var(--rule);
    border-radius: 4px;
    overflow: hidden;
    background: var(--bg);
  }
  .cell {
    padding: 14px 16px;
    border-right: 1px solid var(--rule);
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .cell:last-child { border-right: 0; }
  .l {
    font-family: var(--mono);
    font-size: 9.5px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-3);
  }
  .v {
    font-family: var(--mono);
    font-size: 16px;
    font-weight: 500;
    color: var(--ink);
  }
  .v.warn { color: #c97a1a; }
  :global(html.theme-dark) .v.warn { color: #f0a655; }
  .sub {
    font-family: var(--mono);
    font-size: 10px;
    color: var(--ink-3);
  }
  .grid-err {
    padding: 14px 16px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    font-family: var(--mono);
    font-size: 12px;
    color: var(--ink-3);
  }
  @media (max-width: 640px) {
    .grid { grid-template-columns: repeat(2, 1fr); }
    .cell:nth-child(2) { border-right: 0; }
    .cell:nth-child(3), .cell:nth-child(4) { border-top: 1px solid var(--rule); }
  }
</style>
```

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/components/SysGrid.test.ts --run
```

Expected: PASS all sub-tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SysGrid.svelte web/src/components/SysGrid.test.ts
git commit -m "feat(web): SysGrid — 4-cell admin metric grid"
```

---

### Task 7: Frontend — `ErrorsTable.svelte`

**Files:**
- Create: `web/src/components/ErrorsTable.svelte`
- Test: `web/src/components/ErrorsTable.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/ErrorsTable.test.ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ErrorsTable from './ErrorsTable.svelte';

const events = [
  { time: '2026-05-11T10:42:14Z', level: 'error', event: 'phoronix.com — 502', attrs: {} },
  { time: '2026-05-11T10:38:41Z', level: 'warn',  event: 'poll worker 3 slow', attrs: {} },
  { time: '2026-05-11T09:12:50Z', level: 'info',  event: 'liz signed in', attrs: {} },
];

describe('ErrorsTable', () => {
  it('renders one row per event with level tag and message', () => {
    render(ErrorsTable, { props: { events } });
    expect(screen.getByText('phoronix.com — 502')).toBeInTheDocument();
    expect(screen.getByText('poll worker 3 slow')).toBeInTheDocument();
    expect(screen.getByText('liz signed in')).toBeInTheDocument();
    expect(screen.getAllByText('error')).toHaveLength(1);
    expect(screen.getAllByText('warn')).toHaveLength(1);
    expect(screen.getAllByText('info')).toHaveLength(1);
  });

  it('formats timestamp as HH:MM:SS', () => {
    render(ErrorsTable, { props: { events: [events[0]] } });
    expect(screen.getByTestId('err-time')).toHaveTextContent(/\d{2}:\d{2}:\d{2}/);
  });

  it('renders empty state when no events', () => {
    render(ErrorsTable, { props: { events: [] } });
    expect(screen.getByText(/no recent events/i)).toBeInTheDocument();
  });

  it('applies is-error class to error-level rows', () => {
    const { container } = render(ErrorsTable, { props: { events: [events[0]] } });
    expect(container.querySelector('.err.level-error')).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/components/ErrorsTable.test.ts --run
```

Expected: FAIL.

- [ ] **Step 3: Implement `ErrorsTable.svelte`**

```svelte
<!-- web/src/components/ErrorsTable.svelte -->
<script lang="ts">
  type Event = {
    time: string;       // RFC3339
    level: 'error' | 'warn' | 'info';
    event: string;
    attrs: Record<string, unknown>;
  };

  type Props = { events: Event[] };
  const { events }: Props = $props();

  function fmtTime(iso: string): string {
    const d = new Date(iso);
    const hh = String(d.getHours()).padStart(2, '0');
    const mm = String(d.getMinutes()).padStart(2, '0');
    const ss = String(d.getSeconds()).padStart(2, '0');
    return `${hh}:${mm}:${ss}`;
  }
</script>

{#if events.length === 0}
  <div class="empty">No recent events</div>
{:else}
  <div class="table">
    {#each events as e (e.time + e.event)}
      <div class="err level-{e.level}">
        <span class="t" data-testid="err-time">{fmtTime(e.time)}</span>
        <span class="lvl lvl-{e.level}">{e.level}</span>
        <span class="m">{e.event}</span>
      </div>
    {/each}
  </div>
{/if}

<style>
  .table {
    border: 1px solid var(--rule);
    border-radius: 4px;
    overflow: hidden;
    background: var(--bg);
  }
  .err {
    display: grid;
    grid-template-columns: 76px 58px 1fr;
    gap: 14px;
    align-items: center;
    padding: 9px 14px;
    border-bottom: 1px solid var(--rule);
    font-family: var(--mono);
    font-size: 11.5px;
    color: var(--ink-2);
  }
  .err:last-child { border-bottom: 0; }
  .t { color: var(--ink-3); }
  .m {
    color: var(--ink);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .lvl {
    font-family: var(--mono);
    font-size: 9.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 2px 6px;
    border-radius: 2px;
    text-align: center;
    border: 1px solid var(--rule);
    color: var(--ink-3);
    background: var(--bg);
  }
  .lvl-error { color: #c43a3a; border-color: rgba(196, 58, 58, 0.45); background: rgba(196, 58, 58, 0.06); }
  :global(html.theme-dark) .lvl-error { color: #ec7a7a; border-color: rgba(236, 122, 122, 0.45); background: rgba(236, 122, 122, 0.08); }
  .lvl-warn  { color: #c97a1a; border-color: rgba(201, 122, 26, 0.45); background: rgba(201, 122, 26, 0.06); }
  :global(html.theme-dark) .lvl-warn { color: #f0a655; border-color: rgba(240, 166, 85, 0.45); background: rgba(240, 166, 85, 0.08); }
  .level-error { background: rgba(196, 58, 58, 0.02); }
  :global(html.theme-dark) .level-error { background: rgba(236, 122, 122, 0.04); }
  .empty {
    padding: 16px;
    text-align: center;
    color: var(--ink-3);
    font-family: var(--mono);
    font-size: 11.5px;
    border: 1px solid var(--rule);
    border-radius: 4px;
  }
</style>
```

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/components/ErrorsTable.test.ts --run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ErrorsTable.svelte web/src/components/ErrorsTable.test.ts
git commit -m "feat(web): ErrorsTable — recent events with level tags"
```

---

### Task 8: Frontend — `UserTable.svelte`

**Files:**
- Create: `web/src/components/UserTable.svelte`
- Test: `web/src/components/UserTable.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/UserTable.test.ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import UserTable from './UserTable.svelte';
import type { AdminUser } from '../lib/types';

const me: AdminUser = { id: 1, username: 'liz',    role: 'admin', created_at: 1_700_000_000, disabled_at: null,         has_totp: true,  passkey_count: 3 };
const alice: AdminUser = { id: 2, username: 'alice',  role: 'user', created_at: 1_700_000_000, disabled_at: null,         has_totp: false, passkey_count: 0 };
const bob: AdminUser = { id: 3, username: 'bob',    role: 'user', created_at: 1_700_000_000, disabled_at: 1_700_500_000, has_totp: true,  passkey_count: 1 };

describe('UserTable', () => {
  it('renders one row per user with username + role + status', () => {
    render(UserTable, { props: { users: [me, alice, bob], currentUserId: 1 } });
    expect(screen.getByText('liz')).toBeInTheDocument();
    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('bob')).toBeInTheDocument();
    expect(screen.getAllByText('admin').length).toBeGreaterThanOrEqual(1);
  });

  it('shows "you" badge on current user row', () => {
    render(UserTable, { props: { users: [me, alice], currentUserId: 1 } });
    expect(screen.getByText('you')).toBeInTheDocument();
  });

  it('shows passkey count', () => {
    render(UserTable, { props: { users: [me], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-pk-${me.id}`)).toHaveTextContent('3');
  });

  it('shows 2FA flag on/off', () => {
    render(UserTable, { props: { users: [me, alice], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-totp-${me.id}`)).toHaveTextContent('enabled');
    expect(screen.getByTestId(`urow-totp-${alice.id}`)).toHaveTextContent('not set');
  });

  it('shows disabled pill on disabled row', () => {
    render(UserTable, { props: { users: [bob], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-status-${bob.id}`)).toHaveTextContent('disabled');
  });

  it('disables "Delete" and "Disable" buttons for current user', () => {
    render(UserTable, { props: { users: [me], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-action-delete-${me.id}`)).toBeDisabled();
    expect(screen.getByTestId(`urow-action-disableUser-${me.id}`)).toBeDisabled();
  });

  it('disables "Disable 2FA" when user has no TOTP', () => {
    render(UserTable, { props: { users: [alice], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-action-disable2fa-${alice.id}`)).toBeDisabled();
  });

  it('shows "Re-enable" instead of "Disable" on disabled user', () => {
    render(UserTable, { props: { users: [bob], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-action-enable-${bob.id}`)).toBeInTheDocument();
    expect(screen.queryByTestId(`urow-action-disableUser-${bob.id}`)).toBeNull();
  });

  it('fires onAction callback with action name and user', async () => {
    const onAction = vi.fn();
    render(UserTable, { props: { users: [alice], currentUserId: 1, onAction } });
    await fireEvent.click(screen.getByTestId(`urow-action-reset-${alice.id}`));
    expect(onAction).toHaveBeenCalledWith('reset', alice);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/components/UserTable.test.ts --run
```

Expected: FAIL.

- [ ] **Step 3: Implement `UserTable.svelte`**

```svelte
<!-- web/src/components/UserTable.svelte -->
<script lang="ts">
  import type { AdminUser } from '../lib/types';

  type Action = 'reset' | 'disable2fa' | 'disableUser' | 'enable' | 'delete';

  type Props = {
    users: AdminUser[];
    currentUserId: number;
    onAction?: (action: Action, user: AdminUser) => void;
  };

  const { users, currentUserId, onAction }: Props = $props();

  function fmtDate(ts: number): string {
    return new Date(ts * 1000).toLocaleDateString('en-US', { month: 'short', day: '2-digit', year: 'numeric' });
  }
</script>

<div class="table">
  <div class="head">
    <div>User</div><div>Role</div><div>Created</div><div>Status</div><div>2FA</div><div>Passkeys</div><div>Actions</div>
  </div>
  {#each users as u (u.id)}
    {@const isYou = u.id === currentUserId}
    {@const disabled = !!u.disabled_at}
    <div class="row" class:disabled class:is-you={isYou}>
      <div class="cell user">
        <span class="avatar" aria-hidden="true">{u.username[0].toUpperCase()}</span>
        <div class="stack">
          <span class="name">{u.username}{#if isYou}<span class="you">you</span>{/if}</span>
        </div>
      </div>
      <div class="cell"><span class="role role-{u.role}"><span class="dot"></span>{u.role}</span></div>
      <div class="cell date">{fmtDate(u.created_at)}</div>
      <div class="cell" data-testid="urow-status-{u.id}">
        <span class="pill pill-{disabled ? 'muted' : 'active'}">{disabled ? 'disabled' : 'active'}</span>
      </div>
      <div class="cell" data-testid="urow-totp-{u.id}">
        <span class="flag flag-{u.has_totp ? 'on' : 'off'}">{u.has_totp ? 'enabled' : 'not set'}</span>
      </div>
      <div class="cell pk" data-testid="urow-pk-{u.id}">{u.passkey_count}</div>
      <div class="cell actions">
        <button type="button" data-testid="urow-action-reset-{u.id}" onclick={() => onAction?.('reset', u)}>Reset password</button>
        <button type="button" data-testid="urow-action-disable2fa-{u.id}" disabled={!u.has_totp} onclick={() => onAction?.('disable2fa', u)}>Disable 2FA</button>
        {#if disabled}
          <button type="button" data-testid="urow-action-enable-{u.id}" onclick={() => onAction?.('enable', u)}>Re-enable</button>
        {:else}
          <button type="button" data-testid="urow-action-disableUser-{u.id}" disabled={isYou} onclick={() => onAction?.('disableUser', u)}>Disable</button>
        {/if}
        <button type="button" class="danger" data-testid="urow-action-delete-{u.id}" disabled={isYou} onclick={() => onAction?.('delete', u)}>Delete</button>
      </div>
    </div>
  {/each}
</div>

<style>
  .table {
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--bg);
    overflow: hidden;
  }
  .head, .row {
    display: grid;
    grid-template-columns: minmax(0, 1.7fr) 96px 110px 92px 110px 84px minmax(0, 1.6fr);
    align-items: center;
    gap: 14px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--rule);
  }
  .head {
    font-family: var(--mono);
    font-size: 9.5px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-3);
    background: var(--bg-soft);
  }
  .row:last-child { border-bottom: 0; }
  .row:hover { background: var(--bg-soft); }
  .row.disabled .name, .row.disabled .date { color: var(--ink-3); }
  .row.disabled .avatar { opacity: 0.55; }
  .cell { font-family: var(--sans); font-size: 13.5px; color: var(--ink); min-width: 0; }
  .user { display: flex; align-items: center; gap: 10px; }
  .avatar {
    width: 28px; height: 28px;
    border-radius: 50%;
    background: var(--accent-soft);
    color: var(--accent);
    display: inline-flex; align-items: center; justify-content: center;
    font-family: var(--mono); font-size: 12px; font-weight: 500;
  }
  .stack { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
  .name { display: inline-flex; gap: 6px; align-items: center; }
  .you {
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.08em; text-transform: uppercase;
    color: var(--ink-3);
    border: 1px solid var(--rule); padding: 1px 5px; border-radius: 2px;
  }
  .date { font-family: var(--mono); font-size: 12px; color: var(--ink-2); }
  .pk { display: inline-flex; align-items: center; gap: 6px; color: var(--ink-2); font-family: var(--mono); font-size: 12px; }
  .role {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    padding: 2px 8px; border-radius: 999px;
    border: 1px solid var(--rule); color: var(--ink-2);
    display: inline-flex; align-items: center; gap: 6px;
  }
  .role-admin { color: var(--accent); border-color: var(--accent); background: var(--accent-soft); }
  .dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
  .pill {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    color: var(--ink-2);
    display: inline-flex; align-items: center; gap: 6px;
  }
  .pill::before { content: ''; display: inline-block; width: 6px; height: 6px; border-radius: 50%; }
  .pill-active { color: var(--ink); }
  .pill-active::before { background: #4a9a4a; }
  :global(html.theme-dark) .pill-active::before { background: #6dbf6d; }
  .pill-muted { color: var(--ink-3); }
  .pill-muted::before { background: var(--ink-4); }
  .flag { font-family: var(--mono); font-size: 12px; }
  .flag-on { color: var(--ink); }
  .flag-off { color: var(--ink-3); font-style: italic; }
  .actions { display: inline-flex; gap: 4px; flex-wrap: wrap; justify-content: flex-end; }
  .actions button {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    color: var(--ink-2);
    background: transparent;
    border: 1px solid transparent;
    border-radius: 3px;
    padding: 4px 8px;
    cursor: pointer;
  }
  .actions button:hover { color: var(--ink); border-color: var(--rule); background: var(--bg-soft); }
  .actions button:disabled { color: var(--ink-4); cursor: not-allowed; }
  .actions button:disabled:hover { background: transparent; border-color: transparent; }
  .actions button.danger:hover { color: #c43a3a; border-color: rgba(196, 58, 58, 0.4); background: rgba(196, 58, 58, 0.04); }
  :global(html.theme-dark) .actions button.danger:hover { color: #ec7a7a; }
  @media (max-width: 920px) {
    .head { display: none; }
    .row { grid-template-columns: 1fr; gap: 6px; }
  }
</style>
```

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/components/UserTable.test.ts --run
```

Expected: PASS all sub-tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/UserTable.svelte web/src/components/UserTable.test.ts
git commit -m "feat(web): UserTable — admin user-management grid"
```

---

### Task 9: Frontend — `AdminToolbar.svelte`

**Files:**
- Create: `web/src/components/AdminToolbar.svelte`
- Test: `web/src/components/AdminToolbar.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/AdminToolbar.test.ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import AdminToolbar from './AdminToolbar.svelte';

describe('AdminToolbar', () => {
  it('renders search input and four filter chips', () => {
    render(AdminToolbar, { props: { filter: 'all', query: '', count: 7 } });
    expect(screen.getByPlaceholderText('Filter by username')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'All' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Admins' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Users' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disabled' })).toBeInTheDocument();
  });

  it('marks the active filter chip', () => {
    const { rerender } = render(AdminToolbar, { props: { filter: 'admins', query: '', count: 7 } });
    expect(screen.getByRole('button', { name: 'Admins' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('fires onFilter when chip clicked', async () => {
    const onFilter = vi.fn();
    render(AdminToolbar, { props: { filter: 'all', query: '', count: 7, onFilter } });
    await fireEvent.click(screen.getByRole('button', { name: 'Disabled' }));
    expect(onFilter).toHaveBeenCalledWith('disabled');
  });

  it('fires onQuery on input', async () => {
    const onQuery = vi.fn();
    render(AdminToolbar, { props: { filter: 'all', query: '', count: 7, onQuery } });
    await fireEvent.input(screen.getByPlaceholderText('Filter by username'), { target: { value: 'al' } });
    expect(onQuery).toHaveBeenCalledWith('al');
  });

  it('fires onCreate when "Create user" clicked', async () => {
    const onCreate = vi.fn();
    render(AdminToolbar, { props: { filter: 'all', query: '', count: 7, onCreate } });
    await fireEvent.click(screen.getByRole('button', { name: /create user/i }));
    expect(onCreate).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/components/AdminToolbar.test.ts --run
```

Expected: FAIL.

- [ ] **Step 3: Implement `AdminToolbar.svelte`**

```svelte
<!-- web/src/components/AdminToolbar.svelte -->
<script lang="ts">
  export type AdminFilter = 'all' | 'admins' | 'users' | 'disabled';
  type Props = {
    filter: AdminFilter;
    query: string;
    count: number;
    onFilter?: (f: AdminFilter) => void;
    onQuery?: (q: string) => void;
    onCreate?: () => void;
  };
  const { filter, query, count, onFilter, onQuery, onCreate }: Props = $props();
  const chips: Array<{ id: AdminFilter; label: string }> = [
    { id: 'all', label: 'All' },
    { id: 'admins', label: 'Admins' },
    { id: 'users', label: 'Users' },
    { id: 'disabled', label: 'Disabled' },
  ];
</script>

<div class="bar">
  <div class="left">
    <label class="search">
      <span class="ico" aria-hidden="true">
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><circle cx="7" cy="7" r="4.5"/><path d="m10.5 10.5 3 3"/></svg>
      </span>
      <input
        type="search"
        placeholder="Filter by username"
        value={query}
        oninput={(e) => onQuery?.((e.currentTarget as HTMLInputElement).value)}
      />
    </label>
    <div class="chips" role="group" aria-label="Filter users">
      {#each chips as c (c.id)}
        <button type="button" class="chip" class:active={filter === c.id} aria-pressed={filter === c.id} onclick={() => onFilter?.(c.id)}>{c.label}</button>
      {/each}
    </div>
  </div>
  <button type="button" class="create" onclick={() => onCreate?.()}>
    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M8 3.5v9M3.5 8h9"/></svg>
    <span>Create user</span>
  </button>
</div>

<style>
  .bar {
    display: flex; align-items: center; justify-content: space-between;
    gap: 14px;
    margin-bottom: 14px;
  }
  .left { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  .search {
    display: inline-flex; align-items: center; gap: 8px;
    padding: 6px 10px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--bg);
    min-width: 240px;
  }
  .search .ico { color: var(--ink-3); display: inline-flex; }
  .search input {
    font-family: var(--sans); font-size: 13px; color: var(--ink);
    border: 0; background: transparent; outline: none; flex: 1;
  }
  .search input::placeholder { color: var(--ink-3); }
  .chips { display: inline-flex; gap: 4px; }
  .chip {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    color: var(--ink-2);
    padding: 5px 10px;
    border: 1px solid var(--rule);
    background: var(--bg);
    border-radius: 999px;
    cursor: pointer;
  }
  .chip:hover { color: var(--ink); border-color: var(--ink-4); }
  .chip.active {
    color: var(--accent);
    border-color: var(--accent);
    background: var(--accent-soft);
  }
  .create {
    display: inline-flex; align-items: center; gap: 6px;
    font-family: var(--sans); font-size: 13px; font-weight: 500;
    color: var(--bg);
    background: var(--ink);
    border: 1px solid var(--ink);
    border-radius: 4px;
    padding: 7px 12px;
    cursor: pointer;
  }
  .create:hover { background: var(--ink-2); border-color: var(--ink-2); }
</style>
```

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/components/AdminToolbar.test.ts --run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/AdminToolbar.svelte web/src/components/AdminToolbar.test.ts
git commit -m "feat(web): AdminToolbar — search + filter chips + create button"
```

---

### Task 10: Frontend — `CreateUserDialog.svelte`

**Files:**
- Create: `web/src/components/admin/CreateUserDialog.svelte`
- Test: `web/src/components/admin/CreateUserDialog.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/admin/CreateUserDialog.test.ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import CreateUserDialog from './CreateUserDialog.svelte';

describe('CreateUserDialog', () => {
  it('opens with default role = user', () => {
    render(CreateUserDialog, { props: { open: true } });
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/initial password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'User' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('regenerate-password button replaces password value', async () => {
    render(CreateUserDialog, { props: { open: true } });
    const pwInput = screen.getByLabelText(/initial password/i) as HTMLInputElement;
    const original = pwInput.value;
    await fireEvent.click(screen.getByRole('button', { name: /regenerate password/i }));
    expect(pwInput.value).not.toBe(original);
    expect(pwInput.value.length).toBeGreaterThan(0);
  });

  it('submit fires onSubmit with username, password, role', async () => {
    const onSubmit = vi.fn();
    render(CreateUserDialog, { props: { open: true, onSubmit } });
    await fireEvent.input(screen.getByLabelText(/username/i), { target: { value: 'newbie' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Admin' }));
    await fireEvent.click(screen.getByRole('button', { name: /create user/i }));
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ username: 'newbie', role: 'admin' }));
    expect(onSubmit.mock.calls[0][0].password.length).toBeGreaterThanOrEqual(10);
  });

  it('submit blocked when username empty', async () => {
    const onSubmit = vi.fn();
    render(CreateUserDialog, { props: { open: true, onSubmit } });
    // username is empty by default
    await fireEvent.click(screen.getByRole('button', { name: /create user/i }));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('cancel fires onClose', async () => {
    const onClose = vi.fn();
    render(CreateUserDialog, { props: { open: true, onClose } });
    await fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/components/admin/CreateUserDialog.test.ts --run
```

Expected: FAIL.

- [ ] **Step 3: Implement `CreateUserDialog.svelte`**

```svelte
<!-- web/src/components/admin/CreateUserDialog.svelte -->
<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import Button from '../Button.svelte';

  type Role = 'user' | 'admin';
  type Props = {
    open: boolean;
    onSubmit?: (v: { username: string; password: string; role: Role }) => void;
    onClose?: () => void;
  };
  const { open, onSubmit, onClose }: Props = $props();

  let username = $state('');
  let password = $state(generatePassword());
  let role = $state<Role>('user');

  function generatePassword(): string {
    const words = ['summer','deck','quiet','spark','river','amber','loop','shadow','calm','willow','aspen','ember'];
    const pick = () => words[Math.floor(Math.random() * words.length)];
    const n = Math.floor(1000 + Math.random() * 9000);
    return `${pick()}-${pick()}-${pick()}-${n}`;
  }

  function regenerate() { password = generatePassword(); }

  function submit() {
    if (!username.trim()) return;
    onSubmit?.({ username: username.trim(), password, role });
  }
</script>

<Dialog open={open} title="Create user" onclose={onClose}>
  <div class="body">
    <p class="p">New users sign in with this password and are prompted to enrol in two-factor on first successful sign-in.</p>

    <label class="field">
      <span class="label">Username</span>
      <input type="text" bind:value={username} autocomplete="off" />
    </label>

    <label class="field">
      <span class="label">Initial password</span>
      <div class="field-with-aff">
        <input type="text" bind:value={password} autocomplete="off" />
        <button type="button" class="aff" aria-label="Regenerate password" onclick={regenerate} title="Regenerate">
          <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M14 8a6 6 0 1 1-1.76-4.24"/><path d="M14 2.5V6h-3.5"/></svg>
        </button>
      </div>
      <div class="hint">4-word passphrase · shown once, copy before saving</div>
    </label>

    <div class="field">
      <span class="label">Role</span>
      <div class="seg" role="radiogroup">
        <button type="button" class="seg-btn" class:active={role === 'user'} aria-pressed={role === 'user'} onclick={() => role = 'user'}>User</button>
        <button type="button" class="seg-btn" class:active={role === 'admin'} aria-pressed={role === 'admin'} onclick={() => role = 'admin'}>Admin</button>
      </div>
      <div class="hint">{role === 'admin' ? 'Admins can manage users and view system status.' : 'Users can read, save and manage their own feeds.'}</div>
    </div>
  </div>
  {#snippet footer()}
    <div class="foot-l">password is shown once</div>
    <Button variant="quiet" onclick={onClose}>Cancel</Button>
    <Button variant="primary" onclick={submit}>Create user</Button>
  {/snippet}
</Dialog>

<style>
  .body { display: flex; flex-direction: column; gap: 14px; }
  .p { font-family: var(--sans); font-size: 13.5px; color: var(--ink-2); margin: 0 0 6px; line-height: 1.55; }
  .field { display: flex; flex-direction: column; gap: 6px; }
  .label { font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--ink-3); }
  .field input {
    font-family: var(--mono); font-size: 13px; color: var(--ink);
    border: 1px solid var(--rule); background: var(--bg); border-radius: 4px;
    padding: 8px 10px; outline: none;
  }
  .field input:focus { border-color: var(--ink-4); }
  .field-with-aff { display: flex; align-items: stretch; gap: 6px; }
  .field-with-aff input { flex: 1; }
  .aff {
    width: 34px;
    display: inline-flex; align-items: center; justify-content: center;
    background: var(--bg); border: 1px solid var(--rule); border-radius: 4px;
    color: var(--ink-2); cursor: pointer;
  }
  .aff:hover { color: var(--ink); border-color: var(--ink-4); }
  .hint { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); }
  .seg { display: inline-flex; border: 1px solid var(--rule); border-radius: 4px; overflow: hidden; }
  .seg-btn {
    font-family: var(--sans); font-size: 12.5px; color: var(--ink-2);
    background: var(--bg); border: 0; padding: 6px 12px; cursor: pointer;
  }
  .seg-btn.active { background: var(--ink); color: var(--bg); }
  .foot-l { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-right: auto; }
</style>
```

If the M1 `Dialog` primitive does not expose a `footer` snippet but instead a `foot` slot or `footer` prop with a different name, adapt the snippet reference accordingly. Same for `Button`'s variant prop name.

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/components/admin/CreateUserDialog.test.ts --run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/admin/CreateUserDialog.svelte web/src/components/admin/CreateUserDialog.test.ts
git commit -m "feat(web): CreateUserDialog — username + generated password + role"
```

---

### Task 11: Frontend — `ResetPasswordResultDialog.svelte`

**Files:**
- Create: `web/src/components/admin/ResetPasswordResultDialog.svelte`
- Test: `web/src/components/admin/ResetPasswordResultDialog.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/admin/ResetPasswordResultDialog.test.ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import ResetPasswordResultDialog from './ResetPasswordResultDialog.svelte';

describe('ResetPasswordResultDialog', () => {
  it('displays the temporary password once', () => {
    render(ResetPasswordResultDialog, { props: { open: true, username: 'alice', password: 'amber-loop-shadow-2840' } });
    expect(screen.getByText('amber-loop-shadow-2840')).toBeInTheDocument();
    expect(screen.getByText(/alice/)).toBeInTheDocument();
  });

  it('copy button calls navigator.clipboard.writeText with the password', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(ResetPasswordResultDialog, { props: { open: true, username: 'alice', password: 'pw-123' } });
    await fireEvent.click(screen.getByRole('button', { name: /copy/i }));
    expect(writeText).toHaveBeenCalledWith('pw-123');
  });

  it('Done button fires onClose', async () => {
    const onClose = vi.fn();
    render(ResetPasswordResultDialog, { props: { open: true, username: 'alice', password: 'pw-123', onClose } });
    await fireEvent.click(screen.getByRole('button', { name: /done/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/components/admin/ResetPasswordResultDialog.test.ts --run
```

Expected: FAIL.

- [ ] **Step 3: Implement `ResetPasswordResultDialog.svelte`**

```svelte
<!-- web/src/components/admin/ResetPasswordResultDialog.svelte -->
<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import Button from '../Button.svelte';

  type Props = {
    open: boolean;
    username: string;
    password: string;
    onClose?: () => void;
  };
  const { open, username, password, onClose }: Props = $props();

  let copied = $state(false);
  async function copy() {
    try { await navigator.clipboard.writeText(password); copied = true; setTimeout(() => copied = false, 1500); } catch (_) {}
  }
</script>

<Dialog open={open} title={`Temporary password for ${username}`} onclose={onClose}>
  <div class="body">
    <p>This password works once. <b>{username}</b> will be asked to set a new one on next sign-in. Share it through a secure channel — it will not be shown again.</p>
    <div class="key-row">
      <span class="key">{password}</span>
      <button type="button" class="copy" onclick={copy}>
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="2.5" width="8" height="8" rx="1.2"/><path d="M5.5 13.5h6a1.2 1.2 0 0 0 1.2-1.2v-6"/></svg>
        <span>{copied ? 'Copied' : 'Copy'}</span>
      </button>
    </div>
    <div class="warn">All existing sessions for {username} were just signed out. Any passkeys remain valid.</div>
  </div>
  {#snippet footer()}
    <div class="foot-l">shown once</div>
    <Button variant="primary" onclick={onClose}>Done</Button>
  {/snippet}
</Dialog>

<style>
  .body { display: flex; flex-direction: column; gap: 14px; }
  p { font-family: var(--sans); font-size: 13.5px; color: var(--ink-2); margin: 0; line-height: 1.55; }
  .key-row {
    display: flex; align-items: center; gap: 12px;
    padding: 14px 16px;
    background: var(--bg-soft);
    border: 1px solid var(--rule);
    border-radius: 4px;
  }
  .key {
    font-family: var(--mono); font-size: 15px; letter-spacing: 0.06em;
    color: var(--ink); font-weight: 500; flex: 1;
  }
  .copy {
    display: inline-flex; align-items: center; gap: 6px;
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    color: var(--ink-2);
    background: transparent;
    border: 1px solid var(--rule); border-radius: 3px;
    padding: 5px 9px; cursor: pointer;
  }
  .copy:hover { color: var(--ink); border-color: var(--ink-4); }
  .warn {
    font-family: var(--mono); font-size: 11px; color: var(--ink-3);
    padding: 10px 14px;
    border-left: 2px solid var(--accent);
    background: var(--accent-soft);
  }
  .foot-l { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-right: auto; }
</style>
```

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/components/admin/ResetPasswordResultDialog.test.ts --run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/admin/ResetPasswordResultDialog.svelte web/src/components/admin/ResetPasswordResultDialog.test.ts
git commit -m "feat(web): ResetPasswordResultDialog — one-shot temp password"
```

---

### Task 12: Frontend — `ConfirmDialog.svelte`

**Files:**
- Create: `web/src/components/admin/ConfirmDialog.svelte`
- Test: `web/src/components/admin/ConfirmDialog.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/admin/ConfirmDialog.test.ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import ConfirmDialog from './ConfirmDialog.svelte';

describe('ConfirmDialog', () => {
  it('renders title, body and CTA label', () => {
    render(ConfirmDialog, { props: { open: true, title: 'Delete alice?', body: 'This cannot be undone.', cta: 'Delete alice' } });
    expect(screen.getByText('Delete alice?')).toBeInTheDocument();
    expect(screen.getByText('This cannot be undone.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete alice' })).toBeInTheDocument();
  });

  it('confirm fires onConfirm', async () => {
    const onConfirm = vi.fn();
    render(ConfirmDialog, { props: { open: true, title: 't', body: 'b', cta: 'OK', onConfirm } });
    await fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    expect(onConfirm).toHaveBeenCalled();
  });

  it('cancel fires onCancel', async () => {
    const onCancel = vi.fn();
    render(ConfirmDialog, { props: { open: true, title: 't', body: 'b', cta: 'OK', onCancel } });
    await fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalled();
  });

  it('renders consequence list when supplied', () => {
    render(ConfirmDialog, { props: {
      open: true, title: 't', body: 'b', cta: 'OK',
      list: ['3 passkeys revoked', '42 feeds released', '2,148 records purged'],
    } });
    expect(screen.getByText('3 passkeys revoked')).toBeInTheDocument();
    expect(screen.getByText('42 feeds released')).toBeInTheDocument();
  });

  it('danger flag applies danger variant to CTA', () => {
    render(ConfirmDialog, { props: { open: true, title: 't', body: 'b', cta: 'Delete', danger: true } });
    const btn = screen.getByRole('button', { name: 'Delete' });
    expect(btn).toHaveAttribute('data-variant', 'danger');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/components/admin/ConfirmDialog.test.ts --run
```

Expected: FAIL.

- [ ] **Step 3: Implement `ConfirmDialog.svelte`**

```svelte
<!-- web/src/components/admin/ConfirmDialog.svelte -->
<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import Button from '../Button.svelte';

  type Props = {
    open: boolean;
    title: string;
    body: string;
    cta: string;
    danger?: boolean;
    list?: string[];
    footNote?: string;
    onConfirm?: () => void;
    onCancel?: () => void;
  };
  const { open, title, body, cta, danger = false, list = [], footNote = '', onConfirm, onCancel }: Props = $props();
</script>

<Dialog open={open} title={title} onclose={onCancel}>
  <div class="body">
    <p>{body}</p>
    {#if list.length > 0}
      <ul class="list">
        {#each list as item}
          <li>{item}</li>
        {/each}
      </ul>
    {/if}
  </div>
  {#snippet footer()}
    {#if footNote}<div class="foot-l">{footNote}</div>{/if}
    <Button variant="quiet" onclick={onCancel}>Cancel</Button>
    <Button variant={danger ? 'danger' : 'primary'} data-variant={danger ? 'danger' : 'primary'} onclick={onConfirm}>{cta}</Button>
  {/snippet}
</Dialog>

<style>
  .body { display: flex; flex-direction: column; gap: 12px; }
  p { font-family: var(--sans); font-size: 13.5px; color: var(--ink-2); margin: 0; line-height: 1.55; }
  .list {
    margin: 0; padding: 10px 14px;
    list-style: none;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--bg-soft);
    font-family: var(--mono); font-size: 11.5px; color: var(--ink-2);
  }
  .list li { padding: 4px 0; }
  .foot-l { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-right: auto; }
</style>
```

> Note: the `data-variant` attribute on the Button is **only** for the test to assert the variant string survives. If M1's `Button.svelte` already exposes `data-variant` from its `variant` prop, drop the explicit attribute. If it does not, leave it in.

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/components/admin/ConfirmDialog.test.ts --run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/admin/ConfirmDialog.svelte web/src/components/admin/ConfirmDialog.test.ts
git commit -m "feat(web): ConfirmDialog — title/body/cta/list/danger"
```

---

### Task 13: Frontend — `Admin.svelte` view (access denied + scaffolding) (RED)

**Files:**
- Modify: `web/src/views/Admin.svelte`
- Test: `web/src/views/Admin.test.ts`

- [ ] **Step 1: Write the failing test (access-denied path first)**

```ts
// web/src/views/Admin.test.ts
import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { writable } from 'svelte/store';
import Admin from './Admin.svelte';

// Mock router navigate and auth store
const navigate = vi.fn();
vi.mock('../lib/router', () => ({ navigate: (...a: unknown[]) => navigate(...a) }));

const authStore = writable<{ user: { id: number; username: string; role: 'admin' | 'user' } } | null>({
  user: { id: 1, username: 'liz', role: 'admin' },
});
vi.mock('../lib/auth', () => ({ auth: authStore }));

vi.mock('../lib/api', () => ({
  api: {
    listUsers: vi.fn().mockResolvedValue([]),
    createUser: vi.fn(),
    patchUser: vi.fn(),
    deleteUser: vi.fn(),
    resetUserPassword: vi.fn(),
    disableUserTOTP: vi.fn(),
  },
}));

vi.mock('../lib/admin', () => ({
  getAdminMetrics: vi.fn().mockResolvedValue({
    feedsTotal: 0, feedsOK: 0, entriesTotal: 0, entries24h: 0,
    feedsWithErrors: 0, offendingFeeds: [],
    lastPollAt: null, nextPollAt: null,
  }),
}));

describe('Admin (access denied)', () => {
  beforeEach(() => { navigate.mockReset(); });
  it('non-admin sees access-denied and redirects to /', async () => {
    authStore.set({ user: { id: 2, username: 'alice', role: 'user' } });
    render(Admin, {});
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/'));
    expect(screen.getByText(/access denied/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
```

Expected: FAIL — the current `Admin.svelte` redirects but doesn't render the "Access denied" copy in the new design (it renders `<p>Access denied.</p>` from the old code, but the test asserts the new EmptyState surface).

- [ ] **Step 3: Rewrite `Admin.svelte` minimally — access-denied path only**

```svelte
<!-- web/src/views/Admin.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { get } from 'svelte/store';
  import { auth } from '../lib/auth';
  import { navigate } from '../lib/router';
  import { api } from '../lib/api';
  import { getAdminMetrics, type AdminMetrics } from '../lib/admin';
  import type { AdminUser } from '../lib/types';
  import EmptyState from '../components/EmptyState.svelte';
  import SysGrid from '../components/SysGrid.svelte';
  import ErrorsTable from '../components/ErrorsTable.svelte';
  import UserTable from '../components/UserTable.svelte';
  import AdminToolbar, { type AdminFilter } from '../components/AdminToolbar.svelte';
  import CreateUserDialog from '../components/admin/CreateUserDialog.svelte';
  import ResetPasswordResultDialog from '../components/admin/ResetPasswordResultDialog.svelte';
  import ConfirmDialog from '../components/admin/ConfirmDialog.svelte';

  const authState = $derived(get(auth));
  const isAdmin = $derived(authState?.user?.role === 'admin');
  const currentUserId = $derived(authState?.user?.id ?? -1);

  onMount(() => {
    if (!isAdmin) navigate('/');
  });
</script>

{#if !isAdmin}
  <main class="ts-shell">
    <EmptyState title="Access denied" description="You don't have permission to view this page." />
  </main>
{:else}
  <!-- TODO: render full admin shell — added in next task -->
  <main class="ts-shell"><p>admin</p></main>
{/if}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
```

Expected: PASS (the one test we wrote).

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Admin.svelte web/src/views/Admin.test.ts
git commit -m "feat(web): Admin — access-denied path on .ts-shell"
```

---

### Task 14: Frontend — `Admin.svelte` view (data load + composition) (RED)

**Files:**
- Modify: `web/src/views/Admin.svelte`
- Modify: `web/src/views/Admin.test.ts`

- [ ] **Step 1: Add tests for load + composition**

Append to `Admin.test.ts`:

```ts
import { api } from '../lib/api';
import { getAdminMetrics } from '../lib/admin';

describe('Admin (admin role)', () => {
  beforeEach(() => {
    authStore.set({ user: { id: 1, username: 'liz', role: 'admin' } });
    (api.listUsers as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([
      { id: 1, username: 'liz',   role: 'admin', created_at: 1_700_000_000, disabled_at: null, has_totp: true,  passkey_count: 3 },
      { id: 2, username: 'alice', role: 'user',  created_at: 1_700_000_000, disabled_at: null, has_totp: false, passkey_count: 0 },
    ]);
    (getAdminMetrics as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue({
      feedsTotal: 12, feedsOK: 11, entriesTotal: 1500, entries24h: 80,
      feedsWithErrors: 1, offendingFeeds: ['Phoronix'],
      lastPollAt: 1_700_000_000, nextPollAt: 1_700_000_060,
    });
  });

  it('renders SysGrid + ErrorsTable + UserTable after load', async () => {
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('FEEDS')).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText('liz')).toBeInTheDocument());
    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('Phoronix')).toBeInTheDocument(); // ERRORS sub
  });

  it('filters users by search query', async () => {
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('liz')).toBeInTheDocument());
    const input = screen.getByPlaceholderText('Filter by username') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'ali' } });
    expect(screen.queryByText('liz')).toBeNull();
    expect(screen.getByText('alice')).toBeInTheDocument();
  });

  it('filters users by chip (Admins)', async () => {
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('liz')).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: 'Admins' }));
    expect(screen.getByText('liz')).toBeInTheDocument();
    expect(screen.queryByText('alice')).toBeNull();
  });
});
```

(Make sure `fireEvent` is imported at the top of the test file: `import { ..., fireEvent } from '@testing-library/svelte';`)

- [ ] **Step 2: Run tests to verify they fail**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
```

Expected: FAIL for the three new tests (data not loaded, table not rendered).

- [ ] **Step 3: Implement full composition**

Replace `Admin.svelte` body with:

```svelte
<!-- web/src/views/Admin.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { get } from 'svelte/store';
  import { auth } from '../lib/auth';
  import { navigate } from '../lib/router';
  import { api } from '../lib/api';
  import { getAdminMetrics, type AdminMetrics } from '../lib/admin';
  import type { AdminUser } from '../lib/types';
  import EmptyState from '../components/EmptyState.svelte';
  import SysGrid from '../components/SysGrid.svelte';
  import ErrorsTable from '../components/ErrorsTable.svelte';
  import UserTable from '../components/UserTable.svelte';
  import AdminToolbar, { type AdminFilter } from '../components/AdminToolbar.svelte';
  import CreateUserDialog from '../components/admin/CreateUserDialog.svelte';
  import ResetPasswordResultDialog from '../components/admin/ResetPasswordResultDialog.svelte';
  import ConfirmDialog from '../components/admin/ConfirmDialog.svelte';

  const authState = $derived(get(auth));
  const isAdmin = $derived(authState?.user?.role === 'admin');
  const currentUserId = $derived(authState?.user?.id ?? -1);

  let users = $state<AdminUser[]>([]);
  let metrics = $state<AdminMetrics | null>(null);
  let events = $state<Array<{ time: string; level: 'error' | 'warn' | 'info'; event: string; attrs: Record<string, unknown> }>>([]);
  let loadError = $state('');
  let busy = $state(false);
  let pollInterval = $state<ReturnType<typeof setInterval> | null>(null);
  let nowSec = $state(Math.floor(Date.now() / 1000));

  let filter = $state<AdminFilter>('all');
  let query = $state('');

  let overlay = $state<'create' | 'reset' | 'disable2fa' | 'disableUser' | 'enable' | 'delete' | null>(null);
  let overlayUser = $state<AdminUser | null>(null);
  let tempPassword = $state('');

  const filteredUsers = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return users.filter((u) => {
      if (q && !u.username.toLowerCase().includes(q)) return false;
      switch (filter) {
        case 'admins':   return u.role === 'admin';
        case 'users':    return u.role === 'user';
        case 'disabled': return !!u.disabled_at;
        default:         return true;
      }
    });
  });

  onMount(async () => {
    if (!isAdmin) {
      navigate('/');
      return;
    }
    await Promise.all([loadUsers(), loadMetrics()]);
    pollInterval = setInterval(() => {
      nowSec = Math.floor(Date.now() / 1000);
      loadMetrics();
    }, 60_000);
  });

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
  });

  async function loadUsers() {
    try {
      users = await api.listUsers();
      loadError = '';
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Failed to load users';
    }
  }

  async function loadMetrics() {
    try {
      metrics = await getAdminMetrics();
    } catch (e) {
      // keep showing previous metrics; surface error in cell strip
      if (!metrics) loadError = e instanceof Error ? e.message : 'metrics load failed';
    }
  }

  function onAction(action: 'reset' | 'disable2fa' | 'disableUser' | 'enable' | 'delete', user: AdminUser) {
    overlayUser = user;
    overlay = action;
  }

  function closeOverlay() {
    overlay = null;
    overlayUser = null;
  }

  async function handleCreate(v: { username: string; password: string; role: 'admin' | 'user' }) {
    busy = true;
    try {
      const u = await api.createUser(v.username, v.password, v.role);
      users = [...users, u];
      closeOverlay();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Create user failed';
    } finally {
      busy = false;
    }
  }

  async function handleReset() {
    if (!overlayUser) return;
    busy = true;
    try {
      const r = await api.resetUserPassword(overlayUser.id);
      tempPassword = r.temporary_password;
      // keep overlayUser in scope so the next dialog renders the username
      overlay = 'reset'; // (still); the result dialog branches on tempPassword !== ''
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Reset password failed';
      closeOverlay();
    } finally {
      busy = false;
    }
  }

  async function handleDisable2FA() {
    if (!overlayUser) return;
    busy = true;
    try {
      await api.disableUserTOTP(overlayUser.id);
      await loadUsers();
      closeOverlay();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Disable 2FA failed';
    } finally {
      busy = false;
    }
  }

  async function handleToggleDisabled() {
    if (!overlayUser) return;
    busy = true;
    try {
      const updated = await api.patchUser(overlayUser.id, { disabled: !overlayUser.disabled_at });
      users = users.map((u) => (u.id === updated.id ? updated : u));
      closeOverlay();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Toggle disable failed';
    } finally {
      busy = false;
    }
  }

  async function handleDelete() {
    if (!overlayUser) return;
    busy = true;
    try {
      await api.deleteUser(overlayUser.id);
      users = users.filter((u) => u.id !== overlayUser!.id);
      closeOverlay();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Delete failed';
    } finally {
      busy = false;
    }
  }
</script>

{#if !isAdmin}
  <main class="ts-shell">
    <EmptyState title="Access denied" description="You don't have permission to view this page." />
  </main>
{:else}
  <main class="ts-shell ts-shell-admin">
    <header class="head">
      <div class="eyebrow">Admin</div>
      <h1 class="title">Instance &amp; users</h1>
      <div class="id">
        <span>Signed in as <b>{authState?.user?.username}</b></span>
        <span class="dot" aria-hidden="true"></span>
        <span class="accent">admin</span>
      </div>
    </header>

    {#if loadError}
      <p role="alert" class="err">{loadError}</p>
    {/if}

    <section class="section">
      <div class="section-eyebrow">
        <span>User management</span>
        <span class="rule" aria-hidden="true"></span>
        <span class="tag">{users.length} accounts</span>
      </div>
      <AdminToolbar
        filter={filter}
        query={query}
        count={filteredUsers.length}
        onFilter={(f) => (filter = f)}
        onQuery={(q) => (query = q)}
        onCreate={() => (overlay = 'create')}
      />
      <UserTable users={filteredUsers} currentUserId={currentUserId} onAction={onAction} />
    </section>

    <section class="section">
      <div class="section-eyebrow">
        <span>System status</span>
        <span class="rule" aria-hidden="true"></span>
      </div>
      <SysGrid metrics={metrics} now={nowSec} />
      <div class="errors-head">
        <span>Recent events</span>
        <span class="rule" aria-hidden="true"></span>
      </div>
      <ErrorsTable events={events} />
    </section>
  </main>
{/if}

{#if overlay === 'create'}
  <CreateUserDialog open={true} onSubmit={handleCreate} onClose={closeOverlay} />
{:else if overlay === 'reset' && overlayUser && !tempPassword}
  <ConfirmDialog
    open={true}
    title={`Reset password for ${overlayUser.username}?`}
    body={`A new one-shot password will be generated and shown once. ${overlayUser.username}'s other sessions will be signed out.`}
    cta="Reset password"
    onConfirm={handleReset}
    onCancel={closeOverlay}
  />
{:else if overlay === 'reset' && overlayUser && tempPassword}
  <ResetPasswordResultDialog
    open={true}
    username={overlayUser.username}
    password={tempPassword}
    onClose={() => { tempPassword = ''; closeOverlay(); }}
  />
{:else if overlay === 'disable2fa' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Disable two-factor for ${overlayUser.username}?`}
    body={`This removes the authenticator binding from ${overlayUser.username}'s account. They'll sign in with password only until they re-enrol. Recovery codes are invalidated immediately.`}
    cta="Disable TOTP"
    danger
    footNote="acts immediately"
    onConfirm={handleDisable2FA}
    onCancel={closeOverlay}
  />
{:else if overlay === 'disableUser' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Disable ${overlayUser.username}?`}
    body={`${overlayUser.username} will be signed out everywhere and can't sign back in until you re-enable the account. Their data and feeds are preserved.`}
    cta="Disable account"
    footNote="reversible"
    onConfirm={handleToggleDisabled}
    onCancel={closeOverlay}
  />
{:else if overlay === 'enable' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Re-enable ${overlayUser.username}?`}
    body={`${overlayUser.username} will be able to sign in again with their existing password. Their feeds and saved entries are unchanged.`}
    cta="Re-enable account"
    onConfirm={handleToggleDisabled}
    onCancel={closeOverlay}
  />
{:else if overlay === 'delete' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Delete ${overlayUser.username}?`}
    body={`This permanently removes ${overlayUser.username}, all their feeds, saved entries and sessions. This can't be undone.`}
    cta={`Delete ${overlayUser.username}`}
    danger
    footNote="permanent · cannot be undone"
    list={[
      `${overlayUser.passkey_count} passkey${overlayUser.passkey_count === 1 ? '' : 's'} revoked`,
      'subscribed feeds released',
      'read/save records purged',
    ]}
    onConfirm={handleDelete}
    onCancel={closeOverlay}
  />
{/if}

<style>
  .ts-shell { max-width: 1080px; margin: 0 auto; padding: 24px 32px; }
  .head { padding: 28px 0 18px; border-bottom: 1px solid var(--rule); }
  .eyebrow {
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.14em;
    text-transform: uppercase; color: var(--ink-3); margin-bottom: 8px;
  }
  .title {
    font-family: var(--serif); font-size: 34px; font-weight: 600;
    letter-spacing: -0.02em; line-height: 1.1; margin: 0 0 10px;
  }
  .id {
    font-family: var(--mono); font-size: 11px; color: var(--ink-3);
    display: inline-flex; align-items: center; gap: 8px;
  }
  .id b { color: var(--ink-2); font-weight: 500; }
  .id .dot { width: 3px; height: 3px; border-radius: 50%; background: var(--ink-4); }
  .id .accent { color: var(--accent); }
  .err {
    font-family: var(--mono); font-size: 12px;
    color: #c43a3a; margin: 12px 0;
  }
  :global(html.theme-dark) .err { color: #ec7a7a; }
  .section { padding: 36px 0 6px; }
  .section-eyebrow {
    display: flex; align-items: center; gap: 12px;
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.1em;
    text-transform: uppercase; color: var(--ink-3);
    margin-bottom: 14px;
  }
  .section-eyebrow .rule { flex: 1; height: 1px; background: var(--rule); }
  .section-eyebrow .tag { color: var(--ink-3); }
  .errors-head {
    display: flex; align-items: center; gap: 12px;
    margin-top: 22px; padding-bottom: 8px;
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.1em;
    text-transform: uppercase; color: var(--ink-3);
  }
  .errors-head .rule { flex: 1; height: 1px; background: var(--rule); }
</style>
```

The `events` array is currently empty by default — at present, the backend `/api/v1/status` returns recent errors but the metrics endpoint we added does not. **The errors table reads from `/api/v1/status.recent_errors` via the existing `getStatus` helper.** If M1 deleted `lib/status.ts`, restore the helper or fold it into `lib/admin.ts` as a second function `getRecentEvents()` that calls `/api/v1/status` and maps the `recent_errors` field. Add a tiny test for this. **Sub-step 3.5 below covers that.**

- [ ] **Step 3.5: Add `getRecentEvents()` to `lib/admin.ts`**

Append to `web/src/lib/admin.ts`:

```ts
export type AdminEvent = {
  time: string;
  level: 'error' | 'warn' | 'info';
  event: string;
  attrs: Record<string, unknown>;
};

export async function getRecentEvents(): Promise<AdminEvent[]> {
  const resp = await fetch('/api/v1/status');
  if (!resp.ok) throw new Error(`status: ${resp.status}`);
  const j = await resp.json();
  return j.recent_errors ?? [];
}
```

Add a Vitest case to `web/src/lib/__tests__/admin.test.ts`:

```ts
import { getRecentEvents } from '../admin';

describe('getRecentEvents', () => {
  it('returns recent_errors array', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      recent_errors: [{ time: '2026-05-11T10:00:00Z', level: 'error', event: 'x', attrs: {} }],
    }), { status: 200 }));
    const events = await getRecentEvents();
    expect(events).toHaveLength(1);
    expect(events[0].event).toBe('x');
  });
});
```

Then in `Admin.svelte`, change the `events` initialization and add `getRecentEvents` to the parallel load:

```ts
import { getAdminMetrics, getRecentEvents, type AdminMetrics, type AdminEvent } from '../lib/admin';
// ...
let events = $state<AdminEvent[]>([]);
// ...
onMount(async () => {
  if (!isAdmin) { navigate('/'); return; }
  await Promise.all([loadUsers(), loadMetrics(), loadEvents()]);
  pollInterval = setInterval(() => {
    nowSec = Math.floor(Date.now() / 1000);
    loadMetrics();
    loadEvents();
  }, 60_000);
});

async function loadEvents() {
  try { events = await getRecentEvents(); } catch (_) { /* keep previous */ }
}
```

Update the `vi.mock('../lib/admin', ...)` in `Admin.test.ts` to also stub `getRecentEvents: vi.fn().mockResolvedValue([])`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
pnpm --dir web test -- src/lib/__tests__/admin.test.ts --run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Admin.svelte web/src/views/Admin.test.ts web/src/lib/admin.ts web/src/lib/__tests__/admin.test.ts
git commit -m "feat(web): Admin — compose SysGrid/ErrorsTable/UserTable + dialogs"
```

---

### Task 15: Frontend — mutation flow tests

**Files:**
- Modify: `web/src/views/Admin.test.ts`

- [ ] **Step 1: Add mutation-flow tests**

```ts
import { api } from '../lib/api';

describe('Admin (mutations)', () => {
  beforeEach(() => {
    authStore.set({ user: { id: 1, username: 'liz', role: 'admin' } });
    (api.listUsers as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([
      { id: 1, username: 'liz', role: 'admin', created_at: 1_700_000_000, disabled_at: null, has_totp: true,  passkey_count: 0 },
      { id: 2, username: 'alice', role: 'user', created_at: 1_700_000_000, disabled_at: null, has_totp: false, passkey_count: 0 },
    ]);
    (api.createUser as ReturnType<typeof vi.fn>).mockReset();
    (api.deleteUser as ReturnType<typeof vi.fn>).mockReset();
    (api.resetUserPassword as ReturnType<typeof vi.fn>).mockReset();
    (api.disableUserTOTP as ReturnType<typeof vi.fn>).mockReset();
    (api.patchUser as ReturnType<typeof vi.fn>).mockReset();
  });

  it('Create user dialog → submit → API call → row appended', async () => {
    (api.createUser as ReturnType<typeof vi.fn>).mockResolvedValue({
      id: 3, username: 'newbie', role: 'user', created_at: 1_700_000_100, disabled_at: null, has_totp: false, passkey_count: 0,
    });
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('liz')).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /create user/i }));
    await fireEvent.input(screen.getByLabelText(/username/i), { target: { value: 'newbie' } });
    await fireEvent.click(screen.getByRole('button', { name: /create user/i }));
    await waitFor(() => expect(api.createUser).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByText('newbie')).toBeInTheDocument());
  });

  it('Reset password → confirm → result dialog shows temp password', async () => {
    (api.resetUserPassword as ReturnType<typeof vi.fn>).mockResolvedValue({ temporary_password: 'one-shot-pw' });
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('urow-action-reset-2'));
    // first confirm dialog
    await fireEvent.click(screen.getByRole('button', { name: 'Reset password' }));
    await waitFor(() => expect(screen.getByText('one-shot-pw')).toBeInTheDocument());
  });

  it('Delete user → confirm → row removed', async () => {
    (api.deleteUser as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('urow-action-delete-2'));
    await fireEvent.click(screen.getByRole('button', { name: /^Delete alice$/i }));
    await waitFor(() => expect(api.deleteUser).toHaveBeenCalledWith(2));
    await waitFor(() => expect(screen.queryByText('alice')).toBeNull());
  });

  it('Disable 2FA → confirm → API call', async () => {
    // alice has no totp; use liz (current user) — but you can't self-disable-2FA in current model
    // so swap fixtures: make alice have totp
    (api.listUsers as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, username: 'liz', role: 'admin', created_at: 1_700_000_000, disabled_at: null, has_totp: true,  passkey_count: 0 },
      { id: 2, username: 'alice', role: 'user', created_at: 1_700_000_000, disabled_at: null, has_totp: true, passkey_count: 0 },
    ]);
    (api.disableUserTOTP as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('urow-action-disable2fa-2'));
    await fireEvent.click(screen.getByRole('button', { name: /disable totp/i }));
    await waitFor(() => expect(api.disableUserTOTP).toHaveBeenCalledWith(2));
  });

  it('Disable user → patchUser({disabled:true})', async () => {
    (api.patchUser as ReturnType<typeof vi.fn>).mockResolvedValue({
      id: 2, username: 'alice', role: 'user', created_at: 1_700_000_000, disabled_at: 1_700_500_000, has_totp: false, passkey_count: 0,
    });
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('urow-action-disableUser-2'));
    await fireEvent.click(screen.getByRole('button', { name: /disable account/i }));
    await waitFor(() => expect(api.patchUser).toHaveBeenCalledWith(2, { disabled: true }));
  });
});
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
```

Expected: all PASS. If any flow fails because the component wires events differently, fix the component, not the test (the test pins the intended behaviour).

- [ ] **Step 3: Commit**

```bash
git add web/src/views/Admin.test.ts
git commit -m "test(web): Admin mutation flows — create/reset/delete/disable2fa/disable"
```

---

### Task 16: Frontend — metric-polling lifecycle test

**Files:**
- Modify: `web/src/views/Admin.test.ts`

- [ ] **Step 1: Add polling test**

```ts
describe('Admin (metric polling)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    authStore.set({ user: { id: 1, username: 'liz', role: 'admin' } });
    (api.listUsers as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([]);
    (getAdminMetrics as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue({
      feedsTotal: 1, feedsOK: 1, entriesTotal: 0, entries24h: 0,
      feedsWithErrors: 0, offendingFeeds: [],
      lastPollAt: null, nextPollAt: null,
    });
  });
  afterEach(() => vi.useRealTimers());

  it('polls metrics every 60s and stops on unmount', async () => {
    const { unmount } = render(Admin, {});
    // initial load
    await vi.runOnlyPendingTimersAsync();
    expect(getAdminMetrics).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(60_000);
    await vi.runOnlyPendingTimersAsync();
    expect(getAdminMetrics).toHaveBeenCalledTimes(2);
    unmount();
    vi.advanceTimersByTime(60_000);
    await vi.runOnlyPendingTimersAsync();
    expect(getAdminMetrics).toHaveBeenCalledTimes(2); // no further calls after unmount
  });
});
```

Add `afterEach` to the test file's imports if not already there:
```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
```

- [ ] **Step 2: Run test to verify it passes**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/Admin.test.ts
git commit -m "test(web): Admin — metric polling cycles and stops on unmount"
```

---

### Task 17: Manual smoke checklist + svelte-check

- [ ] **Step 1: Build the SPA and run the full suite**

```bash
make test
```

Expected: PASS. If a Go test failed because the SPA bundle wasn't built (per CLAUDE.md "Build coupling" section), this command both builds and tests.

- [ ] **Step 2: Run svelte-check**

```bash
pnpm --dir web run check
```

Expected: zero errors.

- [ ] **Step 3: Manual smoke (desktop, three themes)**

```bash
make dev   # Go on :8080 + Vite on :5173
```

In a browser:

- Log in as an admin user. Open `/admin`.
- Verify the SysGrid renders all four cells (FEEDS / ENTRIES / ERRORS / POLL) with non-`—` numbers.
- Verify the errors table renders recent events (or "No recent events").
- Click "Create user" — dialog opens, type `smokey`, click "Create user". Expect the new row to appear.
- Click "Reset password" on `smokey` — first dialog asks for confirmation, then the result dialog shows a temp password with a copy button. Click copy, paste somewhere to verify clipboard contents.
- Click "Disable 2FA" on a user without TOTP — button is disabled (cannot click).
- Click "Disable" on `smokey` — confirm dialog opens. Confirm. Row pill goes to "disabled" and the "Disable" button becomes "Re-enable".
- Click "Re-enable" → confirm → pill back to "active".
- Click "Delete" on `smokey` → confirm dialog shows consequence list → click "Delete smokey" → row gone.
- Try to delete or disable yourself — buttons are disabled.
- Toggle each theme (`html.theme-light`, `html.theme-dark`, `html.theme-sepia`) via the AccountMenu — admin chrome (grid borders, ERR row warn colour, pill colours, dialog overlays) reads correctly on all three.
- Reload the page logged in as a non-admin user, navigate to `/admin` manually — `Access denied` empty state, redirected to `/`.

- [ ] **Step 4: Manual smoke (mobile shell)**

In Chrome DevTools, toggle device emulation (iPhone 13).

- Log in as admin. Open the More sheet → tap Admin.
- The admin page renders on the mobile shell. The 4-cell grid wraps to 2×2.
- Action buttons are tappable; dialogs scale to the viewport.

- [ ] **Step 5: Commit any small fixes if smoke uncovered them**

If a token or class adjustment is needed, commit that as a follow-up (one small commit per concern, no scope creep).

---

## Skills and tools for implementers

- **`superpowers:test-driven-development`** — every behavioural change starts with a red test (Task 1, 3, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16).
- **`superpowers:verification-before-completion`** — run `make test` + `pnpm --dir web run check` + manual smoke in Task 17 before claiming done.
- **`svelte-runes`** — `$state`, `$derived`, `$derived.by`, `$props` in every new Svelte component. The state machinery in `Admin.svelte` (filtered users, overlay routing, polling) is the densest user of runes in this milestone.
- **`svelte-styling`** — scoped `<style>` blocks in every component, using `:global(html.theme-dark)` for theme-conditional rules. No global CSS additions; the brand spec's selectors get renamed inside each component.
- **`svelte-template-directives`** — `{@const}` for per-row derived values in `UserTable.svelte`, `{#snippet}` for the Dialog `footer` snippets in dialogs.
- **`golang-database`** — for `internal/db/metrics.go`: prepared queries, `sql.NullInt64` for nullable aggregates, `rows.Close()` deferred, `rows.Err()` after iteration. No new schema changes — relies on existing columns.
- **`golang-testing`** — table-driven tests in `internal/db/metrics_test.go`, helpers reused from `internal/db/*_test.go`, `require.NoError`, fixed clock (pass `time.Time` argument rather than calling `time.Now()` inside the function).
- **`golang-error-handling`** — `fmt.Errorf("count feeds: %w", err)` for wrapping, return early on each query error.
- **`tdd`** (general) — red-green-refactor; every commit is one small step.
- **`golang-code-style`** — `gofmt`, lower-camel local variables, exported names like `AdminMetrics` per Go naming convention.
- **MCP `context7:query-docs`** — when in doubt about Svelte 5 testing-library patterns (e.g., `fireEvent.click` on a `<button>` that lives behind a Dialog primitive overlay), query for current `@testing-library/svelte` examples before adapting.

---

## Acceptance criteria

A milestone PR is mergeable when **all** of these are true:

1. **Backend route exists.** `curl -b cookie http://localhost:8080/api/v1/admin/metrics` returns a 200 JSON document containing `feeds_total`, `feeds_ok`, `entries_total`, `entries_24h`, `feeds_with_errors`, `offending_feeds`, `last_poll_at`, `next_poll_at`. Non-admin gets 403.
2. **SysGrid visible.** Visiting `/admin` as an admin shows a `.ts-shell` page with two sections. Section 2 contains a four-cell grid: each cell's first line matches the brand spec §6.7 label (FEEDS / ENTRIES / ERRORS / POLL), each cell shows a primary number/string and a secondary sub line.
3. **ErrorsTable visible.** Below the grid: a "Recent events" eyebrow, then a table with one row per ring-buffer event. Each row shows timestamp (HH:MM:SS, mono ink-3), a level tag (`error` / `warn` / `info`), and the event message (mono ink, ellipsis on overflow).
4. **UserTable functional.** Section 1 shows the AdminToolbar (search input + four filter chips + Create user button) and a grid of `.ts-urow` rows. Each row shows username + role + created date + status pill + 2FA flag + passkey count + four/five action buttons.
5. **Five overlays functional.**
   - Create user: opens a Dialog, accepts username + generated password + role, calls `POST /api/v1/admin/users` on confirm.
   - Reset password: opens a ConfirmDialog first; on confirm, calls `POST /api/v1/admin/users/:id/password-reset`; opens a ResetPasswordResultDialog showing the temporary password once with a copy button.
   - Disable 2FA: opens a ConfirmDialog (danger variant); on confirm, calls `POST /api/v1/admin/users/:id/disable-totp`.
   - Disable / Re-enable: opens a ConfirmDialog; on confirm, calls `PATCH /api/v1/admin/users/:id` with `{disabled: true/false}`.
   - Delete: opens a ConfirmDialog (danger variant) with consequence list; on confirm, calls `DELETE /api/v1/admin/users/:id` and removes the row.
6. **Self-action guards.** Current user cannot delete or disable themselves (buttons disabled). User without TOTP cannot Disable 2FA (button disabled).
7. **Access-denied path.** Non-admin visiting `/admin` sees an EmptyState "Access denied" and is redirected to `/`.
8. **Polling.** SysGrid + ErrorsTable refresh every 60s while the page is mounted. Clearing on unmount is verified by a vitest test.
9. **Three themes.** Light, dark, sepia all render correctly — verified visually by toggling the AccountMenu theme. The warn ink colour (`#c97a1a` / `#f0a655` dark) applies to the ERRORS cell when `feedsWithErrors > 0`. The error level row has the `level-error` background tint.
10. **Mobile parity.** Admin page renders on the mobile shell. The 4-cell grid collapses to 2×2 at narrow viewports. Dialogs scale.
11. **Tests pass.** `make test` and `pnpm --dir web run check` both clean. `pnpm --dir web test` includes new test files for `admin.test.ts`, `SysGrid.test.ts`, `ErrorsTable.test.ts`, `UserTable.test.ts`, `AdminToolbar.test.ts`, `CreateUserDialog.test.ts`, `ResetPasswordResultDialog.test.ts`, `ConfirmDialog.test.ts`, `Admin.test.ts`. New Go test files cover `metrics_test.go` (db + api).
12. **Spec match.** Brand spec §6.7 selectors map clearly to component classes — even though renamed (`.ts-sys-grid` → `.grid`), the rendered output (4 cells, mono labels, mono values, mono subs, rule border) is faithful.

---

## Verification commands

Run before claiming the task complete:

```bash
# Full build + Go race tests (also builds web/dist)
make test

# Frontend type-check
pnpm --dir web run check

# Just the new Vitest suites
pnpm --dir web test -- src/lib/__tests__/admin.test.ts --run
pnpm --dir web test -- src/components/SysGrid.test.ts --run
pnpm --dir web test -- src/components/ErrorsTable.test.ts --run
pnpm --dir web test -- src/components/UserTable.test.ts --run
pnpm --dir web test -- src/components/AdminToolbar.test.ts --run
pnpm --dir web test -- src/components/admin/CreateUserDialog.test.ts --run
pnpm --dir web test -- src/components/admin/ResetPasswordResultDialog.test.ts --run
pnpm --dir web test -- src/components/admin/ConfirmDialog.test.ts --run
pnpm --dir web test -- src/views/Admin.test.ts --run

# Just the new Go tests
go test ./internal/db -run TestGetAdminMetrics -race -v
go test ./internal/api -run TestAdminMetrics -race -v

# Smoke
make dev
# then navigate manually per Task 17 Step 3+4
```

---

## Risks

1. **Umbrella spec said "no new endpoints required" for M7; this plan adds one.** The umbrella spec (§2.4 + §5 row M7) claims existing endpoints expose the metric data. Inspection shows otherwise: `/api/v1/status` exposes poll info and recent errors but **not** feed/entry totals or feeds-with-errors aggregates, and `/api/v1/subscriptions` is per-user (the requesting admin would only see their own feeds). The cleanest answer is one small additive endpoint, `GET /api/v1/admin/metrics`, gated by `authedAdmin`. The umbrella spec explicitly allows "narrow additions a milestone explicitly justifies" — this plan does. **Mitigation if blocked:** if the team decides no new endpoint, fall back to client-side computation of FEEDS / ENTRIES / ERRORS by calling `GET /api/v1/admin/users` then for each admin user calling `GET /api/v1/subscriptions` and `GET /api/v1/entries?limit=...` — this is O(users × feeds) requests, blows the request budget, and **does not see other admins' feeds without an admin override on subscription listing** that doesn't exist either. Verdict: ship the endpoint.

2. **`SystemStatus.svelte` deletion in M1 may leave dangling references.** Per the umbrella spec §3.3, M1 deletes `SystemStatus.svelte` and folds its content into Admin (M7). Confirm in Task 0 that M1 also deleted or repurposed `web/src/lib/status.ts`; if not, this plan retains and uses it via `getRecentEvents()` (Task 14 Step 3.5). If M1 deleted it outright, restore the minimal `getStatus` shape inside `lib/admin.ts` as planned.

3. **Existing `Admin.svelte` has no test file.** This plan adds `Admin.test.ts` from scratch — there's no prior assertion of behaviour to regress against. **Mitigation:** the manual smoke checklist (Task 17 Step 3) verifies the legacy behaviour persists: every action button still calls the same API endpoint and produces the same DB effect. Run the smoke before merging.

4. **Brand spec divergence in the JSX mockup.** `ui_design/tap-admin.jsx`'s `SystemStatus` cells (Version / Uptime / Database / Active polls) do not match the brand spec §6.7's authoritative list (FEEDS / ENTRIES / ERRORS / POLL). This plan follows the **brand spec** as the user explicitly required. If the team prefers the JSX cells, swap the SysGrid props — the metric endpoint already returns enough data to support either set, plus we can extend `statusHandler` to expose version/uptime if needed without breaking the metrics endpoint.

5. **Polling interval and live-update.** The spec text under "auto-refreshing every 5s" in the JSX is **not** in the brand spec §6.7 and is not adopted. We chose 60s to mirror the existing `SystemStatus.svelte` cadence and avoid hot-loading the metrics endpoint. If a future milestone wants 5s, swap the literal — the test is already parameterised on `vi.advanceTimersByTime(60_000)` and is the only place that pin needs updating.

6. **`onMount` timing in the access-denied path.** The `navigate('/')` call fires inside `onMount`, so a tick of the EmptyState renders before the route changes. Vitest verifies both the navigate call and the EmptyState — both happen, which is correct. No flash for the user because the router unmounts the Admin view in the next macrotask. If the parent router or Svelte 5 behaviour changes this timing, the test will fail and the access-denied behaviour should be moved to a `$derived` check inside the template's top-level `{#if}` to keep the EmptyState mounted regardless of timing.

7. **Filter chip behaviour is client-side only.** The current `/api/v1/admin/users` does not accept role/disabled query params. If user count grows large enough that loading the whole list is expensive, that's a future milestone. The redesign brief's filter chips already exist on the design — implementing them server-side without a corresponding spec request is scope creep.

8. **Migration cost.** No schema change required — all four metric values are computable from existing tables and columns (`subscriptions.error_count`, `subscriptions.last_poll_at`, `subscriptions.next_poll_at`, `entries.created_at`/`published_at`). Verify in Task 2 that the entries timestamp column we query is the one populated on insert (likely `created_at` per the polling pipeline; if not, swap to `published_at`).

---

## Self-review

1. **Spec coverage check.** Cells match brand spec §6.7 (FEEDS / ENTRIES / ERRORS / POLL). Errors table matches (timestamp · message · code chip, though the spec says "code chip" we render `level` which is the semantic-equivalent; if the team wants a distinct "code" field, that's a richer ring buffer event and a separate milestone). User management surface preserves all existing functionality (list, create, reset password, disable 2FA, disable/re-enable, delete). Adopts M1 primitives (Button, Field, Dialog, Chip, EmptyState). Mobile + three themes covered. Access denied path covered. Backend addition justified.

2. **Placeholder scan.** No "TBD" / "implement later" / "add appropriate error handling". Every test ships full code. Every implementation step shows full code. The few notes that adapt to M1's actual API surface (`Button` variants, `Dialog` footer-snippet name) are explicit conditionals, not placeholders.

3. **Type consistency.** `AdminMetrics` field names are camelCase in the TS type and snake_case in the JSON; the mapping in `getAdminMetrics()` is explicit. `Event` type in `ErrorsTable.svelte` and `AdminEvent` in `lib/admin.ts` share the same shape. `Action` enum in `UserTable.svelte` is the same set as the `overlay` union in `Admin.svelte`.

Plan is complete.
