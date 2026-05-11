// Unread list component — the unread reading list.
// Used inside both desktop and mobile frames.

const { useState, useEffect, useCallback, useRef } = React;

function Junction() {
  return (
    <span className="tap-mark" aria-hidden="true">
      <span className="dot"></span>
    </span>
  );
}

function Wordmark() {
  return (
    <span className="wordmark">tap<span className="dot"></span></span>
  );
}

// Simple inline icons (line-based, monochrome)
const Icon = {
  search: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><circle cx="7" cy="7" r="4.5"/><path d="m10.5 10.5 3 3"/></svg>,
  refresh: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><path d="M14 8a6 6 0 1 1-1.76-4.24"/><path d="M14 2.5V6h-3.5"/></svg>,
  check: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><path d="m3 8 3.5 3.5L13 5"/></svg>,
  filter: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><path d="M2 4h12M4 8h8M6 12h4"/></svg>,
  add: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><path d="M8 3v10M3 8h10"/></svg>,
  unread: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><path d="M2 5h12M2 8h12M2 11h12"/></svg>,
  saved: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" strokeLinecap="round"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>,
  savedFilled: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="currentColor" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" strokeLinecap="round"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>,
  all: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><circle cx="3" cy="4" r="0.5"/><circle cx="3" cy="8" r="0.5"/><circle cx="3" cy="12" r="0.5"/><path d="M6 4h8M6 8h8M6 12h8"/></svg>,
  // More recognisable cog (gear with rounded teeth & open center)
  settings: (s=14) => <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round"><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/><circle cx="12" cy="12" r="3"/></svg>,
  more: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="currentColor"><circle cx="3.5" cy="8" r="1.2"/><circle cx="8" cy="8" r="1.2"/><circle cx="12.5" cy="8" r="1.2"/></svg>,
  history: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><path d="M2.5 8a5.5 5.5 0 1 0 1.6-3.9"/><path d="M2.5 2.5V5H5"/><path d="M8 5v3l2 1.5"/></svg>,
  keyboard: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round"><rect x="1.5" y="3.5" width="13" height="9" rx="1.5"/><path d="M4 6.5h.01M7 6.5h.01M10 6.5h.01M13 6.5h.01M4 9h.01M13 9h.01M5.5 11.5h5"/></svg>,
  warning: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><path d="M8 2.5 14.5 13.5h-13z"/><path d="M8 6.5v3.2"/><circle cx="8" cy="11.6" r="0.5" fill="currentColor"/></svg>,
  logout: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><path d="M9.5 3.5H4.5a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h5"/><path d="m11 5.5 2.5 2.5L11 10.5"/><path d="M7 8h6.5"/></svg>,
  user: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><circle cx="8" cy="6" r="2.5"/><path d="M3.5 13.5a4.5 4.5 0 0 1 9 0"/></svg>,
};

// ────── Default account (placeholder identity used by the chip) ──────
// Real apps would hydrate this from the session. Kept here so every
// surface that renders <Sidebar /> shows a consistent user without
// each caller having to pass props.
const DEFAULT_USER = {
  name: "Jodie Park",
  email: "jodie@thecrisp.io",
  initial: "J",
};

// ────── Feed avatar ──────
// Renders the feed-supplied icon (small SVG markup string) when present;
// otherwise falls back to a flat colour square. Sized to caller via CSS.
function FeedAvatar({ feed, size = 12, radius = 2, className = "" }) {
  const base = {
    width: size,
    height: size,
    flexShrink: 0,
    display: "inline-block",
  };
  if (feed?.icon) {
    return (
      <span
        className={`feed-avatar has-icon ${className}`}
        style={{ ...base, borderRadius: radius, overflow: "hidden", lineHeight: 0 }}
        aria-hidden="true"
        dangerouslySetInnerHTML={{ __html: feed.icon }}
      />
    );
  }
  return (
    <span
      className={`feed-avatar no-icon ${className}`}
      style={{ ...base, borderRadius: radius, background: feed?.color || "#999" }}
      aria-hidden="true"
    />
  );
}

