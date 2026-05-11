import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import { auth } from '../../lib/auth';
import { api } from '../../lib/api';

vi.mock('../../lib/router', () => ({
  navigate: vi.fn(),
  route: { subscribe: (fn: (v: { name: string }) => void) => { fn({ name: 'unread' }); return () => {}; } },
}));

vi.mock('../../lib/store', () => ({
  subscriptions: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; }, load: vi.fn() },
  categories: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; }, load: vi.fn() },
}));

import SystemActions from '../SystemActions.svelte';

describe('SystemActions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the Admin link when the user is an admin', () => {
    vi.spyOn(auth, 'subscribe').mockImplementation((cb) => {
      cb({ user: { id: 1, username: 'a', role: 'admin', has_totp: false, passkey_count: 0 }, csrfToken: 'x', bootstrapped: true });
      return () => {};
    });
    const { getByRole } = render(SystemActions);
    expect(getByRole('link', { name: /admin/i })).toBeTruthy();
  });

  it('hides the Admin link for non-admin users', () => {
    vi.spyOn(auth, 'subscribe').mockImplementation((cb) => {
      cb({ user: { id: 1, username: 'u', role: 'user', has_totp: false, passkey_count: 0 }, csrfToken: 'x', bootstrapped: true });
      return () => {};
    });
    const { queryByRole } = render(SystemActions);
    expect(queryByRole('link', { name: /admin/i })).toBeNull();
  });

  it('calls auth.logout when "Sign out" is clicked', async () => {
    const spy = vi.spyOn(auth, 'logout').mockResolvedValue(undefined);
    vi.spyOn(auth, 'subscribe').mockImplementation((cb) => {
      cb({ user: { id: 1, username: 'u', role: 'user', has_totp: false, passkey_count: 0 }, csrfToken: 'x', bootstrapped: true });
      return () => {};
    });
    const { getByRole } = render(SystemActions);
    await fireEvent.click(getByRole('button', { name: /sign out/i }));
    expect(spy).toHaveBeenCalled();
  });

  it('exports OPML when Export OPML is clicked', async () => {
    const spy = vi.spyOn(api, 'exportOPML').mockResolvedValue(new Blob());
    vi.spyOn(auth, 'subscribe').mockImplementation((cb) => {
      cb({ user: { id: 1, username: 'u', role: 'user', has_totp: false, passkey_count: 0 }, csrfToken: 'x', bootstrapped: true });
      return () => {};
    });
    global.URL.createObjectURL = vi.fn(() => 'blob:mock');
    global.URL.revokeObjectURL = vi.fn();
    const { getByRole } = render(SystemActions);
    await fireEvent.click(getByRole('button', { name: /export opml/i }));
    expect(spy).toHaveBeenCalled();
  });
});
