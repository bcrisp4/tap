# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Tap is a self-hosted RSS / Atom / JSON Feed reader. Single Go binary, single SQLite file, embedded SvelteKit SPA, real offline support (PWA).

- **Source-of-truth spec:** `docs/design.md` — read this first.
- **Visual design handoff:** `docs/ui_design_handoff/` — `styles.css` is the design-token contract; the JSX prototypes are reference, not code to import.
- **Module path:** `github.com/bcrisp4/tap`. Apache-2.0.

## Plan-driven workflow

Implementation is split across **19 sub-plans** (00–18; 14–18 were added post-v1 for the UX-cleanup wave) under `docs/superpowers/plans/` (gitignored — per-project working notes, not committed). The index at `docs/superpowers/plans/README.md` is the entry point: it shows the dependency graph and which plans are independently parallelisable.

- Execute plans with `superpowers:subagent-driven-development` (preferred) or `superpowers:executing-plans`.
- Each plan header lists the **recommended skills** to invoke before starting (`cc-skills-golang:*` for Go work, `svelte:*` + the `svelte:svelte-file-editor` agent for SvelteKit work).
- After tasks complete, the implementer subagent **must invoke the `simplify` skill** on its changes and commit any cleanups before requesting spec / code-quality review. This is non-negotiable per `docs/superpowers/plans/README.md` Conventions.
- **Plan files don't carry into fresh worktrees** (gitignored). After `git worktree add ...`, copy them in before dispatching: `mkdir -p .claude/worktrees/<branch>/docs/superpowers/plans && cp docs/superpowers/plans/<plan>.md docs/superpowers/plans/README.md .claude/worktrees/<branch>/docs/superpowers/plans/`.

## Worktrees

Worktrees live under `.claude/worktrees/<slug>` (gitignored — slug is the
branch name with the `feat/` / `chore/` / `docs/` prefix stripped, e.g.
branch `feat/tap-18-mobile-polish` → dir `.claude/worktrees/tap-18-mobile-polish`).
Create with:

```bash
git worktree add .claude/worktrees/<slug> -b feat/tap-NN-<slug>
```

Branches use `feat/tap-NN-<slug>` for plan branches and `docs/<slug>` or `chore/<slug>` for meta work. Worktrees stay until the corresponding PR merges; clean up via `git worktree remove .claude/worktrees/<slug>`.

**`cd` out of the worktree before `git worktree remove`** — running it from inside leaves CWD in a deleted dir and every subsequent shell call errors with `getcwd: cannot access parent directories`.

## Agent dispatch + PR review

- Lead agents that run inline `sleep` polls for Copilot review often exit prematurely with truncated "Waiting for Copilot..." summaries. The implementation work is usually already done — dispatch a separate follow-up agent dedicated to the Copilot review loop after the lead reports.
- Even dedicated polling subagents may yield back after arming `Monitor`. The most reliable polling path is a single `Bash` `until` loop with `timeout: 600000` and `sleep 30` between checks, run from the lead session.
- Detect Copilot review via `gh api repos/<o>/<r>/pulls/<n>/reviews` and `.../pulls/<n>/comments`, filtering author by regex `[Cc]opilot` (login: `copilot-pull-request-reviewer[bot]`). The PR's `reviewRequests` field is unreliable — it can empty out within seconds of the request even while the review is still in flight.
- Reply on Copilot's inline comments **in-thread**, not as a top-level PR comment: `gh api -X POST repos/<o>/<r>/pulls/<n>/comments/<id>/replies --input - <<<'{"body":"..."}'`.
- Bundle Copilot fix commits by logical theme (one commit covering related comments on the same file/concern), not one commit per comment — matches the existing convention in this repo's history.
- Nested `Agent` / `Task` tool dispatch is often unavailable in subagent envs. Agents should `ToolSearch query: "select:Agent" max_results: 1` first; if not exposed, fall back to inline TDD.
- PR# ≠ Plan# under parallel dispatch. Confirm the mapping via `gh pr list --head feat/tap-NN-...` before issuing comment-fix calls against a PR number.
- GitHub auto-requests Copilot on PR creation. **Don't dismiss it trying to re-trigger** — once dismissed, the special `Copilot` reviewer can't be re-requested via API (POST returns 200 but reviewer not attached). For a fresh cycle, close + reopen the PR.
- `gh api` JSON-body POSTs use `--input - <<<'{...}'`, not `-F` flags. `-F reviewers='[...]'` returns 422 for arrays.
- Playwright MCP tools are deferred — load via `ToolSearch query: "select:mcp__plugin_playwright_playwright__browser_navigate,..." max_results: 10` before calling.

