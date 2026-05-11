import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import type { EntryDetail } from '../../lib/types';

// Mock child Svelte components.
vi.mock('../../components/FeedAvatar.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/JunctionDot.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/Sidebar.svelte', () => ({ default: vi.fn() }));

// Mock store.
const mockToggleRead = vi.fn().mockResolvedValue(undefined);
const mockToggleSaved = vi.fn().mockResolvedValue(undefined);
vi.mock('../../lib/store', () => ({
  entries: {
    subscribe: (fn: (v: { items: unknown[] }) => void) => { fn({ items: [] }); return () => {}; },
    toggleRead: (...args: unknown[]) => mockToggleRead(...args),
    toggleSaved: (...args: unknown[]) => mockToggleSaved(...args),
  },
  subscriptions: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; }, load: vi.fn() },
}));

// Mock the router.
const mockNavigate = vi.fn();
vi.mock('../../lib/router', () => ({
  navigate: (...args: unknown[]) => mockNavigate(...args),
  route: { subscribe: (fn: (v: unknown) => void) => { fn({ name: 'reader', params: { id: 1 } }); return () => {}; } },
}));

// Mock the api module.
const mockGetEntry = vi.fn();
const mockPatchEntry = vi.fn();
vi.mock('../../lib/api', () => ({
  api: {
    getEntry: (...args: unknown[]) => mockGetEntry(...args),
    patchEntry: (...args: unknown[]) => mockPatchEntry(...args),
  },
}));

function makeEntry(overrides: Partial<EntryDetail> = {}): EntryDetail {
  return {
    id: 42,
    subscription_id: 1,
    title: 'Test Entry Title',
    author: 'Test Author',
    url: 'https://example.com/article/42',
    content: '<p>This is the article body.</p>',
    published_at: 1700000000,
    fetched_at: 1700000001,
    read: false,
    saved: false,
    extract_failed: false,
    ...overrides,
  };
}

// Import the view after mocks are established.
const { default: Reader } = await import('../Reader.svelte');

describe('Reader view', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('fetches the entry by id on mount via api.getEntry (not via store)', async () => {
    const entry = makeEntry();
    mockGetEntry.mockResolvedValueOnce(entry);
    mockPatchEntry.mockResolvedValueOnce({ ...entry, read: true });

    render(Reader, { props: { id: 42 } });

    await waitFor(() => {
      expect(mockGetEntry).toHaveBeenCalledWith(42);
    });
  });

  it('shows loading state before entry resolves', () => {
    // Never resolves — keeps component in loading state.
    mockGetEntry.mockReturnValueOnce(new Promise(() => {}));

    render(Reader, { props: { id: 42 } });

    expect(screen.getByText(/Loading/)).toBeInTheDocument();
  });

  it('renders entry title and content after load', async () => {
    const entry = makeEntry({ read: true }); // read=true → skip auto-patch
    mockGetEntry.mockResolvedValueOnce(entry);

    render(Reader, { props: { id: 42 } });

    await waitFor(() => {
      expect(screen.getByText('Test Entry Title')).toBeInTheDocument();
    });
  });

  it('auto-marks unread entry as read on mount via entries.toggleRead', async () => {
    const entry = makeEntry({ read: false });
    mockGetEntry.mockResolvedValueOnce(entry);

    render(Reader, { props: { id: 42 } });

    await waitFor(() => {
      expect(mockToggleRead).toHaveBeenCalledWith(42, true);
    });
  });

  it('does NOT call toggleRead if the entry is already read', async () => {
    const entry = makeEntry({ read: true });
    mockGetEntry.mockResolvedValueOnce(entry);

    render(Reader, { props: { id: 42 } });

    await waitFor(() => {
      expect(screen.getByText('Test Entry Title')).toBeInTheDocument();
    });
    expect(mockToggleRead).not.toHaveBeenCalled();
  });

  it('toggleRead button calls entries.toggleRead via store', async () => {
    const entry = makeEntry({ read: true }); // already read → no auto-patch
    mockGetEntry.mockResolvedValueOnce(entry);

    render(Reader, { props: { id: 42 } });

    await waitFor(() => {
      expect(screen.getByText('MARK UNREAD')).toBeInTheDocument();
    });

    const btn = screen.getByText('MARK UNREAD');
    await fireEvent.click(btn);

    await waitFor(() => {
      expect(mockToggleRead).toHaveBeenCalledWith(42, false);
    });
  });

  it('shows error when api.getEntry rejects', async () => {
    mockGetEntry.mockRejectedValueOnce(new Error('Entry not found'));

    render(Reader, { props: { id: 42 } });

    await waitFor(() => {
      expect(screen.getByText('Entry not found')).toBeInTheDocument();
    });
  });

  it('back button navigates to / via navigate()', async () => {
    const entry = makeEntry({ read: true });
    mockGetEntry.mockResolvedValueOnce(entry);

    render(Reader, { props: { id: 42 } });

    const backBtn = screen.getByRole('button', { name: /Back to unread/i });
    await fireEvent.click(backBtn);

    expect(mockNavigate).toHaveBeenCalledWith('/');
  });

  it('renders a two-column layout with sidebar and reader pane', () => {
    mockGetEntry.mockReturnValueOnce(new Promise(() => {}));
    const { container } = render(Reader, { props: { id: 42 } });
    expect(container.querySelector('.layout')).toBeTruthy();
    expect(container.querySelector('.reader-pane')).toBeTruthy();
  });
});
