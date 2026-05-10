# M5 — Article Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land per-subscription opt-in article extraction as specified in `docs/specs/2026-05-09-m5-article-extraction.md` — Readability or per-feed CSS selector mode, bounded-parallel within the worker, sanitised + image-proxied through the existing `processor.Processor`, fail-soft to feed summary on per-entry error.

**Architecture:** New leaf package `internal/extract` wraps `codeberg.org/readeck/go-readability/v2` (Readability mode) and `github.com/andybalholm/cascadia` (selector mode). One new phase in `Worker.Run` between item-iteration and the existing per-item `processor.Process` pass, gated by `sub.Extract`, parallelised by `errgroup.SetLimit(N)`. Three new columns via migration 0004 (`subscriptions.extract`, `subscriptions.extract_selector`, `entries.extract_failed`). Narrow `PATCH /api/v1/subscriptions/:id` validates selectors at write time via `cascadia.Compile`. Article fetches reuse the M4 shared `*http.Client` so SSRF/per-host-cap/`--http-timeout` apply for free.

**Tech Stack:** Go 1.25, modernc.org/sqlite, stretchr/testify, golang.org/x/sync/errgroup, codeberg.org/readeck/go-readability/v2, github.com/andybalholm/cascadia (transitive via readeck).

---

## Skills and tools to apply

Always-on for every code-touching task:

- **`superpowers:test-driven-development`** — red/green/refactor on every behaviour-bearing change. Mandated by `docs/roadmap.md` §"Working cadence" and reaffirmed in the spec. Pure scaffolding (the migration SQL file, README edits, flag declarations) is exempt; everything with branches, error handling, or state is in scope.
- **`superpowers:verification-before-completion`** — before marking a task done or commit message claims success, actually run the test command listed in the task's verification step and confirm the output matches the expected output.

Reach for as needed:

- **`golang-testing`** + **`golang-stretchr-testify`** — test surface design, table-driven cases, `require` vs `assert` usage. The repo already uses testify; match the existing style (`require.NoError(t, err)`, `require.Equal(t, want, got)`).
- **`golang-database`** — for the migration, parameterised queries, struct scanning into `db.DueSubscription`, and `errors.Is(err, sql.ErrNoRows)` mapping.
- **`golang-concurrency`** — for the `errgroup.SetLimit` pattern in the worker. Each goroutine writes to a distinct slice index — no shared writes — so no mutex needed.
- **`golang-context`** — for ctx propagation through `extract.Extract` and the goroutines.
- **`golang-error-handling`** — per-entry extract failures are swallowed (returned as `nil` from the goroutine) and recorded as a per-entry flag, never propagated to fail the poll. WARN-log once at the swallow site.
- **`golang-cli`** — for the new flags (`--extract-concurrency`, `--extract-body-cap`) and the `envOrInt` / `envOrInt64` helpers already in `cmd/tap/main.go`.
- **`golang-naming`** — `Extract` (the function), `ExtractFunc` (the type), `ExtractFailed` (the field). Match the repo's PascalCase exported identifiers.
- **`golang-modernize`** — `for i := range pendings` (loop-var-per-iteration in Go 1.22+) instead of `i := i` shadowing. The repo is on Go 1.25, so modern idioms are safe.

MCP tools:

- **`context7` (`mcp__plugin_context7_context7__query-docs`)** — fetch live docs for `codeberg.org/readeck/go-readability/v2` and `github.com/andybalholm/cascadia` if the API drifts from what's transcribed below. Note: at plan-writing time, context7's index did not have these libraries; fall back to `WebFetch` against `pkg.go.dev/codeberg.org/readeck/go-readability/v2` and `pkg.go.dev/github.com/andybalholm/cascadia` if context7 misses again.

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `internal/db/migrations/0004_extraction.sql` | **create** | One file, three `ALTER TABLE` statements. |
| `internal/db/migrate_test.go` | modify | Add a `TestMigrate_AddsExtractionColumns` test analogous to `TestMigrate_AddsVelocityColumn`. |
| `internal/db/subscriptions.go` | modify | Extend `DueSubscription` with `Extract`/`ExtractSelector`; update `ListDuePolls` SELECT; add `UpdateSubscriptionExtraction` helper. |
| `internal/db/subscriptions_test.go` | modify | Cover the helper + the new fields round-tripping through `ListDuePolls`. |
| `internal/db/entries.go` | modify | Extend `NewEntry` with `ExtractFailed`; update `UpdateAfterPoll`'s INSERT to write the new column. |
| `internal/db/entries_test.go` | modify | Cover `extract_failed` writes. |
| `internal/extract/extract.go` | **create** | `Extract(ctx, client, articleURL, selector, bodyCap) (string, error)`. The only entry point. |
| `internal/extract/extract_test.go` | **create** | Readability happy + empty + bad URL; HTTP non-2xx; body cap; non-HTML content type; selector match + no-match + malformed. |
| `internal/poll/worker.go` | modify | Add `ExtractFunc` type, three new `WorkerOpts` fields (`Extract`, `ExtractConcurrency`, `ExtractBodyCap`), one new phase in `Run` between item-iteration and the per-item `processor.Process` pass. |
| `internal/poll/scheduler.go` | modify | Mirror the three new `SchedulerOpts` fields and propagate into `NewWorker`. |
| `internal/poll/worker_test.go` | modify | Six new tests: extract success, single failure, all failures, concurrency limit, no-link-skip, sanitisation-still-runs. |
| `internal/api/subscriptions.go` | modify | `POST` accepts `extract`; new `PATCH /api/v1/subscriptions/:id`; `subscriptionDTO` grows `extract` + `extract_selector`. |
| `internal/api/entries.go` | modify | `entryListItemDTO` grows `extract_failed`. |
| `internal/api/subscriptions_test.go` | modify | Cover POST extract field, PATCH happy + validation + 404 + 405 + body-cap, GET DTO shape. |
| `internal/api/entries_test.go` | modify | Cover `extract_failed` in DTO. |
| `cmd/tap/main.go` | modify | Add `--extract-concurrency` + `--extract-body-cap` flags; pass them via `SchedulerOpts`. |
| `cmd/tap/main_test.go` | modify | One new end-to-end test: subscribe with `extract=true` against a fixture origin that serves both feed and article. |
| `go.mod` / `go.sum` | modify | `go get codeberg.org/readeck/go-readability/v2@latest` + `go mod tidy`. cascadia comes transitively. |
| `README.md` | modify | New "Article extraction (M5)" section after the current trust-posture block; new `## Configuration knobs added by M5` section; new `## Upgrading from M4` section. |
| `CLAUDE.md` | modify | Status line: `M4 in review` → `M5 in progress` (or similar). |

---

## Phase A — Database foundations

### Task A1: Add migration 0004 and prove it adds the columns

**Files:**
- Create: `internal/db/migrations/0004_extraction.sql`
- Modify: `internal/db/migrate_test.go`

- [ ] **Step 1: Write the failing migrate test**

Append to `internal/db/migrate_test.go`:

```go
func TestMigrate_AddsExtractionColumns(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	cases := []struct {
		table, column, def string
	}{
		{"subscriptions", "extract", "0"},
		{"subscriptions", "extract_selector", "''"},
		{"entries", "extract_failed", "0"},
	}
	for _, c := range cases {
		var name string
		var defaultVal sql.NullString
		err := d.QueryRowContext(ctx,
			`SELECT name, "dflt_value" FROM pragma_table_info(?) WHERE name = ?`,
			c.table, c.column).Scan(&name, &defaultVal)
		require.NoError(t, err, "%s.%s not found", c.table, c.column)
		require.Equal(t, c.def, defaultVal.String, "%s.%s default", c.table, c.column)
	}

	// schema_migrations should advance to 4.
	var version int
	require.NoError(t, d.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version))
	require.Equal(t, 4, version)
}
```

Also update the existing `TestMigrate_AppliesAllMigrationsExactlyOnce` to expect version 4 (two locations: the assertion after first run and after the no-op re-run).

- [ ] **Step 2: Run the test to confirm it fails**

```bash
make test 2>&1 | grep -E "TestMigrate_(AddsExtractionColumns|AppliesAllMigrationsExactlyOnce)" | head -20
```

Expected: both tests fail. `TestMigrate_AddsExtractionColumns` fails on the `extract` column lookup; `TestMigrate_AppliesAllMigrationsExactlyOnce` fails because version is 3, not 4.

- [ ] **Step 3: Create the migration file**

Create `internal/db/migrations/0004_extraction.sql`:

```sql
ALTER TABLE subscriptions ADD COLUMN extract           INTEGER NOT NULL DEFAULT 0;
ALTER TABLE subscriptions ADD COLUMN extract_selector  TEXT    NOT NULL DEFAULT '';
ALTER TABLE entries       ADD COLUMN extract_failed    INTEGER NOT NULL DEFAULT 0;
```

- [ ] **Step 4: Run the test to confirm it passes**

```bash
make test 2>&1 | grep -E "TestMigrate_(AddsExtractionColumns|AppliesAllMigrationsExactlyOnce)" | head -20
```

Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrations/0004_extraction.sql internal/db/migrate_test.go
git commit -m "$(cat <<'EOF'
M5: migration 0004 adds extraction columns

subscriptions.extract (bool) and subscriptions.extract_selector (text)
gate per-feed extraction. entries.extract_failed (bool) records when
extraction was attempted but produced empty/error output. All default
to off/empty so existing M4 rows behave unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A2: Extend `db.DueSubscription` and `ListDuePolls` to carry the new columns

**Files:**
- Modify: `internal/db/subscriptions.go`
- Modify: `internal/db/subscriptions_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/db/subscriptions_test.go` (create a `TestListDuePolls_ReturnsExtractionFields` if no equivalent exists; check the file first and add the cases there):

