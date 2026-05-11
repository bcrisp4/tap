# Handoff: Tap — A Feed Reader

## Overview

**Tap** is a quiet, keyboard-first feed reader (RSS / Atom / JSON-Feed) for programmers, writers, and "old web" types who follow blogs. The product surface is a single-page web app: desktop split-pane / centred shell, plus a responsive mobile PWA. Three themes (light, sepia, dark). One accent colour: International Klein Blue.

This handoff bundles the full design system, an end-to-end set of HTML/React mockups for every main view, and a written spec deep enough to implement against.

## About the design files

The files in this bundle are **design references created in HTML/JSX**. They are prototypes showing the intended look, structure, copy, and behaviour. They are **not production code** — there is no router, no real auth, no real fetcher; data is mocked in `data.js` and rendered via plain React 18 + Babel-in-the-browser.

Your task is to **recreate these designs in the target codebase's existing environment** (React + a real router and state layer, Vue, SwiftUI, etc.) using its established patterns and libraries. If no codebase exists yet, choose the framework you'd reach for normally — React + Vite + TypeScript is a sensible default for this product — and implement the designs there. Lift `styles.css` more or less as-is (it's already token-driven CSS custom properties); lift the JSX as **structural and visual reference**, not source.

## Fidelity

**High-fidelity (hifi).** Every screen here has final colours, typography, spacing, hairlines, hit targets, motion timings, focus rings, and copy. The companion document `Tap Brand and UI Spec.md` defines every token, every component selector, and every state. The developer should recreate the UI **pixel-perfectly**, taking values from the spec or directly from `styles.css`.

## Files in this bundle

| File | Purpose |
|---|---|
| `Tap Brand and UI Spec.md` | **Single source of truth.** Brand, tokens, type scale, every component, every main view, every state. Read this first. |
| `styles.css` | All design tokens (`--bg`, `--ink`, `--accent`, …) and every component selector. Drop straight into the new app. |
| `data.js` | Mock data: feeds, categories, entries. Replace with the real adapter. |
| `Tap.html` | Landing chooser linking to the two design canvases. |
| `Tap Desktop.html` | Design canvas — every desktop view × every state × three themes. |
| `Tap Mobile.html` | Design canvas — every mobile (iOS PWA) view × states × themes. |
| `Tap Login.html` | Sign-in flows (password, passkey, magic link, OTP) — desktop + mobile. |
| `screenshot-harness.html` | One-view-at-a-time renderer keyed by URL hash; used to produce the slide PPTX. |
| `tap-components.jsx` | Primitives: `<Entry>`, `<TopTabs>`, `<KbdChip>`, `<Chip>`, `<Segmented>`, `<Dialog>`, `<Popover>`. |
| `tap-reader.jsx` | `<Reader>`, `<ReaderRail>`, `<ReaderHeader>`, `<ArticleBody>` (+ mobile variants). |
| `tap-simple.jsx` | The centred `.ts-shell` used by Unread, Saved, Settings, etc. — desktop + mobile. |
| `tap-saved.jsx` | Saved page — desktop and mobile (swipe-to-unsave / mark-read). |
| `tap-categories.jsx`, `tap-categories-page.jsx` | Categories management — cards, inline rename, reassign popover, mark-all-read. |
| `tap-feeds-page.jsx` | Feeds management — list, filters, sort, bulk actions, health panel, add/edit/import/export. |
| `tap-settings.jsx` | Settings — Appearance, Reading, Syncing, Account, Security (TOTP, passkeys, recovery codes), Sessions, Data. |
| `tap-admin.jsx` | Admin-only system status + user management. |
| `tap-mobile-nav.jsx` | Mobile bottom tab bar + More sheet. |
| `tap-login.jsx`, `tap-login.css` | Sign-in form shell, all four modes. |
| `design-canvas.jsx`, `ios-frame.jsx`, `browser-window.jsx`, `tweaks-panel.jsx` | Presentation chrome only — **do not port**. These are how the mockups are framed inside the design canvas. |

## How to run the mockups locally

The HTML files are self-contained: open `Tap.html` in a browser, or serve the folder with any static server (`python3 -m http.server`). Everything renders via React 18 + Babel from a CDN.

## Screens / views

Every view below has a full description in **`Tap Brand and UI Spec.md` § 6**. Quick index:

| # | View | Spec | Routes | Shell |
|---|---|---|---|---|
| 6.0 | Sign in (password / passkey / magic / OTP) | § 6.0 | `/sign-in` | `.tl-root` centred minimal |
| 6.1 | Unread (default landing) | § 6.1 | `/unread` | desktop split-pane **or** simple centred |
| 6.2 | Reader (article) | § 6.2, § 4.5 | `/read/:entryId` | split-pane right / `.ts-shell-reader` |
| 6.3 | Saved | § 6.3 | `/saved` | simple centred |
| 6.4 | Categories management | § 6.4 | `/categories` | simple centred |
| 6.5 | Feeds management | § 6.5 | `/feeds` | simple centred |
| 6.6 | Settings | § 6.6 | `/settings` | simple centred |
| 6.7 | Admin / system status | § 6.7 | `/admin` | simple centred — admin-only |
| 6.8 | Mobile PWA shell + reader + sheets | § 6.8 | all routes | `.tap.is-mobile` |

States covered (per view) include: default, loading, error, empty, dialogs (add / edit / delete / mark-all-read / import / export / regen codes), popovers (sort, reassign, account menu), inline editing, bulk selection, expanded health panels, OTP enrolment, recovery codes, passkey add / remove. See the design canvases in `Tap Desktop.html` and `Tap Mobile.html` for the exhaustive list — each artboard is labelled with its state.

## Interactions & behaviour

The interaction model is in **`Tap Brand and UI Spec.md` § 7 (shortcuts)** and **§ 8 (states)**. Highlights:

- **Keyboard-first.** `J/K` navigate, `Enter/O` open, `M` mark read, `S` save, `V` visit original, `R` refresh, `Shift R` refresh all, `?` shortcut help, `G U/S/F/C/,` go-to navigation, `1/2/3` measure width.
- **Three themes**, persisted in `localStorage.tap.theme`, respecting `prefers-color-scheme` only when unset.
- **Marks read on scroll** 1.5 s after the reader scrolls past the lede (configurable in Settings).
- **Polling** is live (status strip below top bar). Errors surface as inline chips on the offending feed row — never toast.
- **No toasts.** "The UI tells the truth on the next render" (§ 11). Destructive acks happen in the confirm dialog.
- **Motion is restrained.** 100–120 ms ease for everything; the only sanctioned keyframes are `tap-pulse` and `tf-spin`. Respect `prefers-reduced-motion: reduce`.

## State management (client)

```ts
selection: { entryId, source: 'list'|'reader' }
route:     { view: 'unread'|'saved'|'today'|'all'|'category'|'feed'|'reader'
                 |'settings'|'feeds'|'categories'|'admin', id?: string }
prefs:     { theme, density, measure, fontMode, sidebarCollapsed }
poll:      { lastSyncAt, inFlight, errors[] }
```

Data shapes (`Feed`, `Category`, `Entry`) are typed in **§ 5**.

## Design tokens

All tokens are CSS custom properties scoped to `.theme-light`, `.theme-dark`, `.theme-sepia` in `styles.css`. The condensed list:

- **Accent (the only colour):** `#002FA7` (light/sepia) · `#5a7fdc` (dark, desaturated for AA).
- **Light:** bg `#fafaf7`, paper `#ffffff`, ink `#1a1a1a` → `#4a4a48` → `#8a8a86` → `#c8c8c2`, rule `#e6e6df`.
- **Sepia:** bg `#f4ecd8`, paper `#faf3df`, ink `#3b2e1a` → `#6b5a3e` → `#9e8d6b` → `#c8b88f`, rule `#d9cca8`.
- **Dark:** bg `#0d0d0e`, paper `#1a1a1c`, ink `#ededea` → `#a8a8a4` → `#6e6e6a` → `#3a3a38`, rule `#232325`.
- **State-only:** error `#c43a3a` / `#ec7a7a`; warning `#c97a1a` / `#f0a655`. Never decorative.
- **Type:** Source Serif 4 (body, titles), Inter Tight (UI), JetBrains Mono (meta, kbd, URLs).
- **Spacing:** 4-pt grid → `4 6 8 10 12 14 16 18 20 22 24 28 32 36 40 44 48 56 64 80`.
- **Radii:** 2 chips · 3–4 inputs/buttons/cards · 6–8 dialogs · 100 pills · 50% dots. Default 4.
- **Hit targets:** ≥ 36 desktop · ≥ 44 mobile.

Full table: **`Tap Brand and UI Spec.md` § 2**.

## Assets

- **Web fonts:** Source Serif 4, Inter Tight, JetBrains Mono — all on Google Fonts.
- **Icon set:** hand-rolled, 16/20 viewBox, stroke-only, currentColor. List in **§ 3**.
- **Brand marks:** wordmark `tap·` (Inter Tight 600, Klein-Blue dot) and the T-junction mark for favicons. Both drawn in `Tap Desktop.html` § "Brand notes" and described in **§ 1**.

No raster images. No third-party logos. No emoji.

## Don'ts

See **`Tap Brand and UI Spec.md` § 11**. Most-violated items: don't add a second accent colour, don't use shadows for depth in the main UI (hairlines only — shadows are reserved for dialogs and popovers), don't ship toasts, don't ship a screen without an empty state.

---

*Questions about a component? Look it up by its selector in `styles.css` — every visible element has one.*
