# M1 Walking Skeleton Plan — Critical Review

Review of [`2026-05-08-m1-walking-skeleton.md`](2026-05-08-m1-walking-skeleton.md), 2026-05-08.

## Blockers

**1. `go:embed` path won't compile (Phase 6, Task 6.1, line 2855)**
`internal/server/spa.go` declares `//go:embed all:web/dist` — but `go:embed` paths are relative to the file's package directory, with no `..` allowed. From `internal/server/`, `web/dist` resolves to `internal/server/web/dist`, which doesn't exist. The plan never compiles.

Fix: put the embed in a Go file at the project root (e.g. a tiny `web/embed.go` in package `web` with `//go:embed all:dist`) and import that `embed.FS` from the server package.

**2. Test files use `*sql.DB` without importing `database/sql` (multiple tasks)**
`internal/db/subscriptions_test.go` (line 772), `internal/db/entries_test.go` (line 1084), `internal/poll/worker_test.go` (line 1806), `internal/api/subscriptions_test.go` (line 2385) all declare helpers returning `*sql.DB` but their import blocks omit `database/sql`. None will compile. Also `internal/api/entries_test.go` line 2629 uses `strconv.FormatInt` with no `strconv` import.

**3. Task 2.5 won't compile until Task 2.6 exists**
`internal/db/subscriptions.go` (line 887) defines `PollResult { NewEntries []NewEntry }`, but `NewEntry` is introduced in `internal/db/entries.go` in Task 2.6. The plan tells you to commit Task 2.5 (line 1059) before Task 2.6 — that commit is broken. Either combine 2.5 and 2.6 into one task or move `NewEntry` into a shared file in 2.5.

**4. Cursor pagination is wrong (Task 2.6, lines 1181–1238)**
`ORDER BY published_at DESC, id DESC` paired with `WHERE id < ?` only works if ID order matches published_at order, which is not guaranteed (backfills, re-imports, clock skew). Concrete failure: entries with `(pub=200, id=5)` and `(pub=100, id=20)`. After page 1 emits `id=5`, cursor=5, page 2 filters `id<5` and drops `id=20` entirely. Use a composite cursor `(published_at, id) < (?, ?)` or encode both in the cursor.

## Serious

**5. No HTTP client timeout (Task 4.2, line 1925; main.go line 3099)**
`http.DefaultClient` has no timeout. Per-call ctx in `workerLoop` (line 2150) caps at 60s, which helps, but you still have no `Transport` settings — no connect timeout, no `MaxIdleConnsPerHost`, no TLS handshake limit. Use `&http.Client{Timeout: 30s, Transport: &http.Transport{...}}`. A misbehaving feed server can otherwise tie up workers near the 60s ceiling for every fetch.

**6. No body size cap on feed fetch (Task 3.3, line 1603)**
`gofeed.NewParser().Parse(resp.Body)` reads the entire response. A 1 GB "feed" OOMs the worker. Wrap with `io.LimitReader(resp.Body, 10<<20)` before parsing. This is the single cheapest change to harden polling.

**7. Shutdown ctx doesn't propagate to in-flight workers (Task 4.3, line 2150)**
`workerLoop` builds `context.WithTimeout(context.Background(), 60s)` — derived from `Background()`, not the scheduler's ctx. SIGTERM cancels `main`'s ctx, but in-flight HTTP fetches continue up to 60s each. `sched.Stop()` then blocks on `wg.Wait()` for that whole duration. With 30s server shutdown + up to 60s scheduler shutdown = 90s tail. Derive the worker ctx from a scheduler-owned ctx that `Stop()` cancels.

**8. `Stop()` and `Start()` race on `s.jobs` (Task 4.3, lines 2158–2207)**
`Start()`'s tick loop runs in a goroutine; on shutdown, `Stop()` does `close(s.stop); close(s.jobs)`. There's no synchronization between "Start's goroutine has observed s.stop and exited" and `close(s.jobs)`. If a tick is mid-flight in `Tick()` (line 2190 `case s.jobs <- sub`), you panic on send to closed channel. Either gate `Tick()` against `s.stop`, or have the tick goroutine acknowledge exit (a `done` channel) before closing `s.jobs`.

**9. New subscription waits up to 60s for first poll (Task 6.3, lines 3099–3105)**
`sched.Start()` runs initial `Tick()` *before* the server is ready to accept POSTs. Next tick is 60s later. DoD-4 ("entries within 60s") is right at the edge — depends on `gofeed` parse + DB commit fitting in single-digit seconds. Either expose `sched.PokeTick()` and call it from the POST handler, or shorten the tick interval. The current design will frustrate the first-time-user moment that the manual smoke test verifies.

**10. Unsanitised `{@html}` in container that binds `0.0.0.0:8080` (Reader.svelte line 4195; Dockerfile line 4350)**
Plan justifies the unsanitised render by saying "server binds to loopback and you're the only feed-source." That's true only for `make build`, not for `make docker` — the entrypoint binds `0.0.0.0:8080`. Anyone who reverse-proxies this on Tailscale or LAN gets stored XSS via any malicious feed. Either: (a) inline `bluemonday`/`microcosm-cc/bluemonday` server-side now (one task, ~50 lines), or (b) make the M1 container refuse to bind non-loopback unless `--allow-unsafe-html` is set.

