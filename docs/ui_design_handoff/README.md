# Handoff: Tap — Feed Reader UI

## Overview

Tap is a self-hosted RSS feed reader (single Go binary + SQLite + Svelte SPA).
This bundle contains the **visual and interaction design** for the two
foundational views of the SPA:

1. **Unread** — chronological list of unread entries (the home screen)
2. **Article reader** — full-text reader for a single entry

Both views are designed for **desktop (browser)** and **mobile (iOS PWA on
home screen)**. Three themes are specified: light, sepia, dark.

## About the Design Files

The HTML/JSX/CSS files in this bundle are **design references created in
HTML** — prototypes showing intended look, layout, and behavior. They are
**not production code to copy directly**.

The implementation task is to recreate these designs in the Tap codebase
(SvelteKit SPA, per the design spec) using its conventions, component
patterns, and CSS architecture. The HTML/React prototypes here are for
reference: pixel measurements, color values, font choices, interaction
semantics. Translate, don't transliterate.

The accompanying `Tap - Design Spec.md` is the source-of-truth product
spec — schema, API, scheduler, brand. Read it first.

## Fidelity

**High-fidelity.** Final colors, typography, spacing, and interaction
semantics are intended to be implemented as shown. Any deviation should be
intentional and justified, not incidental.

---

## Brand & Visual Identity

### Name
**Tap.** Lowercase. The wordmark renders the period after the word as a
filled blue dot, slightly larger than typographic norm — it doubles as a
schematic junction dot.

### The motif
A schematic **junction dot** (the small filled circle at a wire
intersection in an electrical schematic). It appears as:
- The dot in the wordmark
- The unread indicator beside each list entry
- The active-state mark in nav and tab bars
- The accent in section dividers (rule, dot, rule)

> **Visual rule:** Klein Blue is precious. If it appears more than three
> times on a screen, something is wrong.

### Colors

| Token | Light | Sepia | Dark |
|---|---|---|---|
| `--bg` (page) | `#fafaf7` | `#f4ecd8` | `#0d0d0e` |
| `--bg-soft` (panels, hover) | `#f3f3ee` | `#ece3c9` | `#161617` |
| `--surface` (raised) | `#ffffff` | `#faf3df` | `#1a1a1c` |
| `--ink` (primary text) | `#1a1a1a` | `#3b2e1a` | `#ededea` |
| `--ink-2` (secondary text) | `#4a4a48` | `#6b5a3e` | `#a8a8a4` |
| `--ink-3` (tertiary / meta) | `#8a8a86` | `#9e8d6b` | `#6e6e6a` |
| `--ink-4` (faint, dots) | `#c8c8c2` | `#c8b88f` | `#3a3a38` |
| `--rule` (borders) | `#e6e6df` | `#d9cca8` | `#232325` |
| `--accent` (Klein Blue) | `#002FA7` | `#002FA7` | `#5a7fdc` |
| `--accent-soft` | `rgba(0,47,167,0.08)` | `rgba(0,47,167,0.10)` | `rgba(90,127,220,0.14)` |

The dark theme uses a **desaturated Klein** (`#5a7fdc`) for AA contrast
against `#0d0d0e`. Light and sepia use the canonical IKB hex.

### Typography

| Role | Family | Notes |
|---|---|---|
| Body / article content | **Source Serif 4** (with Iowan Old Style, Charter, Georgia fallbacks) | Optical-size aware; weights 400/500/600; italic 400 |
| UI chrome (buttons, sidebar, tabs) | **Inter Tight** | Weights 400/500/600 |
| Metadata / badges / kbd hints / "MARK UNREAD" buttons in reader header | **JetBrains Mono** | Weights 400/500; tabular-nums + zero |

Load via Google Fonts — see `styles.css` `@import` line.

Sentence case throughout. Never all caps **except** in monospace
metadata captions (`SAVED`, `UNREAD · 11`, group titles like `READING`,
reader header buttons), where small uppercase letterspacing is part of
the schematic-meta aesthetic.

Reading measure: **65–72 ch** (≈680px max-width container at 17px body).
Line-height **1.7** for body, **1.55** for lede.

### Iconography
Line-based, monochrome, **1.4–1.5 stroke weight on a 16-unit grid**,
rounded caps and joins. Klein Blue reserved for active state only. All
icons inlined as SVG in `tap-components.jsx` (search the `Icon` object)
and `tap-reader.jsx`.

---

## Screens

### 1. Unread (Desktop)

**Layout:** three columns, full viewport.