```go
func TestListDuePolls_ReturnsExtractionFields(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	_, err = d.ExecContext(ctx,
		`UPDATE subscriptions SET extract = 1, extract_selector = '.body' WHERE id = ?`, id)
	require.NoError(t, err)

	due, err := ListDuePolls(ctx, d, 0, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.True(t, due[0].Extract, "Extract should round-trip true")
	require.Equal(t, ".body", due[0].ExtractSelector, "ExtractSelector should round-trip")
}
```

- [ ] **Step 2: Run the test to confirm it fails (compile error: unknown field)**

```bash
go test ./internal/db -run TestListDuePolls_ReturnsExtractionFields 2>&1 | head -10
```

Expected: build failure — `due[0].Extract` undefined on `DueSubscription`.

- [ ] **Step 3: Implement — extend the struct and the query**

Edit `internal/db/subscriptions.go`. Update the `DueSubscription` type:

```go
type DueSubscription struct {
	ID              int64
	FeedURL         string
	ETag            sql.NullString
	LastModified    sql.NullString
	ErrorCount      int
	Extract         bool
	ExtractSelector string
}
```

Update `ListDuePolls` SELECT and Scan:

```go
func ListDuePolls(ctx context.Context, d *sql.DB, now int64, limit int) ([]DueSubscription, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, feed_url, etag, last_modified, error_count, extract, extract_selector
		FROM subscriptions
		WHERE next_poll_at <= ?
		ORDER BY next_poll_at
		LIMIT ?
	`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due polls: %w", err)
	}
	defer rows.Close()

	var out []DueSubscription
	for rows.Next() {
		var s DueSubscription
		if err := rows.Scan(&s.ID, &s.FeedURL, &s.ETag, &s.LastModified,
			&s.ErrorCount, &s.Extract, &s.ExtractSelector); err != nil {
			return nil, fmt.Errorf("scan due poll: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run the test (and the rest of the package) to confirm everything passes**

```bash
go test ./internal/db -race
```

Expected: PASS. `Extract` defaults to `false` and `ExtractSelector` to `""` for any subscription where the columns weren't manually set.

- [ ] **Step 5: Commit**

```bash
git add internal/db/subscriptions.go internal/db/subscriptions_test.go
git commit -m "$(cat <<'EOF'
M5: db.DueSubscription gains Extract + ExtractSelector

ListDuePolls now reads the two new columns from subscriptions so the
worker has them at dispatch time. Adds zero overhead for non-extract
subscriptions (Extract=false is the default and the existing fast path).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A3: Extend `db.NewEntry` and `UpdateAfterPoll` to write `extract_failed`

**Files:**
- Modify: `internal/db/entries.go`
- Modify: `internal/db/entries_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/db/entries_test.go`:

```go
func TestUpdateAfterPoll_WritesExtractFailed(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	subID, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	_, err = UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix: 1_700_000_000,
		Floor:   15 * time.Minute,
		Ceiling: 24 * time.Hour,
		NewEntries: []NewEntry{
			{Hash: "ok", Title: "ok", URL: "https://x/1", Content: "<p>ok</p>", PublishedAt: 1_700_000_000, ExtractFailed: false},
			{Hash: "bad", Title: "bad", URL: "https://x/2", Content: "<p>summary</p>", PublishedAt: 1_700_000_000, ExtractFailed: true},
		},
	})
	require.NoError(t, err)

	rows, err := d.QueryContext(ctx, `SELECT hash, extract_failed FROM entries WHERE subscription_id = ? ORDER BY hash`, subID)
	require.NoError(t, err)
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var h string
		var ef int
		require.NoError(t, rows.Scan(&h, &ef))
		got[h] = ef
	}
	require.Equal(t, map[string]int{"bad": 1, "ok": 0}, got)
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/db -run TestUpdateAfterPoll_WritesExtractFailed 2>&1 | head -10
```

Expected: build failure — `NewEntry.ExtractFailed` undefined.

- [ ] **Step 3: Implement — extend struct + INSERT**

Edit `internal/db/entries.go`. Update `NewEntry`:

```go
type NewEntry struct {
	Hash          string
	Title         string
	Author        string
	URL           string
	Content       string
	PublishedAt   int64
	ExtractFailed bool
}
```

Update the INSERT inside `UpdateAfterPoll`:

```go
		res, ierr := tx.ExecContext(ctx, `
			INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, extract_failed)
			VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)
			ON CONFLICT (subscription_id, hash) DO NOTHING
		`, subID, e.Hash, e.Title, e.Author, e.URL, e.Content, e.PublishedAt, r.NowUnix, boolToInt(e.ExtractFailed))
```

- [ ] **Step 4: Run all db tests to confirm**

```bash
go test ./internal/db -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/entries.go internal/db/entries_test.go
git commit -m "$(cat <<'EOF'
M5: db.NewEntry gains ExtractFailed; UpdateAfterPoll writes the column

extract_failed defaults to 0 on NewEntry so callers that don't care
(every existing call site, until Phase D wires the worker) stay
unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A4: Add `db.UpdateSubscriptionExtraction` for the PATCH endpoint

**Files:**
- Modify: `internal/db/subscriptions.go`
- Modify: `internal/db/subscriptions_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/db/subscriptions_test.go`:

```go
func TestUpdateSubscriptionExtraction_HappyPath(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	require.NoError(t, UpdateSubscriptionExtraction(ctx, d, id, true, ".article"))

	due, err := ListDuePolls(ctx, d, 0, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.True(t, due[0].Extract)
	require.Equal(t, ".article", due[0].ExtractSelector)

	// Clearing the selector with empty string works.
	require.NoError(t, UpdateSubscriptionExtraction(ctx, d, id, true, ""))
	due, err = ListDuePolls(ctx, d, 0, 10)
	require.NoError(t, err)
	require.Equal(t, "", due[0].ExtractSelector)
}

func TestUpdateSubscriptionExtraction_MissingIDReturnsErrNoRows(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	err := UpdateSubscriptionExtraction(ctx, d, 999, true, "")
	require.ErrorIs(t, err, sql.ErrNoRows)
}
```

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/db -run TestUpdateSubscriptionExtraction 2>&1 | head -10
```

Expected: undefined `UpdateSubscriptionExtraction`.

- [ ] **Step 3: Implement**

Add to `internal/db/subscriptions.go`:

```go
// UpdateSubscriptionExtraction sets extract + extract_selector on one row.
// Returns sql.ErrNoRows if no subscription with that id exists, so the API
// layer can map cleanly to 404. Both fields are written unconditionally —
// the API layer is responsible for layering merge-patch semantics over this.
func UpdateSubscriptionExtraction(ctx context.Context, d *sql.DB, id int64, extract bool, selector string) error {
	res, err := d.ExecContext(ctx, `
		UPDATE subscriptions
		SET extract = ?, extract_selector = ?
		WHERE id = ?
	`, boolToInt(extract), selector, id)
	if err != nil {
		return fmt.Errorf("update subscription extraction %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
```

`boolToInt` already lives in `entries.go` in the same package; it's reusable.

- [ ] **Step 4: Run**

```bash
go test ./internal/db -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/subscriptions.go internal/db/subscriptions_test.go
git commit -m "$(cat <<'EOF'
M5: add db.UpdateSubscriptionExtraction helper for the PATCH endpoint

Single UPDATE; returns sql.ErrNoRows when the subscription doesn't
exist so the API layer maps cleanly to 404. The merge-patch shape
(absent vs present-empty) is the API layer's job — this helper writes
both fields unconditionally.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase B — Extractor package

### Task B1: Add the readeck dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: `go get` the new module**

```bash
go get codeberg.org/readeck/go-readability/v2@v2.1.1
go mod tidy
```

- [ ] **Step 2: Verify the build still passes**

```bash
make test
```

Expected: PASS — no code uses the new dep yet, so no behaviour change.

- [ ] **Step 3: Sanity-check the resulting go.mod**

```bash
grep -E "(readeck|cascadia|dateparse|go-shiori/dom|chardet)" go.mod
```

Expected output should contain at minimum:

```
codeberg.org/readeck/go-readability/v2 v2.1.1
github.com/andybalholm/cascadia ... // indirect    (or direct after Phase B uses it)
```

Plus three small transitives (`araddon/dateparse`, `go-shiori/dom`, `gogs/chardet`) which are pulled by readeck.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "$(cat <<'EOF'
M5: add codeberg.org/readeck/go-readability/v2 dependency

Pure-Go Mozilla Readability port; the maintained successor to the
archived go-shiori/go-readability. MIT, no CGO. cascadia comes
transitively (we'll use it directly in the next commit for selector
mode). Spec: docs/specs/2026-05-09-m5-article-extraction.md.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B2: Scaffold `internal/extract` with the Readability happy path

**Files:**
- Create: `internal/extract/extract.go`
- Create: `internal/extract/extract_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/extract/extract_test.go`:

```go
package extract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const articleHTML = `<!doctype html><html><head><title>Hello</title></head>
<body>
<header><nav><a href="/">home</a></nav></header>
<main>
<article>
<h1>The article title</h1>
<p>This is the first paragraph of a moderately long article body.
It needs enough text for Readability's heuristics to identify it as
the dominant content tree, otherwise the extractor returns empty.</p>
<p>This is the second paragraph, with similarly substantial prose so
the scorer picks &lt;article&gt; as the top candidate. Lorem ipsum dolor
sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt.</p>
<p>And a third paragraph just to be safe.</p>
</article>
</main>
<footer>copyright 2026</footer>
</body></html>`

func TestExtract_ReadabilityHappyPath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTML))
	}))
	t.Cleanup(srv.Close)

	got, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.NoError(t, err)
	require.Contains(t, got, "first paragraph", "article body must survive extraction")
	require.NotContains(t, got, "copyright 2026", "footer must be dropped by Readability")
	require.NotContains(t, got, `href="/"`, "nav links must be dropped by Readability")
}
```

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/extract -run TestExtract_ReadabilityHappyPath 2>&1 | head -10
```

Expected: build failure — package `extract` does not exist.

- [ ] **Step 3: Implement the minimal `Extract`**

Create `internal/extract/extract.go`:

```go
// Package extract fetches an article URL and returns its main body as
// HTML, either via Mozilla Readability heuristics or a per-feed CSS
// selector. The output is raw HTML — sanitisation and image-URL
// rewriting are the caller's job (the polling worker runs the result
// through processor.Process).
package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
)

// Extract fetches articleURL via client and returns the article body
// as HTML. selector empty → Readability mode; non-empty → CSS-selector
// mode. bodyCap caps the response body before parsing.
//
// Errors on: HTTP non-2xx, body cap exceeded, missing or non-HTML
// Content-Type, malformed selector, selector matched no node,
// Readability returned empty content.
//
// The fetch uses the caller-supplied client so the M4 SSRF guard,
// per-host limiter, and timeout apply uniformly with feed and proxy
// fetches.
func Extract(ctx context.Context, client *http.Client, articleURL, selector string, bodyCap int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, articleURL, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "text/html, application/xhtml+xml;q=0.9, */*;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !isHTMLContentType(ct) {
		return "", fmt.Errorf("non-HTML content-type %q", ct)
	}

	body, err := readCapped(resp.Body, bodyCap)
	if err != nil {
		return "", err
	}

	pageURL, perr := url.Parse(articleURL)
	if perr != nil {
		return "", fmt.Errorf("parse article URL: %w", perr)
	}

	if selector != "" {
		// Selector mode is added in Task B5; punt for now.
		return "", errors.New("selector mode not implemented yet")
	}

	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		return "", fmt.Errorf("readability: %w", err)
	}
	if article.Node == nil {
		return "", errors.New("readability returned empty content")
	}
	var buf bytes.Buffer
	if err := article.RenderHTML(&buf); err != nil {
		return "", fmt.Errorf("render html: %w", err)
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return "", errors.New("readability returned empty content")
	}
	return out, nil
}

// readCapped reads up to cap+1 bytes; returns an error if the body
// exceeds cap. We read one extra byte so we can distinguish "exactly
// at the cap" from "the body was truncated."
func readCapped(r io.Reader, cap int64) ([]byte, error) {
	if cap <= 0 {
		cap = 5 << 20 // 5 MiB sane default if a caller passes 0
	}
	body, err := io.ReadAll(io.LimitReader(r, cap+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > cap {
		return nil, fmt.Errorf("body exceeds %d-byte cap", cap)
	}
	return body, nil
}

// isHTMLContentType returns true for text/html and application/xhtml+xml,
// optionally with parameters (charset etc.). Comparison is on the media
// type only; parameters are ignored.
func isHTMLContentType(ct string) bool {
	if ct == "" {
		return false
	}
	mt := strings.TrimSpace(strings.ToLower(strings.SplitN(ct, ";", 2)[0]))
	return mt == "text/html" || mt == "application/xhtml+xml"
}
```

- [ ] **Step 4: Run**

```bash
go test ./internal/extract -run TestExtract_ReadabilityHappyPath -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/extract/extract.go internal/extract/extract_test.go
git commit -m "$(cat <<'EOF'
M5: scaffold internal/extract with Readability mode happy path

Extract(ctx, client, articleURL, selector, bodyCap) — selector empty
runs Readability via readeck/go-readability/v2; selector non-empty is
stubbed and returns an error until Task B5. Body is capped via a
read-one-extra-byte LimitReader so we can detect truncation. Content-Type
is restricted to text/html and application/xhtml+xml.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B3: Cover the Readability error paths (HTTP, body cap, content type, empty)

**Files:**
- Modify: `internal/extract/extract_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/extract/extract_test.go`:

```go
func TestExtract_HTTPNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}

func TestExtract_BodyCapExceeded(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("a", 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(big))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 256)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cap")
}

func TestExtract_NonHTMLContentType(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"k":"v"}`))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-HTML")
}

