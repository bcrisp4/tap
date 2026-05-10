import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import TopBar from '../TopBar.svelte';

describe('TopBar accessibility', () => {
  it('refresh button has an aria-label', () => {
    render(TopBar, { props: { title: 'Unread', countShown: 5, countTotal: 10, onRefresh: () => {} } });
    const refreshBtn = screen.getByRole('button', { name: /refresh/i });
    expect(refreshBtn.getAttribute('aria-label')).toBeTruthy();
  });
});
