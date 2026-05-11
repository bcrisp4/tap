import { describe, it, expect } from 'vitest';
import { bucketByDay, type EntryLike } from '../dayBands';

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

  it('buckets entries from today into today', () => {
    const items = [entry(1, 8 * 3600), entry(2, 60)];
    const out = bucketByDay(items, NOW);
    expect(out.today.map(e => e.id)).toEqual([1, 2]);
    expect(out.yesterday).toHaveLength(0);
  });

  it('buckets entries from yesterday into yesterday', () => {
    const items = [entry(1, -60), entry(2, -23 * 3600)];
    const out = bucketByDay(items, NOW);
    expect(out.yesterday.map(e => e.id)).toEqual([1, 2]);
  });

  it('buckets 2–7 days ago into thisWeek', () => {
    const items = [entry(1, -2 * 86400 - 3600), entry(2, -6 * 86400 - 3600)];
    const out = bucketByDay(items, NOW);
    expect(out.thisWeek.map(e => e.id)).toEqual([1, 2]);
  });

  it('buckets >7 days ago into earlier', () => {
    const items = [entry(1, -8 * 86400 - 3600), entry(2, -150 * 86400)];
    const out = bucketByDay(items, NOW);
    expect(out.earlier.map(e => e.id)).toEqual([1, 2]);
  });

  it('preserves input order within a band', () => {
    const items = [entry(1, 11 * 3600), entry(2, 9 * 3600), entry(3, 3600)];
    const out = bucketByDay(items, NOW);
    expect(out.today.map(e => e.id)).toEqual([1, 2, 3]);
  });

  it('returns empty bands when input is empty', () => {
    const out = bucketByDay([], NOW);
    expect(out.today).toHaveLength(0);
    expect(out.yesterday).toHaveLength(0);
    expect(out.thisWeek).toHaveLength(0);
    expect(out.earlier).toHaveLength(0);
  });

  it('uses the local timezone day boundary, not UTC, for today vs yesterday', () => {
    const justBeforeMidnight = todayStartSec - 1;
    const justAfterMidnight = todayStartSec + 1;
    const out = bucketByDay(
      [{ id: 1, published_at: justBeforeMidnight }, { id: 2, published_at: justAfterMidnight }],
      NOW,
    );
    expect(out.yesterday.map(e => e.id)).toEqual([1]);
    expect(out.today.map(e => e.id)).toEqual([2]);
  });
});
