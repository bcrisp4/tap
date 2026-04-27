# Tap web (SvelteKit SPA)

Single-page client for the Tap feed reader. Svelte 5 + SvelteKit
with `@sveltejs/adapter-static`; built into a static directory that
the Go binary embeds via `//go:embed` and serves at `/`.

## Stack

- **Svelte 5** (runes mode) + **SvelteKit** with `adapter-static`,
  `fallback: 'index.html'` for SPA routing.
- **TanStack Svelte Query 6** + IndexedDB persister (`idb`) — see
  `src/lib/query-client.ts`.
- **Typed fetch client** that unwraps the design.md §6 API envelope
  into `ApiError` on failure — see `src/lib/api/client.ts`.
- **Theme runes** with light / sepia / dark + serif/sans toggle
  persisted in `localStorage` — see `src/lib/theme.svelte.ts`.
- **Vitest** for unit tests under `src/`, **Playwright** for end-to-end
  tests under `tests/`.

Requires Node ≥ 20.10. Run `npm ci` once after cloning.

## Develop

```sh
# Terminal 1: start the Go API
cd .. && make run            # listens on 127.0.0.1:8080

# Terminal 2: start the SPA dev server
npm run dev                  # listens on 127.0.0.1:5173
```

Vite proxies `/api/*` to the Go server (see `vite.config.ts`), so
client-side fetches to `/api/v1/...` work without CORS.

## Build → embed → run

The Go binary embeds the SvelteKit build via `//go:embed all:build`
inside `internal/web/embed_with_spa.go`. Embed paths are relative to
the package, so `make build` stages `web/build/` →
`internal/web/build/` before invoking `go build -tags embed_spa`.

```sh
cd .. && make build          # full pipeline → ./tap
TAP_DB_PATH=/tmp/tap.db ./tap serve
```

The default `make test` skips the embed tag so a fresh checkout works
without running `npm run build`. Use `make test-all` to exercise the
embedded-SPA Go tests after building the SPA.

## Test

```sh
npm run check                # svelte-check + tsc
npm test                     # vitest (src/**/*.test.ts)
npm run e2e                  # Playwright against npm run dev
```

The Playwright suite has two run modes — when `TAP_E2E_BASE` is set,
it skips spinning up `npm run dev` and hits the URL you provide
directly. Use this to test the embedded production binary:

```sh
cd .. && make build && TAP_DB_PATH=/tmp/tap-e2e.db ./tap serve &
TAP_E2E_BASE=http://127.0.0.1:8080 npm run e2e
```

## Layout

```
src/
├── app.html                  template (loads tokens, sets initial theme class)
├── app.css                   global resets + token import
├── lib/
│   ├── api/                  typed client + TanStack Query factories
│   ├── brand/                Wordmark / TJunction Svelte components
│   ├── tokens/tap-tokens.css design-system tokens (ported from docs/ui_design_handoff)
│   ├── theme.svelte.ts       persisted theme + font runes
│   └── query-client.ts       QueryClient + IndexedDB persister
├── routes/
│   ├── +layout.svelte        QueryClientProvider + theme apply
│   ├── +layout.ts            disables SSR, forces CSR
│   └── +page.svelte          placeholder; replaced in Plan 10
└── static/                   favicon, manifest, robots.txt
tests/                        Playwright e2e specs
```

See `docs/design.md` (one level up) for the full product spec and
`docs/superpowers/plans/README.md` for the plan series this SPA is
part of.
