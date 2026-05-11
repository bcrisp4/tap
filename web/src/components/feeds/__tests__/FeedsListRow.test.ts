import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import FeedsListRow from '../FeedsListRow.svelte';
import type { FeedRow } from '../../../lib/feedsFilter';

const feed: FeedRow = {
  id: 1, title: 'Julia Evans', feed_url: 'https://jvns.ca/atom.xml', site_url: 'https://jvns.ca',
  next_poll_at: 0, last_poll_at: Math.floor(Date.now() / 1000) - 60, error_count: 0,
  last_error: undefined, created_at: 0, extract: false, extract_selector: '',
  has_cookie: false, has_basic_auth: false,
  category_id: 3, unread: 12, lastPollAgo: 60,
};

const noop = () => {};
const props = {
  feed, categoryName: 'People', isSelected: false, anySelected: false,
  isExpanded: false, isRefreshing: false,
  onToggleSelect: noop, onToggleExpand: noop, onRefresh: noop,
  onChangeCategory: noop, onEdit: noop, onDelete: noop,
};

describe('FeedsListRow', () => {
  it('renders avatar, name, category chip, and url', () => {
    render(FeedsListRow, { props });
    expect(screen.getByText('Julia Evans')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /people/i })).toBeInTheDocument();
    expect(screen.getByText(/jvns\.ca/)).toBeInTheDocument();
  });

  it('renders an uncategorised chip with the is-uncat class when category_id is null', () => {
    const { container } = render(FeedsListRow, {
      props: { ...props, feed: { ...feed, category_id: null }, categoryName: null },
    });
    const chip = container.querySelector('.ts-feed-cat.is-uncat');
    expect(chip).not.toBeNull();
    expect(chip!.textContent).toMatch(/uncategorised/i);
  });

  it('checkbox emits onToggleSelect; selected adds is-selected class', async () => {
    const onToggleSelect = vi.fn();
    const { container, rerender } = render(FeedsListRow, { props: { ...props, onToggleSelect } });
    const cb = container.querySelector('.ts-feed-check')!;
    await fireEvent.click(cb);
    expect(onToggleSelect).toHaveBeenCalledOnce();
    await rerender({ ...props, isSelected: true });
    expect(container.querySelector('.ts-feed-row.is-selected')).not.toBeNull();
  });

  it('refresh button emits onRefresh; isRefreshing adds is-spinning', async () => {
    const onRefresh = vi.fn();
    const { container, rerender } = render(FeedsListRow, { props: { ...props, onRefresh } });
    await fireEvent.click(container.querySelector('button[data-action="refresh"]')!);
    expect(onRefresh).toHaveBeenCalledOnce();
    await rerender({ ...props, isRefreshing: true });
    expect(container.querySelector('button[data-action="refresh"]')?.classList.contains('is-spinning')).toBe(true);
  });

  it('edit button emits onEdit when clicked', async () => {
    const onEdit = vi.fn();
    render(FeedsListRow, { props: { ...props, onEdit } });
    await fireEvent.click(screen.getByRole('button', { name: /edit feed/i }));
    expect(onEdit).toHaveBeenCalledOnce();
  });

  it('shows error chip when error_count > 0', () => {
    const { container } = render(FeedsListRow, {
      props: { ...props, feed: { ...feed, error_count: 2, last_error: 'HTTP 500' } },
    });
    expect(container.querySelector('.ts-feed-err-chip')).not.toBeNull();
    expect(container.querySelector('.ts-feed-row.has-error')).not.toBeNull();
  });
});
