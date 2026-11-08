import { readFileSync } from "node:fs";
import { join } from "node:path";
import { expect, it } from "vitest";

const read = (file: string) => readFileSync(join(process.cwd(), file), "utf8");

// Mockup HP di landing menyalin layar asli, jadi teksnya harus ikut berubah kalau teks layar berubah.
const SCREENS: Record<string, string[]> = {
  "src/features/kasir/catalog.tsx": ["Catalog", "Search items", "Item name"],
  "src/features/kasir/cart-panel.tsx": ["Cart", "Add discount", "Split breakdown", "Create payment QR"],
  "src/features/kasir/split-table.tsx": ["Consignor share", "Shop commission"],
  "src/features/kasir/payment-panel.tsx": [
    "Payment",
    "Scan with Phantom set to Devnet.",
    "Waiting for payment",
    "Open the pay screen in a browser",
    "Payment confirmed. The goods can be handed over.",
    "New transaction",
  ],
  "src/features/kasir/balance-panel.tsx": ["Wallet balances on Devnet"],
  "src/features/bayar/bayar-screen.tsx": [
    "Locked rate",
    "This is Devnet, so no real money is used.",
    "Pay within",
    "Or scan with Phantom on your phone",
  ],
  "src/features/bayar/wallet-payment.tsx": ["Wallet connected", "Pay once", "Switch wallet"],
  "src/components/ui.tsx": ["View on Solana Explorer"],
};

it.each(Object.entries(SCREENS))("teks %s sama di layar asli dan mockup landing", (file, labels) => {
  const screen = read(file);
  const landing = read("public/landing.html");
  for (const label of labels) {
    expect(screen).toContain(label);
    expect(landing).toContain(label);
  }
});
