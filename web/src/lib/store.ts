import { writable } from 'svelte/store';
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
      // add() callers (AddFeedForm) await and surface errors in the UI,
      // so propagation is intentional here.
      await api.addSubscription({ feed_url });
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions'] });
      await this.load();
    },
  };
}

export const subscriptions = subscriptionsStore();

function categoriesStore() {
  const { subscribe, set } = writable<Category[]>([]);
  return {
    subscribe,
    async load() {
      try {
        set(await api.listCategories());
      } catch (e) {
        console.error('categories.load failed:', e);
      }
    },
  };
}

export const categories = categoriesStore();
