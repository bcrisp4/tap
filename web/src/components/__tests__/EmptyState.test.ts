import { render } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import EmptyState from '../EmptyState.svelte';
import { createRawSnippet } from 'svelte';

describe('EmptyState', () => {
  it('renders string subtitle', () => {
    const { getByText } = render(EmptyState, { title: 'Nothing yet', subtitle: 'Come back later.' });
    expect(getByText('Come back later.')).toBeTruthy();
  });

  it('renders Snippet subtitle (allows inline interactive markup)', () => {
    const subtitle = createRawSnippet(() => ({ render: () => '<span>Press <kbd>S</kbd></span>' }));
    const { getByText } = render(EmptyState, { title: 'Empty', subtitle });
    expect(getByText(/Press/)).toBeTruthy();
  });

  it('renders cta button and fires onClick', async () => {
    const onClick = vi.fn();
    const { getByRole } = render(EmptyState, {
      title: 'No feeds', subtitle: 'Add one to get started.',
      cta: { label: 'Add feed', onClick },
    });
    (getByRole('button', { name: /add feed/i }) as HTMLButtonElement).click();
    expect(onClick).toHaveBeenCalledOnce();
  });
});
