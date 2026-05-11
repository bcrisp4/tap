import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import SavedToolbar from '../SavedToolbar.svelte';

describe('SavedToolbar', () => {
  it('renders zero count as "0 entries"', () => {
    const { container } = render(SavedToolbar, { props: { count: 0 } });
    const countEl = container.querySelector('.count');
    expect(countEl).not.toBeNull();
    expect(countEl!.textContent!.replace(/\s+/g, ' ').trim()).toMatch(/^0\s+entries$/);
  });

  it('renders singular for 1 ("1 entry")', () => {
    const { container } = render(SavedToolbar, { props: { count: 1 } });
    const countEl = container.querySelector('.count');
    expect(countEl).not.toBeNull();
    expect(countEl!.textContent!.replace(/\s+/g, ' ').trim()).toMatch(/^1\s+entry$/);
  });

  it('renders plural for N > 1 ("12 entries")', () => {
    const { container } = render(SavedToolbar, { props: { count: 12 } });
    const countEl = container.querySelector('.count');
    expect(countEl).not.toBeNull();
    expect(countEl!.textContent!.replace(/\s+/g, ' ').trim()).toMatch(/^12\s+entries$/);
  });

  it('renders the find hint with the / kbd chip', () => {
    const { container } = render(SavedToolbar, { props: { count: 3 } });
    const find = container.querySelector('.find');
    expect(find).not.toBeNull();
    expect(find!.textContent).toMatch(/find/i);
    expect(find!.textContent).toContain('/');
  });

  it('renders the "Saved" serif eyebrow', () => {
    render(SavedToolbar, { props: { count: 3 } });
    expect(screen.getByText('Saved')).toBeInTheDocument();
  });
});
