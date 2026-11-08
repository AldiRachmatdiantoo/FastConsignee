export type LocalAccount = {
  email: string;
  name: string;
  role: string;
};

// Akun ini sama dengan hasil seed backend, dengan SEED_OWNER_EMAIL=pemilik@fastconsignee.test.
export const LOCAL_ACCOUNTS: LocalAccount[] = [
  { email: "pemilik@fastconsignee.test", name: "Demo Owner", role: "Store owner" },
  { email: "kasir@fastconsignee.test", name: "Demo Cashier", role: "Cashier" },
  { email: "sari@fastconsignee.test", name: "Sari Anyaman", role: "Consignor" },
  { email: "budi@fastconsignee.test", name: "Budi Kopi", role: "Consignor" },
  { email: "rina@fastconsignee.test", name: "Rina Batik", role: "Consignor" },
];

export function isLocalAccount(email: string): boolean {
  return LOCAL_ACCOUNTS.some((account) => account.email === email);
}
