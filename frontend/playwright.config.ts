import { defineConfig } from "@playwright/test";

// Uji ini butuh API dan `next dev` yang sudah berjalan. Caranya ada di README.
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 15_000 },
  workers: 1,
  fullyParallel: false,
  reporter: [["list"]],
  outputDir: "./test-results",
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    // Tombol nonaktif, misalnya stok seed habis, gagal di barisnya sendiri, bukan sebagai timeout uji.
    actionTimeout: 20_000,
    channel: process.env.E2E_BROWSER_CHANNEL ?? "msedge",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "node e2e/solana-stub.mjs",
    url: `http://127.0.0.1:${process.env.SOLANA_STUB_PORT ?? 8899}`,
    reuseExistingServer: true,
  },
});
