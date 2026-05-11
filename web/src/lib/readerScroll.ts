const PREFIX = 'tap.reader.scroll.';

export function loadScroll(entryId: number): number {
  try {
    const raw = localStorage.getItem(PREFIX + entryId);
    if (raw == null) return 0;
    const n = Number(raw);
    return Number.isFinite(n) ? n : 0;
  } catch {
    return 0;
  }
}

export function saveScroll(entryId: number, scrollTop: number): void {
  try {
    if (!Number.isFinite(scrollTop)) {
      localStorage.removeItem(PREFIX + entryId);
      return;
    }
    localStorage.setItem(PREFIX + entryId, String(Math.max(0, Math.round(scrollTop))));
  } catch {
    /* localStorage may be unavailable in private mode */
  }
}

export function clearScroll(entryId: number): void {
  try { localStorage.removeItem(PREFIX + entryId); } catch { /* ignore */ }
}