```
┌──────────────┬───────────────────────────────────────────┐
│ Sidebar      │ Top bar                                   │
│ 240 px       │ ─ Poller status strip ──────────────────  │
│              │                                           │
│ tap·         │ ● Title of entry                          │
│              │   feed · 32m ago · 12 min read            │
│ READING      │   summary text two lines max …            │
│   Unread  11 │ ─────────────────────────────────────     │
│   All        │ ○ Read entry (lower contrast)             │
│   Saved    3 │   …                                       │
│              │                                           │
│ FEEDS        │ (more rows)                               │
│   ▮ Julia E. │                                           │
│   ▮ Dan Luu  │                                           │
│   …          │                                           │
│              │ ─ Keyboard hints footer ─                 │
└──────────────┴───────────────────────────────────────────┘
```

**Sidebar (`240px` wide, `border-right: 1px solid var(--rule)`):**
- Brand wordmark `tap` + Klein dot, 17px Inter Tight 600.
- Group title `READING` — JetBrains Mono 10px, letter-spacing `0.12em`,
  uppercase, color `--ink-3`, padding `16px 20px 6px`.
- Nav items — Inter Tight 13px, padding `6px 20px`, **2px left border**
  in Klein when active. Badge count right-aligned in JetBrains Mono 10.5px.
- Group title `FEEDS`.
- Feed rows: 12px square color swatch + name (truncated) + unread count
  in mono.
- Group title `SYSTEM` — Add feed, Settings.

**Top bar (sticky, `padding: 14px 24px`, `border-bottom: 1px solid var(--rule)`):**
- Crumb `Unread` (bold, Inter Tight 13px) + count `11 of 15` in mono 11px.
- Right-aligned icon buttons (28×28, color `--ink-2`, hover bg `--bg-soft`):
  search, refresh, mark-all-read (check), filter.

**Poll strip (immediately below top bar):**
- Mono 10px, `--bg-soft` background, `padding: 6px 24px`.
- Pulsing 6px Klein dot (animation: opacity 1→0.35, scale 1→0.8, 2.4s
  ease-in-out infinite).
- Text: `POLLER · 4 workers · 12 feeds tracked` … `NEXT TICK · 47s`.

**Entry row (`.entry`):**
- `padding: 14px 24px 14px 40px;` `border-bottom: 1px solid var(--rule);`
- **Junction indicator** at `left: 22px; top: 22px`: 6px Klein circle if
  unread; 6px transparent circle with 1px `--ink-4` border if read.
- Saved badge: `SAVED` in JetBrains Mono 10px, Klein, top-right.
- Title: Source Serif 4 17px / 1.3 / weight 500. Read = 400 + `--ink-3`.
- Meta: 9px feed swatch + source name (medium) + 3px dot separator + ago
  + dot + reading time (mono).
- Summary: Source Serif 4 14px / 1.5 / `--ink-2`, 2-line clamp.
- Hover: `background: var(--bg-soft)`. Selected: `var(--accent-soft)`.

**Densities:**
- `compact`: hide summary, padding 10/10.
- `default`: as above.
- `comfortable`: padding 16/16.

**Keyboard hints footer:** Inter Tight 11px, kbd chips with
`border-bottom-width: 2px`, version string `tap v0.1 · self-hosted` mono.

### 2. Unread (Mobile)

**Layout:** column. Status bar inherits theme. iOS PWA, `display: standalone`.

- **Top bar:** wordmark + `· unread` + count + search icon button.
- **List:** same entry row component, denser padding (`padding-left:
  36px; padding-right: 18px;` title 16px).
- **Tab bar (bottom, fixed):** 4 columns, **48px min height**, padding
  `10px 0 calc(env(safe-area-inset-bottom) + 14px)`. Each tab is a
  24×24 line icon + 11px Inter Tight label, 5px gap. Active tab has a
  4px Klein junction dot at the top edge and `--ink` color.
  Tabs: **Unread, Saved, Search, Settings**.

### 3. Article reader (Desktop)

**Layout:** four columns.

```
┌──────────┬────────────┬──────────────────────────────┐
│ Sidebar  │ List rail  │ Reader pane                  │
│ 240 px   │ 280 px     │ flex                         │
│          │ bg-soft    │                              │
│          │ UNREAD·11  │ ┌─ Header (sticky) ────────┐ │
│          │            │ │ ‹ UNREAD     MARK · SAVED │ │
│          │ ● Title    │ │              · ORIGINAL  │ │
│          │   feed·32m │ └──────────────────────────┘ │
│          │ ┌────────┐ │                              │
│          │ │● Title │ │      Source · domain         │
│          │ │ feed   │ │      Title (38px serif 600)  │
│          │ └────────┘ │      BY · DATE · MIN READ    │
│          │   (more)   │      ─── ● ───               │
│          │            │      Italic lede paragraph.  │
│          │            │      Body paragraphs at      │
│          │            │      17/1.7, max 680px.      │
│          │            │      ## Section heads in     │
│          │            │      serif 22/1.25.          │
│          │            │      Code blocks: mono 13,   │
│          │            │      bg-soft, 2px Klein left │
│          │            │      border.                 │
│          │            │      ─── ○ ───               │
│          │            │      cached locally · 32m    │
└──────────┴────────────┴──────────────────────────────┘
```

