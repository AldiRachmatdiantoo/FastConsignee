"use client";

import { useConnection, useWallet } from "@solana/wallet-adapter-react";
import { useCallback, useRef, useState } from "react";
import { Button, Card, EmptyState, ExplorerLink, InlineLoading, ListSkeleton, LoadMore, Notice, PageTitle, SkeletonBlock } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { shortAddress } from "@/features/wallet/link-wallet";
import { decodeTransaction } from "@/features/wallet/transaction";
import { SolanaProvider } from "@/features/wallet/solana-provider";
import { WalletPicker } from "@/features/wallet/wallet-picker";
import { ApiError, ApiUnavailableError, unwrap } from "@/lib/api/client";
import { errorMessage } from "@/lib/api/messages";
import type { components } from "@/lib/api/schema";
import { formatDateTime, formatRupiah, formatSol } from "@/lib/format";
import { usePagedList, usePolling, useResource } from "@/lib/hooks";

type OrderSummary = components["schemas"]["OrderSummary"];
type OrderLine = components["schemas"]["OrderLine"];
type Refund = components["schemas"]["Refund"];
type RefundDraft = components["schemas"]["RefundDraft"];
type RefundStatus = components["schemas"]["RefundStatus"];

const POLL_MS = 2000;
// Order dimuat lima-lima. Refund biasanya untuk order terbaru, dan daftar pendek membuat rincian order tetap dekat di HP.
const ORDER_PAGE = 5;

const STATUS_LABELS: Record<RefundStatus, string> = {
  awaiting_signature: "Menunggu tanda tangan",
  confirming: "Menunggu konfirmasi",
  completed: "Selesai",
  failed: "Gagal",
  expired: "Kedaluwarsa",
};

export function RefundScreen() {
  const [connectFailed, setConnectFailed] = useState(false);
  const onError = useCallback(() => setConnectFailed(true), []);

  return (
    <SolanaProvider onError={onError}>
      <RefundContent connectFailed={connectFailed} resetConnect={() => setConnectFailed(false)} />
    </SolanaProvider>
  );
}

function RefundContent({ connectFailed, resetConnect }: { connectFailed: boolean; resetConnect: () => void }) {
  const { session, api } = useSession();
  const [orderId, setOrderId] = useState<string | null>(null);
  const [round, setRound] = useState(0);

  const loadOrders = useCallback(
    (cursor?: string) => unwrap(api.GET("/api/v1/orders", { params: { query: { status: "confirmed", limit: ORDER_PAGE, cursor } } })),
    [api],
  );
  const orders = usePagedList<OrderSummary>(loadOrders);

  if (session.status !== "authenticated") {
    return null;
  }

  return (
    <div className="grid gap-6">
      <PageTitle
        title="Refund"
        text="Refund dikirim dari wallet toko ke wallet pembeli. Bagian penitip dicatat sebagai potongan di penjualan berikutnya."
      />
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <h2 className="text-lg font-semibold">Order terkonfirmasi</h2>
          {orders.message ? <Notice tone="danger">{orders.message}</Notice> : null}
          {/* Kerangka setinggi satu halaman penuh beserta tombol "Muat lagi", jadi rincian dan riwayat di bawahnya tidak bergeser. */}
          {orders.status === "loading" ? (
            <>
              <ListSkeleton rows={ORDER_PAGE} boxed className="mt-2" />
              <div className="mt-4 flex justify-center">
                <SkeletonBlock className="h-11 w-28 rounded-lg" />
              </div>
            </>
          ) : null}
          {orders.status === "ready" && orders.items.length === 0 ? <EmptyState>Belum ada order.</EmptyState> : null}
          <ul className="mt-2 grid gap-2 empty:hidden">
            {orders.items.map((order) => (
              <li key={order.id}>
                <button
                  type="button"
                  aria-pressed={order.id === orderId}
                  onClick={() => setOrderId(order.id)}
                  className={`flex w-full items-baseline justify-between gap-3 rounded-lg border p-3 text-left text-sm hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring ${
                    order.id === orderId ? "border-primary" : "border-border"
                  }`}
                >
                  <span>
                    <span className="block font-semibold">{formatRupiah(order.total_idr)}</span>
                    <span className="block text-muted-foreground">{order.item_count} barang</span>
                  </span>
                  <span className="text-muted-foreground">{formatDateTime(order.confirmed_at ?? order.created_at)}</span>
                </button>
              </li>
            ))}
          </ul>
          <LoadMore visible={orders.cursor !== null} busy={orders.status === "more"} onClick={() => void orders.loadMore()} />
        </Card>

        {orderId ? (
          <OrderRefund
            key={orderId}
            orderId={orderId}
            storeWallet={session.me.wallet?.address ?? null}
            connectFailed={connectFailed}
            resetConnect={resetConnect}
            onRefunded={() => setRound((value) => value + 1)}
          />
        ) : (
          <Card>
            <EmptyState>Pilih order untuk melihat barangnya.</EmptyState>
          </Card>
        )}
      </div>
      <RefundHistory key={round} />
    </div>
  );
}

