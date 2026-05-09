# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Tap is a self-hosted RSS / Atom / JSON Feed reader. It ships as **one static Go binary** with an embedded SQLite database, an embedded Svelte SPA, and no external services. See `docs/concept.md` for the full design and `docs/roadmap.md` for the milestone plan; **M2 in progress** (sanitisation pipeline complete, awaiting merge — spec at `docs/specs/2026-05-08-m2-sanitisation.md`; M1 walking-skeleton spec at `docs/specs/2026-05-08-m1-walking-skeleton.md`).

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

Single-process server with three concerns living alongside each other (M11 will add the archival ticker):

1. **HTTP server** (`internal/server`) — owns lifecycle and the embedded SPA fallback. The SPA handler serves `web/dist` and falls back to `index.html` for unknown paths so client-side routing works for deep links. There is no dev-mode branch in the Go server: in dev, you visit Vite on :5173 and Vite proxies `/api` + `/healthz` to Go on :8080 (config in `web/vite.config.ts`).
2. **Polling pipeline** (`internal/poll`) — `Scheduler` ticks every 60s, picks due subscriptions via `db.ListDuePolls`, and dispatches to a fixed worker pool. Each `Worker.Run` does fetch → parse → commit in one transaction. `Scheduler.Poke()` is wired into `POST /api/v1/subscriptions` so a freshly added feed polls within seconds, not up to `TickInterval`.
3. **REST API** (`internal/api`) — `NewMux(db, poke)` wires `/api/v1/subscriptions` and `/api/v1/entries` plus `/healthz`. `cmd/tap/main.go` mounts the same mux under both `/api/` and `/healthz`.

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

Defence-in-depth defaults that should not be weakened lightly:

- The binary still defaults to `-addr 127.0.0.1:8080` (concept §6.11). Container binds `0.0.0.0:8080` because the network namespace is the boundary there.
- `internal/server.SPAHandler` validates `web/dist/index.html` exists at construction time. Removing that turns a missing-bundle bug into silent 404s on every route.
- `WorkerOpts.Policy` is required (`NewWorker` panics on nil); `SchedulerOpts.Policy` defaults to `sanitise.DefaultPolicy()`. Don't reintroduce a nil-default in the worker — it would silently render unsanitised HTML.
- Per-feed `<iframe>` host allowlist override is a deferred-items entry on the roadmap (post-M6); the current set is hard-coded in `internal/sanitise/sanitise.go`.
