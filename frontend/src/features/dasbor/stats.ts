import { formatDay } from "@/lib/format";

export type Period = { from: string; to?: string };
export type DayPeriod = Period & { label: string };

export type Change = { direction: "up" | "down" | "flat"; percent: number | null };

type Page<T> = { data: T[]; next_cursor: string | null };

// Batas periode memakai zona waktu browser, supaya "hari ini" sama dengan hari di toko.
export function monthPeriod(now: Date, offset = 0): Period {
  const from = new Date(now.getFullYear(), now.getMonth() + offset, 1);
  const to = new Date(now.getFullYear(), now.getMonth() + offset + 1, 1);
  return { from: from.toISOString(), to: to.toISOString() };
}

function dayLabel(date: Date, daysAgo: number): string {
  if (daysAgo === 0) {
    return "Today";
  }
  return daysAgo === 1 ? "Yesterday" : formatDay(date);
}

// lastDays mengembalikan hari ini lebih dulu, lalu hari-hari sebelumnya.
export function lastDays(now: Date, count: number): DayPeriod[] {
  return Array.from({ length: count }, (_, daysAgo) => {
    const from = new Date(now.getFullYear(), now.getMonth(), now.getDate() - daysAgo);
    const to = new Date(now.getFullYear(), now.getMonth(), now.getDate() - daysAgo + 1);
    return { from: from.toISOString(), to: to.toISOString(), label: dayLabel(from, daysAgo) };
  });
}

export function isWithin(iso: string, period: Period): boolean {
  const time = Date.parse(iso);
  return time >= Date.parse(period.from) && (period.to === undefined || time < Date.parse(period.to));
}

// change tidak memberi persen kalau pembandingnya nol, karena hasil bagi nol tidak punya arti.
export function change(current: number, previous: number): Change {
  if (current === previous) {
    return { direction: "flat", percent: 0 };
  }
  return {
    direction: current > previous ? "up" : "down",
    percent: previous === 0 ? null : Math.round((Math.abs(current - previous) / previous) * 100),
  };
}

export function share(part: number, whole: number): number {
  return whole > 0 ? Math.round((part / whole) * 100) : 0;
}

export function sumBy<T>(items: T[], pick: (item: T) => number): number {
  return items.reduce((total, item) => total + pick(item), 0);
}

// walkPages mengikuti cursor sampai habis. Batas halamannya menjaga satu dasbor dari panggilan API tanpa henti.
export async function walkPages<T>(load: (cursor?: string) => Promise<Page<T>>, maxPages = 10) {
  const items: T[] = [];
  let cursor: string | undefined;
  for (let count = 0; count < maxPages; count += 1) {
    const page = await load(cursor);
    items.push(...page.data);
    if (page.next_cursor === null) {
      return { items, complete: true };
    }
    cursor = page.next_cursor;
  }
  return { items, complete: false };
}
