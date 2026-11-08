"use client";

import { WalletReadyState } from "@solana/wallet-adapter-base";
import { useWallet } from "@solana/wallet-adapter-react";
import { useSyncExternalStore } from "react";
import { Button, buttonClassName } from "@/components/ui";

const PHANTOM_DOWNLOAD = "https://phantom.com/download";

// phantomBrowseLink membuka halaman di browser dalam aplikasi Phantom, sesuai deeplink browse di dokumentasi Phantom.
export function phantomBrowseLink(href: string, origin: string) {
  return `https://phantom.com/ul/browse/${encodeURIComponent(href)}?ref=${encodeURIComponent(origin)}`;
}

// Phantom di HP tidak tersambung ke Safari atau Chrome, hanya ke browser di dalam aplikasinya. iPad terbaru melapor sebagai Mac,
// jadi layar sentuh dengan platform Mac juga dihitung HP.
export function isMobileDevice(userAgent: string, platform: string, maxTouchPoints: number) {
  return /Android|iPhone|iPad|iPod/i.test(userAgent) || (platform === "MacIntel" && maxTouchPoints > 1);
}

const subscribe = () => () => {};

function useMobileDevice() {
  return useSyncExternalStore(
    subscribe,
    () => isMobileDevice(navigator.userAgent, navigator.platform, navigator.maxTouchPoints),
    () => false,
  );
}

type WalletPickerProps = {
  onPick?: () => void;
  // Hanya layar yang bisa dibuka tanpa login, seperti layar bayar, yang menawarkan pindah ke browser Phantom,
  // karena login Google tidak berjalan di browser dalam aplikasi.
  openInPhantom?: boolean;
};

// WalletPicker menampilkan wallet yang terpasang, atau cara memasangnya kalau belum ada.
export function WalletPicker({ onPick, openInPhantom = false }: WalletPickerProps) {
  const { wallets, select, connecting } = useWallet();
  const mobile = useMobileDevice();
  const available = wallets.filter(
    ({ readyState }) => readyState === WalletReadyState.Installed || readyState === WalletReadyState.Loadable,
  );

  if (available.length === 0 && mobile) {
    return (
      <div>
        <p className="font-medium">On a phone, wallets connect through the Phantom app</p>
        {openInPhantom ? (
          <>
            <p className="mt-1 text-sm text-muted-foreground">
              Turn on Testnet Mode and choose Solana Devnet in Phantom, then open this page there. You can also scan the payment QR with Phantom.
            </p>
            <a className={buttonClassName("primary", "mt-3 w-full")} href={phantomBrowseLink(location.href, location.origin)}>
              Open in the Phantom app
            </a>
          </>
        ) : (
          <p className="mt-1 text-sm text-muted-foreground">
            Phantom cannot connect to a regular phone browser. Open this page on a laptop with Phantom installed.
          </p>
        )}
        <a
          href={PHANTOM_DOWNLOAD}
          target="_blank"
          rel="noreferrer"
          className="mt-3 inline-block text-sm font-semibold text-accent-foreground underline underline-offset-4"
        >
          No Phantom yet? Install it first
        </a>
      </div>
    );
  }

  if (available.length === 0) {
    return (
      <div>
        <p className="font-medium">No wallet in this browser yet</p>
        <p className="mt-1 text-sm text-muted-foreground">
          Install Phantom, turn on Testnet Mode, choose Solana Devnet, then reload this page.
        </p>
        <a
          href={PHANTOM_DOWNLOAD}
          target="_blank"
          rel="noreferrer"
          className="mt-3 inline-block font-semibold text-accent-foreground underline underline-offset-4"
        >
          Install Phantom
        </a>
      </div>
    );
  }

  return (
    <div>
      <p className="font-medium">Choose a wallet</p>
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
              {connecting ? "Connecting..." : adapter.name}
            </Button>
          </li>
        ))}
      </ul>
    </div>
  );
}
