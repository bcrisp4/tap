# Handoff: Tap — Feed Reader UI

## Overview

Tap is a feed reader. This bundle contains the **visual and interaction
design** for the two foundational views:

1. **Unread** — chronological list of unread entries (the home screen)
2. **Article reader** — full-text reader for a single entry

Both are designed for **desktop (browser)** and **mobile (PWA, iOS-shaped
mockup)**. Three themes: light, sepia, dark.

## About the Design Files

The HTML/JSX/CSS files in this bundle are **design references** —
prototypes showing intended look, layout, and behavior. They are not
production code to copy directly.

The implementation task is to recreate these designs in the target
codebase using its existing component patterns and CSS architecture.
Translate, don't transliterate. The accompanying `Tap - Design Spec.md`
covers the visual identity, brand rules, theming, and interaction model
in more detail.

## Fidelity

**High-fidelity.** Final colors, typography, spacing, and interaction
semantics are intended to be implemented as shown. Any deviation should
be intentional and justified, not incidental.

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
| Metadata / badges / kbd hints / "MARK UNREAD"-style buttons in reader header | **JetBrains Mono** | Weights 400/500; tabular-nums + zero |

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

### Feed avatars (icon-or-color)

Every feed renders an **avatar** in the sidebar feed list and in entry
row metadata. Resolution rule:

1. **If the feed provides an icon** (e.g. supplied by the source as a
   small image): render that icon, cropped to a rounded square.
2. **If no icon is available**, fall back to a flat color square using a
   stable per-feed color.

This is implemented as the `FeedAvatar` component in
`tap-components.jsx`. Sizes used in the design:

| Location | Size | Radius |
|---|---|---|
| Sidebar feed row | 14×14 | 3 |
| Entry-row meta line | 9×9 | 2 |
| Reader source line | 10×10 | 2 |
| Brand notes / docs | 18×18 | 3 |

Never invent or generate an icon client-side from the feed name —
letter-mark fallbacks are not part of this design. The color square is
the only fallback. The `data.js` sample assigns icons to about half the
feeds so the prototype shows both branches of the rule.

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
- Feed rows: 14px feed avatar (see **Feed avatars** above) + name
  (truncated) + unread count in mono. A 12px warning glyph appears in
  place of the count on rows where the feed is in an error state.
- Group title `SYSTEM` — Add feed, Settings.

**Top bar (sticky, `padding: 14px 24px`, `border-bottom: 1px solid var(--rule)`):**

- Crumb `Unread` (bold, Inter Tight 13px) + count `11 of 15` in mono 11px.
- Right-aligned icon buttons (28×28, color `--ink-2`, hover bg `--bg-soft`):
  search, refresh, mark-all-read (check), filter.

**Poll strip (immediately below top bar):**

- Mono 10px, `--bg-soft` background, `padding: 6px 24px`.
- Pulsing 6px Klein dot (animation: opacity 1→0.35, scale 1→0.8, 2.4s
  ease-in-out infinite).
- Text style: `POLLER · 4 workers · 12 feeds tracked` … `NEXT TICK · 47s`.
  (Treat as ambient status copy — exact strings can come from runtime.)

**Entry row (`.entry`):**

- `padding: 14px 24px 14px 40px;` `border-bottom: 1px solid var(--rule);`
- **Junction indicator** at `left: 22px; top: 22px`: 6px Klein circle if
  unread; 6px transparent circle with 1px `--ink-4` border if read.
- Saved badge: `SAVED` in JetBrains Mono 10px, Klein, top-right.
- Title: Source Serif 4 17px / 1.3 / weight 500. Read = 400 + `--ink-3`.
- Meta: 9×9 feed avatar + source name (medium) + 3px dot separator + ago
  + dot + reading time (mono).
- Summary: Source Serif 4 14px / 1.5 / `--ink-2`, 2-line clamp.
- Hover: `background: var(--bg-soft)`. Selected: `var(--accent-soft)`.

**Densities:**

- `compact`: hide summary, padding 10/10.
- `default`: as above.
- `comfortable`: padding 16/16.

**Keyboard hints footer:** Inter Tight 11px, kbd chips with
`border-bottom-width: 2px`.

### 2. Unread (Mobile)

**Layout:** column. Status bar inherits theme. PWA, `display: standalone`.

