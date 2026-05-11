import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import FeedsBulkBar from '../FeedsBulkBar.svelte';

const noop = () => {};

describe('FeedsBulkBar', () => {
  it('renders "N selected" and Clear / Refresh / Set category / Delete', () => {
    render(FeedsBulkBar, { props: { count: 3, onClear: noop, onRefresh: noop, onReassign: noop, onDelete: noop } });
    expect(screen.getByText('3 selected')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /clear/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /set category/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
  });

  it('emits each handler when its button is clicked', async () => {
    const onClear = vi.fn(), onRefresh = vi.fn(), onReassign = vi.fn(), onDelete = vi.fn();
    render(FeedsBulkBar, { props: { count: 2, onClear, onRefresh, onReassign, onDelete } });
    await fireEvent.click(screen.getByRole('button', { name: /clear/i }));
    await fireEvent.click(screen.getByRole('button', { name: /refresh/i }));
    const setCatBtn = screen.getByRole('button', { name: /set category/i });
    await fireEvent.click(setCatBtn);
    await fireEvent.click(screen.getByRole('button', { name: /delete/i }));
    expect(onClear).toHaveBeenCalledOnce();
    expect(onRefresh).toHaveBeenCalledOnce();
    expect(onReassign).toHaveBeenCalledWith(setCatBtn);
    expect(onDelete).toHaveBeenCalledOnce();
  });
});
