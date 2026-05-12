# Tap — Brand, UI, and Implementation Spec

> A quiet, keyboard-first feed reader. This document is the source of truth for the Tap brand, design system, and the main product views. It is written so a web developer can build the entire UI without further design input.

---

## 0. Quick facts

| | |
|---|---|
| **Product** | Tap — a personal feed (RSS / Atom / JSON-Feed) reader |
| **Audience** | Programmers, writers, hobbyists, and "old web" types who follow blogs |
| **Tone** | Serious but not severe. Editorial, not corporate. Quiet by default. |
| **Surfaces** | Web app (desktop), responsive web app (mobile / PWA) |
| **Tech notes** | Single-page app; keyboard-shortcut driven; works offline for cached items |
| **License of mark** | Original work. Not affiliated with any existing reader product. |

---

## 1. Brand

### 1.1 Name & wordmark

The product is **Tap** — lowercased everywhere it appears as a wordmark (`tap`). The capital "T" is only used when "Tap" begins a sentence in body copy. Never write it `TAP`, `tAp`, or `tap.io`.

The wordmark is set in **Inter Tight 600**, letter-spacing `-0.02em`, sized to match the surrounding UI. A small accent dot — the Klein-Blue period — sits immediately after the `p`, vertically aligned to the x-height baseline (it is not a full stop; it is a registration mark).

```
tap·       ← 17px wordmark in sidebar / nav
tap·       ← 56px hero wordmark on the landing canvas
```

Implementation:

```html
<span class="wordmark">tap<span class="dot" aria-hidden="true"></span></span>
```

```css
.wordmark {
  font-family: "Inter Tight", system-ui, sans-serif;
  font-weight: 600;
  letter-spacing: -0.02em;
  display: inline-flex;
  align-items: baseline;
  gap: 1px;
}
.wordmark .dot {
  width: 0.31em; height: 0.31em;          /* scales with font-size */
  border-radius: 50%;
  background: var(--accent);              /* Klein blue */
  transform: translateY(-0.05em);
  margin-left: 2px;
}
```

### 1.2 The T-junction mark

When the wordmark would be too long (favicon, app icon, loading splash) Tap uses the **T-junction mark**: a horizontal rule meeting a vertical descender, with the Klein-Blue dot at the junction.

```
 ─●─
   │
   │
```

Implementation lives in `.tap-mark`:

- Horizontal bar: 1.5px stroke, current text colour
- Vertical descender: 1.5px stroke, current text colour, drops from the junction
- Dot: Klein-Blue circle centred on the junction

The mark is monochrome except for the dot; it inherits `color` from its parent so it works on any theme.

### 1.3 Voice

- **Editorial, not promotional.** Headings use a serif. Calls to action are quiet and verb-led ("Add feed", "Mark all read", "Continue").
- **Plain numbers.** Counts are bare digits (`12`, `1,402`), no badges, no parentheses.
- **Monospace for meta.** Timestamps, URLs, keyboard shortcuts, status, and any "machine-spoken" UI sit in JetBrains Mono.
- **No exclamation marks.** Never "Saved!". Just "Saved" or, better, no toast at all.
- **One accent colour, ever.** Klein Blue. Don't reach for green for success or red for danger except where strictly required (destructive confirmation copy).

---

## 2. Design tokens

All tokens are CSS custom properties scoped to a theme class on the app root.

### 2.1 Themes

Three themes ship: **light** (default), **dark**, **sepia** (reading mode). All three use the same Klein-Blue accent except dark, which uses a desaturated variant for AA contrast.

```css
.theme-light {
  --bg:        #fafaf7;   /* paper-warm off-white */
  --bg-soft:   #f3f3ee;   /* hover, strips, secondary backgrounds */
  --surface:   #ffffff;   /* cards, popovers, keyboard chips */
  --ink:       #1a1a1a;   /* primary text */
  --ink-2:     #4a4a48;   /* secondary text */
  --ink-3:     #8a8a86;   /* tertiary text, meta */
  --ink-4:     #c8c8c2;   /* disabled, faint */
  --rule:      #e6e6df;   /* hairline borders */
  --accent:    #002FA7;   /* International Klein Blue */
  --accent-soft: rgba(0, 47, 167, 0.08);
}

.theme-dark {
  --bg:        #0d0d0e;
  --bg-soft:   #161617;
  --surface:   #1a1a1c;
  --ink:       #ededea;
  --ink-2:     #a8a8a4;
  --ink-3:     #6e6e6a;
  --ink-4:     #3a3a38;
  --rule:      #232325;
  --accent:    #5a7fdc;   /* desaturated Klein for AA */
  --accent-soft: rgba(90, 127, 220, 0.14);
}

.theme-sepia {
  --bg:        #f4ecd8;
  --bg-soft:   #ece3c9;
  --surface:   #faf3df;
  --ink:       #3b2e1a;
  --ink-2:     #6b5a3e;
  --ink-3:     #9e8d6b;
  --ink-4:     #c8b88f;
  --rule:      #d9cca8;
  --accent:    #002FA7;
  --accent-soft: rgba(0, 47, 167, 0.10);
}
```

Theme is set by adding `.theme-{name}` to the app shell. Persist the choice in `localStorage` under `tap.theme`. Honour `prefers-color-scheme` only when the user has not chosen.

Two semi-semantic colours are allowed for state and **only** for state:

| Use | Light | Dark |
|---|---|---|
| Error text (feed health, destructive confirm) | `#c43a3a` | `#ec7a7a` |
| Warning chip (stale feed, backoff) | `#c97a1a` | `#f0a655` |

Never use these as decorative colour.

### 2.2 Type

Tap pairs three families. Each has a clear job; do not cross them.

| Token | Family | Used for |
|---|---|---|
| `--serif` | **Source Serif 4** (8..60 optical sizes; 400/500/600 + italic 400) | Article body, entry titles, settings labels, page titles, dialog titles |
| `--sans`  | **Inter Tight** (400/500/600) | UI chrome, buttons, navigation, byline meta, form fields |
| `--mono`  | **JetBrains Mono** (400/500) | Eyebrows, counts, timestamps, URLs, keyboard chips, status lines, system data |

