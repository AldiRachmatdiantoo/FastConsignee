import type { Me } from "@/lib/api/client";

const ROLE_LABELS: Record<Me["role"], string> = {
  cashier: "Kasir",
  store_owner: "Pemilik toko",
  consignor: "Penitip",
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

// guardTarget mengembalikan tujuan pengalihan, atau null kalau pengguna boleh di layar ini.
export function guardTarget(me: Me, pathname: string): string | null {
  if (me.role === "cashier") {
    return pathname === "/kasir" ? null : "/kasir";
  }
  if (needsWallet(me)) {
    return pathname === "/wallet" ? null : "/wallet";
  }
  if (me.role === "consignor" && pathname === "/kasir") {
    return "/dasbor";
  }
  return null;
}
