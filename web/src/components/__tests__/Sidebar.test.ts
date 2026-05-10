import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';

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