Fallbacks (used until the web fonts load and as a permanent fallback for `theme-print`):

```
"Source Serif 4", "Iowan Old Style", Charter, Georgia, serif
"Inter Tight", -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif
"JetBrains Mono", ui-monospace, "SF Mono", Menlo, monospace
```

Apply `font-feature-settings: "kern", "liga", "onum"` to body text and `"tnum", "zero"` to anything monospaced that contains numbers.

#### Type scale (px)

| Role | Family | Size | Line | Weight | Tracking |
|---|---|---|---|---|---|
| Article title (desktop) | serif | 38 | 1.12 | 600 | -0.02em |
| Article title (mobile) | serif | 28 | 1.15 | 600 | -0.02em |
| Page title | serif | 38 | 1.05 | 600 | -0.02em |
| Section title (category card) | serif | 28 | 1.10 | 600 | -0.02em |
| H2 inside article | serif | 22 | 1.25 | 600 | -0.01em |
| Lede / pull-quote | serif italic | 19 | 1.55 | 400 | normal |
| Entry title | serif | 17–19 | 1.3 | 500 | -0.005em |
| Entry summary | serif | 14–14.5 | 1.55 | 400 | normal |
| Article body | serif | 17 | 1.7 | 400 | normal |
| Settings label | serif | 17 | 1.3 | 500 | -0.005em |
| UI body | sans | 13 | 1.4 | 400/500 | normal |
| Button | sans | 12.5 | 1 | 500 | normal |
| Tab / nav | mono | 11 | 1 | 400/500 | 0.10em UPPER |
| Eyebrow | mono | 10 | 1 | 400 | 0.14em UPPER |
| Section eyebrow w/ numeral | mono | 10 | 1 | 400 | 0.16em UPPER |
| Meta / timestamp | mono | 10.5–11 | 1.2 | 400 | 0.02em |
| Tiny tag | mono | 9.5 | 1 | 400/500 | 0.08em UPPER |
| Keyboard chip | mono | 10–11 | 1.2 | 400 | 0 |

Read entry titles tone down by switching weight 500 → 400 and colour `--ink` → `--ink-3`. They never strike-through.

### 2.3 Spacing

Spacing follows a 4-pt grid with 2-pt half-steps allowed for hairline-aligned chrome.

`4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 28, 32, 36, 40, 44, 48, 56, 64, 80`

Article body measure: `680px` comfortable, `580px` narrow, `760px` wide (set via `.measure-{comfortable|narrow|wide}` on the article wrapper).

Side gutters:
- Desktop content `24–28px`
- Desktop article body `56px` top/bottom and side
- Mobile content `18–22px`
- Mobile article body `22px` side, `22px` top, `28px` bottom

### 2.4 Borders & radii

- **Hairline rule** `1px solid var(--rule)` for every divider, card edge, segmented track, popover, and form input.
- Never use `box-shadow` for elevation in the main UI. Shadows appear in two places only: dialogs (`0 24px 60px rgba(0,0,0,0.32)`) and popovers (`0 10px 30px rgba(0,0,0,0.18)`).
- Radii: `2px` chips/squares, `3–4px` inputs/buttons/cards, `6–8px` dialogs, `100px` pills, `50%` dots. The default radius for a new component is **4px**.

### 2.5 Motion

- All hover/state transitions use `100–120ms ease`.
- No bounces, no springs, no scale-ups beyond `1`.
- Only two named keyframes are sanctioned globally:

```css
@keyframes tap-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%      { opacity: 0.35; transform: scale(0.8); }
}
/* used by .ts-status-dot, .poll-strip .pulse */

@keyframes tf-spin { to { transform: rotate(360deg); } }
/* used by refresh icon when fetching */
```

Reduce motion: respect `prefers-reduced-motion: reduce` — disable both keyframes.

### 2.6 Focus

Always `outline: 2px solid var(--accent); outline-offset: 2px;`. Never remove the focus ring; restyle if you must.

### 2.7 Scrollbar

Within `.tap` only: thumb `var(--ink-4)`, track transparent, 8px. The page-level scrollbar uses OS default.

---

## 3. Iconography

Tap uses a single icon set. All icons:

- Are drawn at **16×16** viewBox (sidebar, list rows) or **20×20** (toolbars, settings).
- Use **stroke**, never fill, except for the accent dot.
- Stroke width: `1.5px` at 16, `1.75px` at 20.
- `stroke-linecap: round; stroke-linejoin: round`.
- Inherit `currentColor`. The accent dot is the only coloured element.

Required icons (semantics — names are for the developer, not necessarily file names):

`refresh`, `search`, `settings`, `keyboard`, `more` (vertical ellipsis), `back` (chevron-left), `forward` (chevron-right), `caret-down`, `caret-right`, `check`, `x`, `plus`, `trash`, `external-link`, `bookmark` (saved), `bookmark-filled`, `eye` (mark read), `eye-off` (mark unread), `sun` (light), `moon` (dark), `book` (sepia), `serif-A`, `sans-A`, `density-comfortable`, `density-compact`, `qr`, `key`, `phone`, `laptop`, `wifi`, `alert-triangle`, `info`, `drag-handle` (6-dot grip).

Feed avatars are 14×14 squares (`border-radius: 3px`). If the feed publishes an inline SVG icon in its feed metadata, render it; otherwise fall back to a flat coloured square with the feed's assigned colour.

---

## 4. Component library

Every selector below assumes the app root has class `.tap` (sets the type, colour, font features, scrollbar) plus a theme class.

### 4.1 Layout shells

Tap ships **three** top-level shells. Pick one per route — never mix.

#### A. Desktop split-pane (`.tap`)

```
┌─────────┬──────────────┬────────────────────────┐
│ sidebar │  list rail   │  detail (reader/empty) │
│ 240px   │  flexible    │  flexible              │
└─────────┴──────────────┴────────────────────────┘
```

