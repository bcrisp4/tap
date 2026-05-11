// Tap — Saved view (desktop + mobile)
// Saved entries differ from Unread on three axes:
//   • Grouped by *when saved* (Today / This week / Earlier), not when published.
//   • Byline is promoted — when you keep something for later, who wrote it matters.
//   • Destructive action: Unsave (removes from this list). Mark-read does not.
// Mobile uses a horizontal swipe to reveal Mark-read (left action) and
// Unsave (right action, accent-destructive).

const { useState: useStateSv, useRef: useRefSv } = React;

// ─── Saved-only metadata (saved_at, author, published date) ──────────────
// In a real backend this would come from saved_entries rows + parsed feed
// fields; here we attach a stable derived blob keyed by entry id, plus a
// few extra saved samples so the list breathes.
const TAP_SAVED_META = {
  102: { author: "burner_42",        savedBucket: "Today",    savedAgo: "2h",   pub: "Apr 26, 2026" },
  107: { author: "Fabien Sanglard",  savedBucket: "This week", savedAgo: "2d",  pub: "Apr 24, 2026" },
  114: { author: "Julia Evans",      savedBucket: "This week", savedAgo: "4d",  pub: "Apr 22, 2026" },
};

const TAP_SAVED_EXTRAS = [
  { id: 901, feed: 6,
    title: "TLA+ for people who already know property-based testing",
    summary: "If you already write Hypothesis or QuickCheck, you already think in invariants and shrinking. TLA+ is the same instinct with a model checker instead of a fuzzer. A twenty-minute on-ramp.",
    rt: 14, ago: "5d", read: true,  saved: true,
    _author: "Hillel Wayne",   _savedBucket: "This week", _savedAgo: "5d", _pub: "Apr 21, 2026" },
  { id: 902, feed: 2,
    title: "Files are fraught with peril",
    summary: "fsync is a lie, rename is a lie, the page cache is a liar, your SSD is a liar, and your filesystem is a liar. A long, deeply-cited tour of every lie.",
    rt: 41, ago: "1w", read: false, saved: true,
    _author: "Dan Luu",        _savedBucket: "Earlier",   _savedAgo: "1w", _pub: "Apr 14, 2026" },
  { id: 903, feed: 11,
    title: "Notes from running a small LLM locally for a month",
    summary: "I put a 7B model on the laptop and tried to use it for everything for thirty days. Some things that surprised me, some that didn't.",
    rt: 18, ago: "2w", read: true,  saved: true,
    _author: "Simon Willison", _savedBucket: "Earlier",   _savedAgo: "2w", _pub: "Apr 03, 2026" },
  { id: 904, feed: 8,
    title: "Go's range-over-func, in practice",
    summary: "I rewrote three small libraries to use range-over-func iterators after upgrading to 1.23. Two were genuinely cleaner. One got worse.",
    rt: 9, ago: "3w", read: true,  saved: true,
    _author: "Eli Bendersky",  _savedBucket: "Earlier",   _savedAgo: "3w", _pub: "Mar 28, 2026" },
];

function metaForSaved(e) {
  if (e._author) {
    return { author: e._author, savedBucket: e._savedBucket, savedAgo: e._savedAgo, pub: e._pub };
  }
  return TAP_SAVED_META[e.id] || { author: "—", savedBucket: "Earlier", savedAgo: e.ago, pub: e.ago };
}

// Build the full saved list = entries.filter(saved) ∪ extras, in order
// most-recently-saved first.
function buildSavedItems(entries) {
  const base = entries.filter((e) => e.saved);
  const all = [...base, ...TAP_SAVED_EXTRAS];
  // Stable order: by bucket then by index added.
  const order = { "Today": 0, "This week": 1, "Earlier": 2 };
  return all.sort((a, b) => {
    const ma = metaForSaved(a), mb = metaForSaved(b);
    return (order[ma.savedBucket] ?? 9) - (order[mb.savedBucket] ?? 9);
  });
}

// ─── Bookmark glyph used in empty state + toolbar ────────────────────────
function SvIcoBookmark({ size = 16 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path d="M4 2.5h8v11l-4-3-4 3z" />
    </svg>
  );
}
function SvIcoCheck({ size = 12 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor"
      strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 8.5 6.5 12 13 5" />
    </svg>
  );
}
function SvIcoOpen({ size = 12 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor"
      strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 8h9M8.5 4.5 12 8l-3.5 3.5" />
    </svg>
  );
}
function SvIcoMinus({ size = 12 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor"
      strokeWidth="1.6" strokeLinecap="round" aria-hidden="true">
      <path d="M3.5 8h9" />
    </svg>
  );
}
function SvIcoSort({ size = 12 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor"
      strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 4v9M4 13l-2-2M4 13l2-2 M12 12V3M12 3l-2 2M12 3l2 2" />
    </svg>
  );
}

