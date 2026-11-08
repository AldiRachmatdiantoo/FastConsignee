"use client";

import { useCallback, useEffect, useState } from "react";
import { readBalances } from "@/features/wallet/rpc";
import { formatSol } from "@/lib/format";
import { usePolling } from "@/lib/hooks";
import type { Recipient } from "./split-table";

type Balances = Record<string, bigint>;

const REFRESH_MS = 2000;

// BalancePanel membaca saldo penerima langsung dari Devnet, jadi kenaikannya bukan angka dari server.
export function BalancePanel({ recipients, confirmed }: { recipients: Recipient[]; confirmed: boolean }) {
  const [before, setBefore] = useState<Balances | null>(null);
  const [after, setAfter] = useState<Balances | null>(null);
  const [failed, setFailed] = useState(false);

  const addresses = recipients.map(({ wallet_address }) => wallet_address).join(",");

  useEffect(() => {
    let active = true;
    readBalances(addresses.split(",")).then(
      (balances) => {
        if (active) {
          setBefore(balances);
        }
      },
      () => {
        if (active) {
          setFailed(true);
        }
      },
    );
    return () => {
      active = false;
    };
  }, [addresses]);

  const refresh = useCallback(async () => {
    setAfter(await readBalances(addresses.split(",")));
  }, [addresses]);

  // Penerima bernilai 0 tidak ikut di transaksi, jadi saldonya memang tidak berubah.
  const paid = recipients.filter(({ amount_lamports }) => amount_lamports !== "0");
  const settled =
    before !== null && after !== null && paid.every((r) => after[r.wallet_address] !== before[r.wallet_address]);
  usePolling(refresh, REFRESH_MS, confirmed && !settled);

  if (failed) {
    return <p className="text-sm text-muted-foreground">Devnet balances cannot be read right now.</p>;
  }
  if (!before) {
    return null;
  }

  return (
    <div>
      <h3 className="font-semibold">Wallet balances on Devnet</h3>
      <ul className="mt-2 divide-y divide-border text-sm">
        {recipients.map(({ wallet_address: address, name }) => {
          const start = before[address];
          const latest = after?.[address] ?? start;
          const gain = latest - start;
          return (
            <li key={address} className="flex flex-wrap items-baseline justify-between gap-x-3 py-2">
              <span className="font-medium">{name}</span>
              <span className="font-mono text-xs">
                {formatSol(latest)}
                {latest !== start ? <span className="ml-2 font-semibold text-success">+{formatSol(gain)}</span> : null}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
