import { render } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import SearchOverlay from '../SearchOverlay.svelte';
import { searchOverlay } from '../../lib/searchOverlay.svelte';

describe('SearchOverlay', () => {
  it('hidden when store closed', () => {
    searchOverlay.close();
    const { container } = render(SearchOverlay);
    expect(container.querySelector('.search-overlay')).toBeNull();
  });

  it('renders when store open', () => {
    searchOverlay.openOverlay();
    const { container } = render(SearchOverlay);
    expect(container.querySelector('.search-overlay')).not.toBeNull();
    searchOverlay.close();
  });
});
