import { expect, linkWallet, login, openActor, rpc, rupiahDigits, test } from "./fixtures";

// Keranjang Bambu menjaga bagian Sari tetap di atas nol saat potongan refund dari putaran uji sebelumnya dibayar.
const ITEMS = ["Dompet Pandan", "Keranjang Bambu", "Kopi Gayo 250 g", "Syal Batik"];

type StubTransaction = { transfers: { from: string; to: string; lamports: number }[] };

test("alur demo berjalan dari kasir sampai dasbor penitip", async ({ browser }) => {
  const owner = await openActor(browser);
  const consignor = await openActor(browser);
  const cashier = await openActor(browser);
  const buyer = await openActor(browser);

  await test.step("pemilik dan penitip menautkan wallet", async () => {
    await login(owner.page, "Pemilik Demo");
    await linkWallet(owner.page);
    await login(consignor.page, "Sari Anyaman");
    await linkWallet(consignor.page);
  });

  await test.step("kasir menyusun keranjang dari tiga penitip", async () => {
    await login(cashier.page, "Kasir Demo");
    for (const name of ITEMS) {
      await cashier.page.getByRole("button", { name: new RegExp(name) }).click();
    }
    await expect(cashier.page.getByRole("heading", { name: "Rincian pembagian" })).toBeVisible();
    await expect(cashier.page.getByText("Bagian penitip")).toHaveCount(3);
    await expect(cashier.page.getByText("Komisi toko")).toHaveCount(1);
  });

  const total = await test.step("kasir membuat QR", async () => {
    const value = rupiahDigits(await cashier.page.locator("span.text-xl").innerText());
    expect(value).toBe(45000 + 95000 + 85000 + 120000);
    await cashier.page.getByRole("button", { name: "Buat QR pembayaran" }).click();
    await expect(cashier.page.locator("svg title", { hasText: "QR pembayaran" })).toHaveCount(1);
    await expect(cashier.page.getByText(/Menunggu pembayaran/)).toBeVisible();
    return value;
  });

  await test.step("pembeli membayar sekali", async () => {
    const href = await cashier.page.getByRole("link", { name: "Buka layar bayar di browser" }).getAttribute("href");
    await buyer.page.goto(href ?? "");
    await expect(buyer.page.getByText("Toko Titip Nusantara")).toBeVisible();
    await buyer.page.getByRole("button", { name: "Test Wallet" }).click();
    await buyer.page.getByRole("button", { name: "Bayar sekali" }).click();
    await expect(buyer.page.getByText("Pembayaran terkonfirmasi. Terima kasih.")).toBeVisible({ timeout: 30_000 });
    await expect(buyer.page.getByRole("link", { name: "Lihat di Solana Explorer" })).toBeVisible();
  });

  const signature = await test.step("kasir melihat status dan saldo naik bersamaan", async () => {
    await expect(cashier.page.getByText("Pembayaran terkonfirmasi. Barang boleh diserahkan.")).toBeVisible({ timeout: 30_000 });
    await expect(cashier.page.locator("span.text-success")).toHaveCount(4);
    const href = await cashier.page.getByRole("link", { name: "Lihat di Solana Explorer" }).getAttribute("href");
    return new URL(href ?? "").pathname.split("/").pop() ?? "";
  });

  await test.step("satu transaksi memuat empat transfer", async () => {
    const transaction = await rpc<StubTransaction>("stubTransaction", [signature]);
    expect(transaction.transfers).toHaveLength(4);
    expect(new Set(transaction.transfers.map(({ from }) => from))).toEqual(new Set([buyer.keypair.publicKey.toBase58()]));
    expect(transaction.transfers.map(({ to }) => to)).toContain(owner.keypair.publicKey.toBase58());
    expect(transaction.transfers.map(({ to }) => to)).toContain(consignor.keypair.publicKey.toBase58());
    expect(total).toBeGreaterThan(0);
  });

  await test.step("pemilik me-refund satu barang", async () => {
    await owner.page.goto("/refund");
    await owner.page.locator("button[aria-pressed=false]").first().click();
    await expect(owner.page.getByText("Wallet terhubung")).toBeVisible();
    await owner.page.getByRole("button", { name: "Refund satu Dompet Pandan" }).click();
    await expect(owner.page.getByText("Refund Dompet Pandan: Selesai")).toBeVisible({ timeout: 30_000 });
    await expect(owner.page.getByText("1 sudah di-refund")).toBeVisible();
    await expect(owner.page.getByRole("button", { name: "Refund satu Dompet Pandan" })).toBeDisabled();
  });

  await test.step("potongan muncul di dasbor penitip", async () => {
    await consignor.page.goto("/dasbor");
    const deductions = consignor.page.locator("section", { hasText: "Potongan refund" });
    await expect(deductions.getByText("Dompet Pandan").first()).toBeVisible();
    const sales = consignor.page.locator("section", { hasText: "Penjualan" });
    await expect(sales.getByText(/Dompet Pandan × 1/).first()).toBeVisible();
    await expect(sales.getByRole("link", { name: "Explorer" }).first()).toBeVisible();
    await expect(sales.getByText("1 unit sudah di-refund").first()).toBeVisible();
  });

  await test.step("transaksi tampil di dasbor toko", async () => {
    await owner.page.goto("/dasbor");
    await expect(owner.page.getByText("Total penjualan")).toBeVisible();
    await expect(owner.page.getByRole("link", { name: "Explorer" }).first()).toBeVisible();
  });

  for (const actor of [owner, consignor, cashier, buyer]) {
    expect(actor.errors).toEqual([]);
  }
});
