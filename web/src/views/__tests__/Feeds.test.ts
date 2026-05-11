import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { writable } from 'svelte/store';

vi.mock('../../lib/store', () => {
  const _subs = writable([
    { id: 1, title: 'Julia', feed_url: 'https://jvns.ca/feed', site_url: '', error_count: 0, last_poll_at: 1000, created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false, category_id: null, next_poll_at: 0 },
    { id: 2, title: 'Dan',   feed_url: 'https://danluu.com/feed', site_url: '', error_count: 2, last_error: 'http 500', last_poll_at: 500, created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false, category_id: null, next_poll_at: 0 },
  ]);
  const subStore = {
    subscribe: _subs.subscribe,
    load: vi.fn(),
    refresh: vi.fn(),
    remove: vi.fn().mockResolvedValue(undefined),
    setCategory: vi.fn(),
    add: vi.fn(),
  };
  return {
    subscriptions: subStore,
    categories: { subscribe: writable([{ id: 3, name: 'People', unread: 0, created_at: 0, position: 0 }]).subscribe, load: vi.fn() },
    entries: { subscribe: writable({ items: [], loading: false, error: null }).subscribe, load: vi.fn() },
  };
});

vi.mock('../../lib/api', () => ({
  api: {
    discoverFeeds: vi.fn(),
    addSubscription: vi.fn(),
    updateSubscription: vi.fn(),
    exportOPML: vi.fn(),
    importOPML: vi.fn(),
    refreshSubscription: vi.fn(),
    deleteSubscription: vi.fn(),
  },
}));

vi.mock('../../lib/auth', () => ({
  auth: { subscribe: vi.fn(() => () => {}), clearOn401: vi.fn(), setCSRFToken: vi.fn() },
  notifySW: vi.fn(),
  ERR_UNAUTHORIZED: 'unauthorized',
}));

vi.mock('../../lib/offlineQueue', () => ({
  offlineQueue: { enqueue: vi.fn(), drain: vi.fn(), clearForUser: vi.fn() },
}));

import Feeds from '../Feeds.svelte';
import { subscriptions } from '../../lib/store';

beforeEach(() => { vi.clearAllMocks(); });

describe('Feeds view', () => {
  it('renders the page head, toolbar, and one row per subscription', () => {
    render(Feeds);
    expect(screen.getByRole('heading', { level: 1, name: /feeds/i })).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/search feeds/i)).toBeInTheDocument();
    expect(screen.getByText('Julia')).toBeInTheDocument();
    expect(screen.getByText('Dan')).toBeInTheDocument();
  });

  it('clicking Add feed opens the AddFeedDialog', async () => {
    render(Feeds);
    // The toolbar Add feed button (not the dialog, which is not yet open)
    const addBtn = screen.getAllByRole('button', { name: /add feed/i })[0];
    await fireEvent.click(addBtn);
    // Dialog opened — the URL input is now visible
    expect(await screen.findByPlaceholderText(/example\.com/i)).toBeInTheDocument();
  });

  it('typing in search filters the list', async () => {
    render(Feeds);
    await fireEvent.input(screen.getByPlaceholderText(/search feeds/i), { target: { value: 'julia' } });
    expect(screen.getByText('Julia')).toBeInTheDocument();
    expect(screen.queryByText('Dan')).not.toBeInTheDocument();
  });

  it('clicking the Errors chip filters to feeds with errors', async () => {
    const { container } = render(Feeds);
    await fireEvent.click(container.querySelector('button[data-filter-key="errors"]')!);
    expect(screen.queryByText('Julia')).not.toBeInTheDocument();
    expect(screen.getByText('Dan')).toBeInTheDocument();
  });

  it('selecting a row shows the bulk bar', async () => {
    const { container } = render(Feeds);
    await fireEvent.click(container.querySelector('.ts-feed-check')!);
    expect(screen.getByText(/1 selected/i)).toBeInTheDocument();
  });

  it('Refresh all calls subscriptions.refresh for every visible feed', async () => {
    vi.mocked(subscriptions.refresh).mockResolvedValue(undefined);
    render(Feeds);
    await fireEvent.click(screen.getByRole('button', { name: /refresh all/i }));
    await waitFor(() => expect(vi.mocked(subscriptions.refresh)).toHaveBeenCalledTimes(2));
  });
});

describe('Feeds view — bulk delete', () => {
  it('bulk delete partial failure surfaces foot message', async () => {
    vi.mocked(subscriptions.remove).mockImplementation(async (id: number) => {
      if (id === 2) throw new Error('csrf invalid');
    });
    const { container } = render(Feeds);
    for (const cb of Array.from(container.querySelectorAll<HTMLButtonElement>('.ts-feed-check'))) {
      await fireEvent.click(cb);
    }
    await fireEvent.click(screen.getByRole('button', { name: /^delete$/i }));
    await fireEvent.click(await screen.findByRole('button', { name: /^delete 2 feeds$/i }));
    await waitFor(() => expect(screen.getByText(/removed 1 of 2.*1 failed/i)).toBeInTheDocument());
  });
});
