# M-Redesign-7 (Admin) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild `views/Admin.svelte` on the new `.ts-shell` design language: a four-cell instance-wide metric grid (FEEDS / ENTRIES / ERRORS / POLL), a recent-events errors table sourced from the existing ring buffer, and the existing user-management surface (list / create / reset password / disable 2FA / disable / re-enable / delete) restyled with M1 primitives and new chrome.

**Architecture:** Frontend rewrite of one view + a small *extension* of one existing backend endpoint. The admin page lives on the centred `.ts-shell` and uses the `.ts-set` page-shell pattern (eyebrow + title + id strip) shared with Settings. The metric grid (`.ts-sys-grid`) and errors table (`.ts-sys-errors`) replace `SystemStatus.svelte` (which M1 deletes). The user table (`.ts-utable`) becomes a grid of `.ts-urow` rows with a toolbar (search input, filter chips, "Create user" primary button) above. All overlays (create user, reset password, disable 2FA, disable, re-enable, delete) use M1's `Dialog` primitive. Server-side, the existing admin-only `/api/v1/status` endpoint is extended with six aggregate fields (`feeds_total`, `feeds_ok`, `feeds_with_errors`, `offending_feeds`, `entries_total`, `entries_24h`) — no new route, no new endpoint registration. The umbrella spec §2.4 explicitly directs this extension; the existing `recent_errors` field continues to serve the errors table.

**Tech Stack:** Svelte 5 runes (`$state`, `$derived`, `$effect`), TypeScript, Vitest + `@testing-library/svelte` for component tests, scoped Svelte `<style>` blocks driven by global tokens, Go 1.26+ `net/http` for the extended handler, `modernc.org/sqlite` queries (no CGO), `testify/require` for Go tests.

---

## Scope

### In scope

- Rewrite `web/src/views/Admin.svelte` on `.ts-shell` for admin-only role.
- Build `web/src/components/SysGrid.svelte` (4-cell metric grid).
- Build `web/src/components/ErrorsTable.svelte` (timestamp · level chip · message rows).
- Build `web/src/components/UserTable.svelte` (grid of `.ts-urow` rows with header).
- Build `web/src/components/AdminToolbar.svelte` (search input + filter chips + "Create user" button).
- Build dialog content components (`CreateUserDialog.svelte`, `ResetPasswordResultDialog.svelte`, `ConfirmDialog.svelte`) that wrap the M1 `Dialog` primitive.
- Repurpose `web/src/lib/status.ts` to add the new aggregate fields to `StatusResponse`. (After M1 deletes `SystemStatus.svelte`, the file otherwise becomes orphaned.)
- Extend backend: `internal/db/metrics.go` with `GetAdminMetrics(ctx, db, now)` aggregating feeds/entries/errors.
- Extend backend: `internal/api/status.go` — augment `statusResponse` with the new fields, call `db.GetAdminMetrics` inside `statusHandler`, no new route.
- Access-denied path: non-admin users navigating to `/admin` see a `<EmptyState>` "Access denied" and the route never renders admin-only chrome.
- All three themes (light, dark, sepia) verified for the new chrome.
- Mobile shell parity: admin tab in `MobileMoreSheet` for admin role only; admin page renders on `.ts-shell` (full-bleed mobile scroll).

### Out of scope

- A new `/api/v1/admin/metrics` route. The umbrella spec §2.4 explicitly directs M7 to extend `statusResponse` instead.
- Live-update via SSE or WebSocket — the metric grid polls every 60s via `setInterval`, matching the existing `SystemStatus.svelte` cadence (`SystemStatus.svelte:20` — `setInterval(refresh, 60_000)`).
- "Re-run all polls now" button from the JSX mockup — not in the brand spec §6.7 metric grid.
- "Open full logs" button — not in the brand spec §6.7; the errors table is the surface.
- Per-user metric breakdown — admin sees instance-wide aggregates only.
- Admin role change via PATCH — the existing `patchUser` API accepts `role` but the UI does not expose it (matches current Admin.svelte behaviour).
- Filter chip behaviour (All / Admins / Users / Disabled) is **in scope** but as client-side filters over the loaded user list (no API filter param).
- A "Download .txt" button on the reset-password result dialog — copy button only. Downloading a temporary password to disk creates a long-lived file in `~/Downloads/` that the admin will forget to delete; copy-and-paste leaves no artefact.
- Widening the ring buffer's level vocabulary. The buffer emits `"warn"` and `"error"` only (`internal/ring/handler.go:22` filters at `slog.LevelWarn`); the errors table renders only those two levels.

---

## Files modified / created / deleted

### Created — frontend

