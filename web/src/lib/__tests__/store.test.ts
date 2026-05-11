import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import type { EntryListItem } from '../types';

// We mock the api module so tests don't make real network calls.
vi.mock('../api', () => ({
  api: {
    listEntries: vi.fn(),
    patchEntry: vi.fn(),
    listSubscriptions: vi.fn(),
    addSubscription: vi.fn(),
    listCategories: vi.fn(),
    createCategory: vi.fn(),
    renameCategory: vi.fn(),
    deleteCategory: vi.fn(),
    markCategoryRead: vi.fn(),
    reorderCategories: vi.fn(),
    markSubscriptionRead: vi.fn(),
    patchSubscription: vi.fn(),
  },
}));

// Mock auth so notifySW doesn't touch real navigator.serviceWorker.
vi.mock('../auth', () => ({
  notifySW: vi.fn(),
  auth: {
    subscribe: vi.fn(() => () => {}),
    clearOn401: vi.fn(),
    setCSRFToken: vi.fn(),
  },
  ERR_UNAUTHORIZED: 'unauthorized',
}));

function makeEntry(overrides: Partial<EntryListItem> = {}): EntryListItem {
  return {
    id: 1,
    subscription_id: 10,
    title: 'Test Entry',
    url: 'https://example.com/entry/1',
    published_at: 1700000000,
    fetched_at: 1700000001,
    read: false,
    saved: false,
    extract_failed: false,
    ...overrides,
  };
}

function getStoreValue<T>(store: { subscribe: (fn: (v: T) => void) => () => void }): T {
  let value!: T;
  const unsub = store.subscribe((v) => { value = v; });
  unsub();
  return value;
}

describe('entriesStore.load', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('sets loading=true before the request resolves, then populates items on success', async () => {
    const { api } = await import('../api');
    const entries = [makeEntry({ id: 1 }), makeEntry({ id: 2 })];
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: entries });

    const { entries: store } = await import('../store');

    const loadPromise = store.load();

    // After resolve:
    await loadPromise;
    const after = getStoreValue(store);
    expect(after.loading).toBe(false);
    expect(after.items).toEqual(entries);
    expect(after.error).toBeNull();
  });

  it('sets error and clears items on API failure', async () => {
    const { api } = await import('../api');
    vi.mocked(api.listEntries).mockRejectedValueOnce(new Error('Network error'));

    const { entries: store } = await import('../store');
    await store.load();

    const state = getStoreValue(store);
    expect(state.loading).toBe(false);
    expect(state.items).toEqual([]);
    expect(state.error).toBe('Network error');
  });
});

describe('entriesStore.toggleRead', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('optimistically marks the entry as read before the API responds', async () => {
    const { api } = await import('../api');
    const entry = makeEntry({ id: 5, read: false });
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [entry] });

    // patchEntry resolves after a tick, so we can inspect optimistic state.
    let resolvePatch!: () => void;
    vi.mocked(api.patchEntry).mockReturnValueOnce(
      new Promise<EntryListItem>((res) => { resolvePatch = () => res({ ...entry, read: true }); }),
    );

    const { entries: store } = await import('../store');
    await store.load();

    const togglePromise = store.toggleRead(5, true);

    // Optimistic update should be applied synchronously.
    const optimistic = getStoreValue(store);
    expect(optimistic.items.find((e) => e.id === 5)?.read).toBe(true);

    resolvePatch();
    await togglePromise;
  });

  it('rolls back optimistic update on API failure', async () => {
    const { api } = await import('../api');
    const entry = makeEntry({ id: 5, read: false });
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [entry] });
    vi.mocked(api.patchEntry).mockRejectedValueOnce(new Error('Server error'));

    const { entries: store } = await import('../store');
    await store.load();

    await expect(store.toggleRead(5, true)).rejects.toThrow('Server error');

    const state = getStoreValue(store);
    expect(state.items.find((e) => e.id === 5)?.read).toBe(false);
  });

  it('is a graceful no-op when the entry is not in the store (direct navigation)', async () => {
    const { api } = await import('../api');
    // Store is empty — entry 999 not loaded.
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [] });
    vi.mocked(api.patchEntry).mockResolvedValueOnce(makeEntry({ id: 999, read: true }));

    const { entries: store } = await import('../store');
    await store.load();

    // Should not throw even though entry 999 isn't in the store.
    await expect(store.toggleRead(999, true)).resolves.not.toThrow();
  });

  it('re-throws the API error after rollback so callers can handle it', async () => {
    const { api } = await import('../api');
    const entry = makeEntry({ id: 3, read: false });
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [entry] });
    vi.mocked(api.patchEntry).mockRejectedValueOnce(new Error('Patch failed'));

    const { entries: store } = await import('../store');
    await store.load();

    await expect(store.toggleRead(3, true)).rejects.toThrow('Patch failed');
  });
});