// ─── Toolbar: count + sort menu + search hint ────────────────────────────
function TSSavedToolbar({ count, sort = "recent", openMenu = false, onSort }) {
  const labels = {
    recent: "Recently saved",
    oldest: "Oldest first",
    title:  "Title (A–Z)",
    source: "Source",
  };
  return (
    <div className="ts-saved-toolbar">
      <div className="ts-saved-toolbar-left">
        <span className="ts-saved-toolbar-eyebrow">Saved</span>
        <span className="ts-saved-toolbar-count">
          <b>{count}</b> entr{count === 1 ? "y" : "ies"}
        </span>
      </div>
      <div className="ts-saved-toolbar-right">
        <div className="ts-saved-sort-wrap">
          <button type="button" className={`ts-saved-sort ${openMenu ? "is-open" : ""}`}>
            <SvIcoSort />
            <span>Sort · {labels[sort]}</span>
            <span className="ts-saved-sort-chev" aria-hidden="true">▾</span>
          </button>
          {openMenu && (
            <div className="ts-saved-sort-menu" role="menu">
              {Object.entries(labels).map(([k, v]) => (
                <button key={k} type="button"
                  className={`ts-saved-sort-item ${sort === k ? "is-current" : ""}`}
                  role="menuitemradio" aria-checked={sort === k}
                  onClick={() => onSort?.(k)}>
                  <span className="ts-saved-sort-bullet" aria-hidden="true">
                    {sort === k ? <SvIcoCheck size={10} /> : null}
                  </span>
                  <span>{v}</span>
                </button>
              ))}
              <div className="ts-saved-sort-foot">
                <span className="mono">↑↓ to move</span>
                <span className="mono">↵ select</span>
              </div>
            </div>
          )}
        </div>
        <span className="ts-saved-toolbar-find">
          Find <kbd className="ts-kbd">/</kbd>
        </span>
      </div>
    </div>
  );
}

// ─── Group heading specifically scoped to "Saved" (reuses ts-group) ──────
function TSSavedGroupHeading({ label, count }) {
  return (
    <div className="ts-group-heading ts-saved-group-heading">
      <span className="ts-group-label">Saved · {label}</span>
      <span className="ts-group-rule" aria-hidden="true"></span>
      <span className="ts-group-count">{count}</span>
    </div>
  );
}

// ─── A single saved row (desktop) ─────────────────────────────────────────
// Click = open the entry. Row exposes Open / Mark read / Unsave on hover
// (always visible on keyboard-selected row). The eyebrow line carries the
// "saved Xd ago · published Apr N · read" status so you can see why this
// row is here without opening it.
function TSSavedRow({ entry, feed, showSummary, hovered, selected, forceActions, onHover, onOpen }) {
  const meta = metaForSaved(entry);
  const cls = [
    "ts-saved-row",
    entry.read ? "is-read" : "",
    hovered ? "is-hovered" : "",
    selected ? "is-selected" : "",
    forceActions ? "is-acting" : "",
  ].filter(Boolean).join(" ");
  return (
    <article className={cls}
      onMouseEnter={() => onHover?.(entry.id)}
      onMouseLeave={() => onHover?.(null)}
      onClick={onOpen}>
      <span className="ts-saved-rail" aria-hidden="true">
        <span className="ts-saved-rail-mark"><SvIcoBookmark size={10} /></span>
      </span>

      <div className="ts-saved-body">
        <div className="ts-saved-eyebrow">
          <span>saved {meta.savedAgo} ago</span>
          <span className="ts-sep" aria-hidden="true">·</span>
          <span>published {meta.pub}</span>
          {entry.read && (
            <>
              <span className="ts-sep" aria-hidden="true">·</span>
              <span className="ts-saved-status-read">read</span>
            </>
          )}
        </div>
        <h3 className="ts-saved-title">{entry.title}</h3>
        <div className="ts-saved-byline">
          <FeedAvatar feed={feed} size={11} radius={2} />
          <span className="ts-saved-source">{feed.name}</span>
          <span className="ts-sep" aria-hidden="true">·</span>
          <span className="ts-saved-author">{meta.author}</span>
          <span className="ts-sep" aria-hidden="true">·</span>
          <span className="ts-rt">{entry.rt} min read</span>
        </div>
        {showSummary && entry.summary && (
          <p className="ts-saved-summary">{entry.summary}</p>
        )}
      </div>

      <div className="ts-saved-actions" onClick={(e) => e.stopPropagation()}>
        <button type="button" className="ts-saved-action">
          <SvIcoOpen />
          <span>Open</span>
          <kbd className="ts-kbd">↵</kbd>
        </button>
        <button type="button" className="ts-saved-action">
          {entry.read ? <SvIcoMinus /> : <SvIcoCheck />}
          <span>{entry.read ? "Mark unread" : "Mark read"}</span>
          <kbd className="ts-kbd">m</kbd>
        </button>
        <button type="button" className="ts-saved-action is-destructive">
          <SvIcoBookmark size={11} />
          <span>Unsave</span>
          <kbd className="ts-kbd">s</kbd>
        </button>
      </div>
    </article>
  );
}

