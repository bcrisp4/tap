# Tap — Design Spec

**Name:** Tap
**Status:** Draft

Tap into your feeds. A clean, content-focused feed reader. This document
is the **visual and UX spec only** — it describes look, feel, brand, and
interaction. It does not describe storage, parsing, or transport.

---

## 1. Product idea (one paragraph)

Tap is a feed reader for people who want to read, not be marketed to. The
interface is monochrome paper-and-ink with a single accent color. The
content fills the viewport when you're reading; everything else collapses
or fades. Three themes, a serif/sans toggle, keyboard-first on desktop
and gesture-first on mobile.

---

## 2. Brand & Visual Identity

### Name & metaphor

**Tap** — to tap into your feeds. The product aggregates multiple
sources into a single chronological stream, the way a signal tap draws a
clean copy off a main line without interrupting it. The name reads on
two levels: the electrical-engineering term, and the everyday gesture of
tapping a screen.

In schematic diagrams a tap is shown as a small filled dot at the
intersection of two wires, marking an electrical connection rather than
a crossover. The junction dot is the load-bearing detail of Tap's
identity.

### Product philosophy

Minimal, fast, clean. No tracking. No algorithm — chronological feeds,
the way RSS was meant to be consumed. The interface is built for
reading, not for engagement metrics.

### Logo system

Three concept variants form a system, all built around the schematic
junction dot:

- **Primary mark (favicon, app icon).** A T-junction. Short horizontal
  line with a vertical line branching down from its midpoint, with a
  filled dot at the intersection. The T-shape doubles as the letter T
  in "tap." Stroke weight medium-bold, rounded line caps. Junction dot
  in Klein Blue against a monochrome line.

- **Wordmark.** Lowercase "tap" in a geometric sans-serif, tight
  letter-spacing. The period after "tap" is rendered slightly larger
  and heavier than typographic norm, so it reads as both a period and a
  schematic junction dot. The dot is Klein Blue.

- **Hero graphic (landing page, marketing).** A multi-tap. One
  horizontal main line with three short vertical branches rising from
  it, junction dots at each branch point, and one branch descending to
  indicate output. Communicates "many sources, one stream." Junction
  dots in Klein Blue.

### Color

- **Klein Blue / International Klein Blue (IKB)** — approximately
  `#002FA7` — is the brand accent. Used for the junction dot,
  interactive elements (links, active states, focus rings), and small
  accents. It evokes electrical signal, voltage, ink. Used sparingly,
  never as a flood.
- The rest of the interface is monochrome: paper-like backgrounds,
  ink-like text.
- **Three themes**, switchable and following the system default:
  - **Light** — off-white background, near-black text, Klein Blue accent.
  - **Dark** — near-black background, off-white text, Klein Blue accent
    (slightly desaturated for contrast in dark mode).
  - **Sepia** — warm cream background (approximately `#F4ECD8`), dark
    brown text, Klein Blue accent retained for continuity. This is the
    dedicated reading mode.

### Typography

- **Body and reading content** — a serif typeface optimised for
  long-form reading. Generous line height (1.6–1.7), comfortable
  measure (65–72 characters per line), no full-width text. Suggested
  options: Source Serif, Iowan Old Style, Charter, or similar.
- **UI chrome, navigation, metadata** — geometric sans-serif. Smaller,
  lower contrast, recedes from the content.
- **Micro-meta and reader-header buttons** (e.g. `MARK UNREAD`,
  `SAVED`, `VIEW ORIGINAL`, kbd hints, group titles like `READING`,
  `FEEDS`) — monospace, small, uppercase, letter-spaced. This is the
  schematic-meta voice.
- **Wordmark and brand** — geometric sans-serif, lowercase, tight
  tracking.
- Sentence case throughout. Never all caps **except** in the monospace
  micro-meta voice above.

### Iconography

Line-based, monochrome, **1.4–1.5 stroke weight on a 16-unit grid**,
rounded caps and joins. Klein Blue reserved for the active state only.

### Feed avatars (icon-or-color)

Each feed has an avatar shown in the sidebar, in entry-row meta, and in
the reader's source line.

1. **If the feed provides an icon**, render the icon cropped to a
   rounded square.
2. **Otherwise**, fall back to a flat color square using a stable
   per-feed color.

The fallback is the **only** fallback — never invent a letter mark or a
generated initial.

### Layout & UI principles

- **Content is the interface.** Article text dominates the viewport
  when reading. Chrome, sidebars, and controls collapse or fade when
  not in active use.
- **No engagement metrics.** No view counts, no reaction icons, no
  share-count badges.
- **Single-column reading view** by default.
- **Keyboard navigation is first-class** on desktop. The full app is
  operable without a mouse.
- **Density is restrained.** Whitespace is the dominant design
  material.
- **Animation is minimal and functional** — fade and short-distance
  state transitions, never decorative motion.

### Visual rules

- The junction dot is always present in some form across brand
  surfaces. It is the load-bearing detail.
- Avoid imagery that suggests faucets, water, drips, or plumbing. The
  tap is electrical, not domestic.
- Avoid full circuit diagrams, PCB traces, or busy schematic clutter.
  The brand uses one symbol, used well.
- Klein Blue is precious. If it appears more than three times on a
  single screen, something is wrong.

### Tone

Quiet confidence. Engineering-literate but not exclusionary. The product
is for people who want to read, not for people who want to be impressed
by software.

---

## 3. Views in scope

| View | Route | Description |
|---|---|---|
| Unread | `/` | Chronological unread entries (default home) |
| Article | `/entry/:id` | Reader view + "view original" button |

Out of scope for this design pass: Saved, Category, Feed, All, Search,
Settings, Add feed, Empty/first-run, Offline. The visual vocabulary
generalises to these.

### Reading experience

- Clean reader view with serif typography by default.
- Single-column layout, comfortable measure (65–72 ch), generous line
  height.
- "View original" button opens source URL in a new tab.
- Reading time displayed per article.
- Feed avatar + source name in article metadata, set in geometric
  sans-serif.

### Interactions

- **Mark as read**: explicit button/tap only — no auto-mark on scroll.
- **Swipe gestures (mobile)**: swipe right → mark read/unread, swipe
  left → save/unsave.
- **Keyboard shortcuts (desktop)**: `j`/`k` navigate, `m` toggle read,
  `s` save, `v` view original, `o` open article, `Esc` back, `/` focus
  search.
- **Bulk actions**: mark all read for feed/category/everything.
- **Pull-to-refresh** on mobile: triggers feed refresh.

### Theming

- Light / dark / sepia / follow system (`prefers-color-scheme`).
- Serif / sans-serif toggle for reading (defaults to serif).
- Klein Blue (`#002FA7`) accent across all themes; slightly desaturated
  in dark.
- CSS custom properties — theme switching is instant, no reload.

### PWA

- Web app manifest with icon for the home screen (uses the T-junction
  primary mark).
- `display: standalone` — looks like a native app.
- Status bar styling matches the active theme.
- Mobile chrome (top bars, status-bar-flush elements) reserves
  `padding-top: 60px` to clear the device status bar.

---

## 4. Inspiration

- **Miniflux** — clean, opinionated, content-focused feed reading.
- **Reeder / NetNewsWire** — restrained, type-driven reading interfaces.
- **Modern reading apps (Reader, Matter, Readwise)** — for the
  full-bleed serif reader and the swipe-driven list.

The full implementation-level spec (typographic ramp, exact tokens,
layout dimensions, per-component CSS) lives in `README.md` and the
prototype files in this bundle.
