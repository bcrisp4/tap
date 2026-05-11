import type { Subscription } from './types';

export type FeedRow = Subscription & {
  unread: number;
  lastPollAgo: number; // seconds; Number.POSITIVE_INFINITY when last_poll_at is unset
};

export type FilterKey = 'all' | 'errors' | 'unread' | 'stale';
export type SortKey = 'recent' | 'name' | 'added' | 'unread';

export const STALE_THRESHOLD_SECONDS = 7 * 24 * 60 * 60;

export function filterFeeds(rows: FeedRow[], key: FilterKey, search: string): FeedRow[] {
  const q = search.trim().toLowerCase();
  return rows.filter((r) => {
    if (key === 'errors' && r.error_count <= 0) return false;
    if (key === 'unread' && r.unread <= 0) return false;
    if (key === 'stale') {
      if (r.error_count > 0) return false;
      if (r.lastPollAgo <= STALE_THRESHOLD_SECONDS) return false;
    }
    if (q) {
      const hay = (r.title + ' ' + r.feed_url).toLowerCase();
      if (!hay.includes(q)) return false;
    }
    return true;
  });
}

const compareByName   = (a: FeedRow, b: FeedRow) => a.title.localeCompare(b.title, undefined, { sensitivity: 'base' });
const compareByRecent = (a: FeedRow, b: FeedRow) => (b.last_poll_at || 0) - (a.last_poll_at || 0);
const compareByAdded  = (a: FeedRow, b: FeedRow) => (b.created_at || 0) - (a.created_at || 0);
const compareByUnread = (a: FeedRow, b: FeedRow) => b.unread - a.unread;

export function sortFeeds(rows: FeedRow[], key: SortKey): FeedRow[] {
  const cmp = { name: compareByName, recent: compareByRecent, added: compareByAdded, unread: compareByUnread }[key];
  return [...rows].sort((a, b) => {
    const c = cmp(a, b);
    return c !== 0 ? c : a.id - b.id;
  });
}

export function countByKey(rows: FeedRow[]): Record<FilterKey, number> {
  return {
    all:    rows.length,
    errors: filterFeeds(rows, 'errors', '').length,
    unread: filterFeeds(rows, 'unread', '').length,
    stale:  filterFeeds(rows, 'stale',  '').length,
  };
}
