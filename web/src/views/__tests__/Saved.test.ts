import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import { api } from '../../lib/api';

vi.mock('../../components/Sidebar.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/TopBar.svelte', () => ({ default: vi.fn() }));

// EntryRow mock renders the title text so tests can find the entry.
vi.mock('../../components/EntryRow.svelte', () => ({
  default: vi.fn().mockImplementation(({ entry }: { entry: { title: string } }) => {
    const el = document.createElement('span');
    el.textContent = entry?.title ?? '';
    return el;
  }),
}));

const { default: Saved } = await import('../Saved.svelte');

describe('Saved view', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders entries returned by api.listEntries({ saved: true })', async () => {
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({
      data: [
        {
          id: 1, subscription_id: 1, title: 'A saved post', url: 'https://x/1',
          published_at: 1, fetched_at: 1, read: false, saved: true, extract_failed: false,
        },
      ],
      cursor: undefined,
    } as any);

    const { container } = render(Saved);
    await waitFor(() => expect(container.querySelector('ul[role="list"]')).toBeTruthy());
    const items = container.querySelectorAll('li[role="listitem"]');
    expect(items).toHaveLength(1);
  });

  it('renders the empty state when there are no saved entries', async () => {
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({ data: [], cursor: undefined } as any);
    render(Saved);
    expect(await screen.findByText('No saved entries yet.')).toBeTruthy();
  });

  it('calls api.listEntries with saved: true', async () => {
    const spy = vi.spyOn(api, 'listEntries').mockResolvedValueOnce({ data: [], cursor: undefined } as any);
    render(Saved);
    await waitFor(() => expect(spy).toHaveBeenCalledWith(expect.objectContaining({ saved: true })));
  });
});