// ────── Entry row ──────
function EntryRow({ entry, feed, isSelected, onClick, showSummary = true }) {
  const cls = [
    "entry",
    entry.read ? "is-read" : "",
    entry.saved ? "is-saved" : "",
    isSelected ? "is-selected" : "",
  ].filter(Boolean).join(" ");
  return (
    <div className={cls} onClick={onClick}>
      <span className="junction" aria-hidden="true"></span>
      {entry.saved && (
        <span className="saved-mark" aria-label="Saved" title="Saved">
          {Icon.savedFilled(13)}
        </span>
      )}
      <h3 className="title">{entry.title}</h3>
      <div className="meta">
        <FeedAvatar feed={feed} size={9} radius={2} className="ico-feed" />
        <span className="source">{feed.name}</span>
        <span className="sep" aria-hidden="true"></span>
        <span className="ago">{entry.ago} ago</span>
        <span className="sep" aria-hidden="true"></span>
        <span className="rt">{entry.rt} min read</span>
      </div>
      {showSummary && entry.summary && <p className="summary">{entry.summary}</p>}
    </div>
  );
}

// ────── Unread list (shared) ──────
function UnreadList({ entries, feeds, selectedId, onSelect, density = "default", showSummary = true }) {
  const cls = `tap-unread density-${density}`;
  return (
    <div className={cls}>
      {entries.map((e) => (
        <EntryRow
          key={e.id}
          entry={e}
          feed={feeds.find((f) => f.id === e.feed)}
          isSelected={selectedId === e.id}
          onClick={() => onSelect(e.id)}
          showSummary={showSummary}
        />
      ))}
    </div>
  );
}

// ────── Sidebar (desktop) ──────
function AccountChip({ user = DEFAULT_USER, isOpen, onClick }) {
  return (
    <button
      type="button"
      className={`account-chip ${isOpen ? "is-open" : ""}`}
      aria-haspopup="menu"
      aria-expanded={isOpen ? "true" : "false"}
      onClick={onClick}
    >
      <span className="avatar" aria-hidden="true">{user.initial || (user.name || user.email || "?")[0].toUpperCase()}</span>
      <span className="who">
        <span className="name">{user.name || "Account"}</span>
        <span className="email">{user.email}</span>
      </span>
      <span className="caret" aria-hidden="true">▾</span>
    </button>
  );
}

function AccountMenu({ open, user = DEFAULT_USER, onClose, onPick }) {
  if (!open) return null;
  const pick = (id) => { onPick?.(id); onClose?.(); };
  return (
    <div className="tap-popover-scrim" onClick={onClose}>
      <div className="tap-popover account-menu" role="menu" onClick={(e) => e.stopPropagation()}>
        <div className="account-menu-head">
          <div className="account-menu-name">{user.name || "Account"}</div>
          <div className="account-menu-email">{user.email}</div>
        </div>
        <button className="more-item" role="menuitem" onClick={() => pick('account')}>
          <span className="more-ico" aria-hidden="true">{Icon.user(13)}</span>
          <span>Account settings</span>
        </button>
        <button className="more-item" role="menuitem" onClick={() => pick('theme')}>
          <span className="more-ico" aria-hidden="true"></span>
          <span>Switch theme</span>
        </button>
        <div className="account-menu-sep" aria-hidden="true"></div>
        <button className="more-item logout" role="menuitem" onClick={() => pick('logout')}>
          <span className="more-ico" aria-hidden="true">{Icon.logout(13)}</span>
          <span>Log out</span>
        </button>
      </div>
    </div>
  );
}