- Sidebar: `240px`, `border-right: 1px solid var(--rule)`, scrolls independently.
- List rail (`.tap-unread`, `.reader-rail`): scrolls independently; rail is `280px` when reader is open, otherwise consumes the remaining width.
- Detail pane: scrolls independently.

#### B. Simple centred shell (`.ts-root` / `.ts-shell`)

A single 720px column, top tabs, no sidebar. Used for routes where the sidebar would feel heavy (Saved, Settings, Categories management, Feeds management, Reader-as-page).

This shell has no sidebar, so the account chip's job is taken by a small **account avatar** (`.ts-account`) positioned absolute against `.ts-root` at `top: 30px; right: 22px` — i.e. floating in the page's top-right corner, **outside** the centered 720px column so it never competes with the tab row for horizontal space. The avatar is a 20×20 neutral-gray circle (`background: var(--ink-4)`, initial in mono 9.5/600 `var(--ink)`) — deliberately quieter than the sidebar's accent-blue chip so it doesn't pull the eye while reading. Clicking opens the same `AccountMenu` popover described in §4.2, repositioned to `top: 56px; right: 18px`.

```
┌────────────────────────────────────────┐
│                                        │
│   ┌── 720px ──────────────────┐        │
│   │  tap·  Unread / Saved /…  │        │
│   │ ─────────────────────────  │        │
│   │  (content)                │        │
│   └───────────────────────────┘        │
│                                        │
└────────────────────────────────────────┘
```

`.ts-shell { max-width: 720px; margin: 0 auto; padding: 36px 24px 80px; }`

#### C. Mobile shell (`.tap.is-mobile`)

Full-bleed column, sticky top bar (`60px` top padding for status bar), bottom tab bar. Add `.is-mobile` to scale entry, reader, and chrome down. iOS-style: target heights ≥ 44px.

### 4.2 Sidebar (`.tap-sidebar`)

Order, top to bottom:

1. **Brand row** — `tap·` wordmark, 8px gap to a small refresh icon, `20px` side padding, `18px` bottom padding.
2. **Group title row** — eyebrow ("Library" / "Categories" / "Feeds") with an optional `+` action button on the right.
3. **Nav items** (`Unread`, `Saved`, `Today`, `All`) — 13px sans, `var(--ink-2)` resting, `var(--ink)` hover, `var(--ink)` + `2px solid var(--accent)` left border when active. Trailing count in mono `10.5px var(--ink-3)`, accent when active.
4. **Categories block** (collapsible) — mono uppercase header with caret, then indented feed rows.
5. **Feed rows** — 14×14 avatar + name (truncates) + mono unread count. `has-error` rows show an inline warn icon.
6. **Account chip** (`.account-chip`) — sticky row above the icon footer. 26×26 accent-blue avatar tile (initial in mono 11.5/600 white), display name (sans 12.5/500), email (mono 10 `var(--ink-3)`) stacked, and a `▾` caret on the right. Hover/open states tint the row with `rgba(0,0,0,0.04–0.05)`. Clicking opens the **Account menu** (`.account-menu`, a `.tap-popover` anchored to the chip): a header echoing name + email, then `Account settings`, `Switch theme`, a 1px rule, and `Log out` (tinted `#b3402c` / `#e9846f` in dark, with the same tint applied on hover). The popover scrim is `position: fixed` so click-outside dismisses from anywhere.
7. **Footer** — sticky bottom strip with 30×30 icon buttons for `Keyboard`, `Settings`, `More`. `border-top: 1px solid var(--rule)`. Logout is **not** in this row — it lives in the Account menu above.

Selectors and full styling: see `.tap-sidebar *` in the styles file.

### 4.3 Top bar (`.tap-topbar`)

`sticky; top: 0; border-bottom: 1px solid var(--rule); padding: 14px 24px; height: ~52px`. Slots:

- **Crumb** — `sans 13px var(--ink-2)`, the active scope name in `var(--ink)` weight 600.
- **Count** — `mono 11px var(--ink-3)`, e.g. `12 unread`.
- **Spacer**
- **Icon buttons** — 28×28, no chrome, hover `var(--bg-soft)`. Always include `search`, `refresh`, `more` (overflow menu).

A **poll strip** (`.poll-strip`) can sit immediately below the top bar to show live polling status: pulsing accent dot + mono "polling 14/24 · 3 with errors".

### 4.4 Entry row (`.entry`)

The atomic unit of every list view.

```
●  Reasons why bugs might feel "impossible"
   Julia Evans · jvns.ca · 12 min · 32m
   A bug feels impossible when one of your assumptions
   is wrong. Here are some categories of wrong assump…
```

Structure:

```html
<article class="entry is-unread">
  <span class="junction" aria-hidden="true"></span>
  <h3 class="title">Reasons why bugs might feel "impossible"</h3>
  <div class="meta">
    <span class="source">Julia Evans</span>
    <span class="sep"></span>
    <span class="rt">12 min</span>
    <span class="sep"></span>
    <time class="rt">32m</time>
  </div>
  <p class="summary">A bug feels impossible when…</p>
  <span class="saved-mark" aria-label="Saved">SAVED</span>
</article>
```

- **Junction dot** at `left: 22px; top: 22px`, 6×6, accent. Read state: hollow ring `1px solid var(--ink-4)`.
- **Title** serif 17px / 500. Read: weight 400, colour `var(--ink-3)`.
- **Meta** sans 12px, source in `var(--ink)` weight 500, separators are 3×3 dots in `var(--ink-4)`, timestamps in mono.
- **Summary** serif 14px, clamped to 2 lines via `-webkit-line-clamp`.
- **Saved mark** mono 10px, accent colour, right gutter.
- `is-selected` background `var(--accent-soft)`.

#### Density modes

Set `.density-compact` or `.density-comfortable` on the list container:

- **Compact** — hides `.summary`, pads `10px` top/bottom.
- **Comfortable** — shows summary, pads `16px` top/bottom (default).
- **Cosy** — same as compact but keeps the summary clamped to 1 line. Optional third density.

#### Mobile variant

