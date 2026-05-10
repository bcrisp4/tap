# M10 — Offline + PWA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add offline-first reading and PWA installability to Tap: service worker with Workbox, localStorage mutation queue keyed by user ID, warm-cache driver, and a PWA manifest with maskable icons.

**Architecture:** `vite-plugin-pwa` in `injectManifest` mode compiles a hand-written `web/src/sw/sw.ts` TypeScript service worker that precaches the app shell, stale-while-revalidates entry-list endpoints, and cache-firsts proxy URLs — all in per-user Workbox caches (`tap-api-{userId}`, `tap-proxy-{userId}`). The mutation queue lives in `localStorage` keyed by user ID, survives full reloads, and drains serially on boot and on reconnect. If drain encounters `401 invalid_session`, it pauses and resumes after re-login.

**Tech Stack:** Svelte 5, TypeScript, Vite 6, vite-plugin-pwa, workbox-precaching, workbox-routing, workbox-strategies, workbox-expiration, vitest, Playwright (E2E offline scenarios). No new Go code.

**Cross-milestone dependencies:**
- M7's auth surface (`/api/v1/sessions/current` returning `csrf_token`) is used by drain's `403 csrf_invalid` recovery path.
- M8's mobile bottom tab bar occupies the bottom of the viewport; the SW update banner must be `position: fixed; top: 0`.
- `background_color` in the manifest (`#fafaf7`) must be confirmed against M8's `--bg` token before shipping — m8-author has confirmed it matches.

---

## Skills and tools to apply

Always-on for every code-touching task:

- **`superpowers:test-driven-development`** — red/green/refactor on every behaviour-bearing change. Mandated by `docs/roadmap.md` §"Working cadence". Pure scaffolding (Vite config additions, icon files, manifest JSON, the `workerTypes.ts` type file) is exempt; everything with branches, error handling, or state is in scope.
- **`superpowers:verification-before-completion`** — before marking a task done, actually run the test command listed in the task's verification step and confirm output matches expectations.

Reach for as needed:

- **`svelte-runes`** — Svelte 5 runes (`$state`, `$derived`, `$effect`) in `App.svelte`. The `needRefresh` store from `vite-plugin-pwa` integrates with Svelte stores; subscribe with `$needRefresh`. The repo is already Svelte 5 — check existing components for established style.
- **`svelte-deployment`** — `pwa-setup.md` reference for `vite-plugin-pwa` `injectManifest` mode, Workbox strategy imports, and SW TypeScript patterns. Reach for it again if the Vite config or SW compilation fails.
- **`svelte-components`** — for the SW update banner component pattern in `App.svelte`. Match the existing component ergonomics.
- **`svelte-styling`** — for the `sw-update-banner` CSS (scoped styles, CSS custom property `--accent`). The banner uses `position: fixed; top: 0` to avoid collision with M8's mobile bottom tab bar.

MCP tools:

- **`context7` (`mcp__plugin_context7_context7__query-docs`)** — already used in this session to verify the `vite-plugin-pwa` injectManifest API (library ID `/vite-pwa/vite-plugin-pwa`). Reach for it again if the Workbox strategy API (`CacheFirst`, `StaleWhileRevalidate`, `ExpirationPlugin`) or the `useRegisterSW` API changes between sessions. Also useful for `workbox-expiration` `ExpirationPlugin` constructor options.
- **`mcp__plugin_playwright_playwright__*`** — for running and debugging the three offline E2E scenarios in Task 10. Use `browser_navigate`, `browser_evaluate` (for localStorage inspection), and `browser_wait_for` rather than fixed `waitForTimeout` where possible.

---

## File Map

| File | Status | Responsibility |
|---|---|---|
| `web/package.json` | Modify | Add vite-plugin-pwa + Workbox deps |
| `web/vite.config.ts` | Modify | Register VitePWA plugin, injectManifest config, manifest |
| `web/src/sw/swPatterns.ts` | Create | URL-matching helpers shared between `sw.ts` and tests |
| `web/src/sw/sw.ts` | Create | Hand-written service worker |
| `web/src/sw/workerTypes.ts` | Create | `SWMessage` type shared between SW and app |
| `web/src/lib/offlineQueue.ts` | Create | localStorage mutation queue |
| `web/src/lib/warmCache.ts` | Create | Boot/reconnect warm-cache driver |
| `web/src/lib/auth.ts` | Modify | SW postMessage on bootstrap/logout; clear queue on logout |
| `web/src/lib/api.ts` | Modify | Enqueue mutations on network error / offline |
| `web/src/App.svelte` | Modify | SW update banner, drain + warm-cache at boot and on `online` |
| `web/public/icons/icon-192.png` | Create | Maskable PWA icon (192×192) |
| `web/public/icons/icon-512.png` | Create | Maskable PWA icon (512×512) |
| `web/src/lib/__tests__/offlineQueue.test.ts` | Create | Unit tests for queue |
| `web/src/lib/__tests__/warmCache.test.ts` | Create | Unit tests for warm-cache driver |
| `web/src/sw/__tests__/sw.test.ts` | Create | SW strategy / URL-pattern tests |

---

### Task 1: Install dependencies and configure vite-plugin-pwa

**Invoke:** `svelte-deployment` skill for PWA setup context (already loaded).

**Files:**
- Modify: `web/package.json`
- Modify: `web/vite.config.ts`
- Create: `web/src/sw/workerTypes.ts`

- [ ] **Step 1: Install packages**

```bash
pnpm --dir web add -D vite-plugin-pwa workbox-precaching workbox-routing workbox-strategies workbox-expiration
```

Expected: packages appear in `web/package.json` devDependencies.

- [ ] **Step 2: Add workerTypes.ts**

Create `web/src/sw/workerTypes.ts`:

```ts
export type SWMessage =
  | { type: 'set-user'; userId: number }
  | { type: 'logout'; userId: number };
```

- [ ] **Step 3: Update vite.config.ts**

Replace the content of `web/vite.config.ts` with:

```ts
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { VitePWA } from 'vite-plugin-pwa';

const GO_BACKEND = 'http://127.0.0.1:8080';

export default defineConfig({
  plugins: [
    svelte(),
    VitePWA({
      strategies: 'injectManifest',
      srcDir: 'src/sw',
      filename: 'sw.ts',
      registerType: 'prompt',
      injectManifest: {
        injectionPoint: 'self.__WB_MANIFEST',
        minify: true,
        sourcemap: false,
        rollupFormat: 'es',
        buildPlugins: { vite: [], rollup: [] },
      },
      manifest: {
        name: 'Tap',
        short_name: 'Tap',
        description: 'Self-hosted feed reader',
        display: 'standalone',
        start_url: '/',
        scope: '/',
        background_color: '#fafaf7',
        theme_color: '#002FA7',
        icons: [
          {
            src: '/icons/icon-192.png',
            sizes: '192x192',
            type: 'image/png',
            purpose: 'maskable',
          },
          {
            src: '/icons/icon-512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      },
    }),
  ],
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

- [ ] **Step 4: Verify TypeScript check passes**

```bash
pnpm --dir web run check
```

Expected: zero errors.

- [ ] **Step 5: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml web/vite.config.ts web/src/sw/workerTypes.ts
git commit -m "M10: install vite-plugin-pwa, configure injectManifest, add SWMessage types"
```

---

### Task 2: Add PWA icons

