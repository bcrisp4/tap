import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import TopBar from '../TopBar.svelte';

describe('TopBar accessibility', () => {
  it('refresh button has an aria-label', () => {
    render(TopBar, { props: { title: 'Unread', countShown: 5, countTotal: 10, onRefresh: () => {} } });
    const refreshBtn = screen.getByRole('button', { name: /refresh/i });
    expect(refreshBtn.getAttribute('aria-label')).toBeTruthy();
  });
});

describe('TopBar actions', () => {
  it('fires onMarkAllRead when the mark-all-read button is clicked', async () => {
    const onMarkAllRead = vi.fn();
    render(TopBar, { props: { title: 'Unread', countShown: 5, countTotal: 10, onMarkAllRead } });
    await fireEvent.click(screen.getByRole('button', { name: /mark all/i }));
    expect(onMarkAllRead).toHaveBeenCalled();
  });

  it('does not render mark-all-read button when onMarkAllRead is not provided', () => {
    render(TopBar, { props: { title: 'Unread', countShown: 5, countTotal: 10 } });
    expect(screen.queryByRole('button', { name: /mark all/i })).toBeNull();
  });

  it('fires onSearch when the search button is clicked', async () => {
    const onSearch = vi.fn();
    render(TopBar, { props: { title: 'Unread', countShown: 5, countTotal: 10, onSearch } });
    await fireEvent.click(screen.getByRole('button', { name: /search/i }));
    expect(onSearch).toHaveBeenCalled();
  });
});