- `web/src/components/SysGrid.svelte` — four-cell metric grid (FEEDS / ENTRIES / ERRORS / POLL). Pure presentation, accepts a `status` prop sourced from the extended `StatusResponse`. Uses `.ts-sys-grid` / `.ts-sys-cell` / `.ts-sys-cell-l` / `.ts-sys-cell-v` / `.ts-sys-cell-sub` classnames (scoped).
- `web/src/components/SysGrid.test.ts` — tests for relative-time rendering, "n ok" sub line, loading/error states.
- `web/src/components/ErrorsTable.svelte` — recent-events table. Accepts `events` prop (the `recent_errors` field of `StatusResponse`). Uses `.ts-sys-errors` / `.ts-sys-err2` / `.ts-sys-err2-t` / `.ts-sys-err2-m` / `.ts-lvl` classnames (scoped). EmptyState when no events. Levels: `warn` and `error` only.
- `web/src/components/ErrorsTable.test.ts` — tests for level-tag rendering (warn/error), timestamp formatting, empty state.
- `web/src/components/UserTable.svelte` — user list (header row + `.ts-urow` rows). Accepts `users` prop and `currentUserId` prop. Emits action events (`reset`, `disable2fa`, `disableUser`, `enable`, `delete`) via a single `onAction` callback prop.
- `web/src/components/UserTable.test.ts` — tests for "you" badge, disabled state styling, disabled-button states (can't delete self, can't disable self, can't disable-2fa when no totp), action callbacks fire with the right user.
- `web/src/components/AdminToolbar.svelte` — search input + filter chips + "Create user" primary button. Emits filter + query state and a `create` event.
- `web/src/components/AdminToolbar.test.ts` — tests for chip activation, query input, create callback.
- `web/src/components/admin/CreateUserDialog.svelte` — wraps `Dialog`. Form: username, generated password (with regenerate button), role segmented. Emits `submit({username, password, role})` / `close`.
- `web/src/components/admin/CreateUserDialog.test.ts` — tests for required field validation, role default = "user", generate-password button, submit emits the right payload.
- `web/src/components/admin/ResetPasswordResultDialog.svelte` — wraps `Dialog`. Shows the temporary password once with a Copy button and Done. **No download button.**
- `web/src/components/admin/ResetPasswordResultDialog.test.ts` — tests for copy-to-clipboard call, close fires, no download button rendered.
- `web/src/components/admin/ConfirmDialog.svelte` — wraps `Dialog`. Configurable title, body, optional list (e.g., "3 passkeys revoked"), CTA label, danger flag. Emits `confirm` / `cancel`.
- `web/src/components/admin/ConfirmDialog.test.ts` — tests for danger styling, confirm fires, cancel fires, consequence-list rendering.
- `web/src/views/Admin.test.ts` — tests for: non-admin redirects to `/`, admin sees grid + table + errors, "Create user" opens dialog and on submit reloads list, "Reset password" opens result dialog with temp password, "Disable 2FA" opens confirm and on confirm reloads list, "Delete" opens confirm with consequence list and on confirm removes row, status polling cycles, polling cleared on unmount.

### Created — backend

- `internal/db/metrics.go` — `GetAdminMetrics(ctx, db, now)` returns:
  - `FeedsTotal` (int): `SELECT COUNT(*) FROM subscriptions`
  - `FeedsOK` (int): `SELECT COUNT(*) FROM subscriptions WHERE error_count = 0`
  - `EntriesTotal` (int): `SELECT COUNT(*) FROM entries`
  - `Entries24h` (int): `SELECT COUNT(*) FROM entries WHERE fetched_at >= ?` (now - 86400). **`fetched_at` not `published_at` or `created_at`** — `entries` has only `published_at` and `fetched_at` per `internal/db/migrations/0001_initial.sql:23-25`; the desired metric is "entries we received in the last 24h" (poller-fed), not "entries the feed dated in the last 24h" (origin-clock), so `fetched_at` is the right column.
  - `FeedsWithErrors` (int): `SELECT COUNT(*) FROM subscriptions WHERE error_count > 0`
  - `OffendingFeeds` ([]string): up to three feed titles `WHERE error_count > 0 ORDER BY error_count DESC, id ASC LIMIT 3`
- `internal/db/metrics_test.go` — table-driven tests for empty DB, populated DB, offending-feeds ordering and limit, 24h window edge.

### Modified — frontend

- `web/src/lib/status.ts` — extend `StatusResponse` to add the six new fields. Repurpose ownership: this file is currently consumed only by `SystemStatus.svelte` (deleted in M1) and its own test (`web/src/lib/__tests__/status.test.ts`). After M1 it becomes orphaned; M7 takes it over as the admin status client. (Verified at plan-time on `main`: `grep -rln 'from.*lib/status' web/src` shows only `SystemStatus.svelte` and the test file. `PollerStatus.svelte` does NOT import from it.)
- `web/src/lib/__tests__/status.test.ts` — update assertions for the new fields.
- `web/src/views/Admin.svelte` — full rewrite. Composes `<AdminToolbar>`, `<UserTable>`, `<SysGrid>`, `<ErrorsTable>`, dialogs. Polls `/api/v1/status` every 60s. Reloads user list after each mutation. Filters users client-side by search query + role/disabled chips.
- `web/src/lib/api.ts` — no changes (the per-user admin endpoints stay).
- `web/src/styles/tokens.css` (already exists per M1) — no changes needed; all colours used (`--accent`, `--rule`, `--ink-3`, `--bg-soft`) are tokens M1 already vends.

### Modified — backend

- `internal/api/status.go` — extend `statusResponse` struct with `FeedsTotal`, `FeedsOK`, `FeedsWithErrors`, `OffendingFeeds`, `EntriesTotal`, `Entries24h`. Inside `statusHandler`, after the existing DB ping, call `db.GetAdminMetrics(r.Context(), deps.db, time.Now())` and copy the values into the response. On error, log and keep zero values (the rest of the response — version/uptime/db/polls/recent_errors — remains intact); the metric cells render `—` if zeros arrive, which is graceful degradation.
- `internal/api/status_test.go` — first locate where the `/api/v1/status` handler is currently exercised (likely `internal/api/api_test.go` or `internal/api/middleware_test.go`). If a dedicated `status_test.go` does not exist yet, create one. Add assertions for the new fields, including a "empty offending_feeds serialises as `[]` not `null`" case.

### Deleted

- None. M1 deletes `web/src/components/SystemStatus.svelte`. `web/src/lib/status.ts` is *repurposed* by this milestone, not deleted (no other consumer remains after M1, and we replace its sole job — fetching `/api/v1/status` — with a richer typed client).

---

## TDD posture per task

Per `CLAUDE.md`: TDD non-negotiable for branches, state, error handling; pure CSS/markup exempt.

| Task | TDD required? | Reason |
|---|---|---|
| Backend metrics DB query | **Yes** | Aggregate SQL — must verify counts and ordering with seeded fixtures. |
| Backend status handler extension | **Yes** | New fields in the JSON shape; graceful degradation on aggregate error. |
| `SysGrid.svelte` rendering | **Yes** (rendering branches) | Loading / error / null-status states; relative-time computation; "n ok" sub line conditional. |
| `ErrorsTable.svelte` rendering | **Yes** | Level-tag colour branch (`warn` / `error`), empty state, timestamp formatting. |
| `UserTable.svelte` | **Yes** | "you" badge, disabled-button branches (no-self-delete, no-self-disable, no-disable-2fa-without-totp), callback fan-out. |
| `AdminToolbar.svelte` | **Yes** | Chip activation, query input, create-callback. |
| `CreateUserDialog.svelte` | **Yes** | Required-field validation, generate-password button, submit payload. |
| `ResetPasswordResultDialog.svelte` | **Yes** | Copy-to-clipboard behaviour, single-shot display, no download button. |
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
- Read: `web/src/components/AppShell.svelte` (M1 deliverable — owns `isMobile` detection)

- [ ] **Step 1: Confirm primitive API surface**

Open each of `Button.svelte`, `Field.svelte`, `Chip.svelte`, `Dialog.svelte`, `EmptyState.svelte` and write down the props each exposes. This plan assumes:

- `Button`: `variant: 'default'|'primary'|'accent'|'danger'|'quiet'`, `size`, `icon` (snippet), `disabled`, `onclick`.
- `Field`: `label`, `value` (`$bindable`), `description`, `mono` flag, `is-stacked` flag.
- `Chip`: `active` flag, `tone: 'neutral'|'error'|'warning'`, `onclick`.
- `Dialog`: `open` (`$bindable`), `title`, body via `children` snippet, `footer` snippet, `wide` flag, `onclose`.
- `EmptyState`: `title`, `description`, `cta` (snippet).

If the actual M1 API differs, **the plan tasks below stay structurally identical but the JSX-equivalent in code must use the real primitive API**. Do not introduce new primitives; instead adapt the integration code.

- [ ] **Step 2: Confirm M1's isMobile wiring**

M1 owns the `isMobile` reactive flag inside `AppShell.svelte`. The Admin page does NOT need a local `matchMedia` listener — the new chrome's `.ts-shell` styling collapses gracefully at narrow widths via CSS media queries inside each component's `<style>` block. CSS-only mobile parity is sufficient for this view.

- [ ] **Step 3: Confirm M1's deletions and lib/status.ts ownership**

```bash
test ! -f web/src/components/SystemStatus.svelte && echo "deleted: SystemStatus.svelte"
test -f web/src/lib/status.ts && echo "kept: status.ts (this plan repurposes it)"
grep -rln "from.*lib/status\|from.*'../lib/status'" web/src/
```

Expected after M1: `SystemStatus.svelte` deleted, `lib/status.ts` still present, only `web/src/lib/__tests__/status.test.ts` left as a consumer. `PollerStatus.svelte` does NOT import from `lib/status.ts` (verified at plan-time on `main`); if M1 changed that, re-verify before continuing.

- [ ] **Step 4: Commit nothing**

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
}

