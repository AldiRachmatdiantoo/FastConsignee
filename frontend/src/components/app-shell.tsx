"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState, type ComponentType, type ReactNode } from "react";
import { homePath, navLinks, roleLabel } from "@/features/auth/routes";
import { useSession } from "@/features/auth/session";
import type { Me } from "@/lib/api/client";
import { BoxIcon, CartIcon, CloseIcon, DashboardIcon, MenuIcon, RefundIcon, UsersIcon, WalletIcon } from "./icons";
import { Button } from "./ui";

type NavLink = ReturnType<typeof navLinks>[number];

const NAV_META: Record<string, { group: string; icon: ComponentType<{ className?: string }> }> = {
  "/dasbor": { group: "Ringkasan", icon: DashboardIcon },
  "/kasir": { group: "Penjualan", icon: CartIcon },
  "/refund": { group: "Penjualan", icon: RefundIcon },
  "/barang": { group: "Kelola", icon: BoxIcon },
  "/penitip": { group: "Kelola", icon: UsersIcon },
  "/wallet": { group: "Akun", icon: WalletIcon },
};

function groupLinks(links: NavLink[]) {
  const groups = new Map<string, NavLink[]>();
  for (const link of links) {
    const group = NAV_META[link.href].group;
    groups.set(group, [...(groups.get(group) ?? []), link]);
  }
  return [...groups];
}

function Brand({ href }: { href: string }) {
  return (
    <Link
      href={href}
      className="inline-flex items-center gap-2.5 text-lg font-semibold tracking-tight focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
    >
      <svg viewBox="0 0 40 28" aria-hidden="true" className="h-5 w-8 flex-none" fill="none" strokeWidth={3} strokeLinecap="round">
        <path className="stroke-primary" d="M3 19C10 5 17 5 21 14s11 9 16-6" />
        <path stroke="currentColor" d="M3 10c7 13 14 13 18 4s11-10 16 5" />
      </svg>
      FastConsignee
    </Link>
  );
}

function initials(name: string): string {
  const [first = "", second = ""] = name.trim().split(/\s+/);
  return `${first.charAt(0)}${second.charAt(0)}`.toUpperCase();
}

export function AppShell({ me, children }: { me: Me; children: ReactNode }) {
  const { signOut } = useSession();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);
  const links = navLinks(me);
  const hasMenu = links.length > 1;
  const home = homePath(me);
  const current = hasMenu ? links.find(({ href }) => href === pathname) : undefined;

  useEffect(() => {
    if (!open) {
      return;
    }
    closeButton.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
        menuButton.current?.focus();
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open]);

  return (
    <div className="min-h-dvh bg-secondary">
      {hasMenu ? (
        <>
          {open ? (
            <button
              type="button"
              aria-label="Tutup menu"
              tabIndex={-1}
              onClick={() => setOpen(false)}
              className="fixed inset-0 z-30 bg-black/50 xl:hidden"
            />
          ) : null}
          <aside
            id="app-menu"
            className={`dark fixed inset-y-0 left-0 z-40 flex w-66 flex-col overflow-y-auto bg-background text-foreground shadow-md transition-[transform,visibility] duration-200 motion-reduce:transition-none xl:visible xl:translate-x-0 ${
              open ? "translate-x-0" : "invisible -translate-x-full"
            }`}
          >
            <div className="flex min-h-18 items-center justify-between gap-2 px-6">
              <Brand href={home} />
              <button
                ref={closeButton}
                type="button"
                aria-label="Tutup menu"
                onClick={() => {
                  setOpen(false);
                  menuButton.current?.focus();
                }}
                className="grid size-11 place-items-center rounded-lg text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring xl:hidden"
              >
                <CloseIcon />
              </button>
            </div>
            <nav aria-label="Menu" className="grid gap-5 py-3">
              {groupLinks(links).map(([group, items]) => (
                <div key={group}>
                  <p className="px-6 pb-2 text-xs font-semibold tracking-wide text-muted-foreground uppercase">{group}</p>
                  <ul>
                    {items.map(({ href, label }) => {
                      const active = pathname === href;
                      const ItemIcon = NAV_META[href].icon;
                      return (
                        <li key={href}>
                          <Link
                            href={href}
                            aria-current={active ? "page" : undefined}
                            onClick={() => setOpen(false)}
                            className={`flex min-h-11 items-center gap-3 border-l-3 px-6 text-sm font-medium focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring ${
                              active
                                ? "border-primary bg-card text-foreground"
                                : "border-transparent text-muted-foreground hover:bg-card hover:text-foreground"
                            }`}
                          >
                            <ItemIcon />
                            {label}
                          </Link>
                        </li>
                      );
                    })}
                  </ul>
                </div>
              ))}
            </nav>
            <p className="mt-auto truncate px-6 py-5 text-sm text-muted-foreground">{me.store.name}</p>
          </aside>
        </>
      ) : null}

      <div inert={open} className={`flex min-h-dvh flex-col ${hasMenu ? "xl:pl-66" : ""}`}>
        <header className="sticky top-0 z-20 bg-secondary/80 backdrop-blur">
          <div className="mx-auto flex min-h-18 max-w-7xl items-center gap-3 px-4 md:px-8">
            {hasMenu ? (
              <button
                ref={menuButton}
                type="button"
                aria-label="Buka menu"
                aria-expanded={open}
                aria-controls="app-menu"
                onClick={() => setOpen(true)}
                className="-ml-2 grid size-11 place-items-center rounded-lg hover:bg-card focus-visible:outline-2 focus-visible:outline-ring xl:hidden"
              >
                <MenuIcon />
              </button>
            ) : (
              <div className="min-w-0">
                <Brand href={home} />
                <p className="truncate text-sm text-muted-foreground">{me.store.name}</p>
              </div>
            )}
            {current ? (
              <p className="hidden truncate text-sm text-muted-foreground sm:block">
                {NAV_META[current.href].group} <span aria-hidden="true">›</span>{" "}
                <span className="font-medium text-foreground">{current.label}</span>
              </p>
            ) : null}
            <div className="ml-auto flex items-center gap-3">
              <span className="rounded-full bg-accent px-2.5 py-1 text-xs font-semibold text-accent-foreground">Devnet</span>
              <span
                aria-hidden="true"
                className="hidden size-9 flex-none place-items-center rounded-full bg-primary text-sm font-semibold text-primary-foreground sm:grid"
              >
                {initials(me.name)}
              </span>
              <div className="hidden text-sm sm:block">
                <p className="font-medium">{me.name}</p>
                <p className="text-muted-foreground">{roleLabel(me.role)}</p>
              </div>
              <Button variant="secondary" onClick={signOut}>
                Keluar
              </Button>
            </div>
          </div>
        </header>

        <main className="mx-auto w-full max-w-7xl flex-1 px-4 pt-3 pb-8 md:px-8">{children}</main>

        <footer className="mx-auto w-full max-w-7xl px-4 pb-6 text-sm text-muted-foreground md:px-8">
          FastConsignee berjalan di Solana Devnet, jadi tidak ada uang sungguhan.
        </footer>
      </div>
    </div>
  );
}
