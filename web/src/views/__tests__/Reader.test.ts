import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import type { EntryDetail } from '../../lib/types';

// Stub IntersectionObserver for jsdom
const mockObserve = vi.fn();
const mockDisconnect = vi.fn();
vi.stubGlobal('IntersectionObserver', class {
  observe = mockObserve;
  disconnect = mockDisconnect;
  unobserve = vi.fn();
  takeRecords() { return []; }
  constructor() {}
});

vi.mock('../../components/FeedAvatar.svelte', () => ({ default: vi.fn() }));
vi.mock('../../lib/breakpoints.svelte', () => ({
  isMobile: { subscribe: (fn: (v: boolean) => void) => { fn(false); return () => {}; } },
}));

const mockToggleRead = vi.fn().mockResolvedValue(undefined);
const mockToggleSaved = vi.fn().mockResolvedValue(undefined);
vi.mock('../../lib/store', () => ({
  entries: {
    subscribe: (fn: (v: { items: unknown[] }) => void) => { fn({ items: [] }); return () => {}; },
    toggleRead: (...a: unknown[]) => mockToggleRead(...a),
    toggleSaved: (...a: unknown[]) => mockToggleSaved(...a),
  },
  subscriptions: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; }, load: vi.fn() },
}));

const mockNavigate = vi.fn();
vi.mock('../../lib/router', () => ({
  navigate: (...a: unknown[]) => mockNavigate(...a),
  route: { subscribe: (fn: (v: unknown) => void) => { fn({ name: 'reader', params: { id: 1 } }); return () => {}; } },
}));

const mockGetEntry = vi.fn();
const mockPatchEntry = vi.fn();
vi.mock('../../lib/api', () => ({
  api: {
    getEntry: (...a: unknown[]) => mockGetEntry(...a),
    patchEntry: (...a: unknown[]) => mockPatchEntry(...a),
  },
}));

const mockLoadScroll = vi.fn().mockReturnValue(0);
const mockSaveScroll = vi.fn();
vi.mock('../../lib/readerScroll', () => ({
  loadScroll: (...a: unknown[]) => mockLoadScroll(...a),
  saveScroll: (...a: unknown[]) => mockSaveScroll(...a),
  clearScroll: vi.fn(),
}));

const prefs = { measure: 'comfortable', font: 'serif', markOnScroll: true };
vi.mock('../../lib/preferences.svelte', () => ({
  measure: { get value() { return prefs.measure; }, set value(v: string) { prefs.measure = v; } },
  font:    { get value() { return prefs.font; },    set value(v: string) { prefs.font = v; } },
  markOnScroll: { get value() { return prefs.markOnScroll; }, set value(v: boolean) { prefs.markOnScroll = v; } },
  density: { get value() { return 'comfortable'; }, set value(_: string) {} },
  theme:   { get resolved() { return 'light'; }, get stored() { return 'light'; }, set stored(_: string) {} },
}));

const { default: Reader } = await import('../Reader.svelte');

function makeEntry(o: Partial<EntryDetail> = {}): EntryDetail {
  return {
    id: 42, subscription_id: 1, title: 'A title', author: 'Author',
    url: 'https://example.com/a', content: '<p>body</p>',
    published_at: 1700000000, fetched_at: 1700000001,
    read: false, saved: false, extract_failed: false,
    ...o,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  prefs.measure = 'comfortable'; prefs.font = 'serif'; prefs.markOnScroll = true;
  mockLoadScroll.mockReturnValue(0);
});

describe('Reader view (M2 ts-article anatomy)', () => {
  it('renders the ts-article structure when entry loads', async () => {
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    const { container } = render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(container.querySelector('.ts-article')).toBeTruthy());
    expect(container.querySelector('.ts-back')).toBeTruthy();
    expect(container.querySelector('.ts-article-title')).toBeTruthy();
    expect(container.querySelector('.ts-article-actions')).toBeTruthy();
    expect(container.querySelector('.ts-article-rule')).toBeTruthy();
    expect(container.querySelector('.ts-article-end')).toBeTruthy();
  });

  it('applies measure-<value> class to the shell wrapper', async () => {
    prefs.measure = 'narrow';
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    const { container } = render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(container.querySelector('.ts-shell-reader.measure-narrow')).toBeTruthy());
  });

  it('does NOT auto-mark on mount when markOnScroll preference is true', async () => {
    prefs.markOnScroll = true;
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: false }));
    render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(screen.queryByText('A title')).toBeTruthy());
    expect(mockToggleRead).not.toHaveBeenCalled();
  });

  it('auto-marks on mount when markOnScroll preference is false (legacy)', async () => {
    prefs.markOnScroll = false;
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: false }));
    render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(mockToggleRead).toHaveBeenCalledWith(42, true));
  });

  it('Mark unread button toggles read state', async () => {
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    render(Reader, { props: { id: 42 } });
    await waitFor(() => screen.getByText(/Mark unread/i));
    await fireEvent.click(screen.getByText(/Mark unread/i));
    await waitFor(() => expect(mockToggleRead).toHaveBeenCalledWith(42, false));
  });

  it('Saved button toggles saved state', async () => {
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true, saved: false }));
    render(Reader, { props: { id: 42 } });
    await waitFor(() => screen.getByText(/^Save$/i));
    await fireEvent.click(screen.getByText(/^Save$/i));
    await waitFor(() => expect(mockToggleSaved).toHaveBeenCalledWith(42, true));
  });

  it('back row navigates to / on click', async () => {
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    const { container } = render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(container.querySelector('.ts-back')).toBeTruthy());
    await fireEvent.click(container.querySelector('.ts-back')!);
    expect(mockNavigate).toHaveBeenCalledWith('/');
  });

  it('shows loading state before entry resolves', () => {
    mockGetEntry.mockReturnValueOnce(new Promise(() => {}));
    render(Reader, { props: { id: 42 } });
    expect(screen.getByText(/Loading/i)).toBeTruthy();
  });

  it('shows error when api.getEntry rejects', async () => {
    mockGetEntry.mockRejectedValueOnce(new Error('Entry not found'));
    render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(screen.getByText('Entry not found')).toBeTruthy());
  });

  it('on mount restores scrollTop from readerScroll.loadScroll', async () => {
    mockLoadScroll.mockReturnValue(880);
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    const { container } = render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(container.querySelector('.ts-article')).toBeTruthy());
    // RAF fires asynchronously; give it a tick to execute
    await new Promise(r => setTimeout(r, 0));
    expect(mockLoadScroll).toHaveBeenCalledWith(42);
  });

  it('debounced saveScroll is called after scroll with 300ms debounce', async () => {
    vi.useFakeTimers();
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    const { container } = render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(container.querySelector('[data-testid="reader-scroll"]')).toBeTruthy());
    const scrollEl = container.querySelector('[data-testid="reader-scroll"]') as HTMLElement;
    await fireEvent.scroll(scrollEl);
    vi.advanceTimersByTime(299);
    expect(mockSaveScroll).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    // saveScroll is debounced; it fires once 300ms after the last scroll event
    expect(mockSaveScroll).toHaveBeenCalledWith(42, expect.any(Number));
    vi.useRealTimers();
  });
});
