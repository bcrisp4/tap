import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../lib/api', () => ({
  api: { importOPML: vi.fn() },
}));

vi.mock('../../../lib/auth', () => ({
  auth: { subscribe: vi.fn(() => () => {}), clearOn401: vi.fn(), setCSRFToken: vi.fn() },
  notifySW: vi.fn(),
  ERR_UNAUTHORIZED: 'unauthorized',
}));

vi.mock('../../../lib/offlineQueue', () => ({
  offlineQueue: { enqueue: vi.fn(), drain: vi.fn(), clearForUser: vi.fn() },
}));

import ImportOpmlDialog from '../ImportOpmlDialog.svelte';
import { api } from '../../../lib/api';
import type { OPMLImportResult } from '../../../lib/types';

const noop = () => {};

beforeEach(() => { vi.clearAllMocks(); });

describe('ImportOpmlDialog', () => {
  it('drop stage: renders a drop zone + browse link', () => {
    render(ImportOpmlDialog, { props: { onClose: noop, onImported: noop } });
    expect(screen.getByText(/drop an \.opml file/i)).toBeInTheDocument();
    expect(screen.getByText(/browse/i)).toBeInTheDocument();
  });

  it('selecting a file calls api.importOPML and shows result summary', async () => {
    const buf = new TextEncoder().encode('<opml/>').buffer;
    const result: OPMLImportResult = { imported: 5, skipped: 1, errors: [] };
    vi.mocked(api.importOPML).mockResolvedValue(result);
    const { container } = render(ImportOpmlDialog, { props: { onClose: noop, onImported: noop } });
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File([buf], 'subs.opml', { type: 'text/x-opml' });
    await fireEvent.change(fileInput, { target: { files: [file] } });
    await waitFor(() => expect(api.importOPML).toHaveBeenCalled());
    expect(await screen.findByText(/5 imported/i)).toBeInTheDocument();
  });

  it('errors are surfaced in the result summary', async () => {
    vi.mocked(api.importOPML).mockResolvedValue({ imported: 1, skipped: 2, errors: ['bad url x', 'bad url y'] });
    const { container } = render(ImportOpmlDialog, { props: { onClose: noop, onImported: noop } });
    const file = new File([new TextEncoder().encode('<opml/>').buffer], 'subs.opml');
    await fireEvent.change(container.querySelector('input[type="file"]') as HTMLInputElement, { target: { files: [file] } });
    expect(await screen.findByText(/bad url x/i)).toBeInTheDocument();
  });

  it('Done dismiss calls onImported then onClose', async () => {
    vi.mocked(api.importOPML).mockResolvedValue({ imported: 2, skipped: 0, errors: [] });
    const onImported = vi.fn(); const onClose = vi.fn();
    const { container } = render(ImportOpmlDialog, { props: { onClose, onImported } });
    const file = new File([new TextEncoder().encode('<opml/>').buffer], 'subs.opml');
    await fireEvent.change(container.querySelector('input[type="file"]') as HTMLInputElement, { target: { files: [file] } });
    await screen.findByText(/2 imported/i);
    await fireEvent.click(screen.getByRole('button', { name: /done/i }));
    expect(onImported).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });
});
