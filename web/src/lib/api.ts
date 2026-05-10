import { get } from 'svelte/store';
import type {
  Subscription,
  EntryListItem,
  EntryDetail,
  ListResponse,
  ApiError,
  PasswordChangeResponse,
} from './types';
import { auth, ERR_UNAUTHORIZED } from './auth';

const BASE = '/api/v1';

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((init.headers ?? {}) as Record<string, string>),
  };

  // Attach CSRF on state-changing methods. Read non-reactively from the store.
  if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
    const csrf = get(auth).csrfToken;
    if (csrf) {
      headers['X-CSRF-Token'] = csrf;
    }
  }

  const res = await fetch(BASE + path, { ...init, headers });

  if (res.status === 401) {
    auth.clearOn401();
    throw new Error(ERR_UNAUTHORIZED);
  }
  if (!res.ok) {
    let detail: ApiError | null = null;
    try { detail = await res.json(); } catch { /* swallow */ }
    throw new Error(detail?.error?.message ?? `${res.status} ${res.statusText}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  listSubscriptions: () =>
    request<ListResponse<Subscription>>('/subscriptions').then(r => r.data),

  addSubscription: (body: {
    feed_url: string;
    title?: string;
    extract?: boolean;
    cookie?: string;
    basic_auth_user?: string;
    basic_auth_pass?: string;
  }) =>
    request<Subscription>('/subscriptions', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  patchSubscription: (
    id: number,
    patch: {
      extract?: boolean;
      extract_selector?: string;
      cookie?: string;
      basic_auth_user?: string;
      basic_auth_pass?: string;
    },
  ) =>
    request<Subscription>(`/subscriptions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
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

  changePassword: async (currentPassword: string, newPassword: string) => {
    const resp = await request<PasswordChangeResponse>('/me/password', {
      method: 'PATCH',
      body: JSON.stringify({
        current_password: currentPassword,
        new_password: newPassword,
      }),
    });
    auth.setCSRFToken(resp.csrf_token);
    return resp;
  },
};
