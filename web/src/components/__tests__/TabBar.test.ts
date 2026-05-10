import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { userEvent } from '@testing-library/user-event';

vi.mock('../../lib/router', () => ({
  route: { subscribe: (fn: (v: { name: string }) => void) => { fn({ name: 'unread' }); return () => {}; } },
  navigate: vi.fn(),
}));

import TabBar from '../TabBar.svelte';
import { navigate } from '../../lib/router';

describe('TabBar', () => {
  it('renders four tabs', () => {
    render(TabBar);
    expect(screen.getAllByRole('button')).toHaveLength(4);
  });

  it('marks the active tab with aria-current="page"', () => {
    render(TabBar);
    const unreadBtn = screen.getByRole('button', { name: 'Unread' });
    expect(unreadBtn.getAttribute('aria-current')).toBe('page');
  });

  it('does not mark inactive tabs with aria-current', () => {
    render(TabBar);
    const savedBtn = screen.getByRole('button', { name: 'Saved' });
    expect(savedBtn.getAttribute('aria-current')).toBeNull();
  });

  it('calls navigate with /saved when Saved tab is clicked', async () => {
    const user = userEvent.setup();
    render(TabBar);
    await user.click(screen.getByRole('button', { name: 'Saved' }));
    expect(navigate).toHaveBeenCalledWith('/saved');
  });
});
