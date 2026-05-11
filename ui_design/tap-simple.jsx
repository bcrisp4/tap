// Tap — simplified single-column layout, no sidebar.
// Top-tab navigation; centered reading column; everything else falls away.

const { useState: useStateSimple, useEffect: useEffectSimple } = React;

// ────── Top-tab navigation ──────
function TopTabs({ active = "unread", unreadCount, onNav }) {
  const tabs = [
    { id: "unread",     label: "Unread"     },
    { id: "saved",      label: "Saved"      },
    { id: "history",    label: "History"    },
    { id: "categories", label: "Categories" },
    { id: "feeds",      label: "Feeds"      },
    { id: "settings",   label: "Settings"   },
  ];
  const [accountOpen, setAccountOpen] = useStateSimple(false);
  const USER = (typeof DEFAULT_USER !== "undefined") ? DEFAULT_USER : { name: "Jodie Park", email: "jodie@thecrisp.io", initial: "J" };
  const handlePick = (id) => {
    setAccountOpen(false);
    if (id === "logout") {
      if (typeof window !== "undefined") window.dispatchEvent(new CustomEvent("tap:logout"));
    } else if (id === "account" || id === "theme") {
      onNav?.("settings");
    }
  };
  return (
    <React.Fragment>
      <nav className="ts-nav">
        <a className="ts-wordmark" href="#" aria-label="Tap home">
          tap<span className="ts-wordmark-dot" aria-hidden="true"></span>
        </a>
        <ul className="ts-tabs">
          {tabs.map((t) => (
            <li key={t.id}>
              <button
                type="button"
                className={`ts-tab ${active === t.id ? "is-active" : ""}`}
                onClick={() => onNav?.(t.id)}
              >
                <span>{t.label}</span>
                {t.id === "unread" && typeof unreadCount === "number" && (
                  <span className="ts-tab-count">{unreadCount}</span>
                )}
              </button>
            </li>
          ))}
        </ul>
      </nav>
      <button
        type="button"
        className={`ts-account ${accountOpen ? "is-open" : ""}`}
        aria-haspopup="menu"
        aria-expanded={accountOpen ? "true" : "false"}
        aria-label={`Account: ${USER.email}`}
        title={USER.email}
        onClick={() => setAccountOpen(true)}
      >
        <span className="ts-account-avatar" aria-hidden="true">
          {USER.initial || (USER.name || USER.email || "?")[0].toUpperCase()}
        </span>
      </button>
      {accountOpen && (typeof AccountMenu !== "undefined") && (
        <AccountMenu open={accountOpen} user={USER} onClose={() => setAccountOpen(false)} onPick={handlePick} />
      )}
    </React.Fragment>
  );
}

// ────── Simplified entry row ──────
function TSEntryRow({ entry, feed, isSelected, onClick, showSummary }) {
  const cls = [
    "ts-entry",
    entry.read ? "is-read" : "",
    entry.saved ? "is-saved" : "",
    isSelected ? "is-selected" : "",
  ].filter(Boolean).join(" ");
  return (
    <article className={cls} onClick={onClick}>
      <span className="ts-entry-dot" aria-hidden="true"></span>
      <h3 className="ts-entry-title">{entry.title}</h3>
      <div className="ts-entry-meta">
        <FeedAvatar feed={feed} size={10} radius={2} />
        <span className="ts-entry-source">{feed.name}</span>
        <span className="ts-sep" aria-hidden="true">·</span>
        <span>{entry.ago} ago</span>
        <span className="ts-sep" aria-hidden="true">·</span>
        <span className="ts-rt">{entry.rt} min</span>
        {entry.saved && (
          <>
            <span className="ts-sep" aria-hidden="true">·</span>
            <span className="ts-saved-tag">saved</span>
          </>
        )}
      </div>
      {showSummary && entry.summary && (
        <p className="ts-entry-summary">{entry.summary}</p>
      )}
    </article>
  );
}

