import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../lib/api', () => ({
  api: {
    discoverFeeds: vi.fn(),
    addSubscription: vi.fn(),
  },
}));

vi.mock('../../../lib/auth', () => ({
  auth: { subscribe: vi.fn(() => () => {}), clearOn401: vi.fn(), setCSRFToken: vi.fn() },
  notifySW: vi.fn(),
  ERR_UNAUTHORIZED: 'unauthorized',
}));

vi.mock('../../../lib/offlineQueue', () => ({
  offlineQueue: { enqueue: vi.fn(), drain: vi.fn(), clearForUser: vi.fn() },
}));

import AddFeedDialog from '../AddFeedDialog.svelte';
import type { Category } from '../../../lib/types';
import { api } from '../../../lib/api';

const noop = () => {};
const cat: Category = { id: 3, name: 'People', unread: 0, created_at: 0, position: 0 };

beforeEach(() => {
  vi.clearAllMocks();
});

describe('AddFeedDialog — URL input + Look up', () => {
  it('renders a mono URL input and a disabled Look up button', () => {
    render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
    expect(screen.getByPlaceholderText(/example\.com/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /look up/i })).toBeDisabled();
  });

  it('Look up enables once a non-blank URL is typed', async () => {
    render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
    await fireEvent.input(screen.getByPlaceholderText(/example\.com/i), { target: { value: 'https://jvns.ca' } });
    expect(screen.getByRole('button', { name: /look up/i })).not.toBeDisabled();
  });

  it('calls api.discoverFeeds with the trimmed URL on Look up', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [] });
    render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
    await fireEvent.input(screen.getByPlaceholderText(/example\.com/i), { target: { value: '  https://jvns.ca ' } });
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
    expect(api.discoverFeeds).toHaveBeenCalledWith('https://jvns.ca');
  });
});

describe('AddFeedDialog — discovery results', () => {
  it('zero candidates: shows "No feeds found" inline', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [] });
    render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
    await fireEvent.input(screen.getByPlaceholderText(/example\.com/i), { target: { value: 'https://nothing/' } });
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
    expect(await screen.findByText(/no feeds found/i)).toBeInTheDocument();
  });

  it('one candidate: pre-picks it', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [{ title: 'Solo', feed_url: 'https://solo/feed.xml', site_url: 'https://solo', type: 'atom' }] });
    const { container } = render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
    await fireEvent.input(screen.getByPlaceholderText(/example\.com/i), { target: { value: 'https://solo' } });
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
    await waitFor(() => {
      expect(container.querySelector('.ts-feeds-disc-row.is-picked')).not.toBeNull();
    });
  });

  it('multiple candidates: lets the user pick one', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({
      candidates: [
        { title: 'main', feed_url: 'https://rbtb/atom.xml', site_url: '', type: 'atom' },
        { title: 'comments', feed_url: 'https://rbtb/comments.xml', site_url: '', type: 'rss' },
      ],
    });
    const { container } = render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
    await fireEvent.input(screen.getByPlaceholderText(/example\.com/i), { target: { value: 'https://rbtb' } });
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
    const rows = await screen.findAllByText(/main|comments/);
    await fireEvent.click(rows[1].closest('.ts-feeds-disc-row')!);
    const picked = container.querySelector('.ts-feeds-disc-row.is-picked')!;
    expect(picked.textContent).toMatch(/comments/);
  });
});

describe('AddFeedDialog — category assignment + subscribe', () => {
  it('renders Uncategorised + a button for each category', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [{ title: 'X', feed_url: 'https://x/feed.xml', site_url: '', type: 'rss' }] });
    render(AddFeedDialog, { props: { categories: [cat], onClose: noop, onAdded: noop } });
    await fireEvent.input(screen.getByPlaceholderText(/example\.com/i), { target: { value: 'https://x' } });
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /uncategorised/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /people/i })).toBeInTheDocument();
    });
  });

  it('Subscribe calls api.addSubscription with feed_url + category_id, then onAdded', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [{ title: 'X', feed_url: 'https://x/feed.xml', site_url: '', type: 'rss' }] });
    vi.mocked(api.addSubscription).mockResolvedValue({ id: 99 } as any);
    const onAdded = vi.fn();
    render(AddFeedDialog, { props: { categories: [cat], onClose: noop, onAdded } });
    await fireEvent.input(screen.getByPlaceholderText(/example\.com/i), { target: { value: 'https://x' } });
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
    await fireEvent.click(await screen.findByRole('button', { name: /people/i }));
    await fireEvent.click(screen.getByRole('button', { name: /^subscribe$/i }));
    expect(api.addSubscription).toHaveBeenCalledWith({ feed_url: 'https://x/feed.xml', category_id: 3 });
    expect(onAdded).toHaveBeenCalledOnce();
  });

  it('Subscribe with Uncategorised omits category_id', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [{ title: 'X', feed_url: 'https://x/feed.xml', site_url: '', type: 'rss' }] });
    vi.mocked(api.addSubscription).mockResolvedValue({ id: 99 } as any);
    render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: vi.fn() } });
    await fireEvent.input(screen.getByPlaceholderText(/example\.com/i), { target: { value: 'https://x' } });
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
    await screen.findByRole('button', { name: /^subscribe$/i });
    await fireEvent.click(screen.getByRole('button', { name: /^subscribe$/i }));
    const callArg = vi.mocked(api.addSubscription).mock.calls[0][0];
    expect(callArg).toEqual({ feed_url: 'https://x/feed.xml' });
  });

  it('Subscribe surfaces api.addSubscription errors inline', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [{ title: 'X', feed_url: 'https://x/feed.xml', site_url: '', type: 'rss' }] });
    vi.mocked(api.addSubscription).mockRejectedValue(new Error('feed already subscribed'));
    render(AddFeedDialog, { props: { categories: [], onClose: noop, onAdded: noop } });
    await fireEvent.input(screen.getByPlaceholderText(/example\.com/i), { target: { value: 'https://x' } });
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }));
    await fireEvent.click(await screen.findByRole('button', { name: /^subscribe$/i }));
    expect(await screen.findByText(/feed already subscribed/i)).toBeInTheDocument();
  });
});
