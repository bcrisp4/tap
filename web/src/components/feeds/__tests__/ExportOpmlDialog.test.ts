import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../lib/api', () => ({
  api: { exportOPML: vi.fn() },
}));

vi.mock('../../../lib/auth', () => ({
  auth: { subscribe: vi.fn(() => () => {}), clearOn401: vi.fn(), setCSRFToken: vi.fn() },
  notifySW: vi.fn(),
  ERR_UNAUTHORIZED: 'unauthorized',
}));

vi.mock('../../../lib/offlineQueue', () => ({
  offlineQueue: { enqueue: vi.fn(), drain: vi.fn(), clearForUser: vi.fn() },
}));

import ExportOpmlDialog from '../ExportOpmlDialog.svelte';
import { api } from '../../../lib/api';

const noop = () => {};

beforeEach(() => { vi.clearAllMocks(); });

describe('ExportOpmlDialog', () => {
  it('renders stats and a Download button', () => {
    render(ExportOpmlDialog, { props: { feedCount: 24, categoryCount: 4, onClose: noop } });
    expect(screen.getByText(/24/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /download/i })).toBeInTheDocument();
  });

  it('Download calls api.exportOPML, builds a blob URL, triggers click, revokes URL', async () => {
    const blob = new Blob(['<opml/>'], { type: 'text/x-opml' });
    vi.mocked(api.exportOPML).mockResolvedValue(blob);
    const createObjectURL = vi.fn().mockReturnValue('blob:fake');
    const revokeObjectURL = vi.fn();
    vi.stubGlobal('URL', { ...URL, createObjectURL, revokeObjectURL });
    const onClose = vi.fn();
    render(ExportOpmlDialog, { props: { feedCount: 1, categoryCount: 1, onClose } });
    await fireEvent.click(screen.getByRole('button', { name: /download/i }));
    expect(api.exportOPML).toHaveBeenCalled();
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    await waitFor(() => expect(revokeObjectURL).toHaveBeenCalledWith('blob:fake'));
    expect(onClose).toHaveBeenCalledOnce();
  });
});
