# M1 — Walking skeleton

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md), with the visual identity and detailed design in [`../../ui_design/`](../../ui_design/). M1 is the first of twelve milestones — see [`../roadmap.md`](../roadmap.md).

The goal of M1 is to build the **thinnest possible end-to-end Tap** that proves the architecture: one static Go binary with the SPA embedded, embedded SQLite as both the source of truth and the work queue, an in-process polling loop, and the basic shape of the REST API. Every safety and polish layer is deliberately deferred to a later milestone.

## Goal

Run the binary (or `docker run` the image), open `http://localhost:8080` in a browser, paste in a real feed URL, see entries appear in the Unread view styled per the design system within sixty seconds, click into one, read its body, and mark it read. Subscribe to a few more feeds; everything persists across restart.

## In scope

### Repository layout

```
tap/
├── cmd/tap/main.go              # entry point
├── internal/
│   ├── api/                     # REST handlers
│   ├── db/                      # SQLite open + migrations + queries
│   ├── feed/                    # gofeed wrapper + hash computation
│   ├── poll/                    # scheduler + worker pool
│   └── server/                  # HTTP server lifecycle
├── web/                         # Svelte SPA (Vite project)
├── docs/                        # already exists
├── ui_design/                   # already exists
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

The Go binary embeds `web/dist/` via `//go:embed all:web/dist`.

### Build pipeline

- `make dev` runs the Vite dev server on `:5173` and `go run ./cmd/tap` together; the Go server proxies non-API requests to Vite during development so the SPA hot-reloads while the API runs in real Go.
- `make build` runs `pnpm --dir web run build` then `go build -o bin/tap ./cmd/tap`. The output is one static binary.
- `make docker` produces a distroless image (`gcr.io/distroless/static-debian12:nonroot`) with the binary embedded. The image declares one volume (`/data`) and one port (`8080`). It runs as a non-root user.

### Database

- SQLite via `modernc.org/sqlite` (pure-Go translation, no CGO).
- File path defaults to `${TAP_DATA_DIR}/tap.db` — `./data/tap.db` for the binary, `/data/tap.db` for the container.
- Migrations are SQL files under `internal/db/migrations/` embedded via `embed.FS` and applied sequentially on startup. A `schema_migrations` table tracks applied versions. The runner is ~50 lines of home-grown code; if it gets gnarly we'll switch to `pressly/goose` later.

#### Schema

```sql
CREATE TABLE subscriptions (
    id            INTEGER PRIMARY KEY,
    title         TEXT NOT NULL,
    feed_url      TEXT NOT NULL UNIQUE,
    site_url      TEXT,
    last_poll_at  INTEGER,            -- unix seconds, NULL until first poll
    next_poll_at  INTEGER NOT NULL,   -- unix seconds, set to 0 on insert
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
    content         TEXT NOT NULL,    -- raw HTML for now; M2 will sanitise
    published_at    INTEGER NOT NULL, -- unix seconds
    fetched_at      INTEGER NOT NULL,
    read            INTEGER NOT NULL DEFAULT 0,
    saved           INTEGER NOT NULL DEFAULT 0,
    UNIQUE (subscription_id, hash)
);
CREATE INDEX idx_entries_published   ON entries(published_at DESC);
CREATE INDEX idx_entries_subscription ON entries(subscription_id, published_at DESC);
CREATE INDEX idx_entries_unread       ON entries(read, published_at DESC);
```

The `(subscription_id, hash)` uniqueness contract drops duplicates silently. The hash is computed per the concept doc: feed-provided GUID if present, falling back to the entry URL, falling back to a SHA-256 of `title || published_at`.

### Polling pipeline

A single goroutine ticks every 60 seconds (the **scheduler**). On each tick it queries:

```sql
SELECT id, feed_url, etag, last_modified
FROM   subscriptions
WHERE  next_poll_at <= unixepoch()
  AND  id NOT IN (... in-flight markers ...)
LIMIT  100;
```

and dispatches each row to a worker pool of size 3. In-flight tracking lives in process memory (`map[int64]struct{}` guarded by a mutex) — **not** in the database. A crashed worker leaves the database row untouched, so the next tick re-dispatches that feed. Polls are idempotent.

Each worker:

