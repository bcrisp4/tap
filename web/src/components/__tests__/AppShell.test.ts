import { render } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import AppShell from '../AppShell.svelte';
import { createRawSnippet } from 'svelte';

const slot = createRawSnippet(() => ({ render: () => '<div>slot</div>' }));

describe('AppShell', () => {
  it('renders children', () => {
    const { getByText } = render(AppShell, { children: slot });
    expect(getByText('slot')).toBeTruthy();
  });

  it('applies tap theme class to root', () => {
    const { container } = render(AppShell, { children: slot });
    expect(container.querySelector('.tap')).not.toBeNull();
  });
});
