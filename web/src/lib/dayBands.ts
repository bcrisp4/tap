export type EntryLike = { id: number; published_at: number };

export type DayBands<T extends EntryLike> = {
  today: T[];
  yesterday: T[];
  thisWeek: T[];
  earlier: T[];
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

  const out: DayBands<T> = { today: [], yesterday: [], thisWeek: [], earlier: [] };
  for (const item of items) {
    if (item.published_at >= todayStartSec) out.today.push(item);
    else if (item.published_at >= yesterdayStartSec) out.yesterday.push(item);
    else if (item.published_at >= weekStartSec) out.thisWeek.push(item);
    else out.earlier.push(item);
  }
  return out;
}