Add `.is-mobile` to the root: padding shifts to `36px` left / `18px` right, title to 16px, junction to `left: 18px`.

### 4.5 Reader (`.reader-pane` / `.ts-article`)

Layout: centred article body, max-width controlled by the user's measure setting (`580 / 680 / 760`).

```
┌──────────────────────────────────────────┐
│ ← Back                  Save  Open  More │ ← reader-header
├──────────────────────────────────────────┤
│                                          │
│   [feed-avatar]  Julia Evans  jvns.ca    │ ← reader-source
│                                          │
│   Reasons why bugs might feel             │ ← reader-title (serif 38)
│   "impossible"                            │
│                                          │
│   JULIA EVANS · 32 MIN AGO · 12 MIN READ │ ← reader-byline (mono caps)
│                                          │
│   ─────────●─────────                    │ ← reader-rule
│                                          │
│   It often feels impossible to find a     │ ← reader-lede (italic 19)
│   bug, until suddenly it doesn't…         │
│                                          │
│   Body paragraphs. 17px / 1.7. Serif.    │ ← reader-p
│                                          │
│   ## Subhead 22px                         │ ← reader-h2
│                                          │
│   <pre> with 2px accent left border      │ ← reader-pre
│                                          │
│   ────────────── ●                        │ ← reader-end
│   END OF ARTICLE                          │ ← reader-foot (mono caps)
└──────────────────────────────────────────┘
```

Header (`.reader-header`):

- Sticky, hairline bottom rule, `14px × 28px` padding.
- Left: `← Back` (mono 11px caps).
- Right: `Save`, `Open ↗`, optional `Mark unread`, `More`. All mono 11px caps with 6px gap, hover `var(--bg-soft)`. Saved state colours the button `var(--accent)`.

Body rules:

- Use `text-wrap: pretty` on paragraphs, `text-wrap: balance` on the title.
- A reader **rule** appears between byline and lede: `[1px line] [6px accent dot] [1px line]`.
- A reader **end** marker mirrors it after the last paragraph and is followed by a mono caps "END OF ARTICLE".

#### Reader font mode

Toggle the article between serif (default) and sans by switching `--serif` to `--sans` on the article wrapper. Two presets only; do not expose arbitrary font choice.

#### Mobile reader (`.m-reader-topbar`, `.m-reader-footbar`)

- Top bar: `[back] [progress strip] [more]`, 40×40 hit targets.
- Bottom action bar: 3-up grid — `Save`, `Open`, `Mark unread`. 52px tall buttons.
- Body padding `22px / 22px / 28px`.

### 4.6 Buttons (`.ts-btn`)

```
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  Add feed       │  │  Save changes   │  │  Open in tab    │
└─────────────────┘  └─────────────────┘  └─────────────────┘
  default               primary               accent
```

| Variant | Background | Border | Color |
|---|---|---|---|
| default | `var(--bg)` | `var(--rule)` | `var(--ink)` |
| primary `.is-primary` | `var(--ink)` | `var(--ink)` | `var(--bg)` |
| accent `.is-accent` | transparent | `var(--accent)` | `var(--accent)` |
| danger `.is-danger` | transparent | `rgba(196,58,58,.4)` | `#c43a3a` |
| quiet `.is-quiet` | transparent | transparent | `var(--ink-2)` |

Sizing: `8px × 14px` padding, sans 12.5/500, radius 4. Quiet variant is `6px × 8px`.

Icon-only buttons are 28×28 with the icon centred, no border, hover background `var(--bg-soft)`.

### 4.7 Segmented control (`.ts-segmented`)

Pill with inner 3px padding. Inactive buttons are quiet text on `--bg-soft`; the active button gains `--bg`, a 1px ring `--rule`, and a subtle 1px-2px shadow.

```html
<div class="ts-segmented">
  <button class="ts-segmented-btn is-active"><span class="sw sw-light"></span>Light</button>
  <button class="ts-segmented-btn"><span class="sw sw-dark"></span>Dark</button>
  <button class="ts-segmented-btn"><span class="sw sw-sepia"></span>Sepia</button>
</div>
```

Preview chips inside the buttons:

- **Theme**: 12×12 rounded square fill (`sw-light`, `sw-dark`, `sw-sepia`, `sw-system` is split-diagonal).
- **Font**: small letter `A` set in the corresponding family (`.font-prev.serif`, `.font-prev.sans`).
- **Density**: 3 stacked 1.5px bars (`.ts-density-prev`).

### 4.8 Chips

| | |
|---|---|
| **Pill chip** (`.cat-chip`, `.ts-feeds-chip`) | 100px radius, `5px × 12px`, hairline border, sans 12/500. Active: `--ink` fill, `--bg` text. |
| **Tag chip** (`.m-cat-feed`, etc) | soft chip on `--bg-soft`, no border, smaller padding. |
| **Status chip** (`.ts-feed-err-chip`, `.ts-feed-backoff-chip`) | mono 9.5 UPPER, 2×8 padding, 100px radius. Error variant uses warning red; backoff variant uses dashed `--ink-4` border. |

### 4.9 Popover (`.tap-popover`, `.ts-cat-pop`)

Background `--bg`, border `1px solid var(--rule)`, radius 5–6, shadow `0 10px 30px rgba(0,0,0,.18)`, inner padding 4px. Each item is `7px × 10px`, hover `--bg-soft`, sans 13/500.

For reassignment popovers (move feed to category), each row carries a leading 10px check column reserved for the current value.

### 4.10 Dialog (`.ts-dialog`, `.tap-modal`)

Centred over a `rgba(0,0,0,.42)` scrim (`.6` in dark). Width `min(520px, 100%)`; wide variant `min(640px, 92vw)`. Anatomy:

```
┌────────────────────────────────────────┐
│ Dialog title                       ×  │ head: 16/14/20, serif 18/600
├────────────────────────────────────────┤
│ Body. Serif 15/1.55 ink-2 prose.       │ body: 18/20
│ Form fields, lists, etc.               │
├────────────────────────────────────────┤
│ tiny hint            Cancel  Continue │ foot: bg-soft, mono left, btns right
└────────────────────────────────────────┘
```

