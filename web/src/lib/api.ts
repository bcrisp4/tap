import type {
  Subscription,
  EntryListItem,
  EntryDetail,
  ListResponse,
  ApiError,
} from './types';

const BASE = '/api/v1';

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json', ...(init.headers ?? {}) },
    ...init,
  });
  if (!res.ok) {
    let detail: ApiError | null = null;
    try { detail = await res.json(); } catch { /* swallow */ }
    throw new Error(detail?.error.message ?? `${res.status} ${res.statusText}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  listSubscriptions: () =>
    request<ListResponse<Subscription>>('/subscriptions').then(r => r.data),

  addSubscription: (feed_url: string) =>
    request<Subscription>('/subscriptions', {
      method: 'POST',
      body: JSON.stringify({ feed_url }),
    }),

  deleteSubscription: (id: number) =>
    request<void>(`/subscriptions/${id}`, { method: 'DELETE' }),

  listEntries: (params: {
    unread?: boolean;
    feed?: number;
    limit?: number;
    cursor?: string;
  } = {}) => {
    const qs = new URLSearchParams();
    if (params.unread) qs.set('unread', '1');
    if (params.feed)   qs.set('feed', String(params.feed));
    if (params.limit)  qs.set('limit', String(params.limit));
    if (params.cursor) qs.set('cursor', params.cursor);
    const suffix = qs.toString() ? `?${qs}` : '';
    return request<ListResponse<EntryListItem>>(`/entries${suffix}`);
  },

  getEntry: (id: number) =>
    request<EntryDetail>(`/entries/${id}`),

  patchEntry: (id: number, patch: { read?: boolean; saved?: boolean }) =>
    request<EntryListItem>(`/entries/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),
};
