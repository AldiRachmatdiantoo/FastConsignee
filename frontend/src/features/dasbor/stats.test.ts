import { describe, expect, it, vi } from "vitest";
import { change, isWithin, lastDays, monthPeriod, share, sumBy, walkPages } from "./stats";

describe("monthPeriod", () => {
  it("mengambil awal bulan sampai awal bulan berikutnya", () => {
    const period = monthPeriod(new Date(2026, 10, 7, 15));
    expect(period.from).toBe(new Date(2026, 10, 1).toISOString());
    expect(period.to).toBe(new Date(2026, 11, 1).toISOString());
  });

  it("bisa mundur melewati pergantian tahun", () => {
    const period = monthPeriod(new Date(2026, 0, 15), -1);
    expect(period.from).toBe(new Date(2025, 11, 1).toISOString());
    expect(period.to).toBe(new Date(2026, 0, 1).toISOString());
  });
});

describe("lastDays", () => {
  const days = lastDays(new Date(2026, 10, 7, 15), 7);

  it("dimulai dari hari ini dan tidak punya celah antar hari", () => {
    expect(days).toHaveLength(7);
    expect(days[0].from).toBe(new Date(2026, 10, 7).toISOString());
    expect(days[0].to).toBe(new Date(2026, 10, 8).toISOString());
    expect(days[1].to).toBe(days[0].from);
    expect(days[6].from).toBe(new Date(2026, 10, 1).toISOString());
  });

  it("memberi nama hari ini dan kemarin", () => {
    expect(days[0].label).toBe("Today");
    expect(days[1].label).toBe("Yesterday");
    expect(days[2].label).toMatch(/5 Nov/);
  });
});

describe("isWithin", () => {
  const period = { from: "2026-11-07T00:00:00.000Z", to: "2026-11-08T00:00:00.000Z" };

  it("memasukkan batas awal dan mengeluarkan batas akhir", () => {
    expect(isWithin("2026-11-07T00:00:00.000Z", period)).toBe(true);
    expect(isWithin("2026-11-07T23:59:59.000Z", period)).toBe(true);
    expect(isWithin("2026-11-08T00:00:00.000Z", period)).toBe(false);
    expect(isWithin("2026-11-06T23:59:59.000Z", period)).toBe(false);
  });
});

describe("change", () => {
  it("menghitung arah dan persen", () => {
    expect(change(150, 100)).toEqual({ direction: "up", percent: 50 });
    expect(change(50, 100)).toEqual({ direction: "down", percent: 50 });
    expect(change(100, 100)).toEqual({ direction: "flat", percent: 0 });
  });

  it("tidak memberi persen kalau pembandingnya nol", () => {
    expect(change(100, 0)).toEqual({ direction: "up", percent: null });
    expect(change(0, 0)).toEqual({ direction: "flat", percent: 0 });
  });
});

describe("share dan sumBy", () => {
  it("tidak membagi dengan nol", () => {
    expect(share(1, 4)).toBe(25);
    expect(share(5, 0)).toBe(0);
  });

  it("menjumlahkan nilai yang dipilih", () => {
    expect(sumBy([{ value: 2 }, { value: 3 }], ({ value }) => value)).toBe(5);
    expect(sumBy([], () => 1)).toBe(0);
  });
});

describe("walkPages", () => {
  it("mengikuti cursor sampai halaman terakhir", async () => {
    const load = vi.fn(async (cursor?: string) =>
      cursor ? { data: [2], next_cursor: null } : { data: [1], next_cursor: "lanjut" },
    );
    await expect(walkPages(load)).resolves.toEqual({ items: [1, 2], complete: true });
    expect(load).toHaveBeenCalledTimes(2);
    expect(load).toHaveBeenLastCalledWith("lanjut");
  });

  it("berhenti di batas halaman dan menandai datanya belum lengkap", async () => {
    const load = vi.fn(async () => ({ data: [1], next_cursor: "lanjut" }));
    await expect(walkPages(load, 3)).resolves.toEqual({ items: [1, 1, 1], complete: false });
    expect(load).toHaveBeenCalledTimes(3);
  });
});
