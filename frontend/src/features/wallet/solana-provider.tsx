"use client";

import { ConnectionProvider, WalletProvider } from "@solana/wallet-adapter-react";
import type { ReactNode } from "react";
import { RPC_URL } from "./rpc";

const NO_ADAPTERS: never[] = [];

// Wallet dideteksi lewat Wallet Standard, jadi tidak ada adapter yang didaftarkan manual.
export function SolanaProvider({ onError, children }: { onError?: () => void; children: ReactNode }) {
  return (
    <ConnectionProvider endpoint={RPC_URL}>
      <WalletProvider wallets={NO_ADAPTERS} autoConnect onError={onError}>
        {children}
      </WalletProvider>
    </ConnectionProvider>
  );
}
