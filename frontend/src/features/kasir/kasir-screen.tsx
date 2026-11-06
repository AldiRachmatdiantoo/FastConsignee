"use client";

import { useState } from "react";
import type { components } from "@/lib/api/schema";
import { CartPanel } from "./cart-panel";
import { addToCart, type Cart } from "./cart";
import { Catalog } from "./catalog";
import { PaymentPanel } from "./payment-panel";

type Order = components["schemas"]["Order"];

export function KasirScreen() {
  const [cart, setCart] = useState<Cart>([]);
  const [order, setOrder] = useState<Order | null>(null);
  const [round, setRound] = useState(0);

  // Katalog dimuat ulang tiap kembali dari layar bayar, karena stok sudah berubah.
  function backToCart() {
    setOrder(null);
    setRound((value) => value + 1);
  }

  if (order) {
    return (
      <>
        <h1 className="sr-only">Kasir</h1>
        <PaymentPanel
          order={order}
          onRetry={backToCart}
          onDone={() => {
            setCart([]);
            backToCart();
          }}
        />
      </>
    );
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_24rem]">
      <h1 className="sr-only">Kasir</h1>
      <Catalog key={round} cart={cart} onAdd={(item) => setCart((current) => addToCart(current, item))} />
      <CartPanel cart={cart} onChange={setCart} onOrder={setOrder} />
    </div>
  );
}
