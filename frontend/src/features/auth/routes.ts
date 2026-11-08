import type { Me } from "@/lib/api/client";

const ROLE_LABELS: Record<Me["role"], string> = {
  cashier: "Cashier",
  store_owner: "Store owner",
  consignor: "Consignors",
};

export function roleLabel(role: Me["role"]): string {
  return ROLE_LABELS[role];
}

function needsWallet(me: Me): boolean {
  return me.wallet_required && me.wallet === null;
}

// homePath adalah layar pertama setelah login.
export function homePath(me: Me): string {
  if (me.role === "cashier") {
    return "/kasir";
  }
  return needsWallet(me) ? "/wallet" : "/dasbor";
}

const OWNER_ONLY = ["/refund", "/barang", "/penitip"];

// guardTarget mengembalikan tujuan pengalihan, atau null kalau pengguna boleh di layar ini.
export function guardTarget(me: Me, pathname: string): string | null {
  if (me.role === "cashier") {
    return pathname === "/kasir" ? null : "/kasir";
  }
  if (needsWallet(me)) {
    return pathname === "/wallet" ? null : "/wallet";
  }
  if (me.role === "consignor" && (pathname === "/kasir" || OWNER_ONLY.includes(pathname))) {
    return "/dasbor";
  }
  return null;
}

type NavLink = { href: string; label: string };

// navLinks hanya memuat layar yang boleh dibuka peran itu.
export function navLinks(me: Me): NavLink[] {
  if (me.role === "cashier") {
    return [{ href: "/kasir", label: "Cashier" }];
  }
  if (me.role === "consignor") {
    return [
      { href: "/dasbor", label: "Dashboard" },
      { href: "/wallet", label: "Wallet" },
    ];
  }
  return [
    { href: "/dasbor", label: "Dashboard" },
    { href: "/kasir", label: "Cashier" },
    { href: "/refund", label: "Refund" },
    { href: "/barang", label: "Items" },
    { href: "/penitip", label: "Consignors" },
    { href: "/wallet", label: "Wallet" },
  ];
}