func TestExtract_AcceptsXHTML(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xhtml+xml")
		_, _ = w.Write([]byte(articleHTML))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.NoError(t, err)
}

func TestExtract_MissingContentType(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// httptest auto-sets Content-Type to text/plain when body is non-empty
		// and no header is set; explicitly clear it to test the empty case.
		w.Header()["Content-Type"] = nil
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.Error(t, err)
}

func TestExtract_ReadabilityEmptyContent(t *testing.T) {
	t.Parallel()
	// A page with only navigation chrome and no article-shaped content
	// should make Readability return an empty Article.
	const empty = `<!doctype html><html><body>
<nav><a href="/a">a</a><a href="/b">b</a></nav>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(empty))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.Error(t, err, "Readability should error on a chrome-only page")
}
```

- [ ] **Step 2: Run**

```bash
go test ./internal/extract -race
```

Expected: all PASS — the implementation already handles each case via the early returns added in Task B2.

If `TestExtract_ReadabilityEmptyContent` unexpectedly passes (Readability returned content) or unexpectedly errors with the wrong message, adjust the fixture HTML until it reliably triggers `article.Node == nil` or empty rendered output. The test's value is in pinning the contract, not in the fixture's exact shape.

- [ ] **Step 3: Commit**

```bash
git add internal/extract/extract_test.go
git commit -m "$(cat <<'EOF'
M5: extract — tests for HTTP, body cap, content-type, empty cases

Six tests pinning the early-return contracts: non-2xx HTTP status,
body exceeds cap, missing/non-HTML Content-Type, accepted XHTML,
Readability returns empty Article. No production-code change needed
beyond Task B2.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B4: Implement and test selector mode

**Files:**
- Modify: `internal/extract/extract.go`
- Modify: `internal/extract/extract_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/extract/extract_test.go`:

```go
func TestExtract_SelectorMode_Match(t *testing.T) {
	t.Parallel()
	const page = `<!doctype html><html><body>
<header>SITE HEADER</header>
<div class="article"><p>extracted body</p></div>
<footer>SITE FOOTER</footer>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)

	got, err := Extract(context.Background(), srv.Client(), srv.URL, ".article", 5<<20)
	require.NoError(t, err)
	require.Contains(t, got, "extracted body")
	require.NotContains(t, got, "SITE HEADER")
	require.NotContains(t, got, "SITE FOOTER")
}

func TestExtract_SelectorMode_NoMatch(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><body><p>x</p></body></html>`))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, ".missing", 5<<20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "matched no node")
}

