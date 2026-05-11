import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import FeedsToolbar from '../FeedsToolbar.svelte';

const baseProps = {
  search: '', filter: 'all' as const, sort: 'name' as const,
  counts: { all: 0, errors: 0, unread: 0, stale: 0 },
  refreshingAll: false,
  onSearch: () => {}, onFilter: () => {}, onSort: () => {},
  onAdd: () => {}, onRefreshAll: () => {}, onImport: () => {}, onExport: () => {},
};

describe('FeedsToolbar row 1', () => {
  it('renders search input and Add feed button', () => {
    render(FeedsToolbar, { props: baseProps });
    expect(screen.getByPlaceholderText(/search feeds by name or url/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add feed/i })).toBeInTheDocument();
  });

  it('emits onSearch on every keystroke', async () => {
    const onSearch = vi.fn();
    render(FeedsToolbar, { props: { ...baseProps, onSearch } });
    await fireEvent.input(screen.getByPlaceholderText(/search feeds/i), { target: { value: 'jvns' } });
    expect(onSearch).toHaveBeenLastCalledWith('jvns');
  });

  it('emits onAdd when the Add feed button is clicked', async () => {
    const onAdd = vi.fn();
    render(FeedsToolbar, { props: { ...baseProps, onAdd } });
    await fireEvent.click(screen.getByRole('button', { name: /add feed/i }));
    expect(onAdd).toHaveBeenCalledOnce();
  });
});

describe('FeedsToolbar row 2 — filter chips', () => {
  it('renders 4 filter chips with counts', () => {
    const { container } = render(FeedsToolbar, { props: { ...baseProps, counts: { all: 12, errors: 1, unread: 7, stale: 2 } } });
    (['all', 'errors', 'unread', 'stale'] as const).forEach((key) => {
      expect(container.querySelector(`button[data-filter-key="${key}"]`)).toBeInTheDocument();
    });
    const cts = Array.from(container.querySelectorAll('.ct')).map(el => el.textContent);
    expect(cts).toContain('12');
    expect(cts).toContain('1');
    expect(cts).toContain('7');
    expect(cts).toContain('2');
  });

  it('marks the active filter chip with is-active', () => {
    const { container } = render(FeedsToolbar, { props: { ...baseProps, filter: 'errors' as const } });
    const errorsChip = container.querySelector('button[data-filter-key="errors"]')!;
    expect(errorsChip.classList.contains('is-active')).toBe(true);
  });

  it('Errors chip carries is-warn when its count is > 0', () => {
    const { container } = render(FeedsToolbar, { props: { ...baseProps, counts: { ...baseProps.counts, errors: 3 } } });
    const errorsChip = container.querySelector('button[data-filter-key="errors"]')!;
    expect(errorsChip.classList.contains('is-warn')).toBe(true);
  });

  it('emits onFilter when a chip is clicked', async () => {
    const onFilter = vi.fn();
    render(FeedsToolbar, { props: { ...baseProps, onFilter } });
    await fireEvent.click(screen.getByRole('button', { name: /unread/i }));
    expect(onFilter).toHaveBeenLastCalledWith('unread');
  });
});

describe('FeedsToolbar row 2 — sort select', () => {
  it('renders a sort select with four options', () => {
    render(FeedsToolbar, { props: baseProps });
    const select = screen.getByRole('combobox', { name: /sort/i });
    expect(select).toBeInTheDocument();
    ['Recently active', 'Name', 'Added', 'Most unread'].forEach((label) => {
      expect(screen.getByRole('option', { name: new RegExp(label, 'i') })).toBeInTheDocument();
    });
  });

  it('emits onSort when the select changes', async () => {
    const onSort = vi.fn();
    render(FeedsToolbar, { props: { ...baseProps, onSort } });
    const select = screen.getByRole('combobox', { name: /sort/i });
    await fireEvent.change(select, { target: { value: 'unread' } });
    expect(onSort).toHaveBeenLastCalledWith('unread');
  });
});

describe('FeedsToolbar row 2 — utility buttons', () => {
  it('renders Refresh all, Import OPML, and Export buttons', () => {
    render(FeedsToolbar, { props: baseProps });
    expect(screen.getByRole('button', { name: /refresh all/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /import opml/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /export/i })).toBeInTheDocument();
  });

  it('Refresh all button gets is-spinning when refreshingAll is true', () => {
    const { container } = render(FeedsToolbar, { props: { ...baseProps, refreshingAll: true } });
    const btn = container.querySelector('button[data-action="refresh-all"]')!;
    expect(btn.classList.contains('is-spinning')).toBe(true);
  });

  it('emits onRefreshAll / onImport / onExport when clicked', async () => {
    const onRefreshAll = vi.fn(), onImport = vi.fn(), onExport = vi.fn();
    render(FeedsToolbar, { props: { ...baseProps, onRefreshAll, onImport, onExport } });
    await fireEvent.click(screen.getByRole('button', { name: /refresh all/i }));
    await fireEvent.click(screen.getByRole('button', { name: /import opml/i }));
    await fireEvent.click(screen.getByRole('button', { name: /export/i }));
    expect(onRefreshAll).toHaveBeenCalledOnce();
    expect(onImport).toHaveBeenCalledOnce();
    expect(onExport).toHaveBeenCalledOnce();
  });
});