**Files:**
- Create: `web/public/icons/icon-192.png`
- Create: `web/public/icons/icon-512.png`

The icons use the junction-dot motif from concept §12: a lowercase `tap` wordmark in geometric sans-serif with a filled junction dot. Both must be maskable (subject centered in the inner 80% safe zone so Android adaptive icons crop correctly).

- [ ] **Step 1: Create the icons directory**

```bash
mkdir -p /home/ben.guest/Users/ben/src/tap/web/public/icons
```

- [ ] **Step 2: Generate placeholder icons**

For now generate minimal valid PNGs so the build doesn't fail. These will be replaced with designed icons before M10 ships. Use any PNG editor, or generate programmatically:

```bash
# Requires ImageMagick — if not available, provide the two PNGs manually.
# Klein Blue (#002FA7) background with white junction-dot motif centred.
convert -size 192x192 xc:'#002FA7' \
  -fill white -draw "circle 96,96 96,56" \
  -fill '#002FA7' -draw "circle 96,96 96,66" \
  /home/ben.guest/Users/ben/src/tap/web/public/icons/icon-192.png

convert -size 512x512 xc:'#002FA7' \
  -fill white -draw "circle 256,256 256,150" \
  -fill '#002FA7' -draw "circle 256,256 256,175" \
  /home/ben.guest/Users/ben/src/tap/web/public/icons/icon-512.png
```

If ImageMagick is unavailable, create the two files by any means — a 192×192 and 512×512 PNG with any content. The PWA build only requires the files to exist; the final artwork is a design task.

- [ ] **Step 3: Verify build includes icons**

```bash
pnpm --dir web build
ls web/dist/icons/
```

Expected: `icon-192.png` and `icon-512.png` present. Also verify `web/dist/manifest.webmanifest` exists.

- [ ] **Step 4: Commit**

```bash
git add web/public/icons/
git commit -m "M10: add placeholder PWA icons (192, 512 maskable)"
```

---

### Task 3: Write the service worker (RED then GREEN)

**Invoke:** `superpowers:test-driven-development` skill at start of this task.

**Files:**
- Create: `web/src/sw/swPatterns.ts`
- Create: `web/src/sw/sw.ts`
- Create: `web/src/sw/__tests__/sw.test.ts`

The service worker tests verify URL-pattern matching and cache-naming configuration in isolation. Because the SW runs in a different global context, the URL patterns are extracted into a shared `web/src/sw/swPatterns.ts` module imported by both `sw.ts` and the test — this makes the tests cover the real patterns rather than inline copies.

- [ ] **Step 1: Create `swPatterns.ts` and write the failing strategy test**

Create `web/src/sw/swPatterns.ts` (imported by both `sw.ts` and the test so tests cover real patterns):

```ts
export const PROXY_PATTERN = /^\/api\/v1\/proxy\//;
export const ENTRIES_PATTERN = /^\/api\/v1\/(entries|subscriptions)/;
export const EXCLUDED_PATTERNS = [
  /^\/api\/v1\/search/,
  /^\/api\/v1\/opml/,
  /^\/api\/v1\/sessions/,
];

export function matchesProxy(pathname: string) { return PROXY_PATTERN.test(pathname); }
export function matchesEntries(pathname: string) { return ENTRIES_PATTERN.test(pathname); }
export function matchesExcluded(pathname: string) { return EXCLUDED_PATTERNS.some(p => p.test(pathname)); }
export function apiCacheName(prefix: string, userId: number) { return `${prefix}-${userId}`; }
```

Create `web/src/sw/__tests__/sw.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import {
  matchesProxy, matchesEntries, matchesExcluded, apiCacheName,
} from '../swPatterns';

// Helper wraps pathname extraction so tests pass full URLs naturally.
const p = (url: string) => new URL(url, 'http://localhost').pathname;

describe('SW URL pattern matching', () => {
  it('proxy pattern matches proxy URLs', () => {
    expect(matchesProxy(p('/api/v1/proxy/abc123.xyz456'))).toBe(true);
  });
  it('proxy pattern does not match entry URLs', () => {
    expect(matchesProxy(p('/api/v1/entries?unread=1'))).toBe(false);
  });
  it('entries pattern matches entries and subscriptions', () => {
    expect(matchesEntries(p('/api/v1/entries?unread=1'))).toBe(true);
    expect(matchesEntries(p('/api/v1/subscriptions'))).toBe(true);
  });
  it('entries pattern does not match proxy URLs', () => {
    expect(matchesEntries(p('/api/v1/proxy/abc'))).toBe(false);
  });
  it('search is excluded', () => {
    expect(matchesExcluded(p('/api/v1/search?q=hello'))).toBe(true);
  });
  it('opml is excluded', () => {
    expect(matchesExcluded(p('/api/v1/opml'))).toBe(true);
  });
  it('sessions is excluded', () => {
    expect(matchesExcluded(p('/api/v1/sessions/current'))).toBe(true);
  });
  it('entries is not excluded', () => {
    expect(matchesExcluded(p('/api/v1/entries'))).toBe(false);
  });
});

describe('SW cache naming', () => {
  it('api cache name includes user ID', () => {
    expect(apiCacheName('tap-api', 42)).toBe('tap-api-42');
  });
  it('proxy cache name includes user ID', () => {
    expect(apiCacheName('tap-proxy', 42)).toBe('tap-proxy-42');
  });
  it('different users get different cache names', () => {
    expect(apiCacheName('tap-api', 1)).not.toBe(apiCacheName('tap-api', 2));
  });
});
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
pnpm --dir web test -- src/sw/__tests__/sw.test.ts
```

Expected: FAIL — `sw/__tests__` not yet scanned or helpers undefined. That's expected at RED.

- [ ] **Step 3: Write the service worker**

Create `web/src/sw/sw.ts`:

