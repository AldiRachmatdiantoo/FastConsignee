import { describe, expect, it } from "vitest";
import { formatCountdown, formatPercent, formatRupiah, formatSol, secondsUntil } from "./format";

describe("formatRupiah", () => {
  it("memakai pemisah ribuan Indonesia tanpa desimal", () => {
    expect(formatRupiah(125000).replace(/\s/g, " ")).toBe("Rp 125.000");
  });
});

describe("formatSol", () => {
  it("menjaga semua digit lamport", () => {
    expect(formatSol("1234567891")).toBe("1,234567891 SOL");
  });

  it("membuang nol di belakang", () => {
    expect(formatSol("1500000000")).toBe("1,5 SOL");
    expect(formatSol("2000000000")).toBe("2 SOL");
  });

  it("menangani nilai kecil dan negatif", () => {
    expect(formatSol("5000")).toBe("0,000005 SOL");
    expect(formatSol(BigInt(-5000))).toBe("-0,000005 SOL");
  });

  it("tidak kehilangan presisi di atas batas Number", () => {
    expect(formatSol("9007199254740993")).toBe("9007199,254740993 SOL");
  });
});

describe("formatPercent", () => {
  it("mengubah basis point menjadi persen", () => {
    expect(formatPercent(2000)).toBe("20%");
    expect(formatPercent(1250)).toBe("12,5%");
  });
});

describe("hitung mundur", () => {
  it("tidak pernah negatif", () => {
    expect(secondsUntil("2026-11-06T00:00:00Z", Date.parse("2026-11-06T00:01:00Z"))).toBe(0);
  });

  it("membulatkan ke atas", () => {
    expect(secondsUntil("2026-11-06T00:00:10Z", Date.parse("2026-11-06T00:00:00.500Z"))).toBe(10);
  });

  it("menampilkan menit dan detik", () => {
    expect(formatCountdown(65)).toBe("1:05");
    expect(formatCountdown(0)).toBe("0:00");
  });
});
