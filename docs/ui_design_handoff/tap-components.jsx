// River component — the unread reading list.
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
  river: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><path d="M2 5h12M2 8h12M2 11h12"/></svg>,
  saved: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" strokeLinecap="round"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>,
  all: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><circle cx="3" cy="4" r="0.5"/><circle cx="3" cy="8" r="0.5"/><circle cx="3" cy="12" r="0.5"/><path d="M6 4h8M6 8h8M6 12h8"/></svg>,
  settings: (s=14) => <svg width={s} height={s} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><circle cx="8" cy="8" r="2"/><path d="M8 1.5v2M8 12.5v2M14.5 8h-2M3.5 8h-2M12.6 3.4l-1.4 1.4M4.8 11.2l-1.4 1.4M12.6 12.6l-1.4-1.4M4.8 4.8 3.4 3.4"/></svg>,
};

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
      {entry.saved && <span className="saved-mark">SAVED</span>}
      <h3 className="title">{entry.title}</h3>
      <div className="meta">
        <span className="ico-feed" style={{
          width: 9, height: 9, borderRadius: 2, background: feed.color,
          display: "inline-block",
        }} aria-hidden="true"></span>
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

// ────── River list (shared) ──────
function RiverList({ entries, feeds, selectedId, onSelect, density = "default", showSummary = true }) {
  const cls = `tap-river density-${density}`;
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
function Sidebar({ active = "river", feeds, unreadCounts, onNav }) {
  return (
    <aside className="tap-sidebar">
      <div className="brand">
        <Wordmark />
      </div>
      <div className="group-title">Reading</div>
      <div className={`nav-item ${active === 'river' ? 'active' : ''}`} onClick={() => onNav?.('river')}>
        <span style={{ width: 14, color: 'currentColor' }}>{Icon.river()}</span>
        <span>Unread</span>
        <span className="badge">{Object.values(unreadCounts).reduce((a, b) => a + b, 0)}</span>
      </div>
      <div className={`nav-item ${active === 'all' ? 'active' : ''}`} onClick={() => onNav?.('all')}>
        <span style={{ width: 14 }}>{Icon.all()}</span>
        <span>All entries</span>
      </div>
      <div className={`nav-item ${active === 'saved' ? 'active' : ''}`} onClick={() => onNav?.('saved')}>
        <span style={{ width: 14 }}>{Icon.saved()}</span>
        <span>Saved</span>
        <span className="badge">3</span>
      </div>

      <div className="group-title">Feeds</div>
      {feeds.map((f) => (
        <div key={f.id} className="feed-row">
          <span className="ico" style={{ background: f.color }}></span>
          <span className="name">{f.name}</span>
          {unreadCounts[f.id] ? <span className="ct">{unreadCounts[f.id]}</span> : null}
        </div>
      ))}

      <div className="group-title">System</div>
      <div className="nav-item">
        <span style={{ width: 14 }}>{Icon.add()}</span>
        <span>Add feed</span>
      </div>
      <div className="nav-item">
        <span style={{ width: 14 }}>{Icon.settings()}</span>
        <span>Settings</span>
      </div>
    </aside>
  );
}

// ────── Desktop top bar ──────
function DesktopTopBar({ unread, total, onMarkAllRead, onRefresh }) {
  return (
    <div className="tap-topbar">
      <div className="crumb"><b>Unread</b></div>
      <span className="count">{unread} of {total}</span>
      <div className="spacer"></div>
      <button className="icon-btn" title="Search (/)">{Icon.search()}</button>
      <button className="icon-btn" title="Refresh (r)" onClick={onRefresh}>{Icon.refresh()}</button>
      <button className="icon-btn" title="Mark all read (Shift+M)" onClick={onMarkAllRead}>{Icon.check()}</button>
      <button className="icon-btn" title="Filter">{Icon.filter()}</button>
    </div>
  );
}

// ────── Poll strip (desktop) ──────
function PollStrip({ next = "47s" }) {
  return (
    <div className="poll-strip">
      <span className="pulse" aria-hidden="true"></span>
      <span>POLLER · 4 workers · 12 feeds tracked</span>
      <span style={{ flex: 1 }}></span>
      <span>NEXT TICK · {next}</span>
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
    { id: 'unread',   label: 'Unread',   icon: Icon.river(22) },
    { id: 'saved',    label: 'Saved',    icon: Icon.saved(22) },
    { id: 'search',   label: 'Search',   icon: Icon.search(22) },
    { id: 'settings', label: 'Settings', icon: Icon.settings(22) },
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
function DesktopRiver({ entries, feeds, theme = "light", density = "default", showSummary = true, fontMode = "serif" }) {
  const [selected, setSelected] = useState(101);
  const [items, setItems] = useState(entries);
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
      <Sidebar active="river" feeds={feeds} unreadCounts={unreadCounts} />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <DesktopTopBar
          unread={unread}
          total={items.length}
          onMarkAllRead={() => setItems(items.map((e) => ({ ...e, read: true })))}
          onRefresh={() => {}}
        />
        <PollStrip />
        <RiverList entries={visible} feeds={feeds} selectedId={selected} onSelect={setSelected} density={density} showSummary={showSummary} />
        <DesktopHints />
      </div>    </div>
  );
}

function DesktopHints() {
  return (
    <div style={{
      borderTop: '1px solid var(--rule)',
      padding: '8px 24px',
      fontFamily: 'var(--sans)',
      fontSize: 11,
      color: 'var(--ink-3)',
      display: 'flex', gap: 14, alignItems: 'center',
      background: 'var(--bg)',
    }}>
      <span><span className="kbd">j</span> <span className="kbd">k</span> navigate</span>
      <span><span className="kbd">m</span> mark</span>
      <span><span className="kbd">s</span> save</span>
      <span><span className="kbd">o</span> open</span>
      <span style={{ flex: 1 }}></span>
      <span className="mono">tap v0.1 · self-hosted</span>
    </div>
  );
}

// ────── Mobile frame (content only, sized for inside ios-frame) ──────
function MobileRiver({ entries, feeds, theme = "light", fontMode = "serif" }) {
  const [items, setItems] = useState(entries);
  const unread = items.filter((e) => !e.read).length;
  const fontStyle = fontMode === 'sans' ? { fontFamily: 'var(--sans)' } : {};
  return (
    <div className={`tap theme-${theme} is-mobile`} style={{ display: 'flex', flexDirection: 'column', height: '100%', ...fontStyle }}>
      <MobileTopBar unread={unread} />
      <div style={{ flex: 1, overflowY: 'auto' }}>
        <RiverList entries={items} feeds={feeds} selectedId={null} onSelect={() => {}} density="default" showSummary={true} />
      </div>
      <MobileTabBar active="river" />
    </div>
  );
}

Object.assign(window, {
  Junction, Wordmark, Icon,
  EntryRow, RiverList, Sidebar,
  DesktopRiver, MobileRiver,
});