```ts
/// <reference lib="webworker" />
import { precacheAndRoute } from 'workbox-precaching';
import { registerRoute } from 'workbox-routing';
import { CacheFirst, StaleWhileRevalidate } from 'workbox-strategies';
import { ExpirationPlugin } from 'workbox-expiration';
import type { SWMessage } from './workerTypes';
import { matchesProxy, matchesEntries, matchesExcluded, apiCacheName } from './swPatterns';

declare let self: ServiceWorkerGlobalScope;

// precacheAndRoute handles the app shell AND the navigation fallback to
// index.html for deep-linked routes — do NOT add a separate registerRoute
// for navigate requests, as that would shadow the precache handler.
precacheAndRoute(self.__WB_MANIFEST);

let userId: number | null = null;

// Per-user strategy instances, created lazily and reused across requests.
// Workbox ExpirationPlugin maintains internal IDB state — never instantiate
// per-request or expiration tracking won't accumulate correctly.
const proxyStrategies = new Map<number, CacheFirst>();
const apiStrategies = new Map<number, StaleWhileRevalidate>();

function getProxyStrategy(uid: number): CacheFirst {
  if (!proxyStrategies.has(uid)) {
    proxyStrategies.set(uid, new CacheFirst({
      cacheName: apiCacheName('tap-proxy', uid),
      plugins: [new ExpirationPlugin({ maxEntries: 500, maxAgeSeconds: 30 * 24 * 60 * 60 })],
    }));
  }
  return proxyStrategies.get(uid)!;
}

function getApiStrategy(uid: number): StaleWhileRevalidate {
  if (!apiStrategies.has(uid)) {
    apiStrategies.set(uid, new StaleWhileRevalidate({
      cacheName: apiCacheName('tap-api', uid),
      plugins: [new ExpirationPlugin({ maxEntries: 200, maxAgeSeconds: 7 * 24 * 60 * 60 })],
    }));
  }
  return apiStrategies.get(uid)!;
}

// Receive user context and logout signals from the app.
self.addEventListener('message', (event: MessageEvent<SWMessage | { type: string }>) => {
  const data = event.data;
  if (data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  } else if (data.type === 'set-user') {
    userId = (data as SWMessage & { type: 'set-user' }).userId;
  } else if (data.type === 'logout') {
    const uid = (data as SWMessage & { type: 'logout' }).userId;
    caches.delete(apiCacheName('tap-proxy', uid));
    caches.delete(apiCacheName('tap-api', uid));
    proxyStrategies.delete(uid);
    apiStrategies.delete(uid);
    userId = null;
  }
});

// Proxy URLs — cache-first, immutable (M3 sets Cache-Control: immutable).
registerRoute(
  ({ url }) => matchesProxy(url.pathname),
  ({ request, event }) => {
    if (userId === null) return fetch(request);
    return getProxyStrategy(userId).handle({ request, event: event as FetchEvent });
  },
);

// Entry list and subscription endpoints — stale-while-revalidate.
// Excluded: search, opml, sessions (volatile or auth-sensitive).
registerRoute(
  ({ url, request }) =>
    request.method === 'GET' &&
    !matchesExcluded(url.pathname) &&
    matchesEntries(url.pathname),
  ({ request, event }) => {
    if (userId === null) return fetch(request);
    return getApiStrategy(userId).handle({ request, event: event as FetchEvent });
  },
);
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
pnpm --dir web test -- src/sw/__tests__/sw.test.ts
```

Expected: all 11 tests pass.

- [ ] **Step 5: TypeScript check**

```bash
pnpm --dir web run check
```

Expected: zero errors. Note: `self.__WB_MANIFEST` is injected at build time; vitest doesn't compile the SW, so any type error there must be caught by `tsc --noEmit` or a build run.

- [ ] **Step 6: Add manifest assertion to sw.test.ts**

Append to `web/src/sw/__tests__/sw.test.ts`:

```ts
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

describe('PWA manifest', () => {
  // web/src/sw/__tests__/ is 4 dirs deep from web/ — four ../ reaches web/dist/
  const manifestPath = resolve(__dirname, '../../../../dist/manifest.webmanifest');

  it('manifest exists after build', () => {
    // Run `pnpm --dir web build` before this test if manifest is missing.
    if (!existsSync(manifestPath)) {
      console.warn('manifest.webmanifest not found — run pnpm build first');
      return;
    }
    const manifest = JSON.parse(readFileSync(manifestPath, 'utf-8'));
    expect(manifest.name).toBe('Tap');
    expect(manifest.display).toBe('standalone');
    expect(manifest.theme_color).toBe('#002FA7');
    const icons: Array<{ sizes: string; purpose: string }> = manifest.icons ?? [];
    expect(icons.some(i => i.sizes === '192x192' && i.purpose === 'maskable')).toBe(true);
    expect(icons.some(i => i.sizes === '512x512' && i.purpose === 'maskable')).toBe(true);
  });
});
```

- [ ] **Step 7: Commit**

```bash
git add web/src/sw/swPatterns.ts web/src/sw/sw.ts web/src/sw/__tests__/sw.test.ts
git commit -m "M10: add service worker with Workbox cache strategies (TDD)"
```

---

### Task 4: Implement the offline mutation queue (RED then GREEN)

**Invoke:** `superpowers:test-driven-development` skill at start of this task.

**Files:**
- Create: `web/src/lib/offlineQueue.ts`
- Create: `web/src/lib/__tests__/offlineQueue.test.ts`

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/__tests__/offlineQueue.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest';

// We test offlineQueue in isolation with a mocked localStorage and fetch.

// Inline the types so tests don't depend on the not-yet-created module.
type QueuedMutation = {
  id: string;
  method: string;
  path: string;
  body: unknown;
  csrfToken: string;
  enqueuedAt: number;
};

// Storage helpers used by the module under test.
const storageKey = (uid: number) => `tap:queue:${uid}`;
const pendingKey = (uid: number) => `tap:queue-pending:${uid}`;

// Minimal localStorage mock (jsdom provides one; we just reset it between tests).
beforeEach(() => localStorage.clear());

// Defer the real import until after we define the mocks.
// vi.resetModules() on every beforeEach gives each test a fresh module instance,
// which also resets the module-level `draining` flag — no separate reset helper needed.
let offlineQueue: typeof import('../offlineQueue').offlineQueue;
beforeEach(async () => {
  vi.resetModules();
  offlineQueue = (await import('../offlineQueue')).offlineQueue;
});

describe('offlineQueue.enqueue', () => {
  it('writes mutation to localStorage keyed by user ID', () => {
    offlineQueue.enqueue(42, { method: 'PATCH', path: '/api/v1/entries/1', body: { read: true }, csrfToken: 'tok' });
    const raw = localStorage.getItem(storageKey(42));
    expect(raw).not.toBeNull();
    const items: QueuedMutation[] = JSON.parse(raw!);
    expect(items).toHaveLength(1);
    expect(items[0].path).toBe('/api/v1/entries/1');
    expect(items[0].csrfToken).toBe('tok');
  });

  it('preserves insertion order on multiple enqueues', () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/1', body: { read: true }, csrfToken: 't' });
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/2', body: { saved: true }, csrfToken: 't' });
    const items: QueuedMutation[] = JSON.parse(localStorage.getItem(storageKey(1))!);
    expect(items[0].path).toBe('/api/v1/entries/1');
    expect(items[1].path).toBe('/api/v1/entries/2');
  });

  it('does not cross-contaminate users', () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/a', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(2, { method: 'PATCH', path: '/b', body: {}, csrfToken: 't' });
    expect(JSON.parse(localStorage.getItem(storageKey(1))!)).toHaveLength(1);
    expect(JSON.parse(localStorage.getItem(storageKey(2))!)).toHaveLength(1);
  });
});

describe('offlineQueue.drain — happy path', () => {
  it('dequeues mutation on 2xx and continues', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/1', body: { read: true }, csrfToken: 'tok' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200 }));
    await offlineQueue.drain(1);
    expect(JSON.parse(localStorage.getItem(storageKey(1)) ?? '[]')).toHaveLength(0);
  });

  it('processes mutations in order', async () => {
    const calls: string[] = [];
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/1', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/2', body: {}, csrfToken: 't' });
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      calls.push(url);
      return Promise.resolve({ ok: true, status: 200 });
    }));
    await offlineQueue.drain(1);
    expect(calls[0]).toContain('/entries/1');
    expect(calls[1]).toContain('/entries/2');
  });
});

describe('offlineQueue.drain — 401 handling', () => {
  it('pauses drain and sets pendingDrain flag on 401', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/1', body: {}, csrfToken: 't' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false, status: 401,
      json: () => Promise.resolve({ error: { code: 'invalid_session' } }),
    }));
    await offlineQueue.drain(1);
    // Queue NOT cleared — preserved for after re-login.
    expect(JSON.parse(localStorage.getItem(storageKey(1))!)).toHaveLength(1);
    // pendingDrain flag set.
    expect(localStorage.getItem(pendingKey(1))).toBe('1');
  });
});

