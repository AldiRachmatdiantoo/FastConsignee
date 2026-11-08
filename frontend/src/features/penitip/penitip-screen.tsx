"use client";

import { useCallback, useState, type FormEvent } from "react";
import { Button, Card, EmptyState, ListSkeleton, LoadMore, Notice, PageTitle, TextField } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { shortAddress } from "@/features/wallet/link-wallet";
import { unwrap } from "@/lib/api/client";
import { errorMessage } from "@/lib/api/messages";
import type { components } from "@/lib/api/schema";
import { usePagedList } from "@/lib/hooks";

type Consignor = components["schemas"]["Consignor"];
type WalletStatus = components["schemas"]["WalletStatus"];

const WALLET_LABELS: Record<WalletStatus, string> = {
  unlinked: "Belum ada wallet",
  unverified: "Belum ditandatangani penitip",
  verified: "Terverifikasi",
};

export function PenitipScreen() {
  const { api } = useSession();
  const [editing, setEditing] = useState<Consignor | "new" | null>(null);

  const load = useCallback(
    (cursor?: string) => unwrap(api.GET("/api/v1/consignors", { params: { query: { limit: 50, cursor } } })),
    [api],
  );
  const { items, cursor, status, message, loadMore, reload } = usePagedList<Consignor>(load);

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <PageTitle title="Penitip" text="Penitip login dengan email ini, lalu menautkan wallet-nya sendiri." />
        <Button onClick={() => setEditing("new")}>Tambah penitip</Button>
      </div>

      {editing ? (
        <ConsignorEditor
          key={editing === "new" ? "new" : editing.id}
          consignor={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            reload();
          }}
        />
      ) : null}

      <Card>
        {message ? <Notice tone="danger">{message}</Notice> : null}
        {status === "loading" ? <ListSkeleton rows={4} lines={3} action /> : null}
        {status === "ready" && items.length === 0 ? <EmptyState>Belum ada penitip.</EmptyState> : null}
        <ul className="divide-y divide-border text-sm empty:hidden">
          {items.map((consignor) => (
            <li key={consignor.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
              <div className="min-w-0">
                <p className="font-semibold">{consignor.name}</p>
                <p className="break-all text-muted-foreground">{consignor.email}</p>
                <p className="text-muted-foreground">
                  {WALLET_LABELS[consignor.wallet_status]}
                  {consignor.wallet_address ? (
                    <span className="ml-1 font-mono text-xs">{shortAddress(consignor.wallet_address)}</span>
                  ) : null}
                </p>
              </div>
              <Button variant="secondary" aria-label={`Ubah ${consignor.name}`} onClick={() => setEditing(consignor)}>
                Ubah
              </Button>
            </li>
          ))}
        </ul>
        <LoadMore visible={cursor !== null} busy={status === "more"} onClick={() => void loadMore()} />
      </Card>
    </div>
  );
}

type ConsignorEditorProps = {
  consignor: Consignor | null;
  onClose: () => void;
  onSaved: () => void;
};

function ConsignorEditor({ consignor, onClose, onSaved }: ConsignorEditorProps) {
  const { api } = useSession();
  const [name, setName] = useState(consignor?.name ?? "");
  const [email, setEmail] = useState("");
  const [wallet, setWallet] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (consignor) {
        await unwrap(
          api.PATCH("/api/v1/consignors/{consignor_id}", {
            params: { path: { consignor_id: consignor.id } },
            body: { name: name.trim() },
          }),
        );
      } else {
        await unwrap(
          api.POST("/api/v1/consignors", {
            body: { name: name.trim(), email: email.trim(), ...(wallet.trim() ? { wallet_address: wallet.trim() } : {}) },
          }),
        );
      }
      onSaved();
    } catch (failure) {
      setError(errorMessage(failure));
      setBusy(false);
    }
  }

  return (
    <Card>
      <h2 className="text-lg font-semibold">{consignor ? `Ubah ${consignor.name}` : "Penitip baru"}</h2>
      <form className="mt-3 grid gap-3 sm:grid-cols-2" onSubmit={(event) => void submit(event)}>
        <TextField label="Nama" required value={name} onChange={(event) => setName(event.target.value)} />
        {consignor ? null : (
          <>
            <TextField label="Email" type="email" required value={email} onChange={(event) => setEmail(event.target.value)} />
            <div className="sm:col-span-2">
              <TextField
                label="Alamat wallet (tidak wajib)"
                value={wallet}
                onChange={(event) => setWallet(event.target.value)}
              />
            </div>
          </>
        )}
        {error ? (
          <div className="sm:col-span-2">
            <Notice tone="danger">{error}</Notice>
          </div>
        ) : null}
        <div className="flex gap-2 sm:col-span-2">
          <Button type="submit" disabled={busy}>
            {busy ? "Menyimpan..." : "Simpan"}
          </Button>
          <Button variant="secondary" disabled={busy} onClick={onClose}>
            Batal
          </Button>
        </div>
      </form>
    </Card>
  );
}