1. Issues an HTTP `GET` with `If-None-Match` (if `etag` present) and `If-Modified-Since` (if `last_modified` present) headers.
2. On `304 Not Modified`: bumps `last_poll_at = now`, `next_poll_at = now + 30 min`, commits, exits.
3. On `200 OK`: parses with `gofeed`, computes hashes, opens a transaction, inserts non-duplicate entries, updates `subscriptions.etag` / `last_modified` / `last_poll_at` / `next_poll_at` / `error_count = 0` / `last_error = NULL`, commits.
4. On any error (network, parse, HTTP 4xx/5xx): increments `error_count`, sets `last_error`, bumps `next_poll_at = now + 30 min` (no exponential backoff in M1), commits.
5. Removes the in-flight marker, even on panic — wrapped in `defer recover()` so a single bad feed cannot kill the worker.

Adaptive cadence, per-host concurrency caps, and the SSRF guard are deferred to **M4**. M1 polls every feed at a fixed 30-minute cadence regardless of publication rate, and trusts the user not to subscribe to private-network URLs.

### HTTP server

`net/http` from the standard library, no router framework. A single mux:

| Method | Path | Description |
|---|---|---|
| `GET`    | `/healthz`                     | `200 OK` body `ok`. |
| `GET`    | `/api/v1/subscriptions`        | List all. Returns `{data: [...]}`. |
| `POST`   | `/api/v1/subscriptions`        | Body `{feed_url}`. Inserts a row with `next_poll_at = 0` so the next tick picks it up. Returns the row. |
| `DELETE` | `/api/v1/subscriptions/:id`    | Cascades to entries. |
| `GET`    | `/api/v1/entries`              | Query `?unread=1`, `?feed=ID`, `?limit=N` (default 50, max 200), `?cursor=<id>`. Body stripped to keep payload small. Returns `{data: [...], next_cursor}`. |
| `GET`    | `/api/v1/entries/:id`          | Full entry including body. |
| `PATCH`  | `/api/v1/entries/:id`          | Body `{read?: bool, saved?: bool}`. Returns the updated row. |
| `GET`    | (anything else)                | Embedded SPA, or proxy to Vite in dev mode. |

Conventions to honour even in M1, since they ripple:

- List responses use `{data: [...], next_cursor}` shape so pagination metadata is uniform.
- List responses strip the entry body; the body is fetched per-entry when the reader opens.
- Errors return `{error: {code, message}}` with a stable enumerated `code` string so the SPA can switch on the code.

The server binds to `127.0.0.1:8080` by default; the container variant binds to `0.0.0.0:8080`. There is no auth in M1 — the loopback bind is the only line of defence on the binary deployment, and the container's network namespace is the boundary on the container deployment.

Graceful shutdown on `SIGTERM`: stop accepting new HTTP requests, drain in-flight requests with a 30-second deadline, wait for the worker pool to drain, close the database.

### SPA

Plain Svelte 5 + Vite + TypeScript, package-managed with pnpm. Two routes managed by a tiny home-grown matcher (we only have two routes in M1):

- **`/` — Unread view.** Sidebar with brand wordmark, READING group with one item ("Unread"), FEEDS group listing subscribed feeds with avatars and unread counts, SYSTEM group with "Add feed" (opens an inline form). Top bar with crumb + "N of M" count + refresh button. Entry list renders the Unread row design from `ui_design/README.md` exactly: junction-dot indicator, serif title, mono meta line, two-line summary clamp. Click an entry → navigate to `/entry/:id`.
- **`/entry/:id` — Reader view.** Reader pane in the centre with the source line, serif title, mono byline, junction-dot top divider, body, junction-dot bottom divider, mono foot. Auto-mark-read fires once on mount per the concept doc. Back button returns to `/`. (The desktop "list rail" column from the design is deferred to M8 — M1 ships the simpler one-column reader.)

#### Visual scope

- **Light theme only.** Sepia and dark themes ship in M8.
- **Desktop layout only.** Mobile breakpoints, swipe gestures, and the bottom tab bar ship in M8.
- **No keyboard shortcuts, no animations beyond CSS hover.** Both ship in M8.
- The design tokens from `ui_design/styles.css` are the source of truth for colours and typography.
- The `FeedAvatar` component is implemented per the icon-or-color rule: render an icon if the feed provides one, else fall back to a flat colour square. M1 only exercises the colour-fallback branch; the icon-fetch path lands later when the icons table arrives.