describe('subscriptionsStore', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('populates subscriptions after load()', async () => {
    const { api } = await import('../api');
    const subs = [{ id: 1, title: 'Feed A' }];
    vi.mocked(api.listSubscriptions).mockResolvedValueOnce(subs as never);

    const { subscriptions: store } = await import('../store');
    await store.load();

    let value: unknown;
    const unsub = store.subscribe((v) => { value = v; });
    unsub();
    expect(value).toEqual(subs);
  });

  it('calls listSubscriptions again after add()', async () => {
    const { api } = await import('../api');
    vi.mocked(api.addSubscription).mockResolvedValueOnce({ id: 2, title: 'New Feed' } as never);
    vi.mocked(api.listSubscriptions).mockResolvedValue([{ id: 2 }] as never);

    const { subscriptions: store } = await import('../store');
    await store.add('https://example.com/feed');

    expect(api.addSubscription).toHaveBeenCalledWith({ feed_url: 'https://example.com/feed' });
    expect(api.listSubscriptions).toHaveBeenCalled();
  });

  it('notifies SW to invalidate subscriptions cache after add()', async () => {
    const { api } = await import('../api');
    const { notifySW } = await import('../auth');
    vi.mocked(api.addSubscription).mockResolvedValueOnce({ id: 3, title: 'Feed' } as never);
    vi.mocked(api.listSubscriptions).mockResolvedValue([] as never);

    const { subscriptions: store } = await import('../store');
    await store.add('https://example.com/feed');

    expect(notifySW).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'invalidate', paths: expect.arrayContaining(['/api/v1/subscriptions']) }),
    );
  });
});

describe('entriesStore.toggleSaved', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('optimistically marks the entry as saved before the API responds', async () => {
    const { api } = await import('../api');
    const entry = makeEntry({ id: 7, saved: false });
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [entry] });

    let resolvePatch!: () => void;
    vi.mocked(api.patchEntry).mockReturnValueOnce(
      new Promise<EntryListItem>((res) => { resolvePatch = () => res({ ...entry, saved: true }); }),
    );

    const { entries: store } = await import('../store');
    await store.load();

    const togglePromise = store.toggleSaved(7, true);
    const optimistic = getStoreValue(store);
    expect(optimistic.items.find((e) => e.id === 7)?.saved).toBe(true);

    resolvePatch();
    await togglePromise;
  });

  it('rolls back optimistic update on API failure', async () => {
    const { api } = await import('../api');
    const entry = makeEntry({ id: 7, saved: false });
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [entry] });
    vi.mocked(api.patchEntry).mockRejectedValueOnce(new Error('Server error'));

    const { entries: store } = await import('../store');
    await store.load();

    await expect(store.toggleSaved(7, true)).rejects.toThrow('Server error');
    const state = getStoreValue(store);
    expect(state.items.find((e) => e.id === 7)?.saved).toBe(false);
  });

  it('notifies SW to invalidate entries cache after toggleSaved succeeds', async () => {
    const { api } = await import('../api');
    const { notifySW } = await import('../auth');
    const entry = makeEntry({ id: 7, saved: false });
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [entry] });
    vi.mocked(api.patchEntry).mockResolvedValueOnce({ ...entry, saved: true } as never);

    const { entries: store } = await import('../store');
    await store.load();
    await store.toggleSaved(7, true);

    expect(notifySW).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'invalidate', paths: expect.arrayContaining(['/api/v1/entries']) }),
    );
  });

  it('notifies SW to invalidate entries cache after toggleRead succeeds', async () => {
    const { api } = await import('../api');
    const { notifySW } = await import('../auth');
    const entry = makeEntry({ id: 5, read: false });
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [entry] });
    vi.mocked(api.patchEntry).mockResolvedValueOnce({ ...entry, read: true } as never);

    const { entries: store } = await import('../store');
    await store.load();
    await store.toggleRead(5, true);

    expect(notifySW).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'invalidate', paths: expect.arrayContaining(['/api/v1/entries']) }),
    );
  });

  it('is a graceful no-op when the entry is not in the store (direct navigation)', async () => {
    const { api } = await import('../api');
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [] });
    vi.mocked(api.patchEntry).mockResolvedValueOnce(makeEntry({ id: 999, saved: true }));

    const { entries: store } = await import('../store');
    await store.load();

    await expect(store.toggleSaved(999, true)).resolves.not.toThrow();
  });
});

