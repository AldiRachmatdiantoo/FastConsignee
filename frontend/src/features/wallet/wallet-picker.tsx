"use client";

import { WalletReadyState } from "@solana/wallet-adapter-base";
import { useWallet } from "@solana/wallet-adapter-react";
import { Button } from "@/components/ui";

// WalletPicker menampilkan wallet yang terpasang, atau cara memasangnya kalau belum ada.
export function WalletPicker({ onPick }: { onPick?: () => void }) {
  const { wallets, select, connecting } = useWallet();
  const available = wallets.filter(
    ({ readyState }) => readyState === WalletReadyState.Installed || readyState === WalletReadyState.Loadable,
  );

  if (available.length === 0) {
    return (
      <div>
        <p className="font-medium">Belum ada wallet di browser ini</p>
        <p className="mt-1 text-sm text-muted-foreground">
          Pasang Phantom, aktifkan Testnet Mode, pilih Solana Devnet, lalu muat ulang halaman ini.
        </p>
        <a
          href="https://phantom.com/download"
          target="_blank"
          rel="noreferrer"
          className="mt-3 inline-block font-semibold text-accent-foreground underline underline-offset-4"
        >
          Pasang Phantom
        </a>
      </div>
    );
  }

  return (
    <div>
      <p className="font-medium">Pilih wallet</p>
      <ul className="mt-3 grid gap-2">
        {available.map(({ adapter }) => (
          <li key={adapter.name}>
            <Button
              variant="secondary"
              className="w-full"
              disabled={connecting}
              onClick={() => {
                onPick?.();
                select(adapter.name);
              }}
            >
              {connecting ? "Menghubungkan..." : adapter.name}
            </Button>
          </li>
        ))}
      </ul>
    </div>
  );
}
