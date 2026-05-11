import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import type { EntryListItem, Subscription } from '../../lib/types';

// Mock heavy child components but NOT Button — we need real Button to test the button.
vi.mock('../../components/EntryRow.svelte', () => ({ default: vi.fn() }));

// Mock the router.
vi.mock('../../lib/router', () => ({
  navigate: vi.fn(),
  route: { subscribe: (fn: (v: unknown) => void) => { fn({ name: 'unread' }); return () => {}; } },
}));

// Mock pulltorefresh to avoid DOM event listener issues.
vi.mock('../../lib/pulltorefresh', () => ({
  pullToRefresh: () => () => {},
}));

type EntriesState = { items: EntryListItem[]; loading: boolean; error: string | null };

const mockEntriesLoad = vi.fn();
const mockSubscriptionsLoad = vi.fn();
const mockToggleRead = vi.fn();

let _entriesState: EntriesState = { items: [], loading: false, error: null };
const _entriesSubs: Array<(v: EntriesState) => void> = [];
const _subsSubs: Array<(v: Subscription[]) => void> = [];

vi.mock('../../lib/store', () => ({
  entries: {
    subscribe: (fn: (v: EntriesState) => void) => {
      _entriesSubs.push(fn);
      fn(_entriesState);
      return () => { _entriesSubs.splice(_entriesSubs.indexOf(fn), 1); };
    },
    load: (...args: unknown[]) => mockEntriesLoad(...args),
    toggleRead: (...args: unknown[]) => mockToggleRead(...args),
  },
  subscriptions: {
    subscribe: (fn: (v: Subscription[]) => void) => {
      _subsSubs.push(fn);
      fn([]);
      return () => { _subsSubs.splice(_subsSubs.indexOf(fn), 1); };
    },
    load: () => mockSubscriptionsLoad(),
  },
}));

function setEntries(state: EntriesState) {
  _entriesState = state;
  _entriesSubs.forEach(fn => fn(state));
}

const { default: Unread } = await import('../Unread.svelte');

describe('Unread mark-all-read', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    _entriesSubs.length = 0;
    _subsSubs.length = 0;
    _entriesState = { items: [], loading: false, error: null };
    mockEntriesLoad.mockResolvedValue(undefined);
    mockSubscriptionsLoad.mockResolvedValue(undefined);
    mockToggleRead.mockResolvedValue(undefined);
  });

  it('calls entries.toggleRead(id, true) for every visible entry when mark-all-read is clicked', async () => {
    setEntries({ items: [
      { id: 1, title: 'Entry One', read: false, saved: false, subscription_id: 10, published_at: 1700000000, fetched_at: 1700000001, url: 'https://a.com', extract_failed: false },
      { id: 2, title: 'Entry Two', read: false, saved: false, subscription_id: 10, published_at: 1700000000, fetched_at: 1700000001, url: 'https://b.com', extract_failed: false },
    ], loading: false, error: null });
    render(Unread);
    await fireEvent.click(screen.getByRole('button', { name: /mark all/i }));
    await waitFor(() => {
      expect(mockToggleRead).toHaveBeenCalledWith(1, true);
      expect(mockToggleRead).toHaveBeenCalledWith(2, true);
    });
  });
});