function Sidebar({ active = "unread", feeds, unreadCounts, onNav, onAddFeed, onOpenShortcuts, onOpenMore, user, onLogout, onAccount }) {
  const [accountOpen, setAccountOpen] = useState(false);
  const handlePick = (id) => {
    if (id === 'logout') onLogout?.();
    else if (id === 'account') (onAccount || ((u) => onNav?.('settings')))();
    else if (id === 'theme') onNav?.('settings');
  };
  return (
    <aside className="tap-sidebar">
      <div className="brand">
        <Wordmark />
      </div>
      <div className="group-title">Reading</div>
      <div className={`nav-item ${active === 'unread' ? 'active' : ''}`} onClick={() => onNav?.('unread')}>
        <span style={{ width: 14, color: 'currentColor' }}>{Icon.unread()}</span>
        <span>Unread</span>
        <span className="badge">{Object.values(unreadCounts).reduce((a, b) => a + b, 0)}</span>
      </div>
      <div className={`nav-item ${active === 'history' ? 'active' : ''}`} onClick={() => onNav?.('history')}>
        <span style={{ width: 14 }}>{Icon.history()}</span>
        <span>History</span>
      </div>
      <div className={`nav-item ${active === 'saved' ? 'active' : ''}`} onClick={() => onNav?.('saved')}>
        <span style={{ width: 14 }}>{Icon.saved()}</span>
        <span>Saved</span>
        <span className="badge">3</span>
      </div>

      <div className="group-title group-title-row">
        <span>Feeds</span>
        <button
          className="group-action"
          title="Add feed"
          aria-label="Add feed"
          onClick={onAddFeed}
        >
          {Icon.add(12)}
        </button>
      </div>
      {feeds.map((f) => (
        <div key={f.id} className={`feed-row ${f.error ? 'has-error' : ''} ${f.icon ? 'has-icon' : ''}`}>
          <FeedAvatar feed={f} size={14} radius={3} className="ico" />
          <span className="name">{f.name}</span>
          {f.error && (
            <span className="feed-warn" title={f.error} aria-label={`Error: ${f.error}`}>
              {Icon.warning(12)}
            </span>
          )}
          {unreadCounts[f.id] ? <span className="ct">{unreadCounts[f.id]}</span> : null}
        </div>
      ))}

      <div className="sidebar-spacer"></div>
      <AccountChip user={user} isOpen={accountOpen} onClick={() => setAccountOpen(true)} />
      <div className="sidebar-footer">
        <button
          className="footer-btn"
          title="Keyboard shortcuts (?)"
          aria-label="Keyboard shortcuts"
          onClick={onOpenShortcuts}
        >
          {Icon.keyboard(16)}
        </button>
        <button
          className="footer-btn"
          title="Settings"
          aria-label="Settings"
          onClick={() => onNav?.('settings')}
        >
          {Icon.settings(16)}
        </button>
        <button
          className="footer-btn"
          title="More"
          aria-label="More"
          onClick={onOpenMore}
        >
          {Icon.more(16)}
        </button>
      </div>
      <AccountMenu
        open={accountOpen}
        user={user}
        onClose={() => setAccountOpen(false)}
        onPick={handlePick}
      />
    </aside>
  );
}

