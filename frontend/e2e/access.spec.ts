import { Keypair } from "@solana/web3.js";
import { expect, linkWallet, login, openActor, paidOrder, test } from "./fixtures";

test("tanpa sesi diarahkan ke login", async ({ browser }) => {
  const { page } = await openActor(browser);
  await page.goto("/dasbor");
  await page.waitForURL("**/login");
});

test("kasir hanya bisa membuka layar kasir", async ({ browser }) => {
  const { page } = await openActor(browser);
  await login(page, "Kasir Demo");
  for (const path of ["/dasbor", "/refund", "/barang", "/penitip", "/wallet"]) {
    await page.goto(path);
    await page.waitForURL("**/kasir");
  }
  await expect(page.getByRole("navigation", { name: "Menu" })).toHaveCount(0);
});

test("penitip tidak bisa membuka layar toko", async ({ browser }) => {
  const { page } = await openActor(browser);
  await login(page, "Budi Kopi");
  await linkWallet(page);
  for (const path of ["/kasir", "/refund", "/barang", "/penitip"]) {
    await page.goto(path);
    await page.waitForURL("**/dasbor");
  }
  await expect(page.getByRole("link", { name: "Barang" })).toHaveCount(0);
});

test("refund menolak wallet selain wallet toko", async ({ browser }) => {
  const owner = await openActor(browser);
  await login(owner.page, "Pemilik Demo");
  await linkWallet(owner.page);
  await paidOrder(browser, "Kopi Toraja 250 g");

  const other = await openActor(browser, Keypair.generate());
  await login(other.page, "Pemilik Demo");
  await other.page.goto("/refund");
  await expect(other.page.getByRole("heading", { name: "Order terkonfirmasi" })).toBeVisible();
  await other.page.locator("button[aria-pressed=false]").first().click();
  await other.page.getByRole("button", { name: "Test Wallet" }).click();
  await expect(other.page.getByText("Ini bukan wallet toko.")).toBeVisible();
  await expect(other.page.getByRole("button", { name: /^Refund satu/ }).first()).toBeDisabled();
});

test("API putus saat reload tidak mengeluarkan pengguna", async ({ browser }) => {
  const cashier = await openActor(browser);
  await login(cashier.page, "Kasir Demo");
  const api = `${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"}/**`;

  await cashier.context.route(api, (route) => route.abort());
  await cashier.page.reload();
  await expect(cashier.page.getByRole("heading", { name: "Belum tersambung" })).toBeVisible();
  await expect(cashier.page).toHaveURL(/\/kasir$/);

  await cashier.context.unroute(api);
  await cashier.page.getByRole("button", { name: "Coba lagi" }).click();
  await expect(cashier.page.getByRole("button", { name: "Keluar" })).toBeVisible();
  await expect(cashier.page).toHaveURL(/\/kasir$/);
});