// ─── Empty state ─────────────────────────────────────────────────────────
function TSSavedEmpty() {
  return (
    <div className="ts-saved-empty">
      <div className="ts-saved-empty-mark" aria-hidden="true">
        <svg width="40" height="56" viewBox="0 0 40 56" fill="none">
          <path d="M3.5 3.5h33v49l-16.5-12.5L3.5 52.5z"
            stroke="var(--ink-4)" strokeWidth="1.4" strokeLinejoin="round" />
          <path d="M14 22h12M14 28h8" stroke="var(--ink-4)"
            strokeWidth="1.4" strokeLinecap="round" />
          <circle cx="20" cy="40" r="2.6" fill="var(--accent)" />
        </svg>
      </div>
      <div className="ts-saved-empty-title">Nothing saved yet.</div>
      <p className="ts-saved-empty-sub">
        Saving keeps a copy of an entry here, cached locally, even if
        the source disappears. Press <kbd className="ts-kbd">s</kbd> on any
        entry to save it — or tap the bookmark in the reader.
      </p>
      <div className="ts-saved-empty-hints">
        <div className="ts-saved-empty-hint">
          <span className="ts-saved-empty-hint-k"><kbd className="ts-kbd">s</kbd></span>
          <span>save the highlighted entry</span>
        </div>
        <div className="ts-saved-empty-hint">
          <span className="ts-saved-empty-hint-k"><kbd className="ts-kbd">g</kbd> <kbd className="ts-kbd">s</kbd></span>
          <span>jump back here</span>
        </div>
        <div className="ts-saved-empty-hint">
          <span className="ts-saved-empty-hint-k"><SvIcoBookmark size={11} /></span>
          <span>row-level bookmark icon</span>
        </div>
      </div>
    </div>
  );
}

// ─── Desktop saved view ──────────────────────────────────────────────────
function TapSavedDesktop({
  entries,
  feeds,
  theme = "light",
  fontMode = "serif",
  showSummary = true,
  force = {},
}) {
  const items = force.empty ? [] : buildSavedItems(entries);
  const unread = entries.filter((e) => !e.read).length;
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};

  const sort = force.sort || "recent";
  const hovered = force.hoveredId ?? null;
  const selected = force.selectedId ?? null;
  const sortOpen = !!force.sortOpen;

  // Bucket the items in display order.
  const buckets = { "Today": [], "This week": [], "Earlier": [] };
  items.forEach((e) => {
    const m = metaForSaved(e);
    (buckets[m.savedBucket] || buckets["Earlier"]).push(e);
  });

  return (
    <div className={`tap theme-${theme} ts-root`} style={fontStyle}>
      <div className="ts-shell">
        <TopTabs active="saved" unreadCount={unread} onNav={() => {}} />
        <main className="ts-main">
          {items.length === 0 ? (
            <TSSavedEmpty />
          ) : (
            <>
              <TSSavedToolbar
                count={items.length}
                sort={sort}
                openMenu={sortOpen}
              />
              <div className="ts-list ts-saved-list">
                {["Today", "This week", "Earlier"].map((bucket) => {
                  const arr = buckets[bucket];
                  if (!arr || arr.length === 0) return null;
                  return (
                    <React.Fragment key={bucket}>
                      <TSSavedGroupHeading label={bucket} count={arr.length} />
                      {arr.map((e) => (
                        <TSSavedRow
                          key={e.id}
                          entry={e}
                          feed={feeds.find((f) => f.id === e.feed)}
                          showSummary={showSummary}
                          hovered={hovered === e.id}
                          selected={selected === e.id}
                          forceActions={hovered === e.id || selected === e.id}
                          onHover={() => {}}
                          onOpen={() => {}}
                        />
                      ))}
                    </React.Fragment>
                  );
                })}
              </div>
            </>
          )}
        </main>
        <footer className="ts-foot">
          <TSStatus unread={unread} lastPoll="6 min ago" />
        </footer>
      </div>
    </div>
  );
}

