import type { components } from "@/lib/api/schema";

export type Item = components["schemas"]["Item"];
export type LineDiscount = components["schemas"]["LineDiscount"];
export type CartRequest = components["schemas"]["CartRequest"];

export type CartEntry = { item: Item; quantity: number; discount: LineDiscount | null };
export type Cart = CartEntry[];

export const MAX_CONSIGNORS = 5;

export type AddBlock = "out_of_stock" | "too_many_consignors" | null;

// addBlock menjelaskan alasan barang tidak bisa ditambah, atau null kalau boleh.
export function addBlock(cart: Cart, item: Item): AddBlock {
  const entry = cart.find((line) => line.item.id === item.id);
  if ((entry?.quantity ?? 0) >= item.stock_available) {
    return "out_of_stock";
  }
  const consignors = new Set(cart.map((line) => line.item.consignor.id));
  if (!consignors.has(item.consignor.id) && consignors.size >= MAX_CONSIGNORS) {
    return "too_many_consignors";
  }
  return null;
}

export function addToCart(cart: Cart, item: Item): Cart {
  if (addBlock(cart, item)) {
    return cart;
  }
  if (cart.some((line) => line.item.id === item.id)) {
    return cart.map((line) => (line.item.id === item.id ? { ...line, quantity: line.quantity + 1 } : line));
  }
  return [...cart, { item, quantity: 1, discount: null }];
}

export function removeFromCart(cart: Cart, itemId: string): Cart {
  return cart.filter((line) => line.item.id !== itemId);
}

// setQuantity menjaga jumlah di antara 1 dan stok tersedia. Nilai 0 menghapus barang.
export function setQuantity(cart: Cart, itemId: string, quantity: number): Cart {
  if (quantity <= 0) {
    return removeFromCart(cart, itemId);
  }
  return cart.map((line) =>
    line.item.id === itemId ? { ...line, quantity: Math.min(quantity, line.item.stock_available) } : line,
  );
}

export function setDiscount(cart: Cart, itemId: string, discount: LineDiscount | null): Cart {
  return cart.map((line) => (line.item.id === itemId ? { ...line, discount } : line));
}

// toCartRequest hanya mengirim barang, jumlah, dan diskon. Harga selalu dihitung server.
export function toCartRequest(cart: Cart): CartRequest {
  return {
    lines: cart.map(({ item, quantity, discount }) => ({
      item_id: item.id,
      quantity,
      ...(discount && discount.amount_idr > 0 ? { discount } : {}),
    })),
  };
}
