"use client";

import { WalletReadyState } from "@solana/wallet-adapter-base";
import { ConnectionProvider, useWallet, WalletProvider } from "@solana/wallet-adapter-react";
import { clusterApiUrl } from "@solana/web3.js";
import { useRouter } from "next/navigation";
import { useCallback, useState } from "react";
import { Button, Notice } from "@/components/ui";
import { homePath } from "@/features/auth/routes";
import { useSession } from "@/features/auth/session";
import { linkErrorMessage, linkWallet, shortAddress } from "./link-wallet";

const ENDPOINT = clusterApiUrl("devnet");
const NO_ADAPTERS: never[] = [];

export function WalletScreen() {
  const [connectFailed, setConnectFailed] = useState(false);
  const onError = useCallback(() => setConnectFailed(true), []);

  return (
    <ConnectionProvider endpoint={ENDPOINT}>
      <WalletProvider wallets={NO_ADAPTERS} autoConnect onError={onError}>
        <LinkWallet connectFailed={connectFailed} resetConnect={() => setConnectFailed(false)} />
      </WalletProvider>
    </ConnectionProvider>
  );
}

type LinkWalletProps = {
  connectFailed: boolean;
  resetConnect: () => void;
};

function LinkWallet({ connectFailed, resetConnect }: LinkWalletProps) {
  const { session, api, refresh } = useSession();
  const { wallets, select, publicKey, signMessage, disconnect, connecting } = useWallet();
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (session.status !== "authenticated") {
    return null;
  }
  const { me } = session;
  const address = publicKey?.toBase58() ?? null;
  const linked = me.wallet?.address ?? null;
  const available = wallets.filter(
    ({ readyState }) => readyState === WalletReadyState.Installed || readyState === WalletReadyState.Loadable,
  );
  const message = error ?? (connectFailed ? "Wallet tidak bisa dihubungkan. Buka wallet, lalu coba lagi." : null);

  async function submit() {
    if (!address) {
      return;
    }
    if (!signMessage) {
      setError("Wallet ini belum bisa menandatangani pesan. Pakai Phantom.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await linkWallet({ api, address, signMessage });
      await refresh();
    } catch (failure) {
      setError(linkErrorMessage(failure));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="mx-auto max-w-lg">
      <h1 className="text-2xl font-bold tracking-tight">Tautkan wallet</h1>
      <p className="mt-2 text-muted-foreground">
        {me.role === "store_owner"
          ? "Wallet ini menerima komisi toko dan membayar refund."
          : "Wallet ini menerima bagian penjualanmu."}{" "}
        Menandatangani pesan tidak memakai saldo.
      </p>

      <div className="mt-6 grid gap-4 rounded-xl border border-border bg-card p-5">
        {linked ? (
          <Notice tone="success">
            Wallet tertaut: <span className="font-mono">{shortAddress(linked)}</span>
          </Notice>
        ) : null}
        {message ? <Notice tone="danger">{message}</Notice> : null}

        {address ? (
          <div>
            <p className="text-sm text-muted-foreground">Wallet terhubung</p>
            <p className="mt-1 font-mono text-sm break-all">{address}</p>
            <div className="mt-4 flex flex-wrap gap-2">
              {linked === address ? (
                <Button onClick={() => router.replace(homePath(me))}>Lanjut ke dasbor</Button>
              ) : (
                <Button onClick={() => void submit()} disabled={busy}>
                  {busy ? "Menunggu wallet..." : "Tandatangani dan tautkan"}
                </Button>
              )}
              <Button variant="secondary" onClick={() => void disconnect()} disabled={busy}>
                Ganti wallet
              </Button>
            </div>
          </div>
        ) : available.length > 0 ? (
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
                      setError(null);
                      resetConnect();
                      select(adapter.name);
                    }}
                  >
                    {connecting ? "Menghubungkan..." : adapter.name}
                  </Button>
                </li>
              ))}
            </ul>
          </div>
        ) : (
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
        )}
      </div>
    </section>
  );
}