**List rail (`280px`, `bg: var(--bg-soft)`, `overflow-y: auto`):**
- Head: `UNREAD · 11` mono 10px, padding `18px 20px 12px`.
- Rows: padding `12px 20px 12px 32px`, border-bottom `--rule`.
- 5px junction dot at `left: 18px, top: 18px`, Klein for unread,
  transparent w/ `--ink-4` border for read.
- Selected row: `bg: var(--bg)` + 2px Klein left border (`::before`).
- Title: Source Serif 4 14/1.35 weight 500, 2-line clamp.
- Meta: Inter Tight 11px / `--ink-3`.

**Header (sticky, `padding: 14px 28px`, `border-bottom: 1px solid var(--rule)`):**
- Left: back button — chevron-left + `UNREAD` in **JetBrains Mono 11px,
  letter-spacing 0.04em, uppercase**, color `--ink-2`, padding `6px 10px
  6px 4px`, hover bg `--bg-soft`. (See user feedback iteration: these
  buttons MUST be monospace, not sans.)
- Right cluster: `MARK UNREAD`, `SAVED`, `VIEW ORIGINAL` — same monospace
  treatment. The active "Saved" button is colored Klein.

**Reader pane (`max-width: 680px`, `margin: 0 auto`, `padding: 56px 56px 80px`):**

| Element | Spec |
|---|---|
| Source line | Inter Tight 12, color `--ink-2`. 10×10 swatch + name (medium) + 3px sep dot + mono URL 11px `--ink-3`. Margin-bottom 18px. |
| Title (`h1`) | Source Serif 4 38/1.15/600, letter-spacing -0.015em, `text-wrap: balance`, margin 0 0 16px |
| Byline | JetBrains Mono 11px, letter-spacing 0.04em, uppercase, `--ink-3`. Items separated by `·` in `--ink-4`. Margin-bottom 28px. |
| Top divider | flex row: 1px line `--rule` / 6px Klein junction dot / 1px line `--rule`. Margin-bottom 32px. |
| Lede | Source Serif 4 19/1.55, italic, color `--ink`. Margin 0 0 28px. |
| Body `p` | Source Serif 4 17/1.7, color `--ink`, margin 0 0 22px, `text-wrap: pretty` |
| `h2` | Source Serif 4 22/1.25/600, letter-spacing -0.01em, margin `40px 0 14px` |
| `pre / code` | JetBrains Mono 13/1.55, color `--ink-2`, bg `--bg-soft`, **2px Klein left border**, padding `14px 16px`, overflow-x auto |
| Bottom divider | Same as top but dot is `--ink-4` instead of Klein. |
| Foot | mono 10px uppercase letter-spacing 0.06em `--ink-3` centered: `Cached locally · last refreshed 32m ago` |

### 4. Article reader (Mobile)

**Top bar (`padding: 10px 14px`, `border-bottom: 1px solid var(--rule)`):**
- 40×40 back button (chevron-left, 18px stroke 1.5).
- Centered **reading-progress bar**: 2px tall, full-width flex, `--rule`
  track, `--accent` fill at the user's scroll position.
- 40×40 save action.

**Body:** `padding: 22px 22px 28px`, sizes step down (`title: 28px`,
`p: 16/1.65`, `h2: 20px`).

**Bottom action bar:** 3-column grid, padding `8px 8px (env-safe + 12px)`,
`border-top: 1px solid var(--rule)`. Each button is 52px min-height
column-flex layout: 20×20 icon + 11px Inter Tight 500 label, 4px gap.
Buttons: **Mark unread / Saved / Original**. Active "Saved" button: color
Klein and the bookmark glyph is filled (`fill="currentColor"`).

---

## Interactions

### Desktop keyboard
- `j` / `↓` — next entry in list
- `k` / `↑` — previous entry
- `m` — toggle read on focused entry
- `s` — toggle saved on focused entry
- `o` — open article (route → `/entry/:id`)
- `v` — view original (open `entry.url` in a new tab)
- `Esc` — back to list from reader
- `/` — focus search

