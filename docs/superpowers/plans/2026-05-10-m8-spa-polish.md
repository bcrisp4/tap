# M8 SPA Polish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the Tap SPA to full design-system fidelity: three themes + system-tracking, self-hosted fonts, serif/sans + density toggles, keyboard shortcuts, desktop reader layout with sidebar, mobile breakpoints + tab bar, swipe gestures, pull-to-refresh, animations, and an accessibility pass — matching `ui_design/` at high fidelity.

**Architecture:** All appearance preferences (`tap.theme`, `tap.font`, `tap.density`) live in `localStorage` and are applied as CSS classes on `<html>`. A single document-level keyboard handler lives in `App.svelte` and dispatches to the active view via a typed Svelte context. Swipe gestures are implemented as a reusable `{@attach}` attachment function. No new API endpoints or Go changes — this is a pure SPA milestone.

**Tech Stack:** Svelte 5 (runes, `{@attach}` directive, `<svelte:window>`), TypeScript, Vite, Vitest + `@testing-library/svelte`, fontsource npm packages (latin-subset woff2 bundles), CSS custom properties.

**Skills to invoke at each task:** noted per task below.

---

## File Map

### New files
| File | Responsibility |
|---|---|
| `web/src/lib/preferences.svelte.ts` | Theme, font, density `$state` + `localStorage` persistence. Single module owns all three prefs. |
| `web/src/lib/keyboard.ts` | Document-level keydown handler + `KeyboardContext` (Svelte context interface). |
| `web/src/lib/swipe.ts` | `swipe()` attachment function — returns `{@attach}` compatible function that emits `swipeleft`/`swiperight` custom events. |
| `web/src/lib/pulltorefresh.ts` | `pullToRefresh()` attachment function for mobile pull-down gesture. |
| `web/src/components/HotkeysModal.svelte` | `<dialog>`-based keyboard shortcuts reference. |
| `web/src/components/TabBar.svelte` | Mobile bottom tab bar (four tabs: Unread, Saved, Search, Settings). |
| `web/src/components/MobileTopBar.svelte` | Mobile top bar (wordmark + count + search icon, 60px status-bar padding). |
| `web/src/components/MobileReaderBar.svelte` | Mobile reader top bar (back + progress bar + save) and bottom action bar. |
| `web/src/views/Saved.svelte` | Saved entries view (stub for tab bar — empty state). |
| `web/src/views/Search.svelte` | Search view (stub for tab bar — empty state until M9). |
| `web/src/views/Settings.svelte` | Unified settings view: Appearance section (theme/font/density) + renders Security section slot for M7 content. |
| `web/src/lib/__tests__/preferences.test.ts` | Vitest: theme/font/density store behaviour. |
| `web/src/lib/__tests__/keyboard.test.ts` | Vitest: keyboard handler bindings, suppression, modal state. |
| `web/src/lib/__tests__/swipe.test.ts` | Vitest: swipe recogniser threshold and angle gate. |
| `web/src/lib/__tests__/pulltorefresh.test.ts` | Vitest: pull-to-refresh threshold and in-flight guard. |

### Modified files
| File | Changes |
|---|---|
| `web/src/styles/global.css` | Remove Google Fonts `@import`; add fontsource imports; add theme/font/density classes; add `prefers-reduced-motion` block. |
| `web/src/styles/tokens.css` | Add `.theme-dark` and `.theme-sepia` blocks from `ui_design/styles.css`; add `font-sans` override; add density classes. |
| `web/src/main.ts` | Import fontsource packages. |
| `web/src/App.svelte` | Mount keyboard handler; render `HotkeysModal`; apply theme/font/density class to `<html>`; wire routes for `/settings`, `/saved`, `/search`; render `TabBar` on mobile; provide `KeyboardContext`. |
| `web/src/views/Reader.svelte` | Add sidebar; add save toggle; add mobile reader chrome; wire swipe gestures on reader body; refactor layout to sidebar + reader pane. |
| `web/src/views/Unread.svelte` | Add swipe on entry rows; add pull-to-refresh; add ARIA attributes; add keyboard focus tracking (`selectedId`). |
| `web/src/components/Sidebar.svelte` | Add Settings + Saved nav items; hide on mobile. |
| `web/src/components/EntryRow.svelte` | Add `is-selected` class for keyboard cursor; add `is-saved` badge; add summary; add density-responsive padding. |
| `web/src/components/TopBar.svelte` | Add mark-all-read and filter icon buttons; ARIA improvements. |
| `web/package.json` | Add fontsource devDependencies. |

---

## Task 1: Self-host fonts via fontsource

**Skills:** none required (package install + CSS change)

**Files:**
- Modify: `web/package.json`
- Modify: `web/src/styles/global.css`
- Modify: `web/src/main.ts`

- [ ] **Step 1: Install fontsource packages**

```bash
cd web && pnpm add -D @fontsource-variable/source-serif-4 @fontsource-variable/inter-tight @fontsource/jetbrains-mono
```

Expected: three packages added to `node_modules`, `package.json` devDependencies updated.

- [ ] **Step 2: Import fonts in `web/src/main.ts`**

Replace the current content of `web/src/main.ts` with:

```typescript
import '@fontsource-variable/source-serif-4';
import '@fontsource-variable/inter-tight';
import '@fontsource/jetbrains-mono/400.css';
import '@fontsource/jetbrains-mono/500.css';
import { mount } from 'svelte';
import App from './App.svelte';
import './styles/tokens.css';
import './styles/global.css';

const app = mount(App, { target: document.getElementById('app')! });

export default app;
```

- [ ] **Step 3: Remove the Google Fonts `@import` from `web/src/styles/global.css`**

Delete the first line of `global.css` (the `@import url("https://fonts.googleapis.com/...")` line). Leave all other rules unchanged.

- [ ] **Step 4: Verify build succeeds and no Google Fonts request fires**

```bash
cd web && pnpm build
```

Expected: build succeeds, `dist/` populated. Open `dist/index.html` in a browser (or run `pnpm dev`) and check Network tab — no requests to `fonts.googleapis.com` or `fonts.gstatic.com`.

- [ ] **Step 5: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml web/src/main.ts web/src/styles/global.css
git commit -m "feat(web): self-host fonts via fontsource, remove Google Fonts"
```

---

## Task 2: Theme, font, and density preference store

**Skills:** invoke `svelte-runes` before implementing.

**Files:**
- Create: `web/src/lib/preferences.svelte.ts`
- Create: `web/src/lib/__tests__/preferences.test.ts`

- [ ] **Step 1: Write failing tests**

Create `web/src/lib/__tests__/preferences.test.ts`:

```typescript
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';

// localStorage mock
const store: Record<string, string> = {};
beforeEach(() => {
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store[k] ?? null,
    setItem: (k: string, v: string) => { store[k] = v; },
    removeItem: (k: string) => { delete store[k]; },
  });
  Object.keys(store).forEach(k => delete store[k]);
  // matchMedia mock: default to light
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: false,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }));
});
afterEach(() => { vi.unstubAllGlobals(); vi.resetModules(); });

describe('theme preference', () => {
  it('defaults to "system" when localStorage is empty', async () => {
    const { theme } = await import('../preferences.svelte');
    expect(theme.stored).toBe('system');
  });

  it('reads stored value from localStorage', async () => {
    store['tap.theme'] = 'dark';
    const { theme } = await import('../preferences.svelte');
    expect(theme.stored).toBe('dark');
  });

  it('resolves system to "light" when matchMedia does not match dark', async () => {
    const { theme } = await import('../preferences.svelte');
    expect(theme.resolved).toBe('light');
  });

  it('resolves system to "dark" when matchMedia matches dark', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query === '(prefers-color-scheme: dark)',
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }));
    const { theme } = await import('../preferences.svelte');
    expect(theme.resolved).toBe('dark');
  });

  it('persists to localStorage when stored is set', async () => {
    const { theme } = await import('../preferences.svelte');
    theme.stored = 'sepia';
    expect(store['tap.theme']).toBe('sepia');
  });

  it('resolved returns stored value directly when not "system"', async () => {
    store['tap.theme'] = 'sepia';
    const { theme } = await import('../preferences.svelte');
    expect(theme.resolved).toBe('sepia');
  });
});

describe('font preference', () => {
  it('defaults to "serif"', async () => {
    const { font } = await import('../preferences.svelte');
    expect(font.value).toBe('serif');
  });

  it('reads stored value', async () => {
    store['tap.font'] = 'sans';
    const { font } = await import('../preferences.svelte');
    expect(font.value).toBe('sans');
  });

  it('persists on set', async () => {
    const { font } = await import('../preferences.svelte');
    font.value = 'sans';
    expect(store['tap.font']).toBe('sans');
  });
});