A `.ts-dialog-warn` block is a left-accent-bordered callout used for irreversible-action copy.

### 4.11 Forms

- Input field `.ts-field`: sans 14, 9×12 padding, `--rule` border, radius 4. Focus: 2px accent outline + accent border.
- Mono input `.ts-feeds-form-input`: same shape but mono 12. Used for URLs, secrets, anything machine-spoken.
- Field label `.ts-field-label` / `.ts-feeds-form-l`: mono 10 UPPER `0.12em`, `--ink-3`. Optional description in sans 11.5 `--ink-3` below.
- Textareas: same chrome, `min-height: 64px`, `resize: vertical`.

### 4.12 Keyboard chip (`.kbd` / `.ts-kbd`)

`mono 10–11px, 1px ×5px padding, 1px border, bottom-border 2px, radius 3`. Render full key names: `J`, `K`, `Esc`, `⌘ K`, `Shift`, `?`.

### 4.13 OTP input (`.ts-otp`)

Six 36×44 cells, each mono 17px, accent ring on the focused cell, accent-soft outer halo `0 0 0 2px`.

### 4.14 QR placeholder (`.ts-qr`)

140×140, hairline border, three 28×28 finder squares (TL/TR/BL) with 4px outer ring and a 5px inset core. Real QR replaces it via `<canvas>` or `<img>` at the same dimensions.

### 4.15 Recovery codes grid (`.ts-codes`)

2-column grid inside a `--bg-soft` panel. Each code: leading 10px numeral in `--ink-4`, mono 13 in `--ink`, letter-spacing `0.06em`. Used codes get `--ink-4` + `text-decoration: line-through`.

### 4.16 Empty states

Always three lines:

```
   ●                          ← 8px dot, accent (default) or ink-4 (inactive)
 Title (serif 17–22)
 Subtitle (sans 13, ink-3, max 360px)
 [optional CTA button]
```

For "empty Saved" use the accent dot; for "no results" use `--ink-4`.

---

## 5. Logical model

The minimum data shapes a developer needs.

```ts
type Feed = {
  id: number;
  name: string;
  url: string;            // host or full URL
  color: string;          // hex — used for fallback avatar
  icon?: string;          // inline 16×16 SVG markup, optional
  category?: string;      // category id; null/undefined = "Uncategorised"
  error?: string;         // human-readable last-fetch error
  backoffUntil?: string;  // ISO timestamp
};

type Category = {
  id: string;
  name: string;
  slug: string;
};

type Entry = {
  id: number;
  feed: number;           // Feed.id
  title: string;
  summary: string;        // 1–3 sentences, pre-rendered
  body?: string;          // full HTML article body, lazy-loaded
  rt: number;             // estimated read time, minutes
  ago: string;            // pre-formatted relative time, e.g. "32m", "1h", "Yesterday"
  read: boolean;
  saved: boolean;
};
```

State the client owns:

- `selection: { entryId, source: 'list'|'reader' }`
- `route: { view: 'unread'|'saved'|'today'|'all'|'category'|'feed'|'reader'|'settings'|'feeds'|'categories'|'admin', id?: string }`
- `prefs: { theme, density, measure, fontMode, sidebarCollapsed }`
- `poll: { lastSyncAt, inFlight, errors[] }`

---

## 6. Main views

### 6.1 Unread (default landing)

**Route** `/unread` · **Shell** Desktop split-pane (or Simple centred for the `tap-simple` layout)

```
┌─────────┬──────────────────────────────────────────┐
│ sidebar │ ← polling 14/24            12 unread  ⟳ │ ← topbar / poll-strip
│         ├──────────────────────────────────────────┤
│         │ ●  Reasons why bugs might feel "impo…"   │
│ Unread12│    Julia Evans · jvns.ca · 12m · 32m    │
│ Saved   │    A bug feels impossible when one of…   │
│ Today   │                                          │
│ All     │ ●  I wrote a tiny SQLite-backed task…   │
│         │    lobste.rs · 6m · 1h · SAVED          │
│ ────    │                                          │
│ People  │ ○  Files are fraught with peril          │
│ Systems │    Dan Luu · 21m · Yesterday             │
│ Letters │                                          │
└─────────┴──────────────────────────────────────────┘
```

- The list is grouped by date band: **TODAY**, **YESTERDAY**, **THIS WEEK**, **EARLIER**. Each band gets a `.ts-group-heading` with a hairline rule and right-aligned mono count.
- Pressing `J`/`K` moves the selection; `O` or `Enter` opens the reader; `M` toggles read; `S` toggles saved; `R` refreshes; `/` focuses search; `?` opens the shortcut modal.
- Selecting an entry sets `.entry.is-selected` and (in split-pane mode) loads the reader in the right pane.

### 6.2 Reader

**Route** `/read/:entryId` · **Shell** split-pane right or Simple centred (`.ts-shell-reader`)

See §4.5 for full anatomy. Behaviour:

- Marks the entry read 1.5 s after the reader scrolls past the lede (or immediately on `M`).
- Saves position in `localStorage` keyed by entry id so a refresh restores scroll.
- The reader-rail (`.reader-rail`) on the left lists the surrounding entries from the same scope (Unread, Category, Feed) with the current one highlighted by a 2px accent strip.
- `Open ↗` opens the original URL in a new tab; `Save` toggles saved; `Mark unread` flips state and pops back to the list.

### 6.3 Saved

**Route** `/saved` · **Shell** Simple centred (`.ts-root`, `.ts-shell`)

A flat chronological list of every entry where `saved === true`. No grouping by feed. Pinned banner at top showing the count.

Empty state: accent dot, "Nothing saved yet", "Press <kbd>S</kbd> on any entry to keep it here."

### 6.4 Categories — management page

**Route** `/categories` · **Shell** Simple centred (`.ts-cats`)

