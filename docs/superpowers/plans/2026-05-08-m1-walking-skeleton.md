# M1 Walking Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the thinnest possible end-to-end Tap that proves the architecture — one static Go binary with embedded Svelte SPA, embedded SQLite as both source-of-truth and work queue, in-process polling loop, basic REST API.

**Architecture:** A single Go service exposes a REST API and the embedded SPA on one HTTP port. The polling scheduler runs in-process, dispatching feed fetches to a small worker pool. The database itself is the queue — `subscriptions.next_poll_at` is the schedule. Each poll commits in one transaction. Production builds as a distroless container; dev mode proxies SPA requests to Vite for hot reload.

**Tech Stack:**
- Backend: Go 1.24+, `net/http` stdlib, `log/slog`, `modernc.org/sqlite` (pure-Go driver), `github.com/mmcdole/gofeed`, `github.com/stretchr/testify`.
- Frontend: Svelte 5 + TypeScript + Vite, plain Svelte (no SvelteKit), pnpm.
- Build: GNU Make, multi-stage Dockerfile based on `gcr.io/distroless/static-debian12:nonroot`.
- Module path: `github.com/bcrisp4/tap`.

**Spec:** [`docs/specs/2026-05-08-m1-walking-skeleton.md`](../../specs/2026-05-08-m1-walking-skeleton.md). The roadmap is at [`docs/roadmap.md`](../../roadmap.md). The full product concept lives in [`docs/concept.md`](../../concept.md). The visual design is in [`ui_design/`](../../../ui_design/).

---

## Skills to invoke

Use the right skill for the right kind of work. Don't skip — these prevent re-deriving things from scratch.

| Phase / task type | Skills to invoke |
|---|---|
| All Go tasks | `golang-naming`, `golang-code-style`, `golang-modernize`, `golang-error-handling` |
| All Svelte tasks | `svelte-runes`, `svelte-styling`, `svelte-components` |
| Repo / module setup (Phase 1) | `golang-project-layout` |
| Database tasks (Phase 2–3) | `golang-database` |
| Polling / concurrency (Phase 4) | `golang-concurrency`, `golang-safety` (panic recovery in workers) |
| HTTP server (Phase 5–6) | `golang-design-patterns` (graceful shutdown), `golang-context` |
| Tests (any phase) | `golang-testing`, `golang-stretchr-testify` |
| Code with logic | `superpowers:test-driven-development` (red-green-refactor) |
| Before claiming any task done | `superpowers:verification-before-completion` |
| Library API questions | Context7 MCP — fetch current docs for `gofeed`, `modernc.org/sqlite`, `slog`, Svelte runes etc. before guessing |
| UI smoke verification | Playwright MCP — navigate to localhost, snapshot, assert |
| Svelte template needs `{@html}` or `{@attach}` | `svelte-template-directives` |

---

## TDD policy

**M1 is built test-first. The discipline is non-negotiable.**

Every task in this plan that creates or modifies behaviour-bearing code follows the red–green–refactor cycle. Steps in each task are explicitly tagged so the cycle is visible:

- **(RED)** — write the failing test, then run it and confirm it fails for the *expected* reason (compile error, assertion mismatch, wrong return value). A test that doesn't fail in red is a test that doesn't exercise real code. Two RED steps in this plan: write the test, then run it and observe the failure.
- **(GREEN)** — write the *minimum* implementation that turns the test green. No speculative features, no extra branches "while you're in there", no error paths the test doesn't cover. Two GREEN steps: write the minimal implementation, then run it and observe the pass.
- **(REFACTOR)** — clean up while green. Run the test after every refactor. Most M1 tasks are simple enough that there's nothing to refactor; when there is, the task calls it out.
- **Commit** — once green and clean, commit. The diff for the commit should include both the test and the implementation.

If you find yourself writing implementation before a test, **stop**. Delete the implementation, write the test first, watch it fail, then re-add the implementation. The point of the discipline is that the test is a *reproducer* — if the implementation regresses in a future milestone, the test catches it. A test written after the fact, while convenient, gives no such guarantee, because you've never seen it fail.

**Exempt from TDD** (these tasks have no behaviour to test):

- Project init (Phase 1.1, 1.2)
- Vite / Svelte / TypeScript config files (Phase 1.3)
- The Vite-build bootstrap of `web/dist/` (Phase 1.4)
- The README (Phase 1.5)
- Design tokens, base styles, fonts (Phase 8.2, 8.3)
- Individual Svelte component templates (Phase 10) — visual verification happens manually in Phase 13
- The Reader and Unread view templates (Phase 11) — same
- Makefile, Dockerfile, .dockerignore (Phase 12)

**Everything else is in scope.** If a task touches conditional logic, branches, error paths, loops, or stateful behaviour and is *not* labelled with RED/GREEN steps below, that's a bug in this plan — flag it before implementing.

Use the `superpowers:test-driven-development` skill on every implementation task. Use `superpowers:verification-before-completion` before claiming any GREEN or commit step done.

---

## File structure

Backend:

```
cmd/tap/main.go                    entry point, flag parsing, lifecycle wiring
internal/
  db/
    db.go                          SQLite connection + PRAGMAs
    migrate.go                     migrations runner
    migrations/0001_initial.sql    initial schema
    subscriptions.go               subscription queries
    entries.go                     entry queries
  feed/
    hash.go                        entry hash (GUID → URL → title+date)
    parse.go                       gofeed wrapper
  poll/
    inflight.go                    in-flight tracker (mutex + map)
    worker.go                      per-feed worker
    scheduler.go                   ticker + dispatcher
  api/
    api.go                         mux + handlers
    errors.go                      error response shape
  server/
    server.go                      HTTP server lifecycle
    spa.go                         embed.FS + dev-mode Vite proxy
```

Frontend:

```
web/
  embed.go                         exports embed.FS for Go to consume
  package.json
  pnpm-lock.yaml                   generated
  tsconfig.json
  vite.config.ts
  svelte.config.js
  index.html
  src/
    main.ts
    App.svelte                     root + routing
    lib/
      api.ts                       typed fetch wrapper
      router.ts                    tiny path matcher
      types.ts                     shared types matching API
      store.ts                     entries/subscriptions state
      colors.ts                    deterministic per-feed color
    styles/
      tokens.css                   light-theme design tokens
      global.css                   base typography + resets
    components/
      JunctionDot.svelte
      FeedAvatar.svelte
      AddFeedForm.svelte
      TopBar.svelte
      Sidebar.svelte
      EntryRow.svelte
    views/
      Unread.svelte
      Reader.svelte
  dist/                            Vite build output (embedded)
```

Build / deploy:

```
Makefile
Dockerfile
.dockerignore
README.md
```

---

## Phase 1 — Repository scaffold

### Task 1.1: Initialize Go module

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Init module**

```bash
go mod init github.com/bcrisp4/tap
go mod edit -go=1.24
```

- [ ] **Step 2: Verify**

```bash
cat go.mod
```

Expected: contains `module github.com/bcrisp4/tap` and `go 1.24`.

- [ ] **Step 3: Commit**

```bash
git add go.mod
git commit -m "Initialize Go module"
```

---

### Task 1.2: Extend .gitignore

**Files:**
- Modify: `.gitignore` (currently contains only `docs/superpowers/`)

- [ ] **Step 1: Append entries**

Append to `.gitignore`:

```
# Build output
bin/
web/dist/*
!web/dist/.gitkeep
web/node_modules/

# Local data
data/
*.db
*.db-shm
*.db-wal

# OS
.DS_Store
```

- [ ] **Step 2: Verify**

```bash
git check-ignore -v bin/tap web/dist/index.html web/node_modules/foo data/tap.db
```

Expected: each line shows the ignore rule that matched.

- [ ] **Step 3: Commit**

```bash
git add .gitignore
git commit -m "Extend .gitignore for build output and local data"
```

---

### Task 1.3: Initialize the Svelte/Vite project (manual setup, no scaffold tool)

**Files:**
- Create: `web/package.json`, `web/svelte.config.js`, `web/vite.config.ts`, `web/tsconfig.json`, `web/tsconfig.node.json`, `web/index.html`, `web/src/main.ts`, `web/src/App.svelte`

- [ ] **Step 1: Create `web/package.json`**

```json
{
  "name": "tap-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "check": "svelte-check --tsconfig ./tsconfig.json"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "^5.0.0",
    "@tsconfig/svelte": "^5.0.0",
    "svelte": "^5.0.0",
    "svelte-check": "^4.0.0",
    "tslib": "^2.6.0",
    "typescript": "^5.5.0",
    "vite": "^6.0.0"
  }
}
```

- [ ] **Step 2: Create `web/svelte.config.js`**

```js
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default {
  preprocess: vitePreprocess(),
};
```

- [ ] **Step 3: Create `web/vite.config.ts`**

In dev, Vite serves the SPA on `:5173` and proxies API + healthz calls to the Go server on `:8080`. The user visits `localhost:5173` in dev. In production, Go serves everything from the embedded bundle on `:8080`.

```ts
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const GO_BACKEND = 'http://127.0.0.1:8080';

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api':     { target: GO_BACKEND, changeOrigin: false },
      '/healthz': { target: GO_BACKEND, changeOrigin: false },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
```

- [ ] **Step 4: Create `web/tsconfig.json`**

```json
{
  "extends": "@tsconfig/svelte/tsconfig.json",
  "compilerOptions": {
    "target": "ESNext",
    "useDefineForClassFields": true,
    "module": "ESNext",
    "resolveJsonModule": true,
    "allowJs": true,
    "checkJs": true,
    "isolatedModules": true,
    "moduleResolution": "bundler",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true
  },
  "include": ["src/**/*.ts", "src/**/*.svelte"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 5: Create `web/tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 6: Create `web/index.html`**

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>tap</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 7: Create `web/src/main.ts` (placeholder, real version comes in Phase 8)**

```ts
import { mount } from 'svelte';
import App from './App.svelte';

const app = mount(App, { target: document.getElementById('app')! });

export default app;
```

- [ ] **Step 8: Create `web/src/App.svelte` (placeholder)**

```svelte
<main>
  <h1>tap. boot ok.</h1>
</main>
```

- [ ] **Step 9: Install deps**

```bash
cd web && pnpm install && cd ..
```

Expected: `web/node_modules/` and `web/pnpm-lock.yaml` created. No errors.

- [ ] **Step 10: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml web/svelte.config.js web/vite.config.ts web/tsconfig.json web/tsconfig.node.json web/index.html web/src/main.ts web/src/App.svelte
git commit -m "Initialize Svelte 5 + Vite + TypeScript project"
```

---

### Task 1.4: Bootstrap `web/dist/` and add `web/embed.go` so Go can consume the SPA bundle

The Go binary embeds the SPA via a `//go:embed all:dist` directive. Two things must exist for that to work: the `web/dist/` directory itself (or `go build` errors out), and a Go file in the `web/` package that holds the directive. Embed paths are interpreted relative to the package directory containing the source file — they cannot escape it — so the directive must live in a Go file *inside* `web/`, not in `internal/server/`.

**Files:**
- Create: `web/embed.go`, `web/dist/.gitkeep`

- [ ] **Step 1: Run an initial Vite build**

```bash
pnpm --dir web build
```

Expected: `web/dist/index.html` and a small JS bundle exist.

- [ ] **Step 2: Add `.gitkeep` so the directory survives even when `dist/*` is gitignored**

```bash
touch web/dist/.gitkeep
```

- [ ] **Step 3: Create `web/embed.go`**

```go
// Package web exposes the built SPA bundle to the Go server as an embedded
// filesystem, so the Tap binary needs no external static assets at runtime.
//
// This file lives in web/ rather than internal/server/ because go:embed paths
// cannot escape the package directory.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
```

- [ ] **Step 4: Verify it compiles**

```bash
go build ./web/...
```

Expected: succeeds with no output.

- [ ] **Step 5: Verify gitignore behaviour**

```bash
git status --porcelain web/
```

Expected: `web/embed.go`, `web/dist/.gitkeep`, and the existing Vite project files show as untracked. The actual `web/dist/*` build artefacts are gitignored.

- [ ] **Step 6: Commit**

```bash
git add web/embed.go web/dist/.gitkeep
git commit -m "Add web/embed.go and web/dist/.gitkeep so go:embed can compile"
```

---

### Task 1.5: Add minimal README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README**

```markdown
# tap

Self-hosted RSS / Atom / JSON Feed reader. Single binary, embedded SQLite,
embedded SPA, no external dependencies.

- [Concept](docs/concept.md)
- [Roadmap](docs/roadmap.md)
- [UI design references](ui_design/)

## Status

Pre-M1 — walking-skeleton implementation in progress.
See [`docs/specs/`](docs/specs/) for milestone specs.

## Development

Requires Go 1.24+, pnpm, Make.

```bash
make dev      # run Go on :8080 and Vite on :5173 — open http://localhost:5173
make build    # build single static binary at bin/tap
make docker   # build distroless container image
```

## M1 deployment safety

In M1 the server renders feed HTML **without sanitisation** — that lands in M2.
The binary defaults to `-addr 127.0.0.1:8080`, which contains the risk to the
local machine. The container variant binds `0.0.0.0:8080` because Docker port
mapping requires it.

**Do not reverse-proxy the M1 container to anywhere a hostile-feed author can
reach** — Tailscale, LAN, the public internet. A malicious feed can plant
stored XSS in your reader otherwise. M2 closes this gap.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "Add README"
```

