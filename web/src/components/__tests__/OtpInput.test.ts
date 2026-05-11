import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import OtpInput from '../OtpInput.svelte';

describe('OtpInput', () => {
  it('renders six cells', () => {
    const { container } = render(OtpInput, { value: '', onChange: () => {} });
    expect(container.querySelectorAll('input').length).toBe(6);
  });

  it('fires onChange with full code on paste', async () => {
    const onChange = vi.fn();
    const { container } = render(OtpInput, { value: '', onChange });
    const first = container.querySelector('input') as HTMLInputElement;
    // jsdom does not have ClipboardEvent; create a plain Event with a mocked clipboardData
    const evt = Object.assign(new Event('paste', { bubbles: true, cancelable: true }), {
      clipboardData: { getData: () => '123456' },
    });
    first.dispatchEvent(evt);
    expect(onChange).toHaveBeenCalledWith('123456');
  });

  it('advances focus after typing a digit', async () => {
    const { container } = render(OtpInput, { value: '', onChange: () => {} });
    const inputs = container.querySelectorAll('input');
    (inputs[0] as HTMLInputElement).focus();
    await fireEvent.input(inputs[0], { target: { value: '7' } });
    expect(document.activeElement).toBe(inputs[1]);
  });

  it('moves focus left on Backspace in empty cell', async () => {
    const { container } = render(OtpInput, { value: '1', onChange: () => {} });
    const inputs = container.querySelectorAll('input');
    (inputs[1] as HTMLInputElement).focus();
    await fireEvent.keyDown(inputs[1], { key: 'Backspace' });
    expect(document.activeElement).toBe(inputs[0]);
  });
});