// ────── Empty / footer status line ──────
function TSStatus({ unread, lastPoll }) {
  return (
    <div className="ts-status">
      <span className="ts-status-dot" aria-hidden="true"></span>
      <span>{unread === 0 ? "all caught up" : `${unread} unread`}</span>
      <span className="ts-status-sep" aria-hidden="true">·</span>
      <span>polled {lastPoll}</span>
      <span className="ts-status-sep" aria-hidden="true">·</span>
      <span>press <kbd className="ts-kbd">?</kbd> for shortcuts</span>
    </div>
  );
}

// ────── Section heading inside the list (e.g., "Today", "Yesterday") ──────
function TSGroupHeading({ label, count }) {
  return (
    <div className="ts-group-heading">
      <span className="ts-group-label">{label}</span>
      <span className="ts-group-rule" aria-hidden="true"></span>
      <span className="ts-group-count">{count}</span>
    </div>
  );
}

// Bucket entries by relative day from `ago` string heuristics.
function bucketByDay(entries) {
  const buckets = { Today: [], Yesterday: [], Earlier: [] };
  entries.forEach((e) => {
    const a = (e.ago || "").toLowerCase();
    if (a.includes("min") || a.includes("hr") || a.includes("hour") || a === "now") buckets.Today.push(e);
    else if (a.includes("1 d") || a.includes("1d") || a.startsWith("yesterday")) buckets.Yesterday.push(e);
    else buckets.Earlier.push(e);
  });
  return buckets;
}

// ────── Saved view: entries with `saved: true` ──────
function TSSavedList({ entries, feeds, selectedId, onSelect, showSummary }) {
  const items = entries.filter((e) => e.saved);
  if (items.length === 0) {
    return (
      <div className="ts-empty">
        <div className="ts-empty-dot" aria-hidden="true"></div>
        <div className="ts-empty-title">No saved entries yet</div>
        <div className="ts-empty-sub">Press <kbd className="ts-kbd">s</kbd> on any entry to save it for later.</div>
      </div>
    );
  }
  return (
    <div className="ts-list">
      {items.map((e) => (
        <TSEntryRow
          key={e.id}
          entry={e}
          feed={feeds.find((f) => f.id === e.feed)}
          isSelected={selectedId === e.id}
          onClick={() => onSelect?.(e.id)}
          showSummary={showSummary}
        />
      ))}
    </div>
  );
}

// ────── Main unread view (grouped by day) ──────
function TSUnreadList({ entries, feeds, selectedId, onSelect, showSummary, grouped = true }) {
  const items = entries.filter((e) => !e.read);
  if (items.length === 0) {
    return (
      <div className="ts-empty">
        <div className="ts-empty-dot" aria-hidden="true"></div>
        <div className="ts-empty-title">Inbox zero.</div>
        <div className="ts-empty-sub">Tap polls every 30 minutes. Come back later.</div>
      </div>
    );
  }
  if (!grouped) {
    return (
      <div className="ts-list">
        {items.map((e) => (
          <TSEntryRow
            key={e.id}
            entry={e}
            feed={feeds.find((f) => f.id === e.feed)}
            isSelected={selectedId === e.id}
            onClick={() => onSelect?.(e.id)}
            showSummary={showSummary}
          />
        ))}
      </div>
    );
  }
  const buckets = bucketByDay(items);
  const order = ["Today", "Yesterday", "Earlier"];
  return (
    <div className="ts-list">
      {order.map((label) => {
        const bucket = buckets[label];
        if (!bucket || bucket.length === 0) return null;
        return (
          <React.Fragment key={label}>
            <TSGroupHeading label={label} count={bucket.length} />
            {bucket.map((e) => (
              <TSEntryRow
                key={e.id}
                entry={e}
                feed={feeds.find((f) => f.id === e.feed)}
                isSelected={selectedId === e.id}
                onClick={() => onSelect?.(e.id)}
                showSummary={showSummary}
              />
            ))}
          </React.Fragment>
        );
      })}
    </div>
  );
}

