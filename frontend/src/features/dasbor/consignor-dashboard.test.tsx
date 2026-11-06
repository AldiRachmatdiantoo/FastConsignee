import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ok, page } from "@/test/api";
import { ConsignorDashboard } from "./consignor-dashboard";

const get = vi.fn();
const api = { GET: get };

vi.mock("@/features/auth/session", () => ({ useSession: () => ({ api }) }));

const sale = {
  order_line_id: "l1",
  item_name: "Dompet Pandan",
  quantity: 2,
  sold_at: "2026-11-07T00:01:00Z",
  line_total_idr: 90000,
  commission_idr: 18000,
  consignor_share_idr: 72000,
  refunded_quantity: 1,
  order_deduction_idr: 5000,
  order_transfer_lamports: "1500000000",
  explorer_url: "https://explorer.test/tx",
};

const deduction = {
  id: "d1",
  item_name: "Tas Rotan Bundar",
  created_at: "2026-11-07T00:02:00Z",
  amount_idr: 20000,
  remaining_idr: 5000,
  status: "outstanding",
};

const stock = { item_id: "i1", item_name: "Keranjang Bambu", price_idr: 95000, stock_available: 4, active: false };

function respond(sales: unknown[], deductions: unknown[], stocks: unknown[], outstanding = 0) {
  get.mockImplementation((path: string) => {
    if (path === "/api/v1/dashboard/consignor/sales") {
      return page(sales);
    }
    if (path === "/api/v1/dashboard/consignor/deductions") {
      return ok({ data: deductions, next_cursor: null, outstanding_total_idr: outstanding });
    }
    return page(stocks);
  });
}

function section(heading: string) {
  return within(screen.getByRole("heading", { name: heading }).closest("section") as HTMLElement);
}

describe("ConsignorDashboard", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("menampilkan penjualan beserta bukti di Explorer", async () => {
    respond([sale], [], []);
    render(<ConsignorDashboard />);

    expect(await screen.findByText("Dompet Pandan × 2")).toBeVisible();
    const sales = section("Penjualan");
    expect(sales.getByRole("link", { name: "Explorer" })).toHaveAttribute("href", "https://explorer.test/tx");
    expect(sales.getByText("1 unit sudah di-refund")).toBeVisible();
    expect(sales.getByText(/Dipotong Rp\s?5\.000 untuk refund lama/)).toBeVisible();
  });

  it("menampilkan potongan yang belum lunas", async () => {
    respond([], [deduction], [], 5000);
    render(<ConsignorDashboard />);

    expect(await screen.findByText("Tas Rotan Bundar")).toBeVisible();
    const deductions = section("Potongan refund");
    expect(deductions.getByText(/Sisa Rp\s?5\.000/)).toBeVisible();
    expect(deductions.getByText(/Belum lunas/)).toHaveTextContent(/Rp\s?5\.000/);
  });

  it("menandai stok titipan yang nonaktif", async () => {
    respond([], [], [stock]);
    render(<ConsignorDashboard />);

    expect(await screen.findByText("Keranjang Bambu")).toBeVisible();
    expect(screen.getByText(/· nonaktif/)).toBeVisible();
    expect(screen.getByText("Stok 4")).toBeVisible();
  });

  it("meringkas bagian, barang terjual, stok, dan potongan", async () => {
    const today = new Date().toISOString();
    respond([{ ...sale, sold_at: today }], [deduction], [{ ...stock, active: true }], 5000);
    render(<ConsignorDashboard />);

    await screen.findByRole("heading", { name: "Bagian hari ini" });
    const tile = (heading: string) => within(screen.getByRole("heading", { name: heading }).closest("section") as HTMLElement);

    expect(tile("Bagian hari ini").getByText(/Rp\s?72\.000/)).toBeVisible();
    expect(tile("Bagian hari ini").getByText("Belum ada pembanding dari kemarin")).toBeVisible();
    expect(tile("Total bagian").getByText("80%")).toBeVisible();
    expect(tile("Barang terjual").getByText("2 unit", { selector: "p" })).toBeVisible();
    expect(tile("Barang terjual").getByText("1 unit di-refund")).toBeVisible();
    expect(tile("Stok tersisa").getByText("4 unit")).toBeVisible();
    expect(tile("Potongan belum lunas").getByText(/Rp\s?5\.000/)).toBeVisible();
  });

  it("tidak menghitung stok barang nonaktif", async () => {
    respond([], [], [stock]);
    render(<ConsignorDashboard />);

    await screen.findByRole("heading", { name: "Stok tersisa" });
    const tile = within(screen.getByRole("heading", { name: "Stok tersisa" }).closest("section") as HTMLElement);
    expect(tile.getByText("0 unit")).toBeVisible();
  });

  it("memberi keterangan saat belum ada data", async () => {
    respond([], [], []);
    render(<ConsignorDashboard />);

    expect(await screen.findByText("Belum ada barang yang terjual.")).toBeVisible();
    expect(screen.getByText("Tidak ada potongan.")).toBeVisible();
    expect(screen.getByText("Belum ada barang titipan.")).toBeVisible();
  });
});