---

## Phase 2 — Database foundation

### Task 2.1: Add database dependencies

- [ ] **Step 1: Pull modernc.org/sqlite (pure-Go driver) and testify**

```bash
go get modernc.org/sqlite@latest
go get github.com/stretchr/testify@latest
```

- [ ] **Step 2: Verify**

```bash
go mod tidy
grep modernc.org/sqlite go.mod
grep stretchr/testify go.mod
```

Expected: both modules listed in `go.mod`.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "Add modernc.org/sqlite and testify deps"
```

---

### Task 2.2: SQLite connection helper (TDD)

**Files:**
- Create: `internal/db/db.go`, `internal/db/db_test.go`

- [ ] **Step 1 (RED): Write the failing test**

`internal/db/db_test.go`:

```go
package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpen_InMemory_AppliesPragmas(t *testing.T) {
	t.Parallel()

	d, err := Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, d.PingContext(context.Background()))

	var fk int
	require.NoError(t, d.QueryRow("PRAGMA foreign_keys").Scan(&fk))
	require.Equal(t, 1, fk, "foreign_keys must be ON")

	var jm string
	require.NoError(t, d.QueryRow("PRAGMA journal_mode").Scan(&jm))
	// in-memory always reports "memory"; for a real file it would be "wal".
	require.Contains(t, []string{"wal", "memory"}, jm)
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

```bash
go test ./internal/db/... -run TestOpen -v
```

Expected: build error (`Open` undefined).

- [ ] **Step 3 (GREEN): Write the minimum implementation of `Open` to make the test pass**

`internal/db/db.go`:

```go
// Package db owns SQLite connection setup, migrations, and per-table query helpers.
package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

// Open opens (or creates) a SQLite database at path and applies the PRAGMAs
// every Tap process expects. Pass ":memory:" for an in-memory DB.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)

	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := d.PingContext(ctx); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return d, nil
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/db/... -run TestOpen -v
```

Expected: `--- PASS: TestOpen_InMemory_AppliesPragmas`.

- [ ] **Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "Add SQLite connection helper with required PRAGMAs"
```

---

### Task 2.3: Initial schema migration

**Files:**
- Create: `internal/db/migrations/0001_initial.sql`

- [ ] **Step 1: Write the schema**

`internal/db/migrations/0001_initial.sql`:

```sql
CREATE TABLE subscriptions (
    id            INTEGER PRIMARY KEY,
    title         TEXT NOT NULL,
    feed_url      TEXT NOT NULL UNIQUE,
    site_url      TEXT,
    last_poll_at  INTEGER,
    next_poll_at  INTEGER NOT NULL,
    etag          TEXT,
    last_modified TEXT,
    error_count   INTEGER NOT NULL DEFAULT 0,
    last_error    TEXT,
    created_at    INTEGER NOT NULL
);
CREATE INDEX idx_subscriptions_next_poll ON subscriptions(next_poll_at);

CREATE TABLE entries (
    id              INTEGER PRIMARY KEY,
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    hash            TEXT NOT NULL,
    title           TEXT NOT NULL,
    author          TEXT,
    url             TEXT NOT NULL,
    content         TEXT NOT NULL,
    published_at    INTEGER NOT NULL,
    fetched_at      INTEGER NOT NULL,
    read            INTEGER NOT NULL DEFAULT 0,
    saved           INTEGER NOT NULL DEFAULT 0,
    UNIQUE (subscription_id, hash)
);
CREATE INDEX idx_entries_published    ON entries(published_at DESC);
CREATE INDEX idx_entries_subscription ON entries(subscription_id, published_at DESC);
CREATE INDEX idx_entries_unread       ON entries(read, published_at DESC);
```

- [ ] **Step 2: Commit (paired with the runner in 2.4)**

(no separate commit — staged together with 2.4)

---

### Task 2.4: Migrations runner (TDD)

**Files:**
- Create: `internal/db/migrate.go`, `internal/db/migrate_test.go`

- [ ] **Step 1 (RED): Write the failing test**

`internal/db/migrate_test.go`:

```go
package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate_AppliesAllMigrationsExactlyOnce(t *testing.T) {
	t.Parallel()

	d, err := Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, Migrate(context.Background(), d))

	// schema_migrations should have version 1 recorded.
	var version int
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 1, version)

	// subscriptions table should exist (introduced in 0001).
	_, err = d.Exec("INSERT INTO subscriptions (title, feed_url, next_poll_at, created_at) VALUES (?, ?, ?, ?)",
		"x", "https://example.com/feed", 0, 0)
	require.NoError(t, err)

	// Re-running Migrate must be a no-op.
	require.NoError(t, Migrate(context.Background(), d))
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 1, version)
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

```bash
go test ./internal/db/... -run TestMigrate -v
```

Expected: build error (`Migrate` undefined).

- [ ] **Step 3 (GREEN): Write the minimum implementation of `Migrate` to make the test pass**

`internal/db/migrate.go`:

```go
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies any pending SQL migrations in lexical order. Idempotent.
func Migrate(ctx context.Context, d *sql.DB) error {
	if _, err := d.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	files, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	var applied int
	if err := d.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&applied); err != nil {
		return fmt.Errorf("query applied version: %w", err)
	}

	for _, name := range files {
		v, err := versionFromFilename(name)
		if err != nil {
			return err
		}
		if v <= applied {
			continue
		}
		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", v, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}
	}
	return nil
}

func versionFromFilename(name string) (int, error) {
	base := strings.TrimPrefix(name, "migrations/")
	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		return 0, fmt.Errorf("migration %q: cannot parse version prefix", name)
	}
	v, err := strconv.Atoi(base[:idx])
	if err != nil {
		return 0, fmt.Errorf("migration %q: version not numeric: %w", name, err)
	}
	return v, nil
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/db/... -run TestMigrate -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrate.go internal/db/migrate_test.go internal/db/migrations/0001_initial.sql
git commit -m "Add migrations runner and initial schema"
```

---

### Task 2.5: Subscription queries

**Files:**
- Create: `internal/db/subscriptions.go`, `internal/db/subscriptions_test.go`

- [ ] **Step 1 (RED): Write the failing tests**

`internal/db/subscriptions_test.go`:

```go
package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, Migrate(context.Background(), d))
	return d
}

func TestSubscription_InsertAndGet(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	now := time.Now().Unix()
	id, err := InsertSubscription(ctx, d, NewSubscription{
		Title:    "Example",
		FeedURL:  "https://example.com/feed",
		SiteURL:  "https://example.com",
		NextPoll: 0,
		Created:  now,
	})
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	got, err := GetSubscription(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, "Example", got.Title)
	require.Equal(t, "https://example.com/feed", got.FeedURL)
	require.Equal(t, int64(0), got.NextPollAt)
}

func TestSubscription_Insert_RejectsDuplicateURL(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	_, err := InsertSubscription(ctx, d, NewSubscription{Title: "a", FeedURL: "https://example.com/x", NextPoll: 0, Created: 0})
	require.NoError(t, err)
	_, err = InsertSubscription(ctx, d, NewSubscription{Title: "b", FeedURL: "https://example.com/x", NextPoll: 0, Created: 0})
	require.Error(t, err)
}

func TestSubscription_ListDuePolls(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	due, _ := InsertSubscription(ctx, d, NewSubscription{Title: "due", FeedURL: "https://example.com/a", NextPoll: 0, Created: 0})
	_, _ = InsertSubscription(ctx, d, NewSubscription{Title: "future", FeedURL: "https://example.com/b", NextPoll: time.Now().Unix() + 86400, Created: 0})

	rows, err := ListDuePolls(ctx, d, time.Now().Unix(), 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, due, rows[0].ID)
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

```bash
go test ./internal/db/... -run TestSubscription -v
```

Expected: build error.

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/db/subscriptions.go`:

```go
package db

import (
	"context"
	"database/sql"
	"fmt"
)

type Subscription struct {
	ID           int64
	Title        string
	FeedURL      string
	SiteURL      sql.NullString
	LastPollAt   sql.NullInt64
	NextPollAt   int64
	ETag         sql.NullString
	LastModified sql.NullString
	ErrorCount   int
	LastError    sql.NullString
	CreatedAt    int64
}

type NewSubscription struct {
	Title    string
	FeedURL  string
	SiteURL  string
	NextPoll int64
	Created  int64
}

type DueSubscription struct {
	ID           int64
	FeedURL      string
	ETag         sql.NullString
	LastModified sql.NullString
}

// PollResult and UpdateAfterPoll live in entries.go because they reference
// db.NewEntry, which is defined there. Keeping them together avoids a forward
// reference and lets Task 2.5 commit cleanly without depending on Task 2.6.

func InsertSubscription(ctx context.Context, d *sql.DB, s NewSubscription) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO subscriptions (title, feed_url, site_url, next_poll_at, created_at)
		VALUES (?, ?, NULLIF(?, ''), ?, ?)
	`, s.Title, s.FeedURL, s.SiteURL, s.NextPoll, s.Created)
	if err != nil {
		return 0, fmt.Errorf("insert subscription: %w", err)
	}
	return res.LastInsertId()
}

func GetSubscription(ctx context.Context, d *sql.DB, id int64) (Subscription, error) {
	var s Subscription
	err := d.QueryRowContext(ctx, `
		SELECT id, title, feed_url, site_url, last_poll_at, next_poll_at,
		       etag, last_modified, error_count, last_error, created_at
		FROM subscriptions WHERE id = ?
	`, id).Scan(&s.ID, &s.Title, &s.FeedURL, &s.SiteURL, &s.LastPollAt, &s.NextPollAt,
		&s.ETag, &s.LastModified, &s.ErrorCount, &s.LastError, &s.CreatedAt)
	if err != nil {
		return Subscription{}, fmt.Errorf("get subscription %d: %w", id, err)
	}
	return s, nil
}

func ListSubscriptions(ctx context.Context, d *sql.DB) ([]Subscription, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, title, feed_url, site_url, last_poll_at, next_poll_at,
		       etag, last_modified, error_count, last_error, created_at
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
			&s.ETag, &s.LastModified, &s.ErrorCount, &s.LastError, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func DeleteSubscription(ctx context.Context, d *sql.DB, id int64) error {
	_, err := d.ExecContext(ctx, "DELETE FROM subscriptions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete subscription %d: %w", id, err)
	}
	return nil
}

