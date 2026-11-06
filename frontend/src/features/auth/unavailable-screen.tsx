"use client";

import { useState } from "react";
import { Button, Notice } from "@/components/ui";

type Props = {
  message: string;
  onRetry: () => Promise<void>;
};

// Token masih ada saat API tidak terjangkau, jadi pengguna tidak perlu dilempar ke halaman login.
export function UnavailableScreen({ message, onRetry }: Props) {
  const [busy, setBusy] = useState(false);

  async function retry() {
    setBusy(true);
    await onRetry();
    setBusy(false);
  }

  return (
    <main className="mx-auto grid min-h-dvh max-w-lg content-center gap-3 px-4 py-8">
      <h1 className="text-2xl font-bold tracking-tight">Belum tersambung</h1>
      <Notice tone="danger">{message}</Notice>
      <Button className="justify-self-start" disabled={busy} onClick={() => void retry()}>
        {busy ? "Mencoba..." : "Coba lagi"}
      </Button>
    </main>
  );
}
