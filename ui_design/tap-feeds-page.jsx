// Tap — Feeds (Feed Management) page.
// Lives inside the .ts-shell (centered column, top tabs).
// Functionality covered:
//   • List feeds with title, URL, category, last/next poll, errors, unread
//   • Filter (all / errors / uncategorised / by category), sort, group-by-category
//   • Add by URL with auto-discovery preview
//   • Edit feed (title, category, extraction, custom selector, cookie, basic auth)
//   • Delete (single + bulk) with confirmation
//   • Refresh single feed; refresh all
//   • Per-feed health (last error, consecutive errors, backoff)
//   • Open site/feed URL in new tab
//   • Mark all entries in a feed as read
//   • Bulk select → delete, reassign category, mark read
//   • Import / Export OPML

const { useState: useStateF, useMemo: useMemoF } = React;

// ─────────────────────────────────────────────
// Synthesised per-feed health (deterministic by id).
// In the real app this is server state; here we lay it
// out so each row has plausible polling + error data.
// ─────────────────────────────────────────────

const TF_HEALTH = {
  1:  { last: "12m",  next: "18m",  cad: "30m",  errors: 0 },
  2:  { last: "7m",   next: "23m",  cad: "30m",  errors: 0 },
  3:  { last: "21m",  next: "9m",   cad: "30m",  errors: 0 },
  4:  { last: "1h",   next: "5h",   cad: "6h",   errors: 0 },
  5:  { last: "2m",   next: "13m",  cad: "15m",  errors: 0 },
  6:  { last: "44m",  next: "16m",  cad: "1h",   errors: 0 },
  7:  { last: "2h 8m",  next: "—",  cad: "1h",   errors: 5,
        err: "Last fetch: HTTP 502 Bad Gateway from buttondown.email",
        firstSeen: "2h 8m ago", backoff: "2h 12m" },
  8:  { last: "38m",  next: "22m",  cad: "1h",   errors: 0 },
  9:  { last: "6m",   next: "9m",   cad: "15m",  errors: 0 },
  10: { last: "11m",  next: "19m",  cad: "30m",  errors: 0 },
  11: { last: "1h 4m", next: "—",   cad: "30m",  errors: 1,
        err: "Cloudflare challenge page returned (1020). Falling back to next interval.",
        firstSeen: "1h 4m ago", backoff: "26m" },
  12: { last: "3d 4h", next: "—",   cad: "30m",  errors: 17,
        err: "TLS certificate expired on Tue, 06 May 2026 — chain incomplete.",
        firstSeen: "3 days ago", backoff: "5h 48m" },
};

function tfHealth(id) {
  return TF_HEALTH[id] || { last: "—", next: "—", cad: "—", errors: 0 };
}

function tfUnread(entries, feedId) {
  let n = 0;
  for (const e of entries) if (e.feed === feedId && !e.read) n++;
  return n;
}

// ─────────────────────────────────────────────
// Icons (small, single-stroke, matching system)
// ─────────────────────────────────────────────

const TF_ICO = {
  plus: <path d="M8 3v10M3 8h10" />,
  check: <path d="m3 8 3.4 3.4L13 5" />,
  refresh: <path d="M3 8a5 5 0 0 1 8.7-3.3L13 6M13 8a5 5 0 0 1-8.7 3.3L3 10M13 3v3h-3M3 13v-3h3" />,
  trash: <g><path d="M3 4h10" /><path d="M6 4V2.5h4V4" /><path d="M4.5 4l.7 8.5a1 1 0 0 0 1 .9h3.6a1 1 0 0 0 1-.9L11.5 4" /></g>,
  caret: <path d="M3 6l3 3 3-3" />,
  more: <g><circle cx="3.5" cy="8" r="0.9" fill="currentColor" stroke="none" /><circle cx="8" cy="8" r="0.9" fill="currentColor" stroke="none" /><circle cx="12.5" cy="8" r="0.9" fill="currentColor" stroke="none" /></g>,
  external: <g><path d="M9 3h4v4" /><path d="M13 3 7 9" /><path d="M11 9v3a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h3" /></g>,
  search: <g><circle cx="7" cy="7" r="4" /><path d="M10 10l3 3" /></g>,
  warn: <g><path d="M8 2.5 14 12.5H2L8 2.5Z" /><path d="M8 6.5v3" /><circle cx="8" cy="11.2" r="0.5" fill="currentColor" stroke="none" /></g>,
  download: <g><path d="M8 3v8" /><path d="M5 8l3 3 3-3" /><path d="M3 13h10" /></g>,
  upload: <g><path d="M8 13V5" /><path d="M5 8l3-3 3 3" /><path d="M3 3h10" /></g>,
  x: <path d="M4 4l8 8M12 4l-8 8" />,
};

function TFI({ k, size = 12, w = 1.4 }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={w} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {TF_ICO[k]}
    </svg>
  );
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

function tfCatName(cats, id) {
  if (!id) return null;
  const c = cats.find((x) => x.id === id);
  return c ? c.name : null;
}

function tfMatchSort(a, b, mode) {
  if (mode === "title") return a.name.localeCompare(b.name);
  if (mode === "unread") return (b._unread || 0) - (a._unread || 0);
  if (mode === "errors") return (b._h.errors || 0) - (a._h.errors || 0);
  if (mode === "last") {
    // approximate sort by "last" string: m < h < d
    const score = (s) => {
      if (!s) return 1e9;
      if (s.includes("d")) return 1e6 + parseInt(s) * 24 * 60;
      if (s.includes("h")) return 1000 + parseInt(s) * 60;
      if (s.includes("m")) return parseInt(s);
      return 1e9;
    };
    return score(a._h.last) - score(b._h.last);
  }
  if (mode === "next") {
    const score = (s) => {
      if (!s || s === "—") return 1e9;
      if (s.includes("h")) return 1000 + parseInt(s) * 60;
      if (s.includes("m")) return parseInt(s);
      return 1e9;
    };
    return score(a._h.next) - score(b._h.next);
  }
  return 0;
}

// ─────────────────────────────────────────────
// Feed row
// ─────────────────────────────────────────────

