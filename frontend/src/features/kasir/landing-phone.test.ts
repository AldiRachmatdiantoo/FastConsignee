import { readFileSync } from "node:fs";
import { join } from "node:path";
import { expect, it } from "vitest";

const read = (file: string) => readFileSync(join(process.cwd(), file), "utf8");

// Mockup HP di landing menyalin layar asli, jadi teksnya harus ikut berubah kalau teks layar berubah.
const SCREENS: Record<string, string[]> = {
  "src/features/kasir/catalog.tsx": ["Katalog", "Cari barang", "Nama barang"],
  "src/features/kasir/cart-panel.tsx": ["Keranjang", "Beri diskon", "Rincian pembagian", "Buat QR pembayaran"],
  "src/features/kasir/split-table.tsx": ["Bagian penitip", "Komisi toko"],
  "src/features/kasir/payment-panel.tsx": [
    "Pembayaran",
    "Pindai dengan Phantom yang diatur ke Devnet.",
    "Menunggu pembayaran",
    "Buka layar bayar di browser",
    "Pembayaran terkonfirmasi. Barang boleh diserahkan.",
    "Transaksi baru",
  ],
  "src/features/kasir/balance-panel.tsx": ["Saldo wallet di Devnet"],
  "src/features/bayar/bayar-screen.tsx": [
    "Kurs terkunci",
    "Ini jaringan Devnet, jadi tidak memakai uang sungguhan.",
    "Bayar dalam",
    "Atau pindai dengan Phantom di HP",
  ],
  "src/features/bayar/wallet-payment.tsx": ["Wallet terhubung", "Bayar sekali", "Ganti wallet"],
  "src/components/ui.tsx": ["Lihat di Solana Explorer"],
};

it.each(Object.entries(SCREENS))("teks %s sama di layar asli dan mockup landing", (file, labels) => {
  const screen = read(file);
  const landing = read("public/landing.html");
  for (const label of labels) {
    expect(screen).toContain(label);
    expect(landing).toContain(label);
  }
});
