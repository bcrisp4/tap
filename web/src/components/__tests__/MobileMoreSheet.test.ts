import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import MobileMoreSheet from '../MobileMoreSheet.svelte';

describe('MobileMoreSheet', () => {
  it('hidden when closed', () => {
    const { queryByText } = render(MobileMoreSheet, { open: false, onClose: () => {} });
    expect(queryByText('History')).toBeNull();
  });

  it('renders History and Settings items when open', () => {
    const { getByText } = render(MobileMoreSheet, { open: true, onClose: () => {} });
    expect(getByText('History')).toBeTruthy();
    expect(getByText('Settings')).toBeTruthy();
  });

  it('closes on backdrop click', async () => {
    const onClose = vi.fn();
    const { container } = render(MobileMoreSheet, { open: true, onClose });
    await fireEvent.click(container.querySelector('.tmnav-sheet-backdrop') as HTMLElement);
    expect(onClose).toHaveBeenCalledOnce();
  });
});