// ListDuePolls returns subscriptions whose next_poll_at is <= now, capped at limit.
// The caller is responsible for excluding currently in-flight subscriptions.
func ListDuePolls(ctx context.Context, d *sql.DB, now int64, limit int) ([]DueSubscription, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, feed_url, etag, last_modified
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
		if err := rows.Scan(&s.ID, &s.FeedURL, &s.ETag, &s.LastModified); err != nil {
			return nil, fmt.Errorf("scan due poll: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateAfterError records a poll failure and pushes next_poll_at out.
func UpdateAfterError(ctx context.Context, d *sql.DB, subID int64, errMsg string, nextPollAt int64) error {
	_, err := d.ExecContext(ctx, `
		UPDATE subscriptions
		SET error_count = error_count + 1,
		    last_error  = ?,
		    next_poll_at = ?
		WHERE id = ?
	`, errMsg, nextPollAt, subID)
	if err != nil {
		return fmt.Errorf("record poll error: %w", err)
	}
	return nil
}

// UpdateAfterNotModified bumps timestamps without inserting anything (304 path).
func UpdateAfterNotModified(ctx context.Context, d *sql.DB, subID int64, nowUnix, nextPollAt int64) error {
	_, err := d.ExecContext(ctx, `
		UPDATE subscriptions
		SET last_poll_at = ?, next_poll_at = ?, error_count = 0, last_error = NULL
		WHERE id = ?
	`, nowUnix, nextPollAt, subID)
	if err != nil {
		return fmt.Errorf("record 304: %w", err)
	}
	return nil
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/db/... -run TestSubscription -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/subscriptions.go internal/db/subscriptions_test.go
git commit -m "Add subscription queries"
```

---

### Task 2.6: Entry queries

**Files:**
- Create: `internal/db/entries.go`, `internal/db/entries_test.go`

- [ ] **Step 1 (RED): Write the failing tests**

`internal/db/entries_test.go`:

```go
package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func seedSub(t *testing.T, d *sql.DB) int64 {
	t.Helper()
	id, err := InsertSubscription(context.Background(), d, NewSubscription{
		Title: "x", FeedURL: "https://example.com/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	return id
}

func TestEntry_ListWithFilters(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID := seedSub(t, d)

	_, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix: 100, NextPollAt: 200, NewEntries: []NewEntry{
			{Hash: "h1", Title: "A", URL: "https://e.com/a", Content: "<p>a</p>", PublishedAt: 50},
			{Hash: "h2", Title: "B", URL: "https://e.com/b", Content: "<p>b</p>", PublishedAt: 60},
		},
	})
	require.NoError(t, err)

	got, _, _, err := ListEntries(ctx, d, ListEntriesParams{Limit: 100})
	require.NoError(t, err)
	require.Len(t, got, 2)

	// PATCH first entry as read.
	require.NoError(t, UpdateEntry(ctx, d, got[0].ID, EntryUpdate{Read: ptrBool(true)}))

	unread, _, _, err := ListEntries(ctx, d, ListEntriesParams{UnreadOnly: true, Limit: 100})
	require.NoError(t, err)
	require.Len(t, unread, 1)
}

func TestEntry_CompositeCursorPaginationDoesNotDropEntries(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	subID := seedSub(t, d)

	// Two entries where ID order disagrees with published_at order.
	// (pub=200, id=lowest) and (pub=100, id=highest). A bare `id < cursor` would
	// drop the second one.
	_, err := UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix: 1000, NextPollAt: 2000, NewEntries: []NewEntry{
			{Hash: "h1", Title: "newer-low-id",  URL: "https://e.com/1", Content: "x", PublishedAt: 200},
		},
	})
	require.NoError(t, err)
	_, err = UpdateAfterPoll(ctx, d, subID, PollResult{
		NowUnix: 1000, NextPollAt: 2000, NewEntries: []NewEntry{
			{Hash: "h2", Title: "older-high-id", URL: "https://e.com/2", Content: "x", PublishedAt: 100},
		},
	})
	require.NoError(t, err)

	page1, nextPub, nextID, err := ListEntries(ctx, d, ListEntriesParams{Limit: 1})
	require.NoError(t, err)
	require.Len(t, page1, 1)
	require.Equal(t, "newer-low-id", page1[0].Title)
	require.Greater(t, nextPub, int64(0), "should have a next cursor")

	page2, _, _, err := ListEntries(ctx, d, ListEntriesParams{
		Limit:             1,
		CursorPublishedAt: nextPub,
		CursorID:          nextID,
	})
	require.NoError(t, err)
	require.Len(t, page2, 1, "older entry must NOT be silently dropped")
	require.Equal(t, "older-high-id", page2[0].Title)
}

func ptrBool(b bool) *bool { return &b }
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

```bash
go test ./internal/db/... -run TestEntry -v
```

Expected: build error.

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/db/entries.go`:

```go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type NewEntry struct {
	Hash        string
	Title       string
	Author      string
	URL         string
	Content     string
	PublishedAt int64
}

// PollResult is the success result of a feed fetch+parse worth committing.
// Lives here (not subscriptions.go) so it can reference NewEntry directly.
type PollResult struct {
	NewETag         sql.NullString
	NewLastModified sql.NullString
	NextPollAt      int64
	NowUnix         int64
	NewEntries      []NewEntry
}

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
}

type ListEntriesParams struct {
	UnreadOnly        bool
	SubscriptionID    int64 // 0 means all
	Limit             int
	CursorPublishedAt int64 // 0 means no cursor (paired with CursorID)
	CursorID          int64 // 0 means no cursor (paired with CursorPublishedAt)
}

type EntryUpdate struct {
	Read  *bool
	Saved *bool
}

// ListEntries returns entries newest-first. Bodies are NOT included to keep payloads small.
// nextPub and nextID together form the cursor for the next page (both zero if no more).
//
// Why a composite (published_at, id) cursor and not just id: backfills, re-imports,
// and clock skew can produce entries whose ID order disagrees with their published_at
// order. A bare `id < cursor` would silently drop entries whose IDs are higher than
// the cursor but whose published_at is lower. The row-value comparison `(published_at, id)
// < (?, ?)` paired with `ORDER BY published_at DESC, id DESC` is correct in all cases.
// SQLite supports row-value comparisons since 3.15.0.
func ListEntries(ctx context.Context, d *sql.DB, p ListEntriesParams) (entries []Entry, nextPub, nextID int64, err error) {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}

	var (
		clauses []string
		args    []any
	)
	if p.UnreadOnly {
		clauses = append(clauses, "read = 0")
	}
	if p.SubscriptionID > 0 {
		clauses = append(clauses, "subscription_id = ?")
		args = append(args, p.SubscriptionID)
	}
	if p.CursorPublishedAt > 0 {
		clauses = append(clauses, "(published_at, id) < (?, ?)")
		args = append(args, p.CursorPublishedAt, p.CursorID)
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}

	q := fmt.Sprintf(`
		SELECT id, subscription_id, hash, title, author, url, '' AS content,
		       published_at, fetched_at, read, saved
		FROM entries %s
		ORDER BY published_at DESC, id DESC
		LIMIT ?
	`, where)
	args = append(args, p.Limit+1)

	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.SubscriptionID, &e.Hash, &e.Title, &e.Author,
			&e.URL, &e.Content, &e.PublishedAt, &e.FetchedAt, &e.Read, &e.Saved); err != nil {
			return nil, 0, 0, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}

	if len(entries) > p.Limit {
		last := entries[p.Limit-1] // the last entry returned on this page
		nextPub = last.PublishedAt
		nextID = last.ID
		entries = entries[:p.Limit]
	}
	return entries, nextPub, nextID, nil
}

func GetEntry(ctx context.Context, d *sql.DB, id int64) (Entry, error) {
	var e Entry
	err := d.QueryRowContext(ctx, `
		SELECT id, subscription_id, hash, title, author, url, content,
		       published_at, fetched_at, read, saved
		FROM entries WHERE id = ?
	`, id).Scan(&e.ID, &e.SubscriptionID, &e.Hash, &e.Title, &e.Author,
		&e.URL, &e.Content, &e.PublishedAt, &e.FetchedAt, &e.Read, &e.Saved)
	if err != nil {
		return Entry{}, fmt.Errorf("get entry %d: %w", id, err)
	}
	return e, nil
}

