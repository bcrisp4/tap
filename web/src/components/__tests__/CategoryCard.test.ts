import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import CategoryCard from '../CategoryCard.svelte';

vi.mock('../CategoryReassignPopover.svelte', () => ({ default: vi.fn() }));
vi.mock('../CategoryReassignSheet.svelte', () => ({ default: vi.fn() }));

const cat = { id: 1, name: 'People', unread: 5, created_at: 0, position: 0 };
const feeds = [
  { id: 10, title: 'jvns', feed_url: 'https://jvns.ca', next_poll_at: 0, error_count: 0,
    created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false,
    category_id: 1 },
];

function baseProps(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    props: {
      category: cat, feeds, unread: 5, isFirst: true, isLast: false,
      onRename: () => {}, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {},
      ...overrides,
    },
  };
}

describe('CategoryCard', () => {
  it('renders the category title and stats', () => {
    render(CategoryCard, baseProps());
    expect(screen.getByText('People')).toBeTruthy();
    expect(screen.getByText('5')).toBeTruthy();
  });

  it('clicking the title swaps to a .ts-cat-title-input and focuses it', async () => {
    const { container } = render(CategoryCard, baseProps({ isFirst: true, isLast: true }));
    await fireEvent.click(screen.getByText('People'));
    const input = container.querySelector('.ts-cat-title-input') as HTMLInputElement;
    expect(input).toBeTruthy();
    expect(input.value).toBe('People');
  });

  it('Enter on rename input calls onRename with trimmed value and exits rename mode', async () => {
    const onRename = vi.fn();
    const { container } = render(CategoryCard, baseProps({ isFirst: true, isLast: true, onRename }));
    await fireEvent.click(screen.getByText('People'));
    const input = container.querySelector('.ts-cat-title-input') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: '  Folks  ' } });
    await fireEvent.keyDown(input, { key: 'Enter' });
    expect(onRename).toHaveBeenCalledWith('Folks');
  });

  it('Esc on rename input cancels without calling onRename', async () => {
    const onRename = vi.fn();
    const { container } = render(CategoryCard, baseProps({ isFirst: true, isLast: true, onRename }));
    await fireEvent.click(screen.getByText('People'));
    const input = container.querySelector('.ts-cat-title-input') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'other' } });
    await fireEvent.keyDown(input, { key: 'Escape' });
    expect(onRename).not.toHaveBeenCalled();
    expect(container.querySelector('.ts-cat-title-input')).toBeNull();
  });

  it('blank rename value cancels instead of calling onRename', async () => {
    const onRename = vi.fn();
    const { container } = render(CategoryCard, baseProps({ isFirst: true, isLast: true, onRename }));
    await fireEvent.click(screen.getByText('People'));
    const input = container.querySelector('.ts-cat-title-input') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: '   ' } });
    await fireEvent.keyDown(input, { key: 'Enter' });
    expect(onRename).not.toHaveBeenCalled();
  });

  it('disables Reorder up when isFirst', () => {
    render(CategoryCard, baseProps({ isFirst: true, isLast: false }));
    const up = screen.getByRole('button', { name: /reorder up/i }) as HTMLButtonElement;
    expect(up.disabled).toBe(true);
  });

  it('disables Reorder down when isLast', () => {
    render(CategoryCard, baseProps({ isFirst: false, isLast: true }));
    const down = screen.getByRole('button', { name: /reorder down/i }) as HTMLButtonElement;
    expect(down.disabled).toBe(true);
  });

  it('disables Mark read when unread = 0', () => {
    render(CategoryCard, baseProps({ unread: 0, isFirst: false, isLast: false }));
    const m = screen.getByRole('button', { name: /nothing unread/i }) as HTMLButtonElement;
    expect(m.disabled).toBe(true);
  });

  it('renders nothing-but-Mark-read for the Uncategorised pseudo-card', () => {
    const uncat = { id: -1, name: 'Uncategorised', unread: 2, created_at: 0, position: 9999 };
    render(CategoryCard, { props: { category: uncat, feeds, unread: 2, isUncategorised: true,
      isFirst: false, isLast: true,
      onRename: () => {}, onDelete: () => {}, onMarkRead: () => {},
      onReorderUp: () => {}, onReorderDown: () => {}, onReassignFeed: () => {} } });
    expect(screen.queryByRole('button', { name: /^rename$/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /^delete$/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /reorder/i })).toBeNull();
    expect(screen.getByRole('button', { name: /mark/i })).toBeTruthy();
  });

  it('keyboard "R" on a focused card opens rename', async () => {
    const { container } = render(CategoryCard, baseProps({ isFirst: true, isLast: true }));
    const card = container.querySelector('.ts-cat') as HTMLElement;
    card.focus();
    await fireEvent.keyDown(card, { key: 'r' });
    expect(container.querySelector('.ts-cat-title-input')).toBeTruthy();
  });
});
