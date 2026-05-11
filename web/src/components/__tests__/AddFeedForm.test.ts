import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import AddFeedForm from '../AddFeedForm.svelte';
import { api } from '../../lib/api';

vi.mock('../../lib/store', () => ({
  subscriptions: {
    subscribe: vi.fn((fn: (v: unknown[]) => void) => {
      fn([]);
      return () => {};
    }),
    add: vi.fn(),
    load: vi.fn(),
  },
}));

vi.mock('../../lib/api', () => ({
  api: {
    discoverFeeds: vi.fn(),
  },
}));

describe('AddFeedForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders a URL input and a Find feed button', () => {
    render(AddFeedForm);
    expect(screen.getByRole('textbox')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /find/i })).toBeInTheDocument();
  });

  it('Find button is disabled when the URL input is empty', () => {
    render(AddFeedForm);
    expect(screen.getByRole('button', { name: /find/i })).toBeDisabled();
  });

  it('Find button becomes enabled once a non-blank URL is typed', async () => {
    render(AddFeedForm);
    await fireEvent.input(screen.getByRole('textbox'), { target: { value: 'https://example.com' } });
    expect(screen.getByRole('button', { name: /find/i })).not.toBeDisabled();
  });

  it('shows candidates returned by api.discoverFeeds and subscribes to the chosen one', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({
      candidates: [
        { title: 'BBC News', feed_url: 'https://bbc/feed.xml', site_url: 'https://bbc', type: 'rss' },
        { title: 'BBC Sport', feed_url: 'https://bbc/sport.xml', site_url: 'https://bbc/sport', type: 'rss' },
      ],
    });
    const { subscriptions } = await import('../../lib/store');
    vi.mocked(subscriptions.add).mockResolvedValue(undefined);

    render(AddFeedForm);
    await fireEvent.input(screen.getByRole('textbox'), { target: { value: 'https://bbc' } });
    await fireEvent.click(screen.getByRole('button', { name: /find/i }));

    const pick = await screen.findByText('BBC Sport');
    await fireEvent.click(pick);

    expect(subscriptions.add).toHaveBeenCalledWith('https://bbc/sport.xml');
  });

  it('subscribes immediately when discovery returns a single candidate', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({
      candidates: [
        { title: 'Solo Feed', feed_url: 'https://solo/feed.xml', site_url: 'https://solo', type: 'atom' },
      ],
    });
    const { subscriptions } = await import('../../lib/store');
    vi.mocked(subscriptions.add).mockResolvedValue(undefined);

    render(AddFeedForm);
    await fireEvent.input(screen.getByRole('textbox'), { target: { value: 'https://solo' } });
    await fireEvent.click(screen.getByRole('button', { name: /find/i }));

    await waitFor(() => {
      expect(subscriptions.add).toHaveBeenCalledWith('https://solo/feed.xml');
    });
    expect(screen.queryByRole('list')).not.toBeInTheDocument();
  });

  it('surfaces no_feeds_found errors', async () => {
    vi.mocked(api.discoverFeeds).mockRejectedValue(new Error('no feeds found at that URL'));

    render(AddFeedForm);
    await fireEvent.input(screen.getByRole('textbox'), { target: { value: 'https://nothing/' } });
    await fireEvent.click(screen.getByRole('button', { name: /find/i }));

    expect(await screen.findByText(/no feeds found/i)).toBeTruthy();
  });

  it('shows "No feeds found" message when discovery returns empty candidates', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({ candidates: [] });

    render(AddFeedForm);
    await fireEvent.input(screen.getByRole('textbox'), { target: { value: 'https://nofeed/' } });
    await fireEvent.click(screen.getByRole('button', { name: /find/i }));

    await waitFor(() => {
      expect(screen.getByText(/no feeds found/i)).toBeInTheDocument();
    });
  });

  it('clears the URL input and candidates after subscribing', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({
      candidates: [
        { title: 'Feed A', feed_url: 'https://a/feed.xml', site_url: 'https://a', type: 'rss' },
      ],
    });
    const { subscriptions } = await import('../../lib/store');
    vi.mocked(subscriptions.add).mockResolvedValue(undefined);

    render(AddFeedForm);
    const input = screen.getByRole('textbox');
    await fireEvent.input(input, { target: { value: 'https://a' } });
    await fireEvent.click(screen.getByRole('button', { name: /find/i }));

    await waitFor(() => {
      expect((input as HTMLInputElement).value).toBe('');
    });
  });

  it('surfaces subscriptions.add error when picking a candidate fails', async () => {
    vi.mocked(api.discoverFeeds).mockResolvedValue({
      candidates: [
        { title: 'Feed B', feed_url: 'https://b/feed.xml', site_url: 'https://b', type: 'rss' },
        { title: 'Feed C', feed_url: 'https://c/feed.xml', site_url: 'https://c', type: 'rss' },
      ],
    });
    const { subscriptions } = await import('../../lib/store');
    vi.mocked(subscriptions.add).mockRejectedValue(new Error('already subscribed'));

    render(AddFeedForm);
    await fireEvent.input(screen.getByRole('textbox'), { target: { value: 'https://b' } });
    await fireEvent.click(screen.getByRole('button', { name: /find/i }));

    const pick = await screen.findByText('Feed B');
    await fireEvent.click(pick);

    await waitFor(() => {
      expect(screen.getByText('already subscribed')).toBeInTheDocument();
    });
  });
});
