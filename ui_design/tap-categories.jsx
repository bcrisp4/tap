// Categories — sidebar grouping, chip strip, and the dedicated manager view.
// Typographic only. No per-category accent colours; Klein Blue stays reserved.

const { useState: useStateC } = React;

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────
function feedsByCategory(feeds, categories) {
  const map = {};
  categories.forEach((c) => { map[c.id] = []; });
  map.__uncat = [];
  feeds.forEach((f) => {
    if (f.category && map[f.category]) map[f.category].push(f);
    else map.__uncat.push(f);
  });
  return map;
}

function unreadCountFor(entries, feedIds) {
  const set = new Set(feedIds);
  return entries.filter((e) => set.has(e.feed) && !e.read).length;
}

// ─────────────────────────────────────────────
// A · Sidebar grouped by category (collapsible)
// ─────────────────────────────────────────────
function CategorizedSidebar({
  active = "unread",
  feeds,
  entries,
  categories,
  activeCategoryId = null,
  onNav,
  onAddFeed,
  onAddCategory,
  onSelectCategory,
  onManageCategories,
  onOpenShortcuts,
  onOpenMore,
}) {
  const [collapsed, setCollapsed] = useStateC({});
  const grouped = feedsByCategory(feeds, categories);
  const unreadByFeed = {};
  feeds.forEach((f) => { unreadByFeed[f.id] = entries.filter((e) => e.feed === f.id && !e.read).length; });
  const totalUnread = entries.filter((e) => !e.read).length;

  const renderCategoryHeader = (cat, count) => {
    const isCollapsed = !!collapsed[cat.id];
    const isFiltered = activeCategoryId === cat.id;
    return (
      <div
        className={`cat-header ${isFiltered ? "is-filtered" : ""}`}
        onClick={() => setCollapsed({ ...collapsed, [cat.id]: !isCollapsed })}
      >
        <span className={`cat-caret ${isCollapsed ? "is-collapsed" : ""}`} aria-hidden="true">
          <svg width="8" height="8" viewBox="0 0 8 8"><path d="M2 1 L6 4 L2 7 Z" fill="currentColor" /></svg>
        </span>
        <span
          className="cat-name"
          onClick={(e) => { e.stopPropagation(); onSelectCategory?.(cat.id); }}
          title={`Filter to ${cat.name}`}
        >
          {cat.name}
        </span>
        <span className="cat-count">{count}</span>
      </div>
    );
  };

  return (
    <aside className="tap-sidebar tap-sidebar-cats">
      <div className="brand">
        <Wordmark />
      </div>
      <div className="group-title">Reading</div>
      <div className={`nav-item ${active === "unread" ? "active" : ""}`} onClick={() => onNav?.("unread")}>
        <span style={{ width: 14, color: "currentColor" }}>{Icon.unread()}</span>
        <span>Unread</span>
        <span className="badge">{totalUnread}</span>
      </div>
      <div className={`nav-item ${active === "saved" ? "active" : ""}`} onClick={() => onNav?.("saved")}>
        <span style={{ width: 14 }}>{Icon.saved()}</span>
        <span>Saved</span>
        <span className="badge">3</span>
      </div>

      <div className="group-title group-title-row">
        <span>Categories</span>
        <span style={{ display: "flex", gap: 2 }}>
          <button className="group-action" title="New category" aria-label="New category" onClick={onAddCategory}>
            {Icon.add(12)}
          </button>
          <button className="group-action" title="Manage categories" aria-label="Manage categories" onClick={onManageCategories}>
            <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><path d="M2 4h12M2 8h8M2 12h12"/></svg>
          </button>
        </span>
      </div>

      {categories.map((cat) => {
        const list = grouped[cat.id] || [];
        const count = unreadCountFor(entries, list.map((f) => f.id));
        const isCollapsed = !!collapsed[cat.id];
        return (
          <div key={cat.id} className="cat-block">
            {renderCategoryHeader(cat, count)}
            {!isCollapsed && list.map((f) => (
              <div key={f.id} className={`feed-row in-cat ${f.error ? "has-error" : ""} ${f.icon ? "has-icon" : ""}`}>
                <FeedAvatar feed={f} size={14} radius={3} className="ico" />
                <span className="name">{f.name}</span>
                {f.error && (
                  <span className="feed-warn" title={f.error} aria-label={`Error: ${f.error}`}>{Icon.warning(12)}</span>
                )}
                {unreadByFeed[f.id] ? <span className="ct">{unreadByFeed[f.id]}</span> : null}
              </div>
            ))}
          </div>
        );
      })}

      {grouped.__uncat.length > 0 && (
        <div className="cat-block">
          <div className="cat-header cat-header-uncat">
            <span className="cat-caret" aria-hidden="true"></span>
            <span className="cat-name">Uncategorised</span>
            <span className="cat-count">{unreadCountFor(entries, grouped.__uncat.map((f) => f.id))}</span>
          </div>
          {grouped.__uncat.map((f) => (
            <div key={f.id} className={`feed-row in-cat ${f.icon ? "has-icon" : ""}`}>
              <FeedAvatar feed={f} size={14} radius={3} className="ico" />
              <span className="name">{f.name}</span>
              {unreadByFeed[f.id] ? <span className="ct">{unreadByFeed[f.id]}</span> : null}
            </div>
          ))}
        </div>
      )}

      <div className="sidebar-spacer"></div>
      <CatSidebarAccount onNav={onNav} />
      <div className="sidebar-footer">
        <button className="footer-btn" title="Keyboard shortcuts (?)" aria-label="Keyboard shortcuts" onClick={onOpenShortcuts}>
          {Icon.keyboard(16)}
        </button>
        <button className="footer-btn" title="Settings" aria-label="Settings" onClick={() => onNav?.("settings")}>
          {Icon.settings(16)}
        </button>
        <button className="footer-btn" title="More" aria-label="More" onClick={onOpenMore}>
          {Icon.more(16)}
        </button>
      </div>
    </aside>
  );
}

