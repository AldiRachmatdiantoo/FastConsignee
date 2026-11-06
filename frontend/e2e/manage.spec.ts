import { expect, login, linkWallet, openActor, test } from "./fixtures";

test("pemilik mengelola barang, dan kasir melihat perubahannya", async ({ browser }) => {
  // Di `next dev` satu reload WebKit bisa makan 10 detik saat mesin sibuk.
  test.setTimeout(120_000);
  const owner = await openActor(browser);
  await login(owner.page, "Pemilik Demo");
  await linkWallet(owner.page);
  const name = `Tes Barang ${Date.now()}`;

  await owner.page.goto("/barang");
  await owner.page.getByRole("button", { name: "Tambah barang" }).click();
  await owner.page.getByLabel("Nama").fill(name);
  await owner.page.getByLabel("Penitip").selectOption({ label: "Sari Anyaman" });
  await owner.page.getByLabel("Harga (Rp)").fill("50000");
  await owner.page.getByLabel("Komisi toko (%)").fill("12.5");
  await owner.page.getByLabel("Stok").fill("7");
  await owner.page.getByLabel("Kirim sertifikat NFT ke pembeli").check();
  await owner.page.getByRole("button", { name: "Simpan" }).click();
  await owner.page.getByLabel("Cari barang").fill(name);
  await expect(owner.page.getByText(name)).toBeVisible();
  await expect(owner.page.getByText(/komisi 12,5%/)).toBeVisible();

  const cashier = await openActor(browser);
  await login(cashier.page, "Kasir Demo");
  await cashier.page.getByLabel("Cari barang").fill(name);
  await expect(cashier.page.getByRole("button", { name: new RegExp(name) })).toBeVisible();

  await owner.page.getByRole("button", { name: `Ubah ${name}` }).click();
  await expect(owner.page.getByLabel("Kirim sertifikat NFT ke pembeli")).toBeChecked();
  await owner.page.getByLabel("Aktif di katalog").uncheck();
  await owner.page.getByRole("button", { name: "Simpan" }).click();
  await expect(owner.page.getByText("Barang tidak ditemukan.")).toBeVisible();
  await owner.page.getByLabel("Tampilkan").selectOption("inactive");
  await expect(owner.page.getByText(`${name} (nonaktif)`)).toBeVisible();

  await expect(async () => {
    await cashier.page.reload();
    await cashier.page.getByLabel("Cari barang").fill(name);
    await expect(cashier.page.getByText("Barang tidak ditemukan.")).toBeVisible({ timeout: 5_000 });
  }).toPass({ timeout: 45_000 });

  expect(owner.errors).toEqual([]);
});

test("email penitip tidak bisa dipakai dua kali", async ({ browser }) => {
  const { page } = await openActor(browser);
  await login(page, "Pemilik Demo");
  await linkWallet(page);
  const email = `tes-${Date.now()}@fastconsignee.test`;

  await page.goto("/penitip");
  for (const expected of ["saved", "taken"]) {
    await page.getByRole("button", { name: "Tambah penitip" }).click();
    await page.getByLabel("Nama").fill("Tes Penitip");
    await page.getByLabel("Email").fill(email);
    await page.getByRole("button", { name: "Simpan" }).click();
    if (expected === "saved") {
      await expect(page.getByText(email)).toBeVisible();
    } else {
      await expect(page.getByText("Email ini sudah dipakai penitip lain.")).toBeVisible();
      await page.getByRole("button", { name: "Batal" }).click();
    }
  }
});
