import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import CategoryMarkReadDialog from '../CategoryMarkReadDialog.svelte';

vi.mock('../Dialog.svelte', async () => {
  const { default: DialogPassThrough } = await import('./helpers/DialogPassThrough.svelte');
  return { default: DialogPassThrough };
});

describe('CategoryMarkReadDialog', () => {
  it('renders the title with unread count', () => {
    render(CategoryMarkReadDialog, { props: { categoryName: 'People', unread: 12, feedCount: 4, onCancel: () => {}, onConfirm: () => {} } });
    expect(screen.getByText(/Mark 12 entries as read\?/)).toBeTruthy();
  });

  it('uses singular when unread = 1', () => {
    render(CategoryMarkReadDialog, { props: { categoryName: 'P', unread: 1, feedCount: 2, onCancel: () => {}, onConfirm: () => {} } });
    expect(screen.getByText(/Mark 1 entry as read\?/)).toBeTruthy();
  });

  it('renders the stats row with unread, feeds, and category', () => {
    render(CategoryMarkReadDialog, { props: { categoryName: 'People', unread: 12, feedCount: 4, onCancel: () => {}, onConfirm: () => {} } });
    expect(screen.getByText('12')).toBeTruthy();
    expect(screen.getByText('unread')).toBeTruthy();
    expect(screen.getByText('4')).toBeTruthy();
    expect(screen.getByText(/across People/i)).toBeTruthy();
  });

  it('calls onConfirm when "Mark all read" is clicked', async () => {
    const onConfirm = vi.fn();
    render(CategoryMarkReadDialog, { props: { categoryName: 'P', unread: 3, feedCount: 1, onCancel: () => {}, onConfirm } });
    await fireEvent.click(screen.getByText('Mark all read'));
    expect(onConfirm).toHaveBeenCalled();
  });
});
