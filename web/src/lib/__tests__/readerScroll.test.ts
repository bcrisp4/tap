import { describe, it, expect, beforeEach } from 'vitest';
import { loadScroll, saveScroll, clearScroll } from '../readerScroll';

beforeEach(() => localStorage.clear());

describe('readerScroll', () => {
  it('round-trips an integer scroll value', () => {
    saveScroll(42, 1200);
    expect(loadScroll(42)).toBe(1200);
  });

  it('returns 0 when no value is stored', () => {
    expect(loadScroll(99)).toBe(0);
  });

  it('returns 0 when stored value is not a finite number', () => {
    localStorage.setItem('tap.reader.scroll.7', 'not-a-number');
    expect(loadScroll(7)).toBe(0);
  });

  it('clearScroll removes the key', () => {
    saveScroll(3, 50);
    clearScroll(3);
    expect(loadScroll(3)).toBe(0);
  });

  it('saveScroll rejects non-finite input by clearing the key', () => {
    saveScroll(5, 100);
    saveScroll(5, NaN);
    expect(loadScroll(5)).toBe(0);
  });

  it('keys are namespaced so different entry ids do not collide', () => {
    saveScroll(1, 10); saveScroll(2, 20);
    expect(loadScroll(1)).toBe(10);
    expect(loadScroll(2)).toBe(20);
  });
});
