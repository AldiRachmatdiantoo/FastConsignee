import { devices } from "@playwright/test";
import { expect, linkWallet, login, openActor, test, type Actor } from "./fixtures";

async function fits(actor: Actor, path: string) {
  await actor.page.goto(path);
  await expect(actor.page.getByRole("heading").first()).toBeVisible();
  const overflow = await actor.page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow, `${path} melebar ${overflow} px`).toBeLessThanOrEqual(0);
}

// Total pergeseran tata letak sejak halaman dibuka sampai data selesai dimuat, tanpa geseran akibat sentuhan pengguna.
async function shiftAfterLoad(actor: Actor, path: string): Promise<number> {
  await actor.page.goto(path);
  await expect(actor.page.getByRole("heading").first()).toBeVisible();
  await expect(actor.page.getByText("Memuat...")).toHaveCount(0);
  await actor.page.waitForTimeout(500);
  return actor.page.evaluate(
    () =>
      new Promise<number>((resolve) => {
        let total = 0;
        new PerformanceObserver((list) => {
          for (const entry of list.getEntries() as (PerformanceEntry & { value: number; hadRecentInput: boolean })[]) {
            total += entry.hadRecentInput ? 0 : entry.value;
          }
        }).observe({ type: "layout-shift", buffered: true });
        setTimeout(() => resolve(total), 200);
      }),
  );
}

test("menu samping tampil di desktop dan terbuka lewat tombol di HP", async ({ browser }) => {
  const owner = await openActor(browser);
  await owner.page.setViewportSize({ width: 1440, height: 900 });
  await login(owner.page, "Pemilik Demo");
  await linkWallet(owner.page);
  await owner.page.goto("/dasbor");
  const menu = owner.page.getByRole("navigation", { name: "Menu" });
  await expect(menu.getByRole("link", { name: "Barang" })).toBeVisible();
  await expect(owner.page.getByRole("button", { name: "Buka menu" })).toBeHidden();

  await owner.page.setViewportSize({ width: 375, height: 812 });
  await expect(menu).toBeHidden();
  await owner.page.getByRole("button", { name: "Buka menu" }).click();
  await menu.getByRole("link", { name: "Barang" }).click();
  await owner.page.waitForURL("**/barang");
  await expect(owner.page.getByRole("heading", { level: 1, name: "Barang" })).toBeVisible();
  await expect(menu).toBeHidden();
  expect(owner.errors).toEqual([]);
});

test("kepala layar kasir tidak bertumpuk di lebar 320 px", async ({ browser }) => {
  const cashier = await openActor(browser);
  await cashier.page.setViewportSize({ width: 320, height: 640 });
  await login(cashier.page, "Kasir Demo");
  const header = cashier.page.locator("header");
  const brand = await header.getByRole("link", { name: "FastConsignee" }).boundingBox();
  const devnet = await header.getByText("Devnet", { exact: true }).boundingBox();
  expect(brand && devnet && brand.x + brand.width <= devnet.x).toBe(true);
  const overflow = await cashier.page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});

test("semua layar muat di lebar 375 px", async ({ browser }) => {
  const owner = await openActor(browser);
  await owner.page.setViewportSize({ width: 375, height: 812 });
  await login(owner.page, "Pemilik Demo");
  await linkWallet(owner.page);
  for (const path of ["/kasir", "/dasbor", "/refund", "/barang", "/penitip", "/wallet"]) {
    await fits(owner, path);
  }

  const consignor = await openActor(browser);
  await consignor.page.setViewportSize({ width: 375, height: 812 });
  await login(consignor.page, "Rina Batik");
  await linkWallet(consignor.page);
  await fits(consignor, "/dasbor");
});

// Kerangka isi menahan tempat data, jadi layar tidak meloncat saat angka dan daftar muncul.
test("layar aplikasi tidak meloncat saat data selesai dimuat di HP", async ({ browser, browserName }) => {
  test.skip(browserName !== "chromium", "Layout Instability API hanya ada di Chromium");
  const owner = await openActor(browser);
  await owner.page.setViewportSize({ width: 390, height: 780 });
  await login(owner.page, "Pemilik Demo");
  await linkWallet(owner.page);
  for (const path of ["/kasir", "/dasbor", "/barang", "/penitip", "/refund"]) {
    const shift = await shiftAfterLoad(owner, path);
    expect(shift, `${path} bergeser ${shift.toFixed(3)}`).toBeLessThan(0.05);
  }

  const consignor = await openActor(browser);
  await consignor.page.setViewportSize({ width: 390, height: 780 });
  await login(consignor.page, "Rina Batik");
  const shift = await shiftAfterLoad(consignor, "/dasbor");
  expect(shift, `dasbor penitip bergeser ${shift.toFixed(3)}`).toBeLessThan(0.05);
  expect(owner.errors).toEqual([]);
});

test("layar bayar di HP tanpa wallet menawarkan buka di aplikasi Phantom", async ({ browser }) => {
  const cashier = await openActor(browser);
  await login(cashier.page, "Kasir Demo");
  await cashier.page.getByRole("button", { name: /Kopi Toraja 250 g/ }).click();
  await cashier.page.getByRole("button", { name: "Buat QR pembayaran" }).click();
  const href = await cashier.page.getByRole("link", { name: "Buka layar bayar di browser" }).getAttribute("href");
  // Seperti Safari di iPhone: aplikasi Phantom sudah terpasang, tetapi browser biasa tidak mendapat wallet darinya.
  const { userAgent, viewport, deviceScaleFactor, isMobile, hasTouch } = devices["iPhone 15"];
  const phone = await browser.newContext({ userAgent, viewport, deviceScaleFactor, isMobile, hasTouch });
  const page = await phone.newPage();
  await page.goto(href ?? "");
  await page.getByRole("button", { name: "Bayar dengan wallet" }).click();
  const open = page.getByRole("link", { name: "Buka di aplikasi Phantom" });
  await expect(open).toBeVisible();
  const url = page.url();
  await expect(open).toHaveAttribute("href", `https://phantom.com/ul/browse/${encodeURIComponent(url)}?ref=${encodeURIComponent(new URL(url).origin)}`);
  await expect(page.getByText("Belum ada wallet di browser ini")).toHaveCount(0);
  await phone.close();
  await cashier.context.close();
});
