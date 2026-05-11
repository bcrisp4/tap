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

const mockUser = (role: 'admin' | 'user') =>
  ({ user: { id: 1, username: 'u', role, has_totp: false, passkey_count: 0 }, csrfToken: 'x', bootstrapped: true });

import SystemActions from '../SystemActions.svelte';

describe('SystemActions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(auth, 'subscribe').mockImplementation((cb) => {
      cb(mockUser('user'));
      return () => {};
    });
  });

  it('calls auth.logout when "Sign out" is clicked', async () => {
    const spy = vi.spyOn(auth, 'logout').mockResolvedValue(undefined);
    const { getByRole } = render(SystemActions);
    await fireEvent.click(getByRole('button', { name: /sign out/i }));
    expect(spy).toHaveBeenCalled();
  });

  it('renders Export OPML and Import OPML as buttons', () => {
    const { getByRole } = render(SystemActions);
    expect(getByRole('button', { name: /export opml/i })).toBeTruthy();
    expect(getByRole('button', { name: /import opml/i })).toBeTruthy();
  });

  it('calls api.exportOPML when Export OPML is clicked', async () => {
    const spy = vi.spyOn(api, 'exportOPML').mockResolvedValue(new Blob());
    global.URL.createObjectURL = vi.fn(() => 'blob:mock');
    global.URL.revokeObjectURL = vi.fn();
    const { getByRole } = render(SystemActions);
    await fireEvent.click(getByRole('button', { name: /export opml/i }));
    expect(spy).toHaveBeenCalled();
  });
});
