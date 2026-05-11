// Tap — Categories page.
// Lives inside the .ts-shell (centered, top-tabs).
// Functionality: list, create, rename, delete (with confirm), mark-all-read
// (with confirm), reassign feeds via inline popover.

const { useState: useStateC, useMemo: useMemoC, useEffect: useEffectC, useRef: useRefC } = React;

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

function tcGroupFeeds(feeds, cats) {
  const out = { __uncat: [] };
  cats.forEach((c) => { out[c.id] = []; });
  feeds.forEach((f) => {
    const k = f.category && out[f.category] ? f.category : "__uncat";
    out[k].push(f);
  });
  return out;
}

function tcUnreadFor(entries, feedIds) {
  const set = new Set(feedIds);
  let n = 0;
  for (const e of entries) {
    if (!e.read && set.has(e.feed)) n++;
  }
  return n;
}

// ─────────────────────────────────────────────
// Icons (small, local — match the rest of the system)
// ─────────────────────────────────────────────

function TCPlus({ size = 11 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" aria-hidden="true">
      <path d="M8 3v10M3 8h10" />
    </svg>
  );
}
function TCCheck({ size = 11 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="m3 8 3.4 3.4L13 5" />
    </svg>
  );
}
function TCPencil({ size = 11 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="m3 13 2.6-.6 7-7-2-2-7 7-.6 2.6Z" />
      <path d="m10.6 3.4 2 2" />
    </svg>
  );
}
function TCTrash({ size = 11 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 4h10" />
      <path d="M6 4V2.5h4V4" />
      <path d="M5 4l.6 9h4.8L11 4" />
    </svg>
  );
}
function TCCaret({ size = 8 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 8 8" aria-hidden="true">
      <path d="M1 2.8l3 3 3-3" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function TCArrow({ size = 10 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M2 5h6m-2.5-2.5L8 5l-2.5 2.5" />
    </svg>
  );
}
function TCWarn({ size = 10 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M5 1.5 9 8.5H1L5 1.5Z" />
      <path d="M5 4v2" />
      <circle cx="5" cy="7.4" r="0.4" fill="currentColor" stroke="none" />
    </svg>
  );
}

// ─────────────────────────────────────────────
// Desktop categories page
// ─────────────────────────────────────────────

function TSCategoriesPage({
  categories: initialCats,
  feeds: initialFeeds,
  entries,
  theme = "light",
  fontMode = "serif",
  force = {},
}) {
  // Hydrate state from props + force overrides (lets each artboard show a different state).
  const [cats, setCats] = useStateC(force.empty ? [] : initialCats);
  const [assignments, setAssignments] = useStateC(() => {
    const m = {};
    initialFeeds.forEach((f) => { m[f.id] = f.category || null; });
    if (force.assignments) Object.assign(m, force.assignments);
    return m;
  });
  const [creating, setCreating] = useStateC(!!force.creating);
  const [createValue, setCreateValue] = useStateC(force.createValue || "");
  const [renamingId, setRenamingId] = useStateC(force.renamingId || null);
  const [renameValue, setRenameValue] = useStateC(force.renameValue || "");
  const [dialog, setDialog] = useStateC(force.dialog || null);
  const [pickerFeedId, setPickerFeedId] = useStateC(force.pickerFeedId || null);

  // Build live feed list with current category from assignments.
  const feeds = useMemoC(
    () => initialFeeds.map((f) => ({ ...f, category: assignments[f.id] })),
    [initialFeeds, assignments],
  );
  const grouped = useMemoC(() => tcGroupFeeds(feeds, cats), [feeds, cats]);
  const totalFeeds = feeds.length;
  const uncatCount = grouped.__uncat.length;

  // Pre-compute unread counts per category — used both in the row and as
  // disabled-state for "Mark N read".
  const catUnread = useMemoC(() => {
    const m = {};
    cats.forEach((c) => {
      const list = grouped[c.id] || [];
      m[c.id] = tcUnreadFor(entries, list.map((f) => f.id));
    });
    m.__uncat = tcUnreadFor(entries, grouped.__uncat.map((f) => f.id));
    return m;
  }, [cats, grouped, entries]);

  // Handlers
  const beginRename = (cat) => {
    setRenamingId(cat.id);
    setRenameValue(cat.name);
  };
  const commitRename = () => {
    const v = renameValue.trim();
    if (!v) { setRenamingId(null); return; }
    setCats(cats.map((c) => (c.id === renamingId ? { ...c, name: v } : c)));
    setRenamingId(null);
  };
  const cancelRename = () => setRenamingId(null);
  const commitCreate = () => {
    const v = createValue.trim();
    if (!v) { setCreating(false); setCreateValue(""); return; }
    const id =
      v.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "") ||
      `cat-${Date.now()}`;
    setCats([...cats, { id, name: v, slug: id }]);
    setCreateValue("");
    setCreating(false);
  };
  const cancelCreate = () => { setCreating(false); setCreateValue(""); };
  const reassignFeed = (feedId, catId) => {
    setAssignments({ ...assignments, [feedId]: catId === "__uncat" ? null : catId });
    setPickerFeedId(null);
  };
  const confirmDelete = (catId) => {
    const next = { ...assignments };
    Object.keys(next).forEach((fid) => {
      if (next[fid] === catId) next[fid] = null;
    });
    setAssignments(next);
    setCats(cats.filter((c) => c.id !== catId));
    setDialog(null);
  };
  const confirmMarkRead = () => {
    // The unread list is read-only in this demo; the confirmation flow is
    // what we're designing. Dismiss the dialog on confirm.
    setDialog(null);
  };

  // ─── render ───
  return (
    <div
      className={`tap theme-${theme} ts-root`}
      style={fontMode === "sans" ? { fontFamily: "var(--sans)" } : {}}
    >
      <div className="ts-shell" style={{ position: "relative", minHeight: "100%" }}>
        <TopTabs
          active="categories"
          unreadCount={entries.filter((e) => !e.read).length}
          onNav={() => {}}
        />
        <main className="ts-main">
          <div className="ts-cats">
            <header className="ts-set-head">
              <div className="ts-set-eyebrow">Organisation · {cats.length} {cats.length === 1 ? "category" : "categories"}</div>
              <h1 className="ts-set-title">Categories</h1>
            </header>

            <div className="ts-cats-toolbar">
              <span className="ts-cats-toolbar-l">
                <b>{cats.length}</b><span>{cats.length === 1 ? "category" : "categories"}</span>
                <span className="dot" aria-hidden="true"></span>
                <b>{totalFeeds}</b><span>{totalFeeds === 1 ? "feed" : "feeds"}</span>
                {uncatCount > 0 && (
                  <>
                    <span className="dot" aria-hidden="true"></span>
                    <b>{uncatCount}</b><span>uncategorised</span>
                  </>
                )}
              </span>
              {!creating && cats.length > 0 && (
                <button className="ts-cats-new" onClick={() => setCreating(true)}>
                  <TCPlus />
                  <span>New category</span>
                </button>
              )}
            </div>

            {creating && (
              <div className="ts-cats-newrow">
                <input
                  autoFocus
                  className="ts-cats-newinput"
                  placeholder="Name the category…"
                  value={createValue}
                  onChange={(e) => setCreateValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") commitCreate();
                    if (e.key === "Escape") cancelCreate();
                  }}
                />
                <div className="ts-cats-newrow-hint">
                  <span className="k">↵</span>create<span style={{ margin: "0 8px", opacity: 0.4 }}>·</span><span className="k">esc</span>cancel
                </div>
              </div>
            )}

            {cats.length === 0 && !creating ? (
              <div className="ts-cats-empty">
                <div className="ts-cats-empty-mark" aria-hidden="true">
                  <span className="ts-cats-empty-dot"></span>
                </div>
                <div className="ts-cats-empty-title">No categories yet.</div>
                <p className="ts-cats-empty-sub">
                  Categories group your {totalFeeds} {totalFeeds === 1 ? "feed" : "feeds"} into named sections.
                  Add one to start sorting; feeds you don't assign keep working as normal.
                </p>
                <div className="ts-cats-empty-examples">
                  <span className="ts-cats-empty-example">People</span>
                  <span className="ts-cats-empty-example">Systems &amp; PL</span>
                  <span className="ts-cats-empty-example">Newsletters</span>
                  <span className="ts-cats-empty-example">Aggregators</span>
                </div>
                <button className="ts-cats-empty-cta" onClick={() => setCreating(true)}>
                  <TCPlus />
                  <span>Create your first category</span>
                </button>
              </div>
            ) : (
              <div className="ts-cats-list">
                {cats.map((c) => {
                  const list = grouped[c.id] || [];
                  const unread = catUnread[c.id] || 0;
                  const isRenaming = renamingId === c.id;
                  return (
                    <section key={c.id} className={`ts-cat ${isRenaming ? "is-renaming" : ""}`}>
                      <header className="ts-cat-head">
                        {isRenaming ? (
                          <div className="ts-cat-rename-row">
                            <input
                              autoFocus
                              className="ts-cat-title-input"
                              value={renameValue}
                              onChange={(e) => setRenameValue(e.target.value)}
                              onKeyDown={(e) => {
                                if (e.key === "Enter") commitRename();
                                if (e.key === "Escape") cancelRename();
                              }}
                            />
                            <span className="ts-cat-rename-hint">
                              ↵ save · esc cancel
                            </span>
                          </div>
                        ) : (
                          <h2 className="ts-cat-title" onClick={() => beginRename(c)}>
                            <span>{c.name}</span>
                            <span className="ts-cat-title-edit">click to rename</span>
                          </h2>
                        )}
                        <div className={`ts-cat-stats ${unread === 0 ? "is-zero" : ""}`}>
                          <span className="ll">{unread}</span><span>unread</span>
                          <span className="dot" aria-hidden="true"></span>
                          <b>{list.length}</b><span>{list.length === 1 ? "feed" : "feeds"}</span>
                        </div>
                      </header>

                      {list.length === 0 ? (
                        <div className="ts-cat-empty">
                          No feeds here yet. Move one in from another category, or from <i>Uncategorised</i> below.
                        </div>
                      ) : (
                        <ul className="ts-cat-feeds">
                          {list.map((f) => (
                            <TCFeedRow
                              key={f.id}
                              feed={f}
                              entries={entries}
                              cats={cats}
                              currentCatId={c.id}
                              isPickerOpen={pickerFeedId === f.id}
                              onTogglePicker={() => setPickerFeedId(pickerFeedId === f.id ? null : f.id)}
                              onReassign={(targetCatId) => reassignFeed(f.id, targetCatId)}
                              groupedCounts={grouped}
                            />
                          ))}
                        </ul>
                      )}

                      <footer className="ts-cat-actions">
                        <button
                          className="ts-cat-action"
                          disabled={unread === 0}
                          onClick={() => setDialog({ type: "markread", catId: c.id })}
                        >
                          <TCCheck />
                          {unread === 0 ? "Nothing unread" : `Mark ${unread} read`}
                        </button>
                        <button className="ts-cat-action" onClick={() => beginRename(c)}>
                          <TCPencil />
                          Rename
                        </button>
                        <button
                          className="ts-cat-action is-danger"
                          onClick={() => setDialog({ type: "delete", catId: c.id })}
                        >
                          <TCTrash />
                          Delete
                        </button>
                      </footer>
                    </section>
                  );
                })}

                {/* Uncategorised — read-only category, no rename/delete. */}
                {grouped.__uncat.length > 0 && (
                  <section className="ts-cat is-uncat">
                    <header className="ts-cat-head">
                      <h2 className="ts-cat-title">
                        <span>Uncategorised</span>
                      </h2>
                      <div className={`ts-cat-stats ${catUnread.__uncat === 0 ? "is-zero" : ""}`}>
                        <span className="ll">{catUnread.__uncat}</span><span>unread</span>
                        <span className="dot" aria-hidden="true"></span>
                        <b>{grouped.__uncat.length}</b><span>{grouped.__uncat.length === 1 ? "feed" : "feeds"}</span>
                      </div>
                    </header>
                    <p className="ts-cat-uncat-note">
                      Feeds without a category — still subscribed, still appear in Unread.
                    </p>
                    <ul className="ts-cat-feeds">
                      {grouped.__uncat.map((f) => (
                        <TCFeedRow
                          key={f.id}
                          feed={f}
                          entries={entries}
                          cats={cats}
                          currentCatId={null}
                          isPickerOpen={pickerFeedId === f.id}
                          onTogglePicker={() => setPickerFeedId(pickerFeedId === f.id ? null : f.id)}
                          onReassign={(targetCatId) => reassignFeed(f.id, targetCatId)}
                          groupedCounts={grouped}
                          assignLabel="Assign"
                        />
                      ))}
                    </ul>
                    <footer className="ts-cat-actions">
                      <button
                        className="ts-cat-action"
                        disabled={catUnread.__uncat === 0}
                        onClick={() => setDialog({ type: "markread", catId: "__uncat" })}
                      >
                        <TCCheck />
                        {catUnread.__uncat === 0 ? "Nothing unread" : `Mark ${catUnread.__uncat} read`}
                      </button>
                    </footer>
                  </section>
                )}
              </div>
            )}
          </div>
        </main>

        {/* Delete confirmation */}
        {dialog?.type === "delete" && (() => {
          const cat = cats.find((c) => c.id === dialog.catId);
          if (!cat) return null;
          const list = grouped[cat.id] || [];
          return (
            <div className="ts-overlay" onClick={() => setDialog(null)}>
              <div className="ts-dialog" onClick={(e) => e.stopPropagation()}>
                <div className="ts-dialog-head">
                  <div className="ts-dialog-title">Delete "{cat.name}"?</div>
                  <button className="ts-dialog-close" onClick={() => setDialog(null)} aria-label="Close">×</button>
                </div>
                <div className="ts-dialog-body">
                  <p className="ts-dialog-p">
                    The category is removed.{" "}
                    {list.length > 0 ? (
                      <>
                        The {list.length} {list.length === 1 ? "feed" : "feeds"} inside stay subscribed —
                        they drop back to <b>Uncategorised</b> and keep appearing in Unread.
                      </>
                    ) : (
                      <>No feeds are assigned to it, so nothing else changes.</>
                    )}
                  </p>
                  {list.length > 0 && (
                    <ul className="ts-dialog-list">
                      {list.slice(0, 5).map((f) => (
                        <li key={f.id} className="ts-dialog-list-item">
                          <FeedAvatar feed={f} />
                          <span className="ts-dialog-list-item-name">{f.name}</span>
                          <span className="ts-dialog-list-item-arrow">→ Uncategorised</span>
                        </li>
                      ))}
                      {list.length > 5 && (
                        <li className="ts-dialog-list-more">
                          + {list.length - 5} more
                        </li>
                      )}
                    </ul>
                  )}
                </div>
                <div className="ts-dialog-foot">
                  <div className="ts-dialog-foot-l">esc to cancel</div>
                  <button className="ts-btn" onClick={() => setDialog(null)}>Cancel</button>
                  <button className="ts-btn is-danger" onClick={() => confirmDelete(cat.id)}>
                    Delete category
                  </button>
                </div>
              </div>
            </div>
          );
        })()}

        {/* Mark-all-read confirmation */}
        {dialog?.type === "markread" && (() => {
          const isUncat = dialog.catId === "__uncat";
          const cat = isUncat ? { id: "__uncat", name: "Uncategorised" } : cats.find((c) => c.id === dialog.catId);
          if (!cat) return null;
          const list = grouped[cat.id] || [];
          const unread = catUnread[cat.id] || 0;
          return (
            <div className="ts-overlay" onClick={() => setDialog(null)}>
              <div className="ts-dialog" onClick={(e) => e.stopPropagation()}>
                <div className="ts-dialog-head">
                  <div className="ts-dialog-title">Mark {unread} {unread === 1 ? "entry" : "entries"} as read?</div>
                  <button className="ts-dialog-close" onClick={() => setDialog(null)} aria-label="Close">×</button>
                </div>
                <div className="ts-dialog-body">
                  <p className="ts-dialog-p">
                    Everything currently unread in <b>{cat.name}</b> will be marked as read.
                    Anything you've saved stays saved — only the unread dot disappears.
                  </p>
                  <div className="ts-dialog-stats">
                    <span><b>{unread}</b> unread</span>
                    <span className="dot" aria-hidden="true"></span>
                    <span><b>{list.length}</b> {list.length === 1 ? "feed" : "feeds"}</span>
                    <span className="dot" aria-hidden="true"></span>
                    <span>across {cat.name}</span>
                  </div>
                </div>
                <div className="ts-dialog-foot">
                  <div className="ts-dialog-foot-l">esc to cancel</div>
                  <button className="ts-btn" onClick={() => setDialog(null)}>Cancel</button>
                  <button className="ts-btn is-primary" onClick={confirmMarkRead}>
                    Mark all read
                  </button>
                </div>
              </div>
            </div>
          );
        })()}
      </div>
    </div>
  );
}