```
   CATEGORIES
   ────────────────────────────
   Categories
   Group feeds into named clusters. Ordering controls
   sidebar order. Type to filter; ⌘ N to add.
   ───────────────────────────────────────  + New
   ─────────────────────────────────────────────
   People                              4 feeds · 12
   ─────────────────────────────────────────────
   ●  jvns                Julia Evans · jvns.ca
   ●  danluu              Dan Luu · danluu.com
   …
   Rename · Reorder up · Reorder down · Delete

   ─────────────────────────────────────────────
   Systems & PL                        2 feeds · 4
   …

   ─────────────────────────────────────────────
   Uncategorised                       3 feeds · 7
   ─────────────────────────────────────────────
```

- The page uses one `.ts-cat` card per category. Card head: serif 28 title + right-aligned `.ts-cat-stats` (`b` count + accent `ll` unread).
- Inline rename: click the title or press `R` while a card is focused — it swaps to a `.ts-cat-title-input` with an accent bottom border. Save on `Enter`, cancel on `Esc`.
- Reassign feed: click the trailing `.ts-cat-feed-pick` (mono caps "Move ▾") to open a popover listing every category. Current category is bold accent with a leading check.
- `Uncategorised` is a pseudo-category, always last, italic `--ink-3` title. It cannot be renamed, reordered, or deleted; its action row only offers "Mark all read".
- Empty list: T-junction mark, "No categories yet", an inline row of example chips (`People`, `Systems`, `Letters`), and a dark CTA button.

### 6.5 Feeds — management page

**Route** `/feeds` · **Shell** Simple centred (`.ts-feeds`)

Toolbar (top to bottom):

1. **Search + Add** — search input fills the row, `Add feed` primary button on the right.
2. **Filter chips** — `All`, `Errors (n)`, `Unread (n)`, `Stale (n)`. Active chip gets `--bg-soft` background and a hairline.
3. **Sort** — mono caps select: `Recently active`, `Name`, `Added`, `Most unread`. Plus a right-side `Refresh all` and `Import OPML` quiet button.
4. **Bulk bar** (appears when ≥ 1 row is checked) — solid `--ink` strip with action buttons (`Refresh`, `Set category…`, `Delete`).

Each feed row (`.ts-feed-row`):

```
☐  [▢]  Julia Evans               PEOPLE              12  ⟳ … 
        jvns.ca · 92 entries · last 32m
```

Columns:
- Checkbox (16×16, accent fill when checked)
- Avatar (18×18)
- Body — line 1: serif 19/600 name, then category chip, then optional error or backoff chip. Line 2: mono URL with leading `→` icon, dot separators, mono unread count.
- Actions — unread count chip, refresh icon, more menu.

Expanded health panel (when a row has `.is-expanded` and an error): a `--bg-soft` panel with a 2px danger left-border, a mono error string, and a 3-cell grid of `Last OK`, `Last try`, `Status`.

Add-feed dialog: a wide `.ts-dialog.is-wide`. URL field is mono. On submit the dialog shows a `.ts-feeds-disc` discovery list — radio rows for each feed Tap autodiscovered at that URL, with name, mono URL, and an optional `JSON` / `ATOM` / `RSS` format tag.

### 6.6 Settings

**Route** `/settings` · **Shell** Simple centred (`.ts-set`)

Sections, each preceded by a numbered eyebrow rule (`01 · APPEARANCE`, `02 · READING`, …):

| # | Section | Rows |
|---|---|---|
| 01 | Appearance | Theme (segmented light/dark/sepia/system), Font (serif/sans segmented), Density (comfortable/compact segmented), Measure (narrow/comfortable/wide segmented) |
| 02 | Reading | Mark read on scroll (toggle), Auto-open next (toggle), Show summaries in list (toggle), Open links in new tab (toggle) |
| 03 | Syncing | Poll interval (segmented 5m/15m/1h/manual), Refresh all now (button), Last sync (mono timestamp + status row) |
| 04 | Account | Email (mono), Display name, Change password (button), Sign out everywhere (danger button) — single-device logout lives in the sidebar Account chip / mobile More sheet, not here |
| 05 | Security | Passkeys (`.ts-pk-list`), TOTP (status row + add/remove buttons), Recovery codes (generate/view, opens dialog) |
| 06 | Sessions | `.ts-sess-list` of devices — icon + label + meta + revoke ✗ |
| 07 | Data | Export OPML, Export saved as JSON, Import OPML (dialog), Delete account (danger, opens confirm dialog) |

Row pattern (`.ts-set-row`): label (serif 17/500) left, optional description (sans 12.5 `--ink-2`) underneath, control right-aligned. Stack on narrow viewports via `.is-stacked`.

The page-id strip under the page title shows `account · ada@example.com · plan: free` in mono with small dot separators.

### 6.7 Admin / System status

**Route** `/admin` · only visible to operator accounts.

Four-cell metric grid (`.ts-sys-grid`):

```
┌──────────┬──────────┬──────────┬──────────┐
│ FEEDS    │ ENTRIES  │ ERRORS   │ POLL     │
│ 24       │ 14,820   │ 2 warn   │ 14m ago  │
│ 22 ok    │ 1,402 24h│ phoronix │ next 1m  │
└──────────┴──────────┴──────────┴──────────┘
```

Below: errors table (`.ts-sys-errors`) — timestamp (mono ink-3) · message (mono ink ellipsis) · code chip.

### 6.0 Sign in

**Route** `/sign-in` · **Shell** Centered minimal (`.tl-root` / `.tl-col`)

The signed-out entry point. Deliberately quiet — no tabs, no footer, no marketing. The wordmark sits alone in the top-left; the form is a single ~380px column centered vertically in the page. No card chrome, no shadow, no second accent — just the same hairlines and Klein-Blue dot that the rest of the app uses.