// Account chip + popover for the categorized sidebar.
// Lives here so this file stays self-contained.
function CatSidebarAccount({ onNav }) {
  const [open, setOpen] = useStateC(false);
  const handle = (id) => {
    setOpen(false);
    if (id === 'logout') {
      // Hook your auth here; for now a no-op confirmation.
      // eslint-disable-next-line no-alert
      if (typeof window !== 'undefined') window.dispatchEvent(new CustomEvent('tap:logout'));
    } else if (id === 'account' || id === 'theme') {
      onNav?.('settings');
    }
  };
  return (
    <React.Fragment>
      <AccountChip isOpen={open} onClick={() => setOpen(true)} />
      <AccountMenu open={open} onClose={() => setOpen(false)} onPick={handle} />
    </React.Fragment>
  );
}

// ─────────────────────────────────────────────
// B · Category chip strip (above unread list)
// ─────────────────────────────────────────────
function CategoryChipStrip({ categories, entries, feeds, activeId = "all", onSelect }) {
  const grouped = feedsByCategory(feeds, categories);
  const totalUnread = entries.filter((e) => !e.read).length;
  const chips = [
    { id: "all", label: "All", count: totalUnread },
    ...categories.map((c) => ({
      id: c.id,
      label: c.name,
      count: unreadCountFor(entries, (grouped[c.id] || []).map((f) => f.id)),
    })),
  ];
  if (grouped.__uncat.length > 0) {
    chips.push({
      id: "__uncat",
      label: "Uncategorised",
      count: unreadCountFor(entries, grouped.__uncat.map((f) => f.id)),
    });
  }
  return (
    <div className="cat-strip">
      {chips.map((c) => (
        <button
          key={c.id}
          className={`cat-chip ${activeId === c.id ? "is-active" : ""}`}
          onClick={() => onSelect?.(c.id)}
        >
          <span className="cat-chip-label">{c.label}</span>
          <span className="cat-chip-count">{c.count}</span>
        </button>
      ))}
      <span className="cat-strip-spacer"></span>
      <button className="cat-strip-edit" title="Manage categories">
        Manage <span style={{ marginLeft: 4 }}>→</span>
      </button>
    </div>
  );
}

