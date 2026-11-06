"use client";

import dynamic from "next/dynamic";
import { useCallback, useState } from "react";
import { PaymentQr } from "@/components/payment-qr";
import { Card, ExplorerLink, InlineLoading, Notice } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { unwrap } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatCountdown, formatDateTime, formatRupiah, formatSol, secondsUntil } from "@/lib/format";
import { useNow, usePolling, useResource } from "@/lib/hooks";

type PublicOrder = components["schemas"]["PublicOrder"];

// Endpoint publik dibatasi 30 permintaan per menit per order, jadi polling di sini lebih longgar.
const POLL_MS = 3000;

function WalletUnavailable() {
  return <Notice tone="danger">Wallet tidak bisa dimuat. Muat ulang halaman, atau pindai QR di bawah.</Notice>;
}

// Kalau berkas wallet gagal dimuat, pembeli masih bisa membayar lewat QR.
const WalletPayment = dynamic(
  () =>
    import("./wallet-payment")
      .then((module) => module.WalletPayment)
      .catch(() => WalletUnavailable),
  { ssr: false, loading: () => <InlineLoading /> },
);

export function BayarScreen({ orderId }: { orderId: string }) {
  return (
    <main className="mx-auto grid min-h-dvh max-w-lg content-start gap-4 px-4 py-8">
      <header className="flex items-center justify-between gap-3">
        <p className="font-semibold text-accent-foreground">FastConsignee</p>
        <span className="rounded-full bg-accent px-2.5 py-1 text-xs font-semibold text-accent-foreground">Devnet</span>
      </header>
      <OrderView orderId={orderId} />
    </main>
  );
}

function OrderView({ orderId }: { orderId: string }) {
  const { api } = useSession();
  const [latest, setLatest] = useState<PublicOrder | null>(null);

  const load = useCallback(
    () => unwrap(api.GET("/api/v1/public/orders/{order_id}", { params: { path: { order_id: orderId } } })),
    [api, orderId],
  );
  const { state } = useResource(load);
  const order = latest ?? (state.status === "ready" ? state.data : null);

  const poll = useCallback(async () => setLatest(await load()), [load]);
  const certificateWaiting = order?.lines.some(({ certificate }) => certificate?.status === "pending") ?? false;
  usePolling(poll, POLL_MS, order?.status === "pending" || certificateWaiting);

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

      {order.status === "pending" ? <PendingPayment order={order} /> : null}
      {order.status === "confirmed" ? <Receipt order={order} /> : null}
      {order.status === "expired" ? <Notice tone="danger">QR sudah kedaluwarsa. Minta kasir membuat QR baru.</Notice> : null}
      {order.status === "failed" ? (
        <Notice tone="danger">Pembayaran gagal, dan saldo tidak berubah. Minta kasir membuat QR baru untuk mencoba lagi.</Notice>
      ) : null}
    </>
  );
}

function PendingPayment({ order }: { order: PublicOrder }) {
  const [sent, setSent] = useState(false);
  const now = useNow(true);

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
      <WalletPayment orderId={order.id} onSent={() => setSent(true)} />

      <div className="border-t border-border pt-4 text-center">
        <p className="text-sm text-muted-foreground">Atau pindai dengan Phantom di HP</p>
        <div className="mt-3">
          <PaymentQr value={order.solana_pay_url} />
        </div>
      </div>
    </Card>
  );
}

type LineCertificate = PublicOrder["lines"][number]["certificate"];

function CertificateRow({ name, certificate }: { name: string; certificate: LineCertificate }) {
  if (certificate?.explorer_url) {
    return <ExplorerLink href={certificate.explorer_url}>Sertifikat {name}</ExplorerLink>;
  }
  if (certificate?.status === "failed") {
    return <p className="text-sm text-muted-foreground">Sertifikat {name} gagal dikirim. Hubungi toko.</p>;
  }
  return <p className="text-sm text-muted-foreground">Sertifikat {name} sedang dikirim ke wallet kamu.</p>;
}

function Receipt({ order }: { order: PublicOrder }) {
  return (
    <Card className="grid gap-3">
      <Notice tone="success">Pembayaran terkonfirmasi. Terima kasih.</Notice>
      {order.confirmed_at ? <p className="text-sm text-muted-foreground">{formatDateTime(order.confirmed_at)}</p> : null}
      {order.explorer_url ? <ExplorerLink href={order.explorer_url} /> : null}
      {order.lines
        .filter(({ certificate }) => certificate)
        .map(({ item_name: name, certificate }) => (
          <CertificateRow key={name} name={name} certificate={certificate} />
        ))}
    </Card>
  );
}