```
┌────────────────────────────────────────────┐
│  tap·                                      │ ← .tl-header  (wordmark only)
│                                            │
│                                            │
│            Sign in                         │ ← .tl-title    (serif 34/600)
│                                            │
│            USERNAME                        │ ← .ts-field-label (mono caps)
│            ┌────────────────────────────┐  │
│            │                            │  │ ← .ts-field   (mono input)
│            └────────────────────────────┘  │
│            PASSWORD                FORGOT? │
│            ┌────────────────────────────┐  │
│            │ ••••••••                   │  │
│            └────────────────────────────┘  │
│            ┌────────────────────────────┐  │
│            │  Continue  →        ⏎      │  │ ← .ts-btn.is-primary.tl-primary
│            └────────────────────────────┘  │
│            ┌────────────────────────────┐  │
│            │   ⚿  Use a passkey         │  │ ← .ts-btn.is-quiet.tl-alt
│            └────────────────────────────┘  │
│                                            │
└────────────────────────────────────────────┘
```

#### Anatomy

- **`.tl-root`** — full-height column. `display: flex; flex-direction: column;` against `var(--bg)`. No header rule, no footer.
- **`.tl-header`** — `padding: 22px 32px`. Wordmark only. No version tag, no nav, no help link.
- **`.tl-main`** — flex-grows; centers `.tl-col` both axes (`padding: 24px`).
- **`.tl-col`** — `max-width: 380px`. All form content lives here.
- **`.tl-title`** — serif 34 / 1.05 / 600, letter-spacing `-0.02em`, `text-wrap: balance`, `margin-bottom: 10px`. Stands alone — no eyebrow, no subtitle. The wordmark in the corner is the page's only label.
- **`.tl-sub`** — only renders when the mode actually needs context (passkey device callout, 2-step destination). For the default password sign-in there is no sub line; the form speaks for itself.
- **`.tl-field`** — wraps `.ts-field-label` (mono 10 UPPER `0.14em`, `--ink-3`) and the `.ts-field` input. The label row is `display: flex; justify-content: space-between` so a trailing link like `FORGOT?` aligns right in the same mono.
- **Username input** (`type="text"`) is `font-family: var(--mono); font-size: 13px;` — URLs and identities sit in mono throughout Tap, and the sign-in screen is no exception. The backend authenticates by username, not email address. Password input uses the default sans `.ts-field`.
- **`.tl-actions`** — vertical button stack, `gap: 8px`, `margin-top: 14px`.
  - **Primary** `.ts-btn.is-primary.tl-primary` — 42px tall, full-width, content centered, gap 8. Label + `→` icon, plus a tinted `<span class="ts-kbd">Enter</span>` at the right (light-on-dark variant). The keyboard chip is hidden in the mobile variant.
  - **Quiet** `.ts-btn.is-quiet.tl-alt` — 38px tall, full-width, centered, used for the alternate auth path (passkey ↔ password ↔ magic link). One alternate only; never two.
- **`.tl-switch`** — only shown in the 2-step state, never on the first screen. `display: flex; justify-content: space-between;` with mono caps "NOT YOU?" / "Use a different account".

#### Modes

The same form shell covers four entry points. The mode is taken from the URL (`?step=`) or chosen by the auth backend.

| Mode | Title | Fields | Primary | Alt |
|---|---|---|---|---|
| `password` (default) | "Sign in" | username + password | "Continue →" | "⚿ Use a passkey" |
| `passkey` | "Welcome back" | none — `.tl-passkey-card` shows the device | "⚿ Continue with passkey" | "Use a password instead" |
| `magic` | "Sign in by email" | email only | "Send sign-in link" | "Sign in with a password" |
| `otp` | "Verification code" | `.ts-otp` six-cell input | "Verify and continue →" | "Use a recovery code" |

The passkey card (`.tl-passkey-card`) is a `--bg-soft` strip with a 30×30 keyed tile on the left and two stacked lines — sans 13/500 device name (`MacBook · Touch ID`) and a mono 10 UPPER `last used 2 days ago` meta. Drop in when the browser advertises a resident credential.

For the 2-step screen, `.tl-otp-row` lays the six `.ts-otp-cell`s on the left and a small mono "PASTE WITH ⌘V" hint on the right (desktop only). The destination email appears in the subtitle as `var(--mono)` text so it can't be missed.

#### Mobile (`.tl-root.is-mobile`)

- `padding-top: 50px` for the iOS status-bar inset.
- `.tl-m-header` keeps the wordmark only (18px). No version, no footer.
- `.tl-m-main` content padding `30px 22px 16px`.
- `.tl-title` drops to **28px**, `.tl-sub` to 14.5px.
- Buttons grow to 48px (primary) / 46px (alt) for fat-thumb hit targets.
- Inputs grow to 46px high, 15px text.
- OTP cells grow to **44×54** with 19px digits so the keypad doesn't dominate the value.
- No keyboard chips (`.ts-kbd` is hidden inside `.ts-root.is-mobile`).

#### States

- **Empty / first paint** — exactly as drawn. Email field autofocuses.
- **Loading** — primary button shows its label with the icon swapped for a 12×12 spinner using `tf-spin`. Inputs go `pointer-events: none`. No skeletons; the form is short enough that flicker isn't a concern.
- **Error** — `.tl-error` chip slots above the first field. Anatomy: 10px left border in `#c43a3a` (`#ec7a7a` in dark), 1px hairline of the same hue at `.28` alpha, `rgba(196,58,58,0.06)` fill, sans 13/1.45. Leading `IconWarn` (1.4px stroke, current colour), then the body — full sentence, no exclamation mark, with the attempts-remaining count in `font-weight: 500`. Example: `That password doesn't match. 2 attempts left before a 10-minute cooldown.`
- **Success** — the form is replaced by the next route. No toast.

#### Don'ts

- Don't add a "Create account" link. Tap is invite-only by design; that link belongs in the marketing site, not the auth surface.
- Don't add a footer (privacy, status, "what is Tap"). The wordmark is enough chrome.
- Don't add an eyebrow ("01 · SIGN IN") or a subtitle ("Continue to your feeds") on the default screen. The wordmark + serif title is the whole heading.
- Don't put a card around the form. The page is the card.
- Don't add a second accent for the primary CTA. Primary is `var(--ink)` like every other primary button in Tap.

### 6.8 Mobile (PWA)

The mobile shell uses the bottom tab bar (`.tmnav`) with five tabs: **Unread**, **Saved**, **Feeds**, **Categories**, **More**. `History` and `Settings` are reached from the More sheet; when either is active, the More tab lights up.

