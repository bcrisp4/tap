import { describe, it, expect } from 'vitest';
import { recognisePull } from '../pulltorefresh';

describe('recognisePull(dy, scrollTop, inFlight)', () => {
  it('returns true when dy >= 60 and scrollTop === 0 and not in flight', () => {
    expect(recognisePull(60, 0, false)).toBe(true);
  });
  it('returns false when dy < 60', () => {
    expect(recognisePull(59, 0, false)).toBe(false);
  });
  it('returns false when scrollTop > 0', () => {
    expect(recognisePull(80, 10, false)).toBe(false);
  });
  it('returns false when in flight', () => {
    expect(recognisePull(80, 0, true)).toBe(false);
  });
});