func TestExtract_SelectorMode_MalformedSelector(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><body><p>x</p></body></html>`))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "[unclosed", 5<<20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "compile selector")
}
```

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/extract -run TestExtract_SelectorMode -race 2>&1 | head -20
```

Expected: all three fail because selector mode currently returns "selector mode not implemented yet."

- [ ] **Step 3: Implement selector mode**

Edit `internal/extract/extract.go`. Add imports:

```go
import (
	// ... existing imports
	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)
```

Replace the selector-mode stub in `Extract` with a real implementation. The relevant block:

```go
	if selector != "" {
		sel, serr := cascadia.Compile(selector)
		if serr != nil {
			return "", fmt.Errorf("compile selector: %w", serr)
		}
		doc, perr := html.Parse(bytes.NewReader(body))
		if perr != nil {
			return "", fmt.Errorf("parse html: %w", perr)
		}
		match := sel.MatchFirst(doc)
		if match == nil {
			return "", errors.New("selector matched no node")
		}
		var buf bytes.Buffer
		if err := html.Render(&buf, match); err != nil {
			return "", fmt.Errorf("render selector match: %w", err)
		}
		out := strings.TrimSpace(buf.String())
		if out == "" {
			return "", errors.New("selector match rendered empty")
		}
		return out, nil
	}
```

- [ ] **Step 4: Run all extract tests**

```bash
go test ./internal/extract -race
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/extract/extract.go internal/extract/extract_test.go
git commit -m "$(cat <<'EOF'
M5: extract — selector mode via cascadia.Compile + MatchFirst

Per-feed override path: when the caller supplies a CSS selector,
parse the page, MatchFirst, render the matched node's outer HTML.
Three error paths pinned: malformed selector (cascadia.Compile),
no matching node (MatchFirst returns nil), match rendered empty.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase C — Worker integration

### Task C1: Add `ExtractFunc` + new `WorkerOpts` / `SchedulerOpts` fields (no behaviour change yet)

**Files:**
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/scheduler.go`
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/poll/worker_test.go`:

```go
func TestNewWorker_DefaultsExtractFunc(t *testing.T) {
	t.Parallel()
	d := newDB(t)
	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	// Defaults: ExtractConcurrency = 4, ExtractBodyCap = 5 MiB,
	// Extract = extract.Extract (non-nil).
	require.Equal(t, 4, w.opts.ExtractConcurrency)
	require.Equal(t, int64(5<<20), w.opts.ExtractBodyCap)
	require.NotNil(t, w.opts.Extract)
}
```

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/poll -run TestNewWorker_DefaultsExtractFunc 2>&1 | head -10
```

Expected: build failure — unknown fields on `WorkerOpts`.

- [ ] **Step 3: Implement**

Edit `internal/poll/worker.go`. Add the imports and the new type:

```go
import (
	// ... existing imports
	"github.com/bcrisp4/tap/internal/extract"
)

// ExtractFunc matches extract.Extract — declared as a type so tests can
// inject deterministic in-memory extractors without spinning up an
// httptest origin. Same shape of test seam as Now func() time.Time.
type ExtractFunc func(ctx context.Context, client *http.Client,
	articleURL, selector string, bodyCap int64) (string, error)
```

Extend `WorkerOpts`:

```go
type WorkerOpts struct {
	Processor          *processor.Processor
	Floor              time.Duration
	Ceiling            time.Duration
	ErrorBase          time.Duration
	Now                func() time.Time
	Extract            ExtractFunc
	ExtractConcurrency int
	ExtractBodyCap     int64
}
```

Add defaulting inside `NewWorker` (after the existing `Now` default block):

```go
	if o.Extract == nil {
		o.Extract = extract.Extract
	}
	if o.ExtractConcurrency <= 0 {
		o.ExtractConcurrency = 4
	}
	if o.ExtractBodyCap <= 0 {
		o.ExtractBodyCap = 5 << 20
	}
```

Mirror the three new fields on `SchedulerOpts` in `internal/poll/scheduler.go`:

```go
type SchedulerOpts struct {
	// ... existing fields
	Extract            ExtractFunc
	ExtractConcurrency int
	ExtractBodyCap     int64
}
```

Propagate them into `NewWorker` inside `NewScheduler`:

```go
		worker: NewWorker(d, c, WorkerOpts{
			Processor:          o.Processor,
			Floor:              o.Floor,
			Ceiling:            o.Ceiling,
			ErrorBase:          o.ErrorBase,
			Now:                o.Now,
			Extract:            o.Extract,
			ExtractConcurrency: o.ExtractConcurrency,
			ExtractBodyCap:     o.ExtractBodyCap,
		}),
```

- [ ] **Step 4: Run the test and the rest of the package**

```bash
go test ./internal/poll -race
```

Expected: PASS. Behaviour unchanged for existing tests because every call site has `Extract` left zero (defaults to `extract.Extract`) and no `sub.Extract = true` anywhere yet.

- [ ] **Step 5: Commit**

```bash
git add internal/poll/worker.go internal/poll/scheduler.go internal/poll/worker_test.go
git commit -m "$(cat <<'EOF'
M5: WorkerOpts/SchedulerOpts gain Extract + ExtractConcurrency + ExtractBodyCap

Test seam pattern matches Now func() time.Time: production wires
extract.Extract, tests inject in-memory fakes. Defaults: concurrency=4,
body cap=5 MiB. No behaviour change yet — every existing call site
still has sub.Extract=false (Phase A added the column at the DB layer).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task C2: Worker calls `Extract` when `sub.Extract` is true

**Files:**
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/poll/worker_test.go`:

```go
func TestWorker_Extract_ReplacesContentOnSuccess(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Linkonly</title>
  <id>urn:linkonly</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>One</title>
    <id>urn:linkonly:1</id>
    <link href="https://link.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;teaser&lt;/p&gt;</content>
  </entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	t.Cleanup(srv.Close)

	d := newDB(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	var calls int
	fakeExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64) (string, error) {
		calls++
		require.Equal(t, "https://link.example/1", url)
		require.Equal(t, "", sel)
		return "<p>extracted body</p>", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   fakeExtract,
	})
	w.Run(context.Background(), db.DueSubscription{
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	require.Equal(t, 1, calls)
	entries, _, _, err := db.ListEntries(context.Background(), d, db.ListEntriesParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	full, err := db.GetEntry(context.Background(), d, entries[0].ID)
	require.NoError(t, err)
	require.Contains(t, full.Content, "extracted body")
	require.NotContains(t, full.Content, "teaser")
}

func TestWorker_Extract_FalseSkipsExtraction(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Linkonly</title>
  <id>urn:linkonly</id>
  <entry><title>One</title><id>urn:linkonly:1</id>
    <link href="https://link.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;teaser&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	t.Cleanup(srv.Close)

	d := newDB(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	var calls int
	fakeExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64) (string, error) {
		calls++
		return "should-not-appear", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   fakeExtract,
	})
	w.Run(context.Background(), db.DueSubscription{
		ID: subID, FeedURL: srv.URL, Extract: false,
	})

	require.Equal(t, 0, calls, "Extract must not be called when sub.Extract is false")
	entries, _, _, _ := db.ListEntries(context.Background(), d, db.ListEntriesParams{Limit: 10})
	full, _ := db.GetEntry(context.Background(), d, entries[0].ID)
	require.Contains(t, full.Content, "teaser")
}
```

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/poll -run TestWorker_Extract -race 2>&1 | head -30
```

Expected: `TestWorker_Extract_ReplacesContentOnSuccess` fails (extract not called, content is the teaser); `TestWorker_Extract_FalseSkipsExtraction` passes (extract still inert).

- [ ] **Step 3: Implement the new worker phase**

Edit `internal/poll/worker.go`. Add import:

```go
import (
	// ... existing imports
	"golang.org/x/sync/errgroup"
	"github.com/mmcdole/gofeed"
)
```

(`gofeed` may already be imported.)

Replace the existing for-loop building `newEntries` (currently lines ~107–126) with the staged shape:

```go
	type pending struct {
		item          *gofeed.Item
		content       string
		extractFailed bool
	}
	pendings := make([]pending, len(res.Feed.Items))
	for i, item := range res.Feed.Items {
		raw := item.Content
		if raw == "" {
			raw = item.Description
		}
		pendings[i] = pending{item: item, content: raw}
	}

	if sub.Extract && len(pendings) > 0 {
		g := new(errgroup.Group)
		g.SetLimit(w.opts.ExtractConcurrency)
		for i := range pendings {
			if pendings[i].item.Link == "" {
				continue
			}
			g.Go(func() error {
				extracted, eerr := w.opts.Extract(ctx, w.client,
					pendings[i].item.Link, sub.ExtractSelector, w.opts.ExtractBodyCap)
				if eerr != nil {
					slog.WarnContext(ctx, "extract failed",
						"feed_id", sub.ID,
						"entry_url", pendings[i].item.Link,
						"err", eerr)
					pendings[i].extractFailed = true
					return nil
				}
				pendings[i].content = extracted
				return nil
			})
		}
		_ = g.Wait()
	}

	newEntries := make([]db.NewEntry, 0, len(pendings))
	for _, p := range pendings {
		pubAt := now.Unix()
		if p.item.PublishedParsed != nil {
			pubAt = p.item.PublishedParsed.Unix()
		}
		newEntries = append(newEntries, db.NewEntry{
			Hash:          feed.EntryHash(sub.ID, p.item),
			Title:         p.item.Title,
			Author:        authorName(p.item),
			URL:           p.item.Link,
			Content:       w.opts.Processor.Process(p.content),
			PublishedAt:   pubAt,
			ExtractFailed: p.extractFailed,
		})
	}
```

- [ ] **Step 4: Run all worker tests + race detector**

```bash
go test ./internal/poll -race
```

Expected: all PASS, including the existing M4 tests (no behaviour change for non-extract polls).

- [ ] **Step 5: Commit**

```bash
git add internal/poll/worker.go internal/poll/worker_test.go
git commit -m "$(cat <<'EOF'
M5: worker calls Extract when sub.Extract is true

New phase between item-iteration and the existing per-item
processor.Process pass. Sequential build of a pending[] slice; one
errgroup.SetLimit(ExtractConcurrency) goroutine per item with a non-
empty Link; ctx propagates through to extract; output replaces the
feed-provided content. sub.Extract=false leaves the existing fast
path intact.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task C3: Per-entry extract failures degrade to summary + `extract_failed=1`

**Files:**
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/poll/worker_test.go`:

```go
func TestWorker_Extract_OneFailureFallsBackToSummary(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Mix</title><id>urn:mix</id>
  <entry><title>Ok</title><id>urn:mix:1</id>
    <link href="https://mix.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;summary-1&lt;/p&gt;</content></entry>
  <entry><title>Bad</title><id>urn:mix:2</id>
    <link href="https://mix.example/2"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;summary-2&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	t.Cleanup(srv.Close)

	d := newDB(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	fakeExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64) (string, error) {
		if strings.HasSuffix(url, "/2") {
			return "", errors.New("simulated extract failure")
		}
		return "<p>extracted-1</p>", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   fakeExtract,
	})
	w.Run(context.Background(), db.DueSubscription{
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	// Subscription error_count must NOT bump — extract failure is per-entry.
	got, err := db.GetSubscription(context.Background(), d, subID)
	require.NoError(t, err)
	require.Equal(t, 0, got.ErrorCount, "extract failure must not bump subscription error_count")
	require.False(t, got.LastError.Valid, "extract failure must not write last_error")

	// Both entries inserted; per-entry extract_failed reflects the outcome.
	rows, err := d.QueryContext(context.Background(),
		`SELECT title, content, extract_failed FROM entries WHERE subscription_id = ? ORDER BY title`, subID)
	require.NoError(t, err)
	defer rows.Close()
	type row struct {
		title, content string
		failed         int
	}
	var got2 []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.title, &r.content, &r.failed))
		got2 = append(got2, r)
	}
	require.Len(t, got2, 2)
	require.Equal(t, "Bad", got2[0].title)
	require.Equal(t, 1, got2[0].failed)
	require.Contains(t, got2[0].content, "summary-2")
	require.Equal(t, "Ok", got2[1].title)
	require.Equal(t, 0, got2[1].failed)
	require.Contains(t, got2[1].content, "extracted-1")
}

func TestWorker_Extract_AllFailuresPollStillSucceeds(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>AllBad</title><id>urn:allbad</id>
  <entry><title>One</title><id>urn:allbad:1</id>
    <link href="https://allbad.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;s1&lt;/p&gt;</content></entry>
  <entry><title>Two</title><id>urn:allbad:2</id>
    <link href="https://allbad.example/2"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;s2&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	t.Cleanup(srv.Close)

	d := newDB(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	failExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64) (string, error) {
		return "", errors.New("always fail")
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   failExtract,
	})
	w.Run(context.Background(), db.DueSubscription{
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	got, err := db.GetSubscription(context.Background(), d, subID)
	require.NoError(t, err)
	require.Equal(t, 0, got.ErrorCount)

	var failed int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM entries WHERE subscription_id = ? AND extract_failed = 1`,
		subID).Scan(&failed))
	require.Equal(t, 2, failed)
}
```

These need `errors` and `strings` in the import block — add them if not already present.

- [ ] **Step 2: Run**

```bash
go test ./internal/poll -run TestWorker_Extract -race
```

Expected: PASS. The implementation from Task C2 already swallows per-entry failures; these tests just pin the contract.

- [ ] **Step 3: Commit**

```bash
git add internal/poll/worker_test.go
git commit -m "$(cat <<'EOF'
M5: worker — extract failures isolated per entry, never abort the poll