#### State management

- Svelte 5 `$state` and `$derived` for component-local state.
- A typed API client (`web/src/lib/api.ts`) wraps `fetch` with the right base URL, request shapes, and response types.
- A small reactive store for the entry list and per-entry mutations. Mark-read and toggle-saved are optimistic: update the store immediately, fire the PATCH, roll back on failure.

### Logging

Stdlib `log/slog`. Text handler in dev (`make dev`), JSON handler in production builds. Every poll start, success, failure, and recovered panic is logged with the feed identifier. No structured request-ID propagation yet — that ships in M12.

### Tests

Two smoke tests in M1:

1. A unit test for the entry-hash computation, covering the GUID → URL → `title || published_at` fallback chain.
2. An integration test that boots the full server (in-memory SQLite, `httptest.Server` for the API), POSTs a subscription pointing at a fixture HTTP server serving a small Atom feed, advances the scheduler, and asserts that entries appear via `GET /api/v1/entries`.

We'll layer in comprehensive coverage as the architecture stabilises. M1 deliberately avoids over-testing throwaway scaffolding.

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| HTML sanitisation, image-URL rewriting, iframe allowlist | M2 |
| Media proxy, signed tokens, FS cache, request coalescing, MIME allowlist | M3 |
| Adaptive cadence, per-host concurrency cap, SSRF guard, retry/backoff, `Retry-After` honouring | M4 |
| Article extraction, per-feed CSS rules | M5 |
| Multi-user, password hashing, sessions, CSRF, admin bootstrap | M6 |
| TOTP, passkeys, recovery codes, session listing | M7 |
| Sepia + dark themes, serif/sans toggle, density toggle, keyboard shortcuts, mobile responsive, swipe gestures, the desktop reader's list-rail column | M8 |
| Categories, OPML import/export, full-text search, feed discovery | M9 |
| Service worker, offline reading, mutation queue, warm-cache driver, PWA manifest | M10 |
| Daily archival, tombstones, media-cache eviction | M11 |
| OTel logs/metrics/traces, system-status panel, admin CLI, healthcheck subcommand, brute-force lockout, rate limiting | M12 |

Anything in `concept.md` not explicitly listed in scope above is out of scope for M1.

## Risks and open questions

- **Migrations runner.** Home-grown ~50-line approach for now. If migrations get gnarly we'll swap to `pressly/goose`. Cheap to revisit.
- **Routing in the SPA.** Home-grown matcher for two routes. If we add a third route before M8 ships and the matcher feels strained, we'll pull in `svelte-spa-router`.
- **Dev-mode proxying.** Go-proxies-Vite is the cleanest UX but costs a little code. The alternative ("two ports + CORS") is uglier in practice. Going with the proxy.
- **Module path.** Default to `github.com/bcrisp4/tap` so the repo can publish naturally if ever made public.
- **Database busy timeout.** SQLite needs a `busy_timeout` PRAGMA set on connection open or concurrent writes can fail with `SQLITE_BUSY`. We'll set it (along with `journal_mode = WAL`, `foreign_keys = ON`, `synchronous = NORMAL`) in the connection setup.

## Definition of done

1. `make build` produces a single static binary that boots cleanly.
2. `make docker` produces an OCI image under 30 MB (distroless static).
3. The binary or container starts cleanly, applies migrations, and accepts API requests.
4. Subscribing to a real-world feed (e.g. The Verge RSS) results in entries appearing in the Unread view within 60 seconds.
5. Clicking an entry opens the Reader view, the body renders, the entry is marked read on open.
6. Marking entries read or saved persists across reload.
7. `docker run -v $PWD/data:/data ...` persists entries across restarts.
8. Both smoke tests pass.

## What this milestone deliberately does *not* prove

- That sanitised HTML renders correctly. (M2.)
- That the design system handles three themes and mobile. (M8.)
- That the polling layer scales to 100+ feeds with diverse cadences. (M4.)
- That auth works. (M6.)
- That the offline shell works. (M10.)

If you find yourself pulling work from the deferred list into M1 to "make it more real," push back. The point of this milestone is to validate the architectural shape with the smallest possible vertical slice. Polish belongs in its own milestone.