// ────── The whole simplified desktop view ──────
function TapSimple({
  entries,
  feeds,
  theme = "light",
  fontMode = "serif",
  showSummary = true,
  view = "unread",
  selectedId: initialSelected = 101,
  grouped = true,
}) {
  const [items, setItems] = useStateSimple(entries);
  const [selected, setSelected] = useStateSimple(initialSelected);
  const [activeView, setActiveView] = useStateSimple(view);

  const unread = items.filter((e) => !e.read).length;
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};

  // Keyboard nav
  useEffectSimple(() => {
    const visible = activeView === "saved"
      ? items.filter((e) => e.saved)
      : items.filter((e) => !e.read);
    const onKey = (ev) => {
      if (ev.target.tagName === "INPUT" || ev.target.tagName === "TEXTAREA") return;
      const idx = visible.findIndex((e) => e.id === selected);
      if (ev.key === "j" || ev.key === "ArrowDown") {
        ev.preventDefault();
        const next = visible[Math.min(idx + 1, visible.length - 1)];
        if (next) setSelected(next.id);
      } else if (ev.key === "k" || ev.key === "ArrowUp") {
        ev.preventDefault();
        const prev = visible[Math.max(idx - 1, 0)];
        if (prev) setSelected(prev.id);
      } else if (ev.key === "m") {
        ev.preventDefault();
        setItems(items.map((e) => e.id === selected ? { ...e, read: !e.read } : e));
      } else if (ev.key === "s") {
        ev.preventDefault();
        setItems(items.map((e) => e.id === selected ? { ...e, saved: !e.saved } : e));
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [selected, items, activeView]);

  return (
    <div className={`tap theme-${theme} ts-root`} style={fontStyle}>
      <div className="ts-shell">
        <TopTabs active={activeView} unreadCount={unread} onNav={setActiveView} />
        <main className="ts-main">
          {activeView === "unread" && (
            <TSUnreadList
              entries={items}
              feeds={feeds}
              selectedId={selected}
              onSelect={setSelected}
              showSummary={showSummary}
              grouped={grouped}
            />
          )}
          {activeView === "saved" && (
            <TSSavedList
              entries={items}
              feeds={feeds}
              selectedId={selected}
              onSelect={setSelected}
              showSummary={showSummary}
            />
          )}
          {activeView === "history" && (
            <div className="ts-empty">
              <div className="ts-empty-dot" aria-hidden="true"></div>
              <div className="ts-empty-title">History</div>
              <div className="ts-empty-sub">Everything you've read, newest first.</div>
            </div>
          )}
          {activeView === "categories" && (
            <div className="ts-empty">
              <div className="ts-empty-dot" aria-hidden="true"></div>
              <div className="ts-empty-title">Categories</div>
              <div className="ts-empty-sub">Group feeds into named buckets.</div>
            </div>
          )}
          {activeView === "feeds" && (
            <div className="ts-empty">
              <div className="ts-empty-dot" aria-hidden="true"></div>
              <div className="ts-empty-title">Feeds</div>
              <div className="ts-empty-sub">Manage your subscriptions.</div>
            </div>
          )}
          {activeView === "settings" && (
            <TapSettings theme={theme} fontMode={fontMode} density="default" totp="off" />
          )}
        </main>
        <footer className="ts-foot">
          <TSStatus unread={unread} lastPoll="6 min ago" />
        </footer>
      </div>
    </div>
  );
}

// ────── Mobile variant — shared TapMobileShell across all top-level views ──────
function TapSimpleMobile({ entries, feeds, theme = "light", fontMode = "serif", initialView = "unread" }) {
  const [items] = useStateSimple(entries);
  const [activeView, setActiveView] = useStateSimple(initialView);
  const unread = items.filter((e) => !e.read).length;
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};

  const TITLES = {
    unread: "Unread",
    saved: "Saved",
    history: "History",
    categories: "Categories",
    feeds: "Feeds",
    settings: "Settings",
  };

  const headMeta = () => {
    if (activeView === "unread") return { count: unread, countLabel: unread === 1 ? "unread" : "unread" };
    if (activeView === "saved")  return { count: items.filter((e) => e.saved).length, countLabel: "saved" };
    if (activeView === "history") return { count: items.filter((e) => e.read).length, countLabel: "read" };
    return { count: null };
  };
  const { count, countLabel } = headMeta();

  return (
    <div className={`tap theme-${theme} ts-root is-mobile`} style={fontStyle}>
      <TapMobileShell
        title={TITLES[activeView] || ""}
        count={count}
        countLabel={countLabel}
        active={activeView}
        unreadCount={unread}
        onNav={setActiveView}
        bodyClass="tmbody-list"
      >
        {activeView === "unread" && (
          <TSUnreadList
            entries={items} feeds={feeds}
            selectedId={null} onSelect={() => {}}
            showSummary={true} grouped={true}
          />
        )}
        {activeView === "saved" && (
          <TSSavedList
            entries={items} feeds={feeds}
            selectedId={null} onSelect={() => {}}
            showSummary={true}
          />
        )}
        {activeView === "history" && (
          <div className="ts-empty">
            <div className="ts-empty-dot" aria-hidden="true"></div>
            <div className="ts-empty-title">History</div>
            <div className="ts-empty-sub">Everything you've read, newest first.</div>
          </div>
        )}
        {activeView === "categories" && (
          <div className="ts-empty">
            <div className="ts-empty-dot" aria-hidden="true"></div>
            <div className="ts-empty-title">Categories</div>
            <div className="ts-empty-sub">Group feeds into named buckets.</div>
          </div>
        )}
        {activeView === "feeds" && (
          <div className="ts-empty">
            <div className="ts-empty-dot" aria-hidden="true"></div>
            <div className="ts-empty-title">Feeds</div>
            <div className="ts-empty-sub">Manage your subscriptions.</div>
          </div>
        )}
        {activeView === "settings" && (
          <div className="ts-empty">
            <div className="ts-empty-dot" aria-hidden="true"></div>
            <div className="ts-empty-title">Settings</div>
            <div className="ts-empty-sub">Theme, account, security.</div>
          </div>
        )}
      </TapMobileShell>
    </div>
  );
}

// ────── Article reader (single-column) ──────
// Uses the same .ts-shell as the list view. A "back" row replaces the
// active-tab underline; the article body sits in the same column.

function TSArticle({ entry, feed }) {
  return (
    <article className="ts-article">
      <div className="ts-article-source">
        <FeedAvatar feed={feed} size={14} radius={3} />
        <span className="ts-article-source-name">{feed.name}</span>
        <span className="ts-sep" aria-hidden="true">·</span>
        <span className="ts-article-source-url">{feed.url}</span>
      </div>
      <h1 className="ts-article-title">{entry.title}</h1>
      <div className="ts-article-byline">
        <span>By Julia Evans</span>
        <span className="ts-sep" aria-hidden="true">·</span>
        <span>April 26, 2026</span>
        <span className="ts-sep" aria-hidden="true">·</span>
        <span>{entry.rt} min read</span>
      </div>

      <div className="ts-article-actions">
        <button type="button" className="ts-article-action">
          <span className="ts-action-dot" aria-hidden="true"></span>
          <span>Mark unread</span>
          <kbd className="ts-kbd">m</kbd>
        </button>
        <button type="button" className="ts-article-action is-saved">
          <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>
          <span>Saved</span>
          <kbd className="ts-kbd">s</kbd>
        </button>
        <button type="button" className="ts-article-action">
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M9 3h4v4M13 3 7 9M11 9.5V13H3V5h3.5"/></svg>
          <span>Original</span>
          <kbd className="ts-kbd">v</kbd>
        </button>
      </div>

      <div className="ts-article-rule" aria-hidden="true">
        <span className="ts-article-rule-line"></span>
        <span className="ts-article-rule-dot"></span>
        <span className="ts-article-rule-line"></span>
      </div>

      {entry.summary && <p className="ts-article-lede">{entry.summary}</p>}

      {(window.READER_BODY || []).map((b, i) => {
        if (b.type === "h2") return <h2 key={i} className="ts-article-h2">{b.text}</h2>;
        if (b.type === "pre") return <pre key={i} className="ts-article-pre"><code>{b.text}</code></pre>;
        return <p key={i} className="ts-article-p">{b.text}</p>;
      })}

      <div className="ts-article-end" aria-hidden="true">
        <span className="ts-article-rule-line"></span>
        <span className="ts-article-end-dot"></span>
        <span className="ts-article-rule-line"></span>
      </div>
      <div className="ts-article-foot">
        Cached locally · last refreshed 32m ago
      </div>
    </article>
  );
}

function TSBackRow({ from = "Unread", onBack }) {
  return (
    <div className="ts-backrow">
      <button type="button" className="ts-back" onClick={onBack}>
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M10 3 5 8l5 5"/></svg>
        <span>Back to {from}</span>
      </button>
      <span className="ts-back-hint">
        <kbd className="ts-kbd">Esc</kbd>
      </span>
    </div>
  );
}

function TapSimpleReader({
  entries,
  feeds,
  entryId = 101,
  theme = "light",
  fontMode = "serif",
}) {
  const entry = entries.find((e) => e.id === entryId);
  const feed = feeds.find((f) => f.id === entry.feed);
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};
  const unread = entries.filter((e) => !e.read).length;
  return (
    <div className={`tap theme-${theme} ts-root ts-root-reader`} style={fontStyle}>
      <div className="ts-shell ts-shell-reader">
        <TopTabs active="unread" unreadCount={unread} onNav={() => {}} />
        <TSBackRow from="Unread" />
        <main className="ts-main">
          <TSArticle entry={entry} feed={feed} />
        </main>
      </div>
    </div>
  );
}