type OrderRefundProps = {
  orderId: string;
  storeWallet: string | null;
  connectFailed: boolean;
  resetConnect: () => void;
  onRefunded: () => void;
};

function OrderRefund({ orderId, storeWallet, connectFailed, resetConnect, onRefunded }: OrderRefundProps) {
  const { api } = useSession();
  const { connection } = useConnection();
  const { publicKey, sendTransaction, disconnect } = useWallet();
  const [draft, setDraft] = useState<RefundDraft | null>(null);
  const [tracked, setTracked] = useState<Refund | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const attempt = useRef<{ lineId: string; idempotencyKey: string } | null>(null);

  const load = useCallback(
    () => unwrap(api.GET("/api/v1/orders/{order_id}", { params: { path: { order_id: orderId } } })),
    [api, orderId],
  );
  const { state, reload } = useResource(load);

  const address = publicKey?.toBase58() ?? null;
  const rightWallet = address !== null && address === storeWallet;
  const message = error ?? (connectFailed ? "Wallet tidak bisa dihubungkan. Buka wallet, lalu coba lagi." : null);

  const poll = useCallback(async () => {
    if (!tracked) {
      return;
    }
    const next = await unwrap(api.GET("/api/v1/refunds/{refund_id}", { params: { path: { refund_id: tracked.id } } }));
    setTracked(next);
    if (next.status !== "confirming" && next.status !== "awaiting_signature") {
      reload();
      onRefunded();
    }
  }, [api, tracked, reload, onRefunded]);
  usePolling(poll, POLL_MS, tracked?.status === "confirming" || tracked?.status === "awaiting_signature");

  async function signAndSend(current: RefundDraft) {
    const transaction = decodeTransaction(current.transaction);
    const signature = await sendTransaction(transaction, connection);
    setDraft(null);
    setTracked(current);
    const updated = await unwrap(
      api.POST("/api/v1/refunds/{refund_id}/signature", {
        params: { path: { refund_id: current.id } },
        body: { signature },
      }),
    ).catch(() => null);
    if (updated) {
      setTracked(updated);
    }
  }

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError(null);
    try {
      await action();
    } catch (failure) {
      const fromApi = failure instanceof ApiError || failure instanceof ApiUnavailableError;
      setError(fromApi ? errorMessage(failure) : "Tanda tangan dibatalkan atau gagal di wallet. Saldo tidak berubah.");
    } finally {
      setBusy(false);
    }
  }

  // Kunci yang sama dipakai ulang kalau jawaban API tidak sampai, jadi refund tidak terbuat dua kali.
  async function start(line: OrderLine) {
    if (attempt.current?.lineId !== line.id) {
      attempt.current = { lineId: line.id, idempotencyKey: crypto.randomUUID() };
    }
    const created = await unwrap(
      api.POST("/api/v1/orders/{order_id}/refunds", {
        params: { path: { order_id: orderId }, header: { "Idempotency-Key": attempt.current.idempotencyKey } },
        body: { order_line_id: line.id },
      }),
    );
    attempt.current = null;
    setDraft(created);
    setTracked(null);
    await signAndSend(created);
  }

  return (
    <Card className="grid content-start gap-4">
      <h2 className="text-lg font-semibold">Barang di order ini</h2>
      {state.status === "loading" ? <InlineLoading /> : null}
      {state.status === "error" ? <Notice tone="danger">{state.message}</Notice> : null}
      {message ? <Notice tone="danger">{message}</Notice> : null}

      {tracked ? (
        <Notice tone={tracked.status === "completed" ? "success" : tracked.status === "failed" || tracked.status === "expired" ? "danger" : "info"}>
          Refund {tracked.item_name}: {STATUS_LABELS[tracked.status]}
        </Notice>
      ) : null}
      {tracked?.status === "completed" && tracked.explorer_url ? <ExplorerLink href={tracked.explorer_url} /> : null}

      {address ? (
        <div className="text-sm">
          <p className="text-muted-foreground">Wallet terhubung</p>
          <p className="font-mono">{shortAddress(address)}</p>
          {!rightWallet ? (
            <div className="mt-2">
              <Notice tone="danger">Ini bukan wallet toko. Pakai wallet yang tertaut ke toko.</Notice>
            </div>
          ) : null}
          <Button variant="secondary" className="mt-2" disabled={busy} onClick={() => void disconnect()}>
            Ganti wallet
          </Button>
        </div>
      ) : (
        <WalletPicker
          onPick={() => {
            setError(null);
            resetConnect();
          }}
        />
      )}

      {draft ? (
        <Button disabled={busy || !rightWallet} onClick={() => void run(() => signAndSend(draft))}>
          {busy ? "Menunggu wallet..." : "Tandatangani lagi"}
        </Button>
      ) : null}

      {state.status === "ready" ? (
        <ul className="divide-y divide-border text-sm">
          {state.data.lines.map((line) => {
            const remaining = line.quantity - line.refunded_quantity;
            return (
              <li key={line.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                <div>
                  <p className="font-semibold">
                    {line.item_name} × {line.quantity}
                  </p>
                  <p className="text-muted-foreground">
                    {line.consignor.name} · {line.refunded_quantity} sudah di-refund
                  </p>
                </div>
                <Button
                  variant="secondary"
                  disabled={busy || remaining <= 0 || !rightWallet || draft !== null}
                  aria-label={`Refund satu ${line.item_name}`}
                  onClick={() => void run(() => start(line))}
                >
                  Refund 1 unit
                </Button>
              </li>
            );
          })}
        </ul>
      ) : null}
    </Card>
  );
}

