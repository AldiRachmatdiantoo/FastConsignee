import { createPrivateKey, sign } from "node:crypto";
import { expect, test as base, type Browser, type BrowserContext, type Page, type Request } from "@playwright/test";
import { Keypair, Transaction } from "@solana/web3.js";
import bs58 from "bs58";

export const STUB_URL = `http://127.0.0.1:${process.env.SOLANA_STUB_PORT ?? 8899}`;

const PKCS8_ED25519_PREFIX = Buffer.from("302e020100300506032b657004220420", "hex");

// rpc memanggil Solana tiruan langsung dari test.
export async function rpc<T>(method: string, params: unknown[]): Promise<T> {
  const response = await fetch(STUB_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
  });
  const body = (await response.json()) as { result?: T; error?: { message: string } };
  if (body.error) {
    throw new Error(body.error.message);
  }
  return body.result as T;
}

function signMessage(keypair: Keypair, message: Buffer): number[] {
  const seed = Buffer.from(keypair.secretKey.subarray(0, 32));
  const key = createPrivateKey({ key: Buffer.concat([PKCS8_ED25519_PREFIX, seed]), format: "der", type: "pkcs8" });
  return Array.from(sign(null, message, key));
}

type WalletInit = { address: string; publicKey: number[] };

// Wallet ini berperilaku seperti Phantom: mendaftar lewat Wallet Standard, lalu menandatangani dan mengirim sendiri.
function registerFakeWallet({ address, publicKey }: WalletInit) {
  type Listener = (event: { accounts: unknown[] }) => void;
  type Fake = Window & {
    fakeSignMessage: (bytes: number[]) => Promise<number[]>;
    fakeSignAndSend: (bytes: number[]) => Promise<number[]>;
  };
  const page = window as unknown as Fake;
  const account = {
    address,
    publicKey: new Uint8Array(publicKey),
    chains: ["solana:devnet"],
    features: ["solana:signMessage", "solana:signTransaction", "solana:signAndSendTransaction"],
  };
  const listeners: Listener[] = [];
  const wallet = {
    version: "1.0.0",
    name: "Test Wallet",
    icon: "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciLz4=",
    chains: ["solana:devnet"],
    accounts: [] as unknown[],
    features: {
      "standard:connect": {
        version: "1.0.0",
        connect: async () => {
          wallet.accounts = [account];
          listeners.forEach((listener) => listener({ accounts: wallet.accounts }));
          return { accounts: wallet.accounts };
        },
      },
      "standard:disconnect": {
        version: "1.0.0",
        disconnect: async () => {
          wallet.accounts = [];
          listeners.forEach((listener) => listener({ accounts: wallet.accounts }));
        },
      },
      "standard:events": {
        version: "1.0.0",
        on: (_event: string, listener: Listener) => {
          listeners.push(listener);
          return () => {};
        },
      },
      "solana:signMessage": {
        version: "1.0.0",
        signMessage: async (...inputs: { message: Uint8Array }[]) =>
          Promise.all(
            inputs.map(async ({ message }) => ({
              signedMessage: message,
              signature: new Uint8Array(await page.fakeSignMessage(Array.from(message))),
            })),
          ),
      },
      "solana:signTransaction": {
        version: "1.0.0",
        supportedTransactionVersions: ["legacy", 0],
        signTransaction: async () => {
          throw new Error("Pakai signAndSendTransaction");
        },
      },
      "solana:signAndSendTransaction": {
        version: "1.0.0",
        supportedTransactionVersions: ["legacy", 0],
        signAndSendTransaction: async (...inputs: { transaction: Uint8Array }[]) =>
          Promise.all(
            inputs.map(async ({ transaction }) => ({
              signature: new Uint8Array(await page.fakeSignAndSend(Array.from(transaction))),
            })),
          ),
      },
    },
  };
  const callback = ({ register }: { register: (candidate: unknown) => void }) => register(wallet);
  window.dispatchEvent(new CustomEvent("wallet-standard:register-wallet", { detail: callback }));
  window.addEventListener("wallet-standard:app-ready", (event) =>
    callback((event as CustomEvent<{ register: (candidate: unknown) => void }>).detail),
  );
}

