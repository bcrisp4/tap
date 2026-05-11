import { describe, it, expect, beforeEach } from 'vitest';
import { searchOverlay } from '../searchOverlay.svelte';

describe('searchOverlay store', () => {
  beforeEach(() => {
    searchOverlay.close();
    searchOverlay.setQuery('');
    searchOverlay.setScope('unread');
  });

  it('opens and closes', () => {
    expect(searchOverlay.open).toBe(false);
    searchOverlay.openOverlay();
    expect(searchOverlay.open).toBe(true);
    searchOverlay.close();
    expect(searchOverlay.open).toBe(false);
  });

  it('tracks query', () => {
    searchOverlay.setQuery('hello');
    expect(searchOverlay.query).toBe('hello');
  });

  it('tracks scope', () => {
    searchOverlay.setScope('saved');
    expect(searchOverlay.scope).toBe('saved');
  });

  it('resets query when closed', () => {
    searchOverlay.openOverlay();
    searchOverlay.setQuery('hello');
    searchOverlay.close();
    expect(searchOverlay.query).toBe('');
  });
});
