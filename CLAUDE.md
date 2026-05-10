# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Tap is a self-hosted RSS / Atom / JSON Feed reader. It ships as **one
static Go binary** with an embedded SQLite database, an embedded Svelte
SPA, and no external services. See `docs/concept.md` for the full design
and `docs/roadmap.md` for the milestone plan; **M11 in progress** (archival +
tombstones — spec at `docs/specs/2026-05-10-m11-archival-tombstones.md`);
**M7 merged** (2FA + passkeys
+ per-user data isolation; **M6 merged** — auth
foundations — spec at
`docs/specs/2026-05-10-m6-auth-foundations.md`; M5 article extraction
merged — spec at `docs/specs/2026-05-09-m5-article-extraction.md`; M4
polling discipline merged — spec at
`docs/specs/2026-05-09-m4-polling-discipline.md`; M3 media proxy merged
— spec at `docs/specs/2026-05-09-m3-media-proxy.md`; M2 sanitisation
pipeline merged — spec at `docs/specs/2026-05-08-m2-sanitisation.md`;
M1 walking-skeleton spec at `docs/specs/2026-05-08-m1-walking-skeleton.md`).

## Commands

Requires Go 1.25+, pnpm, Make.

```bash
make dev      # Go on :8080 + Vite on :5173 — open http://localhost:5173
make build    # static binary at bin/tap (rebuilds web/dist if stale)
make docker   # distroless image
make test     # go test ./... -race  (also rebuilds web/dist)
make clean
```

Frontend-only:

```bash
pnpm --dir web dev          # Vite dev server alone (Go must run separately)
pnpm --dir web test         # vitest unit tests
pnpm --dir web run check    # svelte-check / TypeScript check
pnpm --dir web build        # produces web/dist consumed by go:embed
```

Single-test invocations:

```bash
go test ./internal/poll -run TestSchedulerTick -race
pnpm --dir web test -- src/lib/__tests__/store.test.ts
```

## Build coupling

`web/dist` must exist before `go build` — the `web/embed.go` `//go:embed all:dist` directive fails the Go build otherwise. The Makefile chains this via the `web/dist/index.html` target. **`go test ./...` also depends on `web/dist`** because `internal/server.SPAHandler` validates `index.html` is present at construction time; run via `make test` or rebuild the SPA first.

The embed directive lives in `web/embed.go` (package `web`) rather than under `internal/server/` because `go:embed` paths cannot escape their package directory.

## Architecture

Single-process server with four concerns living alongside each other:

1. **HTTP server** (`internal/server`) — owns lifecycle and the embedded SPA fallback. The SPA handler serves `web/dist` and falls back to `index.html` for unknown paths so client-side routing works for deep links. There is no dev-mode branch in the Go server: in dev, you visit Vite on :5173 and Vite proxies `/api` + `/healthz` to Go on :8080 (config in `web/vite.config.ts`).
2. **Polling pipeline** (`internal/poll`) — `Scheduler` ticks every 60s, picks due subscriptions via `db.ListDuePolls`, and dispatches to a fixed worker pool. Each `Worker.Run` does fetch → parse → commit in one transaction. `Scheduler.Poke()` is wired into `POST /api/v1/subscriptions` so a freshly added feed polls within seconds, not up to `TickInterval`.
3. **REST API** (`internal/api`) — `NewMux(db, poke)` wires `/api/v1/subscriptions` and `/api/v1/entries` plus `/healthz`. `cmd/tap/main.go` mounts the same mux under both `/api/` and `/healthz`.
4. **Archival ticker** (`internal/archival`) — `Archiver` ticks daily (configurable), deletes read-and-unsaved entries older than `--archive-horizon`, writes tombstones atomically, and evicts proxy cache files older than `--cache-age-cap`. Starts after migrations, stops before the scheduler on shutdown.

**Database is the queue.** There is no separate queue table or in-memory queue. `subscriptions.next_poll_at` (unix seconds) is the schedule; new subscriptions insert with `next_poll_at = 0` so the next tick picks them up. The `(subscription_id, hash)` UNIQUE constraint dedupes re-fetched entries silently.

**Pure-Go SQLite.** `modernc.org/sqlite` (not `mattn/go-sqlite3`) is mandatory — `CGO_ENABLED=0` everywhere keeps the binary distroless-compatible. Don't introduce any CGO dependency. In-memory DBs (`":memory:"`) are pinned to one connection in `db.Open` because SQLite memory DBs are per-connection.

**Migrations** live in `internal/db/migrations/NNNN_*.sql`, embedded via `embed.FS`, applied in lexical order by `db.Migrate`. Each migration runs in its own transaction; `schema_migrations` tracks applied versions.

**Shutdown ordering** in `Scheduler.Stop` is load-bearing: cancel parent ctx → wait for tickLoop → close jobs channel → drain workers. Reordering races with `Tick`'s send into `s.jobs`. `cmd/tap/main.go` first calls `srv.Shutdown` (drains HTTP) and then `sched.Stop`.

## Conventions

