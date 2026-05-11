export type EntryLike = { id: number; published_at: number };

export type DayBands<T extends EntryLike> = {
  Today: T[];
  Yesterday: T[];
  ThisWeek: T[];
  Earlier: T[];
};

const DAY = 86400;

export function bucketByDay<T extends EntryLike>(
  items: T[],
  now: number = Math.floor(Date.now() / 1000),
): DayBands<T> {
  const todayStart = new Date(now * 1000);
  todayStart.setHours(0, 0, 0, 0);
  const todayStartSec = Math.floor(todayStart.getTime() / 1000);
  const yesterdayStartSec = todayStartSec - DAY;
  const weekStartSec = todayStartSec - 7 * DAY;

  const out: DayBands<T> = { Today: [], Yesterday: [], ThisWeek: [], Earlier: [] };
  for (const item of items) {
    if (item.published_at >= todayStartSec) out.Today.push(item);
    else if (item.published_at >= yesterdayStartSec) out.Yesterday.push(item);
    else if (item.published_at >= weekStartSec) out.ThisWeek.push(item);
    else out.Earlier.push(item);
  }
  return out;
}
