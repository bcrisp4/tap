import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { auth } from '../../lib/auth';

vi.mock('../../lib/store', () => ({
  subscriptions: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; } },
  categories: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; }, load: vi.fn() },
}));
vi.mock('../../lib/router', () => ({
  route: { subscribe: (fn: (v: { name: string }) => void) => { fn({ name: 'unread' }); return () => {}; } },
  navigate: vi.fn(),
}));

import Sidebar from '../Sidebar.svelte';

describe('Sidebar accessibility', () => {
  it('renders a <nav> element with aria-label', () => {
    render(Sidebar);
    const nav = screen.getByRole('navigation');
    expect(nav).toBeTruthy();
    expect(nav.getAttribute('aria-label')).toBeTruthy();
  });
});

describe('Sidebar Admin link', () => {
  it('shows the Admin link for admin users', () => {
    vi.spyOn(auth, 'subscribe').mockImplementation((cb) => {
      cb({ user: { id: 1, username: 'a', role: 'admin', has_totp: false, passkey_count: 0 }, csrfToken: 'x', bootstrapped: true });
      return () => {};
    });
    render(Sidebar);
    expect(screen.getByRole('link', { name: /admin/i })).toBeTruthy();
  });

  it('hides the Admin link for non-admin users', () => {
    vi.spyOn(auth, 'subscribe').mockImplementation((cb) => {
      cb({ user: { id: 1, username: 'u', role: 'user', has_totp: false, passkey_count: 0 }, csrfToken: 'x', bootstrapped: true });
      return () => {};
    });
    render(Sidebar);
    expect(screen.queryByRole('link', { name: /admin/i })).toBeNull();
  });
});
