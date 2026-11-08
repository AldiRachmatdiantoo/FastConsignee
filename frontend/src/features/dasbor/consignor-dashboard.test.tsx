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
    const sales = section("Sales");
    expect(sales.getByRole("link", { name: "Explorer" })).toHaveAttribute("href", "https://explorer.test/tx");
    expect(sales.getByText("1 units refunded")).toBeVisible();
    expect(sales.getByText(/Deducted Rp\s?5\.000 for an earlier refund/)).toBeVisible();
  });

  it("menampilkan potongan yang belum lunas", async () => {
    respond([], [deduction], [], 5000);
    render(<ConsignorDashboard />);

    expect(await screen.findByText("Tas Rotan Bundar")).toBeVisible();
    const deductions = section("Refund deductions");
    expect(deductions.getByText(/Remaining Rp\s?5\.000/)).toBeVisible();
    expect(deductions.getByText(/Unpaid/)).toHaveTextContent(/Rp\s?5\.000/);
  });

  it("menandai stok titipan yang nonaktif", async () => {
    respond([], [], [stock]);
    render(<ConsignorDashboard />);

    expect(await screen.findByText("Keranjang Bambu")).toBeVisible();
    expect(screen.getByText(/· inactive/)).toBeVisible();
    expect(screen.getByText("Stock 4")).toBeVisible();
  });

  it("meringkas bagian, barang terjual, stok, dan potongan", async () => {
    const today = new Date().toISOString();
    respond([{ ...sale, sold_at: today }], [deduction], [{ ...stock, active: true }], 5000);
    render(<ConsignorDashboard />);

    await screen.findByRole("heading", { name: "Today's share" });
    const tile = (heading: string) => within(screen.getByRole("heading", { name: heading }).closest("section") as HTMLElement);

    expect(tile("Today's share").getByText(/Rp\s?72\.000/)).toBeVisible();
    expect(tile("Today's share").getByText("No comparison with yesterday")).toBeVisible();
    expect(tile("Total share").getByText("80%")).toBeVisible();
    expect(tile("Items sold").getByText("2 pcs", { selector: "p" })).toBeVisible();
    expect(tile("Items sold").getByText("1 units refunded")).toBeVisible();
    expect(tile("Stock left").getByText("4 pcs")).toBeVisible();
    expect(tile("Unpaid deductions").getByText(/Rp\s?5\.000/)).toBeVisible();
  });

  it("tidak menghitung stok barang nonaktif", async () => {
    respond([], [], [stock]);
    render(<ConsignorDashboard />);

    await screen.findByRole("heading", { name: "Stock left" });
    const tile = within(screen.getByRole("heading", { name: "Stock left" }).closest("section") as HTMLElement);
    expect(tile.getByText("0 pcs")).toBeVisible();
  });

  it("memberi keterangan saat belum ada data", async () => {
    respond([], [], []);
    render(<ConsignorDashboard />);

    expect(await screen.findByText("No items sold yet.")).toBeVisible();
    expect(screen.getByText("No deductions.")).toBeVisible();
    expect(screen.getByText("No consigned items yet.")).toBeVisible();
  });
});