### Mobile
- Swipe right on a list row → toggle read
- Swipe left on a list row → toggle saved
- Pull-to-refresh on Unread list → trigger feed refresh
- Tap row → enter reader

### Animation rules
- Hover/active state changes: 120ms ease.
- Junction dot pulse (poll strip): 2.4s ease-in-out infinite, opacity
  1→0.35, scale 1→0.8.
- Theme switch: instant (CSS custom-property swap, no transition).
- No decorative motion.

### Focus
- All focusable elements: `outline: 2px solid var(--accent); outline-offset: 2px;`
  on `:focus-visible`.

---

## State Management (per view)

### Unread
- `entries: Entry[]` — fetched from `GET /api/v1/entries?status=unread`.
- `selectedId: number | null` — keyboard cursor position.
- `density: 'compact' | 'default' | 'comfortable'` — user preference (column on `users` table or local).
- `showSummary: boolean` — user preference.

### Reader
- `entry: Entry` — fetched from `GET /api/v1/entries/:id` (with full content).
- Optimistically toggle `read` on mount (per spec §8: "explicit button/tap
  only — no auto-mark on scroll" — so this is **wrong**; do NOT auto-mark.
  Mark only when the user taps the button or hits `m`).
- Scroll progress: derive from `scrollTop / (scrollHeight - clientHeight)`,
  drive the 2px progress bar on mobile.

---

## Design Tokens

```css
--serif: "Source Serif 4", "Iowan Old Style", Charter, Georgia, serif;
--sans:  "Inter Tight", -apple-system, BlinkMacSystemFont, system-ui, sans-serif;
--mono:  "JetBrains Mono", ui-monospace, "SF Mono", Menlo, monospace;
--klein: #002FA7;
```

Spacing: multiples of 2px. Common values: 4, 6, 8, 10, 12, 14, 18, 22, 28, 32, 40, 56.
Border radius: 2 (chips), 4 (buttons / nav-active), 8 (mobile touch targets).
Shadows: none in chrome. The browser/iOS frames in the prototype use shadows
only for the device mockup itself.

Full token list lives in `styles.css` under the `.theme-light`, `.theme-dark`,
`.theme-sepia` blocks.

---

## Assets

- **Fonts:** Google Fonts — Source Serif 4, Inter Tight, JetBrains Mono.
  Production should self-host (the spec is privacy-by-default).
- **Icons:** all SVG, inlined. No icon-font dependency. Reference set is
  in `tap-components.jsx` (`Icon` object) and inline within `tap-reader.jsx`.
- **Logo / favicon:** primary mark is a T-junction (drawn inline in the
  `brand` section of `Tap River.html`). Vector should be regenerated as
  a clean SVG asset for the build.
- **Sample feed data:** `data.js` contains 12 sample feeds and 15 sample
  entries based on lobste.rs / Dan Luu / Julia Evans-style tech blog
  content. Useful for screenshot tests; do not ship.

---

## Files in this bundle

| File | Purpose |
|---|---|
| `Tap River.html` | Master prototype. Loads everything; design canvas with all six artboards (desktop + mobile × 3 themes for both Unread and Reader). |
| `styles.css` | All design tokens, theme blocks, component styles. The single source of truth for colors and type. |
| `data.js` | Sample feeds + entries. |
| `tap-components.jsx` | Unread view (desktop + mobile), sidebar, top bars, entry row, tab bar. |
| `tap-reader.jsx` | Article reader (desktop + mobile), reader header, body, mobile chrome. |
| `design-canvas.jsx` | Pan/zoom canvas the prototype is laid out in (presentation only — not part of the product). |
| `browser-window.jsx` | Mock Chrome window chrome (presentation only). |
| `ios-frame.jsx` | Mock iOS device frame (presentation only). |
| `tweaks-panel.jsx` | The Tweaks panel for live density/font/summary toggles in the prototype (presentation only). |
| `Tap - Design Spec.md` | The product spec — schema, API, scheduler, brand guidelines. **Read this first.** |

To run the prototype locally, open `Tap River.html` directly in a browser
(it pulls React, ReactDOM, and Babel from unpkg). It uses the Tweaks panel
top-right to toggle density / serif-vs-sans / summaries on the fly.

---

## Out of scope for this handoff

The design spec lists many additional views (Saved, Category, Feed,
Search, Settings, Add feed, Empty/first-run, Offline). Only **Unread**
and **Article reader** are designed in this bundle. The visual
vocabulary (sidebar, mono captions, junction-dot motif, three themes,
type ramp) should generalize directly to those screens.