Two new tests pin concept §6.5: a single extract failure leaves the
other entries unaffected and writes extract_failed=1 on the failed
entry; all-failures still commits the poll with all entries' summaries
and never bumps the subscription's error_count.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task C4: Concurrency limit, no-link skip, and sanitisation-still-runs

**Files:**
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/poll/worker_test.go`:

```go
func TestWorker_Extract_ConcurrencyLimit(t *testing.T) {
	t.Parallel()
	// Build an atom feed with 5 entries; assert the fake extractor never
	// sees more than 2 in flight when ExtractConcurrency=2.
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>Burst</title><id>urn:burst</id>`)
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&sb, `<entry><title>e%d</title><id>urn:burst:%d</id>
		<link href="https://burst.example/%d"/>
		<updated>2026-05-01T00:00:00Z</updated>
		<content type="html">&lt;p&gt;s&lt;/p&gt;</content></entry>`, i, i, i)
	}
	sb.WriteString(`</feed>`)
	atom := sb.String()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	t.Cleanup(srv.Close)

	d := newDB(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	var (
		mu       sync.Mutex
		inFlight int
		peak     int
	)
	release := make(chan struct{})
	barrier := func(ctx context.Context, c *http.Client, url, sel string, cap int64) (string, error) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()
		<-release
		mu.Lock()
		inFlight--
		mu.Unlock()
		return "<p>x</p>", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor:          processor.New(sanitise.DefaultPolicy(), nil),
		Extract:            barrier,
		ExtractConcurrency: 2,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(context.Background(), db.DueSubscription{
			ID: subID, FeedURL: srv.URL, Extract: true,
		})
	}()

	// Wait until the limiter has settled at peak=2, then release everything.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return peak == 2
	}, 2*time.Second, 5*time.Millisecond)

	close(release)
	<-done

	require.Equal(t, 2, peak, "ExtractConcurrency=2 must cap at-most 2 in flight")
}

func TestWorker_Extract_NoLinkSkipsSilently(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>Nolink</title><id>urn:nolink</id>
  <entry><title>NoLink</title><id>urn:nolink:1</id>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;summary&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	t.Cleanup(srv.Close)

	d := newDB(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	var calls int
	fakeExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64) (string, error) {
		calls++
		return "should-not-appear", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   fakeExtract,
	})
	w.Run(context.Background(), db.DueSubscription{
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	require.Equal(t, 0, calls, "no-Link entries must skip extraction silently")
	var failed int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM entries WHERE subscription_id = ? AND extract_failed = 1`,
		subID).Scan(&failed))
	require.Equal(t, 0, failed, "no-Link entries must NOT have extract_failed=1")
}

func TestWorker_Extract_OutputStillSanitised(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>San</title><id>urn:san</id>
  <entry><title>One</title><id>urn:san:1</id>
    <link href="https://san.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;summary&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	t.Cleanup(srv.Close)

	d := newDB(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	hostileExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64) (string, error) {
		return `<p>real article</p><script>alert(1)</script>`, nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   hostileExtract,
	})
	w.Run(context.Background(), db.DueSubscription{
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	entries, _, _, _ := db.ListEntries(context.Background(), d, db.ListEntriesParams{Limit: 10})
	full, _ := db.GetEntry(context.Background(), d, entries[0].ID)
	require.Contains(t, full.Content, "real article")
	require.NotContains(t, full.Content, "<script>", "extracted output must run through processor.Process")
	require.NotContains(t, full.Content, "alert", "extracted output must run through processor.Process")
}
```

These need `sync` in the imports. Add it if not already present.

- [ ] **Step 2: Run**

```bash
go test ./internal/poll -run TestWorker_Extract -race
```

Expected: all PASS. No production-code change is needed beyond Tasks C1 and C2.

- [ ] **Step 3: Commit**

```bash
git add internal/poll/worker_test.go
git commit -m "$(cat <<'EOF'
M5: worker — concurrency limit, no-link skip, sanitisation pins

Three pins on the spec contracts: ExtractConcurrency=2 caps at-most-2
in flight via a barrier-style fake; entries with empty Link skip
extraction silently and keep extract_failed=0; extracted output runs
through processor.Process so script tags are scrubbed and image URLs
get rewritten by the existing M2/M3 pipeline.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase D — API extensions

### Task D1: `POST /api/v1/subscriptions` accepts `extract`; subscription DTO grows two fields

**Files:**
- Modify: `internal/api/subscriptions.go`
- Modify: `internal/api/subscriptions_test.go`
- Modify: `internal/db/subscriptions.go` (add `Extract` + `ExtractSelector` to `Subscription` and read them in `GetSubscription`/`ListSubscriptions`)

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/subscriptions_test.go`:

```go
func TestPostSubscriptions_PersistsExtractFlag(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body := strings.NewReader(`{"feed_url":"https://x.example/feed","extract":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var got map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, true, got["extract"])
	require.Equal(t, "", got["extract_selector"])
}

func TestPostSubscriptions_DefaultsExtractFalse(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body := strings.NewReader(`{"feed_url":"https://x.example/feed"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var got map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, false, got["extract"])
}
```

`newAPI(t) (*http.ServeMux, *sql.DB)` is the existing helper at `internal/api/subscriptions_test.go:16`. These tests live inside `package api`, so use unqualified `NewMux`/`MuxOpts` (no `api.` prefix), the way `TestSubscriptions_PostThenList` does.

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/api -run TestPostSubscriptions -race 2>&1 | head -20
```

Expected: tests fail because `extract`/`extract_selector` aren't in the DTO and the POST handler doesn't accept the field.

- [ ] **Step 3: Implement**

Edit `internal/db/subscriptions.go`. Update the `Subscription` struct:

```go
type Subscription struct {
	ID              int64
	Title           string
	FeedURL         string
	SiteURL         sql.NullString
	LastPollAt      sql.NullInt64
	NextPollAt      int64
	ETag            sql.NullString
	LastModified    sql.NullString
	ErrorCount      int
	LastError       sql.NullString
	CreatedAt       int64
	Extract         bool
	ExtractSelector string
}
```

Update both `GetSubscription` and `ListSubscriptions` to SELECT and Scan the two new columns:

```go
func GetSubscription(ctx context.Context, d *sql.DB, id int64) (Subscription, error) {
	var s Subscription
	err := d.QueryRowContext(ctx, `
		SELECT id, title, feed_url, site_url, last_poll_at, next_poll_at,
		       etag, last_modified, error_count, last_error, created_at,
		       extract, extract_selector
		FROM subscriptions WHERE id = ?
	`, id).Scan(&s.ID, &s.Title, &s.FeedURL, &s.SiteURL, &s.LastPollAt, &s.NextPollAt,
		&s.ETag, &s.LastModified, &s.ErrorCount, &s.LastError, &s.CreatedAt,
		&s.Extract, &s.ExtractSelector)
	if err != nil {
		return Subscription{}, fmt.Errorf("get subscription %d: %w", id, err)
	}
	return s, nil
}

func ListSubscriptions(ctx context.Context, d *sql.DB) ([]Subscription, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, title, feed_url, site_url, last_poll_at, next_poll_at,
		       etag, last_modified, error_count, last_error, created_at,
		       extract, extract_selector
		FROM subscriptions ORDER BY title COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var out []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.Title, &s.FeedURL, &s.SiteURL, &s.LastPollAt, &s.NextPollAt,
			&s.ETag, &s.LastModified, &s.ErrorCount, &s.LastError, &s.CreatedAt,
			&s.Extract, &s.ExtractSelector); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
```

Add an `Extract` field to `NewSubscription`, and update `InsertSubscription` to write it:

```go
type NewSubscription struct {
	Title    string
	FeedURL  string
	SiteURL  string
	NextPoll int64
	Created  int64
	Extract  bool
}

func InsertSubscription(ctx context.Context, d *sql.DB, s NewSubscription) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO subscriptions (title, feed_url, site_url, next_poll_at, created_at, extract)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?)
	`, s.Title, s.FeedURL, s.SiteURL, s.NextPoll, s.Created, boolToInt(s.Extract))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: subscriptions.feed_url") {
			return 0, ErrSubscriptionExists
		}
		return 0, fmt.Errorf("insert subscription: %w", err)
	}
	return res.LastInsertId()
}
```

Edit `internal/api/subscriptions.go`. Grow `subscriptionDTO`:

```go
type subscriptionDTO struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	FeedURL         string `json:"feed_url"`
	SiteURL         string `json:"site_url,omitempty"`
	NextPollAt      int64  `json:"next_poll_at"`
	LastPollAt      int64  `json:"last_poll_at,omitempty"`
	ErrorCount      int    `json:"error_count"`
	LastError       string `json:"last_error,omitempty"`
	CreatedAt       int64  `json:"created_at"`
	Extract         bool   `json:"extract"`
	ExtractSelector string `json:"extract_selector"`
}
```

Update `toDTO` to populate the two new fields:

```go
func toDTO(s db.Subscription) subscriptionDTO {
	d := subscriptionDTO{
		ID:              s.ID,
		Title:           s.Title,
		FeedURL:         s.FeedURL,
		NextPollAt:      s.NextPollAt,
		ErrorCount:      s.ErrorCount,
		CreatedAt:       s.CreatedAt,
		Extract:         s.Extract,
		ExtractSelector: s.ExtractSelector,
	}
	if s.SiteURL.Valid {
		d.SiteURL = s.SiteURL.String
	}
	if s.LastPollAt.Valid {
		d.LastPollAt = s.LastPollAt.Int64
	}
	if s.LastError.Valid {
		d.LastError = s.LastError.String
	}
	return d
}
```

Update the POST handler's request body and call site:

```go
		var body struct {
			FeedURL string `json:"feed_url"`
			Title   string `json:"title"`
			Extract bool   `json:"extract"`
		}
		// ... existing decode + URL validation ...
		id, err := db.InsertSubscription(r.Context(), d, db.NewSubscription{
			Title:    title,
			FeedURL:  body.FeedURL,
			NextPoll: 0,
			Created:  time.Now().Unix(),
			Extract:  body.Extract,
		})