func UpdateEntry(ctx context.Context, d *sql.DB, id int64, u EntryUpdate) error {
	var sets []string
	var args []any
	if u.Read != nil {
		sets = append(sets, "read = ?")
		args = append(args, boolToInt(*u.Read))
	}
	if u.Saved != nil {
		sets = append(sets, "saved = ?")
		args = append(args, boolToInt(*u.Saved))
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := d.ExecContext(ctx, fmt.Sprintf("UPDATE entries SET %s WHERE id = ?", strings.Join(sets, ", ")), args...)
	if err != nil {
		return fmt.Errorf("update entry %d: %w", id, err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpdateAfterPoll commits the success result of a poll in one transaction.
// New entries are inserted (duplicates dropped silently); the subscription
// row is updated. Lives here (not subscriptions.go) so it can reference NewEntry.
func UpdateAfterPoll(ctx context.Context, d *sql.DB, subID int64, r PollResult) (insertedCount int, err error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, e := range r.NewEntries {
		res, ierr := tx.ExecContext(ctx, `
			INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at)
			VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)
			ON CONFLICT (subscription_id, hash) DO NOTHING
		`, subID, e.Hash, e.Title, e.Author, e.URL, e.Content, e.PublishedAt, r.NowUnix)
		if ierr != nil {
			err = fmt.Errorf("insert entry: %w", ierr)
			return 0, err
		}
		n, _ := res.RowsAffected()
		insertedCount += int(n)
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE subscriptions
		SET etag          = ?,
		    last_modified = ?,
		    last_poll_at  = ?,
		    next_poll_at  = ?,
		    error_count   = 0,
		    last_error    = NULL
		WHERE id = ?
	`, r.NewETag, r.NewLastModified, r.NowUnix, r.NextPollAt, subID); err != nil {
		err = fmt.Errorf("update subscription: %w", err)
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		err = fmt.Errorf("commit: %w", err)
		return 0, err
	}
	return insertedCount, nil
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/db/... -run TestEntry -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/entries.go internal/db/entries_test.go
git commit -m "Add entry queries with cursor pagination"
```

---

## Phase 3 — Feed parsing

### Task 3.1: Add gofeed dependency

- [ ] **Step 1: Pull gofeed**

```bash
go get github.com/mmcdole/gofeed@latest
```

- [ ] **Step 2: Commit**

```bash
git add go.mod go.sum
git commit -m "Add gofeed dependency"
```

---

### Task 3.2: Entry hash function (TDD)

The hash is computed per the concept doc: feed-provided GUID if present, falling back to entry URL, falling back to SHA-256 of `title || published_at`. Includes the feed identifier so two feeds with overlapping GUIDs do not collide.

**Files:**
- Create: `internal/feed/hash.go`, `internal/feed/hash_test.go`

- [ ] **Step 1 (RED): Write the failing test**

`internal/feed/hash_test.go`:

```go
package feed

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/require"
)

func TestEntryHash_PrefersGUID(t *testing.T) {
	t.Parallel()
	pub := time.Unix(1700000000, 0)
	item := &gofeed.Item{GUID: "stable-guid", Link: "https://example.com/a", Title: "A", PublishedParsed: &pub}
	h := EntryHash(42, item)
	require.NotEmpty(t, h)

	// Same feed + same GUID = same hash, regardless of other fields.
	item.Link = "different"
	item.Title = "different"
	require.Equal(t, h, EntryHash(42, item))

	// Different feed = different hash even with the same GUID.
	require.NotEqual(t, h, EntryHash(99, item))
}

func TestEntryHash_FallsBackToURL(t *testing.T) {
	t.Parallel()
	item := &gofeed.Item{Link: "https://example.com/a", Title: "A"}
	h := EntryHash(1, item)
	require.NotEmpty(t, h)

	item.Title = "different title"
	require.Equal(t, h, EntryHash(1, item), "URL fallback ignores title")
}

func TestEntryHash_FallsBackToTitlePlusDate(t *testing.T) {
	t.Parallel()
	pub := time.Unix(1700000000, 0)
	item := &gofeed.Item{Title: "A", PublishedParsed: &pub}
	h := EntryHash(1, item)
	require.NotEmpty(t, h)

	pub2 := time.Unix(1800000000, 0)
	item2 := &gofeed.Item{Title: "A", PublishedParsed: &pub2}
	require.NotEqual(t, h, EntryHash(1, item2), "different dates produce different hashes")
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

```bash
go test ./internal/feed/... -run TestEntryHash -v
```

Expected: build error (`EntryHash` undefined).

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/feed/hash.go`:

```go
// Package feed wraps the gofeed parser and exposes the entry-hash contract.
package feed

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/mmcdole/gofeed"
)

// EntryHash computes the per-entry deduplication key. The feed identifier
// is mixed in so two feeds with overlapping GUIDs do not collide.
//
// Resolution order: feed-provided GUID, then entry URL, then SHA-256(title || published_at).
func EntryHash(subID int64, item *gofeed.Item) string {
	if item.GUID != "" {
		return digest(fmt.Sprintf("%d|guid|%s", subID, item.GUID))
	}
	if item.Link != "" {
		return digest(fmt.Sprintf("%d|url|%s", subID, item.Link))
	}
	pub := ""
	if item.PublishedParsed != nil {
		pub = item.PublishedParsed.UTC().Format("20060102T150405Z")
	}
	return digest(fmt.Sprintf("%d|td|%s|%s", subID, item.Title, pub))
}

func digest(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/feed/... -run TestEntryHash -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/feed/hash.go internal/feed/hash_test.go
git commit -m "Add entry hash with GUID/URL/title+date fallback chain"
```

---

### Task 3.3: Feed fetch + parse wrapper (TDD)

**Files:**
- Create: `internal/feed/parse.go`, `internal/feed/parse_test.go`

- [ ] **Step 1 (RED): Write the failing test**

`internal/feed/parse_test.go`:

```go
package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const sampleAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Sample</title>
  <link href="https://sample.example/"/>
  <id>urn:sample</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>First post</title>
    <id>urn:sample:1</id>
    <link href="https://sample.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;hello&lt;/p&gt;</content>
  </entry>
</feed>`

func TestFetch_OK(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Header().Set("ETag", `"abc"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleAtom))
	}))
	defer srv.Close()

	res, err := Fetch(context.Background(), http.DefaultClient, srv.URL, FetchOpts{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.Status)
	require.Equal(t, `"abc"`, res.ETag)
	require.NotNil(t, res.Feed)
	require.Len(t, res.Feed.Items, 1)
	require.Equal(t, "First post", res.Feed.Items[0].Title)
}

func TestFetch_NotModified(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, `"prev"`, r.Header.Get("If-None-Match"))
		require.Equal(t, "Wed, 01 May 2026 00:00:00 GMT", r.Header.Get("If-Modified-Since"))
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	res, err := Fetch(context.Background(), http.DefaultClient, srv.URL, FetchOpts{
		PriorETag:         `"prev"`,
		PriorLastModified: "Wed, 01 May 2026 00:00:00 GMT",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNotModified, res.Status)
	require.Nil(t, res.Feed)
}

func TestFetch_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), http.DefaultClient, srv.URL, FetchOpts{})
	require.Error(t, err)
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

```bash
go test ./internal/feed/... -run TestFetch -v
```

Expected: build error.

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/feed/parse.go`:

```go
package feed

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/mmcdole/gofeed"
)

type FetchOpts struct {
	PriorETag         string
	PriorLastModified string
	UserAgent         string
}

type FetchResult struct {
	Status       int
	ETag         string
	LastModified string
	Feed         *gofeed.Feed // nil on 304
}

// Fetch issues a conditional GET against feedURL and parses the response.
// Returns 304 with Feed=nil on Not Modified. Errors include any non-2xx/304 response.
func Fetch(ctx context.Context, client *http.Client, feedURL string, opts FetchOpts) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("new request: %w", err)
	}
	if opts.PriorETag != "" {
		req.Header.Set("If-None-Match", opts.PriorETag)
	}
	if opts.PriorLastModified != "" {
		req.Header.Set("If-Modified-Since", opts.PriorLastModified)
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = "tap/0.1 (+https://github.com/bcrisp4/tap)"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/json, application/xml;q=0.9, */*;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	res := FetchResult{
		Status:       resp.StatusCode,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	switch {
	case resp.StatusCode == http.StatusNotModified:
		return res, nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Cap how much of the response we'll feed to the parser. A hostile
		// or misconfigured origin returning a 1 GB "feed" must not OOM us.
		const maxBody = 10 << 20 // 10 MiB
		f, err := gofeed.NewParser().Parse(io.LimitReader(resp.Body, maxBody))
		if err != nil {
			return FetchResult{}, fmt.Errorf("parse feed: %w", err)
		}
		res.Feed = f
		return res, nil
	default:
		return FetchResult{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/feed/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/feed/parse.go internal/feed/parse_test.go
git commit -m "Add conditional-GET feed fetch + parse wrapper"
```

---

## Phase 4 — Polling pipeline

### Task 4.1: In-flight tracker (TDD)

**Files:**
- Create: `internal/poll/inflight.go`, `internal/poll/inflight_test.go`

- [ ] **Step 1 (RED): Write the failing test**

`internal/poll/inflight_test.go`:

```go
package poll

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInflight_AcquireAndRelease(t *testing.T) {
	t.Parallel()
	tr := NewInflight()

	require.True(t, tr.TryAcquire(1))
	require.False(t, tr.TryAcquire(1), "duplicate acquire should fail")
	require.True(t, tr.TryAcquire(2), "different id should succeed")

	tr.Release(1)
	require.True(t, tr.TryAcquire(1), "release should let it be re-acquired")
}

func TestInflight_Concurrent(t *testing.T) {
	t.Parallel()
	tr := NewInflight()
	var wg sync.WaitGroup
	wins := make(chan int, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tr.TryAcquire(7) {
				wins <- 1
			}
		}()
	}
	wg.Wait()
	close(wins)

	count := 0
	for range wins {
		count++
	}
	require.Equal(t, 1, count, "exactly one goroutine should win the acquire race")
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

```bash
go test ./internal/poll/... -run TestInflight -v
```

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/poll/inflight.go`:

```go
// Package poll owns the feed-polling scheduler and worker pool.
package poll

import "sync"

// Inflight tracks subscription IDs currently being polled. Used to prevent
// the dispatcher from re-dispatching a feed whose worker hasn't completed.
//
// Lives in process memory only — a process restart clears it, which is safe
// because polls are idempotent (the entry-hash uniqueness contract drops dupes).
type Inflight struct {
	mu  sync.Mutex
	set map[int64]struct{}
}

func NewInflight() *Inflight {
	return &Inflight{set: make(map[int64]struct{})}
}

// TryAcquire returns true if id was not already in flight (and now is).
func (i *Inflight) TryAcquire(id int64) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, ok := i.set[id]; ok {
		return false
	}
	i.set[id] = struct{}{}
	return true
}

func (i *Inflight) Release(id int64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.set, id)
}

// IDs returns a snapshot of currently in-flight IDs (for diagnostics).
func (i *Inflight) IDs() []int64 {
	i.mu.Lock()
	defer i.mu.Unlock()
	out := make([]int64, 0, len(i.set))
	for id := range i.set {
		out = append(out, id)
	}
	return out
}
```

- [ ] **Step 4 (GREEN): Run the test with `-race`, confirm it passes**

```bash
go test ./internal/poll/... -run TestInflight -race -v
```

Expected: PASS, no race warnings.

- [ ] **Step 5: Commit**

```bash
git add internal/poll/inflight.go internal/poll/inflight_test.go
git commit -m "Add in-flight tracker for the poll dispatcher"
```

---

### Task 4.2: Worker (TDD)

The worker is the function that handles one feed end-to-end: fetch → parse → commit. It must recover from panics so a malformed feed cannot kill the worker pool.

**Files:**
- Create: `internal/poll/worker.go`, `internal/poll/worker_test.go`

- [ ] **Step 1 (RED): Write the failing tests**

`internal/poll/worker_test.go`:

```go
package poll

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

const sampleAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Sample</title>
  <link href="https://sample.example/"/>
  <id>urn:sample</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>One</title>
    <id>urn:sample:1</id>
    <link href="https://sample.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;a&lt;/p&gt;</content>
  </entry>
</feed>`

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return d
}

func TestWorker_SuccessfulPoll(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc"`)
		_, _ = w.Write([]byte(sampleAtom))
	}))
	defer srv.Close()

	d := newDB(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	w := NewWorker(d, http.DefaultClient, WorkerOpts{Cadence: 30 * time.Minute})
	w.Run(context.Background(), db.DueSubscription{ID: subID, FeedURL: srv.URL})

	got, err := db.GetSubscription(context.Background(), d, subID)
	require.NoError(t, err)
	require.True(t, got.LastPollAt.Valid)
	require.True(t, got.ETag.Valid)
	require.Equal(t, `"abc"`, got.ETag.String)

	entries, _, err := db.ListEntries(context.Background(), d, db.ListEntriesParams{Limit: 100})
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestWorker_ErrorIncrementsCount(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := newDB(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	w := NewWorker(d, http.DefaultClient, WorkerOpts{Cadence: 30 * time.Minute})
	w.Run(context.Background(), db.DueSubscription{ID: subID, FeedURL: srv.URL})

	got, _ := db.GetSubscription(context.Background(), d, subID)
	require.Equal(t, 1, got.ErrorCount)
	require.True(t, got.LastError.Valid)
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

```bash
go test ./internal/poll/... -run TestWorker -v
```

Expected: build error.

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/poll/worker.go`:

```go
package poll

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/feed"
	"github.com/mmcdole/gofeed"
)

type WorkerOpts struct {
	Cadence time.Duration // fixed retry/next-poll interval for M1
}

type Worker struct {
	db     *sql.DB
	client *http.Client
	opts   WorkerOpts
}

func NewWorker(d *sql.DB, c *http.Client, o WorkerOpts) *Worker {
	if o.Cadence <= 0 {
		o.Cadence = 30 * time.Minute
	}
	if c == nil {
		c = http.DefaultClient
	}
	return &Worker{db: d, client: c, opts: o}
}

// Run polls a single subscription end-to-end. Always returns; never panics.
func (w *Worker) Run(ctx context.Context, sub db.DueSubscription) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "worker panic recovered",
				"feed_id", sub.ID, "feed_url", sub.FeedURL,
				"panic", r, "stack", string(debug.Stack()))
		}
	}()

	now := time.Now().Unix()
	nextPoll := time.Now().Add(w.opts.Cadence).Unix()

	res, err := feed.Fetch(ctx, w.client, sub.FeedURL, feed.FetchOpts{
		PriorETag:         sub.ETag.String,
		PriorLastModified: sub.LastModified.String,
	})
	if err != nil {
		slog.WarnContext(ctx, "poll error", "feed_id", sub.ID, "feed_url", sub.FeedURL, "err", err)
		_ = db.UpdateAfterError(ctx, w.db, sub.ID, err.Error(), nextPoll)
		return
	}

	if res.Status == http.StatusNotModified {
		slog.DebugContext(ctx, "poll 304", "feed_id", sub.ID)
		_ = db.UpdateAfterNotModified(ctx, w.db, sub.ID, now, nextPoll)
		return
	}

	newEntries := make([]db.NewEntry, 0, len(res.Feed.Items))
	for _, item := range res.Feed.Items {
		pubAt := now
		if item.PublishedParsed != nil {
			pubAt = item.PublishedParsed.Unix()
		}
		content := item.Content
		if content == "" {
			content = item.Description
		}
		newEntries = append(newEntries, db.NewEntry{
			Hash:        feed.EntryHash(sub.ID, item),
			Title:       item.Title,
			Author:      authorName(item),
			URL:         item.Link,
			Content:     content,
			PublishedAt: pubAt,
		})
	}

	inserted, err := db.UpdateAfterPoll(ctx, w.db, sub.ID, db.PollResult{
		NewETag:         nullStr(res.ETag),
		NewLastModified: nullStr(res.LastModified),
		NextPollAt:      nextPoll,
		NowUnix:         now,
		NewEntries:      newEntries,
	})
	if err != nil {
		slog.ErrorContext(ctx, "commit poll", "feed_id", sub.ID, "err", err)
		_ = db.UpdateAfterError(ctx, w.db, sub.ID, err.Error(), nextPoll)
		return
	}
	slog.InfoContext(ctx, "poll ok", "feed_id", sub.ID, "inserted", inserted, "total_items", len(res.Feed.Items))
}

func authorName(i *gofeed.Item) string {
	if i.Author != nil {
		return i.Author.Name
	}
	return ""
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
```

- [ ] **Step 4 (GREEN): Run the tests, confirm they pass**

```bash
go test ./internal/poll/... -race -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/poll/worker.go internal/poll/worker_test.go
git commit -m "Add per-feed polling worker with panic recovery"
```

---

### Task 4.3: Scheduler (TDD)

**Files:**
- Create: `internal/poll/scheduler.go`, `internal/poll/scheduler_test.go`

- [ ] **Step 1 (RED): Write the failing test**

The scheduler ticks at a configurable interval. For testability we expose `Tick()` so tests can drive it manually instead of waiting for the wall clock.

`internal/poll/scheduler_test.go`:

```go
package poll

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func TestScheduler_TickDispatchesDueFeeds(t *testing.T) {
	t.Parallel()

	var hits sync.Map
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Store(r.URL.Path, true)
		_, _ = w.Write([]byte(sampleAtom))
	}))
	defer srv.Close()

	d := newDB(t)
	id1, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{Title: "a", FeedURL: srv.URL + "/a", NextPoll: 0, Created: 0})
	id2, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{Title: "b", FeedURL: srv.URL + "/b", NextPoll: 0, Created: 0})

	sch := NewScheduler(context.Background(), d, http.DefaultClient, SchedulerOpts{Workers: 2, Cadence: 30 * time.Minute})
	t.Cleanup(sch.Stop)
	sch.Tick(context.Background())
	require.NoError(t, sch.Wait(5*time.Second))

	_, ok1 := hits.Load("/a")
	_, ok2 := hits.Load("/b")
	require.True(t, ok1, "feed a should have been polled")
	require.True(t, ok2, "feed b should have been polled")

	for _, id := range []int64{id1, id2} {
		s, _ := db.GetSubscription(context.Background(), d, id)
		require.True(t, s.LastPollAt.Valid)
	}
}

func TestScheduler_SkipsInflight(t *testing.T) {
	t.Parallel()

	d := newDB(t)
	id, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{Title: "a", FeedURL: "http://invalid.invalid", NextPoll: 0, Created: 0})

	sch := NewScheduler(context.Background(), d, http.DefaultClient, SchedulerOpts{Workers: 1, Cadence: 30 * time.Minute})
	t.Cleanup(sch.Stop)

	// Manually mark in-flight; Tick must skip it.
	require.True(t, sch.inflight.TryAcquire(id))

	dispatched := sch.Tick(context.Background())
	require.Equal(t, 0, dispatched)
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

```bash
go test ./internal/poll/... -run TestScheduler -v
```

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/poll/scheduler.go`:

```go
package poll

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/bcrisp4/tap/internal/db"
)

type SchedulerOpts struct {
	TickInterval time.Duration // default 60s
	Workers      int           // default 3
	Cadence      time.Duration // default 30m
}

type Scheduler struct {
	db       *sql.DB
	client   *http.Client
	opts     SchedulerOpts
	inflight *Inflight
	worker   *Worker
	jobs     chan db.DueSubscription
	tickDone chan struct{} // closed when the tick goroutine exits
	poke     chan struct{} // buffered; signal "tick now"
	workerWG sync.WaitGroup

	// parentCtx is the lifetime of this scheduler. parentCancel triggers
	// shutdown of both the tick loop and any in-flight worker context.
	parentCtx    context.Context
	parentCancel context.CancelFunc
	stopOnce     sync.Once
}

