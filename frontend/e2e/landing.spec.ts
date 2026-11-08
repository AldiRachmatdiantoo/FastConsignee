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
  expect(text).toContain("Create payment QR");
  expect(text).toContain("Pay once");
  expect(text).not.toMatch(/Demo shop|Approved once/);
  for (const screen of await phone.locator(".phone__page").all()) {
    const lines = (await screen.textContent()) ?? "";
    expect(lines.includes("Waiting for payment") && lines.includes("Payment confirmed")).toBe(false);
  }
});

// Animasi ikut scroll menyala di HP dan desktop. Animasi yang menggeser elemen hanya memakai translate, scale, rotate,
// atau opacity, dan tiap properti hanya punya satu animasi per elemen, supaya GPU menjalankannya seiring jari dan HP tidak bergetar.
test("landing page beranimasi di layar sentuh dan desktop dengan animasi yang bisa dijalankan GPU", async ({ browser }) => {
  for (const options of [{ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true }, { viewport: { width: 1440, height: 900 } }]) {
    const context = await browser.newContext(options);
    const page = await context.newPage();
    await page.goto("/");
    test.skip(!(await page.evaluate(() => CSS.supports("animation-timeline: view()"))), "browser ini belum mendukung animation-timeline");
    await expect(page.locator("html")).toHaveAttribute("data-8b-live", "");
    // Judul dipecah per baris dan diberi animasi reveal saat mendekati layar, jadi semua judul didatangi dulu.
    await page.evaluate(async () => {
      for (const element of document.querySelectorAll("[data-8b-split]")) {
        element.scrollIntoView({ block: "center" });
        await new Promise((resolve) => setTimeout(resolve, 80));
      }
      scrollTo(0, 0);
    });
    await expect(page.locator('[data-8b-motion~="captionIn"] .line__in').first()).toBeAttached();
    const report = await page.evaluate(() => {
      const moving = new Set(["translate", "scale", "rotate", "transform", "opacity"]);
      const meta = new Set(["offset", "computedOffset", "easing", "composite"]);
      const seen = new Map<Element, Set<string>>();
      const problems: string[] = [];
      let count = 0;
      for (const animation of document.getAnimations()) {
        if (!(animation.effect instanceof KeyframeEffect)) {
          continue;
        }
        // Safari menjalankan animasi waktu ber-easing linear() atau steps() di thread utama, jadi reveal yang terpicu saat scroll ikut tersendat.
        // CSS animation menaruh animation-timing-function di tiap keyframe, bukan di timing efeknya.
        if (animation.timeline === document.timeline) {
          const easings = [animation.effect.getTiming().easing, ...animation.effect.getKeyframes().map((frame) => frame.easing)];
          if (easings.some((easing) => /linear\(|steps\(/.test(easing ?? ""))) {
            problems.push(`${(animation as CSSAnimation).animationName} memakai easing linear() atau steps() pada animasi waktu`);
          }
          continue;
        }
        count++;
        const name = (animation as CSSAnimation).animationName;
        const target = animation.effect.target!;
        const props = new Set(animation.effect.getKeyframes().flatMap((frame) => Object.keys(frame).filter((key) => !meta.has(key))));
        const moves = [...props].filter((prop) => moving.has(prop));
        // Chromium tidak menyebut custom property di getKeyframes, jadi keyframes tanpa properti berarti custom property.
        if (props.size === 0 || [...props].some((prop) => prop.startsWith("--"))) {
          problems.push(`${name} menganimasikan custom property`);
        }
        if (moves.length > 0 && moves.length < props.size) {
          problems.push(`${name} mencampur gerakan dengan properti yang tidak bisa dijalankan GPU`);
        }
        const used = seen.get(target) ?? new Set<string>();
        for (const prop of moves) {
          if (used.has(prop)) {
            problems.push(`${name} menumpuk animasi ${prop} di elemen yang sama`);
          }
          used.add(prop);
        }
        seen.set(target, used);
      }
      return { count, problems };
    });
    expect(report.count).toBeGreaterThan(50);
    expect(report.problems).toEqual([]);
    await context.close();
  }
});

// Di Safari iPhone, elemen sticky di dalam induk yang memotong overflow bergetar saat toolbar mengecil atau membesar.
// Clip boleh ada di elemen sticky itu sendiri atau di dalamnya, tidak di induknya.
test("elemen sticky di landing tidak berada di dalam induk yang memotong overflow", async ({ browser }) => {
  for (const viewport of [{ width: 390, height: 844 }, { width: 1024, height: 768 }]) {
    const context = await browser.newContext({ viewport, hasTouch: true, isMobile: viewport.width < 768 });
    const page = await context.newPage();
    await page.goto("/");
    test.skip(!(await page.evaluate(() => CSS.supports("animation-timeline: view()"))), "browser ini belum mendukung animation-timeline");
    await expect(page.locator("html")).toHaveAttribute("data-8b-live", "");
    const report = await page.evaluate(() => {
      const clipped: string[] = [];
      let sticky = 0;
      for (const element of document.querySelectorAll("body *")) {
        if (getComputedStyle(element).position !== "sticky") {
          continue;
        }
        sticky++;
        for (let node = element.parentElement; node && node !== document.body; node = node.parentElement) {
          const style = getComputedStyle(node);
          if (style.overflowX !== "visible" || style.overflowY !== "visible") {
            clipped.push(`${element.className} di dalam ${node.id || node.className}`);
          }
        }
      }
      return { sticky, clipped };
    });
    expect(report.sticky).toBeGreaterThanOrEqual(4);
    expect(report.clipped, `lebar ${viewport.width} px`).toEqual([]);
    await context.close();
  }
});

// Kartu contoh aturan memudar di atas layar HP saat masuk, jadi saat di-scroll balik ia mengecil ke HP sambil memudar, tidak hilang mendadak.
test("kartu Every rupiah muncul dan hilang perlahan saat di-scroll", async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
  const page = await context.newPage();
  await page.goto("/");
  test.skip(!(await page.evaluate(() => CSS.supports("animation-timeline: view()"))), "browser ini belum mendukung animation-timeline");
  await expect(page.locator("#tune")).toHaveAttribute("data-handoff", "");
  const opacityAt = async (share: number) => {
    await page.evaluate((share) => {
      const tune = document.getElementById("tune")!;
      scrollTo(0, tune.getBoundingClientRect().top + scrollY - innerHeight * (1 - share));
    }, share);
    await page.waitForTimeout(150);
    return page.evaluate(() => Number(getComputedStyle(document.querySelector("#tune .tune__stage")!).opacity));
  };
  for (const pass of ["turun", "naik"]) {
    const opacity = new Map<number, number>();
    for (const share of pass === "turun" ? [0, 0.1, 0.4] : [0.4, 0.1, 0]) {
      opacity.set(share, await opacityAt(share));
    }
    expect(opacity.get(0), `saat ${pass}`).toBeLessThanOrEqual(0.01);
    expect(opacity.get(0.1), `saat ${pass}`).toBeGreaterThan(0.2);
    expect(opacity.get(0.1), `saat ${pass}`).toBeLessThan(0.8);
    expect(opacity.get(0.4), `saat ${pass}`).toBe(1);
  }
  await context.close();
});

test("label suara di HP tidak tertimpa gelombang", async ({ browser }) => {
  for (const width of [320, 390]) {
    const context = await browser.newContext({ viewport: { width, height: 800 }, hasTouch: true, isMobile: true });
    const page = await context.newPage();
    await page.goto("/");
    await page.evaluate(() => document.fonts.ready);
    const gap = await page.evaluate(() => {
      const label = document.querySelector("#listen .listen__panel--voice .listen__label")!.getBoundingClientRect();
      const wave = document.querySelector("#listen .listen__panel--voice .listen__wave")!.getBoundingClientRect();
      return wave.left - label.right;
    });
    expect(gap, `lebar ${width} px`).toBeGreaterThanOrEqual(4);
    await context.close();
  }
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
        band: band.content !== "none" && band.backgroundImage.includes("gradient") && parseFloat(band.width) >= document.documentElement.clientWidth - 1,
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

test("judul bagian tidak menempel di bawah navbar setelah lompat lewat menu", async ({ browser }) => {
  const screens = [
    { viewport: { width: 393, height: 659 }, phone: true },
    { viewport: { width: 375, height: 812 }, phone: true },
    { viewport: { width: 1366, height: 768 }, phone: false },
  ];
  for (const { viewport, phone } of screens) {
    const context = await browser.newContext({ viewport, hasTouch: phone, isMobile: phone });
    const page = await context.newPage();
    await page.goto("/");
    await page.evaluate(() => document.fonts.ready);
    for (const id of ["listen", "calls", "rooms", "voices", "faq"]) {
      await page.evaluate(() => scrollTo({ top: 0, behavior: "instant" }));
      if (phone) {
        await page.locator("#nav .nav__toggle").click();
        await page.locator(`#menu a[href="#${id}"]`).click();
      } else {
        await page.locator(`#nav .nav__links a[href="#${id}"]`).click();
      }
      await expect.poll(() => page.evaluate(() => location.hash)).toBe(`#${id}`);
      const gap = await page.evaluate(
        (id) =>
          new Promise<number>((resolve) =>
            requestAnimationFrame(() => {
              const title = document.querySelector(`#${id} h2`)!.getBoundingClientRect();
              const bar = document.querySelector("#nav .nav__bar")!.getBoundingClientRect();
              resolve(title.top - bar.bottom);
            }),
          ),
        id,
      );
      expect(gap, `#${id} pada ${viewport.width} x ${viewport.height}`).toBeGreaterThanOrEqual(16);
    }
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
    await expect(page.getByRole("heading", { level: 1, name: "Every sale, split instantly." })).toBeVisible();
    await expect(page.getByRole("heading", { level: 2, name: "Sign in" })).toBeVisible();

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow).toBeLessThanOrEqual(0);
    expect(errors).toEqual([]);

    await page.getByRole("link", { name: "Back to home" }).click();
    await expect(page.getByRole("heading", { level: 1, name: "Every sale, split instantly." })).toBeVisible();
  }
});

// Foto latar yang membesar saat halaman login dibuka tidak boleh membuat halaman sempat bisa di-scroll,
// karena scrollbar muncul sebentar lalu hilang dan seluruh isi halaman bergeser.
test("halaman login tidak sempat bisa di-scroll saat animasi pembuka", async ({ browser }) => {
  for (const viewport of [{ width: 1920, height: 990 }, { width: 1366, height: 768 }]) {
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();
    await page.addInitScript(() => {
      const samples: number[] = [];
      const state = { samples, done: false };
      Object.assign(window, { scrollSampling: state });
      // Jendela 1,5 detik dihitung dari frame pertama, karena di jaringan lambat CSS bisa menahan render lebih lama dari itu,
      // dan animasi pembuka baru berjalan setelah halaman dirender.
      let start: number | undefined;
      const tick = (now: number) => {
        start ??= now;
        const root = document.documentElement;
        if (root) {
          samples.push(root.scrollHeight - root.clientHeight);
        }
        if (now - start < 1500) {
          requestAnimationFrame(tick);
        } else {
          state.done = true;
        }
      };
      requestAnimationFrame(tick);
    });
    type Sampling = { scrollSampling: { samples: number[]; done: boolean } };
    await page.goto("/login");
    await expect(page.getByRole("heading", { level: 2, name: "Sign in" })).toBeVisible();
    await page.waitForFunction(() => (window as unknown as Sampling).scrollSampling.done);
    const samples = await page.evaluate(() => (window as unknown as Sampling).scrollSampling.samples);
    expect(samples.length).toBeGreaterThan(10);
    expect(Math.max(...samples), `${viewport.width} x ${viewport.height}`).toBeLessThanOrEqual(0);
    await context.close();
  }
});

test("latar di sekitar halaman login di HP gelap, bukan putih", async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 780 }, isMobile: true, hasTouch: true });
  const page = await context.newPage();
  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 2, name: "Sign in" })).toBeVisible();
  const result = await page.evaluate(() => {
    const login = document.querySelector(".login") as HTMLElement;
    return {
      login: getComputedStyle(login).backgroundColor,
      html: getComputedStyle(document.documentElement).backgroundColor,
      body: getComputedStyle(document.body).backgroundColor,
      gap: document.documentElement.scrollHeight - login.getBoundingClientRect().bottom,
    };
  });
  // Safari iOS 26 memakai latar html dan body untuk area di bawah toolbarnya, jadi warnanya harus sama dengan halaman login.
  expect(result.html).toBe(result.login);
  expect(result.body).toBe(result.login);
  // scrollHeight dibulatkan ke piksel penuh, jadi selisih di bawah 1 px hanya pembulatan.
  expect(result.gap).toBeLessThan(1);
  await context.close();
});