**11. Vite HMR through Go reverse proxy is unconfigured (Task 6.1, line 2870; Phase 1 vite.config.ts)**
`httputil.NewSingleHostReverseProxy` does pass the WebSocket Upgrade since Go 1.20+, but Vite's HMR client computes its WS URL from `import.meta.env`, defaulting to `localhost:5173`. When the page is loaded from `:8080`, the HMR client tries `ws://localhost:8080/?token=...` — and you haven't set `server.hmr` in `vite.config.ts`. Likely outcome: HMR silently fails in dev mode, hot reload doesn't work, and you waste an afternoon debugging it. Add `server.hmr.clientPort: 8080` (or run Vite directly and have *Vite* proxy to Go, which is the simpler path).

**12. CGO inconsistency between `make build` and `make docker` (Task 12.1 vs 12.2)**
Makefile line 4299 omits `CGO_ENABLED=0`; Dockerfile line 4340 sets it. Same source produces different binaries depending on path. `net` and `os/user` will pull glibc dependencies on a Linux host with cgo available — `make build` then produces a binary that only runs on machines with matching glibc, while `make docker` produces a true static binary. DoD-1 ("statically linked") will be path-dependent. Set `CGO_ENABLED=0` in Makefile too.

**13. `Wait()` is a busy-loop without an upper bound (Task 4.3, lines 2211–2215)**
Used only by tests, but: if a worker panics in a way that bypasses your recover (e.g., the `defer Release` is itself panicking), `Wait()` hangs forever. Tests then time out at the test framework level after minutes. Add a context with timeout, or signal completion via a channel rather than polling.

## Nits

**14. `healthz` doesn't check the DB (Task 5.2, line 2335)** — returns "ok" even if SQLite is gone. For M1 with one user this is fine; just calling it out.

**15. No request size limit (Phase 5)** — `json.NewDecoder(r.Body).Decode(...)` will happily consume MBs. `http.MaxBytesReader(w, r.Body, 1<<20)` is one line.

**16. No `/version` endpoint, no `-X main.version=` ldflag** — when DoD-3 fails on a deployed container, you won't know which build.

**17. Subscription detail endpoint is missing** — `GET /api/v1/subscriptions/{id}` doesn't exist, only list/post/delete. Probably fine for M1 but inconsistent with entries, where you have list/get/patch.

**18. `error_count` increments forever** (Task 4.2) — no cap, no exponential backoff. M1 says fixed cadence is fine, but 1000 error rows for one stale feed will appear in `last_error` log spam.

**19. The plan calls itself a "walking skeleton" but is 4515 lines and 13 phases** — Cockburn's walking skeleton is the smallest end-to-end vertical slice. Shipping the SPA tokens.css, FeedAvatar deterministic colour palette, and Reader divider styling in M1 is *not* skeleton. None of it is wrong, but pretending it's minimal makes scope creep harder to spot in M2.

**20. `DELETE /api/v1/subscriptions/{id}` doesn't return 404 when nothing matched** (Task 5.3, lines 2532–2543) — `db.DeleteSubscription` doesn't read RowsAffected. Idempotent delete is a defensible choice; just be intentional.

**21. Optimistic update rollback can't recover stale data (Task 9.4, store.ts line 3582)** — if the entry isn't in the store (user navigated direct to `/entry/N`), `prev` stays null and there's no rollback path. Edge case.

---

## Top 3 to fix before any code is written

1. **The `go:embed` path bug + the missing `database/sql` imports + `NewEntry` defined-after-used in Task 2.5.** None of the early backend tasks compile as written. Restructure the embed to live at the project root, fix every test-file import block, and either merge Tasks 2.5/2.6 or move `NewEntry` into 2.5.

2. **Cursor pagination correctness + HTTP client/body limits.** The cursor bug (Task 2.6) is silent data loss; the unbounded HTTP client + unbounded gofeed parse (Tasks 3.3, 4.2) is a one-malicious-feed-OOMs-the-app risk. Composite cursor + `http.Client{Timeout}` + `io.LimitReader`.

3. **Shutdown semantics: derive worker ctx from scheduler ctx, fix Stop/Tick race, and decide what happens when SIGTERM hits during a 30s feed fetch.** As written, shutdowns can hang up to 60s past the server's 30s shutdown budget, and the close-then-tick race is a panic waiting for a busy day.

## One question the author must answer first

**Is the `make docker` artifact intended to be exposed beyond loopback (Tailscale, reverse proxy, LAN, etc.) in M1?** The plan defers HTML sanitisation to M2 and justifies it with "loopback only," but the Dockerfile binds `0.0.0.0:8080`. If yes-to-exposure, sanitisation is a P0 for M1, not M2. If no, the Dockerfile should bind loopback by default and require an explicit flag to expose, so the deferred-security assumption is enforced rather than honour-system.
