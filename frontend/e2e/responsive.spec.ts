import { expect, linkWallet, login, openActor, test, type Actor } from "./fixtures";

async function fits(actor: Actor, path: string) {
  await actor.page.goto(path);
  await expect(actor.page.getByRole("heading").first()).toBeVisible();
  const overflow = await actor.page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow, `${path} melebar ${overflow} px`).toBeLessThanOrEqual(0);
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