describe('offlineQueue.drain — 403 csrf_invalid handling', () => {
  it('re-fetches CSRF token, retries once, dequeues on success', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/1', body: {}, csrfToken: 'old' });
    let callCount = 0;
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      callCount++;
      if (url.includes('/sessions/current')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ csrf_token: 'new', user: {} }) });
      }
      if (callCount === 1) {
        return Promise.resolve({ ok: false, status: 403, json: () => Promise.resolve({ error: { code: 'csrf_invalid' } }) });
      }
      return Promise.resolve({ ok: true, status: 200 });
    }));
    await offlineQueue.drain(1);
    expect(JSON.parse(localStorage.getItem(storageKey(1)) ?? '[]')).toHaveLength(0);
  });

  it('dequeues and logs if retry also returns 403', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/1', body: {}, csrfToken: 'old' });
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      if (url.includes('/sessions/current')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ csrf_token: 'new', user: {} }) });
      }
      return Promise.resolve({ ok: false, status: 403, json: () => Promise.resolve({ error: { code: 'csrf_invalid' } }) });
    }));
    await offlineQueue.drain(1);
    // Dequeued (can't recover from double-403).
    expect(JSON.parse(localStorage.getItem(storageKey(1)) ?? '[]')).toHaveLength(0);
  });
});

describe('offlineQueue.drain — network error', () => {
  it('stops drain on network error; next drain retries from failed mutation', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/1', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/2', body: {}, csrfToken: 't' });
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network error')));
    await offlineQueue.drain(1);
    // Both mutations still queued.
    expect(JSON.parse(localStorage.getItem(storageKey(1))!)).toHaveLength(2);
  });
});

describe('offlineQueue.drain — unrecoverable 4xx', () => {
  it('dequeues and continues on other 4xx', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/1', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/2', body: {}, csrfToken: 't' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false, status: 404, json: () => Promise.resolve({ error: { code: 'not_found' } }),
    }));
    await offlineQueue.drain(1);
    expect(JSON.parse(localStorage.getItem(storageKey(1)) ?? '[]')).toHaveLength(0);
  });
});

describe('offlineQueue.drain — re-entrancy', () => {
  it('ignores concurrent drain calls (draining flag)', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/api/v1/entries/1', body: {}, csrfToken: 't' });
    let fetchCount = 0;
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => {
      fetchCount++;
      return new Promise(resolve => setTimeout(() => resolve({ ok: true, status: 200 }), 10));
    }));
    // Start two drains concurrently.
    const [d1, d2] = [offlineQueue.drain(1), offlineQueue.drain(1)];
    await Promise.all([d1, d2]);
    // Only one fetch should have fired.
    expect(fetchCount).toBe(1);
  });
});

describe('offlineQueue.drain — reload survival', () => {
  it('drains queue written by a previous instance', async () => {
    // Simulate a previous page session writing to localStorage directly.
    const mutation: QueuedMutation = {
      id: 'abc', method: 'PATCH', path: '/api/v1/entries/99',
      body: { read: true }, csrfToken: 'tok', enqueuedAt: Date.now(),
    };
    localStorage.setItem(storageKey(5), JSON.stringify([mutation]));
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200 }));
    // Fresh module instance picks it up.
    vi.resetModules();
    const fresh = (await import('../offlineQueue')).offlineQueue;
    await fresh.drain(5);
    expect(JSON.parse(localStorage.getItem(storageKey(5)) ?? '[]')).toHaveLength(0);
  });
});

describe('offlineQueue.clearForUser', () => {
  it('removes only the specified user keys', () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/a', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(2, { method: 'PATCH', path: '/b', body: {}, csrfToken: 't' });
    localStorage.setItem(pendingKey(1), '1');
    offlineQueue.clearForUser(1);
    expect(localStorage.getItem(storageKey(1))).toBeNull();
    expect(localStorage.getItem(pendingKey(1))).toBeNull();
    // User 2 unaffected.
    expect(localStorage.getItem(storageKey(2))).not.toBeNull();
  });
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
pnpm --dir web test -- src/lib/__tests__/offlineQueue.test.ts
```

Expected: FAIL — module `../offlineQueue` not found.

- [ ] **Step 3: Implement offlineQueue.ts**

Create `web/src/lib/offlineQueue.ts`:

```ts
type QueuedMutation = {
  id: string;
  method: string;
  path: string;
  body: unknown;
  csrfToken: string;
  enqueuedAt: number;
};

type EnqueueInput = Omit<QueuedMutation, 'id' | 'enqueuedAt'>;

const BASE = '/api/v1';

function storageKey(userId: number) { return `tap:queue:${userId}`; }
function pendingKey(userId: number) { return `tap:queue-pending:${userId}`; }

function read(userId: number): QueuedMutation[] {
  try {
    return JSON.parse(localStorage.getItem(storageKey(userId)) ?? '[]');
  } catch {
    return [];
  }
}

function write(userId: number, items: QueuedMutation[]) {
  localStorage.setItem(storageKey(userId), JSON.stringify(items));
}

let draining = false;

async function fetchCSRF(): Promise<string | null> {
  try {
    const res = await fetch(BASE + '/sessions/current', { headers: { 'Content-Type': 'application/json' } });
    if (!res.ok) return null;
    const body = await res.json();
    return body.csrf_token ?? null;
  } catch {
    return null;
  }
}

async function sendMutation(m: QueuedMutation): Promise<Response> {
  return fetch(BASE + m.path, {
    method: m.method,
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': m.csrfToken,
    },
    body: JSON.stringify(m.body),
  });
}

export const offlineQueue = {
  enqueue(userId: number, input: EnqueueInput): void {
    const items = read(userId);
    items.push({ ...input, id: crypto.randomUUID(), enqueuedAt: Date.now() });
    write(userId, items);
  },

  async drain(userId: number): Promise<void> {
    if (draining) return;
    draining = true;
    try {
      while (true) {
        const items = read(userId);
        if (items.length === 0) break;
        const [head, ...rest] = items;
        let res: Response;
        try {
          res = await sendMutation(head);
        } catch {
          // Network error — stop; retry on next drain call.
          break;
        }

        if (res.ok) {
          write(userId, rest);
          continue;
        }

        let code: string | undefined;
        try { code = (await res.json())?.error?.code; } catch { /* swallow */ }

        if (res.status === 401) {
          // Session expired — pause, preserve queue, signal pending drain.
          localStorage.setItem(pendingKey(userId), '1');
          break;
        }

        if (res.status === 403 && code === 'csrf_invalid') {
          const freshToken = await fetchCSRF();
          if (freshToken) {
            head.csrfToken = freshToken;
            let retry: Response;
            try {
              retry = await sendMutation(head);
            } catch {
              break;
            }
            if (retry.ok) {
              write(userId, rest);
              continue;
            }
          }
          // Double-403 or no fresh token — discard.
          console.warn('offlineQueue: discarding unrecoverable mutation', head.path);
          write(userId, rest);
          continue;
        }

        // Other 4xx — unrecoverable, discard.
        console.warn('offlineQueue: discarding unrecoverable mutation', res.status, head.path);
        write(userId, rest);
      }
    } finally {
      // Clear pendingDrain on successful completion (queue empty).
      if (read(userId).length === 0) {
        localStorage.removeItem(pendingKey(userId));
      }
      draining = false;
    }
  },

  clearForUser(userId: number): void {
    localStorage.removeItem(storageKey(userId));
    localStorage.removeItem(pendingKey(userId));
  },
};
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
pnpm --dir web test -- src/lib/__tests__/offlineQueue.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: TypeScript check**