```

- [ ] **Step 4: Run**

```bash
go test ./... -race
```

Expected: all PASS, including the existing API tests (their POST bodies don't set `extract`, so it defaults to false and the tests still pass).

- [ ] **Step 5: Commit**

```bash
git add internal/api/subscriptions.go internal/api/subscriptions_test.go internal/db/subscriptions.go
git commit -m "$(cat <<'EOF'
M5: POST /api/v1/subscriptions accepts extract; DTO grows two fields

POST request body grows an optional 'extract' bool (default false).
subscriptionDTO and db.Subscription both gain extract +
extract_selector so GET responses surface the toggle state for the
SPA. Per spec, extract_selector is PATCH-only at write time — the
user won't know the right selector before they've seen the feed
render.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task D2: `PATCH /api/v1/subscriptions/:id` — happy path

**Files:**
- Modify: `internal/api/subscriptions.go`
- Modify: `internal/api/errors.go`
- Modify: `internal/api/subscriptions_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/api/subscriptions_test.go`:

```go
func TestPatchSubscription_TogglesExtract(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	body := strings.NewReader(`{"extract":true}`)
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10), body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var got map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, true, got["extract"])
}

func TestPatchSubscription_SetsAndClearsSelector(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)
	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	patch := func(body string) map[string]any {
		req := httptest.NewRequest(http.MethodPatch,
			"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10),
			strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var out map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&out))
		return out
	}

	got := patch(`{"extract_selector":".article-body"}`)
	require.Equal(t, ".article-body", got["extract_selector"])

	got = patch(`{"extract_selector":""}`)
	require.Equal(t, "", got["extract_selector"])
}
```

If `postSubscription` doesn't already exist as a test helper, add it at the top of the test file:

```go
func postSubscription(t *testing.T, mux http.Handler, body string) int64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var got struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	return got.ID
}
```

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/api -run TestPatchSubscription -race 2>&1 | head -20
```

Expected: 405 Method Not Allowed (no handler registered for PATCH on the path).

- [ ] **Step 3: Implement the PATCH handler**

Edit `internal/api/errors.go`. Add a new stable error code:

```go
const (
	ErrCodeBadRequest             = "bad_request"
	ErrCodeNotFound               = "not_found"
	ErrCodeConflict               = "conflict"
	ErrCodeInternal               = "internal"
	ErrCodeExtractSelectorInvalid = "extract_selector_invalid"
)
```

Edit `internal/api/subscriptions.go`. Add the import:

```go
import (
	// ... existing
	"github.com/andybalholm/cascadia"
)
```

Add a new handler registration inside `registerSubscriptionRoutes`:

```go
	m.HandleFunc("PATCH /api/v1/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			Extract         *bool   `json:"extract"`
			ExtractSelector *string `json:"extract_selector"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		// Read existing row for merge-patch semantics.
		s, err := db.GetSubscription(r.Context(), d, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		extract := s.Extract
		selector := s.ExtractSelector
		if body.Extract != nil {
			extract = *body.Extract
		}
		if body.ExtractSelector != nil {
			candidate := *body.ExtractSelector
			if candidate != "" {
				if _, cerr := cascadia.Compile(candidate); cerr != nil {
					writeError(w, http.StatusBadRequest, ErrCodeExtractSelectorInvalid,
						"extract_selector did not compile: "+cerr.Error())
					return
				}
			}
			selector = candidate
		}

		if err := db.UpdateSubscriptionExtraction(r.Context(), d, id, extract, selector); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		updated, err := db.GetSubscription(r.Context(), d, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toDTO(updated))
	})
```

The `db.GetSubscription` call requires the `Subscription` struct to be reachable from the API package. It already is (the package imports `db`).

Note: `db.GetSubscription` currently returns a wrapped `sql.ErrNoRows` (`fmt.Errorf("get subscription %d: %w", ...)`); the `errors.Is` check above relies on `%w` preserving the chain.

- [ ] **Step 4: Run**

```bash
go test ./internal/api -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/errors.go internal/api/subscriptions.go internal/api/subscriptions_test.go
git commit -m "$(cat <<'EOF'
M5: PATCH /api/v1/subscriptions/:id toggles extract + sets selector

Narrow merge-patch endpoint accepting {extract?: bool,
extract_selector?: string}. Uses pointer-typed request DTO fields to
distinguish absent from present-empty (empty selector clears the
override). Read-modify-write against db.GetSubscription preserves the
"omitted = unchanged" contract. Selectors compile through cascadia at
write time so the user sees errors immediately rather than silently
on the next poll.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task D3: PATCH error paths — invalid selector, 404, body cap

**Files:**
- Modify: `internal/api/subscriptions_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/subscriptions_test.go`:

```go
func TestPatchSubscription_MalformedSelectorReturns400(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)
	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	body := strings.NewReader(`{"extract_selector":"[unclosed"}`)
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10), body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), `"code":"extract_selector_invalid"`)

	// Confirm the DB row is unchanged.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	require.Equal(t, http.StatusOK, getRR.Code)
	require.Contains(t, getRR.Body.String(), `"extract_selector":""`)
}

