import { describe, it, expect } from 'vitest';
import { recogniseSwipe } from '../swipe';

describe('recogniseSwipe(dx, dy, startX)', () => {
  it('returns "right" for sufficient rightward swipe', () => {
    expect(recogniseSwipe(50, 5, 50)).toBe('right');
  });
  it('returns "left" for sufficient leftward swipe', () => {
    expect(recogniseSwipe(-50, 5, 50)).toBe('left');
  });
  it('returns null when travel < 40px', () => {
    expect(recogniseSwipe(39, 2, 50)).toBeNull();
    expect(recogniseSwipe(-39, 2, 50)).toBeNull();
  });
  it('returns null when angle >= 30° from horizontal', () => {
    // tan(30°) ≈ 0.577; at dx=40, dy must be < 40*0.577 ≈ 23.1
    expect(recogniseSwipe(40, 24, 50)).toBeNull();
  });
  it('returns direction when angle < 30°', () => {
    expect(recogniseSwipe(40, 22, 50)).toBe('right');
  });
  it('returns null when startX <= 20 (iOS edge-swipe guard)', () => {
    expect(recogniseSwipe(50, 5, 15)).toBeNull();
  });
  it('returns direction when startX > 20', () => {
    expect(recogniseSwipe(50, 5, 21)).toBe('right');
  });
});
