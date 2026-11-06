"use client";

import { useEffect, useRef, useState } from "react";
import { Button, Card, EmptyState, Notice, SelectField, TextField } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { unwrap } from "@/lib/api/client";
import { errorMessage } from "@/lib/api/messages";
import type { components } from "@/lib/api/schema";
import { formatRupiah, formatSol } from "@/lib/format";
import { removeFromCart, setDiscount, setQuantity, toCartRequest, type Cart, type CartEntry } from "./cart";
import { SplitTable } from "./split-table";

type Preview = components["schemas"]["OrderPreview"];
type Order = components["schemas"]["Order"];
type PreviewState = { key: string; data: Preview | null; error: string | null };

const PREVIEW_DELAY_MS = 300;

type CartPanelProps = {
  cart: Cart;
  onChange: (cart: Cart) => void;
  onOrder: (order: Order) => void;
};

export function CartPanel({ cart, onChange, onOrder }: CartPanelProps) {
  const { api } = useSession();
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [busy, setBusy] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const attempt = useRef<{ key: string; idempotencyKey: string } | null>(null);

  const request = toCartRequest(cart);
  const key = JSON.stringify(request);
  const empty = cart.length === 0;

  useEffect(() => {
    if (empty) {
      return;
    }
    let active = true;
    const body = JSON.parse(key) as typeof request;
    const timer = setTimeout(() => {
      unwrap(api.POST("/api/v1/orders/preview", { body })).then(
        (data) => {
          if (active) {
            setPreview({ key, data, error: null });
          }
        },
        (error: unknown) => {
          if (active) {
            setPreview({ key, data: null, error: errorMessage(error) });
          }
        },
      );
    }, PREVIEW_DELAY_MS);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [api, key, empty]);

  const current = preview?.key === key ? preview : null;

  // Kunci yang sama dipakai ulang selama isi keranjang sama, jadi klik ganda tidak membuat order ganda.
  async function createOrder() {
    if (attempt.current?.key !== key) {
      attempt.current = { key, idempotencyKey: crypto.randomUUID() };
    }
    setBusy(true);
    setCreateError(null);
    try {
      const order = await unwrap(
        api.POST("/api/v1/orders", {
          params: { header: { "Idempotency-Key": attempt.current.idempotencyKey } },
          body: request,
        }),
      );
      onOrder(order);
    } catch (error) {
      setCreateError(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className="lg:sticky lg:top-6 lg:self-start">
      <h2 className="text-lg font-semibold">Keranjang</h2>
      {empty ? <EmptyState>Pilih barang dari katalog.</EmptyState> : null}

      <ul className="mt-2 divide-y divide-border">
        {cart.map((entry) => (
          <CartRow key={entry.item.id} entry={entry} cart={cart} onChange={onChange} />
        ))}
      </ul>

      {!empty && !current ? (
        <p role="status" className="mt-4 text-sm text-muted-foreground">
          Menghitung pembagian...
        </p>
      ) : null}
      {!empty && current?.error ? (
        <div className="mt-4">
          <Notice tone="danger">{current.error}</Notice>
        </div>
      ) : null}

      {!empty && current?.data ? (
        <div className="mt-4 border-t border-border pt-4">
          <h3 className="font-semibold">Rincian pembagian</h3>
          <SplitTable recipients={current.data.recipients} />
          {current.data.discount_idr > 0 ? (
            <p className="mt-2 flex justify-between text-sm text-muted-foreground">
              <span>Diskon</span>
              <span>-{formatRupiah(current.data.discount_idr)}</span>
            </p>
          ) : null}
          <p className="mt-2 flex items-baseline justify-between border-t border-border pt-2">
            <span className="font-semibold">Total</span>
            <span className="text-right">
              <span className="block text-xl font-bold">{formatRupiah(current.data.total_idr)}</span>
              <span className="block font-mono text-xs text-muted-foreground">{formatSol(current.data.total_lamports)}</span>
            </span>
          </p>
        </div>
      ) : null}

      {createError ? (
        <div className="mt-4">
          <Notice tone="danger">{createError}</Notice>
        </div>
      ) : null}

      {!empty ? (
        <Button className="mt-4 w-full" disabled={!current?.data || busy} onClick={() => void createOrder()}>
          {busy ? "Membuat QR..." : "Buat QR pembayaran"}
        </Button>
      ) : null}
    </Card>
  );
}

function CartRow({ entry, cart, onChange }: { entry: CartEntry; cart: Cart; onChange: (cart: Cart) => void }) {
  const { item, quantity, discount } = entry;
  const [open, setOpen] = useState(discount !== null);

  return (
    <li className="py-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="font-medium">{item.name}</p>
          <p className="text-sm text-muted-foreground">
            {item.consignor.name} · {formatRupiah(item.price_idr)}
          </p>
        </div>
        <button
          type="button"
          aria-label={`Hapus ${item.name}`}
          onClick={() => onChange(removeFromCart(cart, item.id))}
          className="min-h-11 min-w-11 rounded-lg text-muted-foreground hover:bg-muted focus-visible:outline-2 focus-visible:outline-ring"
        >
          ×
        </button>
      </div>
      <div className="mt-2 flex items-center justify-between gap-2">
        <div className="flex items-center gap-1">
          <Button
            variant="secondary"
            aria-label={`Kurangi ${item.name}`}
            onClick={() => onChange(setQuantity(cart, item.id, quantity - 1))}
          >
            −
          </Button>
          <span aria-label={`Jumlah ${item.name}`} className="min-w-8 text-center font-semibold">
            {quantity}
          </span>
          <Button
            variant="secondary"
            aria-label={`Tambah ${item.name}`}
            disabled={quantity >= item.stock_available}
            onClick={() => onChange(setQuantity(cart, item.id, quantity + 1))}
          >
            +
          </Button>
        </div>
        <button
          type="button"
          aria-expanded={open}
          onClick={() => {
            if (open) {
              onChange(setDiscount(cart, item.id, null));
            }
            setOpen(!open);
          }}
          className="min-h-11 rounded-lg px-2 text-sm font-medium text-accent-foreground underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-ring"
        >
          {open ? "Hapus diskon" : "Beri diskon"}
        </button>
      </div>
      {open ? (
        <div className="mt-2 grid grid-cols-2 gap-2">
          <TextField
            label="Diskon (Rp)"
            type="number"
            min={0}
            inputMode="numeric"
            value={discount?.amount_idr ?? ""}
            onChange={(event) =>
              onChange(
                setDiscount(cart, item.id, {
                  amount_idr: Math.max(0, Math.floor(Number(event.target.value) || 0)),
                  borne_by: discount?.borne_by ?? "store",
                }),
              )
            }
          />
          <SelectField
            label="Ditanggung"
            value={discount?.borne_by ?? "store"}
            onChange={(event) =>
              onChange(
                setDiscount(cart, item.id, {
                  amount_idr: discount?.amount_idr ?? 0,
                  borne_by: event.target.value === "consignor" ? "consignor" : "store",
                }),
              )
            }
          >
            <option value="store">Toko</option>
            <option value="consignor">Penitip</option>
          </SelectField>
        </div>
      ) : null}
    </li>
  );
}
