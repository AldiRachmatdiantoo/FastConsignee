"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Notice } from "@/components/ui";
import { LOCAL_ACCOUNTS } from "./accounts";
import { homePath } from "./routes";
import { useSession } from "./session";

export function LoginScreen() {
  const { session, signInLocal } = useSession();
  const router = useRouter();
  const [pending, setPending] = useState<string | null>(null);
  const localLogin = process.env.NEXT_PUBLIC_LOCAL_LOGIN === "true";

  useEffect(() => {
    if (session.status === "authenticated") {
      router.replace(homePath(session.me));
    }
  }, [session, router]);

  async function choose(email: string) {
    setPending(email);
    await signInLocal(email);
    setPending(null);
  }

  return (
    <main className="mx-auto grid min-h-dvh max-w-5xl content-center gap-10 px-4 py-10 md:grid-cols-2 md:items-center md:gap-16 md:px-8">
      <section>
        <p className="font-semibold text-accent-foreground">FastConsignee</p>
        <h1 className="mt-3 text-3xl leading-tight font-bold tracking-tight text-balance md:text-5xl">
          Sekali bayar, semua penitip langsung terima bagiannya.
        </h1>
        <p className="mt-4 max-w-md text-muted-foreground">
          Kasir untuk toko konsinyasi. Bagian penitip dan komisi toko keluar dalam satu transaksi Solana.
        </p>
      </section>

      <section className="rounded-xl border border-border bg-card p-5 md:p-6">
        <h2 className="text-lg font-semibold">Masuk</h2>
        {session.status === "anonymous" && session.error ? (
          <div className="mt-3">
            <Notice tone="danger">{session.error}</Notice>
          </div>
        ) : null}
        {localLogin ? (
          <>
            <p className="mt-1 text-sm text-muted-foreground">Mode lokal. Pilih akun seed untuk masuk.</p>
            <ul className="mt-4 divide-y divide-border">
              {LOCAL_ACCOUNTS.map((account) => (
                <li key={account.email}>
                  <button
                    type="button"
                    disabled={pending !== null}
                    onClick={() => void choose(account.email)}
                    className="flex min-h-14 w-full items-center justify-between gap-3 rounded-lg px-2 py-2 text-left hover:bg-muted focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-60"
                  >
                    <span className="font-medium">{account.name}</span>
                    <span className="text-sm text-muted-foreground">{pending === account.email ? "Masuk..." : account.role}</span>
                  </button>
                </li>
              ))}
            </ul>
          </>
        ) : (
          <p className="mt-2 text-sm text-muted-foreground">Login Google belum tersambung.</p>
        )}
      </section>
    </main>
  );
}
