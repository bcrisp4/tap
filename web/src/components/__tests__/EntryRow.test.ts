import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import EntryRow from '../EntryRow.svelte';
import type { EntryListItem, Subscription } from '../../lib/types';

const baseEntry: EntryListItem = {
  id: 1, subscription_id: 1, title: 'Hello', author: 'Ada',
  url: 'https://x', published_at: Math.floor(Date.now() / 1000) - 600,
  fetched_at: 0, read: false, saved: false, extract_failed: false,
};
const baseFeed: Subscription = {
  id: 1, title: 'Site', feed_url: 'https://x/feed', next_poll_at: 0,
  error_count: 0, created_at: 0, extract: false, extract_selector: '',
  has_cookie: false, has_basic_auth: false, category_id: null,
};

describe('EntryRow', () => {
  it('renders unread by default', () => {
    const { container } = render(EntryRow, { entry: baseEntry, feed: baseFeed });
    expect(container.querySelector('.entry')?.className).not.toMatch(/is-read/);
  });

  it('applies is-read when entry.read', () => {
    const { container } = render(EntryRow, { entry: { ...baseEntry, read: true }, feed: baseFeed });
    expect(container.querySelector('.entry')?.className).toMatch(/is-read/);
  });

  it('applies is-saved and shows mark when entry.saved', () => {
    const { container, getByText } = render(EntryRow, {
      entry: { ...baseEntry, saved: true }, feed: baseFeed,
    });
    expect(container.querySelector('.entry')?.className).toMatch(/is-saved/);
    expect(getByText(/saved/i)).toBeTruthy();
  });

  it('navigates on click', async () => {
    const { container } = render(EntryRow, { entry: baseEntry, feed: baseFeed });
    const pushState = vi.spyOn(window.history, 'pushState');
    await fireEvent.click(container.querySelector('.entry') as HTMLElement);
    expect(pushState).toHaveBeenCalled();
    pushState.mockRestore();
  });

  it('applies density class', () => {
    const { container } = render(EntryRow, { entry: baseEntry, feed: baseFeed, density: 'compact' });
    expect(container.querySelector('.entry')?.className).toMatch(/density-compact/);
  });
});