- Top bar: `60px` top inset for status bar, page title in sans 16/600, count in mono 11 right.
- Entry rows scale down (16px title, 18px gutters).
- Reader uses the dedicated `.m-reader-topbar` + `.m-reader-footbar`.
- Categories list (`.m-cats-*`) renders as cards with feed chips; tapping a feed-chip's `Move ▾` opens a bottom sheet (`.m-cat-sheet`) with a 38×4 grab handle, eyebrow, and the same checked-current item pattern as the desktop popover.

**More sheet (`.tmnav-sheet`)** — bottom sheet, `border-radius: 18px 18px 0 0`, 36×4 grab handle on top. Contents, top to bottom:

1. **Identity row** (`.tmnav-sheet-identity`) — 36×36 accent-blue avatar tile, name (sans 15/600), email (mono 11 `var(--ink-3)`). Bottom border `1px solid var(--rule)`.
2. **Sheet items** (`.tmnav-sheet-item`) — `History` and `Settings`, each a 3-column grid (36 icon · body · 18 chev) with a 15/600 name and a mono 10.5 desc.
3. **1px separator** (`.tmnav-sheet-sep`).
4. **Log out** (`.tmnav-sheet-item.is-logout`) — same row pattern minus the chevron, tinted `#b3402c` (light) / `#e9846f` (dark) on text and icon, with a matching tinted icon background. Desc echoes `Sign out of <email>` so the destructive action is unambiguous.
5. **Foot** — `Close` button, mono 11.

---

## 7. Interactions & shortcuts

| Key | Action |
|---|---|
| `J` / `K` | Next / previous entry |
| `↓` / `↑` | Same as `J`/`K` (with line-by-line scroll inside the reader) |
| `Enter` / `O` | Open selected entry |
| `Esc` / `H` | Back from reader |
| `M` | Toggle read |
| `S` | Toggle saved |
| `V` | Open original (visit) in new tab |
| `R` | Refresh current scope |
| `Shift R` | Refresh all feeds |
| `Shift A` | Mark scope all-read |
| `G U` | Go to Unread |
| `G S` | Go to Saved |
| `G F` | Go to Feeds |
| `G C` | Go to Categories |
| `G ,` | Go to Settings |
| `/` | Focus search |
| `?` | Open shortcut modal |
| `1` / `2` / `3` | Toggle measure narrow / comfortable / wide (in reader) |
| `T` | Cycle theme |

The shortcut modal lives in `.tap-modal` with a two-column body (`.tap-modal-body`) of `.shortcut-group-title` + repeated `.shortcut-row` (`.shortcut-keys` and `.shortcut-desc`). Use `+` mono chip between simultaneous keys; render sequential keys as two separate `.kbd` with a space.

---

## 8. States

Every list, every form, every async surface must define four states. Do not ship a screen without all four:

1. **Empty** — see §4.16. Always end with one CTA or kbd hint.
2. **Loading** — never spinners covering content. Use the polling strip up top and ghosting on rows (entry rows render as 3 hairline blocks with `var(--ink-4)` opacity if needed; in practice, prefer optimistic UI).
3. **Error** — health chip on the offending row, errors filter chip in the toolbar, and a tiny mono status string at the bottom of the page (`.ts-feeds-foot`).
4. **Success / done** — never as a toast. The next render of the list reflects the change. If a destructive action needs ack, do it in the confirmation dialog itself ("This will remove 4 feeds and 308 entries").

---

## 9. Accessibility

- Every interactive element is a `<button>` or `<a>`; never a `<div>` with a click handler.
- The junction dot, accent dot, separators, and decorative SVGs carry `aria-hidden="true"`.
- Read state is reflected with `aria-pressed` on the toggle button **and** via the colour change.
- Live region for poll status: `<div role="status" aria-live="polite" class="poll-strip">`.
- Minimum contrast: all `--ink-2` text passes AA on `--bg` in every theme; `--ink-3` is reserved for non-essential meta and never carries the only signal.
- Hit targets ≥ 36px on desktop, ≥ 44px on mobile.

---

## 10. File map (recommended)

```
styles.css                 — design tokens + every component selector here
data.js                    — sample feeds, categories, entries (replace with real adapter)
tap-components.jsx         — primitives: <Entry>, <Sidebar>, <TopBar>, <KbdChip>, <Chip>, <Segmented>, <Dialog>, <Popover>
tap-reader.jsx             — <Reader>, <ReaderRail>, <ReaderHeader>, <ArticleBody>
tap-categories-page.jsx    — <CategoriesPage>, <CategoryCard>, <ReassignPopover>
tap-feeds-page.jsx         — <FeedsPage>, <FeedRow>, <FeedHealthPanel>, <AddFeedDialog>
tap-saved.jsx              — <SavedPage>
tap-settings.jsx           — <SettingsPage> + section subcomponents
tap-admin.jsx              — <AdminPage>, <SysGrid>, <ErrorsTable>
tap-mobile-nav.jsx         — <MobileTabBar>, <MobileTopBar>, <BottomSheet>
tap-simple.jsx             — <SimpleShell> for the centred layouts
Tap Desktop.html           — desktop canvas (split-pane + simple layouts)
Tap Mobile.html            — mobile canvas
Tap.html                   — landing chooser
```

Everything reads from one `styles.css` and one `data.js` — edit either and both canvases pick it up.

---

## 11. Don'ts

- Don't introduce a second accent colour. Klein Blue is the only colour you get.
- Don't use shadows for depth on the main UI; use hairlines.
- Don't use emoji in any production view.
- Don't use sentence-case "Tap" in product chrome; use the wordmark.
- Don't use serif in form controls; don't use sans in body articles.
- Don't put a toast in the corner. The UI tells the truth on the next render.
- Don't autoplay anything. Don't animate counts. Don't fade pages in.
- Don't ship a screen without an empty state.

---

*End of spec. Questions on a component? Look it up by its selector in `styles.css` — every visible element has one.*
