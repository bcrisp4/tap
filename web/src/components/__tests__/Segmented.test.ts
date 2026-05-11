import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import Segmented from '../Segmented.svelte';

describe('Segmented', () => {
  it('renders options with active marker', () => {
    const { getByRole } = render(Segmented, {
      options: [
        { value: 'light', label: 'Light' },
        { value: 'dark', label: 'Dark' },
      ],
      value: 'dark',
      onChange: () => {},
    });
    const dark = getByRole('radio', { name: /dark/i });
    expect(dark.className).toMatch(/is-active/);
  });

  it('fires onChange on click', async () => {
    const onChange = vi.fn();
    const { getByRole } = render(Segmented, {
      options: [
        { value: 'light', label: 'Light' },
        { value: 'dark', label: 'Dark' },
      ],
      value: 'light',
      onChange,
    });
    await fireEvent.click(getByRole('radio', { name: /dark/i }));
    expect(onChange).toHaveBeenCalledWith('dark');
    expect(onChange).toHaveBeenCalledTimes(1);
  });
});