```bash
pnpm --dir web run check
```

Expected: zero errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/offlineQueue.ts web/src/lib/__tests__/offlineQueue.test.ts
git commit -m "M10: add localStorage mutation queue with serial drain (TDD)"
```

---

### Task 5: Implement the warm-cache driver (RED then GREEN)

**Invoke:** `superpowers:test-driven-development` skill at start of this task.

**Files:**
- Create: `web/src/lib/warmCache.ts`
- Create: `web/src/lib/__tests__/warmCache.test.ts`

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/__tests__/warmCache.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';

const PROXY_REGEX = /\/api\/v1\/proxy\/[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g;

// Helper to build fake entry content with N proxy URLs.
function makeContent(count: number, prefix = 'entry'): string {
  return Array.from({ length: count }, (_, i) =>
    `<img src="/api/v1/proxy/${prefix}-${i}.abc123">`
  ).join('');
}

beforeEach(() => vi.clearAllMocks());

let warmCache: typeof import('../warmCache').warmCache;
beforeEach(async () => {
  vi.resetModules();
  warmCache = (await import('../warmCache')).warmCache;
});

afterEach(() => vi.restoreAllMocks());

describe('warmCache — URL extraction', () => {
  it('extracts proxy URLs from entry content', async () => {
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: [{ id: 1, content: makeContent(2) }],
      }) });
    }));
    await warmCache(1);
    // First call is the entries fetch; subsequent calls are proxy URLs.
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(2);
  });

  it('deduplicates URLs across entries', async () => {
    const sharedUrl = '/api/v1/proxy/shared.abc123';
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: [
          { id: 1, content: `<img src="${sharedUrl}">` },
          { id: 2, content: `<img src="${sharedUrl}">` },
        ],
      }) });
    }));
    await warmCache(1);
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(1);
  });
});

describe('warmCache — per-entry URL cap', () => {
  it('caps at 20 proxy URLs per entry', async () => {
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: [{ id: 1, content: makeContent(30) }], // 30 URLs in one entry
      }) });
    }));
    await warmCache(1);
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(20);
  });
});

describe('warmCache — total URL cap', () => {
  it('caps total warm list at 200 URLs', async () => {
    // 20 entries × 20 URLs each = 400 possible; should be capped at 200.
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: Array.from({ length: 20 }, (_, i) => ({ id: i, content: makeContent(20, `e${i}`) })),
      }) });
    }));
    await warmCache(1);
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(200);
  });
});

describe('warmCache — concurrency', () => {
  it('runs at most 4 fetches concurrently', async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    // Give entries that produce exactly 8 proxy URLs.
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      if (!url.includes('/proxy/')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
          data: [{ id: 1, content: makeContent(8) }],
        }) });
      }
      inFlight++;
      maxInFlight = Math.max(maxInFlight, inFlight);
      return new Promise(resolve =>
        setTimeout(() => {
          inFlight--;
          resolve({ ok: true, status: 200 });
        }, 5)
      );
    }));
    await warmCache(1);
    expect(maxInFlight).toBeLessThanOrEqual(4);
  });
});

describe('warmCache — error handling', () => {
  it('swallows fetch errors silently', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network error')));
    await expect(warmCache(1)).resolves.toBeUndefined();
  });

  it('swallows 401 silently without triggering re-auth', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 401 }));
    await expect(warmCache(1)).resolves.toBeUndefined();
  });

  it('no-ops when entries response is empty', async () => {
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: [] }) });
    }));
    await warmCache(1);
    expect(fetched).toHaveLength(1); // Only the entries fetch.
  });

  it('uses plain fetch (no cache option) so SW CacheFirst strategy is populated', async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: [{ id: 1, content: makeContent(1) }],
      }) });
    });
    vi.stubGlobal('fetch', fetchMock);
    await warmCache(1);
    const proxyCalls = fetchMock.mock.calls.filter(([u]: [string]) => u.includes('/proxy/'));
    // Each proxy fetch must have NO second argument, or second arg with no 'cache' key.
    for (const [, init] of proxyCalls) {
      expect((init as RequestInit | undefined)?.cache).toBeUndefined();
    }
  });
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
pnpm --dir web test -- src/lib/__tests__/warmCache.test.ts
```

Expected: FAIL — module `../warmCache` not found.

- [ ] **Step 3: Implement warmCache.ts**

Create `web/src/lib/warmCache.ts`:

```ts
const PROXY_URL_REGEX = /\/api\/v1\/proxy\/[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g;
const PER_ENTRY_CAP = 20;
const TOTAL_CAP = 200;
const CONCURRENCY = 4;

async function runWithConcurrency(urls: string[], concurrency: number): Promise<void> {
  const queue = [...urls];
  async function worker() {
    while (queue.length > 0) {
      const url = queue.shift()!;
      try {
        await fetch(url);
      } catch {
        // Silently swallow — warm-cache is opportunistic.
      }
    }
  }
  await Promise.all(Array.from({ length: concurrency }, worker));
}

// userId is accepted for interface consistency with callers (drain takes userId)
// and for future per-user cache-warming signals; the browser attaches the session
// cookie implicitly on same-origin GETs so no per-user auth is needed here.
export async function warmCache(_userId: number): Promise<void> {
  try {
    const res = await fetch('/api/v1/entries?limit=50&unread=1');
    if (!res.ok) return;
    const body = await res.json();
    const entries: Array<{ content: string }> = body.data ?? [];

    const seen = new Set<string>();
    const urls: string[] = [];

    for (const entry of entries) {
      if (urls.length >= TOTAL_CAP) break;
      const matches = (entry.content ?? '').matchAll(PROXY_URL_REGEX);
      let count = 0;
      for (const m of matches) {
        if (count >= PER_ENTRY_CAP) break;
        if (urls.length >= TOTAL_CAP) break;
        const url = m[0];
        if (!seen.has(url)) {
          seen.add(url);
          urls.push(url);
          count++;
        }
      }
    }

    await runWithConcurrency(urls, CONCURRENCY);
  } catch {
    // Swallow all errors — warm-cache must never surface to the user.
  }
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
pnpm --dir web test -- src/lib/__tests__/warmCache.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: TypeScript check**

```bash
pnpm --dir web run check
```

Expected: zero errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/warmCache.ts web/src/lib/__tests__/warmCache.test.ts
git commit -m "M10: add warm-cache driver with concurrency and URL caps (TDD)"
```

---

### Task 6: Wire auth.ts — SW postMessage and queue cleanup

**Invoke:** `superpowers:test-driven-development` skill at start of this task.

**Files:**
- Modify: `web/src/lib/auth.ts`
- Modify: `web/src/lib/__tests__/auth.test.ts`

- [ ] **Step 1: Add failing tests**

In `web/src/lib/__tests__/auth.test.ts`, append these test cases (keep existing tests):

