import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ok, page } from "@/test/api";
import { StoreDashboard } from "./store-dashboard";

const get = vi.fn();
const api = { GET: get };

vi.mock("@/features/auth/session", () => ({ useSession: () => ({ api }) }));

function summary(sales: number, commission: number, count: number) {
  return {
    from: null,
    to: null,
    total_sales_idr: sales,
    total_commission_idr: commission,
    transaction_count: count,
    generated_at: "2026-11-07T00:00:00Z",
  };
}

function hoursAgo(hours: number): string {
  return new Date(Date.now() - hours * 3_600_000).toISOString();
}

function dayAt(offset: number): string {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), now.getDate() + offset, 0, 30).toISOString();
}

function order(id: string, total: number, confirmedAt: string) {
  return {
    id,
    status: "confirmed",
    total_idr: total,
    total_lamports: "1000000000",
    commission_idr: total / 10,
    item_count: 2,
    signature: "sig",
    explorer_url: `https://explorer.test/${id}`,
    created_at: confirmedAt,
    confirmed_at: confirmedAt,
  };
}

type Query = { from?: string; to?: string };

function respond(orders: unknown[], totals = summary(300000, 30000, 3)) {
  get.mockImplementation((path: string, options?: { params?: { query?: Query } }) => {
    if (path === "/api/v1/orders") {
      return page(orders);
    }
    const query = options?.params?.query ?? {};
    if (!query.from) {
      return ok(totals);
    }
    return new Date(query.from).getMonth() === new Date().getMonth() ? ok(summary(250000, 25000, 2)) : ok(summary(125000, 12500, 1));
  });
}

function card(heading: string) {
  return within(screen.getByRole("heading", { name: heading }).closest("section") as HTMLElement);
}

describe("StoreDashboard", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("membandingkan penjualan hari ini dengan kemarin", async () => {
    respond([order("a", 100000, dayAt(0)), order("b", 50000, dayAt(-1))]);
    render(<StoreDashboard />);

    expect(await screen.findByRole("heading", { name: "Penjualan hari ini" })).toBeVisible();
    const today = card("Penjualan hari ini");
    expect(today.getByText(/Rp\s?100\.000/)).toBeVisible();
    expect(today.getByText("Naik 100% dari kemarin")).toBeVisible();
    expect(today.getByText("40%")).toBeVisible();
  });

  it("membandingkan penjualan bulan ini dengan bulan lalu", async () => {
    respond([]);
    render(<StoreDashboard />);

    await screen.findByRole("heading", { name: "Penjualan bulan ini" });
    const month = card("Penjualan bulan ini");
    expect(month.getByText(/Rp\s?250\.000/)).toBeVisible();
    expect(month.getByText("Naik 100% dari bulan lalu")).toBeVisible();
  });

  it("menghitung bagian penitip dari penjualan dikurangi komisi", async () => {
    respond([]);
    render(<StoreDashboard />);

    await screen.findByRole("heading", { name: "Bagian penitip" });
    expect(card("Bagian penitip").getByText(/Rp\s?270\.000/)).toBeVisible();
    expect(card("Komisi toko").getByText("10% dari penjualan")).toBeVisible();
    expect(card("Jumlah transaksi").getByText(/Rata-rata Rp\s?100\.000/)).toBeVisible();
  });

  it("tidak mengarang persen saat kemarin belum ada penjualan", async () => {
    respond([order("a", 100000, hoursAgo(0))]);
    render(<StoreDashboard />);

    await screen.findByRole("heading", { name: "Penjualan hari ini" });
    expect(card("Penjualan hari ini").getByText("Belum ada pembanding dari kemarin")).toBeVisible();
  });

  it("menampilkan tujuh hari dengan nilai tertulis", async () => {
    respond([order("a", 100000, dayAt(0))]);
    render(<StoreDashboard />);

    await screen.findByRole("heading", { name: "7 hari terakhir" });
    const week = card("7 hari terakhir");
    expect(week.getAllByRole("listitem")).toHaveLength(7);
    expect(within(week.getByText("Hari ini").closest("li") as HTMLElement).getByText(/Rp\s?100\.000/)).toBeVisible();
    expect(within(week.getByText("Kemarin").closest("li") as HTMLElement).getByText(/Rp\s?0/)).toBeVisible();
  });

  it("menampilkan transaksi beserta bukti di Explorer", async () => {
    respond([order("a", 100000, dayAt(0))]);
    render(<StoreDashboard />);

    expect(await screen.findByRole("link", { name: "Explorer" })).toHaveAttribute("href", "https://explorer.test/a");
    expect(screen.getByText(/2 barang · komisi Rp\s?10\.000/)).toBeVisible();
  });

  it("tetap hemat panggilan API supaya tidak kena batas permintaan", async () => {
    respond([]);
    render(<StoreDashboard />);

    await screen.findByRole("heading", { name: "Penjualan hari ini" });
    expect(get.mock.calls.length).toBeLessThanOrEqual(5);
  });

  it("memberi tahu kalau angka harian tidak lengkap", async () => {
    get.mockImplementation((path: string) =>
      path === "/api/v1/orders" ? ok({ data: [], next_cursor: "lanjut" }) : ok(summary(0, 0, 0)),
    );
    render(<StoreDashboard />);

    expect(await screen.findByText("Angka harian dihitung dari 1000 transaksi terbaru.")).toBeVisible();
  });

  it("memberi keterangan saat belum ada transaksi", async () => {
    respond([], summary(0, 0, 0));
    render(<StoreDashboard />);

    expect(await screen.findByText("Belum ada transaksi.")).toBeVisible();
  });
});