## Build / test / run

After Plan 00 lands, the standard targets are:

```bash
make build   # CGO_ENABLED=0, -trimpath, -ldflags injects version → ./tap
make test    # go test ./...
make run     # build then run
make tidy    # go mod tidy
make clean   # rm -f tap
```

Single-package test: `go test ./internal/version/...` (or any package path).
Single test: `go test -run TestRun ./cmd/tap/`.

The build is fully static (no CGo). Pure-Go SQLite via `modernc.org/sqlite` is the reason — do not introduce CGo dependencies.

## Repo rules

- **Pushes to `main` are blocked** — every change goes through a PR (GitHub branch rule). Push to a feature branch and `gh pr create`.
- **No `--no-verify`, no `--no-gpg-sign`, no `--amend`** on existing commits unless the user explicitly asks.
- **`docs/superpowers/plans/`, `.claude/worktrees/`, `.remember/`** are gitignored and must stay that way.
- The local-by-default bind is `127.0.0.1:8080`; the container build (Plan 13) overrides to `0.0.0.0:8080` via `TAP_LISTEN`.

## Tech stack pins (see design.md §3 for the full table)

- Go (latest stable; toolchain pinned in `go.mod`).
- `peterbourgon/ff/v4` + `ffyaml` + `ff.Command` — flag/env/YAML config layering, precedence flag > env > file > default. YAML keys are hyphenated and match flag long-names (e.g. `db-path`).
- `modernc.org/sqlite` (pure-Go), WAL + foreign keys + FTS5.
- `mmcdole/gofeed` for feed parsing.
- `codeberg.org/readeck/go-readability/v2` for article extraction.
- `microcosm-cc/bluemonday` + a Tap pre-pass for HTML sanitisation.
- Svelte 5 + `@sveltejs/adapter-static` for the SPA. `@tanstack/svelte-query` **v6** (runes API — consumers use `status.isLoading` direct, NOT `$status.isLoading`) + the `idb` persister for offline cache.

## Embed convention

The Go build embeds the SvelteKit static output via `go:embed`. The embed is split across two files using build tags so `go test ./...` works on a fresh checkout where `web/build/` doesn't exist yet:

- `internal/web/embed.go` (no tag) — empty FS + placeholder HTML.
- `internal/web/embed_with_spa.go` (`//go:build embed_spa`) — real `//go:embed all:build`.

`make build` adds `-tags embed_spa`. `make test` does not. Plan 09 wires this up.

## Response shape (HTTP API)

Per design.md §6:

- Lists: `{ "data": [...], "pagination": { "page": N, "per_page": M, "total": K } }`
- Singles: bare object.
- Errors: `{ "error": { "code": "...", "message": "..." } }` with appropriate HTTP status.

## Conventions

- TDD: failing test → run (must fail) → minimal impl → run (must pass) → commit. Mandatory in every plan.
- One commit per task with the exact commit message in the plan; never batch task commits.
- Do not commit `docs/superpowers/plans/`, `.claude/worktrees/`, secrets, build artefacts, or `node_modules`.
- **Credential redaction (Plan 08).** `storage.Feed` `Cookie`/`Username`/`Password`/`ProxyURL` are `json:"-"` on read; the API never returns them. SPA edit forms must **omit** empty credential inputs from PATCH bodies — never send empty strings (would clear stored creds).