// NewScheduler creates a scheduler whose lifetime is bounded by base. Cancel
// base or call Stop() to shut down. Worker goroutines start immediately;
// the tick loop starts when Start() is called.
func NewScheduler(base context.Context, d *sql.DB, c *http.Client, o SchedulerOpts) *Scheduler {
	if o.TickInterval <= 0 {
		o.TickInterval = 60 * time.Second
	}
	if o.Workers <= 0 {
		o.Workers = 3
	}
	if o.Cadence <= 0 {
		o.Cadence = 30 * time.Minute
	}
	parentCtx, parentCancel := context.WithCancel(base)
	s := &Scheduler{
		db:           d,
		client:       c,
		opts:         o,
		inflight:     NewInflight(),
		worker:       NewWorker(d, c, WorkerOpts{Cadence: o.Cadence}),
		jobs:         make(chan db.DueSubscription, o.Workers*2),
		tickDone:     make(chan struct{}),
		poke:         make(chan struct{}, 1),
		parentCtx:    parentCtx,
		parentCancel: parentCancel,
	}
	for i := 0; i < o.Workers; i++ {
		s.workerWG.Add(1)
		go s.workerLoop()
	}
	return s
}

// workerLoop drains the jobs channel. The per-job ctx is derived from
// s.parentCtx so Stop() cancellation propagates into the in-flight HTTP fetch.
func (s *Scheduler) workerLoop() {
	defer s.workerWG.Done()
	for sub := range s.jobs {
		ctx, cancel := context.WithTimeout(s.parentCtx, 60*time.Second)
		s.worker.Run(ctx, sub)
		cancel()
		s.inflight.Release(sub.ID)
	}
}

// Start begins the periodic tick loop in a goroutine.
func (s *Scheduler) Start() {
	go s.tickLoop()
}

func (s *Scheduler) tickLoop() {
	defer close(s.tickDone)

	s.Tick(s.parentCtx) // initial tick at startup

	t := time.NewTicker(s.opts.TickInterval)
	defer t.Stop()
	for {
		select {
		case <-s.parentCtx.Done():
			return
		case <-t.C:
			s.Tick(s.parentCtx)
		case <-s.poke:
			s.Tick(s.parentCtx)
		}
	}
}

// Tick selects due subscriptions and dispatches them. Returns dispatched count.
// Public for tests; production code uses Start() / Poke().
func (s *Scheduler) Tick(ctx context.Context) int {
	now := time.Now().Unix()
	due, err := db.ListDuePolls(ctx, s.db, now, 100)
	if err != nil {
		slog.ErrorContext(ctx, "list due polls", "err", err)
		return 0
	}
	dispatched := 0
	for _, sub := range due {
		if !s.inflight.TryAcquire(sub.ID) {
			continue
		}
		// Guard the send against shutdown — without the parentCtx case here,
		// if Stop() races with Tick(), `s.jobs <- sub` after close(jobs) panics.
		select {
		case <-s.parentCtx.Done():
			s.inflight.Release(sub.ID)
			return dispatched
		case s.jobs <- sub:
			dispatched++
		default:
			s.inflight.Release(sub.ID)
			slog.WarnContext(ctx, "scheduler queue full, deferring", "feed_id", sub.ID)
		}
	}
	return dispatched
}

// Poke triggers an immediate tick. Non-blocking; if a poke is already pending
// it's a no-op (one queued tick is enough). Use after POSTing a new
// subscription so the user doesn't wait up to TickInterval for the first poll.
func (s *Scheduler) Poke() {
	select {
	case s.poke <- struct{}{}:
	default:
	}
}

// Stop initiates orderly shutdown:
//  1. Cancel parentCtx — signals tickLoop and cancels in-flight worker ctxs.
//  2. Wait for tickLoop to exit — guarantees no more sends to s.jobs.
//  3. Close s.jobs — safe now that no senders remain.
//  4. Wait for the worker pool to drain.
//
// This ordering is the only safe one — closing jobs before tickLoop has stopped
// would race with `Tick`'s send.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		s.parentCancel()
		<-s.tickDone
		close(s.jobs)
		s.workerWG.Wait()
	})
}

// Wait blocks until the job queue drains and no workers are in flight, or
// until timeout elapses. Test-only helper — production uses Stop().
func (s *Scheduler) Wait(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		if len(s.jobs) == 0 && len(s.inflight.IDs()) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("scheduler.Wait: timed out after %s", timeout)
		}
		<-tick.C
	}
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/poll/... -race -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/poll/scheduler.go internal/poll/scheduler_test.go
git commit -m "Add polling scheduler with worker pool"
```

---

## Phase 5 — HTTP API

### Task 5.1: Error response shape

**Files:**
- Create: `internal/api/errors.go`

> **TDD note:** this task ships without its own test because the helper is a pure JSON writer and is exercised end-to-end by the handler tests in Tasks 5.3 and 5.4 (which assert error responses for invalid JSON, bad URLs, and the like). If the helper grows logic in a future milestone, it earns its own RED → GREEN cycle then.

- [ ] **Step 1: Implement**

`internal/api/errors.go`:

```go
// Package api owns the REST handlers and JSON wire shapes.
package api

import (
	"encoding/json"
	"net/http"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// Stable error codes the SPA can switch on.
const (
	ErrCodeBadRequest = "bad_request"
	ErrCodeNotFound   = "not_found"
	ErrCodeConflict   = "conflict"
	ErrCodeInternal   = "internal"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: msg}})
}
```

- [ ] **Step 2: Commit (paired with handlers in next task)**

---

### Task 5.2: Healthz handler (TDD)

**Files:**
- Create: `internal/api/api.go`, `internal/api/api_test.go`

- [ ] **Step 1 (RED): Write the failing test**

`internal/api/api_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthz(t *testing.T) {
	t.Parallel()
	mux := NewMux(nil, nil) // nil DB OK — healthz doesn't touch it
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "ok", rr.Body.String())
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

- [ ] **Step 3 (GREEN): Implement the mux scaffold + healthz handler to make the test pass**

`internal/api/api.go`:

```go
package api

import (
	"database/sql"
	"net/http"
)

// NewMux returns the API mux. db is required for everything except /healthz.
// poke (optional) is called after a successful POST /api/v1/subscriptions so the
// scheduler can run an immediate tick instead of waiting for the next interval.
// Pass nil if you don't have a scheduler (tests).
func NewMux(db *sql.DB, poke func()) *http.ServeMux {
	m := http.NewServeMux()

	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	if db != nil {
		registerSubscriptionRoutes(m, db, poke)
		registerEntryRoutes(m, db)
	}

	return m
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/api/... -run TestHealthz -v
```

Expected: PASS.

- [ ] **Step 5: Commit (paired with subscription/entry handlers)**

---

### Task 5.3: Subscription handlers

**Files:**
- Create: `internal/api/subscriptions.go`, `internal/api/subscriptions_test.go`

- [ ] **Step 1 (RED): Write the failing tests**

`internal/api/subscriptions_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func newAPI(t *testing.T) (*http.ServeMux, *sql.DB) {
	t.Helper()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return NewMux(d, nil), d
}

func TestSubscriptions_PostThenList(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body, _ := json.Marshal(map[string]string{"feed_url": "https://example.com/feed"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil))
	require.Equal(t, http.StatusOK, rr2.Code)

	var resp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "https://example.com/feed", resp.Data[0]["feed_url"])
}

func TestSubscriptions_PostRejectsBadURL(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body, _ := json.Marshal(map[string]string{"feed_url": "not-a-url"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/api/subscriptions.go`:

```go
package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bcrisp4/tap/internal/db"
)

type subscriptionDTO struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	FeedURL    string `json:"feed_url"`
	SiteURL    string `json:"site_url,omitempty"`
	NextPollAt int64  `json:"next_poll_at"`
	LastPollAt int64  `json:"last_poll_at,omitempty"`
	ErrorCount int    `json:"error_count"`
	LastError  string `json:"last_error,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}

func toDTO(s db.Subscription) subscriptionDTO {
	d := subscriptionDTO{
		ID:         s.ID,
		Title:      s.Title,
		FeedURL:    s.FeedURL,
		NextPollAt: s.NextPollAt,
		ErrorCount: s.ErrorCount,
		CreatedAt:  s.CreatedAt,
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

func registerSubscriptionRoutes(m *http.ServeMux, d *sql.DB, poke func()) {
	m.HandleFunc("GET /api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		subs, err := db.ListSubscriptions(r.Context(), d)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		out := make([]subscriptionDTO, 0, len(subs))
		for _, s := range subs {
			out = append(out, toDTO(s))
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	})

	m.HandleFunc("POST /api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB cap on request body
		var body struct {
			FeedURL string `json:"feed_url"`
			Title   string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		if _, err := url.ParseRequestURI(body.FeedURL); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "feed_url must be an absolute URL")
			return
		}
		title := body.Title
		if title == "" {
			title = body.FeedURL
		}
		id, err := db.InsertSubscription(r.Context(), d, db.NewSubscription{
			Title:    title,
			FeedURL:  body.FeedURL,
			NextPoll: 0,
			Created:  time.Now().Unix(),
		})
		if err != nil {
			writeError(w, http.StatusConflict, ErrCodeConflict, "subscription already exists")
			return
		}
		s, err := db.GetSubscription(r.Context(), d, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		// Poke the scheduler so the new feed polls within seconds, not
		// up-to-TickInterval. Best-effort: a missed poke just delays
		// the first poll to the next regular tick.
		if poke != nil {
			poke()
		}
		writeJSON(w, http.StatusCreated, toDTO(s))
	})

	m.HandleFunc("DELETE /api/v1/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		// Idempotent: deleting a non-existent ID is a 204, not a 404.
		// Matches the SPA's optimistic-delete model (the client may retry).
		if err := db.DeleteSubscription(r.Context(), d, id); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go internal/api/errors.go internal/api/subscriptions.go internal/api/subscriptions_test.go
git commit -m "Add API mux, healthz, and subscription handlers"
```

---

### Task 5.4: Entry handlers

**Files:**
- Create: `internal/api/entries.go`, `internal/api/entries_test.go`

- [ ] **Step 1 (RED): Write the failing tests**

`internal/api/entries_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func TestEntries_ListAndPatchRead(t *testing.T) {
	t.Parallel()
	mux, d := newAPI(t)

	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: "https://example.com/feed", NextPoll: 0, Created: 0,
	})
	_, err := db.UpdateAfterPoll(context.Background(), d, subID, db.PollResult{
		NowUnix: 100, NextPollAt: 200, NewEntries: []db.NewEntry{
			{Hash: "h1", Title: "A", URL: "https://e.com/a", Content: "<p>a</p>", PublishedAt: 50},
		},
	})
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/entries?unread=1", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var listResp struct {
		Data       []map[string]any `json:"data"`
		NextCursor int64            `json:"next_cursor"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&listResp))
	require.Len(t, listResp.Data, 1)

	id := int64(listResp.Data[0]["id"].(float64))

	patch, _ := json.Marshal(map[string]bool{"read": true})
	pr := httptest.NewRequest(http.MethodPatch, "/api/v1/entries/"+toStr(id), bytes.NewReader(patch))
	pr.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, pr)
	require.Equal(t, http.StatusOK, rr2.Code, rr2.Body.String())

	rr3 := httptest.NewRecorder()
	mux.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/api/v1/entries?unread=1", nil))
	var after struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr3.Body).Decode(&after))
	require.Len(t, after.Data, 0, "marking read should remove from unread list")
}

func toStr(i int64) string {
	return strconv.FormatInt(i, 10)
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/api/entries.go`:

```go
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bcrisp4/tap/internal/db"
)

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
}

type entryDetailDTO struct {
	entryListItemDTO
	Content string `json:"content"`
}

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
	}
	if e.Author.Valid {
		d.Author = e.Author.String
	}
	return d
}

func registerEntryRoutes(m *http.ServeMux, d *sql.DB) {
	m.HandleFunc("GET /api/v1/entries", func(w http.ResponseWriter, r *http.Request) {
		p := db.ListEntriesParams{
			UnreadOnly: r.URL.Query().Get("unread") == "1",
		}
		if v := r.URL.Query().Get("feed"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid feed id")
				return
			}
			p.SubscriptionID = id
		}
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid limit")
				return
			}
			p.Limit = n
		}
		if v := r.URL.Query().Get("cursor"); v != "" {
			// Cursor format: "<published_at>_<id>". Opaque to callers — they
			// just round-trip whatever next_cursor came back from the prior page.
			parts := strings.SplitN(v, "_", 2)
			if len(parts) != 2 {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid cursor")
				return
			}
			cp, err1 := strconv.ParseInt(parts[0], 10, 64)
			ci, err2 := strconv.ParseInt(parts[1], 10, 64)
			if err1 != nil || err2 != nil {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid cursor")
				return
			}
			p.CursorPublishedAt = cp
			p.CursorID = ci
		}

		entries, nextPub, nextID, err := db.ListEntries(r.Context(), d, p)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		out := make([]entryListItemDTO, 0, len(entries))
		for _, e := range entries {
			out = append(out, toListItem(e))
		}
		resp := map[string]any{"data": out}
		if nextPub > 0 {
			resp["next_cursor"] = fmt.Sprintf("%d_%d", nextPub, nextID)
		}
		writeJSON(w, http.StatusOK, resp)
	})

	m.HandleFunc("GET /api/v1/entries/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		e, err := db.GetEntry(r.Context(), d, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "entry not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entryDetailDTO{entryListItemDTO: toListItem(e), Content: e.Content})
	})

	m.HandleFunc("PATCH /api/v1/entries/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			Read  *bool `json:"read"`
			Saved *bool `json:"saved"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		if err := db.UpdateEntry(r.Context(), d, id, db.EntryUpdate{Read: body.Read, Saved: body.Saved}); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		e, err := db.GetEntry(r.Context(), d, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toListItem(e))
	})
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/api/entries.go internal/api/entries_test.go
git commit -m "Add entry list / detail / patch handlers"
```

---

## Phase 6 — HTTP server lifecycle and SPA embed

### Task 6.1: SPA file handler (embed.FS) (TDD)

**Files:**
- Create: `internal/server/spa.go`, `internal/server/spa_test.go`

- [ ] **Step 1 (RED): Write the failing test**

`internal/server/spa_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSPA_ServesIndex(t *testing.T) {
	t.Parallel()
	h := SPAHandler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.True(t, strings.HasPrefix(rr.Header().Get("Content-Type"), "text/html"))
}