describe('density preference', () => {
  it('defaults to "default"', async () => {
    const { density } = await import('../preferences.svelte');
    expect(density.value).toBe('default');
  });

  it('reads stored value', async () => {
    store['tap.density'] = 'compact';
    const { density } = await import('../preferences.svelte');
    expect(density.value).toBe('compact');
  });

  it('persists on set', async () => {
    const { density } = await import('../preferences.svelte');
    density.value = 'comfortable';
    expect(store['tap.density']).toBe('comfortable');
  });
});
```

- [ ] **Step 2: Run tests — expect failures**

```bash
cd web && pnpm test -- src/lib/__tests__/preferences.test.ts
```

Expected: all tests fail with "Cannot find module '../preferences.svelte'".

- [ ] **Step 3: Implement `web/src/lib/preferences.svelte.ts`**

```typescript
// preferences.svelte.ts — localStorage-backed reactive preferences.
// The .svelte.ts extension enables Svelte runes ($state, $derived) in a
// plain TS module (not a component). Rune values are reactive when read
// inside Svelte components or other .svelte.ts files.

type Theme = 'light' | 'dark' | 'sepia' | 'system';
type Font = 'serif' | 'sans';
type Density = 'compact' | 'default' | 'comfortable';

function mediaPrefersDark(): boolean {
  return typeof window !== 'undefined' &&
    window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function makeTheme() {
  let stored = $state<Theme>(
    (localStorage.getItem('tap.theme') as Theme) ?? 'system'
  );

  // resolved is derived: if stored === 'system', inspect matchMedia.
  // We read mediaPrefersDark() inside $derived so it re-runs when stored
  // changes. OS-level changes are handled by an event listener in App.svelte
  // that writes back to stored==='system' branch by re-triggering this.
  const resolved = $derived<'light' | 'dark' | 'sepia'>(
    stored === 'system'
      ? (mediaPrefersDark() ? 'dark' : 'light')
      : stored
  );

  return {
    get stored() { return stored; },
    set stored(v: Theme) {
      stored = v;
      localStorage.setItem('tap.theme', v);
    },
    get resolved() { return resolved; },
  };
}

function makePref<T extends string>(key: string, defaultValue: T) {
  let value = $state<T>(
    (localStorage.getItem(key) as T) ?? defaultValue
  );
  return {
    get value() { return value; },
    set value(v: T) {
      value = v;
      localStorage.setItem(key, v);
    },
  };
}

export const theme = makeTheme();
export const font = makePref<Font>('tap.font', 'serif');
export const density = makePref<Density>('tap.density', 'default');
```

- [ ] **Step 4: Run tests — expect all to pass**

```bash
cd web && pnpm test -- src/lib/__tests__/preferences.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/preferences.svelte.ts web/src/lib/__tests__/preferences.test.ts
git commit -m "feat(web): theme/font/density preference store with localStorage persistence"
```

---

## Task 3: Theme + density + font CSS tokens

**Skills:** invoke `svelte-styling` before implementing.

**Files:**
- Modify: `web/src/styles/tokens.css`
- Modify: `web/src/styles/global.css`

- [ ] **Step 1: Add theme blocks and utility classes to `web/src/styles/tokens.css`**

Append the following to the end of `web/src/styles/tokens.css` (keep the existing `:root` block unchanged):

```css
/* ── Dark theme ── */
html.theme-dark {
  --bg: #0d0d0e;
  --bg-soft: #161617;
  --surface: #1a1a1c;
  --ink: #ededea;
  --ink-2: #a8a8a4;
  --ink-3: #6e6e6a;
  --ink-4: #3a3a38;
  --rule: #232325;
  --accent: #5a7fdc;
  --accent-soft: rgba(90, 127, 220, 0.14);
}

/* ── Sepia theme ── */
html.theme-sepia {
  --bg: #f4ecd8;
  --bg-soft: #ece3c9;
  --surface: #faf3df;
  --ink: #3b2e1a;
  --ink-2: #6b5a3e;
  --ink-3: #9e8d6b;
  --ink-4: #c8b88f;
  --rule: #d9cca8;
  --accent: #002FA7;
  --accent-soft: rgba(0, 47, 167, 0.10);
}

/* ── Font toggle ── */
html.font-sans {
  --serif: var(--sans);
}

/* ── Density ── */
html.density-compact .entry { padding-top: 10px; padding-bottom: 10px; }
html.density-compact .entry .summary { display: none; }
html.density-compact .entry .junction { top: 17px; }
html.density-compact .entry .saved-mark { top: 13px; }
html.density-comfortable .entry { padding-top: 16px; padding-bottom: 16px; }

/* ── Reduced motion ── */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    transition-duration: 0ms !important;
    animation-duration: 0ms !important;
    animation-iteration-count: 1 !important;
  }
}
```

- [ ] **Step 2: Add full design-system component styles to `web/src/styles/global.css`**

Append the content of `ui_design/styles.css` (excluding the `@import` Google Fonts line, and excluding the `:root` block which is already in `tokens.css`) to the end of `web/src/styles/global.css`. The key sections to include are:

- `.tap-mark` / `.wordmark`
- `.entry` and its variants (`.is-read`, `.is-selected`, `.is-saved`, `.junction`, `.title`, `.meta`, `.summary`)
- `.tap-topbar`
- `.tap-sidebar` and all sub-rules
- `.tap-unread`
- `.poll-strip` + `@keyframes tap-pulse`
- `.kbd`
- `.m-topbar`, `.m-tabbar`, `.m-tabbar .tab`
- `.is-mobile .entry` overrides
- `.reader-rail`, `.reader-pane`, `.reader-header`, `.reader-back`, `.reader-actions`, `.reader-action`, `.reader-body`, `.reader-source`, `.reader-title`, `.reader-byline`, `.reader-rule`, `.reader-lede`, `.reader-p`, `.reader-h2`, `.reader-pre`, `.reader-end`, `.reader-foot`
- `.m-reader-topbar`, `.m-reader-footbar`
- `.tap-modal`, `.tap-modal-head`, `.tap-modal-close`, `.tap-modal-body`
- `.shortcut-group-title`, `.shortcut-row`, `.shortcut-keys`, `.shortcut-plus`, `.shortcut-desc`

Run `pnpm --dir web build` to confirm no CSS errors.

- [ ] **Step 3: Verify build clean**

```bash
cd web && pnpm build && pnpm check
```

Expected: zero errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/styles/tokens.css web/src/styles/global.css
git commit -m "feat(web): add dark/sepia theme tokens, font-sans override, density classes, prefers-reduced-motion"
```

---

## Task 4: Wire theme/font/density classes to `<html>` in `App.svelte`

**Skills:** invoke `svelte-runes`, `svelte-template-directives` before implementing.

**Files:**
- Modify: `web/src/App.svelte`

The `<html>` element is outside the Svelte component tree, so we use a `$effect` to imperatively set its `className`. We also attach a `matchMedia` listener to re-resolve when the OS theme changes.

- [ ] **Step 1: Update `web/src/App.svelte`**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { route } from './lib/router';
  import { auth } from './lib/auth';
  import { theme, font, density } from './lib/preferences.svelte';
  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';

  onMount(() => {
    void auth.bootstrap();

    // Re-trigger theme resolution when OS preference changes.
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const onMQChange = () => {
      // Touch stored to force $derived re-evaluation (no-op if not 'system').
      if (theme.stored === 'system') {
        // Re-write 'system' to itself to invalidate derived.
        theme.stored = 'system';
      }
    };
    mq.addEventListener('change', onMQChange);
    return () => mq.removeEventListener('change', onMQChange);
  });

  // Apply theme/font/density classes to <html> reactively.
  $effect(() => {
    const html = document.documentElement;
    // Theme: remove all theme classes, add resolved one.
    html.classList.remove('theme-light', 'theme-dark', 'theme-sepia');
    html.classList.add(`theme-${theme.resolved}`);
  });

  $effect(() => {
    document.documentElement.classList.toggle('font-sans', font.value === 'sans');
  });

  $effect(() => {
    const html = document.documentElement;
    html.classList.remove('density-compact', 'density-comfortable');
    if (density.value !== 'default') {
      html.classList.add(`density-${density.value}`);
    }
  });
</script>

