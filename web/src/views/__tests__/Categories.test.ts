import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const seedCats = [
  { id: 1, name: 'People', unread: 5, created_at: 0, position: 0 },
  { id: 2, name: 'Systems', unread: 3, created_at: 0, position: 1 },
];
const seedSubs = [
  { id: 10, title: 'jvns', feed_url: 'https://jvns.ca', next_poll_at: 0, error_count: 0,
    created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false,
    category_id: 1 },
  { id: 11, title: 'orphan', feed_url: 'https://x', next_poll_at: 0, error_count: 0,
    created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false,
    category_id: null },
];
const seedEntries = {
  items: [
    { id: 100, subscription_id: 10, title: 'a', url: '', published_at: 0, fetched_at: 0, read: false, saved: false, extract_failed: false },
    { id: 101, subscription_id: 11, title: 'b', url: '', published_at: 0, fetched_at: 0, read: false, saved: false, extract_failed: false },
    { id: 102, subscription_id: 11, title: 'c', url: '', published_at: 0, fetched_at: 0, read: false, saved: false, extract_failed: false },
  ],
  loading: false,
  error: null,
};

let catsValue = seedCats;
let subsValue = seedSubs;
let entriesValue = seedEntries;

vi.mock('../../lib/store', () => ({
  categories: {
    subscribe: (fn: (v: typeof catsValue) => void) => { fn(catsValue); return () => {}; },
    load: vi.fn(),
    create: vi.fn(),
    rename: vi.fn(),
    remove: vi.fn(),
    reorder: vi.fn(),
    reassignSubscription: vi.fn(),
    markRead: vi.fn(),
  },
  subscriptions: {
    subscribe: (fn: (v: typeof subsValue) => void) => { fn(subsValue); return () => {}; },
    load: vi.fn(),
  },
  entries: {
    subscribe: (fn: (v: typeof entriesValue) => void) => { fn(entriesValue); return () => {}; },
    load: vi.fn(),
  },
}));

vi.mock('../../components/AppShell.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/TopTabs.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/CategoryDeleteDialog.svelte', async () => {
  const { default: DialogPassThrough } = await import('../../components/__tests__/helpers/DialogPassThrough.svelte');
  return { default: DialogPassThrough };
});
vi.mock('../../components/CategoryMarkReadDialog.svelte', async () => {
  const { default: DialogPassThrough } = await import('../../components/__tests__/helpers/DialogPassThrough.svelte');
  return { default: DialogPassThrough };
});

const { default: Categories } = await import('../Categories.svelte');
const { categories } = await import('../../lib/store');

beforeEach(() => {
  vi.clearAllMocks();
  catsValue = seedCats;
  subsValue = seedSubs;
  entriesValue = seedEntries;
});

describe('Categories view', () => {
  it('renders a card for each category', () => {
    const { container } = render(Categories);
    expect(container.querySelectorAll('.ts-cat:not(.is-uncat)')).toHaveLength(2);
  });

  it('renders the Uncategorised pseudo-card when uncategorised feeds exist', () => {
    const { container } = render(Categories);
    expect(container.querySelector('.ts-cat.is-uncat')).toBeTruthy();
    expect(screen.getAllByText('Uncategorised').length).toBeGreaterThan(0);
  });

  it('does not render the Uncategorised card when all feeds are categorised', () => {
    subsValue = [seedSubs[0]];
    const { container } = render(Categories);
    expect(container.querySelector('.ts-cat.is-uncat')).toBeNull();
  });

  it('renders the empty state when there are zero categories', () => {
    catsValue = [];
    const { container } = render(Categories);
    expect(container.querySelector('.ts-cats-empty')).toBeTruthy();
    expect(screen.getByText(/No categories yet/i)).toBeTruthy();
  });

  it('clicking "+ New category" reveals the inline create input', async () => {
    render(Categories);
    await fireEvent.click(screen.getByRole('button', { name: /new category/i }));
    expect(screen.getByPlaceholderText(/name the category/i)).toBeTruthy();
  });

  it('Enter on the create input calls categories.create with the trimmed value', async () => {
    const createSpy = vi.mocked(categories.create).mockResolvedValue({ id: 99, name: 'New', unread: 0, created_at: 0, position: 2 });
    render(Categories);
    await fireEvent.click(screen.getByRole('button', { name: /new category/i }));
    const input = screen.getByPlaceholderText(/name the category/i) as HTMLInputElement;
    await fireEvent.input(input, { target: { value: '  Letters  ' } });
    await fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(createSpy).toHaveBeenCalledWith('Letters'));
  });

  it('Esc on the create input cancels without calling categories.create', async () => {
    const createSpy = vi.mocked(categories.create);
    render(Categories);
    await fireEvent.click(screen.getByRole('button', { name: /new category/i }));
    const input = screen.getByPlaceholderText(/name the category/i) as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'something' } });
    await fireEvent.keyDown(input, { key: 'Escape' });
    expect(createSpy).not.toHaveBeenCalled();
  });
});