// ─── Mobile saved row (with optional swipe-revealed actions) ─────────────
// swipe = 'left'  → revealed right-side actions (Unsave destructive)
// swipe = 'right' → revealed left-side action (Mark read/unread)
function TSSavedMobileRow({ entry, feed, swipe }) {
  const meta = metaForSaved(entry);
  const cls = [
    "ts-saved-m-row",
    entry.read ? "is-read" : "",
    swipe === "left"  ? "is-swipe-left"  : "",
    swipe === "right" ? "is-swipe-right" : "",
  ].filter(Boolean).join(" ");
  return (
    <div className={cls}>
      {/* Left action (revealed by swiping right) */}
      <div className="ts-saved-m-rev ts-saved-m-rev-left">
        {entry.read ? <SvIcoMinus /> : <SvIcoCheck />}
        <span>{entry.read ? "Mark unread" : "Mark read"}</span>
      </div>
      {/* Right action (revealed by swiping left) */}
      <div className="ts-saved-m-rev ts-saved-m-rev-right">
        <span>Unsave</span>
        <SvIcoBookmark size={14} />
      </div>

      <article className="ts-saved-m-card">
        <div className="ts-saved-m-eyebrow">
          <span>saved {meta.savedAgo} ago</span>
          {entry.read && (
            <>
              <span className="ts-sep" aria-hidden="true">·</span>
              <span className="ts-saved-status-read">read</span>
            </>
          )}
        </div>
        <h3 className="ts-saved-m-title">{entry.title}</h3>
        <div className="ts-saved-m-byline">
          <FeedAvatar feed={feed} size={10} radius={2} />
          <span className="ts-saved-m-source">{feed.name}</span>
          <span className="ts-sep" aria-hidden="true">·</span>
          <span>{meta.author}</span>
        </div>
        <p className="ts-saved-m-summary">{entry.summary}</p>
        <div className="ts-saved-m-foot">
          <span>published {meta.pub}</span>
          <span className="ts-sep" aria-hidden="true">·</span>
          <span className="ts-rt">{entry.rt} min read</span>
        </div>
      </article>
    </div>
  );
}

// ─── Mobile saved view ───────────────────────────────────────────────────
function TapSavedMobile({
  entries,
  feeds,
  theme = "light",
  fontMode = "serif",
  force = {},
}) {
  const items = force.empty ? [] : buildSavedItems(entries);
  const unread = entries.filter((e) => !e.read).length;
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};

  // force.swipe = { id, dir } reveals an action on one row
  const swipeFor = (id) => (force.swipe && force.swipe.id === id ? force.swipe.dir : null);

  // Bucket
  const buckets = { "Today": [], "This week": [], "Earlier": [] };
  items.forEach((e) => {
    const m = metaForSaved(e);
    (buckets[m.savedBucket] || buckets["Earlier"]).push(e);
  });

  return (
    <div className={`tap theme-${theme} ts-root is-mobile`} style={fontStyle}>
      <TapMobileShell
        title="Saved"
        eyebrow="Library"
        count={items.length}
        countLabel={items.length === 1 ? "entry" : "entries"}
        active="saved"
        unreadCount={unread}
        onNav={() => {}}
        bodyClass="tmbody-list tmbody-saved"
      >
        {items.length === 0 ? (
          <TSSavedEmpty />
        ) : (
          <div className="ts-saved-m-list">
            {["Today", "This week", "Earlier"].map((bucket) => {
              const arr = buckets[bucket];
              if (!arr || arr.length === 0) return null;
              return (
                <React.Fragment key={bucket}>
                  <div className="ts-saved-m-grouphead">
                    <span>Saved · {bucket}</span>
                    <span className="ts-saved-m-grouphead-rule" aria-hidden="true"></span>
                    <span className="ts-saved-m-grouphead-count">{arr.length}</span>
                  </div>
                  {arr.map((e) => (
                    <TSSavedMobileRow
                      key={e.id}
                      entry={e}
                      feed={feeds.find((f) => f.id === e.feed)}
                      swipe={swipeFor(e.id)}
                    />
                  ))}
                </React.Fragment>
              );
            })}
            {force.swipe && (
              <div className="ts-saved-m-hint" aria-hidden="true">
                <span className="ts-saved-m-hint-dot"></span>
                {force.swipe.dir === "left"
                  ? "Swipe left to unsave · release past halfway"
                  : "Swipe right to mark read"}
              </div>
            )}
          </div>
        )}
      </TapMobileShell>
    </div>
  );
}

Object.assign(window, {
  TapSavedDesktop, TapSavedMobile,
  TSSavedRow, TSSavedEmpty, TSSavedToolbar,
  buildSavedItems, metaForSaved,
});