func TestSPA_FallsBackToIndexForUnknownRoute(t *testing.T) {
	t.Parallel()
	h := SPAHandler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/entry/42", nil))
	require.Equal(t, http.StatusOK, rr.Code, "unknown SPA routes should fall back to index.html")
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/server/spa.go`:

```go
// Package server owns the HTTP server lifecycle and the embedded SPA glue.
package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/bcrisp4/tap/web"
)

// SPAHandler serves the embedded SPA bundle. Falls back to index.html for
// unknown routes so client-side routing works for deep links.
//
// There is no dev-mode branch: in dev, the developer visits Vite directly on
// :5173, and Vite proxies /api + /healthz to the Go server on :8080. The Go
// server only ever serves the embedded bundle.
func SPAHandler() http.Handler {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		// Build-time failure to embed should never reach runtime.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "SPA bundle missing", http.StatusInternalServerError)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			clean = "index.html"
		}
		// Try the asset; fall back to index.html for SPA routes.
		if _, err := fs.Stat(dist, clean); err == nil {
			http.ServeFileFS(w, r, dist, clean)
			return
		}
		http.ServeFileFS(w, r, dist, "index.html")
	})
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/server/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/spa.go internal/server/spa_test.go
git commit -m "Add embedded SPA handler with index.html fallback for client routes"
```

---

### Task 6.2: HTTP server lifecycle (TDD)

**Files:**
- Create: `internal/server/server.go`, `internal/server/server_test.go`

- [ ] **Step 1 (RED): Write the failing test**

`internal/server/server_test.go`:

```go
package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServer_StartAndShutdown(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/x", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("x")) })

	s := New(Config{Addr: "127.0.0.1:0", Handler: mux})
	require.NoError(t, s.Start())
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })

	resp, err := http.Get("http://" + s.Addr() + "/x")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
}
```

- [ ] **Step 2 (RED): Run the test, confirm it fails for the expected reason**

- [ ] **Step 3 (GREEN): Write the minimum implementation to make the test pass**

`internal/server/server.go`:

```go
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type Config struct {
	Addr    string // ":8080" or "127.0.0.1:8080"
	Handler http.Handler
}

type Server struct {
	cfg      Config
	listener net.Listener
	srv      *http.Server
}

func New(cfg Config) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) Start() error {
	l, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Addr, err)
	}
	s.listener = l
	s.srv = &http.Server{
		Handler:           s.cfg.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := s.srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "err", err)
		}
	}()
	slog.Info("http listening", "addr", l.Addr().String())
	return nil
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Shutdown gracefully drains in-flight requests with the context's deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}
```

- [ ] **Step 4 (GREEN): Run the test, confirm it passes**

```bash
go test ./internal/server/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "Add HTTP server lifecycle with graceful shutdown"
```

---

### Task 6.3: Wire `cmd/tap/main.go`

**Files:**
- Create: `cmd/tap/main.go`

> **TDD note:** `main.go` is wiring — flag parsing, lifecycle, signal handling, dependency assembly. Its behaviour is covered end-to-end by the integration test in Task 7.1, which boots the server through the same code path the binary does. The smoke run in Step 3 below is a manual verification of the wiring, not a substitute for the integration test.

- [ ] **Step 1: Implement**

`cmd/tap/main.go`:

```go
// Package main is the Tap binary entry point.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/poll"
	"github.com/bcrisp4/tap/internal/server"
)

func main() {
	var (
		addr    = flag.String("addr", "127.0.0.1:8080", "HTTP listen address (set 0.0.0.0:8080 in containers)")
		dataDir = flag.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
		logFmt  = flag.String("log-format", "json", "log format: json or text")
	)
	flag.Parse()

	configureLogger(*logFmt)

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		slog.Error("create data dir", "err", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(*dataDir, "tap.db")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	d, err := db.Open(ctx, dbPath)
	if err != nil {
		slog.Error("open db", "path", dbPath, "err", err)
		os.Exit(1)
	}
	defer d.Close()

	if err := db.Migrate(ctx, d); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	// HTTP client used by the polling worker pool. The 30s Client.Timeout
	// bounds total per-request time even when ctx isn't strictly enforced;
	// Transport timeouts cap connect / TLS / idle separately. Without these,
	// a slow origin would tie up a worker for the full 60s ctx ceiling.
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        32,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{})
	sched.Start()

	mux := http.NewServeMux()
	// /api/ and /healthz both go through the same factory; sched.Poke is wired
	// into POST /api/v1/subscriptions so a freshly added feed polls immediately
	// rather than waiting up to TickInterval (60s).
	apiMux := api.NewMux(d, sched.Poke)
	mux.Handle("/api/", apiMux)
	mux.Handle("/healthz", apiMux)
	mux.Handle("/", server.SPAHandler())

	srv := server.New(server.Config{Addr: *addr, Handler: mux})
	if err := srv.Start(); err != nil {
		slog.Error("server start", "err", err)
		os.Exit(1)
	}

	<-ctx.Done()
	slog.Info("shutting down")

	// Stop accepting connections; drain in-flight HTTP. Then stop the scheduler
	// (which cancels in-flight worker ctxs and waits for them to finish).
	shutdownCtx, sCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer sCancel()
	_ = srv.Shutdown(shutdownCtx)
	sched.Stop()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func configureLogger(format string) {
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(h))
}
```

- [ ] **Step 2: Build**

```bash
go build -o bin/tap ./cmd/tap
```

Expected: produces `bin/tap`.

- [ ] **Step 3: Smoke run**

```bash
bin/tap &
TAP_PID=$!
sleep 1
curl -sS http://127.0.0.1:8080/healthz
echo
kill $TAP_PID
```

Expected: `ok`, then clean shutdown.

- [ ] **Step 4: Commit**

```bash
git add cmd/tap/main.go
git commit -m "Wire main.go: open DB, run migrations, start scheduler and HTTP server"
```

---

## Phase 7 — End-to-end smoke test

### Task 7.1: Integration test

**Files:**
- Create: `cmd/tap/main_test.go`

This test boots the API mux against an in-memory DB, points the scheduler at a fixture HTTP server serving a tiny Atom feed, fires a tick, and asserts entries appear via `GET /api/v1/entries`.

- [ ] **Step 1: Implement**

`cmd/tap/main_test.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/poll"
	"github.com/stretchr/testify/require"
)

func TestEndToEnd_SubscribePollServeEntries(t *testing.T) {
	t.Parallel()

	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>End-to-end</title>
  <id>urn:e2e</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>Hello</title>
    <id>urn:e2e:1</id>
    <link href="https://e2e.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;hello&lt;/p&gt;</content>
  </entry>
</feed>`
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	defer feedSrv.Close()

	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(context.Background(), d))

	mux := api.NewMux(d, nil)

	// POST /api/v1/subscriptions
	body := strings.NewReader(`{"feed_url":"` + feedSrv.URL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	// Drive the scheduler.
	sched := poll.NewScheduler(context.Background(), d, http.DefaultClient, poll.SchedulerOpts{Workers: 1, Cadence: time.Hour})
	defer sched.Stop()
	sched.Tick(context.Background())
	require.NoError(t, sched.Wait(5*time.Second))

	// GET /api/v1/entries
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil))
	require.Equal(t, http.StatusOK, rr2.Code)

	var resp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "Hello", resp.Data[0]["title"])
}
```

- [ ] **Step 2: Run**

```bash
go test ./cmd/tap/... -v
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/tap/main_test.go
git commit -m "Add end-to-end integration test"
```

---

## Phase 8 — SPA scaffolding (real)

### Task 8.1: Replace boilerplate `App.svelte` and `main.ts`

**Files:**
- Modify: `web/src/main.ts`, `web/src/App.svelte`
- Create: `web/src/styles/tokens.css`, `web/src/styles/global.css`

- [ ] **Step 1: Update `web/src/main.ts` to import global styles**

```ts
import { mount } from 'svelte';
import App from './App.svelte';
import './styles/tokens.css';
import './styles/global.css';

const app = mount(App, { target: document.getElementById('app')! });

export default app;
```

- [ ] **Step 2: Replace `web/src/App.svelte` with the routing root**

```svelte
<script lang="ts">
  import { route } from './lib/router';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';
</script>

{#if $route.name === 'reader'}
  <Reader id={$route.params.id} />
{:else}
  <Unread />
{/if}
```

(`route`, `Unread`, and `Reader` are defined in later tasks — this file will fail to type-check until Phase 9–11 land.)

---

### Task 8.2: Design tokens (light theme only)

**Files:**
- Create: `web/src/styles/tokens.css`

- [ ] **Step 1: Write tokens**

```css
:root {
  --bg: #fafaf7;
  --bg-soft: #f3f3ee;
  --surface: #ffffff;
  --ink: #1a1a1a;
  --ink-2: #4a4a48;
  --ink-3: #8a8a86;
  --ink-4: #c8c8c2;
  --rule: #e6e6df;
  --accent: #002FA7;
  --accent-soft: rgba(0, 47, 167, 0.08);

  --serif: "Source Serif 4", "Iowan Old Style", Charter, Georgia, serif;
  --sans:  "Inter Tight", -apple-system, BlinkMacSystemFont, system-ui, sans-serif;
  --mono:  "JetBrains Mono", ui-monospace, "SF Mono", Menlo, monospace;
}
```

---

### Task 8.3: Base styles + font imports

**Files:**
- Create: `web/src/styles/global.css`

- [ ] **Step 1: Write base styles**

```css
@import url("https://fonts.googleapis.com/css2?family=Inter+Tight:wght@400;500;600&family=JetBrains+Mono:wght@400;500&family=Source+Serif+4:opsz,wght@8..60,400;8..60,500;8..60,600&family=Source+Serif+4:ital,opsz,wght@1,8..60,400&display=swap");

*,
*::before,
*::after {
  box-sizing: border-box;
}

html, body {
  margin: 0;
  padding: 0;
  background: var(--bg);
  color: var(--ink);
  font-family: var(--sans);
  font-size: 14px;
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
}

button {
  font-family: inherit;
  font-size: inherit;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  color: inherit;
}

a {
  color: var(--accent);
  text-decoration: none;
}

:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/main.ts web/src/App.svelte web/src/styles/tokens.css web/src/styles/global.css
git commit -m "Wire Svelte app shell, design tokens, and base styles"
```

---

## Phase 9 — SPA core libraries

### Task 9.1: Shared types matching the API

**Files:**
- Create: `web/src/lib/types.ts`

```ts
export type Subscription = {
  id: number;
  title: string;
  feed_url: string;
  site_url?: string;
  next_poll_at: number;
  last_poll_at?: number;
  error_count: number;
  last_error?: string;
  created_at: number;
};

export type EntryListItem = {
  id: number;
  subscription_id: number;
  title: string;
  author?: string;
  url: string;
  published_at: number;
  fetched_at: number;
  read: boolean;
  saved: boolean;
};

export type EntryDetail = EntryListItem & {
  content: string;
};

export type ListResponse<T> = {
  data: T[];
  // Opaque cursor string ("<published_at>_<id>"). Pass back to the next request
  // as ?cursor=. Absent when there are no more pages.
  next_cursor?: string;
};

export type ApiError = {
  error: { code: string; message: string };
};
```

---

### Task 9.2: API client

**Files:**
- Create: `web/src/lib/api.ts`

```ts
import type {
  Subscription,
  EntryListItem,
  EntryDetail,
  ListResponse,
  ApiError,
} from './types';

const BASE = '/api/v1';

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json', ...(init.headers ?? {}) },
    ...init,
  });
  if (!res.ok) {
    let detail: ApiError | null = null;
    try { detail = await res.json(); } catch { /* swallow */ }
    throw new Error(detail?.error.message ?? `${res.status} ${res.statusText}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  listSubscriptions: () =>
    request<ListResponse<Subscription>>('/subscriptions').then(r => r.data),

  addSubscription: (feed_url: string) =>
    request<Subscription>('/subscriptions', {
      method: 'POST',
      body: JSON.stringify({ feed_url }),
    }),

  deleteSubscription: (id: number) =>
    request<void>(`/subscriptions/${id}`, { method: 'DELETE' }),

  listEntries: (params: {
    unread?: boolean;
    feed?: number;
    limit?: number;
    cursor?: string;
  } = {}) => {
    const qs = new URLSearchParams();
    if (params.unread) qs.set('unread', '1');
    if (params.feed)   qs.set('feed', String(params.feed));
    if (params.limit)  qs.set('limit', String(params.limit));
    if (params.cursor) qs.set('cursor', params.cursor);
    const suffix = qs.toString() ? `?${qs}` : '';
    return request<ListResponse<EntryListItem>>(`/entries${suffix}`);
  },

  getEntry: (id: number) =>
    request<EntryDetail>(`/entries/${id}`),

  patchEntry: (id: number, patch: { read?: boolean; saved?: boolean }) =>
    request<EntryListItem>(`/entries/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),
};
```

---

### Task 9.3: Tiny client-side router

**Files:**
- Create: `web/src/lib/router.ts`

```ts
import { writable, type Readable } from 'svelte/store';