// Browser memanggil RPC Devnet, jadi panggilannya dialihkan ke Solana tiruan.
async function routeRpc(context: BrowserContext) {
  await context.route("https://api.devnet.solana.com/**", async (route) => {
    const request = route.request();
    if (request.method() !== "POST") {
      await route.fulfill({ status: 204, headers: { "Access-Control-Allow-Origin": "*", "Access-Control-Allow-Headers": "*" } });
      return;
    }
    const response = await fetch(STUB_URL, { method: "POST", body: request.postData() ?? "" });
    await route.fulfill({
      status: response.status,
      contentType: "application/json",
      headers: { "Access-Control-Allow-Origin": "*" },
      body: await response.text(),
    });
  });
}

const running = new WeakMap<Page, Set<Request>>();

function trackRequests(page: Page) {
  const active = new Set<Request>();
  running.set(page, active);
  page.on("request", (request) => active.add(request));
  page.on("requestfinished", (request) => active.delete(request));
  page.on("requestfailed", (request) => active.delete(request));
}

// WebKit mencatat fetch yang terputus karena pindah halaman sebagai error "access control checks".
// `networkidle` tidak cukup, karena pindah layar di sisi klien tidak mengulang status itu.
async function settled(page: Page) {
  let quiet = 0;
  await expect
    .poll(
      () => {
        quiet = running.get(page)?.size ? 0 : quiet + 1;
        return quiet;
      },
      { intervals: [150] },
    )
    .toBeGreaterThanOrEqual(3);
}

export type Actor = { context: BrowserContext; page: Page; errors: string[]; keypair: Keypair };

export async function openActor(browser: Browser, keypair = Keypair.generate()): Promise<Actor> {
  const context = await browser.newContext();
  await routeRpc(context);
  await context.exposeFunction("fakeSignMessage", (bytes: number[]) => signMessage(keypair, Buffer.from(bytes)));
  await context.exposeFunction("fakeSignAndSend", async (bytes: number[]) => {
    const transaction = Transaction.from(Buffer.from(bytes));
    transaction.partialSign(keypair);
    const signature = await rpc<string>("sendTransaction", [transaction.serialize().toString("base64")]);
    return Array.from(bs58.decode(signature));
  });
  await context.addInitScript(registerFakeWallet, {
    address: keypair.publicKey.toBase58(),
    publicKey: Array.from(keypair.publicKey.toBytes()),
  });

  const page = await context.newPage();
  trackRequests(page);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") {
      errors.push(message.text());
    }
  });
  return { context, page, errors, keypair };
}

export async function login(page: Page, name: string) {
  await page.goto("/login");
  await page.getByRole("button", { name: new RegExp(name) }).click();
  await expect(page.getByRole("button", { name: "Keluar" })).toBeVisible();
  await settled(page);
}

// linkStoreWallet menautkan wallet aktor ke toko. Aman diulang, karena wallet lama akan diganti.
export async function linkWallet(page: Page) {
  await page.goto("/wallet");
  await page.getByRole("button", { name: "Test Wallet" }).click();
  await page.getByRole("button", { name: "Tandatangani dan tautkan" }).click();
  await expect(page.getByText("Wallet tertaut:")).toBeVisible();
  await settled(page);
}

// paidOrder membuat satu order lunas, jadi uji tidak bergantung pada sisa data uji lain.
export async function paidOrder(browser: Browser, item: string) {
  const cashier = await openActor(browser);
  const buyer = await openActor(browser);
  await login(cashier.page, "Kasir Demo");
  await cashier.page.getByRole("button", { name: new RegExp(item) }).click();
  await cashier.page.getByRole("button", { name: "Buat QR pembayaran" }).click();
  const href = await cashier.page.getByRole("link", { name: "Buka layar bayar di browser" }).getAttribute("href");
  await buyer.page.goto(href ?? "");
  await buyer.page.getByRole("button", { name: "Test Wallet" }).click();
  await buyer.page.getByRole("button", { name: "Bayar sekali" }).click();
  await expect(buyer.page.getByText("Pembayaran terkonfirmasi. Terima kasih.")).toBeVisible({ timeout: 30_000 });
  await cashier.context.close();
  await buyer.context.close();
}

export function rupiahDigits(text: string): number {
  return Number(text.replace(/\D/g, ""));
}

export const test = base;
export { expect };
