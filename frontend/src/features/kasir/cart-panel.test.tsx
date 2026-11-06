import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ok, problem } from "@/test/api";
import type { Cart, Item } from "./cart";
import { CartPanel } from "./cart-panel";

const post = vi.fn();
const api = { POST: post };

vi.mock("@/features/auth/session", () => ({ useSession: () => ({ api }) }));

const item = {
  id: "i1",
  name: "Dompet Pandan",
  price_idr: 45000,
  stock_available: 3,
  consignor: { id: "c1", name: "Sari Anyaman" },
} as Item;

const preview = {
  discount_idr: 0,
  total_idr: 45000,
  total_lamports: "1000000000",
  recipients: [
    { wallet_address: "w1", name: "Sari Anyaman", recipient_type: "consignor", amount_idr: 36000, amount_lamports: "800000000", deduction_idr: 0 },
    { wallet_address: "w2", name: "Toko", recipient_type: "store", amount_idr: 9000, amount_lamports: "200000000", deduction_idr: 0 },
  ],
};

function cart(quantity = 1): Cart {
  return [{ item, quantity, discount: null }];
}

function orderCalls() {
  return post.mock.calls.filter(([path]) => path === "/api/v1/orders");
}

function idempotencyKey(call: unknown[]) {
  return (call[1] as { params: { header: Record<string, string> } }).params.header["Idempotency-Key"];
}

describe("CartPanel", () => {
  beforeEach(() => {
    post.mockReset();
  });

  it("menampilkan rincian dari server sebelum QR bisa dibuat", async () => {
    post.mockImplementation(() => ok(preview));
    render(<CartPanel cart={cart()} onChange={vi.fn()} onOrder={vi.fn()} />);

    expect(screen.getByRole("status")).toHaveTextContent("Menghitung pembagian...");
    expect(screen.getByRole("button", { name: "Buat QR pembayaran" })).toBeDisabled();

    expect(await screen.findByText("Bagian penitip")).toBeVisible();
    expect(screen.getByText("Komisi toko")).toBeVisible();
    expect(screen.getByRole("button", { name: "Buat QR pembayaran" })).toBeEnabled();
  });

  it("menahan QR kalau pratinjau ditolak", async () => {
    post.mockImplementation(() => problem("INSUFFICIENT_STOCK"));
    render(<CartPanel cart={cart()} onChange={vi.fn()} onOrder={vi.fn()} />);

    expect(await screen.findByRole("alert")).toHaveTextContent("Stok tidak cukup. Kurangi jumlah barang.");
    expect(screen.getByRole("button", { name: "Buat QR pembayaran" })).toBeDisabled();
  });

  it("memakai Idempotency-Key yang sama saat mencoba lagi", async () => {
    const order = { id: "o1" };
    let attempts = 0;
    post.mockImplementation((path: string) => {
      if (path === "/api/v1/orders/preview") {
        return ok(preview);
      }
      attempts += 1;
      return attempts === 1 ? Promise.reject(new TypeError("failed to fetch")) : ok(order);
    });
    const onOrder = vi.fn();
    const user = userEvent.setup();
    render(<CartPanel cart={cart()} onChange={vi.fn()} onOrder={onOrder} />);

    await screen.findByText("Bagian penitip");
    await user.click(screen.getByRole("button", { name: "Buat QR pembayaran" }));
    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "Buat QR pembayaran" }));

    await waitFor(() => expect(onOrder).toHaveBeenCalledWith(order));
    const [first, second] = orderCalls();
    expect(idempotencyKey(first)).toBe(idempotencyKey(second));
  });

  it("membuat Idempotency-Key baru kalau isi keranjang berubah", async () => {
    post.mockImplementation((path: string) => (path === "/api/v1/orders/preview" ? ok(preview) : problem("INSUFFICIENT_STOCK")));
    const user = userEvent.setup();
    const { rerender } = render(<CartPanel cart={cart()} onChange={vi.fn()} onOrder={vi.fn()} />);

    await screen.findByText("Bagian penitip");
    await user.click(screen.getByRole("button", { name: "Buat QR pembayaran" }));
    await screen.findByRole("alert");

    rerender(<CartPanel cart={cart(2)} onChange={vi.fn()} onOrder={vi.fn()} />);
    await screen.findByText("Bagian penitip");
    await user.click(screen.getByRole("button", { name: "Buat QR pembayaran" }));

    await waitFor(() => expect(orderCalls()).toHaveLength(2));
    const [first, second] = orderCalls();
    expect(idempotencyKey(first)).not.toBe(idempotencyKey(second));
  });

  it("tidak boleh menambah melebihi stok tersedia", async () => {
    post.mockImplementation(() => ok(preview));
    render(<CartPanel cart={cart(3)} onChange={vi.fn()} onOrder={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Tambah Dompet Pandan" })).toBeDisabled();
    await screen.findByText("Bagian penitip");
  });

  it("menyimpan diskon beserta pihak yang menanggungnya", async () => {
    post.mockImplementation(() => ok(preview));
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<CartPanel cart={cart()} onChange={onChange} onOrder={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Beri diskon" }));
    await user.selectOptions(screen.getByLabelText("Ditanggung"), "consignor");

    expect(onChange).toHaveBeenLastCalledWith([{ item, quantity: 1, discount: { amount_idr: 0, borne_by: "consignor" } }]);
    await screen.findByText("Bagian penitip");
  });
});