test("halaman login di HP kembali ke atas saat di-refresh", async ({ browser }) => {
  // Tinggi 560 px membuat isi login lebih tinggi dari layar, baik dengan maupun tanpa tombol akun demo.
  const context = await browser.newContext({ viewport: { width: 390, height: 560 }, isMobile: true, hasTouch: true });
  const page = await context.newPage();
  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 2, name: "Sign in" })).toBeVisible();
  const maxScroll = await page.evaluate(() => document.documentElement.scrollHeight - innerHeight);
  expect(maxScroll).toBeGreaterThan(0);
  // Heading sudah tampil dari HTML server sebelum React aktif, jadi tunggu halaman tenang dulu supaya scroll tidak didahului hydration.
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(500);
  await page.evaluate(() => scrollTo(0, document.documentElement.scrollHeight));
  await page.waitForTimeout(300);
  expect(await page.evaluate(() => scrollY)).toBeGreaterThan(0);
  await page.reload();
  await expect(page.getByRole("heading", { level: 2, name: "Sign in" })).toBeVisible();
  await page.waitForTimeout(500);
  expect(await page.evaluate(() => scrollY)).toBe(0);
  await expect(page.locator(".login__nav")).toBeInViewport();
  await context.close();
});

test("isi halaman login tetap terbaca tanpa animasi", async ({ browser }) => {
  const context = await browser.newContext({ reducedMotion: "reduce" });
  const page = await context.newPage();
  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 2, name: "Sign in" })).toBeVisible();
  await expect(page.getByRole("button", { name: /Demo Cashier/ })).toBeVisible();
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
    await expect(page.getByRole("heading", { name: "Page not found" })).toBeVisible();
  }
  await page.getByRole("link", { name: "Back to home" }).click();
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
