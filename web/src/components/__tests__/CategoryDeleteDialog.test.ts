import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import CategoryDeleteDialog from '../CategoryDeleteDialog.svelte';

vi.mock('../Dialog.svelte', async () => {
  const { default: DialogPassThrough } = await import('./helpers/DialogPassThrough.svelte');
  return { default: DialogPassThrough };
});

const feeds = (n: number) => Array.from({ length: n }, (_, i) => ({
  id: i + 1, title: `Feed ${i+1}`, feed_url: `https://feed${i+1}.com`,
  next_poll_at: 0, error_count: 0, created_at: 0, extract: false, extract_selector: '',
  has_cookie: false, has_basic_auth: false, category_id: 1,
}));

describe('CategoryDeleteDialog', () => {
  it('renders the title with the category name', () => {
    render(CategoryDeleteDialog, { props: { categoryName: 'People', affectedFeeds: feeds(2), onCancel: () => {}, onConfirm: () => {} } });
    expect(screen.getByText(/Delete "People"\?/)).toBeTruthy();
  });

  it('shows "no feeds affected" copy when feed list is empty', () => {
    render(CategoryDeleteDialog, { props: { categoryName: 'People', affectedFeeds: [], onCancel: () => {}, onConfirm: () => {} } });
    expect(screen.getByText(/No feeds are assigned/i)).toBeTruthy();
  });

  it('shows up to 5 feeds + a "+ N more" line when over 5', () => {
    render(CategoryDeleteDialog, { props: { categoryName: 'P', affectedFeeds: feeds(7), onCancel: () => {}, onConfirm: () => {} } });
    expect(screen.getByText('Feed 1')).toBeTruthy();
    expect(screen.getByText('Feed 5')).toBeTruthy();
    expect(screen.queryByText('Feed 6')).toBeNull();
    expect(screen.getByText('+ 2 more')).toBeTruthy();
  });

  it('calls onConfirm when "Delete category" is clicked', async () => {
    const onConfirm = vi.fn();
    render(CategoryDeleteDialog, { props: { categoryName: 'P', affectedFeeds: feeds(1), onCancel: () => {}, onConfirm } });
    await fireEvent.click(screen.getByText('Delete category'));
    expect(onConfirm).toHaveBeenCalled();
  });

  it('calls onCancel when Cancel is clicked', async () => {
    const onCancel = vi.fn();
    render(CategoryDeleteDialog, { props: { categoryName: 'P', affectedFeeds: feeds(1), onCancel, onConfirm: () => {} } });
    await fireEvent.click(screen.getByText('Cancel'));
    expect(onCancel).toHaveBeenCalled();
  });
});
