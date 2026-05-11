import { get, writable } from 'svelte/store';
import type { EntryListItem, Subscription, Category } from './types';
import { api } from './api';
import { notifySW } from './auth';

function entriesStore() {
  const { subscribe, update, set } = writable<{
    items: EntryListItem[];
    loading: boolean;
    error: string | null;
  }>({ items: [], loading: false, error: null });

  return {
    subscribe,
    async load(unreadOnly = true) {
      set({ items: [], loading: true, error: null });
      try {
        const r = await api.listEntries({ unread: unreadOnly, limit: 100 });
        set({ items: r.data, loading: false, error: null });
      } catch (e) {
        set({ items: [], loading: false, error: (e as Error).message });
      }
    },
    async loadSaved() {
      set({ items: [], loading: true, error: null });
      try {
        const r = await api.listEntries({ saved: true, limit: 100 });
        set({ items: r.data, loading: false, error: null });
      } catch (e) {
        set({ items: [], loading: false, error: (e as Error).message });
      }
    },
    async toggleRead(id: number, read: boolean) {
      let prev: boolean | null = null;
      update(s => {
        const idx = s.items.findIndex(e => e.id === id);
        if (idx >= 0) {
          prev = s.items[idx].read;
          s.items[idx] = { ...s.items[idx], read };
        }
        return s;
      });
      try {
        await api.patchEntry(id, { read });
        notifySW({ type: 'invalidate', paths: ['/api/v1/entries'] });
      } catch (e) {
        // Roll back only if we recorded a previous value (i.e. entry was in store).
        if (prev !== null) {
          update(s => {
            const idx = s.items.findIndex(e => e.id === id);
            if (idx >= 0) s.items[idx] = { ...s.items[idx], read: prev! };
            return s;
          });
        }
        throw e;
      }
    },
    async toggleSaved(id: number, saved: boolean) {
      let prev: boolean | null = null;
      update(s => {
        const idx = s.items.findIndex(e => e.id === id);
        if (idx >= 0) {
          prev = s.items[idx].saved;
          s.items[idx] = { ...s.items[idx], saved };
        }
        return s;
      });
      try {
        await api.patchEntry(id, { saved });
        notifySW({ type: 'invalidate', paths: ['/api/v1/entries'] });
      } catch (e) {
        if (prev !== null) {
          update(s => {
            const idx = s.items.findIndex(e => e.id === id);
            if (idx >= 0) s.items[idx] = { ...s.items[idx], saved: prev! };
            return s;
          });
        }
        throw e;
      }
    },
  };
}

export const entries = entriesStore();

function subscriptionsStore() {
  const { subscribe, set } = writable<Subscription[]>([]);
  return {
    subscribe,
    async load() {
      // Don't propagate the rejection: this is called from onMount with no
      // awaiter, so an unhandled rejection would crash the page. Sidebar
      // simply renders an empty feed list on failure; the developer sees
      // the cause in console.
      try {
        set(await api.listSubscriptions());
      } catch (e) {
        console.error('subscriptions.load failed:', e);
      }
    },
    async add(feed_url: string) {
      await api.addSubscription({ feed_url });
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions'] });
      await this.load();
    },
    async refresh(id: number) {
      await api.refreshSubscription(id);
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/entries'] });
      await this.load();
    },
    async remove(id: number) {
      await api.deleteSubscription(id);
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/entries'] });
      await this.load();
    },
    async setCategory(id: number, categoryId: number | null) {
      await api.updateSubscription(id, { category_id: categoryId });
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/categories'] });
      await Promise.all([this.load(), categories.load()]);
    },
    // Bulk helpers: call each API once, then invalidate + reload once at the end.
    // This avoids N store reloads and N SW notifications for N-item bulk ops.
    async refreshMany(ids: number[]): Promise<PromiseSettledResult<unknown>[]> {
      const results = await Promise.allSettled(ids.map((id) => api.refreshSubscription(id)));
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/entries'] });
      await this.load();
      return results;
    },
    async removeMany(ids: number[]): Promise<PromiseSettledResult<unknown>[]> {
      const results = await Promise.allSettled(ids.map((id) => api.deleteSubscription(id)));
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/entries'] });
      await this.load();
      return results;
    },
    async setCategoryMany(ids: number[], categoryId: number | null): Promise<PromiseSettledResult<unknown>[]> {
      const results = await Promise.allSettled(
        ids.map((id) => api.updateSubscription(id, { category_id: categoryId })),
      );
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/categories'] });
      await Promise.all([this.load(), categories.load()]);
      return results;
    },
  };
}

export const subscriptions = subscriptionsStore();

function reorderInMemory(cs: Category[], orderedIds: number[]): Category[] {
  const byId = new Map(cs.map(c => [c.id, c]));
  const out: Category[] = [];
  for (const id of orderedIds) {
    const c = byId.get(id);
    if (c) out.push({ ...c, position: out.length });
  }
  return out;
}

function categoriesStore() {
  const { subscribe, set, update } = writable<Category[]>([]);
  return {
    subscribe,
    async load() {
      try {
        set(await api.listCategories());
      } catch (e) {
        console.error('categories.load failed:', e);
      }
    },
    async create(name: string) {
      const c = await api.createCategory(name);
      await this.load();
      return c;
    },
    async rename(id: number, name: string) {
      await api.renameCategory(id, name);
      await this.load();
    },
    async remove(id: number) {
      await api.deleteCategory(id);
      await Promise.all([this.load(), subscriptions.load()]);
    },
    async reorder(orderedIds: number[]) {
      let snapshot: Category[] = [];
      update(cs => { snapshot = cs.slice(); return reorderInMemory(cs, orderedIds); });
      try {
        await api.reorderCategories(orderedIds);
      } catch (e) {
        set(snapshot);
        throw e;
      }
      await this.load();
    },
    async reassignSubscription(subId: number, categoryId: number | null) {
      await api.patchSubscription(subId, { category_id: categoryId });
      await Promise.all([subscriptions.load(), this.load()]);
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/categories'] });
    },
    async markRead(categoryId: number | null) {
      if (categoryId !== null) {
        await api.markCategoryRead(categoryId);
      } else {
        const uncatIds = get(subscriptions).filter(s => s.category_id == null).map(s => s.id);
        await Promise.all(uncatIds.map(id => api.markSubscriptionRead(id)));
      }
      await Promise.all([entries.load(), this.load()]);
      notifySW({ type: 'invalidate', paths: ['/api/v1/entries', '/api/v1/categories'] });
    },
  };
}

export const categories = categoriesStore();
