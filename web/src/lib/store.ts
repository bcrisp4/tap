import { writable } from 'svelte/store';
import type { EntryListItem, Subscription } from './types';
import { api } from './api';

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
      // Optimistic update.
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
  };
}

export const entries = entriesStore();

function subscriptionsStore() {
  const { subscribe, set } = writable<Subscription[]>([]);
  return {
    subscribe,
    async load() {
      set(await api.listSubscriptions());
    },
    async add(feed_url: string) {
      await api.addSubscription(feed_url);
      await this.load();
    },
  };
}

export const subscriptions = subscriptionsStore();
