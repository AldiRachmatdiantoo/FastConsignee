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

// Efek yang mengikuti scroll sebagian dihitung di thread utama, dan di HP layar sentuh gerakannya tertinggal dari jari sehingga terlihat bergetar.
test("landing page di layar sentuh tampil diam, sedangkan di desktop tetap beranimasi", async ({ browser }) => {
  const touch = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
  const phone = await touch.newPage();
  await phone.goto("/");
  const fine = await phone.evaluate(() => matchMedia("(hover: hover) and (pointer: fine)").matches);
  test.skip(fine, "browser ini tidak meniru layar sentuh");
  await expect(phone.locator("html")).not.toHaveAttribute("data-8b-live", /.*/);
  await expect(phone.getByRole("heading", { name: "One screen at the counter." })).toBeVisible();
  await touch.close();

  const desktop = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await desktop.newPage();
  await page.goto("/");
  if (await page.evaluate(() => CSS.supports("animation-timeline: view()"))) {
    await expect(page.locator("html")).toHaveAttribute("data-8b-live", "");
  }
  await desktop.close();
});

test("mockup HP di bagian kasir tidak menutupi teks dan muat di layar HP", async ({ browser }) => {
  for (const viewport of [{ width: 390, height: 844 }, { width: 360, height: 640 }]) {
    const context = await browser.newContext({ viewport, hasTouch: true, isMobile: true });
    const page = await context.newPage();
    await page.goto("/");
    await page.locator("#calls .calls__item").nth(2).scrollIntoViewIfNeeded();
    await page.evaluate(() => scrollBy(0, innerHeight * 0.2));
    const layout = await page.evaluate(() => {
      const sticky = document.querySelector("#calls .calls__sticky")!;
      const band = getComputedStyle(sticky, "::before");
      const phone = document.querySelector("#calls .phone")!.getBoundingClientRect();
      const list = getComputedStyle(document.querySelector("#calls .calls__list")!);
      return {
        band: band.content !== "none" && band.backgroundImage.includes("gradient") && parseFloat(band.width) >= innerWidth,
        phoneHeight: phone.height,
        phoneBottom: phone.bottom,
        listPadding: parseFloat(list.paddingBottom),
        innerHeight,
      };
    });
    expect(layout.band, `pita di belakang HP pada ${viewport.width} px`).toBe(true);
    expect(layout.phoneHeight).toBeLessThanOrEqual(layout.innerHeight * 0.43);
    expect(layout.phoneBottom).toBeLessThanOrEqual(layout.innerHeight);
    expect(layout.listPadding).toBeGreaterThanOrEqual(layout.phoneHeight);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - innerWidth);
    expect(overflow).toBeLessThanOrEqual(0);
    await context.close();
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
