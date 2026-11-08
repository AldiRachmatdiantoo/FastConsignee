"use client";

import { useConnection, useWallet } from "@solana/wallet-adapter-react";
import { useCallback, useState } from "react";
import { Button, Notice } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { shortAddress } from "@/features/wallet/link-wallet";
import { SolanaProvider } from "@/features/wallet/solana-provider";
import { WalletPicker } from "@/features/wallet/wallet-picker";
import { payErrorMessage, requestTransaction } from "./pay";

type WalletPaymentProps = {
  orderId: string;
  onSent: () => void;
};

// Berkas ini membawa web3.js, jadi dimuat terpisah saat pembeli memilih bayar dengan wallet.
export function WalletPayment({ orderId, onSent }: WalletPaymentProps) {
  const [connectFailed, setConnectFailed] = useState(false);
  const onError = useCallback(() => setConnectFailed(true), []);

  return (
    <SolanaProvider onError={onError}>
      <WalletActions
        orderId={orderId}
        onSent={onSent}
        connectFailed={connectFailed}
        resetConnect={() => setConnectFailed(false)}
      />
    </SolanaProvider>
  );
}

type WalletActionsProps = WalletPaymentProps & {
  connectFailed: boolean;
  resetConnect: () => void;
};

function WalletActions({ orderId, onSent, connectFailed, resetConnect }: WalletActionsProps) {
  const { api } = useSession();
  const { connection } = useConnection();
  const { publicKey, sendTransaction, disconnect } = useWallet();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const message = error ?? (connectFailed ? "Wallet tidak bisa dihubungkan. Buka wallet, lalu coba lagi." : null);

  async function pay() {
    if (!publicKey) {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const transaction = await requestTransaction(api, orderId, publicKey.toBase58());
      await sendTransaction(transaction, connection);
      onSent();
    } catch (failure) {
      setError(payErrorMessage(failure));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
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
          openInPhantom
          onPick={() => {
            setError(null);
            resetConnect();
          }}
        />
      )}
    </>
  );
}
