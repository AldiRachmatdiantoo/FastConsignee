"use client";

import { useCallback, useState, type FormEvent } from "react";
import {
  Button,
  Card,
  EmptyState,
  ListSkeleton,
  LoadMore,
  Notice,
  PageTitle,
  SelectField,
  TextField,
} from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { unwrap } from "@/lib/api/client";
import { errorMessage } from "@/lib/api/messages";
import type { components } from "@/lib/api/schema";
import { formatPercent, formatRupiah } from "@/lib/format";
import { useDebounced, usePagedList, useResource } from "@/lib/hooks";
import { EMPTY_FORM, formFromItem, toCreate, toUpdate, type ItemForm } from "./item-form";

type Item = components["schemas"]["Item"];

export function BarangScreen() {
  const { api } = useSession();
  const [search, setSearch] = useState("");
  const [active, setActive] = useState(true);
  const [editing, setEditing] = useState<Item | "new" | null>(null);
  const query = useDebounced(search.trim(), 250);

  const load = useCallback(
    (cursor?: string) =>
      unwrap(api.GET("/api/v1/items", { params: { query: { q: query || undefined, active, limit: 50, cursor } } })),
    [api, query, active],
  );
  const { items, cursor, status, message, loadMore, reload } = usePagedList<Item>(load);

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <PageTitle title="Items" text="New prices and commissions apply from the next sale." />
        <Button onClick={() => setEditing("new")}>Add item</Button>
      </div>

      {editing ? (
        <ItemEditor
          key={editing === "new" ? "new" : editing.id}
          item={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            reload();
          }}
        />
      ) : null}

      <Card>
        <div className="grid gap-3 sm:grid-cols-[1fr_12rem]">
          <TextField
            label="Search items"
            type="search"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Item name"
          />
          <SelectField
            label="Show"
            value={active ? "active" : "inactive"}
            onChange={(event) => setActive(event.target.value === "active")}
          >
            <option value="active">Active items</option>
            <option value="inactive">Inactive items</option>
          </SelectField>
        </div>
        {message ? (
          <div className="mt-3">
            <Notice tone="danger">{message}</Notice>
          </div>
        ) : null}
        {status === "loading" ? <ListSkeleton rows={5} lines={3} action className="mt-2" /> : null}
        {status === "ready" && items.length === 0 ? <EmptyState>No items found.</EmptyState> : null}
        <ul className="mt-2 divide-y divide-border text-sm empty:hidden">
          {items.map((item) => (
            <li key={item.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
              <div className="min-w-0">
                <p className="font-semibold">
                  {item.name}
                  {item.active ? "" : " (nonaktif)"}
                </p>
                <p className="text-muted-foreground">
                  {item.consignor.name} · {formatRupiah(item.price_idr)} · commission {formatPercent(item.commission_bps)}
                </p>
                <p className="text-muted-foreground">
                  Stock {item.stock_on_hand}, held {item.stock_reserved}, available {item.stock_available}
                </p>
              </div>
              <Button variant="secondary" aria-label={`Edit ${item.name}`} onClick={() => setEditing(item)}>
                Edit
              </Button>
            </li>
          ))}
        </ul>
        <LoadMore visible={cursor !== null} busy={status === "more"} onClick={() => void loadMore()} />
      </Card>
    </div>
  );
}

type ItemEditorProps = {
  item: Item | null;
  onClose: () => void;
  onSaved: () => void;
};

function ItemEditor({ item, onClose, onSaved }: ItemEditorProps) {
  const { api } = useSession();
  const [form, setForm] = useState<ItemForm>(item ? formFromItem(item) : EMPTY_FORM);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadConsignors = useCallback(
    () => unwrap(api.GET("/api/v1/consignors", { params: { query: { limit: 100 } } })),
    [api],
  );
  const { state } = useResource(loadConsignors);

  function update(next: Partial<ItemForm>) {
    setForm((current) => ({ ...current, ...next }));
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (item) {
        await unwrap(
          api.PATCH("/api/v1/items/{item_id}", { params: { path: { item_id: item.id } }, body: toUpdate(form) }),
        );
      } else {
        await unwrap(api.POST("/api/v1/items", { body: toCreate(form) }));
      }
      onSaved();
    } catch (failure) {
      setError(errorMessage(failure));
      setBusy(false);
    }
  }

  return (
    <Card>
      <h2 className="text-lg font-semibold">{item ? `Edit ${item.name}` : "New item"}</h2>
      <form className="mt-3 grid gap-3 sm:grid-cols-2" onSubmit={(event) => void submit(event)}>
        <TextField label="Name" required value={form.name} onChange={(event) => update({ name: event.target.value })} />
        {item ? (
          <TextField label="Consignor" value={item.consignor.name} disabled />
        ) : (
          <SelectField
            label="Consignor"
            required
            value={form.consignorId}
            onChange={(event) => update({ consignorId: event.target.value })}
          >
            <option value="">Choose a consignor</option>
            {state.status === "ready"
              ? state.data.data.map((consignor) => (
                  <option key={consignor.id} value={consignor.id}>
                    {consignor.name}
                  </option>
                ))
              : null}
          </SelectField>
        )}
        <TextField
          label="Price (Rp)"
          type="number"
          min={1}
          required
          value={form.price}
          onChange={(event) => update({ price: event.target.value })}
        />
        <TextField
          label="Shop commission (%)"
          type="number"
          min={0}
          max={100}
          step="0.01"
          required
          value={form.commission}
          onChange={(event) => update({ commission: event.target.value })}
        />
        <TextField
          label="Stock"
          type="number"
          min={0}
          required
          value={form.stock}
          onChange={(event) => update({ stock: event.target.value })}
        />
        <label className="flex min-h-11 items-center gap-2 self-end text-sm font-medium">
          <input
            type="checkbox"
            checked={form.certificate}
            onChange={(event) => update({ certificate: event.target.checked })}
          />
          Send an NFT certificate to the buyer
        </label>
        {item ? (
          <label className="flex min-h-11 items-center gap-2 self-end text-sm font-medium">
            <input type="checkbox" checked={form.active} onChange={(event) => update({ active: event.target.checked })} />
            Active in catalog
          </label>
        ) : null}
        {error ? (
          <div className="sm:col-span-2">
            <Notice tone="danger">{error}</Notice>
          </div>
        ) : null}
        <div className="flex gap-2 sm:col-span-2">
          <Button type="submit" disabled={busy}>
            {busy ? "Saving..." : "Save"}
          </Button>
          <Button variant="secondary" disabled={busy} onClick={onClose}>
            Cancel
          </Button>
        </div>
      </form>
    </Card>
  );
}
