import { describe, it, expect } from 'vitest';
import { bucketByDay, type EntryLike } from '../dayBand';

describe('bucketByDay', () => {
  // Use a fixed NOW and compute local-timezone-relative offsets so tests are tz-safe.
  const NOW_MS = new Date('2026-05-11T12:00:00Z').getTime();
  const NOW = NOW_MS / 1000;

  // Local start-of-day for the NOW reference point.
  const todayStart = new Date(NOW_MS);
  todayStart.setHours(0, 0, 0, 0);
  const todayStartSec = todayStart.getTime() / 1000;

  function entry(id: number, secFromTodayStart: number): EntryLike {
    return { id, published_at: Math.floor(todayStartSec + secFromTodayStart) };
  }

  it('buckets entries from today into Today', () => {
    // 8h and 1min after local midnight → still today
    const items = [entry(1, 8 * 3600), entry(2, 60)];
    const out = bucketByDay(items, NOW);
    expect(out.Today.map(e => e.id)).toEqual([1, 2]);
    expect(out.Yesterday).toHaveLength(0);
  });

  it('buckets entries from yesterday into Yesterday', () => {
    // 1 min before local midnight → yesterday; 23h before local midnight → yesterday
    const items = [entry(1, -60), entry(2, -23 * 3600)];
    const out = bucketByDay(items, NOW);
    expect(out.Yesterday.map(e => e.id)).toEqual([1, 2]);
  });

  it('buckets 2–7 days ago into ThisWeek', () => {
    // 2 full days before local midnight start (-2d-1h), and 6 days before
    const items = [entry(1, -2 * 86400 - 3600), entry(2, -6 * 86400 - 3600)];
    const out = bucketByDay(items, NOW);
    expect(out.ThisWeek.map(e => e.id)).toEqual([1, 2]);
  });

  it('buckets >7 days ago into Earlier', () => {
    // 8 days before local midnight start, and 150 days before
    const items = [entry(1, -8 * 86400 - 3600), entry(2, -150 * 86400)];
    const out = bucketByDay(items, NOW);
    expect(out.Earlier.map(e => e.id)).toEqual([1, 2]);
  });

  it('preserves input order within a band', () => {
    const items = [entry(1, 11 * 3600), entry(2, 9 * 3600), entry(3, 3600)];
    const out = bucketByDay(items, NOW);
    expect(out.Today.map(e => e.id)).toEqual([1, 2, 3]);
  });

  it('returns empty bands when input is empty', () => {
    const out = bucketByDay([], NOW);
    expect(out.Today).toHaveLength(0);
    expect(out.Yesterday).toHaveLength(0);
    expect(out.ThisWeek).toHaveLength(0);
    expect(out.Earlier).toHaveLength(0);
  });

  it('uses the local timezone day boundary, not UTC, for Today vs Yesterday', () => {
    const justBeforeMidnight = todayStartSec - 1;
    const justAfterMidnight = todayStartSec + 1;
    const out = bucketByDay(
      [{ id: 1, published_at: justBeforeMidnight }, { id: 2, published_at: justAfterMidnight }],
      NOW,
    );
    expect(out.Yesterday.map(e => e.id)).toEqual([1]);
    expect(out.Today.map(e => e.id)).toEqual([2]);
  });
});