```ts
import { offlineQueue } from '../offlineQueue';
import { vi } from 'vitest';

// Add inside the existing describe blocks or as new describe blocks:

describe('auth — SW postMessage on bootstrap', () => {
  it('posts set-user message to SW controller after successful bootstrap', async () => {
    const postMessage = vi.fn();
    Object.defineProperty(navigator, 'serviceWorker', {
      value: { controller: { postMessage } },
      configurable: true,
    });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true, status: 200,
      json: () => Promise.resolve({ user: { id: 7, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
    }));
    await auth.bootstrap();
    expect(postMessage).toHaveBeenCalledWith({ type: 'set-user', userId: 7 });
  });
});

describe('auth — logout clears queue and notifies SW', () => {
  it('calls offlineQueue.clearForUser and posts logout to SW', async () => {
    const clearSpy = vi.spyOn(offlineQueue, 'clearForUser');
    const postMessage = vi.fn();
    Object.defineProperty(navigator, 'serviceWorker', {
      value: { controller: { postMessage } },
      configurable: true,
    });
    // Pre-set user state.
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 204 }));
    // Manually set internal state to simulate logged-in user.
    await auth.login('ben', 'secret');
    await auth.logout();
    expect(clearSpy).toHaveBeenCalledWith(expect.any(Number));
    expect(postMessage).toHaveBeenCalledWith({ type: 'logout', userId: expect.any(Number) });
  });
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
pnpm --dir web test -- src/lib/__tests__/auth.test.ts
```

Expected: the new tests fail.

- [ ] **Step 3: Update auth.ts**

In `web/src/lib/auth.ts`, make these changes:

At the top, add the import:
```ts
import { offlineQueue } from './offlineQueue';
import type { SWMessage } from '../sw/workerTypes';
```

Add a helper after the imports:
```ts
function notifySW(msg: SWMessage): void {
  navigator.serviceWorker?.controller?.postMessage(msg);
}
```

In `bootstrap()`, after `internal.set({ user: body.user, ...})`:
```ts
notifySW({ type: 'set-user', userId: body.user.id });
```

Replace `logout()` with:
```ts
async logout(): Promise<void> {
  let userId: number | null = null;
  let csrfToken: string | null = null;
  internal.update(s => {
    userId = s.user?.id ?? null;
    csrfToken = s.csrfToken;
    return s;
  });
  // Clear offline queue and SW cache for this user before wiping auth state.
  if (userId !== null) {
    offlineQueue.clearForUser(userId);
    notifySW({ type: 'logout', userId });
  }
  try {
    await fetch(BASE + '/sessions/current', {
      method: 'DELETE',
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
    });
  } catch {
    /* network errors during logout don't matter */
  }
  internal.set({ user: null, csrfToken: null, bootstrapped: true });
},
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
pnpm --dir web test -- src/lib/__tests__/auth.test.ts
```

Expected: all tests pass (including pre-existing ones).

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/auth.ts web/src/lib/__tests__/auth.test.ts
git commit -m "M10: auth.ts posts SW messages on bootstrap/logout, clears queue on logout"
```

---

### Task 7: Wire api.ts — enqueue mutations on offline/network error

**Invoke:** `superpowers:test-driven-development` skill at start of this task.

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/__tests__/api.test.ts`

The goal: when a non-GET request fails because the network is offline (`navigator.onLine === false`) or throws a `TypeError` (network-level failure), `request()` calls `offlineQueue.enqueue(userId, mutation)` instead of propagating the error, and returns a sentinel value so the optimistic store update stands.

- [ ] **Step 1: Add failing tests**

In `web/src/lib/__tests__/api.test.ts`, append:

```ts
import { offlineQueue } from '../offlineQueue';
import { vi } from 'vitest';

describe('api.request — offline enqueue', () => {
  it('enqueues PATCH mutation when navigator.onLine is false', async () => {
    const enqueueSpy = vi.spyOn(offlineQueue, 'enqueue');
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true });
    // Set up auth state with a user.
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true, status: 200,
      json: () => Promise.resolve({ user: { id: 3, username: 'u', role: 'user' }, csrf_token: 'tok' }),
    }));
    await auth.bootstrap();
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true });
    // patchEntry should not throw.
    await api.patchEntry(42, { read: true });
    expect(enqueueSpy).toHaveBeenCalledWith(3, expect.objectContaining({
      method: 'PATCH',
      path: '/entries/42',
    }));
    Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });
  });

  it('enqueues mutation on network TypeError', async () => {
    const enqueueSpy = vi.spyOn(offlineQueue, 'enqueue');
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce({ ok: true, status: 200, json: () => Promise.resolve({ user: { id: 3, username: 'u', role: 'user' }, csrf_token: 'tok' }) })
      .mockRejectedValue(new TypeError('network error')),
    );
    await auth.bootstrap();
    await api.patchEntry(42, { read: true });
    expect(enqueueSpy).toHaveBeenCalled();
  });

  it('does NOT enqueue GET requests', async () => {
    const enqueueSpy = vi.spyOn(offlineQueue, 'enqueue');
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true });
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network error')));
    await expect(api.getEntry(1)).rejects.toThrow();
    expect(enqueueSpy).not.toHaveBeenCalled();
    Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });
  });
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
pnpm --dir web test -- src/lib/__tests__/api.test.ts
```

Expected: the new offline-enqueue tests fail.

- [ ] **Step 3: Update api.ts**

At the top of `web/src/lib/api.ts`, add only the `offlineQueue` import — `get` from `svelte/store` is already imported on line 1:

```ts
import { offlineQueue } from './offlineQueue';
```

Replace the `request<T>` function with:

```ts
async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase();
  const isWriteMethod = method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS';
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((init.headers ?? {}) as Record<string, string>),
  };

  const authState = get(auth);
  if (isWriteMethod && authState.csrfToken) {
    headers['X-CSRF-Token'] = authState.csrfToken;
  }

  // If offline or network fails on a write method, queue the mutation.
  if (isWriteMethod && !navigator.onLine) {
    if (authState.user) {
      offlineQueue.enqueue(authState.user.id, {
        method,
        path,
        body: init.body ? JSON.parse(init.body as string) : undefined,
        csrfToken: authState.csrfToken ?? '',
      });
    }
    return undefined as T;
  }

  let res: Response;
  try {
    res = await fetch(BASE + path, { ...init, headers });
  } catch (err) {
    if (isWriteMethod && err instanceof TypeError) {
      if (authState.user) {
        offlineQueue.enqueue(authState.user.id, {
          method,
          path,
          body: init.body ? JSON.parse(init.body as string) : undefined,
          csrfToken: authState.csrfToken ?? '',
        });
      }
      return undefined as T;
    }
    throw err;
  }

  if (res.status === 401) {
    let detail: ApiError | null = null;
    try { detail = await res.json(); } catch { /* swallow */ }
    if (detail?.error?.code !== 'invalid_credentials') {
      auth.clearOn401();
    }
    throw new Error(detail?.error?.message ?? ERR_UNAUTHORIZED);
  }
  if (!res.ok) {
    let detail: ApiError | null = null;
    try { detail = await res.json(); } catch { /* swallow */ }
    throw new Error(detail?.error?.message ?? `${res.status} ${res.statusText}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}