func TestGetAdminMetrics_PopulatedDB(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, d, "alice")
	now := time.Unix(1_700_000_000, 0)

	// 3 feeds: 2 OK, 1 erroring
	feedOK1 := mustInsertSubscriptionFull(t, d, uid, "Feed OK 1", "https://a.example/feed", 0, now.Unix()-60,    now.Unix()+60)
	feedOK2 := mustInsertSubscriptionFull(t, d, uid, "Feed OK 2", "https://b.example/feed", 0, now.Unix()-120,   now.Unix()+120)
	mustInsertSubscriptionFull(t, d, uid, "Phoronix", "https://phoronix.com/feed", 4, now.Unix()-3600, now.Unix()+30)

	// 2 entries: 1 fetched within last 24h, 1 fetched 48h ago.
	// fetched_at is what GetAdminMetrics filters on.
	mustInsertEntryFetched(t, d, feedOK1, "hash1", "Recent", now.Unix()-1800,         now.Unix()-1800)
	mustInsertEntryFetched(t, d, feedOK2, "hash2", "Old",    now.Unix()-(48*3600),    now.Unix()-(48*3600))

	m, err := db.GetAdminMetrics(ctx, d, now)
	require.NoError(t, err)
	require.Equal(t, 3, m.FeedsTotal)
	require.Equal(t, 2, m.FeedsOK)
	require.Equal(t, 2, m.EntriesTotal)
	require.Equal(t, 1, m.Entries24h)
	require.Equal(t, 1, m.FeedsWithErrors)
	require.Equal(t, []string{"Phoronix"}, m.OffendingFeeds)
}

func TestGetAdminMetrics_OffendingFeedsLimitAndOrder(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, d, "alice")
	now := time.Unix(1_700_000_000, 0)

	// 5 erroring feeds, descending error_count
	mustInsertSubscriptionFull(t, d, uid, "e1", "https://e1/feed", 2,  0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e2", "https://e2/feed", 3,  0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e3", "https://e3/feed", 4,  0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e4", "https://e4/feed", 5,  0, now.Unix()+60)
	mustInsertSubscriptionFull(t, d, uid, "e5", "https://e5/feed", 10, 0, now.Unix()+60)

	m, err := db.GetAdminMetrics(ctx, d, now)
	require.NoError(t, err)
	require.Equal(t, 5, m.FeedsWithErrors)
	require.Equal(t, []string{"e5", "e4", "e3"}, m.OffendingFeeds) // top 3 by error_count desc
}
```

If `newTestDB`, `mustCreateUser`, `mustInsertSubscriptionFull`, `mustInsertEntryFetched` test helpers do not already exist in `internal/db/`, **reuse the closest existing helper in `subscriptions_test.go` / `entries_test.go`** and adapt locally. Do not introduce parallel helpers under different names. If the existing entry helper inserts with `fetched_at` defaulting to `published_at`, write a small wrapper at the top of `metrics_test.go` that calls the existing helper and then `UPDATE entries SET fetched_at=? WHERE id=?` — keep it tiny.

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
}

// GetAdminMetrics returns instance-wide stats for the admin metric grid.
// `now` is injected so tests can use a fixed clock; production callers pass time.Now().
//
// "Entries24h" counts rows where `entries.fetched_at` (the time the poller wrote
// the row, not the origin-feed timestamp) is within the last 24 hours. We do
// NOT use `published_at` — that's the feed-supplied date, which can be in the
// far past for backfilled or republished entries.
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
		`SELECT COUNT(*) FROM entries WHERE fetched_at >= ?`, cutoff,
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
	return m, nil
}
```

- [ ] **Step 2: Run test to verify it passes**

```bash
go test ./internal/db -run TestGetAdminMetrics -race
```

Expected: PASS three subtests.

- [ ] **Step 3: Commit**

```bash
git add internal/db/metrics.go
git commit -m "feat(db): GetAdminMetrics aggregates feeds/entries/errors"
```

---

### Task 3: Backend — extend `statusResponse` (RED)

**Files:**
- Test: `internal/api/status_test.go` (create) or append to whatever file already exercises the route.

First find where `/api/v1/status` is currently tested:

```bash
grep -rln "/api/v1/status\|statusHandler\|TestStatus" internal/api/
```

If a dedicated test file exists, append; otherwise create `internal/api/status_test.go`.

- [ ] **Step 1: Write the failing test**

```go
// internal/api/status_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatus_AdminMetricsFields(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	tok, _ := loginUser(t, srv, "admin1", "admin")

	// Seed 2 subscriptions: 1 OK, 1 erroring; one freshly-fetched entry.
	seedFeed(t, srv, "Feed A", "https://a.example/feed", 0)
	seedFeed(t, srv, "Phoronix", "https://phoronix.com/feed", 5)
	seedEntryFetchedNow(t, srv, "Feed A", "hash-now", "Fresh")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.AddCookie(authCookie(tok))
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Version         string   `json:"version"`
		FeedsTotal      int      `json:"feeds_total"`
		FeedsOK         int      `json:"feeds_ok"`
		FeedsWithErrors int      `json:"feeds_with_errors"`
		OffendingFeeds  []string `json:"offending_feeds"`
		EntriesTotal    int      `json:"entries_total"`
		Entries24h      int      `json:"entries_24h"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.NotEmpty(t, body.Version, "existing fields must still serialise")
	require.Equal(t, 2, body.FeedsTotal)
	require.Equal(t, 1, body.FeedsOK)
	require.Equal(t, 1, body.FeedsWithErrors)
	require.Equal(t, []string{"Phoronix"}, body.OffendingFeeds)
	require.Equal(t, 1, body.EntriesTotal)
	require.Equal(t, 1, body.Entries24h)
}

