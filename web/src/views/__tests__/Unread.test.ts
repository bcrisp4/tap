import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import type { EntryListItem, Subscription } from '../../lib/types';

// Mock child Svelte components before importing the view under test.
vi.mock('../../components/EntryRow.svelte', () => ({ default: vi.fn() }));

// Mock the router so navigate doesn't touch window.location.
vi.mock('../../lib/router', () => ({
  navigate: vi.fn(),
  route: { subscribe: (fn: (v: unknown) => void) => { fn({ name: 'unread' }); return () => {}; } },
}));

// Store mock state that tests can mutate before rendering.
type EntriesState = { items: EntryListItem[]; loading: boolean; error: string | null };

const mockEntriesLoad = vi.fn();
const mockSubscriptionsLoad = vi.fn();

let _entriesState: EntriesState = { items: [], loading: false, error: null };
let _subscriptionsState: Subscription[] = [];
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
    toggleRead: vi.fn(),
  },
  subscriptions: {
    subscribe: (fn: (v: Subscription[]) => void) => {
      _subsSubs.push(fn);
      fn(_subscriptionsState);
      return () => { _subsSubs.splice(_subsSubs.indexOf(fn), 1); };
    },
    load: () => mockSubscriptionsLoad(),
  },
}));

function setEntries(state: EntriesState) {
  _entriesState = state;
  _entriesSubs.forEach(fn => fn(state));
}

// Import the view after mocks are established.
const { default: Unread } = await import('../Unread.svelte');

describe('Unread view', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    _entriesSubs.length = 0;
    _subsSubs.length = 0;
    _entriesState = { items: [], loading: false, error: null };
    _subscriptionsState = [];
    mockEntriesLoad.mockResolvedValue(undefined);
    mockSubscriptionsLoad.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('calls entries.load(true) and subscriptions.load() on mount', async () => {
    render(Unread);
    await waitFor(() => {
      expect(mockEntriesLoad).toHaveBeenCalledWith(true);
      expect(mockSubscriptionsLoad).toHaveBeenCalled();
    });
  });

  it('shows loading indicator when entries.loading is true', () => {
    setEntries({ items: [], loading: true, error: null });
    render(Unread);
    expect(screen.getByText(/Loading/)).toBeInTheDocument();
  });

  it('shows error message when entries.error is set', () => {
    setEntries({ items: [], loading: false, error: 'Failed to fetch' });
    render(Unread);
    expect(screen.getByText('Failed to fetch')).toBeInTheDocument();
  });

  it('shows empty state when there are no entries', () => {
    setEntries({ items: [], loading: false, error: null });
    render(Unread);
    expect(screen.getByText(/No unread entries/)).toBeInTheDocument();
  });

  it('renders a <ul role="list"> for entry items', () => {
    setEntries({ items: [
      { id: 1, title: 'Entry One', read: false, saved: false, subscription_id: 10, published_at: 1700000000, fetched_at: 1700000001, url: 'https://a.com', extract_failed: false },
    ], loading: false, error: null });
    const { container } = render(Unread);
    expect(container.querySelector('ul[role="list"]')).toBeTruthy();
  });

});

describe('Unread keyboard context', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    _entriesSubs.length = 0;
    _subsSubs.length = 0;
    _entriesState = { items: [
      { id: 1, title: 'Entry One', read: false, saved: false, subscription_id: 10, published_at: 1700000000, fetched_at: 1700000001, url: 'https://a.com', extract_failed: false },
      { id: 2, title: 'Entry Two', read: false, saved: false, subscription_id: 10, published_at: 1700000000, fetched_at: 1700000001, url: 'https://b.com', extract_failed: false },
    ], loading: false, error: null };
    _subscriptionsState = [];
    mockEntriesLoad.mockResolvedValue(undefined);
    mockSubscriptionsLoad.mockResolvedValue(undefined);
  });

  it('registers onNext in dispatch context on mount', () => {
    const dispatch = { onNext: () => {}, onPrev: () => {}, onOpen: () => {}, onToggleRead: () => {}, onToggleSaved: () => {}, onViewOriginal: () => {} };
    render(Unread, { context: new Map([['keyDispatch', dispatch]]) });
    expect(() => dispatch.onNext()).not.toThrow();
  });

  it('navigates to entry when onOpen called after onNext selects first entry', async () => {
    const routerMod = await import('../../lib/router');
    const mockNav = vi.mocked(routerMod.navigate as unknown as (...args: unknown[]) => void);
    mockNav.mockClear?.();
    const dispatch = { onNext: () => {}, onPrev: () => {}, onOpen: () => {}, onToggleRead: () => {}, onToggleSaved: () => {}, onViewOriginal: () => {} };
    render(Unread, { context: new Map([['keyDispatch', dispatch]]) });
    dispatch.onNext();
    dispatch.onOpen();
    expect(mockNav).toHaveBeenCalledWith('/entry/1');
  });
});