// Single feed row inside a category. Hover → "Move" reveal.
function TCFeedRow({
  feed,
  entries,
  cats,
  currentCatId,
  isPickerOpen,
  onTogglePicker,
  onReassign,
  groupedCounts,
  assignLabel = "Move",
}) {
  const fUnread = entries.filter((e) => e.feed === feed.id && !e.read).length;
  return (
    <li className={`ts-cat-feed ${fUnread === 0 ? "is-zero" : ""}`}>
      <FeedAvatar feed={feed} />
      <span className="ts-cat-feed-name">{feed.name}</span>
      <span className="ts-cat-feed-url">{feed.url}</span>
      <span className="ts-cat-feed-right">
        <span className="ts-cat-feed-unread">
          <b>{fUnread}</b> unread
        </span>
        <button
          className={`ts-cat-feed-pick ${isPickerOpen ? "is-open" : ""}`}
          onClick={onTogglePicker}
          aria-expanded={isPickerOpen}
        >
          <span>{assignLabel}</span>
          <TCCaret />
        </button>
        {isPickerOpen && (
          <div className="ts-cat-pop" onClick={(e) => e.stopPropagation()}>
            <div className="ts-cat-pop-eyebrow">
              {assignLabel === "Assign" ? "Assign" : "Move"} <b>{feed.name}</b> to
            </div>
            <div className="ts-cat-pop-rule"></div>
            {cats.map((target) => {
              const ct = (groupedCounts[target.id] || []).length;
              return (
                <button
                  key={target.id}
                  className={`ts-cat-pop-item ${target.id === currentCatId ? "is-current" : ""}`}
                  onClick={() => onReassign(target.id)}
                >
                  <span className="check"><TCCheck size={10} /></span>
                  <span>{target.name}</span>
                  <span className="ts-cat-pop-item-ct">{ct}</span>
                </button>
              );
            })}
            <div className="ts-cat-pop-rule"></div>
            <button
              className={`ts-cat-pop-item is-uncat ${currentCatId === null ? "is-current" : ""}`}
              onClick={() => onReassign("__uncat")}
            >
              <span className="check"><TCCheck size={10} /></span>
              <span>Uncategorised</span>
              <span className="ts-cat-pop-item-ct">{(groupedCounts.__uncat || []).length}</span>
            </button>
          </div>
        )}
      </span>
    </li>
  );
}

