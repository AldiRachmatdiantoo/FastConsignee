import { expect, openActor, test } from "./fixtures";

const SIZES = [
  { width: 1440, height: 900 },
  { width: 375, height: 812 },
];

test("landing page tampil tanpa error dan tanpa scroll samping", async ({ browser }) => {
  for (const size of SIZES) {
    const { page, errors } = await openActor(browser);
    await page.setViewportSize(size);
    await page.goto("/");
    await expect(page).toHaveTitle(/FastConsignee/);
    await expect(page.getByRole("heading", { level: 1, name: "Every sale, split instantly." })).toBeVisible();

    const text = await page.locator("body").innerText();
    expect(text).not.toMatch(/linea|hearing|caption/i);
    expect(text).toContain("Devnet");

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow).toBeLessThanOrEqual(0);
    expect(errors).toEqual([]);
  }
});

test("HP di landing page memakai layar asli dan satu status di tiap layar", async ({ browser }) => {
  const { page } = await openActor(browser);
  await page.goto("/");
  const phone = page.locator("#calls .phone");
  await expect(phone).toHaveAttribute("role", "img");
  await expect(phone.locator("[data-step]")).toHaveCount(4);
  const text = (await phone.textContent()) ?? "";
  expect(text).toContain("Buat QR pembayaran");
  expect(text).toContain("Bayar sekali");
  expect(text).not.toMatch(/Demo shop|Waiting for payment|Approved once/);
  for (const screen of await phone.locator(".phone__page").all()) {
    const lines = (await screen.textContent()) ?? "";
    expect(lines.includes("Menunggu pembayaran") && lines.includes("terkonfirmasi")).toBe(false);
  }
});

test("tombol masuk di landing page sampai ke halaman login", async ({ browser }) => {
  const { page } = await openActor(browser);
  await page.goto("/");
  await page.getByRole("link", { name: "Staff sign in" }).first().click();
  await page.waitForURL("**/login");
});

test("halaman login muat di layar dan bisa kembali ke landing page", async ({ browser }) => {
  for (const size of SIZES) {
    const { page, errors } = await openActor(browser);
    await page.setViewportSize(size);
    await page.goto("/login");
    await expect(page.getByRole("heading", { level: 1, name: "Setiap penjualan, langsung terbagi." })).toBeVisible();
    await expect(page.getByRole("heading", { level: 2, name: "Masuk" })).toBeVisible();

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow).toBeLessThanOrEqual(0);
    expect(errors).toEqual([]);

    await page.getByRole("link", { name: "Kembali ke beranda" }).click();
    await expect(page.getByRole("heading", { level: 1, name: "Every sale, split instantly." })).toBeVisible();
  }
});

test("isi halaman login tetap terbaca tanpa animasi", async ({ browser }) => {
  const context = await browser.newContext({ reducedMotion: "reduce" });
  const page = await context.newPage();
  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 2, name: "Masuk" })).toBeVisible();
  await expect(page.getByRole("button", { name: /Kasir Demo/ })).toBeVisible();
  const animations = await page.evaluate(() => document.getAnimations().length);
  expect(animations).toBe(0);
  await context.close();
});

test("isi landing page tetap terbaca tanpa animasi", async ({ browser }) => {
  const context = await browser.newContext({ reducedMotion: "reduce" });
  const page = await context.newPage();
  await page.goto("/");
  await expect(page.locator("html")).not.toHaveAttribute("data-8b-live", /.*/);
  for (const heading of ["One screen at the counter.", "Your shop, your rules.", "Questions, answered plainly."]) {
    await expect(page.getByRole("heading", { name: heading })).toBeVisible();
  }
  await expect(page.getByRole("link", { name: "Source: KY3" })).toBeVisible();
  await context.close();
});

test("alamat yang salah memberi halaman 404 berbahasa Indonesia", async ({ page }) => {
  for (const path of ["/tidak-ada", "/bayar/bukan-uuid"]) {
    const response = await page.goto(path);
    expect(response?.status()).toBe(404);
    await expect(page.getByRole("heading", { name: "Halaman tidak ditemukan" })).toBeVisible();
  }
  await page.getByRole("link", { name: "Kembali ke beranda" }).click();
  await expect(page).toHaveURL(/\/$/);
});

test("halaman mengirim header keamanan", async ({ request }) => {
  for (const path of ["/", "/login", "/landing/inter-latin.woff2"]) {
    const { "x-frame-options": frame, "x-content-type-options": sniff, "content-security-policy": policy } = (await request.get(path)).headers();
    expect(frame).toBe("DENY");
    expect(sniff).toBe("nosniff");
    expect(policy).toContain("frame-ancestors 'none'");
  }
});
