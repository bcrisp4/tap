import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import CategoryReassignSheet from '../CategoryReassignSheet.svelte';

const cats = [{ id: 1, name: 'People', unread: 0, created_at: 0, position: 0 }];

describe('CategoryReassignSheet', () => {
  it('renders the grab handle, eyebrow, feed name, items, and Cancel', () => {
    render(CategoryReassignSheet, { props: { open: true, feedName: 'jvns', currentCategoryId: null, categories: cats, onSelect: () => {}, onClose: () => {} } });
    expect(screen.getByText('Move feed')).toBeTruthy();
    expect(screen.getByText('jvns')).toBeTruthy();
    expect(screen.getByText('People')).toBeTruthy();
    expect(screen.getByText('Uncategorised')).toBeTruthy();
    expect(screen.getByText('Cancel')).toBeTruthy();
  });

  it('does not render when open=false', () => {
    const { container } = render(CategoryReassignSheet, { props: { open: false, feedName: 'x', currentCategoryId: null, categories: cats, onSelect: () => {}, onClose: () => {} } });
    expect(container.querySelector('.m-cat-sheet')).toBeNull();
  });

  it('calls onClose when backdrop is clicked', async () => {
    const onClose = vi.fn();
    const { container } = render(CategoryReassignSheet, { props: { open: true, feedName: 'x', currentCategoryId: null, categories: cats, onSelect: () => {}, onClose } });
    await fireEvent.click(container.querySelector('.m-cat-sheet') as Element);
    expect(onClose).toHaveBeenCalled();
  });

  it('does NOT call onClose when card body is clicked', async () => {
    const onClose = vi.fn();
    const { container } = render(CategoryReassignSheet, { props: { open: true, feedName: 'x', currentCategoryId: null, categories: cats, onSelect: () => {}, onClose } });
    await fireEvent.click(container.querySelector('.m-cat-sheet-card') as Element);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('calls onSelect(id) then onClose when an item is tapped', async () => {
    const onSelect = vi.fn();
    const onClose = vi.fn();
    render(CategoryReassignSheet, { props: { open: true, feedName: 'x', currentCategoryId: null, categories: cats, onSelect, onClose } });
    await fireEvent.click(screen.getByText('People'));
    expect(onSelect).toHaveBeenCalledWith(1);
    expect(onClose).toHaveBeenCalled();
  });
});
