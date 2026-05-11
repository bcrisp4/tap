import { describe, it, expect } from 'vitest';
import { filterFeeds, sortFeeds, countByKey, STALE_THRESHOLD_SECONDS, type FeedRow } from '../feedsFilter';

function row(p: Partial<FeedRow> & { id: number; title: string; feed_url: string }): FeedRow {
  return {
    id: p.id,
    title: p.title,
    feed_url: p.feed_url,
    site_url: p.site_url ?? '',
    next_poll_at: 0,
    last_poll_at: p.last_poll_at ?? 0,
    error_count: p.error_count ?? 0,
    last_error: p.last_error,
    created_at: p.created_at ?? 0,
    extract: false,
    extract_selector: '',
    has_cookie: false,
    has_basic_auth: false,
    category_id: p.category_id ?? null,
    unread: p.unread ?? 0,
    lastPollAgo: p.lastPollAgo ?? 0,
  };
}

describe('filterFeeds', () => {
  const rows = [
    row({ id: 1, title: 'Alpha',  feed_url: 'a.example/feed', unread: 4 }),
    row({ id: 2, title: 'Beta',   feed_url: 'b.example/feed', error_count: 2, last_error: 'http 500' }),
    row({ id: 3, title: 'Gamma',  feed_url: 'g.example/feed', unread: 0, lastPollAgo: STALE_THRESHOLD_SECONDS + 1 }),
    row({ id: 4, title: 'Delta',  feed_url: 'd.example/feed', unread: 1 }),
  ];

  it('all: returns every row', () => {
    expect(filterFeeds(rows, 'all', '').map(r => r.id)).toEqual([1, 2, 3, 4]);
  });
  it('errors: returns rows with error_count > 0', () => {
    expect(filterFeeds(rows, 'errors', '').map(r => r.id)).toEqual([2]);
  });
  it('unread: returns rows where unread > 0', () => {
    expect(filterFeeds(rows, 'unread', '').map(r => r.id)).toEqual([1, 4]);
  });
  it('stale: returns rows whose lastPollAgo exceeds STALE_THRESHOLD_SECONDS AND error_count is 0', () => {
    expect(filterFeeds(rows, 'stale', '').map(r => r.id)).toEqual([3]);
  });
  it('search: case-insensitive substring across title and feed_url; ANDs with filter', () => {
    expect(filterFeeds(rows, 'all', 'ALPHA').map(r => r.id)).toEqual([1]);
    expect(filterFeeds(rows, 'all', 'example').map(r => r.id)).toEqual([1, 2, 3, 4]);
    expect(filterFeeds(rows, 'errors', 'beta').map(r => r.id)).toEqual([2]);
    expect(filterFeeds(rows, 'errors', 'alpha').map(r => r.id)).toEqual([]);
  });
});

describe('sortFeeds', () => {
  const rows = [
    row({ id: 1, title: 'beta',  feed_url: '', last_poll_at: 100, unread: 0, created_at: 1 }),
    row({ id: 2, title: 'Alpha', feed_url: '', last_poll_at: 300, unread: 5, created_at: 2 }),
    row({ id: 3, title: 'alpha', feed_url: '', last_poll_at: 200, unread: 5, created_at: 3 }),
    row({ id: 4, title: 'Gamma', feed_url: '', last_poll_at: 0,   unread: 1, created_at: 4 }),
  ];
  it('name is case-insensitive alphabetical, id-asc on tie', () => {
    expect(sortFeeds(rows, 'name').map(r => r.id)).toEqual([2, 3, 1, 4]);
  });
  it('recent is by last_poll_at desc with zero last (never polled) sinking to bottom', () => {
    expect(sortFeeds(rows, 'recent').map(r => r.id)).toEqual([2, 3, 1, 4]);
  });
  it('added is by created_at desc', () => {
    expect(sortFeeds(rows, 'added').map(r => r.id)).toEqual([4, 3, 2, 1]);
  });
  it('unread is by unread desc, id-asc on tie', () => {
    expect(sortFeeds(rows, 'unread').map(r => r.id)).toEqual([2, 3, 4, 1]);
  });
});

describe('countByKey', () => {
  const rows = [
    row({ id: 1, title: 'Alpha',  feed_url: 'a.example/feed', unread: 4 }),
    row({ id: 2, title: 'Beta',   feed_url: 'b.example/feed', error_count: 2, last_error: 'http 500' }),
    row({ id: 3, title: 'Gamma',  feed_url: 'g.example/feed', unread: 0, lastPollAgo: STALE_THRESHOLD_SECONDS + 1 }),
    row({ id: 4, title: 'Delta',  feed_url: 'd.example/feed', unread: 1 }),
  ];

  it('countByKey returns chip counts for all / errors / unread / stale', () => {
    expect(countByKey(rows)).toEqual({ all: 4, errors: 1, unread: 2, stale: 1 });
  });
});