describe('categories store', () => {
  beforeEach(() => {
    vi.resetModules();
  });
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('reassignSubscription PATCHes the subscription and reloads', async () => {
    const { api } = await import('../api');
    const { categories } = await import('../store');
    const patchSpy = vi.spyOn(api, 'patchSubscription').mockResolvedValueOnce(undefined as any);
    const loadSubs = vi.spyOn(api, 'listSubscriptions').mockResolvedValueOnce([] as any);
    await categories.reassignSubscription(5, 7);
    expect(patchSpy).toHaveBeenCalledWith(5, { category_id: 7 });
    expect(loadSubs).toHaveBeenCalled();
  });

  it('reorder calls api.reorderCategories with the new order and reloads', async () => {
    const { api } = await import('../api');
    const { categories } = await import('../store');
    const reorderSpy = vi.spyOn(api, 'reorderCategories').mockResolvedValueOnce(undefined as any);
    const listSpy = vi.spyOn(api, 'listCategories').mockResolvedValueOnce([] as any);
    await categories.reorder([3, 1, 2]);
    expect(reorderSpy).toHaveBeenCalledWith([3, 1, 2]);
    expect(listSpy).toHaveBeenCalled();
  });

  it('markRead(id) calls api.markCategoryRead and reloads entries', async () => {
    const { api } = await import('../api');
    const { categories } = await import('../store');
    const markSpy = vi.spyOn(api, 'markCategoryRead').mockResolvedValueOnce(undefined as any);
    const entriesSpy = vi.spyOn(api, 'listEntries').mockResolvedValueOnce({ data: [] } as any);
    vi.spyOn(api, 'listCategories').mockResolvedValueOnce([] as any);
    await categories.markRead(7);
    expect(markSpy).toHaveBeenCalledWith(7);
    expect(entriesSpy).toHaveBeenCalled();
  });

  it('markRead(null) calls api.markSubscriptionRead for each uncategorised subscription', async () => {
    const { api } = await import('../api');
    const { categories, subscriptions } = await import('../store');
    const markFeed = vi.spyOn(api, 'markSubscriptionRead').mockResolvedValue(undefined as any);
    vi.spyOn(api, 'listSubscriptions').mockResolvedValueOnce([
      { id: 1, category_id: null }, { id: 2, category_id: null }, { id: 3, category_id: 4 },
    ] as any);
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({ data: [] } as any);
    vi.spyOn(api, 'listCategories').mockResolvedValueOnce([] as any);
    await subscriptions.load();
    await categories.markRead(null);
    expect(markFeed).toHaveBeenCalledTimes(2);
    expect(markFeed).toHaveBeenCalledWith(1);
    expect(markFeed).toHaveBeenCalledWith(2);
  });

  it('reorder rolls back the in-memory order when the API rejects', async () => {
    const { api } = await import('../api');
    const { categories } = await import('../store');
    vi.spyOn(api, 'listCategories').mockResolvedValueOnce([
      { id: 1, name: 'A', unread: 0, created_at: 0, position: 0 },
      { id: 2, name: 'B', unread: 0, created_at: 0, position: 1 },
      { id: 3, name: 'C', unread: 0, created_at: 0, position: 2 },
    ] as any);
    await categories.load();

    vi.spyOn(api, 'reorderCategories').mockRejectedValueOnce(new Error('boom'));

    await expect(categories.reorder([3, 1, 2])).rejects.toThrow('boom');

    const after = get(categories);
    expect(after.map((c: any) => c.id)).toEqual([1, 2, 3]);
  });
});
