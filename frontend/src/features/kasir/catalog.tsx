"use client";

import { useCallback, useState } from "react";
import { Card, EmptyState, LoadingBlock, LoadMore, Notice, Skeleton, TextField } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { unwrap } from "@/lib/api/client";
import { formatRupiah } from "@/lib/format";
import { useDebounced, usePagedList } from "@/lib/hooks";
import { addBlock, type Cart, type Item } from "./cart";

const BLOCK_LABELS = {
  out_of_stock: "Stok habis",
  too_many_consignors: "Keranjang sudah 5 penitip",
};

// Enam kotak menahan tempat katalog selama dimuat, cukup untuk mendorong keranjang ke bawah layar HP seperti katalog asli.
function CatalogSkeleton() {
  return (
    <LoadingBlock className="mt-4 grid gap-2 sm:grid-cols-2">
      {Array.from({ length: 6 }, (_, item) => (
        <div key={item} className="flex min-h-24 flex-col justify-between rounded-lg border border-border p-3">
          <Skeleton className="w-3/5" />
          <Skeleton className="w-2/5 text-sm" />
          <div className="mt-2 flex justify-between gap-2">
            <Skeleton className="w-1/3" />
            <Skeleton className="w-1/5 text-sm" />
          </div>
        </div>
      ))}
    </LoadingBlock>
  );
}

export function Catalog({ cart, onAdd }: { cart: Cart; onAdd: (item: Item) => void }) {
  const { api } = useSession();
  const [search, setSearch] = useState("");
  const query = useDebounced(search.trim(), 250);

  const load = useCallback(
    (cursor?: string) =>
      unwrap(api.GET("/api/v1/items", { params: { query: { q: query || undefined, active: true, limit: 50, cursor } } })),
    [api, query],
  );
  const { items, cursor, status, message, loadMore } = usePagedList<Item>(load);

  return (
    <Card>
      <h2 className="text-lg font-semibold">Katalog</h2>
      <div className="mt-3">
        <TextField
          label="Cari barang"
          type="search"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Nama barang"
        />
      </div>
      {message ? (
        <div className="mt-3">
          <Notice tone="danger">{message}</Notice>
        </div>
      ) : null}
      {status === "loading" ? <CatalogSkeleton /> : null}
      {status !== "loading" && status !== "error" && items.length === 0 ? <EmptyState>Barang tidak ditemukan.</EmptyState> : null}
      <ul className="mt-4 grid gap-2 empty:hidden sm:grid-cols-2">
        {items.map((item) => {
          const block = addBlock(cart, item);
          return (
            <li key={item.id}>
              <button
                type="button"
                disabled={block !== null}
                onClick={() => onAdd(item)}
                className="flex min-h-24 w-full flex-col justify-between rounded-lg border border-border bg-background p-3 text-left hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-60"
              >
                <span className="font-medium">{item.name}</span>
                <span className="text-sm text-muted-foreground">{item.consignor.name}</span>
                <span className="mt-2 flex items-baseline justify-between gap-2">
                  <span className="font-semibold">{formatRupiah(item.price_idr)}</span>
                  <span className="text-sm text-muted-foreground">
                    {block ? BLOCK_LABELS[block] : `Stok ${item.stock_available}`}
                  </span>
                </span>
              </button>
            </li>
          );
        })}
      </ul>
      <LoadMore visible={cursor !== null} busy={status === "more"} onClick={() => void loadMore()} />
    </Card>
  );
}
