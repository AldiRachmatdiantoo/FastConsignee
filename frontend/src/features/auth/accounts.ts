export type LocalAccount = {
  email: string;
  name: string;
  role: string;
};

// Akun ini sama dengan hasil seed backend, dengan SEED_OWNER_EMAIL=pemilik@fastconsignee.test.
export const LOCAL_ACCOUNTS: LocalAccount[] = [
  { email: "pemilik@fastconsignee.test", name: "Pemilik Demo", role: "Pemilik toko" },
  { email: "kasir@fastconsignee.test", name: "Kasir Demo", role: "Kasir" },
  { email: "sari@fastconsignee.test", name: "Sari Anyaman", role: "Penitip" },
  { email: "budi@fastconsignee.test", name: "Budi Kopi", role: "Penitip" },
  { email: "rina@fastconsignee.test", name: "Rina Batik", role: "Penitip" },
];

export function isLocalAccount(email: string): boolean {
  return LOCAL_ACCOUNTS.some((account) => account.email === email);
}