type RouteState =
  | { name: 'unread' }
  | { name: 'reader'; params: { id: number } };

function parse(pathname: string): RouteState {
  const m = pathname.match(/^\/entry\/(\d+)$/);
  if (m) return { name: 'reader', params: { id: Number(m[1]) } };
  return { name: 'unread' };
}

const internal = writable<RouteState>(parse(window.location.pathname));

window.addEventListener('popstate', () => internal.set(parse(window.location.pathname)));

export const route: Readable<RouteState> = { subscribe: internal.subscribe };

export function navigate(to: string) {
  if (window.location.pathname === to) return;
  window.history.pushState({}, '', to);
  internal.set(parse(to));
}
```

> **Note:** uses Svelte's `writable` store rather than the Svelte 5 `$state` rune because routing state needs to be importable as a singleton from any module. Consult the `svelte-runes` skill for the rationale; runes are component-scoped while writable stores are module-scoped.

---

### Task 9.4: Entries store

**Files:**
- Create: `web/src/lib/store.ts`

```ts
import { writable } from 'svelte/store';
import type { EntryListItem, Subscription } from './types';
import { api } from './api';

function entriesStore() {
  const { subscribe, update, set } = writable<{
    items: EntryListItem[];
    loading: boolean;
    error: string | null;
  }>({ items: [], loading: false, error: null });

  return {
    subscribe,
    async load(unreadOnly = true) {
      set({ items: [], loading: true, error: null });
      try {
        const r = await api.listEntries({ unread: unreadOnly, limit: 100 });
        set({ items: r.data, loading: false, error: null });
      } catch (e) {
        set({ items: [], loading: false, error: (e as Error).message });
      }
    },
    async toggleRead(id: number, read: boolean) {
      // Optimistic update.
      let prev: boolean | null = null;
      update(s => {
        const idx = s.items.findIndex(e => e.id === id);
        if (idx >= 0) {
          prev = s.items[idx].read;
          s.items[idx] = { ...s.items[idx], read };
        }
        return s;
      });
      try {
        await api.patchEntry(id, { read });
      } catch (e) {
        // Roll back.
        if (prev !== null) {
          update(s => {
            const idx = s.items.findIndex(e => e.id === id);
            if (idx >= 0) s.items[idx] = { ...s.items[idx], read: prev! };
            return s;
          });
        }
        throw e;
      }
    },
  };
}

export const entries = entriesStore();

function subscriptionsStore() {
  const { subscribe, set } = writable<Subscription[]>([]);
  return {
    subscribe,
    async load() {
      set(await api.listSubscriptions());
    },
    async add(feed_url: string) {
      await api.addSubscription(feed_url);
      await this.load();
    },
  };
}

export const subscriptions = subscriptionsStore();
```

- [ ] **Commit Phase 9**

```bash
git add web/src/lib/
git commit -m "Add SPA core: types, API client, router, and stores"
```

---

## Phase 10 — SPA components

For each component below: create the file, paste the code, save. There are no per-component tests in M1 — visual verification happens in Phase 13.

### Task 10.1: `JunctionDot.svelte`

**Files:** Create `web/src/components/JunctionDot.svelte`

```svelte
<script lang="ts">
  type Props = {
    size?: number;
    filled?: boolean;
    color?: string;
  };
  let { size = 6, filled = true, color = 'var(--accent)' }: Props = $props();
</script>

<span
  class="dot"
  style:--size={`${size}px`}
  style:--c={color}
  class:filled
  aria-hidden="true"
></span>

<style>
  .dot {
    display: inline-block;
    width: var(--size);
    height: var(--size);
    border-radius: 50%;
    background: transparent;
    border: 1px solid var(--ink-4);
  }
  .dot.filled {
    background: var(--c);
    border-color: var(--c);
  }
</style>
```

---

### Task 10.2: `FeedAvatar.svelte`

**Files:**
- Create `web/src/lib/colors.ts`, `web/src/components/FeedAvatar.svelte`

`web/src/lib/colors.ts`:

```ts
const PALETTE = [
  '#3a4a5a', '#7a4a3a', '#4a6a4a', '#5a4a6a',
  '#6a5a3a', '#3a5a6a', '#5a3a4a', '#4a5a3a',
];

export function colorForFeed(feedURL: string): string {
  let hash = 0;
  for (let i = 0; i < feedURL.length; i++) {
    hash = (hash * 31 + feedURL.charCodeAt(i)) | 0;
  }
  return PALETTE[Math.abs(hash) % PALETTE.length];
}
```

`web/src/components/FeedAvatar.svelte`:

```svelte
<script lang="ts">
  import { colorForFeed } from '../lib/colors';

  type Props = {
    feedURL: string;
    size?: number;
    radius?: number;
  };
  let { feedURL, size = 14, radius = 3 }: Props = $props();
  const color = $derived(colorForFeed(feedURL));
</script>

<span
  class="avatar"
  style:--size={`${size}px`}
  style:--r={`${radius}px`}
  style:background={color}
  aria-hidden="true"
></span>

<style>
  .avatar {
    display: inline-block;
    width: var(--size);
    height: var(--size);
    border-radius: var(--r);
    flex: none;
  }
</style>
```

---

### Task 10.3: `AddFeedForm.svelte`

```svelte
<script lang="ts">
  import { subscriptions } from '../lib/store';

  let url = $state('');
  let busy = $state(false);
  let error = $state<string | null>(null);

  async function submit(ev: Event) {
    ev.preventDefault();
    if (!url.trim()) return;
    busy = true;
    error = null;
    try {
      await subscriptions.add(url.trim());
      url = '';
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={submit}>
  <input
    type="url"
    placeholder="Paste a feed URL"
    bind:value={url}
    disabled={busy}
    required
  />
  <button type="submit" disabled={busy || !url.trim()}>Add</button>
  {#if error}
    <p class="error">{error}</p>
  {/if}
</form>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 6px 20px 12px;
  }
  input {
    font-family: var(--sans);
    font-size: 12px;
    padding: 6px 8px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--surface);
  }
  button {
    align-self: flex-start;
    font-family: var(--sans);
    font-size: 12px;
    padding: 6px 10px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--surface);
  }
  .error {
    margin: 0;
    font-family: var(--mono);
    font-size: 10px;
    color: #b14;
  }
</style>
```

---

### Task 10.4: `TopBar.svelte`

```svelte
<script lang="ts">
  type Props = {
    title: string;
    countShown: number;
    countTotal: number;
    onRefresh?: () => void;
  };
  let { title, countShown, countTotal, onRefresh }: Props = $props();
</script>

<header class="topbar">
  <span class="crumb">{title}</span>
  <span class="count">{countShown} of {countTotal}</span>
  <span class="spacer"></span>
  {#if onRefresh}
    <button class="icon" onclick={onRefresh} aria-label="Refresh">↻</button>
  {/if}
</header>

<style>
  .topbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 14px 24px;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
  }
  .crumb {
    font-family: var(--sans);
    font-weight: 600;
    font-size: 13px;
  }
  .count {
    font-family: var(--mono);
    font-size: 11px;
    color: var(--ink-3);
    margin-left: 8px;
  }
  .spacer { flex: 1; }
  .icon {
    width: 28px;
    height: 28px;
    border-radius: 4px;
    color: var(--ink-2);
  }
  .icon:hover { background: var(--bg-soft); }
</style>
```

---

### Task 10.5: `Sidebar.svelte`

```svelte
<script lang="ts">
  import FeedAvatar from './FeedAvatar.svelte';
  import AddFeedForm from './AddFeedForm.svelte';
  import { subscriptions } from '../lib/store';
  import { onMount } from 'svelte';

  onMount(() => { subscriptions.load(); });
</script>

