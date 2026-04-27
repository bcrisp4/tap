# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Tap is a self-hosted RSS / Atom / JSON Feed reader. Single Go binary, single SQLite file, embedded SvelteKit SPA, real offline support (PWA).

- **Source-of-truth spec:** `docs/design.md` — read this first.
- **Visual design handoff:** `docs/ui_design_handoff/` — `styles.css` is the design-token contract; the JSX prototypes are reference, not code to import.
- **Module path:** `github.com/bcrisp4/tap`. Apache-2.0.

## Plan-driven workflow

Implementation is split across **14 sub-plans** under `docs/superpowers/plans/` (gitignored — per-project working notes, not committed). The index at `docs/superpowers/plans/README.md` is the entry point: it shows the dependency graph (00 → 01 → 02 → … → 13) and which plans are independently parallelisable.

- Execute plans with `superpowers:subagent-driven-development` (preferred) or `superpowers:executing-plans`.
- Each plan header lists the **recommended skills** to invoke before starting (`cc-skills-golang:*` for Go work, `svelte:*` + the `svelte:svelte-file-editor` agent for SvelteKit work).
- After tasks complete, the implementer subagent **must invoke the `simplify` skill** on its changes and commit any cleanups before requesting spec / code-quality review. This is non-negotiable per `docs/superpowers/plans/README.md` Conventions.

## Worktrees

Worktrees live under `.claude/worktrees/<branch>` (gitignored). Create with:

```bash
git worktree add .claude/worktrees/<branch> -b <branch>
```

Branches use `feat/tap-NN-<slug>` for plan branches and `docs/<slug>` or `chore/<slug>` for meta work. Worktrees stay until the corresponding PR merges; clean up via `git worktree remove .claude/worktrees/<branch>`.

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
- `peterbourgon/ff/v4` + `ffyaml` + `ff.Command` — flag/env/YAML config layering, precedence flag > env > file > default. YAML keys are underscored.
- `modernc.org/sqlite` (pure-Go), WAL + foreign keys + FTS5.
- `mmcdole/gofeed` for feed parsing.
- `codeberg.org/readeck/go-readability/v2` for article extraction.
- `microcosm-cc/bluemonday` + a Tap pre-pass for HTML sanitisation.
- Svelte 5 + `@sveltejs/adapter-static` for the SPA. `@tanstack/svelte-query` + the `idb` persister for offline cache.

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
