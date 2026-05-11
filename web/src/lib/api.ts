import { get } from 'svelte/store';
import type {
  Subscription,
  EntryListItem,
  EntryDetail,
  ListResponse,
  ApiError,
  PasswordChangeResponse,
  Session,
  TOTPEnrolmentBegin,
  TOTPConfirmResponse,
  Passkey,
  AdminUser,
  Category,
  DiscoverResult,
  OPMLImportResult,
} from './types';
import { auth, ERR_UNAUTHORIZED } from './auth';
import { offlineQueue } from './offlineQueue';

const BASE = '/api/v1';

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase();
  const isWriteMethod = method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS';
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((init.headers ?? {}) as Record<string, string>),
  };

  const authState = get(auth);
  if (isWriteMethod && authState.csrfToken) {
    headers['X-CSRF-Token'] = authState.csrfToken;
  }

  function queueAndReturn(): T {
    if (authState.user) {
      offlineQueue.enqueue(authState.user.id, {
        method,
        path,
        body: init.body ? JSON.parse(init.body as string) : undefined,
        csrfToken: authState.csrfToken ?? '',
      });
    }
    return undefined as T;
  }

  if (isWriteMethod && !navigator.onLine) return queueAndReturn();

  let res: Response;
  try {
    res = await fetch(BASE + path, { ...init, headers });
  } catch (err) {
    if (isWriteMethod && err instanceof TypeError) return queueAndReturn();
    throw err;
  }

  if (res.status === 401) {
    let detail: ApiError | null = null;
    try { detail = await res.json(); } catch { /* swallow */ }
    if (detail?.error?.code !== 'invalid_credentials') {
      auth.clearOn401();
    }
    throw new Error(detail?.error?.message ?? ERR_UNAUTHORIZED);
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
  // --- Subscriptions ---
  listSubscriptions: () =>
    request<ListResponse<Subscription>>('/subscriptions').then(r => r.data),

  addSubscription: (body: {
    feed_url: string;
    title?: string;
    extract?: boolean;
    cookie?: string;
    basic_auth_user?: string;
    basic_auth_pass?: string;
    category_id?: number | null;
  }) =>
    request<Subscription>('/subscriptions', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  updateSubscription: (
    id: number,
    patch: {
      extract?: boolean;
      extract_selector?: string;
      cookie?: string;
      basic_auth_user?: string;
      basic_auth_pass?: string;
      category_id?: number | null;
    },
  ) =>
    request<Subscription>(`/subscriptions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),

  get patchSubscription() { return this.updateSubscription; },

  refreshSubscription: (id: number) =>
    request<Subscription>(`/subscriptions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ refresh_now: true }),
    }),

  deleteSubscription: (id: number) =>
    request<void>(`/subscriptions/${id}`, { method: 'DELETE' }),

  // --- Entries ---
  listEntries: (params: {
    unread?: boolean;
    saved?: boolean;
    feed?: number;
    category?: number;
    limit?: number;
    cursor?: string;
  } = {}) => {
    const qs = new URLSearchParams();
    if (params.unread)    qs.set('unread', '1');
    if (params.saved)     qs.set('saved', '1');
    if (params.feed)      qs.set('feed', String(params.feed));
    if (params.category)  qs.set('category', String(params.category));
    if (params.limit)     qs.set('limit', String(params.limit));
    if (params.cursor)    qs.set('cursor', params.cursor);
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

  // --- Password ---
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

  // --- Sessions (M7) ---
  listSessions: () =>
    request<Session[]>('/sessions'),

  revokeSession: (id: number) =>
    request<void>(`/sessions/${id}`, { method: 'DELETE' }),

  revokeAllOtherSessions: () =>
    request<void>('/sessions', { method: 'DELETE' }),

  // --- TOTP (M7) ---
  beginTOTPEnrolment: () =>
    request<TOTPEnrolmentBegin>('/me/totp', { method: 'POST', body: '{}' }),

  confirmTOTPEnrolment: (code: string) =>
    request<TOTPConfirmResponse>('/me/totp/confirm', {
      method: 'POST',
      body: JSON.stringify({ code }),
    }),

  disableTOTP: (body: { code?: string; recovery_code?: string }) =>
    request<void>('/me/totp', {
      method: 'DELETE',
      body: JSON.stringify(body),
    }),

  regenerateRecoveryCodes: (code: string) =>
    request<TOTPConfirmResponse>('/me/totp/recovery-codes', {
      method: 'POST',
      body: JSON.stringify({ code }),
    }),

  // --- Passkeys (M7) ---
  beginPasskeyRegistration: () =>
    request<unknown>('/me/passkeys/registration/begin', { method: 'POST', body: '{}' }),

  finishPasskeyRegistration: (attestation: unknown, label: string) =>
    request<Passkey>(`/me/passkeys/registration/finish?label=${encodeURIComponent(label)}`, {
      method: 'POST',
      body: JSON.stringify(attestation),
    }),

  listPasskeys: () =>
    request<Passkey[]>('/me/passkeys'),

  deletePasskey: (id: number) =>
    request<void>(`/me/passkeys/${id}`, { method: 'DELETE' }),

  // --- Passkey login (M7) ---
  beginPasskeyLogin: () =>
    request<{ session_id: number; options: unknown }>('/passkey-sessions/begin', {
      method: 'POST',
      body: '{}',
    }),

  finishPasskeyLogin: (sessionId: number, assertion: unknown) =>
    fetch(BASE + '/passkey-sessions/finish', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: sessionId, assertion }),
    }),

  // --- Admin (M7) ---
  listUsers: () =>
    request<AdminUser[]>('/admin/users'),

  createUser: (username: string, password: string, role: 'admin' | 'user') =>
    request<AdminUser>('/admin/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, role }),
    }),

  patchUser: (id: number, patch: { role?: string; disabled?: boolean }) =>
    request<AdminUser>(`/admin/users/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),

  resetUserPassword: (id: number) =>
    request<{ temporary_password: string }>(`/admin/users/${id}/password-reset`, {
      method: 'POST',
      body: '{}',
    }),

  disableUserTOTP: (id: number) =>
    request<void>(`/admin/users/${id}/disable-totp`, { method: 'POST', body: '{}' }),

  deleteUser: (id: number) =>
    request<void>(`/admin/users/${id}`, { method: 'DELETE' }),

  // --- Subscriptions (M9 extensions) ---
  getSubscription: (id: number) =>
    request<Subscription>(`/subscriptions/${id}`),

  // --- Categories (M9) ---
  listCategories: () =>
    request<{ data: Category[] }>('/categories').then(r => r.data),

  createCategory: (name: string) =>
    request<Category>('/categories', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  renameCategory: (id: number, name: string) =>
    request<Category>(`/categories/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ name }),
    }),

  deleteCategory: (id: number) =>
    request<void>(`/categories/${id}`, { method: 'DELETE' }),

  markCategoryRead: (id: number) =>
    request<void>(`/categories/${id}/mark-read`, { method: 'POST', body: '{}' }),

  // --- Categories M-Redesign-4 ---
  reorderCategories: (orderedIds: number[]) =>
    request<void>('/categories/reorder', {
      method: 'POST',
      body: JSON.stringify({ order: orderedIds }),
    }),

  // --- Subscriptions M-Redesign-4 (Uncategorised mark-all-read) ---
  markSubscriptionRead: (id: number) =>
    request<void>(`/subscriptions/${id}/mark-read`, { method: 'POST', body: '{}' }),

  // --- Search (M9) ---
  searchEntries: (q: string, limit = 50) => {
    const qs = new URLSearchParams({ q, limit: String(limit) });
    return request<{ data: EntryListItem[] }>(`/search?${qs}`);
  },

  // --- OPML (M9) ---
  exportOPML: () =>
    fetch('/api/v1/opml').then(async r => {
      if (!r.ok) {
        const detail: ApiError | null = await r.json().catch(() => null);
        throw new Error(detail?.error?.message ?? `${r.status} ${r.statusText}`);
      }
      return r.blob();
    }),

  importOPML: (data: ArrayBuffer) => {
    const csrf = get(auth).csrfToken ?? '';
    return fetch('/api/v1/opml', {
      method: 'POST',
      headers: { 'X-CSRF-Token': csrf },
      body: data,
    }).then(async r => {
      if (!r.ok) {
        const detail: ApiError | null = await r.json().catch(() => null);
        throw new Error(detail?.error?.message ?? `${r.status}`);
      }
      return r.json() as Promise<OPMLImportResult>;
    });
  },

  // --- Discover (M9) ---
  discoverFeeds: (url: string) =>
    request<DiscoverResult>('/discover', {
      method: 'POST',
      body: JSON.stringify({ url }),
    }),
};
