import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import SavedRow from '../SavedRow.svelte';
import type { EntryListItem, Subscription } from '../../lib/types';

const entry: EntryListItem = {
  id: 7, subscription_id: 3, title: 'Reasons bugs feel impossible',
  url: 'https://jvns.ca/x', author: 'Julia Evans',
  published_at: Math.floor(Date.now() / 1000) - 60 * 32,
  fetched_at: Math.floor(Date.now() / 1000),
  read: false, saved: true, extract_failed: false,
};

const feed = {
  id: 3, feed_url: 'https://jvns.ca/feed.xml', title: 'Julia Evans',
} as Subscription;

describe('SavedRow', () => {
  it('renders the title, source name, and author', () => {
    render(SavedRow, { props: { entry, feed } });
    expect(screen.getByText('Reasons bugs feel impossible')).toBeInTheDocument();
    expect(screen.getByText('Julia Evans')).toBeInTheDocument();
  });

  it('fires onOpen when the row body is clicked', async () => {
    const onOpen = vi.fn();
    render(SavedRow, { props: { entry, feed, onOpen } });
    await fireEvent.click(screen.getByText('Reasons bugs feel impossible'));
    expect(onOpen).toHaveBeenCalledOnce();
  });

  it('fires onToggleRead and does NOT fire onOpen when Mark read is clicked', async () => {
    const onOpen = vi.fn(), onToggleRead = vi.fn();
    render(SavedRow, { props: { entry, feed, onOpen, onToggleRead } });
    await fireEvent.click(screen.getByRole('button', { name: /mark read/i }));
    expect(onToggleRead).toHaveBeenCalledOnce();
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('fires onUnsave and does NOT fire onOpen when Unsave is clicked', async () => {
    const onOpen = vi.fn(), onUnsave = vi.fn();
    render(SavedRow, { props: { entry, feed, onOpen, onUnsave } });
    await fireEvent.click(screen.getByRole('button', { name: /unsave/i }));
    expect(onUnsave).toHaveBeenCalledOnce();
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('renders "Mark unread" for a read entry', () => {
    render(SavedRow, { props: { entry: { ...entry, read: true }, feed } });
    expect(screen.getByRole('button', { name: /mark unread/i })).toBeInTheDocument();
  });

  it('applies is-read class when entry.read is true', () => {
    const { container } = render(SavedRow, { props: { entry: { ...entry, read: true }, feed } });
    expect(container.querySelector('.row')?.classList.contains('is-read')).toBe(true);
  });

  it('fires onMouseEnter when the row is hovered', async () => {
    const onMouseEnter = vi.fn();
    const { container } = render(SavedRow, { props: { entry, feed, onMouseEnter } });
    await fireEvent.mouseEnter(container.querySelector('.row')!);
    expect(onMouseEnter).toHaveBeenCalledOnce();
  });

  it('fires onFocus when the row is focused', async () => {
    const onFocus = vi.fn();
    const { container } = render(SavedRow, { props: { entry, feed, onFocus } });
    await fireEvent.focus(container.querySelector('.row')!);
    expect(onFocus).toHaveBeenCalledOnce();
  });

  it('applies is-focused class when isFocused prop is true', () => {
    const { container } = render(SavedRow, { props: { entry, feed, isFocused: true } });
    expect(container.querySelector('.row')?.classList.contains('is-focused')).toBe(true);
  });
});