func TestPatchSubscription_UnknownIDReturns404(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body := strings.NewReader(`{"extract":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/9999", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

func TestPatchSubscription_MalformedJSONReturns400(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)
	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10),
		strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func TestPatchSubscription_PartialUpdate_ExtractOnlyKeepsSelector(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)
	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	// Set selector first.
	patch := func(body string) {
		req := httptest.NewRequest(http.MethodPatch,
			"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10),
			strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	}
	patch(`{"extract_selector":".article"}`)

	// Toggle extract; selector must NOT be cleared.
	patch(`{"extract":true}`)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	require.Contains(t, getRR.Body.String(), `"extract_selector":".article"`)
	require.Contains(t, getRR.Body.String(), `"extract":true`)
}
```

- [ ] **Step 2: Run**

```bash
go test ./internal/api -run TestPatchSubscription -race
```

Expected: all PASS — Task D2's implementation already covers each path.

- [ ] **Step 3: Commit**

```bash
git add internal/api/subscriptions_test.go
git commit -m "$(cat <<'EOF'
M5: PATCH subscription — pin selector validation, 404, partial update

Four tests pin the contracts: malformed selector → 400 with stable
code extract_selector_invalid and the row is unchanged; unknown id →
404; malformed JSON → 400; partial PATCH (extract only) preserves the
existing selector via the read-modify-write merge-patch path.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task D4: Entries DTO grows `extract_failed`

**Files:**
- Modify: `internal/db/entries.go`
- Modify: `internal/api/entries.go`
- Modify: `internal/api/entries_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/api/entries_test.go`:

```go
func TestGetEntry_IncludesExtractFailed(t *testing.T) {
	t.Parallel()
	mux, d := newAPI(t)

	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(), `
		INSERT INTO entries (subscription_id, hash, title, url, content, published_at, fetched_at, extract_failed)
		VALUES (?, 'h', 'T', 'https://x/1', '<p>x</p>', 0, 0, 1)
	`, subID)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"extract_failed":true`)
}
```

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/api -run TestGetEntry_IncludesExtractFailed -race 2>&1 | head -10
```

Expected: fail because the DTO doesn't expose the field.

- [ ] **Step 3: Implement**

Edit `internal/db/entries.go`. Update the `Entry` struct:

```go
type Entry struct {
	ID             int64
	SubscriptionID int64
	Hash           string
	Title          string
	Author         sql.NullString
	URL            string
	Content        string
	PublishedAt    int64
	FetchedAt      int64
	Read           bool
	Saved          bool
	ExtractFailed  bool
}
```

Update both `ListEntries` and `GetEntry` SELECTs and Scans to include `extract_failed`:

```go
// In ListEntries — update the SELECT clause:
		SELECT id, subscription_id, hash, title, author, url, '' AS content,
		       published_at, fetched_at, read, saved, extract_failed
		FROM entries %s
		...

// And the Scan:
		if err := rows.Scan(&e.ID, &e.SubscriptionID, &e.Hash, &e.Title, &e.Author,
			&e.URL, &e.Content, &e.PublishedAt, &e.FetchedAt, &e.Read, &e.Saved,
			&e.ExtractFailed); err != nil {
			...
```

```go
// In GetEntry:
	err := d.QueryRowContext(ctx, `
		SELECT id, subscription_id, hash, title, author, url, content,
		       published_at, fetched_at, read, saved, extract_failed
		FROM entries WHERE id = ?
	`, id).Scan(&e.ID, &e.SubscriptionID, &e.Hash, &e.Title, &e.Author,
		&e.URL, &e.Content, &e.PublishedAt, &e.FetchedAt, &e.Read, &e.Saved,
		&e.ExtractFailed)
```

Edit `internal/api/entries.go`. Grow the DTO:

```go
type entryListItemDTO struct {
	ID             int64  `json:"id"`
	SubscriptionID int64  `json:"subscription_id"`
	Title          string `json:"title"`
	Author         string `json:"author,omitempty"`
	URL            string `json:"url"`
	PublishedAt    int64  `json:"published_at"`
	FetchedAt      int64  `json:"fetched_at"`
	Read           bool   `json:"read"`
	Saved          bool   `json:"saved"`
	ExtractFailed  bool   `json:"extract_failed"`
}
```

Update `toListItem`:

```go
func toListItem(e db.Entry) entryListItemDTO {
	d := entryListItemDTO{
		ID:             e.ID,
		SubscriptionID: e.SubscriptionID,
		Title:          e.Title,
		URL:            e.URL,
		PublishedAt:    e.PublishedAt,
		FetchedAt:      e.FetchedAt,
		Read:           e.Read,
		Saved:          e.Saved,
		ExtractFailed:  e.ExtractFailed,
	}
	if e.Author.Valid {
		d.Author = e.Author.String
	}
	return d
}
```

- [ ] **Step 4: Run**

```bash
go test ./... -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/entries.go internal/api/entries.go internal/api/entries_test.go
git commit -m "$(cat <<'EOF'
M5: entries DTO + db.Entry grow extract_failed

Cheap forward-compat for an M8 SPA badge ("Extraction failed; showing
feed summary"). Added to both list and detail responses; the field
is bool and defaults to false so existing M4 callers see no change.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase E — `cmd/tap` wiring + end-to-end test

### Task E1: Add `--extract-concurrency` and `--extract-body-cap` flags

**Files:**
- Modify: `cmd/tap/main.go`

- [ ] **Step 1: Add the flag declarations**

Edit the flag block in `cmd/tap/main.go`:

```go
		extractConcurrency = flag.Int("extract-concurrency", envOrInt("TAP_EXTRACT_CONCURRENCY", 4),
			"per-worker parallel article fetches when a subscription has extract=true")
		extractBodyCap = flag.Int64("extract-body-cap-bytes", envOrInt64("TAP_EXTRACT_BODY_CAP_BYTES", 5<<20),
			"per-article HTTP body cap before extraction parses it")
```

Pass them through `SchedulerOpts`:

```go
	sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
		Processor:          proc,
		Floor:              *pollFloor,
		Ceiling:            *pollCeiling,
		ErrorBase:          *pollErrorBase,
		ExtractConcurrency: *extractConcurrency,
		ExtractBodyCap:     *extractBodyCap,
	})
```

`SchedulerOpts.Extract` is left zero so `NewWorker` defaults it to `extract.Extract`.

- [ ] **Step 2: Verify the build**

```bash
go build ./cmd/tap
make test
```

Expected: build succeeds; all tests pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/tap/main.go
git commit -m "$(cat <<'EOF'
M5: cmd/tap flags — extract-concurrency, extract-body-cap

Wires the M5 knobs through SchedulerOpts so operators can tune the
per-worker parallel article-fetch count and per-article body cap
without rebuilding. Extract func itself is left at the NewWorker
default (extract.Extract) so the production binary uses the real
extractor.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task E2: End-to-end test — subscribe with `extract=true`, articles roundtrip

**Files:**
- Modify: `cmd/tap/main_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/tap/main_test.go`:

```go
func TestEndToEnd_ExtractRoundtrip(t *testing.T) {
	t.Parallel()

	// Origin serves both the feed and the article HTML on different paths.
	const article = `<!doctype html><html><head><title>Hi</title></head>
<body><header>NAV</header><main><article>
<h1>Real headline</h1>
<p>Substantial article body that Readability can pick out as the
dominant content tree. Enough text here to trip the heuristic.</p>
<p>Second paragraph for ballast — Readability needs roughly two or
three real paragraphs to score the article subtree as the winner.</p>
<p>Third paragraph because the scorer is conservative.</p>
</article></main><footer>FOOT</footer></body></html>`

	var origin *httptest.Server
	originHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/feed" {
			w.Header().Set("Content-Type", "application/atom+xml")
			fmt.Fprintf(w, `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
<title>e2e</title><id>urn:e2e</id>
<entry><title>One</title><id>urn:e2e:1</id>
<link href="%s/article/1"/>
<updated>2026-05-01T00:00:00Z</updated>
<content type="html">&lt;p&gt;teaser&lt;/p&gt;</content>
</entry></feed>`, origin.URL)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(article))
	}
	origin = httptest.NewServer(http.HandlerFunc(originHandler))
	t.Cleanup(origin.Close)

	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))

	// Default httpx client is fine for the test origin (httptest binds to
	// 127.0.0.1, which the SSRF guard rejects by default — so build a no-SSRF
	// client for the test).
	noSSRFClient, err := buildTestClient(t)
	require.NoError(t, err)

	mux := api.NewMux(d, api.MuxOpts{})

	// Subscribe with extract=true.
	subID := postSubE2E(t, mux, fmt.Sprintf(`{"feed_url":"%s/feed","extract":true}`, origin.URL))

	// Drive the scheduler with the no-SSRF client.
	sched := poll.NewScheduler(context.Background(), d, noSSRFClient, poll.SchedulerOpts{
		Workers:   1,
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	defer sched.Stop()
	sched.Tick(context.Background())
	require.NoError(t, sched.Wait(10*time.Second))

	// GET the entry and assert the extracted body, not the teaser, is stored.
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "One", resp.Data[0]["title"])

	entryID := int64(resp.Data[0]["id"].(float64))
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet,
		"/api/v1/entries/"+strconv.FormatInt(entryID, 10), nil))
	require.Equal(t, http.StatusOK, rr2.Code)
	var detail struct {
		Content       string `json:"content"`
		ExtractFailed bool   `json:"extract_failed"`
	}
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&detail))
	require.False(t, detail.ExtractFailed)
	require.Contains(t, detail.Content, "Substantial article body", "extracted content must reach storage")
	require.NotContains(t, detail.Content, "teaser", "feed-provided summary must be replaced")
	require.NotContains(t, detail.Content, "FOOT", "footer must be dropped by Readability")

	_ = subID
}

// buildTestClient returns a client whose SSRF policy allows localhost so the
// test origin (bound to 127.0.0.1 by httptest) is reachable.
func buildTestClient(t *testing.T) (*http.Client, error) {
	t.Helper()
	pol, err := httpx.ParseSSRFPolicy(false, []string{"127.0.0.1/32"})
	if err != nil {
		return nil, err
	}
	return httpx.NewClient(httpx.Opts{
		Timeout:         10 * time.Second,
		PerHostInflight: 4,
		SSRF:            pol,
		UserAgent:       "tap-test/0.1",
	}), nil
}

// postSubE2E creates a subscription via POST and returns its id. Mirrors
// internal/api's postSubscription helper but locally-scoped to cmd/tap.
func postSubE2E(t *testing.T, mux http.Handler, body string) int64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var got struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	return got.ID
}
```

`fmt`, `strings`, and `time` should already be imported via the existing test file; add them if not.

- [ ] **Step 2: Run**

```bash
go test ./cmd/tap -run TestEndToEnd_ExtractRoundtrip -race
```

Expected: PASS.

If it fails because Readability didn't classify the fixture as an article (returns empty), beef up the fixture text until the heuristic accepts it. The test value is in the contract pin (subscribe-extract-true → extracted body in storage), not the exact fixture HTML.

- [ ] **Step 3: Commit**

```bash
git add cmd/tap/main_test.go
git commit -m "$(cat <<'EOF'
M5: e2e test — subscribe with extract=true, article body roundtrips

End-to-end exercise: the test origin serves a feed pointing at an
article URL on the same origin; subscribing with extract=true and
ticking the scheduler results in the extracted article body
(sanitised) reaching storage and surfacing through GET
/api/v1/entries/:id. extract_failed is false on the entry.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task E3: End-to-end test — extract failure path falls back to summary, poll still succeeds

**Files:**
- Modify: `cmd/tap/main_test.go`

This test exercises the full real-component path through the shared httpx client when the article URL returns a 500. Spec DoD #4 ("a poll where 1-of-1 article fetches fails: entry lands with extract_failed=1 + summary; subscription error_count stays 0") in its end-to-end form.

- [ ] **Step 1: Write the failing test**

Append to `cmd/tap/main_test.go`:

```go
func TestEndToEnd_ExtractFailure_FallsBackToSummary(t *testing.T) {
	t.Parallel()

	var origin *httptest.Server
	originHandler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed":
			w.Header().Set("Content-Type", "application/atom+xml")
			fmt.Fprintf(w, `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
<title>e2e-fail</title><id>urn:e2e-fail</id>
<entry><title>One</title><id>urn:e2e-fail:1</id>
<link href="%s/article/1"/>
<updated>2026-05-01T00:00:00Z</updated>
<content type="html">&lt;p&gt;feed-summary&lt;/p&gt;</content>
</entry></feed>`, origin.URL)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
	origin = httptest.NewServer(http.HandlerFunc(originHandler))
	t.Cleanup(origin.Close)

	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))

	noSSRFClient, err := buildTestClient(t)
	require.NoError(t, err)

	mux := api.NewMux(d, api.MuxOpts{})
	postSubE2E(t, mux, fmt.Sprintf(`{"feed_url":"%s/feed","extract":true}`, origin.URL))

	sched := poll.NewScheduler(context.Background(), d, noSSRFClient, poll.SchedulerOpts{
		Workers:   1,
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	defer sched.Stop()
	sched.Tick(context.Background())
	require.NoError(t, sched.Wait(10*time.Second))

	// Subscription error_count must NOT bump — extract failure is per-entry.
	rrSub := httptest.NewRecorder()
	mux.ServeHTTP(rrSub, httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil))
	require.Equal(t, http.StatusOK, rrSub.Code)
	var subResp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rrSub.Body).Decode(&subResp))
	require.Len(t, subResp.Data, 1)
	require.Equal(t, float64(0), subResp.Data[0]["error_count"], "extract failure must not bump subscription error_count")

	// Entry must exist with extract_failed=true and the feed-provided summary.
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil))
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&listResp))
	require.Len(t, listResp.Data, 1)
	require.Equal(t, true, listResp.Data[0]["extract_failed"])

	entryID := int64(listResp.Data[0]["id"].(float64))
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet,
		"/api/v1/entries/"+strconv.FormatInt(entryID, 10), nil))
	var detail struct {
		Content string `json:"content"`
	}
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&detail))
	require.Contains(t, detail.Content, "feed-summary",
		"failed extraction must fall back to the feed-provided summary")
}
```

- [ ] **Step 2: Run**

```bash
go test ./cmd/tap -run TestEndToEnd_ExtractFailure -race
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/tap/main_test.go
git commit -m "$(cat <<'EOF'
M5: e2e test — extract failure falls back to summary, poll succeeds

