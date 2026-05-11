import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from '../../lib/api';

vi.mock('../../lib/api', () => ({
  api: {
    listEntries: vi.fn(),
    listSubscriptions: vi.fn(() => Promise.resolve([])),
  },
}));

vi.mock('../../lib/store', () => ({
  subscriptions: {
    subscribe: (fn: (v: never[]) => void) => { fn([]); return () => {}; },
    load: vi.fn().mockResolvedValue(undefined),
  },
}));

vi.mock('../../components/EntryRow.svelte', () => ({ default: vi.fn() }));

// Fixed reference time — avoids flakiness around local midnight and DST.
// Pattern mirrors web/src/lib/__tests__/dayBands.test.ts.
const NOW_MS = new Date('2026-05-11T12:00:00Z').getTime();
const todayStart = new Date(NOW_MS);
todayStart.setHours(0, 0, 0, 0);
const todayStartSec = Math.floor(todayStart.getTime() / 1000);

// published_at relative to local start-of-day (positive = today, negative = past).
function pub(secFromTodayStart: number): number {
  return todayStartSec + secFromTodayStart;
}

const { default: History } = await import('../History.svelte');

describe('History view', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listEntries as ReturnType<typeof vi.fn>).mockResolvedValue({ data: [], next_cursor: undefined });
  });

  // Task 1: loading state
  it('renders the loading state while listEntries is pending', () => {
    vi.mocked(api.listEntries as ReturnType<typeof vi.fn>).mockReturnValue(new Promise(() => {}) as never);
    render(History);
    expect(screen.getByText(/Loading/i)).toBeTruthy();
  });

  // Task 1: error path
  it('renders the error message when listEntries rejects', async () => {
    vi.mocked(api.listEntries as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('boom'));
    render(History);
    expect(await screen.findByText('boom')).toBeTruthy();
  });

  // Task 1 / Task 7: empty path with EmptyState primitive
  it('renders the empty-state copy when there is no history', async () => {
    vi.mocked(api.listEntries as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ data: [], next_cursor: undefined });
    render(History);
    expect(await screen.findByText(/No history yet/i)).toBeTruthy();
    expect(await screen.findByText(/Subscribed feeds will accumulate/i)).toBeTruthy();
  });

  // Task 2: call shape — no unread, no saved
  it('calls api.listEntries with no unread and no saved filters', async () => {
    const spy = vi.mocked(api.listEntries as ReturnType<typeof vi.fn>);
    spy.mockResolvedValueOnce({ data: [], next_cursor: undefined });
    render(History);
    await waitFor(() => expect(spy).toHaveBeenCalled());
    const args = spy.mock.calls[0][0] ?? {};
    expect((args as Record<string, unknown>).unread).toBeUndefined();
    expect((args as Record<string, unknown>).saved).toBeUndefined();
  });

  // Task 3: renders rows
  it('renders one list item per entry returned', async () => {
    vi.mocked(api.listEntries as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 1, title: 'one', url: 'https://x/1',
          published_at: pub(3600), fetched_at: 1, read: true, saved: false, extract_failed: false },
        { id: 2, subscription_id: 1, title: 'two', url: 'https://x/2',
          published_at: pub(7200), fetched_at: 1, read: true, saved: false, extract_failed: false },
      ],
      next_cursor: undefined,
    });
    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('ul[role="list"]')).toBeTruthy());
    expect(container.querySelectorAll('li[role="listitem"]')).toHaveLength(2);
  });

  // Task 4: day-band ordering — fixed time, local-day-relative offsets
  it('renders day-band group headings in Today → Yesterday → This week → Earlier order', async () => {
    vi.mocked(api.listEntries as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 1, title: 'today',     url: 'https://x/1', published_at: pub(3600),           fetched_at: 1, read: true,  saved: false, extract_failed: false },
        { id: 2, subscription_id: 1, title: 'yesterday', url: 'https://x/2', published_at: pub(-86400 + 3600),   fetched_at: 1, read: true,  saved: false, extract_failed: false },
        { id: 3, subscription_id: 1, title: 'thisweek',  url: 'https://x/3', published_at: pub(-3 * 86400),      fetched_at: 1, read: true,  saved: false, extract_failed: false },
        { id: 4, subscription_id: 1, title: 'earlier',   url: 'https://x/4', published_at: pub(-30 * 86400),     fetched_at: 1, read: true,  saved: false, extract_failed: false },
      ],
      next_cursor: undefined,
    });

    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('section[data-band]')).toBeTruthy());

    const headings = Array.from(container.querySelectorAll('section[data-band]'))
      .map(el => el.getAttribute('data-band'));
    expect(headings).toEqual(['today', 'yesterday', 'thisWeek', 'earlier']);
  });

  // Task 4: empty bands are skipped
  it('omits day-band sections that have no entries', async () => {
    vi.mocked(api.listEntries as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 1, title: 'only today',   url: 'https://x/1', published_at: pub(3600),       fetched_at: 1, read: true, saved: false, extract_failed: false },
        { id: 2, subscription_id: 1, title: 'only earlier', url: 'https://x/2', published_at: pub(-30 * 86400), fetched_at: 1, read: true, saved: false, extract_failed: false },
      ],
      next_cursor: undefined,
    });

    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('section[data-band]')).toBeTruthy());

    const bandKeys = Array.from(container.querySelectorAll('section[data-band]'))
      .map(el => el.getAttribute('data-band'));
    expect(bandKeys).toEqual(['today', 'earlier']);
  });

  // Task 5: Load more button visibility
  it('shows a Load more button only when next_cursor is present', async () => {
    vi.mocked(api.listEntries as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: [{ id: 1, subscription_id: 1, title: 't', url: 'https://x/1', published_at: pub(3600), fetched_at: 1, read: true, saved: false, extract_failed: false }],
      next_cursor: 'CURSOR-1',
    });
    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('section[data-band]')).toBeTruthy());
    expect(container.querySelector('[data-action="load-more"]')).toBeTruthy();
  });

  it('hides Load more when next_cursor is undefined', async () => {
    vi.mocked(api.listEntries as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: [{ id: 1, subscription_id: 1, title: 't', url: 'https://x/1', published_at: pub(3600), fetched_at: 1, read: true, saved: false, extract_failed: false }],
      next_cursor: undefined,
    });
    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('section[data-band]')).toBeTruthy());
    expect(container.querySelector('[data-action="load-more"]')).toBeFalsy();
  });

  // Task 5: pagination
  it('appends entries and consumes the cursor when Load more is clicked', async () => {
    const spy = vi.mocked(api.listEntries as ReturnType<typeof vi.fn>);
    spy
      .mockResolvedValueOnce({
        data: [{ id: 1, subscription_id: 1, title: 'page1-today', url: 'https://x/1', published_at: pub(3600), fetched_at: 1, read: true, saved: false, extract_failed: false }],
        next_cursor: 'CURSOR-1',
      })
      .mockResolvedValueOnce({
        data: [{ id: 2, subscription_id: 1, title: 'page2-earlier', url: 'https://x/2', published_at: pub(-30 * 86400), fetched_at: 1, read: true, saved: false, extract_failed: false }],
        next_cursor: undefined,
      });

    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('[data-action="load-more"]')).toBeTruthy());

    (container.querySelector('[data-action="load-more"]') as HTMLButtonElement).click();

    await waitFor(() => expect(spy).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(container.querySelector('[data-action="load-more"]')).toBeFalsy());

    expect(spy.mock.calls[1][0]).toEqual(expect.objectContaining({ cursor: 'CURSOR-1', limit: 100 }));
    expect(container.querySelectorAll('li[role="listitem"]')).toHaveLength(2);
  });

  // loadMoreError is isolated — does not hide the loaded list
  it('shows inline error on load-more failure without hiding the loaded list', async () => {
    const spy = vi.mocked(api.listEntries as ReturnType<typeof vi.fn>);
    spy
      .mockResolvedValueOnce({
        data: [{ id: 1, subscription_id: 1, title: 'existing', url: 'https://x/1', published_at: pub(3600), fetched_at: 1, read: true, saved: false, extract_failed: false }],
        next_cursor: 'CURSOR-1',
      })
      .mockRejectedValueOnce(new Error('network error'));

    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('[data-action="load-more"]')).toBeTruthy());

    (container.querySelector('[data-action="load-more"]') as HTMLButtonElement).click();

    await waitFor(() => expect(screen.getByText('network error')).toBeTruthy());
    // Loaded list must still be visible
    expect(container.querySelectorAll('li[role="listitem"]')).toHaveLength(1);
    // Error banner (initial-load error branch) must NOT appear
    expect(container.querySelector('[role="alert"]')?.textContent).toBe('network error');
  });
});