- **Top bar:** wordmark + `· unread` + count + search icon button.
  Padding-top accounts for the device status bar (`60px`).
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
| Source line | Inter Tight 12, color `--ink-2`. 10×10 feed avatar + name (medium) + 3px sep dot + mono URL 11px `--ink-3`. Margin-bottom 18px. |
| Title (`h1`) | Source Serif 4 38/1.15/600, letter-spacing -0.015em, `text-wrap: balance`, margin 0 0 16px |
| Byline | JetBrains Mono 11px, letter-spacing 0.04em, uppercase, `--ink-3`. Items separated by `·` in `--ink-4`. Margin-bottom 28px. |
| Top divider | flex row: 1px line `--rule` / 6px Klein junction dot / 1px line `--rule`. Margin-bottom 32px. |
| Lede | Source Serif 4 19/1.55, italic, color `--ink`. Margin 0 0 28px. |
| Body `p` | Source Serif 4 17/1.7, color `--ink`, margin 0 0 22px, `text-wrap: pretty` |
| `h2` | Source Serif 4 22/1.25/600, letter-spacing -0.01em, margin `40px 0 14px` |
| `pre / code` | JetBrains Mono 13/1.55, color `--ink-2`, bg `--bg-soft`, **2px Klein left border**, padding `14px 16px`, overflow-x auto |
| Bottom divider | Same as top but dot is `--ink-4` instead of Klein. |
| Foot | mono 10px uppercase letter-spacing 0.06em `--ink-3` centered, e.g. `Cached locally · last refreshed 32m ago` |

### 4. Article reader (Mobile)

**Top bar (`padding: 60px 14px 10px`, `border-bottom: 1px solid var(--rule)`):**

- Top padding clears the device status bar (matches the unread top bar).
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
- `v` — view original (open the entry's source URL in a new tab)
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

## UI State (per view)

> **Implementation-agnostic.** These are the pieces of state the UI reads
> and writes. Wire them up to whatever data layer the codebase uses.

### Unread

- `entries` — the list to render.
- `selectedId` — keyboard cursor position.
- `density` — `compact` | `default` | `comfortable`. User preference.
- `showSummary` — boolean. User preference.

### Reader

- `entry` — the article being read (full content).
- Read state: toggled **only** by explicit user action — the button or
  the `m` shortcut. Never auto-mark on scroll.
- Scroll progress drives the 2px progress bar on mobile.

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

- **Fonts:** Source Serif 4, Inter Tight, JetBrains Mono. The prototype
  pulls from Google Fonts for convenience; the production app can
  self-host them.
- **Icons:** all SVG, inlined. No icon-font dependency. Reference set is
  in `tap-components.jsx` (`Icon` object) and inline within `tap-reader.jsx`.
- **Logo / favicon:** primary mark is a T-junction (drawn inline in the
  `brand` section of `Tap.html`). Vector should be regenerated as
  a clean SVG asset.
- **Sample feed data:** `data.js` contains 12 sample feeds and 15 sample
  entries with realistic tech-blog content, used to drive the prototype.
  Five feeds ship with inline-SVG icons to exercise the icon-or-color
  rule.

---

## Files in this bundle

| File | Purpose |
|---|---|
| `Tap.html` | Master prototype. Loads everything; design canvas with all six artboards (desktop + mobile × 3 themes for both Unread and Reader) plus a Brand notes section. |
| `styles.css` | All design tokens, theme blocks, component styles. The single source of truth for colors and type. |
| `data.js` | Sample feeds + entries. |
| `tap-components.jsx` | Unread view (desktop + mobile), sidebar, top bars, entry row, tab bar, `FeedAvatar`. |
| `tap-reader.jsx` | Article reader (desktop + mobile), reader header, body, mobile chrome. |
| `design-canvas.jsx` | Pan/zoom canvas the prototype is laid out in (presentation only — not part of the product). |
| `browser-window.jsx` | Mock Chrome window chrome (presentation only). |
| `ios-frame.jsx` | Mock iOS device frame (presentation only). |
| `tweaks-panel.jsx` | The Tweaks panel for live density/font/summary toggles in the prototype (presentation only). |
| `Tap - Design Spec.md` | Visual identity, brand rules, theming, and interaction model. |

To run the prototype locally, open `Tap.html` directly in a browser.
The Tweaks panel in the top-right toggles density, serif-vs-sans, and
summaries on the fly.

---

## Recent design adjustments

- **Feed avatars** are now icon-or-color (not color-only). Five sample
  feeds in `data.js` ship with inline-SVG icons; the rest exercise the
  color fallback. A documenting artboard lives in the **Brand** section
  of the prototype (`brand-feed-avatars`).
- **Mobile reader top bar** (`.m-reader-topbar`) now has
  `padding-top: 60px` to clear the device status bar, matching the Unread
  view's `.m-topbar`. Apply the same offset to any other mobile chrome that sits
  flush against the top of the viewport in a PWA shell.

---

## Out of scope for this handoff

The product has many additional views (Saved, Category, Feed, Search,
Settings, Add feed, Empty/first-run, Offline). Only **Unread** and
**Article reader** are designed in this bundle. The visual vocabulary
(sidebar, mono captions, junction-dot motif, three themes, type ramp)
should generalize directly to those screens.
