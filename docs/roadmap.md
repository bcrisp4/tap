# Tap — Roadmap

This document is the index of milestones for building Tap. Each milestone is small enough to design, plan, implement, review, and ship as one coherent unit. Each gets its own design spec in `docs/specs/` before implementation begins.

The shape is **walking skeleton + iterative thickening**: M1 is the thinnest possible end-to-end Tap that proves the architecture; subsequent milestones add capability layers without restructuring the core.

The full product concept lives in `docs/concept.md`. The visual identity and the detailed design for the Unread and Reader views live in `ui_design/`.

## Foundational technology choices

These are locked in before M1 begins.

| Concern | Choice | Rationale |
|---|---|---|
| Backend language | Go | Mandated in spirit by the static-binary, no-libc, cross-compile constraints. |
| Database | SQLite via `modernc.org/sqlite` | Embedded, pure-Go (no CGO), keeps the static-binary story intact. FTS5 ships in-tree. |
| Frontend | Svelte 5 + TypeScript + Vite, no SvelteKit | The SPA is embedded in the Go binary, so SvelteKit's SSR and server endpoints add nothing. Plain Svelte + Vite is the smaller, simpler fit. |
| Repo shape | Monorepo. Go module at root, Svelte project in `web/`, built SPA embedded via `embed.FS`. | One binary, one volume, one port — same shape as the deployment. |
| Container base | `gcr.io/distroless/static-debian12:nonroot` | No shell, no libc, non-root by default. |

## Milestones

| # | Milestone | One-line goal | Spec |
|---|---|---|---|
| M1 | Walking skeleton | Run the binary, subscribe to a feed, see entries appear, read one. No safety, no polish. | [`specs/2026-05-08-m1-walking-skeleton.md`](specs/2026-05-08-m1-walking-skeleton.md) |
| M2 | Sanitisation pipeline | Allowlist-based HTML cleaning and image-URL rewriting hooks so feed content is safe to render. | [`specs/2026-05-08-m2-sanitisation.md`](specs/2026-05-08-m2-sanitisation.md) |
| M3 | Media proxy + cache | `/api/v1/proxy/{token}` with signed tokens, sharded FS cache, MIME allowlist, request coalescing. | [`specs/2026-05-09-m3-media-proxy.md`](specs/2026-05-09-m3-media-proxy.md) |
| M4 | Polling discipline | Adaptive cadence, per-host concurrency cap, SSRF guard with allowlist + redirect re-check, retry/backoff. | [`specs/2026-05-09-m4-polling-discipline.md`](specs/2026-05-09-m4-polling-discipline.md) |
| M5 | Article extraction | Readability-style extractor + per-feed CSS rules, opt-in flag, graceful degradation. | TBD |
| M6 | Auth foundations | Password login, sessions (cookie, hashed-at-rest, idle + absolute expiry), CSRF, admin bootstrap (CLI + env-var), credential redaction. | TBD |
| M7 | 2FA + passkeys | TOTP enrolment, recovery codes, WebAuthn, session listing/revocation, admin reset paths. | TBD |
| M8 | SPA polish | Three themes, serif/sans toggle, density toggle, keyboard shortcuts, mobile breakpoints, swipe gestures, animations, accessibility pass. | TBD |
| M9 | Categories, OPML, search, add-feed flow | The organisational and ingest UX. SQLite FTS5 for full-text search. Discover-feeds-from-page-URL. | TBD |
| M10 | Offline + PWA | Service worker, mutation queue persisted to local storage, warm-cache driver, manifest, status-bar theming. | TBD |
| M11 | Archival + tombstones | Daily sweep, tombstone consult on insert, two-pass media cache eviction. | TBD |
| M12 | Observability + production hardening | Structured logs, OTel metrics + traces, healthcheck subcommand, admin CLI, recent-errors ring buffer, brute-force lockout, system-status panel, security review pass. | TBD |

## Deferred items (post-M6)

Items committed in the design but not yet sequenced into a specific milestone. They depend on prerequisites (typically the user table from M6) and will be slotted into a milestone — or get their own — once that landscape is clearer.

| Item | Depends on | Notes |
|---|---|---|
| Per-user iframe-host allowlist | M6 user table | M2 ships a hard-coded default sourced from miniflux's `iframeAllowList` (13 hosts including `youtube.com`, `player.vimeo.com`, `bandcamp.com`, etc. — see `internal/sanitise/sanitise.go`). Per-user override stored in DB-backed preferences once the user table exists. |
| SVG support in the media proxy | None (own decision) | M3 ships an `image/{png,jpeg,gif,webp,avif}` MIME allowlist and 415s SVG. No well-trodden pure-Go SVG sanitiser exists; two real options when we revisit: (a) roll our own XML allowlist walker mirroring M2's HTML post-pass (~200 LoC, edge cases around namespaces / SMIL / CSS in style attrs), or (b) rasterise SVG → PNG inside the proxy via `srwiley/oksvg` or similar (heavier dep, loses scalability). Likely a small follow-up extension to `internal/sanitise` rather than its own milestone. |

## Working cadence

For each milestone:

1. **Brainstorm and write the spec** (this is what `superpowers:brainstorming` produces). The spec lives in `docs/specs/YYYY-MM-DD-<slug>.md` and gets committed.
2. **Write the implementation plan** (this is what `superpowers:writing-plans` produces). The plan is *not* committed per the user's standing instruction.
3. **Implement test-first.** Every behaviour-bearing change follows the red–green–refactor cycle:
   - **RED** — write the failing test first; run it and confirm it fails for the expected reason. A test that doesn't fail in red doesn't exercise real code.
   - **GREEN** — write the minimum implementation that turns the test green. No speculative features, no "while you're in there" cleanup.
   - **REFACTOR** — clean up while green. Run the test after every refactor.

   Use the `superpowers:test-driven-development` skill on every implementation task. Pure scaffolding (project init, configs, CSS, design tokens) is exempt; anything with logic, branches, error handling, or state is in scope. **The discipline is non-negotiable.**
4. **Review and merge.** Each milestone ships as a coherent change before the next milestone's spec is written.

If a milestone turns out to be too big once we drill into the spec, we split it in place rather than power through. Better to have M6a + M6b than a milestone that loses the thread.

## Why test-first across all milestones

Two reasons specific to this project:

- **Tap touches network, disk, and a database in every milestone.** Bugs in those regions are expensive to reproduce by hand. A failing test you wrote first is a reproducer you won't lose.
- **Each milestone changes architecture-shaped code** (the polling pipeline in M4, the auth flow in M6, the offline queue in M10). Without tests written before the implementation, you can't tell whether a later milestone's changes regressed an earlier one. The test suite is the only durable record of "this used to work."

If a milestone is going to skip TDD on a particular function, that's a decision that needs to be flagged in the milestone's spec and justified — never an accident.
