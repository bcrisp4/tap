import { writable } from 'svelte/store';
import type { User, SessionResponse } from './types';
import { offlineQueue } from './offlineQueue';
import type { SWMessage } from '../sw/workerTypes';

function notifySW(msg: SWMessage): void {
  const sw = navigator.serviceWorker;
  if (!sw) return;
  if (sw.controller) {
    sw.controller.postMessage(msg);
  } else {
    // SW not yet controlling this page (first load after install); wait for it.
    sw.ready.then(reg => reg.active?.postMessage(msg)).catch(() => {});
  }
}

export const ERR_UNAUTHORIZED = 'unauthorized';

type State = {
  user: User | null;
  csrfToken: string | null;
  bootstrapped: boolean;
};

const internal = writable<State>({ user: null, csrfToken: null, bootstrapped: false });

const BASE = '/api/v1';

async function jsonOr401<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    throw new Error(ERR_UNAUTHORIZED);
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
      notifySW({ type: 'set-user', userId: body.user.id });
    } catch {
      internal.set({ user: null, csrfToken: null, bootstrapped: true });
    }
  },

  async login(username: string, password: string): Promise<unknown> {
    const res = await fetch(BASE + '/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    const body = await jsonOr401<SessionResponse & { totp_required?: boolean; pending_token?: string }>(res);
    if (body.totp_required) {
      // Return the TOTP-required shape without setting user state.
      return body;
    }
    const session = body as SessionResponse;
    internal.set({ user: session.user, csrfToken: session.csrf_token, bootstrapped: true });
    notifySW({ type: 'set-user', userId: session.user.id });
    return body;
  },

  async loginWithTOTP(pendingToken: string, totpCode?: string, recoveryCode?: string): Promise<void> {
    const res = await fetch(BASE + '/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        pending_token: pendingToken,
        ...(totpCode ? { totp_code: totpCode } : {}),
        ...(recoveryCode ? { recovery_code: recoveryCode } : {}),
      }),
    });
    const body: SessionResponse = await jsonOr401(res);
    internal.set({ user: body.user, csrfToken: body.csrf_token, bootstrapped: true });
    notifySW({ type: 'set-user', userId: body.user.id });
  },

  async beginPasskeyLogin(): Promise<{ session_id: number; options: unknown }> {
    const res = await fetch(BASE + '/passkey-sessions/begin', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
    });
    return jsonOr401(res);
  },

  async finishPasskeyLogin(sessionId: number, assertion: unknown): Promise<void> {
    const res = await fetch(BASE + '/passkey-sessions/finish', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: sessionId, assertion }),
    });
    const body: SessionResponse = await jsonOr401(res);
    internal.set({ user: body.user, csrfToken: body.csrf_token, bootstrapped: true });
    notifySW({ type: 'set-user', userId: body.user.id });
  },

  async logout(): Promise<void> {
    let userId: number | null = null;
    let csrfToken: string | null = null;
    internal.update(s => {
      userId = s.user?.id ?? null;
      csrfToken = s.csrfToken;
      return s;
    });
    if (userId !== null) {
      offlineQueue.clearForUser(userId);
      notifySW({ type: 'logout', userId });
    }
    try {
      await fetch(BASE + '/sessions/current', {
        method: 'DELETE',
        headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
      });
    } catch {
      /* network errors during logout don't matter */
    }
    internal.set({ user: null, csrfToken: null, bootstrapped: true });
  },

  setCSRFToken(token: string): void {
    internal.update(s => ({ ...s, csrfToken: token }));
  },

  setUser(user: User): void {
    internal.update(s => ({ ...s, user }));
  },

  clearOn401(): void {
    internal.set({ user: null, csrfToken: null, bootstrapped: true });
  },
};
