import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

import PassThrough from './helpers/PopoverPassThrough.svelte';
import CategoryReassignPopover from '../CategoryReassignPopover.svelte';

// Mock M1's Popover primitive to a transparent pass-through so the test
// exercises this component's content + callbacks, not the primitive's
// positioning math.
vi.mock('../Popover.svelte', () => ({ default: PassThrough }));

const cats = [
  { id: 1, name: 'People', unread: 0, created_at: 0, position: 0 },
  { id: 2, name: 'Systems', unread: 0, created_at: 0, position: 1 },
];

function baseProps(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    props: {
      open: true,
      anchor: document.createElement('div'),
      feedName: 'jvns',
      currentCategoryId: 1 as number | null,
      categories: cats,
      onPick: () => {},
      onClose: () => {},
      ...overrides,
    },
  };
}

describe('CategoryReassignPopover', () => {
  it('renders every category plus Uncategorised when open', () => {
    render(CategoryReassignPopover, baseProps());
    expect(screen.getByText('People')).toBeTruthy();
    expect(screen.getByText('Systems')).toBeTruthy();
    expect(screen.getByText('Uncategorised')).toBeTruthy();
  });

  it('does not render content when open=false', () => {
    const { container } = render(CategoryReassignPopover, baseProps({ open: false }));
    expect(container.querySelector('.ts-cat-pop')).toBeNull();
  });

  it('marks the current category with .is-current', () => {
    const { container } = render(CategoryReassignPopover, baseProps({ currentCategoryId: 1 }));
    const current = container.querySelector('.ts-cat-pop-item.is-current');
    expect(current?.textContent).toContain('People');
  });

  it('marks Uncategorised current when currentCategoryId is null', () => {
    const { container } = render(CategoryReassignPopover, baseProps({ currentCategoryId: null }));
    const current = container.querySelector('.ts-cat-pop-item.is-current');
    expect(current?.textContent).toContain('Uncategorised');
  });

  it('calls onPick(id) when an item is clicked', async () => {
    const onPick = vi.fn();
    render(CategoryReassignPopover, baseProps({ onPick }));
    await fireEvent.click(screen.getByText('Systems'));
    expect(onPick).toHaveBeenCalledWith(2);
  });

  it('calls onPick(null) when Uncategorised is clicked', async () => {
    const onPick = vi.fn();
    render(CategoryReassignPopover, baseProps({ onPick }));
    await fireEvent.click(screen.getByText('Uncategorised'));
    expect(onPick).toHaveBeenCalledWith(null);
  });

  it('renders the eyebrow with the "Move" label by default', () => {
    render(CategoryReassignPopover, baseProps());
    expect(screen.getByText(/Move/)).toBeTruthy();
    expect(screen.getByText('jvns')).toBeTruthy();
  });

  it('renders the eyebrow with the "Assign" label when passed', () => {
    render(CategoryReassignPopover, baseProps({ label: 'Assign' }));
    expect(screen.getByText(/Assign/)).toBeTruthy();
  });
});