// ────── Keyboard shortcuts modal ──────
function ShortcutsModal({ open, onClose }) {
  if (!open) return null;
  const groups = [
    {
      title: 'Navigation',
      rows: [
        { keys: ['j'], desc: 'Next entry' },
        { keys: ['k'], desc: 'Previous entry' },
        { keys: ['o', '↵'], desc: 'Open entry' },
        { keys: ['Esc'], desc: 'Back to list' },
      ],
    },
    {
      title: 'Actions',
      rows: [
        { keys: ['m'], desc: 'Mark read / unread' },
        { keys: ['s'], desc: 'Save / unsave' },
        { keys: ['v'], desc: 'View original' },
        { keys: ['r'], desc: 'Refresh' },
        { keys: ['Shift', 'M'], desc: 'Mark all read' },
      ],
    },
    {
      title: 'App',
      rows: [
        { keys: ['/'], desc: 'Search' },
        { keys: ['g', 'u'], desc: 'Go to unread' },
        { keys: ['g', 's'], desc: 'Go to saved' },
        { keys: ['?'], desc: 'This help' },
      ],
    },
  ];
  return (
    <div className="tap-modal-scrim" onClick={onClose}>
      <div className="tap-modal" onClick={(e) => e.stopPropagation()}>
        <div className="tap-modal-head">
          <span className="mono" style={{ fontSize: 10, letterSpacing: '0.12em', color: 'var(--ink-3)', textTransform: 'uppercase' }}>Keyboard shortcuts</span>
          <button className="tap-modal-close" onClick={onClose} aria-label="Close">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><path d="M3.5 3.5l9 9M12.5 3.5l-9 9"/></svg>
          </button>
        </div>
        <div className="tap-modal-body">
          {groups.map((g) => (
            <div key={g.title} className="shortcut-group">
              <div className="shortcut-group-title">{g.title}</div>
              {g.rows.map((r, i) => (
                <div key={i} className="shortcut-row">
                  <span className="shortcut-keys">
                    {r.keys.map((k, ki) => (
                      <React.Fragment key={ki}>
                        {ki > 0 && <span className="shortcut-plus">then</span>}
                        <span className="kbd">{k}</span>
                      </React.Fragment>
                    ))}
                  </span>
                  <span className="shortcut-desc">{r.desc}</span>
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ────── More menu (popover from sidebar footer) ──────
function MoreMenu({ open, onClose, onPick }) {
  if (!open) return null;
  const items = [
    { id: 'history',   icon: Icon.history(13), label: 'History' },
    { id: 'import',    icon: Icon.add(13),     label: 'Import OPML' },
    { id: 'export',    icon: Icon.add(13),     label: 'Export OPML' },
    { id: 'shortcuts', icon: Icon.keyboard(13),label: 'Keyboard shortcuts' },
    { id: 'theme',     icon: null,             label: 'Theme' },
    { id: 'reset',     icon: null,             label: 'Reset poller' },
    { id: 'about',     icon: null,             label: 'About Tap' },
  ];
  return (
    <div className="tap-popover-scrim" onClick={onClose}>
      <div className="tap-popover more-menu" onClick={(e) => e.stopPropagation()}>
        {items.map((it) => (
          <button key={it.id} className="more-item" onClick={() => { onPick?.(it.id); onClose(); }}>
            <span className="more-ico" aria-hidden="true">{it.icon}</span>
            <span>{it.label}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

// ────── Desktop top bar ──────
function DesktopTopBar({ onMarkAllRead, onRefresh, title = 'Unread' }) {
  return (
    <div className="tap-topbar">
      <div className="crumb"><b>{title}</b></div>
      <div className="spacer"></div>
      <button className="icon-btn" title="Search (/)">{Icon.search()}</button>
      <button className="icon-btn" title="Refresh (r)" onClick={onRefresh}>{Icon.refresh()}</button>
      <button className="icon-btn" title="Mark all read (Shift+M)" onClick={onMarkAllRead}>{Icon.check()}</button>
      <button className="icon-btn" title="Filter">{Icon.filter()}</button>
    </div>
  );
}

// ────── Mobile top bar ──────
function MobileTopBar({ unread }) {
  return (
    <div className="m-topbar">
      <div className="title">
        <Wordmark />
        <span style={{ fontWeight: 400, color: 'var(--ink-3)', fontSize: 13, marginLeft: 4 }}>· unread</span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
        <span className="count">{unread}</span>
        <button className="iconbtn">{Icon.search(16)}</button>
      </div>
    </div>
  );
}

function MobileTabBar({ active = "unread" }) {
  const tabs = [
    { id: 'unread',   label: 'Unread',   icon: Icon.unread(22) },
    { id: 'saved',    label: 'Saved',    icon: Icon.saved(22) },
    { id: 'search',   label: 'Search',   icon: Icon.search(22) },
    { id: 'more',     label: 'More',     icon: Icon.more(22) },
  ];
  return (
    <div className="m-tabbar">
      {tabs.map((t) => (
        <div key={t.id} className={`tab ${active === t.id ? 'active' : ''}`}>
          <span className="ico" aria-hidden="true">{t.icon}</span>
          <span>{t.label}</span>
        </div>
      ))}
    </div>
  );
}

// ────── Desktop frame ──────
function DesktopUnread({ entries, feeds, theme = "light", density = "default", showSummary = true, fontMode = "serif" }) {
  const [selected, setSelected] = useState(101);
  const [items, setItems] = useState(entries);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  const unreadCounts = {};
  feeds.forEach((f) => { unreadCounts[f.id] = items.filter((e) => e.feed === f.id && !e.read).length; });

  const visible = items;
  const unread = items.filter((e) => !e.read).length;

  // Keyboard nav (j/k/m/s)
  useEffect(() => {
    const onKey = (ev) => {
      if (ev.target.tagName === 'INPUT' || ev.target.tagName === 'TEXTAREA') return;
      const idx = visible.findIndex((e) => e.id === selected);
      if (ev.key === 'j' || ev.key === 'ArrowDown') {
        ev.preventDefault();
        const next = visible[Math.min(idx + 1, visible.length - 1)];
        if (next) setSelected(next.id);
      } else if (ev.key === 'k' || ev.key === 'ArrowUp') {
        ev.preventDefault();
        const prev = visible[Math.max(idx - 1, 0)];
        if (prev) setSelected(prev.id);
      } else if (ev.key === 'm') {
        ev.preventDefault();
        setItems(items.map((e) => e.id === selected ? { ...e, read: !e.read } : e));
      } else if (ev.key === 's') {
        ev.preventDefault();
        setItems(items.map((e) => e.id === selected ? { ...e, saved: !e.saved } : e));
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [selected, items, visible]);

  const fontStyle = fontMode === 'sans' ? { fontFamily: 'var(--sans)' } : {};

  return (
    <div className={`tap theme-${theme}`} style={{ display: 'flex', height: '100%', ...fontStyle }}>
      <Sidebar
        active="unread"
        feeds={feeds}
        unreadCounts={unreadCounts}
        onOpenShortcuts={() => setShortcutsOpen(true)}
        onOpenMore={() => setMoreOpen(true)}
      />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <DesktopTopBar
          onMarkAllRead={() => setItems(items.map((e) => ({ ...e, read: true })))}
          onRefresh={() => {}}
        />
        <UnreadList entries={visible} feeds={feeds} selectedId={selected} onSelect={setSelected} density={density} showSummary={showSummary} />
      </div>
      <ShortcutsModal open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
      <MoreMenu open={moreOpen} onClose={() => setMoreOpen(false)} />
    </div>
  );
}

// ────── Mobile frame (content only, sized for inside ios-frame) ──────
function MobileUnread({ entries, feeds, theme = "light", fontMode = "serif" }) {
  const [items, setItems] = useState(entries);
  const unread = items.filter((e) => !e.read).length;
  const fontStyle = fontMode === 'sans' ? { fontFamily: 'var(--sans)' } : {};
  return (
    <div className={`tap theme-${theme} is-mobile`} style={{ display: 'flex', flexDirection: 'column', height: '100%', ...fontStyle }}>
      <MobileTopBar unread={unread} />
      <div style={{ flex: 1, overflowY: 'auto' }}>
        <UnreadList entries={items} feeds={feeds} selectedId={null} onSelect={() => {}} density="default" showSummary={true} />
      </div>
      <MobileTabBar active="unread" />
    </div>
  );
}

Object.assign(window, {
  Junction, Wordmark, Icon, FeedAvatar,
  EntryRow, UnreadList, Sidebar,
  ShortcutsModal, MoreMenu,
  AccountChip, AccountMenu, DEFAULT_USER,
  DesktopUnread, MobileUnread,
});
