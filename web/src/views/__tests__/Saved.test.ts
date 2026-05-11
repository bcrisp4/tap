import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import Saved from '../Saved.svelte';
import { api } from '../../lib/api';

vi.mock('../../lib/api', () => ({
  api: {
    listEntries: vi.fn(),
    listSubscriptions: vi.fn(() => Promise.resolve([])),
    patchEntry: vi.fn(() => Promise.resolve()),
  },
}));

// Force the desktop branch so the view renders SavedRow (not SavedMobileRow).
vi.mock('../../lib/breakpoints.svelte', () => ({
  isMobile: { subscribe: (fn: (v: boolean) => void) => { fn(false); return () => {}; } },
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('Saved view', () => {
  it('shows the empty state when no entries are saved', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [], next_cursor: undefined });
    render(Saved);
    await waitFor(() => {
      expect(screen.getByText(/nothing saved yet/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/press/i).textContent).toMatch(/S/);
  });

  it('lists saved entries when the API returns data', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 3, title: 'Saved one',  url: 'a', author: '',
          published_at: 1700000000, fetched_at: 1700000000,
          read: false, saved: true, extract_failed: false },
        { id: 2, subscription_id: 3, title: 'Saved two',  url: 'b', author: '',
          published_at: 1700000100, fetched_at: 1700000100,
          read: true,  saved: true, extract_failed: false },
      ],
      next_cursor: undefined,
    });
    render(Saved);
    await waitFor(() => {
      expect(screen.getByText('Saved one')).toBeInTheDocument();
      expect(screen.getByText('Saved two')).toBeInTheDocument();
    });
  });

  it('passes saved=true to the API', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [], next_cursor: undefined });
    render(Saved);
    await waitFor(() => {
      expect(api.listEntries).toHaveBeenCalledWith(
        expect.objectContaining({ saved: true }),
      );
    });
  });

  it('shows an error state when the API rejects', async () => {
    vi.mocked(api.listEntries).mockRejectedValueOnce(new Error('boom'));
    render(Saved);
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/boom/);
    });
  });

  it('renders the SavedToolbar with the correct count', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({
      data: Array.from({ length: 4 }).map((_, i) => ({
        id: i + 1, subscription_id: 3, title: `Title ${i}`,
        url: 'u', author: '', published_at: 1700000000 + i,
        fetched_at: 1700000000 + i,
        read: false, saved: true, extract_failed: false,
      })),
      next_cursor: undefined,
    });
    render(Saved);
    await waitFor(() => {
      expect(screen.getByText('4')).toBeInTheDocument();
      expect(screen.getByText(/entries/i)).toBeInTheDocument();
    });
  });

  it('optimistically unsaves an entry when the row Unsave action is clicked', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 3, title: 'Saved one',  url: 'a', author: '',
          published_at: 1700000000, fetched_at: 1700000000,
          read: false, saved: true, extract_failed: false },
      ],
      next_cursor: undefined,
    });
    vi.mocked(api.patchEntry).mockResolvedValueOnce(undefined as unknown as never);
    const { container } = render(Saved);
    await waitFor(() => expect(screen.getByText('Saved one')).toBeInTheDocument());

    const unsaveBtn = screen.getByRole('button', { name: /unsave/i });
    unsaveBtn.click();
    await waitFor(() => {
      expect(api.patchEntry).toHaveBeenCalledWith(1, { saved: false });
    });
    await waitFor(() => {
      expect(container.querySelector('.row')).toBeNull();
    });
  });
});
