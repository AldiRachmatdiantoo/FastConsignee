"use client";

import { useConnection, useWallet } from "@solana/wallet-adapter-react";
import { useCallback, useState } from "react";
import { PaymentQr } from "@/components/payment-qr";
import { Button, Card, ExplorerLink, InlineLoading, Notice } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { shortAddress } from "@/features/wallet/link-wallet";
import { SolanaProvider } from "@/features/wallet/solana-provider";
import { WalletPicker } from "@/features/wallet/wallet-picker";
import { unwrap } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatCountdown, formatDateTime, formatRupiah, formatSol, secondsUntil } from "@/lib/format";
import { useNow, usePolling, useResource } from "@/lib/hooks";
import { payErrorMessage, requestTransaction } from "./pay";

type PublicOrder = components["schemas"]["PublicOrder"];

// Endpoint publik dibatasi 30 permintaan per menit per order, jadi polling di sini lebih longgar.
const POLL_MS = 3000;

export function BayarScreen({ orderId }: { orderId: string }) {
  const [connectFailed, setConnectFailed] = useState(false);
  const onError = useCallback(() => setConnectFailed(true), []);

  return (
    <SolanaProvider onError={onError}>
      <main className="mx-auto grid min-h-dvh max-w-lg content-start gap-4 px-4 py-8">
        <header className="flex items-center justify-between gap-3">
          <p className="font-semibold text-accent-foreground">FastConsignee</p>
          <span className="rounded-full bg-accent px-2.5 py-1 text-xs font-semibold text-accent-foreground">Devnet</span>
        </header>
        <OrderView orderId={orderId} connectFailed={connectFailed} resetConnect={() => setConnectFailed(false)} />
      </main>
    </SolanaProvider>
  );
}

type OrderViewProps = {
  orderId: string;
  connectFailed: boolean;
  resetConnect: () => void;
};

function OrderView({ orderId, connectFailed, resetConnect }: OrderViewProps) {
  const { api } = useSession();
  const [latest, setLatest] = useState<PublicOrder | null>(null);

  const load = useCallback(
    () => unwrap(api.GET("/api/v1/public/orders/{order_id}", { params: { path: { order_id: orderId } } })),
    [api, orderId],
  );
  const { state } = useResource(load);
  const order = latest ?? (state.status === "ready" ? state.data : null);

  const poll = useCallback(async () => setLatest(await load()), [load]);
  usePolling(poll, POLL_MS, order?.status === "pending");

  if (state.status === "error") {
    return <Notice tone="danger">{state.message}</Notice>;
  }
  if (!order) {
    return <InlineLoading />;
  }

  return (
    <>
      <Card>
        <h1 className="text-lg font-semibold">{order.store_name}</h1>
        <ul className="mt-3 divide-y divide-border text-sm">
          {order.lines.map((line) => (
            <li key={line.item_name} className="flex justify-between gap-3 py-2">
              <span>
                {line.item_name} × {line.quantity}
              </span>
              <span className="font-medium whitespace-nowrap">{formatRupiah(line.line_total_idr)}</span>
            </li>
          ))}
        </ul>
        <div className="mt-3 flex items-baseline justify-between border-t border-border pt-3">
          <span className="font-semibold">Total</span>
          <span className="text-right">
            <span className="block text-2xl font-bold">{formatRupiah(order.total_idr)}</span>
            <span className="block font-mono text-sm text-muted-foreground">{formatSol(order.total_lamports)}</span>
          </span>
        </div>
        <p className="mt-2 text-xs text-muted-foreground">
          Kurs terkunci {formatRupiah(order.rate.idr_per_sol)} per SOL. Ini jaringan Devnet, jadi tidak memakai uang sungguhan.
        </p>
      </Card>

      {order.status === "pending" ? (
        <PendingPayment order={order} connectFailed={connectFailed} resetConnect={resetConnect} />
      ) : null}
      {order.status === "confirmed" ? <Receipt order={order} /> : null}
      {order.status === "expired" ? <Notice tone="danger">QR sudah kedaluwarsa. Minta kasir membuat QR baru.</Notice> : null}
      {order.status === "failed" ? (
        <Notice tone="danger">Pembayaran gagal, dan saldo tidak berubah. Minta kasir membuat QR baru untuk mencoba lagi.</Notice>
      ) : null}
    </>
  );
}

type PendingPaymentProps = {
  order: PublicOrder;
  connectFailed: boolean;
  resetConnect: () => void;
};

function PendingPayment({ order, connectFailed, resetConnect }: PendingPaymentProps) {
  const { api } = useSession();
  const { connection } = useConnection();
  const { publicKey, sendTransaction, disconnect } = useWallet();
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const now = useNow(true);

  const message = error ?? (connectFailed ? "Wallet tidak bisa dihubungkan. Buka wallet, lalu coba lagi." : null);

  async function pay() {
    if (!publicKey) {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const transaction = await requestTransaction(api, order.id, publicKey.toBase58());
      await sendTransaction(transaction, connection);
      setSent(true);
    } catch (failure) {
      setError(payErrorMessage(failure));
    } finally {
      setBusy(false);
    }
  }

  if (sent) {
    return (
      <Card>
        <p role="status" className="font-semibold">
          Transaksi terkirim. Menunggu konfirmasi jaringan...
        </p>
      </Card>
    );
  }

  return (
    <Card className="grid gap-4">
      <p role="status" className="text-center font-semibold">
        Bayar dalam {formatCountdown(secondsUntil(order.expires_at, now))}
      </p>
      {message ? <Notice tone="danger">{message}</Notice> : null}

      {publicKey ? (
        <div>
          <p className="text-sm text-muted-foreground">Wallet terhubung</p>
          <p className="mt-1 font-mono text-sm">{shortAddress(publicKey.toBase58())}</p>
          <div className="mt-4 flex flex-wrap gap-2">
            <Button className="flex-1" disabled={busy} onClick={() => void pay()}>
              {busy ? "Menunggu wallet..." : "Bayar sekali"}
            </Button>
            <Button variant="secondary" disabled={busy} onClick={() => void disconnect()}>
              Ganti wallet
            </Button>
          </div>
        </div>
      ) : (
        <WalletPicker
          onPick={() => {
            setError(null);
            resetConnect();
          }}
        />
      )}

      <div className="border-t border-border pt-4 text-center">
        <p className="text-sm text-muted-foreground">Atau pindai dengan Phantom di HP</p>
        <div className="mt-3">
          <PaymentQr value={order.solana_pay_url} />
        </div>
      </div>
    </Card>
  );
}

function Receipt({ order }: { order: PublicOrder }) {
  return (
    <Card className="grid gap-3">
      <Notice tone="success">Pembayaran terkonfirmasi. Terima kasih.</Notice>
      {order.confirmed_at ? <p className="text-sm text-muted-foreground">{formatDateTime(order.confirmed_at)}</p> : null}
      {order.explorer_url ? <ExplorerLink href={order.explorer_url} /> : null}
      {order.lines
        .filter(({ certificate }) => certificate?.explorer_url)
        .map(({ item_name: name, certificate }) => (
          <ExplorerLink key={name} href={certificate?.explorer_url ?? ""}>
            Sertifikat {name}
          </ExplorerLink>
        ))}
    </Card>
  );
}
