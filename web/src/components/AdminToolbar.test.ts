import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import AdminToolbar from './AdminToolbar.svelte';

describe('AdminToolbar', () => {
  it('renders search input and four filter chips', () => {
    render(AdminToolbar, { props: { filter: 'all', query: '', count: 7 } });
    expect(screen.getByPlaceholderText('Filter by username')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'All' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Admins' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Users' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disabled' })).toBeInTheDocument();
  });

  it('marks the active filter chip', () => {
    render(AdminToolbar, { props: { filter: 'admins', query: '', count: 7 } });
    expect(screen.getByRole('button', { name: 'Admins' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('fires onFilter when chip clicked', async () => {
    const onFilter = vi.fn();
    render(AdminToolbar, { props: { filter: 'all', query: '', count: 7, onFilter } });
    await fireEvent.click(screen.getByRole('button', { name: 'Disabled' }));
    expect(onFilter).toHaveBeenCalledWith('disabled');
  });

  it('fires onQuery on input', async () => {
    const onQuery = vi.fn();
    render(AdminToolbar, { props: { filter: 'all', query: '', count: 7, onQuery } });
    await fireEvent.input(screen.getByPlaceholderText('Filter by username'), { target: { value: 'al' } });
    expect(onQuery).toHaveBeenCalledWith('al');
  });

  it('fires onCreate when "Create user" clicked', async () => {
    const onCreate = vi.fn();
    render(AdminToolbar, { props: { filter: 'all', query: '', count: 7, onCreate } });
    await fireEvent.click(screen.getByRole('button', { name: /create user/i }));
    expect(onCreate).toHaveBeenCalled();
  });
});