function TFFeedRow({
  feed, isSelected, onToggleSelect, isExpanded, onToggleExpand,
  onRefresh, onEdit, onDelete, onMarkRead, onOpenSite, onOpenFeed, onPickCat,
  busy, anySelected, cats,
}) {
  const h = feed._h;
  const hasErr = h.errors > 0;
  return (
    <div className={`ts-feed-row ${isSelected ? "is-selected" : ""} ${busy ? "is-busy" : ""} ${hasErr ? "has-error" : ""}`}>
      <button
        className={`ts-feed-check ${isSelected ? "is-checked" : ""}`}
        onClick={onToggleSelect}
        aria-label={isSelected ? "Deselect" : "Select"}
        style={!anySelected && !isSelected ? { opacity: 0.55 } : {}}
      >
        {isSelected && <TFI k="check" size={10} w={1.8} />}
      </button>

      <span className="ts-feed-avatar-wrap">
        <FeedAvatar feed={feed} />
      </span>

      <div className="ts-feed-body">
        <div className="ts-feed-line1">
          <h3 className="ts-feed-name">{feed.name}</h3>
          {feed.category ? (
            <button className="ts-feed-cat" onClick={onPickCat}>
              {tfCatName(cats, feed.category)}
            </button>
          ) : (
            <button className="ts-feed-cat is-uncat" onClick={onPickCat}>
              uncategorised
            </button>
          )}
          {hasErr && (
            <span className="ts-feed-err-chip" title={h.err}>
              <TFI k="warn" size={9} w={1.6} />
              {h.errors} {h.errors === 1 ? "error" : "errors"}
            </span>
          )}
          {hasErr && h.backoff && (
            <span className="ts-feed-backoff-chip">
              backoff · retry in {h.backoff}
            </span>
          )}
        </div>
        <div className="ts-feed-line2">
          <a className="ts-feed-url" href={`https://${feed.url}`} target="_blank" rel="noopener noreferrer" onClick={(e) => { e.preventDefault(); onOpenSite(); }}>
            {feed.url}
            <span className="ico"><TFI k="external" size={10} w={1.4} /></span>
          </a>
          <span className="dot" aria-hidden="true"></span>
          {hasErr ? (
            <>
              <span className="err">last poll <b style={{ color: "inherit" }}>{h.last}</b> ago</span>
              <span className="dot" aria-hidden="true"></span>
              <button
                className="ts-feeds-util-btn"
                onClick={onToggleExpand}
                style={{ padding: "2px 6px", color: "inherit" }}
              >
                {isExpanded ? "Hide details" : "Why?"}
                <TFI k="caret" size={8} />
              </button>
            </>
          ) : (
            <>
              <span>polled <b>{h.last}</b> ago</span>
              <span className="dot" aria-hidden="true"></span>
              <span>next <b>{h.next}</b></span>
              <span className="dot" aria-hidden="true"></span>
              <span>cadence <b>{h.cad}</b></span>
            </>
          )}
        </div>
      </div>

      <div className="ts-feed-actions">
        <span className={`ts-feed-act-unread ${feed._unread === 0 ? "is-zero" : ""}`}>
          <b>{feed._unread}</b><span>unread</span>
        </span>
        <button
          className={`ts-feed-act ${busy ? "is-spinning" : ""}`}
          onClick={onRefresh}
          aria-label="Refresh now"
          title="Refresh now"
        >
          <TFI k="refresh" size={13} w={1.5} />
        </button>
        <button className="ts-feed-act" onClick={onEdit} aria-label="Edit feed" title="Edit">
          <TFI k="more" size={14} w={1.4} />
        </button>
      </div>

      {isExpanded && (
        <div className="ts-feed-health">
          <div className="ts-feed-health-eyebrow">
            <span><TFI k="warn" size={9} w={1.6} /> Last error</span>
            <span className="when">first seen {h.firstSeen}</span>
          </div>
          <div className="ts-feed-health-msg">{h.err}</div>
          <div className="ts-feed-health-grid">
            <div className="ts-feed-health-cell">
              <span className="ts-feed-health-l">Consecutive errors</span>
              <span className="ts-feed-health-v">{h.errors}</span>
            </div>
            <div className="ts-feed-health-cell">
              <span className="ts-feed-health-l">In backoff</span>
              <span className="ts-feed-health-v">yes · retry in {h.backoff}</span>
            </div>
            <div className="ts-feed-health-cell">
              <span className="ts-feed-health-l">Normal cadence</span>
              <span className="ts-feed-health-v">{h.cad}</span>
            </div>
          </div>
          <div className="ts-feed-health-actions">
            <button className="ts-feeds-util-btn" onClick={onRefresh}><TFI k="refresh" size={10} />Retry now</button>
            <button className="ts-feeds-util-btn" onClick={onEdit}>Edit credentials</button>
            <button className="ts-feeds-util-btn" onClick={onMarkRead}>Mark all read</button>
          </div>
        </div>
      )}
    </div>
  );
}

// ─────────────────────────────────────────────
// Bulk action bar
// ─────────────────────────────────────────────

function TFBulkBar({ count, onClear, onMarkRead, onReassign, onDelete }) {
  return (
    <div className="ts-feeds-bulk">
      <span className="ts-feeds-bulk-count">{count} selected</span>
      <button className="ts-feeds-bulk-clear" onClick={onClear}>clear</button>
      <span className="ts-feeds-bulk-spacer" />
      <button className="ts-feeds-bulk-btn" onClick={onMarkRead}>
        <TFI k="check" size={11} /> Mark all read
      </button>
      <button className="ts-feeds-bulk-btn" onClick={onReassign}>
        Reassign category <TFI k="caret" size={9} />
      </button>
      <button className="ts-feeds-bulk-btn is-danger" onClick={onDelete}>
        <TFI k="trash" size={11} /> Delete
      </button>
    </div>
  );
}

// ─────────────────────────────────────────────
// Add Feed dialog (with auto-discovery preview)
// ─────────────────────────────────────────────