{#if !$auth.bootstrapped}
  <!-- empty during bootstrap -->
{:else if $auth.user == null}
  <Login />
{:else if $route.name === 'reader'}
  <Reader id={$route.params.id} />
{:else}
  <Unread />
{/if}
```

- [ ] **Step 2: Run check**

```bash
cd web && pnpm check
```

Expected: no TypeScript errors.

- [ ] **Step 3: Manually verify theme switching**

Run `make dev`, open `http://localhost:5173`. Open browser console and run:

```javascript
localStorage.setItem('tap.theme', 'dark'); location.reload();
```

Expected: page reloads with dark background (`#0d0d0e`). Then:

```javascript
localStorage.setItem('tap.theme', 'sepia'); location.reload();
```

Expected: warm sepia background. Then:

```javascript
localStorage.removeItem('tap.theme'); location.reload();
```

Expected: resolves to light or dark based on OS setting.

- [ ] **Step 4: Commit**

```bash
git add web/src/App.svelte
git commit -m "feat(web): apply theme/font/density classes to <html> reactively"
```

---

## Task 5: Keyboard shortcut handler

**Skills:** invoke `svelte-runes`, `svelte-template-directives` before implementing.

**Files:**
- Create: `web/src/lib/keyboard.ts`
- Create: `web/src/lib/__tests__/keyboard.test.ts`
- Modify: `web/src/App.svelte`

- [ ] **Step 1: Write failing tests**

Create `web/src/lib/__tests__/keyboard.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { isFormControl, buildHandler } from '../keyboard';

describe('isFormControl', () => {
  it('returns true for INPUT elements', () => {
    const el = document.createElement('input');
    expect(isFormControl(el)).toBe(true);
  });

  it('returns true for TEXTAREA', () => {
    const el = document.createElement('textarea');
    expect(isFormControl(el)).toBe(true);
  });

  it('returns true for SELECT', () => {
    const el = document.createElement('select');
    expect(isFormControl(el)).toBe(true);
  });

  it('returns true for contenteditable elements', () => {
    const el = document.createElement('div');
    el.setAttribute('contenteditable', 'true');
    expect(isFormControl(el)).toBe(true);
  });

  it('returns false for a plain div', () => {
    const el = document.createElement('div');
    expect(isFormControl(el)).toBe(false);
  });

  it('returns false for a button', () => {
    const el = document.createElement('button');
    expect(isFormControl(el)).toBe(false);
  });
});

describe('buildHandler', () => {
  const makeEvent = (key: string, target?: Element) => {
    const el = target ?? document.createElement('div');
    return Object.assign(new KeyboardEvent('keydown', { key, bubbles: true }), {
      target: el,
    });
  };

  it('calls onNext for j', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    handler(makeEvent('j'));
    expect(ctx.onNext).toHaveBeenCalledOnce();
  });

  it('calls onNext for ArrowDown', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    handler(makeEvent('ArrowDown'));
    expect(ctx.onNext).toHaveBeenCalledOnce();
  });

  it('calls onPrev for k', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    handler(makeEvent('k'));
    expect(ctx.onPrev).toHaveBeenCalledOnce();
  });

  it('calls onToggleRead for m', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    handler(makeEvent('m'));
    expect(ctx.onToggleRead).toHaveBeenCalledOnce();
  });

  it('calls onToggleSaved for s', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    handler(makeEvent('s'));
    expect(ctx.onToggleSaved).toHaveBeenCalledOnce();
  });

  it('calls onOpen for o and Enter', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    handler(makeEvent('o'));
    handler(makeEvent('Enter'));
    expect(ctx.onOpen).toHaveBeenCalledTimes(2);
  });

  it('calls onViewOriginal for v', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    handler(makeEvent('v'));
    expect(ctx.onViewOriginal).toHaveBeenCalledOnce();
  });

  it('calls setModalOpen(true) for ?', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    handler(makeEvent('?'));
    expect(ctx.setModalOpen).toHaveBeenCalledWith(true);
  });

  it('calls onEscape for Escape', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    handler(makeEvent('Escape'));
    expect(ctx.onEscape).toHaveBeenCalledOnce();
  });

  it('suppresses all bindings when target is an INPUT', () => {
    const ctx = { onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(), onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(), onEscape: vi.fn(), setModalOpen: vi.fn() };
    const handler = buildHandler(ctx);
    const input = document.createElement('input');
    handler(makeEvent('j', input));
    handler(makeEvent('m', input));
    handler(makeEvent('?', input));
    expect(ctx.onNext).not.toHaveBeenCalled();
    expect(ctx.onToggleRead).not.toHaveBeenCalled();
    expect(ctx.setModalOpen).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run tests — expect failures**

```bash
cd web && pnpm test -- src/lib/__tests__/keyboard.test.ts
```

Expected: fail with "Cannot find module '../keyboard'".

- [ ] **Step 3: Implement `web/src/lib/keyboard.ts`**

```typescript
export interface KeyboardContext {
  onNext: () => void;
  onPrev: () => void;
  onOpen: () => void;
  onToggleRead: () => void;
  onToggleSaved: () => void;
  onViewOriginal: () => void;
  onEscape: () => void;
  setModalOpen: (open: boolean) => void;
}

const FORM_TAGS = new Set(['INPUT', 'TEXTAREA', 'SELECT']);

export function isFormControl(el: Element): boolean {
  if (FORM_TAGS.has(el.tagName)) return true;
  return el.getAttribute('contenteditable') !== null;
}

export function buildHandler(ctx: KeyboardContext) {
  return (e: KeyboardEvent) => {
    if (e.target instanceof Element && isFormControl(e.target)) return;
    switch (e.key) {
      case 'j': case 'ArrowDown': ctx.onNext(); break;
      case 'k': case 'ArrowUp':   ctx.onPrev(); break;
      case 'o': case 'Enter':     ctx.onOpen(); break;
      case 'm':                   ctx.onToggleRead(); break;
      case 's':                   ctx.onToggleSaved(); break;
      case 'v':                   ctx.onViewOriginal(); break;
      case 'Escape':              ctx.onEscape(); break;
      case '?':                   ctx.setModalOpen(true); break;
    }
  };
}
```

- [ ] **Step 4: Run tests — expect all to pass**

```bash
cd web && pnpm test -- src/lib/__tests__/keyboard.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/keyboard.ts web/src/lib/__tests__/keyboard.test.ts
git commit -m "feat(web): keyboard handler with form-control suppression"
```

---

## Task 6: Hotkeys modal + wire keyboard handler into `App.svelte`

**Skills:** invoke `svelte-runes`, `svelte-styling` before implementing.

**Files:**
- Create: `web/src/components/HotkeysModal.svelte`
- Modify: `web/src/App.svelte`

- [ ] **Step 1: Create `web/src/components/HotkeysModal.svelte`**

```svelte
<script lang="ts">
  type Props = { open: boolean; onClose: () => void };
  let { open, onClose }: Props = $props();

  // Close on backdrop click.
  function onScrimClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="tap-modal-scrim" role="dialog" aria-modal="true"
       aria-labelledby="hotkeys-title" onclick={onScrimClick}>
    <div class="tap-modal">
      <div class="tap-modal-head">
        <span id="hotkeys-title" class="modal-title">Keyboard shortcuts</span>
        <button class="tap-modal-close" onclick={onClose} aria-label="Close">✕</button>
      </div>
      <div class="tap-modal-body">
        <div>
          <div class="shortcut-group-title">Navigation</div>
          <div class="shortcut-row">
            <span class="shortcut-desc">Next entry</span>
            <span class="shortcut-keys"><kbd class="kbd">j</kbd><span class="shortcut-plus">/</span><kbd class="kbd">↓</kbd></span>
          </div>
          <div class="shortcut-row">
            <span class="shortcut-desc">Previous entry</span>
            <span class="shortcut-keys"><kbd class="kbd">k</kbd><span class="shortcut-plus">/</span><kbd class="kbd">↑</kbd></span>
          </div>
          <div class="shortcut-row">
            <span class="shortcut-desc">Open entry</span>
            <span class="shortcut-keys"><kbd class="kbd">o</kbd><span class="shortcut-plus">/</span><kbd class="kbd">↵</kbd></span>
          </div>
          <div class="shortcut-row">
            <span class="shortcut-desc">Back to list</span>
            <span class="shortcut-keys"><kbd class="kbd">Esc</kbd></span>
          </div>
        </div>
        <div>
          <div class="shortcut-group-title">Actions</div>
          <div class="shortcut-row">
            <span class="shortcut-desc">Toggle read</span>
            <span class="shortcut-keys"><kbd class="kbd">m</kbd></span>
          </div>
          <div class="shortcut-row">
            <span class="shortcut-desc">Toggle saved</span>
            <span class="shortcut-keys"><kbd class="kbd">s</kbd></span>
          </div>
          <div class="shortcut-row">
            <span class="shortcut-desc">View original</span>
            <span class="shortcut-keys"><kbd class="kbd">v</kbd></span>
          </div>
          <div class="shortcut-row">
            <span class="shortcut-desc">This modal</span>
            <span class="shortcut-keys"><kbd class="kbd">?</kbd></span>
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-title {
    font-family: var(--sans);
    font-size: 13px;
    font-weight: 600;
    color: var(--ink);
  }
</style>
```

- [ ] **Step 2: Wire keyboard handler and hotkeys modal into `App.svelte`**

Update `web/src/App.svelte` (keep all existing code, add the highlighted sections):

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { route, navigate } from './lib/router';
  import { auth } from './lib/auth';
  import { theme, font, density } from './lib/preferences.svelte';
  import { buildHandler } from './lib/keyboard';
  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';
  import HotkeysModal from './components/HotkeysModal.svelte';

  let hotkeysOpen = $state(false);

  // Keyboard context — no-ops for actions that depend on the active view.
  // Unread.svelte and Reader.svelte provide their own context overrides via
  // Svelte context so the handler can dispatch to the mounted view.
  const keyCtx = {
    onNext: () => {},
    onPrev: () => {},
    onOpen: () => {},
    onToggleRead: () => {},
    onToggleSaved: () => {},
    onViewOriginal: () => {},
    onEscape: () => {
      if (hotkeysOpen) { hotkeysOpen = false; return; }
      if ($route.name === 'reader') navigate('/');
    },
    setModalOpen: (open: boolean) => { hotkeysOpen = open; },
  };

  const keyHandler = buildHandler(keyCtx);

  onMount(() => {
    void auth.bootstrap();

    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const onMQChange = () => { if (theme.stored === 'system') theme.stored = 'system'; };
    mq.addEventListener('change', onMQChange);
    return () => mq.removeEventListener('change', onMQChange);
  });

  $effect(() => {
    const html = document.documentElement;
    html.classList.remove('theme-light', 'theme-dark', 'theme-sepia');
    html.classList.add(`theme-${theme.resolved}`);
  });

  $effect(() => {
    document.documentElement.classList.toggle('font-sans', font.value === 'sans');
  });

  $effect(() => {
    const html = document.documentElement;
    html.classList.remove('density-compact', 'density-comfortable');
    if (density.value !== 'default') html.classList.add(`density-${density.value}`);
  });
</script>

<svelte:window onkeydown={keyHandler} />

<HotkeysModal open={hotkeysOpen} onClose={() => { hotkeysOpen = false; }} />

{#if !$auth.bootstrapped}
  <!-- empty during bootstrap -->
{:else if $auth.user == null}
  <Login />
{:else if $route.name === 'reader'}
  <Reader id={$route.params.id} />
{:else}
  <Unread />
{/if}
```

- [ ] **Step 3: Run check + test**

```bash
cd web && pnpm check && pnpm test
```

Expected: no errors, all existing tests still pass.

- [ ] **Step 4: Manual smoke test**

Run `make dev`, open app, press `?` — hotkeys modal should appear. Press `Esc` — should close. Pressing `j`/`k` in a text input should not fire.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/HotkeysModal.svelte web/src/App.svelte
git commit -m "feat(web): hotkeys modal and document-level keyboard handler"
```

---

## Task 7: Swipe gesture attachment

**Skills:** invoke `svelte-template-directives` before implementing.

**Files:**
- Create: `web/src/lib/swipe.ts`
- Create: `web/src/lib/__tests__/swipe.test.ts`

- [ ] **Step 1: Write failing tests**

Create `web/src/lib/__tests__/swipe.test.ts`:

```typescript
import { describe, it, expect, vi } from 'vitest';
import { recogniseSwipe } from '../swipe';

// recogniseSwipe(dx, dy, startX): 'left' | 'right' | null
// dx = horizontal delta (positive = right), dy = vertical delta, startX = starting clientX

describe('recogniseSwipe', () => {
  it('returns "right" for sufficient rightward horizontal swipe', () => {
    expect(recogniseSwipe(50, 5, 50)).toBe('right');
  });

  it('returns "left" for sufficient leftward horizontal swipe', () => {
    expect(recogniseSwipe(-50, 5, 50)).toBe('left');
  });

  it('returns null when travel < 40px', () => {
    expect(recogniseSwipe(39, 2, 50)).toBeNull();
    expect(recogniseSwipe(-39, 2, 50)).toBeNull();
  });

  it('returns null when angle >= 30 degrees from horizontal', () => {
    // At 40px horizontal, angle is atan(dy/40)*180/PI.
    // tan(30°) ≈ 0.577, so dy = 40 * 0.577 ≈ 23.1 triggers the gate.
    expect(recogniseSwipe(40, 24, 50)).toBeNull();
  });

  it('returns direction when angle < 30 degrees', () => {
    // dy = 22 < 23.1 threshold → angle just under 30°
    expect(recogniseSwipe(40, 22, 50)).toBe('right');
  });

  it('returns null when startX <= 20 (edge swipe guard)', () => {
    expect(recogniseSwipe(50, 5, 15)).toBeNull();
  });

  it('returns direction when startX > 20', () => {
    expect(recogniseSwipe(50, 5, 21)).toBe('right');
  });
});
```

- [ ] **Step 2: Run tests — expect failures**

```bash
cd web && pnpm test -- src/lib/__tests__/swipe.test.ts
```

Expected: fail with "Cannot find module '../swipe'".

- [ ] **Step 3: Implement `web/src/lib/swipe.ts`**

```typescript
const MIN_TRAVEL = 40;
const MAX_ANGLE_DEG = 30;
const EDGE_GUARD_PX = 20;

export function recogniseSwipe(
  dx: number,
  dy: number,
  startX: number,
): 'left' | 'right' | null {
  if (startX <= EDGE_GUARD_PX) return null;
  const absDx = Math.abs(dx);
  if (absDx < MIN_TRAVEL) return null;
  const angleDeg = Math.atan2(Math.abs(dy), absDx) * (180 / Math.PI);
  if (angleDeg >= MAX_ANGLE_DEG) return null;
  return dx > 0 ? 'right' : 'left';
}

export interface SwipeOptions {
  onSwipeLeft?: () => void;
  onSwipeRight?: () => void;
}

// Svelte {@attach} compatible attachment function.
// Usage: <div {@attach swipe({ onSwipeLeft: ..., onSwipeRight: ... })}>
export function swipe(opts: SwipeOptions) {
  return (el: Element) => {
    let startX = 0;
    let startY = 0;

    function onTouchStart(e: TouchEvent) {
      const t = e.touches[0];
      startX = t.clientX;
      startY = t.clientY;
    }

    function onTouchEnd(e: TouchEvent) {
      const t = e.changedTouches[0];
      const dx = t.clientX - startX;
      const dy = t.clientY - startY;
      const dir = recogniseSwipe(dx, dy, startX);
      if (dir === 'left') opts.onSwipeLeft?.();
      if (dir === 'right') opts.onSwipeRight?.();
    }

    el.addEventListener('touchstart', onTouchStart as EventListener, { passive: true });
    el.addEventListener('touchend', onTouchEnd as EventListener, { passive: true });

    return () => {
      el.removeEventListener('touchstart', onTouchStart as EventListener);
      el.removeEventListener('touchend', onTouchEnd as EventListener);
    };
  };
}
```

- [ ] **Step 4: Run tests — expect all to pass**

```bash
cd web && pnpm test -- src/lib/__tests__/swipe.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/swipe.ts web/src/lib/__tests__/swipe.test.ts
git commit -m "feat(web): swipe gesture recogniser with angle gate and edge guard"
```

---

## Task 8: Pull-to-refresh attachment

**Skills:** invoke `svelte-template-directives` before implementing.

**Files:**
- Create: `web/src/lib/pulltorefresh.ts`
- Create: `web/src/lib/__tests__/pulltorefresh.test.ts`

- [ ] **Step 1: Write failing tests**

Create `web/src/lib/__tests__/pulltorefresh.test.ts`:

```typescript
import { describe, it, expect, vi } from 'vitest';
import { recognisePull } from '../pulltorefresh';

// recognisePull(dy, scrollTop, inFlight): boolean
// dy = vertical drag distance (positive = pulled down), scrollTop = element scroll position

describe('recognisePull', () => {
  it('returns true when dy >= 60 and scrollTop === 0 and not in flight', () => {
    expect(recognisePull(60, 0, false)).toBe(true);
  });

  it('returns false when dy < 60', () => {
    expect(recognisePull(59, 0, false)).toBe(false);
  });

  it('returns false when scrollTop > 0 (not at top)', () => {
    expect(recognisePull(80, 10, false)).toBe(false);
  });

  it('returns false when in flight', () => {
    expect(recognisePull(80, 0, true)).toBe(false);
  });
});
```

- [ ] **Step 2: Run tests — expect failures**

```bash
cd web && pnpm test -- src/lib/__tests__/pulltorefresh.test.ts
```

Expected: fail with "Cannot find module '../pulltorefresh'".

- [ ] **Step 3: Implement `web/src/lib/pulltorefresh.ts`**

```typescript
const THRESHOLD_PX = 60;

export function recognisePull(dy: number, scrollTop: number, inFlight: boolean): boolean {
  return !inFlight && scrollTop === 0 && dy >= THRESHOLD_PX;
}

export interface PullToRefreshOptions {
  onRefresh: () => Promise<void>;
  getScrollTop: () => number;
}

// Svelte {@attach} compatible attachment function.
// Usage: <div {@attach pullToRefresh({ onRefresh: ..., getScrollTop: ... })}>
export function pullToRefresh(opts: PullToRefreshOptions) {
  return (el: Element) => {
    let startY = 0;
    let inFlight = false;

    function onTouchStart(e: TouchEvent) {
      startY = e.touches[0].clientY;
    }

    async function onTouchEnd(e: TouchEvent) {
      const dy = e.changedTouches[0].clientY - startY;
      const scrollTop = opts.getScrollTop();
      if (!recognisePull(dy, scrollTop, inFlight)) return;
      inFlight = true;
      try {
        await opts.onRefresh();
      } finally {
        inFlight = false;
      }
    }

    el.addEventListener('touchstart', onTouchStart as EventListener, { passive: true });
    el.addEventListener('touchend', onTouchEnd as EventListener, { passive: true });

    return () => {
      el.removeEventListener('touchstart', onTouchStart as EventListener);
      el.removeEventListener('touchend', onTouchEnd as EventListener);
    };
  };
}
```

- [ ] **Step 4: Run tests — expect all to pass**

```bash
cd web && pnpm test -- src/lib/__tests__/pulltorefresh.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/pulltorefresh.ts web/src/lib/__tests__/pulltorefresh.test.ts
git commit -m "feat(web): pull-to-refresh attachment with threshold and in-flight guard"
```

---

## Task 9: Desktop reader layout — sidebar visible

**Skills:** invoke `svelte-runes`, `svelte-styling` before implementing.

**Files:**
- Modify: `web/src/views/Reader.svelte`
- Modify: `web/src/components/Sidebar.svelte`

The reader currently hides the sidebar (it's a standalone `div.reader`). We change it to a two-column layout: sidebar (240px) + reader pane (flex), matching the Unread view's `.layout` pattern.

- [ ] **Step 1: Update `Reader.svelte` layout**

Replace the outermost `<div class="reader">` wrapper and `<style>` block. Keep the inner header/article content the same; only the layout shell changes:

```svelte
<script lang="ts">
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import { entries } from '../lib/store';
  import type { EntryDetail } from '../lib/types';
  import FeedAvatar from '../components/FeedAvatar.svelte';
  import JunctionDot from '../components/JunctionDot.svelte';
  import Sidebar from '../components/Sidebar.svelte';

  type Props = { id: number };
  let { id }: Props = $props();

  let entry = $state<EntryDetail | null>(null);
  let error = $state<string | null>(null);
  let saved = $derived(entry?.saved ?? false);

  $effect(() => {
    const targetId = id;
    entry = null;
    error = null;
    let cancelled = false;
    (async () => {
      try {
        const fetched = await api.getEntry(targetId);
        if (cancelled) return;
        entry = fetched;
        if (fetched && !fetched.read) {
          try {
            await api.patchEntry(targetId, { read: true });
            if (cancelled) return;
            entry = { ...fetched, read: true };
          } catch { /* swallow */ }
        }
      } catch (e) {
        if (cancelled) return;
        error = (e as Error).message;
      }
    })();
    return () => { cancelled = true; };
  });

  async function toggleRead() {
    if (!entry) return;
    const want = !entry.read;
    try {
      await api.patchEntry(entry.id, { read: want });
      entry = { ...entry, read: want };
      entries.toggleRead(entry.id, want);
    } catch (e) { error = (e as Error).message; }
  }

  async function toggleSaved() {
    if (!entry) return;
    const want = !entry.saved;
    try {
      await api.patchEntry(entry.id, { saved: want });
      entry = { ...entry, saved: want };
    } catch (e) { error = (e as Error).message; }
  }

  function host(url: string): string {
    try { return new URL(url).host; } catch { return ''; }
  }
</script>

<div class="layout">
  <Sidebar />
  <div class="reader-pane">
    <header class="reader-header">
      <button class="reader-back" onclick={() => navigate('/')} aria-label="Back to unread">
        ‹ <span>UNREAD</span>
      </button>
      {#if entry}
        <div class="reader-actions">
          <button class="reader-action" onclick={toggleRead}>
            {entry.read ? 'MARK UNREAD' : 'MARK READ'}
          </button>
          <button class="reader-action" class:is-saved={saved} onclick={toggleSaved}>
            {saved ? 'SAVED' : 'SAVE'}
          </button>
          <a class="reader-action" href={entry.url} target="_blank" rel="noopener">VIEW ORIGINAL</a>
        </div>
      {/if}
    </header>

    <article class="reader-body">
      {#if error}
        <p class="err">{error}</p>
      {:else if !entry}
        <p class="loading">Loading…</p>
      {:else}
        <div class="reader-source">
          <FeedAvatar feedURL={host(entry.url)} size={10} radius={2} />
          <span class="src-name">{host(entry.url)}</span>
          <span class="src-sep"></span>
          <span class="src-url">{host(entry.url)}</span>
        </div>
        <h1 class="reader-title">{entry.title}</h1>
        <div class="reader-byline">
          {#if entry.author}<span>{entry.author}</span><span aria-hidden="true"> · </span>{/if}
          <span>{new Date(entry.published_at * 1000).toLocaleDateString()}</span>
        </div>
        <div class="reader-rule">
          <span class="reader-rule-line"></span>
          <JunctionDot color="var(--accent)" />
          <span class="reader-rule-line"></span>
        </div>
        <div class="content">{@html entry.content}</div>
        <div class="reader-end">
          <span class="reader-end-line"></span>
          <span class="reader-end-dot"></span>
          <span class="reader-end-line"></span>
        </div>
      {/if}
    </article>
  </div>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .reader-pane {
    flex: 1; display: flex; flex-direction: column;
    min-width: 0; background: var(--bg); overflow: hidden;
  }
  .err { color: #b14; font-family: var(--mono); font-size: 12px; }
  .loading { color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .content :global(p) {
    font-family: var(--serif); font-size: 17px; line-height: 1.7;
    color: var(--ink); margin: 0 0 22px; text-wrap: pretty;
  }
  .content :global(h2) {
    font-family: var(--serif); font-size: 22px; line-height: 1.25;
    font-weight: 600; letter-spacing: -0.01em; margin: 40px 0 14px;
  }
  .content :global(pre), .content :global(code) {
    font-family: var(--mono); font-size: 13px; line-height: 1.55;
    color: var(--ink-2); background: var(--bg-soft);
    padding: 14px 16px; border-left: 2px solid var(--accent); overflow-x: auto;
  }
</style>
```

- [ ] **Step 2: Run check**

```bash
cd web && pnpm check
```

Expected: no TypeScript errors.

- [ ] **Step 3: Visual smoke test**

Run `make dev`, navigate to an entry. Sidebar should be visible on the left, reader content on the right, sidebar does not scroll with the article.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/Reader.svelte
git commit -m "feat(web): desktop reader layout — sidebar visible, content front and centre"
```

---

## Task 10: Mobile layout — tab bar, top bars, breakpoints

**Skills:** invoke `svelte-runes`, `svelte-styling`, `svelte-template-directives` before implementing.

**Files:**
- Create: `web/src/components/TabBar.svelte`
- Create: `web/src/views/Saved.svelte`
- Create: `web/src/views/Search.svelte`
- Modify: `web/src/App.svelte`
- Modify: `web/src/views/Unread.svelte`
- Modify: `web/src/views/Reader.svelte`
- Modify: `web/src/components/Sidebar.svelte`
- Modify: `web/src/lib/router.ts`

- [ ] **Step 1: Add `/saved` and `/search` routes to `web/src/lib/router.ts`**

```typescript
import { writable, type Readable } from 'svelte/store';

type RouteState =
  | { name: 'unread' }
  | { name: 'reader'; params: { id: number } }
  | { name: 'saved' }
  | { name: 'search' }
  | { name: 'settings' };

function parse(pathname: string): RouteState {
  const m = pathname.match(/^\/entry\/(\d+)$/);
  if (m) return { name: 'reader', params: { id: Number(m[1]) } };
  if (pathname === '/saved') return { name: 'saved' };
  if (pathname === '/search') return { name: 'search' };
  if (pathname === '/settings') return { name: 'settings' };
  return { name: 'unread' };
}

const internal = writable<RouteState>(parse(window.location.pathname));
window.addEventListener('popstate', () => internal.set(parse(window.location.pathname)));
export const route: Readable<RouteState> = { subscribe: internal.subscribe };

export function navigate(to: string) {
  if (window.location.pathname === to) return;
  window.history.pushState({}, '', to);
  internal.set(parse(to));
}
```

- [ ] **Step 2: Create stub views**

Create `web/src/views/Saved.svelte`:

```svelte
<script lang="ts">
  import Sidebar from '../components/Sidebar.svelte';
</script>

<div class="layout">
  <Sidebar />
  <main class="main">
    <p class="empty">Saved entries will appear here.</p>
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .main { flex: 1; display: flex; align-items: center; justify-content: center; background: var(--bg); }
  .empty { color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
</style>
```

Create `web/src/views/Search.svelte`:

```svelte
<script lang="ts">
  import Sidebar from '../components/Sidebar.svelte';
</script>

<div class="layout">
  <Sidebar />
  <main class="main">
    <p class="empty">Search coming in M9.</p>
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .main { flex: 1; display: flex; align-items: center; justify-content: center; background: var(--bg); }
  .empty { color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
</style>
```

- [ ] **Step 3: Create `web/src/components/TabBar.svelte`**

```svelte
<script lang="ts">
  import { route, navigate } from '../lib/router';

  const tabs = [
    { name: 'unread',   label: 'Unread',   path: '/',        icon: 'list' },
    { name: 'saved',    label: 'Saved',    path: '/saved',   icon: 'bookmark' },
    { name: 'search',   label: 'Search',   path: '/search',  icon: 'search' },
    { name: 'settings', label: 'Settings', path: '/settings',icon: 'settings' },
  ] as const;

  // Map reader route → unread for active-tab highlighting.
  const activeTab = $derived(
    $route.name === 'reader' ? 'unread' : $route.name
  );
</script>

<nav class="m-tabbar" aria-label="Main navigation">
  {#each tabs as tab (tab.name)}
    <button
      class="tab"
      class:active={activeTab === tab.name}
      onclick={() => navigate(tab.path)}
      aria-current={activeTab === tab.name ? 'page' : undefined}
      aria-label={tab.label}
    >
      <span class="ico" aria-hidden="true">
        {#if tab.icon === 'list'}☰{:else if tab.icon === 'bookmark'}⊟{:else if tab.icon === 'search'}⌕{:else}⚙{/if}
      </span>
      <span>{tab.label}</span>
    </button>
  {/each}
</nav>

<style>
  /* Icons are placeholder text glyphs — replace with SVG in a follow-up. */
  .ico { font-size: 16px; }
</style>
```

Note: the icon glyphs above are placeholders. The final implementation should use the inline SVG icons from `ui_design/tap-components.jsx` (`Icon` object). Extract SVGs as Svelte snippets or inline them per tab in a follow-up pass within this task.

- [ ] **Step 4: Wire mobile layout and tab bar in `App.svelte`**

Add a reactive `isMobile` boolean and render `TabBar` below the active view when on mobile. The breakpoint is 768px, tracked via `window.matchMedia`.

Update `App.svelte` (keep all existing code, add the following):

```svelte
<script lang="ts">
  // ... (all existing imports and code) ...
  import TabBar from './components/TabBar.svelte';
  import Saved from './views/Saved.svelte';
  import Search from './views/Search.svelte';
  import Settings from './views/Settings.svelte';

  let isMobile = $state(
    typeof window !== 'undefined' && window.matchMedia('(max-width: 768px)').matches
  );

  onMount(() => {
    // ... existing onMount code ...
    const mq768 = window.matchMedia('(max-width: 768px)');
    const onResize = (e: MediaQueryListEvent) => { isMobile = e.matches; };
    mq768.addEventListener('change', onResize);
    return () => mq768.removeEventListener('change', onResize);
  });
</script>

<!-- In the template, replace the existing view routing with: -->
<div class="app-shell" class:is-mobile={isMobile}>
  {#if !$auth.bootstrapped}
    <!-- empty -->
  {:else if $auth.user == null}
    <Login />
  {:else if $route.name === 'reader'}
    <Reader id={$route.params.id} />
  {:else if $route.name === 'saved'}
    <Saved />
  {:else if $route.name === 'search'}
    <Search />
  {:else if $route.name === 'settings'}
    <Settings />
  {:else}
    <Unread />
  {/if}
  {#if isMobile && $auth.user != null && $auth.bootstrapped}
    <TabBar />
  {/if}
</div>
```

Add to `App.svelte` `<style>`:

```css
:global(.app-shell) {
  display: flex;
  flex-direction: column;
  height: 100vh;
}
```

- [ ] **Step 5: Hide sidebar on mobile**

In `web/src/components/Sidebar.svelte`, add to the `<style>` block:

```css
@media (max-width: 768px) {
  :global(.tap-sidebar) { display: none; }
}
```

Or add a `class:hidden` prop driven by `isMobile` passed from App. The simpler approach is a CSS media query in `global.css`:

In `web/src/styles/global.css`, append:

```css
@media (max-width: 768px) {
  .tap-sidebar { display: none !important; }
  .app-shell { padding-bottom: 64px; } /* room for tab bar */
}
```

- [ ] **Step 6: Run check**

```bash
cd web && pnpm check
```

Expected: no TypeScript errors.

- [ ] **Step 7: Visual smoke test on mobile viewport**

Run `make dev`, open DevTools, set viewport to 390×844. Sidebar should be hidden; tab bar should appear at the bottom with four tabs. Navigate between tabs.

- [ ] **Step 8: Commit**

```bash
git add web/src/lib/router.ts web/src/views/Saved.svelte web/src/views/Search.svelte \
        web/src/components/TabBar.svelte web/src/views/Settings.svelte \
        web/src/App.svelte web/src/components/Sidebar.svelte web/src/styles/global.css
git commit -m "feat(web): mobile layout — tab bar, breakpoint, stub views for saved/search/settings"
```

---

## Task 11: Swipe gestures on entry rows and reader

**Skills:** invoke `svelte-template-directives` before implementing.

**Files:**
- Modify: `web/src/components/EntryRow.svelte`
- Modify: `web/src/views/Reader.svelte`

- [ ] **Step 1: Wire swipe on `EntryRow.svelte`**

Add `{@attach}` directive to the entry button. Import `swipe` from the lib:

```svelte
<script lang="ts">
  import { swipe } from '../lib/swipe';
  // ... existing imports ...

  type Props = {
    entry: EntryListItem;
    feed: Subscription | undefined;
    onToggleRead?: () => void;
    onToggleSaved?: () => void;
  };
  let { entry, feed, onToggleRead, onToggleSaved }: Props = $props();
</script>

<button
  class="entry"
  class:is-read={entry.read}
  class:is-selected={false}
  onclick={() => navigate(`/entry/${entry.id}`)}
  {@attach swipe({
    onSwipeRight: onToggleRead,
    onSwipeLeft: onToggleSaved,
  })}
>
  <!-- existing inner content unchanged -->
</button>
```

- [ ] **Step 2: Wire swipe on `Reader.svelte` body for prev/next**

In `Reader.svelte`, add swipe to the `reader-body` article. Prev/next navigates by finding adjacent entry IDs in the `entries` store:

```svelte
<script lang="ts">
  import { swipe } from '../lib/swipe';
  import { entries } from '../lib/store';
  // ...

  function navigateRelative(delta: -1 | 1) {
    const items = $entries.items;
    const idx = items.findIndex(e => e.id === id);
    if (idx === -1) return;
    const next = items[idx + delta];
    if (next) navigate(`/entry/${next.id}`);
  }
</script>

<!-- On the reader-body article: -->
<article
  class="reader-body"
  {@attach swipe({
    onSwipeRight: () => navigateRelative(-1),
    onSwipeLeft:  () => navigateRelative(1),
  })}
>
  <!-- existing content unchanged -->
</article>
```

- [ ] **Step 3: Run check**

```bash
cd web && pnpm check
```

Expected: no TypeScript errors.

- [ ] **Step 4: Touch test on mobile viewport**

In DevTools touch simulation, swipe right on an entry row — entry should toggle read. Swipe left — should toggle saved. In reader, swipe left/right — should navigate between entries.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/EntryRow.svelte web/src/views/Reader.svelte
git commit -m "feat(web): swipe gestures — list row read/save, reader prev/next"
```

---

## Task 12: Pull-to-refresh on unread list

**Skills:** invoke `svelte-template-directives` before implementing.

**Files:**
- Modify: `web/src/views/Unread.svelte`

- [ ] **Step 1: Wire pull-to-refresh in `Unread.svelte`**

```svelte
<script lang="ts">
  import { pullToRefresh } from '../lib/pulltorefresh';
  // ... existing imports ...

  let listEl = $state<HTMLElement | null>(null);
  let refreshing = $state(false);

  async function doRefresh() {
    refreshing = true;
    try { await entries.load(true); }
    finally { refreshing = false; }
  }
</script>

<!-- Wrap the list div: -->
<div
  class="list"
  bind:this={listEl}
  {@attach pullToRefresh({
    onRefresh: doRefresh,
    getScrollTop: () => listEl?.scrollTop ?? 0,
  })}
>
  {#if refreshing}
    <div class="refresh-indicator">
      <span class="pulse"></span>
    </div>
  {/if}
  {#each $entries.items as entry (entry.id)}
    <EntryRow {entry} feed={feedFor(entry.subscription_id)} />
  {/each}
</div>
```

Add to `Unread.svelte` `<style>`:

```css
.refresh-indicator {
  display: flex;
  justify-content: center;
  padding: 12px 0;
}
.pulse {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--accent);
  animation: tap-pulse 2.4s ease-in-out infinite;
}
```

- [ ] **Step 2: Run check**

```bash
cd web && pnpm check
```

Expected: no TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/Unread.svelte
git commit -m "feat(web): pull-to-refresh on mobile unread list"
```

---

## Task 13: Settings view — Appearance section

**Skills:** invoke `svelte-runes`, `svelte-styling` before implementing.

**Files:**
- Create: `web/src/views/Settings.svelte`

- [ ] **Step 1: Create `web/src/views/Settings.svelte`**

```svelte
<script lang="ts">
  import Sidebar from '../components/Sidebar.svelte';
  import { theme, font, density } from '../lib/preferences.svelte';

  type Section = 'appearance' | 'security';
  let activeSection = $state<Section>('appearance');
</script>

<div class="layout">
  <Sidebar />
  <main class="settings-main">
    <header class="settings-header">
      <h1 class="settings-title">Settings</h1>
    </header>
    <div class="settings-body">
      <nav class="settings-nav" aria-label="Settings sections">
        <button
          class="settings-nav-item"
          class:active={activeSection === 'appearance'}
          onclick={() => activeSection = 'appearance'}
          aria-current={activeSection === 'appearance' ? 'true' : undefined}
        >Appearance</button>
        <button
          class="settings-nav-item"
          class:active={activeSection === 'security'}
          onclick={() => activeSection = 'security'}
          aria-current={activeSection === 'security' ? 'true' : undefined}
        >Security</button>
      </nav>

      <div class="settings-content">
        {#if activeSection === 'appearance'}
          <section aria-labelledby="appearance-heading">
            <h2 id="appearance-heading" class="section-heading">Appearance</h2>

            <div class="pref-group">
              <label class="pref-label" for="theme-select">Theme</label>
              <select id="theme-select" class="pref-select"
                      bind:value={theme.stored}>
                <option value="system">System</option>
                <option value="light">Light</option>
                <option value="dark">Dark</option>
                <option value="sepia">Sepia</option>
              </select>
            </div>

            <div class="pref-group">
              <label class="pref-label" for="font-select">Reading font</label>
              <select id="font-select" class="pref-select"
                      bind:value={font.value}>
                <option value="serif">Serif</option>
                <option value="sans">Sans-serif</option>
              </select>
            </div>

            <div class="pref-group">
              <label class="pref-label" for="density-select">Density</label>
              <select id="density-select" class="pref-select"
                      bind:value={density.value}>
                <option value="compact">Compact</option>
                <option value="default">Default</option>
                <option value="comfortable">Comfortable</option>
              </select>
            </div>
          </section>
        {:else}
          <section aria-labelledby="security-heading">
            <h2 id="security-heading" class="section-heading">Security</h2>
            <p class="placeholder-text">Security settings from M7 will appear here.</p>
          </section>
        {/if}
      </div>
    </div>
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .settings-main { flex: 1; display: flex; flex-direction: column; background: var(--bg); overflow-y: auto; }
  .settings-header {
    padding: 14px 24px;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
    position: sticky; top: 0;
  }
  .settings-title {
    font-family: var(--sans); font-size: 13px; font-weight: 600; color: var(--ink);
    margin: 0;
  }
  .settings-body { display: flex; flex: 1; }
  .settings-nav {
    width: 180px; flex-shrink: 0;
    padding: 16px 0;
    border-right: 1px solid var(--rule);
  }
  .settings-nav-item {
    display: block; width: 100%;
    text-align: left;
    padding: 8px 20px;
    font-family: var(--sans); font-size: 13px; color: var(--ink-2);
    border-left: 2px solid transparent;
  }
  .settings-nav-item:hover { color: var(--ink); }
  .settings-nav-item.active { color: var(--ink); border-left-color: var(--accent); font-weight: 500; }
  .settings-content { flex: 1; padding: 24px 32px; max-width: 560px; }
  .section-heading {
    font-family: var(--sans); font-size: 13px; font-weight: 600; color: var(--ink);
    margin: 0 0 20px; padding-bottom: 10px; border-bottom: 1px solid var(--rule);
  }
  .pref-group { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
  .pref-label { font-family: var(--sans); font-size: 13px; color: var(--ink-2); }
  .pref-select {
    font-family: var(--sans); font-size: 12px; color: var(--ink);
    background: var(--bg-soft); border: 1px solid var(--rule);
    border-radius: 4px; padding: 4px 8px; cursor: pointer;
  }
  .placeholder-text { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
</style>
```

- [ ] **Step 2: Run check**

```bash
cd web && pnpm check
```

Expected: no TypeScript errors.

- [ ] **Step 3: Manual smoke test**

Navigate to `/settings`. Appearance section should show theme, font, density dropdowns. Changing theme should update the page instantly. Changing font should switch reader content between serif and sans. Switching to Security tab shows the placeholder.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/Settings.svelte
git commit -m "feat(web): Settings view with Appearance section (theme/font/density)"
```

---

## Task 14: Keyboard context — wire next/prev/open to active view

**Skills:** invoke `svelte-runes` before implementing.

**Files:**
- Modify: `web/src/views/Unread.svelte`
- Modify: `web/src/views/Reader.svelte`
- Modify: `web/src/App.svelte`

The keyboard handler in `App.svelte` calls no-ops for list-navigation actions. We wire them up via a `$state`-backed dispatch pattern: `App.svelte` exposes a `keyCtx` object; `Unread.svelte` and `Reader.svelte` overwrite the action callbacks while they are mounted.

- [ ] **Step 1: Export a mutable context ref from `App.svelte` via Svelte context**

In `App.svelte`, replace the inline `keyCtx` object with a `$state` store passed via `setContext`:

```svelte
<script lang="ts">
  import { setContext } from 'svelte';
  // ... existing imports ...

  // Views override these callbacks while mounted.
  const dispatch = $state({
    onNext: () => {},
    onPrev: () => {},
    onOpen: () => {},
    onToggleRead: () => {},
    onToggleSaved: () => {},
    onViewOriginal: () => {},
  });

  setContext('keyDispatch', dispatch);

  const keyCtx = {
    ...dispatch,
    onEscape: () => {
      if (hotkeysOpen) { hotkeysOpen = false; return; }
      if ($route.name === 'reader') navigate('/');
    },
    setModalOpen: (open: boolean) => { hotkeysOpen = open; },
  };
  // Note: keyCtx properties must re-read dispatch properties each call
  // since dispatch is $state. Rebuild with getters:
  const keyHandler = buildHandler({
    get onNext() { return dispatch.onNext; },
    get onPrev() { return dispatch.onPrev; },
    get onOpen() { return dispatch.onOpen; },
    get onToggleRead() { return dispatch.onToggleRead; },
    get onToggleSaved() { return dispatch.onToggleSaved; },
    get onViewOriginal() { return dispatch.onViewOriginal; },
    onEscape: () => {
      if (hotkeysOpen) { hotkeysOpen = false; return; }
      if ($route.name === 'reader') navigate('/');
    },
    setModalOpen: (open: boolean) => { hotkeysOpen = open; },
  });
</script>
```

- [ ] **Step 2: Wire `Unread.svelte` to provide next/prev/open**

In `Unread.svelte`:

```svelte
<script lang="ts">
  import { getContext, onMount, onDestroy } from 'svelte';
  // ... existing imports ...

  let selectedId = $state<number | null>(null);
  const dispatch = getContext<{ onNext: () => void; onPrev: () => void; onOpen: () => void; onToggleRead: () => void; onToggleSaved: () => void; }>('keyDispatch');

  function selectNext() {
    const items = $entries.items;
    if (!items.length) return;
    const idx = selectedId == null ? -1 : items.findIndex(e => e.id === selectedId);
    selectedId = items[Math.min(idx + 1, items.length - 1)].id;
  }

  function selectPrev() {
    const items = $entries.items;
    if (!items.length) return;
    const idx = selectedId == null ? items.length : items.findIndex(e => e.id === selectedId);
    selectedId = items[Math.max(idx - 1, 0)].id;
  }

  function openSelected() {
    if (selectedId != null) navigate(`/entry/${selectedId}`);
  }

  function toggleReadSelected() {
    if (selectedId == null) return;
    const entry = $entries.items.find(e => e.id === selectedId);
    if (entry) entries.toggleRead(entry.id, !entry.read);
  }

  onMount(() => {
    dispatch.onNext = selectNext;
    dispatch.onPrev = selectPrev;
    dispatch.onOpen = openSelected;
    dispatch.onToggleRead = toggleReadSelected;
  });

  onDestroy(() => {
    dispatch.onNext = () => {};
    dispatch.onPrev = () => {};
    dispatch.onOpen = () => {};
    dispatch.onToggleRead = () => {};
  });
</script>
```

Pass `isSelected` prop to `EntryRow`:

```svelte
<EntryRow
  {entry}
  feed={feedFor(entry.subscription_id)}
  isSelected={selectedId === entry.id}
  onToggleRead={() => entries.toggleRead(entry.id, !entry.read)}
  onToggleSaved={() => entries.toggleRead(entry.id, entry.saved)} <!-- will fix with proper saved toggle in next task -->
/>
```

Update `EntryRow.svelte` to accept `isSelected` prop:

```svelte
type Props = {
  entry: EntryListItem;
  feed: Subscription | undefined;
  isSelected?: boolean;
  onToggleRead?: () => void;
  onToggleSaved?: () => void;
};
let { entry, feed, isSelected = false, onToggleRead, onToggleSaved }: Props = $props();
```

And add `class:is-selected={isSelected}` to the `<button class="entry">`.

- [ ] **Step 3: Run check + test**

```bash
cd web && pnpm check && pnpm test
```

Expected: no errors, all tests pass.

- [ ] **Step 4: Commit**

```bash
git add web/src/App.svelte web/src/views/Unread.svelte web/src/components/EntryRow.svelte web/src/views/Reader.svelte
git commit -m "feat(web): keyboard context — next/prev/open wired to active view"
```

---

## Task 15: Accessibility pass

**Skills:** invoke `svelte-runes`, `svelte-styling` before implementing.

**Files:**
- Modify: `web/src/views/Unread.svelte`
- Modify: `web/src/views/Reader.svelte`
- Modify: `web/src/components/Sidebar.svelte`
- Modify: `web/src/components/TopBar.svelte`
- Modify: `web/src/components/TabBar.svelte`

- [ ] **Step 1: Unread view — semantic list + ARIA**

In `Unread.svelte`, change the entry list container:

```svelte
<ul class="list" role="list" aria-label="Unread entries">
  {#each $entries.items as entry (entry.id)}
    <li role="listitem">
      <EntryRow {entry} feed={feedFor(entry.subscription_id)} isSelected={selectedId === entry.id} ... />
    </li>
  {/each}
</ul>
```

In `EntryRow.svelte`, the `<button>` already has `onclick`. Add `aria-label`:

```svelte
<button
  class="entry"
  aria-label="{entry.title}{entry.read ? ' (read)' : ''}"
  ...
>
```

Add `aria-hidden="true"` to the junction dot span (decorative):

```svelte
<span class="indicator" aria-hidden="true">
  <JunctionDot filled={!entry.read} />
</span>
```

- [ ] **Step 2: Reader view — `<main>`, ARIA landmarks**

Wrap the reader body in `<main>`:

```svelte
<main class="reader-pane" id="main-content">
  ...
</main>
```

Ensure the back button has a proper `aria-label="Back to unread entries"`.

- [ ] **Step 3: Sidebar — `<nav>` landmark**

In `Sidebar.svelte`, ensure the root element is `<nav aria-label="Main sidebar">`. Feed rows that are `<button>` elements should have `aria-label="{feed.title}"`.

- [ ] **Step 4: TopBar — icon button labels**

In `TopBar.svelte`, ensure each icon button has `aria-label`. For example:

```svelte
<button class="icon-btn" aria-label="Refresh feeds" onclick={onRefresh}>
  <!-- refresh icon -->
</button>
```

- [ ] **Step 5: TabBar — `aria-current`**

Already added in Task 10. Verify `aria-current="page"` is on the active tab.

- [ ] **Step 6: Run check**

```bash
cd web && pnpm check
```

Expected: no TypeScript or Svelte accessibility errors.

- [ ] **Step 7: Manual accessibility check**

Run `make dev`. Open browser DevTools → Accessibility tab. Tab through the Unread view with keyboard only — every interactive element should receive a visible focus ring. The entry list should read as a list by screen reader.

- [ ] **Step 8: Commit**

```bash
git add web/src/views/Unread.svelte web/src/views/Reader.svelte \
        web/src/components/Sidebar.svelte web/src/components/TopBar.svelte \
        web/src/components/TabBar.svelte web/src/components/EntryRow.svelte
git commit -m "feat(web): accessibility pass — semantic HTML, ARIA labels, focus rings"
```

---

## Task 16: M7 view styling pass

**Skills:** invoke `svelte-styling` before implementing.

**Files:**
- Modify: `web/src/views/Login.svelte`
- Modify: `web/src/views/Settings.svelte` (Security section)

M7 shipped functional Login (with TOTP step), Settings/Security, and Admin views. This task applies the design system to those views. The changes are purely CSS — no logic changes.

- [ ] **Step 1: Style the Login TOTP step**

In `Login.svelte`, find the TOTP code input (6-digit). Apply:
- `font-family: var(--mono)` for the input.
- `letter-spacing: 0.2em` for code spacing.
- The "Use a recovery code instead" toggle should use `color: var(--accent)` as a text link, not a button.
- Input container: `border: 1px solid var(--rule)`, `border-radius: 4px`, `padding: 8px 12px`.

- [ ] **Step 2: Style Settings / Security section**

In `Settings.svelte`, replace the Security section placeholder with the actual M7 Security component if it exists, or add styling rules for:
- Session table: `border-collapse: collapse`, rows with `border-bottom: 1px solid var(--rule)`, `font-family: var(--mono)` for meta columns.
- Revoke button: `color: var(--accent)`, small monospace style.
- "Log out everywhere" button: `color: #c97a1a` (warning colour).
- TOTP and passkey action buttons: monospace, same pattern as reader action buttons (`.reader-action`).

- [ ] **Step 3: Style Admin view**

In `Admin.svelte` (if it exists at `web/src/views/Admin.svelte`):
- User list table uses same table pattern as session table.
- Role badge: small monospace chip with `border: 1px solid var(--rule)`, `border-radius: 2px`, `padding: 1px 5px`.
- Disabled badge: `color: var(--ink-3)`, `opacity: 0.6`.
- Destructive action buttons (Delete, Disable): `color: #c97a1a`.

- [ ] **Step 4: Run check**

```bash
cd web && pnpm check
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Login.svelte web/src/views/Settings.svelte
git commit -m "feat(web): M7 view styling pass — Login TOTP, Settings/Security, Admin"
```

---

## Task 17: Full test run and build verification

- [ ] **Step 1: Run all tests**

```bash
cd web && pnpm test
```

Expected: all Vitest tests pass. Note any failures and fix before proceeding.

- [ ] **Step 2: Run TypeScript check**

```bash
cd web && pnpm check
```

Expected: zero errors.

- [ ] **Step 3: Full build**

```bash
make build
```

Expected: single static binary at `bin/tap` with no errors. `web/dist` populated.

- [ ] **Step 4: Verify no Google Fonts requests**

Start the server (`./bin/tap`) and open `http://localhost:8080` in a browser. Open Network tab, filter by "font". No requests to `fonts.googleapis.com` or `fonts.gstatic.com` should appear.

- [ ] **Step 5: Run Go tests**

```bash
make test
```

Expected: all Go tests pass (M8 has no Go changes, but verify nothing was broken).

- [ ] **Step 6: Commit**

If any fixes were needed in Steps 1–5, commit them:

```bash
git add -p
git commit -m "fix(web): M8 post-integration fixes"
```

---

## Definition-of-done checklist

Run through these before marking M8 complete. Each item maps to a spec requirement.

- [ ] No `fonts.googleapis.com` requests on cold load (Task 1).
- [ ] Theme switches without reload: light → dark → sepia → system (Task 4).
- [ ] Theme persists across page reload (Task 2, localStorage).
- [ ] Density and font toggles persist across reload (Task 2, localStorage).
- [ ] `prefers-reduced-motion` collapses all transitions (Task 3).
- [ ] All keyboard shortcuts work on desktop; suppressed inside form inputs (Tasks 5–6, 14).
- [ ] Hotkeys modal (`?`) opens, shows correct bindings, closes on `Esc` and outside click (Task 6).
- [ ] Sidebar visible in reader view on desktop (Task 9).
- [ ] On 390px viewport: sidebar hidden, tab bar visible (Task 10).
- [ ] Swipe right on list row → toggle read; swipe left → toggle saved (Task 11).
- [ ] Swipe in reader → navigate prev/next entry (Task 11).
- [ ] Pull-to-refresh fires on mobile at top of list (Task 12).
- [ ] Settings Appearance section changes theme/font/density live (Task 13).
- [ ] All interactive elements have visible focus rings in all three themes (Task 15).
- [ ] `pnpm test` green (Task 17).
- [ ] `pnpm check` clean (Task 17).
- [ ] `make build` produces valid binary (Task 17).
- [ ] `make test` green (Task 17).
