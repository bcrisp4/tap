import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
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

function entry(overrides: Partial<EntryListItem> = {}): EntryListItem {
  return { ...baseEntry, ...overrides };
}

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

// M2 consumer tests — Unread list contract
describe('EntryRow — Unread consumer contract', () => {
  const mockNavigate = vi.fn();
  beforeEach(() => mockNavigate.mockReset());

  it('renders title and feed name', () => {
    const { getByText } = render(EntryRow, { props: { entry: baseEntry, feed: baseFeed } });
    expect(getByText('Hello')).toBeTruthy();
    expect(getByText('Site')).toBeTruthy();
  });

  it('junction dot is present for unread entry', () => {
    const { container } = render(EntryRow, { props: { entry: entry(), feed: baseFeed } });
    expect(container.querySelector('.junction')).toBeTruthy();
  });

  it('adds is-selected when isSelected=true', () => {
    const { container } = render(EntryRow, { props: { entry: entry(), feed: baseFeed, isSelected: true } });
    expect(container.querySelector('.entry.is-selected')).toBeTruthy();
  });

  it('hides summary when density=compact', () => {
    const { container } = render(EntryRow, {
      props: { entry: entry({ author: 'A summary line' }), feed: baseFeed, density: 'compact' },
    });
    // density-compact hides via CSS (display:none); verify the class is applied
    expect(container.querySelector('.entry.density-compact')).toBeTruthy();
  });

  it('shows summary element when density=comfortable and author present', () => {
    const { container } = render(EntryRow, {
      props: { entry: entry({ author: 'A summary line' }), feed: baseFeed, density: 'comfortable' },
    });
    expect(container.querySelector('.summary')).toBeTruthy();
  });

  it('navigates to /entry/:id on click', async () => {
    const pushState = vi.spyOn(window.history, 'pushState');
    const { container } = render(EntryRow, { props: { entry: entry({ id: 99 }), feed: baseFeed } });
    await fireEvent.click(container.querySelector('.entry') as HTMLElement);
    expect(pushState).toHaveBeenCalled();
    expect(pushState.mock.calls[0][2]).toContain('/entry/99');
    pushState.mockRestore();
  });

  it('renders even when feed is undefined', () => {
    const { getByText } = render(EntryRow, { props: { entry: entry(), feed: undefined } });
    expect(getByText('Hello')).toBeTruthy();
  });
});