function TFAddDialog({ initialUrl = "", initialState = "idle", cats, onClose, onSubmit }) {
  const [url, setUrl] = useStateF(initialUrl);
  const [state, setState] = useStateF(initialState); // idle | loading | site | feed | error
  const [picked, setPicked] = useStateF(0);
  const [category, setCategory] = useStateF("");

  const candidates = [
    { name: "rachelbythebay — main feed", url: "rachelbythebay.com/w/atom.xml", type: "atom", items: "32 items" },
    { name: "rachelbythebay — comments", url: "rachelbythebay.com/w/comments.xml", type: "rss",  items: "8 items"  },
  ];

  return (
    <div className="ts-overlay" onClick={onClose}>
      <div className="ts-dialog is-wide" onClick={(e) => e.stopPropagation()}>
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">Add a feed</div>
          <button className="ts-dialog-close" onClick={onClose} aria-label="Close">×</button>
        </div>
        <div className="ts-dialog-body">
          <div className="ts-feeds-form">
            <div className="ts-feeds-form-field">
              <label className="ts-feeds-form-l">
                Feed or site URL
                <span className="desc">Paste a feed URL directly, or any page on the site — we'll find the feeds for you.</span>
              </label>
              <div className="ts-feeds-form-field-row">
                <input
                  className="ts-feeds-form-input"
                  placeholder="https://example.com  or  https://example.com/feed.xml"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  autoFocus
                />
                <button
                  className="ts-feeds-form-go"
                  disabled={!url.trim()}
                  onClick={() => setState("site")}
                >
                  Look up
                </button>
              </div>
            </div>

            {state === "loading" && (
              <div className="ts-feeds-disc">
                <div className="ts-feeds-disc-head">
                  <span className="acc">●</span> Probing {url || "URL"}…
                </div>
              </div>
            )}

            {state === "site" && (
              <>
                <div className="ts-feeds-disc">
                  <div className="ts-feeds-disc-head">
                    <span className="acc">●</span> Found {candidates.length} feeds on <b style={{ color: "var(--ink)" }}>rachelbythebay.com</b>
                  </div>
                  {candidates.map((c, i) => (
                    <div
                      key={i}
                      className={`ts-feeds-disc-row ${picked === i ? "is-picked" : ""}`}
                      onClick={() => setPicked(i)}
                    >
                      <span className="ts-feeds-disc-radio" />
                      <div>
                        <div className="ts-feeds-disc-name">{c.name}</div>
                        <div className="ts-feeds-disc-meta">{c.url} · {c.items}</div>
                      </div>
                      <span className="ts-feeds-disc-tag">{c.type}</span>
                    </div>
                  ))}
                </div>

                <div className="ts-feeds-form-field">
                  <label className="ts-feeds-form-l">Assign to category</label>
                  <div className="ts-feeds-edit-cat-list">
                    <button className={`ts-feeds-edit-cat-btn ${category === "" ? "is-active" : ""}`} onClick={() => setCategory("")}>
                      Uncategorised
                    </button>
                    {cats.map((c) => (
                      <button
                        key={c.id}
                        className={`ts-feeds-edit-cat-btn ${category === c.id ? "is-active" : ""}`}
                        onClick={() => setCategory(c.id)}
                      >
                        {c.name}
                      </button>
                    ))}
                  </div>
                </div>
              </>
            )}

            {state === "feed" && (
              <div className="ts-feeds-disc">
                <div className="ts-feeds-disc-head">
                  Direct feed detected · <b style={{ color: "var(--ink)" }}>Atom 1.0</b> · 32 items
                </div>
                <div className="ts-feeds-disc-row is-picked">
                  <span className="ts-feeds-disc-radio" />
                  <div>
                    <div className="ts-feeds-disc-name">{url || "rachelbythebay.com/w/atom.xml"}</div>
                    <div className="ts-feeds-disc-meta">Last updated 4h ago · etag will be respected</div>
                  </div>
                  <span className="ts-feeds-disc-tag">atom</span>
                </div>
              </div>
            )}
          </div>
        </div>
        <div className="ts-dialog-foot">
          <button className="ts-dialog-cancel" onClick={onClose}>Cancel</button>
          <button
            className="ts-dialog-confirm"
            disabled={state === "idle" || !url.trim()}
            onClick={() => onSubmit({ url, category, picked })}
          >
            Subscribe
          </button>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Edit Feed dialog — full form
// ─────────────────────────────────────────────

function TFEditDialog({ feed, cats, onClose, onSave, onDelete }) {
  const [name, setName] = useStateF(feed.name);
  const [category, setCategory] = useStateF(feed.category || "");
  const [extract, setExtract] = useStateF(true);
  const [selector, setSelector] = useStateF("article.post, main article");
  const [cookie, setCookie] = useStateF("");
  const [authUser, setAuthUser] = useStateF("");
  const [authPass, setAuthPass] = useStateF("");

  return (
    <div className="ts-overlay" onClick={onClose}>
      <div className="ts-dialog is-edit" onClick={(e) => e.stopPropagation()}>
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">Edit feed</div>
          <button className="ts-dialog-close" onClick={onClose} aria-label="Close">×</button>
        </div>
        <div className="ts-dialog-body">
          <div className="ts-feeds-edit-grid">
            <div className="head">Basics</div>

            <div className="l">
              Title
              <div className="desc">Shown in the sidebar and in the article list.</div>
            </div>
            <div className="r">
              <input
                className="ts-feeds-form-input"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>

            <div className="l">
              Feed URL
              <div className="desc">Read-only here. Use re-subscribe to move to a different feed.</div>
            </div>
            <div className="r">
              <input
                className="ts-feeds-form-input"
                style={{ color: "var(--ink-3)" }}
                readOnly
                value={`https://${feed.url}`}
              />
            </div>

            <div className="l">
              Category
              <div className="desc">Drag-free assignment. One feed, one category.</div>
            </div>
            <div className="r">
              <div className="ts-feeds-edit-cat-list">
                <button className={`ts-feeds-edit-cat-btn ${category === "" ? "is-active" : ""}`} onClick={() => setCategory("")}>
                  Uncategorised
                </button>
                {cats.map((c) => (
                  <button
                    key={c.id}
                    className={`ts-feeds-edit-cat-btn ${category === c.id ? "is-active" : ""}`}
                    onClick={() => setCategory(c.id)}
                  >
                    {c.name}
                  </button>
                ))}
              </div>
            </div>

            <div className="head">Article extraction</div>

            <div className="l">
              Full text
              <div className="desc">Fetch and parse the source page when the feed only gives a summary.</div>
            </div>
            <div className="r">
              <div
                className={`ts-feeds-edit-toggle ${extract ? "is-on" : ""}`}
                onClick={() => setExtract(!extract)}
              >
                <span className="ts-feeds-edit-toggle-sw" />
                <span className="ts-feeds-edit-toggle-label">{extract ? "Enabled" : "Disabled — use whatever the feed provides"}</span>
              </div>
            </div>

            <div className="l">
              Custom selector
              <div className="desc">CSS selector that picks the article body. Leave blank for auto-detection.</div>
            </div>
            <div className="r">
              <input
                className="ts-feeds-form-input"
                placeholder="article, main, .post-content"
                value={selector}
                onChange={(e) => setSelector(e.target.value)}
                disabled={!extract}
              />
            </div>

            <div className="head">Credentials</div>

            <div className="l">
              Cookie header
              <div className="desc">Sent verbatim on every fetch. One <code style={{ fontSize: 11 }}>name=value</code> pair per line.</div>
            </div>
            <div className="r">
              <textarea
                className="ts-feeds-form-input"
                placeholder="session=abc123…&#10;cf_clearance=…"
                value={cookie}
                onChange={(e) => setCookie(e.target.value)}
              />
            </div>

            <div className="l">
              HTTP Basic
              <div className="desc">Sent on the initial fetch and on every redirect to the same host.</div>
            </div>
            <div className="r">
              <div className="ts-feeds-kv-pair">
                <input
                  className="ts-feeds-form-input"
                  placeholder="Username"
                  value={authUser}
                  onChange={(e) => setAuthUser(e.target.value)}
                />
                <input
                  className="ts-feeds-form-input"
                  placeholder="Password"
                  type="password"
                  value={authPass}
                  onChange={(e) => setAuthPass(e.target.value)}
                />
              </div>
            </div>
          </div>
        </div>
        <div className="ts-dialog-foot" style={{ justifyContent: "space-between" }}>
          <button className="ts-dialog-cancel" style={{ color: "#c43a3a" }} onClick={onDelete}>
            Delete this feed
          </button>
          <span style={{ display: "inline-flex", gap: 8 }}>
            <button className="ts-dialog-cancel" onClick={onClose}>Cancel</button>
            <button className="ts-dialog-confirm" onClick={() => onSave({ name, category, extract, selector, cookie, authUser, authPass })}>Save changes</button>
          </span>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Delete confirm (single + bulk)
// ─────────────────────────────────────────────

function TFDeleteDialog({ feeds, onClose, onConfirm }) {
  const isBulk = feeds.length > 1;
  return (
    <div className="ts-overlay" onClick={onClose}>
      <div className="ts-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">
            {isBulk ? `Delete ${feeds.length} feeds?` : `Delete "${feeds[0].name}"?`}
          </div>
          <button className="ts-dialog-close" onClick={onClose} aria-label="Close">×</button>
        </div>
        <div className="ts-dialog-body">
          <p className="ts-dialog-p">
            {isBulk
              ? "Removes the subscriptions and all unread entries. Saved articles stay in your archive."
              : "The subscription is removed. Unread entries go too. Saved articles you've kept stay in your archive."}
          </p>
          <ul className="ts-dialog-list">
            {feeds.slice(0, 5).map((f) => (
              <li key={f.id} className="ts-dialog-list-item">
                <FeedAvatar feed={f} />
                <span className="ts-dialog-list-item-name">{f.name}</span>
                <span className="ts-dialog-list-item-arrow">{f.url}</span>
              </li>
            ))}
            {feeds.length > 5 && (
              <li className="ts-dialog-list-item">
                <span style={{ width: 14 }} />
                <span className="ts-dialog-list-item-name" style={{ color: "var(--ink-3)", fontStyle: "italic" }}>
                  …and {feeds.length - 5} more
                </span>
              </li>
            )}
          </ul>
        </div>
        <div className="ts-dialog-foot">
          <button className="ts-dialog-cancel" onClick={onClose}>Cancel</button>
          <button className="ts-dialog-confirm is-danger" onClick={onConfirm}>
            {isBulk ? `Delete ${feeds.length} feeds` : "Delete feed"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Import OPML dialog
// ─────────────────────────────────────────────

function TFImportDialog({ stage = "drop", onClose, onConfirm }) {
  return (
    <div className="ts-overlay" onClick={onClose}>
      <div className="ts-dialog is-wide" onClick={(e) => e.stopPropagation()}>
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">Import OPML</div>
          <button className="ts-dialog-close" onClick={onClose} aria-label="Close">×</button>
        </div>
        <div className="ts-dialog-body">
          {stage === "drop" ? (
            <div className="ts-feeds-form">
              <div className="ts-feeds-opml-drop">
                <div className="ts-feeds-opml-drop-ico"><TFI k="upload" size={16} w={1.4} /></div>
                <div>Drop an <b>.opml</b> file here, or <span className="link">browse</span></div>
                <div style={{ fontFamily: "var(--mono)", fontSize: 10.5, color: "var(--ink-4)", marginTop: 6, letterSpacing: "0.04em" }}>
                  exports from Reeder, Feedbin, NetNewsWire, NewsBlur, Inoreader…
                </div>
              </div>
              <p style={{ fontFamily: "var(--sans)", fontSize: 12.5, color: "var(--ink-3)", margin: 0, lineHeight: 1.55 }}>
                Categories are matched by name; new ones are created for unknown buckets. Existing
                subscriptions are not duplicated.
              </p>
            </div>
          ) : (
            <div className="ts-feeds-form">
              <div className="ts-feeds-opml-stats">
                <div className="item"><span>Feeds in file</span><b>34</b></div>
                <div className="item"><span>New to subscribe</span><b>26</b></div>
                <div className="item"><span>Already present</span><b>8</b></div>
                <div className="item"><span>Categories</span><b>5</b></div>
              </div>
              <div>
                <div className="ts-feeds-form-l" style={{ marginBottom: 6 }}>
                  Preview
                  <span className="desc">First 6 of 34 entries · scroll for the rest.</span>
                </div>
                <div className="ts-feeds-opml-preview">
{`<?xml version="1.0" encoding="UTF-8"?>
`}<span className="t">{`<opml`}</span>{` `}<span className="k">version</span>{`="`}<span className="v">2.0</span>{`">
  `}<span className="t">{`<head>`}</span>{`
    `}<span className="t">{`<title>`}</span>{`Subscriptions from `}<span className="v">tap</span><span className="t">{`</title>`}</span>{`
  `}<span className="t">{`</head>`}</span>{`
  `}<span className="t">{`<body>`}</span>{`
    `}<span className="t">{`<outline`}</span>{` `}<span className="k">text</span>{`="`}<span className="v">People</span>{`">
      `}<span className="t">{`<outline`}</span>{` `}<span className="k">type</span>{`="`}<span className="v">rss</span>{`" `}<span className="k">text</span>{`="`}<span className="v">Julia Evans</span>{`" `}<span className="k">xmlUrl</span>{`="`}<span className="v">https://jvns.ca/atom.xml</span>{`"/>
      `}<span className="t">{`<outline`}</span>{` `}<span className="k">type</span>{`="`}<span className="v">rss</span>{`" `}<span className="k">text</span>{`="`}<span className="v">Dan Luu</span>{`" `}<span className="k">xmlUrl</span>{`="`}<span className="v">https://danluu.com/atom.xml</span>{`"/>
      `}<span className="c">{`<!-- 5 more outlines in People -->`}</span>{`
    `}<span className="t">{`</outline>`}</span>{`
    `}<span className="t">{`<outline`}</span>{` `}<span className="k">text</span>{`="`}<span className="v">Systems &amp; PL</span>{`">
      `}<span className="c">{`<!-- 9 outlines collapsed -->`}</span>{`
    `}<span className="t">{`</outline>`}</span>{`
  `}<span className="t">{`</body>`}</span>{`
`}<span className="t">{`</opml>`}</span>
                </div>
              </div>
            </div>
          )}
        </div>
        <div className="ts-dialog-foot">
          <button className="ts-dialog-cancel" onClick={onClose}>Cancel</button>
          <button className="ts-dialog-confirm" onClick={onConfirm}>
            {stage === "drop" ? "Pick file" : "Import 26 new feeds"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Export OPML dialog
// ─────────────────────────────────────────────

function TFExportDialog({ feeds, cats, onClose }) {
  return (
    <div className="ts-overlay" onClick={onClose}>
      <div className="ts-dialog is-wide" onClick={(e) => e.stopPropagation()}>
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">Export OPML</div>
          <button className="ts-dialog-close" onClick={onClose} aria-label="Close">×</button>
        </div>
        <div className="ts-dialog-body">
          <div className="ts-feeds-form">
            <div className="ts-feeds-opml-stats">
              <div className="item"><span>Feeds</span><b>{feeds.length}</b></div>
              <div className="item"><span>Categories</span><b>{cats.length}</b></div>
              <div className="item"><span>With errors</span><b>{feeds.filter((f) => tfHealth(f.id).errors > 0).length}</b></div>
              <div className="item"><span>File size</span><b>~6 KB</b></div>
            </div>
            <div>
              <div className="ts-feeds-form-l" style={{ marginBottom: 6 }}>
                Preview
                <span className="desc">Standard OPML 2.0. Categories become outline groups; per-feed credentials are <i>not</i> exported.</span>
              </div>
              <div className="ts-feeds-opml-preview">
{`<?xml version="1.0" encoding="UTF-8"?>
`}<span className="t">{`<opml`}</span>{` `}<span className="k">version</span>{`="`}<span className="v">2.0</span>{`">
  `}<span className="t">{`<head>`}</span>{`
    `}<span className="t">{`<title>`}</span>{`tap subscriptions`}<span className="t">{`</title>`}</span>{`
    `}<span className="t">{`<dateCreated>`}</span>{`Sun, 10 May 2026 14:22:08 GMT`}<span className="t">{`</dateCreated>`}</span>{`
    `}<span className="t">{`<ownerName>`}</span>{`m.lee`}<span className="t">{`</ownerName>`}</span>{`
  `}<span className="t">{`</head>`}</span>{`
  `}<span className="t">{`<body>`}</span>{`
`}{cats.map((c) => {
  const list = feeds.filter((f) => f.category === c.id);
  return (
    <span key={c.id}>
      {`    `}<span className="t">{`<outline`}</span>{` `}<span className="k">text</span>{`="`}<span className="v">{c.name}</span>{`">\n`}
      {list.slice(0, 2).map((f) => (
        <span key={f.id}>{`      `}<span className="t">{`<outline`}</span>{` `}<span className="k">type</span>{`="`}<span className="v">rss</span>{`" `}<span className="k">text</span>{`="`}<span className="v">{f.name}</span>{`" `}<span className="k">xmlUrl</span>{`="`}<span className="v">https://{f.url}/feed</span>{`"/>\n`}</span>
      ))}
      {list.length > 2 && <span>{`      `}<span className="c">{`<!-- ${list.length - 2} more in ${c.name} -->`}</span>{`\n`}</span>}
      {`    `}<span className="t">{`</outline>`}</span>{`\n`}
    </span>
  );
})}{`  `}<span className="t">{`</body>`}</span>{`
`}<span className="t">{`</opml>`}</span>
              </div>
            </div>
          </div>
        </div>
        <div className="ts-dialog-foot">
          <button className="ts-dialog-cancel" onClick={onClose}>Copy to clipboard</button>
          <button className="ts-dialog-confirm" onClick={onClose}>
            <TFI k="download" size={11} /> &nbsp;Download tap-subscriptions.opml
          </button>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Desktop page
// ─────────────────────────────────────────────

function TSFeedsPage({
  feeds: initialFeeds,
  entries,
  categories,
  theme = "light",
  fontMode = "serif",
  force = {},
}) {
  const [feeds, setFeeds] = useStateF(() =>
    initialFeeds.map((f) => ({ ...f, category: force.assignments?.[f.id] ?? f.category })),
  );
  const [search, setSearch] = useStateF(force.search ?? "");
  const [filter, setFilter] = useStateF(force.filter ?? "all"); // all | errors | uncat | cat:<id>
  const [sort, setSort] = useStateF(force.sort ?? "title");
  const [grouped, setGrouped] = useStateF(!!force.grouped);
  const [selected, setSelected] = useStateF(new Set(force.selected ?? []));
  const [expanded, setExpanded] = useStateF(new Set(force.expanded ?? []));
  const [refreshing, setRefreshing] = useStateF(new Set(force.refreshing ?? []));
  const [refreshingAll, setRefreshingAll] = useStateF(!!force.refreshingAll);
  const [dialog, setDialog] = useStateF(force.dialog ?? null);

  // Compose: each feed gets _unread and _h health stats.
  const feedsX = useMemoF(
    () => feeds.map((f) => ({ ...f, _unread: tfUnread(entries, f.id), _h: tfHealth(f.id) })),
    [feeds, entries],
  );

  const errorCount = useMemoF(() => feedsX.filter((f) => f._h.errors > 0).length, [feedsX]);
  const uncatCount = useMemoF(() => feedsX.filter((f) => !f.category).length, [feedsX]);

  // Apply filter
  const filtered = useMemoF(() => {
    let xs = feedsX;
    if (filter === "errors") xs = xs.filter((f) => f._h.errors > 0);
    else if (filter === "uncat") xs = xs.filter((f) => !f.category);
    else if (filter && filter.startsWith("cat:")) {
      const id = filter.slice(4);
      xs = xs.filter((f) => f.category === id);
    }
    if (search.trim()) {
      const q = search.trim().toLowerCase();
      xs = xs.filter((f) => f.name.toLowerCase().includes(q) || f.url.toLowerCase().includes(q));
    }
    xs = [...xs].sort((a, b) => tfMatchSort(a, b, sort));
    return xs;
  }, [feedsX, filter, search, sort]);

  // Optionally group
  const groups = useMemoF(() => {
    if (!grouped) return null;
    const out = {};
    categories.forEach((c) => (out[c.id] = []));
    out.__uncat = [];
    filtered.forEach((f) => {
      if (f.category && out[f.category]) out[f.category].push(f);
      else out.__uncat.push(f);
    });
    return out;
  }, [filtered, grouped, categories]);

  // Handlers
  const toggleSelected = (id) => {
    const next = new Set(selected);
    next.has(id) ? next.delete(id) : next.add(id);
    setSelected(next);
  };
  const clearSelected = () => setSelected(new Set());
  const toggleExpanded = (id) => {
    const next = new Set(expanded);
    next.has(id) ? next.delete(id) : next.add(id);
    setExpanded(next);
  };
  const refreshOne = (id) => {
    const next = new Set(refreshing); next.add(id); setRefreshing(next);
    setTimeout(() => {
      const n2 = new Set(refreshing); n2.delete(id); setRefreshing(n2);
    }, 1500);
  };
  const refreshAll = () => {
    setRefreshingAll(true);
    setTimeout(() => setRefreshingAll(false), 2000);
  };
  const openDialog = (d) => setDialog(d);
  const closeDialog = () => setDialog(null);

  const selectedFeeds = feedsX.filter((f) => selected.has(f.id));
  const anySelected = selected.size > 0;

  // Filter chip definitions (in order). Build with live counts.
  const filterChips = [
    { key: "all", label: "All", ct: feedsX.length },
    { key: "errors", label: "Errors", ct: errorCount, warn: true },
    { key: "uncat", label: "Uncategorised", ct: uncatCount },
  ];

  const sortOptions = [
    { v: "title",  l: "Title (A–Z)" },
    { v: "unread", l: "Most unread" },
    { v: "last",   l: "Recently polled" },
    { v: "next",   l: "Next poll" },
    { v: "errors", l: "Errors first" },
  ];

  // Render a single feed row (factored to keep both flat and grouped views the same).
  const renderRow = (f) => (
    <TFFeedRow
      key={f.id}
      feed={f}
      isSelected={selected.has(f.id)}
      onToggleSelect={() => toggleSelected(f.id)}
      isExpanded={expanded.has(f.id)}
      onToggleExpand={() => toggleExpanded(f.id)}
      onRefresh={() => refreshOne(f.id)}
      busy={refreshing.has(f.id) || refreshingAll}
      onEdit={() => openDialog({ type: "edit", feedId: f.id })}
      onDelete={() => openDialog({ type: "delete", feedIds: [f.id] })}
      onMarkRead={() => {}}
      onOpenSite={() => {}}
      onOpenFeed={() => {}}
      onPickCat={() => openDialog({ type: "edit", feedId: f.id })}
      anySelected={anySelected}
      cats={categories}
    />
  );

  return (
    <div
      className={`tap theme-${theme} ts-root`}
      style={fontMode === "sans" ? { fontFamily: "var(--sans)" } : {}}
    >
      <div className="ts-shell" style={{ position: "relative", minHeight: "100%" }}>
        <TopTabs
          active="feeds"
          unreadCount={entries.filter((e) => !e.read).length}
          onNav={() => {}}
        />
        <main className="ts-main">
          <div className="ts-feeds">
            <header className="ts-set-head">
              <div className="ts-set-eyebrow">
                Subscriptions · {feedsX.length} feeds · {categories.length} categories
                {errorCount > 0 && (
                  <> · <span style={{ color: "#c43a3a" }}>{errorCount} with errors</span></>
                )}
              </div>
              <h1 className="ts-set-title">Feeds</h1>
            </header>

            {/* Top toolbar */}
            <div className="ts-feeds-top">
              <div className="ts-feeds-search">
                <span className="ts-feeds-search-ico"><TFI k="search" size={14} w={1.4} /></span>
                <input
                  className="ts-feeds-search-input"
                  placeholder="Search feeds by name or URL…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <button className="ts-feeds-add" onClick={() => openDialog({ type: "add" })}>
                <TFI k="plus" size={11} w={1.8} />
                Add feed
              </button>
            </div>

            {/* Secondary toolbar */}
            <div className="ts-feeds-row2">
              {filterChips.map((c) => (
                <button
                  key={c.key}
                  className={`ts-feeds-chip ${filter === c.key ? "is-active" : ""} ${c.warn ? "is-warn" : ""}`}
                  onClick={() => setFilter(c.key)}
                >
                  {c.label}
                  <span className="ct">{c.ct}</span>
                </button>
              ))}

              <span className="ts-feeds-sep" />

              <button
                className={`ts-feeds-chip ${grouped ? "is-active" : ""}`}
                onClick={() => setGrouped(!grouped)}
                title="Group by category"
              >
                Group by category
              </button>

              <span className="ts-feeds-sort">
                Sort
                <select
                  className="ts-feeds-sort-select"
                  value={sort}
                  onChange={(e) => setSort(e.target.value)}
                >
                  {sortOptions.map((o) => (
                    <option key={o.v} value={o.v}>{o.l}</option>
                  ))}
                </select>
              </span>

              <span className="ts-feeds-spacer" />

              <button
                className={`ts-feeds-util-btn ${refreshingAll ? "is-spinning" : ""}`}
                onClick={refreshAll}
                title="Refresh all feeds now"
              >
                <TFI k="refresh" size={11} w={1.5} />
                {refreshingAll ? "Polling…" : "Refresh all"}
              </button>
              <button
                className="ts-feeds-util-btn"
                onClick={() => openDialog({ type: "import" })}
              >
                <TFI k="upload" size={11} w={1.4} />
                Import OPML
              </button>
              <button
                className="ts-feeds-util-btn"
                onClick={() => openDialog({ type: "export" })}
              >
                <TFI k="download" size={11} w={1.4} />
                Export
              </button>
            </div>

            {/* Bulk action bar */}
            {anySelected && (
              <TFBulkBar
                count={selected.size}
                onClear={clearSelected}
                onMarkRead={() => {}}
                onReassign={() => openDialog({ type: "bulkReassign" })}
                onDelete={() => openDialog({ type: "delete", feedIds: [...selected] })}
              />
            )}

            {/* List */}
            <div className="ts-feeds-list" style={{ marginTop: anySelected ? 4 : 6 }}>
              {filtered.length === 0 ? (
                <div className="ts-feeds-empty">
                  {feedsX.length === 0 ? (
                    <>
                      <div className="ts-feeds-empty-title">No feeds yet.</div>
                      <p className="ts-feeds-empty-sub">
                        Tap is empty without something to read. Add a feed by URL, or import an OPML
                        export from another reader to bring your subscriptions over.
                      </p>
                      <div style={{ display: "inline-flex", gap: 6 }}>
                        <button className="ts-feeds-add" onClick={() => openDialog({ type: "add" })}>
                          <TFI k="plus" size={11} w={1.8} /> Add a feed
                        </button>
                        <button className="ts-feeds-util-btn" style={{ padding: "9px 14px", border: "1px solid var(--rule)" }} onClick={() => openDialog({ type: "import" })}>
                          <TFI k="upload" size={11} w={1.4} /> Import OPML
                        </button>
                      </div>
                    </>
                  ) : (
                    <>
                      <div className="ts-feeds-empty-title">No feeds match.</div>
                      <p className="ts-feeds-empty-sub">
                        Nothing matches "{search}" with the current filter. Clear the search or
                        switch back to <b>All</b>.
                      </p>
                      <button
                        className="ts-feeds-util-btn"
                        style={{ padding: "8px 14px", border: "1px solid var(--rule)" }}
                        onClick={() => { setSearch(""); setFilter("all"); }}
                      >
                        Reset filters
                      </button>
                    </>
                  )}
                </div>
              ) : grouped ? (
                <>
                  {categories.map((c) => {
                    const list = groups[c.id] || [];
                    if (list.length === 0) return null;
                    return (
                      <React.Fragment key={c.id}>
                        <div className="ts-feeds-group">
                          <span>{c.name}</span>
                          <span className="rule" />
                          <span className="ct">{list.length} {list.length === 1 ? "feed" : "feeds"}</span>
                        </div>
                        {list.map(renderRow)}
                      </React.Fragment>
                    );
                  })}
                  {groups.__uncat.length > 0 && (
                    <>
                      <div className="ts-feeds-group">
                        <span>Uncategorised</span>
                        <span className="rule" />
                        <span className="ct">{groups.__uncat.length} {groups.__uncat.length === 1 ? "feed" : "feeds"}</span>
                      </div>
                      {groups.__uncat.map(renderRow)}
                    </>
                  )}
                </>
              ) : (
                filtered.map(renderRow)
              )}
            </div>

            {/* Footer status */}
            <div className="ts-feeds-foot">
              <span className="pulse" aria-hidden="true" />
              <span>Auto-polling · last full sweep 2 minutes ago</span>
              <span className="sep">·</span>
              <span>{feedsX.length} feeds across {categories.length} categories</span>
              {errorCount > 0 && (
                <>
                  <span className="sep">·</span>
                  <span style={{ color: "#c43a3a" }}>{errorCount} need attention</span>
                </>
              )}
            </div>
          </div>
        </main>

        {/* Dialogs */}
        {dialog?.type === "add" && (
          <TFAddDialog
            initialUrl={force.addUrl ?? ""}
            initialState={force.addState ?? "idle"}
            cats={categories}
            onClose={closeDialog}
            onSubmit={closeDialog}
          />
        )}
        {dialog?.type === "edit" && (() => {
          const f = feedsX.find((x) => x.id === dialog.feedId);
          if (!f) return null;
          return (
            <TFEditDialog
              feed={f}
              cats={categories}
              onClose={closeDialog}
              onSave={closeDialog}
              onDelete={() => setDialog({ type: "delete", feedIds: [f.id] })}
            />
          );
        })()}
        {dialog?.type === "delete" && (() => {
          const fs = feedsX.filter((x) => dialog.feedIds.includes(x.id));
          return (
            <TFDeleteDialog
              feeds={fs}
              onClose={closeDialog}
              onConfirm={() => {
                setFeeds(feeds.filter((x) => !dialog.feedIds.includes(x.id)));
                clearSelected();
                closeDialog();
              }}
            />
          );
        })()}
        {dialog?.type === "import" && (
          <TFImportDialog
            stage={force.importStage ?? "preview"}
            onClose={closeDialog}
            onConfirm={closeDialog}
          />
        )}
        {dialog?.type === "export" && (
          <TFExportDialog
            feeds={feedsX}
            cats={categories}
            onClose={closeDialog}
          />
        )}
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Mobile feeds view
// ─────────────────────────────────────────────

function TSFeedsMobile({
  feeds: initialFeeds, entries, categories,
  theme = "light", fontMode = "serif", force = {},
}) {
  const feeds = initialFeeds.map((f) => ({ ...f, _unread: tfUnread(entries, f.id), _h: tfHealth(f.id) }));
  const errorCount = feeds.filter((f) => f._h.errors > 0).length;

  return (
    <div
      className={`tap theme-${theme} ts-root is-mobile`}
      style={{ position: "relative", height: "100%", overflow: "hidden", ...(fontMode === "sans" ? { fontFamily: "var(--sans)" } : {}) }}
    >
      <TapMobileShell
        title="Feeds"
        count={feeds.length}
        countLabel={feeds.length === 1 ? "feed" : "feeds"}
        active="feeds"
        unreadCount={entries.filter((e) => !e.read).length}
        onNav={() => {}}
      >
        <div className="ts-mobile-feeds-tools">
          <button className="ts-feeds-chip is-active">All<span className="ct">{feeds.length}</span></button>
          <button className="ts-feeds-chip is-warn is-active">Errors<span className="ct">{errorCount}</span></button>
          <button className="ts-feeds-chip">Uncategorised<span className="ct">0</span></button>
          <button className="ts-feeds-chip">Group</button>
        </div>

        <div className="ts-mobile-feeds-list">
          {feeds.slice(0, 8).map((f) => (
            <div key={f.id} className="ts-mobile-feed-row">
              <span className="ts-feed-avatar-wrap"><FeedAvatar feed={f} /></span>
              <div className="ts-feed-body">
                <div className="ts-feed-line1" style={{ marginBottom: 2 }}>
                  <h3 className="ts-feed-name">{f.name}</h3>
                  {f._h.errors > 0 && (
                    <span className="ts-feed-err-chip"><TFI k="warn" size={9} w={1.6} />{f._h.errors}</span>
                  )}
                </div>
                <div className="ts-feed-line2">
                  <span>{f.url}</span>
                  <span className="dot" />
                  <span>{tfCatName(categories, f.category) || "uncat"}</span>
                  <span className="dot" />
                  <span>{f._h.errors > 0 ? <span className="err">last {f._h.last} ago</span> : <>polled <b>{f._h.last}</b></>}</span>
                </div>
              </div>
              <span className={`ts-feed-act-unread ${f._unread === 0 ? "is-zero" : ""}`}>
                <b>{f._unread}</b>
              </span>
            </div>
          ))}
        </div>
      </TapMobileShell>

      <button className="ts-mobile-feeds-fab" aria-label="Add feed">
        <TFI k="plus" size={18} w={1.8} />
      </button>

      {force.sheet === "edit" && (
        <>
          <div className="ts-mobile-backdrop" />
          <div className="ts-mobile-edit-sheet">
            <div className="ts-mobile-sheet-handle" />
            <h2>Edit feed</h2>
            <div className="ts-feeds-form">
              <div className="ts-feeds-form-field">
                <label className="ts-feeds-form-l">Title</label>
                <input className="ts-feeds-form-input" defaultValue="Julia Evans" />
              </div>
              <div className="ts-feeds-form-field">
                <label className="ts-feeds-form-l">Category</label>
                <div className="ts-feeds-edit-cat-list">
                  <button className="ts-feeds-edit-cat-btn">Uncategorised</button>
                  <button className="ts-feeds-edit-cat-btn is-active">People</button>
                  <button className="ts-feeds-edit-cat-btn">Systems &amp; PL</button>
                  <button className="ts-feeds-edit-cat-btn">Aggregators</button>
                  <button className="ts-feeds-edit-cat-btn">Newsletters</button>
                </div>
              </div>
              <div className="ts-feeds-form-field">
                <label className="ts-feeds-form-l">
                  Full-text extraction
                </label>
                <div className="ts-feeds-edit-toggle is-on">
                  <span className="ts-feeds-edit-toggle-sw" />
                  <span className="ts-feeds-edit-toggle-label">Enabled</span>
                </div>
              </div>
              <div className="ts-feeds-form-field">
                <label className="ts-feeds-form-l">Custom selector</label>
                <input className="ts-feeds-form-input" defaultValue="article.post" />
              </div>
              <button className="ts-feeds-form-go" style={{ width: "100%", padding: "12px 0" }}>Save changes</button>
            </div>
          </div>
        </>
      )}

      {force.sheet === "add" && (
        <>
          <div className="ts-mobile-backdrop" />
          <div className="ts-mobile-edit-sheet">
            <div className="ts-mobile-sheet-handle" />
            <h2>Add a feed</h2>
            <div className="ts-feeds-form">
              <div className="ts-feeds-form-field">
                <label className="ts-feeds-form-l">
                  Feed or site URL
                  <span className="desc">Paste a feed, or any page on the site.</span>
                </label>
                <input className="ts-feeds-form-input" defaultValue="rachelbythebay.com" autoFocus />
              </div>
              <div className="ts-feeds-disc">
                <div className="ts-feeds-disc-head"><span className="acc">●</span> Found 2 feeds</div>
                <div className="ts-feeds-disc-row is-picked">
                  <span className="ts-feeds-disc-radio" />
                  <div>
                    <div className="ts-feeds-disc-name">rachelbythebay — main</div>
                    <div className="ts-feeds-disc-meta">/w/atom.xml · 32 items</div>
                  </div>
                  <span className="ts-feeds-disc-tag">atom</span>
                </div>
                <div className="ts-feeds-disc-row">
                  <span className="ts-feeds-disc-radio" />
                  <div>
                    <div className="ts-feeds-disc-name">comments</div>
                    <div className="ts-feeds-disc-meta">/w/comments.xml · 8 items</div>
                  </div>
                  <span className="ts-feeds-disc-tag">rss</span>
                </div>
              </div>
              <button className="ts-feeds-form-go" style={{ width: "100%", padding: "12px 0" }}>Subscribe</button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

// ─────────────────────────────────────────────
// Frames — small wrappers used by the canvas
// ─────────────────────────────────────────────

function FeedsDesktopFrame({ theme, fontMode, force = {}, height = 1400 }) {
  return (
    <div style={{ width: "100%", height: "100%", position: "relative", overflow: "hidden" }}>
      <TSFeedsPage
        feeds={window.TAP_FEEDS}
        entries={window.TAP_ENTRIES}
        categories={window.TAP_CATEGORIES}
        theme={theme}
        fontMode={fontMode}
        force={force}
      />
    </div>
  );
}

function FeedsMobileFrame({ theme, fontMode, force = {} }) {
  return (
    <div style={{ width: "100%", height: "100%", position: "relative", overflow: "hidden" }}>
      <TSFeedsMobile
        feeds={window.TAP_FEEDS}
        entries={window.TAP_ENTRIES}
        categories={window.TAP_CATEGORIES}
        theme={theme}
        fontMode={fontMode}
        force={force}
      />
    </div>
  );
}

Object.assign(window, {
  TSFeedsPage,
  TSFeedsMobile,
  FeedsDesktopFrame,
  FeedsMobileFrame,
});