function TapSimpleReaderMobile({
  entries,
  feeds,
  entryId = 103,
  theme = "light",
  fontMode = "serif",
  sourceList = "Unread",
}) {
  const entry = entries.find((e) => e.id === entryId);
  const feed = feeds.find((f) => f.id === entry.feed);
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};
  return (
    <div className={`tap theme-${theme} ts-root is-mobile ts-root-reader`} style={{ display: "flex", flexDirection: "column", height: "100%", ...fontStyle }}>
      <div className="ts-mobile-reader-head">
        <button type="button" className="ts-mobile-back" aria-label={`Back to ${sourceList}`}>
          <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M10 3 5 8l5 5"/></svg>
          <span>{sourceList}</span>
        </button>
        <a className="ts-wordmark" href="#" aria-label="Tap home">
          tap<span className="ts-wordmark-dot" aria-hidden="true"></span>
        </a>
        <button type="button" className="ts-mobile-action is-saved" aria-label="Saved">
          <svg width="18" height="18" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>
        </button>
      </div>
      <div className="ts-mobile-reader-body">
        <TSArticle entry={entry} feed={feed} />
      </div>
      <div className="ts-mobile-reader-foot">
        <button type="button" className="ts-mobile-foot-btn">
          <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><circle cx="8" cy="8" r="3.5"/></svg>
          <span>Mark unread</span>
        </button>
        <button type="button" className="ts-mobile-foot-btn">
          <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M9 3h4v4M13 3 7 9M11 9.5V13H3V5h3.5"/></svg>
          <span>Original</span>
        </button>
        <button type="button" className="ts-mobile-foot-btn">
          <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M3 8h10M9 4l4 4-4 4"/></svg>
          <span>Next</span>
        </button>
      </div>
    </div>
  );
}

Object.assign(window, {
  TopTabs, TSEntryRow, TSUnreadList, TSSavedList, TSStatus, TSGroupHeading,
  TapSimple, TapSimpleMobile,
  TSArticle, TSBackRow, TapSimpleReader, TapSimpleReaderMobile,
});