function RefundHistory() {
  const { api } = useSession();
  const load = useCallback(
    (cursor?: string) => unwrap(api.GET("/api/v1/refunds", { params: { query: { limit: 20, cursor } } })),
    [api],
  );
  const { items, cursor, status, message, loadMore } = usePagedList<Refund>(load);

  return (
    <Card>
      <h2 className="text-lg font-semibold">Riwayat refund</h2>
      {message ? <Notice tone="danger">{message}</Notice> : null}
      {status === "loading" ? <ListSkeleton rows={2} lines={3} className="mt-2" /> : null}
      {status === "ready" && items.length === 0 ? <EmptyState>Belum ada refund.</EmptyState> : null}
      <ul className="mt-2 divide-y divide-border text-sm empty:hidden">
        {items.map((refund) => (
          <li key={refund.id} className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 py-3">
            <div>
              <p className="font-semibold">{refund.item_name}</p>
              <p className="text-muted-foreground">
                {STATUS_LABELS[refund.status]} · {formatDateTime(refund.created_at)}
              </p>
            </div>
            <div className="text-right">
              <p className="font-semibold">{formatRupiah(refund.amount_idr)}</p>
              <p className="font-mono text-xs text-muted-foreground">{formatSol(refund.amount_lamports)}</p>
              {refund.explorer_url ? <ExplorerLink href={refund.explorer_url}>Explorer</ExplorerLink> : null}
            </div>
          </li>
        ))}
      </ul>
      <LoadMore visible={cursor !== null} busy={status === "more"} onClick={() => void loadMore()} />
    </Card>
  );
}