Pins spec DoD #4 in its end-to-end form: when the article fetch
returns 500, the entry still lands (with extract_failed=true + the
feed-provided summary), the poll completes, and the subscription's
error_count stays 0. Exercises the real shared httpx client through
extract.Extract, not a worker-test fake.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase F — Documentation

### Task F1: README — trust posture, upgrade note, config knobs

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update the trust-posture section**

Edit `README.md`. After the existing M4 trust-posture paragraph (the one ending "...the binary logs a startup WARN when this flag is on."), add:

```markdown
Subscriptions can opt into full-article extraction (M5):
`POST /api/v1/subscriptions {..., "extract": true}` or
`PATCH /api/v1/subscriptions/:id {"extract": true}`. When enabled, the
worker fetches each new entry's article URL through the shared HTTP
client (so the M4 SSRF guard, per-host cap, and `--http-timeout` apply)
and runs the response through Readability — or, if the subscription
has an `extract_selector` CSS rule set, through that selector.
Extracted HTML flows through the same M2 sanitiser and M3 image proxy
as feed-provided HTML. Per-entry extraction failures degrade to the
feed-provided summary with `extract_failed = 1`; they never abort the
poll or count against the subscription's error budget. Toggling
extract from off→on affects future polls only — existing entries are
not re-fetched.
```

- [ ] **Step 2: Append to "Configuration knobs added by M4" — or add a new "M5" section**

Add a new section after "Configuration knobs added by M4":

```markdown
## Configuration knobs added by M5

- `--extract-concurrency` (env `TAP_EXTRACT_CONCURRENCY`, default `4`)
  — per-worker parallel article fetches when a subscription has
  `extract=true`. The M4 per-host cap further serialises same-host
  bursts.
- `--extract-body-cap-bytes` (env `TAP_EXTRACT_BODY_CAP_BYTES`,
  default `5242880` (5 MiB)) — per-article HTTP body cap before the
  extractor parses it.
```

- [ ] **Step 3: Add an "Upgrading from M4" section**

After the existing `## Upgrading from M3` block:

```markdown
## Upgrading from M4

No destructive change required. Migration 0004 adds three columns:
`subscriptions.extract`, `subscriptions.extract_selector`, and
`entries.extract_failed`. All default to off/empty; existing
subscriptions stay non-extract until opted in via PATCH.
```

- [ ] **Step 4: Verify by building once**

```bash
make build
```

Expected: builds cleanly.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
M5: README — trust posture, M5 config knobs, M4→M5 upgrade note

Documents the per-feed extraction toggle, the two new flags, and the
non-destructive migration story.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task F2: Update `CLAUDE.md` status line and trust posture

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Read the current Project section**

```bash
sed -n '1,15p' CLAUDE.md
```

- [ ] **Step 2: Update the status line**

In `CLAUDE.md`, find the line that reads `**M4 in review** (polling discipline implemented...)` and replace with the M5-in-progress equivalent. Also extend the trust-posture paragraph to mention M5.

Replace:

```
... (M4 in review (polling discipline implemented, awaiting merge — spec at `docs/specs/2026-05-09-m4-polling-discipline.md`; M3 media proxy merged ...
```

with:

```
... (M5 in progress (article extraction landing — spec at `docs/specs/2026-05-09-m5-article-extraction.md`; M4 polling discipline merged — spec at `docs/specs/2026-05-09-m4-polling-discipline.md`; M3 media proxy merged ...
```

In the trust-posture block, add after the existing M4 paragraph:

```
Subscriptions opted into M5 article extraction (`extract = true`)
fetch each new entry's article URL via the shared client (SSRF, per-
host cap, `--http-timeout` all apply). Readability mode by default;
per-feed `extract_selector` CSS override available via PATCH
/api/v1/subscriptions/:id (validated through `cascadia.Compile` at
write time). Extracted HTML flows through `processor.Process` so M2
sanitisation and M3 image proxying still apply. Per-entry failures
set `entries.extract_failed = 1` and degrade to the feed-provided
summary; they never abort the poll. Bounded by
`--extract-concurrency` (default 4) inside each feed worker.
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "$(cat <<'EOF'
M5: CLAUDE.md status line + trust-posture block

Status flips to M5 in progress with a forward link to the spec.
Trust posture grows the article-extraction summary so a future
session has the contracts (cascadia at PATCH time, processor.Process
on output, per-entry isolation) at hand.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task F3: Link the M5 spec from the roadmap

**Files:**
- Modify: `docs/roadmap.md`

- [ ] **Step 1: Locate the M5 row**

Currently:

```markdown
| M5 | Article extraction | Readability-style extractor + per-feed CSS rules, opt-in flag, graceful degradation. | TBD |
```

- [ ] **Step 2: Replace `TBD` with the spec link**

```markdown
| M5 | Article extraction | Readability-style extractor + per-feed CSS rules, opt-in flag, graceful degradation. | [`specs/2026-05-09-m5-article-extraction.md`](specs/2026-05-09-m5-article-extraction.md) |
```

- [ ] **Step 3: Commit**

```bash
git add docs/roadmap.md
git commit -m "$(cat <<'EOF'
M5: link spec from roadmap

Drops TBD on the M5 row.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Final verification

After all tasks land:

- [ ] **Run the full test suite under the race detector:**

```bash
make test
```

Expected: PASS, no race warnings, no skips.

- [ ] **Build the static binary:**

```bash
make build
```

Expected: `bin/tap` produced; no CGO; no missing modules.

- [ ] **Smoke-test the binary against a fresh data dir:**

```bash
rm -rf /tmp/tap-m5-smoke && ./bin/tap --data /tmp/tap-m5-smoke --addr 127.0.0.1:18080 --ssrf-allow 127.0.0.1/32 &
SMOKE_PID=$!
sleep 1
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"feed_url":"http://127.0.0.1:18080/healthz","extract":true}' \
  http://127.0.0.1:18080/api/v1/subscriptions || true
curl -s -X PATCH -H 'Content-Type: application/json' \
  -d '{"extract_selector":"[unclosed"}' \
  http://127.0.0.1:18080/api/v1/subscriptions/1
echo
kill $SMOKE_PID
```

Expected: PATCH returns a 400 with `extract_selector_invalid`. The first POST is opportunistic — the goal is only to prove the binary boots, accepts subscriptions with `extract=true`, and surfaces the new error code.

- [ ] **Verify the spec's Definition of Done:**

Walk each numbered DoD item in `docs/specs/2026-05-09-m5-article-extraction.md` §"Definition of done" and confirm there's a corresponding test or behaviour in the merged code. If any item lacks coverage, add the missing test before declaring the milestone done.

---

## Plan-to-spec coverage map

| Spec section | Task(s) |
|---|---|
| New package `internal/extract` | B1, B2, B3, B4 |
| Schema migration 0004 | A1 |
| db layer extensions | A2, A3, A4 (also touched in D1, D4 for entry/subscription DTOs) |
| Worker integration | C1, C2, C3, C4 |
| API: POST gains `extract` | D1 |
| API: PATCH endpoint | D2, D3 |
| API: subscription DTO grows fields | D1 |
| API: entries DTO grows `extract_failed` | D4 |
| Configuration knobs | E1 |
| `cmd/tap/main.go` wire-up | E1 |
| README update | F1 |
| CLAUDE.md status | F2 |
| Roadmap link | F3 |
| End-to-end test, success path (DoD #2) | E2 |
| End-to-end test, failure path (DoD #4) | E3 |
| Tests and methodology | every Phase A–C task is TDD-first |

---

## Notes for the implementer

- **Don't broaden PATCH.** The spec is explicit (§Risks): title, category, per-feed HTTP overrides each get added in their own milestones (M9, M9, M6 respectively). If the temptation appears mid-task, push back.
- **Don't add bundled hostname rules.** Spec §"Out of scope" rules this out; per-feed override is the policy.
- **Don't backfill old entries on extract toggle on.** Existing entries keep their stored content; extraction only touches future polls.
- **Don't add a Poke-on-PATCH.** Spec §"API extensions" explicitly says PATCH does not call `Scheduler.Poke()` — toggling extract on doesn't change anything until the next normal poll's new entries.
- **Match existing test style.** The repo uses `require.NoError(t, err)`, `require.Equal(t, want, got)`, `t.Parallel()`, `t.Cleanup(...)`. Don't introduce assert/Suite/Convey.
- **Keep `go test ./... -race` clean.** The errgroup goroutines write to distinct slice indices; if you ever change to a shared map or counter, add a mutex.