- **TDD is non-negotiable** per `docs/roadmap.md` — red/green/refactor on every behaviour-bearing change. Pure scaffolding (configs, CSS tokens) is exempt; anything with branches, error handling, or state is in scope. If a milestone's spec needs to skip TDD on a function, flag it in the spec.
- **Specs are committed; implementation plans are not.** Specs live in `docs/specs/YYYY-MM-DD-<slug>.md` (per the user's standing instruction in their global CLAUDE.md). Plans are working documents that get discarded after implementation.
- **Frontend stack:** Svelte 5 + TypeScript + Vite, **no SvelteKit** (the SPA is embedded in the Go binary, so SvelteKit's SSR adds nothing). Stores live in `web/src/lib/store.ts`; do not introduce a router framework — `web/src/lib/router.ts` is hand-rolled.
- **API DTOs are explicit.** `internal/api/*.go` defines per-endpoint DTOs and converts from `db.*` rows; do not return `db` types directly. Errors flow through `writeError(w, status, code, message)` from `internal/api/errors.go` so codes are stable.
- **Request bodies are size-capped** (`http.MaxBytesReader`, 1 MiB) on every write handler.

## Trust posture (relevant when reasoning about defaults)

Feed HTML is sanitised on the server before storage by `internal/sanitise.Policy.Sanitise` (M2 — bluemonday-based allowlist + `golang.org/x/net/html` post-pass for iframe-host allowlisting, pixel-tracker drop, URL tracking-param stripping). The SPA renders the stored HTML directly without a runtime sanitiser, so anything that bypasses the worker's `policy.Sanitise(content)` call lands raw in the DB and gets rendered.

The shared HTTP client is constructed via `httpx.NewClient(opts)` (M4) and is used by both polling and the media proxy. SSRF is enforced by `internal/httpx/ssrf.go` (`SSRFPolicy.AllowAddr` for the dialer, `CheckRedirect` for redirects re-checked independently from the initial URL). Per-host concurrency is capped at 4 by default in `internal/httpx/hostlimit.go`. The polling cadence is adaptive (`internal/cadence/`), driven by the `velocity_24h_x100` column on `subscriptions`: floor 15min, ceiling 24h, with origin-mandated `Retry-After` and `Cache-Control: max-age` honoured as floors. Error backoff is exponential (5m × 2^(n-1), capped 24h, 25% jitter).

Subscriptions opted into M5 article extraction (`extract = true`) fetch each new entry's article URL via the shared client (SSRF, per-host cap, `--http-timeout` all apply). Readability mode by default; per-feed `extract_selector` CSS override available via PATCH /api/v1/subscriptions/:id (validated through `cascadia.Compile` at write time). Extracted HTML flows through `processor.Process` so M2 sanitisation and M3 image proxying still apply. Per-entry failures set `entries.extract_failed = 1` and degrade to the feed-provided summary; they never abort the poll. Bounded by `--extract-concurrency` (default 4) inside each feed worker.

M6 introduces user accounts (argon2id passwords, `users` + `sessions`
tables) and the session/CSRF middleware on every `/api/v1/*` route except
`POST /sessions` and `/healthz`. Session cookies are `HttpOnly`,
`SameSite=Lax`, and conditionally `Secure` (auto-resolves against the
listen address; explicit override via `--cookie-secure`). CSRF tokens
live on the `sessions` row, return in the login JSON, and rotate only
on password change.

Per-feed credentials (`subscriptions.cookie`, `basic_auth_user`,
`basic_auth_pass`) are accepted on POST/PATCH `/api/v1/subscriptions`
but never returned on GET — the read DTO exposes only `has_cookie` /
`has_basic_auth` booleans. Layered onto outbound requests via
`internal/httpx.ApplyFeedCreds` for feed polling and article extraction
only — the media proxy (`internal/proxy/handler.go`) deliberately
fetches origins anonymously, mirroring Miniflux. Same-origin
authenticated images render broken; this is a known cross-ecosystem
limitation, not a Tap-specific deficiency.

Admin bootstrap is out-of-band (concept §7.1): either
`TAP_ADMIN_USERNAME`/`TAP_ADMIN_PASSWORD` on first launch (silent on
populated DBs) or `tap admin create` interactively from the host.
`tap admin passwd <username>` resets a forgotten password and
force-logs-out that user's active sessions. There is no SPA-visible
bootstrap path.

**Multi-user data isolation is M7 scope.** All subscription/entry queries
are scoped by `user_id`; strict per-user privacy with no admin override
on feed visibility.

**Archival + tombstones (M11).** A daily sweep deletes read-and-unsaved
entries older than `--archive-horizon` (default 90d), recording tombstones
so re-published entries do not resurface as unread. A second daily pass
unlinks proxy cache files older than `--cache-age-cap` (default 14d). Both
bounds are configurable via flags or environment variables. The tombstone
table (`tombstones`) is small and grows slowly; tombstones are permanent by
design — the dedup guarantee requires durability. The archival sweep is the
fourth concurrent concern alongside the HTTP server, polling pipeline, and REST API; it
starts after migrations and stops cleanly on shutdown.

**Upgrade note (migration 0010):** adds the `tombstones` table. Existing
databases migrate cleanly. Entries already in the database are subject to
archival on the next sweep if they meet the horizon criterion.

Defence-in-depth defaults that should not be weakened lightly:

- The binary still defaults to `-addr 127.0.0.1:8080` (concept §6.11). Container binds `0.0.0.0:8080` because the network namespace is the boundary there.
- `internal/server.SPAHandler` validates `web/dist/index.html` exists at construction time. Removing that turns a missing-bundle bug into silent 404s on every route.
- `WorkerOpts.Policy` is required (`NewWorker` panics on nil); `SchedulerOpts.Policy` defaults to `sanitise.DefaultPolicy()`. Don't reintroduce a nil-default in the worker — it would silently render unsanitised HTML.
- Per-feed `<iframe>` host allowlist override is a deferred-items entry on the roadmap (post-M6); the current set is hard-coded in `internal/sanitise/sanitise.go`.
- Concept §6.4's strict serialisation default (`PerHostInflight=1`) is intentionally relaxed to 4 in M4 — see the "Risks" section of `docs/specs/2026-05-09-m4-polling-discipline.md`. Don't drop it back to 1 without a corresponding plan-level discussion.
- `--ssrf-disabled` exists as an escape hatch for fully-trusted networks but should not be set in default deployments; the binary logs a startup WARN when it is on.