```

- [ ] **Step 4: Run all api tests to confirm they pass**

```bash
pnpm --dir web test -- src/lib/__tests__/api.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/api.ts web/src/lib/__tests__/api.test.ts
git commit -m "M10: api.ts enqueues write mutations when offline or on network error"
```

---

### Task 8: Update App.svelte — SW update banner, boot drain, online listener

**Invoke:** `superpowers:test-driven-development` skill at start of this task.

**Files:**
- Modify: `web/src/App.svelte`
- Modify: `web/src/views/__tests__/App.test.ts` (create if absent)

- [ ] **Step 1: Write failing tests for App.svelte changes**

Create `web/src/views/__tests__/App.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import App from '../../App.svelte';
import { auth } from '../../lib/auth';
import { offlineQueue } from '../../lib/offlineQueue';
import { warmCache } from '../../lib/warmCache';

vi.mock('../../lib/auth', () => ({
  auth: {
    subscribe: vi.fn((cb: (s: unknown) => void) => {
      cb({ user: null, csrfToken: null, bootstrapped: true });
      return () => {};
    }),
    bootstrap: vi.fn().mockResolvedValue(undefined),
  },
}));
vi.mock('../../lib/offlineQueue', () => ({
  offlineQueue: { drain: vi.fn().mockResolvedValue(undefined) },
}));
vi.mock('../../lib/warmCache', () => ({
  warmCache: vi.fn().mockResolvedValue(undefined),
}));
vi.mock('virtual:pwa-register/svelte', () => ({
  useRegisterSW: () => ({
    needRefresh: { subscribe: (cb: (v: boolean) => void) => { cb(false); return () => {}; } },
    updateServiceWorker: vi.fn(),
  }),
}));

describe('App — SW update banner', () => {
  it('renders update banner when needRefresh is true', async () => {
    vi.mock('virtual:pwa-register/svelte', () => ({
      useRegisterSW: () => ({
        needRefresh: { subscribe: (cb: (v: boolean) => void) => { cb(true); return () => {}; } },
        updateServiceWorker: vi.fn(),
      }),
    }));
    vi.resetModules();
    const AppFresh = (await import('../../App.svelte')).default;
    const { getByText } = render(AppFresh);
    expect(getByText(/Update available/i)).toBeTruthy();
  });

  it('does not render update banner when needRefresh is false', () => {
    const { queryByText } = render(App);
    expect(queryByText(/Update available/i)).toBeNull();
  });
});

