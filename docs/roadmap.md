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
| M2 | Sanitisation pipeline | Allowlist-based HTML cleaning and image-URL rewriting hooks so feed content is safe to render. | TBD |
| M3 | Media proxy + cache | `/api/v1/proxy/{token}` with signed tokens, sharded FS cache, MIME allowlist, request coalescing. | TBD |
| M4 | Polling discipline | Adaptive cadence, per-host concurrency cap, SSRF guard with allowlist + redirect re-check, retry/backoff. | TBD |
| M5 | Article extraction | Readability-style extractor + per-feed CSS rules, opt-in flag, graceful degradation. | TBD |
| M6 | Auth foundations | Password login, sessions (cookie, hashed-at-rest, idle + absolute expiry), CSRF, admin bootstrap (CLI + env-var), credential redaction. | TBD |
| M7 | 2FA + passkeys | TOTP enrolment, recovery codes, WebAuthn, session listing/revocation, admin reset paths. | TBD |
| M8 | SPA polish | Three themes, serif/sans toggle, density toggle, keyboard shortcuts, mobile breakpoints, swipe gestures, animations, accessibility pass. | TBD |
| M9 | Categories, OPML, search, add-feed flow | The organisational and ingest UX. SQLite FTS5 for full-text search. Discover-feeds-from-page-URL. | TBD |
| M10 | Offline + PWA | Service worker, mutation queue persisted to local storage, warm-cache driver, manifest, status-bar theming. | TBD |
| M11 | Archival + tombstones | Daily sweep, tombstone consult on insert, two-pass media cache eviction. | TBD |
| M12 | Observability + production hardening | Structured logs, OTel metrics + traces, healthcheck subcommand, admin CLI, recent-errors ring buffer, brute-force lockout, system-status panel, security review pass. | TBD |

## Working cadence

For each milestone:

1. **Brainstorm and write the spec** (this is what `superpowers:brainstorming` produces). The spec lives in `docs/specs/YYYY-MM-DD-<slug>.md` and gets committed.
2. **Write the implementation plan** (this is what `superpowers:writing-plans` produces). The plan is *not* committed per the user's standing instruction.
3. **Implement**. Test-driven where the design is well-understood; exploratory where it isn't.
4. **Review and merge**. Each milestone ships as a coherent change before the next milestone's spec is written.

If a milestone turns out to be too big once we drill into the spec, we split it in place rather than power through. Better to have M6a + M6b than a milestone that loses the thread.
