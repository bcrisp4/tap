import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import SavedMobileRow from '../SavedMobileRow.svelte';
import type { EntryListItem, Subscription } from '../../lib/types';

const entry: EntryListItem = {
  id: 7, subscription_id: 3, title: 'A mobile saved entry',
  url: 'https://jvns.ca/x', author: 'Julia Evans',
  published_at: Math.floor(Date.now() / 1000) - 60 * 60 * 24,
  fetched_at: Math.floor(Date.now() / 1000),
  read: false, saved: true, extract_failed: false,
};

const feed = {
  id: 3, feed_url: 'https://jvns.ca/feed.xml', title: 'Julia Evans',
} as Subscription;

function touchEvent(name: string, x: number, y: number): TouchEvent {
  // Vitest happy-dom does not implement TouchEvent fully; mock the surface.
  const ev = new Event(name, { bubbles: true }) as unknown as TouchEvent;
  Object.defineProperty(ev, 'touches', { value: [{ clientX: x, clientY: y }] });
  Object.defineProperty(ev, 'changedTouches', { value: [{ clientX: x, clientY: y }] });
  return ev;
}

describe('SavedMobileRow', () => {
  it('fires onUnsave when the user swipes left past threshold', async () => {
    const onUnsave = vi.fn();
    const { container } = render(SavedMobileRow, {
      props: { entry, feed, onUnsave, onToggleRead: vi.fn() },
    });
    const row = container.querySelector('.row')!;
    await fireEvent(row, touchEvent('touchstart', 200, 100));
    await fireEvent(row, touchEvent('touchend', 100, 100)); // dx = -100 (left)
    expect(onUnsave).toHaveBeenCalledOnce();
  });

  it('fires onToggleRead when the user swipes right past threshold', async () => {
    const onToggleRead = vi.fn();
    const { container } = render(SavedMobileRow, {
      props: { entry, feed, onUnsave: vi.fn(), onToggleRead },
    });
    const row = container.querySelector('.row')!;
    await fireEvent(row, touchEvent('touchstart', 100, 100));
    await fireEvent(row, touchEvent('touchend', 200, 100)); // dx = +100 (right)
    expect(onToggleRead).toHaveBeenCalledOnce();
  });

  it('does not fire callbacks for sub-threshold swipes', async () => {
    const onUnsave = vi.fn(), onToggleRead = vi.fn();
    const { container } = render(SavedMobileRow, {
      props: { entry, feed, onUnsave, onToggleRead },
    });
    const row = container.querySelector('.row')!;
    await fireEvent(row, touchEvent('touchstart', 200, 100));
    await fireEvent(row, touchEvent('touchend', 195, 100)); // dx = -5, under threshold
    expect(onUnsave).not.toHaveBeenCalled();
    expect(onToggleRead).not.toHaveBeenCalled();
  });

  it('renders the title and source', () => {
    const { getByText } = render(SavedMobileRow, {
      props: { entry, feed, onUnsave: vi.fn(), onToggleRead: vi.fn() },
    });
    expect(getByText('A mobile saved entry')).toBeInTheDocument();
    expect(getByText('Julia Evans')).toBeInTheDocument();
  });
});