func TestStatus_OffendingFeeds_AlwaysArray(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	tok, _ := loginUser(t, srv, "admin1", "admin")

	// no feeds with errors → offending_feeds must serialise as [] not null
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.AddCookie(authCookie(tok))
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"offending_feeds":[]`)
}
```

If `seedFeed` / `seedEntryFetchedNow` test helpers do not exist, reuse the closest existing ones in `internal/api/main_test.go` (or wherever the test scaffolding lives). Look for `newTestServer`, `loginUser`, `authCookie` patterns first — those should exist from the M6/M7 admin endpoint tests in `internal/api/admin_test.go`.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/api -run TestStatus -race
```

Expected: FAIL — `FeedsTotal`/`FeedsOK`/etc. decode to zero because the fields don't exist yet.

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/api/status_test.go
git commit -m "test(api): /api/v1/status — new admin-metric fields"
```

---

### Task 4: Backend — extend `statusResponse` (GREEN)

**Files:**
- Modify: `internal/api/status.go`

- [ ] **Step 1: Extend the struct and handler**

Replace `internal/api/status.go` with:

```go
// internal/api/status.go
package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/ring"
)

type statusResponse struct {
	Version         string        `json:"version"`
	UptimeSeconds   int64         `json:"uptime_seconds"`
	DB              string        `json:"db"`
	PollsActive     int64         `json:"polls_active"`
	PollsTotal      int64         `json:"polls_total"`
	LastPollAt      *int64        `json:"last_poll_at"`
	RecentErrors    []statusEvent `json:"recent_errors"`
	FeedsTotal      int           `json:"feeds_total"`
	FeedsOK         int           `json:"feeds_ok"`
	FeedsWithErrors int           `json:"feeds_with_errors"`
	OffendingFeeds  []string      `json:"offending_feeds"`
	EntriesTotal    int           `json:"entries_total"`
	Entries24h      int           `json:"entries_24h"`
}

type statusEvent struct {
	Time  string         `json:"time"`
	Level string         `json:"level"`
	Event string         `json:"event"`
	Attrs map[string]any `json:"attrs"`
}

type statusDeps struct {
	db          *sql.DB
	buf         *ring.Buffer
	startTime   time.Time
	version     string
	pollsActive func() int64
	pollsTotal  func() int64
	lastPollAt  func() *int64
}

func statusHandler(deps statusDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok || u.Role != "admin" {
			writeError(w, http.StatusForbidden, ErrCodeForbidden, "forbidden")
			return
		}

		dbStatus := "ok"
		if deps.db != nil {
			if err := deps.db.PingContext(r.Context()); err != nil {
				dbStatus = "degraded"
			}
		}

		var active, total int64
		var lastAt *int64
		if deps.pollsActive != nil { active = deps.pollsActive() }
		if deps.pollsTotal != nil  { total  = deps.pollsTotal()  }
		if deps.lastPollAt != nil  { lastAt = deps.lastPollAt()  }

		var events []ring.Event
		if deps.buf != nil { events = deps.buf.Recent(20) }
		recent := make([]statusEvent, len(events))
		for i, e := range events {
			recent[i] = statusEvent{
				Time:  e.Time.UTC().Format(time.RFC3339),
				Level: e.Level,
				Event: e.Event,
				Attrs: e.Attrs,
			}
		}

		// Aggregates. If DB is degraded or the query fails, keep zero values
		// rather than failing the whole status call — the SPA renders "—" in
		// cells with zero values during a degraded state. This is graceful
		// degradation, not bug-hiding: the operator still sees uptime, version,
		// db_health, polls, and recent_errors.
		var metrics db.AdminMetrics
		if deps.db != nil && dbStatus == "ok" {
			if m, err := db.GetAdminMetrics(r.Context(), deps.db, time.Now()); err != nil {
				slog.Warn("admin metrics aggregate failed", "err", err)
			} else {
				metrics = m
			}
		}
		offending := metrics.OffendingFeeds
		if offending == nil { offending = []string{} } // always [] not null in JSON

		resp := statusResponse{
			Version:         deps.version,
			UptimeSeconds:   int64(time.Since(deps.startTime).Seconds()),
			DB:              dbStatus,
			PollsActive:     active,
			PollsTotal:      total,
			LastPollAt:      lastAt,
			RecentErrors:    recent,
			FeedsTotal:      metrics.FeedsTotal,
			FeedsOK:         metrics.FeedsOK,
			FeedsWithErrors: metrics.FeedsWithErrors,
			OffendingFeeds:  offending,
			EntriesTotal:    metrics.EntriesTotal,
			Entries24h:      metrics.Entries24h,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
go test ./internal/api -run TestStatus -race
go test ./internal/api -race
```

Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/status.go
git commit -m "feat(api): /api/v1/status returns admin metric aggregates"
```

---

### Task 5: Frontend — extend `lib/status.ts` and its test (RED)

**Files:**
- Modify: `web/src/lib/status.ts`
- Modify: `web/src/lib/__tests__/status.test.ts`

- [ ] **Step 1: Update the failing test**

Replace `web/src/lib/__tests__/status.test.ts` with:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { getStatus } from '../status';

describe('getStatus', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('returns parsed status on 200 including admin metric fields', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      version: 'v1.4.2',
      uptime_seconds: 86_400,
      db: 'ok',
      polls_active: 3,
      polls_total: 24,
      last_poll_at: 1_700_000_000,
      recent_errors: [
        { time: '2026-05-11T10:42:14Z', level: 'error', event: 'phoronix.com 502', attrs: {} },
      ],
      feeds_total: 24,
      feeds_ok: 22,
      feeds_with_errors: 2,
      offending_feeds: ['Phoronix', 'LWN'],
      entries_total: 14_820,
      entries_24h: 1_402,
    }), { status: 200 }));

    const s = await getStatus();
    expect(s.version).toBe('v1.4.2');
    expect(s.db).toBe('ok');
    expect(s.feeds_total).toBe(24);
    expect(s.feeds_ok).toBe(22);
    expect(s.feeds_with_errors).toBe(2);
    expect(s.offending_feeds).toEqual(['Phoronix', 'LWN']);
    expect(s.entries_total).toBe(14_820);
    expect(s.entries_24h).toBe(1_402);
    expect(s.recent_errors).toHaveLength(1);
  });

  it('throws on non-2xx', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(new Response('forbidden', { status: 403 }));
    await expect(getStatus()).rejects.toThrow(/403/);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/lib/__tests__/status.test.ts --run
```

Expected: FAIL — `StatusResponse` lacks the new fields.

- [ ] **Step 3: Extend `lib/status.ts`**

```ts
// web/src/lib/status.ts
export type StatusEvent = {
  time: string;
  level: 'warn' | 'error';
  event: string;
  attrs: Record<string, unknown>;
};

export type StatusResponse = {
  version: string;
  uptime_seconds: number;
  db: 'ok' | 'degraded';
  polls_active: number;
  polls_total: number;
  last_poll_at: number | null;
  recent_errors: StatusEvent[];
  feeds_total: number;
  feeds_ok: number;
  feeds_with_errors: number;
  offending_feeds: string[];
  entries_total: number;
  entries_24h: number;
};

export async function getStatus(): Promise<StatusResponse> {
  const resp = await fetch('/api/v1/status');
  if (!resp.ok) throw new Error(`status ${resp.status}`);
  return resp.json();
}
```

Keep field names in snake_case to match the JSON shape exactly — no mapping layer needed since SPA consumers (`SysGrid`, `ErrorsTable`) read these directly.

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/lib/__tests__/status.test.ts --run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/status.ts web/src/lib/__tests__/status.test.ts
git commit -m "feat(web): StatusResponse — admin metric fields"
```

---

### Task 6: Frontend — `SysGrid.svelte`

**Files:**
- Create: `web/src/components/SysGrid.svelte`
- Test: `web/src/components/SysGrid.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/SysGrid.test.ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import SysGrid from './SysGrid.svelte';
import type { StatusResponse } from '../lib/status';

const baseStatus: StatusResponse = {
  version: 'v1.0',
  uptime_seconds: 0,
  db: 'ok',
  polls_active: 0,
  polls_total: 0,
  last_poll_at: 1_700_000_000,
  recent_errors: [],
  feeds_total: 24,
  feeds_ok: 22,
  feeds_with_errors: 2,
  offending_feeds: ['Phoronix', 'LWN'],
  entries_total: 14_820,
  entries_24h: 1_402,
};

describe('SysGrid', () => {
  it('renders four labelled cells', () => {
    render(SysGrid, { props: { status: baseStatus, nextPollAt: 1_700_000_060, now: 1_700_000_840 } });
    expect(screen.getByText('FEEDS')).toBeInTheDocument();
    expect(screen.getByText('ENTRIES')).toBeInTheDocument();
    expect(screen.getByText('ERRORS')).toBeInTheDocument();
    expect(screen.getByText('POLL')).toBeInTheDocument();
  });

  it('FEEDS cell shows total + "n ok" sub line', () => {
    render(SysGrid, { props: { status: baseStatus, nextPollAt: null, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-feeds-v')).toHaveTextContent('24');
    expect(screen.getByTestId('sys-feeds-sub')).toHaveTextContent('22 ok');
  });

  it('ENTRIES cell shows total + 24h sub line', () => {
    render(SysGrid, { props: { status: baseStatus, nextPollAt: null, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-entries-v')).toHaveTextContent('14,820');
    expect(screen.getByTestId('sys-entries-sub')).toHaveTextContent('1,402 24h');
  });

  it('ERRORS cell shows count + first offending feed', () => {
    render(SysGrid, { props: { status: baseStatus, nextPollAt: null, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-errors-v')).toHaveTextContent('2');
    expect(screen.getByTestId('sys-errors-sub')).toHaveTextContent('Phoronix');
  });

  it('ERRORS cell renders "all clear" sub when no offending feeds', () => {
    render(SysGrid, { props: { status: { ...baseStatus, feeds_with_errors: 0, offending_feeds: [] }, nextPollAt: null, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-errors-v')).toHaveTextContent('0');
    expect(screen.getByTestId('sys-errors-sub')).toHaveTextContent('all clear');
  });

  it('POLL cell shows relative last + next', () => {
    const now = 1_700_000_000 + 14 * 60;
    render(SysGrid, { props: { status: { ...baseStatus, last_poll_at: 1_700_000_000 }, nextPollAt: now + 60, now } });
    expect(screen.getByTestId('sys-poll-v')).toHaveTextContent('14m ago');
    expect(screen.getByTestId('sys-poll-sub')).toHaveTextContent('next 1m');
  });

  it('POLL cell handles missing timestamps', () => {
    render(SysGrid, { props: { status: { ...baseStatus, last_poll_at: null }, nextPollAt: null, now: 1_700_000_000 } });
    expect(screen.getByTestId('sys-poll-v')).toHaveTextContent('—');
    expect(screen.getByTestId('sys-poll-sub')).toHaveTextContent('—');
  });

  it('null status renders skeleton dashes in all cells', () => {
    render(SysGrid, { props: { status: null, nextPollAt: null, now: 0 } });
    expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(4);
  });

  it('error prop renders error message in place of grid', () => {
    render(SysGrid, { props: { status: null, error: 'forbidden', nextPollAt: null, now: 0 } });
    expect(screen.getByRole('alert')).toHaveTextContent('forbidden');
  });
});
```

`nextPollAt` is a separate prop because the existing `/api/v1/status` does **not** return a "next poll" timestamp. The parent (`Admin.svelte`) passes `null` for now; if a future milestone adds the field server-side, the prop accepts a number. Rendering `next —` when null is acceptable per the brand spec.

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm --dir web test -- src/components/SysGrid.test.ts --run
```

Expected: FAIL "Cannot find module './SysGrid.svelte'".

- [ ] **Step 3: Implement `SysGrid.svelte`**

```svelte
<!-- web/src/components/SysGrid.svelte -->
<script lang="ts">
  import type { StatusResponse } from '../lib/status';

  type Props = {
    status: StatusResponse | null;
    nextPollAt: number | null;
    error?: string;
    now: number; // unix seconds
  };

  const { status, nextPollAt, error = '', now }: Props = $props();

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
        {#if !status}—{:else}{fmtCount(status.feeds_total)}{/if}
      </span>
      <span class="sub" data-testid="sys-feeds-sub">
        {#if !status}—{:else}{fmtCount(status.feeds_ok)} ok{/if}
      </span>
    </div>
    <div class="cell">
      <span class="l">ENTRIES</span>
      <span class="v" data-testid="sys-entries-v">
        {#if !status}—{:else}{fmtCount(status.entries_total)}{/if}
      </span>
      <span class="sub" data-testid="sys-entries-sub">
        {#if !status}—{:else}{fmtCount(status.entries_24h)} 24h{/if}
      </span>
    </div>
    <div class="cell">
      <span class="l">ERRORS</span>
      <span class="v" class:warn={status && status.feeds_with_errors > 0} data-testid="sys-errors-v">
        {#if !status}—{:else}{status.feeds_with_errors}{/if}
      </span>
      <span class="sub" data-testid="sys-errors-sub">
        {#if !status}—
        {:else if status.offending_feeds.length === 0}all clear
        {:else}{status.offending_feeds[0]}{#if status.offending_feeds.length > 1} +{status.offending_feeds.length - 1}{/if}
        {/if}
      </span>
    </div>
    <div class="cell">
      <span class="l">POLL</span>
      <span class="v" data-testid="sys-poll-v">
        {#if !status}—{:else}{relativePast(status.last_poll_at, now)}{/if}
      </span>
      <span class="sub" data-testid="sys-poll-sub">
        {#if !status}—{:else}{relativeFuture(nextPollAt, now)}{/if}
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

The ring buffer in `internal/ring/handler.go:22` filters at `slog.LevelWarn`, so the buffer emits only `"warn"` and `"error"` levels. The errors table renders only those two. No `info` row branch.

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/ErrorsTable.test.ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ErrorsTable from './ErrorsTable.svelte';
import type { StatusEvent } from '../lib/status';

const events: StatusEvent[] = [
  { time: '2026-05-11T10:42:14Z', level: 'error', event: 'phoronix.com — 502', attrs: {} },
  { time: '2026-05-11T10:38:41Z', level: 'warn',  event: 'poll worker 3 slow', attrs: {} },
];

describe('ErrorsTable', () => {
  it('renders one row per event with level tag and message', () => {
    render(ErrorsTable, { props: { events } });
    expect(screen.getByText('phoronix.com — 502')).toBeInTheDocument();
    expect(screen.getByText('poll worker 3 slow')).toBeInTheDocument();
    expect(screen.getByText('error')).toBeInTheDocument();
    expect(screen.getByText('warn')).toBeInTheDocument();
  });

  it('formats timestamp as HH:MM:SS', () => {
    render(ErrorsTable, { props: { events: [events[0]] } });
    expect(screen.getByTestId('err-time')).toHaveTextContent(/\d{2}:\d{2}:\d{2}/);
  });

  it('renders empty state when no events', () => {
    render(ErrorsTable, { props: { events: [] } });
    expect(screen.getByText(/no recent events/i)).toBeInTheDocument();
  });

  it('applies level-error class to error-level rows', () => {
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
  import type { StatusEvent } from '../lib/status';

  type Props = { events: StatusEvent[] };
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
git commit -m "feat(web): ErrorsTable — recent events with level tags (warn/error)"
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

const me: AdminUser = { id: 1, username: 'liz',   role: 'admin', created_at: 1_700_000_000, disabled_at: null,         has_totp: true,  passkey_count: 3 };
const alice: AdminUser = { id: 2, username: 'alice', role: 'user', created_at: 1_700_000_000, disabled_at: null,         has_totp: false, passkey_count: 0 };
const bob: AdminUser = { id: 3, username: 'bob',   role: 'user', created_at: 1_700_000_000, disabled_at: 1_700_500_000, has_totp: true,  passkey_count: 1 };

describe('UserTable', () => {
  it('renders one row per user with username + role + status', () => {
    render(UserTable, { props: { users: [me, alice, bob], currentUserId: 1 } });
    expect(screen.getByText('liz')).toBeInTheDocument();
    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('bob')).toBeInTheDocument();
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
    render(AdminToolbar, { props: { filter: 'admins', query: '', count: 7 } });
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
  .bar { display: flex; align-items: center; justify-content: space-between; gap: 14px; margin-bottom: 14px; }
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
  .chip.active { color: var(--accent); border-color: var(--accent); background: var(--accent-soft); }
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

If the M1 `Dialog` primitive does not expose a `footer` snippet but instead a different slot/prop name, adapt the snippet reference accordingly. Same for `Button`'s variant prop name.

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

Copy button only — no download button. Temporary passwords should not be persisted to disk; the copy-paste flow leaves no artefact.

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

  it('does not render a download button', () => {
    render(ResetPasswordResultDialog, { props: { open: true, username: 'alice', password: 'pw-123' } });
    expect(screen.queryByRole('button', { name: /download/i })).toBeNull();
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
    try {
      await navigator.clipboard.writeText(password);
      copied = true;
      setTimeout(() => copied = false, 1500);
    } catch (_) { /* ignore */ }
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
  .key { font-family: var(--mono); font-size: 15px; letter-spacing: 0.06em; color: var(--ink); font-weight: 500; flex: 1; }
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
git commit -m "feat(web): ResetPasswordResultDialog — one-shot temp password (copy only)"
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

> Note: the `data-variant` attribute on the Button is only for the test to assert the variant string survives. If M1's `Button.svelte` already exposes `data-variant` from its `variant` prop, drop the explicit attribute. If it does not, leave it in.

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

### Task 13: Frontend — `Admin.svelte` view (access denied path) (RED)

**Files:**
- Modify: `web/src/views/Admin.svelte`
- Create: `web/src/views/Admin.test.ts`

- [ ] **Step 1: Write the failing test (access-denied path first)**

```ts
// web/src/views/Admin.test.ts
import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { writable } from 'svelte/store';
import Admin from './Admin.svelte';

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

vi.mock('../lib/status', () => ({
  getStatus: vi.fn().mockResolvedValue({
    version: 'v1', uptime_seconds: 0, db: 'ok',
    polls_active: 0, polls_total: 0, last_poll_at: null,
    recent_errors: [],
    feeds_total: 0, feeds_ok: 0, feeds_with_errors: 0, offending_feeds: [],
    entries_total: 0, entries_24h: 0,
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

Expected: FAIL — the current `Admin.svelte` renders `<p>Access denied.</p>`, not the new EmptyState chrome.

- [ ] **Step 3: Rewrite `Admin.svelte` minimally — access-denied path only**

```svelte
<!-- web/src/views/Admin.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { auth } from '../lib/auth';
  import { navigate } from '../lib/router';
  import EmptyState from '../components/EmptyState.svelte';

  const authState = $derived(get(auth));
  const isAdmin = $derived(authState?.user?.role === 'admin');

  onMount(() => {
    if (!isAdmin) navigate('/');
  });
</script>

{#if !isAdmin}
  <main class="ts-shell">
    <EmptyState title="Access denied" description="You don't have permission to view this page." />
  </main>
{:else}
  <!-- TODO next task: full chrome -->
  <main class="ts-shell"><p>admin</p></main>
{/if}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Admin.svelte web/src/views/Admin.test.ts
git commit -m "feat(web): Admin — access-denied path on .ts-shell"
```

---

### Task 14: Frontend — `Admin.svelte` view (composition) (RED)

**Files:**
- Modify: `web/src/views/Admin.svelte`
- Modify: `web/src/views/Admin.test.ts`

- [ ] **Step 1: Add composition tests**

Append to `Admin.test.ts`:

```ts
import { fireEvent } from '@testing-library/svelte';
import { api } from '../lib/api';
import { getStatus } from '../lib/status';

describe('Admin (admin role)', () => {
  beforeEach(() => {
    authStore.set({ user: { id: 1, username: 'liz', role: 'admin' } });
    (api.listUsers as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([
      { id: 1, username: 'liz',   role: 'admin', created_at: 1_700_000_000, disabled_at: null, has_totp: true,  passkey_count: 3 },
      { id: 2, username: 'alice', role: 'user',  created_at: 1_700_000_000, disabled_at: null, has_totp: false, passkey_count: 0 },
    ]);
    (getStatus as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue({
      version: 'v1', uptime_seconds: 0, db: 'ok',
      polls_active: 0, polls_total: 0, last_poll_at: 1_700_000_000,
      recent_errors: [
        { time: '2026-05-11T10:42:14Z', level: 'error', event: 'phoronix.com 502', attrs: {} },
      ],
      feeds_total: 12, feeds_ok: 11, feeds_with_errors: 1, offending_feeds: ['Phoronix'],
      entries_total: 1500, entries_24h: 80,
    });
  });

  it('renders SysGrid + ErrorsTable + UserTable after load', async () => {
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('FEEDS')).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText('liz')).toBeInTheDocument());
    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('Phoronix')).toBeInTheDocument(); // ERRORS sub
    expect(screen.getByText('phoronix.com 502')).toBeInTheDocument();
  });

  it('filters users by search query', async () => {
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('liz')).toBeInTheDocument());
    await fireEvent.input(screen.getByPlaceholderText('Filter by username'), { target: { value: 'ali' } });
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

- [ ] **Step 2: Run tests to verify they fail**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
```

Expected: FAIL on the three new tests.

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
  import { getStatus, type StatusResponse } from '../lib/status';
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
  let status = $state<StatusResponse | null>(null);
  let loadError = $state('');
  let busy = $state(false);
  let pollInterval = $state<ReturnType<typeof setInterval> | null>(null);
  let nowSec = $state(Math.floor(Date.now() / 1000));

  let filter = $state<AdminFilter>('all');
  let query = $state('');

  let overlay = $state<'create' | 'resetConfirm' | 'resetResult' | 'disable2fa' | 'disableUser' | 'enable' | 'delete' | null>(null);
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
    await Promise.all([loadUsers(), loadStatus()]);
    pollInterval = setInterval(() => {
      nowSec = Math.floor(Date.now() / 1000);
      loadStatus();
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

  async function loadStatus() {
    try {
      status = await getStatus();
    } catch (e) {
      if (!status) loadError = e instanceof Error ? e.message : 'status load failed';
    }
  }

  function onAction(action: 'reset' | 'disable2fa' | 'disableUser' | 'enable' | 'delete', user: AdminUser) {
    overlayUser = user;
    if (action === 'reset')            overlay = 'resetConfirm';
    else if (action === 'disable2fa')  overlay = 'disable2fa';
    else if (action === 'disableUser') overlay = 'disableUser';
    else if (action === 'enable')      overlay = 'enable';
    else if (action === 'delete')      overlay = 'delete';
  }

  function closeOverlay() {
    overlay = null;
    overlayUser = null;
    tempPassword = '';
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

  async function handleResetConfirm() {
    if (!overlayUser) return;
    busy = true;
    try {
      const r = await api.resetUserPassword(overlayUser.id);
      tempPassword = r.temporary_password;
      overlay = 'resetResult';
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
      <SysGrid status={status} nextPollAt={null} now={nowSec} />
      <div class="errors-head">
        <span>Recent events</span>
        <span class="rule" aria-hidden="true"></span>
      </div>
      <ErrorsTable events={status?.recent_errors ?? []} />
    </section>
  </main>
{/if}

{#if overlay === 'create'}
  <CreateUserDialog open={true} onSubmit={handleCreate} onClose={closeOverlay} />
{:else if overlay === 'resetConfirm' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Reset password for ${overlayUser.username}?`}
    body={`A new one-shot password will be generated and shown once. ${overlayUser.username}'s other sessions will be signed out.`}
    cta="Reset password"
    onConfirm={handleResetConfirm}
    onCancel={closeOverlay}
  />
{:else if overlay === 'resetResult' && overlayUser}
  <ResetPasswordResultDialog
    open={true}
    username={overlayUser.username}
    password={tempPassword}
    onClose={closeOverlay}
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

- [ ] **Step 4: Run tests to verify they pass**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Admin.svelte web/src/views/Admin.test.ts
git commit -m "feat(web): Admin — compose SysGrid/ErrorsTable/UserTable + dialogs"
```

---

### Task 15: Frontend — mutation flow tests

**Files:**
- Modify: `web/src/views/Admin.test.ts`

- [ ] **Step 1: Add mutation-flow tests**

Append to `Admin.test.ts`:

```ts
describe('Admin (mutations)', () => {
  beforeEach(() => {
    authStore.set({ user: { id: 1, username: 'liz', role: 'admin' } });
    (api.listUsers as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([
      { id: 1, username: 'liz',   role: 'admin', created_at: 1_700_000_000, disabled_at: null, has_totp: true,  passkey_count: 0 },
      { id: 2, username: 'alice', role: 'user',  created_at: 1_700_000_000, disabled_at: null, has_totp: false, passkey_count: 0 },
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
    (api.listUsers as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, username: 'liz',   role: 'admin', created_at: 1_700_000_000, disabled_at: null, has_totp: true, passkey_count: 0 },
      { id: 2, username: 'alice', role: 'user',  created_at: 1_700_000_000, disabled_at: null, has_totp: true, passkey_count: 0 },
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

Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/Admin.test.ts
git commit -m "test(web): Admin mutation flows — create/reset/delete/disable2fa/disable"
```

---

### Task 16: Frontend — status-polling lifecycle test

**Files:**
- Modify: `web/src/views/Admin.test.ts`

- [ ] **Step 1: Add polling test**

Append to `Admin.test.ts` (add `afterEach` to the imports — `import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';`):

```ts
describe('Admin (status polling)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    authStore.set({ user: { id: 1, username: 'liz', role: 'admin' } });
    (api.listUsers as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([]);
    (getStatus as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue({
      version: 'v1', uptime_seconds: 0, db: 'ok',
      polls_active: 0, polls_total: 0, last_poll_at: null,
      recent_errors: [],
      feeds_total: 1, feeds_ok: 1, feeds_with_errors: 0, offending_feeds: [],
      entries_total: 0, entries_24h: 0,
    });
  });
  afterEach(() => vi.useRealTimers());

  it('polls status every 60s and stops on unmount', async () => {
    const { unmount } = render(Admin, {});
    await vi.runOnlyPendingTimersAsync();
    expect(getStatus).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(60_000);
    await vi.runOnlyPendingTimersAsync();
    expect(getStatus).toHaveBeenCalledTimes(2);
    unmount();
    vi.advanceTimersByTime(60_000);
    await vi.runOnlyPendingTimersAsync();
    expect(getStatus).toHaveBeenCalledTimes(2); // no further calls after unmount
  });
});
```

- [ ] **Step 2: Run test to verify it passes**

```bash
pnpm --dir web test -- src/views/Admin.test.ts --run
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/Admin.test.ts
git commit -m "test(web): Admin — status polling cycles and stops on unmount"
```

---

### Task 17: Manual smoke checklist + svelte-check

- [ ] **Step 1: Build the SPA and run the full suite**

```bash
make test
```

Expected: PASS. The Go test step depends on `web/dist` per CLAUDE.md; `make test` rebuilds it.

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
- Verify the errors table renders recent events from the ring buffer (warn/error only) or "No recent events".
- Click "Create user" — dialog opens, type `smokey`, click "Create user". Expect the new row to appear.
- Click "Reset password" on `smokey` — first dialog asks for confirmation, then the result dialog shows a temp password with a copy button (NOT a download button). Click copy, paste somewhere to verify clipboard contents.
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

- [ ] **Step 5: Verify endpoint shape directly**

```bash
curl -s -b cookie http://localhost:8080/api/v1/status | jq '{ feeds_total, feeds_ok, feeds_with_errors, offending_feeds, entries_total, entries_24h }'
```

Expected: all six fields present and reasonable for the seed data.

- [ ] **Step 6: Commit any small fixes if smoke uncovered them**

If a token or class adjustment is needed, commit that as a follow-up (one small commit per concern, no scope creep).

---

## Skills and tools for implementers

- **`superpowers:test-driven-development`** — every behavioural change starts with a red test (Tasks 1, 3, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16).
- **`superpowers:verification-before-completion`** — run `make test` + `pnpm --dir web run check` + manual smoke in Task 17 before claiming done.
- **`svelte-runes`** — `$state`, `$derived`, `$derived.by`, `$props` in every new Svelte component. The state machinery in `Admin.svelte` (filtered users, overlay routing, polling) is the densest user of runes in this milestone.
- **`svelte-styling`** — scoped `<style>` blocks in every component, using `:global(html.theme-dark)` for theme-conditional rules. No global CSS additions; the brand spec's selectors get renamed inside each component.
- **`svelte-template-directives`** — `{@const}` for per-row derived values in `UserTable.svelte`, `{#snippet}` for the Dialog `footer` snippets in dialogs.
- **`golang-database`** — for `internal/db/metrics.go`: prepared queries, `defer rows.Close()`, `rows.Err()` after iteration. No new schema changes — relies on existing `subscriptions` and `entries` columns.
- **`golang-testing`** — table-driven tests in `internal/db/metrics_test.go` and `internal/api/status_test.go`, helpers reused from `internal/db/*_test.go` and `internal/api/main_test.go`, `require.NoError`, fixed clock (pass `time.Time` argument rather than calling `time.Now()` inside the function).
- **`golang-error-handling`** — `fmt.Errorf("count feeds: %w", err)` for wrapping, return early on each query error. In `statusHandler`, swallow-and-log on aggregate error to keep the rest of the status response intact (graceful degradation).
- **`tdd`** (general) — red-green-refactor; every commit is one small step.
- **`golang-code-style`** — `gofmt`, lower-camel local variables, exported names like `AdminMetrics` per Go naming convention.
- **MCP `context7:query-docs`** — when in doubt about Svelte 5 testing-library patterns (e.g., `fireEvent.click` on a `<button>` that lives behind a Dialog primitive overlay), query for current `@testing-library/svelte` examples before adapting.

---

## Acceptance criteria

A milestone PR is mergeable when **all** of these are true:

1. **`/api/v1/status` exposes the six new fields.** `curl -b cookie http://localhost:8080/api/v1/status | jq` returns a JSON document whose top-level keys include `feeds_total`, `feeds_ok`, `feeds_with_errors`, `offending_feeds`, `entries_total`, `entries_24h` in addition to the existing fields. `offending_feeds` is `[]` not `null` when empty. Non-admin gets 403 (unchanged).
2. **SysGrid visible.** Visiting `/admin` as an admin shows a `.ts-shell` page with two sections. Section 2 contains a four-cell grid: each cell's first line matches the brand spec §6.7 label (FEEDS / ENTRIES / ERRORS / POLL), each cell shows a primary number/string and a secondary sub line.
3. **ErrorsTable visible.** Below the grid: a "Recent events" eyebrow, then a table with one row per ring-buffer event. Each row shows timestamp (HH:MM:SS, mono ink-3), a level tag (`warn` or `error` — only those two; `info` is not present in the ring buffer), and the event message (mono ink, ellipsis on overflow).
4. **UserTable functional.** Section 1 shows the AdminToolbar (search input + four filter chips + Create user button) and a grid of `.ts-urow` rows. Each row shows username + role + created date + status pill + 2FA flag + passkey count + four/five action buttons.
5. **Five overlay flows functional.**
   - Create user: opens a Dialog, accepts username + generated password + role, calls `POST /api/v1/admin/users` on confirm.
   - Reset password: opens a ConfirmDialog first; on confirm, calls `POST /api/v1/admin/users/:id/password-reset`; opens a ResetPasswordResultDialog showing the temporary password once with a copy button (no download button).
   - Disable 2FA: opens a ConfirmDialog (danger variant); on confirm, calls `POST /api/v1/admin/users/:id/disable-totp`.
   - Disable / Re-enable: opens a ConfirmDialog; on confirm, calls `PATCH /api/v1/admin/users/:id` with `{disabled: true/false}`.
   - Delete: opens a ConfirmDialog (danger variant) with consequence list; on confirm, calls `DELETE /api/v1/admin/users/:id` and removes the row.
6. **Self-action guards.** Current user cannot delete or disable themselves (buttons disabled). User without TOTP cannot Disable 2FA (button disabled).
7. **Access-denied path.** Non-admin visiting `/admin` sees an EmptyState "Access denied" and is redirected to `/`.
8. **Polling.** SysGrid + ErrorsTable refresh every 60s while the page is mounted. Clearing on unmount is verified by a vitest test.
9. **Three themes.** Light, dark, sepia all render correctly — verified visually by toggling the AccountMenu theme. The warn ink colour (`#c97a1a` / `#f0a655` dark) applies to the ERRORS cell when `feeds_with_errors > 0`. The error level row has the `level-error` background tint.
10. **Mobile parity.** Admin page renders on the mobile shell. The 4-cell grid collapses to 2×2 at narrow viewports. Dialogs scale.
11. **Tests pass.** `make test` and `pnpm --dir web run check` both clean. `pnpm --dir web test` includes new test files: `SysGrid.test.ts`, `ErrorsTable.test.ts`, `UserTable.test.ts`, `AdminToolbar.test.ts`, `CreateUserDialog.test.ts`, `ResetPasswordResultDialog.test.ts`, `ConfirmDialog.test.ts`, `Admin.test.ts`, plus the updated `lib/__tests__/status.test.ts`. New / updated Go tests: `internal/db/metrics_test.go`, `internal/api/status_test.go`.
12. **Spec match.** Brand spec §6.7 selectors map clearly to component classes — even though renamed (`.ts-sys-grid` → `.grid`), the rendered output (4 cells, mono labels, mono values, mono subs, rule border) is faithful. Umbrella spec §2.4 directive ("extend the existing `statusResponse`") is followed exactly: no new route is registered.

---

## Verification commands

Run before claiming the task complete:

```bash
# Full build + Go race tests (also builds web/dist)
make test

# Frontend type-check
pnpm --dir web run check

# Just the new / updated Vitest suites
pnpm --dir web test -- src/lib/__tests__/status.test.ts --run
pnpm --dir web test -- src/components/SysGrid.test.ts --run
pnpm --dir web test -- src/components/ErrorsTable.test.ts --run
pnpm --dir web test -- src/components/UserTable.test.ts --run
pnpm --dir web test -- src/components/AdminToolbar.test.ts --run
pnpm --dir web test -- src/components/admin/CreateUserDialog.test.ts --run
pnpm --dir web test -- src/components/admin/ResetPasswordResultDialog.test.ts --run
pnpm --dir web test -- src/components/admin/ConfirmDialog.test.ts --run
pnpm --dir web test -- src/views/Admin.test.ts --run

# Just the new / updated Go tests
go test ./internal/db -run TestGetAdminMetrics -race -v
go test ./internal/api -run TestStatus -race -v

# Smoke
make dev
# then navigate manually per Task 17 Step 3 and Step 4
```

---

## Risks

1. **Extending `statusResponse` widens the data returned to every admin status poll.** Six small integer fields + an optional ≤3-string slice. Trivial wire-cost. The aggregate queries are six `SELECT COUNT(*)` calls on small instance-scale tables — for a single-user deployment of a few hundred feeds, this is sub-millisecond. If a future deployment hits 100k+ entries and the cost becomes measurable, add a 5s in-process cache inside `db.GetAdminMetrics` keyed on `time.Now().Unix()/5` — but **do not pre-optimise**. Aggregation graceful-degrades: on DB error, the aggregates return zero, the SPA renders `—`, and the rest of the status payload (poll info, recent_errors) is unaffected.

2. **`SystemStatus.svelte` deletion in M1 leaves `lib/status.ts` orphaned.** This plan repurposes the file (extends `StatusResponse` and keeps `getStatus`). Verified at plan-time: only `SystemStatus.svelte` and `web/src/lib/__tests__/status.test.ts` import from `lib/status.ts` on `main`; `PollerStatus.svelte` does NOT (it has its own polling via `lib/store.ts`). If M1 deletes `lib/status.ts` too, recreate it inside this milestone (it's ~25 lines).

3. **Existing `Admin.svelte` has no test file.** This plan adds `Admin.test.ts` from scratch — there's no prior assertion of behaviour to regress against. **Mitigation:** the manual smoke checklist (Task 17 Step 3) verifies the legacy behaviour persists: every action button still calls the same API endpoint and produces the same DB effect. Run the smoke before merging.

4. **Brand spec divergence in the JSX mockup.** `ui_design/tap-admin.jsx`'s `SystemStatus` cells (Version / Uptime / Database / Active polls) do not match the brand spec §6.7's authoritative list (FEEDS / ENTRIES / ERRORS / POLL). This plan follows the brand spec as the team-lead's umbrella explicitly requires. Version/uptime/db are already in `/api/v1/status` and the implementer could surface them in the page's id strip if needed — but the metric grid follows §6.7.

5. **Polling cadence pinned at 60s.** `web/src/components/SystemStatus.svelte:20` uses `setInterval(refresh, 60_000)`; the brand spec §6.7 says "auto-refreshing every 5s" in the JSX but does not mandate 5s in the spec text. 60s avoids hot-loading the aggregate queries; 5s would require the in-process cache mentioned in Risk 1. The polling test pins `vi.advanceTimersByTime(60_000)` — if cadence changes, update both the literal and the test.

6. **`onMount` timing in the access-denied path.** The `navigate('/')` call fires inside `onMount`, so a tick of the EmptyState renders before the route changes. Vitest verifies both the navigate call and the EmptyState — both happen, which is correct. The access-denied behaviour is also guarded by the top-level `{#if !isAdmin}` derivation, so the admin chrome never renders for non-admins even if `navigate` is slow.

7. **Filter chip behaviour is client-side only.** The current `/api/v1/admin/users` does not accept role/disabled query params. If user count grows large enough that loading the whole list is expensive, that's a future milestone. The four-chip set (All / Admins / Users / Disabled) matches the `ui_design/tap-admin.jsx:463-467` design canvas.

8. **No `next_poll_at` field in `/api/v1/status`.** The brand spec §6.7 POLL cell shows "next 1m" but the existing status endpoint exposes only `last_poll_at`, not `next_poll_at`. This plan accepts the gap: the SysGrid takes `nextPollAt` as a separate prop (defaulting to `null`, rendering `next —`). If a future milestone wants the next-poll readout, add `next_poll_at *int64` to `statusResponse` (a single `SELECT MIN(next_poll_at) FROM subscriptions WHERE next_poll_at > 0`); plan-level overhead is < 30 lines. Out of scope for M7.

---

## Self-review

1. **Spec coverage check.** Cells match brand spec §6.7 (FEEDS / ENTRIES / ERRORS / POLL). Errors table matches (timestamp · level chip · message). User management surface preserves all existing functionality (list, create, reset password, disable 2FA, disable/re-enable, delete). Adopts M1 primitives (Button, Dialog, EmptyState). Mobile + three themes covered. Access denied path covered. Backend addition follows umbrella spec §2.4 directive (extend `statusResponse`, no new route).

2. **Placeholder scan.** No "TBD" / "implement later" / "add appropriate error handling". Every test ships full code. Every implementation step shows full code. The few notes that adapt to M1's actual API surface (`Button` variants, `Dialog` footer-snippet name) are explicit conditionals, not placeholders.

3. **Type consistency.** `StatusResponse` field names are snake_case in both the TS type and the JSON (matches the existing `StatusResponse` shape — no mapping layer). The `Action` enum in `UserTable.svelte` (`reset | disable2fa | disableUser | enable | delete`) is the same set as the `onAction` callback signature in `Admin.svelte`, and routes to the corresponding `overlay` state values (`resetConfirm`/`resetResult` for the two-stage reset flow). `entries.fetched_at` is the column name used in both the DB layer and tests (not `created_at`, which doesn't exist; not `published_at`, which is the wrong semantics).

Plan is complete.
