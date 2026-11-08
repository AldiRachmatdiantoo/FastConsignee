import { describe, expect, it } from "vitest";
import { addBlock, addToCart, removeFromCart, setDiscount, setQuantity, toCartRequest, type Cart, type Item } from "./cart";

function item(id: string, consignor = "c1", stock = 3): Item {
  return {
    id,
    name: `Items ${id}`,
    price_idr: 10000,
    commission_bps: 2000,
    consignor: { id: consignor, name: `Consignor ${consignor}` },
    stock_on_hand: stock,
    stock_reserved: 0,
    stock_available: stock,
    active: true,
    certificate_enabled: false,
    created_at: "2026-11-06T00:00:00Z",
    updated_at: "2026-11-06T00:00:00Z",
  };
}

describe("keranjang", () => {
  it("menambah barang baru dengan jumlah 1", () => {
    expect(addToCart([], item("a"))).toEqual([{ item: item("a"), quantity: 1, discount: null }]);
  });

  it("menaikkan jumlah barang yang sama", () => {
    const cart = addToCart(addToCart([], item("a")), item("a"));
    expect(cart).toHaveLength(1);
    expect(cart[0].quantity).toBe(2);
  });

  it("menolak barang habis", () => {
    const empty = item("a", "c1", 0);
    expect(addBlock([], empty)).toBe("out_of_stock");
    expect(addToCart([], empty)).toEqual([]);
  });

  it("tidak melewati stok tersedia", () => {
    const limited = item("a", "c1", 1);
    const cart = addToCart([], limited);
    expect(addBlock(cart, limited)).toBe("out_of_stock");
    expect(addToCart(cart, limited)[0].quantity).toBe(1);
  });

  it("memuat paling banyak 5 penitip", () => {
    const cart = ["1", "2", "3", "4", "5"].reduce<Cart>((current, id) => addToCart(current, item(id, `c${id}`)), []);
    expect(addBlock(cart, item("6", "c6"))).toBe("too_many_consignors");
    expect(addBlock(cart, item("7", "c1"))).toBeNull();
  });

  it("membatasi jumlah di stok tersedia", () => {
    const cart = addToCart([], item("a"));
    expect(setQuantity(cart, "a", 99)[0].quantity).toBe(3);
  });

  it("menghapus barang saat jumlahnya 0", () => {
    const cart = addToCart([], item("a"));
    expect(setQuantity(cart, "a", 0)).toEqual([]);
    expect(removeFromCart(cart, "a")).toEqual([]);
  });

  it("hanya mengirim id, jumlah, dan diskon yang berisi", () => {
    let cart = addToCart(addToCart([], item("a")), item("b"));
    cart = setDiscount(cart, "a", { amount_idr: 500, borne_by: "store" });
    cart = setDiscount(cart, "b", { amount_idr: 0, borne_by: "store" });
    expect(toCartRequest(cart)).toEqual({
      lines: [
        { item_id: "a", quantity: 1, discount: { amount_idr: 500, borne_by: "store" } },
        { item_id: "b", quantity: 1 },
      ],
    });
  });
});
