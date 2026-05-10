import { writable } from 'svelte/store';
import type { User, SessionResponse, PasswordChangeResponse } from './types';

type State = {
  user: User | null;
  csrfToken: string | null;
  bootstrapped: boolean;
};

const internal = writable<State>({ user: null, csrfToken: null, bootstrapped: false });

const BASE = '/api/v1';

async function jsonOr401<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    throw new Error('unauthorized');
  }
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body?.error?.message) message = body.error.message;
    } catch {
      /* swallow */
    }
    throw new Error(message);
  }
  return res.json() as Promise<T>;
}

export const auth = {
  subscribe: internal.subscribe,

  async bootstrap(): Promise<void> {
    try {
      const res = await fetch(BASE + '/sessions/current', {
        headers: { 'Content-Type': 'application/json' },
      });
      if (res.status === 401) {
        internal.set({ user: null, csrfToken: null, bootstrapped: true });
        return;
      }
      const body: SessionResponse = await jsonOr401(res);
      internal.set({ user: body.user, csrfToken: body.csrf_token, bootstrapped: true });
    } catch {
      internal.set({ user: null, csrfToken: null, bootstrapped: true });
    }
  },

  async login(username: string, password: string): Promise<void> {
    const res = await fetch(BASE + '/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    const body: SessionResponse = await jsonOr401(res);
    internal.set({ user: body.user, csrfToken: body.csrf_token, bootstrapped: true });
  },

  async logout(): Promise<void> {
    // Pull the current csrf token without subscribing.
    let csrfToken: string | null = null;
    internal.update(s => { csrfToken = s.csrfToken; return s; });
    try {
      await fetch(BASE + '/sessions/current', {
        method: 'DELETE',
        headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
      });
    } catch {
      /* network errors during logout don't matter — clear local state regardless */
    }
    internal.set({ user: null, csrfToken: null, bootstrapped: true });
  },

  /**
   * setCSRFToken updates the in-memory CSRF token. Called by the API client
   * after PATCH /me/password returns a freshly rotated token.
   */
  setCSRFToken(token: string): void {
    internal.update(s => ({ ...s, csrfToken: token }));
  },

  /**
   * clearOn401 wipes auth state without making a network call. Used by the
   * API client when any request returns 401 (e.g. session expired) so the
   * SPA reactively renders the login screen.
   */
  clearOn401(): void {
    internal.set({ user: null, csrfToken: null, bootstrapped: true });
  },
};

// Suppress an unused-import warning while PasswordChangeResponse is referenced
// only in api.ts.
const _typeAnchor: PasswordChangeResponse | null = null;
void _typeAnchor;