describe('App — online event triggers drain and warmCache', () => {
  beforeEach(() => vi.clearAllMocks());

  it('calls drain and warmCache when online event fires with authenticated user', async () => {
    vi.mocked(auth.subscribe).mockImplementation((cb: (s: unknown) => void) => {
      cb({ user: { id: 5, username: 'ben', role: 'admin' }, csrfToken: 'tok', bootstrapped: true });
      return () => {};
    });
    render(App);
    // Simulate online event.
    fireEvent(window, new Event('online'));
    await vi.runAllTimersAsync();
    expect(offlineQueue.drain).toHaveBeenCalledWith(5);
    expect(warmCache).toHaveBeenCalledWith(5);
  });
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
pnpm --dir web test -- src/views/__tests__/App.test.ts
```

Expected: FAIL — `App.svelte` does not yet import `useRegisterSW` or render the banner.

- [ ] **Step 3: Replace App.svelte**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { route } from './lib/router';
  import { auth } from './lib/auth';
  import { offlineQueue } from './lib/offlineQueue';
  import { warmCache } from './lib/warmCache';
  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';
  import { useRegisterSW } from 'virtual:pwa-register/svelte';

  const { needRefresh, updateServiceWorker } = useRegisterSW({
    onNeedRefresh() {
      // needRefresh is already reactive; no extra work needed.
    },
  });

  function notifySWUser(userId: number) {
    navigator.serviceWorker?.controller?.postMessage({ type: 'set-user', userId });
  }

  async function bootSequence() {
    await auth.bootstrap();
    const user = get(auth).user;
    if (user) {
      notifySWUser(user.id);
      await offlineQueue.drain(user.id);
      setTimeout(() => { void warmCache(user.id); }, 2000);
    }
  }

  onMount(() => {
    void bootSequence();

    const handleOnline = async () => {
      const userId = get(auth).user?.id ?? null;
      if (userId !== null) {
        await offlineQueue.drain(userId);
        void warmCache(userId);
      }
    };

    window.addEventListener('online', handleOnline);
    return () => window.removeEventListener('online', handleOnline);
  });
</script>

{#if $needRefresh}
  <div class="sw-update-banner">
    Update available —
    <button onclick={() => updateServiceWorker(true)}>Reload</button>
  </div>
{/if}

{#if !$auth.bootstrapped}
  <!-- empty during bootstrap — prevents flashing login for authenticated users -->
{:else if $auth.user == null}
  <Login />
{:else if $route.name === 'reader'}
  <Reader id={$route.params.id} />
{:else}
  <Unread />
{/if}

<style>
  .sw-update-banner {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    z-index: 9999;
    background: var(--accent, #002FA7);
    color: #fff;
    padding: 0.5rem 1rem;
    font-size: 0.875rem;
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .sw-update-banner button {
    background: rgba(255,255,255,0.2);
    border: 1px solid rgba(255,255,255,0.5);
    color: #fff;
    padding: 0.25rem 0.75rem;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
```

- [ ] **Step 4: Run App.svelte tests to confirm they pass**

```bash
pnpm --dir web test -- src/views/__tests__/App.test.ts
```

Expected: both banner tests and the online-event test pass.

- [ ] **Step 5: TypeScript check**

```bash
pnpm --dir web run check
```

Expected: zero errors. Note: `virtual:pwa-register/svelte` is provided by `vite-plugin-pwa` at build time; check may need a build to resolve this virtual module. If check fails on this import alone, add a type shim:

Create `web/src/vite-env.d.ts` additions:
```ts
declare module 'virtual:pwa-register/svelte' {
  import type { Writable } from 'svelte/store';
  export function useRegisterSW(options?: { onNeedRefresh?: () => void }): {
    needRefresh: Writable<boolean>;
    updateServiceWorker: (reloadPage?: boolean) => Promise<void>;
  };
}
```

- [ ] **Step 6: Build to confirm no errors**

```bash
pnpm --dir web build
```

Expected: builds successfully; `web/dist` contains `sw.js`, `manifest.webmanifest`, `icons/`.

- [ ] **Step 7: Commit**

```bash
git add web/src/App.svelte web/src/vite-env.d.ts web/src/views/__tests__/App.test.ts
git commit -m "M10: App.svelte wires SW update banner, boot drain, online listener (TDD)"
```

---

### Task 9: Run the full test suite

**Files:** none new — regression check only.

- [ ] **Step 1: Run all frontend tests**

```bash
pnpm --dir web test
```

Expected: all tests pass, including pre-existing store, router, auth, api, component tests plus the new offlineQueue, warmCache, and sw strategy tests.

- [ ] **Step 2: Run Go tests (regression)**

```bash
make test
```

Expected: `go test ./... -race` passes. No Go code changed, so this is purely a regression guard.

- [ ] **Step 3: Run TypeScript check**

```bash
pnpm --dir web run check
```

Expected: zero errors.

- [ ] **Step 4: Build the binary**

```bash
make build
```

Expected: `bin/tap` produced; `web/dist` contains `sw.js`, `manifest.webmanifest`, `icons/icon-192.png`, `icons/icon-512.png`.

- [ ] **Step 5: Commit if any fixes were needed**

```bash
git add -p
git commit -m "M10: fix any issues found in full-suite regression"
```

---

### Task 10: Playwright E2E offline tests

**Invoke:** `superpowers:test-driven-development` skill at start of this task.

**Files:**
- Create: `web/e2e/offline.test.ts` (new Playwright test file)

Note: Playwright must be configured for this project. If `web/playwright.config.ts` does not exist, create it first (Step 1). The ARM64 chromium symlink at `/opt/google/chrome/chrome` is already in place.

- [ ] **Step 1: Check/create Playwright config**

```bash
ls /home/ben.guest/Users/ben/src/tap/web/playwright.config.ts 2>/dev/null || echo "missing"
```

If missing, create `web/playwright.config.ts`:

```ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  retries: 0,
  use: {
    baseURL: 'http://localhost:5173',
    channel: 'chrome',
    headless: true,
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    command: 'pnpm --dir web dev',
    url: 'http://localhost:5173',
    reuseExistingServer: true,
  },
});
```

Also install Playwright if needed:
```bash
pnpm --dir web add -D @playwright/test
```

- [ ] **Step 2: Create the E2E test file**

Create `web/e2e/offline.test.ts`:

```ts
import { test, expect } from '@playwright/test';

// These tests require the Go server to be running on :8080 with a test user.
// Set TAP_ADMIN_USERNAME=test TAP_ADMIN_PASSWORD=test1234 on first boot.
// The Vite dev server proxies /api to the Go server.

const USERNAME = process.env.TAP_E2E_USER ?? 'test';
const PASSWORD = process.env.TAP_E2E_PASS ?? 'test1234';

async function login(page: import('@playwright/test').Page) {
  await page.goto('/');
  await page.waitForSelector('input[name="username"], input[type="text"]');
  await page.fill('input[name="username"], input[type="text"]', USERNAME);
  await page.fill('input[name="password"], input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForSelector('.unread-view, [data-testid="unread"]', { timeout: 10000 });
}

test.describe('Offline reading (Scenario 1)', () => {
  test('app shell and entry load from SW cache when offline', async ({ page, context }) => {
    await login(page);

    // Wait for warm-cache to run (2s defer + some fetch time).
    await page.waitForTimeout(5000);

    // Open an entry while online so it's in the SW cache.
    const firstEntry = page.locator('.entry-row, [data-testid="entry-row"]').first();
    await firstEntry.click();
    await page.waitForSelector('.reader-body, [data-testid="reader-body"]');

    // Go offline.
    await context.setOffline(true);

    // Reload the page.
    await page.reload();

    // App shell must load (from precache).
    await page.waitForSelector('body', { timeout: 5000 });

    // Navigate back to the entry.
    await page.goto(page.url());

    // Entry body must render (from stale-while-revalidate cache).
    await expect(page.locator('.reader-body, [data-testid="reader-body"]')).toBeVisible({ timeout: 5000 });

    await context.setOffline(false);
  });
});

test.describe('Offline mutation replay (Scenario 2)', () => {
  test('mutations queued offline are replayed on reconnect', async ({ page, context }) => {
    await login(page);

    // Get current user ID from localStorage after bootstrap.
    await page.waitForTimeout(1000);

    await context.setOffline(true);

    // Open an entry — auto-mark-read fires and is queued.
    const firstEntry = page.locator('.entry-row, [data-testid="entry-row"]').first();
    const entryId = await firstEntry.getAttribute('data-entry-id');
    await firstEntry.click();
    await page.waitForTimeout(500);

    // Verify mutation is in the localStorage queue.
    const queueJson = await page.evaluate(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.length > 0 ? localStorage.getItem(keys[0]) : '[]';
    });
    const queue = JSON.parse(queueJson ?? '[]');
    expect(queue.length).toBeGreaterThan(0);

    // Go back online.
    await context.setOffline(false);

    // Wait for drain to complete (queue emptied).
    await page.waitForFunction(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.every(k => JSON.parse(localStorage.getItem(k) ?? '[]').length === 0);
    }, { timeout: 10000 });

    if (entryId) {
      // Verify the server reflects the read state.
      const resp = await page.evaluate(async (id) => {
        const r = await fetch(`/api/v1/entries/${id}`);
        return r.json();
      }, entryId);
      expect(resp.read).toBe(true);
    }
  });
});

test.describe('Queue survives reload (Scenario 3)', () => {
  test('queued mutations persist across full page reload and drain on reconnect', async ({ page, context }) => {
    await login(page);

    await context.setOffline(true);

    // Mark an entry read while offline.
    const firstEntry = page.locator('.entry-row, [data-testid="entry-row"]').first();
    await firstEntry.click();
    await page.waitForTimeout(500);

    // Verify queued.
    const hasQueue = await page.evaluate(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.some(k => JSON.parse(localStorage.getItem(k) ?? '[]').length > 0);
    });
    expect(hasQueue).toBe(true);

    // Reload while still offline.
    await page.reload();

    // Still queued after reload.
    const stillQueued = await page.evaluate(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.some(k => JSON.parse(localStorage.getItem(k) ?? '[]').length > 0);
    });
    expect(stillQueued).toBe(true);

    // Go back online.
    await context.setOffline(false);

    // Wait for drain.
    await page.waitForFunction(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.every(k => JSON.parse(localStorage.getItem(k) ?? '[]').length === 0);
    }, { timeout: 10000 });
  });
});
```

- [ ] **Step 3: Start the Go server and run E2E tests**

In a separate terminal (or use `! TAP_ADMIN_USERNAME=test TAP_ADMIN_PASSWORD=test1234 ./bin/tap`), ensure the Go server is running. Then:

```bash
pnpm --dir web exec playwright test e2e/offline.test.ts --reporter=list
```

Expected: all three scenarios pass. Service workers require a built SW (run `pnpm --dir web build` first if testing against the dev server fails on SW registration).

- [ ] **Step 4: Commit**

```bash
git add web/e2e/offline.test.ts web/playwright.config.ts web/package.json
git commit -m "M10: add Playwright E2E offline scenarios (3 scenarios)"
```

---

### Task 11: Definition-of-done checklist

- [ ] `pnpm --dir web test` — all unit tests pass.
- [ ] `make test` — Go tests pass (regression).
- [ ] `pnpm --dir web run check` — zero TypeScript errors.
- [ ] `make build` — static binary built; `web/dist` contains `sw.js`, `manifest.webmanifest`, `icons/icon-192.png`, `icons/icon-512.png`.
- [ ] Open Chrome → DevTools → Application → Manifest: no errors; icons display; `display: standalone`.
- [ ] DevTools → Network → Offline: app shell loads, unread list renders from cache, a previously opened entry renders.
- [ ] DevTools → Network → Offline: mark entry read → queue appears in localStorage; go online → queue drains; server reflects change.
- [ ] Logout: SW caches for that user deleted; localStorage queue cleared.
- [ ] All three Playwright offline E2E scenarios pass.

---

### Task 12: Final commit and invoke /simplify

- [ ] **Step 1: Confirm all tests green**

```bash
pnpm --dir web test && make test && pnpm --dir web run check
```

- [ ] **Step 2: Invoke /simplify**

Per the global CLAUDE.md instruction: run `/simplify` once all tasks are complete.

- [ ] **Step 3: Final commit**

```bash
git add -p
git commit -m "M10: offline + PWA complete — service worker, mutation queue, warm-cache, manifest"
```