// ─────────────────────────────────────────────
// C · Category manager — the dedicated CRUD view
// ─────────────────────────────────────────────
function CategoryManager({ categories: initialCats, feeds, entries }) {
  const [cats, setCats] = useStateC(initialCats);
  const [editingId, setEditingId] = useStateC(null);
  const [editValue, setEditValue] = useStateC("");
  const [draftOpen, setDraftOpen] = useStateC(false);
  const [draftValue, setDraftValue] = useStateC("");

  const grouped = feedsByCategory(feeds, cats);

  const beginRename = (cat) => {
    setEditingId(cat.id);
    setEditValue(cat.name);
  };
  const commitRename = () => {
    if (!editingId) return;
    setCats(cats.map((c) => c.id === editingId ? { ...c, name: editValue || c.name } : c));
    setEditingId(null);
  };
  const commitDraft = () => {
    if (!draftValue.trim()) { setDraftOpen(false); return; }
    const id = draftValue.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
    setCats([...cats, { id, name: draftValue.trim(), slug: id }]);
    setDraftValue("");
    setDraftOpen(false);
  };
  const move = (idx, delta) => {
    const next = idx + delta;
    if (next < 0 || next >= cats.length) return;
    const copy = cats.slice();
    [copy[idx], copy[next]] = [copy[next], copy[idx]];
    setCats(copy);
  };

  return (
    <div className="cat-manager cat-manager-narrow">
      {/* Page header */}
      <div className="cat-mgr-head">
        <div>
          <div className="mono cat-mgr-eyebrow">Settings · Organisation</div>
          <h1 className="cat-mgr-title">Categories</h1>
          <p className="cat-mgr-sub">
            Name the groups you'd like to sort feeds into. Categories appear as section
            headers in the sidebar and as filter chips above the unread list.
          </p>
        </div>
        <div className="cat-mgr-stat">
          <span className="cat-mgr-stat-n">{cats.length}</span>
          <span className="cat-mgr-stat-l">categories</span>
          <span className="cat-mgr-stat-dot"></span>
          <span className="cat-mgr-stat-n">{feeds.length}</span>
          <span className="cat-mgr-stat-l">feeds total</span>
          {grouped.__uncat.length > 0 && (
            <>
              <span className="cat-mgr-stat-dot"></span>
              <span className="cat-mgr-stat-n cat-mgr-stat-warn">{grouped.__uncat.length}</span>
              <span className="cat-mgr-stat-l">uncategorised</span>
            </>
          )}
        </div>
      </div>

      {/* Single column — categories list */}
      <div className="cat-mgr-list">
        <div className="cat-mgr-list-head">
          <span className="mono cat-mgr-col-eyebrow">Categories · drag to reorder</span>
          <button className="cat-mgr-new" onClick={() => setDraftOpen(true)}>
            {Icon.add(12)} <span>New category</span>
          </button>
        </div>

        {cats.map((c, idx) => {
          const list = grouped[c.id] || [];
          const unread = unreadCountFor(entries, list.map((f) => f.id));
          const isEditing = editingId === c.id;
          return (
            <div key={c.id} className="cat-mgr-row">
              <span className="cat-mgr-drag" title="Drag to reorder">
                <svg width="10" height="14" viewBox="0 0 10 14" fill="currentColor"><circle cx="3" cy="3" r="1"/><circle cx="7" cy="3" r="1"/><circle cx="3" cy="7" r="1"/><circle cx="7" cy="7" r="1"/><circle cx="3" cy="11" r="1"/><circle cx="7" cy="11" r="1"/></svg>
              </span>
              {isEditing ? (
                <input
                  autoFocus
                  className="cat-mgr-input"
                  value={editValue}
                  onChange={(e) => setEditValue(e.target.value)}
                  onBlur={commitRename}
                  onKeyDown={(e) => { if (e.key === "Enter") commitRename(); if (e.key === "Escape") setEditingId(null); }}
                />
              ) : (
                <span className="cat-mgr-name" onDoubleClick={() => beginRename(c)}>{c.name}</span>
              )}
              <span className="cat-mgr-counts">
                <span className="cat-mgr-count-n">{list.length}</span>
                <span className="cat-mgr-count-l">feeds</span>
                {unread > 0 && (
                  <>
                    <span className="cat-mgr-count-dot"></span>
                    <span className="cat-mgr-count-n">{unread}</span>
                    <span className="cat-mgr-count-l">unread</span>
                  </>
                )}
              </span>
              <span className="cat-mgr-actions">
                <button className="cat-mgr-icon" title="Move up" onClick={() => move(idx, -1)} disabled={idx === 0}>
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="m4 9 4-4 4 4"/></svg>
                </button>
                <button className="cat-mgr-icon" title="Move down" onClick={() => move(idx, 1)} disabled={idx === cats.length - 1}>
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="m4 7 4 4 4-4"/></svg>
                </button>
                <button className="cat-mgr-icon" title="Rename" onClick={() => beginRename(c)}>
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><path d="m3 13 2.5-.5 7-7-2-2-7 7z"/><path d="m11 3 2 2"/></svg>
                </button>
                <button className="cat-mgr-icon" title="Delete" onClick={() => setCats(cats.filter((x) => x.id !== c.id))}>
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><path d="M3 4h10M6 4V2.5h4V4M5 4l.6 9h4.8L11 4"/></svg>
                </button>
              </span>
            </div>
          );
        })}

        {/* Draft row */}
        {draftOpen && (
          <div className="cat-mgr-row is-draft">
            <span className="cat-mgr-drag" style={{ visibility: "hidden" }}></span>
            <input
              autoFocus
              className="cat-mgr-input"
              placeholder="Category name…"
              value={draftValue}
              onChange={(e) => setDraftValue(e.target.value)}
              onBlur={commitDraft}
              onKeyDown={(e) => { if (e.key === "Enter") commitDraft(); if (e.key === "Escape") { setDraftValue(""); setDraftOpen(false); } }}
            />
            <span className="cat-mgr-counts"><span className="cat-mgr-count-l" style={{ opacity: 0.6 }}>↵ to create · esc to cancel</span></span>
            <span className="cat-mgr-actions"></span>
          </div>
        )}

        {!draftOpen && (
          <button className="cat-mgr-newrow" onClick={() => setDraftOpen(true)}>
            <span className="cat-mgr-drag" style={{ visibility: "hidden" }}></span>
            <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
              {Icon.add(12)} <span>New category</span>
            </span>
          </button>
        )}
      </div>

      <div className="cat-mgr-foot mono">
        To assign feeds to a category, open <span className="cat-mgr-foot-link">Settings → Feeds</span> and edit each feed.
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// D · Mobile categories browser
// ─────────────────────────────────────────────
function MobileCategoriesScreen({ categories, feeds, entries, theme = "light", fontMode = "serif" }) {
  const grouped = feedsByCategory(feeds, categories);
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};
  return (
    <div className={`tap theme-${theme} is-mobile`} style={{ display: "flex", flexDirection: "column", height: "100%", ...fontStyle }}>
      <div className="m-topbar">
        <div className="title">
          <Wordmark />
          <span style={{ fontWeight: 400, color: "var(--ink-3)", fontSize: 13, marginLeft: 4 }}>· categories</span>
        </div>
        <button className="iconbtn">{Icon.add(18)}</button>
      </div>

      <div style={{ flex: 1, overflowY: "auto" }}>
        {categories.map((c) => {
          const list = grouped[c.id] || [];
          const unread = unreadCountFor(entries, list.map((f) => f.id));
          return (
            <div key={c.id} className="m-cat-row">
              <div className="m-cat-row-head">
                <span className="m-cat-name">{c.name}</span>
                <span className="m-cat-meta mono">{list.length} feeds · {unread} unread</span>
              </div>
              <div className="m-cat-feeds">
                {list.slice(0, 4).map((f) => (
                  <span key={f.id} className="m-cat-feed">
                    <FeedAvatar feed={f} size={14} radius={3} />
                    <span>{f.name}</span>
                  </span>
                ))}
                {list.length > 4 && <span className="m-cat-more">+{list.length - 4}</span>}
              </div>
            </div>
          );
        })}

        {grouped.__uncat.length > 0 && (
          <div className="m-cat-row m-cat-row-uncat">
            <div className="m-cat-row-head">
              <span className="m-cat-name">Uncategorised</span>
              <span className="m-cat-meta mono">{grouped.__uncat.length} feeds</span>
            </div>
            <div className="m-cat-feeds">
              {grouped.__uncat.map((f) => (
                <span key={f.id} className="m-cat-feed">
                  <FeedAvatar feed={f} size={14} radius={3} />
                  <span>{f.name}</span>
                </span>
              ))}
            </div>
          </div>
        )}
      </div>

      <MobileTabBar active="more" />
    </div>
  );
}

// ─────────────────────────────────────────────
// Unread + chip strip combo (variant B preview)
// ─────────────────────────────────────────────
function DesktopUnreadWithChips({ entries, feeds, categories, theme = "light", density = "default", showSummary = true, fontMode = "serif" }) {
  const [activeCat, setActiveCat] = useStateC("all");
  const [selected, setSelected] = useStateC(101);
  const filtered = activeCat === "all"
    ? entries
    : activeCat === "__uncat"
      ? entries.filter((e) => { const f = feeds.find((x) => x.id === e.feed); return !f?.category; })
      : entries.filter((e) => { const f = feeds.find((x) => x.id === e.feed); return f?.category === activeCat; });
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};

  const unreadCounts = {};
  feeds.forEach((f) => { unreadCounts[f.id] = entries.filter((e) => e.feed === f.id && !e.read).length; });

  return (
    <div className={`tap theme-${theme}`} style={{ display: "flex", height: "100%", ...fontStyle }}>
      <Sidebar
        active="unread"
        feeds={feeds}
        unreadCounts={unreadCounts}
        onOpenShortcuts={() => {}}
        onOpenMore={() => {}}
      />
      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        <DesktopTopBar onMarkAllRead={() => {}} onRefresh={() => {}} />
        <CategoryChipStrip
          categories={categories}
          entries={entries}
          feeds={feeds}
          activeId={activeCat}
          onSelect={setActiveCat}
        />
        <UnreadList
          entries={filtered}
          feeds={feeds}
          selectedId={selected}
          onSelect={setSelected}
          density={density}
          showSummary={showSummary}
        />
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Categorized sidebar combo (variant A preview)
// ─────────────────────────────────────────────
function DesktopUnreadWithCatSidebar({ entries, feeds, categories, theme = "light", density = "default", showSummary = true, fontMode = "serif" }) {
  const [selected, setSelected] = useStateC(101);
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};
  return (
    <div className={`tap theme-${theme}`} style={{ display: "flex", height: "100%", ...fontStyle }}>
      <CategorizedSidebar
        active="unread"
        feeds={feeds}
        entries={entries}
        categories={categories}
      />
      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        <DesktopTopBar onMarkAllRead={() => {}} onRefresh={() => {}} />
        <UnreadList
          entries={entries}
          feeds={feeds}
          selectedId={selected}
          onSelect={setSelected}
          density={density}
          showSummary={showSummary}
        />
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Manager wrapped inside the app chrome
// ─────────────────────────────────────────────
function DesktopCategoryManager({ entries, feeds, categories, theme = "light", fontMode = "serif" }) {
  const fontStyle = fontMode === "sans" ? { fontFamily: "var(--sans)" } : {};
  return (
    <div className={`tap theme-${theme}`} style={{ display: "flex", height: "100%", ...fontStyle }}>
      <CategorizedSidebar
        active="settings"
        feeds={feeds}
        entries={entries}
        categories={categories}
      />
      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0, overflow: "hidden" }}>
        <div className="tap-topbar">
          <div className="crumb">Settings · <b>Categories</b></div>
          <div className="spacer"></div>
        </div>
        <div style={{ flex: 1, overflowY: "auto", background: "var(--bg)" }}>
          <CategoryManager categories={categories} feeds={feeds} entries={entries} />
        </div>
      </div>
    </div>
  );
}

Object.assign(window, {
  CategorizedSidebar,
  CategoryChipStrip,
  CategoryManager,
  MobileCategoriesScreen,
  DesktopUnreadWithChips,
  DesktopUnreadWithCatSidebar,
  DesktopCategoryManager,
});