// ─────────────────────────────────────────────
// Mobile categories screen
// ─────────────────────────────────────────────

function TSCategoriesMobile({
  categories: initialCats,
  feeds: initialFeeds,
  entries,
  theme = "light",
  fontMode = "serif",
  force = {},
}) {
  const [cats] = useStateC(initialCats);
  const [assignments] = useStateC(() => {
    const m = {};
    initialFeeds.forEach((f) => { m[f.id] = f.category || null; });
    return m;
  });
  const [sheetFor, setSheetFor] = useStateC(force.sheetFor || null);

  const feeds = initialFeeds.map((f) => ({ ...f, category: assignments[f.id] }));
  const grouped = tcGroupFeeds(feeds, cats);

  const catUnread = {};
  cats.forEach((c) => {
    catUnread[c.id] = tcUnreadFor(entries, (grouped[c.id] || []).map((f) => f.id));
  });
  catUnread.__uncat = tcUnreadFor(entries, grouped.__uncat.map((f) => f.id));

  const renderCat = (cat, list, isUncat = false) => (
    <div key={cat.id} className={`m-cat-card ${isUncat ? "is-uncat" : ""}`}>
      <div className="m-cat-card-head">
        <div className="m-cat-card-name">{cat.name}</div>
        <div className="m-cat-card-stats">
          <span className="ll">{catUnread[cat.id] || 0}</span>
          <span> unread </span>
          <span style={{ color: "var(--ink-4)", margin: "0 4px" }}>·</span>
          <b>{list.length}</b>
          <span> {list.length === 1 ? "feed" : "feeds"}</span>
        </div>
      </div>
      {list.length === 0 ? (
        <div style={{ fontFamily: "var(--mono)", fontSize: 11, color: "var(--ink-3)", padding: "6px 0 14px" }}>
          No feeds yet.
        </div>
      ) : (
        <div className="m-cat-card-feeds">
          {list.map((f) => {
            const fu = entries.filter((e) => e.feed === f.id && !e.read).length;
            return (
              <div key={f.id} className="m-cat-card-feed">
                <FeedAvatar feed={f} />
                <div>
                  <div className="m-cat-card-feed-name">{f.name}</div>
                  <div className="m-cat-card-feed-meta">
                    <b>{fu}</b> unread · {f.url}
                  </div>
                </div>
                <button className="m-cat-card-feed-move" onClick={() => setSheetFor(f.id)}>
                  Move
                </button>
              </div>
            );
          })}
        </div>
      )}
      {!isUncat && (
        <div className="m-cat-card-actions">
          <button className="m-cat-card-action" disabled={(catUnread[cat.id] || 0) === 0}>
            <TCCheck /> Mark read
          </button>
          <button className="m-cat-card-action">
            <TCPencil /> Rename
          </button>
          <button className="m-cat-card-action is-danger">
            <TCTrash /> Delete
          </button>
        </div>
      )}
    </div>
  );

  const sheetFeed = sheetFor ? feeds.find((f) => f.id === sheetFor) : null;

  return (
    <div
      className={`tap theme-${theme} m-cats-root is-mobile`}
      style={fontMode === "sans" ? { fontFamily: "var(--sans)" } : { position: "relative" }}
    >
      <TapMobileShell
        title="Categories"
        count={cats.length}
        countLabel={cats.length === 1 ? "category" : "categories"}
        active="categories"
        unreadCount={entries.filter((e) => !e.read).length}
        onNav={() => {}}
      >
        <div className="m-cats-toolbar">
          <span>
            <b>{cats.length}</b> categories
            <span className="dot" aria-hidden="true"></span>
            <b>{grouped.__uncat.length}</b> uncategorised
          </span>
          <button className="m-cats-new">
            <TCPlus /> New
          </button>
        </div>
        <div className="m-cats-body">
          {cats.map((c) => renderCat(c, grouped[c.id] || []))}
          {grouped.__uncat.length > 0 &&
            renderCat({ id: "__uncat", name: "Uncategorised" }, grouped.__uncat, true)}
        </div>
      </TapMobileShell>

      {sheetFeed && (
        <div className="m-cat-sheet" onClick={() => setSheetFor(null)}>
          <div className="m-cat-sheet-card" onClick={(e) => e.stopPropagation()}>
            <div className="m-cat-sheet-handle"></div>
            <div className="m-cat-sheet-head">
              <div className="m-cat-sheet-eyebrow">Move feed</div>
              <div className="m-cat-sheet-title">
                <FeedAvatar feed={sheetFeed} />
                <span>{sheetFeed.name}</span>
              </div>
            </div>
            <div className="m-cat-sheet-list">
              {cats.map((target) => {
                const ct = (grouped[target.id] || []).length;
                return (
                  <button
                    key={target.id}
                    className={`m-cat-sheet-item ${sheetFeed.category === target.id ? "is-current" : ""}`}
                    onClick={() => setSheetFor(null)}
                  >
                    <span className="m-cat-sheet-item-check">✓</span>
                    <span>{target.name}</span>
                    <span className="m-cat-sheet-item-ct">{ct} feeds</span>
                  </button>
                );
              })}
              <button
                className={`m-cat-sheet-item is-uncat ${sheetFeed.category == null ? "is-current" : ""}`}
                onClick={() => setSheetFor(null)}
              >
                <span className="m-cat-sheet-item-check">✓</span>
                <span>Uncategorised</span>
                <span className="m-cat-sheet-item-ct">{grouped.__uncat.length} feeds</span>
              </button>
            </div>
            <div className="m-cat-sheet-foot">
              <span style={{ fontFamily: "var(--mono)", fontSize: 10.5, color: "var(--ink-3)" }}>
                tap to move
              </span>
              <button className="m-cat-sheet-cancel" onClick={() => setSheetFor(null)}>
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

Object.assign(window, {
  TSCategoriesPage,
  TSCategoriesMobile,
});
