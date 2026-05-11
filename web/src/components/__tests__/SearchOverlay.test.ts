import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, waitFor } from '@testing-library/svelte';

// Use the real store module but mock internals
vi.mock('../../lib/searchOverlay.svelte', () => {
  let _open = false;
  let _query = '';
  return {
    searchOverlay: {
      get open() { return _open; },
      get query() { return _query; },
      get scope() { return 'unread'; },
      openOverlay() { _open = true; },
      close() { _open = false; _query = ''; },
      setQuery(v: string) { _query = v; },
      setScope() {},
      // allow tests to set open directly for simplicity
      _setOpen(v: boolean) { _open = v; },
    },
  };
});

const mockSearch = vi.fn();
vi.mock('../../lib/api', () => ({
  api: { searchEntries: (...a: unknown[]) => mockSearch(...a) },
}));

const mockNavigate = vi.fn();
vi.mock('../../lib/router', () => ({ navigate: (...a: unknown[]) => mockNavigate(...a) }));

import { searchOverlay } from '../../lib/searchOverlay.svelte';
import SearchOverlay from '../SearchOverlay.svelte';

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
  searchOverlay.close();
});
afterEach(() => vi.useRealTimers());

describe('SearchOverlay', () => {
  it('does not render when closed', () => {
    const { container } = render(SearchOverlay);
    expect(container.querySelector('.search-overlay, .panel')).toBeNull();
  });

  it('renders input when opened', () => {
    searchOverlay.openOverlay();
    const { container } = render(SearchOverlay);
    expect(container.querySelector('input')).toBeTruthy();
  });

  it('shows "at least 3" hint when query is 1-2 chars', async () => {
    searchOverlay.openOverlay();
    render(SearchOverlay);
    const input = screen.getByRole('searchbox');
    await fireEvent.input(input, { target: { value: 'ab' } });
    vi.advanceTimersByTime(300);
    // The hint is conditionally rendered based on searchOverlay.query.length.
    // Since the mock propagates setQuery, the rendered hint should be visible.
    await waitFor(() => expect(screen.getByText(/at least 3/i)).toBeTruthy());
  });

  it('debounces api.searchEntries by 250ms', async () => {
    searchOverlay.openOverlay();
    mockSearch.mockResolvedValue({ data: [] });
    render(SearchOverlay);
    const input = screen.getByRole('searchbox');
    await fireEvent.input(input, { target: { value: 'abc' } });
    vi.advanceTimersByTime(249);
    expect(mockSearch).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    await waitFor(() => expect(mockSearch).toHaveBeenCalledWith('abc'));
  });

  it('closes on Esc', async () => {
    searchOverlay.openOverlay();
    const { container } = render(SearchOverlay);
    const overlay = container.querySelector('.search-overlay') as HTMLElement;
    await fireEvent.keyDown(overlay, { key: 'Escape' });
    expect(searchOverlay.open).toBe(false);
  });

  it('closes on scrim click', async () => {
    searchOverlay.openOverlay();
    const { container } = render(SearchOverlay);
    const overlay = container.querySelector('.search-overlay') as HTMLElement;
    await fireEvent.click(overlay);
    expect(searchOverlay.open).toBe(false);
  });

  it('navigates to /entry/:id on result click and closes', async () => {
    searchOverlay.openOverlay();
    mockSearch.mockResolvedValue({ data: [
      { id: 7, subscription_id: 1, title: 'hit', published_at: 0, fetched_at: 0,
        read: false, saved: false, extract_failed: false, url: 'https://e/' },
    ] });
    render(SearchOverlay);
    const input = screen.getByRole('searchbox');
    await fireEvent.input(input, { target: { value: 'hit' } });
    vi.advanceTimersByTime(250);
    await waitFor(() => expect(screen.getByText('hit')).toBeTruthy());
    await fireEvent.click(screen.getByText('hit'));
    expect(mockNavigate).toHaveBeenCalledWith('/entry/7');
    expect(searchOverlay.open).toBe(false);
  });
});