<aside class="sidebar">
  <div class="brand">tap<span class="dot">.</span></div>

  <div class="group-title">READING</div>
  <a class="navitem active" href="/">Unread</a>

  <div class="group-title">FEEDS</div>
  {#each $subscriptions as sub (sub.id)}
    <div class="feedrow">
      <FeedAvatar feedURL={sub.feed_url} />
      <span class="feedname" title={sub.title}>{sub.title}</span>
    </div>
  {/each}

  <div class="group-title">SYSTEM</div>
  <AddFeedForm />
</aside>

<style>
  .sidebar {
    width: 240px;
    border-right: 1px solid var(--rule);
    background: var(--bg);
    height: 100vh;
    overflow-y: auto;
    flex: none;
  }
  .brand {
    font-family: var(--sans);
    font-weight: 600;
    font-size: 17px;
    padding: 18px 20px 8px;
  }
  .brand .dot {
    color: var(--accent);
    font-weight: 700;
    margin-left: 1px;
  }
  .group-title {
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-3);
    padding: 16px 20px 6px;
  }
  .navitem {
    display: block;
    padding: 6px 20px;
    font-family: var(--sans);
    font-size: 13px;
    color: var(--ink);
    border-left: 2px solid transparent;
  }
  .navitem.active { border-left-color: var(--accent); }
  .feedrow {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 20px;
  }
  .feedname {
    font-family: var(--sans);
    font-size: 12px;
    color: var(--ink-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
```

---

### Task 10.6: `EntryRow.svelte`

```svelte
<script lang="ts">
  import JunctionDot from './JunctionDot.svelte';
  import FeedAvatar from './FeedAvatar.svelte';
  import type { EntryListItem, Subscription } from '../lib/types';
  import { navigate } from '../lib/router';

  type Props = {
    entry: EntryListItem;
    feed: Subscription | undefined;
  };
  let { entry, feed }: Props = $props();

  function ago(ts: number): string {
    const sec = Math.max(1, Math.floor(Date.now() / 1000) - ts);
    if (sec < 60) return `${sec}s ago`;
    if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
    return `${Math.floor(sec / 86400)}d ago`;
  }
</script>

<button class="entry" onclick={() => navigate(`/entry/${entry.id}`)}>
  <span class="indicator">
    <JunctionDot filled={!entry.read} />
  </span>
  <div class="body">
    <div class="title" class:read={entry.read}>{entry.title}</div>
    <div class="meta">
      {#if feed}
        <FeedAvatar feedURL={feed.feed_url} size={9} radius={2} />
        <span class="src">{feed.title}</span>
        <span class="sep">·</span>
      {/if}
      <span class="ago">{ago(entry.published_at)}</span>
    </div>
  </div>
</button>

<style>
  .entry {
    display: flex;
    width: 100%;
    text-align: left;
    padding: 14px 24px 14px 40px;
    border-bottom: 1px solid var(--rule);
    position: relative;
  }
  .entry:hover { background: var(--bg-soft); }
  .indicator {
    position: absolute;
    left: 22px;
    top: 22px;
  }
  .body { flex: 1; min-width: 0; }
  .title {
    font-family: var(--serif);
    font-size: 17px;
    line-height: 1.3;
    font-weight: 500;
  }
  .title.read {
    font-weight: 400;
    color: var(--ink-3);
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
    font-family: var(--mono);
    font-size: 11px;
    color: var(--ink-3);
  }
  .src { font-weight: 500; color: var(--ink-2); font-family: var(--sans); }
  .sep { color: var(--ink-4); }
</style>
```

- [ ] **Commit Phase 10**

```bash
git add web/src/components/ web/src/lib/colors.ts
git commit -m "Add SPA components: junction dot, feed avatar, sidebar, top bar, entry row, add-feed form"
```

---

## Phase 11 — SPA views

### Task 11.1: `Unread.svelte`

**Files:** Create `web/src/views/Unread.svelte`

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import TopBar from '../components/TopBar.svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import { entries, subscriptions } from '../lib/store';

  onMount(() => {
    entries.load(true);
    subscriptions.load();
  });

  function feedFor(subId: number) {
    return $subscriptions.find(s => s.id === subId);
  }
</script>

<div class="layout">
  <Sidebar />
  <main class="main">
    <TopBar
      title="Unread"
      countShown={$entries.items.length}
      countTotal={$entries.items.length}
      onRefresh={() => entries.load(true)}
    />
    {#if $entries.loading}
      <p class="status">Loading…</p>
    {:else if $entries.error}
      <p class="status err">{$entries.error}</p>
    {:else if $entries.items.length === 0}
      <p class="status empty">No unread entries. Subscribe to a feed in the sidebar.</p>
    {:else}
      <div class="list">
        {#each $entries.items as entry (entry.id)}
          <EntryRow {entry} feed={feedFor(entry.subscription_id)} />
        {/each}
      </div>
    {/if}
  </main>
</div>

<style>
  .layout {
    display: flex;
    height: 100vh;
  }
  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    background: var(--bg);
  }
  .status {
    padding: 24px;
    color: var(--ink-3);
    font-family: var(--mono);
    font-size: 11px;
  }
  .status.err { color: #b14; }
  .list { flex: 1; }
</style>
```

---

### Task 11.2: `Reader.svelte`

**Files:** Create `web/src/views/Reader.svelte`

Uses `{@html}` to render the (M1: unsanitised, M2: sanitised) entry body. Per `svelte-template-directives`, this is appropriate because the server is responsible for sanitisation — the client trusts what it receives.

> **M1 security caveat:** the server does not sanitise feed HTML in M1 — that lands in M2. The binary defaults to a loopback bind, which contains the risk to the local machine. The container variant intentionally binds `0.0.0.0:8080` because Docker port mapping requires it. **Do not reverse-proxy the M1 container to any address a hostile-feed author can reach** (Tailscale, LAN, the public internet) until M2 lands sanitisation. The README also calls this out.

The Reader does **not** route mark-read through the entries store. The store's optimistic flow assumes the entry already lives there, but a user can deep-link to `/entry/N` without ever passing through the Unread view, in which case the store rollback path can't run. Talking to `api.patchEntry` directly is simpler and correct in both cases. When the user navigates back to Unread, that view re-fetches on mount, so store consistency takes care of itself.

```svelte
<script lang="ts">
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import type { EntryDetail } from '../lib/types';
  import FeedAvatar from '../components/FeedAvatar.svelte';
  import JunctionDot from '../components/JunctionDot.svelte';
  import { onMount } from 'svelte';

  type Props = { id: number };
  let { id }: Props = $props();

  let entry = $state<EntryDetail | null>(null);
  let error = $state<string | null>(null);

  onMount(async () => {
    try {
      entry = await api.getEntry(id);
      // Auto-mark-read on open. If the PATCH fails we leave the entry as unread
      // — the user can retry via the MARK READ button, which has the same shape.
      if (entry && !entry.read) {
        try {
          await api.patchEntry(id, { read: true });
          entry = { ...entry, read: true };
        } catch { /* swallow; user can manually toggle */ }
      }
    } catch (e) {
      error = (e as Error).message;
    }
  });

  async function toggleRead() {
    if (!entry) return;
    const want = !entry.read;
    try {
      await api.patchEntry(entry.id, { read: want });
      entry = { ...entry, read: want };
    } catch (e) {
      error = (e as Error).message;
    }
  }

  function host(url: string): string {
    try { return new URL(url).host; } catch { return ''; }
  }
</script>

<div class="reader">
  <header class="header">
    <button class="back" onclick={() => navigate('/')} aria-label="Back to unread">
      ‹ <span class="back-label">UNREAD</span>
    </button>
    {#if entry}
      <div class="actions">
        <button class="action" onclick={toggleRead}>
          {entry.read ? 'MARK UNREAD' : 'MARK READ'}
        </button>
        <a class="action" href={entry.url} target="_blank" rel="noopener">VIEW ORIGINAL</a>
      </div>
    {/if}
  </header>

  <article class="body">
    {#if error}
      <p class="err">{error}</p>
    {:else if !entry}
      <p class="loading">Loading…</p>
    {:else}
      <div class="source">
        <FeedAvatar feedURL={entry.url} size={10} radius={2} />
        <span class="src-host">{host(entry.url)}</span>
      </div>
      <h1>{entry.title}</h1>
      <div class="byline">
        {#if entry.author}{entry.author}<span class="sep"> · </span>{/if}
        {new Date(entry.published_at * 1000).toLocaleDateString()}
      </div>
      <div class="divider">
        <span class="rule"></span>
        <JunctionDot color="var(--accent)" />
        <span class="rule"></span>
      </div>
      <div class="content">{@html entry.content}</div>
      <div class="divider">
        <span class="rule"></span>
        <JunctionDot color="var(--ink-4)" filled={false} />
        <span class="rule"></span>
      </div>
    {/if}
  </article>
</div>

<style>
  .reader { height: 100vh; display: flex; flex-direction: column; background: var(--bg); }
  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 28px;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
    position: sticky; top: 0;
  }
  .back, .action {
    font-family: var(--mono);
    font-size: 11px;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--ink-2);
    padding: 6px 10px 6px 4px;
    border-radius: 4px;
  }
  .back:hover, .action:hover { background: var(--bg-soft); }
  .actions { display: flex; gap: 8px; }
  .body { max-width: 680px; margin: 0 auto; padding: 56px 56px 80px; flex: 1; overflow-y: auto; }
  .source {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--sans); font-size: 12px; color: var(--ink-2);
    margin-bottom: 18px;
  }
  .src-host { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
  h1 {
    font-family: var(--serif); font-size: 38px; line-height: 1.15;
    font-weight: 600; letter-spacing: -0.015em; margin: 0 0 16px;
    text-wrap: balance;
  }
  .byline {
    font-family: var(--mono); font-size: 11px; letter-spacing: 0.04em;
    text-transform: uppercase; color: var(--ink-3); margin-bottom: 28px;
  }
  .divider {
    display: flex; align-items: center; gap: 8px;
    margin: 32px 0;
  }
  .rule {
    flex: 1; height: 1px; background: var(--rule);
  }
  .content :global(p) {
    font-family: var(--serif); font-size: 17px; line-height: 1.7;
    color: var(--ink); margin: 0 0 22px; text-wrap: pretty;
  }
  .content :global(h2) {
    font-family: var(--serif); font-size: 22px; line-height: 1.25;
    font-weight: 600; letter-spacing: -0.01em; margin: 40px 0 14px;
  }
  .content :global(pre), .content :global(code) {
    font-family: var(--mono); font-size: 13px; line-height: 1.55;
    color: var(--ink-2); background: var(--bg-soft);
    padding: 14px 16px; border-left: 2px solid var(--accent); overflow-x: auto;
  }
  .err { color: #b14; font-family: var(--mono); font-size: 12px; }
  .loading { color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .sep { color: var(--ink-4); }
</style>
```

- [ ] **Commit Phase 11**

```bash
git add web/src/views/
git commit -m "Add Unread and Reader views"
```

---

## Phase 12 — Build pipeline

### Task 12.1: `Makefile`

**Files:** Create `Makefile`

In dev mode you visit `http://localhost:5173` (Vite). Vite proxies `/api` and `/healthz` to the Go server on `:8080`. In production you visit `:8080` and Go serves everything.

```make
.PHONY: dev build docker clean test

GO   ?= go
PNPM ?= pnpm
BIN  := bin/tap

dev:
	@echo "Starting Go on :8080 and Vite on :5173 — open http://localhost:5173"
	@$(GO) run ./cmd/tap & \
	 trap 'kill %1' EXIT; \
	 $(PNPM) --dir web dev

# CGO_ENABLED=0 keeps the binary truly static so it runs on a distroless image
# without glibc — same flag the Dockerfile sets, kept here so `make build`
# and `make docker` produce equivalent artefacts.
build: web/dist/index.html
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/tap

web/dist/index.html: $(shell find web/src -type f) web/index.html web/package.json
	$(PNPM) --dir web install --frozen-lockfile
	$(PNPM) --dir web build

docker: build
	docker build -t tap:dev .

test:
	$(GO) test ./... -race

clean:
	rm -rf bin/ web/dist/
```

---

### Task 12.2: `Dockerfile`

**Files:** Create `Dockerfile`

```dockerfile
# syntax=docker/dockerfile:1.7

# ---- web build stage ----
FROM node:22-alpine AS web
WORKDIR /web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ---- go build stage ----
FROM golang:1.24-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) \
    go build -trimpath -ldflags="-s -w" -o /out/tap ./cmd/tap

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/tap /tap
USER nonroot:nonroot
VOLUME ["/data"]
EXPOSE 8080
ENV TAP_DATA_DIR=/data
ENTRYPOINT ["/tap", "-addr", "0.0.0.0:8080"]
```

---

### Task 12.3: `.dockerignore`

**Files:** Create `.dockerignore`

```
.git
.github
.claude
.agents
.playwright-mcp
docs
ui_design
bin
**/node_modules
**/dist
*.db
*.db-shm
*.db-wal
data
README.md
Makefile
```

---

### Task 12.4: Build and verify

- [ ] **Step 1: Build the binary**

```bash
make build
ls -lh bin/tap
```

Expected: binary exists, ~10-20 MB.

- [ ] **Step 2: Run it**

```bash
mkdir -p data
./bin/tap &
TAP_PID=$!
sleep 1

curl -sS http://127.0.0.1:8080/healthz
echo
curl -sS -X POST -H "Content-Type: application/json" \
  -d '{"feed_url":"https://www.theverge.com/rss/index.xml"}' \
  http://127.0.0.1:8080/api/v1/subscriptions
echo

# Wait for the dispatcher tick.
sleep 65

curl -sS 'http://127.0.0.1:8080/api/v1/entries?unread=1&limit=5' | head -c 500
echo

kill $TAP_PID
```

Expected: healthz returns `ok`; POST returns the subscription DTO; after ~60s, GET /entries shows real entries.

- [ ] **Step 3: Build container**

```bash
make docker
docker images tap:dev
```

Expected: image exists, < 30 MB.

- [ ] **Step 4: Run container**

```bash
mkdir -p /tmp/tap-data
docker run -d --name tap -p 8080:8080 -v /tmp/tap-data:/data tap:dev
sleep 2
curl -sS http://127.0.0.1:8080/healthz
echo
docker logs tap | tail -10
docker stop tap && docker rm tap
```

Expected: container starts, healthz works, clean stop.

- [ ] **Step 5: Commit Phase 12**

```bash
git add Makefile Dockerfile .dockerignore
git commit -m "Add Makefile, Dockerfile, and .dockerignore"
```

---

## Phase 13 — Definition-of-done verification

This is the manual checklist from the spec. Each item must be ticked and demonstrated before M1 is considered complete.

### Task 13.1: DoD checklist

Use Playwright MCP for UI verification (the chromium symlink is in place — see `~/.claude/projects/-home-ben-guest-Users-ben-src-tap/memory/playwright_chromium_arm64.md`).

- [ ] **DoD-1:** `make build` produces a single static binary at `bin/tap`.
  ```bash
  make build && file bin/tap
  ```
  Expected: ELF 64-bit, statically linked.

- [ ] **DoD-2:** `make docker` produces an OCI image under 30 MB.
  ```bash
  make docker && docker images tap:dev
  ```

- [ ] **DoD-3:** Binary starts cleanly, applies migrations, accepts API requests.
  ```bash
  rm -rf data && ./bin/tap & sleep 1; curl -sS http://127.0.0.1:8080/healthz; kill %1
  ```

- [ ] **DoD-4:** Subscribing to a real feed produces entries within 60s.

  Manual: open `http://127.0.0.1:8080/`, paste a real feed URL into the Add Feed form, wait up to 60s, refresh the Unread view. Use Playwright to script this if running unattended:
  ```
  navigate(http://127.0.0.1:8080/) → snapshot → fill add-feed form → wait 65s → snapshot → assert entries visible
  ```

- [ ] **DoD-5:** Reader view renders, entry is auto-marked read on open.
  Click an entry. Verify it loads, body renders. Return to Unread. Verify the entry is now greyed (read state).

- [ ] **DoD-6:** Read/saved state persists across reload.
  Reload the page. Verify the same entries are still in their read/unread state.

- [ ] **DoD-7:** Container persists data across restart.
  ```bash
  docker run -d --name tap -p 8080:8080 -v /tmp/tap-data:/data tap:dev
  # subscribe + wait + see entries via the SPA
  docker stop tap && docker rm tap
  docker run -d --name tap -p 8080:8080 -v /tmp/tap-data:/data tap:dev
  # entries still present
  ```

- [ ] **DoD-8:** All tests pass.
  ```bash
  go test ./... -race -v
  ```
  Expected: 0 failures.

When all eight ticks are green, M1 is done. Commit any remaining work and prepare to brainstorm M2.

---

## What this plan deliberately does *not* cover

- Sanitisation, media proxy, article extraction, SSRF guard — explicit M2–M5 scope.
- Auth (any kind), 2FA, passkeys — M6–M7.
- Sepia/dark themes, mobile responsive, keyboard shortcuts, gestures — M8.
- Categories, OPML, search, feed discovery — M9.
- Service worker, offline reading, PWA — M10.
- Archival, tombstones — M11.
- Observability (OTel), system status panel, admin CLI — M12.

If during implementation an "obvious improvement" from one of these areas surfaces, **resist it**. The point of M1 is to validate the architectural shape; polish belongs in its own milestone.
