import { expect, login, linkWallet, openActor, test } from "./fixtures";

test("pemilik mengelola barang, dan kasir melihat perubahannya", async ({ browser }) => {
  // Di `next dev` satu reload WebKit bisa makan 10 detik saat mesin sibuk.
  test.setTimeout(120_000);
  const owner = await openActor(browser);
  await login(owner.page, "Demo Owner");
  await linkWallet(owner.page);
  const name = `Tes Barang ${Date.now()}`;

  await owner.page.goto("/barang");
  await owner.page.getByRole("button", { name: "Add item" }).click();
  await owner.page.getByLabel("Name").fill(name);
  await owner.page.getByLabel("Consignor").selectOption({ label: "Sari Anyaman" });
  await owner.page.getByLabel("Price (Rp)").fill("50000");
  await owner.page.getByLabel("Shop commission (%)").fill("12.5");
  await owner.page.getByLabel("Stock").fill("7");
  await owner.page.getByLabel("Send an NFT certificate to the buyer").check();
  await owner.page.getByRole("button", { name: "Save" }).click();
  await owner.page.getByLabel("Search items").fill(name);
  await expect(owner.page.getByText(name)).toBeVisible();
  // Barang dari uji sebelumnya bisa masih tampil sampai pencarian selesai menyaring.
  await expect(owner.page.getByText(/commission 12,5%/)).toHaveCount(1);
  await expect(owner.page.getByText(/commission 12,5%/)).toBeVisible();

  const cashier = await openActor(browser);
  await login(cashier.page, "Demo Cashier");
  await cashier.page.getByLabel("Search items").fill(name);
  await expect(cashier.page.getByRole("button", { name: new RegExp(name) })).toBeVisible();

  await owner.page.getByRole("button", { name: `Edit ${name}` }).click();
  await expect(owner.page.getByLabel("Send an NFT certificate to the buyer")).toBeChecked();
  await owner.page.getByLabel("Active in catalog").uncheck();
  await owner.page.getByRole("button", { name: "Save" }).click();
  await expect(owner.page.getByText("No items found.")).toBeVisible();
  await owner.page.getByLabel("Show").selectOption("inactive");
  await expect(owner.page.getByText(`${name} (nonaktif)`)).toBeVisible();

  await expect(async () => {
    await cashier.page.reload();
    await cashier.page.getByLabel("Search items").fill(name);
    await expect(cashier.page.getByText("No items found.")).toBeVisible({ timeout: 5_000 });
  }).toPass({ timeout: 45_000 });

  expect(owner.errors).toEqual([]);
});

test("email penitip tidak bisa dipakai dua kali", async ({ browser }) => {
  const { page } = await openActor(browser);
  await login(page, "Demo Owner");
  await linkWallet(page);
  const email = `tes-${Date.now()}@fastconsignee.test`;

  await page.goto("/penitip");
  for (const expected of ["saved", "taken"]) {
    await page.getByRole("button", { name: "Add consignor" }).click();
    await page.getByLabel("Name").fill("Tes Penitip");
    await page.getByLabel("Email").fill(email);
    await page.getByRole("button", { name: "Save" }).click();
    if (expected === "saved") {
      await expect(page.getByText(email)).toBeVisible();
    } else {
      await expect(page.getByText("This email is already used by another consignor.")).toBeVisible();
      await page.getByRole("button", { name: "Cancel" }).click();
    }
  }
});
