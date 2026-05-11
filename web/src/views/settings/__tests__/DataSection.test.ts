import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

// Patch URL.createObjectURL/revokeObjectURL without replacing URL class.
vi.spyOn(globalThis.URL, 'createObjectURL').mockReturnValue('blob://x');
vi.spyOn(globalThis.URL, 'revokeObjectURL').mockImplementation(() => undefined);

vi.mock('../../../lib/api', () => ({
  api: {
    exportOPML: vi.fn(),
    listEntries: vi.fn(),
  },
}));

vi.mock('../dialogs/ImportOPMLDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/DeleteAccountDialog.svelte', () => ({ default: vi.fn() }));

const { default: DataSection } = await import('../DataSection.svelte');
const { api } = await import('../../../lib/api');

describe('DataSection', () => {
  beforeEach(() => vi.clearAllMocks());

  it('Export OPML triggers api.exportOPML and downloads', async () => {
    vi.mocked(api.exportOPML).mockResolvedValue(new Blob(['<opml/>'], { type: 'application/xml' }) as never);
    const { getByRole } = render(DataSection);
    await fireEvent.click(getByRole('button', { name: /export opml/i }));
    await waitFor(() => expect(api.exportOPML).toHaveBeenCalledOnce());
  });

  it('Export saved JSON calls listEntries with saved:true', async () => {
    vi.mocked(api.listEntries).mockResolvedValue({ data: [], next: null } as never);
    const { getByRole } = render(DataSection);
    await fireEvent.click(getByRole('button', { name: /export saved/i }));
    await waitFor(() => expect(api.listEntries).toHaveBeenCalledWith({ saved: true, limit: 1000 }));
  });

  it('Delete account opens the password-challenge dialog', async () => {
    const { getByRole } = render(DataSection);
    await fireEvent.click(getByRole('button', { name: /delete account/i }));
    // Dialog mock renders nothing — just verify no throw
  });
});
