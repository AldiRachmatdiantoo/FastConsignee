"use client";

import { useCallback, useState } from "react";
import { PaymentQr } from "@/components/payment-qr";
import { Button, Card, ExplorerLink, Notice } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { unwrap } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatCountdown, formatRupiah, formatSol, secondsUntil } from "@/lib/format";
import { useNow, usePolling } from "@/lib/hooks";
import { BalancePanel } from "./balance-panel";
import { SplitTable } from "./split-table";

type Order = components["schemas"]["Order"];
type OrderStatusView = components["schemas"]["OrderStatusView"];
type FailureCode = components["schemas"]["FailureCode"];

const POLL_MS = 2000;

const FAILURE_MESSAGES: Record<FailureCode, string> = {
  TRANSACTION_FAILED: "Transaksi gagal di jaringan, jadi tidak ada saldo yang berubah.",
  VALIDATION_MISMATCH: "Isi transaksi tidak cocok dengan order. Hubungi pemilik toko sebelum menyerahkan barang.",
};

type PaymentPanelProps = {
  order: Order;
  onRetry: () => void;
  onDone: () => void;
};

export function PaymentPanel({ order, onRetry, onDone }: PaymentPanelProps) {
  const { api } = useSession();
  const [status, setStatus] = useState<OrderStatusView>({
    id: order.id,
    status: order.status,
    expires_at: order.expires_at,
    signature: order.payment.signature,
    explorer_url: order.payment.explorer_url,
    failure_code: order.payment.failure_code,
    updated_at: order.created_at,
  });
  const pending = status.status === "pending";
  const now = useNow(pending);

  const poll = useCallback(async () => {
    setStatus(await unwrap(api.GET("/api/v1/orders/{order_id}/status", { params: { path: { order_id: order.id } } })));
  }, [api, order.id]);
  usePolling(poll, POLL_MS, pending);

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      <Card className="text-center">
        <h2 className="text-lg font-semibold">Pembayaran</h2>
        <p className="mt-2 text-3xl font-bold">{formatRupiah(order.total_idr)}</p>
        <p className="font-mono text-sm text-muted-foreground">{formatSol(order.total_lamports)}</p>

        {pending ? (
          <>
            <div className="mt-4">
              <PaymentQr value={order.payment.solana_pay_url} />
            </div>
            <p className="mt-3 text-sm text-muted-foreground">Pindai dengan Phantom yang diatur ke Devnet.</p>
            <p role="status" className="mt-2 font-semibold">
              Menunggu pembayaran · {formatCountdown(secondsUntil(status.expires_at, now))}
            </p>
            <a
              href={`/bayar/${order.id}`}
              target="_blank"
              rel="noreferrer"
              className="mt-2 inline-block text-sm font-semibold text-accent-foreground underline underline-offset-4"
            >
              Buka layar bayar di browser
            </a>
          </>
        ) : null}

        {status.status === "confirmed" ? (
          <div className="mt-4 grid gap-3">
            <Notice tone="success">Pembayaran terkonfirmasi. Barang boleh diserahkan.</Notice>
            {status.explorer_url ? <ExplorerLink href={status.explorer_url} /> : null}
            <Button onClick={onDone}>Transaksi baru</Button>
          </div>
        ) : null}

        {status.status === "expired" ? (
          <div className="mt-4 grid gap-3">
            <Notice tone="danger">QR kedaluwarsa, dan stok sudah dilepas lagi.</Notice>
            <Button onClick={onRetry}>Buat QR baru</Button>
          </div>
        ) : null}

        {status.status === "failed" ? (
          <div className="mt-4 grid gap-3">
            <Notice tone="danger">
              {status.failure_code ? FAILURE_MESSAGES[status.failure_code] : "Pembayaran gagal."}
            </Notice>
            <Button onClick={onRetry}>Coba lagi</Button>
          </div>
        ) : null}
      </Card>

      <Card className="grid content-start gap-5">
        <div>
          <h2 className="text-lg font-semibold">Rincian pembagian</h2>
          <SplitTable recipients={order.recipients} />
        </div>
        <BalancePanel recipients={order.recipients} confirmed={status.status === "confirmed"} />
      </Card>
    </div>
  );
}
