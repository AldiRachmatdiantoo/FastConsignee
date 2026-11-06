"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { roleLabel } from "@/features/auth/routes";
import { useSession } from "@/features/auth/session";
import type { Me } from "@/lib/api/client";
import { Button } from "./ui";

export function AppShell({ me, children }: { me: Me; children: ReactNode }) {
  const { signOut } = useSession();

  return (
    <div className="min-h-dvh">
      <header className="border-b border-border bg-card">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-3 md:px-8">
          <div className="min-w-0">
            <Link href="/" className="font-semibold text-accent-foreground">
              FastConsignee
            </Link>
            <p className="truncate text-sm text-muted-foreground">{me.store.name}</p>
          </div>
          <div className="flex items-center gap-3">
            <span className="rounded-full bg-accent px-2.5 py-1 text-xs font-semibold text-accent-foreground">Devnet</span>
            <div className="text-right text-sm">
              <p className="font-medium">{me.name}</p>
              <p className="text-muted-foreground">{roleLabel(me.role)}</p>
            </div>
            <Button variant="secondary" onClick={signOut}>
              Keluar
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-8 md:px-8">{children}</main>
    </div>
  );
}
